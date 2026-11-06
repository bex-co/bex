#!/usr/bin/env bash
# tracking-issue.test.sh — scripts/lib/tracking-issue.sh against a stateful
# gh stub: open, update, close, and a create whose response is lost.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }

# State: $tmp/issues holds "<number> <state> <label>" lines; every call is logged.
cat >"$tmp/gh" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$STATE/calls"
touch "$STATE/issues"
arg() { local want=$1; shift; while [ $# -gt 0 ]; do [ "$1" = "$want" ] && { echo "$2"; return; }; shift; done; }
case "$1 $2" in
  "label create") exit 0 ;;
  "issue list")
    label=$(arg --label "$@")
    awk -v l="$label" '$2 == "open" && $3 == l { n = $1 } END { if (n) print n }' "$STATE/issues"
    ;;
  "issue edit") echo "$3" >>"$STATE/edited" ;;
  "issue create")
    n=$(($(wc -l <"$STATE/issues") + 1))
    echo "$n open $(arg --label "$@")" >>"$STATE/issues"
    # LOSE_CREATES=N: the first N creates land but the client sees a failure.
    lost=$(cat "$STATE/lost" 2>/dev/null || echo 0)
    if [ "$lost" -lt "${LOSE_CREATES:-0}" ]; then echo $((lost + 1)) >"$STATE/lost"; exit 1; fi
    ;;
  "issue close") sed -i.bak "s/^$3 open /$3 closed /" "$STATE/issues" ;;
esac
STUB
chmod +x "$tmp/gh"
echo "body" >"$tmp/body"
helper() { PATH="$tmp:$PATH" STATE="$tmp/state" GITHUB_REPOSITORY=bex-co/bex GH_RETRY_DELAY=0 bash "$root/scripts/lib/tracking-issue.sh" "$@"; }
reset() { rm -rf "$tmp/state"; mkdir -p "$tmp/state"; }
open_count() { awk -v l="$1" '$2 == "open" && $3 == l' "$tmp/state/issues" | wc -l | tr -d ' '; }

reset
helper open --label probe-down --title "Probe is down" --body-file "$tmp/body" --description "d" || fail "first open"
[ "$(open_count probe-down)" = 1 ] || fail "first open did not create one issue"
helper open --label probe-down --title "Probe is down" --body-file "$tmp/body" || fail "second open"
[ "$(open_count probe-down)" = 1 ] || fail "a second open created another issue"
grep -qx 1 "$tmp/state/edited" || fail "a second open did not edit the existing issue"
helper close --label probe-down --comment "Recovered" || fail "close"
[ "$(open_count probe-down)" = 0 ] || fail "close left the issue open"
helper close --label probe-down --comment "Recovered" || fail "close with nothing open"
echo "ok: open, update, close"

reset
LOSE_CREATES=1 helper open --label probe-down --title "Probe is down" --body-file "$tmp/body" || fail "lost-response open"
[ "$(open_count probe-down)" = 1 ] || fail "a lost create response opened $(open_count probe-down) issues"
echo "ok: a create whose response is lost opens one issue"

mkdir -p "$tmp/fail"
printf '#!/bin/sh\ncase "$1 $2" in "issue create") exit 1 ;; esac\nexit 0\n' >"$tmp/fail/gh"
chmod +x "$tmp/fail/gh"
if PATH="$tmp/fail:$PATH" GITHUB_REPOSITORY=bex-co/bex GH_RETRY_DELAY=0 bash "$root/scripts/lib/tracking-issue.sh" \
  open --label probe-down --title t --body-file "$tmp/body" 2>/dev/null; then
  fail "a create that never succeeds reported success"
fi
echo "ok: a create that keeps failing fails"

if helper open --label probe-down --body-file "$tmp/body" 2>/dev/null; then fail "open without a title was accepted"; fi
echo "PASS: tracking-issue"
