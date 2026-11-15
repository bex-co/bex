#!/usr/bin/env bash
# Production measurement leg for w22/m321 t005 and w22/m313 t006: a suspended
# sandbox is sized from the registry, accrues storage and nothing else while it
# is suspended, stops accruing when it ends, and its images are deleted by the
# reaper without touching any other repository.
#
# Run by sandbox-measure.yml (leg `storage`) through
# scripts/verify-sandbox-isolation-live.sh, which provides one disposable,
# billing-excluded workspace A. Two sandboxes are created (S1, S2), each writes
# about 100 MB of incompressible data under /root, and both are paused. Then:
#
#   1. the registry holds each image; its size (config + layers, as bex-api
#      sizes it) is compared with the meter's `sandbox_storage_objects.bytes`;
#   2. after the meter's 15-minute accrual interval, `usage_hourly` carries
#      `sandbox_snapshot_gb_seconds` for each, and every other kind the sandbox
#      owns did not grow while it was suspended;
#   3. S1 is resumed (accrual stops) and terminated, then S2 is terminated while
#      still suspended; for each the reaper must delete the `snap-gen*` tags,
#      `reap_pending` must clear, S2's repository must be untouched while S1 is
#      reaped, and a repository outside this workspace's prefix must be
#      byte-for-byte unchanged throughout.
#
# Everything is printed as `MEASURE <name> <json>` lines; the run's log is the
# evidence. BEX_MEASURE_ACCRUAL_WAIT_MIN (default 22) bounds step 2.

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

payload_mb="${BEX_MEASURE_PAYLOAD_MB:-100}"
accrual_wait_min="${BEX_MEASURE_ACCRUAL_WAIT_MIN:-22}"
ws="$BEX_WORKSPACE_A"
failures=0
check() { # check <name> <true|false> <detail>
  if [ "$2" = true ]; then pass "$1"; else echo "FAIL: $1 ($3)"; failures=$((failures + 1)); fi
}

zot_forward
prom_forward

echo "==> baseline: stuck reap rows and the fleet's storage gauges"
reap_rows_before="$(db_query <<<"select count(*) from sandbox_storage_objects where reap_pending;")"
measure baseline "$(jq -cn --argjson r "${reap_rows_before:-0}" \
  --argjson g "$(prom_query 'bex_sandbox_storage_objects')" \
  '{reapPendingRowsFleetwide: $r, storageObjectsGauge: [$g[]? | {state: .metric.state, value: .value[1]}]}')"

# Images left behind by earlier disposable workspaces: a repository under
# snapshots/<ws>-sandbox/ whose namespace no longer exists (a workspace deleted
# while a sandbox was suspended leaves its image, as far as the code reads).
orphans="$(curl --config "$fixture_dir/zot.curl" --silent --max-time 30 "$zot_url/v2/_catalog?n=1000" \
  | jq -r '.repositories[]? | select(startswith("snapshots/")) | split("/")[1]' | sort -u | while read -r ns; do
      kubectl get namespace "$ns" >/dev/null 2>&1 || printf '%s\n' "$ns"
    done | jq -Rsc 'split("\n") | map(select(length > 0))')"
measure orphan-snapshot-namespaces "$(jq -cn --argjson o "${orphans:-[]}" '{namespacesWithImagesAndNoNamespace: $o, count: ($o | length)}')"

# A repository this run did not write: the reaper must leave it alone.
control_repo="$(curl --config "$fixture_dir/zot.curl" --silent --max-time 30 "$zot_url/v2/_catalog?n=1000" \
  | jq -r --arg p "snapshots/${ws}-sandbox/" '[.repositories[]? | select(startswith($p) | not)][0] // empty')"
control_before="[]"
[ -n "$control_repo" ] && control_before="$(zot_repo_tags "$control_repo")"
measure control-repository "$(jq -cn --arg r "${control_repo:-}" --argjson t "$control_before" '{repository: $r, tags: $t}')"

echo "==> two sandboxes write $payload_mb MB and are paused"
declare -A cr_of repo_of
for n in 1 2; do
  id="$(create_sandbox "$fixture_dir/s$n.json" '{"type":"deny-all"}')"
  printf -v "s$n" '%s' "$id"
  wait_status "$id" running 300
  read -r pod node cr < <(find_pod "$id")
  cr_of[$id]="$cr"
  sandbox_exec "$id" "dd if=/dev/urandom of=/root/payload bs=1M count=$payload_mb 2>&1 | tail -1; sync" "$fixture_dir/write-$n.txt" \
    || fail "sandbox $id could not write its payload"
  expect_status 204 POST "/v1/sandboxes/$id/pause?ownerId=$ws" "$fixture_dir/pause-$n.json"
done
for id in "$s1" "$s2"; do wait_status "$id" suspended 300; done
paused_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
measure paused "$(jq -cn --arg a "$s1" --arg b "$s2" --arg t "$paused_at" --arg ca "${cr_of[$s1]}" --arg cb "${cr_of[$s2]}" \
  '{sandboxes: [$a, $b], substrateIds: [$ca, $cb], pausedBy: $t}')"

