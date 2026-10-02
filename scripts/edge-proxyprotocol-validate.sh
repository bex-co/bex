#!/usr/bin/env bash
# The edge's PROXY-protocol contract, end to end — pure client-side, no cluster
# (w1/m150). Every Hetzner load-balancer listener and the Traefik entrypoint
# behind it must agree on whether a PROXY header is sent and who may send it:
#
#   1. Each Terraform edge listener maps its port to the pinned NodePort (with a
#      health check on the same port) and sets the proxyprotocol value in EDGE
#      below. http/https/postgres/valkey send the LB's own header, so the
#      backend sees the real client; ssh does not (its own PROXY chain runs
#      Traefik → ssh-gateway, w4/m82, w6/m132).
#   2. Traefik's web and websecure entrypoints trust PROXY headers from exactly
#      the load balancer's private address, 10.10.0.7/32 — never wider, never
#      `insecure`. A wider trust set lets any peer in it assert a client address,
#      which is what an inbound IP allow-list matches on.
#
# The two halves fail differently if they drift apart: a listener that sends a
# header to an entrypoint that does not expect it breaks all HTTP(S); an
# entrypoint that trusts headers while its listener sends none believes a
# header the CLIENT writes (the LB is byte-transparent), so any client could
# spoof its source past an allow-list — observed live on 2026-10-01 between the
# m150 t001 and t002 rollouts.
#
# Called by scripts/gitops-validate.sh; scripts/edge-proxyprotocol-validate.test.sh
# drives it against mutated copies via TERRAFORM_MAIN / TRAEFIK_PROD_VALUES.
# Requires: yq v4.
set -euo pipefail
cd "$(dirname "$0")/.."

TERRAFORM_MAIN="${TERRAFORM_MAIN:-infra/terraform/main.tf}"
TRAEFIK_PROD_VALUES="${TRAEFIK_PROD_VALUES:-deploy/gitops/overlays/prod/values/traefik.values.yaml}"
LB_PRIVATE_CIDR=10.10.0.7/32

# listener:listen_port:node_port:proxyprotocol
EDGE=(
  ssh:22:32207:false
  http:80:31218:true
  https:443:31976:true
  postgres:5432:31056:true
  valkey:6379:31892:true
)

fail=0

for edge in "${EDGE[@]}"; do
  IFS=: read -r name listen destination proxyprotocol <<<"$edge"
  block="$(awk -v name="$name" '
    $0 == "resource \"hcloud_load_balancer_service\" \"" name "\" {" { found=1 }
    found { print }
    found && /^}$/ { exit }
  ' "$TERRAFORM_MAIN")"
  if [ -z "$block" ] ||
    ! grep -Eq "^[[:space:]]*listen_port[[:space:]]*=[[:space:]]*${listen}[[:space:]]*$" <<<"$block" ||
    ! grep -Eq "^[[:space:]]*destination_port[[:space:]]*=[[:space:]]*${destination}[[:space:]]*$" <<<"$block" ||
    ! grep -Eq "^[[:space:]]*port[[:space:]]*=[[:space:]]*${destination}[[:space:]]*$" <<<"$block"; then
    echo "FAIL: Terraform edge listener $name must map :$listen to NodePort $destination with the same health-check port" >&2
    fail=1
  fi
  if ! grep -Eq "^[[:space:]]*proxyprotocol[[:space:]]*=[[:space:]]*${proxyprotocol}[[:space:]]*$" <<<"$block"; then
    echo "FAIL: Terraform edge listener $name must set proxyprotocol=$proxyprotocol" >&2
    fail=1
  fi
done

for entrypoint in web websecure; do
  trust="$(yq -N -o=json -I=0 ".ports.${entrypoint}.proxyProtocol" "$TRAEFIK_PROD_VALUES")"
  if [ "$trust" != "{\"trustedIPs\":[\"$LB_PRIVATE_CIDR\"]}" ]; then
    echo "FAIL: Traefik entrypoint $entrypoint proxyProtocol is '$trust', want trustedIPs exactly [$LB_PRIVATE_CIDR] and nothing else" >&2
    fail=1
  fi
done

exit "$fail"
