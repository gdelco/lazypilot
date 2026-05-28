# lazypilot — Roadmap

A curated, opinionated feature list, organized by how much they'd actually move the needle for the worktree-+-agents workflow lazypilot is built for. Each entry says **what** it is, **why** it matters, and (where applicable) **where the idea came from** (ATM / herdr / original).

Status legend:
- 🟢 shipped
- 🟡 in progress
- ⚪ planned
- ⚫ idea (not yet decided)

References:
- [damelLP/agent-tmux-manager](https://github.com/damelLP/agent-tmux-manager) (Rust, daemon + adapters, Claude Code / Pi)
- [ogulcancelik/herdr](https://github.com/ogulcancelik/herdr) (Rust, own multiplexer, mouse-native, 14+ supported agents)

---

## Tier 0 — already shipped

- 🟢 Three-tab TUI (Sessions / Projects / Worktrees) with vim navigation
- 🟢 OSC-title-based agent state detection (claude / codex / gemini / pi)
- 🟢 opencode plugin (file-based status channel)
- 🟢 Worktree wizard (codename + container picker + base-ref search)
- 🟢 Confirmation modals (remove / kill)
- 🟢 Help overlay (`?`)
- 🟢 `/` live filter on every view
- 🟢 3-panel layout (list + detail + live preview)
- 🟢 Pane drill-in with `Tab` + per-pane Enter attach
- 🟢 AI-picker on new-session create (editor + AI in 60/40 split, configurable)
- 🟢 Recent-activity digest preview for Projects (branch + tracking + status counts + commits)

---

## Tier 1 — next, highest leverage

These are the features that change lazypilot from "I open it to check on agents" to "agents reach out to me."

### ⚪ Daemon (`lazypilotd`) + tmux status-bar integration
> *inspiration: ATM (`atmd` + `atm status` subcommand)*

Background process that:
- polls tmux + opencode hooks + Claude OSC titles continuously
- exposes a unix socket at `$XDG_RUNTIME_DIR/lazypilot.sock`
- writes a one-line summary to a file (`/tmp/lazypilot-status`) that tmux's status bar can show: `⚠ 2 agents waiting · 1 working`

Without this, you only see state when you open the TUI. With it, the status bar pings you while you're in nvim.

Wiring example for `~/.tmux.conf`:
```tmux
set -g status-right '#(cat /tmp/lazypilot-status 2>/dev/null) | %H:%M'
set -g status-interval 2
```

### ⚪ Desktop notifications when an agent flips to "needs input"
> *inspiration: herdr (toasts + sounds), my own existing roadmap entry*

`notify-send` on Linux, `osascript` on macOS. Optional sound. The trigger fires only on **state transitions** (idle/working → needs-input), not on every poll, so it's not noisy.

Configurable per agent type (`~/.config/lazypilot/config.yaml`):
```yaml
notifications:
  enabled: true
  sound: true
  on:
    - needs_input
    - done   # optional: also ping when claude finishes
```

### ⚪ CLI subcommands
> *inspiration: ATM (`atm list`, `atm status`, `atm send`, `atm reply`)*

Beyond just the TUI, expose lazypilot's data and actions over a CLI:

```bash
lazypilot list                          # all sessions + agent status, plain output
lazypilot list -f json                  # JSON for scripting
lazypilot list --status needs_input     # filter to blocked agents
lazypilot status                        # one-line summary (for tmux status-right)
lazypilot kill <session>                # kill a tmux session
lazypilot peek <session>                # extract last lines of pane content
lazypilot send <session> "fix the tests"  # type text into the agent's pane
lazypilot reply <session> --yes         # answer a y/n prompt
```

Most of these are thin wrappers around the existing logic + the daemon's socket.

---

## Tier 2 — workflow improvements

### ⚪ Stale-worktree cleanup
> *original*

Detect worktrees whose branch was merged on `origin` or deleted upstream. Add a `Shift-D` action that bulk-removes selected stale worktrees. Worktree clutter is a real problem after a few months of parallel work.

### ⚪ Quick git ops on a selected worktree
> *original*

`gp` pull, `gP` push, `gs` stash, `gc` commit (opens nvim commit buffer in a temp split). Avoids the round-trip into the worktree just to type two characters.

### ⚪ Activity log / unread indicators
> *inspiration: herdr's "🔵 done" state (work finished, you have not looked at it yet)*

Track when each session's content last changed, and mark a session as "🔵 has new output" until you've drilled into its preview. Across multiple parallel agents this is genuinely how you know which session to look at first.

### ⚪ State filters in the picker
> *inspiration: herdr (b/w/i/d keys in their session navigator)*

While in lazypilot, press:
- `Sb` — filter to **blocked** (needs-input) sessions
- `Sw` — filter to **working** sessions
- `Sd` — filter to **done** (since-last-look) sessions
- `Si` — filter to **idle** sessions

Stack with `/` text filter.

### ⚪ Persistent last-view
> *original*

Save the active tab + cursor position to `~/.cache/lazypilot/state.json` on exit. Restore on next launch. Already obvious once you've used it twice.

---

## Tier 3 — quality of life

### ⚫ Mouse support
> *inspiration: herdr (click panes/tabs, drag borders, double-click to copy)*

Click a row to select it. Click a tab to switch. Drag the panel borders to resize the list/detail/preview split. Right-click for a context menu (open / kill / new-worktree).

In bubbletea, `tea.WithMouseAllMotion()` enables mouse — implementation cost is moderate; the main win is for users who *like* mouse-driven TUIs. Vim diehards will ignore.

### ⚫ Themes / palettes
> *inspiration: herdr (18 themes, light/dark variants)*

Move the hardcoded 16-color palette into a theme system. Ship a few presets: `everforest-dark`, `everforest-light`, `catppuccin-mocha`, `tokyo-night`, `gruvbox`, `solarized-light`. Theme selectable via `~/.config/lazypilot/config.yaml` and a `t` keybinding in lazypilot to cycle.

### ⚫ Customizable keybindings
> *inspiration: herdr (`[keys]` section in config.toml)*

```yaml
keys:
  quit: "q,ctrl+c"
  open: "enter"
  new_worktree: "n"
  filter: "/"
  custom_pull:
    keys: "gp"
    command: "git pull"
    scope: pane    # run in the worktree's pane
```

Lets people who hate `K` for kill remap it. Also enables the "shell-command" keybinding pattern herdr has (launch lazygit / btop / whatever in a temporary pane).

---

## Tier 4 — agent-aware

This tier is what makes lazypilot specifically useful for *agent-driven coding*, not just tmux session management.

### ⚫ Context / cost tracking
> *inspiration: ATM (context bars + cost tracking in the dashboard)*

Claude Code writes everything to `~/.claude/projects/<hashed-project>/*.jsonl`:
- input/output token counts per turn
- tool calls
- model used
- session start/end times

A small parser can surface, per session:
- **% of context window used** ("80% — auto-compact soon")
- **$ spent today** in this session
- **Model in use** (Sonnet 4.5 / Opus 4.7)

Surface in the preview pane next to the agent status. Maybe a global "today's spend" total in the footer.

### ⚫ Per-agent adapters (formalized)
> *inspiration: ATM (`atm-claude-adapter`, `atm-pi-adapter`)*

Today, agent detection is split across hard-coded logic in `internal/detect/title.go` + `internal/opencodehook/`. Formalize this:

```
internal/adapters/
├── adapter.go         # interface
├── claude.go          # OSC titles, ~/.claude/projects logs
├── codex.go           # OSC titles
├── opencode.go        # plugin file
├── gemini.go          # OSC titles (Gemini-specific glyphs)
├── aider.go
└── droid.go
```

Adding a new agent becomes "implement the Adapter interface." Plays nicely with herdr's much wider supported-agents list (14+).

### ⚫ Agent control (send / reply / interrupt)
> *inspiration: ATM (`atm send`, `atm reply`, `atm interrupt`)*

When you're not at the agent's pane and lazypilot is showing it as "needs input":
- `R` on a needs-input session — prompt for a single-char reply (y/n) and send it via `tmux send-keys`
- `I` — send `C-c` (interrupt the agent)
- `S` — open a one-line input modal, send the typed text via `tmux send-keys`

Most useful for the "babysitting permission prompts" case.

### ⚫ Wait-for-state CLI (for scripting)
> *inspiration: herdr (`herdr wait agent-status 1-1 --status done`)*

```bash
lazypilot wait <session> --status done --timeout 600
```

Blocks until the agent in that session transitions to the named state, then exits 0. Useful for shell scripts that orchestrate agents:

```bash
# launch claude in a worktree, wait for it to finish, then merge
lazypilot open ~/work/feature-x --ai claude
lazypilot send feature-x "implement the design doc"
lazypilot wait feature-x --status done
git -C ~/work/feature-x push -u origin
gh pr create
```

---

## Tier 5 — power features

### ⚫ Workspace layout presets
> *inspiration: ATM (`atm layout pair`, `atm workspace create` with sidebars)*

Beyond the current "editor + 1 AI" layout, define more in config:

```yaml
layouts:
  pair:          # 2 AIs side-by-side
    - { cmd: claude,  split: vertical, size: "50%" }
    - { cmd: opencode, split: right,    size: "50%" }
  squad:         # editor + 3 agents in a quad
    - { cmd: nvim ., split: tile }
    - { cmd: claude, split: tile }
    - { cmd: codex,  split: tile }
    - { cmd: aider,  split: tile }
  review:        # editor + lazygit + ai
    - { cmd: nvim .,   split: left,  size: "60%" }
    - { cmd: lazygit,  split: top,   size: "40%" }
    - { cmd: claude,   split: right, size: "40%" }
```

`L` key opens a layout picker after pressing Enter on a new session.

### ⚫ Mini-sidebar mode (inside an existing tmux session)
> *inspiration: ATM (`atm workspace attach` — injects a sidebar into the current session)*

Instead of always running lazypilot in a popup, allow attaching it as a persistent **3-column sidebar** inside an existing tmux window: editor on the left, agent panel in the middle, lazypilot status sidebar on the right. Updates live while you code.

### ⚫ Remote attach
> *inspiration: herdr (`herdr --remote workbox`)*

`lazypilot --remote workbox` SSHes to `workbox`, runs `lazypilot` there, and streams the TUI back to your local terminal. Same UI, but the data + agents live remotely. Lets you supervise dev-box agents from a laptop / phone over SSH.

### ⚫ Socket API for agents to orchestrate lazypilot
> *inspiration: herdr (full socket API + SKILL.md for agents)*

Once `lazypilotd` exists, expose its socket for agents to call:
- `lazypilot worktree create --branch feature/x` (agent spins up a worktree itself before doing work)
- `lazypilot session split --add claude` (agent forks itself into a paired claude pane)
- `lazypilot wait session=X status=done` (agent A waits for agent B to finish before consuming its output)

This turns lazypilot into a tool agents themselves can use, not just one humans use to watch agents.

---

## Explicit non-goals

- **Replace tmux.** We stay inside tmux. herdr's path of being its own multiplexer means losing tmux-resurrect, your existing config, your muscle memory. Wrong trade for this project.
- **GUI / Electron / web view.** Lives in the terminal only.
- **Per-OS-specific code** beyond notifications. The core experience must be the same on Linux and macOS.
- **Hide the underlying tools.** Users should always be able to drop into `git`, `tmux`, `claude` etc. directly without lazypilot in the way. lazypilot is an *assistant*, not an *abstraction*.

---

## Priority order I'd actually code in

If the goal is "make daily work with worktrees + agents materially better":

1. **Daemon + status bar + notifications** (Tier 1) — closes the "I missed an agent for 20 minutes" gap
2. **Stale-worktree cleanup + activity log** (Tier 2) — keeps the worktree pile manageable
3. **Context / cost tracking** (Tier 4) — Claude Code's auto-compact warnings are useful but easy to miss
4. **CLI subcommands** (Tier 1) — unlocks scripting and tmux-status-bar embedding
5. **State filters + persistent last-view** (Tier 2) — small QoL but compounds

Themes, mouse, layouts, remote attach are nice-to-haves I'd skip until someone actually asks for them.
