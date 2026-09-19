# Network Egress Filter Sidecar OCI Image (ADR 00032)
#
# Provides default-deny egress packet filtering and DNS snooping via dnsmasq + iptables + ipset.
FROM docker.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    dnsmasq \
    iptables \
    ipset \
    iproute2 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Ensure dnsmasq configuration directory exists
RUN mkdir -p /etc/dnsmasq.d && \
    echo "conf-dir=/etc/dnsmasq.d/,*.conf" >> /etc/dnsmasq.conf

COPY images/egress-filter/dnsmasq.conf /etc/dnsmasq.d/egress.conf
COPY images/egress-filter/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 53/udp 53/tcp 2222/tcp

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
