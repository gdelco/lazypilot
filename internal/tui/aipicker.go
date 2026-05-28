package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gdelco/lazypilot/internal/config"
	"github.com/gdelco/lazypilot/internal/tmuxctl"
)

// aiPickerModal asks which AI assistant to launch alongside the editor when
// creating a new tmux session. Choices come from ~/.config/lazypilot/config.yaml.
type aiPickerModal struct {
	target  string // tmux session name to create
	dir     string // working directory
	editor  string // editor to launch in the left pane (defaults to nvim)
	choices []config.AIAssistant
	cursor  int
}

func newAIPicker(target, dir, editor string, choices []config.AIAssistant) *aiPickerModal {
	return &aiPickerModal{target: target, dir: dir, editor: editor, choices: choices}
}

func (m *aiPickerModal) Update(msg tea.Msg) (cmd tea.Cmd, done bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	switch km.String() {
	case "esc", "q":
		return nil, true
	case "j", "down":
		if m.cursor < len(m.choices)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		choice := m.choices[m.cursor]
		return openWithLayout(m.target, m.dir, m.editor, choice.Cmd), true
	}
	// number shortcuts: 1, 2, 3, … jump straight to that AI
	if r := km.Runes; len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
		idx := int(r[0] - '1')
		if idx < len(m.choices) {
			m.cursor = idx
			choice := m.choices[idx]
			return openWithLayout(m.target, m.dir, m.editor, choice.Cmd), true
		}
	}
	return nil, false
}

func (m *aiPickerModal) View(s styles, screenW, screenH int) string {
	width := 60
	if width > screenW-6 {
		width = screenW - 6
	}

	var b strings.Builder
	b.WriteString(s.Dim.Render("Creating session ") + s.Heading.Render(m.target) + "\n")
	b.WriteString(s.Dim.Render("in ") + m.dir + "\n\n")
	b.WriteString(s.Heading.Render("Pick AI assistant for the right pane:") + "\n\n")

	for i, c := range m.choices {
		num := fmt.Sprintf(" %d ", i+1)
		label := c.Name
		if c.Cmd == "" {
			label += s.Dim.Render("  (no AI pane — just the editor)")
		} else {
			label += s.Dim.Render("  → "+c.Cmd)
		}
		row := s.FooterKey.Render(num) + "  " + label
		if i == m.cursor {
			row = s.ListSelected.Render("▶ ") + row
		} else {
			row = "  " + row
		}
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + s.Dim.Render("[j/k] move   [enter] select   [1-9] direct pick   [esc] cancel"))

	box := s.Panel("NEW SESSION · PICK AI", b.String(), width, lipgloss.Height(b.String()), true)
	return lipgloss.Place(screenW, screenH, lipgloss.Center, lipgloss.Center, box)
}

// openWithLayout creates a tmux session running `editor` in pane 0 and
// `aiCmd` in a 60/40 split on the right (skipped when aiCmd is empty),
// then fires the usual attach request so the TUI hands off to tmux.
func openWithLayout(name, dir, editor, aiCmd string) tea.Cmd {
	return func() tea.Msg {
		if !tmuxctl.HasSession(name) {
			// Pane 0 — editor.
			editorCmd := editor
			if editor == "" {
				editorCmd = "nvim"
			}
			_ = exec.Command("tmux", "new-session", "-ds", name, "-c", dir, editorCmd+" .").Run()

			// Optional pane 1 — AI assistant, on the right.
			if aiCmd != "" {
				_ = exec.Command("tmux", "split-window", "-h",
					"-t", name+":0",
					"-c", dir,
					"-l", "40%",
					aiCmd).Run()
				// Re-select pane 0 so the editor has focus when the user attaches.
				_ = exec.Command("tmux", "select-pane", "-t", name+":0.0").Run()
			}
		}
		return attachRequestMsg{name: name}
	}
}
