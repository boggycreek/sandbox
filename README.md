# Agent Sandbox

A local, host-managed, multi-agent containerization platform designed to run autonomous AI coding agents in isolated Podman environments on physical machines (macOS and Linux).

The sandbox gives AI agents autonomous execution capability (`git`, `deno`, `rm`, compilers, file edits) inside rootless containers without per-command human approval prompts, isolating agent workloads and protecting host files and credentials.

---

## Quick Start / Installation

### Option 1: Remote Installation (via `curl`)

Install the Agent Sandbox directly from the repository:

```bash
curl -fsSL https://raw.githubusercontent.com/boggycreek/sandbox/main/install.sh | bash
```

The installer will:
1. Detect host OS (macOS or Linux) and CPU architecture (`amd64` / `arm64`).
2. Verify host prerequisites (`git`, `curl`, `podman`, `ssh-keygen`).
3. Clone and sync the repository into `${XDG_DATA_HOME:-~/.local/share}/agent-sandbox/repo`.
4. Generate a dedicated IDE SSH keypair (`~/.ssh/agent-sandbox.pub`) for container access.
5. Create an initial environment configuration file at `~/.local/share/agent-sandbox/.env`.
6. Install the `sndbx` host management CLI and `bp` messaging CLI to `~/.local/bin`.

### Option 2: Installation from a Cloned Repository

If you have already cloned the repository locally:

```bash
git clone https://github.com/boggycreek/sandbox.git
cd sandbox
./install.sh
```

The installer recognizes the local checkout and sets it up as the active sandbox installation root.

---

## Core Architecture & Components

- **Layered OCI Container Hierarchy**:
  - `images/base/`: Neutral base image with Debian Bookworm, unprivileged user (`agent`, UID 1000), internal SSH daemon (port 2222), tmux supervisor, Deno 2.x runtime, and the `bpd` entrypoint daemon.
  - `images/agents/`: Derivative agent harnesses (`opencode`, `claude`, `agy`).
- **Native Go Monorepo & Zero-Dependency Binaries**:
  - `sndbx`: Unified host CLI for managing agent lifecycles, shared infrastructure, and IDE plugins.
  - `bp`: Cross-agent messaging CLI.
  - `bpd`: In-container backplane daemon handling background message polling, headless command execution, and turn dispatching.
  - `libbp`: Core backplane client library with RESP protocol parser and Ed25519 signing.
- **Valkey Communication Backplane**: Asynchronous cross-agent messaging bus utilizing Redis Streams (`<id>:out`, `<id>:inbox`, `<id>:seq`), per-agent Valkey ACLs, Ed25519 digital signatures, and threaded reply citations.
- **Local Gitea Git Forge**: A host-local, rootless Gitea server on the `agent-sandbox-infra` network providing Git hosting, inter-agent pull requests, and distributed task tracking with Beads (`bd`).
- **In-Container Task Tracking (Beads)**: Task graph and dependency tracking (`~/tasks`, `fleet-tasks ready`, `bd ready`) backed by `http://gitea:3000/fleet/tasks.git`.
- **Local Model Gateway (`llm-gateway`)**: Host gateway routing (`http://llm-gateway:<port>/v1`) allowing containerized agents to query local OpenAI-compatible inference servers (Ollama, llama.cpp, vLLM, LiteLLM) running on the host.
- **IDE Remote Access**: Direct remote development from VS Code and JetBrains IDEs over unprivileged SSH with isolated per-agent host aliases (`sndbx-<name>`).

---

## Host CLI (`sndbx`) Command Reference

```bash
# Shared Infrastructure (Valkey Backplane + Gitea Forge)
sndbx infra up                                           # Start Valkey and Gitea containers
sndbx infra list                                         # View service status and connection endpoints
sndbx infra doctor                                       # Diagnose shared infrastructure and auto-heal defects
sndbx infra down                                         # Stop shared services (preserves data volumes)

# Agent Instance Lifecycle
sndbx agent create <name> [as <type>] [--role <role>]    # Provision a new agent (base, opencode, claude, agy)
sndbx agent start <name>                                 # Start the agent container with Podman
sndbx agent tmux <name>                                  # Attach interactively to the container tmux session
sndbx agent open <name> [in <ide>] [--no-launch]         # Launch desktop IDE remote development (VS Code, JetBrains)
sndbx agent ssh <name>                                   # SSH directly into unprivileged environment
sndbx agent ssh-config [name] [--all]                    # Generate OpenSSH config stanzas for IDE Remote-SSH
sndbx agent doctor <name>                                # Diagnose configuration, keys, storage, and auto-heal
sndbx agent list [--json]                                # List configured instances, status, and SSH ports
sndbx agent stop [name] [--all]                          # Stop agent container(s)
sndbx agent clean <name>                                 # Remove container (preserves persistent home volume)
sndbx agent retire <name> [--force]                      # Fully deprovision agent (container, volume, Valkey, Gitea)

# IDE Plugins & Integration
sndbx plugin add <gateway|toolbox|vscode>                # Install & configure IDE plugin / SSH integration
sndbx plugin remove <gateway|toolbox|vscode>             # Uninstall & unlink IDE plugin configuration
sndbx plugin list                                        # List supported and installed IDE plugins

# Installation & Workspace Updates
sndbx update                                             # Synchronize git repo, rebuild host CLIs, and build OCI images
```

---

## Architecture & Design Documentation

Detailed architectural specifications and decision records:

- **[AGENTS.md](AGENTS.md)**: System guide covering runtime architecture, messaging protocol, local inference gateway, security boundaries, and engineering quality gates.
- **[Architecture Decision Records (`doc/adr/`)](doc/adr/README.md)**: Complete log of architectural decision records (00001 through 00028).
- **[Developer Workflow & Contributing](CONTRIBUTING.md)**: Contributor guide, code quality gates, and Beads issue tracking workflow.

---

## System Requirements

- **Operating System**: macOS 12+ (Apple Silicon or Intel) or Linux (Ubuntu, Debian, Fedora, Arch, Pop!_OS, etc.).
- **Container Engine**: Podman (required for rootless container execution).
- **Go Toolchain**: Go 1.23+ (for building native static binaries from source).
- **Core Utilities**: `git`, `curl`, `ssh-keygen`, `make`.

---

## License & Open Source Attribution

- **License**: Agent Sandbox is licensed under the [MIT License](LICENSE) by **Boggy Creek Software LLC**.
- **Acknowledgements**: See [ACKNOWLEDGEMENTS.md](ACKNOWLEDGEMENTS.md) for ecosystem credits and community appreciation.
- **Third-Party Notices**: See [NOTICES.md](NOTICES.md) for third-party copyright statements, SPDX license identifiers, and legal notices.


