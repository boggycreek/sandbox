---
adr: "00010"
title: "Valkey Streams Messaging Bus and ACL Isolation"
topic: "Security, Backplane & Messaging"
theme: "THEME-SECURITY"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - messaging
  - valkey
  - streams
  - acl
  - security
executive_summary: "Inter-agent and telemetry communication utilizes a shared Valkey Streams message bus governed by strict per-agent ACL rules and isolated stream keys."
---

# 00010. Valkey Streams Messaging Bus and ACL Isolation

## Context
Multi-agent collaboration and host orchestration require a durable, high-throughput, low-latency messaging backbone for events, task distribution, and mailboxes. Transient pub/sub mechanisms drop messages when recipient containers are offline, pausing, or occupied. Furthermore, an unpartitioned message broker allows rogue or compromised agents to snoop on fleet traffic, spoof identity, or truncate mailboxes.

## Decision (What)
The messaging backbone is powered by an unprivileged Valkey instance running on the shared infrastructure network (`agent-sandbox-infra`).

Message transport and isolation adhere to the following architectural rules:
- **Durable Streams over Ephemeral Pub/Sub**: Inter-agent messaging utilizes append-only Valkey Streams (`XADD`, `XREAD`) rather than fire-and-forget pub/sub channels. This provides durable mailboxes (`<agent>:inbox`), public broadcasts (`<agent>:out`), and human operator interactions (`human:inbox`, `human:out`) that tolerate container restarts and scheduling delays.
- **Stream Key Partitioning & ACL Boundaries**: Each agent is granted full read/write access to its own stream namespace (`~<agent>:*`) and write-only append access to peer mailboxes (`~*:inbox`).
- **Command Restrictions & Guardrails**: Agent accounts are restricted to non-destructive stream commands (`XADD`, `XREAD`, `XRANGE`, `PING`) and barred from administrative commands (`CONFIG`, `FLUSHALL`, `KEYS`, `SHUTDOWN`). Client-side guards prevent agents from executing destructive truncation (`MAXLEN`, `MINID`) on peer inboxes.
- **Per-Agent Credentials**: Distinct usernames and high-entropy credentials are generated during agent provisioning and injected into the container environment.
- **Administrative Privileges**: Only host orchestrator processes possess full ACL and server administration privileges.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Append-only stream durability guarantees messages are preserved across agent container restarts and transient disconnects.
- Sub-millisecond latency supports dense multi-agent communication without external cloud brokers or daemons.
- Strict ACL rules prevent eavesdropping, mailbox truncation, or unauthorized message injection across agent boundaries.

### Negative / Trade-offs
- Stream memory retention must be managed via retention policies on outbound feeds to prevent unbounded memory growth over prolonged fleet operations.
