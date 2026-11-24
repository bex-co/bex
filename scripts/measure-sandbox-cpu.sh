#!/usr/bin/env bash
# Production measurement leg `cpu`: what a gVisor (runsc) sandbox pod's cgroup
# CPU counter, the figure the used-CPU meter bills (ADR104 D8), reads for a
# known load, against the same load in a plain runc pod on the same node.
#
# w22/m314 t001 (2026-11-15, run 43696023922) fixed its thresholds before it
# measured, in the board item w22/m314 t001:
#   accuracy  a one-core busy load must read its CPU-seconds within +-5%;
#   idle      an idle running sandbox must read at most 2% of one core;
#   syscalls  a syscall-heavy load must read at most 2x the same load under runc.
# Its runc pod was refused by PodSecurity, so the last was only bounded.
#
# w22/019 asks for the factor a gVisor overhead divisor would use: the pod CPU
# one unit of the same work costs under gVisor, divided by what it costs under
# runc. Both pods have the sandbox's CPU limit as their quota (w22/020), and a
# load that fills the quota reads the same CPU-seconds in both, so each load
# also counts its own work (loop iterations, bytes copied) and the factor is
# taken per unit of work. Both pods run the loads through `kubectl exec`, the
# same path on both sides; the public exec stream is not what is compared, and
# it is cut on long commands (w22/028).
#
# Run by sandbox-measure.yml (leg `cpu`) through
# scripts/verify-sandbox-isolation-live.sh, which provides one disposable,
# billing-excluded workspace. One sandbox and one reference pod are created;
# every number is printed as `MEASURE <name> <json>`. Bounded: BEX_MEASURE_BURSTS
# x 60 s of one-core load in the sandbox (default 10) and BEX_MEASURE_REF_BURSTS
# in the reference pod (default 5), BEX_MEASURE_IDLE_SECONDS of idle (default
# 600), and 3 x 60 s of syscall-heavy load in each.

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

bursts="${BEX_MEASURE_BURSTS:-10}"
ref_bursts="${BEX_MEASURE_REF_BURSTS:-5}"
idle_seconds="${BEX_MEASURE_IDLE_SECONDS:-600}"
sys_runs=3
ref_ns="${BEX_MEASURE_REF_NAMESPACE:-opensandbox-system}"
ref="cpu-runc-ref-$(date -u +%H%M%S)"
trap 'kubectl -n "$ref_ns" delete pod "$ref" --ignore-not-found --wait=false >/dev/null 2>&1; measure_cleanup' EXIT

