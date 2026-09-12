# 00003. Persisted State Across Container Recreation

## Context
When container images are updated, rebuilt, or refreshed, an agent should not lose its shell history, scratch notes, git working tree state, custom aliases, or authentication sessions. Early iterations persisted only specific subfolders (e.g. `workspace/` and `.claude/`), but agents routinely create state across their entire home directory (scratch files, tool caches, SSH host keys).

## Decision
Persist the entire `/home/agent` directory into a dedicated named volume per instance (`sndbx-agent-<name>-home`).
- When a fresh named volume is first attached, the container engine auto-seeds the volume from the files baked into the image under `/home/agent`.
- Subsequent container recreations (`sndbx agent refresh` or `sndbx agent clean`) mount the existing volume, preserving all state.
- A full volume wipe is explicitly separated into the `sndbx agent clean-all` command.

## Status
Accepted.

## Consequences
- Image rebuilds and container restarts do not discard in-progress agent work, custom aliases, or session state.
- Upgrading image-baked files (e.g. baseline dotfiles or entrypoints) requires runtime synchronization in `entrypoint.sh` because existing named volumes do not automatically overwrite modified files upon subsequent mounts.
