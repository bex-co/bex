#!/usr/bin/env bash

# shellcheck source=scripts/lib/curl-auth.sh
source "$(dirname "${BASH_SOURCE[0]}")/curl-auth.sh"

# Print the public IP addresses of the Terraform-owned production edge. The
# Kubernetes Traefik Service is intentionally a NodePort and has no
# status.loadBalancer ingress after the shared-LB ownership fix.
bex_ssh_public_ingress_addresses() {
  local name="${BEX_EDGE_LOAD_BALANCER:-bex-traefik}"
  local response count
  : "${HCLOUD_TOKEN:?set the Hetzner Cloud API token}"

  # The token reaches curl on stdin, never in its argv (w3/m472).
  response="$(bex_curl_bearer HCLOUD_TOKEN --fail --silent --show-error \
    "https://api.hetzner.cloud/v1/load_balancers?name=${name}")"
  count="$(jq -r '.load_balancers | length' <<<"$response")"
  if [[ "$count" != 1 ]]; then
    echo "expected exactly one Hetzner Load Balancer named $name, found $count" >&2
    return 1
  fi
  jq -r '
    .load_balancers[0].public_net |
    select(.enabled == true) |
    .ipv4.ip, .ipv6.ip |
    select(. != null and . != "")
  ' <<<"$response" | sort -u
}
