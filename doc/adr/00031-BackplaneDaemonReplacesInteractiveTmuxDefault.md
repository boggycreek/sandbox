# 00031. Backplane Daemon (bpd) Replaces Interactive Tmux Session as Default Operating Mode

## Context
In the initial architecture of Agent Sandbox, agent containers were designed around interactive terminal sessions:
- ADR 00005 established direct `tmux` attach as the default entrypoint mechanism, spawning an interactive agent CLI (e.g. Claude Code or OpenCode) inside a background `tmux` session on container boot.
- ADR 00007 introduced the Valkey-based Agent Backplane Messaging Bus, where agents were awoken to check their inboxes using periodic session-scoped cron tasks (`bp recv`, `bp human`).
- Operators monitored and communicated with agents primarily via `sndbx agent connect <name>`, attaching directly to the running `tmux` session.

In practice, this interactive-by-default architecture exhibited critical inefficiencies when scaling to multi-agent fleets:
1. **Token Inefficiency from Cron Ticks**: Cron-driven wake-ups invoked agent reasoning turns on fixed schedules regardless of whether actionable messages had arrived. Each unnecessary tick burned LLM tokens re-evaluating context. Conversely, when bursts of messages arrived, cron polling introduced arbitrary turn latency.
2. **Interactive TUI Resource Contention & Drift**: Spawning interactive TUI loops (`claude` in tmux) inside every container pinned CPU and memory for pseudo-terminal emulation even when idle. Terminal escape sequence rendering, prompt redraws, and unhandled terminal input occasionally hung agents or caused state drift.
3. **Lack of Headless Orchestration Primitives**: Interactive sessions lacked structured turn management: message deduplication, batch draining, durable failure handling, dynamic backoff, and standard health state transitions (`IDLE`, `BUSY`, `DEGRADED`).
4. **Human/Agent Collision**: When human operators attached to the interactive tmux session to inspect files or test commands, the active agent loop could process concurrent events or send keys into the same pseudo-terminal, causing race conditions and terminal corruption.

## Decision
We replace the interactive `tmux` session as the default operating mode with a native headless event-driven daemon: the **Backplane Daemon (`bpd`)**.

### 1. `bpd` as Default Container Entrypoint
- `bpd` is executed as the primary `CMD` process within the container entrypoint (`/usr/local/bin/bpd`).
- Rather than launching an interactive TUI session in `tmux`, `bpd` runs headlessly, monitoring the agent's identity streams and orchestrating agent execution turns on demand.
- Container lifecycle states are directly tied to `bpd` execution.

### 2. Batched Message Draining from Valkey Streams
- `bpd` listens to the agent's point-to-point inbox (`<id>:inbox`) and subscribed broadcast channels (`<id>:out`) on the Valkey backplane bus.
- When new messages arrive, `bpd` atomically drains and batches pending entries, presenting them as a cohesive multi-message turn to the underlying agent runner.
- Draining messages in batches prevents token thrashing, preserves conversation context, and allows the agent to reason over related inquiries simultaneously.

### 3. Dynamic Poll Interval (`poll-interval` Key)
- Rather than hardcoding sleep or polling intervals in the daemon, `bpd` reads a dynamic configuration key `poll-interval` from Valkey (scoped globally or per-agent, e.g. `config:<id>:poll-interval` with fallback to `config:poll-interval`).
- Operators or backplane controllers can dynamically accelerate polling during high-throughput collaboration or back off polling during idle or maintenance periods without restarting containers.

### 4. Local Durable Failure Queue on Disk
- If an agent turn fails (e.g. LLM API timeouts, rate limits, non-zero execution exit codes, or unparseable responses), `bpd` records the failed message turn to a local durable failure queue on disk (`/home/agent/.bpd/failure_queue.jsonl`).
- Messages in the durable failure queue persist across container restarts. `bpd` applies exponential backoff before retrying queued items, preventing message loss during upstream outages.

### 5. `DEGRADED` Status Reporting
- `bpd` updates presence status keys in Valkey (`<id>:status`).
- If an agent encounters three (3) consecutive turn failures, `bpd` transitions the agent's reported status from `IDLE`/`BUSY` to `DEGRADED`.
- Backplane observers, monitoring tools, and `sndbx agent doctor` inspect this status to surface failing agents and trigger automated alerts or remediation workflows.

### 6. Advisory Attach Lock (`~/.bpd-attached`)
- To preserve the ability for human operators to inspect or debug an agent interactively without collisions, an advisory locking mechanism is introduced:
  - When an operator launches an interactive debugging session or diagnostic shell, an advisory lock file `~/.bpd-attached` is touched in the agent's home directory.
  - When `~/.bpd-attached` is present, `bpd` acknowledges human intervention, suspends autonomous turn dispatching, and pauses message consumption.
  - When the operator detaches and clears the lock (or upon timeout), `bpd` resumes normal autonomous message draining.
- Interactive `tmux` sessions are retired as the default boot target and are repurposed strictly as on-demand diagnostic environments.

## Status
Accepted.

## Consequences
- **Token and Compute Efficiency**: Agents consume LLM tokens strictly when actionable messages arrive, eliminating wasted polling turns and reducing baseline resource utilization across the sandbox fleet.
- **Event-Driven Responsiveness**: Incoming messages are processed immediately upon arrival via stream notifications rather than waiting for cron cycle intervals.
- **Operational Resilience**: The local failure queue and `DEGRADED` health reporting prevent silent message loss and provide fleet-wide visibility into agent health.
- **Shift in Operator Visibility**: Operators no longer monitor agents by default through an interactive tmux console; visibility transitions to backplane presence (`bp who`, `sndbx agent status`), stream logs, and structured container logs.
- **On-Demand Diagnostics**: Interactive tmux sessions remain fully supported for human pair-programming and troubleshooting, coordinated safely via the advisory attach lock (`~/.bpd-attached`).
