---
adr: "00013"
title: "Per-Instance Network Isolation"
topic: "Network Isolation & Perimeter Defense"
theme: "THEME-NETWORKING"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - networking
  - netns
  - isolation
  - bridge
  - security
executive_summary: "Sandboxes run in isolated rootless network namespaces with independent bridge interfaces and dedicated localhost SSH port allocations."
---

# 00013. Per-Instance Network Isolation

## Context
If multiple agent containers share the host network stack, port collisions occur, localhost services exposed on the host become accessible to the containers, and agents can inspect or interfere with each other's network sockets.

## Decision (What)
Every agent container executes within an isolated, dedicated Linux network namespace (`netns`) created by rootless Podman.

The networking model mandates:
- **Rootless Bridge**: Containers attach to a dedicated user-space bridge (`sndbx-net`), providing DHCP and DNS name resolution (`sndbx-<name>`).
- **No Host Network Sharing**: Containers never use `--net=host`.
- **Dedicated Port Mapping**: Each agent is allocated a dedicated host port (defaulting to the `2222+` port range) bound strictly to `127.0.0.1` for SSH access. Port assignments are persisted in the agent's configuration descriptor.
- **Inter-Agent Isolation**: Direct container-to-container socket binding is prevented; communication must route through the authorized Valkey backplane or egress gateway.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Zero risk of port collisions across agents running identical servers (e.g. web apps on port 3000/8080).
- Prevents agents from accessing private host loopback services (databases, cloud credentials, debuggers).
- Predictable DNS routing within the private sandbox network.

### Negative / Trade-offs
- Requires managing port allocation tables in `sndbx` to prevent localhost collisions.
- Minor network latency introduced by rootless packet forwarding (pasta/slirp4netns).
