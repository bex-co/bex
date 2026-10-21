#!/usr/bin/env bash
# ship-gates.test.sh — ship-gates.sh's red-main report against a stubbed `gh`
# that prints raw run JSON, so the gate's own selection of the newest
# completed, non-cancelled run is what is tested (no network). A red main is a
# warning, never a stop (user decision 2026-10-21). The base ref is HEAD, so
# the backend-suite gate has no changes to test and does not run.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
stub() { # the JSON `gh run list --json conclusion,status,headSha,headBranch,url,createdAt` prints
  printf '%s\n' "$1" >"$tmp/runs.json"
  printf '#!/bin/sh\ncat "%s"\n' "$tmp/runs.json" >"$tmp/gh"
  chmod +x "$tmp/gh"
}
run() { PATH="$tmp:$PATH" bash "$root/scripts/ship-gates.sh" "$@" 2>"$tmp/err"; }
run_ok() { run "$@" || fail "$1: expected the ship to proceed: $(cat "$tmp/err")"; }
# The ship proceeds and the red run is named, so the caller can fix CI first.
run_warned() {
  run "$@" || fail "$1: a red main must never stop the ship: $(cat "$tmp/err")"
  grep -q "warning: main is red" "$tmp/err" || fail "red-main warning missing: $(cat "$tmp/err")"
}
run_quiet() {
  run_ok "$@"
  if grep -q "main is red" "$tmp/err"; then fail "$1: a green main reported red: $(cat "$tmp/err")"; fi
}

# One run as `gh run list` prints it: <conclusion> <status> <hour> [branch].
# The hour is its createdAt, the only thing that orders runs.
r() {
  printf '{"conclusion":"%s","status":"%s","headSha":"%02dabc123def4","headBranch":"%s","url":"https://example.test/runs/%d","createdAt":"2026-10-05T%02d:00:00Z"}' \
    "$1" "$2" "$3" "${4:-main}" "$3" "$3"
}

stub "[$(r failure completed 10)]"
run_warned HEAD
stub "[$(r timed_out completed 10)]"
run_warned HEAD
stub "[$(r startup_failure completed 10)]"
run_warned HEAD
stub "[$(r failure completed 10)]"
run_warned HEAD --fixes-red-main
grep -q "marked as the fix" "$tmp/err" || fail "--fixes-red-main not acknowledged: $(cat "$tmp/err")"
run_warned --fixes-red-main HEAD

# A run still in progress or a cancelled one is not evidence of health: look
# past it to the newest real verdict.
stub "[$(r "" in_progress 12),$(r failure completed 11),$(r success completed 10)]"
run_warned HEAD
stub "[$(r cancelled completed 12),$(r failure completed 11)]"
run_warned HEAD
stub "[$(r skipped completed 13),$(r neutral completed 12),$(r failure completed 11)]"
run_warned HEAD
stub "[$(r "" in_progress 13),$(r cancelled completed 12),$(r success completed 11),$(r failure completed 10)]"
run_quiet HEAD

# Whatever the listing order, the newest run decides.
stub "[$(r failure completed 10),$(r success completed 11)]"
run_quiet HEAD
stub "[$(r success completed 10),$(r failure completed 11)]"
run_warned HEAD

# Only main's runs judge main: a manual dispatch elsewhere does not.
stub "[$(r failure completed 12 feature),$(r success completed 11)]"
run_quiet HEAD
stub "[$(r success completed 12 feature),$(r failure completed 11)]"
run_warned HEAD
grep -q "(created 2026-10-05T11:00:00Z)" "$tmp/err" || fail "the warning names no run date: $(cat "$tmp/err")"

stub "[]"
run_ok HEAD
grep -q "warning" "$tmp/err" || fail "no completed run gave no warning"

printf '#!/bin/sh\nexit 1\n' >"$tmp/gh"
run_ok HEAD
grep -q "warning" "$tmp/err" || fail "unreadable status gave no warning"

if bash "$root/scripts/ship-gates.sh" 2>/dev/null; then fail "a missing base ref must be refused"; fi

echo "ship-gates.test: ok"
