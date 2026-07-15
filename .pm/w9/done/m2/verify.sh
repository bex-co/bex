#!/usr/bin/env bash
# Re-run docs/cli-compatibility-checklist.md's ✅ rows against a live bex-api
# and fail loudly on regression (the scripts/webhooks-verify.sh pattern, t006).
# Invoked via `scripts/cli-compat.sh verify` (which sets up RENDER_HOST/
# RENDER_API_KEY/RENDER_WORKSPACE first) — do not run this file directly.
#
# Two groups: side-effect-free rows re-run as-is (login, workspace current,
# workspaces, projects); the KeyValue lifecycle (RC3/RC4 fix, 2026-07-15,
# dfff3034) creates a uniquely-named real resource and cleans it up in a trap
# so a failure mid-way never leaves it behind. `postgres create` is a genuine
# ✅ too but isn't cleaned up here — see .pm/w9/done/m2/t002's evidence log.
set -uo pipefail
RENDER_BIN="${RENDER_BIN:-.pm/w9/dev-9/bin/render}"
fail=0
INSTANCE_NAME=""
KV_NAME=""

cleanup() {
  if [ -n "$INSTANCE_NAME" ]; then
    "$RENDER_BIN" services delete "$INSTANCE_NAME" --confirm -o json >/dev/null 2>&1 || true
  fi
  if [ -n "$KV_NAME" ]; then
    "$RENDER_BIN" keyvalues delete "$KV_NAME" --confirm -o json >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# check DESC WANT CMD... — runs CMD, requires exit 0 AND that its output
# contains WANT (empty WANT skips the content check, exit-status-only). WANT
# is an extended-regex, not a literal — `-o json` pretty-prints with a space
# after each `:`, so JSON-field checks below use `"key":[[:space:]]*"value"`.
check() {
  local desc="$1" want="$2"; shift 2
  local got status
  got="$("$@" 2>&1)"; status=$?
  if [ "$status" != 0 ]; then
    echo "FAIL: $desc (exit $status)"
    echo "  got: $got"
    fail=1
  elif [ -n "$want" ] && ! grep -qE "$want" <<<"$got"; then
    echo "FAIL: $desc"
    echo "  expected to match: $want"
    echo "  got: $got"
    fail=1
  else
    echo "PASS: $desc"
  fi
}

# checkFields DESC CMD... — runs CMD once, then asserts every pattern in
# WANT_FIELDS (space-separated, set by the caller before calling this) is
# present in the single output. Exists because `check`'s single-pattern form
# missed a real regression once: every keyvalues subcommand exited 0 and
# echoed the record's id/name back fine while bex-api silently sent `ownerId`
# empty and dropped `maxmemoryPolicy`/`persistenceMode` entirely (Render's
# real KeyValueDetail nests them under `owner`/`options`, bex-api sent them
# flat) — 2026-07-15. A single-field spot check never would have caught it.
checkFields() {
  local desc="$1"; shift
  local got status field missing=""
  got="$("$@" 2>&1)"; status=$?
  if [ "$status" != 0 ]; then
    echo "FAIL: $desc (exit $status)"
    echo "  got: $got"
    fail=1
    return
  fi
  for field in $WANT_FIELDS; do
    grep -qE "$field" <<<"$got" || missing="$missing $field"
  done
  if [ -n "$missing" ]; then
    echo "FAIL: $desc — missing:$missing"
    echo "  got: $got"
    fail=1
  else
    echo "PASS: $desc"
  fi
}

# valid_instance_json reads a CLI JSON response from stdin and accepts only a
# non-empty ServiceInstance array with populated ids and parseable timestamps.
# Kept as one parser so the rollout poll and final assertion cannot drift.
valid_instance_json() {
  python3 -c '
import datetime, json, sys
rows = json.load(sys.stdin)
assert isinstance(rows, list) and rows
for row in rows:
    assert isinstance(row.get("id"), str) and row["id"]
    datetime.datetime.fromisoformat(row["createdAt"].replace("Z", "+00:00"))
'
}

check "login recognizes RENDER_API_KEY" \
  "Success: CLI is already authenticated." \
  "$RENDER_BIN" login --confirm -o json

# whoami (RC6 route + the BEX_KRATOS_ADMIN_URL harness wiring): an API-key
# caller reports the key-minting user's email via ownerEmail's Kratos-admin
# lookup. Exact match when cli-compat.sh exported CLI_COMPAT_EMAIL; any
# populated email otherwise (an empty `Email:` line must fail either way).
check "whoami reports the key-minting user's email (RC6)" \
  "Email:[[:space:]]*${CLI_COMPAT_EMAIL:-[^[:space:]]+@[^[:space:]]+}" \
  "$RENDER_BIN" whoami -o json

check "workspace current returns the active tenant" \
  "$RENDER_WORKSPACE" \
  "$RENDER_BIN" workspace current -o json

check "workspaces lists the active tenant" \
  "$RENDER_WORKSPACE" \
  "$RENDER_BIN" workspaces -o json

check "projects responds without error" \
  "" \
  "$RENDER_BIN" projects -o json

# Service-instance route (w6/m26): create one image-backed Deployment service,
# wait for the operator to materialize its Pod, and make the unmodified CLI
# decode the live endpoint in both JSON and text modes. The JSON validator
# rejects null, an empty rollout result, a missing id/createdAt, or a malformed
# timestamp. Then suspend through the REST lifecycle verb and prove the SAME
# CLI path returns [] before cleanup. The trap removes the service on every
# exit path, including a timeout or assertion failure.
INSTANCE_NAME="verify-instances-$$"
instance_create=$(
  "$RENDER_BIN" services create --name "$INSTANCE_NAME" --type web_service \
    --image traefik/whoami:v1.10.3 --plan free --num-instances 1 --confirm -o json 2>&1
)
instance_create_status=$?
if [ "$instance_create_status" != 0 ]; then
  echo "FAIL: services instances setup create (exit $instance_create_status)"
  echo "  got: $instance_create"
  fail=1
else
  INSTANCE_ID=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["id"])' <<<"$instance_create" 2>/dev/null || true)
  if [ -z "$INSTANCE_ID" ]; then
    echo "FAIL: services instances setup create returned no service id"
    echo "  got: $instance_create"
    fail=1
  else
    instance_json=""
    instance_status=1
    for _ in $(seq 1 30); do
      instance_json=$("$RENDER_BIN" services instances "$INSTANCE_ID" -o json 2>&1)
      instance_status=$?
      if [ "$instance_status" = 0 ] && valid_instance_json <<<"$instance_json" 2>/dev/null; then
        break
      fi
      sleep 2
    done
    if [ "$instance_status" != 0 ] || ! valid_instance_json <<<"$instance_json" 2>/dev/null; then
      echo "FAIL: services instances JSON decodes live id + createdAt"
      echo "  got: $instance_json"
      fail=1
    else
      echo "PASS: services instances JSON decodes live id + createdAt"
      first_instance_id=$(python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["id"])' <<<"$instance_json")
      check "services instances text mode renders the real instance" \
        "$first_instance_id" \
        "$RENDER_BIN" services instances "$INSTANCE_ID" -o text

      suspend_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
        -H "Authorization: Bearer $RENDER_API_KEY" \
        "${RENDER_HOST%/}/services/$INSTANCE_ID/suspend")
      if [ "$suspend_status" != 202 ]; then
        echo "FAIL: services instances suspend setup returned HTTP $suspend_status"
        fail=1
      else
        empty_json=$("$RENDER_BIN" services instances "$INSTANCE_ID" -o json 2>&1)
        empty_status=$?
        if [ "$empty_status" != 0 ] || ! python3 -c 'import json,sys; rows=json.load(sys.stdin); assert isinstance(rows,list) and not rows' <<<"$empty_json" 2>/dev/null; then
          echo "FAIL: services instances suspended service returns []"
          echo "  got: $empty_json"
          fail=1
        else
          echo "PASS: services instances suspended service returns []"
        fi
      fi
    fi
  fi
