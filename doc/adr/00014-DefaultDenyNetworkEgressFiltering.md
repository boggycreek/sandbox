---
adr: "00014"
title: "Default-Deny Network Egress Filtering"
topic: "Network Isolation & Perimeter Defense"
theme: "THEME-NETWORKING"
status: "proposed"
version: "v0.1.0-alpha"
as_built: false
tags:
  - networking
  - egress
  - security
  - sidecar
  - nftables
  - default-deny
executive_summary: "Outbound container network traffic is restricted by default via an egress sidecar filter, permitting only approved LLM API endpoints and package repositories."
---

# 00014. Default-Deny Network Egress Filtering

## Context
Autonomous agents executing untrusted code or processing untrusted inputs present data exfiltration and command-and-control (C2) risks. An unconstrained network connection allows an agent or compromised dependency to exfiltrate private code or environment secrets.

## Decision (What)
Sandboxes enforce a **default-deny outbound network egress policy** managed through a dedicated egress sidecar filter container (`agent-sandbox-egress:latest` built from `images/egress-filter`).

All outbound packets originating from the sandbox network namespace pass through a packet inspection and filtering layer:
1. **Default Deny**: All outbound connections are rejected by default.
2. **Strict Domain/IP Allowlist**: Outbound traffic is permitted solely to explicitly allowlisted endpoints:
   - Approved LLM API endpoints (Anthropic, OpenAI, local inference gateway).
   - Essential package managers and language mirrors (PyPI, npm, crates.io, apt repositories).
   - The local Gitea instance and Valkey backplane.
3. **Audit Logging**: Any rejected outbound connection attempt is logged to the backplane audit stream.

## Status
Proposed (Sidecar image `images/egress-filter` and runtime orchestration in `pkg/runtime` are built; automatic sidecar attachment during `agent start` is planned).

## Consequences
### Positive
- Strong defense-in-depth against prompt injection, malicious supply-chain packages, and data exfiltration.
- Comprehensive audit trail of all external connection attempts.
- Compliance with enterprise zero-trust security postures.

### Negative / Trade-offs
- Developing applications that consume third-party APIs requires explicitly adding domains to the agent's egress allowlist.
- DNS-based allowlisting requires active DNS proxying or IP resolution synchronization.
