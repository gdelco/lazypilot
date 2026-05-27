package scan

import (
	"path/filepath"
	"strings"
)

// Kind classifies a discovered directory.
type Kind int

const (
	KindWorkspace Kind = iota // no .git
	KindRepo                  // .git is a directory
	KindWorktree              // .git is a file (gitlink)
)

func (k Kind) String() string {
	switch k {
	case KindRepo:
		return "repo"
	case KindWorktree:
		return "worktree"
	default:
		return "workspace"
	}
}

// Icon returns a single-rune symbol for the kind (Material Design Nerd Font).
func (k Kind) Icon() string {
	switch k {
	case KindRepo:
		return "\U000F0CA2" // 󰊢
	case KindWorktree:
		return "\U000F062C" // 󰘬
	default:
		return "\U000F024B" // 󰉋
	}
}

// Project is one entry the picker can act on: a repo, a worktree, or a plain
// workspace folder.
type Project struct {
	Path       string // absolute path
	Kind       Kind
	SourceRepo string // for KindWorktree, the main repo's path. Empty otherwise.
}

// Short returns the path with $HOME replaced by ~.
func (p Project) Short(home string) string {
	if home == "" {
		return p.Path
	}
	if strings.HasPrefix(p.Path, home) {
		return "~" + strings.TrimPrefix(p.Path, home)
	}
	return p.Path
}

// Name returns the basename of the project path.
func (p Project) Name() string { return filepath.Base(p.Path) }
