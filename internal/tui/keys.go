package tui

import "github.com/charmbracelet/bubbles/key"

type keymap struct {
	Up         key.Binding
	Down       key.Binding
	Top        key.Binding
	Bottom     key.Binding
	HalfUp     key.Binding
	HalfDown   key.Binding
	View1      key.Binding
	View2      key.Binding
	View3      key.Binding
	NextTab    key.Binding
	PrevTab    key.Binding
	Open       key.Binding
	NewWT      key.Binding
	Remove     key.Binding
	KillSesh   key.Binding
	Refresh    key.Binding
	Filter     key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func newKeymap() keymap {
	return keymap{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		HalfUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("^u", "half-up")),
		HalfDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("^d", "half-down")),
		View1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "sessions")),
		View2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "projects")),
		View3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "worktrees")),
		NextTab:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next view")),
		PrevTab:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧tab", "prev view")),
		Open:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open")),
		NewWT:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new worktree")),
		Remove:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "remove")),
		KillSesh: key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "kill session")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
