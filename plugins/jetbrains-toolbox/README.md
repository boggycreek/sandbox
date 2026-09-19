# Agent Sandbox - JetBrains Toolbox App Plugin

This module provides the dedicated **JetBrains Toolbox App** integration for **Agent Sandbox**, enabling zero-friction discovery and remote connection to running rootless Podman agent containers directly from the JetBrains Toolbox desktop application.

## Overview

Unlike standalone JetBrains Gateway or IntelliJ IDEs which consume `plugin.xml` plugins, the JetBrains Toolbox desktop application is a Kotlin Compose Desktop application. It discovers external remote development plugins from `~/.local/share/JetBrains/Toolbox/plugins/<plugin-id>/` via an `extension.json` descriptor and Java `ServiceLoader` SPI (`com.jetbrains.toolbox.api.remoteDev.RemoteDevExtension`).

Additionally, JetBrains Toolbox bundles a native OpenSSH provider that parses `~/.ssh/config` line-by-line and maintains explicit connection targets in `~/.local/share/JetBrains/Toolbox/plugins/ssh/settings.json`.

The Agent Sandbox Toolbox integration provides dual-layer support:
1. **Dedicated Toolbox Provider**: Deploys `extension.json`, SVG icon, and extension JAR to `~/.local/share/JetBrains/Toolbox/plugins/sndbx/`.
2. **Native SSH Provider Synchronization**: Synchronizes running `agent@sndbx-<name>` endpoints directly into `settings.json` and inlines `Host sndbx-*` definitions into `~/.ssh/config` (bypassing the line parser's lack of `Include` directive recursion).

## Installation & Removal via `sndbx` CLI

You can install or remove the JetBrains Toolbox integration using standard CLI commands:

```bash
# Add / Install JetBrains Toolbox Integration
sndbx plugin add toolbox

# Remove / Uninstall JetBrains Toolbox Integration
sndbx plugin remove toolbox
```

The `sndbx plugin add toolbox` command:
- Deploys `extension.json`, `pluginIcon.svg`, and `sndbx-toolbox.jar` into `~/.local/share/JetBrains/Toolbox/plugins/sndbx/`.
- Syncs active agent containers into `~/.local/share/JetBrains/Toolbox/plugins/ssh/settings.json`.
- Inlines managed `Host sndbx-<name>` blocks directly into `~/.ssh/config`.

The `sndbx plugin remove toolbox` command:
- Removes `~/.local/share/JetBrains/Toolbox/plugins/sndbx/`.
- Purges `sndbx-*` remotes from `settings.json`.
- Strips the managed host block from `~/.ssh/config`.
