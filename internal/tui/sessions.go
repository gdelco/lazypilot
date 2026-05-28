package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/gdelco/lazypilot/internal/detect"
	"github.com/gdelco/lazypilot/internal/opencodehook"
	"github.com/gdelco/lazypilot/internal/tmuxctl"
)

type sessionsModel struct {
	app      *App
	all      []tmuxctl.Session
	panes    []paneStatus // flat list across all sessions, refreshed on tick
	statusBy map[string]detect.Status // session name → aggregated status
	cursor   int
	// paneCursor selects a pane within the currently selected session.
	// Active only when paneFocus is true. j/k navigates this cursor instead
	// of the session list, and Enter attaches to that specific pane.
	paneCursor int
	paneFocus  bool
	filter     filterState
	aiList     []string // recognized AI process names, for IsAI matching
}

// paneStatus is a tmux pane plus its classified AI status (StatusUnknown if not AI).
type paneStatus struct {
	tmuxctl.Pane
	Status detect.Status
}

func (s sessionsModel) filtered() []tmuxctl.Session {
	if s.filter.text == "" {
		return s.all
	}
	out := []tmuxctl.Session{}
	for _, x := range s.all {
		if s.filter.Matches(x.Name) || s.filter.Matches(x.Path) {
			out = append(out, x)
		}
	}
	return out
}

func newSessionsModel(a *App) sessionsModel {
	return sessionsModel{
		app:      a,
		statusBy: map[string]detect.Status{},
		aiList:   []string{"claude", "opencode", "codex", "aider", "copilot"},
	}
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

// detectCmd polls every pane across all sessions: classifies AI panes via
// CPU + capture-pane. Returns a panesDetectedMsg.
func (s sessionsModel) detectCmd() tea.Cmd {
	aiList := append([]string(nil), s.aiList...)
	return func() tea.Msg {
		panes, _ := tmuxctl.ListPanes()
		out := make([]paneStatus, 0, len(panes))
		for _, p := range panes {
			status := detect.StatusUnknown

			// 1. opencode → our installed plugin writes state to /tmp/lazypilot-opencode/<pid>.json.
			//    This is authoritative for opencode panes (it lacks a useful OSC title convention).
			if p.Command == "opencode" {
				if s, _ := opencodehook.Read(p.PID); s != nil {
					switch s.State {
					case "working":
						status = detect.StatusWorking
					case "needs_input":
						status = detect.StatusNeedsInput
					case "idle":
						status = detect.StatusIdle
					}
				}
			}

			// 2. Title-based detection (Orca approach — Claude, Codex, Gemini, etc.
			//    announce state via OSC titles that tmux exposes as pane_title).
			if status == detect.StatusUnknown {
				if titleStatus := detect.DetectFromTitle(p.Title); titleStatus != detect.StatusUnknown {
					status = titleStatus
				}
			}

			// 3. CPU + capture-pane regex fallback for AI processes lacking both signals.
			if status == detect.StatusUnknown && detect.IsAI(p.Command, aiList) {
				cpu := detect.AggregateCPU(p.PID)
				content, _ := tmuxctl.CapturePane(p.PaneID, 10)
				status = detect.Classify("", cpu, content)
			}

			out = append(out, paneStatus{Pane: p, Status: status})
		}
		return panesDetectedMsg(out)
	}
}

func (s *sessionsModel) applyDetect(m panesDetectedMsg) {
	s.panes = []paneStatus(m)
	// Aggregate the highest-severity status per session
	// (NeedsInput > Working > Idle > Unknown).
	severity := func(st detect.Status) int {
		switch st {
		case detect.StatusNeedsInput:
			return 3
		case detect.StatusWorking:
			return 2
		case detect.StatusIdle:
			return 1
		}
		return 0
	}
	by := map[string]detect.Status{}
	for _, p := range s.panes {
		cur := by[p.SessionName]
		if severity(p.Status) > severity(cur) {
			by[p.SessionName] = p.Status
		}
	}
	s.statusBy = by
}

// panesIn returns all paneStatus entries belonging to the given session name.
func (s sessionsModel) panesIn(session string) []paneStatus {
	out := []paneStatus{}
	for _, p := range s.panes {
		if p.SessionName == session {
			out = append(out, p)
		}
	}
	return out
}

// tickCmd schedules the next detect refresh.
func detectTickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return detectTickMsg(t) })
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
	items := s.filtered()
	title := fmt.Sprintf("Sessions (%d/%d)", len(items), len(s.all))
	if len(s.all) == 0 {
		return st.Panel(title, st.Dim.Render("\n  no tmux sessions running."), width, height, true)
	}
	filterBar := s.filter.Render(st)
	visible := height
	if filterBar != "" {
		visible--
	}
	if visible < 1 {
		visible = 1
	}
	if s.cursor >= len(items) {
		s.cursor = max(0, len(items)-1)
	}
	var rows []string
	for i := 0; i < min(len(items), visible); i++ {
		sess := items[i]
		marker := s.statusMarker(sess.Name, st)
		attached := "  "
		if sess.Attached {
			attached = st.OK.Render("●")
		}
		label := sess.Name
		row := st.Cursor(i == s.cursor) + marker + " " + attached + " "
		if i == s.cursor {
			row += st.ListSelected.Render(label)
		} else {
			row += st.ListItem.Render(label)
		}
		rows = append(rows, row)
	}
	for len(rows) < visible {
		rows = append(rows, "")
	}
	if filterBar != "" {
		rows = append(rows, filterBar)
	}
	return st.Panel(title, strings.Join(rows, "\n"), width, height, true)
}

