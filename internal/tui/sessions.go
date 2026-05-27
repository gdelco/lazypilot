package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gdelco/lazypilot/internal/tmuxctl"
)

type sessionsModel struct {
	app    *App
	all    []tmuxctl.Session
	cursor int
}

func newSessionsModel(a *App) sessionsModel {
	return sessionsModel{app: a}
}

func (s sessionsModel) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, _ := tmuxctl.ListSessions()
		return sessionsRefreshedMsg(sessions)
	}
}

func (s *sessionsModel) applyRefresh(m sessionsRefreshedMsg) {
	s.all = []tmuxctl.Session(m)
	if s.cursor >= len(s.all) {
		s.cursor = max(0, len(s.all)-1)
	}
}

func (s sessionsModel) view(width, height int) string {
	listW := width * 4 / 10
	detailW := width - listW - 2
	if listW < 25 {
		listW = 25
	}
	if detailW < 30 {
		detailW = 30
	}
	list := s.renderList(listW, height)
	detail := s.renderDetail(detailW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}

func (s sessionsModel) renderList(width, height int) string {
	st := s.app.styles
	border := st.Border.Width(width).Height(height)
	if len(s.all) == 0 {
		return border.Render(st.Dim.Render("\n  no tmux sessions running."))
	}
	var rows []string
	visible := height - 2
	if visible < 1 {
		visible = 1
	}
	for i := 0; i < min(len(s.all), visible); i++ {
		sess := s.all[i]
		marker := "  "
		if sess.Attached {
			marker = st.OK.Render("● ")
		}
		label := marker + sess.Name
		if i == s.cursor {
			label = st.ListSelected.Render(label)
		} else {
			label = st.ListItem.Render(label)
		}
		rows = append(rows, label)
	}
	for len(rows) < visible {
		rows = append(rows, "")
	}
	return border.Render(strings.Join(rows, "\n"))
}

func (s sessionsModel) renderDetail(width, height int) string {
	st := s.app.styles
	if len(s.all) == 0 {
		return st.Border.Width(width).Height(height).Render("")
	}
	sess := s.all[s.cursor]
	var b strings.Builder
	b.WriteString(st.Heading.Render(sess.Name) + "\n")
	b.WriteString(st.Dim.Render(sess.Path) + "\n\n")

	panes, _ := tmuxctl.ListPanesIn(sess.Name)
	b.WriteString(st.Heading.Render("Panes") + ":\n")
	if len(panes) == 0 {
		b.WriteString(st.Dim.Render("  (none)\n"))
	}
	for _, p := range panes {
		b.WriteString("  • " + p.Label() + "\n")
	}
	return st.Border.Width(width).Height(height).Render(b.String())
}

func (s sessionsModel) handleKey(m tea.KeyMsg, k keymap) (sessionsModel, tea.Cmd) {
	switch {
	case keyMatches(m, k.Up):
		if s.cursor > 0 {
			s.cursor--
		}
	case keyMatches(m, k.Down):
		if s.cursor < len(s.all)-1 {
			s.cursor++
		}
	case keyMatches(m, k.Open):
		if s.cursor < len(s.all) {
			name := s.all[s.cursor].Name
			return s, func() tea.Msg { return attachRequestMsg{name: name} }
		}
	}
	return s, nil
}
