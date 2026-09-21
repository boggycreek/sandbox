# Contributing to Agent Sandbox

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

Thank you for your interest in **Agent Sandbox**! This document provides governance guidelines and complete instructions for configuring your local developer environment, managing multi-version Go toolchains under XDG paths, running system diagnostics via `--doctor`, executing test suites, and tracking tasks using Beads (`bd`).

---

## 1. Governance & External Contributions

At this time, **Agent Sandbox is maintained solely by Boggy Creek Software LLC and is not accepting external pull requests or feature requests**. Public contributions and pull requests are disabled to maintain strict security boundaries and development velocity.

If you are inspecting, developing, or maintaining a fork of Agent Sandbox, feel free to inspect and modify the code under the terms of the [MIT License](LICENSE). The instructions below detail the environment setup, tooling, and standards used to build and test the project.

---

## 2. Automated Development Environment Setup (`setup.sh`)

To streamline onboarding across macOS and Linux developer workstations, `agent-sandbox` provides a root setup script: [`setup.sh`](setup.sh).

The script detects your operating system, CPU architecture, and available package manager, then installs and configures all required toolchains and dependencies.

### Quick Start

From the root of the repository, execute:

```bash
./setup.sh
```

For unattended or automated environments (e.g., CI runners, container builds, cloud devboxes), pass `-y` to run without confirmation prompts:

```bash
./setup.sh -y
```

If running without root/sudo privileges:

```bash
./setup.sh --no-sudo
```

---

## 3. Environment Diagnostics (`setup.sh --doctor`)

To verify whether your system has all required build tools, rootless Podman configuration, proper directory permissions, and valid XDG paths, run the diagnostic doctor:

```bash
./setup.sh --doctor
```

### What `--doctor` Inspects:

1. **Operating System & Hardware Architecture**: Platform (`darwin` / `linux`), kernel release, and CPU architecture (`amd64` / `arm64`).
2. **XDG Base Directory Compliance**:
   - `XDG_BIN_HOME` (`~/.local/bin`), `XDG_DATA_HOME` (`~/.local/share`), `XDG_CONFIG_HOME` (`~/.config`), and `XDG_CACHE_HOME` (`~/.cache`).
   - PATH inclusion of `~/.local/bin`.
   - Permissions of `.beads` directory (verifies `0700` mode).
3. **Go SDKs & Multi-Version Toolchains**:
   - Active Go binary path and resolved compiler version (`go version`).
   - Compatibility against the minimum version required by `go.mod` (e.g., Go 1.25.8).
   - Inventory of all installed XDG Go SDKs, indicating which one is active.
4. **Core Build Tools**: Presence and versions of `git`, `make`, C compiler (`gcc` or `clang`), `cmake`, and `pkg-config`.
5. **Container Engine (Podman — ADR 00019)**:
   - Podman binary presence and version.
   - Rootless container execution status.
6. **Developer Quality Gates, Linters & SCA Tools**:
   - `golangci-lint`: Go monorepo linter.
   - `shellcheck`: Bash script safety and correctness analyzer.
   - `govulncheck`: Go Software Composition Analysis (SCA).
   - `gosec`: AST-based security vulnerability scanner.
   - `deadcode`: Unreachable code reachability analyzer.
   - `syft`: Software Bill of Materials (SBOM) generator.
7. **Beads Issue Tracker (`bd`)**:
   - Validates that `bd` is installed and executable.
   - Verifies local issue database workspace status.
8. **Actionable Remediation**: If any checks produce `[WARN]` or `[FAIL]`, the doctor outputs exact shell commands to resolve the issue.

---

## 4. Supported Platforms & Package Managers

`setup.sh` provides first-class support for three primary package management ecosystems:

| Platform / Distro Family | Package Manager | Detected By | Default Packages Installed |
| :--- | :--- | :--- | :--- |
| **macOS (Darwin)** | Homebrew (`brew`) | `uname -s == Darwin` | `git`, `curl`, `make`, `pkg-config`, `cmake`, `podman`, Go SDK |
| **Linux (APT-based)** | APT (`apt-get`) | `apt-get` on PATH (Ubuntu, Debian, Pop!_OS, Linux Mint) | `curl`, `git`, `make`, `build-essential`, `pkg-config`, `tar`, `gzip`, `ca-certificates`, `cmake`, `podman`, Go SDK |
| **Linux (RPM-based)** | DNF / YUM (`dnf` / `yum`) | `dnf` or `yum` on PATH (Fedora, RHEL, CentOS, Rocky Linux, AlmaLinux) | `curl`, `git`, `make`, `gcc`, `gcc-c++`, `pkgconfig`, `tar`, `gzip`, `ca-certificates`, `cmake`, `podman`, Go SDK |

---

## 5. Multi-Version Go SDK Management (XDG Paths)

Standard Go multi-version tools pollute `$HOME/sdk`, and distro package managers frequently ship outdated Go compilers. `setup.sh` installs and manages Go SDKs entirely in user space adhering strictly to the XDG Base Directory Specification:

### Directory Layout

```
~/.local/
├── bin/
│   ├── go              -> ~/.local/share/go/sdk/current/bin/go       # Active Go binary
│   ├── gofmt           -> ~/.local/share/go/sdk/current/bin/gofmt    # Active gofmt
│   ├── go1.25.8        -> ~/.local/share/go/sdk/go1.25.8/bin/go      # Direct version alias
│   └── bd                                                            # Beads issue tracker CLI
│
└── share/
    └── go/
        └── sdk/
            ├── current -> go1.25.8                                   # Active SDK pointer
            └── go1.25.8/                                             # Full Go 1.25.8 SDK
```

### Managing Go Versions

- **Install a specific Go version** (e.g., 1.25.8):
  ```bash
  ./setup.sh --go-version 1.25.8
  ```
