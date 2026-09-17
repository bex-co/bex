#!/usr/bin/env bash
# ci-red-streak.sh — find workflows that have been failing on main long enough
# that they are no longer telling anyone anything.
#
# Why this exists: `gitops (render)` failed on EVERY push to main from
# 2026-09-12 to 2026-09-17 and nobody noticed. The assertion that broke it
# shipped broken and never passed once. Nothing caught it because nothing has
# to: deploy.yml's build job gates on the test suites, secret-scan and
# supersession — not on gitops — so a red render check blocks nothing and
# costs nothing to ignore. Five days and a dozen pushes later the only reason
# it surfaced was someone reading run history by hand.
#
# A permanently-red check is worse than no check. It guards nothing, and it
# teaches people that red is the normal colour — which is exactly how a REAL
# failure gets waved through next time. This does not gate anything (that trade
# was considered and rejected: binding production deploys to a static renderer
# means any flake in it stops releases). It just makes a standing red loud.
#
# A streak is consecutive failures among main's push/schedule runs, newest
# first. Cancelled and skipped runs are ignored rather than counted either way:
# a cancelled run is not evidence of health, and treating it as success would
# hide a streak that spans one.
#
# Config:
#   BEX_CI_STREAK_THRESHOLD  consecutive failures before reporting (default 3)
#   BEX_CI_RUNS_JSON         read runs from this file instead of the GitHub API
#                            (the self-test's seam; never set in CI)
#   BEX_CI_RUN_LIMIT         runs to fetch per workflow (default 30)
#
# Exit 0 when every workflow is within the threshold, 1 when at least one is
# over it, 2 when the script cannot do its job (missing tool, API failure).

set -euo pipefail

THRESHOLD="${BEX_CI_STREAK_THRESHOLD:-3}"
LIMIT="${BEX_CI_RUN_LIMIT:-30}"

unusable() { echo "UNUSABLE: $*" >&2; exit 2; }

command -v jq >/dev/null || unusable "missing required command: jq"

if [ -n "${BEX_CI_RUNS_JSON:-}" ]; then
  [ -r "$BEX_CI_RUNS_JSON" ] || unusable "BEX_CI_RUNS_JSON is not readable: $BEX_CI_RUNS_JSON"
  runs="$(cat "$BEX_CI_RUNS_JSON")"
else
  command -v gh >/dev/null || unusable "missing required command: gh"
  runs="$(gh run list --branch main --limit "$LIMIT" \
    --json name,conclusion,status,event,headSha,createdAt,url 2>/dev/null)" \
    || unusable "could not list workflow runs"
fi

echo "$runs" | jq -e 'type == "array"' >/dev/null 2>&1 \
  || unusable "run data is not a JSON array"

# One row per workflow: streak, and the run that broke it. `last_good` is the
# newest success under the streak — naming it is the whole point, because the
# commit that turned a check red is the one that has to be looked at, and
# finding it by hand is what made the gitops case take an afternoon.
report="$(echo "$runs" | jq -r --argjson threshold "$THRESHOLD" '
  [ .[]
    | select(.status == "completed")
    | select(.event == "push" or .event == "schedule")
    | select(.conclusion != "cancelled" and .conclusion != "skipped" and .conclusion != "neutral")
  ]
  | group_by(.name)
  | map({
      name: .[0].name,
      # group_by does not preserve order; sort newest-first within each workflow.
      runs: (. | sort_by(.createdAt) | reverse)
    })
  | map(
      . as $w
      # Index of the newest success == how many failures precede it. When there
      # is no success at all the streak is the whole window, which is the case
      # that matters: `first` of an empty list is null, and null // X takes the
      # fallback. Bound through $w because inside the collector `.` is the array,
      # not the workflow.
      | . + { streak: (
          ([ $w.runs | to_entries[]
             | select(.value.conclusion == "success")
             | .key ] | first) // ($w.runs | length)
        ) }
    )
  | map(
      . as $w
      | . + {
          first_failure: ($w.runs[$w.streak - 1] // null),
          last_good: ($w.runs[$w.streak] // null)
        }
    )
  | map(select(.streak >= $threshold))
  | sort_by(-.streak)
  | .[]
  | "\(.name)\t\(.streak)\t\(.first_failure.headSha[0:9] // "?")\t\(.first_failure.createdAt[0:16] // "?")\t\(.last_good.headSha[0:9] // "never")\t\(.first_failure.url // "-")"
')"

if [ -z "$report" ]; then
  echo "OK: no workflow has failed $THRESHOLD or more times in a row on main."
  exit 0
fi

echo "Workflows failing persistently on main (threshold: $THRESHOLD consecutive runs):"
echo
while IFS=$'\t' read -r name streak first_sha first_at last_good url; do
  [ -n "$name" ] || continue
  echo "- **$name** — $streak consecutive failures"
  if [ "$last_good" = never ]; then
    echo "  Never passed in the runs inspected. The check may have shipped broken."
  else
    echo "  Last good: \`$last_good\`. First failure: \`$first_sha\` ($first_at)."
  fi
  echo "  $url"
  echo
done <<EOF
$report
EOF

cat <<'EOF'
A check this red is not reporting anything. Either the assertion is wrong — as
in `d82cc98cd`, where a yq expression returned one line per document and so
could never compare equal, failing regardless of the manifest — or it is right
and something has been broken for as long as the streak.

Fix it or delete it. Leaving it red is the one option that costs something: it
normalizes a red workflow, and the next genuine failure goes unread.
EOF
exit 1
