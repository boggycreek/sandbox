#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox OpenCode Derivative Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Extends base entrypoint setup and starts OpenCode session in tmux.

set -euo pipefail

TMUX_SESSION="${AGENT_NAME:-opencode-sandbox}"

# Ensure workspace exists
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Configure OpenCode settings if missing in user config
mkdir -p /home/agent/.config/opencode
if [ ! -f /home/agent/.config/opencode/config.json ] && [ -f /etc/opencode/opencode.json ]; then
  cp /etc/opencode/opencode.json /home/agent/.config/opencode/config.json
fi

# Run base entrypoint initialization (sshd, host keys, valkey announce) in background
/usr/local/bin/entrypoint.sh true &

# Start opencode session in tmux
if ! tmux has-session -t "${TMUX_SESSION}" 2>/dev/null; then
  tmux new-session -d -s "${TMUX_SESSION}" -c "/home/agent/workspace" bash
  tmux send-keys -t "${TMUX_SESSION}" "opencode" C-m
fi

echo "OpenCode Sandbox container initialized [$(hostname)]. Session: ${TMUX_SESSION}"

if [ $# -gt 0 ]; then
  exec "$@"
fi

exec tail -f /dev/null
