// Command lazypilot is a TUI for tmux sessions, projects, and git worktrees.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gdelco/lazypilot/internal/config"
	"github.com/gdelco/lazypilot/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazypilot: %v\n", err)
		os.Exit(1)
	}

	app := tui.New(cfg.Roots)
	p := tea.NewProgram(app, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazypilot: %v\n", err)
		os.Exit(1)
	}

	// Execute any deferred attach now that the alt screen is gone.
	if final, ok := finalModel.(tui.App); ok {
		if err := final.RunDeferred(); err != nil {
			fmt.Fprintf(os.Stderr, "lazypilot: attach failed: %v\n", err)
			os.Exit(1)
		}
	}
}
