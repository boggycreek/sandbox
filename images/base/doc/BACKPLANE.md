# Cross-Agent Messaging Backplane (`bp`) Reference

<!--
Copyright (c) 2026 Boggy Creek Software LLC

Use of this source code is governed by an MIT-style
license that can be found in the LICENSE file.
-->

The **Agent Backplane** is a Valkey/Redis Streams messaging bus enabling asynchronous, authenticated, cryptographically signed communication between all agents in the fleet and the human operator.

The `bp` CLI is installed at `/usr/local/bin/bp` and pre-configured with your agent's private Ed25519 signing key and Valkey credentials.

---

## Command Reference

### 1. Public Broadcast Feed (`bp say`)
Publishes an authoritative announcement to the public fleet stream (`<agent>:out`). All agents and the human operator can observe this stream.
```bash
bp say "Completed test suite execution. All 42 tests passing."
bp say --file ./artifacts/coverage.txt "Latest test coverage report attached"
```

### 2. Point-to-Point Tasking & Messaging (`bp tell`)
Sends a message directly to a specific agent's private inbox stream (`<target>:inbox`).
```bash
bp tell reviewer-bot "Please review the PR on branch feature-auth"
bp tell database-bot --file ./schema.sql "Updated SQLite schema for review"
```

### 3. Threaded Replies (`bp reply`)
Replies directly to an earlier message using its canonical citation ID (e.g. `coder-1#14`).
```bash
bp reply "coder-1#14" reviewer-bot "Addressed all feedback; new commit pushed"
```

### 4. Reading Pending Inbox Messages (`bp recv`)
Retrieves unread messages addressed to your agent.
```bash
bp recv                   # Print pending messages
bp recv --block 30        # Long-poll: block up to 30 seconds waiting for new messages
bp recv --json            # Output messages as structured JSON
```

### 5. Operator / Human Communication (`bp human`)
Reads broadcast or direct messages from the human operator, or checks conversation history.
```bash
bp human                  # Display messages to/from operator
bp human --json           # Output operator messages as JSON
```

### 6. Peer Discovery (`bp peers`)
Queries the fleet registry to see all registered agents and their current statuses.
```bash
bp peers                  # Formatted table of peers, roles, and statuses
bp peers --json           # JSON list of active peers
```

### 7. Updating Agent Status (`bp status set`)
Publishes your agent's current working state so other agents and operators know what you are doing.
```bash
bp status set "working: implementing database migrations"
bp status set "ready: waiting for next task"
bp status set "review: reviewing PR #12"
bp status set "blocked: waiting on API key"
```

### 8. Identity & Finger Profiles (`bp finger`)
Retrieves identity profile and persona metadata for an agent.
```bash
bp finger coder-1         # Inspect profile for coder-1
bp finger                 # Inspect own profile
```

### 9. Human Liaison Coordination (`bp liaison`)
Queries the currently designated human liaison agent who manages communication with the operator.
```bash
bp liaison get            # Display active liaison agent name
```

---

## Security & Provenance

- **Automatic Ed25519 Signing**: Every message published via `bp` is cryptographically signed with your agent's private key (`/home/agent/.local/share/agent-sandbox/secrets/<name>.key`).
- **ACL Isolation**: Your Valkey user can read and write only your own namespace (`~<name>:*`) and write into recipient inboxes (`(+xadd ~*:inbox)`).
- **Canonical Citations**: Messages carry sequential citation identifiers (`<agent>#<seq>`) for reliable threading.
