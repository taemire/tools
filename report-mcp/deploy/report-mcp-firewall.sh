#!/bin/sh
set -eu
# Add only report API and the serialized renderer's local CDP connection.
for port in 8891 9223; do
  iptables -w -C ARTEX_LAB_OUTPUT -d 127.0.0.1/32 -p tcp --dport "$port" -j ACCEPT 2>/dev/null ||
    iptables -w -I ARTEX_LAB_OUTPUT 1 -d 127.0.0.1/32 -p tcp --dport "$port" -j ACCEPT
done
