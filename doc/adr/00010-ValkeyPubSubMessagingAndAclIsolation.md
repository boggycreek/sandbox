---
adr: "00010"
title: "Valkey PubSub Messaging Bus and ACL Isolation"
topic: "Security, Backplane & Messaging"
theme: "THEME-SECURITY"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - messaging
  - valkey
  - acl
  - pubsub
  - security
executive_summary: "Inter-agent and telemetry communication utilizes a shared Valkey message bus governed by strict per-agent ACL rules and isolated channel prefixes."
---

# 00010. Valkey PubSub Messaging Bus and ACL Isolation

## Context
Multi-agent collaboration and host orchestration require a high-throughput, low-latency messaging backbone for events, task distribution, and heartbeats. A naive shared message bus without authorization allows rogue or compromised agents to snoop on fleet traffic, spoof messages, or flood the broker.

## Decision (What)
The messaging backbone is powered by an unprivileged, rootless Valkey instance (`sndbx-valkey`) running on the shared backplane network.

Security and channel isolation are strictly enforced via Valkey Access Control Lists (ACLs):
- **Channel Partitioning**: Each agent has read/write access restricted to its own private channel namespace (`agent:<name>:*`) and designated broadcast ingress channels (`fleet:broadcast`).
- **Command Restrictions**: Agent accounts are restricted to basic pub/sub commands (`PUBLISH`, `SUBSCRIBE`, `PING`) and barred from administrative commands (`CONFIG`, `FLUSHALL`, `KEYS`, `SHUTDOWN`).
- **Per-Agent Credentials**: Distinct usernames and high-entropy passwords are generated during `sndbx agent create` and injected into the container environment via `/home/agent/.config/sndbx/backplane.env`.
- **Administrative Privileges**: Only the host `sndbx` CLI and backplane coordinator possess administrative access.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Sub-millisecond pub/sub latency supporting dense multi-agent communication.
- Strict ACL rules prevent eavesdropping or unauthorized message injection across agent boundaries.
- Uses open-source Valkey without vendor lock-in or external cloud dependencies.

### Negative / Trade-offs
- Valkey does not provide built-in message persistence or store-and-forward queuing for offline agents out of the box (requires streams or external storage).
