#!/usr/bin/env bash
# E2E for bex-api auth (docs/auth.md, docs/bex-api.md#auth) against the current
# kubeconfig cluster's Ory substrate — no shared static token exists:
#   1. Seed the bootstrap OAuth2 client; exchange it for a bearer token.
#   2. The token authenticates REST + GraphQL; garbage/missing tokens get 401.
#   3. The bootstrap identity mints an API key via POST /v1/api-keys; the new
#      key's token also authenticates; the key lists without its secret.
#   4. Revoking the key kills it: no new tokens can be minted with it.
#      (Already-issued tokens may ride bex-api's ≤30s introspection cache.)
#   5. Authorization (BEX_OPENFGA_URL set): the seeded bootstrap identity (admin
#      of tenant:default) passes and can mint; the freshly minted key has no
#      tuples and gets 403; with the env unset the same key passes again
#      (allow-all regression).
#
# This script runs bex-api store-OFF (no BEX_CP_DB_URI): bootstrap + tuple-less
# key against workspace:default. The store-ON cross-tenant flow (w1/m9 — two
# Kratos identities mint tenants, each key sees only its own services, a
# cross-tenant Get is 403) lives in scripts/auth-tenant-e2e.sh.
#
# bex-api runs on the host (go build ./cmd/api) talking to the cluster's
# apiserver; Hydra is reached via kubectl port-forwards — so this works on the
# CAPD mock cluster with no operator image. The App CRD must be installed
# (operator/: `make install`).
#
# Usage: scripts/auth-e2e.sh    # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, yq v4, go.
set -euo pipefail
cd "$(dirname "$0")/.."

NS=auth
HYDRA_PUBLIC=127.0.0.1:24444
HYDRA_ADMIN=127.0.0.1:24445
API=127.0.0.1:18090
PIDS=()