echo "==> the registry's image for each sandbox, and the meter's reading of it"
for id in "$s1" "$s2"; do
  repo=""
  for _ in $(seq 1 30); do
    repo="$(zot_sandbox_repos | grep -F "${cr_of[$id]}" | head -1 || true)"
    [ -n "$repo" ] && break
    sleep 10
  done
  [ -n "$repo" ] || fail "no snapshot repository for sandbox $id (substrate ${cr_of[$id]}) after 5 minutes"
  repo_of[$id]="$repo"
done
for id in "$s1" "$s2"; do
  zot_size="$(zot_newest_image_bytes "${repo_of[$id]}")"
  meter_bytes=""
  for _ in $(seq 1 20); do
    meter_bytes="$(db_query -v ws="$ws" -v id="$id" <<<"select coalesce(bytes::text, '') from sandbox_storage_objects where workspace_id = :'ws' and object_id = :'id' order by started_at desc limit 1;")"
    [ -n "$meter_bytes" ] && break
    sleep 15
  done
  zot_bytes="${zot_size##* }"
  row="$(db_query -v ws="$ws" -v id="$id" <<<"select kind, started_at, ended_at, reap_pending from sandbox_storage_objects where workspace_id = :'ws' and object_id = :'id' order by started_at desc limit 1;")"
  within=false
  if [[ "${meter_bytes:-}" =~ ^[0-9]+$ ]] && [[ "$zot_bytes" =~ ^[0-9]+$ ]] && [ "$zot_bytes" -gt 0 ]; then
    diff=$((meter_bytes > zot_bytes ? meter_bytes - zot_bytes : zot_bytes - meter_bytes))
    [ $((diff * 100)) -le "$zot_bytes" ] && within=true
  fi
  measure image "$(jq -cn --arg id "$id" --arg repo "${repo_of[$id]}" --arg z "$zot_size" --arg m "${meter_bytes:-}" --arg row "$row" \
    '{sandbox: $id, repository: $repo, registryNewestImage: $z, meterBytes: $m, meterRow: $row}')"
  check "sandbox $id: the meter's bytes are within 1% of the registry image" "$within" "registry '$zot_size', meter '${meter_bytes:-}'"
done
measure unsized-gauge "$(jq -cn --argjson g "$(prom_query 'bex_sandbox_storage_objects{state="unsized_open"}')" '{unsizedOpen: [$g[]? | .value[1]]}')"

echo "==> accrual: storage only, while suspended"
kinds_sql="select kind, sum(quantity) from usage_hourly where workspace_id = :'ws' and service_id = :'id' group by kind order by kind;"
snap() { # snap <id>: the sandbox's usage_hourly totals by kind, as JSON
  db_query -v ws="$ws" -v id="$1" <<<"$kinds_sql" | jq -Rsc 'split("\n") | map(select(length > 0) | split("|") | {(.[0]): (.[1] | tonumber)}) | add // {}'
}
usage_t1="$(jq -cn --argjson a "$(snap "$s1")" --argjson b "$(snap "$s2")" '{s1: $a, s2: $b}')"
measure usage-after-pause "$usage_t1"
deadline=$((SECONDS + accrual_wait_min * 60))
accrued=false
while ((SECONDS < deadline)); do
  a="$(snap "$s1" | jq -r '.sandbox_snapshot_gb_seconds // 0')"
  b="$(snap "$s2" | jq -r '.sandbox_snapshot_gb_seconds // 0')"
  if awk -v a="$a" -v b="$b" 'BEGIN { exit !(a > 0 && b > 0) }'; then accrued=true; break; fi
  sleep 60
done
usage_t2="$(jq -cn --argjson a "$(snap "$s1")" --argjson b "$(snap "$s2")" '{s1: $a, s2: $b}')"
api_usage_ok=false
if [ "$(api_call "$fixture_dir/owner.curl" GET "/v1/usage?ownerId=$ws" "$fixture_dir/usage.json")" = 200 ]; then api_usage_ok=true; fi
measure usage-after-accrual "$(jq -cn --argjson t1 "$usage_t1" --argjson t2 "$usage_t2" --argjson ok "$api_usage_ok" \
  --argjson api "$(jq -c --arg a "$s1" --arg b "$s2" '[.services[]? | select(.serviceId == $a or .serviceId == $b) | {serviceId, resourceKind, rows}]' "$fixture_dir/usage.json" 2>/dev/null || echo '[]')" \
  '{before: $t1, after: $t2, apiUsageRead: $ok, apiRows: $api}')"
check "both suspended sandboxes accrued sandbox_snapshot_gb_seconds" "$accrued" "see usage-after-accrual"
only_storage="$(jq -cn --argjson t1 "$usage_t1" --argjson t2 "$usage_t2" '
  def strip: with_entries(.value |= with_entries(select(.key != "sandbox_snapshot_gb_seconds")));
  ($t1 | strip) == ($t2 | strip)')"
