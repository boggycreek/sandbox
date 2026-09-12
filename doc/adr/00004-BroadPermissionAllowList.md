# 00004. Broad Permission Allow-List Instead of Per-Command Prompts

## Context
Standard AI coding agents operating directly on a developer's host machine prompt the user before running terminal commands (`git push`, `rm`, `npm install`, `sed`) to prevent accidental destruction of host files. However, in an autonomous multi-agent environment, per-command human prompts create severe latency and prevent unattended execution.

## Decision
Because the container boundary and non-root execution model form the primary security boundary, agents running in the sandbox are configured with broad, unprompted command allow-lists:
- For Claude Code: `defaultMode: "acceptEdits"` and an allow-list covering the complete development loop (`git`, `tea`, `gh`, `bd`, `qmd`, `npm`, `npx`, `python`, `rm`, `mkdir`, `cp`, `mv`, `rg`, `fd`, `find`, `cat`, `jq`, `sed`, `awk`, `bp`).
- The security perimeter is maintained by isolating host filesystem mounts and restricting network egress rather than relying on per-command interactive prompts.

## Status
Accepted.

## Consequences
- Agents can operate autonomously overnight or over long-running task cycles without blocking on user approval prompts.
- Destruction of files within `/home/agent/workspace` is confined strictly to that instance's container volume and recoverable via Git.
