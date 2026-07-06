#!/usr/bin/env bash
# E2E verify of the auth substrate (Ory Kratos + Hydra backed by CNPG Postgres,
# docs/auth.md) against the current kubeconfig cluster:
#   1. Kratos admin API: create an identity, read it back.
#   2. Hydra: register an OAuth2 client, complete a client_credentials flow,
#      introspect the token (active: true).
#   3. Negative: wrong client secret -> no token; garbage token -> active: false.
#   4. Restart both pods; identity + client survive and tokens still issue
#      (state lives in the kratos-db/hydra-db CNPG clusters, not the pods).
#
# Usage: scripts/auth-verify.sh    # respects $KUBECONFIG; exits 0 on pass
# Talks to the cluster-internal Services via kubectl port-forward — no ingress
# or DNS needed, so it works on the CAPD mock cluster as-is.
# Requires: kubectl, curl, yq v4.
set -euo pipefail

NS=auth
KRATOS_ADMIN=127.0.0.1:14434
HYDRA_PUBLIC=127.0.0.1:14444
HYDRA_ADMIN=127.0.0.1:14445
PF_PIDS=()

stop_forwards() {
  if [ "${#PF_PIDS[@]}" -gt 0 ]; then
    kill "${PF_PIDS[@]}" 2>/dev/null || true
    wait "${PF_PIDS[@]}" 2>/dev/null || true # reap so bash doesn't print "Terminated"
  fi
  PF_PIDS=()
}
trap stop_forwards EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

# wait_http URL — poll until the endpoint answers 2xx (port-forward + pod warmup).
wait_http() {
  for _ in $(seq 1 30); do
    curl -sf -o /dev/null "$1" && return 0
    sleep 2
  done
  fail "$1 did not become ready"
}

port_forward() {
  kubectl -n "$NS" port-forward "service/kratos-admin" 14434:80 >/dev/null 2>&1 &
  PF_PIDS+=($!)
  kubectl -n "$NS" port-forward "service/hydra-public" 14444:4444 >/dev/null 2>&1 &
  PF_PIDS+=($!)
  kubectl -n "$NS" port-forward "service/hydra-admin" 14445:4445 >/dev/null 2>&1 &
  PF_PIDS+=($!)
  wait_http "http://$KRATOS_ADMIN/admin/health/ready"
  wait_http "http://$HYDRA_PUBLIC/.well-known/openid-configuration"
  wait_http "http://$HYDRA_ADMIN/health/ready"
}

echo "==> waiting for kratos + hydra rollouts"
kubectl -n "$NS" rollout status deploy/kratos --timeout=300s
kubectl -n "$NS" rollout status deploy/hydra --timeout=300s

port_forward

# --- 1. Kratos: create an identity via the admin API ------------------------
EMAIL="verify-$(date +%s)@bex.co"
echo "==> creating identity $EMAIL"
identity_id="$(curl -sf -X POST "http://$KRATOS_ADMIN/admin/identities" \
  -H 'Content-Type: application/json' \
  -d "{\"schema_id\":\"default\",\"traits\":{\"email\":\"$EMAIL\"}}" | yq '.id' -)"
[ -n "$identity_id" ] && [ "$identity_id" != "null" ] || fail "identity creation returned no id"
curl -sf "http://$KRATOS_ADMIN/admin/identities/$identity_id" >/dev/null \
  || fail "identity $identity_id not readable after create"
echo "    identity: $identity_id"

# --- 2. Hydra: client_credentials flow ---------------------------------------
CLIENT_ID=bex-auth-verify
CLIENT_SECRET="verify-secret-$(date +%s)"
echo "==> (re)creating OAuth2 client $CLIENT_ID"
curl -s -o /dev/null -X DELETE "http://$HYDRA_ADMIN/admin/clients/$CLIENT_ID" || true
curl -sf -X POST "http://$HYDRA_ADMIN/admin/clients" \
  -H 'Content-Type: application/json' \
  -d "{\"client_id\":\"$CLIENT_ID\",\"client_secret\":\"$CLIENT_SECRET\",\"grant_types\":[\"client_credentials\"],\"token_endpoint_auth_method\":\"client_secret_post\"}" \
  >/dev/null || fail "client creation failed"

token_request() { # secret -> body
  curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
    -d "grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=$1"
}

echo "==> requesting client_credentials token"
access_token="$(token_request "$CLIENT_SECRET" | yq '.access_token' -)"
[ -n "$access_token" ] && [ "$access_token" != "null" ] || fail "no access_token issued"

echo "==> introspecting token"
active="$(curl -sf -X POST "http://$HYDRA_ADMIN/admin/oauth2/introspect" \
  -d "token=$access_token" | yq '.active' -)"
[ "$active" = "true" ] || fail "introspection returned active=$active, want true"

# --- 3. Negative cases --------------------------------------------------------
echo "==> negative: wrong client secret must be rejected"
bad="$(token_request "definitely-not-the-secret")"
[ "$(echo "$bad" | yq '.access_token' -)" = "null" ] || fail "token issued with a wrong client secret"
[ "$(echo "$bad" | yq '.error' -)" != "null" ] || fail "expected an OAuth2 error for a wrong client secret"

echo "==> negative: garbage token must introspect inactive"
garbage_active="$(curl -sf -X POST "http://$HYDRA_ADMIN/admin/oauth2/introspect" \
  -d "token=ory_at_garbage" | yq '.active' -)"
[ "$garbage_active" = "false" ] || fail "garbage token introspected active=$garbage_active, want false"

# --- 4. Restart both pods: state must live in Postgres, not the pods ----------
echo "==> restarting kratos + hydra"
stop_forwards
kubectl -n "$NS" rollout restart deploy/kratos deploy/hydra
kubectl -n "$NS" rollout status deploy/kratos --timeout=300s
kubectl -n "$NS" rollout status deploy/hydra --timeout=300s
port_forward

echo "==> identity survives restart"
curl -sf "http://$KRATOS_ADMIN/admin/identities/$identity_id" >/dev/null \
  || fail "identity $identity_id lost after pod restart"

echo "==> token issuance survives restart (same client)"
access_token2="$(token_request "$CLIENT_SECRET" | yq '.access_token' -)"
[ -n "$access_token2" ] && [ "$access_token2" != "null" ] || fail "no access_token after restart"

echo "PASS: identity + client_credentials verified, state survives pod restarts"
