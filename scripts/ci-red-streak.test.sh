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

# runs_json <mk output> — the runs as one JSON array, in the order given.
runs_json() { printf '[%s]\n' "$(printf '%s' "$1" | sed 's/,$//')"; }

mk() { # mk <name> <conclusion> <sha> <day> [event] [status] [branch]
  printf '{"name":"%s","conclusion":"%s","status":"%s","event":"%s","headBranch":"%s","headSha":"%s","createdAt":"2026-09-%02dT00:00","url":"https://x/%s"},' \
    "$1" "$2" "${6:-completed}" "${5:-push}" "${7:-main}" "$3" "$4" "$3"
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

# (e2) Another branch's push runs are not main's either: without `--branch`
# (w5/082) the listing holds them, and the filter must drop them.
check "ignores other branches" \
  "$(runs_json "$(mk w failure a 4 push completed feature)$(mk w failure b 3 push completed feature)$(mk w failure c 2 push completed feature)$(mk w success d 1)")" \
  0

# (e3) Listing order is not trusted: a stale run listed first must not hide
# the newer failures behind it.
check "listing order does not hide a streak" \
  "$(runs_json "$(mk w success old 1)$(mk w failure a 4)$(mk w failure b 3)$(mk w failure c 2)")" \
  1 "3 consecutive failures"

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

# mkid <workflow id> <name> <conclusion> <sha> <day> [event] — a run as the
# real fetch lists it, with the workflow id the detector groups by.
mkid() {
  printf '{"workflowDatabaseId":%s,"name":"%s","conclusion":"%s","status":"completed","event":"%s","headBranch":"main","headSha":"%s","createdAt":"2026-09-%02dT00:00","url":"https://x/%s"},' \
    "$1" "$2" "$3" "${6:-push}" "$4" "$5" "$4"
}
P=.github/workflows/nb.yml # how GitHub names a run it could not parse the workflow for

# (k) native-base-image.yml, 2026-10-30: pushes that failed because the file
# did not parse (named after the path, no jobs), then a workflow that is now
# dispatch-only and passes. Grouped by name the path-named runs could never
# clear; by id, the later dispatch success ends them.
check "later dispatch success clears parse failures" \
  "$(runs_json "$(mkid 7 nb success e 5 workflow_dispatch)$(mkid 7 $P failure d 4)$(mkid 7 $P failure c 3)$(mkid 7 $P failure b 2)$(mkid 7 $P failure a 1)")" \
  0

# (l) A dispatch success does not clear real push failures: it does not show
# the push path is healthy.
check "dispatch success keeps a real push streak" \
  "$(runs_json "$(mkid 7 nb success e 5 workflow_dispatch)$(mkid 7 nb failure d 4)$(mkid 7 nb failure c 3)$(mkid 7 nb failure b 2)$(mkid 7 nb success a 1)")" \
  1 "**nb** — 3 consecutive failures"

# (m) Parse failures with no later success are a real red (the file is still
# broken), reported under the workflow name when one is known.
check "parse failures with no later success stay red" \
  "$(runs_json "$(mkid 7 $P failure d 4)$(mkid 7 $P failure c 3)$(mkid 7 $P failure b 2)$(mkid 7 nb success a 1)")" \
  1 "**nb** — 3 consecutive failures"

# (n) Parse failures newer than the last success still count.
check "parse failures after the last success count" \
  "$(runs_json "$(mkid 7 $P failure e 5)$(mkid 7 $P failure d 4)$(mkid 7 $P failure c 3)$(mkid 7 nb success b 2 workflow_dispatch)")" \
  1 "3 consecutive failures"

# (n2) A pull request whose head branch is called main (a fork's main) runs
# its own copy of the workflow file; its success does not clear main's parse
# failures.
check "fork pull_request success keeps parse failures" \
  "$(runs_json "$(mkid 7 nb success e 5 pull_request)$(mkid 7 $P failure d 4)$(mkid 7 $P failure c 3)$(mkid 7 $P failure b 2)")" \
  1 "3 consecutive failures"

# (o) Two workflows that share a name stay separate by id.
check "same name, different workflows" \
  "$(runs_json "$(mkid 1 dup failure c 3)$(mkid 1 dup failure b 2)$(mkid 1 dup failure a 1)$(mkid 2 dup success f 3)")" \
  1 "**dup** — 3 consecutive failures"

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

# (j) The 2026-09-22 deploy.yml stall, through the real fetch path with a stub
# `gh`. deploy's three failures are spread among supersession cancels and other
# workflows' runs, so the newest $LIMIT runs across ALL workflows hold only one
# of them. The old global `gh run list --limit N` read a streak of 1 and stayed
# silent; fetching per workflow sees all three. The stub answers `--branch`
# with a stale page, as GitHub's search can, so a detector that filters by
# branch again fails here too (w5/082).
stub="$tmp/stub"
mkdir -p "$stub"
{
  printf '['
  m=59
  for c in failure cancelled cancelled cancelled cancelled failure cancelled cancelled cancelled cancelled failure success; do
    printf '{"name":"deploy","conclusion":"%s","status":"completed","event":"push","headBranch":"main","headSha":"d%02d","createdAt":"2026-09-22T00:%02d","url":"https://x/d%02d"},' "$c" "$m" "$m" "$m"
    m=$((m - 1))
    for _ in 1 2 3; do
      printf '{"name":"busy","conclusion":"success","status":"completed","event":"push","headBranch":"main","headSha":"b%02d","createdAt":"2026-09-22T00:%02d","url":"https://x/b%02d"},' "$m" "$m" "$m"
      m=$((m - 1))
    done
  done
  printf '{"name":"busy","conclusion":"success","status":"completed","event":"push","headBranch":"main","headSha":"b00","createdAt":"2026-09-22T00:00","url":"https://x/b00"}]'
} >"$stub/runs.json"
cat >"$stub/gh" <<'STUB'
#!/usr/bin/env bash
# Minimal `gh` for the fetch path: `workflow list [--jq F]` and
# `run list [--workflow W] [--branch B] --limit N`.
set -euo pipefail
runs="$(dirname "$0")/runs.json"
case "$1 $2" in
  "workflow list")
    shift 2
    filter=.
    while [ $# -gt 0 ]; do
      case "$1" in
        --jq) filter="$2"; shift 2 ;;
        *) shift ;;
      esac
    done
    jq -r "$filter" <<<'[{"id":"deploy","name":"deploy","path":".github/workflows/deploy.yml"},{"id":"busy","name":"busy","path":".github/workflows/busy.yml"},{"id":"copilot","name":"Copilot","path":"dynamic/copilot-swe-agent/copilot"}]'
    ;;
  "run list")
    shift 2
    wf="" limit=20 branch=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --workflow) wf="$2"; shift 2 ;;
        --limit) limit="$2"; shift 2 ;;
        --branch) branch="$2"; shift 2 ;;
        --json) shift 2 ;;
        *) shift ;;
      esac
    done
    # GitHub lists a workflow's runs newest first, in the order runs.json
    # holds them. A branch filter turns that into a search, which can answer
    # with a page of only old runs (cli/cli#7341): played here as the older
    # half, so a detector that filters by branch misses the newest failures.
    if [ -n "$branch" ]; then
      jq --arg wf "$wf" --argjson n "$limit" \
        '[.[] | select($wf == "" or .name == $wf)] | .[(length / 2 | floor):] | .[:$n]' "$runs"
    else
      jq --arg wf "$wf" --argjson n "$limit" \
        '[.[] | select($wf == "" or .name == $wf)] | .[:$n]' "$runs"
    fi
    ;;
  *) echo "stub gh: unexpected $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$stub/gh"
