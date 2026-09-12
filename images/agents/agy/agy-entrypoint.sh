#!/usr/bin/env bash
# Agent Sandbox Antigravity (agy) Derivative Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Handles:
#   1. Initializing agy directory and configurations (~/.gemini/antigravity-cli).
#   2. Invoking base entrypoint (sshd, host keys, valkey announce).
#   3. Spawning agy interactive session in tmux.

set -euo pipefail

TMUX_SESSION="${AGENT_NAME:-agy-sandbox}"

# Ensure workspace exists
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Initialize agy configuration directory
AGY_CONFIG_DIR="/home/agent/.gemini/antigravity-cli"
mkdir -p "${AGY_CONFIG_DIR}"

if [ ! -f "${AGY_CONFIG_DIR}/config.json" ] && [ -f /etc/agy/agy-config.json ]; then
  cp /etc/agy/agy-config.json "${AGY_CONFIG_DIR}/config.json"
fi

# Run base entrypoint initialization (sshd, host keys, valkey announce) in background
/usr/local/bin/entrypoint.sh true &

# Start agy interactive session in tmux
if ! tmux has-session -t "${TMUX_SESSION}" 2>/dev/null; then
  tmux new-session -d -s "${TMUX_SESSION}" -c "/home/agent/workspace" bash
  tmux send-keys -t "${TMUX_SESSION}" "agy --mode=accept-edits" C-m
fi

echo "Antigravity (agy) Sandbox container initialized [$(hostname)]. Session: ${TMUX_SESSION}"

if [ $# -gt 0 ]; then
  exec "$@"
fi

exec tail -f /dev/null
