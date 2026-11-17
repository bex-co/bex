#!/usr/bin/env bash
# Self-test for scripts/tenant-view-liveness.sh stage 4 (w6/110, issue #134; w4/676; w4/704): an
# activator-held wake answers 200 once after ~14s with no retry, and the probe
# must still treat it as a wake and poll for that wake's service_woken instead
# of reading the events feed once and losing the race with the projection. A
# stub curl stands in for the canary host and bex-api.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/tenant-view-liveness.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

mkdir -p "$tmp/bin"
cat >"$tmp/bin/curl" <<'STUB'
#!/usr/bin/env bash
# Stub: the canary host (-w http_code) sleeps STUB_HOLD then answers 200; the
# API writes canned JSON to -o. Events answer [] for the first
# STUB_EVENTS_EMPTY calls, then one STUB_EVENT_TYPE event stamped now; the
# first STUB_EVENTS_FAIL of those calls fail as a curl transport error.
out="" url="" wfmt=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift ;;
    -w) wfmt="$2"; shift ;;
    -K | --connect-timeout | --max-time | --data-urlencode | -X | -H | --data) shift ;;
    http*) url="$1" ;;
  esac
  shift
done
if [ -n "$wfmt" ]; then sleep "${STUB_HOLD:-0}"; printf 200; exit 0; fi
# iso <epoch> — RFC3339 UTC on GNU or BSD date.
iso() { date -u -d "@$1" +%Y-%m-%dT%H:%M:%S.123Z 2>/dev/null || date -u -r "$1" +%Y-%m-%dT%H:%M:%S.123Z; }
case "$url" in
  */v1/logs*)
    # The probe's own line, plus STUB_REQ_GAPS: space-separated seconds between
    # earlier requests from a repeating caller, newest first.
    now=$(date -u +%s)
    body='{"logs":[{"message":"requestID=\"r\" responseTimeMS=1","timestamp":"'"$(iso "$now")"'","labels":[{"name":"path","value":"/bexProbe/'"${STUB_NONCE_ANY:-x}"'"}]}'
    t=$((now - 30))
    for gap in ${STUB_REQ_GAPS:-}; do
      body+=',{"message":"clientIP=\"85.239.239.149\" requestID=\"q\" responseTimeMS=1 userAgent=\"OpenStatus/1.0\"","timestamp":"'"$(iso "$t")"'","labels":[{"name":"path","value":"/"}]}'
      t=$((t - gap))
    done
    body+=']}'
    ;;
  */v1/metrics/*) body='[{"values":[{"value":1}]}]' ;;
  */events*)
    n=$(($(cat "$STUB_STATE" 2>/dev/null || echo 0) + 1))
    echo "$n" >"$STUB_STATE"
    if [ "$n" -le "${STUB_EVENTS_FAIL:-0}" ]; then
      # w4/704: a read that never completes, as curl reports one.
      echo "curl: (28) Operation timed out after 60001 milliseconds with 0 bytes received" >&2
      exit 28
    elif [ "$n" -le "${STUB_EVENTS_EMPTY:-0}" ]; then
      body='[]'
    else
      body='[{"event":{"type":"'"$STUB_EVENT_TYPE"'","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"}}]'
    fi
    ;;
  *) body='{}' ;;
esac
printf '%s' "$body" >"${out:-/dev/stdout}"
STUB
chmod +x "$tmp/bin/curl"

# Cases that want a fast (under-threshold) answer pass BEX_CANARY_WAKE_THRESHOLD=5:
# the stub's own startup can push a 0s hold across a 1s threshold.
# run <label> <want-rc> <needle> VAR=value… — runs the probe with the stub.
run() {
  local label="$1" want="$2" needle="$3" out got
  shift 3
  rm -f "$tmp/state"
  set +e
  out="$(env PATH="$tmp/bin:$PATH" STUB_STATE="$tmp/state" \
    BEX_CANARY_API_KEY=token BEX_CANARY_SERVICE_ID=srv-test \
    BEX_CANARY_URL=https://canary.example BEX_CANARY_REQUIRE_NONCE=0 \
    BEX_CANARY_WAKE_THRESHOLD=1 BEX_CANARY_POLL_INTERVAL=1 "$@" bash "$SCRIPT" 2>&1)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want: $out" >&2
    fails=$((fails + 1))
  elif ! printf '%s' "$out" | grep -qF "$needle"; then
    echo "FAIL: $label — output lacks '$needle': $out" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
}

