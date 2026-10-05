#!/usr/bin/env bash
# ship-gates.test.sh — ship-gates.sh's red-main gate against a stubbed `gh`
# that prints raw run JSON, so the gate's own selection of the newest
# completed, non-cancelled run is what is tested (no network). The base ref is
# HEAD, so the backend-suite gate has no changes to test and does not run.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
stub() { # the JSON `gh run list --json conclusion,status,headSha,url` prints
  printf '%s\n' "$1" >"$tmp/runs.json"
  printf '#!/bin/sh\ncat "%s"\n' "$tmp/runs.json" >"$tmp/gh"
  chmod +x "$tmp/gh"
}
run() { PATH="$tmp:$PATH" bash "$root/scripts/ship-gates.sh" "$@" 2>"$tmp/err"; }
run_ok() { run "$@" || fail "$1: expected the ship to proceed: $(cat "$tmp/err")"; }
run_blocked() {
  if run "$@"; then fail "$1: expected the ship to stop"; fi
  grep -q "main is red" "$tmp/err" || fail "red-main message missing: $(cat "$tmp/err")"
}

red='{"conclusion":"failure","status":"completed","headSha":"abc123def456","url":"https://example.test/runs/1"}'
green='{"conclusion":"success","status":"completed","headSha":"def456abc123","url":"https://example.test/runs/2"}'
pending='{"conclusion":"","status":"in_progress","headSha":"0123456789ab","url":"https://example.test/runs/3"}'
cancelled='{"conclusion":"cancelled","status":"completed","headSha":"ba9876543210","url":"https://example.test/runs/4"}'

stub "[$red]"
run_blocked HEAD
run_ok HEAD --fixes-red-main
run_ok --fixes-red-main HEAD

# A run still in progress or a cancelled one is not evidence of health: look
# past it to the newest real verdict.
stub "[$pending,$red,$green]"
run_blocked HEAD
stub "[$cancelled,$red]"
run_blocked HEAD
stub "[$pending,$cancelled,$green,$red]"
run_ok HEAD

stub "[]"
run_ok HEAD
grep -q "warning" "$tmp/err" || fail "no completed run gave no warning"

printf '#!/bin/sh\nexit 1\n' >"$tmp/gh"
run_ok HEAD
grep -q "warning" "$tmp/err" || fail "unreadable status gave no warning"

if bash "$root/scripts/ship-gates.sh" 2>/dev/null; then fail "a missing base ref must be refused"; fi

echo "ship-gates.test: ok"
