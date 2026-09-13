# Architecture Decision Records (ADRs)

This directory contains records of the architectural decisions made for the **Agent Sandbox** project.

Each ADR follows the Context, Decision, Status, and Consequences format and is numbered sequentially with a 5-digit zero-padded index (`00001-<Topic>.md`).

## Index of Decisions

- [00001 — Non-Root Container User](00001-NonRootContainerUser.md)
- [00002 — Per-Instance Isolation for Parallel Sandboxes](00002-PerInstanceIsolation.md)
- [00003 — Persisted State Across Container Recreation](00003-PersistedStateAcrossContainerRecreation.md)
- [00004 — Broad Permission Allow-List](00004-BroadPermissionAllowList.md)
- [00005 — Direct Tmux Attach Without Host Wrapper](00005-DirectTmuxAttach.md)
- [00006 — Unprivileged SSH Daemon for IDE Remote Access](00006-UnprivilegedSshdForIdeAccess.md)
- [00007 — Agent Backplane Messaging Bus on Valkey Streams](00007-AgentBackplaneMessagingBus.md)
- [00008 — Valkey ACL Security Model](00008-ValkeyAclSecurityModel.md)
- [00009 — Ed25519 Message Signing for Cryptographic Provenance](00009-Ed25519MessageSigning.md)
- [00010 — Go Monorepo and Native Static Binaries](00010-GoMonorepoAndNativeStaticBinaries.md)
- [00011 — Unified Host CLI (`sndbx`)](00011-UnifiedHostCliSndbx.md)
- [00012 — Core Backplane Client Library (`libbp`) with C-ABI FFI](00012-LibbpCoreClientLibraryAndCAbi.md)
- [00013 — Layered OCI Container Hierarchy (Base vs Derivative)](00013-LayeredOciContainerHierarchy.md)
- [00014 — Local Gitea Server for Fleet Collaboration](00014-LocalGiteaServerForFleetCollaboration.md)
- [00015 — Agent Dotfiles and Memory Remote Backup to Git](00015-AgentDotfilesAndMemoryBackupToGit.md)
- [00016 — Wayland-Native Linux GUI with GTK4 and Libadwaita](00016-WaylandNativeLinuxGuiWithGtk4.md)
- [00017 — Native SVG Diagram and Markdown Rendering Without Browser Engine](00017-NativeSvgDiagramRenderingForMarkdown.md)
- [00018 — Quality Engineering, >90% Test Coverage, Linters, and SCA](00018-QualityEngineeringTestingAndSCA.md)
- [00019 — Podman as Required Dev and Runtime Dependency](00019-PodmanAsRequiredDependency.md)
- [00020 — Agent Creation and Lifecycle Separation in sndbx CLI](00020-AgentCreationAndLifecycleSeparation.md)
- [00021 — Local OpenAI-Compatible Inference Support and Host Gateway Routing](00021-LocalOpenAICompatibleInferenceSupport.md)
- [00022 — Agent Deprovisioning and Full Infrastructure Retirement](00022-AgentDeprovisioningAndRetirement.md)
- [00023 — In-Container Environment Documentation and Dedicated doc Directory](00023-InContainerEnvironmentDocumentationAndDocDirectory.md)

