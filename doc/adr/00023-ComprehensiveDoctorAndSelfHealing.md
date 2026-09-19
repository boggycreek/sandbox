---
adr: "00023"
title: "Comprehensive Diagnostic Doctor and Self-Healing"
topic: "Operations, Diagnostics & Quality"
theme: "THEME-OPERATIONS"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - doctor
  - diagnostics
  - self-healing
  - operations
  - reliability
executive_summary: "A unified diagnostic engine (sndbx doctor [--infra] [--fix]) audits permissions, container states, network bridges, and shared daemons with automated remediation."
---

# 00023. Comprehensive Diagnostic Doctor and Self-Healing

## Context
Complex rootless container topologies, SSH key permissions, network bridges, subuid allocations, and shared infrastructure daemons can experience transient drift or corruption due to host updates, reboots, or operator errors. Diagnosing disparate failure modes manually requires deep systems expertise and slows development.

## Decision (What)
Agent Sandbox includes a comprehensive, unified diagnostic doctor subsystem accessible via `sndbx doctor`:

- **Per-Agent Checks (`sndbx doctor [name]`)**: Audits container status, persistent volume integrity, SSH daemon responsiveness, authorized keys permissions (`0600`), and Valkey credentials.
- **Infrastructure Checks (`sndbx doctor --infra`)**: Audits host Podman versions, `/etc/subuid` and `/etc/subgid` allocations, rootless network namespace paths (`/run/user/$UID/netns`), Valkey broker health, and Gitea responsiveness.
- **Automated Self-Healing (`--fix`)**: When invoked with the `--fix` flag, the engine automatically remedies identified issues: re-creates missing network directories, resets invalid file permissions, regenerates stale configuration links, and restarts degraded shared services.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- One-command diagnosis and automated resolution for common host and container failures.
- Non-destructive healing preserves agent home volumes and workspace state.
- Embeddable in CI/CD preflight pipelines and container startup hooks.

### Negative / Trade-offs
- Maintaining comprehensive healing logic across diverse Linux distributions.
