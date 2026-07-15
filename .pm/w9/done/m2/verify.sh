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

check "login recognizes RENDER_API_KEY" \
  "Success: CLI is already authenticated." \
  "$RENDER_BIN" login --confirm -o json

check "workspace current returns the active tenant" \
  "$RENDER_WORKSPACE" \
  "$RENDER_BIN" workspace current -o json

check "workspaces lists the active tenant" \
  "$RENDER_WORKSPACE" \
  "$RENDER_BIN" workspaces -o json

check "projects responds without error" \
  "" \
  "$RENDER_BIN" projects -o json

# KeyValue lifecycle (RC3 envelope + RC4 maxmemoryPolicy fix). Cleanup runs
# even on failure so a broken run never leaves the test instance behind.
KV_NAME="verify-kv-$$"
cleanup_kv() { "$RENDER_BIN" keyvalues delete "$KV_NAME" --confirm -o json >/dev/null 2>&1 || true; }
trap cleanup_kv EXIT

check "keyvalues create accepts Render's underscore maxmemoryPolicy (RC4)" \
  "\"name\":[[:space:]]*\"$KV_NAME\"" \
  "$RENDER_BIN" keyvalues create --name "$KV_NAME" --confirm -o json

check "keyvalues list returns the real record, not zero values (RC3)" \
  "\"name\":[[:space:]]*\"$KV_NAME\"" \
  "$RENDER_BIN" keyvalues list -o json

check "keyvalues get resolves by name without 'multiple instances found' (RC3)" \
  "\"id\":[[:space:]]*\"$KV_NAME\"" \
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