# #134: one held 200 is a wake; its service_woken lands after the first read.
run "held 200 counts as a wake and polls for service_woken" 0 "ok: service_woken" \
  STUB_HOLD=1 STUB_EVENTS_EMPTY=1 STUB_EVENT_TYPE=service_woken BEX_CANARY_WOKEN_BUDGET=10
# A wake whose projection never records it is still the failure it always was.
run "held 200 with no service_woken fails" 6 "no service_woken since" \
  STUB_HOLD=1 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=service_woken BEX_CANARY_WOKEN_BUDGET=2
# A fast answer polls too, so a late-landing event is not a false red.
run "fast 200 polls for a recent event" 0 "ok: newest event" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_EMPTY=1 STUB_EVENT_TYPE=server_available BEX_CANARY_WOKEN_BUDGET=10
run "fast 200 with an empty feed fails after polling" 6 "after 2s of polling" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=2

# w4/676: an uptime check every 60s keeps the fixture awake, so its feed is
# empty by design — name the caller (exit 7) instead of blaming the feed.
every60="$(printf '60 %.0s' $(seq 1 62))"
run "fast 200, empty feed, a caller every 60s is held awake" 7 \
  "held awake by OpenStatus/1.0 from 85.239.239.149 every 60s" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=1 \
  STUB_REQ_GAPS="$every60"
# A caller that began 20 min into the window left an idle stretch before it.
run "fast 200, empty feed, a caller that started late is a stall" 6 "no event in the last" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=1 \
  STUB_REQ_GAPS="$(printf '60 %.0s' $(seq 1 40))"
# Only the probe's own request and no events: nothing held it awake, so a real stall.
run "fast 200, empty feed, no other requests is a stall" 6 "no event in the last" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=1
# A 20-minute gap should have let it hibernate; no service_hibernated is a stall.
run "fast 200, empty feed, a 20-minute gap is a stall" 6 "no event in the last" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=1 \
  STUB_REQ_GAPS="60 60 1200 60 60"

# w4/704: one events read that fails to complete is retried inside the budget,
# on both stage-4 paths, instead of failing the run as a stalled feed.
run "held 200, one failed events read is retried" 0 "ok: service_woken" \
  STUB_HOLD=1 STUB_EVENTS_FAIL=1 STUB_EVENTS_EMPTY=1 STUB_EVENT_TYPE=service_woken BEX_CANARY_WOKEN_BUDGET=10
run "fast 200, one failed events read is retried" 0 "ok: newest event" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_FAIL=1 STUB_EVENTS_EMPTY=1 STUB_EVENT_TYPE=server_available BEX_CANARY_WOKEN_BUDGET=10
# A read that never answers in the budget is its own failure (8) and quotes curl.
run "fast 200, an events read that never answers exits 8" 8 "curl: (28) Operation timed out" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_FAIL=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=2
run "held 200, an events read that never answers exits 8" 8 "never answered" \
  STUB_HOLD=1 STUB_EVENTS_FAIL=99 STUB_EVENT_TYPE=service_woken BEX_CANARY_WOKEN_BUDGET=2
# Reads that fail and then answer with a stalled feed are still a stall (6).
run "fast 200, failed reads then a stalled feed is still 6" 6 "no event in the last" \
  STUB_HOLD=0 BEX_CANARY_WAKE_THRESHOLD=5 STUB_EVENTS_FAIL=1 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=2

[ "$fails" -eq 0 ] || exit 1
echo "PASS: tenant-view-liveness stage 4"
