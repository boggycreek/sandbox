# 00007. Agent Backplane Messaging Bus on Valkey Streams

## Context
Multi-agent collaboration requires cross-agent messaging for broadcasting announcements, delegating tasks, sharing intermediate discoveries, and receiving instructions from the human operator. Standard HTTP webhooks or point-to-point sockets require complex service discovery, port mapping, and retry handling that fail when agents restart or are idle.

## Decision
Use Valkey (Redis-compatible server) as a shared message backplane running in the `agent-sandbox-infra` network, leveraging Redis Streams:
1. **Broadcast Log (`<id>:out`)**: Append-only stream for agent broadcast messages (`bp say`).
2. **Point-to-Point Inbox (`<id>:inbox`)**: Append-only stream for direct agent messages (`bp tell`, `bp reply`).
3. **Citation Counter (`<id>:seq`)**: Monotonic message sequence counter providing human-friendly citation IDs (`<id>#14`).
4. **Read Cursor (`<id>:cursor`)**: Persisted stream position for each identity across `bp recv` invocations.
5. **Presence & Profile (`<id>:status`, `<id>:finger`)**: Ephemeral (300s TTL) presence updates and static capability profiles.
6. **Reactive Turn Wakeups**: Agents use session-scoped cron tasks to periodically execute `bp recv` and `bp human`, reacting to new messages without running continuous busy-loops.

## Status
Accepted.

## Consequences
- High-throughput, low-latency, persistent message bus with decoupled sender/receiver lifecycles.
- Message history survives container restarts and is pruned by a bounded retention sweep daemon.
