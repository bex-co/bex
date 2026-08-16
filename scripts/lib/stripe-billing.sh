#!/usr/bin/env bash
# Shared shell guards for Stripe Billing operator tooling. Callers remain
# responsible for keeping the accepted key out of argv and output.

# bex_stripe_require_key_for MODE KEY — MODE is "read-only" or "mutating".
# Test keys always pass. Live keys pass only for read-only callers carrying
# the explicit BEX_STRIPE_ALLOW_LIVE=1 go-live decision (w4/m81 t005), so
# post-cutover drift is observable; mutating tooling refuses live keys
# unconditionally (a misfired mutation bills real money).
bex_stripe_require_key_for() {
  local mode="$1" key="${2:-}"
  case "$key" in
    rk_test_*|sk_test_*) return 0 ;;
    rk_live_*|sk_live_*)
      if [ "$mode" = read-only ]; then
        [ "${BEX_STRIPE_ALLOW_LIVE:-0}" = 1 ] && return 0
        echo "error: live Stripe keys require BEX_STRIPE_ALLOW_LIVE=1 even for read-only reconciliation" >&2
      else
        echo "error: live Stripe keys are refused for mutating billing tooling" >&2
      fi
      ;;
    *) echo "error: BEX_STRIPE_SECRET_KEY must be a Stripe rk_test_*/rk_live_* key" >&2 ;;
  esac
  return 2
}

bex_stripe_require_test_key() {
  bex_stripe_require_key_for mutating "${1:-}"
}
