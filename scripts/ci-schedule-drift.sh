#!/usr/bin/env bash
# ci-schedule-drift.sh — find scheduled workflows that have stopped running on
# time, even when every run they do make is green.
#
# Why this exists: ci-red-streak.sh catches a workflow that keeps failing, but a
# scheduled probe that passes and fires hours late is invisible to it. w7/055
# found exactly that by reading run history by hand: ssh-edge-liveness requested
# every six hours and GitHub created the runs 3h04m-5h27m after their slots, so
# half the gaps blew past the window it advertised. Each run looked fine; only
# the sequence showed the canary was not watching when it claimed to be.
#
# The same read found something worse while this was being built: every daily
# `infra (terraform)` drift run from 2026-08-30 to 2026-09-09 was CANCELLED. The
# schedule fired eleven times and not one run looked at the infrastructure. A
# checker that counted "the schedule fired" would have reported those eleven days
# as healthy, so a run only counts once it reaches a verdict — the same filter
# ci-red-streak applies: completed, and not cancelled, skipped or neutral.
#
# Like ci-red-streak this gates nothing (binding releases to an advisory checker
# is a worse trade than the problem). It runs daily and keeps one issue open
# while any workflow is outside its window.
#
# Usage:
#   ci-schedule-drift.sh            check run history against the windows below
#   ci-schedule-drift.sh validate   every scheduled workflow declares a window,
#                                   and every declared window names one
#
# A workflow is reported when, among its `event: schedule` runs:
#   - none reached a verdict in the runs inspected (never fired — the case a
#     naive gap computation reads as healthy, because there is no gap);
#   - the newest verdict is older than its window (overdue now); or
#   - two consecutive verdicts were further apart than its window, and the later
#     one landed inside the lookback (late, since recovered — a daily checker
#     would otherwise never see a gap that closed between two of its own runs).
#
# Config:
#   BEX_CI_DRIFT_LOOKBACK_HOURS  how far back a recovered gap is still reported
#                                (default 48: two daily checks see it)
#   BEX_CI_RUN_LIMIT             scheduled runs to fetch per workflow (default 20)
#   BEX_CI_DRIFT_RUNS_JSON       read runs from this file, a JSON object keyed by
#                                workflow file, instead of the GitHub API
#   BEX_CI_DRIFT_WINDOWS         read windows from this file instead of the table
#   BEX_CI_DRIFT_WORKFLOWS_DIR   workflows to validate (default .github/workflows)
#   BEX_CI_DRIFT_NOW             "now" as epoch seconds
#   (the last four are the self-test's seams; never set in CI)
#
# Exit 0 when every workflow is inside its window, 1 when at least one is not
# (or `validate` finds a mismatch), 2 when the script cannot do its job.

set -euo pipefail

# The windows, in one place. Each is an ALERTING THRESHOLD in hours, set above
# the worst gap measured for that workflow — not a promised detection bound, and
# not to be quoted as one (ADR088 §6). Measured 2026-09-23 over up to 100
# `event: schedule` runs each, counting only runs that reached a verdict and
# excluding the 2026-08-30..09-09 cancellation outage, which is what a window
# exists to catch. `-` exempts a workflow, with the reason on the line above.
WINDOWS="
# w7/055's handoff. Requested every 6h; measured gaps 4h25m-9h30m since 09-05.
ssh-edge-liveness.yml 12
# Daily. Worst measured 34h24m (infra); one skipped day (~48h) must still trip.
infra.yml 40
ci-red-streak.yml 40
# Weekly. Worst measured 178h30m (build-toolchain-freshness); one skipped week
# (~336h) must still trip.
build-toolchain-freshness.yml 216
cli-release-staleness.yml 216
deploy-canary.yml 216
isolation-matrix.yml 216
render-schema-drift.yml 216
# This checker. It cannot observe its own absence, and its first run would see
# no completed run of itself and report \"never fired\".
ci-schedule-drift.yml -
"