cleanup() {
  if [ "${#PIDS[@]}" -gt 0 ]; then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

wait_http() { # url — wait until it answers at all (any HTTP status)
  for _ in $(seq 1 30); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)"
    [ "$code" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

echo "==> port-forwarding hydra + building bex-api"
# Forwards first so their warm-up overlaps the compile.
kubectl -n "$NS" port-forward service/hydra-public 24444:4444 >/dev/null 2>&1 &
PIDS+=($!)
kubectl -n "$NS" port-forward service/hydra-admin 24445:4445 >/dev/null 2>&1 &
PIDS+=($!)
bin="$(mktemp -d)/bex-api"
(cd operator && go build -o "$bin" ./cmd/api)
wait_http "http://$HYDRA_ADMIN/health/ready"
wait_http "http://$HYDRA_PUBLIC/.well-known/openid-configuration"

echo "==> seeding bootstrap client"
export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-e2e-bootstrap-$(date +%s)}"
HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" bash scripts/auth-bootstrap-client.sh | tail -1

# token CLIENT_ID CLIENT_SECRET -> access token (empty on failure)
token_for() {
  { curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
    -d "grant_type=client_credentials&client_id=$1&client_secret=$2" || true; } \
    | yq '.access_token // ""' -
}

boot_token="$(token_for bex-bootstrap "$BEX_BOOTSTRAP_CLIENT_SECRET")"
[ -n "$boot_token" ] || fail "no access_token for the bootstrap client"

API_PID=""
start_api() { # extra env...
  env "$@" "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" BEX_API_ADDR=":18090" BEX_API_NAMESPACE=default \
    "$bin" >/dev/null 2>&1 &
  API_PID=$!
  PIDS+=("$API_PID")
  wait_http "http://$API/healthz"
}

stop_api() {
  kill "$API_PID" 2>/dev/null || true
  wait "$API_PID" 2>/dev/null || true
}

echo "==> starting bex-api"
start_api

# request METHOD PATH TOKEN BODY — sets LAST_CODE + LAST_BODY (no subshell, so
# the globals survive; capturing via $() would lose them).
request() {
  local args=(-s -w '\n%{http_code}' -X "$1" "http://$API$2")
  [ -n "$3" ] && args+=(-H "Authorization: Bearer $3")
  [ -n "$4" ] && args+=(-H 'Content-Type: application/json' -d "$4")
  local out
  out="$(curl "${args[@]}")"
  LAST_CODE="${out##*$'\n'}"
  LAST_BODY="${out%$'\n'*}"
}

assert_code() { [ "$LAST_CODE" = "$1" ] || fail "$2: got $LAST_CODE, want $1 (body: $LAST_BODY)"; echo "    ok: $2 ($LAST_CODE)"; }

request GET /v1/services "$boot_token" "";                       assert_code 200 "bootstrap token authenticates REST"
request POST /graphql "$boot_token" '{"query":"{ services { id } }"}'; assert_code 200 "bootstrap token authenticates GraphQL"
request GET /v1/services "definitely-garbage" "";                assert_code 401 "garbage token rejected"
request GET /v1/services "" "";                                  assert_code 401 "missing credentials rejected"

echo "==> minting an API key via the API"
request POST /v1/api-keys "$boot_token" '{"name":"e2e-agent"}'
assert_code 201 "create api key"
key_id="$(printf '%s' "$LAST_BODY" | yq '.id' -)"
key_secret="$(printf '%s' "$LAST_BODY" | yq '.secret' -)"
[ -n "$key_id" ] && [ -n "$key_secret" ] && [ "$key_id" != "null" ] || fail "create returned no id/secret"

key_token="$(token_for "$key_id" "$key_secret")"
[ -n "$key_token" ] || fail "no access_token for the minted key"
request GET /v1/services "$key_token" "";                        assert_code 200 "minted key's token authenticates REST"

request GET /v1/api-keys "$boot_token" ""
assert_code 200 "list api keys"
printf '%s' "$LAST_BODY" | yq -e ".[] | select(.id == \"$key_id\") | (.secret // \"\") == \"\"" - >/dev/null 2>&1 \
  || fail "list must contain $key_id without its secret"
echo "    ok: list shows the key, secret omitted"

echo "==> revoking the key"
request DELETE "/v1/api-keys/$key_id" "$boot_token" "";          assert_code 204 "revoke api key"
[ -z "$(token_for "$key_id" "$key_secret")" ] || fail "revoked key still mints tokens"
echo "    ok: revoked key can no longer mint tokens"

# --- act 5: authorization (docs/auth.md#authorization) -------------------------
echo "==> enabling authorization (OpenFGA)"
OPENFGA=127.0.0.1:24446
kubectl -n "$NS" port-forward service/openfga 24446:8080 >/dev/null 2>&1 &
PIDS+=($!)
wait_http "http://$OPENFGA/healthz"
# Ensure store + model + seed tuples (idempotent) with the same key bex-api uses.
if [ -z "${OPENFGA_PRESHARED_KEY:-}" ] && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
OPENFGA_URL="http://$OPENFGA" bash scripts/authz-model.sh | tail -1

stop_api
start_api "BEX_OPENFGA_URL=http://$OPENFGA" "BEX_OPENFGA_TOKEN=$OPENFGA_PRESHARED_KEY"

request GET /v1/services "$boot_token" "";                       assert_code 200 "authz: seeded bootstrap may read"
request POST /v1/api-keys "$boot_token" '{"name":"e2e-unauthorized"}'
assert_code 201 "authz: seeded bootstrap may mint"
k2_id="$(printf '%s' "$LAST_BODY" | yq '.id' -)"
k2_secret="$(printf '%s' "$LAST_BODY" | yq '.secret' -)"
k2_token="$(token_for "$k2_id" "$k2_secret")"
[ -n "$k2_token" ] || fail "no token for the unauthorized key"
request GET /v1/services "$k2_token" "";                         assert_code 403 "authz: tuple-less key is forbidden"
request POST /v1/api-keys "$k2_token" '{"name":"nope"}';         assert_code 403 "authz: tuple-less key may not mint"

echo "==> disabling authorization (regression: allow-all)"
stop_api
start_api
request GET /v1/services "$k2_token" "";                         assert_code 200 "no-authz: same key passes again"
request DELETE "/v1/api-keys/$k2_id" "$boot_token" "";           assert_code 204 "cleanup: revoke the test key"

echo "PASS: bootstrap client + API-key lifecycle + authorization verified end-to-end"
