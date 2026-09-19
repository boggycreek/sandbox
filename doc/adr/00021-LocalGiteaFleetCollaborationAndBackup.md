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
executive_summary: "An internal rootless Gitea service provides local git hosting, inter-agent code review, and automated synchronization of agent dotfiles and memory."
---

# 00021. Local Gitea Fleet Collaboration and Memory Backup

## Context
Multi-agent engineering workflows require a centralized version control system to exchange branches, open pull requests, perform code reviews, and snapshot persistent agent state. Relying solely on external git hosts (GitHub, GitLab) exposes private scratchpad repositories, introduces rate-limiting risks, requires external internet egress, and complicates offline execution.

## Decision (What)
Agent Sandbox manages an internal, rootless Gitea service container (`sndbx-gitea`) accessible over the shared bridge network.

The local Gitea instance fulfills three architectural roles:
1. **Fleet Code Collaboration**: Provides local Git repositories where agents branch, submit pull requests, and review each other's code locally without internet connectivity.
2. **Automated User Provisioning**: `sndbx agent create` automatically creates an authenticated user account on Gitea for the agent, provisioning SSH access keys and personal repository namespaces.
3. **Memory and Dotfile Backup**: A background synchronization routine pushes the agent's `/home/agent/.memory` vector logs and environment dotfiles to private backup repositories on Gitea, ensuring agent learnings survive container retirement.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Fully self-contained Git hosting enabling offline multi-agent software development.
- Zero external internet traffic or token leakage for internal scratchpad code.
- Automated versioned backup of agent memories and workspace dotfiles.

### Negative / Trade-offs
- Local Gitea container consumes persistent disk space and memory on the host (~150MB).
