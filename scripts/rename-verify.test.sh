#!/usr/bin/env bash
# Pure-bash regression tests for the snapshot error handling shared by
# keyvalue-rename-verify.sh and postgres-rename-verify.sh. A shell-function
# kubectl fake keeps the test clusterless. Guards the w9/m40 production
# finding: a transient list failure used to be swallowed (2>/dev/null || true),
# so the vanished resource read as "identity changed" in compare. Only an
# uninstalled resource type may be skipped; any other list failure must fail
# loudly and name the resource.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Fake kubectl: serves `get <resource> -o json`. The primary CR resource and
# services return one object matching MOCK_ID; everything else is empty.
# MOCK_FAIL_RESOURCE triggers MOCK_FAIL_MSG on stderr + exit 1 for that one
# resource.
kubectl() {
  local args=("$@") resource="" i
  for ((i = 0; i < ${#args[@]}; i++)); do
    if [ "${args[$i]}" = "get" ]; then resource="${args[$((i + 1))]}"; fi
  done
  if [ -n "${MOCK_FAIL_RESOURCE:-}" ] && [ "$resource" = "$MOCK_FAIL_RESOURCE" ]; then
    echo "${MOCK_FAIL_MSG:?}" >&2
    return 1
  fi
  case "$resource" in
    keyvalues.app.bex.co | databases.app.bex.co)
      jq -nc --arg id "$MOCK_ID" \
        '{items:[{apiVersion:"app.bex.co/v1alpha1",kind:"Store",metadata:{namespace:"tenant",name:$id,uid:"uid-cr"}}]}'
      ;;
    services)
      jq -nc --arg id "$MOCK_ID" \
        '{items:[{apiVersion:"v1",kind:"Service",metadata:{namespace:"tenant",name:$id,uid:"uid-svc"}}]}'
      ;;
    *)
      echo '{"items":[]}'
      ;;
  esac
}
export -f kubectl

fails=0
assert() {
  local description="$1"
  shift
  if "$@"; then
    echo "    ok: $description"
  else
    echo "FAIL: $description" >&2
    fails=$((fails + 1))
  fi
}

run_suite() {
  local script="$1" flag="$2" id="$3"
  export MOCK_ID="$id"
  local before="$tmp/$id.before" out rc

  echo "==> $script: uninstalled resource types are still tolerated"
  unset MOCK_FAIL_RESOURCE MOCK_FAIL_MSG || true
  export MOCK_FAIL_RESOURCE="ingressroutetcps.traefik.io"
  export MOCK_FAIL_MSG='error: the server doesn'\''t have a resource type "ingressroutetcps"'
  out="$(bash "scripts/$script" snapshot --namespace tenant "$flag" "$id" --output "$before" 2>&1)" && rc=0 || rc=$?
  assert "snapshot succeeds without the optional CRD" test "$rc" -eq 0
  assert "snapshot captured the CR and Service identities" test "$(wc -l <"$before" | tr -d ' ')" -eq 2

  echo "==> $script: any other list failure is loud, not silently dropped"
  export MOCK_FAIL_RESOURCE="services"
  export MOCK_FAIL_MSG='Unable to connect to the server: dial tcp: connection refused'
  out="$(bash "scripts/$script" snapshot --namespace tenant "$flag" "$id" --output "$tmp/unused" 2>&1)" && rc=0 || rc=$?
  assert "snapshot exits non-zero on a transient list failure" test "$rc" -ne 0
  assert "the failing resource is named" grep -q 'listing services in tenant failed' <<<"$out"

  echo "==> $script: compare reports the list failure, never identity churn"
  out="$(bash "scripts/$script" compare --namespace tenant "$flag" "$id" --before "$before" 2>&1)" && rc=0 || rc=$?
  assert "compare exits non-zero on a transient list failure" test "$rc" -ne 0
  assert "compare error is the list failure" grep -q 'listing services in tenant failed' <<<"$out"
  assert "compare does not claim identity changed" bash -c "! grep -q 'identity changed' <<<'$out'"

  echo "==> $script: clean compare still verifies identity"
  unset MOCK_FAIL_RESOURCE MOCK_FAIL_MSG || true
  out="$(bash "scripts/$script" compare --namespace tenant "$flag" "$id" --before "$before" 2>&1)" && rc=0 || rc=$?
  assert "clean compare passes" test "$rc" -eq 0
  assert "clean compare confirms preserved identity" grep -q 'kept every recorded object identity' <<<"$out"
}

run_suite keyvalue-rename-verify.sh --keyvalue red-test1
run_suite postgres-rename-verify.sh --database dpg-test1

if [ "$fails" -eq 0 ]; then
  echo "PASS: rename-verify snapshot error handling (keyvalue + postgres)"
  exit 0
fi
echo "FAIL: $fails assertion(s) failed" >&2
exit 1
