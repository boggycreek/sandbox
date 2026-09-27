---
adr: "00034"
title: "Multi-Version Go Toolchain Management via XDG Standard"
topic: "Development Environment & Toolchain"
theme: "THEME-CORE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - go
  - toolchain
  - xdg
  - setup
  - multi-platform
  - developer-experience
executive_summary: "Standardizes automated Go toolchain provisioning and multi-version management under the XDG Base Directory specification (~/.local/share/go/sdk), ensuring reproducible builds and quality gate execution across host environments without requiring root/sudo privileges or system-level modifications."
---

# 00034. Multi-Version Go Toolchain Management via XDG Standard

## Context
Developing the `agent-sandbox` platform requires modern Go language features, rigorous static analysis (`golangci-lint`), and strict quality gate enforcement (ADR 00001, ADR 00025).

Host workstations across Linux and macOS frequently exhibit disparate, outdated, or customized Go toolchain installations. Upgrading system packages or writing to `/usr/local/go` requires `sudo` privileges, risks breaking existing developer projects, and creates installation friction on managed or restricted workstations.

## Decision (What)
We standardize isolated, automated Go toolchain provisioning adhering strictly to the **XDG Base Directory Specification**:

1. **Unprivileged Directory Layout**: The platform installs and manages pinned Go distributions under `$XDG_DATA_HOME/go/sdk` (defaulting to `~/.local/share/go/sdk`), with isolated `GOPATH` at `~/.local/share/go` and `GOBIN` at `~/.local/bin`.
2. **Zero Root / Sudo Requirement**: The developer setup script (`setup.sh`) downloads and verifies official pre-compiled Go archives for the host OS and architecture (Linux AMD64/ARM64, macOS Apple Silicon/Intel) entirely within user space.
3. **Dedicated Tool Suite**: Quality gate and operational tools (`golangci-lint`, `shellcheck`, `govulncheck`, `gosec`, `bd`, `dolt`) are compiled or installed directly into `~/.local/bin` and isolated Go paths, preventing contamination of host system binaries.
4. **Environment Sourcing & Modularity**: An environment profile (`~/.local/share/agent-sandbox/env`) exports necessary `PATH`, `GOROOT`, and `GOPATH` modifications, allowing developers to switch or source toolchain environments cleanly without permanent destructive shell profile alterations.
5. **Preflight Diagnostics (`setup.sh --doctor`)**: An integrated preflight check inspects OS architecture, kernel cgroups, rootless Podman configuration, subuid ranges, and toolchain paths to verify complete readiness before building.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Guarantees bit-for-bit compiler and linter parity across all physical developer machines and CI pipelines.
- Requires zero administrative or `sudo` privileges to set up, update, or run quality gates.
- Eliminates side effects or version collisions with existing system Go installations.
- Fully adheres to XDG conventions for clean, predictable user filesystem layouts.

### Negative / Trade-offs
- Consumes additional disk space (~300–400MB) for the isolated Go distribution and cached tool binaries.
- Developers must include `~/.local/bin` in their interactive shell `PATH`.
