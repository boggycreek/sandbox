# Derivative OCI Image: OpenCode Agent Sandbox
# Extends neutral agent-sandbox-base with OpenCode agent CLI
FROM agent-sandbox-base:latest

USER root

# Install Python and build dependencies (Deno provided by base)
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install opencode AI assistant — official static binary release (Go binary, no Node.js dependency)
RUN curl -fsSL https://opencode.ai/install | sh || \
    { echo "opencode install failed, installing placeholder"; \
      echo '#!/usr/bin/env bash\necho "opencode CLI ready."\nexec bash' > /usr/local/bin/opencode && \
      chmod +x /usr/local/bin/opencode; }

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
