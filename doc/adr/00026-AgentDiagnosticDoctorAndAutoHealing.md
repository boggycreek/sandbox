# 00026. Agent Diagnostic Doctor and Automated Self-Healing

## Context
In a multi-agent sandbox environment with persistent named volumes, dynamically assigned host ports, cryptographic Ed25519 signing keys, Valkey ACL permissions, and Gitea account integrations, various subsystems can occasionally drift into inconsistent states. For instance:
1. Container image updates or podman volume changes might leave volumes or networks unlinked.
2. Valkey ACL users or Gitea accounts might fail to register if shared infrastructure was offline when `sndbx agent create` was executed.
3. Private signing keys (`signing-key.pem`) or host IDE SSH keys (`~/.ssh/agent-sandbox`) may be missing or unreadable.
4. OpenSSH `Include` directives in `~/.ssh/config` or active host stanzas in `~/.local/share/agent-sandbox/ssh_config` may become desynchronized.

Human operators need a single, reliable command to inspect an agent sandbox, diagnose misconfigurations or provisioning errors across all layers, automatically repair any recoverable issues, and clearly report any unrepairable conditions.

## Decision
Introduce the `sndbx agent doctor <agentname>` command powered by a self-healing diagnostic engine (`pkg/doctor`):

1. **Diagnostic Layers Checked**:
   - **Configuration Integrity**: Verifies metadata file presence, parses JSON, and ensures non-empty container names, volume names, and passwords.
   - **Cryptographic Keys**: Checks Ed25519 private key file validity and public key hex matching.
   - **Host SSH & IDE Integration**: Checks presence of host IDE keypair (`~/.ssh/agent-sandbox[.pub]`), managed `ssh_config` file, and `~/.ssh/config` `Include` directive.
   - **Network & Volume Storage**: Verifies that the Podman bridge network (`agent-sandbox-infra`) and persistent home volume (`agent-sandbox-<name>-home`) exist.
   - **Backplane (Valkey) Registration**: When infrastructure is reachable, verifies ACL user permissions and agent registry entry.
   - **Local Forge (Gitea) Registration**: When infrastructure is reachable, verifies user account existence, SSH public key registration, and `fleet` organization membership.
   - **Container & Port Synchronization**: Inspects container state, validates dynamic SSH port binding, and refreshes the managed SSH configuration snippet.

2. **Automated Healing**:
   - For any recoverable defect (missing network, missing volume, uncreated Valkey ACL, missing Gitea user/key/org, missing signing key, or missing SSH Include directive), the doctor automatically repairs the state and marks the item as `[Auto-Healed]`.

3. **Unrepairable Alerting**:
   - If a failure is fatal and unrecoverable (such as complete loss of the agent configuration file on disk), the doctor marks it as `[Unrepairable]` and gives actionable instructions to the operator.

4. **Operator-Only Security Perimeter**:
   - `sndbx agent doctor` is exclusively a host CLI tool for the human operator.
   - Autonomous agents inside container environments do not have access to `sndbx`, the host Podman socket, or host filesystem paths, maintaining the strict security perimeter (ADR 00001).

## Status
Accepted.

## Consequences
- Operators can easily diagnose and recover broken agents with a single command (`sndbx agent doctor <name>`).
- Agents created while infrastructure was offline can be fully provisioned once infrastructure comes online simply by running `sndbx agent doctor`.
- Clear, formatted diagnostic output provides full visibility into the agent's multi-layered health status without expanding in-container agent privileges.

