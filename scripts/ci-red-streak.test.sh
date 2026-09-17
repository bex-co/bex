#!/usr/bin/env bash
# Self-test for scripts/ci-red-streak.sh. Synthetic run histories prove the
# streak arithmetic, the event/status filters, and the "never passed" case —
# without contacting GitHub.
#
# The case that matters most is (f): a workflow whose every inspected run
# failed. That is the real gitops (render) incident — an assertion that shipped
# broken on 2026-09-12 and never passed once — and a naive "count failures
# since the last success" reads an empty list there and reports a streak of
# zero, i.e. reports nothing, for the one situation most worth reporting.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/ci-red-streak.sh"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# run <name> <conclusion> <n> — n runs, newest first by construction.
runs_json() { printf '[%s]\n' "$(printf '%s' "$1" | sed 's/,$//')"; }

mk() { # mk <name> <conclusion> <sha> <minute> [event] [status]
  printf '{"name":"%s","conclusion":"%s","status":"%s","event":"%s","headSha":"%s","createdAt":"2026-09-%02dT00:00","url":"https://x/%s"},' \
    "$1" "$2" "${6:-completed}" "${5:-push}" "$3" "$4" "$3"
}

check() { # check <label> <json> <want_exit> [want_substring]
  local label="$1" json="$2" want="$3" needle="${4:-}"
  local file="$tmp/$RANDOM.json" out rc
  printf '%s' "$json" >"$file"
  set +e
  out="$(BEX_CI_RUNS_JSON="$file" bash "$SCRIPT" 2>&1)"
  rc=$?
  set -e
  if [ "$rc" != "$want" ]; then
    echo "FAIL [$label]: exit=$rc want=$want" >&2
    echo "$out" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$out" | grep -qF "$needle"; then
    echo "FAIL [$label]: output missing '$needle'" >&2
    echo "$out" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok   [$label]"
}

# (a) All green — silent.
check "all green" "$(runs_json "$(mk w success a 3)$(mk w success b 2)$(mk w success c 1)")" 0

# (b) Two failures over a success is under the default threshold of 3.
check "under threshold" "$(runs_json "$(mk w failure a 3)$(mk w failure b 2)$(mk w success c 1)")" 0

# (c) Three in a row reports, and names the last good run and the first failure.
check "at threshold" \
  "$(runs_json "$(mk w failure a 4)$(mk w failure b 3)$(mk w failure c 2)$(mk w success d 1)")" \
  1 "Last good: \`d\`. First failure: \`c\`"

# (d) A cancelled run neither counts as a failure nor resets the streak — it is
# not evidence of health, and treating it as success would hide a streak that
# happens to span one.
check "cancelled does not break the streak" \
  "$(runs_json "$(mk w failure a 5)$(mk w cancelled b 4)$(mk w failure c 3)$(mk w failure d 2)$(mk w success e 1)")" \
  1 "3 consecutive failures"

# (e) Only main's push/schedule runs count; a failing pull_request run is not a
# statement about main, and an in-progress run has no verdict yet.
check "ignores pull_request and in-progress" \
  "$(runs_json "$(mk w failure a 4 pull_request)$(mk w failure b 3 pull_request)$(mk w failure c 2 push in_progress)$(mk w success d 1)")" \
  0

# (f) Never passed in the inspected window — the gitops case. Must report the
# full window as the streak and say so, not silently read "0 failures since the
# last success" off an empty list.
check "never passed" \
  "$(runs_json "$(mk w failure a 3)$(mk w failure b 2)$(mk w failure c 1)")" \
  1 "Never passed in the runs inspected"

# (g) Workflows are independent: one red neighbour does not implicate a green one.
check "per-workflow streaks" \
  "$(runs_json "$(mk red failure a 3)$(mk red failure b 2)$(mk red failure c 1)$(mk green success d 3)$(mk green success e 2)$(mk green success f 1)")" \
  1 "**red** — 3 consecutive failures"

# (h) The threshold is configurable, and raising it above a real streak silences it.
file="$tmp/threshold.json"
printf '%s' "$(runs_json "$(mk w failure a 3)$(mk w failure b 2)$(mk w failure c 1)")" >"$file"
set +e
BEX_CI_STREAK_THRESHOLD=4 BEX_CI_RUNS_JSON="$file" bash "$SCRIPT" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" != 0 ]; then
  echo "FAIL [threshold honoured]: exit=$rc want=0 with threshold 4 over a streak of 3" >&2
  fails=$((fails + 1))
else
  echo "ok   [threshold honoured]"
fi

# (i) Unusable input is exit 2 — an operator problem, never a silent pass.
set +e
BEX_CI_RUNS_JSON="$tmp/does-not-exist.json" bash "$SCRIPT" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" != 2 ]; then
  echo "FAIL [unreadable input]: exit=$rc want=2" >&2
  fails=$((fails + 1))
else
  echo "ok   [unreadable input]"
fi

if [ "$fails" -ne 0 ]; then
  echo "ci-red-streak self-test FAILED ($fails)" >&2
  exit 1
fi
echo "PASS: ci-red-streak self-test"
