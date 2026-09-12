# 00012. Core Backplane Client Library (`libbp`) with C-ABI FFI

## Context
Developing separate backplane client implementations in Go (for CLI utilities), Swift (for macOS native GUI), and C/C++/Rust (for Linux desktop GUIs) leads to code duplication, protocol divergence, and maintenance overhead whenever Redis stream schemas, signing algorithms, or ACL selectors evolve.

## Decision
Implement a single, authoritative core client library in Go (`pkg/libbp`) and export it via a C-compatible shared library (`cmd/libbp-c`):
1. **Pure Go Engine (`pkg/libbp`)**: Handles RESP stream decoding, Valkey connection lifecycles, Ed25519 signing, payload serialization, and safety guards.
2. **C-ABI Export (`cmd/libbp-c`)**: Built using `go build -buildmode=c-shared`, emitting `libbp.dylib` (macOS), `libbp.so` (Linux), and `libbp.h`.
3. **Cross-Platform GUI Linkage**:
   - **macOS GUI (Swift/SwiftUI)**: Bridges to `libbp.dylib` via Swift C-interop/modulemap and wraps `bp_subscribe_live` in Swift `AsyncStream`.
   - **Linux GUI (GTK4/Libadwaita or Qt)**: Directly links `libbp.so` into C/C++/Rust application code.
   - **Go CLI (`cmd/bp`)**: Imports `pkg/libbp` natively as an internal module.

## Status
Accepted.

## Consequences
- Single source of truth for the Agent Backplane Protocol across CLI and graphical interfaces.
- Zero protocol drift between CLI tools and native desktop frontends.
- High performance, low memory footprint, and native Go concurrency handling stream subscriptions.
