package tui

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gdelco/lazypilot/internal/scan"
	"github.com/gdelco/lazypilot/internal/tmuxctl"
)

type projectsModel struct {
	app     *App
	all     []scan.Project
	cursor  int
	offset  int // scroll offset
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
	border := s.Border.Width(width).Height(height)
	if len(p.all) == 0 {
		return border.Render(s.Dim.Render("\n  no projects found.\n  edit ~/.config/lazypilot/config.yaml\n  to add roots."))
	}

	rows := []string{}
	visibleRows := height - 2 // border top + bottom
	if visibleRows < 1 {
		visibleRows = 1
	}
	// keep cursor visible
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+visibleRows {
		p.offset = p.cursor - visibleRows + 1
	}
	end := p.offset + visibleRows
	if end > len(p.all) {
		end = len(p.all)
	}

	for i := p.offset; i < end; i++ {
		proj := p.all[i]
		icon := p.iconFor(proj.Kind)
		label := proj.Short(p.app.home)
		// Truncate to fit
		labelW := width - 6
		if labelW < 10 {
			labelW = 10
		}
		if len(label) > labelW {
			label = "…" + label[len(label)-labelW+1:]
		}
		row := icon + " " + label
		if i == p.cursor {
			row = s.ListSelected.Render(row)
		} else {
			row = s.ListItem.Render(row)
		}
		rows = append(rows, row)
	}
	for len(rows) < visibleRows {
		rows = append(rows, s.ListDim.Render(""))
	}

	return border.Render(strings.Join(rows, "\n"))
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
	if len(p.all) == 0 {
		return s.Border.Width(width).Height(height).Render("")
	}
	proj := p.all[p.cursor]

	var b strings.Builder
	b.WriteString(s.Heading.Render(proj.Path))
	b.WriteString("\n\n")

	if proj.Kind == scan.KindRepo || proj.Kind == scan.KindWorktree {
		b.WriteString(s.Heading.Render("Branch") + ": " + gitBranch(proj.Path) + "\n\n")
		b.WriteString(s.Heading.Render("Status") + ":\n")
		st := gitStatus(proj.Path)
		if st == "" {
			b.WriteString("  " + s.OK.Render("(clean)") + "\n")
		} else {
			b.WriteString(st)
		}
		b.WriteString("\n")
		b.WriteString(s.Heading.Render("Recent commits") + ":\n")
		b.WriteString(gitLog(proj.Path, 5))
		b.WriteString("\n")
		if proj.Kind == scan.KindWorktree && proj.SourceRepo != "" {
			b.WriteString(s.Dim.Render("Source repo: "+proj.SourceRepo) + "\n")
		}
	} else {
		b.WriteString(s.Heading.Render("Contents") + ":\n")
		b.WriteString(listDir(proj.Path, 12))
	}

	// Session info
	name := tmuxctl.SessionNameFor(proj.Path)
	b.WriteString("\n")
	if tmuxctl.HasSession(name) {
		b.WriteString(s.Heading.Render("tmux session") + ": " + s.OK.Render(name+" (running)") + "\n")
		panes, _ := tmuxctl.ListPanesIn(name)
		for _, pn := range panes {
			b.WriteString("  • " + pn.Label() + "\n")
		}
	} else {
		b.WriteString(s.Heading.Render("tmux session") + ": " + s.Dim.Render("(none)") + "\n")
	}

	content := b.String()
	return s.Border.Width(width).Height(height).Render(content)
}

func (p projectsModel) handleKey(m tea.KeyMsg, k keymap) (projectsModel, tea.Cmd) {
	switch {
	case keyMatches(m, k.Up):
		if p.cursor > 0 {
			p.cursor--
		}
	case keyMatches(m, k.Down):
		if p.cursor < len(p.all)-1 {
			p.cursor++
		}
	case keyMatches(m, k.Top):
		p.cursor = 0
	case keyMatches(m, k.Bottom):
		p.cursor = max(0, len(p.all)-1)
	case keyMatches(m, k.HalfUp):
		p.cursor = max(0, p.cursor-10)
	case keyMatches(m, k.HalfDown):
		p.cursor = min(len(p.all)-1, p.cursor+10)
	case keyMatches(m, k.Open):
		if p.cursor < len(p.all) {
			return p, p.openCmd(p.all[p.cursor].Path)
		}
	}
	return p, nil
}

func (p projectsModel) openCmd(path string) tea.Cmd {
	return func() tea.Msg {
		name := tmuxctl.SessionNameFor(path)
		if !tmuxctl.HasSession(name) {
			_ = tmuxctl.NewSession(name, path)
		}
		return attachRequestMsg{name: name}
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

