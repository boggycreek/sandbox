---
adr: "00005"
title: "Persisted Home Volume Across Container Recreation"
topic: "Container Runtime & Storage"
theme: "THEME-RUNTIME"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - storage
  - volumes
  - persistence
  - memory
executive_summary: "Agent persistent state and memory reside in a dedicated named volume mounted to /home/agent that survives container restarts, updates, and recreation."
---

# 00005. Persisted Home Volume Across Container Recreation

## Context
Container instances are inherently ephemeral and subject to replacement during image upgrades, crash recovery, or diagnostic rebuilds. However, developer agents accumulate valuable context, local git checkouts, shell history, tool configurations, and memory vectors that must persist indefinitely.

## Decision (What)
Every agent has a dedicated, named Podman volume (`sndbx-<name>-home`) mounted directly to `/home/agent`.

The root filesystem of the container is treated as disposable. All state intended to survive container destruction—including source checkouts (`/home/agent/workspace`), SSH authorized keys, shell configuration, and agent state databases—must reside in `/home/agent`. Container recreation (`sndbx agent clean` followed by `start`) rebinds the existing persistent volume without data loss.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Container images can be updated and rebuilt continuously without destroying agent work or memory.
- Backup and snapshotting can target discrete named volumes.
- Fast recovery from corrupted container system environments.

### Negative / Trade-offs
- Stale configurations or polluted user environments in `/home/agent` survive across image updates.
- Deprovisioning an agent requires an explicit volume purge step (`sndbx agent retire`).
