package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 16-color terminal palette indices — follows whatever theme the terminal is set to.
var (
	colGreen  = lipgloss.ANSIColor(2)
	colYellow = lipgloss.ANSIColor(3)
	colBlue   = lipgloss.ANSIColor(4)
	colCyan   = lipgloss.ANSIColor(6)
	colGrey   = lipgloss.ANSIColor(8)
	colRed    = lipgloss.ANSIColor(1)
)

type styles struct {
	Tab          lipgloss.Style
	TabActive    lipgloss.Style
	Box          lipgloss.Style
	Title        lipgloss.Style
	TitleDim     lipgloss.Style
	ListItem     lipgloss.Style
	ListSelected lipgloss.Style
	ListDim      lipgloss.Style
	IconRepo     lipgloss.Style
	IconWorktree lipgloss.Style
	IconWorkSp   lipgloss.Style
	Footer       lipgloss.Style
	FooterKey    lipgloss.Style
	FooterSep    lipgloss.Style
	Heading      lipgloss.Style
	Dim          lipgloss.Style
	OK           lipgloss.Style
	Warn         lipgloss.Style
	Bad          lipgloss.Style
}

func newStyles() styles {
	return styles{
		// Tabs use uppercase + bracketed numbers for a retro feel.
		Tab:       lipgloss.NewStyle().Padding(0, 1).Foreground(colGrey),
		TabActive: lipgloss.NewStyle().Padding(0, 1).Foreground(colCyan).Bold(true).Reverse(true),

		// Retro double-line border (╔═══╗).
		Box: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colCyan).
			Padding(0, 1),

		// Panel titles rendered ABOVE the box (not injected into the border).
		Title:    lipgloss.NewStyle().Foreground(colCyan).Bold(true).Padding(0, 1),
		TitleDim: lipgloss.NewStyle().Foreground(colGrey).Padding(0, 1),

		ListItem:     lipgloss.NewStyle(),
		ListSelected: lipgloss.NewStyle().Bold(true).Foreground(colCyan),
		ListDim:      lipgloss.NewStyle().Foreground(colGrey),

		IconRepo:     lipgloss.NewStyle().Foreground(colGreen),
		IconWorktree: lipgloss.NewStyle().Foreground(colYellow),
		IconWorkSp:   lipgloss.NewStyle().Foreground(colBlue),

		Footer:    lipgloss.NewStyle().Foreground(colGrey),
		FooterKey: lipgloss.NewStyle().Foreground(colCyan).Bold(true),
		FooterSep: lipgloss.NewStyle().Foreground(colGrey),

		Heading: lipgloss.NewStyle().Bold(true).Foreground(colCyan),
		Dim:     lipgloss.NewStyle().Foreground(colGrey),
		OK:      lipgloss.NewStyle().Foreground(colGreen),
		Warn:    lipgloss.NewStyle().Foreground(colYellow),
		Bad:     lipgloss.NewStyle().Foreground(colRed),
	}
}

// Cursor returns the prefix used for the highlighted row.
func (s styles) Cursor(selected bool) string {
	if selected {
		return s.ListSelected.Render("▶ ")
	}
	return "  "
}

// Panel renders a double-bordered box with the title embedded in the top edge,
// lazygit-style:  ╔══ TITLE (n) ══════════════╗
//
// If `focused` is true, the border and title use the accent color (cyan); if
// false, both fade to dim grey. Total rendered height = height+2 (border rows).
func (s styles) Panel(title, content string, width, height int, focused bool) string {
	borderColor := colCyan
	if !focused {
		borderColor = colGrey
	}
	titleColor := colCyan
	if !focused {
		titleColor = colGrey
	}

	// Build the box WITHOUT a top border; we draw the top manually with the title.
	body := s.Box.
		BorderTop(false).
		BorderForeground(borderColor).
		Width(width).
		Height(height).
		Render(content)

	// Manually construct the top edge with the title embedded.
	// Layout (5 fixed cells):  ╔ ═ ═ <title> <filler ═*> ═ ╗
	// Total visible cells across = width + 2 (box adds 1 border char each side).
	totalCells := width + 2
	titleStr := " " + strings.ToUpper(title) + " "
	// 5 fixed chars: "╔══" (3) + "═╗" (2)
	const fixed = 5
	maxTitle := totalCells - fixed
	titleRunes := []rune(titleStr)
	if maxTitle < 1 {
		maxTitle = 1
	}
	if len(titleRunes) > maxTitle {
		titleRunes = titleRunes[:maxTitle]
	}
	filler := totalCells - fixed - len(titleRunes)
	if filler < 0 {
		filler = 0
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)

	topEdge := borderStyle.Render("╔══") +
		titleStyle.Render(string(titleRunes)) +
		borderStyle.Render(strings.Repeat("═", filler)+"═╗")

	return lipgloss.JoinVertical(lipgloss.Left, topEdge, body)
}
