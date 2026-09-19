---
adr: "00025"
title: "Quality Gates and Coverage Enforcement"
topic: "Operations, Diagnostics & Quality"
theme: "THEME-OPERATIONS"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - quality
  - testing
  - coverage
  - linting
  - gates
executive_summary: "Enforces continuous quality gates requiring >=90% statement test coverage (make test-coverage), static analysis (golangci-lint), and isolated ephemeral integration test fixtures."
---

# 00025. Quality Gates and Coverage Enforcement

## Context
Agent Sandbox orchestrates security boundaries, rootless namespaces, and autonomous AI agents. Flaws in credential handling, permission checks, or network filtering can introduce severe system vulnerabilities. A lax testing culture risks regressions in core infrastructure.

## Decision (What)
The codebase enforces strict, automated engineering quality gates across all development branches:

1. **Mandatory Test Coverage**: Statement test coverage across the Go monorepo must remain **$\ge$90.0%**. The build pipeline enforces this via `make test-coverage`; any commit dropping below 90.0% fails CI immediately.
2. **Static Code Analysis**: Strict `golangci-lint` rules enforce formatting, error checking, and security best practices.
3. **Ephemeral Integration Testing**: Integration tests execute against isolated, ephemeral rootless Podman fixtures (dynamic ports, isolated Valkey instances, temporary directories) ensuring tests run without side effects on developer workstations.
4. **Pre-Commit Verification**: No architectural changes or feature PRs are merged without passing full coverage and static analysis gates.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- High confidence in core runtime stability and security boundary enforcement.
- Ephemeral test harnesses prevent test flakiness and workstation pollution.
- Continuous coverage enforcement prevents technical debt accumulation.

### Negative / Trade-offs
- Maintaining $\ge$90% coverage requires comprehensive unit testing for all CLI error paths and edge cases.
