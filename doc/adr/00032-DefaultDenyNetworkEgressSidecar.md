# 00032. Default-Deny Network Egress via Dedicated Container Sidecar

## Context
Autonomous AI coding agents execute arbitrary code, download third-party packages, compile binaries, and execute tests. Granting unrestricted outbound network access to agent containers introduces severe security vulnerabilities:
1. **Data Exfiltration**: Malicious or hallucinated code, poisoned dependencies, or prompt injection payloads can exfiltrate sensitive repository contents, environment secrets, and credentials to arbitrary remote endpoints.
2. **Command & Control (C2) and Lateral Movement**: An agent container with unrestricted egress could establish reverse shells, connect to C2 infrastructure, or pivot toward internal networks.

We evaluated multiple enforcement locations:
1. **Host-Side Firewall / Packet Filtering (macOS `pf` / Linux `nftables`)**:
   - On macOS and Windows, rootless Podman runs inside a virtual machine (`podman-machine`) utilizing virtual networking layers (`gvproxy` or `slirp4netns`).
   - Host-level firewall rules cannot reliably inspect or enforce per-container egress policies across the VM boundary.
2. **In-Container Packet Filtering (`iptables` / `nftables` in agent container)**:
   - Enforcing firewall rules directly inside the agent container requires granting `CAP_NET_ADMIN` and `CAP_NET_RAW` Linux capabilities to the container.
   - This directly violates the non-root, unprivileged container principle (ADR 00001), opening significant container breakout surfaces and granting the agent the capability to reconfigure or disable its own network controls.

## Decision
We implement a **Default-Deny Network Egress Filter via a Dedicated Container Sidecar**:

### 1. Dedicated Egress Filter Sidecar (`egress-filter`)
- For each agent instance `<name>`, a companion sidecar container `<name>-egress` is deployed using a minimal, hardened Alpine-based image.
- The sidecar is granted `CAP_NET_ADMIN` and manages the network namespace for the pairing.
- The agent container runs without network capabilities and joins the sidecar's network namespace via `--net=container:<name>-egress`.
- All network traffic originating from the agent container must traverse the sidecar's virtual network stack.

### 2. Default-Deny Packet Filtering (`iptables` + `ipset`)
- The sidecar configures `iptables` with an explicit default `DROP` policy on the `OUTPUT` and `FORWARD` chains:
  - Inbound and outbound `ESTABLISHED,RELATED` traffic is permitted.
  - Loopback (`lo`) and local inter-container traffic is permitted.
  - Egress to the Agent Backplane network (Valkey and Gitea on the Docker/Podman bridge) is explicitly allowed.
  - All other outbound IPv4/IPv6 traffic is dropped by default.
- Outbound TCP/UDP packets are only allowed if their destination IP exists within a dynamically managed kernel `ipset` named `allowed-egress`.

### 3. DNS Snooping & Dynamic IP Allowlisting (`dnsmasq`)
- The sidecar runs a local `dnsmasq` resolver listening on `127.0.0.1:53`. The agent container's `/etc/resolv.conf` directs all DNS queries to this local resolver.
- `dnsmasq` is configured with an explicit domain allowlist corresponding to required agent development tools:
  - Anthropic API endpoints (`api.anthropic.com`)
  - Code hosting & version control (`github.com`, `api.github.com`, `gitlab.com`, and local Gitea)
  - Package registries and mirrors (`registry.npmjs.org`, `pypi.org`, `files.pythonhosted.org`, Debian/Ubuntu mirror repositories, Go proxy `proxy.golang.org`)
- When a domain query matches the allowlist, `dnsmasq` resolves the upstream DNS record and dynamically inserts the resolved IP address into the `allowed-egress` ipset (`ipset=/<domain>/allowed-egress`).
- Direct IP bypass attempts (e.g. attempting to connect directly to an IP address without prior DNS resolution, or resolving unapproved domains) fail immediately because the destination IP will not match the `allowed-egress` ipset.

### 4. Relocation of Published SSH Port (2222) to Sidecar
- Because the agent container shares the network namespace of the sidecar (`--net=container:<name>-egress`), port bindings must be declared on the primary network namespace holder.
- The dynamic SSH port mapping (`-p 127.0.0.1::<port>:2222` from ADR 00024) is moved from the agent container definition to the `<name>-egress` sidecar container.
- Inbound SSH connections terminate on the sidecar's loopback bridge and route transparently to the unprivileged `sshd` process running in the agent container.

## Status
Accepted.

## Consequences
- **Minimal Attack Surface**: The agent container operates in an egress-restricted environment where unapproved network exfiltration, reverse shells, and external C2 channels are blocked at the packet layer.
- **Strict Least Privilege**: The agent container itself retains zero network management capabilities (`CAP_NET_ADMIN` remains strictly confined to the isolated sidecar). The agent cannot disable or circumvent its network rules.
- **Direct IP Bypass Prevention**: Connections cannot evade domain restrictions by hardcoding raw IP addresses.
- **Operational Overhead**: Each agent sandbox requires two lightweight containers (`<name>` and `<name>-egress`). The CLI lifecycle management (`sndbx agent start`, `stop`, `retire`) orchestrates paired containers atomically.
- **Explicit Egress Management**: New developer tooling or custom API dependencies requiring external access must be explicitly added to the approved domain configuration.
