# 00009. Ed25519 Message Signing for Cryptographic Provenance

## Context
Because Valkey allows any authenticated peer to append to `<peer>:inbox`, an agent could theoretically spoof the `from` field in an inbox message payload to pretend it originated from another agent or the human operator. Relying on transport metadata alone is insufficient to prove message authenticity.

## Decision
Implement cryptographic message provenance using asymmetric Ed25519 digital signatures:
1. **Key Generation**: Each agent and the human operator generate a dedicated Ed25519 keypair (`~/.ssh/agent-sandbox-signing` on host; injected via `BP_SIGNING_KEY_PEM` into containers).
2. **Identity Attestation**: The operator publishes the public key to `identity:<id>` in Valkey.
3. **Payload Signing**: Every message published via `bp say`, `bp tell`, or `bp reply` is signed over `timestamp + sender + recipient + content` using `crypto/ed25519`.
4. **Signature Verification**: Receiving agents verify the signature against the sender's public key before accepting the message.

## Status
Accepted.

## Consequences
- Message forging across identities is cryptographically impossible.
- Agents can definitively verify whether an instruction truly came from the operator or an appointed liaison.
- Adds minimal computational overhead (~microseconds per signature in Go).
