#!/usr/bin/env bash
# Shared shell guards for Stripe Billing operator tooling. Callers remain
# responsible for keeping the accepted test key out of argv and output.

bex_stripe_require_test_key() {
  local key="${1:-}"
  case "$key" in
    rk_test_*|sk_test_*) return 0 ;;
    rk_live_*|sk_live_*) echo "error: live Stripe keys are refused" >&2 ;;
    *) echo "error: BEX_STRIPE_SECRET_KEY must be a Stripe test-mode key" >&2 ;;
  esac
  return 2
}