echo "==> create a sandbox and find its pod"
# No idle suspension: kubectl exec bypasses bex-api, which sees no activity, so
# the default 900 s window paused the sandbox mid-leg (run 45396288630).
sandbox_id="$(create_sandbox "$fixture_dir/sandbox.json" '{"type":"deny-all"}' '"idleTimeoutSeconds":0')"
wait_status "$sandbox_id" running 300
read -r pod node _ < <(find_pod "$sandbox_id")
[ -n "${pod:-}" ] && [ -n "${node:-}" ] || fail "no pod or node found for sandbox $sandbox_id"
IFS='|' read -r container image cpu_limit runtime < <(kubectl -n "$namespace_a" get pod "$pod" -o json | jq -r '
  ([.spec.containers[] | select(.name | test("execd") | not)][0]) as $c
  | "\($c.name // "")|\($c.image // "")|\($c.resources.limits.cpu // "")|\(.spec.runtimeClassName // "")"')
[ -n "${container:-}" ] && [ -n "${image:-}" ] && [ -n "${cpu_limit:-}" ] || fail "pod $pod has no workload container with a CPU limit"
measure pod "$(jq -cn --arg ns "$namespace_a" --arg pod "$pod" --arg node "$node" --arg cpu "$cpu_limit" --arg rt "$runtime" \
  '{namespace: $ns, pod: $pod, node: $node, cpuLimit: $cpu, runtimeClass: $rt}')"
[ "$runtime" = gvisor ] || fail "sandbox pod $pod runs RuntimeClass '$runtime', want gvisor"
kubectl -n "$namespace_a" exec "$pod" -c "$container" -- bash -c 'for t in dd sleep; do command -v $t >/dev/null || exit 1; done' \
  || fail "the sandbox's image lacks bash, dd or sleep"

echo "==> a plain runc pod on the same node, with the sandbox's image and CPU limit"
ref_ok=false
if runc_reference_pod "$ref" "$node" "$image" "$cpu_limit" | kubectl -n "$ref_ns" apply -f - >"$fixture_dir/ref-apply.txt" 2>&1 \
  && kubectl -n "$ref_ns" wait --for=condition=Ready "pod/$ref" --timeout=240s >"$fixture_dir/ref-wait.txt" 2>&1; then
  ref_ok=true
  ref_runtime="$(kubectl -n "$ref_ns" get pod "$ref" -o jsonpath='{.spec.runtimeClassName}')"
  [ -z "$ref_runtime" ] || fail "the reference pod runs RuntimeClass '$ref_runtime', want the node's default (runc)"
fi
measure reference "$(jq -cn --argjson ok "$ref_ok" --arg ns "$ref_ns" --arg pod "$ref" \
  --arg a "$(head -c 400 "$fixture_dir/ref-apply.txt" 2>/dev/null)" --arg w "$(head -c 400 "$fixture_dir/ref-wait.txt" 2>/dev/null)" \
  '{started: $ok, namespace: $ns, pod: $pod} + (if $ok then {} else {apply: $a, wait: $w} end)')"

# run_load <name> <namespace> <pod> <container> <runs> <busy|syscall>: runs the
# load <runs> times, one exec each, and prints and saves ($fixture_dir/<name>.json)
# the pod counter's delta over the window, the work done and the load's own CPU.
run_load() {
  local name="$1" ns="$2" p="$3" c="$4" runs="$5" kind="$6" cmd start finish i out w o work=0 own=0
  [ "$kind" = busy ] && cmd="$busy_load" || cmd="$syscall_load"
  sleep 20
  start="$(pod_cpu_new "$p" "$node")"
  [ -n "$start" ] || fail "$name: the node's cAdvisor has no pod-level CPU row for $p"
  for i in $(seq 1 "$runs"); do
    out="$fixture_dir/$name-$i.txt"
    kubectl -n "$ns" exec "$p" -c "$c" -- bash -c "$cmd" >"$out" 2>"$out.err" \
      || fail "$name run $i did not complete: $(head -c 300 "$out.err" 2>/dev/null)"
    w="$(load_work "$kind" <"$out")"
    o="$(load_own_cpu "$kind" <"$out")"
    [ -n "$w" ] && [ -n "$o" ] || fail "$name run $i printed no work count or times: $(head -c 300 "$out")"
    work="$(awk -v a="$work" -v b="$w" 'BEGIN { printf "%.0f", a + b }')"
    own="$(awk -v a="$own" -v b="$o" 'BEGIN { print a + b }')"
  done
  sleep 30 # cAdvisor refreshes a pod's row about every 15 s
  finish="$(pod_cpu_new "$p" "$node")"
  [ -n "$finish" ] || fail "$name: the pod-level CPU row for $p disappeared"
  jq -cn --arg s "$start" --arg f "$finish" --argjson work "$work" --argjson own "$own" --argjson runs "$runs" '
    (($f | tonumber) - ($s | tonumber)) as $cpu
    | {runs: $runs, requestedSeconds: ($runs * 60), podCpuSeconds: $cpu, ownCpuSeconds: $own, work: $work,
       workPerPodCpuSecond: (if $cpu > 0 then $work / $cpu else null end),
       podOverOwn: (if $own > 0 then $cpu / $own else null end)}' >"$fixture_dir/$name.json"
  measure "$name" "$(cat "$fixture_dir/$name.json")"
}

# factor <name> <gvisor load> <runc load>: the pod CPU one unit of work costs
# under gVisor over what it costs under runc (above 1: gVisor bills more).
factor() {
  measure "$1" "$(jq -cn --slurpfile g "$fixture_dir/$2.json" --slurpfile r "$fixture_dir/$3.json" '
    ($g[0].workPerPodCpuSecond) as $gw | ($r[0].workPerPodCpuSecond) as $rw
    | {gvisorWorkPerPodCpuSecond: $gw, runcWorkPerPodCpuSecond: $rw,
       factor: (if ($gw // 0) > 0 and $rw != null then $rw / $gw else null end)}')"
}

echo "==> busy load in the sandbox: $bursts x 60 s of one core"
run_load busy-gvisor "$namespace_a" "$pod" "$container" "$bursts" busy

echo "==> idle baseline: $idle_seconds s with nothing running"
sleep 30
ci="$(pod_cpu_new "$pod" "$node")"
sleep "$idle_seconds"
cf="$(pod_cpu_new "$pod" "$node")"
[ -n "$ci" ] && [ -n "$cf" ] || fail "idle: no pod-level CPU row for $pod"
measure idle "$(jq -cn --arg s "$ci" --arg f "$cf" --argjson secs "$idle_seconds" \
  '(($f | tonumber) - ($s | tonumber)) as $cpu | {windowSeconds: $secs, podCpuSeconds: $cpu, coreShare: ($cpu / $secs)}')"

echo "==> syscall-heavy load in the sandbox: $sys_runs x 60 s of dd bs=1"
run_load syscall-gvisor "$namespace_a" "$pod" "$container" "$sys_runs" syscall

if [ "$ref_ok" = true ]; then
  echo "==> the same loads under runc: $ref_bursts x 60 s busy, $sys_runs x 60 s of dd bs=1"
  run_load busy-runc "$ref_ns" "$ref" ref "$ref_bursts" busy
  run_load syscall-runc "$ref_ns" "$ref" ref "$sys_runs" syscall
  factor factor-busy busy-gvisor busy-runc
  factor factor-syscall syscall-gvisor syscall-runc
fi
check "the runc reference pod started on the sandbox's node" "$ref_ok" "see MEASURE reference"
[ "$failures" = 0 ] || fail "cpu leg: $failures check(s) failed"
pass "cpu leg measured pod $pod on $node against $ref (read the MEASURE lines against w22/m314 t001 and w22/019)"
