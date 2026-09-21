---
adr: "00031"
title: "Automated Software Bill of Materials (SBOM) Generation and Release Publishing"
topic: "Supply Chain & Operations"
theme: "THEME-OPERATIONS"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - sbom
  - spdx
  - cyclonedx
  - syft
  - release
  - supply-chain
  - security
  - compliance
executive_summary: "Automates the generation and distribution of machine-readable Software Bill of Materials (SBOM) across release pipelines and local build targets. Leverages Syft to produce both SPDX and CycloneDX JSON formats for source repositories and compiled release archives, publishing them directly as release assets."
---

# 00031. Automated Software Bill of Materials (SBOM) Generation and Release Publishing

## Context

In modern enterprise and open-source software supply chain security (e.g. Executive Order 14028, NTIA minimum elements, OpenSSF guidelines), machine-readable Software Bill of Materials (SBOM) are required for automated vulnerability tracking, dependency auditing, and provenance attestation.

While human-readable attribution ([ADR 00030](00030-OpenSourceAttributionAndThirdPartyLicensingNotices.md)) provides developer and legal context, automated security scanners and package inventory managers require structured, machine-parseable data standards.

The ecosystem has converged on two primary standard formats:
1. **SPDX (Software Package Data Exchange)**: ISO/IEC 5962:2021 international standard.
2. **CycloneDX**: OWASP flagship standard optimized for application security and software supply chain analysis.

Agent Sandbox requires automated, repeatable generation and publishing of both formats without introducing manual toil or brittle release steps.

---

## Decision

We establish automated SBOM generation integrated into our release workflows and developer Makefile:

### 1. Tooling Standard: Syft
- We standardize on **[Syft](https://github.com/anchore/syft)** (Anchore) as the official SBOM generation engine for Agent Sandbox.
- Syft provides robust, multi-ecosystem package identification (Go modules, Linux OS packages, C libraries, binaries, container images) and native dual-format output for SPDX JSON and CycloneDX JSON.

### 2. Automated CI/CD Release Pipeline (`.github/workflows/release.yml`)
- During automated GitHub Releases:
  1. Syft scans the source tree to generate repository-level source SBOMs:
     - `agent-sandbox-source.spdx.json`
     - `agent-sandbox-source.cyclonedx.json`
  2. Syft scans each compiled binary release archive (`sandbox_${TAG}_${OS}_${ARCH}.tar.gz`) to produce binary package SBOMs:
     - `sandbox_${TAG}_${OS}_${ARCH}.spdx.json`
     - `sandbox_${TAG}_${OS}_${ARCH}.cyclonedx.json`
  3. All generated SBOM JSON files are included in the cryptographic SHA256 checksum manifest (`checksums.txt`) and uploaded directly to GitHub Releases alongside native executables.

### 3. Local Developer Target (`Makefile`)
- The repository provides a `make sbom` target:
  - Outputs `dist/sbom/agent-sandbox.spdx.json` and `dist/sbom/agent-sandbox.cyclonedx.json`.
  - Audits local repository and compiled artifacts on demand.
- `setup.sh` inspects the local presence of `syft` and provides installation guidance.

---

## Consequences

### Positive
- **Dual Standard Support**: Produces both SPDX (ISO standard) and CycloneDX (OWASP standard) formats, satisfying diverse enterprise compliance requirements.
- **Automated Supply Chain Transparency**: Zero manual effort required during release cycles; every GitHub Release automatically ships with verifiable SBOMs.
- **Cryptographic Integrity**: All SBOM artifacts are included in release SHA256 checksum manifests.

### Neutral / Trade-offs
- Adds a lightweight Syft step to CI/CD release workflow execution.
