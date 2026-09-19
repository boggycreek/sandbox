---
adr: "00011"
title: "Ed25519 Cryptographic Message Signing"
topic: "Security, Backplane & Messaging"
theme: "THEME-SECURITY"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - cryptography
  - ed25519
  - signing
  - non-repudiation
  - security
executive_summary: "All backplane events and commands require cryptographic Ed25519 signatures verified against the sending agent's public key to guarantee authenticity."
---

# 00011. Ed25519 Cryptographic Message Signing

## Context
While Valkey ACLs isolate channel access, they authenticate connections rather than individual message payloads. In complex multi-hop architectures or relayed communications, messages could theoretically be forged or altered in transit without cryptographic non-repudiation.

## Decision (What)
Every message transmitted across the Agent Sandbox backplane must carry a cryptographic digital signature generated using the Ed25519 signature scheme.

During `sndbx agent create <name>`, an Ed25519 private key is generated and stored securely in `/home/agent/.ssh/id_ed25519`, with the public key recorded in the agent descriptor and published to the fleet registry. Every message envelope published by `bp` or `bpd` includes:
1. `sender`: Identifier of the sending agent.
2. `timestamp`: Nanosecond-resolution timestamp to prevent replay attacks.
3. `payload`: Serialized message body.
4. `signature`: Base64-encoded Ed25519 signature covering sender, timestamp, and payload.

Receivers (both agents and host monitors) automatically verify the signature against the sender's public key before processing or executing message contents.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Guarantees message authenticity, integrity, and non-repudiation across the entire fleet.
- Replay attacks are mitigated by timestamp validation bounds.
- Reuses fast, secure, compact Ed25519 algorithms identical to standard SSH keypairs.

### Negative / Trade-offs
- Slight serialization and cryptographic verification overhead per message (negligible in Go/C, ~sub-millisecond).
- Requires distributing public keys or maintaining a public key trust store.
