package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmModal is a yes/no overlay used for destructive actions.
type confirmModal struct {
	title   string
	message string
	yesText string
	noText  string
	onYes   func() tea.Cmd
	focused bool // focused = "Yes" selected; false = "No" selected
}

func newConfirm(title, message string, onYes func() tea.Cmd) *confirmModal {
	return &confirmModal{
		title:   title,
		message: message,
		yesText: "Yes",
		noText:  "No",
		onYes:   onYes,
		focused: false, // default focus on No — safer for destructive ops
	}
}

// Update returns the new modal state, an optional cmd to run, and `done=true`
// when the modal should be dismissed.
func (c *confirmModal) Update(msg tea.Msg) (cmd tea.Cmd, done bool) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			if c.onYes != nil {
				return c.onYes(), true
			}
			return nil, true
		case "n", "N", "esc", "q":
			return nil, true
		case "enter":
			if c.focused && c.onYes != nil {
				return c.onYes(), true
			}
			return nil, true
		case "left", "h", "right", "l", "tab":
			c.focused = !c.focused
		}
	}
	return nil, false
}

func (c *confirmModal) View(s styles, screenW, screenH int) string {
	// Modal width: cap at min(message length + padding, screen-10).
	width := lipgloss.Width(c.message) + 8
	if width < 40 {
		width = 40
	}
	if width > screenW-10 {
		width = screenW - 10
	}

	// Buttons
	yesStyle := lipgloss.NewStyle().Padding(0, 2).Foreground(colYellow).Bold(true)
	noStyle := lipgloss.NewStyle().Padding(0, 2).Foreground(colGrey)
	if c.focused {
		yesStyle = yesStyle.Reverse(true)
	} else {
		noStyle = noStyle.Reverse(true)
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top,
		yesStyle.Render("[ "+c.yesText+" ]"),
		"  ",
		noStyle.Render("[ "+c.noText+" ]"),
	)
	buttonRow := lipgloss.PlaceHorizontal(width, lipgloss.Center, buttons)

	body := lipgloss.JoinVertical(lipgloss.Left,
		"",
		c.message,
		"",
		buttonRow,
		"",
		s.Dim.Render("  [y] yes   [n/esc] no   [⇄] move"),
	)

	// Use Panel with a warning color: ╔══ CONFIRM ══╗
	box := s.Panel(c.title, body, width, lipgloss.Height(body), true)
	return lipgloss.Place(screenW, screenH, lipgloss.Center, lipgloss.Center, box)
}

// overlay returns `under` with `over` rendered centered on top of it.
// (lipgloss.Place inside the modal handles the centering; we just return the
// overlay as the rendered frame.)
func overlay(over string, width, height int) string {
	_ = width
	_ = height
	return strings.TrimRight(over, "\n")
}
