# 00008. Valkey ACL Security Model

## Context
In a shared message backplane, a software defect or rogue prompt injection in one agent must not be able to wipe other agents' streams, alter historical logs, or impersonate other agents' broadcast channels.

## Decision
Enforce strict per-agent Valkey Access Control Lists (ACLs):
1. **Disabled Default User**: Disable Valkey's unauthenticated default superuser (`user default off`).
2. **Per-Agent User Accounts**: Each agent receives a dedicated username and password generated at start time.
3. **ACL Definition**:
   ```text
   user <name> on ><password> ~<name>:* %R~*:* &* +@all -@admin -@dangerous (+xadd ~*:inbox)
   ```
   - **Own Namespace (`~<name>:*`)**: Full read/write access.
   - **Peer Namespaces (`%R~*:*`)**: Strict read-only access.
   - **Selective Peer Delivery (`(+xadd ~*:inbox)`)**: Uses ACL selectors to permit only `XADD` operations against peer inbox streams.
4. **Client-Side Guardrails (`guard.go`)**: Client-side argument validation prevents accidental injection of stream-trimming options (`MAXLEN`/`MINID`) when writing to peer inboxes.

## Status
Accepted.

## Consequences
- Agents cannot write to or tamper with peers' broadcast streams or configuration keys.
- Write privileges against peers are confined strictly to appending to inboxes.
- Valkey ACL reload (`ACL LOAD`) can update permissions dynamically without service interruption.
