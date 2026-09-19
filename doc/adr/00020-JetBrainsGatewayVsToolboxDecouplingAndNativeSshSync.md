---
adr: "00020"
title: "JetBrains Gateway vs. Toolbox Decoupling and Native SSH Sync"
topic: "IDE Plugin Management"
theme: "THEME-PLUGINS"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - plugins
  - jetbrains
  - toolbox
  - gateway
  - ssh-sync
  - reverse-engineering
executive_summary: "Decouples IntelliJ Gateway plugins (gateway) from JetBrains Toolbox App integration (toolbox), synchronizing running sandboxes directly to Toolbox's native ssh/settings.json and inlined host blocks."
---

# 00020. JetBrains Gateway vs. Toolbox Decoupling and Native SSH Sync

## Context
JetBrains remote development operates across two fundamentally distinct architectures:
1. **JetBrains Gateway / IntelliJ IDEs**: Traditional Java/IntelliJ Platform applications discovering plugins via `plugin.xml` located in `~/.local/share/JetBrains/<IDE>/`.
2. **JetBrains Toolbox Desktop App**: A standalone Kotlin Compose Desktop application that scans `~/.local/share/JetBrains/Toolbox/plugins/<folder>/` for `extension.json` descriptors and Java `RemoteDevExtension` SPI declarations. Furthermore, Toolbox's internal OpenSSH importer parses `~/.ssh/config` using a basic line filter (`lines.filter { it.startsWith("Host ") }`) and fails to recursively resolve OpenSSH `Include` directives.

Treating Gateway and Toolbox as a single target resulted in plugins failing to appear inside the JetBrains Toolbox desktop launcher.

## Decision (What)
`sndbx` decouples JetBrains Gateway and JetBrains Toolbox into distinct, specialized integration targets:

1. **`sndbx plugin add gateway`**: Deploys the IntelliJ Platform connector JAR (`sndbx-gateway.jar` declaring `<gatewayConnector>`) across all detected IDE directories and links `~/.ssh/config`.
2. **`sndbx plugin add toolbox`**:
   - Deploys a dedicated Toolbox plugin package (`extension.json`, `pluginIcon.svg`, `sndbx-toolbox.jar`) to `~/.local/share/JetBrains/Toolbox/plugins/sndbx/`.
   - Directly synchronizes active `sndbx-<name>` endpoints into Toolbox's native settings file (`~/.local/share/JetBrains/Toolbox/plugins/ssh/settings.json`), preserving existing user remotes.
   - Inlines managed host blocks delimited by `# --- BEGIN AGENT SANDBOX MANAGED HOSTS ---` directly into `~/.ssh/config` to satisfy Toolbox's line-by-line parser.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Resolves the discovery gap: running agents appear immediately in JetBrains Toolbox under native SSH connections.
- Native provider extension displayed inside the JetBrains Toolbox launcher.
- Preserves all pre-existing user configurations in both `~/.ssh/config` and Toolbox settings.

### Negative / Trade-offs
- Maintaining two separate JetBrains artifact packaging formats (`plugin.xml` vs. `extension.json`).
