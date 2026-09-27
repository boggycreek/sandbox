---
adr: "00018"
title: "Root-Owned IDE Settings Protection"
topic: "Developer Experience & IDE Ensembling"
theme: "THEME-DEVEXP"
status: "proposed"
version: "v0.1.0-alpha"
as_built: false
tags:
  - ide
  - security
  - settings-protection
  - trust-boundary
  - extensions
executive_summary: "In-container IDE configuration directories (.vscode, .cursor) are planned to be root-owned and read-only to agent UID 1000, preventing unauthorized extensions or policy tampering."
---

# 00018. Root-Owned IDE Settings Protection

## Context
When host IDEs attach remotely to in-container workspaces, autonomous agents could theoretically modify workspace configuration folders (e.g. `.vscode/settings.json`, `.cursor/extensions`) to disable security telemetry, install unauthorized plugins, or bypass developer constraints.

## Decision (What)
In-container IDE workspace configuration directories are planned to be protected across a security boundary:

1. **Root Ownership**: Critical workspace configuration paths (`/home/agent/workspace/.vscode`, `/home/agent/.vscode-server/extensions`) will be owned by `root:root` within the container.
2. **Read-Only Permissions**: Agent processes running as `UID 1000` (`agent`) have read-only access (`0555` / `0444`) to settings files, precluding runtime modification.
3. **Pre-Authorized Extensions**: Approved IDE extensions (language servers, debuggers) are pre-installed into system directories during container image build time.

## Status
Proposed (Design accepted; root-owned scaffolding in `images/base/Dockerfile` planned).

## Consequences
### Positive
- Enforces organizational security baselines across IDE sessions inside untrusted agent environments.
- Prevents autonomous agents from disabling safety linters, prompt monitors, or audit hooks.
- Ensures consistent IDE tooling and configuration across all developers ensembling into the fleet.

### Negative / Trade-offs
- Developers who genuinely require custom IDE extensions inside the container must pre-install them in custom preset images.
