#!/usr/bin/env bash
# Build and install lazypilot to ~/.local/bin, plus seed the default config
# at ~/.config/lazypilot/config.yaml if it doesn't already exist.
#
# Safe to re-run — overwrites the binary, leaves your config alone.

set -euo pipefail

cd "$(dirname "$0")"

c_dim=$'\033[2m'
c_green=$'\033[32m'
c_red=$'\033[31m'
c_reset=$'\033[0m'

step() { printf '%s» %s%s\n' "$c_dim" "$1" "$c_reset"; }
ok()   { printf '  %s✓ %s%s\n' "$c_green" "$1" "$c_reset"; }
warn() { printf '  %s! %s%s\n' "$c_red" "$1" "$c_reset"; }

# Sanity check the Go toolchain.
if ! command -v go >/dev/null 2>&1; then
    warn "Go is not on \$PATH — install Go 1.22+ first"
    warn "  on Fedora:  sudo dnf install golang"
    warn "  on macOS:   brew install go"
    exit 1
fi

GOVERSION=$(go env GOVERSION 2>/dev/null | sed 's/^go//')
ok "Go ${GOVERSION}"

step "Building lazypilot…"
BIN_DIR="${HOME}/.local/bin"
mkdir -p "${BIN_DIR}"
go build -o "${BIN_DIR}/lazypilot" ./cmd/lazypilot
ok "binary → ${BIN_DIR}/lazypilot"

# Seed the config if it's missing.
CONFIG_DIR="${HOME}/.config/lazypilot"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
mkdir -p "${CONFIG_DIR}"
if [ ! -f "${CONFIG_FILE}" ]; then
    step "Writing default config…"
    cat > "${CONFIG_FILE}" <<'EOF'
# lazypilot — auto-generated config. Edit to taste.

roots:
  - ~/code
  - ~/projects
  - ~/dev

branch_prefix: ""

ai_processes:
  - claude
  - opencode
  - codex
  - aider
  - copilot

# Editor launched in the LEFT pane when lazypilot creates a new tmux session.
editor: nvim

# AI assistants offered by the "pick AI" picker that fires whenever lazypilot
# creates a new session. The selected AI runs in the RIGHT pane (60/40 split).
# An empty `cmd` means "no AI pane — just the editor."
ai_assistants:
  - { name: claude,   cmd: claude }
  - { name: opencode, cmd: opencode }
  - { name: codex,    cmd: codex }
  - { name: none,     cmd: "" }

refresh_interval: 2s

worktree_containers:
  - "{parent}/worktrees/{repo}"
  - "{parent}/worktrees"
  - "{parent}/{repo}-worktrees"
  - "{parent}"
EOF
    ok "config → ${CONFIG_FILE}"
else
    ok "config (already present) → ${CONFIG_FILE}"
fi

# Check ~/.local/bin is on PATH; warn if not.
case ":${PATH}:" in
    *:"${BIN_DIR}":*) ok "${BIN_DIR} is on \$PATH" ;;
    *)
        warn "${BIN_DIR} is NOT on \$PATH"
        warn "  add to your shell rc, e.g.:"
        warn "    fish:  set -gx PATH \$HOME/.local/bin \$PATH"
        warn "    bash:  export PATH=\"\$HOME/.local/bin:\$PATH\""
        ;;
esac

echo
ok "lazypilot installed."
printf '  next steps:\n'
printf '    1. edit %s — add your project roots\n' "${CONFIG_FILE}"
printf '    2. add to ~/.tmux.conf:\n'
printf '         bind f display-popup -E -w 95%% -h 90%% "%s/lazypilot"\n' "${BIN_DIR}"
printf '    3. reload tmux:  tmux source-file ~/.tmux.conf\n'
printf '    4. <prefix> f opens lazypilot\n\n'
