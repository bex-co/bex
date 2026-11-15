#!/usr/bin/env bash
# Production measurement leg for w22/008 and w22/002: what does the kubelet (and
# cAdvisor, and Prometheus) report for a gVisor sandbox pod?
#
#   w22/008: does the kubelet's ephemeral-storage accounting follow what a
#            sandbox writes to its rootfs? (If it does not, a 40 GiB
#            `ephemeral-storage` limit would be inert or evict a healthy pod.)
#   w22/002: does any network byte counter exist for a sandbox pod, on the node
#            (kubelet summary, raw cAdvisor) and in Prometheus (which drops
#            container="" series), so a sandbox's egress could be metered?
#
# Run by sandbox-measure.yml (leg `kubelet`) through
# scripts/verify-sandbox-isolation-live.sh, which provides one disposable,
# billing-excluded workspace. One allow-all sandbox is created, written to in
# 1 GiB steps up to BEX_MEASURE_FILL_GIB (default 5) and made to download and
# upload a few tens of MB, then terminated. Nothing but measurement lines is
# printed (`MEASURE <name> <json>`).

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

fill_gib="${BEX_MEASURE_FILL_GIB:-5}"
down_bytes="${BEX_MEASURE_DOWN_BYTES:-52428800}"
up_bytes="${BEX_MEASURE_UP_BYTES:-20000000}"

echo "==> create an allow-all sandbox and find its pod"
sandbox_id="$(create_sandbox "$fixture_dir/sandbox.json" '{"type":"allow-all"}')"
wait_status "$sandbox_id" running 300
read -r pod node cr < <(find_pod "$sandbox_id")
[ -n "${pod:-}" ] && [ -n "${node:-}" ] || fail "no pod or node found for sandbox $sandbox_id"
runtime="$(kubectl -n "$namespace_a" get pod "$pod" -o jsonpath='{.spec.runtimeClassName}')"
measure pod "$(jq -cn --arg ns "$namespace_a" --arg pod "$pod" --arg node "$node" --arg cr "$cr" --arg rt "$runtime" \
  '{namespace: $ns, pod: $pod, node: $node, batchsandbox: $cr, runtimeClassName: $rt}')"
[ "$runtime" = gvisor ] || fail "pod $pod runs with runtimeClassName '$runtime', want gvisor"

# pod_stats <summary file>: this pod's entry of the kubelet summary, reduced to
# the fields the two questions need.
pod_stats() {
  jq -c --arg ns "$namespace_a" --arg p "$pod" '
    [.pods[] | select(.podRef.namespace == $ns and .podRef.name == $p)][0]
    | if . == null then {found: false} else {
        found: true,
        ephemeral: .["ephemeral-storage"],
        containers: [.containers[]? | {name, rootfs, logs}],
        network: (.network // null | if . == null then null else {rxBytes, txBytes, interfaces: [.interfaces[]? | {name, rxBytes, txBytes}]} end),
        memoryWorkingSetBytes: .memory.workingSetBytes
      } end' "$1"
}

echo "==> what the sandbox has to work with"
tools="$(sandbox_exec "$sandbox_id" 'for t in dd curl python3 df; do command -v $t >/dev/null 2>&1 && printf "%s " $t; done; echo' "$fixture_dir/tools.txt" && tr -d '\n' <"$fixture_dir/tools.txt" || true)"
measure tools "$(jq -cn --arg t "$tools" '{available: $t}')"

echo "==> ephemeral-storage accounting while the sandbox writes (w22/008)"
node_summary "$node" "$fixture_dir/summary-0.json"
measure ephemeral-baseline "$(pod_stats "$fixture_dir/summary-0.json")"
written=0
for step in $(seq 1 "$fill_gib"); do
  if ! sandbox_exec "$sandbox_id" "dd if=/dev/zero of=/root/fill-$step bs=1M count=1024 conv=fsync 2>&1 | tail -1; df -k /root | tail -1" "$fixture_dir/fill-$step.txt"; then
    measure ephemeral-step "$(jq -cn --argjson s "$step" --arg o "$(head -c 300 "$fixture_dir/fill-$step.sse" 2>/dev/null | tr '\n' ' ')" \
      '{step: $s, failed: true, detail: $o}')"
    break
  fi
  written=$((step * 1024 * 1024 * 1024))
  sleep 20 # the kubelet refreshes its volume stats on a timer
  node_summary "$node" "$fixture_dir/summary-$step.json"
  measure ephemeral-step "$(jq -cn --argjson s "$step" --argjson w "$written" --arg df "$(tail -c 200 "$fixture_dir/fill-$step.txt" | tr '\n' ' ')" \
    --argjson st "$(pod_stats "$fixture_dir/summary-$step.json")" '{step: $s, writtenBytes: $w, df: $df, kubelet: $st}')"
  alive="$(sandbox_status "$sandbox_id")"
  [ "$alive" = running ] || {
    measure ephemeral-step "$(jq -cn --argjson s "$step" --arg a "$alive" '{step: $s, sandboxStatusAfter: $a}')"
    break
  }
done
node_cadvisor "$node" "$fixture_dir/cadvisor-fs.txt"
measure cadvisor-fs-series "$(grep -F "pod=\"$pod\"" "$fixture_dir/cadvisor-fs.txt" | grep -E '^container_fs_(usage|limit)_bytes' \
  | awk '{print substr($1, 1, 400), $2}' | jq -Rsc 'split("\n") | map(select(length > 0))')"

