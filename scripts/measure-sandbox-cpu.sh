#!/usr/bin/env bash
# Production measurement leg for w22/m314 t001: does cAdvisor report usable
# per-sandbox CPU for a gVisor (runsc) pod, as the figure a "billed on use"
# meter would read from Prometheus? The thresholds are fixed in
# the board item w22/m314 t001 before any measurement:
#
#   accuracy  a one-core busy load must read its CPU-seconds within +-5%;
#   idle      an idle running sandbox must read at most 2% of one core;
#   syscalls  a syscall-heavy load must read at most 2x the same load under runc.
#
# Run by sandbox-measure.yml (leg `cpu`) through
# scripts/verify-sandbox-isolation-live.sh, which provides one disposable,
# billing-excluded workspace. One sandbox is created; every number is printed as
# `MEASURE <name> <json>`. Bounded: BEX_MEASURE_BURSTS x 60 s of one-core load
# (default 10), BEX_MEASURE_IDLE_SECONDS of idle (default 600), 3 x 60 s of
# syscall-heavy load, one reference pod for the runc comparison.

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

bursts="${BEX_MEASURE_BURSTS:-10}"
idle_seconds="${BEX_MEASURE_IDLE_SECONDS:-600}"
sys_runs=3

echo "==> create a sandbox and find its pod"
sandbox_id="$(create_sandbox "$fixture_dir/sandbox.json" '{"type":"deny-all"}')"
wait_status "$sandbox_id" running 300
read -r pod node cr < <(find_pod "$sandbox_id")
[ -n "${pod:-}" ] && [ -n "${node:-}" ] || fail "no pod or node found for sandbox $sandbox_id"
measure pod "$(jq -cn --arg ns "$namespace_a" --arg pod "$pod" --arg node "$node" '{namespace: $ns, pod: $pod, node: $node}')"
prom_forward
[ -n "$prom_url" ] || fail "Prometheus is not reachable through a port-forward"
tools="$(sandbox_exec "$sandbox_id" 'for t in timeout dd bash; do command -v $t >/dev/null 2>&1 && printf "%s " $t; done; echo' "$fixture_dir/tools.txt" && tr -d '\n' <"$fixture_dir/tools.txt" || true)"
measure tools "$(jq -cn --arg t "$tools" '{available: $t}')"
printf '%s' "$tools" | grep -qw timeout && printf '%s' "$tools" | grep -qw dd || fail "the sandbox lacks timeout or dd (have: $tools)"

# cpu_prom <pod> <namespace>: the pod's cumulative CPU seconds in Prometheus,
# summed over its series with a container label (the job drops container="").
cpu_prom() {
  prom_query "sum(last_over_time(container_cpu_usage_seconds_total{namespace=\"$2\",pod=\"$1\",container!=\"\"}[3m]))" \
    | jq -r '.[0].value[1] // "null"'
}
# cpu_cadvisor <pod> <node>: the pod-level cgroup's cumulative CPU seconds read
# straight from the node's cAdvisor. Under gVisor the workload container's own
# cgroup carries no series, so the only CPU series cAdvisor holds for a sandbox
# pod have container="" (the pod slice and the pause container): exactly the
# series Prometheus' kubernetes-cadvisor job drops. Pod slice = id ends in .slice.
cpu_cadvisor() {
  node_cadvisor "$2" "$fixture_dir/cadvisor-cpu.txt"
  grep -F "pod=\"$1\"" "$fixture_dir/cadvisor-cpu.txt" | grep '^container_cpu_usage_seconds_total' | grep 'id="[^"]*\.slice"' | awk '{print $2}' | head -1
}
# cpu_kubelet <pod> <namespace> <node>: the same, from the kubelet summary at this instant.
cpu_kubelet() {
  node_summary "$3" "$fixture_dir/summary-cpu.json"
  jq -r --arg ns "$2" --arg p "$1" '[.pods[] | select(.podRef.namespace == $ns and .podRef.name == $p)][0].cpu.usageCoreNanoSeconds // empty | . / 1e9' "$fixture_dir/summary-cpu.json"
}
# window <name> <pod> <ns> <node> <start prom> <start kubelet> <seconds> <start cadvisor>: print the deltas.
window() {
  local name="$1" p="$2" ns="$3" nd="$4" sp="$5" sk="$6" secs="$7" sc="${8:-}" ep ek ec
  sleep 45 # two scrapes, so the series carries the end of the window
  ep="$(cpu_prom "$p" "$ns")"
  ek="$(cpu_kubelet "$p" "$ns" "$nd")"
  ec="$(cpu_cadvisor "$p" "$nd")"
  measure "$name" "$(jq -cn --arg sp "$sp" --arg ep "$ep" --arg sk "$sk" --arg ek "$ek" --arg sc "$sc" --arg ec "$ec" --argjson s "$secs" '
    {windowSeconds: $s,
     prometheusCpuSeconds: (if ($sp == "null" or $ep == "null") then null else (($ep | tonumber) - ($sp | tonumber)) end),
     kubeletCpuSeconds: (if ($sk == "" or $ek == "") then null else (($ek | tonumber) - ($sk | tonumber)) end),
     cadvisorPodCpuSeconds: (if ($sc == "" or $ec == "") then null else (($ec | tonumber) - ($sc | tonumber)) end)}')"
}

