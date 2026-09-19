---
adr: "00018"
title: "Root-Owned IDE Settings Protection"
topic: "Developer Experience & IDE Ensembling"
theme: "THEME-DEVEXP"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - ide
  - security
  - settings-protection
  - trust-boundary
  - extensions
executive_summary: "In-container IDE configuration directories (.vscode, .cursor) are root-owned and read-only to agent UID 1000, preventing unauthorized extensions or policy tampering."
---

# 00018. Root-Owned IDE Settings Protection

## Context
When host IDEs attach remotely to in-container workspaces, extension managers and IDE settings sync engines attempt to install extensions, execute workspace trust scripts, or alter recommended settings. If an autonomous agent has write permissions to workspace configuration folders (e.g. `.vscode/settings.json`, `.cursor/extensions`), it can disable security telemetry, install unauthorized plugins, or bypass developer constraints.

## Decision (What)
In-container IDE workspace configuration directories are protected across a strict security trust boundary:

1. **Root Ownership**: Critical workspace configuration paths (e.g. `/home/agent/workspace/.vscode`, `/home/agent/.vscode-server/extensions`) are owned by `root:root` within the container.
2. **Read-Only Permissions**: Agent processes running as `UID 1000` (`agent`) have read-only access (`0555` or `0444`) to enterprise settings files, precluding runtime modification.
3. **Pre-Authorized Extensions**: Approved IDE extensions (language servers, debuggers) are pre-installed into system directories during container image build time.
4. **Marketplace Integrity**: Untrusted marketplace extension installations requested at runtime by the agent are rejected by default.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Enforces organizational security baselines across IDE sessions inside untrusted agent environments.
- Prevents autonomous agents from disabling safety linters, prompt monitors, or audit hooks.
- Ensures consistent IDE tooling and configuration across all developers ensembling into the fleet.

### Negative / Trade-offs
- Developers who genuinely require custom IDE extensions inside the container must pre-install them in custom preset images.
