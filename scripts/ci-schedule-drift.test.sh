#!/usr/bin/env bash
# Self-test for scripts/ci-schedule-drift.sh. Synthetic run histories prove the
# gap arithmetic, the event/status filters and the window table — without
# contacting GitHub.
#
# The cases that matter most are the ones where a naive checker reports
# nothing: a workflow with ZERO scheduled runs (never fired — there is no gap to
# compute, so it reads healthy), one whose recent runs were all cancelled (the
# 2026-08-30..09-09 `infra (terraform)` outage, where the schedule fired
# daily and nothing ran), and a workflow_dispatch run standing in for a missing
# scheduled one.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/ci-schedule-drift.sh"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# "Now" is fixed so every age below is exact: 2026-09-23T12:00:00Z.
NOW=1790164800
printf 'w.yml 12\n' >"$tmp/windows"

# run <hours before NOW> [conclusion] [event] [status] — one run object.
run() {
  local at
  at="$(jq -rn --argjson t "$((NOW - $1 * 3600))" '$t | todate')"
  printf '{"createdAt":"%s","conclusion":"%s","event":"%s","status":"%s","url":"https://x/%s"},' \
    "$at" "${2:-success}" "${3:-schedule}" "${4:-completed}" "$1"
}
# runs <run>... — w.yml's history as the seam's workflow-keyed object.
runs() { printf '{"w.yml":[%s]}' "$(printf '%s' "$*" | sed 's/,$//')"; }

check() { # check <label> <json> <want_exit> [want_substring]
  local label="$1" json="$2" want="$3" needle="${4:-}"
  local file="$tmp/$RANDOM.json" out rc
  printf '%s' "$json" >"$file"
  set +e
  out="$(BEX_CI_DRIFT_NOW="$NOW" BEX_CI_DRIFT_WINDOWS="$tmp/windows" \
    BEX_CI_DRIFT_RUNS_JSON="$file" bash "$SCRIPT" 2>&1)"
  rc=$?
  set -e
  if [ "$rc" != "$want" ]; then
    echo "FAIL [$label]: exit=$rc want=$want" >&2
    echo "$out" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$out" | grep -qF -- "$needle"; then
    echo "FAIL [$label]: output missing '$needle'" >&2
    echo "$out" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok   [$label]"
}

# (a) On time — every gap inside the 12h window, newest 5h old. Silent.
check "on time" "$(runs "$(run 5)$(run 11)$(run 17)$(run 23)")" 0

# (b) One scheduled run: no gap between runs is computable, but the open gap to
# now is, and 5h is inside the window.
check "single recent run" "$(runs "$(run 5)")" 0

# (c) One scheduled run, 30h old: overdue, and says for how long.
check "single stale run" "$(runs "$(run 30)")" 1 "for 30h00m (window 12h)"

# (d) Zero scheduled runs — never fired. The worst case: a gap computation over
# an empty list finds no gap and reports nothing.
check "never fired" "$(runs)" 1 "no scheduled run reached a verdict in the 0 inspected"

# (e) A workflow_dispatch run is not a scheduled run. A manual run an hour ago
# must not hide that the schedule has not delivered for 20h.
check "workflow_dispatch not counted" "$(runs "$(run 1 success workflow_dispatch)$(run 20)")" \
  1 "for 20h00m"

# (f) Cancelled runs are not verdicts — the infra outage. The schedule fired
# every 6h, and nothing has actually looked since 24h ago.
check "cancelled runs not counted" \
  "$(runs "$(run 2 cancelled)$(run 8 cancelled)$(run 14 cancelled)$(run 20 cancelled)$(run 24)")" \
  1 "for 24h00m"

# (g) A failed run IS a verdict: the probe ran and looked. Failing is
# ci-red-streak's report, not this one's.
check "failure counts as having run" "$(runs "$(run 3 failure)$(run 9 failure)$(run 15)")" 0

# (h) An in-progress run has no verdict yet and does not reset the clock.
check "in-progress not counted" "$(runs "$(run 1 '' schedule in_progress)$(run 13)")" 1 "for 13h00m"

# (i) A late gap that has since recovered is still reported inside the lookback
# (default 48h) — a daily checker would otherwise never see a gap that closed
# between two of its own runs.
check "recovered gap inside lookback" "$(runs "$(run 2)$(run 8)$(run 30)")" \
  1 "22h00m apart"

