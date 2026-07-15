#!/usr/bin/env bash

# Print the public IP addresses currently advertised by the Traefik
# LoadBalancer. Hetzner includes the LoadBalancer's private-network address in
# the same status list, but that address must never be published in public DNS.
bex_ssh_public_ingress_addresses() {
  local namespace="${BEX_TRAEFIK_NAMESPACE:-traefik}"
  local service="${BEX_TRAEFIK_SERVICE:-traefik}"

  kubectl -n "$namespace" get service "$service" \
    -o jsonpath='{range .status.loadBalancer.ingress[*]}{.ip}{"\n"}{end}' |
    awk '
      function private_v4(ip, octets) {
        split(ip, octets, ".")
        return octets[1] == 10 ||
          (octets[1] == 172 && octets[2] >= 16 && octets[2] <= 31) ||
          (octets[1] == 192 && octets[2] == 168)
      }
      {
        ip = tolower($0)
        if (ip ~ /^[0-9]+\./ && private_v4(ip)) next
        if (ip ~ /:/ && (ip ~ /^f[cd]/ || ip ~ /^fe[89ab]/)) next
        if (ip != "") print ip
      }
    ' |
    sort -u
}
