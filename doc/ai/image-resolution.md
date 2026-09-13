---
name: agent-sandbox-image-resolution
description: >-
  Explains how agent container images are resolved across well-known presets, local Podman image storage, and remote OCI registries.
  Use when creating agents, configuring custom container harnesses, or debugging image resolution.
---

# OCI Image Resolution & Tagging (ADR 00029)

## Image Resolution Tiers
When creating agents (`sndbx agent create <name> [as <target>]`):
1. **Tier 1 (Well-Known Presets)**:
   - `base` -> `agent-sandbox-base:latest` (neutral base harness for custom or manual operator build-up)
   - `opencode` -> `agent-sandbox-opencode:latest`
   - `claude` -> `agent-sandbox-claude:latest`
   - `agy` -> `agent-sandbox-agy:latest`
   *Note*: `base` is an explicit preset target, not an implicit fallback. When `<target>` cannot be resolved, `sndbx agent create` should emit a warning/error.
2. **Tier 2 (Local Podman Store Discovery)**:
   - Evaluates whether custom images built on the workstation exist locally before querying remote registries.
   - Probes: `<target>`, `<target>:latest`, `localhost/<target>`, `localhost/<target>:latest`.
   - Uses matched local store tag without requiring remote registry publishing.
3. **Tier 3 (Remote OCI Reference)**:
   - External registry references (`docker.io/...`, `ghcr.io/...`, `quay.io/...`).
   - Standardizes untagged inputs to `:latest`.

## Building Native Images
- Build CLI binaries: `make build-cli`
- Build base image: `make build-image-base`
- Build all images: `make build-images`
- Remote base images in Dockerfiles must use fully qualified references (e.g. `docker.io/library/golang:1.25-bookworm`) to prevent unqualified-search errors on strict Podman hosts.

Further reading:
- [ADR 00029 — Flexible Agent Image Resolution and Local Store Support](../adr/00029-FlexibleAgentImageResolutionAndLocalStoreSupport.md)
