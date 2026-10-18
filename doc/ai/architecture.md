---
name: fleet-architecture
description: >-
  Architectural overview of the Fleet runtime, Podman container security perimeter, layered OCI hierarchy, and LLM gateway.
  Use when designing or debugging agent containerization and runtime environments.
---

# Architecture & Container Runtime

## Container Isolation & Security Boundary
- **Engine**: Rootless Podman exclusively (ADR 00019). No Docker or docker-compose daemon dependencies.
- **Security Perimeter**: Container boundary isolates agent actions. Agents run unprivileged (UID 1000) inside containers.
- **Network**: Shared bridge network `fleet-infra`.
- **Storage**: Named persistent Podman volumes (`fleet-<name>-home`).

## Layered OCI Container Hierarchy
- **Base Images**:
  - `fleet-core` (`images/agent-core/Dockerfile`): Foundational Debian Bookworm with unprivileged `agent` user (UID 1000).
  - `fleet-daemon-base` (`images/agent-daemon-base/Dockerfile`): Autonomous daemon base with internal SSH daemon (port 2222), tmux supervisor, and `bpd` entrypoint daemon.
  - `fleet-tool-base` (`images/agent-tool-base/Dockerfile`): Interactive tool container base.
- **Derivative Agent Images** (`images/agents/`):
  - `opencode`: OpenCode agent harness.
  - `claude`: Claude Code agent harness.
  - `agy`: Antigravity CLI runner harness.
  - `pig`: PiG (Pi in Go) agent harness.
  - `native`: Native Go agent harness.

## Local LLM Gateway
- Host routing: `--add-host=llm-gateway:host-gateway`.
- Translates `localhost` / `127.0.0.1` URLs to `http://llm-gateway:<port>/v1`.

Further reading:
- [ADR 00013 — Layered OCI Container Hierarchy](../adr/00013-LayeredOciContainerHierarchy.md)
- [ADR 00019 — Podman as Required Dependency](../adr/00019-PodmanAsRequiredDependency.md)
- [ADR 00021 — Local OpenAI Inference Support](../adr/00021-LocalOpenAICompatibleInferenceSupport.md)
- [ADR 00024 — In-Container SSH Daemon](../adr/00024-InContainerSshdAndIdeEnsemblingWorkflow.md)