func (s sessionsModel) renderDetail(width, height int) string {
	st := s.app.styles
	title := "Details"
	items := s.filtered()
	if len(items) == 0 {
		return st.Panel(title, "", width, height, false)
	}
	if s.cursor >= len(items) {
		s.cursor = len(items) - 1
	}
	sess := items[s.cursor]
	var b strings.Builder
	b.WriteString(st.Heading.Render(sess.Name) + "\n")
	b.WriteString(st.Dim.Render(sess.Path) + "\n\n")

	section := func(name string) string { return st.Heading.Render("▸ " + name) }

	heading := section("Panes")
	if s.paneFocus {
		heading += st.OK.Render(" ← focus (h/esc to exit)")
	} else {
		heading += st.Dim.Render("  (tab/l to focus)")
	}
	b.WriteString(heading + "\n")

	panes := s.panesIn(sess.Name)
	if len(panes) == 0 {
		b.WriteString(st.Dim.Render("  (none)\n"))
	}
	for i, p := range panes {
		marker := paneStatusIcon(p.Status, st)
		label := fmt.Sprintf("%d.%d %s", p.WindowIndex, p.PaneIndex, p.Command)
		suffix := ""
		switch p.Status {
		case detect.StatusWorking:
			suffix = st.Warn.Render(" (working)")
		case detect.StatusNeedsInput:
			suffix = st.Bad.Render(" (needs input)")
		case detect.StatusIdle:
			suffix = st.Dim.Render(" (idle)")
		}
		row := marker + " " + label + suffix
		if s.paneFocus && i == s.paneCursor {
			row = st.ListSelected.Render("▶ " + row)
		} else {
			row = "  " + row
		}
		b.WriteString(row + "\n")
	}
	return st.Panel(title, b.String(), width, height, false)
}

// renderPreview returns a panel showing the live captured output of the
// currently selected session's pane(s). Refreshes each detect tick.
func (s sessionsModel) renderPreview(width, height int) string {
	st := s.app.styles
	items := s.filtered()
	title := "Live preview"
	if len(items) == 0 {
		return st.Panel(title, "", width, height, false)
	}
	if s.cursor >= len(items) {
		s.cursor = len(items) - 1
	}
	sess := items[s.cursor]

	panes := s.panesIn(sess.Name)
	if len(panes) == 0 {
		return st.Panel(title, st.Dim.Render("\n  (no panes in this session)"), width, height, false)
	}

	// Pick the selected pane when in paneFocus mode, else default to first.
	idx := 0
	if s.paneFocus && s.paneCursor < len(panes) {
		idx = s.paneCursor
	}
	p := panes[idx]

	captureW := width - 4
	if captureW < 20 {
		captureW = 20
	}
	captureH := height - 2
	if captureH < 5 {
		captureH = 5
	}

	content, _ := tmuxctl.CapturePane(p.PaneID, captureH)
	content = clipToBox(content, captureW, captureH)

	title = fmt.Sprintf("Preview · %s [%d.%d %s]",
		sess.Name, p.WindowIndex, p.PaneIndex, p.Command)
	return st.Panel(title, content, width, height, false)
}

