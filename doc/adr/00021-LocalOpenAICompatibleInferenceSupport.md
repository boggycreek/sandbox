# 00021. Local OpenAI-Compatible Inference Support and Host Gateway Routing

## Context
When deploying AI coding agents in sandbox containers, teams frequently utilize local, on-premise, or self-hosted Large Language Model inference endpoints (such as Ollama, llama.cpp, LocalAI, vLLM, LiteLLM, or custom inference routers).

Running agents against local inference servers introduces two primary challenges:
1. **Container-to-Host Network Gateway Addressing**: Inside containerized network bridges (Podman/Docker), referencing `localhost` or `127.0.0.1` refers to the container itself rather than the physical host running the inference server. Without automatic translation, operators must determine the internal bridge gateway IP manually.
2. **Configuration Fragmentation Across Agent CLIs**: Different agent runners (OpenCode, Claude, Antigravity, Aider) expect model configurations in varying formats (JSON configs, environment variables like `OPENAI_BASE_URL`, `MODEL_NAME`, or `--model` flags).

## Decision
The Agent Sandbox implements unified, declarative support for OpenAI-compatible local inference servers:

1. **Declarative Agent Descriptor Attributes**:
   - `AgentConfig` in `pkg/config/agent.go` includes `model_url`, `model_name`, and `model_api_key`.
   - `sndbx agent create` accepts `--model-url <url>`, `--model-name <name>`, and `--model-key <key>`, with automatic fallback to host environment variables (`OPENAI_BASE_URL`, `OPENAI_MODEL`, `OPENAI_API_KEY`).
   - If `--model-url` is specified without `--model-name`, agent runners auto-detect or default to their configured models.

2. **Well-Known Host Gateway Address Translation (`llm-gateway`)**:
   - In `pkg/runtime/podman.go`, containers are launched with `--add-host=llm-gateway:host-gateway`.
   - If `model_url` references `localhost` or `127.0.0.1` (e.g. `http://localhost:11434/v1`), the runtime automatically translates the URL to `http://llm-gateway:11434/v1` when constructing container arguments. This creates a clean, predictable, and platform-agnostic hostname across all container runners.

3. **Standard Container Environment Injection**:
   - The runtime injects the standard OpenAI variables:
     - `OPENAI_BASE_URL` and `OPENAI_API_BASE`
     - `MODEL_NAME`, `OPENAI_MODEL`, and `LLM_MODEL`
     - `OPENAI_API_KEY` (defaulting to `local-openai-key` if omitted to satisfy clients requiring authorization header presence).

4. **Derivative OCI Image Runtime Adaptation (e.g. OpenCode)**:
   - Derivative image entrypoints (such as `opencode-entrypoint.sh`) read `OPENAI_BASE_URL` and `MODEL_NAME` at container startup and dynamically configure `/home/agent/.config/opencode/config.json` with the OpenAI provider configuration.

## Status
Accepted.

## Consequences
- **Zero-Friction Local LLM Execution**: Developers can run Ollama or llama.cpp on the host and launch agents pointing to `http://localhost:11434/v1` without manual network configuration.
- **Provider Portability**: The same sandbox container architecture seamlessly supports cloud providers (OpenAI, Anthropic) and air-gapped local clusters.
- **Persistent LLM Binding**: Agent configurations remember their assigned model endpoint across container recreations.
