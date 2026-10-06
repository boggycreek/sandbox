---
adr: "00036"
title: "PiG Derivative Agent OCI Image"
topic: "Container Runtimes & Agent Harnesses"
theme: "THEME-RUNTIME"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - pig
  - agent
  - podman
  - oci
  - runtime
  - harness
executive_summary: "Introduces PiG (Pi in Go) as a first-class derivative agent harness OCI image (agent-sandbox-pig:latest), integrating single-binary Go execution, non-root isolation, tmux session supervision, and native sndbx lifecycle preset management."
---

# 00036. PiG Derivative Agent OCI Image

## Context
The Agent Sandbox runtime supports diverse autonomous coding agents by layering agent-specific harnesses over a neutral base image (`agent-sandbox-base:latest`). Existing derivatives include OpenCode (`agent-sandbox-opencode`), Claude Code (`agent-sandbox-claude`), and Antigravity (`agent-sandbox-agy`).

PiG ([Pi in Go](https://github.com/MichaelKinsy/PiG)) is a high-performance, single-binary Go rewrite and port of the upstream Pi terminal coding agent. Unlike Node-heavy agent runtimes, PiG operates without Node.js dependencies for its core lifecycle, starts up in ~20 milliseconds, and exposes native extension points, custom provider definitions via `models.json`, and agent profiles termed "Piglets."

To allow developers to evaluate and deploy PiG in isolated multi-agent fleet topologies, the sandbox platform requires a first-class OCI derivative image and unified lifecycle integration.

## Decision (What)
We establish the `agent-sandbox-pig` derivative container harness and integrate it into the `sndbx` host management workflow:

1. **Layered Derivative OCI Image (`images/agents/pig/Dockerfile`)**:
   - Extends `agent-sandbox-base:latest`.
   - Provisions the official static binary release of `pig` (amd64/arm64) to `/usr/local/bin/pig`.
   - Installs standard Python and build tools for native extension runtime compatibility.
   - Enforces execution under the unprivileged non-root user `agent` (UID 1000).

2. **Autonomous Entrypoint Supervision (`pig-entrypoint.sh`)**:
   - Invokes base entrypoint scripts in the background for OpenSSH daemon initialization, host key publication, and Valkey presence registration.
   - Dynamically initializes `~/.pig/agent/models.json` mapping OpenAI-compatible inference endpoints (including `http://llm-gateway:<port>/v1`).
   - Launches an interactive or headless session under container `tmux` supervisor (`TMUX_SESSION`), enabling host attachment via `sndbx agent tmux <name>`.

3. **Well-Known Image Preset & Lifecycle Integration**:
   - Adds `pig` to `pkg/config/agent.go` well-known image presets (`agent-sandbox-pig:latest`).
   - Enables one-touch agent provisioning via `sndbx agent create <name> as pig`.
   - Adds `build-image-pig` to the monorepo [Makefile](../../Makefile) and `sndbx update` compilation workflow.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Zero Node.js runtime footprint for the core agent harness, minimizing container image size and memory usage.
- Sub-second startup latency suitable for rapid horizontal scaling in multi-agent fleet setups.
- Transparent host gateway inference routing through standard `MODEL_URL` / `OPENAI_BASE_URL` container injection.
- Seamless compatibility with IDE remote-SSH attachment (`sndbx agent open <name>`).

### Negative / Trade-offs
- PiG is in active 0.x community evolution; upstream feature parity and extension interfaces are actively hardening.
