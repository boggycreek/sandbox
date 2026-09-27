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
The lifecycle of an agent is strictly decoupled into distinct, stateful operational phases:

- **Creation Phase (`agent create`)**: Idempotently initializes the agent's persistent home storage volume, generates asymmetric cryptographic keys (Ed25519), allocates dedicated local SSH ports, provisions scoped infrastructure credentials (Valkey ACLs, Gitea accounts, SonarQube tokens), and records the agent configuration descriptor. Crucially, this phase does *not* instantiate compute containers.
- **Execution Phase (`agent start`)**: Validates preflight prerequisites (rootless network namespaces, backplane connectivity, container image resolution) and launches the container instance bound to its dedicated persistent volume and network perimeter.
- **Halting Phase (`agent stop`)**: Dispatches graceful termination signals (`SIGTERM` followed by timeout-bounded `SIGKILL`) to in-container processes, stopping compute activity while leaving volume, configuration, and network allocations intact.
- **State Inspection Phase (`agent list`)**: Inspects runtime container states and ports, reporting deterministic lifecycle status (`running`, `stopped`, `degraded`).

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Clear state transitions simplify automation scripts, external schedulers, and supervisor daemons.
- Eliminates race conditions during credential distribution and fleet provisioning.
- Stopped containers consume zero CPU/memory while preserving their identity and ports.

### Negative / Trade-offs
- Launching an agent from scratch requires two commands (`create` then `start`) unless automated by scripting.
