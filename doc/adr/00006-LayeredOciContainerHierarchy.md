---
adr: "00006"
title: "Layered OCI Container Hierarchy and Resolution"
topic: "Container Runtime & Storage"
theme: "THEME-RUNTIME"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - images
  - oci
  - layering
  - inheritance
  - presets
executive_summary: "Agent images follow a strict inheritance chain (base -> preset variants) resolved from local storage before falling back to external registries."
---

# 00006. Layered OCI Container Hierarchy and Resolution

## Context
Different AI agents require distinct toolchains (e.g. Claude Code requires Node and specific CLI wrappers, OpenCode requires Python/Go developer tooling, Antigravity requires language servers). Monolithic container images become bloated and slow to build, while completely independent images duplicate core system packages, runtime daemons, and SSH scaffolding.

## Decision (What)
Agent container images adhere to a strict, layered inheritance hierarchy rooted in a standardized base image:

1. **`sndbx-base`**: Minimal Debian/Ubuntu rootless foundation containing `agent` user UID 1000, `bpd` entrypoint daemon, `bp` CLI, rootless OpenSSH server, git, Deno 2.x runtime, and foundational build utilities.
2. **Preset Images**: Specialize `sndbx-base` by layering domain-specific runtimes:
   - `sndbx-opencode`: Polyglot developer toolchains (Python, Go, Rust) and the opencode static binary.
   - `sndbx-claude`: Python build tooling and Claude Code CLI installed via Deno's npm compatibility layer.
   - `sndbx-agy`: Antigravity autonomous development environment and language servers.

Image resolution in `sndbx` checks the local Podman image store (`localhost/<image>:latest`) first before attempting to pull from external container registries.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Common daemon (`bpd`), user setup, and SSH plumbing are maintained in a single base Containerfile.
- Fast builds via layer caching across all agent variants.
- Fully offline local development support by prioritizing local image stores.

### Negative / Trade-offs
- Modifying `sndbx-base` necessitates rebuilding all downstream preset images.
