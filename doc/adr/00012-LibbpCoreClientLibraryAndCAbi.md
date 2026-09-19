---
adr: "00012"
title: "libbp Core Client Library and Shared C ABI"
topic: "Security, Backplane & Messaging"
theme: "THEME-SECURITY"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - libbp
  - c-abi
  - sdk
  - backplane
  - polyglot
executive_summary: "Backplane IPC protocol logic is implemented in a native Go library (libbp) and exposed as a shared C ABI (libbp.so) for polyglot agent runtimes."
---

# 00012. libbp Core Client Library and Shared C ABI

## Context
Agents executing in sandboxes are written in various programming languages (Python, TypeScript/Node, Rust, Go). Reimplementing the backplane protocol—including Valkey connection management, ACL authentication, Ed25519 signing/verification, and envelope parsing—in every language introduces divergence, security vulnerabilities, and maintenance drag.

## Decision (What)
The canonical backplane protocol logic is implemented as a core Go package (`pkg/libbp`) and exported via cgo as a shared dynamic C library (`libbp.so`) accompanied by standard C header definitions (`libbp.h`).

The library provides a zero-dependency, stable C ABI exposing:
- `bp_client_create`, `bp_client_destroy`: Lifecycle management.
- `bp_publish_signed`: Message signing and publication.
- `bp_subscribe`: Event consumption with automatic signature verification.
- `bp_heartbeat`: Automated background telemetry reporting.

Higher-level language bindings (Python ctypes, Node.js FFI, Rust FFI) consume `libbp.so` directly, ensuring identical cryptographic and protocol adherence across all agent implementations.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Single source of truth for backplane protocol, signing, and security validation.
- Consistent behavior and bug fixes automatically propagate to all language bindings.
- High performance, memory-efficient networking core.

### Negative / Trade-offs
- Cross-compilation of `libbp.so` requires a working C toolchain (`gcc`/`clang`).
- Foreign Function Interface (FFI) bindings introduce minimal runtime overhead compared to native code.
