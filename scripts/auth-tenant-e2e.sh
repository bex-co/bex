#!/usr/bin/env bash
# Tenant-isolation E2E (w1/m9, docs/ADR012-auth.md#authorization) — proves the
# control-plane store + enforced OpenFGA isolate tenants at the API layer, on
# the CAPD mock cluster's Ory + bex-db substrate:
#   1. Two Kratos identities sign up (API registration flow) → each's first
#      authenticated call mints a personal tenant (tenant_members row +
#      user:<id> admin workspace:tea-<id> grant).
#   2. Each mints an API key → bound to its tenant (tenant_members row +
#      user:<client_id> developer workspace:tea-<id> grant).
#   3. Each key lists ONLY its own services; a cross-tenant Get is 403; an
#      unauthenticated call is 401.
#
# bex-api runs on the host (go build ./cmd/api) with BEX_CP_DB_URI +
# BEX_OPENFGA_URL + BEX_KRATOS_URL set, talking to the cluster via
# port-forwards. The App CRD must be installed (operator/: `make install`).
#
# Usage: scripts/auth-tenant-e2e.sh    # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, yq v4, go. Reads .env for OPENFGA_PRESHARED_KEY.
set -euo pipefail
cd "$(dirname "$0")/.."

NS=auth
KRATOS=127.0.0.1:14433
HYDRA_PUBLIC=127.0.0.1:24444
HYDRA_ADMIN=127.0.0.1:24445
OPENFGA=127.0.0.1:24446
DB=127.0.0.1:15432
API=127.0.0.1:18091
CP_API=127.0.0.1:18092
PIDS=()

cleanup() {
  if [ "${#PIDS[@]}" -gt 0 ]; then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

wait_http() {
  for _ in $(seq 1 30); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)"
    [ "$code" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

echo "==> port-forwarding kratos + hydra + openfga + bex-db"
kubectl -n "$NS" port-forward service/kratos-public 14433:80 >/dev/null 2>&1 &
PIDS+=($!)
kubectl -n "$NS" port-forward service/hydra-public 24444:4444 >/dev/null 2>&1 &
PIDS+=($!)
kubectl -n "$NS" port-forward service/hydra-admin 24445:4445 >/dev/null 2>&1 &
PIDS+=($!)
kubectl -n "$NS" port-forward service/openfga 24446:8080 >/dev/null 2>&1 &
PIDS+=($!)
kubectl -n default port-forward service/bex-db-rw 15432:5432 >/dev/null 2>&1 &
PIDS+=($!)
for url in "http://$KRATOS/health/ready" "http://$HYDRA_ADMIN/health/ready" "http://$OPENFGA/healthz"; do
  wait_http "$url"
done

if [ -z "${OPENFGA_PRESHARED_KEY:-}" ] && [ -f .env ]; then
  set -a; # shellcheck disable=SC1091
  source ./.env; set +a
fi
[ -n "${OPENFGA_PRESHARED_KEY:-}" ] || fail "OPENFGA_PRESHARED_KEY not set (source .env)"
OPENFGA_URL="http://$OPENFGA" bash scripts/authz-model.sh | tail -1

# Build the bex-db URI from the CNPG-generated app secret.
DB_USER="$(kubectl -n default get secret bex-db-app -o jsonpath='{.data.username}' | base64 -d)"
DB_PW="$(kubectl -n default get secret bex-db-app -o jsonpath='{.data.password}' | base64 -d)"
DB_URI="postgres://${DB_USER}:${DB_PW}@${DB}/bex?sslmode=disable"
CP_TOKEN="${BEX_CP_TOKEN:-tenant-e2e-$$}"

echo "==> building bex-api"
bin="$(mktemp -d)/bex-api"
(cd lego/operator && go build -o "$bin" ./cmd/api) || fail "build failed"

echo "==> starting bex-api (store + authz on)"
BEX_HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" BEX_KRATOS_URL="http://$KRATOS" \
  BEX_OPENFGA_URL="http://$OPENFGA" BEX_OPENFGA_TOKEN="$OPENFGA_PRESHARED_KEY" \
  BEX_CP_DB_URI="$DB_URI" BEX_CP_TOKEN="$CP_TOKEN" BEX_CP_ADDR=":18092" \
  BEX_API_ADDR=":18091" BEX_API_NAMESPACE=default "$bin" >/dev/null 2>&1 &
PIDS+=($!)
wait_http "http://$API/healthz"

# signup EMAIL PASSWORD -> echoes a Kratos session token (the session hook fires
# on registration, so a session_token comes back immediately).
signup() {
  local flow
  flow="$(curl -s -X POST "http://$KRATOS/self-service/registration/api" | yq '.id' -)"
  [ -n "$flow" ] || fail "registration flow init failed"
  curl -s -X POST "http://$KRATOS/self-service/registration?flow=$flow" \
    -H 'Content-Type: application/json' \
    -d "{\"method\":\"password\",\"traits\":{\"email\":\"$1\"},\"password\":\"$2\"}" \
    | yq '.session_token // ""' -
}

# token CLIENT_ID CLIENT_SECRET -> a Hydra access token (empty on failure).
token_for() {
  { curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
    -d "grant_type=client_credentials&client_id=$1&client_secret=$2" || true; } \
    | yq '.access_token // ""' -
}

# request METHOD PORT PATH TOKEN BODY — sets LAST_CODE + LAST_BODY.
request() {
  local args=(-s -w '\n%{http_code}' -X "$1" "http://$2$3")
  [ -n "$4" ] && args+=(-H "Authorization: Bearer $4")
  [ -n "$5" ] && args+=(-H 'Content-Type: application/json' -d "$5")
  local out; out="$(curl "${args[@]}")"
  LAST_CODE="${out##*$'\n'}"; LAST_BODY="${out%$'\n'*}"
}
assert_code() { [ "$LAST_CODE" = "$1" ] || fail "$2: got $LAST_CODE, want $1 (body: $LAST_BODY)"; echo "    ok: $2 ($LAST_CODE)"; }

# --- tenant A -------------------------------------------------------------
echo "==> onboarding tenant A"
tok_a="$(signup "a-$(date +%s)@e2e.bex" "pw-a-$$")"
[ -n "$tok_a" ] || fail "no session token for A"
# First authenticated call mints tenant A (the auth gate's onboarding hook).
# The session rides X-Session-Token, not Bearer — request() uses Bearer, so warm
# the mint with a one-off curl carrying the session header.
curl -s -o /dev/null -H "X-Session-Token: $tok_a" "http://$API/v1/services" || true

# Internal API: list tenants, find A's (the one whose admin is identity A).
ten_a="$(curl -s -H "Authorization: Bearer $CP_TOKEN" "http://$CP_API/v1/tenants" \
  | yq ".[-1].id" -)"   # the most-recently created tenant
[ -n "$ten_a" ] || fail "no tenant minted for A"
# Create an app for tenant A (projected CR <tenant-id>-web, labeled tea-<id>).
curl -s -X POST -H "Authorization: Bearer $CP_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"tenantId\":\"$ten_a\",\"name\":\"web\",\"image\":\"traefik/whoami\"}" \
  "http://$CP_API/v1/apps" >/dev/null
# Mint an API key as tenant A (session caller → bound to tenant A).
key_a="$(curl -s -X POST -H "X-Session-Token: $tok_a" -H 'Content-Type: application/json' \
  -d '{"name":"a-agent"}' "http://$API/v1/api-keys")"
key_a_id="$(printf '%s' "$key_a" | yq '.id' -)"
key_a_secret="$(printf '%s' "$key_a" | yq '.secret' -)"
[ -n "$key_a_id" ] && [ "$key_a_id" != "null" ] || fail "A minted no key"
tok_key_a="$(token_for "$key_a_id" "$key_a_secret")"

# --- tenant B (same shape) ------------------------------------------------
echo "==> onboarding tenant B"
tok_b="$(signup "b-$(date +%s)@e2e.bex" "pw-b-$$")"
[ -n "$tok_b" ] || fail "no session token for B"
curl -s -o /dev/null -H "X-Session-Token: $tok_b" "http://$API/v1/services" || true
ten_b="$(curl -s -H "Authorization: Bearer $CP_TOKEN" "http://$CP_API/v1/tenants" \
  | yq ".[-1].id" -)"
