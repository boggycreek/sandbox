#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox Tool Base Entrypoint (ADR 00043)
#
# Runs as non-root user 'agent' (UID 1000).
# Direct foreground execution attached to standard input/output.
# Omits background process supervision (bpd, sshd, tmux).

set -euo pipefail

# Ensure workspace directory exists
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Configure default git author identity if not configured
if [ ! -f "/home/agent/.gitconfig" ]; then
  git config --global user.name "${AGENT_NAME:-agent}"
  git config --global user.email "${AGENT_NAME:-agent}@local.sndbx"
  git config --global init.defaultBranch main
fi

# Ensure NVM and default Node LTS are seeded into persistent home if missing
if [ ! -d "/home/agent/.nvm" ]; then
  if [ -d "/usr/local/share/nvm-template" ]; then
    cp -a /usr/local/share/nvm-template /home/agent/.nvm 2>/dev/null || true
  elif [ -d "/etc/skel/.nvm" ]; then
    cp -a /etc/skel/.nvm /home/agent/.nvm 2>/dev/null || true
  fi
fi

# Announce online presence to Valkey backplane if bp is available and configured
if command -v bp >/dev/null 2>&1 && [ -n "${BP_HOST:-}" ]; then
  bp say "online (host tool container started)" 2>/dev/null || true
  bp status set "ready" 2>/dev/null || true
fi

# Just-in-time install target agent harness if requested and missing
if [ -n "${AGENT_HARNESS:-}" ] && [ ! -x "/home/agent/.local/bin/${AGENT_HARNESS}" ]; then
  if command -v agent-install >/dev/null 2>&1; then
    agent-install "${AGENT_HARNESS}" || echo "agent-tool-base: JIT install of ${AGENT_HARNESS} failed, continuing" >&2
  fi
fi

# If command arguments passed, execute them directly in the foreground
if [ $# -gt 0 ]; then
  exec "$@"
fi

# Default to interactive shell
exec /bin/bash
