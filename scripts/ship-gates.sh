#!/usr/bin/env bash
# ship-gates.sh — the checks /ship runs after committing and before pushing
# (w5/m112). Exits non-zero when the push must not happen:
#
#   1. main is red: the newest completed (not cancelled or skipped)
#      `deploy (bex via Argo)` run on main failed. Stacking more commits on a
#      red main is how three changes landed on top of a broken backend suite on
#      2026-10-05. Pass --fixes-red-main when this push is the fix (say so in
#      the commit). One read, no waiting: /ship still does not watch CI.
#   2. The push changes lego/backend, lego/types or the seeded authz model and
#      the backend suite fails against its real Postgres + OpenFGA + OpenBao
#      (scripts/backend-test-deps.sh), which local `go test` otherwise skips.
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

if [ "$fixes_red_main" = 0 ]; then
  if runs=$(gh run list --workflow deploy.yml --branch main --limit 20 \
    --json conclusion,status,headSha,url 2>/dev/null); then
    latest=$(jq -r '[.[] | select(.status == "completed" and .conclusion != "cancelled" and .conclusion != "skipped")][0]
      | if . == null then "" else "\(.conclusion) \(.headSha[0:9]) \(.url)" end' <<<"$runs")
    if [ -z "$latest" ]; then
      echo "ship-gates: warning: no completed deploy run on main to judge; not blocking" >&2
    else
      read -r conclusion sha url <<<"$latest"
      if [ "$conclusion" = failure ]; then
        echo "ship-gates: main is red — deploy run for $sha failed: $url" >&2
        echo "ship-gates: fix main first, or run with --fixes-red-main if this push is the fix" >&2
        exit 1
      fi
    fi
  else
    echo "ship-gates: warning: could not read main's deploy status (gh unavailable?); not blocking" >&2
  fi
fi

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
  (cd lego/backend && env -u KUBECONFIG GOWORK=off BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...)
fi
echo "ship-gates: ok" >&2
