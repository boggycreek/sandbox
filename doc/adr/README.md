# Architecture Decision Records (ADRs)

This directory documents the foundational architectural decisions governing the **Agent Sandbox** system for the **v0.1.0-alpha** release.

Records are numbered serially (`00001` through `00031`) and organized by topic domain to reflect the current **as-built** architecture. Each decision record includes machine-readable YAML front matter (with standardized thematic markers, tags, and executive summaries) for consumption by automated agents and tooling.

---

## Thematic Groupings

| Theme Code | Topic Domain | Scope |
| :--- | :--- | :--- |
| **`THEME-CORE`** | Foundations & Architecture | Monorepo structure, native static binaries, single host CLI, rootless engine requirement. |
| **`THEME-RUNTIME`** | Container Runtime & Storage | Unprivileged user boundaries, persistent named volumes, layered image hierarchy. |
| **`THEME-LIFECYCLE`** | Agent Lifecycle & Process Model | Phase separation, Backplane Daemon (`bpd`) as PID 1, non-destructive cleaning vs. retirement. |
| **`THEME-SECURITY`** | Security, Backplane & Messaging | Valkey ACL isolation, Ed25519 digital signatures, `libbp` C ABI library. |
| **`THEME-NETWORKING`** | Network Isolation & Perimeter | Dedicated rootless netns, default-deny egress filtering sidecar, netns self-healing. |
| **`THEME-DEVEXP`** | Developer Experience & IDEs | One-shot IDE launching (`open`), managed OpenSSH config include, root-owned settings protection. |
| **`THEME-PLUGINS`** | IDE Plugin Management | Strict three-verb plugin interface, JetBrains Gateway vs. Toolbox decoupling & native SSH sync. |
| **`THEME-FLEET`** | Shared Fleet Services | Local Gitea git hosting & memory backup, standardized local OpenAI inference proxy. |
| **`THEME-OPERATIONS`** | Operations, Diagnostics & Quality | Diagnostic doctor self-healing (`--fix`), unified update pipeline, >=90% test coverage gate, SBOM generation. |
| **`THEME-GOVERNANCE`** | Compliance & Governance | Open-source attribution standards, third-party licensing compliance, legal notices. |

---

## Architectural Decision Records

### Foundations & Architecture (`THEME-CORE`)
- **[00001 — Go Monorepo and Native Static Binaries](00001-GoMonorepoAndNativeStaticBinaries.md)**  
  *Executive Summary:* All Agent Sandbox host and in-container utilities are developed in a unified Go monorepo and compiled into statically linked, zero-dependency native binaries.
- **[00002 — Unified Host CLI (sndbx)](00002-UnifiedHostCliSndbx.md)**  
  *Executive Summary:* A single multi-command binary (`sndbx`) acts as the sole operator entry point for fleet management, diagnostics, plugin setup, and orchestration.
- **[00003 — Podman as Required Container Engine](00003-PodmanAsRequiredContainerEngine.md)**  
  *Executive Summary:* Rootless Podman is the mandatory container runtime dependency, eliminating root-owned daemon requirements and enforcing user-space privilege boundaries.

### Container Runtime & Storage (`THEME-RUNTIME`)
- **[00004 — Non-Root Container User and Permission Bounds](00004-NonRootContainerUserAndPermissionBounds.md)**  
  *Executive Summary:* Sandboxes execute strictly as unprivileged user `agent` (UID/GID 1000) under restricted Linux capabilities and rootless subuid mappings.
- **[00005 — Persisted Home Volume Across Container Recreation](00005-PersistedHomeVolumeAcrossRecreation.md)**  
  *Executive Summary:* Agent persistent state and memory reside in a dedicated named volume mounted to `/home/agent` that survives container restarts, updates, and recreation.
- **[00006 — Layered OCI Container Hierarchy and Resolution](00006-LayeredOciContainerHierarchy.md)**  
  *Executive Summary:* Agent images follow a strict inheritance chain (`sndbx-base` -> preset variants) resolved from local storage before falling back to external registries.
- **[00026 — Deno as Standard JavaScript Runtime for OCI Container Agents](00026-DenoAsStandardJsRuntimeForOciContainerAgents.md)**  
  *Executive Summary:* Deno 2.x replaces Node.js as the standard JavaScript/TypeScript runtime in all agent OCI images, providing native TypeScript execution, granular capability sandboxing, and a leaner container footprint without compromising npm ecosystem compatibility.

### Agent Lifecycle & Process Model (`THEME-LIFECYCLE`)
- **[00007 — Agent Lifecycle Phase Separation](00007-AgentLifecyclePhaseSeparation.md)**  
  *Executive Summary:* Decouples agent specification and credential generation (`create`) from container execution (`start`), halting (`stop`), and runtime status reporting.
