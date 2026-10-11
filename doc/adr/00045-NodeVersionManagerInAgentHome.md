---
adr: "00045"
title: "Node Version Manager (NVM) Scoped to Agent Persistent Home"
topic: "Container Runtime & Storage"
theme: "THEME-RUNTIME"
status: "accepted"
version: "v0.1.0-alpha"
as_built: false
supersedes:
  - "00026"
tags:
  - runtime
  - nodejs
  - nvm
  - javascript
  - home-volume
executive_summary: "Adopts Node Version Manager (NVM) scoped to the unprivileged agent's persistent home directory (/home/agent/.nvm) with the greatest LTS release pre-installed as default. Decoupled from OCI image creation, allowing the operator and agent to manage and switch Node versions dynamically without rebuilding OCI images."
---

# 00045. Node Version Manager (NVM) Scoped to Agent Persistent Home

## Context

Following the deprecation of Deno ([ADR 00026](00026-DenoAsStandardJsRuntimeForOciContainerAgents.md)), sandbox agents and tool harnesses (including npm-based agent CLI tools, AI SDKs, and code evaluators) require a robust Node.js JavaScript/TypeScript runtime.

However, baking a static version of Node.js or system-wide package into the root filesystem of OCI images presents two major drawbacks:
1. **Inflexible Version Upgrades**: Switching or upgrading Node versions requires modifying Dockerfiles and rebuilding and redistributing OCI images.
2. **Permission Conflicts**: A system-level Node or root-owned `/usr/local` directory prevents unprivileged agent users (`agent`, UID 1000) from installing alternate versions, global npm packages, or switching runtime targets at execution time.

## Decision (What)

We adopt **Node Version Manager (`nvm`)** scoped exclusively to the unprivileged agent user's home directory (`/home/agent/.nvm`):

1. **Home Directory Scoping**:
   - `NVM_DIR` is configured as `/home/agent/.nvm`.
   - All Node runtimes, aliases, and global npm packages reside inside `/home/agent/.nvm` on the persistent named volume (`sndbx-agent-<name>-home`).
   - Operators and agents have full autonomy to run `nvm install <version>`, `nvm use <version>`, and update runtimes without modifying container images or needing root privileges.

2. **Default Greatest LTS Release**:
   - The greatest available Long-Term Support (LTS) release of Node.js (`lts/*`) is pre-installed during base image provisioning and set as the default alias.
   - Symlinks for `node`, `npm`, and `npx` are placed in `/usr/local/bin` and `/home/agent/.local/bin` pointing to the LTS release so non-interactive, non-login commands function seamlessly without explicitly sourcing `nvm.sh`.

3. **Persistent Volume Seeding**:
   - To support freshly mounted persistent home volumes (which shadow image `/home/agent`), a canonical NVM installation template is cached in `/usr/local/share/nvm-template` and `/etc/skel/.nvm`.
   - Container entrypoints automatically seed `/usr/local/share/nvm-template` into `/home/agent/.nvm` on container startup if the directory does not exist on the volume.

4. **Shell Integration**:
   - `/etc/profile.d/nvm.sh` and `/home/agent/.bashrc` configure `NVM_DIR="/home/agent/.nvm"` and source `nvm.sh` for interactive bash sessions.

## Status

Accepted (Supersedes ADR 00026).

## Consequences

### Positive
- **Decoupled from OCI Lifecycle**: Agents and operators can install, switch, or upgrade Node.js versions dynamically without rebuilding images.
- **Unprivileged Operation**: The `agent` user owns `/home/agent/.nvm`, allowing unprivileged npm global package installations and runtime changes.
- **Persistence Across Container Recreations**: Installed Node versions and tools persist across `sndbx agent clean` and recreate cycles because they reside on the persistent home volume.
- **Standard Compatibility**: Provides full compatibility with npm/npx tooling and agent harnesses requiring Node.js LTS.

### Negative / Trade-offs
- Volume initialization requires copying the template (~100MB) on the very first container boot for a fresh agent volume.
