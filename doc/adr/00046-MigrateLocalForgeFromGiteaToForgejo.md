---
adr: "00046"
title: "Migrate Local Git Forge from Gitea to Forgejo"
topic: "Shared Fleet Services"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
supersedes:
  - "00021"
tags:
  - forgejo
  - git
  - collaboration
  - backup
  - memory
  - self-hosted
  - governance
executive_summary: "Migrates the Agent Sandbox local git forge infrastructure from Gitea to the community-governed Forgejo hard fork (codeberg.org/forgejo/forgejo:9-rootless). Replaces all as-built Gitea container names, volumes, packages, and tooling while preserving drop-in backward compatibility with existing SQLite databases and REST API v1 endpoints."
---

# 00046. Migrate Local Git Forge from Gitea to Forgejo

## Context

In [ADR 00021](00021-LocalGiteaFleetCollaborationAndBackup.md), Agent Sandbox adopted Gitea (`docker.io/gitea/gitea:1.22-rootless`) as the internal, air-gapped git forge for multi-agent code reviews, remote memory persistence, and in-container task backlogs (`http://gitea:3000/fleet/tasks.git`).

However, structural governance and commercial developments surrounding the upstream Gitea project (including trademark transfers to a commercial entity without community consensus) prompted the open-source community to establish **Forgejo**, a soft-fork turned independent, copyleft-backed hard fork maintained by Codeberg e.V. under strict community stewardship.

To ensure long-term alignment with open-source software sustainability, vendor independence, and community governance, Agent Sandbox must transition its internal git forge to Forgejo. Because Forgejo 9 is a direct fork corresponding to Gitea 1.22 (the exact version deployed in Agent Sandbox), executing this migration immediately guarantees a seamless, zero-divergence drop-in swap before upstream database schemas and API semantics diverge further.

## Decision (What)

Agent Sandbox completely replaces Gitea with **Forgejo** across all runtime components, infrastructure orchestration, client libraries, MCP servers, and as-built naming conventions:

1. **Rootless OCI Container Image**:
   - Container image is updated from `docker.io/gitea/gitea:1.22-rootless` to `codeberg.org/forgejo/forgejo:9-rootless`.
   - The container runs as unprivileged user `git` (UID/GID 1000) inside rootless Podman on the `agent-sandbox-infra` network.

2. **As-Built Resource Naming**:
   - Primary container name: `agent-sandbox-forgejo` (supersedes `agent-sandbox-gitea`).
   - Primary persistent volume: `agent-sandbox-forgejo-data` (supersedes `agent-sandbox-gitea-data`).
   - Container hostname & DNS alias: `--hostname forgejo --network-alias forgejo --network-alias gitea` (preserving DNS resolution for legacy scripts while establishing `forgejo` as canonical).
   - Local forge URLs: `http://127.0.0.1:3000` (host) and `http://forgejo:3000` (in-container).

3. **Data Volume Migration**:
   - Runtime startup checks automatically detect existing legacy volumes (`agent-sandbox-gitea-data`) if `agent-sandbox-forgejo-data` does not yet exist and safely migrate all SQLite databases and git repositories.
   - Forgejo 9 seamlessly mounts existing Gitea 1.22 SQLite databases without requiring schema conversion.

4. **Codebase & Tooling Renaming**:
   - Go client package: `sandbox/pkg/gitea` is migrated to `sandbox/pkg/forgejo`.
   - MCP server: `mcp/cmd/gitea-mcp` is renamed to `mcp/cmd/forgejo-mcp`, compiled as `forgejo-mcp`.
   - Environment variables: `FORGEJO_URL`, `FORGEJO_ADMIN_PASSWORD`, `FORGEJO_DISABLED`, and `FORGEJO_ADMIN_USER` become canonical, while transparently accepting legacy `GITEA_*` equivalents as fallbacks.
   - Container configuration variables are passed as `FORGEJO__*` (with `GITEA__*` retained as fallback).
   - In-container documentation and helper scripts (`images/agent-base/doc/FORGEJO.md`, `fleet-tasks`) are updated to target Forgejo.

5. **Administrative CLI Bootstrapping**:
   - Container user creation and password synchronizations execute via `/usr/local/bin/forgejo admin user create` and `/usr/local/bin/forgejo admin user change-password`.

## Status

Accepted (Supersedes ADR 00021).

## Consequences

### Positive
- **Community Governance & Sustainability**: Eliminates dependency on commercialized upstream Gitea in favor of Codeberg's non-profit, community-governed Forgejo.
- **Immediate Compatibility**: Because Forgejo 9 is aligned with Gitea 1.22, database schemas, `/api/v1` REST endpoints, and authentication tokens require zero translation.
- **Consistent As-Built Architecture**: Purges legacy Gitea naming across containers, volumes, Go packages, and MCP binaries, maintaining codebase clarity.
- **Backward Compatibility**: Dual network aliases (`forgejo`, `gitea`) and environment variable fallbacks prevent breaking existing scripts and container environments.

### Negative / Trade-offs
- OCI container image must be pulled from `codeberg.org/forgejo/forgejo:9-rootless` (or Docker Hub mirror).
