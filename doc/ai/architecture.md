---
name: agent-sandbox-architecture
description: >-
  Architectural overview of the Agent Sandbox runtime, Podman container security perimeter, layered OCI hierarchy, and LLM gateway.
  Use when designing or debugging agent containerization and runtime environments.
---

# Architecture & Container Runtime

## Container Isolation & Security Boundary
- **Engine**: Rootless Podman exclusively (ADR 00019). No Docker or docker-compose daemon dependencies.
- **Security Perimeter**: Container boundary isolates agent actions. Agents run unprivileged (UID 1000) inside containers.
- **Network**: Shared bridge network `agent-sandbox-infra`.
- **Storage**: Named persistent Podman volumes (`agent-sandbox-<name>-home`).

## Layered OCI Container Hierarchy
- **Base Image** (`images/base/Dockerfile`): Neutral Debian Bookworm with unprivileged `agent` user (UID 1000), internal SSH daemon on port 2222, tmux session supervisor, and native `bp` binary.
- **Derivative Agent Images** (`images/agents/`):
  - `opencode`: OpenCode agent harness.
  - `claude`: Claude Code agent harness.
  - `agy`: Antigravity CLI runner harness.

## Local LLM Gateway
- Host routing: `--add-host=llm-gateway:host-gateway`.
- Translates `localhost` / `127.0.0.1` URLs to `http://llm-gateway:<port>/v1`.

Further reading:
- [ADR 00013 — Layered OCI Container Hierarchy](../adr/00013-LayeredOciContainerHierarchy.md)
- [ADR 00019 — Podman as Required Dependency](../adr/00019-PodmanAsRequiredDependency.md)
- [ADR 00021 — Local OpenAI Inference Support](../adr/00021-LocalOpenAICompatibleInferenceSupport.md)
- [ADR 00024 — In-Container SSH Daemon](../adr/00024-InContainerSshdAndIdeEnsemblingWorkflow.md)