# (j) ...and stops being reported once it ends outside the lookback.
check "recovered gap outside lookback" \
  "$(runs "$(run 2)$(run 8)$(run 14)$(run 20)$(run 26)$(run 32)$(run 38)$(run 44)$(run 50)$(run 70)")" 0

# (k) Workflows are independent, and an exempt one is never fetched or reported.
printf 'w.yml 12\nok.yml 12\nself.yml -\n' >"$tmp/windows"
check "per-workflow and exempt" \
  "$(printf '{"w.yml":[%s],"ok.yml":[%s]}' "$(run 30 | sed 's/,$//')" "$(run 1 | sed 's/,$//')")" \
  1 "**w.yml**"
check "exempt names itself when all is well" \
  "$(printf '{"w.yml":[%s],"ok.yml":[%s]}' "$(run 1 | sed 's/,$//')" "$(run 1 | sed 's/,$//')")" \
  0 "Not checked: self.yml."
printf 'w.yml 12\n' >"$tmp/windows"

# (l) Unusable input is exit 2 — an operator problem, never a silent pass.
unusable() { # unusable <label> <env assignment>...
  local label="$1" rc
  shift
  set +e
  env BEX_CI_DRIFT_NOW="$NOW" BEX_CI_DRIFT_WINDOWS="$tmp/windows" "$@" bash "$SCRIPT" >/dev/null 2>&1
  rc=$?
  set -e
  if [ "$rc" != 2 ]; then
    echo "FAIL [$label]: exit=$rc want=2" >&2
    fails=$((fails + 1))
  else
    echo "ok   [$label]"
  fi
}
unusable "unreadable runs" BEX_CI_DRIFT_RUNS_JSON="$tmp/does-not-exist.json"
printf '[]' >"$tmp/array.json"
unusable "runs not keyed by workflow" BEX_CI_DRIFT_RUNS_JSON="$tmp/array.json"
printf 'w.yml twelve\n' >"$tmp/bad-windows"
unusable "non-numeric window" BEX_CI_DRIFT_WINDOWS="$tmp/bad-windows" BEX_CI_DRIFT_RUNS_JSON="$tmp/array.json"

# (m) validate: every scheduled workflow declares a window, every window names
# a scheduled workflow. Adding a probe without a window must fail here, not go
# silently unwatched.
wf="$tmp/workflows"
mkdir -p "$wf"
printf 'on:\n  schedule:\n    - cron: "0 * * * *"\n' >"$wf/w.yml"
printf 'on:\n  push:\n    branches: [main]\n' >"$wf/push-only.yml"
validate() { # validate <label> <want_exit> [want_substring]
  local out rc
  set +e
  out="$(BEX_CI_DRIFT_WINDOWS="$tmp/windows" BEX_CI_DRIFT_WORKFLOWS_DIR="$wf" bash "$SCRIPT" validate 2>&1)"
  rc=$?
  set -e
  if [ "$rc" != "$2" ] || { [ -n "${3:-}" ] && ! printf '%s' "$out" | grep -qF -- "$3"; }; then
    echo "FAIL [$1]: exit=$rc want=$2${3:+ with '$3'}" >&2
    echo "$out" | sed 's/^/    /' >&2
    fails=$((fails + 1))
  else
    echo "ok   [$1]"
  fi
}
validate "validate: declared" 0
printf 'on:\n  schedule:\n    - cron: "0 0 * * 0"\n' >"$wf/new-probe.yml"
validate "validate: undeclared schedule fails" 1 "new-probe.yml"
rm "$wf/new-probe.yml"
printf 'w.yml 12\npush-only.yml 12\n' >"$tmp/windows"
validate "validate: window without a schedule fails" 1 "push-only.yml"
printf 'w.yml 12\nw.yml 24\n' >"$tmp/windows"
validate "validate: duplicate window fails" 1 "more than one drift window"
printf 'w.yml 12\n' >"$tmp/windows"

# (n) The real table matches the real workflows.
set +e
out="$(bash "$SCRIPT" validate 2>&1)"
rc=$?
set -e
if [ "$rc" != 0 ]; then
  echo "FAIL [repo windows match repo workflows]: exit=$rc" >&2
  echo "$out" | sed 's/^/    /' >&2
  fails=$((fails + 1))
else
  echo "ok   [repo windows match repo workflows]"
fi

if [ "$fails" -ne 0 ]; then
  echo "ci-schedule-drift self-test FAILED ($fails)" >&2
  exit 1
fi
echo "PASS: ci-schedule-drift self-test"
