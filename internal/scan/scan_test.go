package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// touchGit creates an empty .git/ dir at p (i.e. simulates a repo).
func touchGit(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(p, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// touchGitFile creates a .git file at p (i.e. simulates a worktree).
func touchGitFile(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, ".git"), []byte("gitdir: somewhere"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCollect_FlatLayout(t *testing.T) {
	root := t.TempDir()
	touchGit(t, filepath.Join(root, "alpha"))
	touchGit(t, filepath.Join(root, "beta"))
	mkdir(t, filepath.Join(root, "gamma"))

	got := Collect([]string{root})

	want := map[string]Kind{
		filepath.Join(root, "alpha"): KindRepo,
		filepath.Join(root, "beta"):  KindRepo,
		filepath.Join(root, "gamma"): KindWorkspace,
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d (%+v)", len(got), len(want), got)
	}
	for _, p := range got {
		wantKind, ok := want[p.Path]
		if !ok {
			t.Errorf("unexpected path: %s", p.Path)
			continue
		}
		if p.Kind != wantKind {
			t.Errorf("%s: kind %s, want %s", p.Path, p.Kind, wantKind)
		}
	}
}

func TestCollect_NestedRepo(t *testing.T) {
	root := t.TempDir()
	// workspace/diga is a non-repo dir, containing nested repos
	touchGit(t, filepath.Join(root, "workspace", "diga", "backend"))
	touchGitFile(t, filepath.Join(root, "workspace", "diga", "worktrees", "backend", "lucky-otter"))

	got := Collect([]string{root})

	have := map[string]bool{}
	for _, p := range got {
		have[p.Path] = true
	}

	// Depth-1 workspaces are emitted; deeper non-repo dirs are not (matches bash behavior).
	// Nested repos and worktrees at any depth up to MaxDepth are picked up.
	for _, expected := range []string{
		filepath.Join(root, "workspace"),
		filepath.Join(root, "workspace", "diga", "backend"),
		filepath.Join(root, "workspace", "diga", "worktrees", "backend", "lucky-otter"),
	} {
		if !have[expected] {
			t.Errorf("missing expected path: %s", expected)
		}
	}
	// And depth-2 workspace `diga` should NOT be emitted (it's a non-repo dir at depth > 1).
	digaPath := filepath.Join(root, "workspace", "diga")
	if have[digaPath] {
		t.Errorf("depth-2 non-repo workspace leaked: %s", digaPath)
	}
}

func TestCollect_PrunesNoiseDirs(t *testing.T) {
	root := t.TempDir()
	touchGit(t, filepath.Join(root, "myapp", "node_modules", "some-pkg")) // must not be discovered

	got := Collect([]string{root})

	for _, p := range got {
		if filepath.Base(filepath.Dir(p.Path)) == "node_modules" {
			t.Errorf("node_modules entry leaked: %s", p.Path)
		}
	}
}

func TestCollect_RootIsRepo(t *testing.T) {
	root := t.TempDir()
	touchGit(t, root)
	// Add a junk file under root that would normally be enumerated — should be skipped
	// because when the root IS a repo we don't descend.
	mkdir(t, filepath.Join(root, "src"))

	got := Collect([]string{root})

	if len(got) == 0 || got[0].Path != root || got[0].Kind != KindRepo {
		t.Fatalf("expected root itself as repo, got %+v", got)
	}
	// And `src` should NOT appear as a separate entry.
	for _, p := range got {
		if p.Path == filepath.Join(root, "src") {
			t.Error("src/ leaked into picker (root-is-repo should not descend)")
		}
	}
}

func TestProject_Short(t *testing.T) {
	p := Project{Path: "/home/x/Documents/foo"}
	if got := p.Short("/home/x"); got != "~/Documents/foo" {
		t.Errorf("Short: got %q, want %q", got, "~/Documents/foo")
	}
	if got := p.Short("/other"); got != "/home/x/Documents/foo" {
		t.Errorf("Short with non-prefix home should return full path, got %q", got)
	}
}
