---
adr: "00017"
title: "Managed OpenSSH Configuration Include"
topic: "Developer Experience & IDE Ensembling"
theme: "THEME-DEVEXP"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - ssh
  - openssh
  - ssh-config
  - connectivity
  - zero-friction
executive_summary: "sndbx maintains a dedicated managed ssh_config file and idempotently links it into ~/.ssh/config via an Include directive for zero-configuration host SSH access."
---

# 00017. Managed OpenSSH Configuration Include

## Context
Standard command-line SSH tools, git clients, and IDE remote development extensions rely on OpenSSH configuration files (`~/.ssh/config`) to discover host aliases, usernames, identities, and port mappings. Directly rewriting user SSH config files risks corrupting personal settings or causing merge collisions.

## Decision (What)
Agent Sandbox manages host SSH configuration through an isolated, external config file linked via the standard OpenSSH `Include` directive:

1. **Managed File**: All agent host blocks (`Host sndbx-<name>`) are dynamically written to `~/.local/share/agent-sandbox/ssh_config`.
2. **Standardized Options**: Each block defines `HostName 127.0.0.1`, `Port <allocated-port>`, `User agent`, `IdentityFile ~/.local/share/agent-sandbox/id_ed25519`, `StrictHostKeyChecking accept-new`, and `UserKnownHostsFile ~/.local/share/agent-sandbox/known_hosts`.
3. **Idempotent Link**: `sndbx` checks `~/.ssh/config` and prepends or appends a single line:
   `Include ~/.local/share/agent-sandbox/ssh_config`
4. **Safe Lifecycle**: Adding or removing agents updates only the managed file, never altering the user's primary `~/.ssh/config`.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Transparent CLI access: `ssh sndbx-<name>` and `scp` work out of the box anywhere on the host.
- Zero risk of damaging personal host entries or custom SSH directives.
- Clean isolation of known hosts prevents host key mismatch warnings across container recreations.

### Negative / Trade-offs
- Tools or legacy SSH parsers that do not recursively parse OpenSSH `Include` directives cannot resolve sandbox hosts via this mechanism alone (addressed specifically in ADR 00020 for JetBrains Toolbox).
