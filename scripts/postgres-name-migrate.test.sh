#!/usr/bin/env bash
# Pure-bash regression tests for postgres-name-migrate.sh. A shell-function
# kubectl fake keeps the test clusterless while exercising the real script's
# dry-run, apply, idempotence, tenant-scoped duplicate preflight, and
# no-sensitive-output behavior.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export MOCK_DATABASE_STATE="$tmp/databases.json"
export MOCK_PATCH_LOG="$tmp/patches.tsv"
: >"$MOCK_PATCH_LOG"

kubectl() {
  local args=("$@") id="" payload="" i
  if [[ " $* " == *" get databases.app.bex.co "* ]]; then
    command cat "$MOCK_DATABASE_STATE"
    return
  fi
  if [[ " $* " == *" patch databases.app.bex.co "* ]]; then
    for ((i = 0; i < ${#args[@]}; i++)); do
      if [ "${args[$i]}" = "databases.app.bex.co" ]; then id="${args[$((i + 1))]}"; fi
      if [ "${args[$i]}" = "-p" ]; then payload="${args[$((i + 1))]}"; fi
    done
    [ -n "$id" ] && [ -n "$payload" ] || { echo "mock kubectl: malformed patch" >&2; return 2; }
    jq --arg id "$id" --argjson patch "$payload" '
      (.items[] | select(.metadata.name == $id).spec) += $patch.spec
    ' "$MOCK_DATABASE_STATE" >"$MOCK_DATABASE_STATE.next"
    command mv "$MOCK_DATABASE_STATE.next" "$MOCK_DATABASE_STATE"
    printf '%s\t%s\n' "$id" "$payload" >>"$MOCK_PATCH_LOG"
    return
  fi
  echo "mock kubectl: unsupported invocation: $*" >&2
  return 2
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

write_valid_state() {
  jq -nc '{apiVersion:"app.bex.co/v1alpha1",kind:"DatabaseList",items:[
    {metadata:{namespace:"tenant",name:"legacy-a",labels:{"bex.co/tenant":"tea-a"}},spec:{plan:"free",testSensitiveValue:"DO-NOT-PRINT"}},
    {metadata:{namespace:"tenant",name:"dpg-other-workspace",labels:{"bex.co/tenant":"tea-b"}},spec:{name:"legacy-a",plan:"free"}},
    {metadata:{namespace:"tenant",name:"dpg-current",labels:{"bex.co/tenant":"tea-a"}},spec:{name:"current",plan:"free"}}
  ]}' >"$MOCK_DATABASE_STATE"
  : >"$MOCK_PATCH_LOG"
}

echo "==> dry-run and tenant-scoped preflight"
write_valid_state
out="$(bash scripts/postgres-name-migrate.sh --namespace tenant)"
assert "dry-run plans exactly the missing legacy name" grep -q '^plan: tenant/legacy-a spec.name <- legacy-a$' <<<"$out"
assert "same display name in another workspace is allowed" test "$(grep -c '^plan:' <<<"$out")" -eq 1
assert "dry-run performs no patch" test ! -s "$MOCK_PATCH_LOG"
assert "output contains no unrelated spec value" test "${out#*DO-NOT-PRINT}" = "$out"

echo "==> apply and idempotence"
out="$(bash scripts/postgres-name-migrate.sh --apply --namespace tenant)"
assert "apply records one backfill" test "$(wc -l <"$MOCK_PATCH_LOG" | tr -d ' ')" -eq 1
assert "patch changes spec.name only" jq -e '.["spec"] == {"name":"legacy-a"}' <<<"$(cut -f2- "$MOCK_PATCH_LOG")" >/dev/null
assert "state now carries the explicit display name" jq -e '.items[] | select(.metadata.name=="legacy-a") | .spec.name=="legacy-a"' "$MOCK_DATABASE_STATE" >/dev/null
out="$(bash scripts/postgres-name-migrate.sh --apply --namespace tenant)"
assert "second apply reports already complete" grep -q 'already complete' <<<"$out"
assert "second apply performs no extra patch" test "$(wc -l <"$MOCK_PATCH_LOG" | tr -d ' ')" -eq 1

echo "==> invalid and duplicate preflight fail before writes"
jq -nc '{apiVersion:"app.bex.co/v1alpha1",kind:"DatabaseList",items:[
  {metadata:{namespace:"tenant",name:"Bad_Name"},spec:{}}
]}' >"$MOCK_DATABASE_STATE"
: >"$MOCK_PATCH_LOG"
if bash scripts/postgres-name-migrate.sh --apply --namespace tenant >"$tmp/invalid.out" 2>&1; then
  echo "FAIL: invalid legacy name was accepted" >&2
  fails=$((fails + 1))
else
  echo "    ok: invalid legacy name is rejected"
fi
assert "invalid preflight performs no patch" test ! -s "$MOCK_PATCH_LOG"

jq -nc '{apiVersion:"app.bex.co/v1alpha1",kind:"DatabaseList",items:[
  {metadata:{namespace:"tenant",name:"dpg-one",labels:{"bex.co/tenant":"tea-a"}},spec:{name:"duplicate"}},
  {metadata:{namespace:"tenant",name:"dpg-two",labels:{"bex.co/tenant":"tea-a"}},spec:{name:"duplicate"}}
]}' >"$MOCK_DATABASE_STATE"
: >"$MOCK_PATCH_LOG"
if bash scripts/postgres-name-migrate.sh --apply --namespace tenant >"$tmp/duplicate.out" 2>&1; then
  echo "FAIL: same-workspace duplicate was accepted" >&2
  fails=$((fails + 1))
else
  echo "    ok: same-workspace duplicate is rejected"
fi
assert "duplicate preflight performs no patch" test ! -s "$MOCK_PATCH_LOG"

if [ "$fails" -eq 0 ]; then
  echo "PASS: postgres name migration dry-run/apply/idempotence/preflight"
  exit 0
fi
echo "FAIL: $fails assertion(s) failed" >&2
exit 1
