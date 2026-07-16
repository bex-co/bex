#!/usr/bin/env bash
# Pure-bash regression tests for ipallowlist-normalize.sh. A shell-function
# kubectl fake keeps the test clusterless while exercising dry-run, apply,
# idempotence, description preservation, and the malformed-entry preflight —
# the keyvalue-name-migrate.test.sh harness shape.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export MOCK_DB_STATE="$tmp/databases.json"
export MOCK_KV_STATE="$tmp/keyvalues.json"
export MOCK_PATCH_LOG="$tmp/patches.tsv"
: >"$MOCK_PATCH_LOG"

kubectl() {
  local args=("$@") kind="" id="" payload="" i state
  for ((i = 0; i < ${#args[@]}; i++)); do
    case "${args[$i]}" in
      databases.app.bex.co|keyvalues.app.bex.co) kind="${args[$i]}" ;;
    esac
  done
  case "$kind" in
    databases.app.bex.co) state="$MOCK_DB_STATE" ;;
    keyvalues.app.bex.co) state="$MOCK_KV_STATE" ;;
    *) echo "mock kubectl: unsupported invocation: $*" >&2; return 2 ;;
  esac
  if [[ " $* " == *" get $kind "* ]]; then
    command cat "$state"
    return
  fi
  if [[ " $* " == *" patch $kind "* ]]; then
    for ((i = 0; i < ${#args[@]}; i++)); do
      if [ "${args[$i]}" = "$kind" ]; then id="${args[$((i + 1))]}"; fi
      if [ "${args[$i]}" = "-p" ]; then payload="${args[$((i + 1))]}"; fi
    done
    [ -n "$id" ] && [ -n "$payload" ] || { echo "mock kubectl: malformed patch" >&2; return 2; }
    jq --arg id "$id" --argjson patch "$payload" '
      (.items[] | select(.metadata.name == $id).spec.ipAllowList) = $patch.spec.ipAllowList
    ' "$state" >"$state.next"
    command mv "$state.next" "$state"
    printf '%s\t%s\t%s\n' "$kind" "$id" "$payload" >>"$MOCK_PATCH_LOG"
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

write_state() {
  # databases: one mixed legacy/object CR, one already-clean CR, one with no list.
  jq -nc '{items:[
    {metadata:{namespace:"tenant",name:"dpg-mixed"},spec:{plan:"free",ipAllowList:["10.0.0.0/8",{cidr:"192.168.0.0/16",description:"office"}]}},
    {metadata:{namespace:"tenant",name:"dpg-clean"},spec:{plan:"free",ipAllowList:[{cidr:"172.16.0.0/12",description:"vpn"}]}},
    {metadata:{namespace:"tenant",name:"dpg-nolist"},spec:{plan:"free"}}
  ]}' >"$MOCK_DB_STATE"
  # keyvalues: one all-legacy CR.
  jq -nc '{items:[
    {metadata:{namespace:"tenant",name:"red-legacy"},spec:{plan:"free",ipAllowList:["203.0.113.0/24","198.51.100.7/32"]}}
  ]}' >"$MOCK_KV_STATE"
  : >"$MOCK_PATCH_LOG"
}

echo "== dry run plans without writing =="
write_state
out="$(bash scripts/ipallowlist-normalize.sh --namespace tenant)"
assert "plan names the mixed database" grep -q 'plan: tenant/dpg-mixed' <<<"$out"
assert "plan names the legacy keyvalue" grep -q 'plan: tenant/red-legacy' <<<"$out"
assert "clean CR not planned" bash -c "! grep -q 'dpg-clean' <<<\"$out\""
assert "dry run wrote nothing" test ! -s "$MOCK_PATCH_LOG"

echo "== apply normalizes strings, preserves objects =="
out="$(bash scripts/ipallowlist-normalize.sh --namespace tenant --apply)"
assert "apply reports the database" grep -q 'normalized: tenant/dpg-mixed' <<<"$out"
assert "apply reports the keyvalue" grep -q 'normalized: tenant/red-legacy' <<<"$out"
assert "exactly two patches issued" test "$(wc -l <"$MOCK_PATCH_LOG" | tr -d ' ')" = 2
assert "string entry became {cidr}" bash -c "jq -e '.items[0].spec.ipAllowList[0] == {cidr:\"10.0.0.0/8\"}' \"$MOCK_DB_STATE\" >/dev/null"
assert "object entry untouched, description preserved" bash -c "jq -e '.items[0].spec.ipAllowList[1] == {cidr:\"192.168.0.0/16\",description:\"office\"}' \"$MOCK_DB_STATE\" >/dev/null"
assert "keyvalue entries all objects" bash -c "jq -e '.items[0].spec.ipAllowList | map(type) | all(. == \"object\")' \"$MOCK_KV_STATE\" >/dev/null"

echo "== second run is a no-op =="
: >"$MOCK_PATCH_LOG"
out="$(bash scripts/ipallowlist-normalize.sh --namespace tenant --apply)"
assert "databases already complete" grep -q 'databases.app.bex.co: ipAllowList normalization already complete' <<<"$out"
assert "keyvalues already complete" grep -q 'keyvalues.app.bex.co: ipAllowList normalization already complete' <<<"$out"
assert "idempotent rerun wrote nothing" test ! -s "$MOCK_PATCH_LOG"

echo "== malformed entry aborts before any write =="
jq -nc '{items:[
  {metadata:{namespace:"tenant",name:"dpg-bad"},spec:{ipAllowList:[42]}}
]}' >"$MOCK_DB_STATE"
: >"$MOCK_PATCH_LOG"
if out="$(bash scripts/ipallowlist-normalize.sh --namespace tenant --apply 2>&1)"; then
  echo "FAIL: malformed entry should abort" >&2
  fails=$((fails + 1))
else
  assert "abort names the malformed CR" grep -q 'dpg-bad' <<<"$out"
  assert "abort wrote nothing" test ! -s "$MOCK_PATCH_LOG"
fi

if [ "$fails" -gt 0 ]; then
  echo "$fails failing assertion(s)" >&2
  exit 1
fi
echo "all ipallowlist-normalize tests passed"
