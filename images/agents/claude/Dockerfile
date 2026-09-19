# Derivative OCI Image: Claude Code Agent Sandbox
# Extends neutral agent-sandbox-base with Claude Code CLI
FROM agent-sandbox-base:latest

USER root

# Install Python and build dependencies (Deno provided by base)
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Anthropic Claude Code CLI via Deno npm compatibility layer
RUN deno install --global --allow-scripts=npm:@anthropic-ai/claude-code -n claude npm:@anthropic-ai/claude-code

# Copy default settings with broad permissions (acceptEdits)
RUN mkdir -p /etc/claude
COPY images/agents/claude/claude-settings.json /etc/claude/settings.json

# Copy derivative entrypoint
COPY images/agents/claude/claude-entrypoint.sh /usr/local/bin/claude-entrypoint.sh
RUN chmod +x /usr/local/bin/claude-entrypoint.sh

# Ensure home directory ownership belongs to agent
RUN chown -R agent:agent /home/agent

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/claude-entrypoint.sh"]
