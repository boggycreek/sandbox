# 00030. Unified Update Command for CLI Tooling and Container Images

## Context
Previously, the `sndbx repo` domain provided a set of manual subcommands:
- `sndbx repo path`: Emits the absolute path of the local repository checkout.
- `sndbx repo build`: Runs `make build-cli` to compile native CLI binaries into `bin/`.
- `sndbx repo build-images`: Runs `make build-images` to compile all native OCI container images with Podman.

This command structure was confusing and burdened the human operator with understanding the internals of monorepo build artifacts and choosing whether/when to rebuild images. Furthermore, `sndbx repo build` only compiled binaries into the repository's `bin/` directory rather than updating the user's active host executables in `~/.local/bin`, requiring manual re-installation steps.

When an operator wants to update their local installation, their true intent is straightforward: pull the latest changes from the upstream remote repository, ensure the host management CLI binaries are rebuilt and installed into `~/.local/bin`, and rebuild all well-known native OCI container images (`base`, `opencode`, `claude`, `agy`) so that subsequent agent creations and restarts leverage the updated images.

## Decision
1. **Deprecate and Eliminate the `sndbx repo` Domain**:
   - Completely remove `sndbx repo` (`build`, `build-images`, `path`) from the CLI command tree, usage documentation, and tests.
   - Any invocation of `sndbx repo` will fall through to the standard unknown command error.

2. **Introduce the Unified `sndbx update` Command**:
   - Provide a top-level command: `sndbx update`.
   - The command executes a comprehensive, end-to-end orchestration without requiring convenience flags:
     1. **Sync Local Checkout**: Resolves the repository root directory via `paths.ResolveRepoDir()` and synchronizes with upstream via `git pull --ff-only` (or `git pull` fallback).
     2. **Build and Install Host Tooling**: Rebuilds native Go CLI binaries (`sndbx`, `bp`) and installs them directly into `paths.BinDir` (`~/.local/bin`).
     3. **Build Native OCI Images**: Executes `make build-images` using Podman inside the repository checkout to rebuild all well-known native images (`agent-sandbox-base:latest`, `agent-sandbox-opencode:latest`, `agent-sandbox-claude:latest`, `agent-sandbox-agy:latest`).

## Status
Accepted.

## Consequences
- **Simplified Developer & Operator Experience**: Updating the entire Agent Sandbox stack is accomplished with a single command: `sndbx update`.
- **Zero Ambiguity**: Operators no longer have to manually decide which images or binaries to compile.
- **Clean Interface**: Eliminates the vestigial and confusing `repo` domain.
