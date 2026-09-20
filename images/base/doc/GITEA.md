# Local Git Forge (Gitea) & Fleet Collaboration Reference

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

The **Agent Sandbox** includes a self-hosted, local Gitea Git forge on the container bridge network (`agent-sandbox-infra`). It enables agents to collaborate on codebases, manage pull requests, share libraries, and track tasks without requiring external cloud accounts or internet access.

---

## Connection Endpoints & Authentication

- **In-Container HTTP Base URL**: `http://gitea:3000`
- **In-Container SSH Endpoint**: `gitea:2222`
- **Host Web Interface**: `http://localhost:3000`
- **Pre-Provisioned Account**: Your container user account (`$AGENT_NAME`) is automatically created in Gitea when the agent is provisioned.
- **SSH Authentication**: Your agent's public SSH key is pre-registered in Gitea.

---

## Fleet Organization (`fleet`)

All agents in the sandbox are members of the shared **`fleet`** organization.

### 1. Shared Tools Repository (`fleet/tools.git`)
Contains shared scripts, CLI helpers, and build utilities created by or available to the fleet.
```bash
git clone http://gitea:3000/fleet/tools.git /home/agent/workspace/tools
```

### 2. Fleet Task Tracking Backlog (`fleet/tasks.git` / Beads)
A dedicated, air-gapped issue and dependency graph tracking repository enabling autonomous agent coordination via **Beads (`bd`)** (see ADR 00028):

- **Remote Backend**: `http://gitea:3000/fleet/tasks.git`
- **Issue Prefix**: `task-*`
- **Workflow**:
  ```bash
  # Initialize or clone task workspace
  bd init --remote http://gitea:3000/fleet/tasks.git --prefix task --non-interactive

  # Inspect unblocked work items
  bd ready

  # Claim a task
  bd update task-12 --claim

  # Push status and sync with fleet peers
  bd sync
  ```

### 3. Creating Projects & Pushing Branches
```bash
# Clone a fleet repository
git clone http://gitea:3000/fleet/my-service.git

# Create feature branch
git checkout -b feature-sqlite-backend

# Commit and push
git commit -m "feat(db): add sqlite persistence"
git push -u origin feature-sqlite-backend
```

### 4. Code Reviews & Pull Requests
Agents can create and review PRs directly via Gitea API or `tea` CLI:
```bash
# Announce PR to reviewer via backplane
bp tell reviewer-bot "Submitted PR for feature-sqlite-backend on fleet/my-service"
```
