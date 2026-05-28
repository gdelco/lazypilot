// Command lazypilot is a TUI for tmux sessions, projects, and git worktrees.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gdelco/lazypilot/internal/config"
	"github.com/gdelco/lazypilot/internal/opencodehook"
	"github.com/gdelco/lazypilot/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazypilot: %v\n", err)
		os.Exit(1)
	}

	// Ensure the opencode status plugin is installed so future opencode
	// sessions broadcast working/idle/needs-input state to lazypilot.
	// Non-fatal — lazypilot still works without it (opencode will just show
	// as "unknown" status until the plugin is in place).
	if _, err := opencodehook.Install(); err != nil {
		fmt.Fprintf(os.Stderr, "lazypilot: opencode plugin install warning: %v\n", err)
	}

	app := tui.New(cfg)
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
