---
adr: "00004"
title: "Non-Root Container User and Permission Bounds"
topic: "Container Runtime & Storage"
theme: "THEME-RUNTIME"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - security
  - container-user
  - permissions
  - uid-mapping
executive_summary: "Sandboxes execute strictly as unprivileged user agent (UID/GID 1000) under restricted Linux capabilities and rootless subuid mappings."
---

# 00004. Non-Root Container User and Permission Bounds

## Context
Autonomous agents execute arbitrary code, download third-party libraries, and interact with shell processes. Running container workloads as root—even in rootless containers—encourages sloppy security practices and risks accidental modification of container system libraries.

## Decision (What)
All agent processes inside the container execute as a dedicated, unprivileged user named `agent` with fixed `UID 1000` and `GID 1000`.

The container runtime drops unnecessary Linux capabilities (e.g., `CAP_SYS_ADMIN`, `CAP_NET_ADMIN`, `CAP_SYS_RAWIO`) while retaining standard developer rights (`CAP_CHOWN`, `CAP_SETUID`, `CAP_SETGID`, `CAP_KILL`). Sudo privileges inside the container are strictly passwordless but restricted to bounded administrative tasks when explicitly enabled, ensuring agent memory and workspace data remain owned by `UID 1000`.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Predictable file ownership matching standard single-user host Linux workstation mappings.
- Confines agent-generated files to user-space boundaries.
- Minimizes blast radius of compromised language runtimes or malicious dependencies.

### Negative / Trade-offs
- System package installations at runtime via `apt`/`dnf` require explicit sudo escalation.
- Requires careful UID/GID translation when mounting host directory bind-mounts.
