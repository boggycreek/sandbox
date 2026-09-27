---
adr: "00001"
title: "Go Monorepo and Native Static Binaries"
topic: "Foundations & Architecture"
theme: "THEME-CORE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - architecture
  - monorepo
  - go
  - packaging
  - static-binary
executive_summary: "All Agent Sandbox host and in-container utilities are developed in a unified Go monorepo and compiled into statically linked, zero-dependency native binaries."
---

# 00001. Go Monorepo and Native Static Binaries

## Context
Agent Sandbox orchestrates autonomous developer agents across Linux host environments and isolated rootless containers. Distributed shell scripts, dynamic interpreters (Node, Python), and fragmented repositories introduce dependency drift, runtime version incompatibilities, and slow execution overhead across heterogeneous systems.

## Decision (What)
All core Agent Sandbox components—including the operator CLI (`sndbx`), backplane client (`bp`), in-container daemon (`bpd`), Model Context Protocol servers (`*-mcp`), and core libraries (`pkg/*`)—reside in a single unified Go monorepo.

All primary executables are compiled into self-contained, statically linked native binaries with debug symbols stripped. They require zero external runtime interpreters, dynamic libraries, or platform package dependencies on either the physical host or within containers.

To serve non-Go agent implementations executing inside containers, the monorepo additionally exports a C-shared library (`libbp.so`, ADR 00012) providing an unopinionated C ABI for language FFI (Python, Rust, C). In-container developer toolchains (e.g. Deno, ADR 00026) and infrastructure services (e.g. SonarQube, ADR 00027) run inside isolated containers, ensuring the host machine remains completely free of external runtime dependencies beyond Podman.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Zero runtime dependency prerequisites on host systems beyond the rootless container engine.
- Instant binary startup times and deterministic execution inside ephemeral containers.
- Atomic cross-component refactoring and unified code quality enforcement.
- Single versioned codebase shared across host orchestration and in-container daemons.
- Polyglot agent support enabled via dedicated C ABI without compromising host binary simplicity.

### Negative / Trade-offs
- Recompilation required when crossing host CPU architectures (x86_64 vs. aarch64).
- Go static binary footprint (~15–25MB per utility) compared to minimal shell scripts.
