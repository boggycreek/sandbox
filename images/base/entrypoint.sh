#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox Base OCI Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Handles:
#   1. Initializing unprivileged SSH host keys and authorized keys.
#   2. Starting background unprivileged sshd on port 2222.
#   3. Announcing container online presence via bp say (if Valkey configured).
#   4. Spawning or attaching to the main tmux session.

set -euo pipefail

SSH_DIR="/home/agent/.ssh"
mkdir -p "${SSH_DIR}"
chmod 700 "${SSH_DIR}"

# Generate ephemeral host keys if missing
if [ ! -f "${SSH_DIR}/ssh_host_ed25519_key" ]; then
  ssh-keygen -t ed25519 -N "" -f "${SSH_DIR}/ssh_host_ed25519_key" >/dev/null 2>&1 || true
fi
if [ ! -f "${SSH_DIR}/ssh_host_rsa_key" ]; then
  ssh-keygen -t rsa -b 2048 -N "" -f "${SSH_DIR}/ssh_host_rsa_key" >/dev/null 2>&1 || true
fi

# Import host IDE public key if mounted
if [ -f "/tmp/host-keys/agent-sandbox.pub" ]; then
  cat "/tmp/host-keys/agent-sandbox.pub" >> "${SSH_DIR}/authorized_keys"
  sort -u "${SSH_DIR}/authorized_keys" -o "${SSH_DIR}/authorized_keys"
  chmod 600 "${SSH_DIR}/authorized_keys"
fi

# Start unprivileged sshd daemon on port 2222
if [ -x "/usr/sbin/sshd" ] || command -v sshd >/dev/null 2>&1; then
  /usr/sbin/sshd -f /etc/ssh/sshd_config -E "${SSH_DIR}/sshd.log" 2>/dev/null || true
fi

# Ensure in-container platform documentation is populated in persistent home
DOC_DIR="/home/agent/doc"
mkdir -p "${DOC_DIR}"
if [ -d "/usr/local/share/doc/agent-sandbox" ]; then
  cp -ru /usr/local/share/doc/agent-sandbox/* "${DOC_DIR}/" 2>/dev/null || cp -r /usr/local/share/doc/agent-sandbox/* "${DOC_DIR}/" 2>/dev/null || true
fi

# Announce online presence to Valkey backplane if bp is available and configured
if command -v bp >/dev/null 2>&1 && [ -n "${BP_HOST:-}" ]; then
  bp say "online (container started)" 2>/dev/null || true
  bp status set "ready" 2>/dev/null || true
fi

# Set default prompt
export PS1='[\u@\h:\w]\$ '

# If command arguments passed, execute them
if [ $# -gt 0 ]; then
  exec "$@"
fi

# Otherwise start background tmux session and sleep or loop
TMUX_SESSION="${AGENT_NAME:-sandbox}"
if ! tmux has-session -t "${TMUX_SESSION}" 2>/dev/null; then
  tmux new-session -d -s "${TMUX_SESSION}" -c "/home/agent/workspace" bash
fi

echo "Agent Sandbox container ready [$(hostname)]. Session: ${TMUX_SESSION}"
# Keep container foreground process alive
exec tail -f /dev/null
