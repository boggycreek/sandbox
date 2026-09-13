# 00025. Auto-Managed OpenSSH Include File for Agent Lifecycle Integration

## Context
In ADR 00024, an in-container unprivileged SSH daemon on port 2222 was adopted to enable human operators to ensemble with autonomous agents via desktop IDEs (VS Code Remote-SSH, Cursor, JetBrains Gateway) and terminal SSH.

Because container SSH ports are dynamically allocated by rootless Podman at runtime (`127.0.0.1::<port>`), the mapped port is a runtime property of a started container rather than a static property:
1. When an agent is created (`sndbx agent create`), it does not have an active port yet.
2. When an agent starts (`sndbx agent start`), a dynamic loopback port is bound.
3. If an agent is stopped and later restarted, Podman may allocate a different dynamic port.
4. If an agent is retired (`sndbx agent retire`), its SSH configuration should be removed.

Directly mutating the developer's personal `~/.ssh/config` during routine container starts and stops is risky: automated string edits can corrupt user-defined host rules, comments, or formatting. However, requiring manual copy-pasting of `sndbx agent ssh-config` creates friction.

OpenSSH 7.3+ supports the `Include` directive, allowing modular, isolated configuration snippets.

## Decision
We implement an **Auto-Managed OpenSSH Include File** synchronized automatically across agent runtime lifecycle events:

1. **Dedicated Managed Configuration File (`ssh_config`)**:
   - `sndbx` manages a dedicated configuration file at `~/.local/share/agent-sandbox/ssh_config` (`Paths.SSHConfigFile`).
   - The file contains auto-generated OpenSSH `Host sndbx-<name>` blocks for all currently running agent containers:
     ```ssh
     # Agent Sandbox Auto-Generated SSH Configuration
     # Managed automatically by sndbx. Do not edit manually.

     Host sndbx-<name>
         HostName 127.0.0.1
         Port <port>
         User agent
         IdentityFile ~/.ssh/agent-sandbox
         StrictHostKeyChecking no
         UserKnownHostsFile /dev/null
     ```

2. **Lifecycle Synchronization**:
   - `sndbx agent start`: After starting agent container(s), `SyncSSHConfigFile` regenerates `ssh_config` with live dynamic ports.
   - `sndbx agent stop`: After stopping agent container(s), `SyncSSHConfigFile` removes stopped agents from `ssh_config`.
   - `sndbx agent clean` / `retire`: `SyncSSHConfigFile` purges decommissioned agents immediately.

3. **Installer Integration**:
   - `install.sh` ensures `Include ~/.local/share/agent-sandbox/ssh_config` (or XDG equivalent) is present at the top of the user's `~/.ssh/config`.
   - The user's `~/.ssh/config` is modified only once during installation, referencing the managed file without further direct edits.

4. **Query CLI Preservation**:
   - `sndbx agent ssh-config [name] [--all]` remains available for on-demand output, inspection, and scripting.

## Status
Accepted.

## Consequences
- Desktop IDEs (VS Code Remote-SSH, Cursor, JetBrains Gateway) and terminal `ssh sndbx-<name>` discover and connect to running agents immediately with zero manual configuration.
- The user's personal `~/.ssh/config` remains clean, untouched by routine runtime lifecycle events.
- Stale port mappings are automatically cleaned up when containers are stopped or retired.