set +e
out="$(PATH="$stub:$PATH" BEX_CI_RUN_LIMIT=15 bash "$SCRIPT" 2>&1)"
rc=$?
set -e
if [ "$rc" != 1 ] || ! printf '%s' "$out" | grep -qF "**deploy** — 3 consecutive failures"; then
  echo "FAIL [interleaved deploy streak]: exit=$rc want=1 naming deploy's 3-failure streak" >&2
  echo "$out" | sed 's/^/    /' >&2
  fails=$((fails + 1))
else
  echo "ok   [interleaved deploy streak]"
fi

# (k) One transient `gh run list` failure is retried, not reported unusable
# (run 42065334477: "could not list workflow runs").
flaky="$tmp/flaky"
mkdir -p "$flaky"
cat >"$flaky/gh" <<STUB
#!/usr/bin/env bash
if [ "\$1 \$2" = "run list" ] && [ ! -e "$flaky/failed-once" ]; then
  touch "$flaky/failed-once"
  echo "Post https://api.github.com/graphql: net/http: TLS handshake timeout" >&2
  exit 1
fi
exec "$stub/gh" "\$@"
STUB
chmod +x "$flaky/gh"
set +e
out="$(PATH="$flaky:$stub:$PATH" GH_RETRY_DELAY=0 BEX_CI_RUN_LIMIT=15 bash "$SCRIPT" 2>&1)"
rc=$?
set -e
if [ "$rc" != 1 ] || [ ! -e "$flaky/failed-once" ] || ! printf '%s' "$out" | grep -qF "**deploy** — 3 consecutive failures"; then
  echo "FAIL [transient gh failure retried]: exit=$rc want=1 naming deploy's streak" >&2
  echo "$out" | sed 's/^/    /' >&2
  fails=$((fails + 1))
else
  echo "ok   [transient gh failure retried]"
fi

if [ "$fails" -ne 0 ]; then
  echo "ci-red-streak self-test FAILED ($fails)" >&2
  exit 1
fi
echo "PASS: ci-red-streak self-test"
