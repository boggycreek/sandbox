# 00018. Quality Engineering, >90% Test Coverage, Linters, and SCA

## Context
A mission-critical agent sandbox platform that manages parallel container runtimes, executes cryptographic signing, routes network traffic, and isolates host environments must maintain strict software quality, security, and verification discipline. Regressions or unchecked security vulnerabilities in dependencies or shell code can undermine host isolation and cause fleet-wide failures.

## Decision
Enforce automated software quality engineering, high test coverage, strict linting, and continuous Software Composition Analysis (SCA):

1. **Test Coverage Threshold (>90%)**:
   - Unit tests are required for all Go modules in `pkg/` and `cmd/`.
   - `make test-coverage` executes tests with race detection (`-race`), generates atomic coverage profiles (`-covermode=atomic`), and enforces a strict `>= 90.0%` test coverage threshold across packages. Builds below this threshold fail.
2. **Comprehensive Linting**:
   - **Go**: Managed via `.golangci.yml` incorporating `govet`, `staticcheck`, `errcheck`, `revive`, `gocritic`, `misspell`, `nilerr`, `bodyclose`, `unparam`, and `errorlint`.
   - **Shell**: Validated with `shellcheck` for all scripts (`install.sh`, `dev-setup.sh`, entrypoints).
3. **Software Composition Analysis (SCA) & Security Scanning**:
   - **Dependency Vulnerability Scanning**: `govulncheck` scans all imported Go libraries and call graphs against the official Go vulnerability database.
   - **Static Security Analysis**: `gosec` scans AST for insecure operations, weak randomness, and file permission flaws.
4. **Automated Quality Gate (`make check`)**:
   - Running `make check` executes `lint` + `test-coverage` + `sca` as the unified gating criteria before merging code.

## Status
Accepted.

## Consequences
- Every new feature or protocol modification must include rigorous unit and integration tests.
- Static analysis catches potential security leaks and code quality regressions before container image deployment.
- High developer confidence across both macOS and Linux builds.