check "no other usage kind grew while the sandboxes were suspended" "$only_storage" "see usage-after-accrual"
# Expected storage: bytes/1e9 GB for the seconds the object was open and accrued.
measure accrual-expectation "$(db_query -v ws="$ws" <<<"select object_id, bytes, extract(epoch from (accrued_through - started_at))::int as accrued_seconds, remainder from sandbox_storage_objects where workspace_id = :'ws' order by object_id;" \
  | jq -Rsc 'split("\n") | map(select(length > 0) | split("|") | {object: .[0], bytes: .[1], accruedSeconds: .[2], remainder: .[3]})')"

echo "==> S1 resumes (accrual stops) and is terminated; S2 stays suspended meanwhile"
expect_status 204 POST "/v1/sandboxes/$s1/resume?ownerId=$ws" "$fixture_dir/resume-1.json"
wait_status "$s1" running 300
sleep 90
usage_resumed_a="$(snap "$s1" | jq -c '.sandbox_snapshot_gb_seconds // 0')"
sleep 150
usage_resumed_b="$(snap "$s1" | jq -c '.sandbox_snapshot_gb_seconds // 0')"
measure accrual-stops-on-resume "$(jq -cn --argjson a "$usage_resumed_a" --argjson b "$usage_resumed_b" '{storageAfter90s: $a, storageAfter240s: $b}')"
check "S1's storage accrual stopped after the resume" "$([ "$usage_resumed_a" = "$usage_resumed_b" ] && echo true || echo false)" "$usage_resumed_a then $usage_resumed_b"

tags_s2_before="$(zot_repo_tags "${repo_of[$s2]}")"
expect_status 204 POST "/v1/sandboxes/$s1/terminate?ownerId=$ws" "$fixture_dir/terminate-1.json"
reaped_1=false
for _ in $(seq 1 30); do
  left="$(zot_repo_tags "${repo_of[$s1]}" | jq '[.[] | select(startswith("snap-gen"))] | length')"
  [ "$left" = 0 ] && { reaped_1=true; break; }
  sleep 15
done
tags_s2_after="$(zot_repo_tags "${repo_of[$s2]}")"
control_mid="[]"
[ -n "$control_repo" ] && control_mid="$(zot_repo_tags "$control_repo")"
measure reap-s1 "$(jq -cn --argjson ok "$reaped_1" --argjson b "$tags_s2_before" --argjson a "$tags_s2_after" --argjson c "$control_mid" \
  '{s1TagsDeleted: $ok, s2TagsBefore: $b, s2TagsAfter: $a, controlTags: $c}')"
check "S1 (resumed, then terminated): its snap-gen tags were deleted" "$reaped_1" "tags still present after 7.5 minutes"
check "S2's repository was untouched while S1 was reaped" "$([ "$tags_s2_before" = "$tags_s2_after" ] && echo true || echo false)" "$tags_s2_before vs $tags_s2_after"

echo "==> S2 is terminated while suspended"
expect_status 204 POST "/v1/sandboxes/$s2/terminate?ownerId=$ws" "$fixture_dir/terminate-2.json"
reaped_2=false
for _ in $(seq 1 30); do
  left="$(zot_repo_tags "${repo_of[$s2]}" | jq '[.[] | select(startswith("snap-gen"))] | length')"
  [ "$left" = 0 ] && { reaped_2=true; break; }
  sleep 15
done
pending="$(db_query -v ws="$ws" <<<"select count(*) from sandbox_storage_objects where workspace_id = :'ws' and reap_pending;")"
control_after="[]"
[ -n "$control_repo" ] && control_after="$(zot_repo_tags "$control_repo")"
measure reap-s2 "$(jq -cn --argjson ok "$reaped_2" --argjson p "${pending:-0}" --argjson cb "$control_before" --argjson ca "$control_after" \
  '{s2TagsDeleted: $ok, thisWorkspaceReapPendingRows: $p, controlBefore: $cb, controlAfter: $ca}')"
check "S2 (terminated while suspended): its snap-gen tags were deleted" "$reaped_2" "tags still present after 7.5 minutes"
check "reap_pending cleared for this workspace" "$([ "${pending:-1}" = 0 ] && echo true || echo false)" "rows pending: ${pending:-?}"
# Other workspaces may add tags to it during the run; only a removal is the reaper's doing.
control_kept="$(jq -cn --argjson b "$control_before" --argjson a "$control_after" '($b - $a) | length == 0')"
check "the reaper removed no tag from the repository this run did not write" "$control_kept" "$control_before vs $control_after"
reap_rows_after="$(db_query <<<"select count(*) from sandbox_storage_objects where reap_pending;")"
measure reap-rows-fleetwide "$(jq -cn --argjson b "${reap_rows_before:-0}" --argjson a "${reap_rows_after:-0}" '{before: $b, after: $a}')"

measure_ids=() # both are terminated; skip the cleanup trap's extra wait
[ "$failures" = 0 ] || fail "$failures storage check(s) failed (see the lines above)"
pass "storage leg: sizing, accrual, resume and reaping behave as designed"
