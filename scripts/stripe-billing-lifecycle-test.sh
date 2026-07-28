#!/usr/bin/env bash
# Assert one production-hosted Stripe test-mode lifecycle checkpoint over REST,
# GraphQL, MCP, and (optionally) the tenant App CRs. The operator advances the
# Stripe test clock separately; this verifier never receives a Stripe key.
#
# Required:
#   BEX_VERIFY_SESSION_TOKEN  disposable workspace-admin Kratos session token
#   BEX_VERIFY_WORKSPACE_ID   disposable tea-* workspace
#   BEX_VERIFY_EXPECT_STATUS  healthy|grace|enforcing|enforced|recovering|excluded|comped
# Optional:
#   BEX_VERIFY_API_URL        default https://api.bex.co
#   BEX_VERIFY_KUBE_NAMESPACE namespace containing disposable App CRs
#   BEX_VERIFY_RECOVERED_APP  App expected to be billing-owned then resumed
#   BEX_VERIFY_PRESERVED_APP  pre-suspended App that must remain unowned/suspended
set -euo pipefail

api_url="${BEX_VERIFY_API_URL:-https://api.bex.co}"
workspace_id="${BEX_VERIFY_WORKSPACE_ID:-}"
session_token="${BEX_VERIFY_SESSION_TOKEN:-}"
expected="${BEX_VERIFY_EXPECT_STATUS:-}"

[ -n "$session_token" ] || { echo "error: BEX_VERIFY_SESSION_TOKEN is required" >&2; exit 1; }
case "$workspace_id" in tea-*) ;; *) echo "error: BEX_VERIFY_WORKSPACE_ID must be a disposable tea-* id" >&2; exit 1 ;; esac
case "$expected" in healthy|grace|enforcing|enforced|recovering|excluded|comped) ;; *) echo "error: invalid BEX_VERIFY_EXPECT_STATUS" >&2; exit 1 ;; esac

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
normalize() {
  jq -Sc '{status:(.status // ""),reason:(.reason // ""),graceDeadline:(.graceDeadline // ""),enforcementOwned:(.enforcementOwned // false),recoveryPending:(.recoveryPending // false),allowedActions:(.allowedActions // []),updatedAt:(.updatedAt // "")}'
}

rest_body="$(api_curl "$api_url/v1/workspaces/$workspace_id/billing")" || fail "REST readiness failed"
[ "$(printf '%s' "$rest_body" | jq -r .mode)" = test ] || fail "provider mode is not test"
rest="$(printf '%s' "$rest_body" | jq -c .lifecycle | normalize)" || fail "REST lifecycle malformed"

gql_body="$(jq -cn --arg workspace "$workspace_id" '{query:"query LifecycleVerify($workspace: String!) { workspaceBillingReadiness(workspaceId: $workspace) { mode lifecycle { status reason graceDeadline enforcementOwned recoveryPending allowedActions updatedAt } } }",variables:{workspace:$workspace}}')"
gql_response="$(api_curl -H 'Content-Type: application/json' -X POST -d "$gql_body" "$api_url/graphql")" || fail "GraphQL readiness failed"
[ "$(printf '%s' "$gql_response" | jq '.errors // [] | length')" = 0 ] || fail "GraphQL returned errors"
[ "$(printf '%s' "$gql_response" | jq -r .data.workspaceBillingReadiness.mode)" = test ] || fail "GraphQL provider mode is not test"
gql="$(printf '%s' "$gql_response" | jq -c .data.workspaceBillingReadiness.lifecycle | normalize)"

mcp_body="$(jq -cn --arg workspace "$workspace_id" '{jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"get_billing_readiness",arguments:{workspaceId:$workspace}}}')"
mcp_response="$(api_curl -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-06-18' -X POST -d "$mcp_body" "$api_url/mcp")" || fail "MCP readiness failed"
if printf '%s' "$mcp_response" | grep -q '^data: '; then
  mcp_response="$(printf '%s\n' "$mcp_response" | sed -n 's/^data: //p' | tail -1)"
fi
[ "$(printf '%s' "$mcp_response" | jq -r '.result.structuredContent.mode')" = test ] || fail "MCP provider mode is not test"
mcp="$(printf '%s' "$mcp_response" | jq -c .result.structuredContent.lifecycle | normalize)"

[ "$rest" = "$gql" ] || fail "REST and GraphQL lifecycle differ"
[ "$rest" = "$mcp" ] || fail "REST and MCP lifecycle differ"
[ "$(printf '%s' "$rest" | jq -r .status)" = "$expected" ] || fail "expected $expected, got $(printf '%s' "$rest" | jq -r .status)"
if [ "$expected" = grace ]; then
  [ -n "$(printf '%s' "$rest" | jq -r .graceDeadline)" ] || fail "grace has no deadline"
fi
case "$expected" in
  enforcing|enforced|recovering)
    [ "$(printf '%s' "$rest" | jq -r .enforcementOwned)" = true ] || fail "$expected is not marked enforcement-owned"
    ;;
esac
if [ "$expected" = recovering ]; then
  [ "$(printf '%s' "$rest" | jq -r .recoveryPending)" = true ] || fail "recovering is not recovery-pending"
fi

kube_namespace="${BEX_VERIFY_KUBE_NAMESPACE:-}"
if [ -n "$kube_namespace" ]; then
  recovered_app="${BEX_VERIFY_RECOVERED_APP:-}"
  preserved_app="${BEX_VERIFY_PRESERVED_APP:-}"
  inspect_app() {
    kubectl -n "$kube_namespace" get apps.app.bex.co "$1" -o json
  }
  if [ -n "$recovered_app" ]; then
    app="$(inspect_app "$recovered_app")"
    suspended="$(printf '%s' "$app" | jq -r '.spec.suspended // false')"
    marker="$(printf '%s' "$app" | jq -r '.metadata.annotations["billing.bex.co/enforcement"] // ""')"
    case "$expected" in
      enforcing|enforced) [ "$suspended" = true ] && [ -n "$marker" ] || fail "$recovered_app lacks exact enforcement intent" ;;
      healthy) [ "$suspended" = false ] && [ -z "$marker" ] || fail "$recovered_app was not precisely recovered" ;;
    esac
  fi
  if [ -n "$preserved_app" ]; then
    app="$(inspect_app "$preserved_app")"
    [ "$(printf '%s' "$app" | jq -r '.spec.suspended // false')" = true ] || fail "$preserved_app lost pre-suspended intent"
    [ -z "$(printf '%s' "$app" | jq -r '.metadata.annotations["billing.bex.co/enforcement"] // ""')" ] || fail "$preserved_app gained a billing marker"
  fi
fi

echo "PASS: prod Stripe lifecycle workspace=$workspace_id mode=test status=$expected surfaces=REST,GraphQL,MCP"
