# ADR 00032 — Fleet Model Context Protocol (MCP) Suite for Agent Workspaces

## Status
Accepted

## Context
Autonomous AI coding agents running inside containerized sandboxes interact with their environment and peer agents via standard CLI tools (`bp`, `bd`, `git`, `curl`). While CLI commands are expressive, modern agent harnesses (such as Anthropic Claude Code, OpenCode, and Antigravity) natively leverage the **Model Context Protocol (MCP)** over STDIO JSON-RPC 2.0.

MCP equips AI models with strongly-typed, JSON-schema-validated tool execution, structured responses, and deterministic error reporting. To provide seamless native integration across our multi-agent fleet without sacrificing security boundaries or cryptographic signing, we need dedicated in-container MCP servers for core platform capabilities.

## Decision
We implement a modular suite of Go-native, statically compiled STDIO JSON-RPC 2.0 MCP servers installed directly inside all sandbox agent containers:

1. **Fleet Backplane MCP Server (`bp-mcp`)**:
   - Binary: `/usr/local/bin/bp-mcp`
   - Tools:
     - `fleet_send_message`: Sends point-to-point signed messages or threaded replies to peer agent or human operator inboxes.
     - `fleet_broadcast`: Broadcasts signed public announcements to the fleet feed (`<agent>:out`).
     - `fleet_read_inbox`: Retrieves pending direct messages and peer broadcasts.
     - `fleet_list_peers`: Discovers active fleet agents, operational statuses, roles, and liaison state.
     - `fleet_set_status`: Updates agent status broadcasted across the fleet.

2. **Gitea Fleet Forge MCP Server (`gitea-mcp`)** (ADR / implementation in progress):
   - Exposes local forge task tracking, issue creation, pull request reviews, and raw file inspection.

3. **Beads Graph Issue Tracker MCP Server (`beads-mcp`)** (ADR / implementation in progress):
   - Exposes graph-based task discovery (`bd_ready`, `bd_claim`, `bd_close`, `bd_sync`).

4. **Episodic Memory & Reflection MCP Server (`memory-mcp`)**:
   - Exposes agent episodic memory and knowledge retrieval tools.

5. **Build & Test Diagnostics MCP Server (`test-runner-mcp`)**:
   - Exposes deterministic build, test execution, and failure triage diagnostics.

6. **In-Container Environment & Doctor MCP Server (`agent-doctor-mcp`)**:
   - Exposes container health checks, connectivity tests, resource inspections, and human escalation.

### Architecture & Security Invariants
- **STDIO JSON-RPC 2.0**: Every MCP server communicates strictly over `stdin`/`stdout` using JSON-RPC 2.0 protocol specifications.
- **Zero-Touch Configuration**: MCP servers automatically discover credentials, network addresses, and cryptographic keys from the injected container environment (e.g., `$BP_HOST`, `$BP_PORT`, `$AGENT_NAME`, `$AGENT_PASSWORD`, `$BP_SIGNING_KEY`).
- **Cryptographic Provenance**: Backplane messages dispatched via `bp-mcp` are signed using the agent's Ed25519 private key, maintaining full cryptographic provenance matching the `bp` CLI.
- **Strict Quality Gates**: Every MCP server maintains `>= 90.0%` unit test coverage and conforms to Go monorepo linting and static analysis standards.

## Consequences

### Positive
- AI agents in sandboxes gain first-class, schema-validated MCP tool calling for all fleet communications.
- Direct JSON structured return objects reduce LLM token overhead and parsing errors compared to scraping raw CLI text outputs.
- Retains existing ACL isolation and Ed25519 signing guarantees of the underlying `libbp` backplane client.

### Negative / Trade-offs
- Additional native binaries compiled during base image build and installed in `/usr/local/bin`.
- Requires maintaining unit test suites and schema documentation for each MCP server.
