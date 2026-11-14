#!/usr/bin/env bash
# ship-gates.sh — the checks /ship runs after committing and before pushing
# (w5/m112). Exits non-zero only when THIS push's own code fails:
#
#   1. Main's CI status is reported, never enforced (user decision
#      2026-10-21: never stop or wait because main is red). When the newest
#      completed (not cancelled, skipped or neutral) run on main of
#      `deploy (bex via Argo)` or of a test suite (each runs on every main push
#      touching its inputs) failed or timed out, the gate prints a warning
#      naming it so the caller pauses to fix CI first, then ships either way.
#      Pass --fixes-red-main when this push is that fix: it only changes the
#      message. One read per workflow, no waiting.
#   2. The push changes lego/backend, lego/types or the seeded authz model and
#      the backend suite fails against its real Postgres + OpenFGA + OpenBao
#      (scripts/backend-test-deps.sh), which local `go test` otherwise skips.
#      This one blocks: a failure here is the push's own defect, fixed before
#      it ships.
#   3. The push changes dashboard/ (markdown excluded) and the dashboard's own
#      CI checks fail: `yarn lint` (typecheck, eslint, knip) and `yarn test`,
#      about two minutes locally. Blocks: four red mains in two days were
#      dashboard pushes nothing checked before CI (w6/m159).
#   4. A commit in the push adds a build artifact or evidence dump: a blob over
#      1 MiB or a binary/archive under .pm/ (scripts/repo-artifact-guard.sh,
#      w6/100 D10). Blocks; this is also what stops a stale checkout from
#      replaying pre-rewrite history.
#   5. The push changes a native custody-pinned path (native-common-qualify.py
#      custody-paths), a .github/scripts native module or a native-* workflow,
#      and the native custody suites fail (about a minute). Blocks: one-line
#      edits to pinned files passed every local check and turned native-common
#      red for hours (w6/107).
#   6. The push changes a test source, a suite workflow or the checker, and a
#      file a suite's tests read is missing from that suite's trigger paths
#      (scripts/ci_test_inputs.py). Blocks: a change to the unlisted file would
#      skip the suite on every push until the next batched deploy.
#
# Gates 2, 3, 5 and 6 and the gofmt check run in a detached worktree at HEAD,
# never in the working tree: a gate run beside an edit in progress tested the
# edit (a false failure), and an uncommitted file a test needed would pass a
# push whose commit lacks it. One worktree per checkout, reused so Go's build
# cache and the dashboard's node_modules stay warm (the first run checks out
# the tree and installs from scratch); a gate already running from the same
# checkout makes the next one use a throwaway worktree.
# BEX_SHIP_GATES_TREE overrides the worktree path.
#
#   bash scripts/ship-gates.sh [--fixes-red-main] <base-ref>   # e.g. origin/main
set -euo pipefail

fixes_red_main=0
base=""
for arg in "$@"; do
  case "$arg" in
    --fixes-red-main) fixes_red_main=1 ;;
    *) base=$arg ;;
  esac
done
[ -n "$base" ] || {
  echo "usage: $0 [--fixes-red-main] <base-ref>" >&2
  exit 2
}
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

# Repository security settings (w3/m365): a warning, never a stop. Push
# protection is readable only with an admin token, which shippers hold and the
# daily github-repo-security-drift workflow's GITHUB_TOKEN does not, so this is
# where it is checked in full. It runs in the background, one try per call, and
# is read at the end; an unreadable answer (no gh, no network) is quiet.
security_out=$(mktemp)
GH_RETRY_ATTEMPTS=1 bash scripts/github-repo-security-drift.sh >"$security_out" 2>/dev/null &
security_pid=$!

