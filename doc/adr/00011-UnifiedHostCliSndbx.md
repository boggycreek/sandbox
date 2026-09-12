# 00011. Unified Host CLI (`sndbx`)

## Context
Operating the sandbox environment previously required navigating multiple shell scripts (`agent.sh`, `infra.sh`, `install.sh`, `setup.sh`) or using an application-specific acronym (`cas`). Operators need a clear, unified, single command-line interface on their `$PATH` to manage instances, infrastructure, and tool upgrades.

## Decision
Create the **`sndbx`** native Go binary as the primary host-side CLI installed at `${HOME}/.local/bin/sndbx`:
- `sndbx agent <verb>`: Subcommands for instance lifecycle (`start`, `connect`, `ssh`, `list`, `stop`, `clean`, `clean-login`, `clean-all`, `refresh`).
- `sndbx infra <verb>`: Subcommands for shared services (`up`, `down`, `list`, `connect`, `clean`).
- `sndbx repo <verb>`: Monorepo management (`build`, `upgrade`, `path`, `uninstall`).
- `sndbx gui`: Launches the native desktop backplane viewer.
- **XDG Standards Compliance**: Stores persistent data in `${XDG_DATA_HOME:-~/.local/share}/agent-sandbox` and binaries in `${HOME}/.local/bin`.

## Status
Accepted.

## Consequences
- Clean, consistent command ergonomics across macOS and Linux developer workstations.
- Replaces legacy multi-script entry points with a single, discoverable command (`sndbx help`).
