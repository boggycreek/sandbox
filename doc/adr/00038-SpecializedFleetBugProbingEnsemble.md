---
adr: "00038"
title: "Specialized Fleet Bug Probing Ensemble"
topic: "Fleet Coordination & Testing Workflows"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - fleet
  - testing
  - fuzzer
  - bug-hunting
  - backplane
  - valkey
  - beads
executive_summary: "Establishes a three-tier collaborative agent topology (Fuzzer, Auditor, Verifier) inside isolated sandbox containers, coordinating via the Valkey backplane (bp) and tracking findings in local Gitea (bd)."
---

# 00038. Specialized Fleet Bug Probing Ensemble

## Context
Deploying autonomous AI agents to discover software defects, security vulnerabilities, and regression bugs within software projects requires both broad exploration and strict verification:
- A single monolithic agent attempting to read code, write test harnesses, execute edge-case fuzzing, and verify reproducible failures frequently suffers from context window saturation, tool confusion, and hallucinated defect reports.
- Dividing testing responsibilities across multiple specialized agents running in isolated container environments isolates failure blast radiuses and improves defect discovery efficiency.

The Agent Sandbox platform provides a high-performance cross-agent communication bus ([ADR 00010](00010-ValkeyPubSubMessagingAndAclIsolation.md), [ADR 00011](00011-Ed25519CryptographicMessageSigning.md)) and local forge issue tracking ([ADR 00028](00028-InContainerFleetTaskCoordinationViaLocalGiteaBackedBeads.md)).

## Decision (What)
We establish a canonical three-agent collaborative testing topology for probing containerized codebases:

```
               ┌──────────────────────────────────────────────┐
               │         Host GPU Inference Gateway           │
               └──────────────────────▲───────────────────────┘
                                      │
            ┌─────────────────────────┼─────────────────────────┐
            │                         │                         │
 ┌──────────▼───────────┐  ┌──────────▼───────────┐  ┌──────────▼───────────┐
 │   Container: pig-1   │  │   Container: pig-2   │  │   Container: pig-3   │
 │   Role: fuzzer       │  │   Role: auditor      │  │   Role: verifier     │
 │                      │  │                      │  │                      │
 │ Tools: bash, edit, rw│  │ Tools: read, grep, ls│  │ Tools: bash, read, rw│
 └──────────┬───────────┘  └──────────┬───────────┘  └──────────┬───────────┘
            │                         │                         │
            └────────────────►  Valkey Backplane  ◄─────────────┘
                               (bp say / bp tell)
```

1. **Role Decomposition**:
   - **`fuzzer` (Boundary & Crash Prober)**: Generates randomized inputs, property-based tests, and boundary assertions. Runs active bash execution to identify panics, crashes, unhandled errors, or unexpected process termination.
   - **`auditor` (Static Code & Logic Reviewer)**: Uses read-only search tools (`read`, `grep`, `find`, `ls`) to analyze ASTs, concurrency primitives, resource lifecycles, and cryptographic implementations without mutating code.
   - **`verifier` (Triage & Reproduction Specialist)**: Ingests anomaly signals from the fuzzer and auditor, strips environment-dependent noise, constructs minimal standalone reproducible test cases, and registers verified defects.

2. **Cross-Agent Signaling via Backplane (`bp`)**:
   - When the `fuzzer` encounters an unexpected exit, it broadcasts a finding alert:
     ```bash
     bp tell pig-verifier "Potential crash detected in pkg/auth/token.go: exit code 139 with input payload: $PAYLOAD"
     ```
   - When the `auditor` spots suspect code patterns, it signals the `fuzzer` to target the affected package:
     ```bash
     bp tell pig-fuzzer "Auditing identified unchecked slice indexing in pkg/net/parser.go line 42; probe with empty buffers"
     ```
   - When the `verifier` confirms a genuine bug with a clean reproduction script, it issues a signed public announcement via `bp say` and creates a tracked issue in local Gitea (`bd create`).

3. **Container Sandboxing Boundary**:
   - Each prober operates within its own dedicated rootless container and home volume.
   - Malicious inputs, catastrophic process crashes, or destructive code edits remain strictly contained within the test sandbox.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Minimizes hallucination by decoupling candidate generation (fuzzer/auditor) from verification (verifier).
- Preserves context window budgets by distributing inspection across focused agent roles.
- Creates transparent, auditable coordination logs over Valkey streams with cryptographic Ed25519 signatures.

### Negative / Trade-offs
- Requires concurrent inference scheduling or sequential slot polling across multiple agent instances on single-GPU workstations.
