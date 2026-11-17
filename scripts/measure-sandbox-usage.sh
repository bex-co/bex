#!/usr/bin/env bash
# Production check for w22/m314 t005: a sandbox billed on use is charged for the
# CPU it used and the memory it was allocated, an idle one pays about no CPU,
# and a suspended one accrues neither.
#
# Run by sandbox-measure.yml (leg `usage`) through
# scripts/verify-sandbox-isolation-live.sh, which provides one disposable,
# billing-excluded workspace. Two sandboxes are created:
#
#   busy  runs BEX_MEASURE_BURSTS x 60 s of one busy core (default 5), then is
#         paused, which closes its running interval so the used-CPU meter reads
#         it within a few minutes. Its sandbox_cpu_seconds is compared with the
#         CPU the load itself used (the shell's `times`) and with the pod-level
#         cgroup counter read straight from the node's cAdvisor (what the meter
#         bills, gVisor's overhead included: t001 measured 1.098x the load's own
#         count). It then stays suspended: its CPU, memory and weighted compute
#         must not grow.
#   idle  runs nothing for BEX_MEASURE_USAGE_IDLE_SECONDS (default 3600) and is
#         paused: it must read at most 2% of one core and its memory in full.
#
# A third sandbox, on the smallest plan (`starter`, half a CPU), proves the cap
# the bill depends on (w22/020): each pod's CPU quota is its plan's CPU (the
# execd-installer init container no longer raises it to the namespace default),
# and four syscall-heavy loops for 60 s move the pod's counter by about half a
# core, not two. Its start is timed from the pod's own timestamps, the install
# step included, so the half-CPU init container's cost is on record.
#
# The run also reads GET /v1/usage as a customer does. Before the switch instant
# (estimatedCost.sandboxUsageBilledFrom) the CPU and memory lines are unbilled
# and the weighted compute line is the charge; from it on they are the charge,
# at the price sheet's rates, and the sandbox has no weighted compute row.
#
# Everything is printed as `MEASURE <name> <json>` lines; the run's log is the
# evidence.

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

bursts="${BEX_MEASURE_BURSTS:-5}"
idle_seconds="${BEX_MEASURE_USAGE_IDLE_SECONDS:-3600}"
ws="$BEX_WORKSPACE_A"
[[ "$bursts" =~ ^[0-9]{1,2}$ ]] && [[ "$idle_seconds" =~ ^[0-9]{1,5}$ ]] || fail "BEX_MEASURE_BURSTS and BEX_MEASURE_USAGE_IDLE_SECONDS must be whole numbers"

