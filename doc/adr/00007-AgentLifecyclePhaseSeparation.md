---
adr: "00007"
title: "Agent Lifecycle Phase Separation"
topic: "Agent Lifecycle & Process Model"
theme: "THEME-LIFECYCLE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - lifecycle
  - create
  - start
  - stop
  - state-machine
executive_summary: "Decouples agent specification and credential generation (create) from container execution (start), halting (stop), and runtime status reporting."
---

# 00007. Agent Lifecycle Phase Separation

## Context
Combining agent configuration generation, credential issuance, volume creation, and container execution into a single monolithic command hinders orchestration. Multi-agent deployments need to generate configurations and network allocations ahead of time without immediately launching resource-heavy containers.

## Decision (What)
The lifecycle of an agent is strictly divided into distinct, stateful operational phases:

- **`create` (`sndbx agent create <name> [--preset ...]`)**: Idempotently initializes the agent's persistent home volume, generates Ed25519 authentication keys, allocates dedicated SSH host ports, provisions Valkey credentials, creates local Gitea accounts, and writes the JSON configuration descriptor (`~/.local/share/agent-sandbox/agents/<name>.json`). Does *not* instantiate the container.
- **`start` (`sndbx agent start <name>`)**: Validates preflight prerequisites (network namespaces, Valkey bus, images) and starts the container instance (`sndbx-<name>`) bound to its persistent volume and network.
- **`stop` (`sndbx agent stop <name>`)**: Sends graceful termination signals (`SIGTERM` followed by `SIGKILL`) to the container, cleanly stopping processes while leaving volume and network state intact.
- **`status` / `list` (`sndbx agent status <name>`, `sndbx agent list`)**: Reports deterministic lifecycle state (`created`, `running`, `stopped`, `degraded`).

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Clear state transitions simplify automation scripts, external schedulers, and supervisor daemons.
- Eliminates race conditions during credential distribution and fleet provisioning.
- Stopped containers consume zero CPU/memory while preserving their identity and ports.

### Negative / Trade-offs
- Launching an agent from scratch requires two commands (`create` then `start`) unless automated by scripting.
