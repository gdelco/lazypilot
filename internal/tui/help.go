package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpModal is a full keybinding reference, dismissed with any key.
type helpModal struct{}

func newHelp() *helpModal { return &helpModal{} }

// Update closes the modal on any keypress.
func (h *helpModal) Update(msg tea.Msg) (cmd tea.Cmd, done bool) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return nil, true
	}
	return nil, false
}

func (h *helpModal) View(s styles, screenW, screenH int) string {
	width := 78
	if width > screenW-6 {
		width = screenW - 6
	}

	row := func(key, desc string) string {
		return s.FooterKey.Render(padRight(key, 14)) + s.Dim.Render("  ") + desc
	}

	section := func(name string) string {
		return "\n" + s.Heading.Render("▸ "+strings.ToUpper(name)) + "\n"
	}

	var b strings.Builder

	b.WriteString(section("navigation"))
	b.WriteString(row("j / ↓", "down") + "\n")
	b.WriteString(row("k / ↑", "up") + "\n")
	b.WriteString(row("g / G", "first / last") + "\n")
	b.WriteString(row("ctrl-d / ctrl-u", "half-page down / up") + "\n")
	b.WriteString(row("/", "filter the current view") + "\n")

	b.WriteString(section("views"))
	b.WriteString(row("1 / 2 / 3", "Sessions / Projects / Worktrees") + "\n")
	b.WriteString(row("tab / shift-tab", "cycle views") + "\n")

	b.WriteString(section("actions"))
	b.WriteString(row("enter", "open / attach the tmux session for selected entry") + "\n")
	b.WriteString(row("n", "new worktree (on a repo or worktree row)") + "\n")
	b.WriteString(row("d", "remove worktree (with confirmation)") + "\n")
	b.WriteString(row("K", "kill the tmux session for selected entry") + "\n")
	b.WriteString(row("r", "reload (rescan projects + tmux + AI detection)") + "\n")

	b.WriteString(section("status icons"))
	b.WriteString(row(s.Warn.Render("◐"), "working — agent is actively responding") + "\n")
	b.WriteString(row(s.Bad.Render("!"), "needs input — agent is waiting on you") + "\n")
	b.WriteString(row(s.OK.Render("○"), "idle — agent is alive and ready") + "\n")
	b.WriteString(row(s.Dim.Render("·"), "unknown — no AI process detected") + "\n")

	b.WriteString(section("type icons (projects view)"))
	b.WriteString(row(s.IconRepo.Render("󰊢"), "git repository") + "\n")
	b.WriteString(row(s.IconWorktree.Render("󰘬"), "linked git worktree") + "\n")
	b.WriteString(row(s.IconWorkSp.Render("󰉋"), "workspace / non-git folder") + "\n")

	b.WriteString(section("config"))
	b.WriteString(row("~/.config/lazypilot/config.yaml", "") + "\n")
	b.WriteString(s.Dim.Render("    keys: roots, branch_prefix, ai_processes, refresh_interval") + "\n")

	b.WriteString("\n" + s.Dim.Render("    press any key to close"))

	box := s.Panel("HELP — KEYBINDINGS", b.String(), width, lipgloss.Height(b.String()), true)
	return lipgloss.Place(screenW, screenH, lipgloss.Center, lipgloss.Center, box)
}

func padRight(s string, n int) string {
	if w := lipgloss.Width(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}
