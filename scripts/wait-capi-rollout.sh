#!/usr/bin/env bash
# Wait until the self-managed production CAPI fleet has converged to each
# controller's declared version. KCP and MachineDeployments may intentionally be
# staged at different minors; this checks each owner against its own desired
# version instead of requiring the whole fleet to change at once.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-bex}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-3600}"
POLL_SECONDS="${POLL_SECONDS:-10}"
KUBECTL_GET_RETRIES="${KUBECTL_GET_RETRIES:-5}"
KUBECTL_REQUEST_TIMEOUT="${KUBECTL_REQUEST_TIMEOUT:-30s}"

case "$TIMEOUT_SECONDS:$POLL_SECONDS:$KUBECTL_GET_RETRIES" in
  *[!0-9:]* | :* | *: | *::*) echo "TIMEOUT_SECONDS, POLL_SECONDS, and KUBECTL_GET_RETRIES must be positive integers" >&2; exit 2 ;;
esac
[ "$TIMEOUT_SECONDS" -gt 0 ] && [ "$POLL_SECONDS" -gt 0 ] && [ "$KUBECTL_GET_RETRIES" -gt 0 ] \
  || { echo "TIMEOUT_SECONDS, POLL_SECONDS, and KUBECTL_GET_RETRIES must be positive integers" >&2; exit 2; }

deadline=$((SECONDS + TIMEOUT_SECONDS))
attempt=0

# A self-managed control-plane replacement can briefly interrupt an individual
# API request even while quorum and the load balancer remain healthy. Treat a
# bounded read timeout as a retryable observation failure; a persistent failure
# still aborts the gate instead of being mistaken for convergence.
kubectl_get() {
  local try output
  for ((try = 1; try <= KUBECTL_GET_RETRIES; try++)); do
    if output=$(kubectl --request-timeout="$KUBECTL_REQUEST_TIMEOUT" get "$@"); then
      printf '%s\n' "$output"
      return 0
    fi
    echo "warning: kubectl get failed ($try/$KUBECTL_GET_RETRIES); retrying" >&2
    sleep 2
  done
  return 1
}

machine_group_ready() {
  local selector="$1" desired="$2" expected="$3" group="$4"
  local machines count bad node got
  machines=$(kubectl_get machines -n default -l "$selector" -o json)
  count=$(jq '.items | length' <<<"$machines")
  bad=$(jq --arg desired "$desired" '[.items[] |
    select(.spec.version != $desired or .status.phase != "Running" or .status.nodeRef.name == null)] |
    length' <<<"$machines")
  if [ "$count" -ne "$expected" ] || [ "$bad" -ne 0 ]; then
    return 1
  fi

  while IFS= read -r node; do
    got=$(kubectl_get node "$node" -o jsonpath='{.status.nodeInfo.kubeletVersion}')
    if [ "$got" != "$desired" ]; then
      return 1
    fi
  done < <(jq -r '.items[].status.nodeRef.name' <<<"$machines")

  printf '  %s: %s/%s Running+Ready at %s\n' "$group" "$count" "$expected" "$desired"
}

while [ "$SECONDS" -lt "$deadline" ]; do
  attempt=$((attempt + 1))
  converged=true
  summary=""

  kcp=$(kubectl_get kubeadmcontrolplane "${CLUSTER_NAME}-control-plane" -n default -o json)
  kcp_desired=$(jq -r '.spec.version' <<<"$kcp")
  kcp_replicas=$(jq -r '.spec.replicas // 0' <<<"$kcp")
  kcp_ready=$(jq -r '.status.readyReplicas // 0' <<<"$kcp")
  kcp_available=$(jq -r '[.status.conditions[]? | select(.type == "Available") | .status][0] // "False"' <<<"$kcp")
  kcp_rolling=$(jq -r '[.status.conditions[]? | select(.type == "RollingOut") | .status][0] // "False"' <<<"$kcp")
  if [ "$kcp_ready" -ne "$kcp_replicas" ] || [ "$kcp_available" != "True" ] || [ "$kcp_rolling" != "False" ]; then
    converged=false
  elif ! summary=$(machine_group_ready \
    'cluster.x-k8s.io/control-plane' "$kcp_desired" "$kcp_replicas" 'control-plane'); then
    converged=false
  fi

  mds=$(kubectl_get machinedeployments -n default -o json)
  while IFS= read -r md; do
    md_json=$(kubectl_get machinedeployment "$md" -n default -o json)
    md_desired=$(jq -r '.spec.template.spec.version' <<<"$md_json")
    md_replicas=$(jq -r '.spec.replicas // 0' <<<"$md_json")
    md_ready=$(jq -r '.status.readyReplicas // 0' <<<"$md_json")
    md_available=$(jq -r '.status.availableReplicas // 0' <<<"$md_json")
    if [ "$md_ready" -ne "$md_replicas" ] || [ "$md_available" -ne "$md_replicas" ]; then
      converged=false
    elif ! md_summary=$(machine_group_ready \
      "cluster.x-k8s.io/deployment-name=$md" "$md_desired" "$md_replicas" "$md"); then
      converged=false
    else
      summary="${summary}${summary:+$'\n'}${md_summary}"
    fi
  done < <(jq -r '.items[].metadata.name' <<<"$mds")

  if [ "$converged" = true ]; then
    echo "CAPI fleet converged:"
    printf '%s\n' "$summary"
    exit 0
  fi

  if (( attempt == 1 || attempt % 6 == 0 )); then
    echo "[$attempt] waiting for CAPI fleet convergence..."
    kubectl get kubeadmcontrolplane,machinedeployment,machines -n default -o wide \
      || echo "warning: diagnostic fleet read failed" >&2
  fi
  sleep "$POLL_SECONDS"
done

echo "timed out after ${TIMEOUT_SECONDS}s waiting for the CAPI fleet" >&2
kubectl get kubeadmcontrolplane,machinedeployment,machines,nodes -A -o wide >&2 \
  || echo "warning: final diagnostic fleet read failed" >&2
exit 1
