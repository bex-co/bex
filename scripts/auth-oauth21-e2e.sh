#!/usr/bin/env bash
# End-to-end OAuth 2.1 provider check (docs/ADR012-auth.md, w4/m9): drives the full
# agent story against REAL components — real Hydra, real Kratos with the native
# `oauth2_provider` bridge (Kratos accepts the login challenge itself; no custom
# login provider), the dashboard's real headless consent route, and the real
# bex-api resource server:
#
#   discovery (RFC 9728 on bex-api) -> DCR (RFC 7591) -> authorize (PKCE S256)
#   -> Kratos login (browser flow + login_challenge, as the dashboard login page
#   drives it) -> headless consent (dashboard /auth/consent) -> code -> token
#   -> Authorization: Bearer -> bex-api (introspection + audience check)
#
# Self-contained: Hydra + Kratos run as throwaway Docker containers (in-memory
# DSNs); the dashboard dev server and bex-api are started if not already up.
# Requires: docker, curl, python3, go, yarn, kubectl context for bex-api's
# kube client (any cluster; no auth stack needed on it).
#
# Usage: scripts/auth-oauth21-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."

NET=bexoauth-e2e
HYDRA=bexoauth-e2e-hydra
KRATOS=bexoauth-e2e-kratos
DASH_PORT=5199
API_PORT=8090
ISSUER=http://localhost:4444
KRATOS_PUB=http://localhost:4433
KRATOS_ADM=http://localhost:4434
HYDRA_ADM=http://localhost:4445
RESOURCE="http://localhost:$API_PORT/mcp"
TMP="$(mktemp -d)"
DASH_PID=""
API_PID=""

