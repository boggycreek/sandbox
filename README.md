# Agent Sandbox

A local, host-managed, multi-agent containerization platform designed to run autonomous AI coding agents safely in isolated environments on physical machines (macOS and Linux).

The sandbox gives AI agents full developmental agency (`git`, `npm`, `rm`, file creation, compilers) without prompts by using the container boundary as the primary security perimeter—ensuring zero host secret leakage and isolating workloads across parallel instances.

---

## Quick Start / Installation

### Option 1: Remote Installation (One-Liner via `curl`)

Install the Agent Sandbox directly from the repository without cloning manually:

```bash
curl -fsSL https://raw.githubusercontent.com/boggycreek/agent-sandbox/main/install.sh | bash
```

The installer will:
1. Detect your OS (macOS or Linux) and CPU architecture (`amd64` / `arm64`).
2. Verify host prerequisites (`git`, `curl`, `podman`, `ssh-keygen`).
3. Clone and sync the sandbox into `${XDG_DATA_HOME:-~/.local/share}/agent-sandbox/repo`.
4. Generate a dedicated IDE SSH keypair (`~/.ssh/agent-sandbox.pub`) for unprivileged container access.
5. Create an initial environment configuration file at `~/.local/share/agent-sandbox/.env`.
6. Install the `sndbx` host management CLI to `~/.local/bin/sndbx`.

### Option 2: Installation from a Cloned Repository

If you have already cloned the repository locally:

```bash
git clone https://github.com/boggycreek/agent-sandbox.git
cd agent-sandbox
./install.sh
```

The installer automatically recognizes the local checkout and sets it up as the active sandbox installation root.

---

## Key Features

- **Layered OCI Container Architecture**: A clean, agent-agnostic base OCI image (`images/base/`) provides the OS, unprivileged user (`agent`, UID 1000), unprivileged SSH daemon (port 2222), and tmux supervisor. Derivative images (`images/agents/`) add specific agent harnesses (Claude Code, Aider, custom runners).
- **Native Go Monorepo & Zero-Dependency Binaries**: Core tools are written in Go and compile to standalone static executables:
  - `sndbx`: Unified host CLI for agent lifecycle and shared infra.
  - `bp`: Cross-agent messaging CLI.
  - `libbp`: Shared core library exporting a C-ABI (`libbp.dylib`/`libbp.so`) for native macOS (Swift) and Linux (GTK4) desktop GUI frontends.
- **Valkey Communication Backplane**: Asynchronous cross-agent messaging bus utilizing Redis Streams (`<id>:out`, `<id>:inbox`, `<id>:seq`), per-agent Valkey ACLs, Ed25519 digital signatures, and reactive session turn wakeups.
- **Local Gitea Git Server**: A host-local, rootless Gitea server on the shared network providing a local Git forge for fleet code collaboration, pull requests, and git-native task management using `bd` (Beads).
- **Agent Dotfiles & Memory Backup**: Automatic synchronization of agent shell configurations and long-term reflection notes to dedicated Git repositories (`fleet/agent-<name>-dotfiles` and `fleet/agent-<name>-memory`) on Gitea.
- **IDE Remote Access**: Direct remote development from VS Code, Cursor, or WebStorm over unprivileged SSH without mounting personal host SSH keys.

---

## Host CLI (`sndbx`) Cheat Sheet

The `sndbx` CLI manages the fleet, shared infrastructure, and build targets:

```bash
# Shared Infrastructure (Valkey Backplane + Gitea Git Server)
sndbx infra up                                           # Start Valkey and Gitea stack
sndbx infra list                                         # View service status and connection endpoints
sndbx infra down                                         # Stop shared services (preserves data volumes)

# Agent Instance Lifecycle
sndbx agent create <name> [as <type>] [--role <role>]    # Provision a new named agent
sndbx agent start <name>                                 # Start the agent daemon container
sndbx agent connect <name>                               # Attach directly to running tmux supervisor
sndbx agent ssh <name>                                   # SSH directly into unprivileged environment
sndbx agent list [--json]                                # List configured instances, status, and SSH ports
sndbx agent stop [name] [--all]                          # Stop agent container(s)
sndbx agent clean <name>                                 # Remove container (preserves persistent home volume)
sndbx agent destroy <name>                               # Purge container, home volume, and secrets

# Repository Operations
sndbx repo path                                          # Print sandbox installation root path
sndbx repo build                                         # Compile native CLI binaries (bin/sndbx, bin/bp)
sndbx repo build-images                                  # Build base and derivative OCI container images
```

---

## Architecture & Design Documentation

Detailed architectural specifications and decision records are maintained in the repository:

- **[AGENTS.md](AGENTS.md)**: Comprehensive guide covering system architecture, messaging protocol, local inference gateway, security boundaries, and engineering quality gates.
- **[Architecture Decision Records (`doc/adr/`)](doc/adr/README.md)**: Complete log of sequential architectural decisions (00001 through 00021).

---

## Development & Contributing

To set up your local development environment for contributing to the repository, run:

```bash
./dev-setup.sh
```

To check prerequisite tools without modifying configuration:

```bash
./dev-setup.sh --dry-run
```

---

## Requirements

- **Operating System**: macOS 12+ (Apple Silicon or Intel) or Linux (Ubuntu, Debian, Fedora, Arch, Pop!_OS, etc.).
- **Container Engine**: Podman (required for rootless container execution).
- **Go Toolchain**: Go 1.23+ (for building native static binaries from source).
- **Core Utilities**: `git`, `curl`, `ssh-keygen`, `make`.
