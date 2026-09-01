#!/usr/bin/env bash
# Install the direct Stripe Billing runtime configuration out of band as
# bex-system/bex-stripe. The Secret is deliberately not committed or rendered
# by GitOps; the bex-api Deployment only carries optional secretKeyRefs.
#
# Values come from the environment or the repo-local, gitignored .env:
#   BEX_STRIPE_SECRET_KEY       dedicated rk_test_*/rk_live_* runtime key
#   BEX_STRIPE_PUBLISHABLE_KEY  matching pk_test_*/pk_live_* Stripe.js key
#   BEX_REQUIRE_PAYMENT_METHOD  1 = ADR046 paid-intent gating; all = every plan
#                               incl. free tier (ADR075 D7); default 0
#   BEX_STRIPE_WEBHOOK_SECRET  whsec_* for /v1/webhooks/stripe
#   BEX_STRIPE_EPOCH           explicit RFC3339 billing-start floor (required)
#   BEX_STRIPE_SEAL_HOURS      rewrite horizon (default 48)
#   BEX_STRIPE_COMP_COUPON_ID  Mode-B coupon (default bex-comp-100)
#   BEX_STRIPE_PORTAL_CONFIGURATION_ID  operator-owned bpc_* portal (optional)
#   BEX_STRIPE_TAX_CODE        confirmed canonical txcd_* (optional pair)
#   BEX_STRIPE_TAX_BEHAVIOR    exclusive|inclusive (optional pair)
#   BEX_STRIPE_DUNNING_ENABLED 1 enables reversible dunning (default 0; test only)
#   BEX_STRIPE_GRACE_PERIOD    grace duration (default 168h)
#   BEX_STRIPE_RECONCILE_INTERVAL provider-poll cadence (default 5m)
#
# Usage:
#   scripts/stripe-billing-secret.sh
#   DRY_RUN=1 scripts/stripe-billing-secret.sh
#
# kubectl respects KUBECONFIG/current-context. Secret material is written only
# to a mode-0600 temporary env file, streamed to kubectl, and deleted on exit;
# it is never printed or placed in process arguments.
set -euo pipefail
cd "$(dirname "$0")/.."

namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="${BEX_STRIPE_SECRET_NAME:-bex-stripe}"

env_file="${BEX_STRIPE_ENV_FILE:-.env}"
if [ -f "$env_file" ]; then
  set -a
  # shellcheck disable=SC1091
  source "$env_file"
  set +a
fi

require() {
  local name="$1" value="${!1:-}"
  [ -n "$value" ] || { echo "error: $name is missing or empty (.env or environment)" >&2; exit 1; }
}

require BEX_STRIPE_SECRET_KEY
require BEX_STRIPE_EPOCH

case "$BEX_STRIPE_SECRET_KEY" in
  rk_test_*) stripe_mode=test ;;
  rk_live_*)
    [ "${BEX_STRIPE_ALLOW_LIVE:-0}" = 1 ] || {
      echo "error: live restricted keys are refused by default; BEX_STRIPE_ALLOW_LIVE=1 is a separate future go-live decision" >&2
      exit 1
    }
    stripe_mode=live
    ;;
  *)
    echo "error: BEX_STRIPE_SECRET_KEY must be a dedicated restricted key (rk_test_* or rk_live_*); refusing an unrestricted or non-Stripe credential" >&2
    exit 1
    ;;
esac

BEX_REQUIRE_PAYMENT_METHOD="${BEX_REQUIRE_PAYMENT_METHOD:-0}"
case "$BEX_REQUIRE_PAYMENT_METHOD" in
  0|1|all) ;;
  *) echo "error: BEX_REQUIRE_PAYMENT_METHOD must be 0, 1, or all" >&2; exit 1 ;;
esac
if [ "$BEX_REQUIRE_PAYMENT_METHOD" != 0 ]; then
  require BEX_STRIPE_PUBLISHABLE_KEY
fi
if [ -n "${BEX_STRIPE_PUBLISHABLE_KEY:-}" ]; then
  case "$stripe_mode:$BEX_STRIPE_PUBLISHABLE_KEY" in
    test:pk_test_*|live:pk_live_*) ;;
    *) echo "error: BEX_STRIPE_PUBLISHABLE_KEY must match the restricted key's test/live mode" >&2; exit 1 ;;
  esac
fi

# The test endpoint created by the operator setup flow is also kept in the
# macOS login keychain. This fallback keeps whsec_* out of shell history and
# .env; it is test-only by construction and is never consulted for rk_live_*.
if [ "$stripe_mode" = test ] && [ -z "${BEX_STRIPE_WEBHOOK_SECRET:-}" ] && command -v security >/dev/null; then
  BEX_STRIPE_WEBHOOK_SECRET="$(security find-generic-password -s bex-stripe-test-webhook -w 2>/dev/null || true)"
fi
require BEX_STRIPE_WEBHOOK_SECRET
case "$BEX_STRIPE_WEBHOOK_SECRET" in
  whsec_*) ;;
  *) echo "error: BEX_STRIPE_WEBHOOK_SECRET must start with whsec_" >&2; exit 1 ;;
