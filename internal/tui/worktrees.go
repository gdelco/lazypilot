package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gdelco/lazypilot/internal/scan"
	"github.com/gdelco/lazypilot/internal/tmuxctl"
)

// worktreesModel groups projects of kind worktree by their source repo.
type worktreesModel struct {
	app    *App
	groups []worktreeGroup
	flat   []worktreeEntry // flattened for cursor navigation (only worktree rows)
	cursor int
	filter filterState
}

func (w worktreesModel) filteredFlat() []worktreeEntry {
	if w.filter.text == "" {
		return w.flat
	}
	out := []worktreeEntry{}
	for _, e := range w.flat {
		if w.filter.Matches(e.Worktree.Path) || w.filter.Matches(e.Worktree.SourceRepo) {
			out = append(out, e)
		}
	}
	return out
}

type worktreeGroup struct {
	SourceRepo string
	Worktrees  []scan.Project
}

type worktreeEntry struct {
	GroupIndex int
	Worktree   scan.Project
}

func newWorktreesModel(a *App) worktreesModel {
	return worktreesModel{app: a}
}

func (w *worktreesModel) applyFromProjects(projects []scan.Project) {
	byRepo := map[string][]scan.Project{}
	for _, p := range projects {
		if p.Kind != scan.KindWorktree {
			continue
		}
		key := p.SourceRepo
		if key == "" {
			key = "(unknown source)"
		}
		byRepo[key] = append(byRepo[key], p)
	}

	w.groups = nil
	keys := make([]string, 0, len(byRepo))
	for k := range byRepo {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w.flat = nil
	for _, k := range keys {
		group := worktreeGroup{SourceRepo: k, Worktrees: byRepo[k]}
		idx := len(w.groups)
		w.groups = append(w.groups, group)
		for _, wt := range group.Worktrees {
			w.flat = append(w.flat, worktreeEntry{GroupIndex: idx, Worktree: wt})
		}
	}
	if w.cursor >= len(w.flat) {
		w.cursor = max(0, len(w.flat)-1)
	}
}

func (w worktreesModel) view(width, height int) string {
	listW := width * 5 / 10
	detailW := width - listW - 2
	if listW < 30 {
		listW = 30
	}
	list := w.renderList(listW, height)
	detail := w.renderDetail(detailW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}

func (w worktreesModel) renderList(width, height int) string {
	s := w.app.styles
	filtered := w.filteredFlat()
	title := fmt.Sprintf("Worktrees (%d/%d)", len(filtered), len(w.flat))
	if len(w.groups) == 0 {
		return s.Panel(title, s.Dim.Render("\n  no worktrees found. Press n on a repo to create one."), width, height, true)
	}

	// When filtering, render a flat list. Without filter, render grouped.
	var rows []string
	if w.filter.text != "" {
		if w.cursor >= len(filtered) {
			w.cursor = max(0, len(filtered)-1)
		}
		for i, e := range filtered {
			label := s.IconWorktree.Render(scan.KindWorktree.Icon()) + " " + displayPath(e.Worktree.Path, w.app.home)
			row := s.Cursor(i == w.cursor)
			if i == w.cursor {
				row += s.ListSelected.Render(label)
			} else {
				row += s.ListItem.Render(label)
			}
			rows = append(rows, row)
		}
	} else {
		flatIdx := 0
		for _, g := range w.groups {
			rows = append(rows, s.Dim.Render("  "+displayPath(g.SourceRepo, w.app.home)))
			for _, wt := range g.Worktrees {
				label := s.IconWorktree.Render(scan.KindWorktree.Icon()) + " " + displayPath(wt.Path, w.app.home)
				row := s.Cursor(flatIdx == w.cursor)
				if flatIdx == w.cursor {
					row += s.ListSelected.Render(label)
				} else {
					row += s.ListItem.Render(label)
				}
				rows = append(rows, row)
				flatIdx++
			}
		}
	}

	filterBar := w.filter.Render(s)
	visible := height
	if filterBar != "" {
		visible--
	}
	if visible > 0 && len(rows) > visible {
		rows = rows[:visible]
	}
	if filterBar != "" {
		rows = append(rows, filterBar)
	}
	return s.Panel(title, strings.Join(rows, "\n"), width, height, true)
}

func (w worktreesModel) renderDetail(width, height int) string {
	s := w.app.styles
	title := "Details"
	items := w.filteredFlat()
	if len(items) == 0 {
		return s.Panel(title, "", width, height, false)
	}
	if w.cursor >= len(items) {
		w.cursor = len(items) - 1
	}
	wt := items[w.cursor].Worktree
	section := func(name string) string { return s.Heading.Render("▸ " + name) }
	var b strings.Builder
	b.WriteString(s.Heading.Render(wt.Path) + "\n\n")
	b.WriteString(section("Source repo") + "  " + wt.SourceRepo + "\n")
	b.WriteString(section("Branch") + "  " + gitBranch(wt.Path) + "\n\n")
	b.WriteString(section("Status") + "\n")
	st := gitStatus(wt.Path)
	if st == "" {
		b.WriteString("  " + s.OK.Render("(clean)") + "\n")
	} else {
		b.WriteString(st)
	}
	return s.Panel(title, b.String(), width, height, false)
}

// renderPreview shows `git diff --stat` against the worktree's source repo.
func (w worktreesModel) renderPreview(width, height int) string {
	s := w.app.styles
	title := "Diff"
	items := w.filteredFlat()
	if len(items) == 0 {
		return s.Panel(title, "", width, height, false)
	}
	if w.cursor >= len(items) {
		w.cursor = len(items) - 1
	}
	wt := items[w.cursor].Worktree

	var b strings.Builder
	b.WriteString(s.Heading.Render("▸ Diff (staged + unstaged)") + "\n")
	stat, _ := exec.Command("git", "-C", wt.Path,
		"-c", "color.ui=always",
		"diff", "--stat", "HEAD").Output()
	if len(stat) == 0 {
		b.WriteString(s.Dim.Render("  (no changes)") + "\n")
	} else {
		b.WriteString(string(stat))
	}

	b.WriteString("\n" + s.Heading.Render("▸ Recent commits") + "\n")
	out, _ := exec.Command("git", "-C", wt.Path,
		"-c", "color.ui=always",
		"log", "--oneline", "--decorate", fmt.Sprintf("-%d", height-10)).Output()
	b.WriteString(string(out))

	content := clipToBox(b.String(), width-4, height)
	return s.Panel(title, content, width, height, false)
}

func (w worktreesModel) handleKey(m tea.KeyMsg, k keymap) (worktreesModel, tea.Cmd) {
	if w.filter.active {
		w.filter.Update(m)
		w.cursor = 0
		return w, nil
	}
	items := w.filteredFlat()
	switch {
	case keyMatches(m, k.Filter):
		w.filter.Begin()
	case keyMatches(m, k.Up):
		if w.cursor > 0 {
			w.cursor--
		}
	case keyMatches(m, k.Down):
		if w.cursor < len(items)-1 {
			w.cursor++
		}
	case keyMatches(m, k.Open):
		if w.cursor < len(items) {
			path := items[w.cursor].Worktree.Path
			return w, func() tea.Msg {
				name := tmuxctl.SessionNameFor(path)
				if tmuxctl.HasSession(name) {
					return attachRequestMsg{name: name}
				}
				return aiPickerMsg{target: name, dir: path}
			}
		}
	case keyMatches(m, k.NewWT):
		if w.cursor < len(items) {
			return w, requestCreateWorktree(items[w.cursor].Worktree.Path)
		}
	case keyMatches(m, k.Remove):
		if w.cursor < len(items) {
			return w, requestRemoveWorktree(items[w.cursor].Worktree.Path)
		}
	case keyMatches(m, k.KillSesh):
		if w.cursor < len(items) {
			path := items[w.cursor].Worktree.Path
			name := tmuxctl.SessionNameFor(path)
			if tmuxctl.HasSession(name) {
				return w, requestKillSession(name)
			}
		}
	}
	return w, nil
}

func displayPath(p, home string) string {
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