# pod_cpu <pod> <node>: the pod-level cgroup's cumulative CPU seconds, from the
# node's cAdvisor (the row Prometheus keeps as sandbox_pod_cpu_usage_seconds_total).
pod_cpu() {
  node_cadvisor "$2" "$fixture_dir/cadvisor.txt"
  grep -F "pod=\"$1\"" "$fixture_dir/cadvisor.txt" | grep '^container_cpu_usage_seconds_total' | grep 'name=""' | awk '{print $2}' | head -1
}
# pod_quota_cores <pod>: the pod cgroup's CPU quota in cores, from the cAdvisor
# file pod_cpu last fetched ("" when the pod has none). It caps what the pod
# can use, and so what the used-CPU meter can bill: under gVisor the sentry and
# the workload share this cgroup.
pod_quota_cores() {
  local quota period
  quota="$(grep -F "pod=\"$1\"" "$fixture_dir/cadvisor.txt" | grep '^container_spec_cpu_quota' | grep 'name=""' | awk '{print $2}' | head -1)"
  period="$(grep -F "pod=\"$1\"" "$fixture_dir/cadvisor.txt" | grep '^container_spec_cpu_period' | grep 'name=""' | awk '{print $2}' | head -1)"
  [ -n "$quota" ] && [ -n "$period" ] && awk -v q="$quota" -v p="$period" 'BEGIN { print q / p }'
}
# pod_limit_cores <pod>: the sum of the pod's containers' CPU limits in cores,
# which is its plan's CPU (init containers are not counted: they must not
# exceed it).
pod_limit_cores() {
  kubectl -n "$namespace_a" get pod "$1" -o json \
    | jq -r '[.spec.containers[].resources.limits.cpu // empty | if endswith("m") then (.[:-1] | tonumber / 1000) else tonumber end] | add // 0'
}
# pod_start <pod>: when the pod was created, when its install step ran and when
# its first container started, as JSON with the two durations in seconds.
pod_start() {
  kubectl -n "$namespace_a" get pod "$1" -o json | jq -c '
    (.status.initContainerStatuses // [] | map(select(.name == "execd-installer")) | .[0].state.terminated // {}) as $init |
    (.status.containerStatuses // [] | map(.state.running.startedAt // empty) | min) as $started |
    {created: .metadata.creationTimestamp, installStarted: ($init.startedAt // null), installFinished: ($init.finishedAt // null), containerStarted: ($started // null),
     installCpuLimit: ([.spec.initContainers[]? | select(.name == "execd-installer") | .resources.limits.cpu] | .[0] // null)}
    | .installSeconds = (if .installStarted and .installFinished then (.installFinished | fromdateiso8601) - (.installStarted | fromdateiso8601) else null end)
    | .createdToStartedSeconds = (if .containerStarted then (.containerStarted | fromdateiso8601) - (.created | fromdateiso8601) else null end)'
}
# usage_of <id>: the sandbox's usage_hourly totals by kind, as JSON.
usage_of() {
  db_query -v ws="$ws" -v id="$1" <<<"select kind, sum(quantity) from usage_hourly where workspace_id = :'ws' and service_id = :'id' group by kind order by kind;" \
    | jq -Rsc 'split("\n") | map(select(length > 0) | split("|") | {(.[0]): (.[1] | tonumber)}) | add // {}'
}
# intervals_of <id>: the sandbox's running intervals, as JSON.
intervals_of() {
  db_query -v ws="$ws" -v id="$1" <<<"select pod_name, usage_billed, to_char(started_at at time zone 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"'), coalesce(to_char(ended_at at time zone 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"'), ''), coalesce(extract(epoch from (ended_at - started_at))::int::text, ''), cpu_settled, cpu_deferred_since is not null from sandbox_running_intervals where workspace_id = :'ws' and sandbox_id = :'id' order by started_at;" \
    | jq -Rsc 'split("\n") | map(select(length > 0) | split("|") | {pod: .[0], usageBilled: (.[1] == "t"), startedAt: .[2], endedAt: .[3], seconds: (.[4] | if . == "" then null else tonumber end), cpuSettled: (.[5] == "t"), deferred: (.[6] == "t")})'
}
# wait_settled <id> <seconds>: until every running interval of the sandbox is
# closed and read by the CPU meter.
wait_settled() {
  local deadline=$((SECONDS + $2)) left=""
  while ((SECONDS < deadline)); do
    left="$(db_query -v ws="$ws" -v id="$1" <<<"select count(*) filter (where not cpu_settled or ended_at is null), count(*) from sandbox_running_intervals where workspace_id = :'ws' and sandbox_id = :'id';")"
    [ "${left%%|*}" = 0 ] && [ "${left##*|}" != 0 ] && return 0
    sleep 15
  done
  return 1
}
# within <value> <low> <high>
within() { awk -v v="$1" -v lo="$2" -v hi="$3" 'BEGIN { exit !(v >= lo && v <= hi) }' && echo true || echo false; }

echo "==> create the busy and the idle sandbox"
busy="$(create_sandbox "$fixture_dir/busy.json" '{"type":"deny-all"}')"
idle="$(create_sandbox "$fixture_dir/idle.json" '{"type":"deny-all"}')"
wait_status "$busy" running 300
wait_status "$idle" running 300
idle_from=$SECONDS
read -r busy_pod busy_node busy_cr < <(find_pod "$busy")
read -r idle_pod idle_node idle_cr < <(find_pod "$idle")
memory_mib="$(kubectl -n "$namespace_a" get pod "$busy_pod" -o json | jq -r '.metadata.labels["bex.co/memory-mib"] // empty')"
limits="$(kubectl -n "$namespace_a" get pod "$busy_pod" -o json | jq -c '{plan: .metadata.labels["bex.co/plan"], containers: [.spec.containers[] | {name, cpu: .resources.limits.cpu, memory: .resources.limits.memory}], initContainers: [.spec.initContainers[]? | {name, cpu: .resources.limits.cpu}], overhead: .spec.overhead}')"
measure sandboxes "$(jq -cn --arg b "$busy" --arg bp "$busy_pod" --arg bc "$busy_cr" --arg i "$idle" --arg ip "$idle_pod" --arg ic "$idle_cr" --arg m "${memory_mib:-}" --argjson l "$limits" \
  '{busy: {id: $b, pod: $bp, substrate: $bc}, idle: {id: $i, pod: $ip, substrate: $ic}, memoryMiB: $m, podLimits: $l}')"
[[ "${memory_mib:-}" =~ ^[0-9]+$ ]] || fail "the sandbox pod carries no bex.co/memory-mib label"

echo "==> busy: $bursts bursts of one busy core for 60 s each"
c0="$(pod_cpu "$busy_pod" "$busy_node")"
busy_quota="$(pod_quota_cores "$busy_pod" || true)"
busy_limit="$(pod_limit_cores "$busy_pod")"
measure pod-cpu-quota "$(jq -cn --arg q "$busy_quota" --arg l "$busy_limit" --argjson s "$(pod_start "$busy_pod")" '{podQuotaCores: $q, planCpuCores: $l, start: $s}')"
check "cap: the default plan's pod CPU quota is its plan's CPU" \
  "$([ -n "$busy_quota" ] && within "$busy_quota" "$busy_limit" "$busy_limit" || echo false)" "pod quota '$busy_quota' cores, plan $busy_limit"
inside_cpu=0
for i in $(seq 1 "$bursts"); do
  sandbox_exec "$busy" 'end=$((SECONDS + 60)); while [ "$SECONDS" -lt "$end" ]; do :; done; times' "$fixture_dir/burst-$i.txt" \
    || fail "busy burst $i did not complete: $(head -c 300 "$fixture_dir/burst-$i.sse" 2>/dev/null)"
  used="$(head -1 "$fixture_dir/burst-$i.txt" | awk '{ for (i = 1; i <= NF; i++) { split($i, a, "m"); s += a[1] * 60 + a[2] } print s + 0 }')"
  inside_cpu="$(awk -v a="$inside_cpu" -v b="${used:-0}" 'BEGIN { print a + b }')"
done
sleep 20 # the node's cAdvisor refreshes about every 15 s
c1="$(pod_cpu "$busy_pod" "$busy_node")"
[ -n "$c0" ] && [ -n "$c1" ] || fail "the node's cAdvisor has no pod-level CPU row for $busy_pod"

echo "==> pause the busy sandbox: its running interval closes and the meter reads it"
expect_status 204 POST "/v1/sandboxes/$busy/pause?ownerId=$ws" "$fixture_dir/pause-busy.json"
wait_status "$busy" suspended 300
settled=false
wait_settled "$busy" 600 && settled=true
busy_intervals="$(intervals_of "$busy")"
busy_usage="$(usage_of "$busy")"
measure busy "$(jq -cn --argjson inside "$inside_cpu" --arg c0 "$c0" --arg c1 "$c1" --argjson requested "$((bursts * 60))" --argjson iv "$busy_intervals" --argjson u "$busy_usage" '
  ($u.sandbox_cpu_seconds // 0) as $m | (($c1 | tonumber) - ($c0 | tonumber)) as $load |
  {requestedSeconds: $requested, inSandboxCpuSeconds: $inside, podCpuSecondsOverLoad: $load, podCpuSecondsTotal: ($c1 | tonumber),
   meteredCpuSeconds: $m, meteredOverInSandbox: (if $inside > 0 then $m / $inside else null end),
   meteredOverPodLoad: (if $load > 0 then $m / $load else null end), intervals: $iv, usage: $u}')"
check "busy: the CPU meter read every running interval" "$settled" "intervals: $busy_intervals"
metered="$(jq -r '.sandbox_cpu_seconds // 0' <<<"$busy_usage")"
load="$(awk -v a="$c0" -v b="$c1" 'BEGIN { print b - a }')"
# The meter reads the whole interval (the load plus a little idle and the exec
# overhead): never less than the pod counter moved during the load, less the
# scrape edges, and never more than the counter's total.
check "busy: metered CPU matches the pod-level counter" \
  "$(within "$metered" "$(awk -v l="$load" 'BEGIN { print l * 0.95 - 1 }')" "$(awk -v t="$c1" 'BEGIN { print t * 1.02 + 2 }')")" \
  "metered $metered, pod counter moved $load during the load and totals $c1"
# Against what the load itself used: t001's ratio was 1.098; allow 1.0 to 1.25.
check "busy: metered CPU is the load's own CPU plus gVisor's overhead" \
  "$(within "$metered" "$inside_cpu" "$(awk -v i="$inside_cpu" 'BEGIN { print i * 1.25 + 5 }')")" \
  "metered $metered, the load used $inside_cpu"
check "busy: the interval names the pod that ran it" \
  "$(jq -r --arg p "$busy_pod" 'length >= 1 and all(.[]; .pod == $p and .usageBilled)' <<<"$busy_intervals")" "$busy_intervals, pod $busy_pod"
busy_seconds="$(jq -r '[.[].seconds // 0] | add' <<<"$busy_intervals")"
want_memory="$(awk -v m="$memory_mib" -v s="$busy_seconds" 'BEGIN { print m / 1024 * s }')"
check "busy: memory is the allocated GiB for the seconds it ran" \
  "$(within "$(jq -r '.sandbox_memory_gb_seconds // 0' <<<"$busy_usage")" "$(awk -v w="$want_memory" 'BEGIN { print w * 0.98 - 2 }')" "$(awk -v w="$want_memory" 'BEGIN { print w * 1.02 + 2 }')")" \
  "want about $want_memory GiB-seconds over $busy_seconds s, usage $busy_usage"

echo "==> cap: a starter sandbox under four syscall-heavy loops uses its half CPU, not the namespace default"
small_from=$SECONDS
small="$(create_sandbox "$fixture_dir/small.json" '{"type":"deny-all"}' '"plan":"starter"')"
wait_status "$small" running 300
small_running=$((SECONDS - small_from))
read -r small_pod small_node _ < <(find_pod "$small")
small_limit="$(pod_limit_cores "$small_pod")"
s0=""
for _ in $(seq 1 12); do # a new pod's cAdvisor row appears within a refresh or two
  s0="$(pod_cpu "$small_pod" "$small_node" || true)"
  [ -n "$s0" ] && break
  sleep 10
done
small_quota="$(pod_quota_cores "$small_pod" || true)"
# Each loop forks a process per turn: nearly all of its CPU is gVisor's, which
# is the load that read 2.00 cores on a 1-CPU plan before the cap (m314 t001).
sandbox_exec "$small" 'for n in 1 2 3 4; do (end=$((SECONDS + 60)); while [ "$SECONDS" -lt "$end" ]; do cat /proc/uptime >/dev/null; done) & done; wait' "$fixture_dir/small-load.txt" \
  || fail "the starter sandbox's load did not complete: $(head -c 300 "$fixture_dir/small-load.txt.sse" 2>/dev/null)"
sleep 20
s1="$(pod_cpu "$small_pod" "$small_node")"
[ -n "$s0" ] && [ -n "$s1" ] || fail "the node's cAdvisor has no pod-level CPU row for $small_pod"
small_used="$(awk -v a="$s0" -v b="$s1" 'BEGIN { print b - a }')"
measure cap "$(jq -cn --arg id "$small" --arg pod "$small_pod" --arg q "$small_quota" --arg l "$small_limit" --argjson used "$small_used" --argjson api "$small_running" --argjson s "$(pod_start "$small_pod")" \
  '{id: $id, pod: $pod, plan: "starter", podQuotaCores: $q, planCpuCores: $l, loadSeconds: 60, loops: 4, podCpuSeconds: $used,
    coresOverLoad: ($used / 60), createToRunningSecondsByApi: $api, start: $s}')"
check "cap: the starter pod's CPU quota is its plan's CPU" \
  "$([ -n "$small_quota" ] && within "$small_quota" "$small_limit" "$small_limit" || echo false)" "pod quota '$small_quota' cores, plan $small_limit"
# The quota binds each 100 ms period, so 60 s of load cannot use more than the
# plan's CPU for 60 s; 15% and 2 s cover the exec's own start and end. The
# floor shows the load did reach the cap, so the ceiling is not passed idle.
check "cap: a syscall-heavy load uses about the plan's CPU and no more" \
  "$(within "$small_used" "$(awk -v l="$small_limit" 'BEGIN { print l * 60 * 0.8 }')" "$(awk -v l="$small_limit" 'BEGIN { print l * 60 * 1.15 + 2 }')")" \
  "the pod used $small_used CPU-seconds in 60 s of load on a $small_limit-CPU plan"
expect_status 204 POST "/v1/sandboxes/$small/terminate?ownerId=$ws" "$fixture_dir/terminate-small.json"

echo "==> idle: nothing runs for $idle_seconds s"
remaining=$((idle_seconds - (SECONDS - idle_from)))
[ "$remaining" -gt 0 ] && sleep "$remaining"
refresh_owner_token || true
expect_status 204 POST "/v1/sandboxes/$idle/pause?ownerId=$ws" "$fixture_dir/pause-idle.json"
wait_status "$idle" suspended 300
idle_settled=false
wait_settled "$idle" 600 && idle_settled=true
idle_intervals="$(intervals_of "$idle")"
idle_usage="$(usage_of "$idle")"
idle_ran="$(jq -r '[.[].seconds // 0] | add' <<<"$idle_intervals")"
idle_cpu="$(jq -r '.sandbox_cpu_seconds // 0' <<<"$idle_usage")"
measure idle "$(jq -cn --argjson iv "$idle_intervals" --argjson u "$idle_usage" --argjson ran "$idle_ran" --argjson cpu "$idle_cpu" \
  '{ranSeconds: $ran, meteredCpuSeconds: $cpu, shareOfOneCore: (if $ran > 0 then $cpu / $ran else null end), intervals: $iv, usage: $u}')"
check "idle: the CPU meter read every running interval" "$idle_settled" "intervals: $idle_intervals"
check "idle: at most 2% of one core" "$(within "$idle_cpu" 0 "$(awk -v s="$idle_ran" 'BEGIN { print s * 0.02 + 1 }')")" "$idle_cpu CPU-seconds over $idle_ran s"
want_idle_memory="$(awk -v m="$memory_mib" -v s="$idle_ran" 'BEGIN { print m / 1024 * s }')"
check "idle: memory is charged in full" \
  "$(within "$(jq -r '.sandbox_memory_gb_seconds // 0' <<<"$idle_usage")" "$(awk -v w="$want_idle_memory" 'BEGIN { print w * 0.98 - 2 }')" "$(awk -v w="$want_idle_memory" 'BEGIN { print w * 1.02 + 2 }')")" \
  "want about $want_idle_memory GiB-seconds over $idle_ran s, usage $idle_usage"

echo "==> suspended: the busy sandbox accrued no CPU, memory or weighted compute since its pause"
busy_later="$(usage_of "$busy")"
measure suspended "$(jq -cn --argjson a "$busy_usage" --argjson b "$busy_later" '{atPause: $a, now: $b}')"
check "suspended: only snapshot storage grew" \
  "$(jq -n --argjson a "$busy_usage" --argjson b "$busy_later" 'def running: with_entries(select(.key != "sandbox_snapshot_gb_seconds")); ($a | running) == ($b | running)')" \
  "$busy_usage then $busy_later"

echo "==> what the customer reads: GET /v1/usage"
expect_status 200 GET "/v1/usage?ownerId=$ws" "$fixture_dir/usage.json"
switch="$(jq -r '.estimatedCost.sandboxUsageBilledFrom // ""' "$fixture_dir/usage.json")"
api="$(jq -c --arg a "$busy" --arg b "$idle" '{sandboxUsageBilledFrom: (.estimatedCost.sandboxUsageBilledFrom // ""),
  rows: [.services[]? | select(.serviceId == $a or .serviceId == $b) | {serviceId, rows}],
  charges: [.estimatedCost.resources[]? | select(.serviceId == $a or .serviceId == $b) | {serviceId, costUsd, charges}]}' "$fixture_dir/usage.json")"
measure api "$api"
[ -n "$switch" ] || fail "GET /v1/usage carries no estimatedCost.sandboxUsageBilledFrom for a workspace with sandbox usage"
billed="$(jq -rn --arg s "$switch" --arg first "$(jq -r '.[0].startedAt' <<<"$busy_intervals")" '($first | fromdateiso8601) >= ($s | fromdateiso8601)')"
measure billing-model "$(jq -cn --arg s "$switch" --argjson b "$billed" '{sandboxUsageBilledFrom: $s, thisRunIsBilledOnUse: $b}')"
lines() { jq -c --arg id "$1" '[.estimatedCost.resources[]? | select(.serviceId == $id) | .charges[]]' "$fixture_dir/usage.json"; }
if [ "$billed" = true ]; then
  # CPU-seconds x the CPU-hour rate, to the cent the line shows.
  check "billed on use: the busy sandbox's CPU line is its CPU-seconds at \$0.084 per CPU-hour" \
    "$(jq -n --argjson l "$(lines "$busy")" --argjson m "$metered" '
      [$l[] | select(.kind == "sandbox_cpu_seconds")] as $cpu |
      ($cpu | length) == 1 and ($cpu[0].unbilled // false) == false and $cpu[0].rateUsd == "0.084"
      and ((($cpu[0].costUsd | tonumber) - ($m * 0.084 / 3600)) | fabs) <= 0.0051')" "$(lines "$busy")"
  check "billed on use: memory is charged at \$0.0133 per GiB-hour and nothing on the weighted meter" \
    "$(jq -n --argjson b "$(lines "$busy")" --argjson i "$(lines "$idle")" '
      all(($b + $i)[]; .kind != "sandbox_compute_seconds")
      and all(($b + $i)[] | select(.kind == "sandbox_memory_gb_seconds"); .rateUsd == "0.0133" and (.unbilled // false) == false)
      and any($i[]; .kind == "sandbox_memory_gb_seconds")')" "$(lines "$busy") $(lines "$idle")"
  check "billed on use: the idle sandbox's CPU charge is about nothing" \
    "$(jq -n --argjson i "$(lines "$idle")" 'all($i[] | select(.kind == "sandbox_cpu_seconds"); (.costUsd | tonumber) <= 0.01)')" "$(lines "$idle")"
else
  check "before the switch: CPU and memory are shown unbilled beside the weighted compute charge" \
    "$(jq -n --argjson b "$(lines "$busy")" '
      any($b[]; .kind == "sandbox_compute_seconds")
      and all($b[] | select(.kind == "sandbox_cpu_seconds" or .kind == "sandbox_memory_gb_seconds"); .unbilled == true and .rateUsd == "0" and .costUsd == "0.00")
      and any($b[]; .kind == "sandbox_cpu_seconds")')" "$(lines "$busy")"
fi

[ "$failures" = 0 ] || fail "$failures usage check(s) failed (see the lines above)"
pass "usage leg: a busy, an idle and a suspended sandbox are metered as designed (billed on use: $billed)"
