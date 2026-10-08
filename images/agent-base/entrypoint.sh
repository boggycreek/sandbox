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
#   3. Starting background backplane daemon (bpd) if configured.
#   4. Announcing container online presence via bp say (if Valkey configured).
#   5. Executing arguments or idling with tail -f /dev/null.

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

# Start backplane daemon (bpd) in background if available and configured
if command -v bpd >/dev/null 2>&1 && [ -n "${BP_HOST:-}" ]; then
  bpd &
fi

# Announce online presence to Valkey backplane if bp is available and configured
if command -v bp >/dev/null 2>&1 && [ -n "${BP_HOST:-}" ]; then
  bp say "online (container started)" 2>/dev/null || true
  bp status set "ready" 2>/dev/null || true
fi

# Configure default git author identity if not configured
if [ ! -f "/home/agent/.gitconfig" ]; then
  git config --global user.name "${AGENT_NAME:-agent}"
  git config --global user.email "${AGENT_NAME:-agent}@local.sndbx"
  git config --global init.defaultBranch main
fi

# Bootstrap fleet tasks repository at ~/tasks if fleet-tasks is available
if command -v fleet-tasks >/dev/null 2>&1; then
  (
    sleep 1
    fleet-tasks init >/dev/null 2>&1 || true
  ) &
fi

# Set default prompt
export PS1='[\u@\h:\w]\$ '

# If command arguments passed, execute them
if [ $# -gt 0 ]; then
  exec "$@"
fi

echo "Agent Sandbox container ready [$(hostname)]."
echo "Documentation available at: ~/doc/INDEX.md"
echo "Fleet task backlog: ~/tasks (or run 'fleet-tasks ready')"
# Keep container foreground process alive
exec tail -f /dev/null
