---
adr: "00022"
title: "Local OpenAI-Compatible Inference Proxy"
topic: "Shared Fleet Services"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - inference
  - proxy
  - openai
  - llm
  - security
executive_summary: "Sandboxes access LLM inference through a standardized local OpenAI-compatible HTTP gateway, shielding agents from direct external API credentials."
---

# 00022. Local OpenAI-Compatible Inference Proxy

## Context
Autonomous agents require access to Large Language Models (both cloud APIs like Claude/OpenAI and local inference runtimes like Ollama/vLLM). Directly injecting root API secret tokens into container environments creates severe security risks: rogue code or prompt-injected agents could exfiltrate master credentials or generate runaway cloud billing.

## Decision (What)
Sandboxes access LLM inference exclusively through a standardized, local OpenAI-compatible HTTP gateway (`http://sndbx-inference:8000/v1` or `http://10.88.0.1:8000/v1`).

The inference architecture guarantees:
- **Credential Encapsulation**: Upstream API keys (Anthropic, OpenAI) reside strictly on the host. Containers receive zero external API keys in their environment variables.
- **Unified Interface**: All agent runtimes configure standard `OPENAI_BASE_URL` and dummy tokens, decoupling the agent implementation from specific providers.
- **Flexible Backend Routing**: The host proxy transparently routes requests to local GPU runtimes (vLLM, Ollama) or upstream cloud models based on configuration policies.
- **Rate-Limiting and Cost Controls**: Centralized throttling and token counting prevent runaway loops.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Master cloud API credentials never enter the container environment.
- Seamless switching between local on-premise models and frontier cloud providers.
- Centralized auditing and token expenditure monitoring across all active agents.

### Negative / Trade-offs
- The inference gateway service must be running for agents to query LLMs.
- Provider-specific non-standard features (e.g. Anthropic-specific beta headers) must be normalized by the proxy.
