#!/usr/bin/env bash
# gh-retry.test.sh — gh_retry retries a failing command, passes its output
# through, and gives up after three attempts.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$root/scripts/lib/gh-retry.sh"
sleep() { :; }
fail() { echo "FAIL: $*" >&2; exit 1; }

flaky() { # fails until the Nth call, then prints its argument
  local n; n=$(($(cat "$tmp/calls" 2>/dev/null || echo 0) + 1)); echo "$n" > "$tmp/calls"
  [ "$n" -ge "$1" ] && echo "ok:$2"
}
rm -f "$tmp/calls"; out=$(gh_retry flaky 3 a) || fail "third attempt should succeed"
[ "$out" = "ok:a" ] && [ "$(cat "$tmp/calls")" = 3 ] || fail "want ok:a after 3 calls, got '$out' after $(cat "$tmp/calls")"
rm -f "$tmp/calls"; if gh_retry flaky 4 b >/dev/null; then fail "a fourth attempt was made"; fi
[ "$(cat "$tmp/calls")" = 3 ] || fail "want 3 attempts, got $(cat "$tmp/calls")"
rm -f "$tmp/calls"; out=$(gh_retry flaky 1 c); [ "$out" = "ok:c" ] && [ "$(cat "$tmp/calls")" = 1 ] || fail "a success was retried"
echo "PASS: gh-retry"