- **List installed Go versions**:
  ```bash
  ./setup.sh --list-go
  ```
- **Switch active Go version**:
  ```bash
  ./setup.sh --switch-go 1.25.8
  ```
- **Invoke a specific version directly**:
  ```bash
  go1.25.8 version
  go1.25.8 test ./...
  ```

---

## 6. Options and Flags for `setup.sh`

```bash
Usage: setup.sh [options]

Options:
  -y, --yes, --non-interactive   Run without prompting (assumes 'yes' to package installs)
  --doctor                       Run comprehensive development environment diagnostic checks
  --go-version <version>         Install and activate specific Go SDK version (e.g. 1.25.8)
  --list-go                      List all installed XDG Go SDK versions
  --switch-go <version>          Switch active Go SDK to an already installed version
  --no-sudo                      Do not use sudo (for rootless or unprivileged environments)
  --skip-go                      Skip Go toolchain installation/check
  --skip-beads                   Skip Beads (bd) CLI installation
  --skip-cmake                   Skip CMake build tool installation
  --skip-podman                  Skip Podman container engine installation
  --skip-tools                   Skip developer quality tools check
  --dry-run                      Print actions without executing commands
  -h, --help                     Show this help message
```

---

## 7. Task Tracking for Contributors: Beads (`bd`)

We use **Beads (`bd`)** — a distributed, Git- and Dolt-backed graph issue tracker — to manage backlogs, epics, features, and bug fixes for the `agent-sandbox` codebase.

> [!IMPORTANT]
> **Two-Plane Architectural Separation: Platform vs. Fleet**
> 
> Please note the strict architectural distinction between the two separate applications of Beads:
> - **Operator / Contributor Plane (This Repository)**: Used on the host workstation by human developers and AI coding agents working *on* the `agent-sandbox` platform. Issues use the prefix `sndbx-*` and synchronize with the upstream GitHub repository (`origin` / `boggycreek/sandbox.git`).
> - **Fleet / Workload Plane (In-Infrastructure)**: Autonomous agents executing *inside* running sandbox containers use a completely separate, air-gapped Beads instance backed by the local Gitea server (`http://gitea:3000/fleet/tasks.git`).
> 
> These two issue trackers share **zero storage, zero network endpoints, and zero database records**. Do not confuse or merge them.

### Contributor Workflow with `bd`

1. **Check for Ready / Unblocked Tasks**:
   ```bash
   bd ready
   ```
2. **Inspect the Task Hierarchy**:
   ```bash
   bd list
   ```
3. **View Issue Details & Acceptance Criteria**:
   ```bash
   bd show <id>
   ```
4. **Claim a Task**:
   ```bash
   bd update <id> --claim
   ```
5. **Create a New Feature, Bug, or Subtask**:
   ```bash
   bd create --title="Feature description" --description="Details" --type=feature --priority=2
   ```
6. **Close a Completed Task**:
   ```bash
   bd close <id> --reason="Resolved and validated by quality gates"
   ```

---

## 8. Building, Testing, and Local Infrastructure

Once dependencies are installed and `~/.local/bin` is in your `PATH`, verify the build with the monorepo `Makefile` targets:

### 1. Run Complete Quality Gate
```bash
make check
```
Executes format verification, Go linters (`golangci-lint`), shell linter (`shellcheck`), atomic test coverage enforcement (>= 90.0%), and static security analysis (`govulncheck`, `gosec`).

### 2. Run Unit Tests with Race Detection
```bash
make test
```

### 3. Generate Test Coverage Report
```bash
make test-coverage
```
Produces `coverage/coverage.html` and asserts that atomic statement coverage meets or exceeds the **>= 90.0%** gate.

### 4. Build Native Binaries
```bash
make build
# or:
make build-cli
```
Compiles `bin/sndbx` and `bin/bp`.

### 5. Local Infrastructure
```bash
# Start shared infrastructure (Valkey Backplane + Gitea Forge)
sndbx infra up
# or:
make infra-up

# Verify service status
sndbx infra list

# Stop shared infrastructure
sndbx infra down
```

---

## 9. Branching, Pull Request & Quality Guidelines

1. **Branch Isolation**:
   - `main` is protected. Never commit directly to `main`.
   - Work in dedicated feature branches or isolated git worktrees (`feat/<topic>`, `fix/<topic>`, `docs/<topic>`).
   - If working concurrently with other agents or team members, **always use isolated git worktrees** (`git worktree add ...`) to avoid colliding with working tree edits.
2. **Quality Gates (Strictly Enforced)**:
   Every pull request must pass the automated monorepo quality gate:
   ```bash
   make check
   ```
   This executes:
   - `make format`: Go formatting verification.
   - `make lint`: `golangci-lint` and `shellcheck`.
   - `make test-coverage`: **Strict >= 90.0% statement test coverage** across all packages.
   - `make sca`: Static application security analysis.
3. **Required Container Engine**:
   - **Podman** is the required container engine (ADR 00019). Do not introduce dependencies on Docker or `docker-compose` binaries.
4. **Conventional Commits**:
   Format commit messages following Conventional Commits (`feat: ...`, `fix: ...`, `docs: ...`, `test: ...`, `refactor: ...`).
5. **Architecture Decision Records (ADRs)**:
   Any significant architectural change, new infrastructure service, CLI command group, or protocol addition requires an ADR in `doc/adr/`.
6. **Copyright & License Header**:
   Every source file (`.go`, `.sh`, `.yml`) must begin with the standard **Boggy Creek Software LLC** MIT header:
   ```go
   // Copyright (c) 2026 Boggy Creek Software LLC
   //
   // Use of this source code is governed by an MIT-style
   // license that can be found in the LICENSE file.
   ```
