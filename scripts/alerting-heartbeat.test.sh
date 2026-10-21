#!/usr/bin/env bash
# Self-test for scripts/alerting-heartbeat.sh (w3/m174): every failure the
# out-of-cluster heartbeat exists to catch must be reported by name, and the
# healthy shape must pass. Fixtures stand in for the cluster and the network.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/alerting-heartbeat.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fails=0
healthy() {
  dir="$tmp/f$RANDOM$RANDOM"; mkdir -p "$dir"
  printf '%s' '[{"labels":{"alertname":"Watchdog"},"status":{"state":"active"}}]' >"$dir/alerts.json"
  printf '%s' '{"status":"success","data":{"result":[{"value":[0,"0"]}]}}' >"$dir/failures.json"
  echo 200 >"$dir/api.code"; echo 404 >"$dir/dashboard.code"
}
case_run() { # <label> <want-rc> <needle>
  local out got
  set +e; out="$(BEX_HEARTBEAT_FIXTURE_DIR="$dir" "$SCRIPT" 2>&1)"; got=$?; set -e
  if [ "$got" -ne "$2" ]; then echo "FAIL: $1 — exit $got, want $2: $out" >&2; fails=$((fails + 1)); return; fi
  if [ -n "$3" ] && ! printf '%s' "$out" | grep -qF "$3"; then echo "FAIL: $1 — missing '$3': $out" >&2; fails=$((fails + 1)); return; fi
  echo "ok: $1"
}
healthy; case_run "healthy pipeline passes (a 404 page is still up)" 0 "OK: Watchdog active"
healthy; rm "$dir/alerts.json"; case_run "cluster unreachable is a failure, not a pass" 1 "cluster API or Alertmanager unreachable"
healthy; printf '[]' >"$dir/alerts.json"; case_run "missing Watchdog fails" 1 "Watchdog is not active"
healthy; printf '%s' '[{"labels":{"alertname":"Watchdog"},"status":{"state":"suppressed"}}]' >"$dir/alerts.json"; case_run "silenced Watchdog fails" 1 "Watchdog is not active"
healthy; printf '%s' '{"status":"success","data":{"result":[{"value":[0,"3"]}]}}' >"$dir/failures.json"; case_run "failed notifications fail" 1 "failed 3 notification(s)"
healthy; printf '%s' '{"status":"success","data":{"result":[]}}' >"$dir/failures.json"; case_run "no failure series reads as zero" 0 "OK:"
healthy; rm "$dir/failures.json"; case_run "Prometheus unreachable fails" 1 "Prometheus unreachable"
healthy; echo 502 >"$dir/api.code"; case_run "API 5xx fails" 1 "https://api.bex.co is down (HTTP 502"
healthy; echo 000 >"$dir/dashboard.code"; case_run "dashboard unreachable fails" 1 "https://dashboard.bex.co is down (HTTP 000"
healthy; printf '%s' '{"status":"success","data":{"result":[{"value":[0,"NaN"]}]}}' >"$dir/failures.json"; case_run "NaN failure count is not read as healthy" 1 "without a readable notification-failure count"
healthy; printf "garbage" >"$dir/api.code"; case_run "unreadable status code counts as down" 1 "https://api.bex.co is down (HTTP 000"
[ "$fails" -eq 0 ] && echo "PASS: alerting-heartbeat.sh" || { echo "FAIL: $fails case(s)" >&2; exit 1; }
