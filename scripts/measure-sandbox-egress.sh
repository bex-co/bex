#!/usr/bin/env bash
# Production proof leg for w22/m414: the operator's egress meter counts the
# public-internet bytes a /v1/sandboxes sandbox sends, attributed to its
# workspace's sandbox namespace.
#
# The meter counts bytes the pod SENDS (source = the pod's IP): a download moves
# only acknowledgements, so the leg uploads. The reference is the sandbox's own
# transmit counter (/proc/net/dev inside it), so the check is meter delta against
# what the sandbox really sent, not against what it was asked to send.
#
# Run by sandbox-measure.yml (leg `egress`) through
# scripts/verify-sandbox-isolation-live.sh, which provides one disposable,
# billing-excluded workspace. One allow-all sandbox; everything is printed as
# `MEASURE <name> <json>`.

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

up_bytes="${BEX_MEASURE_UP_BYTES:-52428800}"

echo "==> create an allow-all sandbox and read its labels"
sandbox_id="$(create_sandbox "$fixture_dir/sandbox.json" '{"type":"allow-all"}')"
wait_status "$sandbox_id" running 300
read -r pod node cr < <(find_pod "$sandbox_id")
[ -n "${pod:-}" ] && [ -n "${node:-}" ] || fail "no pod or node found for sandbox $sandbox_id"
labels="$(kubectl -n "$namespace_a" get pod "$pod" -o json | jq -c '.metadata.labels | {sandboxId: .["bex.co/sandbox-id"], agentSession: .["bex.co/agent-session"], app: .["app.bex.co/app"]}')"
measure pod "$(jq -cn --arg ns "$namespace_a" --arg pod "$pod" --arg node "$node" --argjson l "$labels" '{namespace: $ns, pod: $pod, node: $node, labels: $l}')"
check "the sandbox pod carries bex.co/sandbox-id" "$(jq -r '.sandboxId != null' <<<"$labels")" "$labels"

prom_forward
[ -n "$prom_url" ] || fail "Prometheus is not reachable through a port-forward"
series="bex_app_direct_egress_bytes_total{namespace=\"$namespace_a\",app_id=\"sandboxes\"}"
meter_value() { prom_query "$series" | jq -r '.[0].value[1] // "0"'; }

echo "==> the meter on the sandbox's node"
sleep 60 # the meter reconciles pod IPs on a timer; give it time to see the new pod
measure meter-health "$(jq -cn \
  --argjson h "$(prom_query 'bex_egress_meter_healthy')" \
  --argjson a "$(prom_query 'bex_egress_meter_attributed_pod_ips')" \
  --argjson e "$(prom_query 'bex_egress_meter_expected_pods')" \
  '{healthy: [$h[]? | {node: .metric.node, instance: .metric.instance, value: .value[1]}],
    attributedPodIps: [$a[]? | {node: .metric.node, instance: .metric.instance, value: .value[1]}],
    expectedPods: [$e[]? | {node: .metric.node, instance: .metric.instance, value: .value[1]}]}')"

echo "==> send $up_bytes bytes to a public host and read both counters"
tx() { # tx: the sandbox's own transmitted byte counter (eth0 in /proc/net/dev)
  sandbox_exec "$sandbox_id" "awk '/eth0/ { gsub(\":\", \" \"); print \$10 }' /proc/net/dev" "$fixture_dir/tx.txt" && tr -d ' \n' <"$fixture_dir/tx.txt"
}
tx_before="$(tx || echo 0)"
meter_before="$(meter_value)"
# The upload is a POST over HTTPS with the image's busybox wget to Cloudflare's
# public speed-test endpoint (the sandbox image has no curl, and a sandbox's
# plain port 80 does not reach every public host: speedtest.tele2.net timed out
# on the first production runs). The 50 MiB body is written to the sandbox's
# disk first, so what crosses the network is only the upload; wget's own error
# and exit code are part of the output.
sink_url="${BEX_MEASURE_UP_URL:-https://speed.cloudflare.com/__up}"
send_cmd="dd if=/dev/urandom of=/root/up.bin bs=1M count=$((up_bytes / 1048576)) 2>/dev/null; wget -q -T 120 -O /dev/null --post-file=/root/up.bin $sink_url 2>&1; echo \"wget-exit=\$?\"; rm -f /root/up.bin"
sandbox_exec "$sandbox_id" "$send_cmd" "$fixture_dir/send.txt" || true
measure send-output "$(jq -cn --arg o "$(head -c 300 "$fixture_dir/send.txt" 2>/dev/null | tr '\n' ' ')" --arg s "$(head -c 300 "$fixture_dir/send.sse" 2>/dev/null | tr '\n' ' ')" '{stdout: $o, sse: $s}')"
sleep 5
tx_after="$(tx || echo 0)"
sleep 75 # the meter job's scrape interval and the rule evaluation
meter_after="$(meter_value)"
sandbox_sent="$(awk -v a="${tx_before:-0}" -v b="${tx_after:-0}" 'BEGIN { printf "%.0f", b - a }')"
meter_delta="$(awk -v a="$meter_before" -v b="$meter_after" 'BEGIN { printf "%.0f", b - a }')"
measure egress "$(jq -cn --argjson req "$up_bytes" --argjson sent "${sandbox_sent:-0}" --argjson delta "${meter_delta:-0}" --arg mb "$meter_before" --arg ma "$meter_after" \
  '{requestedBytes: $req, sandboxTransmitDelta: $sent, meterBefore: $mb, meterAfter: $ma, meterDelta: $delta, meterOverSandbox: (if $sent > 0 then ($delta / $sent) else null end)}')"
check "the sandbox really sent most of what it was asked to" "$(awk -v s="${sandbox_sent:-0}" -v r="$up_bytes" 'BEGIN { print (s >= r * 0.5) ? "true" : "false" }')" "sent $sandbox_sent of $up_bytes"
check "the meter's series for this workspace rose" "$(awk -v d="${meter_delta:-0}" 'BEGIN { print (d > 0) ? "true" : "false" }')" "delta $meter_delta"
check "the meter's delta is within 10% of the bytes the sandbox sent" \
  "$(awk -v s="${sandbox_sent:-0}" -v d="${meter_delta:-0}" 'BEGIN { print (s > 0 && d >= s * 0.9 && d <= s * 1.1) ? "true" : "false" }')" "meter $meter_delta vs sandbox $sandbox_sent"

[ "$failures" = 0 ] || fail "$failures egress check(s) failed (see the lines above)"
pass "egress leg: the meter counts a sandbox's sent bytes under its workspace"
