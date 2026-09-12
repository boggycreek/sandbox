#!/usr/bin/env bash
# Agent Sandbox Claude Code Derivative Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Handles:
#   1. Initializing Claude settings (acceptEdits mode).
#   2. Invoking base entrypoint (sshd, host keys, valkey announce).
#   3. Spawning Claude Code interactive session inside tmux.

set -euo pipefail

TMUX_SESSION="${AGENT_NAME:-claude-sandbox}"

# Ensure workspace exists
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Initialize Claude settings directory and configuration
CLAUDE_CONFIG_DIR="/home/agent/.claude"
mkdir -p "${CLAUDE_CONFIG_DIR}"
if [ ! -f "${CLAUDE_CONFIG_DIR}/settings.json" ] && [ -f /etc/claude/settings.json ]; then
  cp /etc/claude/settings.json "${CLAUDE_CONFIG_DIR}/settings.json"
fi

# Run base entrypoint initialization (sshd, host keys, valkey announce) in background
/usr/local/bin/entrypoint.sh true &

# Start Claude Code in tmux
if ! tmux has-session -t "${TMUX_SESSION}" 2>/dev/null; then
  tmux new-session -d -s "${TMUX_SESSION}" -c "/home/agent/workspace" bash
  tmux send-keys -t "${TMUX_SESSION}" "claude --dangerously-skip-permissions" C-m
fi

echo "Claude Code Sandbox container initialized [$(hostname)]. Session: ${TMUX_SESSION}"

if [ $# -gt 0 ]; then
  exec "$@"
fi

exec tail -f /dev/null
