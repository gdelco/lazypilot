// Package opencodehook installs and reads an opencode plugin that broadcasts
// session state (busy / idle / permission requests) by writing per-process
// status files. This mirrors Orca IDE's approach but without the loopback
// HTTP server — opencode lacks an OSC-title state convention, so its own
// event hooks are the only reliable signal.
package opencodehook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PluginFilename is the name we use under the opencode plugin directory.
const PluginFilename = "lazypilot-status.js"

// StatusDir holds per-PID state files written by the plugin.
// Lives in /tmp so it's volatile, world-writable for the user, and survives
// container/restart isolation.
const StatusDir = "/tmp/lazypilot-opencode"

// Status is the parsed state from the plugin's JSON output.
type Status struct {
	PID        int    `json:"pid"`
	SessionID  string `json:"session_id"`
	State      string `json:"state"` // "working" / "idle" / "needs_input"
	UpdatedAt  string `json:"updated_at"`
	Permission bool   `json:"permission"`
}

// PluginSource is the JavaScript plugin shipped to opencode. It maps
// opencode's session/permission events to status entries in StatusDir.
//
// Events handled (subset of opencode's plugin API):
//
//   - session.status busy/idle/retry → flip working/idle
//   - session.idle / session.error   → idle
//   - permission.asked               → needs_input (until next status update)
//   - permission.replied             → clear needs_input
//
// The plugin writes JSON keyed by its own process PID so lazypilot can match
// the file to the tmux pane's `pane_pid`.
const pluginSource = `// lazypilot opencode hook — installed by lazypilot, do not edit by hand.
// Writes session state to /tmp/lazypilot-opencode/<pid>.json so a separate
// lazypilot process can render real-time per-pane status.

const fs = require("fs");
const path = require("path");

const STATUS_DIR = "/tmp/lazypilot-opencode";
const PID = process.pid;
const STATUS_FILE = path.join(STATUS_DIR, PID + ".json");

let state = "idle";
let permission = false;
let sessionID = "";

function ensureDir() {
  try { fs.mkdirSync(STATUS_DIR, { recursive: true, mode: 0o755 }); } catch (_) {}
}

function write() {
  ensureDir();
  const payload = {
    pid: PID,
    session_id: sessionID,
    state: permission ? "needs_input" : state,
    permission,
    updated_at: new Date().toISOString(),
  };
  try { fs.writeFileSync(STATUS_FILE, JSON.stringify(payload)); } catch (_) {}
}

function set(next) { if (state !== next) { state = next; write(); } }

// Clean up our status file when opencode exits so we don't show stale state.
function cleanup() {
  try { fs.unlinkSync(STATUS_FILE); } catch (_) {}
}
process.once("exit", cleanup);
process.once("SIGINT", () => { cleanup(); process.exit(0); });
process.once("SIGTERM", () => { cleanup(); process.exit(0); });

export const LazypilotStatusPlugin = async (_ctx) => ({
  event: async ({ event }) => {
    if (!event || !event.type) return;
    const props = event.properties || {};
    if (props.sessionID) sessionID = props.sessionID;

    switch (event.type) {
      case "session.idle":
      case "session.error":
        set("idle");
        return;

      case "session.status": {
        const t = props.status && props.status.type;
        if (t === "busy" || t === "retry") set("working");
        else if (t === "idle") set("idle");
        return;
      }

      case "permission.asked":
      case "question.asked":
        permission = true;
        write();
        return;

      case "permission.replied":
        permission = false;
        write();
        return;
    }
  },
});

// Initial write so lazypilot can show the pane immediately on startup.
write();
`

// Install writes the plugin to the user's global opencode plugin directory
// if it's missing or out of date.  Returns the path it wrote (or would have
// written) and any error.
func Install() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "opencode", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create opencode plugin dir: %w", err)
	}
	dst := filepath.Join(dir, PluginFilename)

	// Skip write if content is already identical.
	if existing, err := os.ReadFile(dst); err == nil && string(existing) == pluginSource {
		return dst, nil
	}

	if err := os.WriteFile(dst, []byte(pluginSource), 0o644); err != nil {
		return dst, fmt.Errorf("write plugin: %w", err)
	}
	return dst, nil
}

// Read returns the most recent status reported by the plugin for the given
// process pid. Returns (nil, nil) when no status file exists or it's stale
// (older than 30s, which means the opencode process died without cleanup).
func Read(pid int) (*Status, error) {
	if pid <= 0 {
		return nil, nil
	}
	path := filepath.Join(StatusDir, strconv.Itoa(pid)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	// Drop stale entries; protects against zombie status files when the
	// plugin's cleanup didn't run (kill -9, crash, etc.).
	if s.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, s.UpdatedAt); err == nil {
			if time.Since(t) > 30*time.Second {
				return nil, nil
			}
		}
	}
	return &s, nil
}

// Cleanup deletes stale status files (older than 30s). Best-effort; intended
// to be called periodically from lazypilot to keep /tmp clean.
func Cleanup() {
	entries, err := os.ReadDir(StatusDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-30 * time.Second)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		full := filepath.Join(StatusDir, e.Name())
		info, err := e.Info()
		if err != nil || info.ModTime().Before(cutoff) {
			_ = os.Remove(full)
		}
	}
}
