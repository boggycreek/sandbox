# Agent Sandbox — AI Agent Knowledge Base

This directory provides progressive disclosure of architectural principles, engineering constraints, and development guidelines for this repository. It is designed for optimal consumption by autonomous coding agents.

## Knowledge Index
- [Development Workflow & PR Protocol](dev-workflow.md): Branching rules, PR requirements, conventional commits, quality gates checklist.
- [Architecture & Runtime](architecture.md): Podman container runtime, security perimeter, layered OCI hierarchy, local LLM gateway.
- [OCI Image Resolution & Tagging](image-resolution.md): 3-tier image resolution, well-known presets, local store discovery, remote OCI refs (ADR 00029).
- [Testing Guidelines & Coverage Gates](testing-guidelines.md): Strict >=90% test coverage enforcement, rootless Podman cleanup patterns, containerized smoke testing.
- [Diagnostic Doctor & Auto-Healing](doctor-diagnostics.md): \`sndbx agent doctor\` and \`sndbx infra doctor\` architecture, check categories, healing behavior.

## Core Directives for Autonomous Agents
1. **Never commit directly to \`main\`**: Create a descriptive feature branch and submit a PR via \`gh pr create\`.
2. **Quality Gates are non-negotiable**: Run \`make check\` before proposing or submitting any code changes; statement test coverage must remain >= 90.0%.
3. **Preserve Security Perimeter**: Agents operate in rootless Podman containers; never bypass Podman or access host secrets.
4. **License & Copyright**: Always maintain the Boggy Creek Software LLC MIT copyright header in all source files.
