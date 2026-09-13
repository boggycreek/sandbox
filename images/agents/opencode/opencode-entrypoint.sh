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

# Configure OpenCode settings
mkdir -p /home/agent/.config/opencode
if [ ! -f /home/agent/.config/opencode/config.json ] && [ -f /etc/opencode/opencode.json ]; then
  cp /etc/opencode/opencode.json /home/agent/.config/opencode/config.json
fi

# Configure OpenAI-compatible endpoint in OpenCode config if provided
INFERENCE_URL="${OPENAI_BASE_URL:-${MODEL_URL:-}}"
if [ -n "${INFERENCE_URL}" ]; then
  # Inject local openai provider configuration into ~/.config/opencode/config.json using Python standard library
  python3 -c '
import json, os

config_path = "/home/agent/.config/opencode/config.json"
cfg = {}
if os.path.exists(config_path):
    try:
        with open(config_path, "r") as f:
            cfg = json.load(f)
    except Exception:
        cfg = {}

inference_url = os.getenv("OPENAI_BASE_URL") or os.getenv("MODEL_URL", "")
model_name = os.getenv("MODEL_NAME") or os.getenv("OPENAI_MODEL", "")

provider = {
    "baseURL": inference_url,
    "apiKey": os.getenv("OPENAI_API_KEY", "local-openai-key")
}
if model_name:
    provider["model"] = model_name

if "provider" not in cfg:
    cfg["provider"] = {}
cfg["provider"]["openai"] = provider

with open(config_path, "w") as f:
    json.dump(cfg, f, indent=2)
'
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
