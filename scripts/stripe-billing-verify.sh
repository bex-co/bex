#!/usr/bin/env bash
# Verify the production-hosted customer-billing contract without handling a
# Stripe key or payment details. The caller supplies a disposable workspace
# admin's Kratos session token out of band; the script keeps it out of process
# arguments and compares the same readiness value over REST, GraphQL, and MCP.
# It also asks each HTTP adapter for Stripe-hosted test-mode sessions.
#
# Required:
#   BEX_VERIFY_SESSION_TOKEN  disposable workspace-admin Kratos session token
#   BEX_VERIFY_WORKSPACE_ID   that disposable workspace's tea-* id
# Optional:
#   BEX_VERIFY_API_URL        default https://api.bex.co
#   BEX_VERIFY_DASHBOARD_URL  default https://dashboard.bex.co
#   BEX_VERIFY_HOSTED_URL_FILE  create this mode-0600 JSON file with the
#                               short-lived Checkout/Portal URLs (must not exist)
#   BEX_VERIFY_REQUIRE_PAYMENT_READY=1  fail unless Checkout completion has
#                                       bound a default payment method
set -euo pipefail

api_url="${BEX_VERIFY_API_URL:-https://api.bex.co}"
dashboard_url="${BEX_VERIFY_DASHBOARD_URL:-https://dashboard.bex.co}"
workspace_id="${BEX_VERIFY_WORKSPACE_ID:-}"
session_token="${BEX_VERIFY_SESSION_TOKEN:-}"
hosted_url_file="${BEX_VERIFY_HOSTED_URL_FILE:-}"
require_payment_ready="${BEX_VERIFY_REQUIRE_PAYMENT_READY:-0}"

[ -n "$workspace_id" ] || { echo "error: BEX_VERIFY_WORKSPACE_ID is required" >&2; exit 1; }
[ -n "$session_token" ] || { echo "error: BEX_VERIFY_SESSION_TOKEN is required" >&2; exit 1; }
case "$workspace_id" in tea-*) ;; *) echo "error: verification requires a disposable tea-* workspace" >&2; exit 1 ;; esac
case "$require_payment_ready" in 0|1) ;; *) echo "error: BEX_VERIFY_REQUIRE_PAYMENT_READY must be 0 or 1" >&2; exit 1 ;; esac

verify_dir="$(mktemp -d)"
auth_config="$verify_dir/curl-auth"
cleanup_verify_tmp() {
  unlink "$auth_config"
  rmdir "$verify_dir"
}
trap cleanup_verify_tmp EXIT
umask 077
printf 'header = "X-Session-Token: %s"\n' "$session_token" >"$auth_config"
unset session_token

fail() { echo "FAIL: $*" >&2; exit 1; }
api_curl() { curl -fsS --config "$auth_config" "$@"; }
post_json() { api_curl -H 'Content-Type: application/json' -X POST -d "$2" "$1"; }

normalize_readiness() {
  jq -Sc '{workspaceId,mode,customerReady,subscriptionReady,paymentMethodReady,tax:{configured:.tax.configured,enabled:.tax.enabled,reason:(.tax.reason // ""),productTaxCode:(.tax.productTaxCode // ""),taxBehavior:(.tax.taxBehavior // ""),registrationCount:.tax.registrationCount}}'
}

rest="$(api_curl "$api_url/v1/workspaces/$workspace_id/billing")" \
  || fail "REST readiness request failed"
rest_norm="$(printf '%s' "$rest" | normalize_readiness)" \
  || fail "REST readiness response is malformed"
[ "$(printf '%s' "$rest_norm" | jq -r .mode)" = test ] \
  || fail "production verification must remain in Stripe test mode"
[ "$(printf '%s' "$rest_norm" | jq -r .customerReady)" = true ] \
  || fail "workspace has no unique Stripe test Customer"
[ "$(printf '%s' "$rest_norm" | jq -r .subscriptionReady)" = true ] \
  || fail "workspace has no complete Stripe test billing Subscription"
if [ "$require_payment_ready" = 1 ]; then
  [ "$(printf '%s' "$rest_norm" | jq -r .paymentMethodReady)" = true ] \
    || fail "Checkout completion has not bound the default payment method"
fi

gql_query="$(jq -cn --arg workspace "$workspace_id" '{query:"query BillingVerify($workspace: String!) { workspaceBillingReadiness(workspaceId: $workspace) { workspaceId mode customerReady subscriptionReady paymentMethodReady tax { configured enabled reason productTaxCode taxBehavior registrationCount } } }",variables:{workspace:$workspace}}')"
gql="$(post_json "$api_url/graphql" "$gql_query")" \
  || fail "GraphQL readiness request failed"
[ "$(printf '%s' "$gql" | jq '.errors // [] | length')" = 0 ] \
  || fail "GraphQL readiness returned an error"
