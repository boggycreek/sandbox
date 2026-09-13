# 00022. Agent Deprovisioning and Full Infrastructure Retirement

## Context
When an agent is created via `sndbx agent create <name>`, resources and identities are provisioned across multiple distinct layers:
1. **Host & Runtime**: Persistent container volume (`sndbx-agent-<name>-home`), runtime container (`sndbx-agent-<name>`), local metadata JSON (`agents/<name>.json`), and Ed25519 cryptographic keypairs (`secrets/<name>.key`).
2. **Backplane Messaging (Valkey)**: Dedicated Valkey ACL user (`ACL SETUSER <name>`), backplane streams (`<name>:inbox`, `<name>:out`, `<name>:seq`), and identity hash (`identity:<name>`).
3. **Local Git Forge (Gitea)**: Gitea user account (`<name>`), authorized SSH public keys, and `fleet` organization membership.

The existing `clean` and `destroy` commands operate primarily at the container runtime and local filesystem levels:
- `clean`: Removes the container while preserving the home directory volume.
- `destroy`: Removes the container, home volume, and local config files.

Neither command revoked Valkey ACL users, cleaned active backplane queues, or deleted the agent's Gitea user account. A dedicated domain verb is needed to perform a clean, atomic, system-wide deprovisioning when an agent is permanently retired from the fleet.

## Decision
Introduce the `sndbx agent retire <name>` verb to the host CLI:

1. **Full Deprovisioning Sequence**:
   - **Container & Storage Teardown**: Halts running containers, removes the Podman container, and purges the persistent home volume (`DestroyAgentContainer`).
   - **Local Secrets & Config Purge**: Removes the agent descriptor file and private signing keys from `$XDG_DATA_HOME/agent-sandbox`.
   - **Valkey ACL & Queue Purge**: Executes `ACL DELUSER <name>` and deletes the agent's identity record (`identity:<name>`), inbox stream (`<name>:inbox`), outbox stream (`<name>:out`), sequence tracker (`<name>:seq`), finger profile (`<name>:finger`), and status indicator (`<name>:status`).
   - **Gitea Account Purge**: Calls the Gitea administrative API (`DELETE /api/v1/admin/users/{username}?purge=true`) to delete the user account, revoke SSH keys, and remove organization memberships.

2. **Error Resilience**:
   - Deprovisioning steps operate idempotently: if Gitea or Valkey is offline during retirement, local container/storage destruction completes without blocking, and 404 responses are treated as success.

## Status
Accepted.

## Consequences
- Single, authoritative CLI command (`sndbx agent retire <name>`) to completely decommission an agent across all layers.
- Prevents credential sprawl, orphaned ACL rules, dead message queues, and leftover Git forge users.
- Clean distinction between container cleanup (`clean`), local purge (`destroy`), and full fleet retirement (`retire`).