# No --branch: that filter turns GitHub's run listing into a search, which can
# answer with a page of only old runs (it once blocked a green main on a
# month-old failure). These workflows also run on pull requests and manual
# dispatches, so filter the branch and event here and take the newest verdict
# by createdAt; the verdict filter is ci-red-streak.sh's. A batched deploy run
# that deploys nothing concludes success, so the suites, which since then run
# in their own workflows, are read directly. The reads run in parallel.
red_main=0
ci_runs=$(mktemp -d)
ci_pids=""
workflows="deploy.yml $(python3 -B scripts/ci_test_inputs.py --suites)"
for workflow in $workflows; do
  (gh run list --workflow "$workflow" --limit 20 \
    --json conclusion,status,headSha,headBranch,event,url,createdAt >"$ci_runs/$workflow" 2>/dev/null \
    || rm -f "$ci_runs/$workflow") &
  ci_pids="$ci_pids $!"
done
# shellcheck disable=SC2086 # one pid per word
wait $ci_pids
for workflow in $workflows; do
  if [ ! -s "$ci_runs/$workflow" ]; then
    echo "ship-gates: warning: could not read main's CI status (gh unavailable?)" >&2
    break
  fi
  latest=$(jq -r 'map(select(.headBranch == "main" and .event != "pull_request" and .status == "completed"
        and .conclusion != "cancelled" and .conclusion != "skipped" and .conclusion != "neutral"))
    | max_by(.createdAt)
    | if . == null then "" else "\(.conclusion) \(.headSha[0:9]) \(.createdAt) \(.url)" end' "$ci_runs/$workflow")
  if [ -z "$latest" ]; then
    [ "$workflow" != deploy.yml ] || echo "ship-gates: warning: no completed deploy run on main to judge" >&2
    continue
  fi
  read -r conclusion sha created url <<<"$latest"
  case "$conclusion" in
    failure | timed_out | startup_failure)
      echo "ship-gates: warning: main is red — $workflow run for $sha (created $created) ended $conclusion: $url" >&2
      red_main=1
      ;;
  esac
done
rm -rf "$ci_runs"
if [ "$red_main" = 1 ]; then
  if [ "$fixes_red_main" = 1 ]; then
    echo "ship-gates: this push is marked as the fix; shipping" >&2
  else
    echo "ship-gates: fix CI first if you can (ship the fix alone, then resume); if it cannot be fixed now, ship anyway — never stop or wait" >&2
  fi
fi

bash scripts/repo-artifact-guard.sh "$base"

# Which gates this push needs, from git objects only. Captured, not piped into
# grep: under pipefail an early-exiting grep can SIGPIPE git and make a large
# change look like no change. A bad base fails the assignment, and set -e
# fails the gate closed.
go_changed=$(git diff --name-only --diff-filter=ACMR "$base"..HEAD -- 'lego/*.go')
backend_changed=$(git diff --name-only "$base"..HEAD -- lego/backend lego/types deploy/gitops/authz ':(exclude)*.md')
dashboard_changed=$(git diff --name-only "$base"..HEAD -- dashboard ':(exclude)*.md')
all_changed=$(git diff --name-only "$base"..HEAD)
# The suite definitions and the custody list come from their modules (no copy
# to drift), captured first so a broken module fails the gate closed.
suite_inputs_changed=$(python3 -B scripts/ci_test_inputs.py --affected <<<"$all_changed")
custody=$(python3 -B .github/scripts/native-common-qualify.py custody-paths)
native_changed=$({
  grep -xF -f <(printf '%s\n' "$custody") <<<"$all_changed"
  grep -E '^\.github/(scripts/[^/]*native[^/]*|workflows/native-[^/]*)$' <<<"$all_changed"
} || true)