echo "==> accuracy: $bursts bursts of one busy core for 60 s each (expected about $((bursts * 60)) CPU-seconds)"
sleep 30
p0="$(cpu_prom "$pod" "$namespace_a")"
k0="$(cpu_kubelet "$pod" "$namespace_a" "$node")"
c0="$(cpu_cadvisor "$pod" "$node")"
measure accuracy-start "$(jq -cn --arg p "$p0" --arg k "$k0" --arg c "$c0" '{prometheus: $p, kubelet: $k, cadvisorPod: $c}')"
# Each burst reports its own user+sys CPU (bash's `times`, the shell is the only
# process), so the pod-level reading is compared with what the load really used,
# not with the 60 s it was asked for (the loop overruns by up to a second).
inside_cpu=0
for i in $(seq 1 "$bursts"); do
  sandbox_exec "$sandbox_id" 'end=$((SECONDS + 60)); while [ "$SECONDS" -lt "$end" ]; do :; done; times' "$fixture_dir/burst-$i.txt" \
    || fail "busy burst $i did not complete: $(head -c 300 "$fixture_dir/burst-$i.sse" 2>/dev/null)"
  used="$(head -1 "$fixture_dir/burst-$i.txt" | awk '{ for (i = 1; i <= NF; i++) { split($i, a, "m"); s += a[1] * 60 + a[2] } print s + 0 }')"
  inside_cpu="$(awk -v a="$inside_cpu" -v b="${used:-0}" 'BEGIN { print a + b }')"
done
measure accuracy-reference "$(jq -cn --argjson inside "$inside_cpu" --argjson bursts "$bursts" '{inSandboxCpuSeconds: $inside, requestedSeconds: ($bursts * 60)}')"
window accuracy "$pod" "$namespace_a" "$node" "$p0" "$k0" "$((bursts * 60))" "$c0"

echo "==> idle baseline: $idle_seconds s with nothing running"
sleep 30
pi="$(cpu_prom "$pod" "$namespace_a")"
ki="$(cpu_kubelet "$pod" "$namespace_a" "$node")"
ci="$(cpu_cadvisor "$pod" "$node")"
sleep "$idle_seconds"
window idle "$pod" "$namespace_a" "$node" "$pi" "$ki" "$idle_seconds" "$ci"

echo "==> syscall-heavy load in the sandbox: $sys_runs x 60 s of dd bs=1"
ps_="$(cpu_prom "$pod" "$namespace_a")"
ks="$(cpu_kubelet "$pod" "$namespace_a" "$node")"
cs="$(cpu_cadvisor "$pod" "$node")"
for i in $(seq 1 "$sys_runs"); do
  sandbox_exec "$sandbox_id" 'timeout 60 dd if=/dev/zero of=/dev/null bs=1 2>/dev/null; echo finished' "$fixture_dir/sys-$i.txt" \
    || fail "syscall burst $i did not complete: out='$(head -c 200 "$fixture_dir/sys-$i.txt" 2>/dev/null)' sse='$(head -c 400 "$fixture_dir/sys-$i.sse" 2>/dev/null | tr '\n' ' ')'"
done
window syscall-gvisor "$pod" "$namespace_a" "$node" "$ps_" "$ks" "$((sys_runs * 60))" "$cs"

echo "==> the same load under runc, on the same node (best effort)"
image="$(kubectl -n "$namespace_a" get pod "$pod" -o json | jq -r '[.spec.containers[] | select(.name != "execd" and (.name | test("execd") | not))][0].image // empty')"
ref="m314-runc-ref-$(date -u +%H%M%S)"
ref_ns="${BEX_MEASURE_REF_NAMESPACE:-opensandbox-system}"
ref_ok=false
if [ -n "$image" ]; then
  if kubectl -n "$ref_ns" apply -f - >"$fixture_dir/ref-apply.txt" 2>&1 <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $ref
  labels: {app.bex.co/m314-fixture: "true"}
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  nodeName: $node
  tolerations: [{operator: Exists}]
  containers:
    - name: ref
      image: $image
      command: ["bash", "-c", "sleep 3600"]
      resources: {requests: {cpu: "1", memory: 256Mi}, limits: {cpu: "1", memory: 1Gi}}
EOF
  then
    if kubectl -n "$ref_ns" wait --for=condition=Ready "pod/$ref" --timeout=240s >"$fixture_dir/ref-wait.txt" 2>&1; then ref_ok=true; fi
  fi
fi
if [ "$ref_ok" = true ]; then
  pr="$(cpu_prom "$ref" "$ref_ns")"
  kr="$(cpu_kubelet "$ref" "$ref_ns" "$node")"
  cr_ref="$(cpu_cadvisor "$ref" "$node")"
  for i in $(seq 1 "$sys_runs"); do
    kubectl -n "$ref_ns" exec "$ref" -- bash -c 'timeout 60 dd if=/dev/zero of=/dev/null bs=1 2>/dev/null; true' || true
  done
  window syscall-runc "$ref" "$ref_ns" "$node" "$pr" "$kr" "$((sys_runs * 60))" "$cr_ref"
else
  measure syscall-runc "$(jq -cn --arg a "$(head -c 400 "$fixture_dir/ref-apply.txt" 2>/dev/null)" --arg w "$(head -c 400 "$fixture_dir/ref-wait.txt" 2>/dev/null)" --arg i "${image:-}" \
    '{skipped: "the runc reference pod did not start", image: $i, apply: $a, wait: $w}')"
fi
kubectl -n "$ref_ns" delete pod "$ref" --ignore-not-found --wait=false >/dev/null 2>&1 || true

pass "cpu leg measured pod $pod on $node (read the MEASURE lines against the thresholds in the board item w22/m314 t001)"
