---
adr: "00037"
title: "Dynamic Piglet Synthesis from Hardware Probes"
topic: "Agent Profiles & Inference Tuning"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - pig
  - piglet
  - hardware-probe
  - gpu
  - vram
  - inference
  - tuning
executive_summary: "Automates the generation of specialized PiG 'Piglet' YAML profiles based on dynamic host GPU and inference engine probes, tuning context windows, thinking levels, tool allowlists, and system prompts to physical workstation hardware."
---

# 00037. Dynamic Piglet Synthesis from Hardware Probes

## Context
Running autonomous coding agents against local hardware acceleration (GPUs/NPUs) requires balancing memory footprints and model capabilities:
- An NVIDIA RTX 3060 with 12 GB VRAM hosting a Qwen 2.5 Coder 7B model can comfortably sustain a 64k context window with Q8_0 KV cache (~6.5 GB VRAM), leaving ~5 GB headroom.
- A smaller GPU or multi-slot concurrent inference setup may require clamping context windows to 32k or 16k to avoid out-of-memory crashes.
- Smaller models (7B-14B) struggle with ambiguous, unconstrained tool scopes or overly complex Chain-of-Thought prompts, benefiting significantly from strict tool whitelists (`bash`, `read`, `write`, `edit` vs. read-only `grep`, `find`, `ls`).

In PiG, agent configuration is formalized through **Piglets**—declarative YAML specifications that define model selection, provider mappings, context windows, thinking levels, allowed tools, and specialized system prompts.

Manually crafting static Piglet files for every workstation and agent role creates configuration drift and operational friction.

## Decision (What)
We establish dynamic Piglet manifest synthesis driven by hardware and model capability probing:

1. **Hardware & Engine Probing**:
   - Inspects host GPU compute devices (`nvidia-smi` / ROCm / Apple Silicon) for total and available VRAM.
   - Queries active inference engine endpoints (such as `http://127.0.0.1:8080/v1/models` or Ollama on port `11434`) to extract model identifier, maximum context window (`n_ctx`), parameter count, and quantization format.

2. **Dynamic Manifest Synthesis**:
   - The agent harness or provisioning entrypoint dynamically synthesizes a targeted Piglet manifest (e.g. `~/.pig/piglets/<role>.yaml`):
     - **Context Window Tuning**: Clamps `contextWindow` to the active inference server capacity (e.g. 65,536 tokens).
     - **Thinking & Sampling Optimization**: Sets reasoning effort (`thinking: off` or `thinking: minimal`) appropriate for compact coder models.
     - **Role-Based Tool Scoping**: Enforces an explicit `tools` allowlist tailored to the designated agent role (e.g., fuzzers receive file and bash mutation tools; auditors receive non-mutating search tools).
     - **Role-Tailored System Prompt**: Generates concise instructions focused on the specific operational boundary.

3. **Fallback & Override Hierarchy**:
   - Pre-existing workspace Piglets (`.pig/piglets/`) take precedence if committed by developers.
   - If no custom Piglet is provided, dynamic synthesis activates automatically using the probed inference profile.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Prevents out-of-memory inference failures and HTTP 400 context overflow errors.
- Maximizes local hardware utilization by aligning context allocations with actual GPU VRAM headroom.
- Tightens container security boundaries by pruning unneeded agent tools per role.

### Negative / Trade-offs
- Dynamic synthesis requires local inference servers to be reachable at probe time to inspect model metadata.
