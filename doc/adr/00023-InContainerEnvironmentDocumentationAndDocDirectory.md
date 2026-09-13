# 00023. In-Container Environment Documentation and Dedicated doc Directory

## Context
When an autonomous AI agent operates inside a container:
1. It needs unambiguous instructions on how to use the local platform capabilities: the cross-agent messaging bus (`bp`), the local Git forge (Gitea), fleet repositories (`fleet/tools.git`, `fleet/tasks.git`), task management via `bd` (Beads), and local LLM inference gateway routing (`http://llm-gateway:<port>/v1`).
2. If this platform documentation is placed at the root of the workspace or named `AGENTS.md` / `CLAUDE.md`, it conflicts directly with project-specific documentation and guidelines present in repositories cloned into the workspace.
3. Because `/home/agent` is backed by a persistent named volume that survives container recreation, new or updated platform documentation in container images would not automatically appear unless synchronized by the runtime entrypoint.

## Decision
Establish `/home/agent/doc/` as the dedicated in-container location for all platform and environment reference documentation:

1. **Dedicated In-Container Directory (`~/doc/`)**:
   - The agent's home directory includes a permanent `~/doc/` directory dedicated to platform guides, separated entirely from workspace code repositories (`/home/agent/workspace/`).
   - The primary environment guide is placed at `~/doc/ENVIRONMENT.md`.

2. **Image Baking & Entrypoint Synchronization**:
   - Canonical documentation is baked into the neutral base OCI image under `/usr/local/share/doc/agent-sandbox/`.
   - The base image entrypoint (`images/base/entrypoint.sh`) synchronizes `/usr/local/share/doc/agent-sandbox/*` into `/home/agent/doc/` on container startup, ensuring existing persistent volumes receive documentation updates automatically upon image refresh.

3. **Content Covered**:
   - Runtime filesystem topology and user permissions.
   - Comprehensive `bp` backplane messaging commands and syntax (`say`, `tell`, `reply`, `recv`, `human`, `peers`, `status`, `finger`, `liaison`).
   - Local Gitea access (`http://gitea:3000`), user credentials, and `fleet` organization repositories (`fleet/tools.git`, `fleet/tasks.git`).
   - Local model inference routing (`http://llm-gateway:<port>/v1`).
   - Git-backed agent memory and dotfile remote synchronization.

## Status
Accepted.

## Consequences
- Zero documentation collision between platform tooling guides and project-level workspace repositories.
- Agents have a predictable, standardized reference manual at `~/doc/ENVIRONMENT.md`.
- Documentation stays in sync with image builds across persistent volume lifecycles.
