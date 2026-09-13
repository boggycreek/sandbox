# Agent Sandbox — Agent Guidelines & Repository Guide

## Overview

The **Agent Sandbox** is a local, host-managed, multi-agent containerization platform created by **Boggy Creek Software LLC**. It is designed to run autonomous AI coding agents safely in isolated environments on physical machines (Linux and macOS) using **Podman**.

The container boundary serves as the primary security perimeter: agents operate with full autonomy (`git`, `npm`, `rm`, compilers, file edits) inside isolated containers without requiring per-command human approval prompts, while guaranteeing zero leakage of host secrets.

---

## Repository Architecture & Structure

This repository is structured as a pure **Go monorepo**:

```
agent-sandbox/
├── cmd/
│   ├── sndbx/              # Unified host management CLI
│   ├── bp/                 # Cross-agent backplane messaging CLI
│   └── libbp-c/            # C-shared library wrapper (libbp.so / libbp.dylib)
├── pkg/
│   ├── config/             # Agent configuration, XDG paths, and .env loader
│   ├── gitea/              # Gitea REST API client (user, key, org, repo management)
│   ├── libbp/              # Core backplane client, Valkey protocol, Ed25519 signing
│   │   └── resp/           # Custom high-performance Valkey/Redis RESP protocol parser
│   └── runtime/            # Podman container runtime, networks, infra orchestration
├── images/
│   ├── base/               # Neutral base OCI image (Debian Bookworm, non-root agent, sshd, tmux)
│   └── agents/             # Derivative agent harnesses (opencode, claude, agy)
├── infra/                  # Shared local infrastructure definition (docker-compose.yml)
├── test/
│   ├── harness/            # Embedded Valkey test server harness
│   └── integration/        # End-to-end integration tests (bp, sndbx, infra lifecycle)
├── doc/adr/                # Architecture Decision Records (00001 - 00021)
├── install.sh              # Host installation & bootstrap script
├── dev-setup.sh            # Developer environment setup & verification script
└── Makefile                # Quality gates, tests, linting, and build targets
```

---

## Core Components & Capabilities

### 1. Host Management CLI (`sndbx`)
Installed to `~/.local/bin/sndbx`, provides host-side management:
- **Agent Lifecycle**:
  - `sndbx agent create <name> [as <type>] [--role <role>] [--model-url <url>] [--model-name <name>] [--model-key <key>]`
  - `sndbx agent start <name>`: Starts container via Podman.
  - `sndbx agent connect <name>`: Attaches interactively to container tmux supervisor.
  - `sndbx agent ssh <name>`: Direct SSH into unprivileged agent environment.
  - `sndbx agent list [--json]`: Lists all agents, container status, and dynamic SSH ports.
  - `sndbx agent stop [name] [--all]`: Stops agent containers.
  - `sndbx agent clean <name>`: Removes container while preserving home volume.
  - `sndbx agent destroy <name>`: Purges container, persistent volume, and configs.
- **Shared Infrastructure**:
  - `sndbx infra up`: Starts shared Valkey 8 (`agent-sandbox-valkey`) and Gitea 1.22 (`agent-sandbox-gitea`) containers on the `agent-sandbox-infra` bridge network.
  - `sndbx infra list`: Inspects runtime status and ports.
  - `sndbx infra down`: Halts infrastructure containers.
- **Repository Operations**:
  - `sndbx repo path`: Prints installation root path.
  - `sndbx repo build`: Compiles native CLI binaries.
  - `sndbx repo build-images`: Builds base and derivative OCI container images.

### 2. Backplane Messaging CLI & Protocol (`bp`, `pkg/libbp`)
Cross-agent communication bus built on Valkey/Redis Streams:
- **Commands**:
  - `bp say <message> [--file <path>]`: Broadcasts message to public feed.
  - `bp tell <agent> <message> [--file <path>]`: Sends point-to-point message to inbox.
  - `bp reply <id> <recipient> <message>`: Replies to a specific threaded message citation.
  - `bp recv [--json] [--block <sec>]`: Reads pending inbox messages.
  - `bp human [--json]`: Reads messages addressed to or from the human operator.
  - `bp peers [--json]`: Lists registered active agents and status.
  - `bp status set <status>`: Updates agent status.
  - `bp finger [agent]`: Queries agent identity profile.
  - `bp liaison <get|set>`: Queries or appoints human liaison agent.
- **Security & Signatures**:
  - Every message is signed with the agent's Ed25519 private key (`crypto/ed25519`).
  - Messages carry canonical citations (`<agent>#<seq>`).
  - Valkey ACLs isolate agent access (`~<name>:*` read/write, `~*:inbox` write-only).

### 3. Local Gitea Forge (`pkg/gitea`)
- Automated user creation, public SSH key registration, and `fleet` organization membership on `sndbx agent create`.
- Pre-seeded repositories: `fleet/tools.git` (shared tools) and `fleet/tasks.git` (task tracking via `bd`).

### 4. Local Model Gateway (`llm-gateway`)
- All agent containers receive `--add-host=llm-gateway:host-gateway` to route traffic to local OpenAI-compatible inference servers (Ollama, llama.cpp, vLLM, LiteLLM) running on the physical host.
- Local URLs (`localhost` or `127.0.0.1`) are translated to `http://llm-gateway:<port>/v1` in agent configuration.

---

## Quality Gates & Engineering Guidelines

All code changes in this repository must adhere to the following standards:

1. **Copyright & Licensing**:
   - Every source file (`.go`, `.sh`, `.yml`) must begin with the standard **Boggy Creek Software LLC** MIT header:
     ```go
     // Copyright (c) 2026 Boggy Creek Software LLC
     //
     // Use of this source code is governed by an MIT-style
     // license that can be found in the LICENSE file.
     ```

2. **Git Commits**:
   - Commits must use conventional commit prefixes: `feat:`, `fix:`, `test:`, `docs:`, `refactor:`.
   - Maintain clean, atomic commits with informative commit messages.

3. **Required Container Engine**:
   - **Podman** is the required container engine (ADR 00019). Do not introduce dependencies on the Docker daemon or `docker-compose` binaries.

4. **Strict Test Coverage Threshold (>= 90.0%)**:
   - `make test-coverage` enforces **>= 90.0% atomic statement test coverage** across all packages (`./pkg/...` and `./cmd/...`).
   - Run `make check` to verify linting (`golangci-lint`, `shellcheck`), unit test coverage, and security SCA scans (`govulncheck`, `gosec`).

5. **Rootless Podman Test Best Practices**:
   - For unit tests that use `t.TempDir()`, register a cleanup hook with `t.Cleanup(func() { _ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run() })` to prevent subuid permission errors when removing test containers/storage.
   - Comprehensive integration tests belong in `test/integration/`.

---

## Helpful Commands

```bash
# Quality Gates
make check             # Run full suite: lint + test-coverage (>= 90.0%) + sca
make test              # Run unit tests with race detector
make test-coverage     # Run unit tests and generate coverage report (coverage/coverage.html)
make lint              # Run golangci-lint and shellcheck
make format            # Auto-format Go source code (gofmt)
make build             # Compile all native binaries into bin/

# Development Setup
./dev-setup.sh         # Verify/install development tools (golangci-lint, shellcheck, gosec, etc.)
./install.sh           # Install/update sndbx and bp binaries in ~/.local/bin
```
