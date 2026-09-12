# Derivative OCI Image: Claude Code Agent Sandbox
# Extends neutral agent-sandbox-base with Claude Code CLI
FROM agent-sandbox-base:latest

USER root

# Install Node.js (v20/current LTS) and build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Configure npm prefix for non-root user
ENV NPM_CONFIG_PREFIX=/home/agent/.npm-global
ENV PATH="/home/agent/.npm-global/bin:${PATH}"

# Install Anthropic Claude Code CLI globally in npm-global
RUN npm install -g @anthropic-ai/claude-code

# Copy default settings with broad permissions (acceptEdits)
RUN mkdir -p /etc/claude
COPY images/agents/claude/claude-settings.json /etc/claude/settings.json

# Copy derivative entrypoint
COPY images/agents/claude/claude-entrypoint.sh /usr/local/bin/claude-entrypoint.sh
RUN chmod +x /usr/local/bin/claude-entrypoint.sh

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/claude-entrypoint.sh"]
