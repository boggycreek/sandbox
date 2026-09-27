---
adr: "00002"
title: "Unified Host CLI (sndbx)"
topic: "Foundations & Architecture"
theme: "THEME-CORE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - cli
  - operator-tooling
  - architecture
  - ux
executive_summary: "A single multi-command binary (sndbx) acts as the sole operator entry point for fleet management, diagnostics, plugin setup, and orchestration."
---

# 00002. Unified Host CLI (sndbx)

## Context
Operators and AI agents managing sandboxes require a cohesive, discoverable operational interface. Fragmented scripts and distinct standalone executables create cognitive friction, inconsistent argument parsing, and divergent error reporting.

## Decision (What)
A single binary (`sndbx`), installed to the user's local binary path (`~/.local/bin/sndbx`), serves as the sole official CLI interface for all Agent Sandbox host operations.

The CLI organizes capabilities under canonical, noun-first domain subcommand hierarchies:
- **`agent`**: Agent lifecycle management (specification, provisioning, startup, process halting, clean resets, and permanent deprovisioning), interactive terminal attachment (`tmux`), IDE remote-development launching (`open`), SSH configuration emission, and per-agent health diagnostics with autonomous self-healing.
- **`infra`**: Shared fleet infrastructure management (starting, inspecting, health auditing, and stopping the shared Valkey, Gitea, SonarQube, and PostgreSQL service containers).
- **`plugin`**: Host IDE integration management, installing and unlinking thin-client remote development configurations for supported IDE families (JetBrains Gateway, JetBrains Toolbox, VS Code).
- **`update`**: Host environment synchronization, distributing prebuilt native binaries or compiling from source, and rebuilding container images.

All commands adhere to consistent POSIX exit codes, structured output formats (supporting programmatic JSON alongside human-readable tables), and actionable error diagnostics without silent failure modes.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Single binary entrypoint simplifies installation, onboarding, and PATH management.
- Cohesive subcommand domain model prevents tool sprawl.
- Unified exit codes and error structures simplify scripting and automated agent orchestration.

### Negative / Trade-offs
- Monolithic CLI binary bundles code paths for commands an individual invocation may not use.
