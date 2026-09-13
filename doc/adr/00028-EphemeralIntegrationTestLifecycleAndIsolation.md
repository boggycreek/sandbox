# 00028. Ephemeral Integration Test Lifecycles and Podman Test Isolation

## Context
Integration tests for the backplane (`bp`), management CLI (`sndbx`), and shared infrastructure services (Valkey and Gitea) must run reliably across local developer machines and CI pipelines. 

Previously, integration tests directly targeted fixed port assignments (`6379`, `3000`, `2223`) and shared production container names (`agent-sandbox-valkey`, `agent-sandbox-gitea`). This caused severe issues:
1. **Port and Container Collisions**: Running tests while host infrastructure was active caused immediate port bind conflicts, altered host state, or caused tests to mutate real operator data.
2. **Resource Leaks and Netns Corruption**: Test runs that aborted or failed before explicit teardown left dangling rootless netns references and orphan containers, leading to subsequent Podman runtime errors (e.g., `failed to mount runtime directory for rootless netns: no such file or directory`).
3. **Flaky Lifecycle Management**: Lack of unified, guaranteed test teardowns prevented reliable repeatability.

## Decision
Establish strict lifecycle rules and dedicated test harnesses in `test/harness/` for all integration tests:

1. **Strict Ephemeral Isolation**:
   - All integration tests must execute against completely isolated, ephemeral container environments.
   - Dynamic port allocation (`getFreePort`) is required for every service (Valkey, Gitea HTTP, Gitea SSH).
   - Container names, bridge networks, and storage volumes must be uniquely namespaced per test run (e.g., `test-infra-valkey-<pid>-<port>`, `test-infra-net-<pid>-<port>`).
   - File system configuration paths must be scoped to `t.TempDir()` via `XDG_DATA_HOME` and `XDG_CONFIG_HOME`.

2. **Automated Lifecycle Teardown (`t.Cleanup`)**:
   - Test harnesses (`StartEphemeralInfraHarness`, `StartValkeyHarness`) must automatically register cleanup routines via `t.Cleanup(h.Teardown)`.
   - Container teardowns must execute both `stop` and forceful removal (`podman rm -f`) along with network and volume deletion to ensure zero lingering rootless container or netns artifacts.

3. **Reusable Harness Abstractions**:
   - `test/harness/infra.go` encapsulates the entire start, health check wait, configuration bootstrap, command execution, and teardown lifecycle for infrastructure integration tests.
   - `test/harness/valkey.go` provides an isolated backplane instance for direct protocol and CLI testing.

## Status
Accepted.

## Consequences
- Integration tests can run concurrently and safely even when host infrastructure (`sndbx infra up`) is actively running.
- No dangling containers, networks, volumes, or netns mounts are left behind on test failures or cancellations.
- Deterministic, repeatable integration testing with clean start-test-teardown semantics.
