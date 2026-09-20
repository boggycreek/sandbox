---
adr: "00030"
title: "Open Source Attribution and Third-Party Licensing Notices"
topic: "Compliance & Governance"
theme: "THEME-GOVERNANCE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - open-source
  - attribution
  - licensing
  - compliance
  - notices
  - acknowledgements
  - third-party
executive_summary: "Establishes a clear boundary between human-centric open-source recognition (ACKNOWLEDGEMENTS.md) and formal legal license texts (NOTICES.md). Reinforces our zero-dependency standard library Go architecture while systematically preserving third-party copyright notices and disclaimers for all container runtime, infrastructure, and session tooling components."
---

# 00030. Open Source Attribution and Third-Party Licensing Notices

## Context

The **Agent Sandbox** platform relies on foundational open-source projects across its multi-tier runtime (Podman, Valkey, Gitea, PostgreSQL, SonarQube, Beads, Dolt, OpenSSH, tmux, Debian GNU/Linux).

While our native Go CLI binaries (`sndbx`, `bp`, `bpd`, `sonar-mcp`) and core libraries (`pkg/...`) are intentionally engineered with **zero external Go module dependencies** using the Go standard library, the platform orchestrates and distributes container environments with third-party software governed by various open-source licenses (MIT, BSD-3-Clause, Apache-2.0, PostgreSQL, LGPL-3.0, ISC).

To balance community gratitude and ethical open-source citizenship with formal legal compliance and license obligations, the project requires distinct, purposeful documentation artifacts.

---

## Decision

We establish two complementary, dedicated documentation artifacts at the repository root:

### 1. Human-Centric Attribution (`ACKNOWLEDGEMENTS.md`)
- **Audience**: Developers, contributors, users, and the open-source community.
- **Tone**: Appreciative, informative, and context-rich.
- **Content**:
  - Highlights our **zero-dependency** design philosophy powered by the Go standard library.
  - Explains the architectural role of each upstream project across process isolation (Podman, Conmon, crun, Debian), messaging (Valkey), forge hosting (Gitea), databases (PostgreSQL, Dolt), static analysis (SonarQube), issue tracking (Beads `bd`), session multiplexing (tmux), and IDE integrations (VS Code, JetBrains).
  - Provides canonical links to upstream project homepages, documentation, and maintainer organizations.

### 2. Formal Third-Party Legal Notices (`NOTICES.md`)
- **Audience**: Legal auditors, compliance teams, and downstream binary/container distributors.
- **Tone**: Formal, structured, and legally compliant.
- **Content**:
  - A structured inventory table detailing component names, upstream authors/organizations, SPDX license identifiers, and usage context.
  - Complete, verbatim copyright notices, permission grants, and warranty disclaimers required by upstream licenses (BSD, MIT, Apache 2.0, PostgreSQL, ISC).

---

## Consequences

### Positive
- **Clear Separation of Concerns**: Human-friendly narrative credits do not clutter legal compliance requirements, and vice versa.
- **Rigorous Legal Compliance**: Fully satisfies attribution and copyright preservation clauses required by redistributable open-source licenses.
- **Zero Runtime Overhead**: Documentation resides purely in source repositories and binary distributions without runtime memory or latency penalties.

### Neutral / Trade-offs
- Introducing new runtime or container tooling requires updating both files during development.
