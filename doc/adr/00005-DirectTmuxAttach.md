# 00005. Direct Tmux Attach Without Host Wrapper

## Context
When an operator connects to an agent session, managing the terminal multiplexer on the host machine while running another multiplexer inside the container causes nested `tmux`-in-`tmux` sessions. This nesting breaks terminal escape sequence handling, leads to corrupted truecolor rendering in TUIs, and creates conflicting keybindings (e.g. `Ctrl-b`).

## Decision
Do not spawn a host-side `tmux` wrapper session.
- The container runs its own standalone `tmux` daemon initialized in `entrypoint.sh`.
- Connecting to an agent (`sndbx agent connect <name>` or `agent.sh connect`) attaches directly into the container's session via `podman exec -it <container> tmux attach -t agent`.
- Detaching with `Ctrl-b d` disconnects the host client while leaving the agent process running unaffected inside the container.

## Status
Accepted.

## Consequences
- Clean, reliable truecolor TUI rendering and zero keybinding conflicts.
- Agents survive unexpected host terminal window closures or SSH disconnects.