esac

BEX_STRIPE_SEAL_HOURS="${BEX_STRIPE_SEAL_HOURS:-48}"
BEX_STRIPE_COMP_COUPON_ID="${BEX_STRIPE_COMP_COUPON_ID:-bex-comp-100}"
BEX_STRIPE_DUNNING_ENABLED="${BEX_STRIPE_DUNNING_ENABLED:-0}"
BEX_STRIPE_GRACE_PERIOD="${BEX_STRIPE_GRACE_PERIOD:-168h}"
BEX_STRIPE_RECONCILE_INTERVAL="${BEX_STRIPE_RECONCILE_INTERVAL:-5m}"
case "$BEX_STRIPE_SEAL_HOURS" in
  *[!0-9]*|0|'') echo "error: BEX_STRIPE_SEAL_HOURS must be an integer >= 1" >&2; exit 1 ;;
esac
case "$BEX_STRIPE_DUNNING_ENABLED" in
  0|1) ;;
  *) echo "error: BEX_STRIPE_DUNNING_ENABLED must be 0 or 1" >&2; exit 1 ;;
esac
# Live dunning is an operator choice since w4/m81 t002 (the w7/m52 test-only
# fence is lifted): BEX_STRIPE_ALLOW_LIVE=1 remains the single deliberate live
# gate, and BEX_STRIPE_DUNNING_ENABLED opts enforcement in or out per mode.
if [ "$BEX_STRIPE_DUNNING_ENABLED" = 1 ]; then
  python3 - "$BEX_STRIPE_GRACE_PERIOD" "$BEX_STRIPE_RECONCILE_INTERVAL" <<'PY'
from decimal import Decimal
import re
import sys

units = {
    "ns": Decimal("0.000000001"),
    "us": Decimal("0.000001"),
    "µs": Decimal("0.000001"),
    "ms": Decimal("0.001"),
    "s": Decimal(1),
    "m": Decimal(60),
    "h": Decimal(3600),
}
token = re.compile(r"([0-9]+(?:\.[0-9]+)?)(ns|us|µs|ms|s|m|h)")


def duration_seconds(name, value):
    position = 0
    total = Decimal(0)
    for match in token.finditer(value):
        if match.start() != position:
            break
        total += Decimal(match.group(1)) * units[match.group(2)]
        position = match.end()
    if position != len(value) or position == 0 or total <= 0:
        raise SystemExit(f"error: {name} must be a positive Go duration")
    return total


duration_seconds("BEX_STRIPE_GRACE_PERIOD", sys.argv[1])
if duration_seconds("BEX_STRIPE_RECONCILE_INTERVAL", sys.argv[2]) < 60:
    raise SystemExit("error: BEX_STRIPE_RECONCILE_INTERVAL must be >= 1m")
PY
fi

# Live mode requires the operator-owned scoped portal configuration (w4/m81
# t003): without it, portal sessions fall back to the account's default
# configuration, which either errors (none activated) or allows subscription
# cancellation/plan changes — breaking the one-subscription-per-workspace
# invariant the scoped bpc_* disables. Provision it with
# `stripe-billing-setup.py --live --dashboard-url https://dashboard.bex.co`.
if [ "$stripe_mode" = live ] && [ -z "${BEX_STRIPE_PORTAL_CONFIGURATION_ID:-}" ]; then
  echo "error: live mode requires BEX_STRIPE_PORTAL_CONFIGURATION_ID (the scoped bpc_* from stripe-billing-setup.py --live --dashboard-url ...); the account-default portal allows subscription cancellation" >&2
  exit 1
fi
if [ -n "${BEX_STRIPE_PORTAL_CONFIGURATION_ID:-}" ]; then
  case "$BEX_STRIPE_PORTAL_CONFIGURATION_ID" in
    bpc_*) ;;
    *) echo "error: BEX_STRIPE_PORTAL_CONFIGURATION_ID must start with bpc_" >&2; exit 1 ;;
  esac
fi

if [ -n "${BEX_STRIPE_TAX_CODE:-}" ] || [ -n "${BEX_STRIPE_TAX_BEHAVIOR:-}" ]; then
  require BEX_STRIPE_TAX_CODE
  require BEX_STRIPE_TAX_BEHAVIOR
  case "$BEX_STRIPE_TAX_CODE" in
    txcd_*) ;;
    *) echo "error: BEX_STRIPE_TAX_CODE must be an operator-confirmed canonical txcd_* value" >&2; exit 1 ;;
  esac
  case "$BEX_STRIPE_TAX_BEHAVIOR" in
    exclusive|inclusive) ;;
    *) echo "error: BEX_STRIPE_TAX_BEHAVIOR must be exclusive or inclusive" >&2; exit 1 ;;
  esac
fi

python3 - "$BEX_STRIPE_EPOCH" <<'PY'
import datetime
import sys

value = sys.argv[1]
try:
    parsed = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
except ValueError as exc:
    raise SystemExit(f"error: BEX_STRIPE_EPOCH must be RFC3339: {exc}")
