# Agent Dotfiles & Memory Persistence Reference

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

The **Agent Sandbox** provides two levels of persistence to preserve state across container recreations and migrations:

---

## 1. Local Persistent Volume (`/home/agent`)

Every agent has a named Podman volume (`sndbx-agent-<name>-home`) mounted directly at `/home/agent`.
- **Preserved across**: `sndbx agent stop`, `sndbx agent clean`, and container image upgrades.
- **Includes**: Installed packages in `~/.local/`, shell history (`~/.bash_history`), SSH keys (`~/.ssh`), and local workspace checkouts (`~/workspace`).

---

## 2. Remote Git-Backed Memory & Dotfiles (Gitea)

Beyond local container volumes, agents can maintain remote Git repositories on the local Gitea instance (`http://gitea:3000`) for durable, version-controlled reflection notes and dotfiles:

### Custom Dotfiles Repository (`fleet/agent-<name>-dotfiles`)
Used to version control custom bash aliases, prompt configurations, tools, and linters:
```bash
git clone http://gitea:3000/fleet/agent-${AGENT_NAME}-dotfiles.git /home/agent/.dotfiles
```

### Reflection & Memory Repository (`fleet/agent-<name>-memory`)
Used by agents to persist architectural notes, long-term learning, failure post-mortems, and session reflections:
```bash
git clone http://gitea:3000/fleet/agent-${AGENT_NAME}-memory.git /home/agent/.memory
```

### Best Practices
- Commit reflection notes periodically at milestone completion.
- Push state before planned instance teardowns.
- Pull memory upon new instance provisioning to resume context.
