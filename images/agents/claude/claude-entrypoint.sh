#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox Claude Code Derivative Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Handles:
#   1. Initializing Claude settings (acceptEdits mode per ADR 00034).
#   2. Delegating to base entrypoint.

set -euo pipefail

# Ensure workspace exists
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Initialize Claude settings directory and configuration (ADR 00034)
CLAUDE_CONFIG_DIR="/home/agent/.claude"
mkdir -p "${CLAUDE_CONFIG_DIR}"
if [ ! -f "${CLAUDE_CONFIG_DIR}/settings.json" ] && [ -f /etc/claude/settings.json ]; then
  cp /etc/claude/settings.json "${CLAUDE_CONFIG_DIR}/settings.json"
fi
if [ -f "${CLAUDE_CONFIG_DIR}/settings.json" ]; then
  chmod 0644 "${CLAUDE_CONFIG_DIR}/settings.json"
fi

# Delegate directly to base entrypoint
exec /usr/local/bin/entrypoint.sh "$@"
