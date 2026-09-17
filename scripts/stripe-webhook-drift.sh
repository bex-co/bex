#!/usr/bin/env bash
# stripe-webhook-drift.sh — assert bex's live Stripe webhook endpoint still
# agrees with the code that consumes it.
#
# Why this exists: on 2026-09-16 the endpoint was configured correctly and
# nothing enforced it. Two drifts are silent and unrecoverable if they happen:
#
#   1. enabled_events losing a type bex handles — those events simply never
#      arrive. Nothing errors, nothing retries, nothing logs.
#   2. api_version drifting from the SDK's — historically that made the receiver
#      reject every delivery with a 400, and Stripe never retries a 4xx. The
#      receiver no longer rejects on version (it logs and dispatches), so this is
#      now a warning rather than a failure, but a drift still wants a human.
#
# The bex endpoint is identified by URL. The Stripe account is shared with other
# products (beancount, blockeden, cuckoo, tianpan); this script MUST NOT read
# their configuration into its output or modify them. It performs no writes.
#
# Note on scope: filtering by event type cannot isolate products on a shared
# account — Stripe endpoints are account-wide and the noisy types (invoice.*,
# customer.subscription.*) are exactly the ones bex needs. Ownership is enforced
# by bex_workspace metadata in the handlers, not here. This script is about
# drift, not isolation.
#
# Required:
#   BEX_STRIPE_SECRET_KEY   a Stripe key with webhook-endpoint read access
# Optional:
#   BEX_WEBHOOK_URL         default https://api.bex.co/v1/webhooks/stripe
#
# Exit 0 when the endpoint matches; 1 on drift; 2 when the script was not given
# what it needs (missing tool or key, endpoint absent) — an operator to-do.

set -euo pipefail

WEBHOOK_URL="${BEX_WEBHOOK_URL:-https://api.bex.co/v1/webhooks/stripe}"
BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../lego/backend" && pwd)"

fail() { echo "FAIL: $*" >&2; exit 1; }
unusable() { echo "UNUSABLE: $*" >&2; exit 2; }

for cmd in curl jq go; do
  command -v "$cmd" >/dev/null || unusable "missing required command: $cmd"
done
: "${BEX_STRIPE_SECRET_KEY:?set BEX_STRIPE_SECRET_KEY (read-only use)}"

# The expected sets come from the Go code itself, so this can never drift from
# what the receiver actually handles — the failure mode a duplicated literal here
# would reintroduce.
expected_events="$(cd "$BACKEND_DIR" && go run ./internal/billing/cmd/webhookcontract events)" \
  || unusable "could not read the handled event set from the backend module"
expected_version="$(cd "$BACKEND_DIR" && go run ./internal/billing/cmd/webhookcontract version)" \
  || unusable "could not read the pinned Stripe API version from the backend module"

endpoints="$(curl -sS -u "$BEX_STRIPE_SECRET_KEY:" "https://api.stripe.com/v1/webhook_endpoints?limit=100")" \
  || unusable "could not list Stripe webhook endpoints"
if [ "$(echo "$endpoints" | jq -r 'has("error")')" = "true" ]; then
  unusable "Stripe rejected the endpoint list: $(echo "$endpoints" | jq -r '.error.message')"
fi

# Select ONLY bex's endpoint. Every other product's configuration stays unread.
endpoint="$(echo "$endpoints" | jq --arg url "$WEBHOOK_URL" '.data[] | select(.url==$url)')"
[ -n "$endpoint" ] || unusable "no Stripe webhook endpoint is configured for $WEBHOOK_URL"

status="$(echo "$endpoint" | jq -r '.status')"
version="$(echo "$endpoint" | jq -r '.api_version // "account default"')"
actual_events="$(echo "$endpoint" | jq -r '.enabled_events[]' | sort)"

problems=0

if [ "$status" != "enabled" ]; then
  echo "DRIFT: endpoint status is '$status', want 'enabled' — Stripe disables endpoints that fail persistently" >&2
  problems=1
fi

missing="$(comm -23 <(echo "$expected_events") <(echo "$actual_events") || true)"
extra="$(comm -13 <(echo "$expected_events") <(echo "$actual_events") || true)"

if [ -n "$missing" ]; then
  echo "DRIFT: bex handles these events but Stripe does not send them (they will never arrive, silently):" >&2
  echo "$missing" | sed 's/^/  - /' >&2
  problems=1
fi
if [ -n "$extra" ]; then
  echo "DRIFT: Stripe sends these events but bex handles none of them (harmless, but the contract is stale):" >&2
  echo "$extra" | sed 's/^/  + /' >&2
  problems=1
fi

if [ "$version" != "$expected_version" ]; then
  # Not fatal: the receiver logs and dispatches a mismatched version, and every
  # handler re-reads its objects from Stripe by id rather than trusting the
  # delivered payload. Still worth a human's attention.
  echo "WARNING: endpoint api_version is '$version', SDK expects '$expected_version'" >&2
fi

if [ "$problems" -ne 0 ]; then
  fail "the Stripe webhook endpoint for $WEBHOOK_URL has drifted from the code"
fi

echo "OK: $WEBHOOK_URL is enabled and subscribes to exactly the $(echo "$expected_events" | wc -l | tr -d ' ') events bex handles (api_version $version)"
