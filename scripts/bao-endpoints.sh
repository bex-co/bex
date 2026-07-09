#!/usr/bin/env bash
# Shared OpenBao endpoint resolution for bao-init.sh / bao-k8s-auth.sh.
# Sourced, never executed directly — defines two functions:
#
#   bao_resolve_endpoints [namespace]   builds the ordered list of reachable
#       OpenBao endpoints and sets the global array `bao_urls` (plus
#       `bao_leader` = its first element, the raft leader). When BAO_ADDR is
#       set, that single off-cluster endpoint is the list. Otherwise every
#       OpenBao pod is port-forwarded directly — the `openbao` Service
#       round-robins across sealed+unsealed members, so per-pod is the only
#       reliable target in HA (docs/secrets.md §3). Pods are visited in ordinal
#       order so openbao-0 leads init. Launches background port-forwards whose
#       PIDs land in the global `BAO_PF_PIDS` for the caller's cleanup trap.
#
#   bao_cleanup_forwards                tears down those port-forwards.
#
# Keeping this logic in one place is what lets both prod-deploy scripts target
# the leader directly instead of the round-robin Service (the HA fix
# bao-init.sh already had, now shared with bao-k8s-auth.sh).
bao_resolve_endpoints() {
  local NS="${1:-secrets}"
  local port="${BAO_PF_PORT_START:-38200}"
  BAO_PF_PIDS=()
  bao_urls=()
  if [ -n "${BAO_ADDR:-}" ]; then
    bao_urls=("$BAO_ADDR")
  else
    local pods=() pod local_url ready
    mapfile -t pods < <(kubectl -n "$NS" get pods -l app.kubernetes.io/name=openbao \
      -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | tr ' ' '\n' | sort)
    [ "${#pods[@]}" -gt 0 ] || { echo "error: no openbao pods found in ns $NS (is the OpenBao rollout done?)" >&2; exit 1; }
    for pod in "${pods[@]}"; do
      kubectl -n "$NS" port-forward "pod/$pod" "${port}:8200" >/dev/null 2>&1 &
      BAO_PF_PIDS+=("$!")
      local_url="http://127.0.0.1:${port}"
      # Wait until the forward is up AND the listener is really OpenBao (a
      # seal-status body with a `sealed` field). A foreign process already bound
      # to this port would otherwise be treated as the pod and handed the unseal
      # keys / root token — so validate the payload, not just "got a response".
      ready=""
      for _ in $(seq 1 30); do
        if curl -sf "$local_url/v1/sys/seal-status" 2>/dev/null | yq -e 'has("sealed")' - >/dev/null 2>&1; then
          ready=1; break
        fi
        sleep 2
      done
      [ -n "$ready" ] || { echo "error: pod/$pod never answered a valid OpenBao seal-status on :${port} (port already in use?)" >&2; exit 1; }
      bao_urls+=("$local_url")
      port=$((port + 1))
    done
  fi
  bao_leader="${bao_urls[0]}"
}

bao_cleanup_forwards() {
  for pid in "${BAO_PF_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
}
