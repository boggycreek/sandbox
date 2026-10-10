#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox Native Agent Derivative Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Starts base infrastructure (unprivileged sshd on port 2222) and executes
# sndbx-agent directly as PID 1.

set -euo pipefail

SSH_DIR="/home/agent/.ssh"
mkdir -p "${SSH_DIR}"
chmod 700 "${SSH_DIR}"

# Ephemeral host keys
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

# Configure default git author identity and credentials
if [ ! -f "/home/agent/.gitconfig" ]; then
  git config --global user.name "${AGENT_NAME:-agent}"
  git config --global user.email "${AGENT_NAME:-agent}@local.sndbx"
  git config --global init.defaultBranch main
  git config --global beads.role contributor
fi

if [ ! -f "/home/agent/.git-credentials" ]; then
  PASS="${AGENT_PASSWORD:-${BP_PASSWORD:-${VALKEY_PASSWORD:-}}}"
  if [ -n "${PASS}" ]; then
    git config --global credential.helper store
    ENCODED_NAME=$(python3 -c "import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1], safe=''))" "${AGENT_NAME:-agent}" 2>/dev/null || echo "${AGENT_NAME:-agent}")
    ENCODED_PASS=$(python3 -c "import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1], safe=''))" "${PASS}" 2>/dev/null || echo "${PASS}")
    echo "http://${ENCODED_NAME}:${ENCODED_PASS}@gitea:3000" > /home/agent/.git-credentials
    chmod 600 /home/agent/.git-credentials
  fi
fi

# Export FLEET_TASKS_DIR and bootstrap fleet tasks repository if available
export FLEET_TASKS_DIR="${FLEET_TASKS_DIR:-/home/agent/tasks}"
mkdir -p "${FLEET_TASKS_DIR}"
if command -v fleet-tasks >/dev/null 2>&1; then
  (
    sleep 1
    fleet-tasks init >/dev/null 2>&1 || true
  ) &
fi

# Ensure workspace exists and set as current working directory
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Disable legacy bpd since sndbx-agent handles the backplane inbox directly
export BPD_DISABLED=1
# Auto-discover installed MCP binaries if MCP_BINARIES is not explicitly set
if [ -z "${MCP_BINARIES:-}" ]; then
    DISCOVERED=""
    for bin in /usr/local/bin/beads-mcp /usr/local/bin/bp-mcp /usr/local/bin/gitea-mcp /usr/local/bin/sonar-mcp; do
        if [ -x "$bin" ]; then
            DISCOVERED="${DISCOVERED:+$DISCOVERED,}$bin"
        fi
    done
    if [ -n "$DISCOVERED" ]; then
        export MCP_BINARIES="$DISCOVERED"
    fi
fi

echo "Native Agent Sandbox container initialized [$(hostname)]. Starting sndbx-agent..."

exec /usr/local/bin/sndbx-agent "$@"
