---
adr: "00035"
title: "Host-Bridged Local LLM Inference Gateway"
topic: "Networking & Local Inference"
theme: "THEME-FLEET"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - inference
  - networking
  - host-gateway
  - llm
  - ollama
  - podman
  - fleet
executive_summary: "Resolves host-bound local LLM inference engines (Ollama, llama.cpp, vLLM, LiteLLM) from unprivileged rootless containers via Podman host-gateway resolution (--add-host=llm-gateway:host-gateway) and automatic configuration URL translation."
---

# 00035. Host-Bridged Local LLM Inference Gateway

## Context
Autonomous agents running in container sandboxes frequently query local LLM inference servers (e.g., Ollama, llama.cpp, vLLM, LiteLLM) running on the physical host machine to leverage hardware acceleration (GPUs/NPUs) without cloud subscription costs or external data egress.

While ADR 00022 established the contract for standardizing on an OpenAI-compatible proxy interface to encapsulate secrets, networking between rootless containers and host-bound services introduces distinct operational barriers:
- In rootless Podman, containers operate within isolated network namespaces where `localhost` and `127.0.0.1` resolve exclusively within the container itself.
- Hardcoding static bridge gateway IP addresses (such as `10.88.0.1`) is fragile across different Linux distributions, dynamic DHCP changes, multiple custom bridge networks, and virtualization layers (e.g., macOS Podman machine VMs).
- Manually configuring network routes or host firewall rules introduces substantial developer friction.

## Decision (What)
We establish automated, dynamic container-to-host network routing for all local inference workloads:

1. **Podman Host-Gateway Resolution**: Every agent container is started with `--add-host=llm-gateway:host-gateway`. In Podman, the special `host-gateway` keyword dynamically maps `llm-gateway` in `/etc/hosts` to the host's actual gateway IP address on the active bridge network.
2. **Transparent Endpoint Translation**: During agent provisioning and startup, `sndbx` inspects configured model URLs. Any endpoint defined as `localhost` or `127.0.0.1` (e.g., `http://localhost:11434/v1`) is automatically rewritten to `http://llm-gateway:<port>/v1` prior to injecting environment variables into the container.
3. **Host Service Binding**: Local inference engines running on the physical host are configured to bind beyond host loopback (listening on `0.0.0.0` or explicitly including the container bridge interface), permitting incoming traffic from unprivileged rootless sandboxes.
4. **Architectural Synergy**: This gateway mechanism complements ADR 00022: the agent interacts with standard OpenAI-compatible endpoints (`OPENAI_BASE_URL=http://llm-gateway:<port>/v1`), while the underlying network route remains resilient and fully automated across both Linux and macOS workstations.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Frictionless access to host GPU inference without manual IP discovery or network routing scripts.
- Consistent behavior across Linux physical workstations and macOS virtualized Podman machine environments.
- Eliminates hardcoded IP addresses from configuration profiles and scripts.
- Preserves standard developer ergonomics: users can specify `http://localhost:11434` without worrying about container networking boundaries.

### Negative / Trade-offs
- Host inference services must accept connections from the container bridge network rather than strictly binding to host loopback `127.0.0.1`.
