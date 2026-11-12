#!/usr/bin/env bash
# Regression tests for scripts/obs-scrape-keep-check.sh (w6/122): the current
# tree passes; dropping a family an alert reads from its job's keep-list fails
# and names the series (the w6/m334 bex_usage_* miss).
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail=0

if ! bash scripts/obs-scrape-keep-check.sh >"$tmp/pass.out" 2>&1; then
  echo "FAIL: the current tree should pass:" >&2
  tail -5 "$tmp/pass.out" >&2
  fail=1
fi

sed 's/|api|usage)_\.\*/|api)_.*/' deploy/gitops/base/prometheus.yaml >"$tmp/prometheus.yaml"
if cmp -s deploy/gitops/base/prometheus.yaml "$tmp/prometheus.yaml"; then
  echo "FAIL: fixture edit did not apply; update this test with the bex-api keep-list" >&2
  fail=1
elif OBS_PROMETHEUS_FILE="$tmp/prometheus.yaml" bash scripts/obs-scrape-keep-check.sh >"$tmp/drop.out" 2>&1; then
  echo "FAIL: a dropped bex_usage_* family should fail the check" >&2
  fail=1
elif ! grep -q 'bex_usage_oldest_unmetered_hour_timestamp_seconds (read by UsageHourNearlyUnmeterable) is kept by no scrape job' "$tmp/drop.out"; then
  echo "FAIL: the failure should name the dropped series and its alert:" >&2
  tail -3 "$tmp/drop.out" >&2
  fail=1
fi

[ "$fail" = 0 ] && echo "PASS: obs-scrape-keep-check tests"
exit "$fail"
