---
adr: "00012"
title: "libbp Core Client Library and Shared C ABI"
topic: "Security, Backplane & Messaging"
theme: "THEME-SECURITY"
status: "proposed"
version: "v0.1.0-alpha"
as_built: false
tags:
  - libbp
  - c-abi
  - sdk
  - backplane
  - polyglot
executive_summary: "Backplane IPC protocol logic is implemented in a native Go library (libbp) and planned for export as a shared C ABI (libbp.so) for polyglot agent runtimes."
---

# 00012. libbp Core Client Library and Shared C ABI

## Context
Agents executing in sandboxes are written in various programming languages (Python, TypeScript/Deno, Rust, Go). Reimplementing the backplane protocol—Valkey connection management, ACL authentication, Ed25519 signing, and envelope parsing—in every language risks divergence and security vulnerabilities.

## Decision (What)
The canonical backplane protocol logic is implemented as a core Go package (`pkg/libbp`) and exported via cgo as a shared dynamic C library (`libbp.so` / `libbp.dylib`) with standard C header definitions (`libbp.h`).

The planned library provides a stable C ABI exposing:
- `bp_client_create`, `bp_client_destroy`: Client lifecycle management.
- `bp_publish_signed`: Message signing and publication.
- `bp_subscribe`: Event consumption with automated signature verification.
- `bp_heartbeat`: Background telemetry reporting.

Higher-level language bindings (Python ctypes, Node/Deno FFI, Rust FFI) consume `libbp.so` directly, ensuring identical protocol and signing guarantees across all agent harnesses.

## Status
Proposed (Core `pkg/libbp` client library is accepted and built; `cmd/libbp-c` wrapper is planned).

## Consequences
### Positive
- Single source of truth for backplane protocol, signing, and security validation.
- Consistent behavior and bug fixes automatically propagate to all language bindings.
- High performance, memory-efficient networking core.

### Negative / Trade-offs
- Cross-compilation of `libbp.so` requires a working C toolchain (`gcc`/`clang`).
- Foreign Function Interface (FFI) bindings introduce minimal runtime overhead compared to native code.
