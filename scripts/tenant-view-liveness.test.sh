#!/usr/bin/env bash
# Self-test for scripts/tenant-view-liveness.sh stage 4 (w6/110, issue #134): an
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
# STUB_EVENTS_EMPTY calls, then one STUB_EVENT_TYPE event stamped now.
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
case "$url" in
  */v1/logs*) body='{"logs":[{"message":"GET /bexProbe/'"${STUB_NONCE_ANY:-x}"'"}]}' ;;
  */v1/metrics/*) body='[{"values":[{"value":1}]}]' ;;
  */events*)
    n=$(($(cat "$STUB_STATE" 2>/dev/null || echo 0) + 1))
    echo "$n" >"$STUB_STATE"
    if [ "$n" -le "${STUB_EVENTS_EMPTY:-0}" ]; then
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

# run <label> <want-rc> <needle> VAR=value… — runs the probe with the stub.
run() {
  local label="$1" want="$2" needle="$3" out got
  shift 3
  rm -f "$tmp/state"
  set +e
  out="$(env PATH="$tmp/bin:$PATH" STUB_STATE="$tmp/state" \
    BEX_CANARY_API_KEY=token BEX_CANARY_SERVICE_ID=srv-test \
    BEX_CANARY_URL=https://canary.example BEX_CANARY_REQUIRE_NONCE=0 \
    BEX_CANARY_WAKE_THRESHOLD=2 "$@" bash "$SCRIPT" 2>&1)"
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
  STUB_HOLD=3 STUB_EVENTS_EMPTY=1 STUB_EVENT_TYPE=service_woken BEX_CANARY_WOKEN_BUDGET=10
# A wake whose projection never records it is still the failure it always was.
run "held 200 with no service_woken fails" 6 "no service_woken since" \
  STUB_HOLD=3 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=service_woken BEX_CANARY_WOKEN_BUDGET=5
# A fast answer polls too, so a late-landing event is not a false red.
run "fast 200 polls for a recent event" 0 "ok: newest event" \
  STUB_HOLD=0 STUB_EVENTS_EMPTY=1 STUB_EVENT_TYPE=server_available BEX_CANARY_WOKEN_BUDGET=10
run "fast 200 with an empty feed fails after polling" 6 "after 5s of polling" \
  STUB_HOLD=0 STUB_EVENTS_EMPTY=99 STUB_EVENT_TYPE=x BEX_CANARY_WOKEN_BUDGET=5

[ "$fails" -eq 0 ] || exit 1
echo "PASS: tenant-view-liveness stage 4"
