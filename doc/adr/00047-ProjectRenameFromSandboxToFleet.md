---
adr: "00047"
title: "Project Rename from Sandbox to Fleet"
topic: "Foundations & Architecture"
theme: "THEME-CORE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
supersedes:
  - "00002"
tags:
  - fleet
  - naming
  - cli
  - architecture
  - breaking-change
executive_summary: "Formally renames the project from Agent Sandbox ('sandbox', 'sndbx') to Fleet ('fleet', short alias 'ft'). As an alpha-stage project, this transition constitutes a clean break without backward compatibility shims or deprecation layers. Migrates the primary host management CLI binary to 'fleet' (with 'ft' alias), in-container binary to 'fleet-agent', shared infrastructure containers/networks to 'fleet-*', host paths to ~/.local/share/fleet and ~/.ssh/fleet, SSH host aliases to fleet-<name>, and upstream GitHub repository to boggycreek/fleet."
---

# 00047. Project Rename from Sandbox to Fleet

## Context

The project originally began under the name **Agent Sandbox** (with host CLI `sndbx` and alias `sb`). As the architecture evolved to support multi-agent orchestration, backplane coordination, local inference gateways, shared infrastructure services (Valkey, Forgejo, PostgreSQL, SonarQube), and multi-agent task tracking, the original scope outgrew the concept of a single "sandbox."

The term **Fleet** already pervades the architecture:
- Shared Forgejo organization: `fleet/`
- Shared repositories: `fleet/tools.git` and `fleet/tasks.git`
- Task tracking and coordination paradigms: agent ensembles and fleet management.

Renaming the platform and its host CLI from `sndbx` / `sandbox` to **Fleet** (`fleet`) aligns the product branding, codebase identity, and operator user experience with its actual mission.

Because Fleet is in active alpha development (`v0.1.0-alpha`), maintaining complex legacy compatibility shims or dual-state migration layers would impose unnecessary technical debt and cognitive overhead. The rename is therefore executed as a **clean break**.

## Decision (What)

The project, host CLI, container harness, and operational resources are comprehensively renamed from **`sandbox` / `sndbx`** to **`fleet`**:

1. **Host CLI Binaries & Aliases**:
   - Primary CLI binary: `fleet` (supersedes `sndbx`).
   - Short CLI alias: `ft` symlinked to `fleet` (supersedes `sb`).
   - CLI package & entrypoint directory: `sandbox/cmd/sndbx/` migrated to `sandbox/cmd/fleet/` (and compiled to `bin/fleet`).

2. **In-Container Native Agent**:
   - In-container binary: `fleet-agent` (supersedes `sndbx-agent`).
   - Entrypoint directory: `agent/cmd/sndbx-agent/` migrated to `agent/cmd/fleet-agent/` (and compiled to `bin/fleet-agent`).
   - In-container client identification: `fleet-agent`.

3. **Shared Infrastructure Resources**:
   - Bridge network: `fleet-infra` (supersedes `agent-sandbox-infra`).
   - Shared containers:
     - `fleet-valkey` (supersedes `agent-sandbox-valkey`)
     - `fleet-forgejo` (supersedes `agent-sandbox-forgejo`)
     - `fleet-postgres` (supersedes `agent-sandbox-postgres`)
     - `fleet-sonarqube` (supersedes `agent-sandbox-sonarqube`)
   - Persistent volumes: `fleet-valkey-data`, `fleet-valkey-config`, `fleet-forgejo-data`, `fleet-postgres-data`, `fleet-sonarqube-data`, `fleet-sonarqube-extensions`, `fleet-sonarqube-logs`.

4. **Agent Containers & Volumes**:
   - Agent container naming pattern: `fleet-agent-%s` (or `fleet-%s`).
   - Agent home volume: `fleet-agent-%s-home` (or `fleet-%s-home`).
   - Egress companion container: `fleet-agent-%s-egress`.
   - OCI image tags: `fleet-core:latest`, `fleet-base:latest`, `fleet-daemon-base:latest`, `fleet-tool-base:latest`, `fleet-native:latest`, `fleet-opencode:latest`, `fleet-claude:latest`, `fleet-agy:latest`, `fleet-pig:latest`, `fleet-egress:latest`, `fleet-sonarqube:latest`.
   - Preset alias: preset `"fleet"` maps to `fleet-native:latest`.

5. **Host Storage Paths & OpenSSH Configuration**:
   - XDG data directory: `~/.local/share/fleet` (supersedes `~/.local/share/agent-sandbox`).
   - XDG configuration directory: `~/.config/fleet` (supersedes `~/.config/sndbx`).
   - Host IDE SSH private key: `~/.ssh/fleet` (supersedes `~/.ssh/agent-sandbox`).
   - Managed OpenSSH configuration file: `~/.local/share/fleet/ssh_config`.
   - OpenSSH include block in `~/.ssh/config`: `Include ~/.local/share/fleet/ssh_config`.
   - Host SSH aliases: `Host fleet-<name>` (supersedes `Host sndbx-<name>`).

6. **Environment Variables**:
   - `FLEET_ROOT` (supersedes `SANDBOX_ROOT`).
   - `FLEET_REPO` (supersedes `AGENT_SANDBOX_REPO`).
   - `FLEET_REMOTE` (supersedes `AGENT_SANDBOX_REMOTE`).
   - `FLEET_BRANCH` (supersedes `AGENT_SANDBOX_BRANCH`).
   - `FLEET_UPDATE_TIMEOUT` (supersedes `SNDBX_UPDATE_TIMEOUT`).

7. **IDE Plugins & Integrations**:
   - JetBrains Gateway plugin: `fleet-gateway.jar` / plugin ID `com.boggycreek.fleet.gateway`.
   - JetBrains Toolbox extension: `fleet-toolbox.jar` / extension ID `com.boggycreek.fleet.toolbox`.

8. **Repository & Remote Hosting**:
   - GitHub upstream repository: `github.com/boggycreek/fleet` (renamed from `boggycreek/sandbox`).
   - Beads issue tracking prefix: `fleet-*` (supersedes `sndbx-*`).

## Status

Accepted (Supersedes ADR 00002).

## Consequences

### Positive
- **Cohesive Platform Identity**: Unifies product naming, CLI commands, container infrastructure, and shared services under the `fleet` brand.
- **Improved Usability**: Eliminates the cryptic abbreviation `sndbx` in favor of natural `fleet`, while providing a collision-free 2-letter alias `ft`.
- **Zero Legacy Overhead**: Clean break in alpha avoids maintaining backward-compatibility wrappers, obsolete environment variable aliases, and dual-system migration logic.
- **Accurate Alignment**: Matches existing `fleet/` organization and repository structures already established in the local Forgejo forge.

### Negative / Trade-offs
- **Breaking Change**: Existing alpha installations must be redeployed using `install.sh`. Legacy containers (`agent-sandbox-*`) and paths (`~/.local/share/agent-sandbox`, `~/.ssh/agent-sandbox`) must be cleaned up manually or via clean reinstall.
- **Remote Git URL Update**: Local contributor clones must update their git remote URLs to point to `boggycreek/fleet.git`.
