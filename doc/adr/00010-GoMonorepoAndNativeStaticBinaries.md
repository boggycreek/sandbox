# 00010. Go Monorepo and Native Static Binaries

## Context
Initial iterations relied on shell scripts (`agent.sh`, `cas.sh`, `infra.sh`) and TypeScript/Node CLI scripts run via `ts-node` or compiled with `esbuild`. This introduced external dependencies (Node.js runtime, npm packages) onto host environments and container images, leading to path resolution issues, type-stripping limitations with extensionless binaries, and packaging friction.

## Decision
Structure the entire codebase as a Go monorepo:
1. **Core Packages in `pkg/`**: Shared domain logic for backplane client operations (`pkg/libbp`), crypto signing (`pkg/crypto`), runtime container orchestration (`pkg/runtime`), and Gitea integration (`pkg/gitea`).
2. **Static Standalone Binaries in `cmd/`**:
   - `cmd/sndbx`: Host management CLI.
   - `cmd/bp`: Backplane messaging CLI.
   - `cmd/retention-sweep`: Stream pruning utility.
3. **Static Cross-Compilation**: Binaries compile with `CGO_ENABLED=0` into single static executables that run seamlessly across macOS (Darwin arm64/amd64) and Linux (Linux amd64/arm64) with zero runtime dependencies.

## Status
Accepted.

## Consequences
- Single toolchain (`go`) builds all CLI and core libraries.
- Static binaries are injected into base OCI images without requiring Node or python runtimes in the base image.
- High execution speed and instant CLI startup times.
