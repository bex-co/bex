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
  "issue edit")
    echo "$3" >>"$STATE/edited"
    cp "$(arg --body-file "$@")" "$STATE/body-$3"
    title=$(arg --title "$@")
    [ -z "$title" ] || echo "$title" >"$STATE/title-$3"
    ;;
  "issue view") cat "$STATE/body-$3" ;;
  "issue create")
    n=$(($(wc -l <"$STATE/issues") + 1))
    echo "$n open $(arg --label "$@")" >>"$STATE/issues"
    cp "$(arg --body-file "$@")" "$STATE/body-$n"
    arg --title "$@" >"$STATE/title-$n"
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
helper open --label probe-down --title "Probe is down: stage a" --body-file "$tmp/body" || fail "first titled open"
helper open --label probe-down --title "Probe is down: stage b" --body-file "$tmp/body" || fail "second titled open"
[ "$(open_count probe-down)" = 1 ] || fail "a retitling open created another issue"
[ "$(cat "$tmp/state/title-1")" = "Probe is down: stage b" ] || fail "an update kept the first title: $(cat "$tmp/state/title-1")"
echo "ok: an update retitles the open issue to the current failure"

reset
LOSE_CREATES=1 helper open --label probe-down --title "Probe is down" --body-file "$tmp/body" || fail "lost-response open"
[ "$(open_count probe-down)" = 1 ] || fail "a lost create response opened $(open_count probe-down) issues"
echo "ok: a create whose response is lost opens one issue"

reset
printf 'header\n\n' >"$tmp/header"
helper append --label secret-scan --title "Leak" --body-file "$tmp/header" --line "- run 1" || fail "first append"
helper append --label secret-scan --title "Leak" --body-file "$tmp/header" --line "- run 2" || fail "second append"
[ "$(open_count secret-scan)" = 1 ] || fail "append opened $(open_count secret-scan) issues"
[ "$(cat "$tmp/state/body-1")" = "$(printf 'header\n\n- run 1\n- run 2')" ] || fail "append lost a line: $(cat "$tmp/state/body-1")"
echo "ok: append opens one issue and keeps every line"

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
