# 00029. Flexible Agent Image Resolution and Local Store Support

## Context
When an operator provisions a new agent using `sndbx agent create <name> as <target>`, the specified container image can originate from several distinct sources:
1. **Well-known presets**: Built-in harnesses provided by the platform (`base`, `opencode`, `claude`, `agy`), mapping canonically to `agent-sandbox-<preset>:latest`.
2. **Remote OCI Registry URLs**: Public or private container registries (`ghcr.io/org/repo:tag`, `quay.io/...`, `docker.io/...`).
3. **Local Podman Store Images**: Custom images crafted and built locally by developers on their workstations (e.g. `podman build -t my-coder:latest .`) without being pushed or published to any external registry.

Previously, `ResolveImage` only looked up well-known presets and otherwise passed the string through verbatim. In environments with strict unqualified-search rules, or when users omitted explicit `:latest` tags or registry qualifiers (e.g., `sndbx agent create coder as my-coder`), container creation could fail with registry resolution errors.

Furthermore, there is the question of how native well-known images built by the repository should be tagged.

## Decision
1. **Explicit Tagging Policy for Native Images**:
   - All native images built by `make build-images` or `sndbx repo build-images` are tagged with `:latest` (and during releases, version tags or git commit SHAs).
   - In local Podman storage, these images are accessible as `agent-sandbox-<preset>:latest` and `localhost/agent-sandbox-<preset>:latest`.

2. **Three-Tier Image Resolution Order**:
   When resolving an image input in `sndbx agent create <name> [as <target>]`:
   - **Tier 1: Well-Known Presets**: If `<target>` matches a recognized preset name (`base`, `opencode`, `claude`, `agy`), resolve to `agent-sandbox-<preset>:latest`. If `<target>` is empty, default to `agent-sandbox-base:latest`.
   - **Tier 2: Local Podman Store Discovery**: If `<target>` matches an image present in the host's local Podman storage:
     - Check if `<target>` or `localhost/<target>` exists in the local image store.
     - If untagged (no colon `:` present), append `:latest` and verify existence.
     - If present locally, use the canonical local image identifier (`localhost/<target>` or `<target>:latest`).
   - **Tier 3: Remote OCI Reference**: If the image is not a well-known preset and not currently present in local storage, assume it is an external OCI repository reference (e.g., `quay.io/user/image:v1` or `docker.io/...`), appending `:latest` if no tag or digest is specified.

3. **Preflight Verification**:
   - During `sndbx agent create` and `sndbx agent doctor`, verify image accessibility. For local images, confirm local store presence; for remote images, flag whether image pull is required upon `sndbx agent start`.

## Status
Accepted.

## Consequences
- **Developer Freedom**: Developers can quickly test custom harnesses created with `podman build -t custom-agent .` without having to configure local registry servers or push to external registries.
- **Predictable Behavior**: Untagged images default consistently to `:latest`.
- **Zero Configuration**: Resolves correctly even when `/etc/containers/registries.conf` disables unqualified search registries.
