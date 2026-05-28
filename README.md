# lazypilot

A lazygit-style TUI for tmux sessions, git worktrees, and AI agents.

Built because the bash sessionizer ([gdelco/tmux-config](https://github.com/gdelco/tmux-config)) hit fzf's single-panel limit and couldn't surface what each session is actually doing.

## What it shows

Three views, switchable with `1` / `2` / `3` (or `Tab`):

```
╔══ SESSIONS (3/5) ═══════════════╗   ╔══ DETAILS ═══════════════════╗
║ ▶ ◐ ● backend                   ║   ║ backend                      ║
║   ! ○ frontend                  ║   ║ ~/code/foo                   ║
║   ○ ○ docs                      ║   ║                              ║
║   · ○ random                    ║   ║ ▸ Panes                      ║
║                                 ║   ║   ◐ 1.0 claude  (working)    ║
║                                 ║   ║   ! 1.1 opencode  (needs)    ║
║                                 ║   ║   · 1.2 zsh                  ║
╚═════════════════════════════════╝   ╚══════════════════════════════╝
```

- **Sessions** — every running tmux session with the AI status of each pane (working / needs-input / idle / unknown). Detected from OSC terminal titles (Claude, Codex, Gemini, Pi) and from a small plugin installed into opencode.
- **Projects** — every repo / worktree / workspace under your configured roots. Detail pane shows branch, status, recent commits, and any active tmux session.
- **Worktrees** — every git worktree grouped under its source repo, with a wizard (`n`) for creating new ones.

## Install

Requires Go 1.22+ and tmux. Optional: a Nerd Font terminal for the icons.

```bash
git clone git@github.com:gdelco/lazypilot.git ~/code/lazypilot
cd ~/code/lazypilot
./install.sh
```

The installer builds the binary, symlinks it to `~/.local/bin/lazypilot`, and writes a default config to `~/.config/lazypilot/config.yaml` if one doesn't exist.

## Wire it into tmux

Replace your fzf-based sessionizer binding with lazypilot:

```tmux
# ~/.tmux.conf
bind f display-popup -E -w 95% -h 90% "~/.local/bin/lazypilot"
```

Reload: `tmux source-file ~/.tmux.conf`. Then `<prefix> f` opens lazypilot in a floating popup.

## Configuration

`~/.config/lazypilot/config.yaml`:

```yaml
# Directories to scan for projects, repos, and worktrees.
# Defaults to ~/code, ~/projects, ~/dev when this file is missing.
roots:
  - ~/Documents/github
  - ~/Documents/proyectos
  - ~/.config

# Prepended to the auto-generated codename when the worktree wizard
# pre-fills the branch name. E.g. "wt/" → branch defaults to wt/lucky-otter.
branch_prefix: ""

# Process names treated as AI agents for fallback CPU/pane-content detection
# (only used when OSC-title / opencode-plugin signals are absent).
ai_processes:
  - claude
  - opencode
  - codex
  - aider
  - copilot

# Editor launched in the LEFT pane when lazypilot creates a new tmux session.
editor: nvim

# AI assistants offered by the "pick AI" picker that fires whenever you open
# a new session via lazypilot. Each entry becomes the RIGHT pane in a 60/40
# split next to the editor; an empty `cmd` means "no AI pane, just the editor."
ai_assistants:
  - { name: claude,   cmd: claude }
  - { name: opencode, cmd: opencode }
  - { name: codex,    cmd: codex }
  - { name: none,     cmd: "" }

# How often to poll tmux + AI status. The CPU/title checks are cheap, so 2s
# is comfortable. Bump up for less flicker on big session counts.
refresh_interval: 2s

# Container directory templates for the worktree wizard. {parent} / {repo}
# expand at wizard time. Edit to match your layout.
worktree_containers:
  - "{parent}/worktrees/{repo}"
  - "{parent}/worktrees"
  - "{parent}/{repo}-worktrees"
  - "{parent}"
```

## Key bindings

Press `?` in lazypilot for the full reference. The greatest hits:

| Key | What it does |
|---|---|
| `1` / `2` / `3` | switch view (Sessions / Projects / Worktrees) |
| `j` / `k`, `g` / `G`, `⌃d` / `⌃u` | navigation |
| `/` | live filter the current view |
| `enter` | open or attach the tmux session for the selected entry |
| `n` | new worktree (on a repo or worktree row) — wizard with codename + container picker + base-ref search |
| `d` | remove worktree (with confirmation) |
| `K` | kill the tmux session for the selected entry |
| `r` | reload everything |
| `q` / `esc` | quit |

## How agent status detection works

Three signals, in priority order:

1. **opencode plugin** — lazypilot installs a small JS plugin at `~/.config/opencode/plugins/lazypilot-status.js` on first run. The plugin runs inside opencode, listens to `session.status` / `session.idle` / `permission.asked` events, and writes JSON to `/tmp/lazypilot-opencode/<pid>.json`. lazypilot reads these files keyed by the pane's PID. Restart any running opencode session after the first lazypilot launch to pick up the plugin.

2. **OSC terminal title** (mirrors Orca IDE's approach) — agents like Claude Code, Codex, Gemini, Pi already set the terminal title via OSC escape sequences to announce their state (`✳` Claude idle, `⠋⠙⠹` braille spinner for working, `✦` `◇` `✋` Gemini states, etc.). tmux exposes this as `pane_title`. Detection is in `internal/detect/title.go`.

3. **CPU + capture-pane fallback** — for AI processes that don't broadcast state, lazypilot sums child-process CPU and matches the visible pane content against a regex set for common prompts (`(y/n)`, `press enter`, etc.). Less reliable; only used as a last resort.

## Status icons

| Icon | Meaning |
|---|---|
| ◐ yellow | working — agent is actively responding |
| ! red | needs input — agent is waiting on you |
| ○ green | idle — agent is alive at its prompt |
| · dim | unknown — no AI process detected in the pane |

## Project layout

```
lazypilot/
├── cmd/lazypilot/main.go            # entrypoint
├── internal/
│   ├── tui/                         # bubbletea models (one file per view)
│   ├── scan/                        # filesystem scan for projects/repos/worktrees
│   ├── tmuxctl/                     # tmux CLI wrappers (list-sessions, list-panes, capture-pane)
│   ├── gitctl/                      # git worktree add/remove + branch listing
│   ├── detect/                      # AI status classification (title + CPU + pane content)
│   ├── opencodehook/                # opencode plugin: installer + status file reader
│   ├── codename/                    # adjective-animal codename generator
│   └── config/                      # ~/.config/lazypilot/config.yaml loader
└── install.sh
```

## Roadmap

A full feature roadmap lives in **[`docs/ROADMAP.md`](docs/ROADMAP.md)** — categorized by impact, with attribution to ideas borrowed from [ATM](https://github.com/damelLP/agent-tmux-manager) and [herdr](https://github.com/ogulcancelik/herdr).

Top of the queue:

1. **Daemon (`lazypilotd`) + tmux status-bar integration + desktop notifications** — so you find out an agent is waiting on you *without* opening lazypilot.
2. **CLI subcommands** (`lazypilot list / status / send / reply`) — scripting + tmux-status-right embedding.
3. **Stale-worktree cleanup** + activity log on Sessions.
4. **Context / cost tracking** from Claude Code's local `~/.claude/projects/**/*.jsonl` logs.

Open an issue or PR if you want to drive one of these.

## Acknowledgements

The OSC-title detection approach (`internal/detect/title.go`) is ported from [stablyai/orca](https://github.com/stablyai/orca)'s `src/shared/agent-detection.ts`. The opencode plugin architecture in `internal/opencodehook` mirrors Orca's `src/main/opencode/hook-service.ts`, simplified to a file-based status channel instead of an HTTP IPC server.

The fzf-based bash predecessor lives at [gdelco/tmux-config](https://github.com/gdelco/tmux-config).
