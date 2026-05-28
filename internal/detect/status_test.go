package detect

import "testing"

// Title-based detection takes priority — agents announce their state via OSC
// titles (this is how Orca IDE does it).
func TestDetectFromTitle(t *testing.T) {
	cases := []struct {
		title string
		want  Status
	}{
		// Claude Code
		{"✳ Editing foo.py", StatusIdle},
		{". Working on refactor", StatusWorking},
		{"* Claude Code", StatusIdle},

		// Gemini
		{"✦ Gemini CLI", StatusWorking},
		{"◇ Gemini CLI", StatusIdle},
		{"✋ Gemini CLI", StatusNeedsInput},

		// Braille spinner → working (3+ dot frames only)
		{"⠋ Codex generating", StatusWorking},
		{"⠹ Aider running", StatusWorking},
		{"⠏ Claude working", StatusWorking},

		// Single-dot braille (⠂ U+2802) is Claude Code's STATIC session-ready
		// indicator — NOT a spinner. Must NOT classify as working.
		{"⠂ lazypilot-tui-plan", StatusIdle},
		{"⠁ some-other-task", StatusIdle},

		// Keyword-based
		{"Codex done", StatusIdle},
		{"OpenCode ready", StatusIdle},
		{"Aider thinking", StatusWorking},
		{"Claude - action required", StatusNeedsInput},

		// Negative cases
		{"", StatusUnknown},
		{"bash", StatusUnknown},
		{"~/codex already built", StatusIdle}, // mentions agent → assumed idle (no specific state)
		{"Cursor Agent", StatusUnknown},       // bare cursor native title

		// Path / hyphen false-positive guards
		{"is-ready-cap", StatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			if got := DetectFromTitle(tc.title); got != tc.want {
				t.Errorf("DetectFromTitle(%q) = %v, want %v", tc.title, got, tc.want)
			}
		})
	}
}

// Classify falls back to CPU + pane content only when the title yields no verdict.
func TestClassify_TitlePreferred(t *testing.T) {
	// Title says working — even with low CPU and pane saying "press enter".
	got := Classify("✦ Gemini CLI", 0.1, "press enter to continue")
	if got != StatusWorking {
		t.Errorf("title should override fallback signals, got %v", got)
	}
}

func TestClassify_FallbackCPU(t *testing.T) {
	if got := Classify("", 42.0, "anything"); got != StatusWorking {
		t.Errorf("no title + high CPU should be Working, got %v", got)
	}
}

func TestClassify_FallbackPaneContent(t *testing.T) {
	if got := Classify("", 0.5, "Are you sure? (y/n)"); got != StatusNeedsInput {
		t.Errorf("no title + (y/n) pattern should be NeedsInput, got %v", got)
	}
}

func TestClassify_FallbackIdle(t *testing.T) {
	if got := Classify("", 0.5, "$ "); got != StatusIdle {
		t.Errorf("no title + bare prompt should be Idle, got %v", got)
	}
}

func TestIsAI(t *testing.T) {
	ai := []string{"claude", "codex"}
	if !IsAI("claude", ai) {
		t.Error("claude should be AI")
	}
	if IsAI("bash", ai) {
		t.Error("bash should NOT be AI")
	}
}
