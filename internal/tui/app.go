// Package tui contains the bubbletea models for lazypilot.
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	projects  projectsModel
	sessions  sessionsModel
	worktrees worktreesModel
	confirm   *confirmModal
	wizard    *createWizard
	help      *helpModal
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
		view:   viewSessions, // start on Sessions — most useful when opening lazypilot mid-work
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
		a.sessions.detectCmd(),
		detectTickCmd(2*time.Second),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height

	case tea.KeyMsg:
		// Active text-input modes (filter / wizard / modal) capture all keys.
		// Route everything to the active view if its filter is open, so the
		// user can type `/` plus arbitrary text without triggering global
		// shortcuts like 1/2/3 or q.
		if a.isFiltering() {
			updated, cmd := a.routeKey(m)
			a = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}

		// Modals take priority over everything else.
		if a.confirm != nil {
			cmd, done := a.confirm.Update(m)
			if done {
				a.confirm = nil
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
		if a.wizard != nil {
			cmd, done := a.wizard.Update(m)
			if done {
				a.wizard = nil
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
		if a.help != nil {
			_, done := a.help.Update(m)
			if done {
				a.help = nil
			}
			return a, tea.Batch(cmds...)
		}

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
		case keyMatches(m, a.keys.Help):
			a.help = newHelp()
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
		a.worktrees.applyFromProjects(a.projects.all) // worktrees are derived from the project scan
	case sessionsRefreshedMsg:
		a.sessions.applyRefresh(m)
	case statusMsg:
		a.statusMsg = string(m)
	case attachRequestMsg:
		a.deferred = &deferredAction{kind: "attach", name: m.name}
		return a, tea.Quit
	case confirmMsg:
		a.confirm = m.modal
	case wizardMsg:
		a.wizard = m.wizard
	case rescanMsg:
		cmds = append(cmds, a.projects.scanCmd(), a.sessions.refreshCmd(), a.sessions.detectCmd())
		if m.status != "" {
			a.statusMsg = m.status
		}
	case panesDetectedMsg:
		a.sessions.applyDetect(m)
	case detectTickMsg:
		// Refresh sessions + AI status, then schedule the next tick.
		cmds = append(cmds, a.sessions.refreshCmd(), a.sessions.detectCmd(), detectTickCmd(2*time.Second))
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

	// Retro double-line rule under tabs.
	rule := a.styles.Dim.Render(strings.Repeat("═", a.width))

	base := lipgloss.JoinVertical(lipgloss.Left, tabs, rule, body, footer)

	// Overlay any active modal on top, centered.
	if a.confirm != nil {
		return a.confirm.View(a.styles, a.width, a.height)
	}
	if a.wizard != nil {
		return a.wizard.View(a.styles, a.width, a.height)
	}
	if a.help != nil {
		return a.help.View(a.styles, a.width, a.height)
	}
	return base
}

func (a App) renderTabs() string {
	cells := make([]string, 0, 3)
	for i := viewSessions; i <= viewWorktrees; i++ {
		label := fmt.Sprintf(" %d %s ", i+1, strings.ToUpper(i.Label()))
		if i == a.view {
			cells = append(cells, a.styles.TabActive.Render(label))
		} else {
			cells = append(cells, a.styles.Tab.Render(label))
		}
	}
	sep := a.styles.Dim.Render(" ")
	bar := strings.Join(cells, sep)
	hint := "    " + a.styles.Dim.Render("[?] HELP  ·  [q] QUIT")
	return " " + bar + hint
}

func (a App) renderBody() string {
	// Title is embedded in the top border now, so each panel is bodyHeight + 2 rows
	// (top border with title + content + bottom border).
	// Total: tabs(1) + rule(1) + panel(bodyHeight+2) + footer(1) = a.height → bodyHeight = a.height - 5.
	bodyHeight := a.height - 5
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
		a.styles.FooterKey.Render("[⏎]") + " OPEN",
		a.styles.FooterKey.Render("[n]") + " NEW",
		a.styles.FooterKey.Render("[d]") + " REMOVE",
		a.styles.FooterKey.Render("[K]") + " KILL",
		a.styles.FooterKey.Render("[r]") + " RELOAD",
		a.styles.FooterKey.Render("[1/2/3]") + " SWITCH",
		a.styles.FooterKey.Render("[?]") + " HELP",
		a.styles.FooterKey.Render("[q]") + " QUIT",
	}
	sep := a.styles.FooterSep.Render("  ")
	bar := strings.Join(keys, sep)
	if a.statusMsg != "" {
		bar += "  " + a.styles.Dim.Render("· "+a.statusMsg)
	}
	return " " + a.styles.Footer.Render(bar)
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

// isFiltering reports whether the active view has its filter open and capturing keys.
func (a App) isFiltering() bool {
	switch a.view {
	case viewSessions:
		return a.sessions.filter.active
	case viewProjects:
		return a.projects.filter.active
	case viewWorktrees:
		return a.worktrees.filter.active
	}
	return false
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
type confirmMsg struct{ modal *confirmModal }
type wizardMsg struct{ wizard *createWizard }
type rescanMsg struct{ status string }
type panesDetectedMsg []paneStatus
type detectTickMsg time.Time