fi

missing_output=$("$RENDER_BIN" services instances srv-00000000000000000000 -o json 2>&1)
missing_status=$?
if [ "$missing_status" = 0 ] || ! grep -qE '404|not found' <<<"$missing_output"; then
  echo "FAIL: services instances missing service preserves a not-found failure"
  echo "  got (exit $missing_status): $missing_output"
  fail=1
else
  echo "PASS: services instances missing service preserves a not-found failure"
fi

# KeyValue lifecycle (RC3 envelope + RC4 maxmemoryPolicy fix, plus the
# 2026-07-15 owner/options-nesting + ipAllowList wire-shape fix below).
# Cleanup runs even on failure so a broken run never leaves the test instance
# behind.
KV_NAME="verify-kv-$$"

# Every field Render's real KeyValueDetail carries that bex-api has
# historically dropped or zero-valued silently (RC3/RC4/owner-options-nesting/
# ipAllowList-shape) — checked together so a partial regression can't hide
# behind one passing field, per checkFields' doc comment above.
WANT_FIELDS='"id":[[:space:]]*"'"$KV_NAME"'" "name":[[:space:]]*"'"$KV_NAME"'" "ownerId":[[:space:]]*"tea-[a-z0-9]+" "maxmemoryPolicy":[[:space:]]*"allkeys_lru" "persistenceMode":[[:space:]]*"journal_snapshot" "cidrBlock":[[:space:]]*"10\.0\.0\.0/8"'

checkFields "keyvalues create: owner/options nested + underscore maxmemoryPolicy (RC4) + ipAllowList shape all correct" \
  "$RENDER_BIN" keyvalues create --name "$KV_NAME" \
    --ip-allow-list "cidr=10.0.0.0/8,description=verify" --confirm -o json

checkFields "keyvalues list: same fields survive the cursor envelope (RC3)" \
  "$RENDER_BIN" keyvalues list -o json

checkFields "keyvalues get: resolves by name (RC3) with every field intact" \
  "$RENDER_BIN" keyvalues get "$KV_NAME" -o json

check "keyvalues suspend resolves and applies" \
  "\"suspended\":[[:space:]]*true" \
  "$RENDER_BIN" keyvalues suspend "$KV_NAME" --confirm -o json

# resume's downstream `status` field is async (K8s reconciliation may still say
# "unavailable" moments after the call returns) — only exit 0 + the right id
# is asserted, not the transient status, to avoid a flaky false failure.
check "keyvalues resume resolves and applies" \
  "\"id\":[[:space:]]*\"$KV_NAME\"" \
  "$RENDER_BIN" keyvalues resume "$KV_NAME" --confirm -o json

check "keyvalues delete resolves and applies" \
  "\"deleted\":[[:space:]]*true" \
  "$RENDER_BIN" keyvalues delete "$KV_NAME" --confirm -o json

if [ "$fail" != "0" ]; then
  echo "verify: one or more ✅ rows regressed" >&2
  exit 1
fi
echo "verify: all ✅ rows still hold"