[ "$ten_b" != "$ten_a" ] || fail "B reused A's tenant (mint not isolated)"
curl -s -X POST -H "Authorization: Bearer $CP_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"tenantId\":\"$ten_b\",\"name\":\"web\",\"image\":\"traefik/whoami\"}" \
  "http://$CP_API/v1/apps" >/dev/null
key_b_id="$(printf '%s' "$(curl -s -X POST -H "X-Session-Token: $tok_b" -H 'Content-Type: application/json' \
  -d '{"name":"b-agent"}' "http://$API/v1/api-keys")" | yq '.id' -)"

# Wait for the projector to create both App CRs.
for _ in $(seq 1 15); do
  count="$(curl -s -H "Authorization: Bearer $CP_TOKEN" "http://$CP_API/v1/apps" | yq 'length' -)"
  [ "$count" -ge 2 ] && break
  sleep 1
done

# --- cross-tenant checks --------------------------------------------------
echo "==> cross-tenant isolation checks"
request GET "$API" /v1/services "$tok_key_a" ""
assert_code 200 "A's key lists services"
own="$(printf '%s' "$LAST_BODY" | yq '[.[].name] | length' -)"
[ "$own" = "1" ] || fail "A's key sees $own services, want 1 (its own)"
echo "    ok: A's key sees only its own service"

# A's key GETs B's service by CR name -> 403 (not 404-leak).
app_b_name="${ten_b}-web"
request GET "$API" "/v1/services/$app_b_name" "$tok_key_a" ""
assert_code 403 "A's key forbidden on B's service"

# Unauthenticated -> 401.
request GET "$API" /v1/services "" ""
assert_code 401 "unauthenticated rejected"

echo "PASS: tenant onboarding + cross-tenant isolation verified end-to-end"
