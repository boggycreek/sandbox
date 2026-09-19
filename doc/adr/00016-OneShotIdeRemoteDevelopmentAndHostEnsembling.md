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
IDE connectivity is unified under a single one-shot CLI verb: `sndbx agent open <name> [--ide <auto|vscode|cursor|clion|goland|pycharm|webstorm|idea>]`.

The architectural contract specifies:
- **In-Container Rootless SSH Server**: Each agent container runs an unprivileged OpenSSH daemon on port 2222 bound to `agent` UID 1000, managed by `bpd`.
- **Automated Host Launch**: `sndbx agent open` validates container health, resolves the assigned host port and private key, and directly launches the host IDE client targeting `agent@sndbx-<name>`.
- **Concurrent IDE Ensembling**: Multiple IDE clients can connect concurrently to the same running sandbox without port conflicts, sharing the live `/home/agent/workspace` filesystem.
- **Retirement of `connect`**: The earlier ambiguous `connect` verb is formally retired in favor of `open` (for IDE launch) and `attach` (for interactive terminal attachment).

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Zero manual host configuration needed to open a fully featured IDE inside an agent sandbox.
- True multi-IDE ensembling enables using specialized IDEs for different tasks in the same agent.
- Clear distinction between graphical IDE launching (`open`) and terminal sessions (`attach`).

### Negative / Trade-offs
- In-container `sshd` consumes minimal container memory (~10–15MB).
- Host machine must have the respective IDE CLI commands installed and available on PATH.
