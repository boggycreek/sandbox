#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Network Egress Filter Sidecar Entrypoint (ADR 00032)
#
# Configures default-deny packet filtering via iptables and dynamic domain
# allowlisting via dnsmasq + ipset.

set -euo pipefail

# 1. Setup allowed-egress ipset
ipset create allowed-egress hash:ip -exist

# 2. Configure iptables firewall

# Flush existing rules and user-defined chains
iptables -F
iptables -X 2>/dev/null || true

# Set default policies to DROP
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

# Allow loopback (lo)
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

# Allow established and related connections
if iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT 2>/dev/null; then
  iptables -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
else
  iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
  iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
fi

# Allow traffic on backplane/infra network interface or subnet (10.89.0.0/16 and local subnet)
iptables -A INPUT -s 10.89.0.0/16 -j ACCEPT
iptables -A OUTPUT -d 10.89.0.0/16 -j ACCEPT

LOCAL_SUBNET=$(ip -4 route show dev eth0 2>/dev/null | grep -v default | awk '{print $1}' | head -n1 || true)
if [ -n "${LOCAL_SUBNET}" ] && [ "${LOCAL_SUBNET}" != "10.89.0.0/16" ]; then
  iptables -A INPUT -s "${LOCAL_SUBNET}" -j ACCEPT
  iptables -A OUTPUT -d "${LOCAL_SUBNET}" -j ACCEPT
fi

# Allow local DNS to port 53
iptables -A INPUT -p udp --dport 53 -j ACCEPT
iptables -A INPUT -p tcp --dport 53 -j ACCEPT
iptables -A OUTPUT -p udp --sport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --sport 53 -j ACCEPT

# Allow outbound DNS to upstream resolvers (1.1.1.1:53, 8.8.8.8:53)
iptables -A OUTPUT -p udp -d 1.1.1.1 --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp -d 1.1.1.1 --dport 53 -j ACCEPT
iptables -A OUTPUT -p udp -d 8.8.8.8 --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp -d 8.8.8.8 --dport 53 -j ACCEPT

# Allow outbound TCP 80/443 to allowed-egress ipset
iptables -A OUTPUT -p tcp --dport 80 -m set --match-set allowed-egress dst -j ACCEPT
iptables -A OUTPUT -p tcp --dport 443 -m set --match-set allowed-egress dst -j ACCEPT

# Allow inbound SSH on port 2222
iptables -A INPUT -p tcp --dport 2222 -j ACCEPT
iptables -A OUTPUT -p tcp --sport 2222 -j ACCEPT

# Dynamically add non-lo container interfaces to dnsmasq config if detected
for iface in $(ip -o link show 2>/dev/null | awk -F': ' '{print $2}' | grep -v '^lo$' || true); do
  if ! grep -q "^interface=${iface}" /etc/dnsmasq.d/egress.conf 2>/dev/null; then
    echo "interface=${iface}" >> /etc/dnsmasq.d/egress.conf
  fi
done

echo "[egress-filter] Initialized iptables and ipset. Starting dnsmasq..."

# Keep foreground with dnsmasq -k
exec dnsmasq -k
