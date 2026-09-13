# Multi-stage build: 
# Stage 1: Build static bp CLI binary
FROM docker.io/library/golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod ./
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bp ./cmd/bp

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

# Symlink fdfind to fd
RUN ln -s $(which fdfind) /usr/local/bin/fd || true

# Generate default locale
RUN echo "en_US.UTF-8 UTF-8" > /etc/locale.gen && locale-gen
ENV LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8

# Create unprivileged agent user (UID 1000, GID 1000)
RUN groupadd -g 1000 agent && \
    useradd -u 1000 -g agent -m -s /bin/bash agent && \
    mkdir -p /home/agent/workspace /home/agent/.ssh /home/agent/.local/bin /home/agent/.config /home/agent/doc && \
    chown -R agent:agent /home/agent

# Configure OpenSSH daemon for unprivileged execution
COPY images/base/sshd_config /etc/ssh/sshd_config
RUN mkdir -p /var/run/sshd && chmod 755 /var/run/sshd

# Install static bp CLI binary from builder
COPY --from=builder /build/bp /usr/local/bin/bp
RUN chmod +x /usr/local/bin/bp

# Install in-container documentation
COPY images/base/doc/ /usr/local/share/doc/agent-sandbox/
COPY images/base/doc/ /home/agent/doc/
RUN chown -R agent:agent /home/agent/doc

# Install tmux configuration and entrypoint
COPY images/base/tmux.conf /home/agent/.tmux.conf
COPY images/base/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh && \
    chown agent:agent /home/agent/.tmux.conf

USER agent
WORKDIR /home/agent/workspace

ENV PATH="/home/agent/.local/bin:/usr/local/bin:${PATH}" \
    HOME="/home/agent" \
    TERM=xterm-256color

EXPOSE 2222

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
