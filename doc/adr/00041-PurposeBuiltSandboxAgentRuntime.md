Here is the updated, complete **ADR 00041** incorporating the generalized **Modular Subsystem Supervisor Architecture**:

```markdown
```yaml
adr: "00041"
title: "Purpose-Built Headless Native Agent Runtime Engine"
topic: "In-Container Cognitive Architecture & Concurrency"
theme: "THEME-LIFECYCLE"
status: "accepted"
version: "v0.1.0-alpha"
as_built: false
tags:
  - native-agent
  - dual-lobe
  - supervisor
  - subsystem
  - goroutines
  - concurrency
  - headless
  - repl
  - libbp
  - event-bus
executive_summary: "Establishes a purpose-built, multi-threaded native Go agent runtime binary (sndbx-agent) as PID 1 for headless containers. Evolves the Dual-Lobe model (ADR 00040) into a generalized Modular Subsystem Supervisor Architecture using Go goroutines, channels, cancellation contexts, and a unified event bus. Enables flexible registration of specialized concurrent workers—including fast-reflex communication gateways, executive REPL engines, background watchers, callback handlers, and lightweight in-process sub-agents—without process-level multiplexing (tmux), STDIN/STDOUT scraping, or file-based IPC polling."

```

### 00041. Purpose-Built Headless Native Agent Runtime Engine

#### Context

Containerized agent environments previously relied on wrapping interactive developer CLIs (Claude Code, OpenCode, PiG) inside a background supervisor (`bpd`) dispatching turns via `/usr/local/bin/agent-runner` inside `tmux` sessions. While ADR 00040 introduced the **Dual-Lobe Single-Identity** model to decouple backplane responsiveness from deep execution, its implementation relied on process-level IPC—using `bpd` for the Front Lobe, `tmux` for the Deep Lobe, and filesystem polling (`~/.agent/state.json` and `~/.agent/queue.jsonl`) to synchronize state.

While effective as a compatibility bridge for external CLIs, this wrapper model introduces significant operational drawbacks for purely autonomous, unattended agents:

1. **Process Overhead & Brittle Interception**: Managing nested processes (`bpd` $\rightarrow$ `tmux` $\rightarrow$ CLI harness $\rightarrow$ child sub-processes) makes precise process control, clean shutdown propagation, and immediate task cancellation difficult.


2. **Terminal Rendering Bloat**: Interactive CLIs incur execution overhead rendering ANSI sequences, TUI layouts, and terminal formatting that are discarded in headless containers.


3. **Turn-Taking Latency & Disk Polling**: Synchronizing state via disk reads and writes creates file lock contention, disk IPC latency, and fragile state recovery upon unexpected container termination.


4. **Context Saturation**: Non-native runners struggle to filter routine telemetry, status inquiries, or peer notifications out of the primary LLM reasoning context window without burning token allocations or interrupting multi-step tool executions.


5. **Rigid Concurrency Model**: The hardcoded two-process architecture cannot easily accommodate auxiliary concurrent capabilities—such as HTTP webhook listeners, local filesystem watchers, automated background diagnostics, or dynamically spawned in-process sub-agents—without adding further external daemons or sidecars.



A purpose-built, native Go agent runtime designed strictly for headless, unattended execution can resolve these limitations by implementing the Dual-Lobe model within a generalized, multi-threaded **Subsystem Supervisor Architecture**.

---

#### Decision (What)

We establish the **Purpose-Built Headless Native Agent Runtime Engine (`sndbx-agent`)** as a statically linked Go binary running as PID 1 inside native agent container images.

`sndbx-agent` generalizes the Dual-Lobe concept from ADR 00040 into an in-process **Modular Subsystem Supervisor Architecture** powered by Go goroutines, channels, cancellation contexts, and a thread-safe in-memory state store.

```
+---------------------------------------------------------------------------------------+
| CONTAINER SECURITY PERIMETER (UID/GID 1000 - Non-Root / Rootless Subuid)[cite: 3]        |
|                                                                                       |
|  +---------------------------------------------------------------------------------+  |
|  |             NATIVE AGENT RUNTIME PROCESS (`sndbx-agent` - PID 1)                |  |
|  |                                                                                 |  |
|  |  +---------------------------------------------------------------------------+  |  |
|  |  |                     RUNTIME SUPERVISOR (`pkg/runtime`)                    |  |  |
|  |  |            (Lifecycle Management, Signal Handling, Graceful Shutdown)     |  |  |
|  |  +-------------------------------------+-------------------------------------+  |  |
|  |                                        |                                        |  |
|  |               Registers & Supervises Subsystems (`Worker` Interface)             |  |  |
|  |                                        |                                        |  |
|  |   +-----------------------+  +---------+-------------+  +--------------------+  |  |
|  |   | Fast-Reflex Gateway   |  | Executive REPL Engine |  | In-Process Sub-Agent|  |  |
|  |   | (Valkey / libbp)      |  | (Deep-Lobe REPL)      |  | (Worker Goroutine) |  |  |
|  |   |[cite: 24, 37]        |  |                       |  |                    |  |  |
|  |   +-----------+-----------+  +-----------+-----------+  +----------+---------+  |  |
|  |               |                          |                         |            |  |
|  |               |  +-----------------------+---+                     |            |  |
|  |               |  | Callback / Webhook        |                     |            |  |
|  |               |  | Handler (HTTP / gRPC)     |                     |            |  |
|  |               |  +-----------+---------------+                     |            |  |
|  |               |              |                                     |            |  |
|  |               |              |   +-----------------------+         |            |  |
|  |               |              |   | Background Watcher    |         |            |  |
|  |               |              |   | (Sonar / Git / Timer) |         |            |  |
|  |               |              |   |[cite: 30]            |         |            |  |
|  |               |              |   +-----------+-----------+         |            |  |
|  |               |              |               |                     |            |  |
|  |               v              v               v                     v            |  |
|  |  +---------------------------------------------------------------------------+  |  |
|  |  |              UNIFIED IN-MEMORY EVENT BUS & PRIORITY DISPATCHER            |  |  |
|  |  |                  (Priority Channels: P0, P1, P2, P3)                      |  |  |
|  |  +-------------------------------------+-------------------------------------+  |  |
|  |                                        |                                        |  |
|  |                                        v                                        |  |
|  |  +---------------------------------------------------------------------------+  |  |
|  |  |           THREAD-SAFE SHARED STATE & WORKSPACE LOCK MANAGER               |  |  |
|  |  |             (`sync.RWMutex` | Single-Writer Workspace Lock)               |  |  |
|  |  +---------------------------------------------------------------------------+  |  |
|  +----------------------+---------------------------------------+------------------+  |
|                         |                                       |                     |
|                         v                                       v                     |
|            [ Valkey Streams Messaging ]            [ Local LLM Inference Gateway ]    |
|            (<agent>:inbox / <agent>:out)           (http://llm-gateway:11434/v1)      |
|             (ADR 00010 / ADR 00011)[cite: 24, 33]  (ADR 00022 / ADR 00035)[cite: 4, 6]  |
+---------------------------------------------------------------------------------------+

```

##### 1. Single Native Binary & Runtime Supervisor (PID 1)

* The agent container entrypoint executes `/usr/local/bin/sndbx-agent` directly as PID 1, serving as process supervisor, signal handler (SIGTERM/SIGINT), and host for all in-container agent subsystems.


* Incorporates `pkg/libbp` for native Valkey stream I/O, `pkg/mcp` for Model Context Protocol client bindings, and a native LLM completion loop.


* Operates strictly under unprivileged user `agent` (UID 1000) with zero `sudo` capability escalations, making the rootless container boundary the security perimeter.



##### 2. Generalized Subsystem Architecture

The runtime models specialized tasks as modular concurrent workers implementing a unified `Subsystem` interface, managed centrally by the Runtime Supervisor (`pkg/runtime/supervisor.go`):

* **Fast-Reflex Gateway Subsystem (Front Lobe)**:
1. Runs an independent async event loop continuously polling incoming Valkey streams (`<agent>:inbox`) via `libbp`.


2. Verifies Ed25519 digital signatures on all inbound envelopes against sender public keys before processing.


3. **P3 Telemetry / Inquiries**: Immediately formats and signs responses to status checks, peer pings, or health audits directly from the in-memory state struct without invoking LLM inference or altering the main context window.


4. **P0 Interrupts**: Triggers a Go `context.CancelFunc` on active child subprocesses (e.g., hanging tests or runaway build loops) when a cancellation or emergency stop directive is received.
5. **P1/P2 Directives**: Dispatches structured directives onto the internal priority event bus for consumption by the REPL engine.


* **Executive REPL Engine Subsystem (Deep Lobe)**:
1. Runs the primary cognitive task loop: `Assemble Context` $\rightarrow$ `Query Inference Gateway` $\rightarrow$ `Parse Tool Call` $\rightarrow$ `Execute Tool` $\rightarrow$ `Append Observation`.
2. Queries local OpenAI-compatible inference endpoints (`http://llm-gateway:<port>/v1`) configured via environment variables.


3. Non-blockingly inspects the priority event bus between tool execution steps and turn boundaries.
4. Updates the shared in-memory state struct at every operational phase change.




* **Extensible Subsystem Modules (Auxiliary Workers)**:
1. **In-Process Delegated Sub-Agents**: The Executive REPL Engine can ask the Supervisor to dynamically spawn lightweight sub-agent goroutines with isolated execution contexts, scoped workspace sub-directories, and dedicated token budgets (e.g., a background fuzzer or static code reviewer).


2. **Callback & Webhook Listeners**: Embedded HTTP/gRPC servers ingesting direct external trigger signals (e.g., Gitea push hooks, SonarQube quality gate callbacks).


3. **Background Watchers & Diagnostics**: Periodic, non-LLM background workers inspecting local filesystem changes, SonarQube quality gate statuses, or system resource metrics, writing updates directly to the shared state store.





##### 3. Concurrency & Message Interception Hierarchy

| Priority | Signal Type | Target / Condition | In-Process Handling Mechanism | Executive Loop Impact |
| --- | --- | --- | --- | --- |
| **P0** | **Control Signal** | Halt, cancellation, emergency safety | Triggers `context.CancelFunc` on active tool/sub-agent context immediately. | Aborts child process execution; flushes current turn; resets loop. |
| **P1** | **High-Priority Directive** | Task pivot, direct human instruction | Pulled from priority event bus at the next tool boundary. | Injects directive into prompt context before the next LLM call. |
| **P2** | **Standard Async** | Peer message, webhook, task update | Queued in memory buffer; drained at natural turn boundary. | Appended as user/system observation when the current turn finishes. |
| **P3** | **State Query / Ping** | Operator status check, health audit | Answered directly by Gateway subsystem from `sync.RWMutex` state struct. | **Zero impact**. No LLM tokens consumed; no reasoning context added. |

##### 4. Concurrency Guardrails & Thread Safety

* **Single-Writer Workspace Lock (`sync.RWMutex`)**: While multiple subsystems can read state or emit telemetry concurrently, **only one subsystem** (typically the active Executive REPL or a delegated sub-agent holding an explicit lock) may write to the workspace filesystem (`/home/agent/workspace`) or execute mutating `git` commands at any given moment, precluding repository corruption.
* **Context Window Isolation & Turn Boundaries**: Non-P0 messages originating from webhooks, file watchers, or peer agents enter an `InboundQueue` buffer. Only the Executive REPL Engine controls when and how to drain this queue during prompt assembly, preventing context rot and mid-turn derailment.



##### 5. Native Tool & MCP Protocol Subsystem

* **Built-in System Tools**: Implements `bash` (subprocess runner with execution timeout and stdout/stderr stream truncation), `read_file`, `write_file`, `edit_file`, and `list_dir` natively in Go.
* **In-Container MCP Adapter**: Launches helper binaries (`bp-mcp`, `beads-mcp`, `sonar-mcp`) over STDIO JSON-RPC 2.0 per ADR 00032. Dynamically converts MCP tool schemas into JSON-Schema arrays for standard LLM function-calling APIs.


* **YOLO Execution Mode**: Operates in fully autonomous execution mode within `/home/agent/workspace`, executing all LLM-requested tool calls without human interactive confirmation prompts.



##### 6. Persistent State & Crash Recovery Checkpoints

* Primary state lives in memory (`sync.RWMutex`) for sub-millisecond thread access.
* To survive unexpected container terminations or host reboots, the runtime asynchronously flushes state snapshots to `/home/agent/.agent/state.json` on the persisted home volume (`sndbx-agent-<name>-home`) whenever operational state transitions occur.



---

#### Specification & Schema Definitions

##### A. Subsystem & Event Interfaces (`pkg/runtime/subsystem.go`)

```go
package runtime

import (
	"context"
)

// Subsystem represents a concurrent worker managed by the agent runtime supervisor.
type Subsystem interface {
	// ID returns a unique identifier for the subsystem (e.g., "gateway-valkey", "repl-main", "watcher-sonar").
	ID() string
	
	// Start initializes the worker and runs until ctx is canceled or an unrecoverable error occurs.
	Start(ctx context.Context, bus *EventBus, state *SharedState) error
}

// Event represents a normalized payload routed through the internal event bus.
type Event struct {
	ID        string        `json:"id"`
	Priority  PriorityLevel `json:"priority"` // P0, P1, P2, P3
	Source    string        `json:"source"`   // e.g., "valkey:inbox", "webhook:gitea", "subagent:fuzzer"
	Target    string        `json:"target"`   // e.g., "repl-main", "state-store", "*"
	Payload   any           `json:"payload"`
	Timestamp int64         `json:"timestamp"`
}

```

##### B. In-Memory Shared State Struct (`pkg/runtime/state.go`)

```go
type SharedState struct {
    sync.RWMutex
    AgentID         string    `json:"agent_id"`
    Status          string    `json:"status"`           // "idle", "working", "interrupted", "error"
    CurrentTaskID   string    `json:"current_task_id"`  // e.g., "task-104.2"
    TaskDescription string    `json:"task_description"`
    ActiveBranch    string    `json:"active_branch"`
    CurrentActivity string    `json:"current_activity"`
    LastTestResult  string    `json:"last_test_result"`
    QueueDepth      int       `json:"queue_depth"`
    UpdatedAt       time.Time `json:"updated_at"`
}

```

##### C. Priority Message Envelope (`pkg/runtime/types.go`)

```go
type PriorityLevel int

const (
    P0_Control PriorityLevel = iota
    P1_HighPriority
    P2_StandardAsync
    P3_TelemetryQuery
)

type BackplaneMessage struct {
    ID        string        `json:"id"`
    Priority  PriorityLevel `json:"priority"`
    Sender    string        `json:"sender"`
    Timestamp time.Time     `json:"timestamp"`
    Payload   string        `json:"payload"`
    Signature string        `json:"signature"`
}

```

---

#### Status

Accepted (Implementation spec for purpose-built native agent OCI images, e.g., `agent-sandbox-native`).

---

#### Consequences

##### Positive

* **Sub-Millisecond Reflexes**: P3 status inquiries and health audits are served instantly from memory by the Gateway subsystem with zero LLM API calls or context expansion.


* **Immediate Subprocess Cancellation**: P0 control signals immediately cancel long-running tool subprocesses or delegated sub-agent goroutines via Go context propagation.
* **Extensible Architecture**: Auxiliary workers (sub-agents, webhooks, background pollers) can be added or dynamically spawned without altering the core REPL loop or adding sidecar processes.


* **Elimination of TUI & Process Overhead**: Eliminates `tmux` process management, ANSI terminal stripping, and STDIN/STDOUT scraping.


* **Context Preservation**: Casual backplane traffic, webhooks, and background status updates do not enter the primary LLM reasoning context window until explicitly drained at turn boundaries.


* **Coexistence with Legacy CLIs**: Native agents run alongside existing CLI wrapper agents without modifying or deprecating ADR 00040.



##### Negative / Trade-offs

* **In-Process Thread Safety**: Requires careful synchronization (`sync.RWMutex`, workspace write locks, atomic channels) within the Go runtime to prevent race conditions across concurrent subsystems.
* **Loss of Manual Interactive Attachment**: Because no `tmux` session exists, human operators cannot attach an interactive terminal directly to the agent's live TUI stream. Operational debugging relies on `sndbx agent open` (IDE remote SSH), log streams (`ndjson`), or backplane inspection.



```

```