if parsed.tzinfo is None:
    raise SystemExit("error: BEX_STRIPE_EPOCH must include a timezone (use Z for UTC)")
PY

if [ "${DRY_RUN:-0}" = "1" ]; then
  optional_keys=""
  [ -n "${BEX_STRIPE_PORTAL_CONFIGURATION_ID:-}" ] && optional_keys="$optional_keys BEX_STRIPE_PORTAL_CONFIGURATION_ID"
  [ -n "${BEX_STRIPE_TAX_CODE:-}" ] && optional_keys="$optional_keys BEX_STRIPE_TAX_CODE BEX_STRIPE_TAX_BEHAVIOR"
  [ -n "${BEX_STRIPE_PUBLISHABLE_KEY:-}" ] && optional_keys="$optional_keys BEX_STRIPE_PUBLISHABLE_KEY"
  echo "would apply Secret $namespace/$secret_name (keys: BEX_STRIPE_SECRET_KEY BEX_REQUIRE_PAYMENT_METHOD BEX_STRIPE_WEBHOOK_SECRET BEX_STRIPE_EPOCH BEX_STRIPE_SEAL_HOURS BEX_STRIPE_COMP_COUPON_ID BEX_STRIPE_DUNNING_ENABLED BEX_STRIPE_GRACE_PERIOD BEX_STRIPE_RECONCILE_INTERVAL$optional_keys)"
  echo "mode=$stripe_mode epoch=$BEX_STRIPE_EPOCH seal_hours=$BEX_STRIPE_SEAL_HOURS coupon=$BEX_STRIPE_COMP_COUPON_ID payment_gate=$BEX_REQUIRE_PAYMENT_METHOD dunning=$BEX_STRIPE_DUNNING_ENABLED grace=$BEX_STRIPE_GRACE_PERIOD reconcile=$BEX_STRIPE_RECONCILE_INTERVAL tax_configured=$([ -n "${BEX_STRIPE_TAX_CODE:-}" ] && echo true || echo false)"
  exit 0
fi

command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }

umask 077
secret_dir="$(mktemp -d)"
secret_env="$secret_dir/stripe.env"
cleanup() {
  rm -f "$secret_env"
  rmdir "$secret_dir" 2>/dev/null || true
}
trap cleanup EXIT

{
  printf 'BEX_STRIPE_SECRET_KEY=%s\n' "$BEX_STRIPE_SECRET_KEY"
  [ -z "${BEX_STRIPE_PUBLISHABLE_KEY:-}" ] || printf 'BEX_STRIPE_PUBLISHABLE_KEY=%s\n' "$BEX_STRIPE_PUBLISHABLE_KEY"
  printf 'BEX_REQUIRE_PAYMENT_METHOD=%s\n' "$BEX_REQUIRE_PAYMENT_METHOD"
  printf 'BEX_STRIPE_WEBHOOK_SECRET=%s\n' "$BEX_STRIPE_WEBHOOK_SECRET"
  printf 'BEX_STRIPE_EPOCH=%s\n' "$BEX_STRIPE_EPOCH"
  printf 'BEX_STRIPE_SEAL_HOURS=%s\n' "$BEX_STRIPE_SEAL_HOURS"
  printf 'BEX_STRIPE_COMP_COUPON_ID=%s\n' "$BEX_STRIPE_COMP_COUPON_ID"
  printf 'BEX_STRIPE_DUNNING_ENABLED=%s\n' "$BEX_STRIPE_DUNNING_ENABLED"
  printf 'BEX_STRIPE_GRACE_PERIOD=%s\n' "$BEX_STRIPE_GRACE_PERIOD"
  printf 'BEX_STRIPE_RECONCILE_INTERVAL=%s\n' "$BEX_STRIPE_RECONCILE_INTERVAL"
  [ -z "${BEX_STRIPE_PORTAL_CONFIGURATION_ID:-}" ] || printf 'BEX_STRIPE_PORTAL_CONFIGURATION_ID=%s\n' "$BEX_STRIPE_PORTAL_CONFIGURATION_ID"
  [ -z "${BEX_STRIPE_TAX_CODE:-}" ] || printf 'BEX_STRIPE_TAX_CODE=%s\n' "$BEX_STRIPE_TAX_CODE"
  [ -z "${BEX_STRIPE_TAX_BEHAVIOR:-}" ] || printf 'BEX_STRIPE_TAX_BEHAVIOR=%s\n' "$BEX_STRIPE_TAX_BEHAVIOR"
} >"$secret_env"

kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null
kubectl -n "$namespace" create secret generic "$secret_name" \
  --from-env-file="$secret_env" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

if kubectl -n "$namespace" get deployment/bex-api >/dev/null 2>&1; then
  kubectl -n "$namespace" rollout restart deployment/bex-api >/dev/null
  kubectl -n "$namespace" rollout status deployment/bex-api --timeout=300s >/dev/null
fi

echo "installed $namespace/$secret_name for Stripe $stripe_mode mode; bex-api rollout is ready"
