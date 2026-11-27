#!/usr/bin/env bash
# Check, but never update, that every GraphQL operation and every feature-flag
# gate in Render's public dashboard bundle has a row in
# docs/render-artifacts/dashboard-operations.md (w1/m645), and every dashboard
# URL it builds or routes a row in docs/render-artifacts/dashboard-routes.md's
# URL table (w3/m704).
#
# render-changelog-drift.sh catches what Render announces. A feature Render
# ships only in its dashboard, or keeps behind a per-workspace experiment flag,
# moves no API contract and no changelog entry: the rollback comparison, the
# 2FA reminders and avatars (w1/m646-m648) were found only by reading the
# bundle, and a flagged Postgres overrides editor nearly reversed a user
# decision (w2/066) before anyone saw its flag. The bundle is public, so this
# job reads all of it every week and fails when an operation or flag has no
# row, or a row's operation or flag left the bundle. A flag gate that leaves
# means Render made the feature GA or dropped it: its row names what to reopen.
# Render-shaped links are a contract too (Render's API returns them as
# dashboardUrl, its CLI builds them, people paste them), so a URL Render adds
# or drops fails the same way, instead of turning up as a user's 404.
#
#   render-dashboard-ops.sh [check]   compare the bundle with the ledger
#   render-dashboard-ops.sh list      print the bundle's inventory as JSON
#
# The parser is scripts/lib/render_dashboard_ops.py. It reads the chunks as
# text in memory, never executes or stores them, and fails with a message
# naming what it could not find rather than reporting every row as removed.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
parser="$repo_root/scripts/lib/render_dashboard_ops.py"
ledger="${RENDER_DASHBOARD_OPS_LEDGER:-$repo_root/docs/render-artifacts/dashboard-operations.md}"
routes_ledger="${RENDER_DASHBOARD_ROUTES_LEDGER:-$repo_root/docs/render-artifacts/dashboard-routes.md}"
page_url="${RENDER_DASHBOARD_URL:-https://dashboard.render.com/}"

case "${1:-check}" in
  list)
    exec python3 -I "$parser" list "$page_url"
    ;;
  check)
    for file in "$ledger" "$routes_ledger"; do
      if [[ ! -f "$file" ]]; then
        echo "::error title=Render dashboard ledger missing::$file does not exist" >&2
        exit 1
      fi
    done
    exec python3 -I "$parser" check "$page_url" "$ledger" "$routes_ledger"
    ;;
  *)
    echo "usage: $0 [check|list]" >&2
    exit 2
    ;;
esac
