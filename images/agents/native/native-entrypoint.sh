#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox Native Agent Derivative Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Starts base infrastructure (unprivileged sshd on port 2222) and launches
# sndbx-agent within a managed tmux session.

set -euo pipefail

TMUX_SESSION="${AGENT_NAME:-sndbx-native}"

# Ensure workspace exists and set as current working directory
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Run base entrypoint initialization (sshd on port 2222, host keys, Valkey announce) in background
/usr/local/bin/entrypoint.sh true &

# Start sndbx-agent session in tmux
if ! tmux has-session -t "${TMUX_SESSION}" 2>/dev/null; then
  tmux new-session -d -s "${TMUX_SESSION}" -c "/home/agent/workspace" bash
  tmux send-keys -t "${TMUX_SESSION}" "exec /usr/local/bin/sndbx-agent" C-m
fi

echo "Native Agent Sandbox container initialized [$(hostname)]. Session: ${TMUX_SESSION}"

if [ $# -gt 0 ]; then
  exec "$@"
fi

exec tail -f /dev/null
