# Interactive Tool Base OCI Image (ADR 00043)
# Extends fleet-core with foreground interactive tool execution
FROM fleet-core:latest

USER root

# Install tool entrypoint
COPY images/agent-tool-base/tool-entrypoint.sh /usr/local/bin/tool-entrypoint.sh
RUN chmod +x /usr/local/bin/tool-entrypoint.sh

USER agent
WORKDIR /home/agent/workspace

ENTRYPOINT ["/usr/local/bin/tool-entrypoint.sh"]
