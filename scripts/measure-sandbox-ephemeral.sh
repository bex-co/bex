#!/usr/bin/env bash
# Production proof leg for w22/008: a sandbox carries Render's 40 GB writable
# filesystem, and the kubelet enforces it.
#
#   1. the sandbox pod's container has an ephemeral-storage limit of 40Gi (and a
#      request of a quarter of it);
#   2. a sandbox can write up to the limit without being evicted (2 GiB steps up
#      to BEX_MEASURE_BELOW_GIB, default 38, the kubelet's usedBytes following);
#   3. writing past it ends the sandbox (the kubelet evicts the pod), and bex
#      reports it as ended, with the termination reason it already names for an
#      ephemeral-storage eviction ("ran out of disk space").
#
# Run by sandbox-measure.yml (leg `ephemeral`) through
# scripts/verify-sandbox-isolation-live.sh. One sandbox, up to about 44 GiB
# written to the node's disk (released when the pod ends), everything printed as
# `MEASURE <name> <json>`.

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

below_gib="${BEX_MEASURE_BELOW_GIB:-38}"
past_gib="${BEX_MEASURE_PAST_GIB:-6}"

echo "==> create a sandbox and read its pod's limits"
sandbox_id="$(create_sandbox "$fixture_dir/sandbox.json" '{"type":"deny-all"}')"
wait_status "$sandbox_id" running 300
read -r pod node cr < <(find_pod "$sandbox_id")
[ -n "${pod:-}" ] && [ -n "${node:-}" ] || fail "no pod or node found for sandbox $sandbox_id"
resources="$(kubectl -n "$namespace_a" get pod "$pod" -o json | jq -c '[.spec.containers[] | select(.name | test("execd") | not) | {name, limits: .resources.limits, requests: .resources.requests}][0]')"
measure pod-resources "$(jq -cn --arg pod "$pod" --argjson r "$resources" '{pod: $pod, container: $r}')"
check "the sandbox container's ephemeral-storage limit is 40Gi" \
  "$(jq -r '.limits["ephemeral-storage"] == "40Gi"' <<<"$resources")" "$resources"

used_bytes() { # used_bytes: the pod's ephemeral-storage usedBytes from the kubelet summary
  node_summary "$node" "$fixture_dir/summary.json"
  jq -r --arg ns "$namespace_a" --arg p "$pod" '[.pods[] | select(.podRef.namespace == $ns and .podRef.name == $p)][0]["ephemeral-storage"].usedBytes // "null"' "$fixture_dir/summary.json"
}

echo "==> write up to $below_gib GiB in 2 GiB steps: no eviction below the limit"
step=0
alive=true
while [ $((step * 2)) -lt "$below_gib" ]; do
  step=$((step + 1))
  if ! sandbox_exec "$sandbox_id" "dd if=/dev/zero of=/root/fill-$step bs=1M count=2048 conv=fsync 2>&1 | tail -1" "$fixture_dir/fill-$step.txt"; then
    measure below-limit-step "$(jq -cn --argjson s "$step" --arg o "$(head -c 300 "$fixture_dir/fill-$step.sse" 2>/dev/null | tr '\n' ' ')" '{step: $s, failed: true, detail: $o}')"
    alive=false
    break
  fi
  if [ $((step % 4)) = 0 ]; then
    sleep 15
    measure below-limit-step "$(jq -cn --argjson s "$step" --argjson w "$((step * 2 * 1024 * 1024 * 1024))" --arg u "$(used_bytes)" --arg st "$(sandbox_status "$sandbox_id")" \
      '{step: $s, writtenBytes: $w, kubeletUsedBytes: $u, sandboxStatus: $st}')"
  fi
done
status_below="$(sandbox_status "$sandbox_id")"
check "the sandbox is still running after writing $((step * 2)) GiB" "$([ "$status_below" = running ] && [ "$alive" = true ] && echo true || echo false)" "status $status_below"

echo "==> write past the limit: the kubelet ends the sandbox"
evicted=false
empty_object='{}'
for extra in $(seq 1 "$past_gib"); do
  step=$((step + 1))
  sandbox_exec "$sandbox_id" "dd if=/dev/zero of=/root/fill-$step bs=1M count=2048 conv=fsync 2>&1 | tail -1" "$fixture_dir/fill-$step.txt" || true
  sleep 12
  phase="$(kubectl -n "$namespace_a" get pod "$pod" -o json 2>/dev/null | jq -c '{phase: .status.phase, reason: .status.reason, message: .status.message}' || echo '{}')"
  case "$phase" in *Evicted*) evicted=true ;; esac
  measure past-limit-step "$(jq -cn --argjson s "$step" --argjson w "$((step * 2 * 1024 * 1024 * 1024))" --argjson p "${phase:-$empty_object}" --arg st "$(sandbox_status "$sandbox_id")" \
    '{step: $s, writtenBytes: $w, pod: $p, sandboxStatus: $st}')"
  [ "$evicted" = true ] && break
done
for _ in $(seq 1 24); do
  final="$(sandbox_status "$sandbox_id")"
  case "$final" in running | suspending | resuming | creating) sleep 5 ;; *) break ;; esac
done
api_ok=false
[ "$(api_call "$fixture_dir/owner.curl" GET "/v1/sandboxes/$sandbox_id?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/final.json")" = 200 ] && api_ok=true
measure final "$(jq -cn --argjson ev "$evicted" --argjson ok "$api_ok" --arg st "${final:-}" --argjson body "$(jq -c '.' "$fixture_dir/final.json" 2>/dev/null || echo '{}')" \
  '{podEvicted: $ev, apiRead: $ok, sandboxStatus: $st, sandbox: ($body | del(.env, .networkPolicy))}')"
check "the kubelet evicted the pod that wrote past 40Gi" "$evicted" "no Evicted pod after writing $((step * 2)) GiB"
check "bex no longer reports the sandbox as running" "$([ "${final:-running}" != running ] && echo true || echo false)" "status ${final:-?}"

[ "$failures" = 0 ] || fail "$failures ephemeral-storage check(s) failed (see the lines above)"
pass "ephemeral leg: a sandbox has 40Gi, can use it, and is ended past it"
