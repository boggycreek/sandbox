# 00027. Shared Infrastructure Diagnostic Doctor and Automated Self-Healing

## Context
Shared infrastructure services in Agent Sandbox — Valkey 8 (backplane messaging bus) and Gitea 1.22 (local Git forge for fleet collaboration) — run as rootless Podman containers attached to the `agent-sandbox-infra` bridge network with persistent named volumes (`agent-sandbox-valkey-data`, `agent-sandbox-gitea-data`).

Over time, infrastructure components can encounter various inconsistencies:
1. Podman bridge networks or named volumes may be removed or unlinked.
2. Host environment configuration files (`.env`) or passwords may be missing or desynchronized.
3. Gitea admin accounts, `fleet` organization structures, or seeded repositories (`fleet/tools.git`, `fleet/tasks.git`) may be uninitialized or corrupted.
4. Valkey admin and human operator ACL permissions may be unapplied.

Similar to `sndbx agent doctor` (ADR 00026), human operators need a single host management command to inspect all shared infrastructure, diagnose defects across all layers, automatically repair any recoverable issues, and report overall infrastructure health.

## Decision
Introduce the `sndbx infra doctor` command powered by the diagnostic and self-healing engine (`pkg/doctor`):

1. **Diagnostic Layers Checked**:
   - **Host Environment & Secrets**: Verifies `~/.local/share/agent-sandbox/.env` presence, ensures non-empty `ADMIN_BACKPLANE_PASSWORD` and `HUMAN_BACKPLANE_PASSWORD`, and validates host IDE SSH keypair and `~/.ssh/config` `Include` directive.
   - **Podman Storage & Network**: Verifies that the bridge network `agent-sandbox-infra` and named volumes (`agent-sandbox-valkey-data`, `agent-sandbox-gitea-data`) exist.
   - **Valkey Service & Backplane**: Inspects `agent-sandbox-valkey` container state, tests RESP protocol connectivity on port 6379, validates admin authentication, and verifies human operator ACL permissions and the `agents:registry` hash.
   - **Gitea Service & Fleet Repositories**: Inspects `agent-sandbox-gitea` container state, tests REST API connectivity on port 3000, ensures the `giteaadmin` user is provisioned, and verifies the `fleet` organization and pre-seeded repositories (`fleet/tools`, `fleet/tasks`).

2. **Automated Healing**:
   - Automatically repairs missing networks, volumes, missing `.env` credentials, unprovisioned Gitea admin/org/repo configurations, and missing Valkey ACLs.

3. **Operator-Only Security Perimeter**:
   - `sndbx infra doctor` is strictly an unprivileged host management command for the human operator. Autonomous agents running inside containers cannot invoke host infrastructure diagnostics.

## Status
Accepted.

## Consequences
- Operators can verify and restore shared infrastructure health with a single command (`sndbx infra doctor`).
- Any configuration drift in Valkey ACLs, Gitea organizations, or network links is automatically healed without requiring manual container restarts or complex CLI scripts.
- Consistent diagnostic reporting syntax across `sndbx agent doctor` and `sndbx infra doctor`.