- **[00008 — Backplane Daemon (bpd) as Primary Entrypoint](00008-BackplaneDaemonAsPrimaryEntrypoint.md)**  
  *Executive Summary:* Sandboxes run `bpd` as PID 1 entrypoint managing background workers; interactive `tmux` is an on-demand debug attachment tool rather than the container entrypoint.
- **[00009 — Deprovisioning and Retirement Phases](00009-DeprovisioningAndRetirementPhases.md)**  
  *Executive Summary:* Enforces clear teardown boundaries: `clean` destroys ephemeral container instances while preserving data; `retire` purges volumes, keys, Valkey credentials, and Gitea accounts.

### Security, Backplane & Messaging (`THEME-SECURITY`)
- **[00010 — Valkey PubSub Messaging Bus and ACL Isolation](00010-ValkeyPubSubMessagingAndAclIsolation.md)**  
  *Executive Summary:* Inter-agent and telemetry communication utilizes a shared Valkey message bus governed by strict per-agent ACL rules and isolated channel prefixes.
- **[00011 — Ed25519 Cryptographic Message Signing](00011-Ed25519CryptographicMessageSigning.md)**  
  *Executive Summary:* All backplane events and commands require cryptographic Ed25519 signatures verified against the sending agent's public key to guarantee authenticity.
- **[00012 — libbp Core Client Library and Shared C ABI](00012-LibbpCoreClientLibraryAndCAbi.md)**  
  *Executive Summary:* Backplane IPC protocol logic is implemented in a native Go library (`libbp`) and exposed as a shared C ABI (`libbp.so`) for polyglot agent runtimes.

### Network Isolation & Perimeter Defense (`THEME-NETWORKING`)
- **[00013 — Per-Instance Network Isolation](00013-PerInstanceNetworkIsolation.md)**  
  *Executive Summary:* Sandboxes run in isolated rootless network namespaces with independent bridge interfaces and dedicated localhost SSH port allocations.
- **[00014 — Default-Deny Network Egress Filtering](00014-DefaultDenyNetworkEgressFiltering.md)**  
  *Executive Summary:* Outbound container network traffic is restricted by default via a sidecar filter, permitting only approved LLM API endpoints and package repositories.
- **[00015 — Rootless Netns Runtime Directory Auto-Healing](00015-RootlessNetnsRuntimeDirectoryAutoHealing.md)**  
  *Executive Summary:* Runtime preflight hooks, transparent failure interception, and test cleanup routines validate directory permissions and reconcile desynchronized rootless network namespace mounts.

### Developer Experience & IDE Ensembling (`THEME-DEVEXP`)
- **[00016 — One-Shot IDE Remote Development and Host Ensembling](00016-OneShotIdeRemoteDevelopmentAndHostEnsembling.md)**  
  *Executive Summary:* A unified command (`sndbx agent open`) launches host thin-client IDEs against in-container `sshd`, supporting simultaneous multi-IDE attachment to a single agent.
- **[00017 — Managed OpenSSH Configuration Include](00017-ManagedOpenSshConfigurationInclude.md)**  
  *Executive Summary:* `sndbx` maintains a dedicated managed `ssh_config` file and idempotently links it into `~/.ssh/config` via an `Include` directive for zero-configuration host SSH access.
- **[00018 — Root-Owned IDE Settings Protection](00018-RootOwnedIdeSettingsProtection.md)**  
  *Executive Summary:* In-container IDE configuration directories (`.vscode`, `.cursor`) are root-owned and read-only to agent UID 1000, preventing unauthorized extensions or policy tampering.

### IDE Plugin Management (`THEME-PLUGINS`)
- **[00019 — Explicit Plugin Manager Interface](00019-ExplicitPluginManagerInterface.md)**  
  *Executive Summary:* IDE plugin lifecycles are governed strictly through `sndbx plugin <add|remove|list> <target>` without aliases or shorthand commands.
- **[00020 — JetBrains Gateway vs. Toolbox Decoupling and Native SSH Sync](00020-JetBrainsGatewayVsToolboxDecouplingAndNativeSshSync.md)**  
  *Executive Summary:* Decouples IntelliJ Gateway plugins (`gateway`) from JetBrains Toolbox App integration (`toolbox`), synchronizing running sandboxes directly to Toolbox's native `ssh/settings.json` and inlined host blocks.

