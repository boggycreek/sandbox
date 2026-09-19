# Agent Sandbox - JetBrains Gateway & Toolbox Plugin

This module provides the JetBrains Gateway and Toolbox connector for **Agent Sandbox**, enabling zero-friction remote development into running rootless Podman agent containers.

## Overview

JetBrains Gateway connects thin clients to headless IDE backends running inside containers or remote hosts over SSH. This plugin integrates directly with the `sndbx` CLI to:
1. Detect running `sndbx-<name>` agent containers locally.
2. Provide a first-class "Agent Sandbox" provider tab inside the JetBrains Gateway and JetBrains Toolbox launcher.
3. Handle deep linking via `jetbrains-gateway://connect#type=sndbx&name=<agent>`.

## Fast Installation via `sndbx` CLI

Rather than manually downloading or building from source, you can install the plugin directly into all local JetBrains IDE and Gateway installations with one command:

```bash
sndbx plugin toolbox
# Or:
sndbx plugin install toolbox
```

This command:
- Generates the valid `sndbx-gateway.jar` plugin package.
- Deploys it to `~/.local/share/agent-sandbox/plugins/jetbrains-gateway/` and discovered JetBrains IDE / Gateway directories (`WebStorm`, `GoLand`, `PyCharm`, `CLion`, etc.).
- Verifies that `~/.ssh/config` includes the managed `~/.local/share/agent-sandbox/ssh_config` file.

## Manual Build with Gradle

If developing or compiling the Kotlin extension points directly:

```bash
./gradlew buildPlugin
```

The resulting distribution archive will be located at `build/distributions/sndbx-gateway-0.1.0-alpha.zip`.
Install via JetBrains Gateway: **Settings -> Plugins -> Install Plugin from Disk...**.
