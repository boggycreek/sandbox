---
adr: "00029"
title: "Per-Agent SonarQube User Account and Analysis Token Provisioning Lifecycle"
topic: "Security & Isolation"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - sonarqube
  - authentication
  - security
  - tokens
  - provisioning
  - deprovisioning
  - doctor
  - lifecycle
executive_summary: "Automates the provisioning and deprovisioning of dedicated SonarQube user accounts and analysis tokens for sandbox agents. During agent creation, a scoped analysis token is generated and injected into the container environment as SONAR_TOKEN. Agent retirement revokes all active analysis tokens and deactivates the SonarQube user account."
---

# 00029. Per-Agent SonarQube User Account and Analysis Token Provisioning Lifecycle

## Context

In multi-agent sandbox fleets, individual autonomous coding agents execute static analysis, security vulnerability scanning, and Quality Gate verification against the shared local SonarQube instance ([ADR 00027](00027-LocalSonarQubeDeterministicMechanicalAnalysisAndQualityGateInfra.md)).

Without per-agent identity and token management, the platform would face several structural drawbacks:
1. **Admin Credential Exposure**: Agent containers would either need administrative credentials (violating least-privilege security bounds) or require SonarQube server to run with anonymous execution enabled (which prevents granular auditing and breaks standard enterprise SonarQube configurations).
2. **Attribution Drift**: Analyses and project creations performed by different agents would lack distinct audit trails in the SonarQube dashboard.
3. **Orphaned Access Artifacts**: Retiring an agent would leave orphaned tokens and unmanaged accounts in the static analysis server.

Analogous to our automated per-agent Gitea account ([ADR 00021](00021-LocalGiteaFleetCollaborationAndBackup.md)) and Valkey ACL user ([ADR 00010](00010-ValkeyPubSubMessagingAndAclIsolation.md)) provisioning lifecycles, SonarQube requires automated per-agent account and token lifecycle management.

---

## Decision

We establish an automated, scoped SonarQube user and analysis token lifecycle managed host-side by `sndbx`:

1. **Agent Creation Phase (`sndbx agent create`)**:
   - `sndbx` connects to the local SonarQube API (`http://127.0.0.1:9000`).
   - Creates a dedicated user account matching the agent name (`/api/users/create`).
   - Generates a dedicated user analysis token (`/api/user_tokens/generate`) named `<name>-agent-token`.
   - Stores the generated token securely in the agent configuration (`~/.local/share/agent-sandbox/agents/<name>.json`).

2. **Container Launch Phase (`sndbx agent start`)**:
   - If `SONAR_TOKEN` is present in the agent config, `sndbx` injects `-e SONAR_TOKEN=<token>` into the Podman container runtime invocation.
   - In-container tools (`sonar-mcp`, SonarScanner CLI, language plugins) use `$SONAR_TOKEN` for seamless, authenticated communication without requiring user intervention or host secrets.

3. **Self-Healing & Diagnostics (`sndbx agent doctor`)**:
   - Audits whether the SonarQube user exists and whether a valid analysis token is present.
   - If SonarQube is online and the token is missing from the agent configuration, `doctor` automatically generates a replacement token and persists it to config without breaking container workflows.

4. **Agent Retirement Phase (`sndbx agent retire`)**:
   - Explicitly revokes active analysis tokens (`/api/user_tokens/revoke`).
   - Deactivates the SonarQube user account (`/api/users/deactivate`).
   - Purges local configuration and secrets.

---

## Consequences

### Positive
- **Zero Admin Credential Leakage**: Agents only receive scoped user tokens; master administrative passwords never enter unprivileged container boundaries.
- **Auditable Telemetry**: Scans, project ownership, and Quality Gate events are unambiguously attributed to the specific agent that performed them.
- **Clean Deprovisioning**: Deprovisioning removes all credentials, tokens, and access privileges across Valkey, Gitea, and SonarQube in lockstep.

### Neutral / Trade-offs
- Requires SonarQube to be running during `sndbx agent create` or `sndbx agent doctor` for immediate token issuance; if offline during creation, `sndbx agent doctor` or subsequent creation automatically heals the token when the infrastructure stack is started.