tree=""
lock=""
throwaway=""
release_tree() {
  if [ -n "$throwaway" ]; then
    git -C "$root" worktree remove --force "$tree" >/dev/null 2>&1 || true
    rm -rf "$throwaway"
  fi
  [ -z "$lock" ] || rm -rf "$lock"
}
trap release_tree EXIT
common_dir() { (cd "$1" && cd "$(git rev-parse --git-common-dir)" && pwd -P) 2>/dev/null; }
# Every gate below that reads the tree must appear here.
if [ -n "$go_changed$backend_changed$dashboard_changed$suite_inputs_changed$native_changed" ]; then
  # Keyed on the repository, not the checkout: linked worktrees (agent
  # checkouts) share one warm worktree instead of each leaving its own.
  common=$(common_dir .)
  tree=${BEX_SHIP_GATES_TREE:-"${XDG_CACHE_HOME:-$HOME/.cache}/bex-ship-gates/$(printf '%s' "$common" | shasum | cut -c1-8)"}
  mkdir -p "$(dirname "$tree")"
  # One gate at a time per worktree: a lock whose holder has exited is stale,
  # and a live one sends this gate to a throwaway worktree.
  if mkdir "$tree.lock" 2>/dev/null \
    || { ! kill -0 "$(cat "$tree.lock/pid" 2>/dev/null)" 2>/dev/null && rm -rf "$tree.lock" && mkdir "$tree.lock" 2>/dev/null; }; then
    lock="$tree.lock"
    echo $$ >"$lock/pid"
  else
    throwaway=$(mktemp -d)
    tree="$throwaway/tree"
  fi
  head=$(git rev-parse HEAD)
  if [ "$(common_dir "$tree")" = "$common" ]; then
    git -C "$tree" checkout -q --force --detach "$head"
    # Untracked files from an earlier commit go; ignored caches stay.
    git -C "$tree" clean -qfd
  else
    rm -rf "$tree"
    git worktree prune
    git worktree add -q --force --detach "$tree" "$head"
  fi
  echo "ship-gates: testing $(git rev-parse --short HEAD) in $tree" >&2
  cd "$tree"
fi

# The Go files this push changes must be gofmt-clean: only the operator's
# golangci-lint config runs the formatters, so backend, types and cli files
# drifted without any gate noticing (w21/060). A failing gofmt fails the gate
# closed.
if [ -n "$go_changed" ]; then
  unformatted=$(xargs gofmt -l <<<"$go_changed")
  if [ -n "$unformatted" ]; then
    echo "ship-gates: gofmt would change these files; run gofmt -w on them and amend:" >&2
    printf '%s\n' "$unformatted" >&2
    exit 1
  fi
fi
if [ -n "$suite_inputs_changed" ]; then
  python3 -B scripts/ci_test_inputs.py >&2
fi
if [ -n "$backend_changed" ]; then
  echo "ship-gates: backend/types/authz changed — running the backend suite against real dependencies" >&2
  bash scripts/backend-test-deps.sh up
  eval "$(bash scripts/backend-test-deps.sh env)"
  # No KUBECONFIG: with the test database set, it would point live-acceptance
  # tests at whatever cluster the caller has open.
  # The CRD contract's own tests, as backend-test.yml runs them (w4/589).
  (cd lego/types && GOWORK=off go test ./...)
  (cd lego/backend && env -u KUBECONFIG GOWORK=off BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...)
  # Only on success: a failed suite leaves its database up to inspect.
  bash scripts/backend-test-deps.sh down
fi
if [ -n "$dashboard_changed" ]; then
  echo "ship-gates: dashboard changed — running its lint (typecheck, eslint, knip) and tests" >&2
  install_log=$(mktemp)
  (cd dashboard && yarn install --immutable >"$install_log" 2>&1) || {
    cat "$install_log" >&2
    exit 1
  }
  rm -f "$install_log"
  (cd dashboard && yarn lint && yarn test)
fi
if [ -n "$native_changed" ]; then
  echo "ship-gates: native custody paths changed — running the native custody suites" >&2
  python3 -B -m unittest discover -s .github/scripts -p 'test_native_oci_*.py'
fi
security_rc=0
wait "$security_pid" || security_rc=$?
cd "$root"
if [ "$security_rc" -eq 1 ]; then
  echo "ship-gates: warning: the repository's secret scanning or main's history protection drifted (w3/m365):" >&2
  grep '^DRIFT' "$security_out" >&2 || true
fi
rm -f "$security_out"
echo "ship-gates: ok" >&2
