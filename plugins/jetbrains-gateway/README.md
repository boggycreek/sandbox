# Agent Sandbox - JetBrains Gateway Plugin

This module provides the JetBrains Gateway and IntelliJ Platform connector for **Agent Sandbox**, enabling zero-friction remote development into running rootless Podman agent containers.

## Overview

JetBrains Gateway connects thin clients to headless IDE backends running inside containers or remote hosts over SSH. This plugin integrates directly with the `sndbx` CLI to:
1. Detect running `sndbx-<name>` agent containers locally.
2. Provide a first-class "Agent Sandbox" provider tab inside the JetBrains Gateway and IDE remote development launchers.
3. Handle deep linking via `jetbrains-gateway://connect#type=sndbx&name=<agent>`.

## Fast Installation & Removal via `sndbx` CLI

Rather than manually downloading or building from source, you can install or remove the plugin directly across all local JetBrains IDE and Gateway installations with one command:

```bash
# Add / Install
sndbx plugin add gateway

# Remove / Uninstall
sndbx plugin remove gateway
```

The `sndbx plugin add gateway` command:
- Generates the valid `sndbx-gateway.jar` plugin package.
- Deploys it to `~/.local/share/agent-sandbox/plugins/jetbrains-gateway/` and discovered JetBrains IDE / Gateway directories (`WebStorm`, `GoLand`, `PyCharm`, `CLion`, etc.).
- Verifies that `~/.ssh/config` includes the managed `~/.local/share/agent-sandbox/ssh_config` file.

The `sndbx plugin remove gateway` command:
- Purges all deployed plugin JARs across IDE directories and the central repository.
- Safely unlinks the managed configuration from `~/.ssh/config`.

## Manual Build with Gradle

If developing or compiling the Kotlin extension points directly:

```bash
./gradlew buildPlugin
```

The resulting distribution archive will be located at `build/distributions/sndbx-gateway-0.1.0-alpha.zip`.
Install via JetBrains Gateway: **Settings -> Plugins -> Install Plugin from Disk...**.
