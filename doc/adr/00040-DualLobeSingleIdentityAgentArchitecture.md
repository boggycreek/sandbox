---
adr: "00040"
title: "Dual-Lobe Single-Identity Agent Architecture"
topic: "In-Container Cognitive Architecture & Communication"
theme: "THEME-LIFECYCLE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - cognitive-architecture
  - dual-lobe
  - reflex
  - bpd
  - tmux
  - subagents
  - state-sharing
executive_summary: "Establishes a dual-lobe cognitive model for containerized agents: a Fast-Reflex Front Lobe (bpd + minimal triage context) for immediate signed backplane communication, and a Deep-Focus Worker (24/7 persistent session) for heavy multi-turn execution, sharing state via ~/.agent/state.json under a single cryptographic identity."
---

# 00040. Dual-Lobe Single-Identity Agent Architecture

## Context
Autonomous coding agents operating within containerized sandboxes face two competing operational imperatives:
1. **Deep, Multi-Turn Execution**: High-order software engineering (refactoring, fuzzing, static auditing, defect reproduction, and integration testing) requires deep focus, long-running multi-turn tool execution, and significant context window allocation (source code, compiler errors, diffs).
2. **Immediate Backplane Responsiveness**: The Valkey backplane ([ADR 00010](00010-ValkeyPubSubMessagingAndAclIsolation.md)) is an asynchronous bus where the human operator and peer agents frequently send status checks, coordination inquiries, or high-priority directives.

Attempting to handle both imperatives in a single monolithic thread creates critical failure modes:
- **Token Flooding & Context Rot**: Ingesting raw backplane chat into a heavy coding session forces the model to tokenize entire conversation transcripts just to answer a simple status query, burning tokens and diluting reasoning focus.
- **Mid-Execution Derailment**: An asynchronous backplane ping arriving while an agent is mid-edit in a multi-file refactor interrupts the tool loop, frequently causing abandoned edits or hallucinations.
- **Stateless Stalling in Headless Mode**: Conversely, running agents purely via event-driven headless daemons (`bpd`) causes them to execute one-shot impulses and halt, unable to sustain multi-turn autonomous workflows.

## Decision (What)
We establish the **Dual-Lobe Single-Identity Agent Architecture** inside containerized sandboxes:

```
                         ┌──────────────────────────────────────────────┐
                         │          Valkey Backplane Bus (bp)           │
                         └──────────────▲────────────────▲──────────────┘
                                        │                │
                                 Incoming Message        │ Signed Immediate Reply
                                        │                │ (Same Ed25519 Key)
                                        ▼                │
                     ┌───────────────────────────────────┴─────────────┐
                     │          Front Lobe: Fast Reflex (`bpd`)        │
                     │          (Low-latency, lightweight context)     │
                     │                                                 │
                     │ Ingests:                                        │
                     │ 1. Current State from `~/.agent/state.json`     │
                     │ 2. Incoming message                             │
                     └───────────────────┬─────────────────────────────┘
                                         │
                        Is it a directive│
                        requiring work?  │
                                         ▼
                               ┌───────────────────┐
                               │  In-Memory / IPC  │
                               │  Work Queue       │
                               │  (~/.agent/queue) │
                               └─────────┬─────────┘
                                         │
                                         │ Clean checkpoint pickup
                                         ▼
                     ┌─────────────────────────────────────────────────┐
                     │          Deep Lobe: Heavy Focus (24/7 tmux)     │
                     │                                                 │
                     │ • Executes code changes, git, compilers         │
                     │ • Spawns subagents for long-running reasoning   │
                     │ • Continually writes state to                   │
                     │   `~/.agent/state.json`                         │
                     └─────────────────────────────────────────────────┘
```

### 1. Single Logical Identity Invariant
To the physical host, human operator, and peer agents, there is only **one indivisible agent**:
- Both lobes run inside the same container environment under unprivileged user `agent` (`UID 1000`).
- Both lobes sign messages using the same cryptographic Ed25519 private key (`BP_SIGNING_KEY`).
- Both lobes bind to the same Valkey streams (`<agent>:out`, `<agent>:inbox`).
- The agent does not present split-brain personas or conflicting roles to the fleet.

### 2. Front Lobe: Fast Reflex Communication (`bpd`)
- Supervised by `bpd` running as a background service inside the container.
- Operates with minimal context (~300–500 tokens).
- Upon receiving a backplane message:
  1. Reads the current working state from `/home/agent/.agent/state.json`.
  2. If the message is an inquiry, coordination ping, or status query: formulates an immediate, accurate response using the shared state and replies over the backplane via `bp reply`.
  3. If the message is a new directive, task assignment, or human instruction: immediately emits a signed acknowledgment (`"Understood. Queued for execution: [summary]"`) and appends the directive to `/home/agent/.agent/queue.jsonl`.
  4. The Front Lobe never modifies project code, never runs git mutations, and never runs tests.

### 3. Deep Lobe: Heavy Focus Execution (Persistent Session)
- Runs continuously inside a persistent `tmux` session (or supervised runner) 24/7.
- Maintains deep conversational context, AST representations, active branch states, and test harnesses.
- At clean task checkpoints (between issue tasks or after tool completion), the Deep Lobe drains `/home/agent/.agent/queue.jsonl`.
- If an urgent directive from the human operator (`HUMAN_NAME`) is detected in the queue, the Deep Lobe prioritizes it immediately.
- Updates `/home/agent/.agent/state.json` upon transitioning between operational states.
- For harnesses that support subagents (Claude Code, OpenCode, Antigravity), the Deep Lobe delegates long-running reasoning or heavy compilation runs to background subagents, keeping its own loop responsive.

### 4. Direct Valkey Key-Value Baseline
- Static profile data and short-lived status strings continue to be maintained directly in Valkey Key-Value pairs:
  - `<agent>:status`: Ephemeral status string with 300s TTL ([ADR 00010](00010-ValkeyPubSubMessagingAndAclIsolation.md)).
  - `<agent>:finger`: Hash containing static role, capabilities, and platform metadata.
- Peer agents and host CLIs querying `bp finger` or `bp peers` read directly from Valkey KV without generating backplane messages or invoking LLM tokens.

### 5. Shared State Schema (`~/.agent/state.json`)
```json
{
  "agent_id": "pig-fuzzer",
  "status": "working",
  "current_task": "sndbx-d46.7",
  "task_description": "DRY Sonar client HTTP boilerplate and libbp Say/Tell/Reply",
  "active_branch": "feat/dry-sonar-client-libbp",
  "current_activity": "Refactoring pkg/sonar/client.go to extract doRequest helper",
  "last_test_result": "PASS (pkg/sonar 93.6%)",
  "blockers": [],
  "updated_at": "2026-10-07T14:15:00Z"
}
```

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- **Instant Backplane Responsiveness**: Pings and status inquiries receive signed responses in 1–2 seconds without waiting for long-running compilation or fuzzing loops.
- **Zero Token Waste**: Casual chatter and peer coordination messages never enter the heavy coding thread's context window.
- **Uninterrupted Code Focus**: Deep engineering turns run to completion without mid-turn interruptions.
- **Self-Driving Autonomy**: Eliminates the "stateless stalling" bug where headless event-driven agents halt after one impulse.

### Negative / Trade-offs
- Requires file-based state synchronization (`state.json` and `queue.jsonl`) between the two in-container lobes.
