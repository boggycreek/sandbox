---
adr: "00021"
title: "Local Gitea Fleet Collaboration and Memory Backup"
topic: "Shared Fleet Services"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - gitea
  - git
  - collaboration
  - backup
  - memory
  - self-hosted
executive_summary: "An internal rootless Gitea service provides local git hosting, inter-agent code review, task tracking backplane, and persistent remote dotfiles and memory repositories."
---

# 00021. Local Gitea Fleet Collaboration and Memory Backup

## Context
Multi-agent engineering workflows require a centralized version control system to exchange branches, open pull requests, perform code reviews, and track task state. Relying solely on external git hosts (GitHub, GitLab) introduces rate limits, requires external internet egress, and complicates offline execution.

## Decision (What)
Agent Sandbox manages an internal, rootless Gitea service container (`agent-sandbox-gitea`) accessible over the shared bridge network (`agent-sandbox-infra`):

1. **Fleet Code Collaboration & Task Coordination**: Provides local Git repositories (`fleet/tools.git`, `fleet/tasks.git`) where agents collaborate, review code, and track tasks via Beads without external internet egress.
2. **Automated User Provisioning**: `sndbx agent create` provisions an authenticated Gitea user account for the agent, automatically registering SSH public keys and adding the agent to the `fleet` organization.
3. **Durable Memory & Dotfile Repositories**: Agents can maintain private remote Git repositories (`fleet/agent-<name>-memory.git`, `fleet/agent-<name>-dotfiles.git`) on the local forge for durable, versioned dotfiles and reflection notes.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Fully self-contained Git hosting enabling offline multi-agent software development.
- Zero external internet traffic or token leakage for internal scratchpad code.
- Automated versioned backup of agent memories and workspace dotfiles.

### Negative / Trade-offs
- Local Gitea container consumes persistent disk space and memory on the host (~150MB).
