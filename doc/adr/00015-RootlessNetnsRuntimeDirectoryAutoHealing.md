---
adr: "00015"
title: "Rootless Netns Runtime Directory Auto-Healing"
topic: "Network Isolation & Perimeter Defense"
theme: "THEME-NETWORKING"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - networking
  - netns
  - auto-healing
  - runtime
  - podman
executive_summary: "Runtime preflight hooks automatically validate and repair rootless network namespace directory permissions (/run/user/$UID/netns) prior to container launch."
---

# 00015. Rootless Netns Runtime Directory Auto-Healing

## Context
Under rootless Podman on Linux, creating and mounting network namespaces requires the presence and correct POSIX permissions of `/run/user/<UID>/netns`. On certain system distributions, tmpfs cleanups, or following unexpected host reboots, this directory can be absent or possess invalid permissions, causing rootless container launches to fail catastrophically with `exit status 127 (no such file or directory)`.

## Decision (What)
The runtime engine (`pkg/runtime`) and diagnostic doctor (`pkg/doctor`) incorporate automated preflight validation and self-healing for rootless network namespace paths:

1. **Preflight Verification**: Prior to invoking any Podman network or container lifecycle command, `sndbx` verifies that `/run/user/<UID>/netns` exists and is owned by the current user with `0700` permissions.
2. **Transparent Healing**: If absent or misconfigured, `sndbx` automatically creates the directory and enforces the required mode (`os.MkdirAll(path, 0700)`) without failing the parent command or requiring manual user intervention.
3. **Doctor Diagnostic Integration**: `sndbx doctor --infra` explicitly audits this path and marks it passing or repaired.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Eliminates a frequent, confusing failure mode during rootless container and test harness execution.
- Zero manual host configuration required after reboots or ephemeral tmpfs resets.
- Enhances reliability of CI/CD runners and ephemeral integration test environments.

### Negative / Trade-offs
- Minor filesystem stat check executed during runtime preflight routines.