LOOKBACK="${BEX_CI_DRIFT_LOOKBACK_HOURS:-48}"
LIMIT="${BEX_CI_RUN_LIMIT:-20}"
here="$(cd "$(dirname "$0")" && pwd)"
WORKFLOWS_DIR="${BEX_CI_DRIFT_WORKFLOWS_DIR:-$here/../.github/workflows}"

unusable() { echo "UNUSABLE: $*" >&2; exit 2; }

command -v jq >/dev/null || unusable "missing required command: jq"

if [ -n "${BEX_CI_DRIFT_WINDOWS:-}" ]; then
  [ -r "$BEX_CI_DRIFT_WINDOWS" ] || unusable "BEX_CI_DRIFT_WINDOWS is not readable: $BEX_CI_DRIFT_WINDOWS"
  WINDOWS="$(cat "$BEX_CI_DRIFT_WINDOWS")"
fi
# "<workflow file> <hours|->" per line, comments and blanks dropped.
windows="$(printf '%s\n' "$WINDOWS" | sed -e 's/#.*//' -e '/^[[:space:]]*$/d')"
while read -r file hours extra; do
  [ -z "$extra" ] || unusable "window line has extra fields: $file $hours $extra"
  case "$hours" in
    -) ;;
    '' | *[!0-9]* | 0) unusable "window for $file is not a positive whole number of hours: '$hours'" ;;
  esac
done <<EOF
$windows
EOF

if [ "${1:-check}" = validate ]; then
  [ -d "$WORKFLOWS_DIR" ] || unusable "workflows directory not found: $WORKFLOWS_DIR"
  # A workflow is scheduled when its `on:` block has a `schedule:` key, which in
  # this repo's layout is always at two-space indent.
  scheduled="$(grep -l -E '^  schedule:' "$WORKFLOWS_DIR"/*.yml 2>/dev/null | xargs -n1 basename | sort || true)"
  declared="$(printf '%s\n' "$windows" | awk '{print $1}' | sort)"
  missing="$(comm -23 <(printf '%s\n' "$scheduled") <(printf '%s\n' "$declared") | sed '/^$/d')"
  stale="$(comm -13 <(printf '%s\n' "$scheduled") <(printf '%s\n' "$declared") | sed '/^$/d')"
  dupes="$(printf '%s\n' "$declared" | uniq -d)"
  rc=0
  if [ -n "$missing" ]; then
    echo "FAIL: scheduled workflows with no drift window — add each to WINDOWS in scripts/ci-schedule-drift.sh, from measured run history:" >&2
    printf '%s\n' "$missing" | sed 's/^/  /' >&2
    rc=1
  fi
  if [ -n "$stale" ]; then
    echo "FAIL: drift windows naming no scheduled workflow — remove them, or restore the schedule:" >&2
    printf '%s\n' "$stale" | sed 's/^/  /' >&2
    rc=1
  fi
  if [ -n "$dupes" ]; then
    echo "FAIL: workflows with more than one drift window:" >&2
    printf '%s\n' "$dupes" | sed 's/^/  /' >&2
    rc=1
  fi
  [ "$rc" -ne 0 ] || echo "PASS: every scheduled workflow has exactly one drift window ($(printf '%s\n' "$scheduled" | wc -l | tr -d ' ') workflows)."
  exit "$rc"
fi

now="${BEX_CI_DRIFT_NOW:-$(date -u +%s)}"

if [ -n "${BEX_CI_DRIFT_RUNS_JSON:-}" ]; then
  [ -r "$BEX_CI_DRIFT_RUNS_JSON" ] || unusable "BEX_CI_DRIFT_RUNS_JSON is not readable: $BEX_CI_DRIFT_RUNS_JSON"
  all_runs="$(cat "$BEX_CI_DRIFT_RUNS_JSON")"
  echo "$all_runs" | jq -e 'type == "object"' >/dev/null 2>&1 \
    || unusable "run data is not a JSON object keyed by workflow file"
else
  command -v gh >/dev/null || unusable "missing required command: gh"
fi

