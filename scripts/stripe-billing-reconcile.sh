#!/usr/bin/env bash
# Reconcile or explicitly repair the production-hosted Stripe test-mode outbox.
# `report` and `issues` are read-only. `repair` is dry-run unless APPLY=1.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib/stripe-billing.sh
source scripts/lib/stripe-billing.sh

cp_url="${BEX_CP_URL:-http://127.0.0.1:8091}"
cp_token="${BEX_CP_TOKEN:-}"
[ -n "$cp_token" ] || { echo "error: BEX_CP_TOKEN is required" >&2; exit 2; }

tmp_dir="$(mktemp -d)"
auth_config="$tmp_dir/curl-auth"
cleanup() {
  unlink "$auth_config" 2>/dev/null || true
  find "$tmp_dir" -type f -delete
  rmdir "$tmp_dir" 2>/dev/null || true
}
trap cleanup EXIT
umask 077
printf 'header = "Authorization: Bearer %s"\n' "$cp_token" >"$auth_config"
unset cp_token

cp_curl() { curl -fsS --config "$auth_config" "$@"; }
usage() {
  echo "usage: $0 report WORKSPACE START_RFC3339 END_RFC3339 | issues | repair TRANSACTION_ID acknowledge|retry|mark_repaired ACTOR REASON" >&2
  exit 2
}

command_name="${1:-}"
case "$command_name" in
  report)
    [ "$#" -eq 4 ] || usage
    workspace="$2" start="$3" end="$4"
    case "$workspace" in tea-*) ;; *) echo "error: workspace must be a disposable tea-* id" >&2; exit 2 ;; esac
    bex_stripe_require_test_key "${BEX_STRIPE_SECRET_KEY:-}"
    command -v stripe >/dev/null || { echo "error: stripe CLI is required" >&2; exit 2; }
    local_json="$tmp_dir/local.json"
    cp_curl --get --data-urlencode "start=$start" --data-urlencode "end=$end" \
      "$cp_url/v1/tenants/$workspace/billing-export" >"$local_json"
    python3 scripts/stripe_billing_reconcile.py --local "$local_json" --start "$start" --end "$end"
    ;;
  issues)
    [ "$#" -eq 1 ] || usage
    cp_curl "$cp_url/v1/billing/export-issues" | jq -S .
    ;;
  repair)
    [ "$#" -eq 5 ] || usage
    transaction_id="$2" action="$3" actor="$4" reason="$5"
    case "$transaction_id" in *[!A-Za-z0-9_-]*|'') echo "error: invalid transaction id" >&2; exit 2 ;; esac
    case "$action" in acknowledge|retry|mark_repaired) ;; *) usage ;; esac
    selected="$(cp_curl "$cp_url/v1/billing/export-issues" | jq -c --arg id "$transaction_id" '.issues[] | select(.transactionId==$id)')"
    [ -n "$selected" ] || { echo "error: open issue not found" >&2; exit 1; }
    if [ "${APPLY:-0}" != 1 ]; then
      printf '%s\n' "$selected" | jq -S .
      echo "DRY RUN: would action=$action transaction=$transaction_id actor=$actor; set APPLY=1 after provider reconciliation"
      exit 0
    fi
    body="$(jq -cn --arg action "$action" --arg actor "$actor" --arg reason "$reason" '{action:$action,actor:$actor,reason:$reason}')"
    cp_curl -H 'Content-Type: application/json' -X POST -d "$body" \
      "$cp_url/v1/billing/export-issues/$transaction_id/resolve" | jq -S .
    ;;
  *) usage ;;
esac
