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
measure pod-cpu-quota "$(jq -cn --arg q "$(pod_quota_cores "$busy_pod" || true)" '{podQuotaCores: $q}')"
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
