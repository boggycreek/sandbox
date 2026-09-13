# 00022. Agent Deprovisioning and Full Infrastructure Retirement

## Context
When an agent is created via `sndbx agent create <name>`, resources and identities are provisioned across multiple distinct layers:
1. **Host & Runtime**: Persistent container volume (`sndbx-agent-<name>-home`), runtime container (`sndbx-agent-<name>`), local metadata JSON (`agents/<name>.json`), and Ed25519 cryptographic keypairs (`secrets/<name>.key`).
2. **Backplane Messaging (Valkey)**: Dedicated Valkey ACL user (`ACL SETUSER <name>`), backplane streams (`<name>:inbox`, `<name>:out`, `<name>:seq`), and identity hash (`identity:<name>`).
3. **Local Git Forge (Gitea)**: Gitea user account (`<name>`), authorized SSH public keys, and `fleet` organization membership.

The `clean` command operates at the container runtime level:
- `clean`: Removes the container while preserving the home directory volume, allowing reconstitution via `start`.

Previously, a partial `destroy` command removed local files and volumes but orphaned Valkey ACL users, backplane streams, and Gitea user accounts. Having both `destroy` and `retire` was redundant and vague, leading to incomplete deprovisioning. A single, authoritative domain verb is needed to perform clean, atomic, system-wide deprovisioning when an agent is permanently retired from the fleet.

## Decision
Establish `sndbx agent retire <name> [--force]` as the single, authoritative deprovisioning command and remove the redundant `destroy` verb:

1. **Full Deprovisioning Sequence**:
   - **Container & Storage Teardown**: Halts running containers, removes the Podman container, and purges the persistent home volume (`DestroyAgentContainer`).
   - **Local Secrets & Config Purge**: Removes the agent descriptor file and private signing keys from `$XDG_DATA_HOME/agent-sandbox`.
   - **Valkey ACL & Queue Purge**: Executes `ACL DELUSER <name>` and deletes the agent's identity record (`identity:<name>`), inbox stream (`<name>:inbox`), outbox stream (`<name>:out`), sequence tracker (`<name>:seq`), finger profile (`<name>:finger`), and status indicator (`<name>:status`).
   - **Gitea Account Purge**: Calls the Gitea administrative API (`DELETE /api/v1/admin/users/{username}?purge=true`) to delete the user account, revoke SSH keys, and remove organization memberships.

2. **Error Resilience & Safety**:
   - Deprovisioning steps operate idempotently: if Gitea or Valkey is offline during retirement, local container/storage destruction completes without blocking, and 404 responses are treated as success.
   - Accepts `--force` for automated and non-interactive scripts.

## Status
Accepted.

## Consequences
- Single, authoritative CLI command (`sndbx agent retire <name> [--force]`) to completely decommission an agent across all layers.
- Eliminates ambiguous `destroy` verb and prevents credential sprawl, orphaned ACL rules, dead message queues, and leftover Git forge users.
- Clean distinction between runtime container cleanup (`clean`) and full fleet deprovisioning (`retire`).
