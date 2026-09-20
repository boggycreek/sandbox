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
├── doc/
│   ├── adr/                # Architecture Decision Records (00001 - 00028)
│   └── ai/                 # Progressive disclosure knowledge base optimized for AI agents
├── install.sh              # Host installation & bootstrap script
├── dev-setup.sh            # Developer environment setup & verification script
└── Makefile                # Quality gates, tests, linting, and build targets
```

---

## Agent Knowledge Base (`doc/ai/`)

For in-depth architectural specifications and operational protocols, refer to the progressive disclosure documents:
- **[Development Workflow & PR Protocol](doc/ai/dev-workflow.md)**: Branching, PR requirements, conventional commits, quality gates checklist.
- **[Architecture & Runtime](doc/ai/architecture.md)**: Podman isolation, security boundary, layered OCI hierarchy, local LLM gateway.
- **[OCI Image Resolution & Tagging](doc/ai/image-resolution.md)**: 3-tier image resolution, well-known presets, local store discovery, remote OCI refs (ADR 00029).
- **[Testing Guidelines & Coverage Gates](doc/ai/testing-guidelines.md)**: Strict >=90% test coverage enforcement, rootless Podman cleanup patterns, containerized smoke testing.
- **[Diagnostic Doctor & Auto-Healing](doc/ai/doctor-diagnostics.md)**: `sndbx agent doctor` and `sndbx infra doctor` diagnostic checks and self-healing.
- **[Contributor Guide & Task Tracking](CONTRIBUTING.md)**: Standards, quality gates, and Beads (`bd`) task tracking for developers of this repository.

---

## Repository Task Tracking for Contributors: Beads (`bd`)

Contributors and AI coding agents working on the `agent-sandbox` repository itself use **Beads (`bd`)** for graph issue tracking and backlog management:
- **Prefix**: `sndbx-*` (e.g., `sndbx-cpm`)
- **Dolt Remote**: Upstream GitHub (`git+ssh://git@github.com/boggycreek/sandbox.git`)
- **Key Commands**: `bd ready` (find unblocked work), `bd list` (view hierarchy), `bd update <id> --claim`, `bd sync` (push/pull Dolt issue commits to GitHub).

> **Architectural Boundary Notice**: This host repository issue tracker (`sndbx-*`) is strictly for platform development of `agent-sandbox`. It is completely distinct and isolated from the in-container Beads infrastructure used by autonomous fleet agents inside running sandboxes (which use local Gitea at `http://gitea:3000/fleet/tasks.git` as detailed in ADR 00028).

---

## Core Components & Capabilities

### 1. Host Management CLI (`sndbx`)
Installed to `~/.local/bin/sndbx`, provides host-side management:
- **Agent Lifecycle**:
  - `sndbx agent create <name> [as <type>] [--role <role>] [--model-url <url>] [--model-name <name>] [--model-key <key>]`
  - `sndbx agent start <name>`: Starts container via Podman.
  - `sndbx agent tmux <name>`: Attaches interactively to container tmux supervisor.
  - `sndbx agent open <name> [in <ide>] [--no-launch]`: Launches desktop IDE remote development (VS Code, JetBrains).
  - `sndbx agent ssh <name>`: Direct SSH into unprivileged agent environment.
  - `sndbx agent ssh-config [name] [--all]`: Emits OpenSSH configuration stanzas for IDE Remote-SSH discovery.
  - `sndbx agent doctor <name>`: Diagnoses agent configuration, keys, storage, and infrastructure provisioning, and auto-heals defects.
  - `sndbx agent list [--json]`: Lists all agents, container status, and dynamic SSH ports.
  - `sndbx agent stop [name] [--all]`: Stops agent containers.
  - `sndbx agent clean <name>`: Removes container while preserving home volume.
  - `sndbx agent retire <name> [--force]`: Fully deprovisions agent across container, volumes, secrets, Valkey ACLs/streams, and Gitea account.
- **Shared Infrastructure**:
  - `sndbx infra up`: Starts shared Valkey 8 (`agent-sandbox-valkey`) and Gitea 1.22 (`agent-sandbox-gitea`) containers on the `agent-sandbox-infra` bridge network.
  - `sndbx infra list`: Inspects runtime status and ports.
  - `sndbx infra doctor`: Diagnoses shared infrastructure networks, volumes, Valkey ACLs, Gitea repos, and auto-heals defects.
  - `sndbx infra down`: Halts infrastructure containers.
- **IDE Plugins & Integration**:
  - `sndbx plugin add <gateway|toolbox|vscode>`: Installs and configures IDE plugin / SSH integration.
  - `sndbx plugin remove <gateway|toolbox|vscode>`: Uninstalls and unlinks IDE plugin configuration.
  - `sndbx plugin list`: Lists supported and installed IDE plugins.
- **Update & Synchronization**:
  - `sndbx update`: Orchestrates complete host update (synchronizes git repository, rebuilds and installs native CLI binaries, and builds all native OCI images).

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

### 5. IDE Remote-SSH Ensembling & Managed SSH Config
- Unprivileged OpenSSH daemon runs on port `2222` inside each agent container.
- `install.sh` manages a dedicated keypair (`~/.ssh/agent-sandbox`) and adds `Include ~/.local/share/agent-sandbox/ssh_config` to `~/.ssh/config`.
- `sndbx` automatically updates `~/.local/share/agent-sandbox/ssh_config` on `start`, `stop`, `clean`, and `retire`, providing zero-touch host discovery in VS Code, Cursor, and JetBrains Gateway (`ssh sndbx-<name>`).


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

# Environment & Test Cleanup
make clean-test-env    # Safely clear stale test containers, orphaned netns, and test conmon processes
make clean-all         # Clean build artifacts + test environment state
```

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:46cd31e7 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   bd dolt push
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->

