#!/usr/bin/env bash
# Regression tests for scripts/metrics-retention-guard.sh (w7/m427): the
# current tree passes; a production retention below the longest plan window, a
# missing size cap, and a cap the fill alert cannot get ahead of each fail.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail=0

kubectl kustomize deploy/gitops/overlays/prod >"$tmp/prod.yaml"
if ! METRICS_RETENTION_APPS="$tmp/prod.yaml" bash scripts/metrics-retention-guard.sh >"$tmp/pass.out" 2>&1; then
  echo "FAIL: the current tree should pass:" >&2
  tail -5 "$tmp/pass.out" >&2
  fail=1
fi

# The prometheus Application's Helm parameter named $1, as a yq path.
param() {
  echo '(select(.kind == "Application" and .metadata.name == "prometheus") | .spec.source.helm.parameters[] | select(.name == "'"$1"'"))'
}

# expect_fail <name> <yq edit of the rendered overlay> <message the guard must print>
expect_fail() {
  local name="$1" edit="$2" want="$3"
  yq "$edit" "$tmp/prod.yaml" >"$tmp/$name.yaml" # no -N: the documents stay separate
  if cmp -s "$tmp/prod.yaml" "$tmp/$name.yaml"; then
    echo "FAIL: $name: the fixture edit did not apply; update this test with the overlay's parameters" >&2
    fail=1
  elif METRICS_RETENTION_APPS="$tmp/$name.yaml" bash scripts/metrics-retention-guard.sh >"$tmp/$name.out" 2>&1; then
    echo "FAIL: $name should fail the guard" >&2
    fail=1
  elif ! grep -qF -- "$want" "$tmp/$name.out"; then
    echo "FAIL: $name should say '$want':" >&2
    tail -3 "$tmp/$name.out" >&2
    fail=1
  fi
}

expect_fail short-retention "$(param server.retention).value = \"14d\"" \
  'keeps 14d, below the 30-day metrics window'
expect_fail retention-in-hours "$(param server.retention).value = \"720h\"" \
  "server.retention is '720h'"
expect_fail no-retention "del($(param server.retention))" \
  "server.retention is 'unset'"
expect_fail no-cap "del($(param server.retentionSize))" \
  'needs server.retentionSize in GB'
expect_fail cap-at-the-volume "$(param server.retentionSize).value = \"40GB\"" \
  'at most 80%'
expect_fail cap-below-the-alert "$(param server.retentionSize).value = \"28GB\"" \
  'above 70%'

[ "$fail" = 0 ] && echo "PASS: metrics-retention-guard tests"
exit "$fail"
