# 00019. Podman as Required Dev and Runtime Dependency

## Context
Running multi-agent sandboxes requires rootless container execution, unprivileged port mapping, named volume management, and daemonless process supervision. Supporting multiple container engines (e.g. Docker and Podman) introduces diverging daemon models, socket permission differences (rootless vs root-daemon), Compose implementations (`podman compose` vs `docker compose`), and test harness inconsistencies.

Podman provides a daemonless, natively rootless architecture that aligns directly with the Agent Sandbox security philosophy of non-root boundaries and zero host privilege escalation.

## Decision
**Podman is the required container runtime and development dependency** across both host runtime environments and contributor development setups:
1. `install.sh` enforces `podman` as a required host prerequisite during installation and halts if `podman` is not found on `PATH`.
2. `dev-setup.sh` verifies `podman` (and `podman-compose`) as required core dependencies for contributors.
3. Test harnesses (`test/harness/valkey.go`), CLI orchestrators (`sndbx`), and Compose orchestration target Podman rootless semantics as the canonical container engine.

## Status
Accepted.

## Consequences
- **Security & Consistency**: Daemonless, rootless container execution is guaranteed across macOS and Linux without requiring a privileged background daemon or root group membership.
- **Simplicity**: Eliminates fallback branches and conditional execution logic across scripts, CLI orchestration, and integration tests.
- **Clear Guidance**: User and contributor installation instructions provide explicit Podman setup instructions across macOS (`brew install podman`) and Linux package managers (`apt`, `dnf`, `pacman`).
