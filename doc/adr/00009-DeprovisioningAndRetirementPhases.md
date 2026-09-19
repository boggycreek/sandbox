---
adr: "00009"
title: "Deprovisioning and Retirement Phases"
topic: "Agent Lifecycle & Process Model"
theme: "THEME-LIFECYCLE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - lifecycle
  - teardown
  - clean
  - retire
  - garbage-collection
executive_summary: "Enforces clear teardown boundaries: clean destroys ephemeral container instances while preserving data; retire purges volumes, keys, Valkey credentials, and Gitea accounts."
---

# 00009. Deprovisioning and Retirement Phases

## Context
In containerized agent environments, destructive cleanup operations must strictly distinguish between resetting an ephemeral execution container (e.g. to recover from a corrupted OS state) and permanently obliterating all stored memory, keys, and cloud resources associated with an agent. Ambiguous commands risk accidental data loss.

## Decision (What)
Agent Sandbox establishes two distinct, mutually exclusive teardown commands with unambiguous blast radiuses:

1. **`clean` (`sndbx agent clean <name>`)**: Destroys only the ephemeral Podman container instance (`sndbx-<name>`). The persistent named home volume (`sndbx-<name>-home`), configuration descriptor, SSH keypairs, Valkey ACLs, and Gitea accounts remain completely untouched. Running `sndbx agent start <name>` immediately launches a fresh container rebinding the existing state.
2. **`retire` (`sndbx agent retire <name> [--force]`)**: Permanently deprovisions the agent from the fleet. This purges the container instance, deletes the persistent home volume, revokes and removes Valkey user credentials and ACL rules, deactivates Gitea accounts, deletes host configuration descriptors, and cleans SSH host mappings.

The deprecated `destroy` verb is fully removed.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Eliminates catastrophic operator or automated agent errors where resetting a container wiped project data.
- Clear contract for container rebuilding versus fleet deprovisioning.
- Thorough cleanup during retirement prevents orphaned Valkey users, Gitea accounts, and stale SSH mappings.

### Negative / Trade-offs
- Permanent deletion requires deliberate execution of `retire`, which prompts for confirmation unless `--force` is supplied.
