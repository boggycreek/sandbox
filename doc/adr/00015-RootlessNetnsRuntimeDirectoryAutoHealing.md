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
executive_summary: "Runtime preflight hooks, transparent failure interception, and test cleanup routines validate directory permissions and reconcile desynchronized rootless network namespace mounts."
---

# 00015. Rootless Netns Runtime Directory Auto-Healing

## Context
Under rootless Podman on Linux, creating and mounting network namespaces requires the presence and correct POSIX permissions of `/run/user/<UID>/netns`. When containers join isolated user bridge networks (such as `agent-sandbox-infra` or `agent-sandbox-net`), Podman mounts the unshared network namespace under `$XDG_RUNTIME_DIR/libpod/tmp/rootless-netns`.

During ephemeral test suite execution, aggressive cleanup routines (e.g. removing `$XDG_RUNTIME_DIR/libpod/tmp`), or unexpected host reboots, this internal runtime mount directory structure can become missing or desynchronized while kernel network namespace descriptors (`nsfs`) remain pinned by slirp4netns/pasta helper processes. When this occurs, subsequent container launches (`podman run`, `podman start`) and network provisioning (`podman network create`) fail catastrophically with:
```
exit status 127 (Error: failed to mount runtime directory for rootless netns: no such file or directory)
```
Manual resolution previously required developers to execute cryptic commands (`podman unshare --rootless-netns true`) or reboot their physical machines.

## Decision (What)
The runtime engine (`pkg/runtime`), diagnostic doctor (`pkg/doctor`), and test harnesses implement automated rootless network namespace validation, mount reconciliation, and transparent fault tolerance:

1. **Preflight Verification and Directory Initialization**:
   `runtime.EnsureRootlessNetNS` guarantees that `/run/user/<UID>/netns` exists and possesses `0700` permissions owned by the unprivileged user before interacting with the rootless container network stack.

2. **Namespace Mount Reconciliation**:
   `runtime.EnsureRootlessNetNS` executes `podman unshare --rootless-netns true`. This commands Podman's internal rootless network setup routine to recreate the unshared runtime mount tree under `$XDG_RUNTIME_DIR/libpod/tmp/rootless-netns` and rebind to the active kernel netns.

3. **Transparent Failure Interception and Retry**:
   Container lifecycle functions (`StartAgentContainer`, `StartEgressFilterContainer`, `EnsureNetwork`, and `StartInfraStack`) intercept exit status 127 / "failed to mount runtime directory for rootless netns" errors via `isRootlessNetnsError`. Upon detecting this condition, `sndbx` automatically calls `EnsureRootlessNetNS` to heal the namespace mount tree and retries the command once without bubbling the failure to the user.

4. **Test Environment Cleanup Integration**:
   The test environment reset script (`scripts/clean-test-env.sh`) explicitly runs netns reconciliation (`podman unshare --rootless-netns true`) following test container and network cleanup, ensuring unit and integration test runs do not leave the host in a desynchronized state.

5. **Diagnostic Doctor Auditing**:
   `sndbx agent doctor` and `sndbx infra doctor` audit `/run/user/<UID>/netns` during storage and runtime diagnostics, auto-remediating invalid permissions and broken namespace mounts during health checks.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Completely eliminates cryptic `failed to mount runtime directory for rootless netns: exit status 127` failures when starting agents or infrastructure.
- Zero manual host intervention required following test runs, tmpfs pruning, or unexpected host restarts.
- Transparent single-retry recovery ensures resilience even if runtime mount directories are cleared while containers are active.
- Fully compatible with rootless Podman security constraints on physical Linux workstations without requiring host root/sudo privileges.

### Negative / Trade-offs
- One additional sub-process execution (`podman unshare --rootless-netns true`) if an initial netns mount error occurs before retrying.
