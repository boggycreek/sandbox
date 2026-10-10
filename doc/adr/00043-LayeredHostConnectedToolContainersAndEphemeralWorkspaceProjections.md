---
adr: "00043"
title: "Layered Host-Connected Tool Containers and Ephemeral Workspace Projections"
topic: "Container Runtime & Storage"
theme: "THEME-RUNTIME"
status: "accepted"
version: "v0.1.0-alpha"
as_built: false
tags:
  - host-workspace
  - containers
  - oci-layering
  - runtime
  - podman
  - permissions
  - security
executive_summary: "Establishes a dual-tier OCI container architecture separating 24/7 background daemon sandboxes from interactive host-connected tool containers. Host-connected agents execute with full tool autonomy and open host networking, projecting the operator's current working directory into an unprivileged rootless container with preserved host file ownership (keep-id) and zero background daemon overhead."
---

# 00043. Layered Host-Connected Tool Containers and Ephemeral Workspace Projections

## Context

The Agent Sandbox was originally architected to host fully isolated, background fleet agents inside unprivileged containers ([ADR 00004](00004-NonRootContainerUserAndPermissionBounds.md), [ADR 00008](00008-BackplaneDaemonAsPrimaryEntrypoint.md), [ADR 00013](00013-PerInstanceNetworkIsolation.md)). These sandboxes run persistent process supervisors (`bpd`), maintain rootless SSH daemons, run continuous `tmux` sessions, and operate within an egress-filtered bridge network against dedicated home volumes (`/home/agent`).

While this model provides an optimal security perimeter for autonomous fleet agents working on internal repositories, it introduces significant friction when a human operator wants to pair interactively with an AI agent (such as Claude Code, Antigravity, or PiG) on the physical host's active projects:

1. **Host Security Vulnerability in Bare Execution**: Running autonomous coding agents directly on the host operating system grants them unconstrained access to the operator's user environment (`~/.ssh`, `~/.config`, shell histories, private credentials). A runaway script, hallucinated command, or compromised supply-chain dependency can inflict severe host-level damage.
2. **Topology Mismatch in Daemon Sandboxes**: Conversely, running interactive pair-programming agents inside standard fleet sandboxes requires pushing host code through internal Gitea repositories, navigating bridge network egress restrictions to reach local LLM servers or development web apps, and coping with persistent supervisor overhead (`bpd`, `sshd`, detached `tmux`).
3. **File Permission & Ownership Degradation**: Naively mounting host directories into standard container runtimes often creates UID/GID mismatches where files edited or compiled by the container agent become root-owned or mapped to obscure subuids on the host, breaking local git workflows and IDE access.

A clean, architectural separation is required between **autonomous fleet daemons** and **host-connected interactive pair-programming containers**.

---

## Decision (What)

We establish the **Layered Host-Connected Tool Container Architecture**:

```
                         ┌──────────────────────────────────┐
                         │   agent-sandbox-core:latest      │
                         │   (Debian bookworm, tools, Deno, │
                         │    bd, git, bp CLI, agent user)  │
                         └─────────────────┬────────────────┘
                                           │
                  ┌────────────────────────┴────────────────────────┐
                  │                                                 │
                  ▼                                                 ▼
   ┌───────────────────────────────┐               ┌─────────────────────────────────┐
   │   agent-sandbox-daemon-base   │               │    agent-sandbox-tool-base      │
   │ (Fleet sandboxes:             │               │   (Host-connected interactive:  │
   │  bpd supervisor, sshd daemon, │               │    NO bpd, NO sshd, NO tmux,    │
   │  fleet-tasks, tmux session,   │               │    direct foreground exec of    │
   │  idle event loop)             │               │    agent runner / tool binary)  │
   └──────────────┬────────────────┘               └────────────────┬────────────────┘
                  │                                                 │
        ┌─────────┴─────────┐                             ┌─────────┴─────────┐
        ▼                   ▼                             ▼                   ▼
┌───────────────┐   ┌───────────────┐             ┌───────────────┐   ┌───────────────┐
│ claude-daemon │   │  agy-daemon   │             │  claude-tool  │   │   agy-tool    │
│ (Fleet agent) │   │ (Fleet agent) │             │ (Host agent)  │   │ (Host agent)  │
└───────────────┘   └───────────────┘             └───────────────┘   └───────────────┘
```

### 1. Finer-Grained OCI Hierarchy
The container image hierarchy is partitioned into distinct base tiers:
- **`agent-sandbox-core`**: Foundational OCI layer providing Debian Bookworm, unprivileged user `agent` (UID 1000), standard language runtimes (Deno, Go tools), repository tracking (`bd`), and backplane utilities (`bp`).
- **`agent-sandbox-daemon-base`**: Specializes `agent-sandbox-core` for long-running fleet workers by adding `bpd` (PID 1 supervisor), unprivileged OpenSSH server, `fleet-tasks`, and persistent `tmux` session management.
- **`agent-sandbox-tool-base`**: Specializes `agent-sandbox-core` for interactive tool execution by stripping out daemons, SSH, and `tmux`, setting the container entrypoint to invoke the agent cognitive harness directly in the foreground attached to the operator's terminal STDIN/STDOUT.

### 2. Ephemeral Workspace Projection & User ID Preservation
Host-connected agents bind the operator's physical working directory into the container:
- The operator's current working directory (`pwd` or explicit `--pwd <path>`) is mounted to `/home/agent/workspace`.
- Containers utilize user namespace mapping (`--userns=keep-id:uid=1000,gid=1000`), guaranteeing that all files created or modified by the agent inside the container retain the operator's exact physical host UID and GID without permission friction.

### 3. Open Network Fabric
Unlike fleet sandboxes attached to the private `agent-sandbox-infra` bridge with egress filters:
- Host-connected tool containers utilize open host networking (`--net=host`).
- Agents can seamlessly communicate with local LLM inference engines (such as `llama-server` or Ollama on `localhost:<port>`), workstation services, and external developer endpoints without proxy configuration.

### 4. Sandboxed Autonomy (YOLO Mode Without Sudo)
- Host-connected containers run with autonomous approval bypass enabled (e.g., auto-approving file edits, commands, and tool invocations).
- Because the process executes strictly as an unprivileged user inside rootless Podman without `sudo` access, the physical host environment (including `~/.ssh`, host configuration, and root filesystems) remains entirely protected from rogue modifications or runaway commands.

### 5. Bare Host Escape Hatch (`--no-container`)
- When an operator or automation pipeline explicitly chooses to execute a CLI directly on the physical host machine without Podman, the platform provisions credentials, Valkey ACLs, and profile `.env` files via `--no-container` without invoking the container engine.

---

## Status

Accepted.

---

## Consequences

### Positive
- **Maximum Operator Safety**: Enables complete agent autonomy (unconstrained file edits, compilations, test runs) while eliminating the risk of host operating system or credential compromise.
- **Zero Permission Drift**: Preserves host UID and GID mapping across all workspace file operations via rootless `keep-id`.
- **Zero Overhead**: Eliminates daemon, SSH, and multiplexer overhead for interactive pairing sessions.
- **Unified Lifecycle Management**: Operators manage both fleet sandboxes and host-connected agents through the familiar `sndbx agent` CLI interface.

### Negative / Trade-offs
- **Image Footprint**: Requires maintaining parallel derivative OCI images (`-daemon` and `-tool`) built from the common `agent-sandbox-core` foundation.
- **Network Exfiltration Risk**: Because host-connected agents utilize open host networking, egress filtering is bypassed, requiring trust in the underlying model provider or local inference endpoint.
