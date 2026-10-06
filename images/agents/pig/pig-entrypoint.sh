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
PIGLET_FILE="${PIGLET_DIR}/${ROLE}.yaml"

# Ensure workspace and pig directories exist
mkdir -p /home/agent/workspace "${PIG_CONFIG_DIR}" "${PIGLET_DIR}"
cd /home/agent/workspace

# Configure inference provider in models.json if endpoint provided
INFERENCE_URL="${OPENAI_BASE_URL:-${MODEL_URL:-}}"
MODEL_ID="${MODEL_NAME:-${OPENAI_MODEL:-/home/brian/models/qwen2.5-coder-7b-instruct-q4_k_m.gguf}}"
MODEL_KEY="${OPENAI_API_KEY:-local-key}"
MODEL_CTX="${MODEL_CTX:-65536}"

if [ -n "${INFERENCE_URL}" ]; then
  python3 -c "
import json, os

config_path = '${PIG_CONFIG_DIR}/models.json'
cfg = {}
if os.path.exists(config_path):
    try:
        with open(config_path, 'r') as f:
            cfg = json.load(f)
    except Exception:
        cfg = {}

if 'providers' not in cfg:
    cfg['providers'] = {}

cfg['providers']['local-llama'] = {
    'baseUrl': '${INFERENCE_URL}',
    'apiKey': '${MODEL_KEY}',
    'api': 'openai-completions',
    'models': [
        {
            'id': '${MODEL_ID}',
            'name': 'Local-Model',
            'reasoning': False,
            'input': ['text'],
            'contextWindow': int('${MODEL_CTX}'),
            'maxTokens': 4096
        }
    ]
}

with open(config_path, 'w') as f:
    json.dump(cfg, f, indent=2)
"
fi

# Dynamically synthesize role-specialized Piglet manifest if not present
if [ ! -f "${PIGLET_FILE}" ]; then
  python3 -c "
import yaml

role = '${ROLE}'
model_id = '${MODEL_ID}'
model_ctx = int('${MODEL_CTX}')

tools_by_role = {
    'fuzzer': ['bash', 'read', 'write', 'edit'],
    'auditor': ['read', 'grep', 'find', 'ls'],
    'verifier': ['bash', 'read', 'write'],
    'coder': ['bash', 'read', 'write', 'edit']
}

prompts_by_role = {
    'fuzzer': '''You are an autonomous fuzzing and boundary-probing specialist running inside an isolated sandbox container.
Your mission is to probe the target software for runtime panics, boundary errors, unexpected crashes, and invalid states.
Generate targeted test vectors, execute them using bash, and report anomalous exits or crash dumps.''',
    'auditor': '''You are a static code and logic auditor running inside an isolated sandbox container.
Your mission is to inspect the codebase for security vulnerabilities, concurrency race conditions, unhandled errors, and memory leaks.
Use read-only discovery tools (read, grep, find, ls) to pinpoint defects without mutating files.''',
    'verifier': '''You are a triage and defect verification specialist running inside an isolated sandbox container.
Your mission is to ingest suspected anomalies, isolate the root cause, and produce minimal, standalone reproducible test cases.
Verify that the failure reliably triggers, document the defect, and prepare reports for the team.''',
    'coder': '''You are an autonomous software engineering assistant running inside an isolated sandbox container.
Collaborate with peer agents to build, test, and refine software components.'''
}

selected_tools = tools_by_role.get(role, tools_by_role['coder'])
selected_prompt = prompts_by_role.get(role, prompts_by_role['coder'])

piglet = {
    'name': f'piglet-{role}',
    'description': f'Dynamically synthesized Piglet profile for {role} tuned to {model_ctx} context',
    'tools': selected_tools,
    'model': {
        'provider': 'local-llama',
        'name': model_id,
        'contextWindow': model_ctx,
        'thinking': 'off'
    },
    'systemPrompt': {
        'text': selected_prompt
    }
}

with open('${PIGLET_FILE}', 'w') as f:
    yaml.dump(piglet, f, sort_keys=False)
"
fi

# Run base entrypoint initialization (sshd, host keys, valkey announce) in background
/usr/local/bin/entrypoint.sh true &

# Start PiG session in tmux with synthesized Piglet
if ! tmux has-session -t "${TMUX_SESSION}" 2>/dev/null; then
  tmux new-session -d -s "${TMUX_SESSION}" -c "/home/agent/workspace" bash
  if [ -f "${PIGLET_FILE}" ]; then
    tmux send-keys -t "${TMUX_SESSION}" "pig --piglet ${PIGLET_FILE}" C-m
  else
    tmux send-keys -t "${TMUX_SESSION}" "pig" C-m
  fi
fi

echo "PiG Sandbox container initialized [$(hostname)]. Session: ${TMUX_SESSION} (Piglet: ${ROLE})"

if [ $# -gt 0 ]; then
  exec "$@"
fi

exec tail -f /dev/null
