package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gdelco/lazypilot/internal/gitctl"
	"github.com/gdelco/lazypilot/internal/scan"
	"github.com/gdelco/lazypilot/internal/tmuxctl"
)

func getenv(k string) string { return os.Getenv(k) }

type projectsModel struct {
	app    *App
	all    []scan.Project
	cursor int
	offset int // scroll offset
	filter filterState
}

// filtered returns the subset of projects that pass the current filter.
func (p projectsModel) filtered() []scan.Project {
	if p.filter.text == "" {
		return p.all
	}
	out := []scan.Project{}
	for _, x := range p.all {
		if p.filter.Matches(x.Path) {
			out = append(out, x)
		}
	}
	return out
}

func newProjectsModel(a *App) projectsModel {
	return projectsModel{app: a}
}

// scanCmd kicks off the filesystem scan asynchronously.
func (p projectsModel) scanCmd() tea.Cmd {
	roots := p.app.roots
	return func() tea.Msg {
		return projectsScannedMsg(scan.Collect(roots))
	}
}

func (p *projectsModel) applyScan(m projectsScannedMsg) {
	p.all = []scan.Project(m)
	if p.cursor >= len(p.all) {
		p.cursor = max(0, len(p.all)-1)
	}
}

func (p projectsModel) view(width, height int) string {
	listW := width * 4 / 10
	detailW := width - listW - 2
	if listW < 25 {
		listW = 25
	}
	if detailW < 30 {
		detailW = 30
	}

	list := p.renderList(listW, height)
	detail := p.renderDetail(detailW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}

func (p projectsModel) renderList(width, height int) string {
	s := p.app.styles
	items := p.filtered()
	title := fmt.Sprintf("Projects (%d/%d)", len(items), len(p.all))

	if len(p.all) == 0 {
		return s.Panel(title, s.Dim.Render("\n  no projects found.\n  edit ~/.config/lazypilot/config.yaml\n  to add roots."), width, height, true)
	}

	// Reserve a row at the bottom for the filter prompt if it's active or set.
	filterBar := p.filter.Render(s)
	visibleRows := height
	if filterBar != "" {
		visibleRows--
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	if p.cursor >= len(items) {
		p.cursor = max(0, len(items)-1)
	}
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+visibleRows {
		p.offset = p.cursor - visibleRows + 1
	}
	end := p.offset + visibleRows
	if end > len(items) {
		end = len(items)
	}

	rows := []string{}
	labelW := width - 8
	if labelW < 10 {
		labelW = 10
	}
	for i := p.offset; i < end; i++ {
		proj := items[i]
		icon := p.iconFor(proj.Kind)
		label := proj.Short(p.app.home)
		if len(label) > labelW {
			label = "…" + label[len(label)-labelW+1:]
		}
		row := s.Cursor(i == p.cursor) + icon + " "
		if i == p.cursor {
			row += s.ListSelected.Render(label)
		} else {
			row += s.ListItem.Render(label)
		}
		rows = append(rows, row)
	}
	for len(rows) < visibleRows {
		rows = append(rows, "")
	}
	if filterBar != "" {
		rows = append(rows, filterBar)
	}

	return s.Panel(title, strings.Join(rows, "\n"), width, height, true)
}

// renderPreview shows a recent-activity digest for the selected project:
// branch / tracking state, a status summary (X staged, Y modified, Z
// untracked), and the last few commits. For non-repo workspaces, a depth-2
// file tree so you can see project structure at a glance.
func (p projectsModel) renderPreview(width, height int) string {
	s := p.app.styles
	items := p.filtered()
	title := "Activity"
	if len(items) == 0 {
		return s.Panel(title, "", width, height, false)
	}
	if p.cursor >= len(items) {
		p.cursor = len(items) - 1
	}
	proj := items[p.cursor]

	var b strings.Builder
	if proj.Kind == scan.KindRepo || proj.Kind == scan.KindWorktree {
		b.WriteString(s.Heading.Render("▸ Branch") + "\n")
		b.WriteString("  " + gitBranch(proj.Path))
		if track := gitTracking(proj.Path); track != "" {
			b.WriteString("  " + s.Dim.Render(track))
		}
		b.WriteString("\n\n")

		staged, modified, untracked := gitStatusCounts(proj.Path)
		b.WriteString(s.Heading.Render("▸ Status") + "\n")
		if staged+modified+untracked == 0 {
			b.WriteString("  " + s.OK.Render("clean") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s staged · %s modified · %s untracked\n",
				s.OK.Render(fmt.Sprintf("%d", staged)),
				s.Warn.Render(fmt.Sprintf("%d", modified)),
				s.Dim.Render(fmt.Sprintf("%d", untracked))))
		}
		b.WriteString("\n")

		if n := gitStashCount(proj.Path); n > 0 {
			b.WriteString(s.Heading.Render("▸ Stash") + fmt.Sprintf("  %d entries\n\n", n))
		}

		b.WriteString(s.Heading.Render("▸ Recent commits") + "\n")
		out, _ := exec.Command("git", "-C", proj.Path,
			"-c", "color.ui=always",
			"log", "--pretty=format:  %C(auto)%h%Creset %s %C(dim)(%cr)%Creset", "-10").Output()
		b.WriteString(string(out))
	} else {
		b.WriteString(s.Heading.Render("▸ Tree") + "\n")
		b.WriteString(fileTree(proj.Path, 2, 20))
	}

	content := clipToBox(b.String(), width-4, height)
	return s.Panel(title, content, width, height, false)
}

// gitTracking returns a compact string like "↑2 ↓1" (ahead/behind) when the
// branch tracks a remote; empty when not tracking.
func gitTracking(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-list", "--left-right", "--count", "HEAD...@{u}").Output()
	if err != nil {
		return ""
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return ""
	}
	ahead, _ := strconv.Atoi(parts[0])
	behind, _ := strconv.Atoi(parts[1])
	if ahead == 0 && behind == 0 {
		return "(in sync)"
	}
	pieces := []string{}
	if ahead > 0 {
		pieces = append(pieces, fmt.Sprintf("↑%d", ahead))
	}
	if behind > 0 {
		pieces = append(pieces, fmt.Sprintf("↓%d", behind))
	}
	return strings.Join(pieces, " ")
}

// gitStatusCounts returns counts of (staged, modified-in-worktree, untracked).
func gitStatusCounts(repo string) (staged, modified, untracked int) {
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain=v1").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' && y == '?' {
			untracked++
			continue
		}
		if x != ' ' && x != '?' {
			staged++
		}
		if y != ' ' && y != '?' {
			modified++
		}
	}
	return
}

