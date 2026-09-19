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
All core Agent Sandbox components—including the operator CLI (`sndbx`), backplane client (`bp`), in-container daemon (`bpd`), and shared libraries (`pkg/*`)—reside in a single unified Go monorepo.

All executables are compiled into self-contained, statically linked native ELF binaries with debug symbols stripped (`-ldflags="-s -w"`). They require zero external runtime interpreters, dynamic libraries, or platform package dependencies on either the host or within containers.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Zero runtime dependency prerequisites on host systems beyond the container engine.
- Instant binary startup times and predictable execution inside ephemeral containers.
- Atomic cross-component refactoring and unified code quality enforcement.
- Single versioned codebase shared across host orchestration and in-container daemons.

### Negative / Trade-offs
- Recompilation required when crossing host CPU architectures (e.g. x86_64 vs. aarch64).
- Go static binary footprint (~15–25MB per utility) compared to minimal shell scripts.
