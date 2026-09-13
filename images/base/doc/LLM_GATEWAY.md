# Local Model Gateway (`llm-gateway`) Reference

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

The **Agent Sandbox** provides host gateway routing so agents can access OpenAI-compatible local LLM inference engines (such as **Ollama**, **llama.cpp**, **vLLM**, or **LiteLLM**) running directly on the physical host machine.

---

## How It Works

- **DNS / Host Mapping**: Every sandbox container receives `--add-host=llm-gateway:host-gateway`, mapping the hostname `llm-gateway` to the host machine's gateway interface.
- **Port Access**: Local inference ports on the host (e.g. `11434` for Ollama, `8000` for vLLM/LiteLLM, `8080` for llama.cpp) are accessible from inside the container via `http://llm-gateway:<port>/v1`.

---

## Environment Variables & Configuration

When an agent is created with `--model-url`, `--model-name`, and `--model-key`, the sandbox runtime automatically injects the resolved gateway endpoint:

| Environment Variable | Description | Example |
|:---|:---|:---|
| `OPENAI_BASE_URL` | Base URL pointing to the model inference API | `http://llm-gateway:11434/v1` |
| `OPENAI_MODEL` | Target model name | `qwen2.5-coder:32b` |
| `OPENAI_API_KEY` | API key (or dummy token for local inference) | `ollama` or `dummy-key` |

---

## Testing Local Inference from Inside Container

You can verify inference connectivity using `curl`:

```bash
# Test model listing
curl -s "${OPENAI_BASE_URL}/models" | jq .

# Test chat completion
curl -s "${OPENAI_BASE_URL}/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENAI_API_KEY:-dummy}" \
  -d '{
    "model": "'"${OPENAI_MODEL:-default}"'",
    "messages": [{"role": "user", "content": "Write a hello world program in Go"}],
    "max_tokens": 100
  }' | jq .
```
