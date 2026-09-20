---
adr: "00024"
title: "Unified Update Pipeline (sndbx update)"
topic: "Operations, Diagnostics & Quality"
theme: "THEME-OPERATIONS"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - update
  - deployment
  - build
  - pipeline
  - atomicity
executive_summary: "The 'sndbx update' command executes an atomic three-stage local deployment: git repository synchronization, native CLI compilation to ~/.local/bin, and OCI image rebuilding."
---

# 00024. Unified Update Pipeline (sndbx update)

## Context
As Agent Sandbox evolves, updates span multiple independent layers: git source updates, host binary compilation (`sndbx`, `bp`), and container base/preset image rebuilding. Manually coordinating git pulls, make invocations, and container builds leads to partial updates, schema mismatches, and broken runtime states.

## Decision (What)
The `sndbx update` command provides an atomic, sequential three-stage pipeline to bring the entire local environment up to date:

1. **Repository Synchronization**: Synchronizes or clones the local repository (`~/.local/share/agent-sandbox/repo`) with upstream `main` via safe git fetch and fast-forward pull.
2. **Native Binary Distribution & Installation**: Downloads prebuilt native CLI binaries (`sndbx`, `bp`, `bpd`, `sonar-mcp`) from GitHub Releases directly to `~/.local/bin`, eliminating local Go toolchain requirements and environment variable conflicts (`GOROOT`/`GOPATH`). If prebuilt releases are unavailable (e.g. air-gapped or private environments), falls back to compiling from source with sanitized environment flags (`-ldflags="-s -w"`).
3. **OCI Image Rebuilding**: Executes `make build-images`, systematically rebuilding `sndbx-base` and all preset images (`sndbx-opencode`, `sndbx-claude`, `sndbx-agy`, `sndbx-egress`) into the local Podman store.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Single command keeps the entire host and container ecosystem consistent.
- Prebuilt binary distribution removes Go compiler requirements and prevents GOROOT/GOPATH conflicts on operator machines.
- Fast-forward git synchronization prevents accidental overwrites of uncommitted developer changes.
- Automatically ensures local container images match latest host CLI capabilities.

### Negative / Trade-offs
- Running `sndbx update` can take several minutes if container images require extensive layer rebuilding.
