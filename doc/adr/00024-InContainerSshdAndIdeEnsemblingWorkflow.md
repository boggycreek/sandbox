# 00024. In-Container Unprivileged SSH Daemon and IDE Ensembling Workflow

## Context
A primary design requirement of Agent Sandbox is enabling seamless "ensembling" — human operators pair-programming, inspecting code, debugging, and executing tests collaboratively with autonomous AI coding agents directly within the agent's isolated container environment.

Developers rely heavily on modern desktop graphical IDEs such as VS Code (Remote-SSH), Cursor, JetBrains Gateway (IntelliJ, PyCharm, GoLand, WebStorm), and standard terminal SSH clients.

We evaluated three architectural approaches:
1. **Container Exec Only (`podman exec` / `podman attach`)**:
   - *Pros*: No network listeners or SSH daemon required inside the container.
   - *Cons*: Incompatible with IDE Remote-SSH ecosystems (VS Code Remote-SSH, Cursor, JetBrains Gateway) which require standard OpenSSH protocol handshakes, SFTP server subsystems, and persistent multi-channel multiplexing.
2. **Privileged Root SSH Daemon on Port 22**:
   - *Pros*: Standard OpenSSH port.
   - *Cons*: Severe security risk; requires root privileges inside the container, conflicts with the non-root principle (ADR 00001), and risks host privilege escalation.
3. **Unprivileged OpenSSH Daemon on Port 2222 with Dedicated Keypair & Host CLI Integration**:
   - *Pros*: Runs entirely as unprivileged user `agent` (UID 1000); uses a dedicated host keypair (`~/.ssh/agent-sandbox`) with zero exposure of human personal host keys; dynamic host port mapping prevents port collisions; automated host CLI helpers (`sndbx agent ssh-config` and `sndbx agent ssh`) provide frictionless one-click IDE attachment.
   - *Cons*: Requires running an in-container background SSH process and dynamic port discovery.

## Decision
We standardize on **Option 3: Unprivileged In-Container OpenSSH Daemon with Host Ensembling Tooling**:

1. **Unprivileged In-Container SSH Daemon**:
   - The base container image installs OpenSSH server (`openssh-server`) and configures `/etc/ssh/sshd_config` to listen on port `2222` with rootless privileges for user `agent` (UID 1000).
   - Password authentication is strictly disabled (`PasswordAuthentication no`); only authorized public key authentication is permitted.

2. **Dedicated Host Keypair Isolation**:
   - `install.sh` provisions a dedicated SSH keypair at `~/.ssh/agent-sandbox` and `~/.ssh/agent-sandbox.pub`.
   - Personal host private keys (`~/.ssh/id_rsa`, `~/.ssh/id_ed25519`) are never mounted into or accessible from containers.
   - The public key (`~/.ssh/agent-sandbox.pub`) is mounted read-only into `/tmp/host-keys/agent-sandbox.pub:ro` and imported into `/home/agent/.ssh/authorized_keys` during container boot (`entrypoint.sh`).

3. **Dynamic Host Port Mapping**:
   - Containers bind port 2222 to a dynamic loopback port (`-p 127.0.0.1::2222`), allowing any number of agent sandboxes to run simultaneously on the same host without port contention.

4. **Host IDE Ensembling & OpenSSH Config Generator**:
   - `sndbx agent ssh <name>`: Opens an immediate interactive SSH shell into the target agent environment.
   - `sndbx agent ssh-config [name] [--all]`: Emits standard OpenSSH `Host sndbx-<name>` configuration blocks, enabling instant host discovery in VS Code Remote-SSH, Cursor, JetBrains Gateway, and standard `ssh sndbx-<name>`.
   - `sndbx agent list`: Displays dynamic connection strings (`agent@localhost:<port>`).

5. **Ensembling Collaboration Modes**:
   - **IDE Remote Workspace**: Human edits files in `/home/agent/workspace/` and runs integrated IDE debuggers and terminal tasks while the autonomous agent operates concurrently.
   - **Live Terminal Pairing**: Human attaches directly to the agent's supervisor tmux session via `sndbx agent connect <name>` for real-time dual-cursor terminal collaboration.

## Status
Accepted.

## Consequences
- Operators can ensemble with any agent container using any Remote-SSH compatible IDE with complete feature fidelity (language servers, breakpoints, port forwarding, terminal multiplexing).
- Host security perimeter is maintained: no host private keys enter the container, and the in-container SSH process possesses no root capabilities.
- `sndbx agent ssh-config` eliminates manual port lookup friction for IDE configurations.
