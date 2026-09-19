# 00035. JetBrains Gateway & Toolbox Plugin Architecture and Local Installation

## Context
As documented in ADR 00033, JetBrains Dev Containers integration is incompatible with pre-orchestrated, rootless Podman containers managed externally by `sndbx` (JetBrains issue `IJPL-172884`). Remote development into agent sandboxes therefore relies on the JetBrains Gateway thin-client architecture connecting via SSH and extracting the headless IDE backend into the container.

While OpenSSH host stanzas (`Host sndbx-<name>`) enable operators to manually connect via JetBrains Gateway's generic SSH provider, this workflow has several limitations:
1. **Discoverability**: Operators must know to navigate to Gateway's SSH connection dialog, enter host aliases, and configure port forwarding manually.
2. **Toolbox Integration**: JetBrains Toolbox displays installed IDEs and remote runtimes, but lacks awareness of running agent sandboxes without a dedicated connector plugin.
3. **Marketplace Distribution Delay**: Publishing a connector plugin to the public JetBrains Marketplace requires formal review, signature verification, and vendor approval cycles. During early alpha development, operators require immediate, zero-friction local installation without waiting for marketplace distribution.

## Decision
We introduce a first-class `sndbx plugin` command domain in the `sndbx` CLI and an accompanying JetBrains Gateway / Toolbox plugin architecture (`com.boggycreek.sndbx.gateway`) using an explicit `<add|remove|list>` verb model:

### 1. `sndbx plugin <add|remove|list> <toolbox|vscode>` CLI Syntax
Operators can install, remove, and audit IDE integrations directly via the CLI:
```bash
# Install and configure JetBrains Gateway & Toolbox plugin
sndbx plugin add toolbox
# Or:
sndbx plugin add vscode

# Uninstall and purge plugin files & SSH config linkage
sndbx plugin remove toolbox
sndbx plugin remove vscode

# Enumerate installed plugins and target locations
sndbx plugin list
```

Aliases are supported for operator ergonomics:
- `add`: `install`
- `remove`: `rm`, `uninstall`, `delete`
- `list`: `ls`

### 2. Autonomous Local Plugin Packaging and Deployment
Rather than requiring a pre-installed JDK or Gradle build environment on the operator's machine, `sndbx plugin add toolbox` dynamically generates and deploys a valid plugin JAR archive:
- **Archive Format**: A compliant JAR archive (`sndbx-gateway.jar`) containing `META-INF/MANIFEST.MF` and `META-INF/plugin.xml`.
- **Extension Points**:
  - `gatewayConnector` (`com.boggycreek.sndbx.gateway.SndbxGatewayConnector`): Registers "Agent Sandbox" as a first-class remote runtime provider in Gateway's left navigation drawer.
  - `gatewayConnectionProvider` (`com.boggycreek.sndbx.gateway.SndbxGatewayConnectionProvider`): Enables protocol handler deep linking (`jetbrains-gateway://connect#type=sndbx&name=<agent>`).
- **Target Directories**: The CLI automatically scans standard JetBrains directories:
  - Linux: `~/.local/share/JetBrains/` and `~/.config/JetBrains/` (targeting Gateway and installed IDEs: WebStorm, GoLand, CLion, PyCharm, etc.).
  - macOS: `~/Library/Application Support/JetBrains/`.
  - Windows: `%APPDATA%\JetBrains\`.
- **Central Storage**: Also maintains a copy at `~/.local/share/agent-sandbox/plugins/jetbrains-gateway/sndbx-gateway/lib/sndbx-gateway.jar` for manual disk loading if needed.

### 3. Transparent Host SSH Configuration Inclusion & Removal
JetBrains Gateway and Toolbox leverage OpenSSH host definitions from `~/.ssh/config`. `sndbx plugin add toolbox` and `sndbx plugin add vscode` inspect `~/.ssh/config` and guarantee the presence of:
```ssh
Include ~/.local/share/agent-sandbox/ssh_config
```
Conversely, `sndbx plugin remove <plugin>` cleanly unlinks the directive when desired, ensuring no stale configuration remains on the host.

### 4. Marketplace Independence
By distributing the plugin through `sndbx plugin add toolbox`, development velocity is decoupled from JetBrains Marketplace publishing timelines. Once marketplace publishing is desired in a later release, the plugin can be submitted to plugins.jetbrains.com while retaining `sndbx plugin add` as an offline and development fallback.

## Status
Accepted.

## Consequences
### Positive
- Consistent `<add|remove|list>` verb semantics matching other domain subcommands.
- One-command onboarding (`sndbx plugin add toolbox`) and clean removal (`sndbx plugin remove toolbox`).
- Zero external build tool dependencies required to install and activate the plugin locally.
- Instant visibility of running containers in JetBrains Gateway.
- Clean separation between local development distribution and future JetBrains Marketplace listing.

### Negative
- Local plugin directory deployment requires restarting active JetBrains IDE or Gateway instances to pick up newly added or removed plugin jars.