func gitStashCount(repo string) int {
	out, err := exec.Command("git", "-C", repo, "stash", "list").Output()
	if err != nil {
		return 0
	}
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// fileTree renders a depth-limited directory listing of `root`.
// Up to `maxEntriesPerDir` entries per level; hidden files skipped except .git.
func fileTree(root string, maxDepth, maxEntriesPerDir int) string {
	var b strings.Builder
	var walk func(dir, prefix string, depth int)
	walk = func(dir, prefix string, depth int) {
		if depth > maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		count := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			count++
			if count > maxEntriesPerDir {
				b.WriteString(prefix + "… (more)\n")
				return
			}
			marker := "├── "
			if count == len(entries) || count == maxEntriesPerDir {
				marker = "└── "
			}
			b.WriteString(prefix + marker + e.Name() + "\n")
			if e.IsDir() && depth < maxDepth {
				next := prefix + "    "
				walk(filepath.Join(dir, e.Name()), next, depth+1)
			}
		}
	}
	b.WriteString("  " + root + "\n")
	walk(root, "  ", 1)
	return b.String()
}

func (p projectsModel) iconFor(k scan.Kind) string {
	s := p.app.styles
	switch k {
	case scan.KindRepo:
		return s.IconRepo.Render(k.Icon())
	case scan.KindWorktree:
		return s.IconWorktree.Render(k.Icon())
	default:
		return s.IconWorkSp.Render(k.Icon())
	}
}

func (p projectsModel) renderDetail(width, height int) string {
	s := p.app.styles
	title := "Details"
	items := p.filtered()
	if len(items) == 0 {
		return s.Panel(title, "", width, height, false)
	}
	if p.cursor >= len(items) {
		p.cursor = len(items) - 1
	}
	proj := items[p.cursor]

	var b strings.Builder
	b.WriteString(s.Heading.Render(proj.Path) + "\n\n")

	section := func(name string) string { return s.Heading.Render("▸ " + name) }

	if proj.Kind == scan.KindRepo || proj.Kind == scan.KindWorktree {
		b.WriteString(section("Branch") + "  " + gitBranch(proj.Path) + "\n\n")
		b.WriteString(section("Status") + "\n")
		st := gitStatus(proj.Path)
		if st == "" {
			b.WriteString("  " + s.OK.Render("(clean)") + "\n")
		} else {
			b.WriteString(st)
		}
		b.WriteString("\n")
		b.WriteString(section("Recent commits") + "\n")
		b.WriteString(gitLog(proj.Path, 5))
		b.WriteString("\n")
		if proj.Kind == scan.KindWorktree && proj.SourceRepo != "" {
			b.WriteString(s.Dim.Render("Source repo: "+proj.SourceRepo) + "\n")
		}
	} else {
		b.WriteString(section("Contents") + "\n")
		b.WriteString(listDir(proj.Path, 12))
	}

	// Session info
	name := tmuxctl.SessionNameFor(proj.Path)
	b.WriteString("\n")
	if tmuxctl.HasSession(name) {
		b.WriteString(section("tmux session") + "  " + s.OK.Render(name+" (running)") + "\n")
		panes, _ := tmuxctl.ListPanesIn(name)
		for _, pn := range panes {
			b.WriteString("  • " + pn.Label() + "\n")
		}
	} else {
		b.WriteString(section("tmux session") + "  " + s.Dim.Render("(none)") + "\n")
	}

	return s.Panel(title, b.String(), width, height, false)
}

