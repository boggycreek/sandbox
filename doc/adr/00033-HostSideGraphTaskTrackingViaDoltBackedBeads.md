---
adr: "00033"
title: "Host-Side Graph Task Tracking via Dolt-Backed Beads"
topic: "Task Tracking & Repository Operations"
theme: "THEME-OPERATIONS"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - beads
  - dolt
  - task-tracking
  - github
  - operations
  - dependency-graph
executive_summary: "Adopts Beads (bd) backed by a local Dolt database and synchronized with upstream GitHub (refs/dolt/data) for host-side platform task tracking (sndbx-*), maintaining strict two-plane isolation from in-container fleet task workloads."
---

# 00033. Host-Side Graph Task Tracking via Dolt-Backed Beads

## Context
Engineering the `agent-sandbox` platform requires coordinated task tracking across human developers and autonomous host coding assistants. 

Traditional markdown checklists (`TODO.md`) suffer severe flaws: concurrent branches generate intractable merge conflicts, language models consume excessive context tokens parsing flat text, and directed acyclic graph (DAG) dependencies cannot be enforced mechanically. Conversely, relying exclusively on web-based trackers (such as GitHub Issues or Jira) hinders offline development and disrupts headless CLI agent loops.

Furthermore, platform development needs strict isolation from workloads running inside sandbox containers. As outlined in ADR 00028, conflating host repository development with in-container agent tasks risks leaking host credentials, corrupting issue backlogs with ephemeral agent experiments, and degrading platform reliability.

## Decision (What)
We adopt **Beads (`bd`)** as the host-level graph issue tracker for all development on the `boggycreek/sandbox` repository:

1. **Namespace & Prefix**: Host-level platform tasks use the dedicated issue prefix `sndbx-*` (e.g., `sndbx-d46.9`), preventing collision with in-container tasks.
2. **Dolt-Backed Relational Storage**: Issue graphs, dependencies, and claim metadata are stored locally in `.beads/` using Dolt (version-controlled SQL database), eliminating text-based git merge conflicts.
3. **Upstream Git Synchronization**: Issue synchronization pushes and pulls Dolt database commits to upstream GitHub via dedicated data references (`refs/dolt/data` on `boggycreek/sandbox.git`), leaving production source branches unpolluted by issue churn.
4. **Strict Two-Plane Separation**: The host task graph is completely decoupled from the in-container fleet task tracker (which operates on local Gitea at `http://gitea:3000/fleet/tasks.git` per ADR 00028). Sandbox containers have zero access to host Dolt state, SSH keys, or GitHub remote data refs.
5. **Agent Workflow Protocol**: Host coding assistants interact with the task backlog via CLI commands (`bd ready` to query unblocked work, `bd update <id> --claim` to lock ownership, `bd close <id>` on completion, and `bd dolt push` for remote synchronization).

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Machine-readable DAG dependency tracking enables autonomous agents to select unblocked work without human intervention.
- Relational Dolt storage eliminates git merge conflicts on concurrent issue updates.
- Works offline and headlessly across physical developer workstations and automated CI runners.
- Enforces an immutable architectural boundary between platform development and container fleet workloads.

### Negative / Trade-offs
- Requires `bd` and `dolt` CLI tooling installed on contributor host machines (automated by `setup.sh`).
- Synchronizing Dolt refs requires push permissions to the upstream GitHub repository.
