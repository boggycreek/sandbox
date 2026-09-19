# Multi-stage build: 
# Stage 1: Build static bp and bpd CLI binaries
FROM docker.io/library/golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod ./
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bp ./cmd/bp && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bpd ./cmd/bpd

# Stage 2: Clean, neutral Base OCI Image
FROM docker.io/library/debian:bookworm-slim

# Install core CLI tools, unprivileged OpenSSH, tmux, git, and system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    openssh-server \
    openssh-client \
    openssl \
    locales \
    procps \
    tmux \
    jq \
    ripgrep \
    fd-find \
    tar \
    gzip \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Install Deno — system-wide, root-owned, available to all users
RUN curl -fsSL https://deno.land/install.sh | DENO_INSTALL=/usr/local sh

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
    mkdir -p /home/agent/workspace /home/agent/tasks /home/agent/.ssh /home/agent/.local/bin /home/agent/.config /home/agent/doc /home/agent/.cache/deno && \
    chown -R agent:agent /home/agent

# Configure OpenSSH daemon for unprivileged execution
COPY images/base/sshd_config /etc/ssh/sshd_config
RUN mkdir -p /var/run/sshd && chmod 755 /var/run/sshd

# Install static bp and bpd CLI binaries from builder
COPY --from=builder /build/bp /usr/local/bin/bp
COPY --from=builder /build/bpd /usr/local/bin/bpd
RUN chmod +x /usr/local/bin/bp /usr/local/bin/bpd

# Install in-container documentation
COPY images/base/doc/ /usr/local/share/doc/agent-sandbox/
COPY images/base/doc/ /home/agent/doc/
RUN chown -R agent:agent /home/agent/doc

# Install tmux configuration and entrypoint
# Install fleet-tasks coordinator helper
COPY images/base/fleet-tasks /usr/local/bin/fleet-tasks
RUN chmod 755 /usr/local/bin/fleet-tasks

COPY images/base/tmux.conf /home/agent/.tmux.conf
COPY images/base/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh && \
    chown agent:agent /home/agent/.tmux.conf

USER agent
WORKDIR /home/agent/workspace

ENV DENO_DIR=/home/agent/.cache/deno \
    PATH="/home/agent/.local/bin:/usr/local/bin:${PATH}" \
    HOME="/home/agent" \
    TERM=xterm-256color

EXPOSE 2222

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
