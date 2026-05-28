package gitctl

import (
	"os/exec"
	"strings"
)

// ListBranches returns local and remote branch refs of the repo, with the
// current branch first and duplicates removed. Bare remote names (like
// "origin" alone) and "HEAD -> …" pseudo-refs are filtered out.
func ListBranches(repo string) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	add(CurrentBranch(repo))

	if b, err := exec.Command("git", "-C", repo, "branch", "--format=%(refname:short)").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			add(line)
		}
	}

	if b, err := exec.Command("git", "-C", repo, "branch", "-r", "--format=%(refname:short)").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			// Skip "HEAD -> ..." pseudo-refs and bare remote names without "/".
			if strings.Contains(line, "HEAD ->") || !strings.Contains(line, "/") {
				continue
			}
			add(line)
		}
	}

	return out
}
