---
adr: "00008"
title: "Backplane Daemon (bpd) as Primary Entrypoint"
topic: "Agent Lifecycle & Process Model"
theme: "THEME-LIFECYCLE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - process-model
  - bpd
  - pid1
  - entrypoint
  - supervisor
executive_summary: "Sandboxes run bpd as PID 1 entrypoint managing background workers; interactive tmux is an on-demand debug attachment tool rather than the container entrypoint."
---

# 00008. Backplane Daemon (bpd) as Primary Entrypoint

## Context
Early prototypes ran interactive `tmux` sessions directly as the container's PID 1 entrypoint. This created severe operational liabilities: container lifecycles were bound to interactive terminal sessions, background tasks could not survive terminal disconnection, process health monitoring was opaque, and headless automated agents had no clean supervisory harness.

## Decision (What)
Sandboxes execute `bpd` (Backplane Daemon) as their immutable PID 1 entrypoint inside the container.

The daemon acts as a lightweight sub-reaper and process supervisor responsible for:
1. Reaping orphaned zombie processes within the container PID namespace.
2. Launching and supervising in-container background services (rootless `sshd`, agent runtime daemons).
3. Maintaining persistent heartbeat and status telemetry reporting to the Valkey backplane bus.
4. Handling graceful shutdown propagation upon receiving container stop signals (`SIGTERM`).

Interactive terminal access is decoupled from PID 1; operators attach to running sandboxes on demand via `sndbx agent attach <name>` (or SSH terminal sessions) without altering the container's lifecycle.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Container remains running reliably regardless of operator terminal disconnections.
- Clean process reaping prevents container resource exhaustion from runaway sub-processes.
- Headless, autonomous agent workflows execute stably in the background.

### Negative / Trade-offs
- Direct debugging of PID 1 requires inspecting `bpd` log outputs rather than attaching directly to init.
