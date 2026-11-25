#!/usr/bin/env bash
# Pins the shape of deny-send-only-private-ingress (w6/m640, ADR022 §Send-only
# pods): it selects app.bex.co/private-network=send-only alone, keeps
# enableDefaultDeny.ingress false with no egress default, and denies exactly
# one source: endpoints carrying app.bex.co/workspace, in any namespace.
#
# Each guarded change is an outage or a leak. Default deny (ingress or egress)
# would cut kubelet probes, Traefik or every worker's outbound traffic. A
# dropped or widened selector would refuse private traffic to paid web and
# private services, which receive it on Render (w9/m57). A wider source
# (an entity, a CIDR, an empty endpoint selector) would refuse the platform's
# own probes, edge and port reads. A narrower one (a namespace or extra match)
# would let a sibling through.
#
# Usage: bash scripts/private-ingress-deny-check.sh [manifest]
# (default deploy/gitops/base/tenant-node-egress.yaml). Run by
# scripts/gitops-validate.sh; scripts/private-ingress-deny-check.test.sh
# proves each failure.
set -euo pipefail
cd "$(dirname "$0")/.."

manifest="${1:-deploy/gitops/base/tenant-node-egress.yaml}"
policy=deny-send-only-private-ingress
want='spec=enableDefaultDeny,endpointSelector,ingressDeny|selector=matchLabels|labels=app.bex.co/private-network=send-only|default=ingress:false|deny=1|rule=fromEndpoints|endpoints=1|endpoint=matchExpressions|exprs=app.bex.co/workspace/Exists/'

export POLICY="$policy"
selected='select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == strenv(POLICY))'

count="$(yq ea "[$selected] | length" "$manifest")"
if [ "$count" != 1 ]; then
  echo "FAIL: $manifest must hold exactly one CiliumClusterwideNetworkPolicy $policy; found $count" >&2
  exit 1
fi

# $s is a yq variable, not a shell one.
# shellcheck disable=SC2016
got="$(yq ea "$selected"' | .spec as $s |
  ["spec=" + ($s | keys | sort | join(",")),
   "selector=" + ($s.endpointSelector | keys | sort | join(",")),
   "labels=" + (($s.endpointSelector.matchLabels // {}) | to_entries | map(.key + "=" + (.value | tostring)) | join(",")),
   "default=" + (($s.enableDefaultDeny // {}) | to_entries | map(.key + ":" + (.value | tostring)) | join(",")),
   "deny=" + (($s.ingressDeny // []) | length | tostring),
   "rule=" + (($s.ingressDeny[0] // {}) | keys | sort | join(",")),
   "endpoints=" + (($s.ingressDeny[0].fromEndpoints // []) | length | tostring),
   "endpoint=" + (($s.ingressDeny[0].fromEndpoints[0] // {}) | keys | sort | join(",")),
   "exprs=" + (($s.ingressDeny[0].fromEndpoints[0].matchExpressions // []) | map(.key + "/" + .operator + "/" + ((.values // []) | join(","))) | join(";"))
  ] | join("|")' "$manifest" | tr -d '\n')"

if [ "$got" != "$want" ]; then
  echo "FAIL: $policy must select app.bex.co/private-network=send-only alone, keep enableDefaultDeny.ingress=false and deny only fromEndpoints with app.bex.co/workspace Exists (kubelet probes, Traefik, the activator and the operator carry no workspace label)" >&2
  echo "  want: $want" >&2
  echo "  got:  $got" >&2
  exit 1
fi
echo "ok: $policy denies only workspace-labeled sources to send-only pods"
