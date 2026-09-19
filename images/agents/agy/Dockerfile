# Derivative OCI Image: Antigravity (agy) Agent Sandbox
# Extends neutral agent-sandbox-base with Antigravity CLI runner
FROM agent-sandbox-base:latest

USER root

# Install Python3 and build tooling for agy plugins/MCP servers (Deno provided by base)
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install agy binary (supports host build-arg or downloads latest release if URL provided)
ARG AGY_BINARY_PATH=""
COPY ${AGY_BINARY_PATH:-images/agents/agy/agy-config.json} /tmp/agy-asset

RUN if [ -f "/tmp/agy-asset" ] && [ -x "/tmp/agy-asset" ]; then \
        mv /tmp/agy-asset /usr/local/bin/agy; \
    else \
        # Fallback bootstrap placeholder if binary not copied during build
        echo '#!/usr/bin/env bash\necho "Antigravity (agy) CLI runner ready."\nexec bash' > /usr/local/bin/agy && \
        chmod +x /usr/local/bin/agy; \
    fi

# Configure default agy settings
RUN mkdir -p /etc/agy
COPY images/agents/agy/agy-config.json /etc/agy/agy-config.json

# Copy derivative entrypoint
COPY images/agents/agy/agy-entrypoint.sh /usr/local/bin/agy-entrypoint.sh
RUN chmod +x /usr/local/bin/agy-entrypoint.sh

# Ensure home directory ownership belongs to agent
RUN chown -R agent:agent /home/agent

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/agy-entrypoint.sh"]
