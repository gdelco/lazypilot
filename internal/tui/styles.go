package tui

import "github.com/charmbracelet/lipgloss"

// We use 16-color palette indices so the theme follows whatever the terminal
// is set to (light or dark) — same trick as the bash sessionizer.
//
// lipgloss.ANSIColor("N") maps to terminal color N.
var (
	colGreen  = lipgloss.ANSIColor(2)
	colYellow = lipgloss.ANSIColor(3)
	colBlue   = lipgloss.ANSIColor(4)
	colCyan   = lipgloss.ANSIColor(6)
	colGrey   = lipgloss.ANSIColor(8)
	colRed    = lipgloss.ANSIColor(1)
	colWhite  = lipgloss.ANSIColor(7)
	colBlack  = lipgloss.ANSIColor(0)
)

// Style returns a configured lipgloss style for the given role.
type styles struct {
	Tab          lipgloss.Style
	TabActive    lipgloss.Style
	Border       lipgloss.Style
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
		Tab:          lipgloss.NewStyle().Padding(0, 2).Foreground(colGrey),
		TabActive:    lipgloss.NewStyle().Padding(0, 2).Foreground(colWhite).Background(colBlack).Bold(true),
		Border:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colCyan),
		ListItem:     lipgloss.NewStyle().Padding(0, 1),
		ListSelected: lipgloss.NewStyle().Padding(0, 1).Bold(true).Background(colWhite).Foreground(colBlack),
		ListDim:      lipgloss.NewStyle().Padding(0, 1).Foreground(colGrey),
		IconRepo:     lipgloss.NewStyle().Foreground(colGreen),
		IconWorktree: lipgloss.NewStyle().Foreground(colYellow),
		IconWorkSp:   lipgloss.NewStyle().Foreground(colBlue),
		Footer:       lipgloss.NewStyle().Foreground(colGrey),
		FooterKey:    lipgloss.NewStyle().Foreground(colGreen).Bold(true),
		FooterSep:    lipgloss.NewStyle().Foreground(colGrey),
		Heading:      lipgloss.NewStyle().Bold(true).Foreground(colCyan),
		Dim:          lipgloss.NewStyle().Foreground(colGrey),
		OK:           lipgloss.NewStyle().Foreground(colGreen),
		Warn:         lipgloss.NewStyle().Foreground(colYellow),
		Bad:          lipgloss.NewStyle().Foreground(colRed),
	}
}
