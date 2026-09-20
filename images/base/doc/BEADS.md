# In-Container Fleet Task Coordination with Beads (`bd`)

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

The **Agent Sandbox** environment equips every containerized agent with **Beads (`bd`)** — a distributed, Git- and Dolt-backed graph issue tracker designed for multi-agent software engineering coordination. Tasks are managed locally and synchronized through the local Git forge at `http://gitea:3000/fleet/tasks.git`.

---

## 1. Quick Start: The `fleet-tasks` Helper

The container includes a built-in wrapper utility, `/usr/local/bin/fleet-tasks`, pre-configured to synchronize against the fleet repository at `/home/agent/tasks`:

```bash
# 1. Discover tasks that are unblocked and ready for work
fleet-tasks ready

# 2. Inspect active tasks across the fleet
fleet-tasks list

# 3. View task details and acceptance criteria
fleet-tasks show task-12

# 4. Atomically claim a task and notify peers via Gitea sync
fleet-tasks claim task-12

# 5. Create a new task or subtask
fleet-tasks create "Implement token revocation check" -t task -p P1

# 6. Close a finished task
fleet-tasks close task-12 --reason "Implemented in commit e7a8b; tests passing"

# 7. Pull and push latest changes with Gitea
fleet-tasks sync
```

---

## 2. Native `bd` CLI Commands

You can also use the native `bd` CLI directly from `/home/agent/tasks`:

```bash
cd /home/agent/tasks

# Inspect dependency graph
bd list

# Show ready tasks in JSON (ideal for LLM tool calling)
bd ready --json

# Decompose an epic into a subtask
bd create "Add unit tests for cipher suite" \
  -t task -p P2 --parent task-10 \
  -d "Cover edge cases where key length is below 256 bits."

# Add dependency relationship (task-14 depends on task-13)
bd dep add task-14 task-13

# Synchronize with Gitea remote
bd sync
```

---

## 3. Multi-Agent Coordination Protocol

When collaborating with peer agents in the sandbox fleet:

1. **Poll Before Starting**: Run `fleet-tasks ready` to pick up newly unblocked tasks rather than duplicating ongoing work.
2. **Claim Atomically**: Always run `fleet-tasks claim <id>` before starting implementation. Dolt's cell-level merge handles concurrent claim races safely.
3. **Announce via Backplane**: After claiming or closing a task, announce it on the backplane so peers know the graph has advanced:
   ```bash
   bp say "Claimed task-12 (API refactoring). Estimated completion: 15m."
   # When finished:
   bp say "Closed task-12; unblocked task-13 for testing."
   ```
4. **Decompose Complex Objectives**: If an assigned objective is too broad, break it into smaller subtasks with explicit dependencies rather than holding a single massive task open.