gql_norm="$(printf '%s' "$gql" | jq -c '.data.workspaceBillingReadiness' | normalize_readiness)" \
  || fail "GraphQL readiness response is malformed"

mcp_request="$(jq -cn --arg workspace "$workspace_id" '{jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"get_billing_readiness",arguments:{workspaceId:$workspace}}}')"
mcp="$(api_curl -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-06-18' -X POST -d "$mcp_request" "$api_url/mcp")" \
  || fail "MCP readiness request failed"
if printf '%s' "$mcp" | grep -q '^data: '; then
  mcp="$(printf '%s\n' "$mcp" | sed -n 's/^data: //p' | tail -1)"
fi
[ "$(printf '%s' "$mcp" | jq -r '.error.message // empty')" = "" ] \
  || fail "MCP readiness returned an error"
mcp_norm="$(printf '%s' "$mcp" | jq -c '.result.structuredContent' | normalize_readiness)" \
  || fail "MCP readiness response is malformed"

[ "$rest_norm" = "$gql_norm" ] || fail "REST and GraphQL readiness differ"
[ "$rest_norm" = "$mcp_norm" ] || fail "REST and MCP readiness differ"

checkout_body="$(jq -cn --arg success "$dashboard_url/usage?billing=success" --arg cancel "$dashboard_url/usage?billing=cancelled" '{successUrl:$success,cancelUrl:$cancel}')"
checkout="$(post_json "$api_url/v1/workspaces/$workspace_id/billing/checkout-session" "$checkout_body")" \
  || fail "REST Checkout Session creation failed"
checkout_url="$(printf '%s' "$checkout" | jq -er .url)" \
  || fail "REST Checkout response has no URL"
[ -n "$(printf '%s' "$checkout" | jq -r '.expiresAt // empty')" ] \
  || fail "REST Checkout response has no expiry"

portal_query="$(jq -cn --arg workspace "$workspace_id" --arg returnUrl "$dashboard_url/usage" '{query:"mutation PortalVerify($workspace: String!, $returnUrl: String!) { createBillingPortalSession(workspaceId: $workspace, returnUrl: $returnUrl) { url } }",variables:{workspace:$workspace,returnUrl:$returnUrl}}')"
portal="$(post_json "$api_url/graphql" "$portal_query")" \
  || fail "GraphQL Portal Session creation failed"
[ "$(printf '%s' "$portal" | jq '.errors // [] | length')" = 0 ] \
  || fail "GraphQL Portal Session returned an error"
portal_url="$(printf '%s' "$portal" | jq -er '.data.createBillingPortalSession.url')" \
  || fail "GraphQL Portal response has no URL"

mcp_portal_request="$(jq -cn --arg workspace "$workspace_id" --arg returnUrl "$dashboard_url/usage" '{jsonrpc:"2.0",id:2,method:"tools/call",params:{name:"create_billing_portal_session",arguments:{workspaceId:$workspace,returnUrl:$returnUrl}}}')"
mcp_portal="$(api_curl -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-06-18' -X POST -d "$mcp_portal_request" "$api_url/mcp")" \
  || fail "MCP Portal Session creation failed"
if printf '%s' "$mcp_portal" | grep -q '^data: '; then
  mcp_portal="$(printf '%s\n' "$mcp_portal" | sed -n 's/^data: //p' | tail -1)"
fi
mcp_portal_url="$(printf '%s' "$mcp_portal" | jq -er '.result.structuredContent.url')" \
  || fail "MCP Portal response has no URL"

validate_hosted_url() {
  local raw="$1" expected_host="$2"
  case "$raw" in
    *$'\r'*|*$'\n'*) fail "hosted session URL contains a control character" ;;
    "https://$expected_host/"*) ;;
    *) fail "hosted session URL is not the expected Stripe HTTPS host ($expected_host)" ;;
  esac
}
validate_hosted_url "$checkout_url" checkout.stripe.com
validate_hosted_url "$portal_url" billing.stripe.com
validate_hosted_url "$mcp_portal_url" billing.stripe.com

if [ -n "$hosted_url_file" ]; then
  if ! (
    set -o noclobber
    exec 3>"$hosted_url_file"
    printf '%s\n%s\n%s\n' "$checkout_url" "$portal_url" "$mcp_portal_url" \
      | jq -Rn '[inputs] | {checkoutUrl:.[0], graphqlPortalUrl:.[1], mcpPortalUrl:.[2]}' >&3
  ); then
    fail "cannot create BEX_VERIFY_HOSTED_URL_FILE (it must not already exist)"
  fi
fi

payment_ready="$(printf '%s' "$rest_norm" | jq -r .paymentMethodReady)"
tax_reason="$(printf '%s' "$rest_norm" | jq -r '.tax.reason // "ready"')"
echo "PASS: prod billing parity + hosted sessions workspace=$workspace_id mode=test payment_ready=$payment_ready tax=$tax_reason"
