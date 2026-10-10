---
adr: "00011"
title: "Ed25519 Cryptographic Message Signing"
topic: "Security, Backplane & Messaging"
theme: "THEME-SECURITY"
status: "superseded"
superseded_by: "00042"
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
Every message published to the backplane carries an Ed25519 digital signature:

- **Key Generation & Injection**: During `sndbx agent create`, an Ed25519 keypair is generated. The private key is injected into the container environment via `BP_SIGNING_KEY_PEM` (or config file), while the public key is registered in Valkey under `identity:<agent>`.
- **Canonical Envelope & Signature**: The signature is computed over a canonical payload (`<timestamp>\n<sender>\n<destination>\n<reply_to>\n<content>\n<blob_path>`), guaranteeing authenticity, tamper evidence, and non-repudiation.
- **Automatic Verification**: `libbp` and `bp` automatically verify inbound signatures against the sender's public key before processing or displaying messages.

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
