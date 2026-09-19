# 00036. JetBrains Gateway vs. Toolbox Plugin Decoupling and Native SSH Synchronization

## Context
In ADR 00035, JetBrains remote development integration was introduced under the `sndbx plugin <add|remove|list>` interface. However, empirical analysis and reverse engineering of the JetBrains Toolbox binary (`jetbrains-toolbox-3.1.0.62320`) and its runtime libraries revealed fundamental architectural differences between **JetBrains Gateway / IntelliJ Platform IDEs** and the **JetBrains Toolbox Desktop Application**:

1. **IntelliJ Platform vs. JetBrains Toolbox Architecture**:
   - Standalone JetBrains Gateway and individual JetBrains IDEs (GoLand, CLion, WebStorm, PyCharm) are IntelliJ Platform applications. They discover extensions using `META-INF/plugin.xml` declarations (`gatewayConnector`, `gatewayConnectionProvider`) deployed to `~/.local/share/JetBrains/<IDE>/`.
   - The JetBrains Toolbox App is a standalone Kotlin Multiplatform / Compose Desktop application (`jetbrains-toolbox`). It **never** reads IntelliJ `plugin.xml` or scans IDE plugin directories. It discovers remote development plugins strictly from `~/.local/share/JetBrains/Toolbox/plugins/<plugin-id>/` via an `extension.json` descriptor and Java `ServiceLoader` SPI (`com.jetbrains.toolbox.api.remoteDev.RemoteDevExtension`).

2. **OpenSSH Configuration Limitations in JetBrains Toolbox**:
   - JetBrains Toolbox bundles a native OpenSSH provider (`OpenSshImportedConfigsProviderImpl.kt`).
   - The internal implementation inspects `~/.ssh/config` using a line-by-line filter (`lines.filter { it.startsWith("Host ") }`) and **does not resolve OpenSSH `Include` directives**.
   - As a result, hosts defined in `~/.local/share/agent-sandbox/ssh_config` (included via `Include` in `~/.ssh/config`) are completely invisible to JetBrains Toolbox unless either inlined or registered directly in the Toolbox SSH provider settings file (`~/.local/share/JetBrains/Toolbox/plugins/ssh/settings.json`).

## Decision
We decouple JetBrains Gateway and JetBrains Toolbox into distinct, specialized targets within the `sndbx plugin` command domain, providing comprehensive support across both platforms:

### 1. Distinct Plugin Targets in CLI
The `sndbx plugin` command interface strictly supports the following targets:
- `sndbx plugin add gateway`: Installs the IntelliJ Platform JetBrains Gateway connector plugin (`sndbx-gateway.jar`) across discovered IDEs and links `~/.ssh/config`.
- `sndbx plugin add toolbox`: Installs the dedicated JetBrains Toolbox plugin to `~/.local/share/JetBrains/Toolbox/plugins/sndbx/`, synchronizes active agent endpoints directly into Toolbox's `ssh/settings.json`, and inlines managed host definitions into `~/.ssh/config`.
- `sndbx plugin add vscode`: Configures Visual Studio Code Remote-SSH integration and optional Claude Code extension ensembling.
- `sndbx plugin remove <gateway|toolbox|vscode>`: Cleanly removes deployed artifacts and configuration links for the respective target.
- `sndbx plugin list`: Enumerates installed and available integrations across all three targets.

### 2. Dual-Layer Toolbox Integration
To ensure running agent containers appear immediately in JetBrains Toolbox without friction:
1. **Dedicated Provider (`plugins/sndbx/`)**:
   Deploys `extension.json` (metadata adhering to `ExtensionJson` schema), `pluginIcon.svg`, and `sndbx-toolbox.jar`.
2. **Native SSH Provider Synchronization**:
   Directly parses and updates `~/.local/share/JetBrains/Toolbox/plugins/ssh/settings.json`:
   - Adds running `sndbx-<name>` containers to `envsJson.remotes` and `CONNECTION_STRINGS_CACHE`.
   - Sets activation flags (`TBX_DEPLOYMENT_TARGET_ENV` and `TBX_AUTO_CONNECT_ENV`).
   - Preserves all pre-existing user SSH remotes.
3. **Inlined Host Block in `~/.ssh/config`**:
   Injects running container host blocks demarcated by `# --- BEGIN AGENT SANDBOX MANAGED HOSTS ---` and `# --- END AGENT SANDBOX MANAGED HOSTS ---`, ensuring Toolbox's OpenSSH line parser detects sandbox hosts without manual intervention.

## Status
Accepted.

## Consequences
### Positive
- Solves the discovery gap where plugins installed for IntelliJ Gateway were invisible inside JetBrains Toolbox App.
- Active agent sandboxes appear directly in the JetBrains Toolbox tray UI under native SSH connections.
- Zero breakage of existing user configurations in `~/.ssh/config` or Toolbox's `ssh/settings.json`.
- Idempotent and clean state removal via `sndbx plugin remove toolbox`.

### Negative
- Requires maintaining two separate JetBrains descriptor formats (`plugin.xml` for Gateway/IDEs and `extension.json` for Toolbox App).
