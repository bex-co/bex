#!/usr/bin/env bash
# Is production's image pin older than <max-seconds>? (w9/m173)
#
# The pre-build supersession check skips a run when newer production manifests
# landed after it (deploy-superseded.sh exit 4): the queued run will build the
# newer tree. Under steady manifest commits and slow tests that queued run is
# skipped in turn, and production keeps its last pin indefinitely. On
# 2026-11-06 no run pinned from 00:51 to past 13:00 UTC although every run
# passed its tests. deploy.yml builds anyway once the pin is older than the
# bound this script checks.
#
# Usage:  scripts/deploy-pin-stale.sh <max-seconds>
# Exit:   0 = stale (older than max-seconds, or no pin commit in reach)
#         1 = fresh
#         2 = error (could not fetch or read history)
set -uo pipefail
MAX="${1:?usage: deploy-pin-stale.sh <max-seconds>}"
case "$MAX" in *[!0-9]* | '') echo "deploy-pin-stale: max-seconds must be an integer" >&2; exit 2 ;; esac
if ! git fetch --no-tags origin main >/dev/null 2>&1; then
  echo "deploy-pin-stale: could not fetch origin/main" >&2
  exit 2
fi
if [ "$(git rev-parse --is-shallow-repository 2>/dev/null)" = true ]; then
  git fetch --no-tags --deepen=500 origin main >/dev/null 2>&1 || true
fi
pinned_at="$(git log origin/main -1 --format=%ct -E \
  --grep='^chore\(deploy\): pin platform images to [0-9a-f]{12}' 2>/dev/null)"
if [ -z "$pinned_at" ]; then
  echo "no pin commit within reach of origin/main; treating the pin as stale"
  exit 0
fi
now="${DEPLOY_PIN_STALE_NOW:-$(date +%s)}"
age=$((now - pinned_at))
if [ "$age" -ge "$MAX" ]; then
  echo "the production pin is ${age}s old (bound ${MAX}s): stale"
  exit 0
fi
echo "the production pin is ${age}s old (bound ${MAX}s): fresh"
exit 1
