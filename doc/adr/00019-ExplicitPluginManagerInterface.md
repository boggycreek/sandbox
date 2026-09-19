---
adr: "00019"
title: "Explicit Plugin Manager Interface"
topic: "IDE Plugin Management"
theme: "THEME-PLUGINS"
status: "accepted"
version: "v0.1.0-alpha"
as_built: true
tags:
  - plugins
  - cli
  - ux
  - interface
  - strict-syntax
executive_summary: "IDE plugin lifecycles are governed strictly through 'sndbx plugin <add|remove|list> <target>' without aliases or shorthand commands."
---

# 00019. Explicit Plugin Manager Interface

## Context
Early development considered various shorthand commands and aliases for managing IDE plugins (e.g. `sndbx plugin install`, `sndbx plugin rm`, `sndbx plugin toolbox`, `sndbx install plugin`). Multiple overlapping syntax paths create CLI confusion, complicate documentation, increase maintenance complexity, and hinder programmatic script generation.

## Decision (What)
The `sndbx plugin` command domain strictly enforces an explicit three-verb interface with mandatory target arguments:

```bash
sndbx plugin <command> [target]
```

- **Canonical Verbs**:
  - `add <target>`: Installs and configures the specified IDE integration.
  - `remove <target>`: Uninstalls artifacts and cleans configuration links for the target.
  - `list`: Displays a tabular overview of supported targets and their current installation status.
  - `help`: Prints syntax documentation and available targets.
- **Strict Grammar**:
  - Shorthand execution (e.g. `sndbx plugin toolbox`) is strictly rejected with a usage error.
  - Command aliases (e.g. `install`, `uninstall`, `rm`, `delete`, `ls`) are explicitly barred.

Supported target keywords are `gateway`, `toolbox`, and `vscode`.

## Status
Accepted (Alpha as-built).

## Consequences
### Positive
- Highly predictable, unambiguous CLI ergonomics for operators and AI agents alike.
- Single code path for each operational action simplifies testing and documentation.
- Eliminates edge-case bugs caused by alias permutations.

### Negative / Trade-offs
- Users accustomed to `install`/`uninstall` syntax must use `add`/`remove`.
