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
executive_summary: "Dedicated diagnostic doctor domains (sndbx agent doctor and sndbx infra doctor) audit configuration, keys, storage, network bridges, and shared services with intrinsic automated remediation."
---

# 00023. Comprehensive Diagnostic Doctor and Self-Healing

## Context
Complex rootless container topologies, SSH key permissions, network bridges, subuid allocations, and shared infrastructure daemons can experience transient drift or corruption due to host updates, reboots, or operator errors. Diagnosing disparate failure modes manually requires deep systems expertise and slows development.

## Decision (What)
Agent Sandbox provides comprehensive diagnostic and self-healing subsystems integrated into noun-first management domains:

- **Per-Agent Diagnostics & Healing (`sndbx agent doctor <name>`)**: Audits container status, persistent volume integrity, SSH daemon responsiveness, authorized keys permissions (`0600`), and Valkey credentials, automatically repairing configuration inconsistencies and healing degraded container state.
- **Infrastructure Diagnostics & Healing (`sndbx infra doctor`)**: Audits host Podman engine versions, rootless network namespace paths (`/run/user/$UID/netns`), shared bridge networks, persistent infrastructure volumes, Valkey broker health, and Gitea service responsiveness.
- **Intrinsic Auto-Healing**: Rather than requiring manual flag toggling, diagnostic doctor routines intrinsically execute idempotent safe remediation: regenerating missing network directories, correcting permission bounds, recreating stale configuration links, and repairing provisioned services.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- One-command diagnosis and automated resolution for common host and container failures.
- Non-destructive healing preserves agent home volumes and workspace state.
- Embeddable in CI/CD preflight pipelines and container startup hooks.

### Negative / Trade-offs
- Maintaining comprehensive healing logic across diverse Linux distributions.