func (p projectsModel) handleKey(m tea.KeyMsg, k keymap) (projectsModel, tea.Cmd) {
	// If the filter is capturing keys, route there first.
	if p.filter.active {
		p.filter.Update(m)
		p.cursor = 0 // jump to top whenever the filter text changes
		return p, nil
	}

	items := p.filtered()
	switch {
	case keyMatches(m, k.Filter):
		p.filter.Begin()
	case keyMatches(m, k.Up):
		if p.cursor > 0 {
			p.cursor--
		}
	case keyMatches(m, k.Down):
		if p.cursor < len(items)-1 {
			p.cursor++
		}
	case keyMatches(m, k.Top):
		p.cursor = 0
	case keyMatches(m, k.Bottom):
		p.cursor = max(0, len(items)-1)
	case keyMatches(m, k.HalfUp):
		p.cursor = max(0, p.cursor-10)
	case keyMatches(m, k.HalfDown):
		p.cursor = min(len(items)-1, p.cursor+10)
	case keyMatches(m, k.Open):
		if p.cursor < len(items) {
			return p, p.openCmd(items[p.cursor].Path)
		}
	case keyMatches(m, k.NewWT):
		if p.cursor < len(items) {
			proj := items[p.cursor]
			if proj.Kind == scan.KindRepo || proj.Kind == scan.KindWorktree {
				return p, requestCreateWorktree(proj.Path)
			}
		}
	case keyMatches(m, k.Remove):
		if p.cursor < len(items) {
			proj := items[p.cursor]
			if proj.Kind == scan.KindWorktree {
				return p, requestRemoveWorktree(proj.Path)
			}
		}
	case keyMatches(m, k.KillSesh):
		if p.cursor < len(items) {
			path := items[p.cursor].Path
			name := tmuxctl.SessionNameFor(path)
			if tmuxctl.HasSession(name) {
				return p, requestKillSession(name)
			}
		}
	}
	return p, nil
}

// --- Modal-launch helpers (return tea.Cmds that emit confirmMsg) ---

func requestCreateWorktree(repo string) tea.Cmd {
	return func() tea.Msg {
		return wizardMsg{wizard: newCreateWizard(repo, branchPrefixFromEnv())}
	}
}

func branchPrefixFromEnv() string {
	// Honor the same env var the bash sessionizer used.
	return getenv("TMUX_SESSIONIZER_BRANCH_PREFIX")
}

func requestRemoveWorktree(path string) tea.Cmd {
	return func() tea.Msg {
		dirty := gitctl.IsDirty(path)
		branch := gitctl.CurrentBranch(path)
		msg := "Remove worktree:\n  " + path + "\n  branch: " + branch
		if dirty {
			msg += "\n  ⚠ uncommitted changes — removal will fail without --force"
		}
		return confirmMsg{
			modal: newConfirm("REMOVE WORKTREE", msg, func() tea.Cmd {
				return func() tea.Msg {
					_ = gitctl.RemoveWorktree(path, false)
					return rescanMsg{status: "worktree removed: " + path}
				}
			}),
		}
	}
}

func requestKillSession(name string) tea.Cmd {
	return func() tea.Msg {
		return confirmMsg{
			modal: newConfirm("KILL SESSION", "Kill tmux session '"+name+"'?", func() tea.Cmd {
				return func() tea.Msg {
					_ = tmuxctl.KillSession(name)
					return rescanMsg{status: "session killed: " + name}
				}
			}),
		}
	}
}

// openCmd: if the tmux session already exists, attach. Otherwise pop the
// AI picker so the user can decide how to lay out the new session.
func (p projectsModel) openCmd(path string) tea.Cmd {
	return func() tea.Msg {
		name := tmuxctl.SessionNameFor(path)
		if tmuxctl.HasSession(name) {
			return attachRequestMsg{name: name}
		}
		return aiPickerMsg{target: name, dir: path}
	}
}

// --- git helpers (small, run sync; called only on cursor change for one item) ---

func gitBranch(repo string) string {
	out, err := exec.Command("git", "-C", repo, "branch", "--show-current").Output()
	if err != nil {
		return "(detached)"
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "(detached)"
	}
	return s
}

func gitStatus(repo string) string {
	out, err := exec.Command("git", "-C", repo, "status", "-s").Output()
	if err != nil {
		return ""
	}
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return ""
	}
	// Indent each line.
	var b strings.Builder
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if i >= 12 {
			b.WriteString("  …\n")
			break
		}
		b.WriteString("  " + l + "\n")
	}
	return b.String()
}

func gitLog(repo string, n int) string {
	out, err := exec.Command("git", "-C", repo, "log", "--oneline", "--decorate", fmt.Sprintf("-%d", n)).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func listDir(path string, max int) string {
	out, err := exec.Command("ls", "-la", path).Output()
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	var b strings.Builder
	count := 0
	for scanner.Scan() {
		count++
		if count > max {
			b.WriteString("  …\n")
			break
		}
		b.WriteString("  " + scanner.Text() + "\n")
	}
	return b.String()
}

