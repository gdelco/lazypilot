// Package tmuxctl wraps the tmux CLI for the operations lazypilot needs.
// Every call is a one-shot exec of `tmux ...` — no control-mode for now.
package tmuxctl

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Session is a tmux session as reported by `tmux list-sessions`.
type Session struct {
	Name        string
	Path        string // session_path = working dir of the session
	WindowCount int
	Attached    bool
	Created     int64 // unix timestamp
}

// ListSessions runs `tmux list-sessions` and parses the result.
func ListSessions() ([]Session, error) {
	out, err := tmux("list-sessions",
		"-F", "#{session_name}\t#{session_path}\t#{session_windows}\t#{session_attached}\t#{session_created}").Output()
	if err != nil {
		// No server / no sessions — not an error for us.
		if isNoServer(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, line := range splitLines(string(out)) {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		attached, _ := strconv.Atoi(fields[3])
		windows, _ := strconv.Atoi(fields[2])
		created, _ := strconv.ParseInt(fields[4], 10, 64)
		sessions = append(sessions, Session{
			Name:        fields[0],
			Path:        fields[1],
			WindowCount: windows,
			Attached:    attached > 0,
			Created:     created,
		})
	}
	return sessions, nil
}

// HasSession returns true if a session with the given name exists.
func HasSession(name string) bool {
	return tmux("has-session", "-t="+name).Run() == nil
}

// SessionPath returns the session_path of an existing session, or "" if missing.
func SessionPath(name string) string {
	out, err := tmux("display-message", "-t="+name, "-p", "#{session_path}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// NewSession creates a detached session.
func NewSession(name, dir string) error {
	return tmux("new-session", "-ds", name, "-c", dir).Run()
}

// KillSession terminates the named session.
func KillSession(name string) error {
	return tmux("kill-session", "-t", name).Run()
}

// AttachOrSwitch attaches (when outside tmux) or switches-client (when inside).
// Returns the *exec.Cmd so the caller can release the terminal first if needed.
func AttachOrSwitch(name string) *exec.Cmd {
	if os.Getenv("TMUX") != "" {
		return tmux("switch-client", "-t", name)
	}
	return tmux("attach", "-t", name)
}

// IsInsideTmux reports whether the current process is running inside a tmux
// session (i.e. $TMUX is set).
func IsInsideTmux() bool { return os.Getenv("TMUX") != "" }

// tmux returns an *exec.Cmd for `tmux ...`.
func tmux(args ...string) *exec.Cmd { return exec.Command("tmux", args...) }

// splitLines returns the lines of s with trailing newline stripped and empty
// lines removed.
func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func isNoServer(err error) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return false
	}
	return bytes.Contains(ee.Stderr, []byte("no server running")) ||
		bytes.Contains(ee.Stderr, []byte("error connecting"))
}
