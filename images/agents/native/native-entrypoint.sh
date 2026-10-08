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

# Ensure workspace exists and set as current working directory
mkdir -p /home/agent/workspace
cd /home/agent/workspace

# Run base entrypoint initialization (sshd on port 2222, host keys, git config)
/usr/local/bin/entrypoint.sh true

echo "Native Agent Sandbox container initialized [$(hostname)]. Starting sndbx-agent..."

exec /usr/local/bin/sndbx-agent "$@"
