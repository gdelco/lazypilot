package tmuxctl

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Pane is a tmux pane snapshot.
type Pane struct {
	SessionName string
	WindowIndex int
	PaneIndex   int
	Command     string // pane_current_command
	PID         int
	PaneID      string // e.g. "%17"
	Title       string // pane_title — terminal title set via OSC escape sequence by the agent
}

// Label returns a display name like "1.0  claude".
func (p Pane) Label() string {
	return strings.TrimSpace(strings.Join([]string{
		strconv.Itoa(p.WindowIndex) + "." + strconv.Itoa(p.PaneIndex),
		p.Command,
	}, "  "))
}

// ListPanes returns every pane across every tmux session.
func ListPanes() ([]Pane, error) {
	out, err := tmux("list-panes", "-a",
		"-F", "#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_current_command}\t#{pane_pid}\t#{pane_id}\t#{pane_title}").Output()
	if err != nil {
		if isNoServer(err) {
			return nil, nil
		}
		return nil, err
	}

	var panes []Pane
	for _, line := range splitLines(string(out)) {
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		wi, _ := strconv.Atoi(f[1])
		pi, _ := strconv.Atoi(f[2])
		pid, _ := strconv.Atoi(f[4])
		panes = append(panes, Pane{
			SessionName: f[0],
			WindowIndex: wi,
			PaneIndex:   pi,
			Command:     f[3],
			PID:         pid,
			PaneID:      f[5],
			Title:       f[6],
		})
	}
	return panes, nil
}

// ListPanesIn returns the panes of a single session.
func ListPanesIn(session string) ([]Pane, error) {
	all, err := ListPanes()
	if err != nil {
		return nil, err
	}
	var out []Pane
	for _, p := range all {
		if p.SessionName == session {
			out = append(out, p)
		}
	}
	return out, nil
}

// CapturePane returns the full visible content of the given pane with ANSI
// escape sequences preserved.
//
// We do NOT pass `-J` (join wrapped lines) or `-S/-E` (custom range):
//
//   - Full-screen TUIs (opencode, vim, htop) lay their UI out across the
//     entire visible pane height. `-J` collapses each TUI row into a single
//     joined line, destroying the layout.
//   - `-S -N` slices off the bottom N rows of visible, which is wrong for
//     a TUI whose interesting content sits at the TOP.
//
// `lines` is kept on the signature for symmetry with future plain-text
// callers, but currently ignored — we always return the full visible region
// and let the caller clip via clipToBox.
func CapturePane(paneID string, lines int) (string, error) {
	_ = lines
	out, err := tmux("capture-pane", "-p", "-e", "-t", paneID).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SessionNameFor computes the tmux session name we'd use for a given path.
//
// Rules (mirror the bash `session_name_for`):
//
//  1. Prefer the bare basename of the path (with " .:" -> "_") if (a) no session
//     by that name exists, OR (b) one exists and its session_path matches the
//     exact path.
//  2. Otherwise fall back to parent__leaf so we don't collide.
func SessionNameFor(path string) string {
	leaf := sanitize(filepath.Base(path))
	if existing := SessionPath(leaf); existing == "" || existing == path {
		return leaf
	}
	parent := sanitize(filepath.Base(filepath.Dir(path)))
	return parent + "__" + leaf
}

func sanitize(s string) string {
	r := strings.NewReplacer(" ", "_", ".", "_", ":", "_")
	return r.Replace(s)
}
