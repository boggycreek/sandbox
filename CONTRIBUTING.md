# Contributing to Agent Sandbox

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

Thank you for contributing to the **Agent Sandbox** platform. This guide outlines the development standards, contribution protocols, and repository task tracking tools.

---

## 1. Development Prerequisites & Setup

The sandbox repository is a pure **Go monorepo** with rootless **Podman** container dependencies:

1. **Go 1.24+**: Ensure standard Go toolchain is installed.
2. **Podman 5.0+**: Configured for rootless user operation (`subuid`/`subgid` allocated).
3. **Beads CLI (`bd`)**: Version 1.3.0+ for repository task tracking (`brew install beads` or download from [gastownhall/beads releases](https://github.com/gastownhall/beads/releases)).
4. **Environment Setup**: Run `./dev-setup.sh` to verify system dependencies, linters (`golangci-lint`, `shellcheck`), and PATH configurations.

---

## 2. Task Tracking for Contributors: Beads (`bd`)

We use **Beads (`bd`)** — a distributed, Git- and Dolt-backed graph issue tracker — to manage backlogs, epics, features, and bug fixes for the `agent-sandbox` codebase.

> [!IMPORTANT]
> **Two-Plane Architectural Separation: Platform vs. Fleet**
> 
> Please note the strict architectural distinction between the two separate applications of Beads:
> - **Operator / Contributor Plane (This Repository)**: Used on the host workstation by human developers and AI coding agents working *on* the `agent-sandbox` platform. Issues use the prefix `sndbx-*` and synchronize with the upstream GitHub repository (`origin` / `boggycreek/agent-sandbox.git`).
> - **Fleet / Workload Plane (In-Infrastructure)**: Autonomous agents executing *inside* running sandbox containers use a completely separate, air-gapped Beads instance backed by the local Gitea server (`http://gitea:3000/fleet/tasks.git`).
> 
> These two issue trackers share **zero storage, zero network endpoints, and zero database records**. Do not confuse or merge them.

### Contributor Workflow with `bd`

1. **Check for Ready / Unblocked Tasks**:
   ```bash
   bd ready
   ```
   Surfaces open tasks that have no blocking dependencies.

2. **Inspect the Task Hierarchy**:
   ```bash
   bd list
   ```

3. **View Issue Details & Acceptance Criteria**:
   ```bash
   bd show sndbx-cpm.1
   ```

4. **Claim a Task**:
   ```bash
   bd update sndbx-cpm.1 --claim
   ```

5. **Create a New Feature, Bug, or Subtask**:
   ```bash
   # Create a child task under an existing epic
   bd create "Add support for custom registry mirrors" \
     -t feature -p P2 -l "oci,runtime" --parent sndbx-cpm \
     -d "Allow users to specify alternate mirror registries in ~/.config/sndbx/config.yaml."
   ```

6. **Close a Completed Task**:
   ```bash
   bd close sndbx-cpm.1 --reason "Implemented in commit abc1234; all unit tests passing"
   ```

7. **Synchronize with GitHub Remote**:
   ```bash
   bd sync
   ```
   Reconciles your local Dolt database with the upstream GitHub replica (`origin`).

---

## 3. Branching & Pull Request Protocols

1. **Branch Isolation**:
   - `main` is protected. Never commit directly to `main`.
   - Work in dedicated feature branches or isolated git worktrees (`feat/<topic>`, `fix/<topic>`, `docs/<topic>`).
   - If working concurrently with other agents or team members, **always use isolated git worktrees** (`git worktree add ...`) to avoid colliding with working tree edits.
2. **Quality Gates (Strictly Enforced)**:
   Every pull request must pass the automated monorepo quality gate:
   ```bash
   make check
   ```
   This executes:
   - `make format`: Go formatting verification.
   - `make lint`: `golangci-lint` and `shellcheck`.
   - `make test-coverage`: **Strict >= 90.0% statement test coverage** across all packages.
   - `make sca`: Static application security analysis.
3. **Conventional Commits**:
   Format commit messages following Conventional Commits (`feat: ...`, `fix: ...`, `docs: ...`, `test: ...`, `refactor: ...`).
4. **Architecture Decision Records (ADRs)**:
   Any significant architectural change, new infrastructure service, CLI command group, or protocol addition requires an ADR in `doc/adr/`.
