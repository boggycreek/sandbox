---
adr: "00026"
title: "Deno as Standard JavaScript Runtime for OCI Container Agents"
topic: "Container Runtime & Storage"
theme: "THEME-RUNTIME"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - runtime
  - deno
  - javascript
  - typescript
  - security
  - oci
executive_summary: "Deno 2.x replaces Node.js as the standard JavaScript/TypeScript runtime in all agent OCI images, providing native TypeScript execution, granular capability sandboxing, and a leaner container footprint without compromising npm ecosystem compatibility."
---

# 00026. Deno as Standard JavaScript Runtime for OCI Container Agents

## Context
Agent containers require a JavaScript/TypeScript runtime for:
- Executing AI agent harnesses (Claude Code, opencode, custom tool runners)
- Running npm-distributed AI SDK tooling (openai, LangChain, etc.)
- Evaluating dynamic agent-generated code and tool call payloads

Previously, Node.js 22.x LTS was installed in the base OCI image via the NodeSource APT repository. This provided maximum npm compatibility but offered no runtime-level security controls — any agent process or compromised tool call inherited full container access to the filesystem, environment variables, and network with no enforcement boundary short of the OCI container perimeter itself.

A data-driven evaluation benchmarked Node.js v24.14.0 (via tsx) against Deno 2.9.7 across five test dimensions:

| Metric | Node.js | Deno | Result |
|---|---|---|---|
| Throughput (50k iters) | 2.480s | 1.999s | Deno **+20%** faster |
| Initial RSS | 76.65 MB | 50.34 MB | Deno **−34%** |
| Peak RSS | 83.40 MB | 75.14 MB | Deno **−10%** |
| Memory growth (500k iters) | +93 MB | +90 MB | Statistical tie |
| Cold start (avg 8 runs) | 182ms | 23ms | Deno **8×** faster |
| Concurrent peak RSS (1000 tasks) | 82.33 MB | 58.47 MB | Deno **−29%** |
| Security boundary enforcement | None | 3/3 PASS | Deno categorical win |
| OpenAI SDK (npm:) compat | ✅ | ✅ | Tie |

The decisive factor was the security boundary test: Deno's `--allow-*` permission flags enforced at the runtime level blocked unauthorized `fetch()`, `Deno.env`, and subprocess spawning independently of the OCI container perimeter. Node.js has no equivalent mechanism. For agent workloads that execute LLM-generated tool call payloads — which are subject to prompt injection — this runtime-level enforcement provides meaningful defense-in-depth that Node.js cannot offer.

Both runtimes exhibited equivalent linear V8 heap growth at 500k iterations (+18 MB/100k), confirming that previously observed "Node.js stability advantage" at 50k iterations did not hold at production scale. Graceful process restart cycles are required for multi-day agent uptime regardless of runtime choice.

## Decision (What)

Node.js is removed from the base OCI image. Deno 2.x is installed as the standard JavaScript/TypeScript runtime across the entire agent image hierarchy:

1. **Base image (`images/base/Dockerfile`)**:
   - Remove NodeSource APT setup and `nodejs` package installation.
   - Remove `NPM_CONFIG_PREFIX` and `~/.npm-global` scaffolding from `ENV`/`PATH`.
   - Install Deno via the official install script into `/usr/local/bin/deno` (root-owned, available system-wide).
   - Set `DENO_DIR=/home/agent/.cache/deno` for persistent npm package caching across container recreations (backed by the agent home volume).

2. **Agent derivative images**:
   - Replace `npm install -g <tool>` with `deno install --global npm:<tool>` where Deno-native installs are available, or with direct binary installs from official releases.
   - Claude Code: installed via `npm install -g @anthropic-ai/claude-code` using Deno's built-in npm compatibility (`deno install --allow-scripts=npm:@anthropic-ai/claude-code npm:@anthropic-ai/claude-code`). Node.js is not required — Deno's npm compatibility layer handles the package.
   - opencode: installed via official binary release download (Go static binary; no Node.js dependency).
   - agy: no npm dependencies; binary-only install.

3. **Runtime permission policy**:
   - Agent entrypoints that invoke Deno scripts should use explicit `--allow-*` allowlists rather than `--allow-all`.
   - Recommended baseline: `--allow-net=<llm-gateway,valkey,gitea>`, `--allow-read=/home/agent`, `--allow-write=/home/agent`, `--allow-env=<explicit list>`, `--deny-run`.
   - The `bpd` Go binary is unaffected; it does not use the JS runtime.

4. **ADR 00006 update**: `sndbx-base` description updated to reflect Deno replacing Node.js LTS.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Runtime-level capability enforcement provides defense-in-depth against prompt injection attacks that attempt unauthorized network exfiltration or environment variable access.
- Native TypeScript execution eliminates `tsx`/`tsc` transpilation from agent image build steps.
- ~25–30 MB lower baseline RSS per container; 8× faster cold start on process restart.
- No `node_modules` directories in OCI layers; Deno caches packages in `$DENO_DIR` on the persistent home volume.
- `deno cache` pre-warms packages at image build time, eliminating first-run network round-trips.

### Negative / Trade-offs
- Native C++ Node.js add-ons (`sharp`, `canvas`, legacy SQLite bindings) are incompatible. These are not present in the current agent toolchain; alternatives exist if needed.
- Enterprise APM agents (Datadog Node tracer, New Relic) require Deno-compatible OpenTelemetry SDK substitutes. OpenTelemetry SDK for JS works under Deno.
- Agent harnesses that shell out to `node` or `npx` directly will need migration to `deno run npm:<pkg>` or static binary equivalents.
- Both runtimes exhibit equivalent linear V8 heap growth at sustained load (~18 MB/100k iterations). Graceful restart cycles must be implemented in `bpd` for multi-day agent processes; this is not a Deno-specific requirement.
