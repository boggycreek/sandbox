---
adr: "00003"
title: "Podman as Required Container Engine"
topic: "Foundations & Architecture"
theme: "THEME-CORE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - container-engine
  - podman
  - rootless
  - security
executive_summary: "Rootless Podman is the mandatory container runtime dependency, eliminating root-owned daemon requirements and enforcing user-space privilege boundaries."
---

# 00003. Podman as Required Container Engine

## Context
Running untrusted AI code generators and multi-agent systems requires strong kernel isolation. Traditional container runtimes relying on root-owned daemons (such as Docker) expose the host to daemon privilege escalation risks and require elevated root privileges to manage containers, networks, and storage.

## Decision (What)
Podman (`>= 4.9.0`) running in rootless mode is the mandatory, non-negotiable container engine dependency for Agent Sandbox.

All containers, user namespaces, rootless bridges, and storage volumes execute under the unprivileged user account via user namespaces (`subuid`/`subgid`). The system strictly forbids running containers under the host `root` account, requires no background system daemon, and communicates with Podman solely via native CLI invocation and rootless user socket endpoints.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Zero host root privileges required for installation, runtime, or maintenance.
- Root inside the container maps to an unprivileged host UID, mitigating container breakout risks.
- Compatible with strict multi-tenant enterprise and high-security developer workstations.

### Negative / Trade-offs
- Host system must support user namespaces and have valid `/etc/subuid` and `/etc/subgid` allocations.
- Rootless networking and storage performance introduces minor kernel overhead compared to host-root execution.
