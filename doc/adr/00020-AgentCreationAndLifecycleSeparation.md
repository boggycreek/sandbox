# 00020. Agent Creation and Lifecycle Separation in sndbx CLI

## Context
When managing containerized AI agents, an agent instance requires:
1. An OCI image target (either a well-known preset such as `opencode`, `claude`, `agy`, `base`, or an arbitrary container registry URL).
2. Dedicated persistent home volume storage (`sndbx-agent-<name>-home`).
3. Dedicated Valkey backplane authentication credentials and ACL rules.
4. An Ed25519 cryptographic identity keypair and attestation.
5. Role and persona metadata.

Requiring the operator to specify image flags or role definitions on every container launch (`start`) leads to redundant typing, configuration drift, and brittle shell scripts. Conversely, conflating starting with attaching (`--connect`) complicates non-interactive process supervision and programmatic orchestration.

## Decision
The `sndbx` host management CLI separates **agent provisioning and configuration** (`create`) from **lifecycle execution** (`start`, `stop`, `connect`):

1. **Explicit Creation (`sndbx agent create <name> [as <type|oci>] [--image <type|oci>] [--role <role>]`)**:
   - Provisions and stores persistent agent metadata in `${XDG_DATA_HOME}/agent-sandbox/agents/<name>.json`.
   - Generates and stores the agent's dedicated Valkey password and Ed25519 keypair in `${XDG_DATA_HOME}/agent-sandbox/secrets/<name>/`.
   - Pre-creates the named persistent volume (`sndbx-agent-<name>-home`).
   - Supports well-known presets (`opencode`, `claude`, `agy`, `base`) or full OCI repository URLs.

2. **Decoupled Lifecycle Commands**:
   - `sndbx agent start <name>`: Reads the stored agent configuration and starts the container in daemon mode. No connection options or image arguments needed.
   - `sndbx agent connect <name>`: Attaches directly to the running container's tmux session via `podman exec -it`.
   - `sndbx agent ssh <name>`: Discovers the dynamic SSH port and connects via host SSH.
   - `sndbx agent stop [name]`: Halts running containers.
   - `sndbx agent clean <name>`: Removes the container while keeping persistent volumes.
   - `sndbx agent retire <name> [--force]`: Permanently deprovisions agent across container, volumes, secrets, Valkey ACLs, and Gitea account.

3. **Implementation in Go**:
   - `sndbx` is implemented as a native compiled Go binary in `cmd/sndbx`, sharing the Go monorepo's `pkg/libbp` client library and Podman container runtime package `pkg/runtime`.

## Status
Accepted.

## Consequences
- **Declarative & Idempotent**: Agent identities and image preferences are persisted across container restarts and host reboots.
- **Clean Tooling**: Eliminates redundant flags from daily commands (`start`, `stop`, `connect`).
- **Automation-Friendly**: Agent containers can be scripted, supervised, or started in background batches cleanly without interactive terminal hijacking.
