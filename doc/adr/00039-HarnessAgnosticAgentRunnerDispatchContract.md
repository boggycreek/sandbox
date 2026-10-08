---
adr: "00039"
title: "Harness-Agnostic Agent Runner Dispatch Contract"
topic: "In-Container Agent Execution & Supervision"
theme: "THEME-LIFECYCLE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - bpd
  - agent-runner
  - dispatch
  - oci
  - autonomy
  - backplane
executive_summary: "Decouples bpd from harness-specific CLI invocation by establishing a canonical in-container /usr/local/bin/agent-runner dispatch contract across derivative agent images."
---

# 00039. Harness-Agnostic Agent Runner Dispatch Contract

## Context
The backplane daemon ([`bpd`](00008-BackplaneDaemonAsPrimaryEntrypoint.md)) is the in-container process responsible for polling an agent's inbox stream on the Valkey backplane, batching incoming messages, and executing an autonomous agent turn.

Previously, `bpd` hardcoded Claude-specific execution mechanics:
- Defaulted `BPD_RUNNER_CMD` to `claude -p`.
- Injected Claude-specific CLI flags (`--session-id <uuid>` and `--resume <uuid>`) into process arguments.
- Expected `claude` to exist on `$PATH`.

When running derivative agent images such as PiG ([ADR 00036](00036-PiGDerivativeAgentOciImage.md), [ADR 00038](00038-SpecializedFleetBugProbingEnsemble.md)), OpenCode, or Agy, `bpd` failed immediately with `exec: "claude": executable file not found in $PATH`. Consequently, derivative agents could only be driven via interactive `tmux` sessions and could not execute autonomous turns when messaged over the Valkey backplane.

## Decision (What)
We establish a uniform, engine-agnostic dispatch contract between `bpd` and derivative agent containers:

1. **Standard In-Container Dispatch Executable (`/usr/local/bin/agent-runner`)**:
   - `bpd` defaults to executing `/usr/local/bin/agent-runner` if present in the container filesystem.
   - If `/usr/local/bin/agent-runner` is absent and `BPD_RUNNER_CMD` is unset, `bpd` gracefully falls back to `claude -p` for legacy compatibility.
   - Explicit `BPD_RUNNER_CMD` environment variable configuration retains highest precedence.

2. **The Dispatch Contract**:
   - **Input Stream (`stdin`)**: `bpd` streams the drained, formatted backplane message batch to `stdin`.
   - **Environment Context**: `bpd` exports `AGENT_SESSION_ID`, `AGENT_NAME`, and standard backplane environment variables into the runner process.
   - **Arguments (`$@`)**: `bpd` passes session lifecycle flags (`--session-id <uuid>` or `--resume <uuid>`) to allow stateful session continuation when supported by the underlying harness.
   - **Exit Status**: Exit code `0` signals successful completion of the turn; non-zero exit codes signal failure, triggering `bpd` retry and failure accounting.

3. **Per-Harness Derivative Specialization**:
   - Each derivative agent image provides its own `/usr/local/bin/agent-runner` tailored to its specific CLI syntax:
     - **PiG (`images/agents/pig`)**: Consumes prompt from `stdin` and executes `pig --piglet "${PIGLET_FILE}" -p "${PROMPT}"`.
     - **Claude (`images/agents/claude`)**: Executes `claude -p "$@"`.
     - **OpenCode (`images/agents/opencode`)**: Executes `opencode run "$@"`.
     - **Agy (`images/agents/agy`)**: Executes `agy -p "$@"`.
     - **Base (`images/agent-base`)**: Provides a dynamic fallback script probing available CLIs on `$PATH`.

4. **Background Daemon Supervision in Derivative Entrypoints**:
   - All derivative entrypoints (including `images/agents/pig/pig-entrypoint.sh`) supervise `bpd` in the background when `BP_HOST` is configured, ensuring agents react to `bp say` and `bp tell` without human terminal interaction.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Completely decouples `bpd` Go code from specific AI coding agent CLIs.
- Enables true autonomous, multi-agent fleet operations where agents wake up on incoming backplane messages across any supported harness (PiG, Claude, OpenCode, Agy).
- Retains backward compatibility for existing Claude-based setups.
- Centralizes engine-specific CLI flag quirks within the respective derivative image layers.

### Negative / Trade-offs
- Derivative images must include or synthesize their `/usr/local/bin/agent-runner` wrapper script.
