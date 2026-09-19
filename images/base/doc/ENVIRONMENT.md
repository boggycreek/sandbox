# Agent Sandbox Environment & Runtime Topology

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

Welcome to the **Agent Sandbox** environment. This guide details the container topology, user permissions, and directory structure.

For specific topics, see the **[`INDEX.md`](INDEX.md)** hub or the following specialized guides:
- **[BACKPLANE.md](BACKPLANE.md)**: Cross-agent messaging (`bp`) and protocol syntax.
- **[BEADS.md](BEADS.md)**: Fleet task graph, dependency scheduling (`bd ready`), and atomic claiming.
- **[GITEA.md](GITEA.md)**: Local Git forge (`http://gitea:3000`), `fleet` organization, and task tracking via `bd`.
- **[LLM_GATEWAY.md](LLM_GATEWAY.md)**: Host inference routing via `http://llm-gateway:<port>/v1`.
- **[MEMORY.md](MEMORY.md)**: Git-backed memory and dotfiles backup.
- **[SONARQUBE.md](SONARQUBE.md)**: Local SonarQube server, MCP tools, and deterministic mechanical analysis Quality Gates.

---

## 1. User & Permissions Model

- **Non-Root User**: `agent` (UID `1000`, GID `1000`).
- **Autonomy**: You operate within a secure container perimeter and have full unprompted authority to create files, execute builds, run compilers, and invoke development tools.
- **No Host Sudo**: Privilege escalation to host root is blocked by Podman user namespaces.

---

## 2. Directory Structure

| Path | Description | Persistence |
|:---|:---|:---|
| `/home/agent/workspace/` | Primary directory for cloning and editing project codebases | Persisted in home volume |
| `/home/agent/tasks/` | Fleet task graph and backlog (`bd`) backed by local Gitea | Persisted in home volume / synced to Gitea |
| `/home/agent/doc/` | In-container platform and tooling reference documentation | Synchronized by entrypoint |
| `/home/agent/.ssh/` | SSH keys, authorized host IDE keys, and host identity | Persisted in home volume |
| `/home/agent/.local/bin/` | User-installed CLI binaries and tools | Persisted in home volume |
| `/home/agent/.config/` | Tool configuration (e.g. opencode, claude settings) | Persisted in home volume |

---

## 3. Background Services & Supervisor

- **Process Supervisor**: Background `tmux` session named after `$AGENT_NAME` (or `sandbox`).
  - Attach interactively from host: `sndbx agent connect <name>`
- **Unprivileged SSH Daemon**: Running on port `2222` inside the container.
  - Connect via host SSH: `sndbx agent ssh <name>`
