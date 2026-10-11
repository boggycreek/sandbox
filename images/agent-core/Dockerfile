# Multi-stage build:
# Stage 1: Build static bp, bpd, and MCP CLI binaries
FROM docker.io/library/golang:1.27-bookworm AS builder
WORKDIR /src
COPY go.work ./
COPY agent/ ./agent/
COPY backplane/ ./backplane/
COPY mcp/ ./mcp/
COPY sandbox/ ./sandbox/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bp ./backplane/cmd/bp && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bpd ./backplane/cmd/bpd && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bp-mcp ./mcp/cmd/bp-mcp && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/gitea-mcp ./mcp/cmd/gitea-mcp && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/beads-mcp ./mcp/cmd/beads-mcp && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/sonar-mcp ./mcp/cmd/sonar-mcp

# Stage 2: Foundational Core OCI Image (ADR 00043)
FROM docker.io/library/debian:bookworm-slim

# Install core CLI tools, git, and system utilities (no openssh-server, no tmux)
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    openssh-client \
    openssl \
    locales \
    procps \
    jq \
    ripgrep \
    fd-find \
    tar \
    gzip \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Install Beads (bd) CLI — system-wide, root-owned, available to all users
ARG BEADS_VERSION=1.3.0
RUN TARGETARCH=$(dpkg --print-architecture) && \
    case "${TARGETARCH}" in \
        amd64) BEADS_ARCH="linux_amd64" ;; \
        arm64) BEADS_ARCH="linux_arm64" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/gastownhall/beads/releases/download/v${BEADS_VERSION}/beads_${BEADS_VERSION}_${BEADS_ARCH}.tar.gz" | \
    tar -xzf - -C /usr/local/bin bd && \
    chmod 755 /usr/local/bin/bd

# Symlink fdfind to fd
RUN ln -s $(which fdfind) /usr/local/bin/fd || true

# Generate default locale
RUN echo "en_US.UTF-8 UTF-8" > /etc/locale.gen && locale-gen
ENV LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8

# Create unprivileged agent user (UID 1000, GID 1000)
RUN groupadd -g 1000 agent && \
    useradd -u 1000 -g agent -m -s /bin/bash agent && \
    mkdir -p /home/agent/workspace /home/agent/tasks /home/agent/.ssh /home/agent/.local/bin /home/agent/.config /home/agent/doc && \
    chown -R agent:agent /home/agent

# Install static CLI binaries from builder
COPY --from=builder /build/bp /usr/local/bin/bp
COPY --from=builder /build/bpd /usr/local/bin/bpd
COPY --from=builder /build/bp-mcp /usr/local/bin/bp-mcp
COPY --from=builder /build/gitea-mcp /usr/local/bin/gitea-mcp
COPY --from=builder /build/beads-mcp /usr/local/bin/beads-mcp
COPY --from=builder /build/sonar-mcp /usr/local/bin/sonar-mcp
RUN chmod +x /usr/local/bin/bp /usr/local/bin/bpd /usr/local/bin/bp-mcp /usr/local/bin/gitea-mcp /usr/local/bin/beads-mcp /usr/local/bin/sonar-mcp

# Install in-container documentation
COPY images/agent-base/doc/ /usr/local/share/doc/agent-sandbox/
COPY images/agent-base/doc/ /home/agent/doc/
RUN chown -R agent:agent /home/agent/doc

USER agent
WORKDIR /home/agent/workspace

ENV PATH="/home/agent/.local/bin:/usr/local/bin:${PATH}" \
    HOME="/home/agent" \
    TERM=xterm-256color
