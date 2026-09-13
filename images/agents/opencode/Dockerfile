# Derivative OCI Image: OpenCode Agent Sandbox
# Extends neutral agent-sandbox-base with OpenCode agent CLI
FROM agent-sandbox-base:latest

USER root

# Install Node.js (v20 LTS / current) and npm for OpenCode CLI runner
RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Configure npm prefix for non-root global tool installations
ENV NPM_CONFIG_PREFIX=/home/agent/.npm-global
ENV PATH="/home/agent/.npm-global/bin:${PATH}"

# Install opencode AI assistant CLI
RUN npm install -g opencode-ai || npm install -g opencode || true

# Copy default OpenCode settings
RUN mkdir -p /etc/opencode
COPY images/agents/opencode/opencode.json /etc/opencode/opencode.json

# Copy derivative entrypoint
COPY images/agents/opencode/opencode-entrypoint.sh /usr/local/bin/opencode-entrypoint.sh
RUN chmod +x /usr/local/bin/opencode-entrypoint.sh

# Ensure home directory ownership belongs to agent
RUN chown -R agent:agent /home/agent

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/opencode-entrypoint.sh"]
