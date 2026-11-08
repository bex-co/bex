#!/usr/bin/env bash
# Self-test for scripts/cli-release-staleness.sh (w6/112, issue #111): after a
# history rewrite the newest bex-cli tag sits on a commit HEAD no longer
# contains, and the script must count from HEAD's rewritten copy of it — not
# from an old merge-base — or say it cannot tell.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

# repo <dir> — a repo with the script and a CLI history: c1, c2 (tagged), c3.
repo() {
  local dir="$1"
  mkdir -p "$dir/scripts" "$dir/lego/cli"
  cp "$here/cli-release-staleness.sh" "$dir/scripts/"
  git -C "$dir" init -q -b main
  for n in 1 2 3; do
    echo "$n" >"$dir/lego/cli/f.txt"
    git -C "$dir" add -A
    GIT_AUTHOR_DATE="2026-11-0${n}T00:00:00Z" git -C "$dir" commit -q -m "feat(cli): change $n"
    if [ "$n" = 2 ]; then git -C "$dir" tag bex-cli/v0.2.0; fi
  done
}

# check <label> <dir> <want-rc> <needle> [<absent>]
check() {
  local label="$1" dir="$2" want="$3" needle="$4" absent="${5:-}" out got
  set +e
  out="$(bash "$dir/scripts/cli-release-staleness.sh" 2>&1)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ] || ! printf '%s' "$out" | grep -qF "$needle" ||
    { [ -n "$absent" ] && printf '%s' "$out" | grep -qF "$absent"; }; then
    echo "FAIL: $label — exit $got (want $want): $out" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
}

repo "$tmp/plain"
check "a tag on HEAD's history lists only later commits" "$tmp/plain" 1 "change 3" "change 2"

# Rewrite every commit (new hashes, same author time and subject), keeping the
# tag on the old one, as the 2026-11-04 purge did.
repo "$tmp/rewritten"
git -C "$tmp/rewritten" filter-branch -f --msg-filter cat --env-filter 'export GIT_COMMITTER_DATE="2026-11-08T00:00:00Z"' -- main >/dev/null 2>&1
check "a rewritten tag counts from its copy, not a merge-base" "$tmp/rewritten" 1 "rewritten copy" "change 1"

repo "$tmp/lost"
git -C "$tmp/lost" checkout -q --orphan fresh
git -C "$tmp/lost" commit -q -m "feat(cli): unrelated root"
git -C "$tmp/lost" branch -q -D main
check "a tag with no copy on HEAD is reported, not miscounted" "$tmp/lost" 2 "no rewritten copy"

[ "$fails" -eq 0 ] || exit 1
echo "PASS: cli-release-staleness"
