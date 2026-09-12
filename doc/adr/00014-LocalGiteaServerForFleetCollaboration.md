# 00014. Local Gitea Server for Fleet Collaboration

## Context
Autonomous agents in a fleet need to share custom software tools, collaborate on multi-file feature codebases, perform code reviews via diffs and pull requests, and manage task backlogs without requiring external internet access or exposing work-in-progress code to cloud-hosted repositories.

## Decision
Add **Gitea** (lightweight, self-hosted Git service) as a core service in the shared local infrastructure (`infra/docker-compose.yml`):
1. **Network & Access**: Joined to the `agent-sandbox-infra` network (`http://gitea:3000` internal, `127.0.0.1:3000` host web UI, and `127.0.0.1:2223` SSH).
2. **Fleet Organization (`fleet`)**: Provides a shared namespace for fleet utility repositories (`fleet/tools.git`), shared libraries, and inter-agent PR reviews.
3. **Task Tracking with Beads (`bd`)**: Hosts a git-backed issue repository (`fleet/tasks.git`) enabling agents to asynchronously create, claim, and close work tickets via `bd`.
4. **Automated Bootstrapping**: `sndbx infra up` automatically provisions the Gitea admin account and default `fleet` organization using the Gitea CLI.

## Status
Accepted.

## Consequences
- Full-featured Git forge, PR review UI, and git-native task tracking running entirely local to the host machine.
- Zero reliance on external GitHub APIs for fleet-internal coordination.
- Minimal resource overhead (runs as lightweight rootless container with SQLite backend).
