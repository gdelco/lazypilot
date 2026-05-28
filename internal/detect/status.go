// Package detect classifies the state of an AI agent running inside a tmux
// pane. The "universal" detector combines two signals:
//
//  1. CPU usage of the pane's process tree — high CPU means the agent is
//     actively generating / running tools.
//  2. The last few lines of pane content — agents tend to display predictable
//     prompts when they're blocked on the user.
//
// Neither signal alone is perfect; together they distinguish working / idle /
// needs-input with reasonable accuracy across claude, codex, opencode, aider.
package detect

// Status is the classification of a single AI pane.
type Status int

const (
	StatusUnknown    Status = iota // not an AI process
	StatusWorking                  // CPU high → generating
	StatusIdle                     // CPU low + no input prompt detected
	StatusNeedsInput               // CPU low + prompt pattern matched in last lines
)

func (s Status) String() string {
	switch s {
	case StatusWorking:
		return "working"
	case StatusIdle:
		return "idle"
	case StatusNeedsInput:
		return "needs-input"
	}
	return "unknown"
}

// Icon returns a short colored bullet used for display.
func (s Status) Icon() string {
	switch s {
	case StatusWorking:
		return "●" // yellow in the renderer
	case StatusNeedsInput:
		return "●" // red
	case StatusIdle:
		return "●" // green
	}
	return "○" // dim
}

// IsAI reports whether the given pane command is one of the known AI agents
// (matched exactly against the configured list).
func IsAI(command string, aiList []string) bool {
	for _, name := range aiList {
		if command == name {
			return true
		}
	}
	return false
}

// Classify decides the status using the most reliable signal available.
//
// Priority order (Orca-IDE approach):
//  1. **Terminal title** (`pane_title`) — agents announce their state via OSC
//     title escapes. This is authoritative and updates instantly. Always
//     preferred when the title yields a non-Unknown classification.
//  2. **CPU + pane content fallback** — for agents that don't set titles, or
//     when the title doesn't yield a verdict, fall back to CPU heuristic +
//     regex on the pane's last visible lines.
func Classify(title string, cpu float64, paneContent string) Status {
	if s := DetectFromTitle(title); s != StatusUnknown {
		return s
	}
	if cpu > cpuWorkingThreshold {
		return StatusWorking
	}
	if matchesNeedsInput(paneContent) {
		return StatusNeedsInput
	}
	return StatusIdle
}

const cpuWorkingThreshold = 5.0