// clipToWidth truncates every line of `s` to at most `w` visible cells.
// ANSI-aware: zero-width escape sequences (color codes from `git log
// --color=always`, etc.) are preserved correctly, and we never cut a line
// mid-escape — which was the bug behind "git tree gets buggy on big repos."
func clipToWidth(s string, w int) string {
	if w <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > w {
			lines[i] = ansi.Truncate(line, w, "")
		}
	}
	return strings.Join(lines, "\n")
}

// clipToBox truncates a multi-line string to at most `lines` lines AND at
// most `width` visible cells per line. Use for preview content that must fit
// in a fixed-size panel — without this, a very tall `git log --graph` blew
// the layout because Panel rendered every line and lipgloss padded around it.
func clipToBox(s string, width, lines int) string {
	parts := strings.Split(s, "\n")
	if lines > 0 && len(parts) > lines {
		parts = parts[:lines]
	}
	for i, line := range parts {
		if lipgloss.Width(line) > width {
			parts[i] = ansi.Truncate(line, width, "")
		}
	}
	return strings.Join(parts, "\n")
}

// statusMarker returns the aggregated status icon for the given session,
// or a dim "○" if no AI processes are present.
func (s sessionsModel) statusMarker(session string, st styles) string {
	status, ok := s.statusBy[session]
	if !ok {
		return st.Dim.Render("○")
	}
	return paneStatusIcon(status, st)
}

// paneStatusIcon returns a one-cell-wide glyph reflecting the AI status.
// Shape carries the meaning so the indicator still reads in monochrome:
//
//	◐ working      (half-filled — in progress)
//	! needs input  (loud attention mark)
//	○ idle         (empty — at rest)
//	· unknown      (small dot — nothing detected)
func paneStatusIcon(status detect.Status, st styles) string {
	switch status {
	case detect.StatusWorking:
		return st.Warn.Render("◐")
	case detect.StatusNeedsInput:
		return st.Bad.Render("!")
	case detect.StatusIdle:
		return st.OK.Render("○")
	}
	return st.Dim.Render("·")
}

func (s sessionsModel) handleKey(m tea.KeyMsg, k keymap) (sessionsModel, tea.Cmd) {
	if s.filter.active {
		s.filter.Update(m)
		s.cursor = 0
		s.paneFocus = false
		s.paneCursor = 0
		return s, nil
	}
	items := s.filtered()

	// Drill-in / drill-out keys come BEFORE the navigation switch so they
	// take effect regardless of which sub-cursor has focus.
	switch m.String() {
	case "tab", "l", "right":
		if !s.paneFocus && s.cursor < len(items) {
			panes := s.panesIn(items[s.cursor].Name)
			if len(panes) > 0 {
				s.paneFocus = true
				if s.paneCursor >= len(panes) {
					s.paneCursor = 0
				}
				return s, nil
			}
		}
	case "h", "left":
		if s.paneFocus {
			s.paneFocus = false
			return s, nil
		}
	}

	// j/k navigation: route to whichever cursor has focus.
	if s.paneFocus {
		panes := s.panesIn(items[s.cursor].Name)
		switch {
		case keyMatches(m, k.Up):
			if s.paneCursor > 0 {
				s.paneCursor--
			}
		case keyMatches(m, k.Down):
			if s.paneCursor < len(panes)-1 {
				s.paneCursor++
			}
		case keyMatches(m, k.Open):
			if s.paneCursor < len(panes) {
				p := panes[s.paneCursor]
				target := fmt.Sprintf("%s:%d.%d", p.SessionName, p.WindowIndex, p.PaneIndex)
				return s, func() tea.Msg { return attachRequestMsg{name: target} }
			}
		}
		return s, nil
	}

	switch {
	case keyMatches(m, k.Filter):
		s.filter.Begin()
	case keyMatches(m, k.Up):
		if s.cursor > 0 {
			s.cursor--
			s.paneCursor = 0
		}
	case keyMatches(m, k.Down):
		if s.cursor < len(items)-1 {
			s.cursor++
			s.paneCursor = 0
		}
	case keyMatches(m, k.Open):
		if s.cursor < len(items) {
			name := items[s.cursor].Name
			return s, func() tea.Msg { return attachRequestMsg{name: name} }
		}
	case keyMatches(m, k.KillSesh):
		if s.cursor < len(items) {
			return s, requestKillSession(items[s.cursor].Name)
		}
	}
	return s, nil
}