cleanup() {
  docker rm -f "$HYDRA" "$KRATOS" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  [ -n "$DASH_PID" ] && kill "$DASH_PID" 2>/dev/null || true
  [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null || true
  rm -rf "$TMP"
}
trap cleanup EXIT

wait_http() { # url [attempts]
  for _ in $(seq 1 "${2:-45}"); do
    curl -sf -o /dev/null "$1" && return 0
    sleep 1
  done
  echo "error: $1 never became ready" >&2
  return 1
}

echo "-> 1/8 hydra (in-memory, DCR on, login/consent -> dashboard :$DASH_PORT)..."
docker network create "$NET" >/dev/null 2>&1 || true
docker rm -f "$HYDRA" >/dev/null 2>&1 || true
docker run -d --name "$HYDRA" --network "$NET" --network-alias hydra \
  -p 4444:4444 -p 4445:4445 \
  -e DSN=memory \
  -e URLS_SELF_ISSUER=$ISSUER \
  -e URLS_LOGIN=http://localhost:$DASH_PORT/auth/login \
  -e URLS_CONSENT=http://localhost:$DASH_PORT/auth/consent \
  -e OIDC_DYNAMIC_CLIENT_REGISTRATION_ENABLED=true \
  -e SECRETS_SYSTEM=e2e-only-system-secret-32-chars-x \
  oryd/hydra:v2.2.0 serve all --dev >/dev/null
wait_http "$HYDRA_ADM/health/ready"

echo "-> 2/8 kratos (in-memory, native oauth2_provider -> hydra admin)..."
cat >"$TMP/identity.schema.json" <<'JSON'
{
  "$id": "https://schemas.bex.co/e2e.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Person",
  "type": "object",
  "properties": {
    "traits": {
      "type": "object",
      "properties": {
        "email": {
          "type": "string",
          "format": "email",
          "ory.sh/kratos": { "credentials": { "password": { "identifier": true } } }
        }
      },
      "required": ["email"],
      "additionalProperties": false
    }
  }
}
JSON
cat >"$TMP/kratos.yaml" <<YAML
dsn: memory
serve:
  public:
    base_url: $KRATOS_PUB/
    cors:
      enabled: true
      allowed_origins: [http://localhost:$DASH_PORT]
      allow_credentials: true
  admin:
    base_url: http://kratos:4434/
# The whole point of w4/m9: Kratos itself accepts Hydra login challenges.
oauth2_provider:
  url: http://hydra:4445
  override_return_to: true
selfservice:
  default_browser_return_url: http://localhost:$DASH_PORT/
  allowed_return_urls: [http://localhost:$DASH_PORT, $ISSUER]
  methods:
    password: { enabled: true }
  flows:
    login:
      ui_url: http://localhost:$DASH_PORT/auth/login
    registration:
      enabled: true
      ui_url: http://localhost:$DASH_PORT/auth/sign-up
      after:
        password:
          hooks: [{ hook: session }]
    error:
      ui_url: http://localhost:$DASH_PORT/auth/error
identity:
  default_schema_id: default
  schemas: [{ id: default, url: file:///etc/config/kratos/identity.schema.json }]
secrets:
  cookie: [e2e-only-cookie-secret-32-chars-xx]
  cipher: [e2e-only-cipher-secret-32-chars!]
YAML
docker rm -f "$KRATOS" >/dev/null 2>&1 || true
docker run -d --name "$KRATOS" --network "$NET" --network-alias kratos \
  -p 4433:4433 -p 4434:4434 \
  -v "$TMP:/etc/config/kratos" \
  oryd/kratos:v1.3.1 serve -c /etc/config/kratos/kratos.yaml --dev --watch-courier=false >/dev/null
wait_http "$KRATOS_PUB/health/ready"

echo "-> 3/8 bex-api (issuer + resource configured)..."
if ! curl -sf -o /dev/null "http://localhost:$API_PORT/healthz"; then
  (cd lego/backend && BEX_HYDRA_ADMIN_URL=$HYDRA_ADM BEX_KRATOS_URL=$KRATOS_PUB \
    BEX_OAUTH_ISSUER=$ISSUER BEX_OAUTH_RESOURCE=$RESOURCE \
    BEX_API_ADDR=":$API_PORT" nohup go run ./cmd/api >"$TMP/bexapi.log" 2>&1 & echo $! >"$TMP/api.pid")
  API_PID="$(cat "$TMP/api.pid")"
  wait_http "http://localhost:$API_PORT/healthz" 60
fi

echo "-> 4/8 dashboard dev server (consent acceptor) on :$DASH_PORT..."
if ! curl -sf -o /dev/null "http://localhost:$DASH_PORT/"; then
  (cd dashboard && VITE_KRATOS_PUBLIC_URL=$KRATOS_PUB VITE_API_URL=http://localhost:$API_PORT/graphql \
    HYDRA_ADMIN_URL=$HYDRA_ADM nohup yarn dev --port $DASH_PORT >"$TMP/dash.log" 2>&1 & echo $! >"$TMP/dash.pid")
  DASH_PID="$(cat "$TMP/dash.pid")"
  wait_http "http://localhost:$DASH_PORT/auth/consent" 90 || true
fi

echo "-> 5/8 RFC 9728 discovery on bex-api..."
meta="$(curl -sf "http://localhost:$API_PORT/.well-known/oauth-protected-resource")"
echo "$meta" | python3 -c "
import sys, json
m = json.load(sys.stdin)
assert m['resource'] == '$RESOURCE', m
assert '$ISSUER' in m['authorization_servers'], m
print('  ✓ metadata advertises the issuer')
"
www="$(curl -s -o /dev/null -D - "http://localhost:$API_PORT/mcp" | grep -i '^www-authenticate' | tr -d '\r')"
echo "$www" | grep -q 'resource_metadata=' || { echo "error: 401 lacks resource_metadata: $www" >&2; exit 1; }
echo "  ✓ 401 WWW-Authenticate carries resource_metadata"

echo "-> 6/8 DCR: agent self-registers a public PKCE client..."
# Hydra 2.x serves DCR at /oauth2/register (the pinned image above).
reg_ep="$ISSUER/oauth2/register"
code="$(curl -s -o "$TMP/reg.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{
  "client_name": "claude-code-shaped agent (e2e)",
  "redirect_uris": ["http://127.0.0.1:9876/cb"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none",
  "scope": "openid offline_access",
  "audience": ["'"$RESOURCE"'"]
}' "$reg_ep")"
[ "$code" = "201" ] || { echo "error: DCR returned $code: $(cat "$TMP/reg.json")" >&2; exit 1; }
CLIENT_ID="$(python3 -c "import json;print(json.load(open('$TMP/reg.json'))['client_id'])")"
echo "  ✓ registered client_id=$CLIENT_ID at $reg_ep"

echo "-> 7/8 operator blesses the client (skip_consent) + creates the test user..."
# An operator marking an agent trusted is an admin action — exactly how a real
# blessed agent would be onboarded. DCR clients stay untrusted by default.
curl -sf -X PATCH -H 'Content-Type: application/json' \
  -d '[{"op":"replace","path":"/skip_consent","value":true}]' \
  "$HYDRA_ADM/admin/clients/$CLIENT_ID" >/dev/null
curl -sf -X POST -H 'Content-Type: application/json' -d '{
  "schema_id": "default",
  "traits": { "email": "agent-user@bex.co" },
  "credentials": { "password": { "config": { "password": "e2e-password-123!" } } }
}' "$KRATOS_ADM/admin/identities" >/dev/null
echo "  ✓ client trusted, identity created"

echo "-> 8/8 the flow: PKCE authorize -> Kratos login -> consent -> code -> token -> Bearer..."
VERIFIER="dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
CHALLENGE="E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
STATE="e2e-state-$(date +%s)"
JAR="$TMP/cookies.txt"

# (a) authorize -> Hydra 302s to the dashboard login with a login_challenge.
auth_url="$ISSUER/oauth2/auth?client_id=$CLIENT_ID&response_type=code&scope=openid+offline_access&redirect_uri=http://127.0.0.1:9876/cb&state=$STATE&code_challenge=$CHALLENGE&code_challenge_method=S256&audience=$RESOURCE"
loc="$(curl -s -o /dev/null -D - -c "$JAR" "$auth_url" | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2)"
challenge="$(python3 -c "from urllib.parse import urlparse,parse_qs;print(parse_qs(urlparse('$loc').query)['login_challenge'][0])")"
echo "  ✓ authorize -> login_challenge=${challenge:0:8}..."

# (b) the dashboard login page's exact call: create a browser login flow bound
# to the challenge (t002's passthrough), then submit the password form.
flow="$(curl -sf -c "$JAR" -b "$JAR" -H 'Accept: application/json' \
  "$KRATOS_PUB/self-service/login/browser?login_challenge=$challenge")"
action="$(echo "$flow" | python3 -c "import sys,json;print(json.load(sys.stdin)['ui']['action'])")"
csrf="$(echo "$flow" | python3 -c "
import sys, json
f = json.load(sys.stdin)
print(next(n['attributes']['value'] for n in f['ui']['nodes'] if n['attributes'].get('name') == 'csrf_token'))
")"
# Success answers 422 browser_location_change_required -> Hydra's continue URL
# (Kratos accepted the login challenge natively — the w4/m9 crux).
submit="$(curl -s -c "$JAR" -b "$JAR" -H 'Accept: application/json' -H 'Content-Type: application/json' \
  -d "{\"method\":\"password\",\"identifier\":\"agent-user@bex.co\",\"password\":\"e2e-password-123!\",\"csrf_token\":\"$csrf\"}" \
  "$action")"
continue_url="$(echo "$submit" | python3 -c "import sys,json;print(json.load(sys.stdin).get('redirect_browser_to',''))")"
[ -n "$continue_url" ] || { echo "error: login did not yield a continue URL: $submit" >&2; exit 1; }
echo "  ✓ Kratos accepted the login challenge (native oauth2_provider)"

# (c) follow the continue URL: Hydra -> dashboard consent route (headless
# auto-accept) -> Hydra -> redirect to the agent's callback with the code.
# Walk the redirects manually (curl -L would die on the refused loopback
# callback) and stop as soon as the Location targets the agent's callback.
final="$continue_url"
for _ in 1 2 3 4 5 6; do
  nxt="$(curl -s -o /dev/null -D - -c "$JAR" -b "$JAR" "$final" | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2 || true)"
  [ -z "$nxt" ] && break
  final="$nxt"
  [[ "$final" == *"127.0.0.1:9876/cb"* ]] && break
done
[[ "$final" == *"127.0.0.1:9876/cb"* ]] || { echo "error: flow did not reach the agent callback: $final" >&2; exit 1; }
codeval="$(python3 -c "from urllib.parse import urlparse,parse_qs;q=parse_qs(urlparse('$final').query);print(q['code'][0])")"
python3 -c "from urllib.parse import urlparse,parse_qs;q=parse_qs(urlparse('$final').query);assert q['state'][0]=='$STATE','state mismatch'"
echo "  ✓ consent auto-accepted headlessly -> authorization code delivered"

# (d) token exchange (public client + PKCE verifier).
tok="$(curl -sf -X POST -d "grant_type=authorization_code&code=$codeval&redirect_uri=http://127.0.0.1:9876/cb&client_id=$CLIENT_ID&code_verifier=$VERIFIER" "$ISSUER/oauth2/token")"
ACCESS="$(echo "$tok" | python3 -c "import sys,json;print(json.load(sys.stdin)['access_token'])")"
echo "  ✓ token exchange OK (refresh: $(echo "$tok" | python3 -c "import sys,json;print('yes' if json.load(sys.stdin).get('refresh_token') else 'no')"))"

# (e) the Bearer authorizes bex-api (introspection + audience check).
status="$(curl -s -o "$TMP/api.out" -w '%{http_code}' -H "Authorization: Bearer $ACCESS" "http://localhost:$API_PORT/v1/services")"
[ "$status" = "200" ] || { echo "error: bex-api with Bearer returned $status: $(cat "$TMP/api.out")" >&2; exit 1; }
echo "  ✓ Authorization: Bearer passed bex-api introspection + audience check (200)"

echo
echo "✓ w4/m9 end-to-end: DCR -> PKCE -> dashboard-driven Kratos login (native"
echo "  challenge accept) -> headless consent -> token -> Bearer-authorized bex-api"
