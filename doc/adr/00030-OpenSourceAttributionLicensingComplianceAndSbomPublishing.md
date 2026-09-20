---
adr: "00030"
title: "Open Source Attribution, Licensing Compliance, and SBOM Publishing Standards"
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
  - sbom
  - spdx
  - cyclonedx
  - notices
  - acknowledgements
executive_summary: "Establishes a comprehensive open-source attribution, legal licensing compliance, and automated Software Bill of Materials (SBOM) publishing standard. Differentiates human-centric recognition (ACKNOWLEDGEMENTS.md) from formal legal license texts (NOTICES.md), and automates SPDX/CycloneDX SBOM generation across CI/CD release pipelines and local build targets."
---

# 00030. Open Source Attribution, Licensing Compliance, and SBOM Publishing Standards

## Context

The **Agent Sandbox** platform orchestrates a multi-tier runtime composed of host management binaries (`sndbx`, `bp`, `bpd`, `sonar-mcp`), shared infrastructure services (Valkey, Gitea, PostgreSQL, SonarQube), rootless OCI container base images (Debian Bookworm, unprivileged OpenSSH, tmux), and distributed fleet coordination tooling (Beads `bd`, Dolt).

While the native Go codebase strictly adheres to a **zero-dependency** design philosophy using pure Go standard library, the broader execution environment relies on foundational open-source components, runtimes, and community platforms.

To ensure ethical open-source stewardship, transparent supply-chain governance, and legal license compliance, the project requires clear architectural boundaries for:
1. **Human-Centric Ecosystem Attribution**: Giving readable, prominent credit and technical context to upstream creators, maintainers, and foundations.
2. **Formal Third-Party Legal Notices**: Fulfilling mandatory copyright notice preservation and license disclaimer requirements (such as BSD, MIT, Apache 2.0, ISC, and PostgreSQL licenses).
3. **Machine-Readable Software Supply Chain Transparency**: Producing standardized, automated Software Bill of Materials (SBOM) artifacts in machine-readable formats (SPDX and CycloneDX) attached to release distributions.

---

## Decision

We adopt a three-tier open-source attribution and supply chain transparency architecture:

### 1. Human-Centric Attribution (`ACKNOWLEDGEMENTS.md`)
- Located at repository root.
- Designed for developers, contributors, and the broader open-source community.
- Highlights the zero-dependency standard library architecture of the native Go binaries.
- Highlights and links upstream projects categorized by domain:
  - *Process Isolation & Runtime*: Podman, Conmon, crun, Debian GNU/Linux
  - *Messaging & Infrastructure*: Valkey (Linux Foundation), Gitea, PostgreSQL, SonarQube Community Build
  - *Task Backlog & Distributed Database*: Beads (`bd`), Dolt (DoltHub)
  - *Session & Daemon Tooling*: OpenSSH, tmux, Git
  - *IDE Integrations*: VS Code / VSCodium, JetBrains

### 2. Formal Third-Party Legal Notices (`NOTICES.md`)
- Located at repository root.
- Designed for legal, compliance, and distribution auditing.
- Provides a comprehensive, structured component inventory mapping each dependency/runtime component to its author/copyright holder, SPDX license identifier, and usage context.
- Bundles complete redistributable license texts, copyright notices, and warranty disclaimers as required by respective upstream licenses.

### 3. Automated Software Bill of Materials (SBOM) Generation
- **Release CI/CD Automation (`.github/workflows/release.yml`)**:
  - Leverages **Syft** to systematically scan source code and packaged release archives (`.tar.gz`).
  - Automatically produces both **SPDX JSON** (`*.spdx.json`) and **CycloneDX JSON** (`*.cyclonedx.json`) formats.
  - Publishes SBOM artifacts directly to GitHub Releases alongside native static binaries and SHA256 checksums (`checksums.txt`).
- **Developer Local Target (`Makefile`)**:
  - Exposes `make sbom` to generate `dist/sbom/agent-sandbox.spdx.json` and `dist/sbom/agent-sandbox.cyclonedx.json` on local workstations.

---

## Consequences

### Positive
- **Community Respect & Transparency**: Upstream creators, maintainers, and foundations receive clear attribution and appreciation.
- **Strict Compliance Verification**: Fulfills legal copyright and redistribution obligations across all bundled and orchestrated components.
- **Enterprise Supply Chain Readiness**: Machine-readable SPDX and CycloneDX SBOMs enable automated vulnerability tracking, policy enforcement, and provenance attestation in enterprise environments.
- **Zero Runtime Overhead**: All attribution and SBOM mechanisms operate purely at build/release and documentation time without introducing binary bloat or runtime performance overhead.

### Neutral / Trade-offs
- Adding new foundational tools or container base dependencies requires updating `ACKNOWLEDGEMENTS.md` and `NOTICES.md` as part of repository quality gate workflows.
