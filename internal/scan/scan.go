// Package scan walks the configured project roots and produces a deduplicated
// list of projects (repos, worktrees, workspaces). It mirrors the bash
// `collect()` function in tmux-sessionizer.
package scan

import (
	"bufio"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Pruned directory names — we never descend into these during the find pass.
var prunedDirs = map[string]bool{
	"node_modules": true,
	".next":        true,
	"target":       true,
	"dist":         true,
	".git":         true, // handled separately
}

// MaxDepth is how deep we walk looking for .git entries (mirrors bash maxdepth 6).
const MaxDepth = 6

// Collect returns the deduplicated, ordered list of projects found under all
// given roots. The order is: for each root in turn, the root itself if it's a
// repo, then its depth-1 children, then any nested repos discovered by walking.
// Worktrees registered with each repo (via `git worktree list`) are also
// included even when they live outside the roots.
func Collect(roots []string) []Project {
	seen := map[string]bool{}
	out := []Project{}
	add := func(p Project) {
		if p.Path == "" || seen[p.Path] {
			return
		}
		seen[p.Path] = true
		out = append(out, p)
	}

	for _, root := range roots {
		root = expandHome(root)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}

		// If the root itself is a repo, emit + worktrees, no descent.
		if k := classify(root); k == KindRepo || k == KindWorktree {
			add(Project{Path: root, Kind: k, SourceRepo: sourceRepo(root)})
			for _, wt := range listWorktrees(root) {
				add(wt)
			}
			continue
		}

		// Otherwise emit depth-1 children for navigation.
		entries, err := os.ReadDir(root)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				p := filepath.Join(root, e.Name())
				add(Project{Path: p, Kind: classify(p)})
			}
		}

		// Walk for nested .git entries up to MaxDepth, pruning noise.
		walkRoot(root, &add)
	}

	return out
}

// walkRoot finds nested .git entries inside root (between depths 2 and MaxDepth)
// and emits the parent dir as a repo/worktree, plus any worktrees registered
// with that repo.
func walkRoot(root string, add *func(Project)) {
	baseDepth := strings.Count(root, string(os.PathSeparator))

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		depth := strings.Count(path, string(os.PathSeparator)) - baseDepth
		if depth > MaxDepth {
			return fs.SkipDir
		}

		name := d.Name()
		if d.IsDir() && prunedDirs[name] && name != ".git" {
			return fs.SkipDir
		}

		if name == ".git" && depth >= 2 {
			parent := filepath.Dir(path)
			kind := classify(parent)
			(*add)(Project{Path: parent, Kind: kind, SourceRepo: sourceRepo(parent)})
			for _, wt := range listWorktrees(parent) {
				(*add)(wt)
			}
			return fs.SkipDir // don't descend INTO .git
		}
		return nil
	})
}

// classify returns the Kind of a directory by inspecting its .git entry.
func classify(dir string) Kind {
	info, err := os.Lstat(filepath.Join(dir, ".git"))
	if err != nil {
		return KindWorkspace
	}
	if info.IsDir() {
		return KindRepo
	}
	// Regular file (gitlink) or symlink → worktree.
	return KindWorktree
}

// sourceRepo returns the main repo of a worktree (the first entry from
// `git worktree list --porcelain`). For a non-worktree, returns "".
func sourceRepo(p string) string {
	if classify(p) != KindWorktree {
		return ""
	}
	out, err := exec.Command("git", "-C", p, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree ")
		}
	}
	return ""
}

// listWorktrees runs `git worktree list --porcelain` and returns every
// registered worktree of a repo as Projects. The first entry of the porcelain
// output is the main repo; the rest are linked worktrees.
func listWorktrees(repo string) []Project {
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil
	}

	var main string
	var paths []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			p := strings.TrimPrefix(line, "worktree ")
			if main == "" {
				main = p
				continue
			}
			paths = append(paths, p)
		}
	}

	projects := make([]Project, 0, len(paths))
	for _, p := range paths {
		projects = append(projects, Project{
			Path:       p,
			Kind:       KindWorktree,
			SourceRepo: main,
		})
	}
	return projects
}

// expandHome replaces a leading ~ with $HOME.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
