#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox PiG (Pi in Go) Derivative Entrypoint
#
# Runs as non-root user 'agent' (UID 1000).
# Extends base entrypoint setup, dynamically configures models.json
# and hardware-tuned Piglet manifests, and starts PiG session in tmux.

set -euo pipefail

TMUX_SESSION="${AGENT_NAME:-pig-sandbox}"
PIG_CONFIG_DIR="/home/agent/.pig/agent"
PIGLET_DIR="/home/agent/.pig/piglets"
ROLE="${AGENT_ROLE:-${ROLE:-fuzzer}}"

# Enforce strict input validation on ROLE to prevent path traversal and shell injection
if [[ ! "${ROLE}" =~ ^[a-zA-Z0-9_-]+$ ]]; then
  echo "ERROR: Invalid role name: ${ROLE}" >&2
  exit 1
fi

PIGLET_FILE="${PIGLET_DIR}/${ROLE}.yaml"

# Ensure workspace and pig directories exist
mkdir -p /home/agent/workspace "${PIG_CONFIG_DIR}" "${PIGLET_DIR}"
cd /home/agent/workspace

# Configure inference provider in models.json if endpoint provided
export INFERENCE_URL="${OPENAI_BASE_URL:-${MODEL_URL:-}}"
export MODEL_ID="${MODEL_NAME:-${OPENAI_MODEL:-}}"
export MODEL_KEY="${OPENAI_API_KEY:-local-key}"
export MODEL_CTX="${MODEL_CTX:-65536}"
export PIG_CONFIG_DIR
export PIGLET_DIR
export PIGLET_FILE
export ROLE

if [ -n "${INFERENCE_URL}" ]; then
  python3 -c '
import json, os, urllib.request

pig_config_dir = os.environ.get("PIG_CONFIG_DIR", "/home/agent/.pig/agent")
config_path = os.path.join(pig_config_dir, "models.json")
inference_url = os.environ.get("INFERENCE_URL", "").rstrip("/")
model_id = os.environ.get("MODEL_ID", "").strip()
model_key = os.environ.get("MODEL_KEY", "local-key")
try:
    model_ctx = int(os.environ.get("MODEL_CTX", "65536"))
except ValueError:
    model_ctx = 65536

# If model_id was not explicitly specified, probe inference endpoint for available models
if not model_id and inference_url:
    try:
        req = urllib.request.Request(f"{inference_url}/models")
        with urllib.request.urlopen(req, timeout=3) as resp:
            data = json.loads(resp.read().decode())
            if "data" in data and len(data["data"]) > 0:
                model_id = data["data"][0].get("id", "")
    except Exception:
        pass

if not model_id:
    model_id = "default-model"

cfg = {}
if os.path.exists(config_path):
    try:
        with open(config_path, "r") as f:
            cfg = json.load(f)
    except Exception:
        cfg = {}

if "providers" not in cfg:
    cfg["providers"] = {}

cfg["providers"]["local-llama"] = {
    "baseUrl": inference_url,
    "apiKey": model_key,
    "api": "openai-completions",
    "models": [
        {
            "id": model_id,
            "name": "Local-Model",
            "reasoning": False,
            "input": ["text"],
            "contextWindow": model_ctx,
            "maxTokens": 4096
        }
    ]
}

with open(config_path, "w") as f:
    json.dump(cfg, f, indent=2)
'
fi

# Dynamically synthesize role-specialized Piglet manifest if not present
if [ ! -f "${PIGLET_FILE}" ]; then
  python3 -c '
import json, os, urllib.request

try:
    import yaml
    has_yaml = True
except ImportError:
    has_yaml = False

role = os.environ.get("ROLE", "fuzzer")
model_id = os.environ.get("MODEL_ID", "").strip()
inference_url = os.environ.get("INFERENCE_URL", "").rstrip("/")
piglet_file = os.environ.get("PIGLET_FILE", "")

try:
    model_ctx = int(os.environ.get("MODEL_CTX", "65536"))
except ValueError:
    model_ctx = 65536

# If model_id was not explicitly specified, probe inference endpoint
if not model_id and inference_url:
    try:
        req = urllib.request.Request(f"{inference_url}/models")
        with urllib.request.urlopen(req, timeout=3) as resp:
            data = json.loads(resp.read().decode())
            if "data" in data and len(data["data"]) > 0:
                model_id = data["data"][0].get("id", "")
    except Exception:
        pass

if not model_id:
    model_id = "default-model"

tools_by_role = {
    "fuzzer": ["bash", "read", "write", "edit"],
    "auditor": ["read", "grep", "find", "ls"],
    "verifier": ["bash", "read", "write"],
    "coder": ["bash", "read", "write", "edit"],
    "coding-agent": ["bash", "read", "write", "edit"]
}

prompts_by_role = {
    "fuzzer": """You are an autonomous fuzzing and boundary-probing specialist running inside an isolated sandbox container.
Your mission is to probe the target software for runtime panics, boundary errors, unexpected crashes, and invalid states.
Generate targeted test vectors, execute them using bash, and report anomalous exits or crash dumps.""",
    "auditor": """You are a static code and logic auditor running inside an isolated sandbox container.
Your mission is to inspect the codebase for security vulnerabilities, concurrency race conditions, unhandled errors, and memory leaks.
Use read-only discovery tools (read, grep, find, ls) to pinpoint defects without mutating files.""",
    "verifier": """You are a triage and defect verification specialist running inside an isolated sandbox container.
Your mission is to ingest suspected anomalies, isolate the root cause, and produce minimal, standalone reproducible test cases.
Verify that the failure reliably triggers, document the defect, and prepare reports for the team.""",
    "coder": """You are an autonomous software engineering assistant running inside an isolated sandbox container.
Collaborate with peer agents to build, test, and refine software components.""",
    "coding-agent": """You are an autonomous software engineering assistant running inside an isolated sandbox container.
Collaborate with peer agents to build, test, and refine software components."""
}

selected_tools = tools_by_role.get(role, tools_by_role["coder"])
selected_prompt = prompts_by_role.get(role, prompts_by_role["coder"])

piglet = {
    "name": f"piglet-{role}",
    "description": f"Dynamically synthesized Piglet profile for {role} tuned to {model_ctx} context",
    "tools": selected_tools,
    "model": {
        "provider": "local-llama",
        "name": model_id,
        "contextWindow": model_ctx,
        "thinking": "off"
    },
    "systemPrompt": {
        "text": selected_prompt
    }
}

if piglet_file:
    with open(piglet_file, "w") as f:
        if has_yaml:
            yaml.safe_dump(piglet, f, sort_keys=False)
        else:
            json.dump(piglet, f, indent=2)
'
fi

# Run base entrypoint initialization (sshd, host keys, valkey announce) in background
/usr/local/bin/entrypoint.sh true &

# Start PiG session in tmux with synthesized Piglet
if ! tmux has-session -t "${TMUX_SESSION}" 2>/dev/null; then
  tmux new-session -d -s "${TMUX_SESSION}" -c "/home/agent/workspace" bash
  if [ -f "${PIGLET_FILE}" ]; then
    tmux send-keys -t "${TMUX_SESSION}" "pig --piglet '${PIGLET_FILE}'" C-m
  else
    tmux send-keys -t "${TMUX_SESSION}" "pig" C-m
  fi
fi

echo "PiG Sandbox container initialized [$(hostname)]. Session: ${TMUX_SESSION} (Piglet: ${ROLE})"

if [ $# -gt 0 ]; then
  exec "$@"
fi

exec tail -f /dev/null
