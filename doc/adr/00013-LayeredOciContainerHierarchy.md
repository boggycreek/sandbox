# 00013. Layered OCI Container Hierarchy (Base vs. Derivative)

## Context
Baking a specific AI agent runner (such as Claude Code) and application-specific plugins directly into the base sandbox image tightly couples the infrastructure to a single vendor/tool and forces large rebuilds whenever agent harnesses change. Additionally, operators want to run heterogeneous agent fleets (e.g. Claude Code, Aider, OpenCode, Codex, or custom LLM scripts) concurrently.

## Decision
Decouple container images into an **Agent-Agnostic Base Image** and **Derivative Agent Images**:
1. **Base Image (`images/base/Dockerfile`)**: Contains only neutral infrastructure: Debian base OS, unprivileged user `agent`, unprivileged `sshd` (port 2222), `tmux` with truecolor configuration, core VCS tools (`git`, `tea`, `bd`), inspection utilities (`jq`, `rg`, `fd`, `curl`), compiled `/usr/local/bin/bp` static binary, and the base entrypoint. Contains **no** AI agent runners or LLM SDKs.
2. **Derivative Agent Images (`images/agents/<type>/Dockerfile`)**: Built `FROM agent-sandbox-base:latest`. Installs specific agent runners (e.g. `@anthropic-ai/claude-code`, Python/`aider-chat`), agent-specific settings/allow-lists, and reactive launch prompts.
3. **Runtime Selection**: `sndbx agent start <name> --type=<type>` launches the requested derivative image.

## Status
Accepted.

## Consequences
- The base container image is completely reusable across any project or agent framework.
- Adding a new agent type requires creating a small downstream Dockerfile without touching base infrastructure.
- Faster build times due to efficient Docker/Podman layer caching.
