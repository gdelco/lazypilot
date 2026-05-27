package tui

import (
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
	border := s.Border.Width(width).Height(height)
	if len(w.groups) == 0 {
		return border.Render(s.Dim.Render("\n  no worktrees found. Press n on a repo to create one."))
	}

	var rows []string
	flatIdx := 0
	for _, g := range w.groups {
		rows = append(rows, s.Heading.Render(displayPath(g.SourceRepo, w.app.home)))
		for _, wt := range g.Worktrees {
			label := "  " + s.IconWorktree.Render(scan.KindWorktree.Icon()) + " " + displayPath(wt.Path, w.app.home)
			if flatIdx == w.cursor {
				label = s.ListSelected.Render(label)
			} else {
				label = s.ListItem.Render(label)
			}
			rows = append(rows, label)
			flatIdx++
		}
	}

	visible := height - 2
	if visible > 0 && len(rows) > visible {
		rows = rows[:visible]
	}
	return border.Render(strings.Join(rows, "\n"))
}

func (w worktreesModel) renderDetail(width, height int) string {
	s := w.app.styles
	border := s.Border.Width(width).Height(height)
	if len(w.flat) == 0 {
		return border.Render("")
	}
	wt := w.flat[w.cursor].Worktree
	var b strings.Builder
	b.WriteString(s.Heading.Render(wt.Path) + "\n\n")
	b.WriteString(s.Heading.Render("Source repo") + ": " + wt.SourceRepo + "\n")
	b.WriteString(s.Heading.Render("Branch") + ": " + gitBranch(wt.Path) + "\n\n")
	b.WriteString(s.Heading.Render("Status") + ":\n")
	st := gitStatus(wt.Path)
	if st == "" {
		b.WriteString("  " + s.OK.Render("(clean)") + "\n")
	} else {
		b.WriteString(st)
	}
	return border.Render(b.String())
}

func (w worktreesModel) handleKey(m tea.KeyMsg, k keymap) (worktreesModel, tea.Cmd) {
	switch {
	case keyMatches(m, k.Up):
		if w.cursor > 0 {
			w.cursor--
		}
	case keyMatches(m, k.Down):
		if w.cursor < len(w.flat)-1 {
			w.cursor++
		}
	case keyMatches(m, k.Open):
		if w.cursor < len(w.flat) {
			path := w.flat[w.cursor].Worktree.Path
			return w, func() tea.Msg {
				name := tmuxctl.SessionNameFor(path)
				if !tmuxctl.HasSession(name) {
					_ = tmuxctl.NewSession(name, path)
				}
				return attachRequestMsg{name: name}
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
