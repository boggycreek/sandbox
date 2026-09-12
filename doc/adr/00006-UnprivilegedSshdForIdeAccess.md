# 00006. Unprivileged SSH Daemon for IDE Remote Access

## Context
Developers often want to attach graphical IDEs (such as VS Code Remote-SSH, Cursor, or WebStorm Remote SSH interpreter) directly to an agent's workspace to inspect code, run tests, and debug alongside the agent. Standard SSH servers run as root on port 22 and typically require mounting personal host SSH keys.

## Decision
Run an unprivileged OpenSSH daemon (`sshd`) as user `agent` on port 2222 inside each container:
1. **Dedicated Keypair**: Host setup generates a dedicated, separate keypair (`~/.ssh/agent-sandbox` and `~/.ssh/agent-sandbox.pub`). The human's personal SSH keys are never mounted into containers.
2. **Public Key Import**: The host public key `~/.ssh/agent-sandbox.pub` is mounted read-only into `/home/agent/.ssh-host-keys/` and copied to `/home/agent/.ssh/authorized_keys` with correct ownership on container boot.
3. **Ephemeral Port Mapping**: Port 2222 maps to a dynamic host port (e.g., `127.0.0.1::2222`). `sndbx agent list` outputs connection targets formatted for CLI (`ssh -p <port>`) and IDEs (`agent@localhost:<port>`).

## Status
Accepted.

## Consequences
- Developers can attach any Remote-SSH IDE directly into a running sandbox with zero configuration.
- The human operator's personal private keys never enter container filesystems.
- The SSH daemon cannot perform root-level operations.
