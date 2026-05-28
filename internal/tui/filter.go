package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// filterState backs the `/` substring filter on each view. It captures keys
// while `active`, persists the typed query, and exposes `Matches` for the
// view to filter its rows.
type filterState struct {
	text   string
	active bool
}

// Begin starts capturing keystrokes; preserves any existing text so re-entering
// edits the previous query.
func (f *filterState) Begin() { f.active = true }

// Done turns off capture but keeps the current text (so the list stays filtered).
func (f *filterState) Done() { f.active = false }

// Reset clears the filter completely.
func (f *filterState) Reset() { f.text = ""; f.active = false }

// Update consumes a keypress while active. Returns true if the key was handled
// (i.e. the caller should NOT process it further).
func (f *filterState) Update(km tea.KeyMsg) bool {
	if !f.active {
		return false
	}
	switch km.String() {
	case "esc":
		f.Reset()
	case "enter":
		f.Done()
	case "backspace":
		if len(f.text) > 0 {
			r := []rune(f.text)
			f.text = string(r[:len(r)-1])
		}
	case "ctrl+u":
		f.text = ""
	default:
		// Runes / printable characters extend the filter.
		if len(km.Runes) > 0 {
			f.text += string(km.Runes)
		}
	}
	return true
}

// Matches reports whether `s` passes the current filter (case-insensitive substring).
// Empty filter matches everything.
func (f filterState) Matches(s string) bool {
	if f.text == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(f.text))
}

// Render returns a single-line filter bar to append to the list panel.
// Empty string when there's nothing to show.
func (f filterState) Render(s styles) string {
	if !f.active && f.text == "" {
		return ""
	}
	cursor := ""
	if f.active {
		cursor = "█"
	}
	prompt := s.Dim.Render("/")
	body := s.ListSelected.Render(f.text) + cursor
	if !f.active {
		body = s.Dim.Render(f.text) + s.Dim.Render("  (esc to clear)")
	}
	return prompt + body
}
