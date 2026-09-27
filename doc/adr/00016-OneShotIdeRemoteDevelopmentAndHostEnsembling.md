---
adr: "00016"
title: "One-Shot IDE Remote Development and Host Ensembling"
topic: "Developer Experience & IDE Ensembling"
theme: "THEME-DEVEXP"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - ide
  - remote-development
  - ensembling
  - sshd
  - open
executive_summary: "A unified command (sndbx agent open) launches host thin-client IDEs against in-container sshd, supporting simultaneous multi-IDE attachment to a single agent."
---

# 00016. One-Shot IDE Remote Development and Host Ensembling

## Context
Developers working alongside autonomous agents require direct graphical IDE access to the agent's live workspace, debugger, and terminal. Manually finding mapped SSH ports, configuring remote development settings, and launching IDE thin clients creates substantial user friction. Furthermore, developers often want to attach multiple distinct IDEs (e.g. GoLand for backend debugging and VS Code for Claude Code) to the same agent simultaneously.

## Decision (What)
IDE connectivity is unified under a single one-shot lifecycle command: `sndbx agent open`.

The architectural contract specifies:
- **In-Container Rootless SSH Server**: Each agent container runs an unprivileged OpenSSH daemon on port 2222 bound to `agent` UID 1000, supervised by `bpd`.
- **Automated Host Launch**: `sndbx agent open` validates container health, resolves the assigned host port and authentication credentials, and launches the requested or auto-detected host IDE client targeting the managed host alias (`sndbx-<name>`).
- **Concurrent IDE Ensembling**: Multiple IDE clients can connect concurrently to the same running sandbox without port conflicts, sharing the live `/home/agent/workspace` filesystem.
- **Separation of Concerns**: Graphical IDE launching (`open`) is cleanly decoupled from interactive terminal multiplexing (`tmux`) and raw shell sessions (`ssh`).

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Zero manual host configuration needed to open a fully featured IDE inside an agent sandbox.
- True multi-IDE ensembling enables using specialized IDEs (e.g., GoLand for Go services, VS Code for frontend or AI tooling) concurrently on the same workspace.
- Clear separation between graphical IDE orchestration and terminal session multiplexing.

### Negative / Trade-offs
- In-container `sshd` consumes minimal container memory (~10–15MB).
- Host machine must have the respective IDE executables installed and accessible on PATH for automated launching.
