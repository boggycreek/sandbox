# Acknowledgements & Open Source Credits

The **Agent Sandbox** platform is developed by **Boggy Creek Software LLC**. We believe deeply in the open-source ethos and acknowledge that this project is made possible by the incredible work of open-source software creators, maintainers, foundations, and communities worldwide.

---

## Zero-Dependency Core Architecture

The core native binaries (`sndbx`, `bp`, `bpd`, `sonar-mcp`) and shared Go libraries (`pkg/...`) are engineered intentionally with **zero external Go module dependencies**. We build exclusively on the standard library of the **Go Programming Language**, maintained by the Go Authors and Google:

- **[The Go Programming Language](https://go.dev/)** — BSD-3-Clause License  
  *Provided the robust concurrency model, cryptographic primitives (`crypto/ed25519`), networking, and tooling enabling reliable, cross-platform static binaries.*

---

## Foundational Open Source Projects & Communities

We express our sincere gratitude to the following open-source projects, tools, and platforms that form the pillars of Agent Sandbox:

### 1. Container Runtime & Process Isolation
- **[Podman](https://podman.io/)** (Red Hat / Containers Community) — Apache-2.0 License  
  *Provides daemonless, rootless OCI container orchestration, unprivileged user namespaces (`subuid`/`subgid`), and secure container perimeter enforcement without requiring a host daemon.*
- **[Conmon & crun](https://github.com/containers)** (Containers Community / Red Hat) — Apache-2.0 & GPL-2.0 Licenses  
  *Provides lightweight OCI container monitoring and low-level container execution.*
- **[Debian GNU/Linux](https://www.debian.org/)** (The Debian Project) — Debian Free Software Guidelines (DFSG)  
  *Serves as the solid, predictable base operating system layer for our `agent-sandbox-base` container image.*

### 2. High-Performance Messaging & Shared Infrastructure
- **[Valkey](https://valkey.io/)** (Linux Foundation / Valkey Community) — BSD-3-Clause License  
  *Powers our high-performance, real-time agent backplane bus (`bp`), Valkey Streams, pub/sub channels, and granular per-agent ACL security boundaries.*
- **[Gitea](https://about.gitea.com/)** (Gitea Authors / Community) — MIT License  
  *Provides lightweight, self-hosted Git repository hosting, forge REST APIs, and automated SSH key management for local multi-agent codebases.*
- **[PostgreSQL](https://www.postgresql.org/)** (The PostgreSQL Global Development Group) — PostgreSQL License  
  *Provides industrial-grade relational database storage for shared infrastructure services.*
- **[SonarQube Community Build](https://www.sonarsource.com/products/sonarqube/)** (SonarSource) — LGPL-3.0 License  
  *Provides continuous automated code quality inspection, static analysis, and security vulnerability scanning for autonomous agent work.*

### 3. Agent Backlog & Graph Issue Tracking
- **[Beads (`bd`)](https://github.com/gastownhall/beads)** (Gas Town Hall / Steve Yegge) — MIT / Apache-2.0 Licenses  
  *Provides durable, graph-based issue tracking, blocker resolution, and multi-agent task coordination.*
- **[Dolt](https://www.dolthub.com/)** (DoltHub, Inc.) — Apache-2.0 License  
  *Provides the world's first version-controlled SQL relational database that powers distributed, peer-to-peer Beads issue synchronization.*

### 4. In-Container Session & Connectivity Tooling
- **[OpenSSH](https://www.openssh.com/)** (The OpenBSD Project) — BSD-style Licenses  
  *Enables unprivileged, zero-trust cryptographic SSH access into isolated agent sandboxes.*
- **[tmux](https://github.com/tmux/tmux)** (Nicholas Marriott & Contributors) — ISC License  
  *Provides terminal multiplexing, supervisor session management, and live operator inspection inside sandbox containers.*
- **[Git](https://git-scm.com/)** (Linus Torvalds, Junio C Hamano & Git Community) — GPL-2.0 License  
  *The ubiquitous version control system underpinning all agent version tracking and patch workflows.*

### 5. Developer Ecosystem & IDE Providers
- **[Visual Studio Code](https://code.visualstudio.com/) & [VSCodium](https://vscodium.com/)** — MIT License  
  *Enables desktop Remote-SSH development and agent pair-programming.*
- **[JetBrains](https://www.jetbrains.com/) Gateway & Toolbox** — JetBrains API Ecosystem  
  *Enables remote IDE backend attaching to in-container agent development environments.*

---

## Legal Notices & Third-Party Licenses

For formal legal copyright notices, SPDX license identifiers, and redistributable third-party license texts, please consult **[NOTICES.md](file:///home/brian/Workspaces/boggycreek/sandbox/NOTICES.md)**.
