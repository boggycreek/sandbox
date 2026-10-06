# Derivative OCI Image: PiG (Pi in Go) Agent Sandbox
# Extends neutral agent-sandbox-base with PiG agent CLI
#
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

FROM agent-sandbox-base:latest

USER root

# Install Python, YAML, and build dependencies (Deno provided by base)
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    python3-yaml \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install PiG (Pi in Go) — official static binary release with checksum validation
ARG PIG_VERSION=0.4.1
ARG TARGETARCH
RUN case "${TARGETARCH:-amd64}" in \
        amd64) \
            PIG_ARCH="amd64" ; \
            PIG_SHA256="3b4de3587cbfb617def483ec62dddcf35827acc3436b1454cc5694c22dcdaf3f" ;; \
        arm64) \
            PIG_ARCH="arm64" ; \
            PIG_SHA256="e1768bbfce29b784d76001eb47ebd989f05ed042fbe81ad8629816e181fa5ac8" ;; \
        *) \
            echo "Unsupported architecture: ${TARGETARCH}" >&2 ; exit 1 ;; \
    esac && \
    curl -fsSL -o /tmp/pig.tar.gz "https://github.com/MichaelKinsy/PiG/releases/download/v${PIG_VERSION}/pig-${PIG_VERSION}-linux-${PIG_ARCH}.tar.gz" && \
    echo "${PIG_SHA256}  /tmp/pig.tar.gz" | sha256sum -c - && \
    tar -xzf /tmp/pig.tar.gz -C /tmp && \
    mv "/tmp/pig-${PIG_VERSION}-linux-${PIG_ARCH}/pig" /usr/local/bin/pig && \
    chmod +x /usr/local/bin/pig && \
    rm -rf /tmp/pig*

# Copy derivative entrypoint
COPY images/agents/pig/pig-entrypoint.sh /usr/local/bin/pig-entrypoint.sh
RUN chmod +x /usr/local/bin/pig-entrypoint.sh

# Ensure home directory ownership belongs to agent
RUN chown -R agent:agent /home/agent

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/pig-entrypoint.sh"]
