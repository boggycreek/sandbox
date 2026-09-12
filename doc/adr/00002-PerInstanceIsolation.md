# 00002. Per-Instance Isolation for Parallel Sandboxes

## Context
Operators frequently run multiple agent instances concurrently on a single physical host machine (e.g., `<name-1>`, `<name-2>`). Sharing a single container or single workspace volume across multiple agents causes collisions in git working trees, conflicting dependency installs (`node_modules`), race conditions in tmux sessions, and overlapping port bindings.

## Decision
Each sandbox instance is completely isolated from sibling instances:
1. **Dedicated Container**: Named `cams-agent-sandbox-<name>` or `sndbx-agent-<name>`.
2. **Dedicated Persistent Volume**: Mounted at `/home/agent` (named `sndbx-agent-<name>-home`), ensuring independent workspace checkouts and shell histories.
3. **Dynamic Host SSH Port Allocation**: Port 2222 inside each container binds to an OS-assigned ephemeral host port (e.g. `127.0.0.1::2222`), dynamically discovered by `sndbx agent list` or `sndbx agent ssh`.
4. **Distinct Hostname & Shell Prompt**: Hostname is explicitly set to `<name>-sandbox` and the shell prompt displays `[agent@<name>-sandbox]:~$`.

## Status
Accepted.

## Consequences
- Multiple agents can execute parallel long-running workflows on independent git branches without interference.
- Operators can easily identify which agent terminal or SSH session they are in.
- Ephemeral host port mapping requires a lookup helper (`sndbx agent list` / `sndbx agent ssh <name>`) rather than static host ports.
