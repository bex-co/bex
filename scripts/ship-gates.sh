#!/usr/bin/env bash
# ship-gates.sh — the checks /ship runs after committing and before pushing
# (w5/m112). Exits non-zero only when THIS push's own code fails:
#
#   1. Main's CI status is reported, never enforced (user decision
#      2026-10-21: never stop or wait because main is red). When the newest
#      completed (not cancelled, skipped or neutral) `deploy (bex via Argo)`
#      run on main failed or timed out, the gate prints a warning naming it so
#      the caller pauses to fix CI first, then ships either way. Pass
#      --fixes-red-main when this push is that fix: it only changes the
#      message. One read, no waiting.
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

# No --branch: that filter turns GitHub's run listing into a search, which can
# answer with a page of only old runs (it once blocked a green main on a
# month-old failure). deploy.yml runs on main pushes and manual dispatches, so
# filter the branch here and take the newest verdict by createdAt; the verdict
# filter is ci-red-streak.sh's.
if runs=$(gh run list --workflow deploy.yml --limit 20 \
  --json conclusion,status,headSha,headBranch,url,createdAt 2>/dev/null); then
  latest=$(jq -r 'map(select(.headBranch == "main" and .status == "completed"
        and .conclusion != "cancelled" and .conclusion != "skipped" and .conclusion != "neutral"))
    | max_by(.createdAt)
    | if . == null then "" else "\(.conclusion) \(.headSha[0:9]) \(.createdAt) \(.url)" end' <<<"$runs")
  if [ -z "$latest" ]; then
    echo "ship-gates: warning: no completed deploy run on main to judge" >&2
  else
    read -r conclusion sha created url <<<"$latest"
    case "$conclusion" in
      failure | timed_out | startup_failure)
        echo "ship-gates: warning: main is red — deploy run for $sha (created $created) ended $conclusion: $url" >&2
        if [ "$fixes_red_main" = 1 ]; then
          echo "ship-gates: this push is marked as the fix; shipping" >&2
        else
          echo "ship-gates: fix CI first if you can (ship the fix alone, then resume); if it cannot be fixed now, ship anyway — never stop or wait" >&2
        fi
        ;;
    esac
  fi
else
  echo "ship-gates: warning: could not read main's deploy status (gh unavailable?)" >&2
fi

bash scripts/repo-artifact-guard.sh "$base"

# Captured, not piped into grep: under pipefail an early-exiting grep can
# SIGPIPE git and make a large change look like no change. A bad base fails
# the assignment, and set -e fails the gate closed.
changed=$(git diff --name-only "$base"..HEAD -- lego/backend lego/types deploy/gitops/authz ':(exclude)*.md')
if [ -n "$changed" ]; then
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
dashboard_changed=$(git diff --name-only "$base"..HEAD -- dashboard ':(exclude)*.md')
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
# The custody list comes from the module itself (no copy to drift), and is
# captured first so a broken module fails the gate closed.
custody=$(python3 -B .github/scripts/native-common-qualify.py custody-paths)
all_changed=$(git diff --name-only "$base"..HEAD)
native_changed=$({
  grep -xF -f <(printf '%s\n' "$custody") <<<"$all_changed"
  grep -E '^\.github/(scripts/[^/]*native[^/]*|workflows/native-[^/]*)$' <<<"$all_changed"
} || true)
if [ -n "$native_changed" ]; then
  echo "ship-gates: native custody paths changed — running the native custody suites" >&2
  python3 -B -m unittest discover -s .github/scripts -p 'test_native_oci_*.py'
fi
echo "ship-gates: ok" >&2
