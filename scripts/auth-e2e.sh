#!/usr/bin/env bash
# E2E for bex-api auth (docs/auth.md, docs/bex-api.md#auth) against the current
# kubeconfig cluster's Ory substrate — no shared static token exists:
#   1. Seed the bootstrap OAuth2 client; exchange it for a bearer token.
#   2. The token authenticates REST + GraphQL; garbage/missing tokens get 401.
#   3. The bootstrap identity mints an API key via POST /v1/api-keys; the new
#      key's token also authenticates; the key lists without its secret.
#   4. Revoking the key kills it: no new tokens can be minted with it.
#      (Already-issued tokens may ride bex-api's ≤30s introspection cache.)
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

echo "==> starting bex-api"
env "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" BEX_API_ADDR=":18090" BEX_API_NAMESPACE=default \
  "$bin" >/dev/null 2>&1 &
PIDS+=($!)
wait_http "http://$API/healthz"

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

echo "PASS: bootstrap client + API-key lifecycle verified end-to-end"
