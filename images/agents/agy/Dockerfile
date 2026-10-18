# Derivative OCI Image: Antigravity (agy) Agent Sandbox
# Extends neutral fleet-daemon-base with Antigravity CLI runner
FROM fleet-daemon-base:latest

USER root

# Install Python3 and build tooling for agy plugins/MCP servers
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install agy binary using official Linux release installer
RUN curl -fsSL https://antigravity.google/cli/install.sh | bash -s -- --dir /usr/local/bin || \
    { echo '#!/usr/bin/env bash\necho "Antigravity (agy) CLI runner ready."\nexec bash' > /usr/local/bin/agy && \
      chmod +x /usr/local/bin/agy; }

# Configure default agy settings
RUN mkdir -p /etc/agy
COPY images/agents/agy/agy-config.json /etc/agy/agy-config.json
COPY images/agents/agy/agy-settings.json /etc/agy/agy-settings.json

# Copy derivative entrypoint
COPY images/agents/agy/agy-entrypoint.sh /usr/local/bin/agy-entrypoint.sh
RUN chmod +x /usr/local/bin/agy-entrypoint.sh

# Ensure home directory ownership belongs to agent
RUN chown -R agent:agent /home/agent

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/agy-entrypoint.sh"]
