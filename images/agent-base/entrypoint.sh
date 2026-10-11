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

# Seed operational context and guidelines for autonomous agents from template
AGENT_NAME_VAL="${AGENT_NAME:-agent}"
AGENT_ROLE_VAL="${ROLE:-${AGENT_ROLE:-coding-agent}}"
HUMAN_NAME_VAL="${HUMAN_NAME:-brian}"
HOSTNAME_VAL="$(hostname 2>/dev/null || echo 'sandbox')"

TEMPLATE_FILE="/etc/agent-sandbox/templates/AGENTS.md.template"
if [ -f "${TEMPLATE_FILE}" ]; then
  sed -e "s/__AGENT_NAME__/${AGENT_NAME_VAL}/g" \
      -e "s/__AGENT_ROLE__/${AGENT_ROLE_VAL}/g" \
      -e "s/__HUMAN_NAME__/${HUMAN_NAME_VAL}/g" \
      -e "s/__HOSTNAME__/${HOSTNAME_VAL}/g" \
      "${TEMPLATE_FILE}" > /home/agent/AGENTS.md
fi

# Ensure symlinks for multi-harness compatibility in home directory
ln -sf /home/agent/AGENTS.md /home/agent/GEMINI.md
ln -sf /home/agent/AGENTS.md /home/agent/CLAUDE.md

# Seed into primary workspace if directory exists and file not present
if [ -d "/home/agent/workspace" ] && [ ! -f "/home/agent/workspace/AGENTS.md" ]; then
  cp -f /home/agent/AGENTS.md /home/agent/workspace/AGENTS.md 2>/dev/null || true
  ln -sf /home/agent/workspace/AGENTS.md /home/agent/workspace/GEMINI.md 2>/dev/null || true
  ln -sf /home/agent/workspace/AGENTS.md /home/agent/workspace/CLAUDE.md 2>/dev/null || true
fi

# Ensure headless/YOLO aliases in interactive shells
if [ -f "/home/agent/.bashrc" ]; then
  if ! grep -q "dangerously-skip-permissions" "/home/agent/.bashrc"; then
    echo 'alias agy="agy --dangerously-skip-permissions"' >> "/home/agent/.bashrc"
    echo 'alias claude="claude --dangerously-skip-permissions"' >> "/home/agent/.bashrc"
  fi
fi

# Seed default configurations for harnesses if templates are present in /etc/agent-sandbox/configs
if [ -d "/etc/agent-sandbox/configs" ]; then
  # Antigravity (agy)
  mkdir -p /home/agent/.gemini/antigravity-cli
  if [ ! -f /home/agent/.gemini/antigravity-cli/config.json ] && [ -f /etc/agent-sandbox/configs/agy/default/config.json ]; then
    cp /etc/agent-sandbox/configs/agy/default/config.json /home/agent/.gemini/antigravity-cli/config.json
  fi
  if [ ! -f /home/agent/.gemini/antigravity-cli/settings.json ] && [ -f /etc/agent-sandbox/configs/agy/default/settings.json ]; then
    cp /etc/agent-sandbox/configs/agy/default/settings.json /home/agent/.gemini/antigravity-cli/settings.json
  fi
  ln -sf /home/agent/AGENTS.md /home/agent/.gemini/antigravity-cli/GEMINI.md 2>/dev/null || true

  # Claude Code
  mkdir -p /home/agent/.claude
  if [ ! -f /home/agent/.claude/settings.json ] && [ -f /etc/agent-sandbox/configs/claude/default/settings.json ]; then
    cp /etc/agent-sandbox/configs/claude/default/settings.json /home/agent/.claude/settings.json
  fi
  ln -sf /home/agent/AGENTS.md /home/agent/.claude/CLAUDE.md 2>/dev/null || true

  # OpenCode
  mkdir -p /home/agent/.config/opencode
  if [ ! -f /home/agent/.config/opencode/config.json ] && [ -f /etc/agent-sandbox/configs/opencode/default/config.json ]; then
    cp /etc/agent-sandbox/configs/opencode/default/config.json /home/agent/.config/opencode/config.json
  fi
  ln -sf /home/agent/AGENTS.md /home/agent/.config/opencode/AGENTS.md 2>/dev/null || true
fi

# Ensure NVM and default Node LTS are seeded into persistent home if missing
if [ ! -d "/home/agent/.nvm" ]; then
  if [ -d "/usr/local/share/nvm-template" ]; then
    cp -a /usr/local/share/nvm-template /home/agent/.nvm 2>/dev/null || true
  elif [ -d "/etc/skel/.nvm" ]; then
    cp -a /etc/skel/.nvm /home/agent/.nvm 2>/dev/null || true
  fi
fi

# Start backplane daemon (bpd) in background if available and configured
if command -v bpd >/dev/null 2>&1 && [ -n "${BP_HOST:-}" ] && [ "${BPD_DISABLED:-0}" != "1" ]; then
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

# Just-in-time install target agent harness if requested and missing
if [ -n "${AGENT_HARNESS:-}" ] && [ ! -x "/home/agent/.local/bin/${AGENT_HARNESS}" ]; then
  if command -v agent-install >/dev/null 2>&1; then
    agent-install "${AGENT_HARNESS}" || echo "agent-base: JIT install of ${AGENT_HARNESS} failed, continuing" >&2
  fi
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
