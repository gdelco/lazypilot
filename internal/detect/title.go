package detect

import (
	"regexp"
	"strings"
)

// Title-based agent detection — ported from Orca IDE
// (src/shared/agent-detection.ts). Agent CLIs set the terminal title via OSC
// escape sequences to announce their state; the title is a far more reliable
// signal than CPU usage or pane-content regex matching.
//
// tmux exposes the current OSC title as `pane_title`, so we read it via
// `tmux list-panes -F '#{pane_title}'` and classify here.

// Specific status glyphs used by individual agents.
const (
	claudeIdle           = "✳" // ✳  Claude Code idle prefix
	geminiWorking        = "✦" // ✦
	geminiSilentWorking  = "⏲" // ⏲
	geminiIdle           = "◇" // ◇
	geminiPermission     = "✋" // ✋
	piIdlePrefix         = "π - "
)

// agentNames is the substring set used to detect "this title belongs to an
// agent". Intentionally narrow so plain shell titles like "timestamp ready"
// don't accidentally classify.
var agentNames = []string{
	"claude", "codex", "copilot", "cursor", "gemini",
	"antigravity", "opencode", "openclaw", "aider", "grok",
}

// Keyword regexes — asymmetric lookarounds to avoid matching substrings inside
// paths (`~/codex/ready`) or hyphenated compounds (`is-ready-cap`).
// Match keywords like "Codex done.", "OpenCode ready!"
var (
	idleKeywordsRE       = regexp.MustCompile(`(?i)(?:^|[^\w./\\-])(ready|idle|done)(?:[^\w-]|$)`)
	workingKeywordsRE    = regexp.MustCompile(`(?i)(?:^|[^\w./\\-])(working|thinking|running)(?:[^\w-]|$)`)
	permissionKeywordsRE = regexp.MustCompile(`(?i)action required|permission|waiting`)
)

// DetectFromTitle classifies an agent state purely from the terminal title.
// Returns StatusUnknown when no agent is detected, so callers can fall back
// (e.g. to the CPU heuristic) for unclassified panes.
func DetectFromTitle(title string) Status {
	if title == "" {
		return StatusUnknown
	}
	// Cursor's bare "Cursor Agent" title carries no state — skip.
	if strings.EqualFold(strings.TrimSpace(title), "cursor agent") {
		return StatusUnknown
	}

	// Gemini's specific glyphs are the most precise — check first.
	if strings.Contains(title, geminiPermission) {
		return StatusNeedsInput
	}
	if strings.Contains(title, geminiWorking) || strings.Contains(title, geminiSilentWorking) {
		return StatusWorking
	}
	if strings.Contains(title, geminiIdle) {
		return StatusIdle
	}

	// Claude Code idle prefix.
	if strings.HasPrefix(title, claudeIdle+" ") || title == claudeIdle {
		return StatusIdle
	}

	// Pi titlebar extension.
	if strings.HasPrefix(title, piIdlePrefix) {
		return StatusIdle
	}

	// A single-dot braille prefix like "⠂ <task-name>" is Claude Code's
	// quiet session-ready indicator — must be checked BEFORE the spinner
	// branch so it's not mistaken for an active spinner frame.
	if r, n := firstRune(title); n > 0 && isStaticBrailleIndicator(r) {
		return StatusIdle
	}

	// Braille spinner (3+ set dots — actual cycling frames only).
	if containsBrailleSpinner(title) {
		return StatusWorking
	}

	// Claude Code prefixes: ". " = working, "* " = idle.
	if strings.HasPrefix(title, ". ") {
		return StatusWorking
	}
	if strings.HasPrefix(title, "* ") {
		return StatusIdle
	}

	// Title must mention an agent name to apply the keyword heuristics.
	if !containsAgentName(title) {
		return StatusUnknown
	}

	if permissionKeywordsRE.MatchString(title) {
		return StatusNeedsInput
	}
	if idleKeywordsRE.MatchString(title) {
		return StatusIdle
	}
	if workingKeywordsRE.MatchString(title) {
		return StatusWorking
	}

	// Title mentions an agent but says nothing about state → assume idle.
	return StatusIdle
}

func containsAgentName(title string) bool {
	lower := strings.ToLower(title)
	for _, name := range agentNames {
		if strings.Contains(lower, name) {
			return true
		}
	}
	return false
}

// containsBrailleSpinner returns true if the title contains a character that
// looks like a real cycling spinner frame.
//
// Cycling-spinner frames Claude Code / Codex / Aider use (⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏) all
// have 3-4 set dots. Single- and double-dot braille glyphs like ⠂ (U+2802)
// are STATIC indicators agents emit when a task is complete — treating them
// as "working" caused false positives where lazypilot showed sessions as
// working long after Claude went idle. So we require at least 3 set dots
// before classifying as a spinner.
func containsBrailleSpinner(title string) bool {
	for _, r := range title {
		if r < 0x2800 || r > 0x28FF {
			continue
		}
		if popcount8(byte(r-0x2800)) >= 3 {
			return true
		}
	}
	return false
}

// popcount8 returns the number of set bits in a byte.
func popcount8(b byte) int {
	c := 0
	for b != 0 {
		c += int(b & 1)
		b >>= 1
	}
	return c
}

// isStaticBrailleIndicator reports whether `r` is a 1- or 2-dot braille
// character — agents use these as quiet "ready" / "summary" markers, not as
// active spinner frames.
func isStaticBrailleIndicator(r rune) bool {
	if r < 0x2800 || r > 0x28FF {
		return false
	}
	return popcount8(byte(r-0x2800)) <= 2
}

// firstRune returns the first rune of s and its byte width.
func firstRune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}
