# 00034. Root-Owned Agent Settings and Marketplace Authorization vs Plugin Version Split

## Context
Headless agent execution (ADR 00031) requires pre-authorized command execution without interactive human prompts:
- In ADR 00004, a broad permission allow-list (`defaultMode: "acceptEdits"` and allowlisted development commands) was adopted to ensure agents can execute autonomous loops (`git`, `bp`, build tools) without stalling on approval dialogs.
- These settings were persisted in `/home/agent/.claude/settings.json`.

However, the file ownership model presented a fundamental security flaw:
- Because `/home/agent/.claude/settings.json` was owned by the unprivileged container user `agent:agent` (UID 1000), any process running within the agent's turn could rewrite this configuration.
- Under prompt injection attacks, dependency confusion exploits, or compromised build scripts, an LLM turn could modify `settings.json` to grant itself dangerous privileges, alter safety filters, or register arbitrary third-party plugin marketplaces.
- Conversely, making the entire `.claude` directory read-only prevented the agent from installing or updating plugins, caching plugin versions, or storing legitimate runtime state.

## Decision
We establish a strict in-container trust boundary by separating immutable administrative authorization policies from mutable runtime plugin state:

### 1. Root-Owned Core Agent Settings (`settings.json`)
- The primary configuration file `/home/agent/.claude/settings.json` is owned by `root:root` with permissions `0644` (read-only to user `agent`).
- During image build or container initialization (`entrypoint.sh`), base permissions and pre-authorized commands (`defaultMode: "acceptEdits"`, allowlisted binaries such as `bp`, `git`, `make`, `npm`) are provisioned by root.
- The unprivileged `agent` user cannot overwrite, truncate, delete, or append to `settings.json`. Attempts by prompt injections or malicious scripts to escalate permissions fail with `Permission denied`.

### 2. Separation of Marketplace Authorization from Plugin Version State
We decompose plugin management into two distinct layers:
1. **Marketplace Authorization (`known_marketplaces.json`) — Root-Owned Build-Time Artifact**:
   - Located at `/home/agent/.claude/plugins/known_marketplaces.json` (or `/etc/claude/plugins/known_marketplaces.json`).
   - Owned by `root:root` with permissions `0644`.
   - Defines the cryptographic origins and trusted registries from which plugins may be sourced.
   - The unprivileged agent cannot register unvetted or attacker-controlled marketplaces.
2. **Plugin Version Caching & Installation (`installed_plugins.json`) — Agent-Owned Runtime State**:
   - Located at `/home/agent/.claude/plugins/installed_plugins.json`.
   - Owned by `agent:agent` with permissions `0644` (directory permissions `0755`).
   - Allows autonomous agent turns or plugin update commands to download approved plugin packages, record active versions, and cache operational metadata without requiring root privilege escalation.

### 3. Directory Layout and Inode Hardening
- To prevent user `agent` from deleting the parent directory or unlinking the root-owned files, the directory hierarchy is structured to enforce immutable permissions:
  ```text
  /home/agent/.claude/
  ├── settings.json                   (root:root, 0644 - Immutable permissions & policies)
  └── plugins/
      ├── known_marketplaces.json     (root:root, 0644 - Immutable authorized registries)
      └── installed_plugins.json      (agent:agent, 0644 - Mutable runtime version state)
  ```
- File permissions are validated during container initialization and verified by diagnostic health checks (`sndbx agent doctor`).

## Status
Accepted.

## Consequences
- **True In-Container Security Boundary**: The security perimeter inside the container cannot be subverted by the agent runtime itself. Prompt injections and untrusted third-party code cannot self-grant arbitrary bash permissions or disable safety controls.
- **Controlled Extensibility**: Autonomous agents retain the ability to install and update tools and plugins from pre-authorized marketplaces, ensuring runtime flexibility without sacrificing isolation.
- **Administrative Immutability**: Policy modifications to base command allow-lists or marketplace trust roots require deliberate administrative action via container image builds or root-privileged provisioning tasks.
