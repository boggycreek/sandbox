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
A single binary (`sndbx`), deployed to `~/.local/bin/sndbx`, serves as the sole official CLI interface for all Agent Sandbox host operations.

The CLI organizes capabilities under canonical, noun-first subcommand trees:
- `sndbx agent <create|start|stop|list|status|open|attach|clean|retire>`: Container lifecycle, monitoring, and interactive access.
- `sndbx infra <start|stop|status>`: Shared fleet service management (Valkey, Gitea).
- `sndbx doctor [--infra] [--fix]`: Environment, runtime, and infrastructure diagnostic self-healing.
- `sndbx plugin <add|remove|list>`: IDE extension management for JetBrains Gateway, JetBrains Toolbox, and VS Code.
- `sndbx update`: End-to-end repository sync, compilation, and container image rebuilding.

All commands enforce consistent POSIX exit codes, standardized JSON or tabular output flags, and actionable error messages.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Single binary entrypoint simplifies installation, onboarding, and PATH management.
- Cohesive subcommand domain model prevents tool sprawl.
- Unified exit codes and error structures simplify scripting and automated agent orchestration.

### Negative / Trade-offs
- Monolithic CLI binary bundles code paths for commands an individual invocation may not use.
