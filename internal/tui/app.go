// Package tui contains the bubbletea models for lazypilot.
package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gdelco/lazypilot/internal/scan"
	"github.com/gdelco/lazypilot/internal/tmuxctl"
)

// view identifies the active tab.
type view int

const (
	viewSessions view = iota
	viewProjects
	viewWorktrees
)

func (v view) Label() string {
	switch v {
	case viewSessions:
		return "Sessions"
	case viewProjects:
		return "Projects"
	case viewWorktrees:
		return "Worktrees"
	}
	return "?"
}

// Roots provides the list of project root directories (loaded from config).
type Roots []string

// App is the root bubbletea model.
type App struct {
	roots   Roots
	home    string
	view    view
	width   int
	height  int
	keys    keymap
	styles  styles
	projects projectsModel
	sessions sessionsModel
	worktrees worktreesModel
	statusMsg string
	// Action requested on Quit so main.go can run it after the TUI tears down
	// (e.g. switching tmux client requires the TUI to give back the terminal first).
	deferred *deferredAction
}

type deferredAction struct {
	kind string // "attach"
	name string
}

// New builds the root App model.
func New(roots Roots) App {
	home, _ := os.UserHomeDir()
	s := newStyles()
	a := App{
		roots:  roots,
		home:   home,
		keys:   newKeymap(),
		styles: s,
		view:   viewProjects, // start on Projects (most-used today)
	}
	a.projects = newProjectsModel(&a)
	a.sessions = newSessionsModel(&a)
	a.worktrees = newWorktreesModel(&a)
	return a
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.projects.scanCmd(),
		a.sessions.refreshCmd(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height

	case tea.KeyMsg:
		switch {
		case keyMatches(m, a.keys.Quit):
			return a, tea.Quit
		case keyMatches(m, a.keys.View1):
			a.view = viewSessions
		case keyMatches(m, a.keys.View2):
			a.view = viewProjects
		case keyMatches(m, a.keys.View3):
			a.view = viewWorktrees
		case keyMatches(m, a.keys.NextTab):
			a.view = (a.view + 1) % 3
		case keyMatches(m, a.keys.PrevTab):
			a.view = (a.view + 2) % 3
		case keyMatches(m, a.keys.Refresh):
			cmds = append(cmds, a.projects.scanCmd(), a.sessions.refreshCmd())
			a.statusMsg = "refreshing…"
		default:
			// Route view-specific keys.
			updated, cmd := a.routeKey(m)
			a = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case projectsScannedMsg:
		a.projects.applyScan(m)
	case sessionsRefreshedMsg:
		a.sessions.applyRefresh(m)
		a.worktrees.applyFromProjects(a.projects.all)
	case statusMsg:
		a.statusMsg = string(m)
	case attachRequestMsg:
		a.deferred = &deferredAction{kind: "attach", name: m.name}
		return a, tea.Quit
	}

	return a, tea.Batch(cmds...)
}

func (a App) View() string {
	if a.width == 0 {
		return "loading…"
	}

	tabs := a.renderTabs()
	body := a.renderBody()
	footer := a.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, tabs, body, footer)
}

func (a App) renderTabs() string {
	cells := make([]string, 0, 3)
	for i := viewSessions; i <= viewWorktrees; i++ {
		label := fmt.Sprintf("[%d] %s", i+1, i.Label())
		if i == a.view {
			cells = append(cells, a.styles.TabActive.Render(label))
		} else {
			cells = append(cells, a.styles.Tab.Render(label))
		}
	}
	bar := strings.Join(cells, "")
	hint := a.styles.Dim.Render("  ? help · q quit")
	return bar + hint
}

func (a App) renderBody() string {
	// Reserve 2 rows for tab strip and 1 for footer.
	bodyHeight := a.height - 3
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	switch a.view {
	case viewSessions:
		return a.sessions.view(a.width, bodyHeight)
	case viewProjects:
		return a.projects.view(a.width, bodyHeight)
	case viewWorktrees:
		return a.worktrees.view(a.width, bodyHeight)
	}
	return ""
}

func (a App) renderFooter() string {
	keys := []string{
		a.styles.FooterKey.Render("⏎") + " open",
		a.styles.FooterKey.Render("n") + " new",
		a.styles.FooterKey.Render("d") + " remove",
		a.styles.FooterKey.Render("K") + " kill",
		a.styles.FooterKey.Render("r") + " reload",
		a.styles.FooterKey.Render("1") + "/" + a.styles.FooterKey.Render("2") + "/" + a.styles.FooterKey.Render("3") + " switch",
		a.styles.FooterKey.Render("?") + " help",
		a.styles.FooterKey.Render("q") + " quit",
	}
	sep := a.styles.FooterSep.Render(" │ ")
	bar := strings.Join(keys, sep)
	if a.statusMsg != "" {
		bar += "  " + a.styles.Dim.Render("· "+a.statusMsg)
	}
	return a.styles.Footer.Render(bar)
}

// routeKey dispatches non-global keys to the active view's model.
func (a App) routeKey(m tea.KeyMsg) (App, tea.Cmd) {
	switch a.view {
	case viewSessions:
		updated, cmd := a.sessions.handleKey(m, a.keys)
		a.sessions = updated
		return a, cmd
	case viewProjects:
		updated, cmd := a.projects.handleKey(m, a.keys)
		a.projects = updated
		return a, cmd
	case viewWorktrees:
		updated, cmd := a.worktrees.handleKey(m, a.keys)
		a.worktrees = updated
		return a, cmd
	}
	return a, nil
}

// Deferred returns the action (if any) to run after the TUI exits.
func (a App) Deferred() *deferredAction { return a.deferred }

// RunDeferred executes the deferred attach so main.go has a single hook.
func (a App) RunDeferred() error {
	if a.deferred == nil {
		return nil
	}
	switch a.deferred.kind {
	case "attach":
		cmd := tmuxctl.AttachOrSwitch(a.deferred.name)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

// keyMatches reports whether the keypress matches a binding.
func keyMatches(m tea.KeyMsg, b interface{ Keys() []string }) bool {
	pressed := m.String()
	for _, k := range b.Keys() {
		if k == pressed {
			return true
		}
	}
	return false
}

// Messages
type projectsScannedMsg []scan.Project
type sessionsRefreshedMsg []tmuxctl.Session
type statusMsg string
type attachRequestMsg struct{ name string }
