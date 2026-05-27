// Package config loads user configuration from ~/.config/lazypilot/config.yaml.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Roots              []string      `yaml:"roots"`
	BranchPrefix       string        `yaml:"branch_prefix"`
	AIProcesses        []string      `yaml:"ai_processes"`
	RefreshInterval    time.Duration `yaml:"refresh_interval"`
	WorktreeContainers []string      `yaml:"worktree_containers"`
}

// Defaults are used when no config file is present, or to fill in unset keys.
func Defaults() Config {
	return Config{
		Roots:           []string{"~/code", "~/projects", "~/dev"},
		AIProcesses:     []string{"claude", "opencode", "codex", "aider", "copilot"},
		RefreshInterval: 2 * time.Second,
		WorktreeContainers: []string{
			"{parent}/worktrees/{repo}",
			"{parent}/worktrees",
			"{parent}/{repo}-worktrees",
			"{parent}",
		},
	}
}

// Load reads config.yaml from $XDG_CONFIG_HOME/lazypilot (or ~/.config/lazypilot)
// and merges it onto the defaults. Missing file is not an error.
func Load() (Config, error) {
	c := Defaults()
	path, err := configPath()
	if err != nil {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return c, err
	}
	// Expand ~ in roots.
	for i, r := range c.Roots {
		c.Roots[i] = expand(r)
	}
	return c, nil
}

func configPath() (string, error) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "lazypilot", "config.yaml"), nil
}

func expand(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
