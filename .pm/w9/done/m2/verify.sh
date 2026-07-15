#!/usr/bin/env bash
# Re-run docs/cli-compatibility-checklist.md's ✅ rows against a live bex-api
# and fail loudly on regression (the scripts/webhooks-verify.sh pattern, t006).
# Invoked via `scripts/cli-compat.sh verify` (which sets up RENDER_HOST/
# RENDER_API_KEY/RENDER_WORKSPACE first) — do not run this file directly.
#
# Only checks rows that are side-effect-free and safe to re-run repeatedly
# (login, workspace current, workspaces, projects). `postgres create` is a
# genuine ✅ too but creates a live resource each run, so it's exercised by
# .pm/w9/m2/t002's evidence log instead of here.
set -uo pipefail
RENDER_BIN="${RENDER_BIN:-.pm/w9/dev-9/bin/render}"
fail=0

# check DESC WANT CMD... — runs CMD, requires exit 0 AND that its output
# contains WANT (empty WANT skips the content check, exit-status-only).
check() {
  local desc="$1" want="$2"; shift 2
  local got status
  got="$("$@" 2>&1)"; status=$?
  if [ "$status" != 0 ]; then
    echo "FAIL: $desc (exit $status)"
    echo "  got: $got"
    fail=1
  elif [ -n "$want" ] && ! grep -qF "$want" <<<"$got"; then
    echo "FAIL: $desc"
    echo "  expected to contain: $want"
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

if [ "$fail" != "0" ]; then
  echo "verify: one or more ✅ rows regressed" >&2
  exit 1
fi
echo "verify: all ✅ rows still hold"