### Shared Fleet Services (`THEME-FLEET`)
- **[00021 — Local Gitea Fleet Collaboration and Memory Backup](00021-LocalGiteaFleetCollaborationAndBackup.md)**  
  *Executive Summary:* An internal rootless Gitea service provides local git hosting, inter-agent code review, and automated synchronization of agent dotfiles and memory.
- **[00022 — Local OpenAI-Compatible Inference Proxy](00022-LocalOpenAiCompatibleInferenceProxy.md)**  
  *Executive Summary:* Sandboxes access LLM inference through a standardized local OpenAI-compatible HTTP gateway, shielding agents from direct external API credentials.
- **[00027 — Local SonarQube Deterministic Mechanical Analysis and Quality Gate Infrastructure](00027-LocalSonarQubeDeterministicMechanicalAnalysisAndQualityGateInfra.md)**  
  *Executive Summary:* Integrates SonarQube Server Community Edition into the shared sandbox infrastructure alongside Valkey and Gitea. Agents utilize SonarQube for multi-tiered deterministic mechanical analysis — contrasting with probabilistic LLM code review — through automated defect discovery via a native Model Context Protocol (MCP) server and strict Quality Gate verification across fleet repositories.
- **[00028 — In-Container Fleet Task Coordination via Local Gitea-Backed Beads](00028-InContainerFleetTaskCoordinationViaLocalGiteaBackedBeads.md)**  
  *Executive Summary:* Establishes an air-gapped, distributed task and dependency tracking architecture for autonomous in-container agents using Beads (`bd`) backed by the local Gitea server (`http://gitea:3000/fleet/tasks.git`), maintaining strict architectural separation from host-level platform development on GitHub.
- **[00029 — Per-Agent SonarQube User Account and Analysis Token Provisioning Lifecycle](00029-SonarQubeUserAccountAndTokenProvisioningLifecycle.md)**  
  *Executive Summary:* Automates the provisioning and deprovisioning of dedicated SonarQube user accounts and analysis tokens for sandbox agents. During agent creation, a scoped analysis token is generated and injected into the container environment as `SONAR_TOKEN`. Agent retirement revokes active analysis tokens and deactivates the SonarQube user account.
- **[00032 — Fleet Model Context Protocol (MCP) Suite for Agent Workspaces](00032-FleetModelContextProtocolSuiteForAgentWorkspaces.md)**  
  *Executive Summary:* Defines the in-container Model Context Protocol (MCP) suite over STDIO JSON-RPC 2.0, equipping autonomous sandbox agents with schema-validated tools for inter-agent communication (`bp-mcp`), task management, episodic memory, and diagnostics while maintaining Ed25519 cryptographic signing and ACL boundaries.

### Operations, Diagnostics & Quality (`THEME-OPERATIONS`)
- **[00023 — Comprehensive Diagnostic Doctor and Self-Healing](00023-ComprehensiveDoctorAndSelfHealing.md)**  
  *Executive Summary:* A unified diagnostic engine (`sndbx doctor [--infra] [--fix]`) audits permissions, container states, network bridges, and shared daemons with automated remediation.
- **[00024 — Unified Update Pipeline (sndbx update)](00024-UnifiedUpdatePipeline.md)**  
  *Executive Summary:* The `sndbx update` command executes an atomic three-stage local deployment: git repository synchronization, native CLI compilation to `~/.local/bin`, and OCI image rebuilding.
- **[00025 — Quality Gates and Coverage Enforcement](00025-QualityGatesAndCoverageEnforcement.md)**  
  *Executive Summary:* Enforces continuous quality gates requiring >=90% statement test coverage (`make test-coverage`), static analysis (`golangci-lint`), and isolated ephemeral integration test fixtures.
- **[00031 — Automated Software Bill of Materials (SBOM) Generation and Release Publishing](00031-AutomatedSoftwareBillOfMaterialsAndReleasePublishing.md)**  
  *Executive Summary:* Automates the generation and distribution of machine-readable Software Bill of Materials (SBOM) across release pipelines and local build targets. Leverages Syft to produce both SPDX and CycloneDX JSON formats for source repositories and compiled release archives, publishing them directly as release assets.

### Compliance, Licensing & Governance (`THEME-GOVERNANCE`)
- **[00030 — Open Source Attribution and Third-Party Licensing Notices](00030-OpenSourceAttributionAndThirdPartyLicensingNotices.md)**  
  *Executive Summary:* Establishes a clear boundary between human-centric open-source recognition (`ACKNOWLEDGEMENTS.md`) and formal legal license texts (`NOTICES.md`). Reinforces our zero-dependency standard library Go architecture while systematically preserving third-party copyright notices and disclaimers for all container runtime, infrastructure, and session tooling components.


