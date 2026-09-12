# 00001. Non-Root Container User

## Context
AI coding agents (e.g., Claude Code, Aider) execute shell commands, edit files, manage packages, and interact with version control. If an agent executes as `root` inside the container or has `sudo` access, any software defect, prompt injection, or errant script could potentially exploit container escape vulnerabilities or manipulate host mounts.

## Decision
All agent processes and commands inside the container execute as a dedicated, unprivileged user named `agent` (UID 1000, GID 1000). 
- The container contains no `sudo` binary or permissions.
- There are no privilege escalation paths within the container.
- Required system packages, OS utilities, and global CLI binaries are installed at build time by `root` before switching to `USER agent`.

## Status
Accepted.

## Consequences
- **Safety**: Errant commands cannot modify system libraries, manipulate container network interfaces, or escalate privileges.
- **Predictability**: File ownership in persisted volumes consistently matches UID 1000, avoiding file permission mismatch issues on host volume mounts.
- **Build Discipline**: Any new system-level package dependency must be explicitly added to the Dockerfile and rebuilt, rather than ad-hoc installed at runtime.
