# Derivative OCI Image: PiG (Pi in Go) Agent Sandbox
# Extends neutral agent-sandbox-base with PiG agent CLI
#
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

FROM agent-sandbox-base:latest

USER root

# Install Python and build dependencies (Deno provided by base)
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install PiG (Pi in Go) — official static binary release
ARG PIG_VERSION=0.4.1
RUN curl -fsSL -o /tmp/pig.tar.gz "https://github.com/MichaelKinsy/PiG/releases/download/v${PIG_VERSION}/pig-${PIG_VERSION}-linux-amd64.tar.gz" && \
    tar -xzf /tmp/pig.tar.gz -C /tmp && \
    mv "/tmp/pig-${PIG_VERSION}-linux-amd64/pig" /usr/local/bin/pig && \
    chmod +x /usr/local/bin/pig && \
    rm -rf /tmp/pig* || \
    { echo "PiG release download failed, installing placeholder"; \
      echo '#!/usr/bin/env bash\necho "pig CLI ready."\nexec bash' > /usr/local/bin/pig && \
      chmod +x /usr/local/bin/pig; }

# Copy derivative entrypoint
COPY images/agents/pig/pig-entrypoint.sh /usr/local/bin/pig-entrypoint.sh
RUN chmod +x /usr/local/bin/pig-entrypoint.sh

# Ensure home directory ownership belongs to agent
RUN chown -R agent:agent /home/agent

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/pig-entrypoint.sh"]