runs_for() { # runs_for <workflow file> — its recent scheduled runs, newest first
  if [ -n "${BEX_CI_DRIFT_RUNS_JSON:-}" ]; then
    echo "$all_runs" | jq --arg f "$1" '.[$f] // []'
  else
    gh run list --workflow "$1" --event schedule --limit "$LIMIT" \
      --json conclusion,status,event,createdAt,url 2>/dev/null \
      || unusable "could not list runs of $1"
  fi
}

hours() { # hours <seconds> — "27h04m"
  printf '%dh%02dm' $(($1 / 3600)) $(($1 % 3600 / 60))
}

report=""
exempt=""
while read -r file window; do
  if [ "$window" = - ]; then
    exempt="$exempt $file"
    continue
  fi
  runs="$(runs_for "$file")"
  echo "$runs" | jq -e 'type == "array"' >/dev/null 2>&1 \
    || unusable "run data for $file is not a JSON array"
  # One tab-separated finding per line: kind, gap seconds, gap start, gap end,
  # url. Empty fields are "-", because `read` with a tab IFS merges adjacent tabs.
  findings="$(echo "$runs" | jq -r --argjson now "$now" --argjson window "$((window * 3600))" \
    --argjson lookback "$((LOOKBACK * 3600))" '
    [ .[]
      | select(.event == "schedule" and .status == "completed")
      | select(.conclusion != "cancelled" and .conclusion != "skipped" and .conclusion != "neutral")
      | { at: (.createdAt | fromdateiso8601), createdAt, url }
    ]
    | sort_by(.at) as $v
    | if ($v | length) == 0 then "never\t-\t-\t-\t-"
      else
        # The open gap (newest verdict to now) first, then closed gaps that
        # ended inside the lookback, newest first.
        ( [ { kind: "overdue", gap: ($now - $v[-1].at), from: $v[-1], to: null } ]
          + [ range(($v | length) - 1; 0; -1)
              | { kind: "late", gap: ($v[.].at - $v[. - 1].at), from: $v[. - 1], to: $v[.] }
              | select(.to.at >= $now - $lookback) ] )
        | map(select(.gap > $window))
        | .[]
        | "\(.kind)\t\(.gap)\t\(.from.createdAt)\t\(.to.createdAt // "-")\t\(.to.url // .from.url)"
      end
  ')"
  [ -n "$findings" ] || continue
  inspected="$(echo "$runs" | jq 'length')"
  while IFS=$'\t' read -r kind gap from to url; do
    case "$kind" in
      never)
        report="$report- **$file** — no scheduled run reached a verdict in the $inspected inspected (window ${window}h). A schedule that never fires looks healthy to every other check."$'\n\n' ;;
      overdue)
        report="$report- **$file** — no scheduled run has reached a verdict for $(hours "$gap") (window ${window}h). Last: $from."$'\n'"  $url"$'\n\n' ;;
      late)
        report="$report- **$file** — consecutive scheduled verdicts $(hours "$gap") apart, $from → $to (window ${window}h)."$'\n'"  $url"$'\n\n' ;;
    esac
  done <<EOF
$findings
EOF
done <<EOF
$windows
EOF

if [ -z "$report" ]; then
  echo "OK: every scheduled workflow reached a verdict inside its window.${exempt:+ Not checked:$exempt.}"
  exit 0
fi

echo "Scheduled workflows outside their drift window (runs that reached a verdict; windows in scripts/ci-schedule-drift.sh):"
echo
printf '%s' "$report"
cat <<'EOF'
A window is an alerting threshold, not a promise: GitHub documents scheduled
workflows as best-effort, and w7/055 measured runs created up to 5h27m after
their slot. Crossing one means the probe was not watching for that long.

Look at the runs around the gap before anything else. Cancelled runs do not
count — eleven days of them is how `infra (terraform)` stopped detecting drift
in 2026-09 while its schedule kept firing. If the lateness is GitHub-side and
the window is simply wrong, widen it from fresh measurements and say so in the
commit; if it keeps growing, that reopens moving the trigger in-cluster.
EOF
exit 1
