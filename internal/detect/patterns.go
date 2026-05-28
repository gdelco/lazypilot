package detect

import (
	"regexp"
	"strings"
)

// needsInputPatterns are regexes matched against the LAST non-empty line of
// the pane capture (and the second-to-last, since cursor often sits on a
// blank line after the prompt). A match means the agent is blocked on the user.
//
// Curated to be conservative — false positives mean we tell the user "needs
// input" when nothing actually does, which is more annoying than missing a
// real prompt. Add patterns as you observe them in real sessions.
var needsInputPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\([yY]/[nN]\)`),                  // (y/n), (Y/n), (y/N)
	regexp.MustCompile(`(?i)\([nN]/[yY]\)`),                  // (n/y)
	regexp.MustCompile(`(?i)\(yes/no\)`),                     // (yes/no)
	regexp.MustCompile(`(?i)\byes/no\?\s*$`),                 // ... yes/no?
	regexp.MustCompile(`(?i)press\s+(enter|return|any\s+key)`), // "press enter to continue"
	regexp.MustCompile(`(?i)\bconfirm\b.*\?\s*$`),            // "Confirm ...?"
	regexp.MustCompile(`(?i)\bapprov(e|al)\b`),               // "approve", "approval"
	regexp.MustCompile(`(?i)\ballow\b.*\?\s*$`),              // "allow this ...?"
	regexp.MustCompile(`(?i)\bpermission\b`),                 // "needs permission"
	regexp.MustCompile(`(?i)\bwaiting\s+for\b`),              // "waiting for input"
	regexp.MustCompile(`(?i)1\.\s.*\n\s*2\.\s.*\n\s*3\.\s`),  // numbered choice list
}

// matchesNeedsInput returns true if any pattern matches the tail of the pane content.
func matchesNeedsInput(content string) bool {
	if content == "" {
		return false
	}
	// Inspect just the tail; agents print prompts at the bottom.
	tail := tailLines(content, 6)

	// Match against the full tail (multi-line patterns work).
	for _, re := range needsInputPatterns {
		if re.MatchString(tail) {
			return true
		}
	}
	return false
}

// tailLines returns the last `n` non-empty lines of s as a single string
// (newline-joined). If fewer than n lines exist, returns what's available.
func tailLines(s string, n int) string {
	s = strings.TrimRight(s, "\n \t")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	// Drop trailing empty lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