echo "==> network byte counters while the sandbox downloads and uploads (w22/002)"
node_cadvisor "$node" "$fixture_dir/cadvisor-net-0.txt"
node_summary "$node" "$fixture_dir/summary-net-0.json"
if printf '%s' "$tools" | grep -qw curl; then
  net_cmd="curl -fsS -o /dev/null --max-time 120 'https://speed.cloudflare.com/__down?bytes=$down_bytes' && head -c $up_bytes /dev/urandom | curl -fsS -o /dev/null --max-time 120 -X POST --data-binary @- https://speed.cloudflare.com/__up"
elif printf '%s' "$tools" | grep -qw python3; then
  net_cmd="python3 -c \"import urllib.request as u; u.urlopen('https://speed.cloudflare.com/__down?bytes=$down_bytes', timeout=120).read()\""
else
  net_cmd=""
fi
if [ -n "$net_cmd" ]; then
  if sandbox_exec "$sandbox_id" "$net_cmd" "$fixture_dir/net.txt"; then net_ok=true; else net_ok=false; fi
  sleep 20
  node_cadvisor "$node" "$fixture_dir/cadvisor-net-1.txt"
  node_summary "$node" "$fixture_dir/summary-net-1.json"
  # Every cAdvisor network series of this pod, with its labels, before and after.
  series() { grep -F "pod=\"$pod\"" "$1" | grep -E '^container_network_(receive|transmit)_bytes_total' | awk '{print substr($1, 1, 400), $2}' | jq -Rsc 'split("\n") | map(select(length > 0))'; }
  measure network "$(jq -cn --argjson ok "$net_ok" --argjson down "$down_bytes" --argjson up "$up_bytes" \
    --argjson sb "$(pod_stats "$fixture_dir/summary-net-0.json")" --argjson sa "$(pod_stats "$fixture_dir/summary-net-1.json")" \
    --argjson cb "$(series "$fixture_dir/cadvisor-net-0.txt")" --argjson ca "$(series "$fixture_dir/cadvisor-net-1.txt")" \
    '{commandOk: $ok, expectedRxBytes: $down, expectedTxBytes: $up, kubeletBefore: $sb.network, kubeletAfter: $sa.network, cadvisorBefore: $cb, cadvisorAfter: $ca}')"
else
  measure network '{"skipped":"the sandbox has neither curl nor python3"}'
fi

echo "==> the same counters through Prometheus (its cAdvisor job drops container=\"\" series)"
prom_forward
measure prometheus "$(jq -cn --arg up "$([ -n "$prom_url" ] && echo true || echo false)" \
  --argjson rx "$(prom_query "container_network_receive_bytes_total{namespace=\"$namespace_a\",pod=\"$pod\"}")" \
  --argjson tx "$(prom_query "container_network_transmit_bytes_total{namespace=\"$namespace_a\",pod=\"$pod\"}")" \
  --argjson cpu "$(prom_query "count(container_cpu_usage_seconds_total{namespace=\"$namespace_a\",pod=\"$pod\",container!=\"\"})")" \
  --argjson fs "$(prom_query "container_fs_usage_bytes{namespace=\"$namespace_a\",pod=\"$pod\"}")" \
  '{reachable: ($up == "true"), networkReceiveSeries: ($rx | length), networkTransmitSeries: ($tx | length), cpuSeriesWithContainer: $cpu, fsUsageSeries: ($fs | length)}')"

pass "kubelet leg measured pod $pod on $node"
