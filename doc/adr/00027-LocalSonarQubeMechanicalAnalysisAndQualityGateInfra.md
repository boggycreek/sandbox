---
adr: "00027"
title: "Local SonarQube Mechanical Analysis and Quality Gate Infrastructure"
topic: "Operations & Observability"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - sonarqube
  - mechanical-analysis
  - quality-gates
  - static-analysis
  - mcp
  - gitea
  - fleet
executive_summary: "Integrates SonarQube Server Community Edition into the shared sandbox infrastructure stack alongside Valkey and Gitea. Agents utilize SonarQube for multi-tiered mechanical analysis, automated defect discovery via a native Model Context Protocol (MCP) server, and strict Quality Gate verification across fleet repositories."
---

# 00027. Local SonarQube Mechanical Analysis and Quality Gate Infrastructure

## Context

Autonomous AI agents developing software within isolated containers require rigorous mechanical feedback loops to evaluate code health, security posture, and compliance with architectural invariants before changes reach peer agents or human operators. While compiler passes, basic linters (`go vet`, `staticcheck`), and unit test runners catch syntactical and localized failures, they do not provide holistic mechanical analysis across:
- Security vulnerabilities and taint flow analysis
- Deep code smells, cyclomatic/cognitive complexity regressions, and duplication hotspots
- Quality Gate threshold verification (enforcing zero new blocker bugs and >= 80% coverage on new code)
- Centralized fleet-wide quality telemetry and debt trends

SonarQube Server Community Edition provides an industry-standard static analysis and quality governance platform. Providing an air-gapped, host-local SonarQube server within the shared sandbox infrastructure gives agents a dedicated mechanical analysis oracle without requiring internet egress or third-party SaaS dependencies.

---

## Decision

We integrate SonarQube Server Community Edition as a first-class shared infrastructure service within the `agent-sandbox` platform:

1. **OCI Packaging & Runtime Base**:
   - Containerized via Podman using `docker.io/library/eclipse-temurin:21-jdk` as base image.
   - Satisfies SonarQube's embedded Elasticsearch 9.4.3 requirement for full Java Development Kit modules (`jdk.jdi`), which are omitted in headless JRE distributions.
   - Configured to execute as unprivileged user `sonarqube` (UID `1000`, GID `1000`), complying with rootless Podman constraints.

2. **Network Topology & Infrastructure Lifecycle**:
   - Deployed on the shared bridge network `agent-sandbox-infra` under hostname `sonarqube`.
   - Port `9000` mapped to host loopback `127.0.0.1:9000` for operator observability and container address `http://sonarqube:9000` for agent access.
   - Persistent Podman volumes manage state across lifecycles:
     - `agent-sandbox-sonarqube-data` (embedded database and search indexes)
     - `agent-sandbox-sonarqube-extensions` (community plugins)
     - `agent-sandbox-sonarqube-logs` (service execution logs)
   - Integrated into `sndbx infra up`, `sndbx infra stop`, and `sndbx doctor --infra` with automated health validation (`/api/system/status`).

3. **Multi-Tier Quality at Depth**:
   Mechanical analysis is structured in four synergistic layers:
   - **Tier 1 (Fast In-Container Verification)**: Language linters and unit tests run locally prior to staging.
   - **Tier 2 (Interactive MCP Analysis)**: Native Go Model Context Protocol server (`sonar-mcp`) provides direct tool calls (`sonar_issues`, `sonar_quality_gate`, `sonar_measures`) for agent self-healing.
   - **Tier 3 (Git Forge Quality Gates)**: Local Gitea git backplane (`http://gitea:3000`) verifies Quality Gate passage on pull requests before merging.
   - **Tier 4 (Fleet Observability)**: Operators and orchestrator monitor fleet-wide tech debt and security metrics via the SonarQube dashboard.

4. **Agent Context & Discovery**:
   - Agent containers automatically receive `SONAR_HOST_URL=http://sonarqube:9000` and `SONARQUBE_URL=http://sonarqube:9000`.
   - In-container documentation hub (`/home/agent/doc/SONARQUBE.md`, linked in `INDEX.md` and `ENVIRONMENT.md`) guides agent reasoning engines on how to query SonarQube tools during development.

---

## Consequences

### Positive
- **Deterministic Quality Verification**: Agents receive objective mechanical feedback on bug counts, security flaws, and coverage before committing or submitting PRs.
- **Air-Gapped & Local**: All analysis executes entirely on the local workstation; no external network access or proprietary cloud credentials required.
- **Native MCP Ergonomics**: Agents interact with SonarQube via standard MCP JSON-RPC tools rather than parsing complex raw API schemas.
- **Persistence & Reproducibility**: Analysis histories and quality metrics persist across agent container re-creations in dedicated Podman volumes.

### Negative / Trade-Offs
- **Resource Footprint**: SonarQube Community Edition and its embedded Elasticsearch engine require approximately 1.5GB to 2GB of host RAM when operational.
- **Cold-Start Latency**: SonarQube initialization and rule registration takes 30-45 seconds upon initial container start. `sndbx doctor` accounts for this with asynchronous startup reporting.
