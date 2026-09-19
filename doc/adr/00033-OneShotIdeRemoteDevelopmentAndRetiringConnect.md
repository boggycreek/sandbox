# 00033. One-Shot IDE Remote Development, Per-Alias Known Hosts, and Retiring the Connect Verb

## Context
As the Agent Sandbox project transitioned toward headless daemon operation (ADR 00031) and desktop IDE ensembling (ADR 00024, ADR 00025), human operator workflows shifted significantly:
1. **Ambiguity of `sndbx agent connect`**:
   - The legacy `sndbx agent connect <name>` verb originally attached directly to the agent's interactive `tmux` session (ADR 00005).
   - In practice, operators frequently confused `connect` with opening an SSH terminal or opening their IDE workspace. With `bpd` running headlessly, attaching to `tmux` became a diagnostic fallback rather than the primary entrypoint.
2. **SSH Host Key Collisions on Dynamic Port Reuse**:
   - Containers bind SSH port 2222 to dynamic loopback ports (`127.0.0.1::<port>`). When containers are stopped, recreated, or restarted, Podman often reallocates a previously used port to a different container with a different host key.
   - ADR 00025 mitigated verification failures by specifying `StrictHostKeyChecking no` and `UserKnownHostsFile /dev/null`. However, this disabled host key verification entirely, exposing connections to man-in-the-middle (MITM) risks and causing IDE connection warnings. Conversely, using a standard shared `~/.ssh/known_hosts` resulted in constant `Host key verification failed` errors when ports were reused across instances.
3. **IDE Remote Development Friction**:
   - Operators using desktop IDEs (VS Code, Cursor, JetBrains WebStorm/GoLand/IntelliJ) require an automated, one-shot mechanism to launch remote workspaces.
   - Developers frequently had to manually copy connection strings, locate workspace directories, install remote extensions, and configure remote settings.
   - For JetBrains IDEs, JetBrains Dev Containers integration proved brittle because it attempts to manage container lifecycles from scratch rather than attaching to existing, running sandbox containers.

## Decision
We streamline the operator CLI commands, introduce one-shot IDE launching, implement per-alias known hosts files, and formally retire the ambiguous `connect` verb:

### 1. Retiring `sndbx agent connect` in Favor of `sndbx agent tmux`
- Deprecate and remove `sndbx agent connect <name>`.
- Introduce `sndbx agent tmux <name>` as an explicit, dedicated diagnostic verb:
  - Directly attaches to the in-container tmux diagnostic session (`podman exec -it <container> tmux attach -t ...`).
  - Sets the advisory attach lock (`~/.bpd-attached`) to pause autonomous background turns during human interaction (ADR 00031).
  - Provides clear symmetry across CLI verbs:
    - `sndbx agent ssh <name>`: Interactive terminal SSH shell.
    - `sndbx agent tmux <name>`: Low-level diagnostic terminal multiplexer attach.
    - `sndbx agent open <name>`: Desktop IDE remote development launcher.

### 2. Dedicated Per-Alias Known Hosts (`~/.ssh/known_hosts.sndbx-<name>`)
- Rather than setting `UserKnownHostsFile /dev/null` or polluting the operator's global `~/.ssh/known_hosts`, the managed SSH configuration (`~/.local/share/agent-sandbox/ssh_config`) specifies an instance-specific known hosts file:
  ```ssh
  Host sndbx-<name>
      HostName 127.0.0.1
      Port <dynamic-port>
      User agent
      IdentityFile ~/.ssh/agent-sandbox
      StrictHostKeyChecking accept-new
      UserKnownHostsFile ~/.ssh/known_hosts.sndbx-<name>
  ```
- `StrictHostKeyChecking accept-new` automatically trusts the host key upon the container's first connection and strictly verifies subsequent connections.
- When an agent is recreated or retired (`sndbx agent clean`, `sndbx agent retire`), its isolated `known_hosts.sndbx-<name>` is cleaned up, eliminating host key mismatch collisions when ports are reassigned.

### 3. One-Shot IDE Remote Development (`sndbx agent open`)
- Introduce `sndbx agent open <name> [--ide <code|webstorm>] [--no-launch]`:
  - Automatically resolves the agent's connection parameters and ensures the container is running.
  - `--ide <code|webstorm>` specifies the target IDE (default: `code` / VS Code).
  - `--no-launch` prepares SSH configurations, known hosts, and multi-root workspace definitions without launching the host IDE executable (useful for headless scripting and CI).
- **VS Code Integration**:
  - Automatically invokes VS Code Remote-SSH URI scheme:
    ```bash
    code --folder-uri "vscode-remote://ssh-remote+sndbx-<name>/home/agent/workspace"
    ```
  - Generates a multi-root workspace file (`agent.code-workspace`) exposing `/home/agent/workspace` alongside `/home/agent/.claude` or doc directories.
  - Ensures automated remote installation of essential IDE extensions (such as the Claude extension and language tools) inside the container upon launch.
- **JetBrains Gateway Integration**:
  - JetBrains Dev Containers is rejected due to container lifecycle conflicts and lack of support for attaching to existing running rootless Podman containers.
  - Standardizes on JetBrains Gateway over Remote-SSH via the prepared `sndbx-<name>` SSH host configuration.

### 4. SFTP Subsystem Enforcement in In-Container SSH Daemon
- Ensure the OpenSSH server configuration (`/etc/ssh/sshd_config`) inside `agent-sandbox-base` explicitly enables the SFTP subsystem:
  ```ssh
  Subsystem sftp internal-sftp
  ```
- IDE remote backends (such as VS Code Remote and JetBrains Gateway backend installers) depend on SFTP to upload agent scripts, transfer language server binaries, and index remote workspaces.

## Status
Accepted.

## Consequences
- **Elimination of Ambiguity**: Operators have clear, explicit commands for each workflow: `open` for IDEs, `ssh` for shell access, and `tmux` for troubleshooting.
- **Strong Host Key Verification**: Replaces insecure `UserKnownHostsFile /dev/null` with isolated per-agent host key files (`accept-new`), maintaining cryptographic integrity against MITM attacks while eliminating port-reuse collision errors.
- **Frictionless Developer Experience**: A single command (`sndbx agent open my-agent`) instantly connects VS Code or JetBrains IDEs to the running container with all extensions, workspaces, and SSH tunnels pre-configured.
- **Standardized Remote Protocol**: By relying on OpenSSH and SFTP rather than proprietary container exec hooks, any Remote-SSH compatible toolchain can seamlessly attach to Agent Sandbox environments.
