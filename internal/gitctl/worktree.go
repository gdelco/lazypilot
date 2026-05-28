// Package gitctl wraps the git CLI for the worktree + branch operations
// lazypilot needs.
package gitctl

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// WorktreeEntry is one row from `git worktree list --porcelain`.
type WorktreeEntry struct {
	Path   string
	HEAD   string
	Branch string
	Bare   bool
}

// ListWorktrees returns every worktree of `repo` as reported by git.
// The first entry is always the main worktree.
func ListWorktrees(repo string) ([]WorktreeEntry, error) {
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, err
	}

	var entries []WorktreeEntry
	var cur WorktreeEntry
	flush := func() {
		if cur.Path != "" {
			entries = append(entries, cur)
		}
		cur = WorktreeEntry{}
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(line, "branch ")
		case line == "bare":
			cur.Bare = true
		case line == "":
			flush()
		}
	}
	flush()
	return entries, nil
}

// AddWorktree creates a new worktree at `path`, on a new branch `branch`,
// forked from `base` (which may be empty to default to HEAD).
func AddWorktree(repo, path, branch, base string) error {
	args := []string{"-C", repo, "worktree", "add", path, "-b", branch}
	if base != "" {
		args = append(args, base)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %v\n%s", err, string(out))
	}
	return nil
}

// RemoveWorktree removes the worktree at path. force allows removing a
// worktree with uncommitted changes.
func RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %v\n%s", err, string(out))
	}
	return nil
}

// IsDirty returns true if `git status --porcelain` from path has any output.
func IsDirty(path string) bool {
	out, err := exec.Command("git", "-C", path, "status", "--porcelain").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// CurrentBranch returns the current branch of the repo or worktree at path.
// Returns "" for detached HEAD.
func CurrentBranch(path string) string {
	out, err := exec.Command("git", "-C", path, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// MainWorktree returns the path of the main worktree of the repo at path
// (i.e. the canonical clone, even if path is a worktree of it).
func MainWorktree(path string) string {
	entries, err := ListWorktrees(path)
	if err != nil || len(entries) == 0 {
		return path
	}
	return entries[0].Path
}
