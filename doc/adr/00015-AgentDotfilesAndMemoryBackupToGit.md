# 00015. Agent Dotfiles and Memory Remote Backup to Git

## Context
While named container volumes persist state during routine restarts, volume purges (`sndbx agent clean-all`), host migrations, or hardware failures permanently destroy uncommitted agent customizations (dotfiles, custom aliases, bash functions) and learned context (agent reflections, project notes, long-term memory logs).

## Decision
Establish an automated remote backup model using dedicated Git repositories on the local Gitea instance for each agent:
1. **Per-Agent Repositories**:
   - `fleet/agent-<name>-dotfiles`: Backs up `~/.bashrc`, `~/.bash_aliases`, `~/.tmux.conf`, and custom scripts.
   - `fleet/agent-<name>-memory`: Backs up reflection logs, learned skills, scratchpads, and project context notes.
2. **Startup Reconciliation (`entrypoint.sh`)**: On container boot, queries Gitea. If repositories exist, pulls latest revisions and restores symlinks; if new, provisions repositories via Gitea API and pushes initial seed templates.
3. **Automated & Manual Sync**: Periodic cron jobs push updates every 15 minutes, with manual backup available via `bp backup` or `sndbx agent backup <name>`.

## Status
Accepted.

## Consequences
- Agent knowledge and shell customizations survive complete container volume destruction and hardware migrations.
- Provides a full audit trail (git commit history) of an agent's evolving knowledge base over time.
