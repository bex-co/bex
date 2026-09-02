#!/usr/bin/env bash
# End-to-end OAuth 2.1 provider check (docs/ADR012-auth.md, w4/m9 + m16 + m17 + m18):
# drives the full agent story against REAL components — real Hydra, real Kratos
# with the native `oauth2_provider` bridge (Kratos accepts the login challenge
# itself; no custom login provider), the dashboard's real consent + Connected
# Agents routes, and the real bex-api resource server:
#
#   discovery (RFC 9728 on bex-api) -> DCR (RFC 7591) -> authorize (PKCE S256)
#   -> Kratos login (browser flow + login_challenge, as the dashboard login page
#   drives it) -> consent (dashboard /auth/consent) -> code -> token
#   -> Authorization: Bearer -> bex-api (introspection + audience check)
#   -> revoke via Settings -> Security & Compliance's Connected Agents card
#   (dashboard /api/connected-agents) -> the same token 401s
#
# Both consent paths, since w4/m16:
#   * trusted client (operator-blessed `skip_consent`) -> headless auto-accept;
#   * self-registered client with no blessing -> the signed-in human decides on
#     the consent page: approve (-> working token), deny (-> access_denied), and
#     a second authorization inside the remember window skips the page.
#
# Both login states, since w4/m17: legs 8-12 each drive a fresh browser (a clean
# cookie jar), which keeps them independent — but it also means none of them ever
# exercised the commonest path of all. Leg 13 reuses a jar that is already signed
# in, the state that used to dead-end on the login page (see kratos_login).
#
# Since w4/m18: leg 14 proves the revocation half of the agent-token story — the
# thing that made "revocable, scoped token" only half true after m16 shipped
# remembered consent with no way to undo it (docs/ADR018-render-parity.md
# § bex ahead). Reuses JAR_USER (its live Kratos session cookie is exactly what
# `/api/connected-agents` reads) and leg 13's still-live access token.
#
# The consent page is a server-rendered HTML form posting back to /auth/consent,
# so curl drives it exactly as a browser would — no Playwright, no framework-
# internal RPC serialization to reverse-engineer.
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
DASH_PORT=${DASH_PORT:-5199}
API_PORT=${API_PORT:-8090}
ISSUER=${ISSUER:-http://localhost:4444}
KRATOS_PUB=${KRATOS_PUBLIC_URL:-http://localhost:4433}
KRATOS_ADM=${KRATOS_ADMIN_URL:-http://localhost:4434}
HYDRA_ADM=${HYDRA_ADMIN_URL:-http://localhost:4445}
DASH=http://localhost:$DASH_PORT
RESOURCE="http://localhost:$API_PORT/mcp"
CALLBACK=http://127.0.0.1:9876/cb
USER_EMAIL=agent-user@bex.co
USER_PASSWORD='e2e-password-123!'
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

echo "-> 1/14 hydra (in-memory, DCR on, login/consent -> dashboard :$DASH_PORT)..."
docker network create "$NET" >/dev/null 2>&1 || true
docker rm -f "$HYDRA" >/dev/null 2>&1 || true
docker run -d --name "$HYDRA" --network "$NET" --network-alias hydra \
  -p 4444:4444 -p 4445:4445 \
  -e DSN=memory \
  -e URLS_SELF_ISSUER=$ISSUER \
  -e URLS_LOGIN=$DASH/auth/login \
  -e URLS_CONSENT=$DASH/auth/consent \
  -e OIDC_DYNAMIC_CLIENT_REGISTRATION_ENABLED=true \
  -e SECRETS_SYSTEM=e2e-only-system-secret-32-chars-x \
  oryd/hydra:v2.2.0 serve all --dev >/dev/null
wait_http "$HYDRA_ADM/health/ready"

echo "-> 2/14 kratos (in-memory, native oauth2_provider -> hydra admin)..."
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
      allowed_origins: [$DASH]
      allow_credentials: true
  admin:
    base_url: http://kratos:4434/
# The whole point of w4/m9: Kratos itself accepts Hydra login challenges.
oauth2_provider:
  url: http://hydra:4445
  override_return_to: true
selfservice:
  default_browser_return_url: $DASH/
  allowed_return_urls: [$DASH, $ISSUER]
  methods:
    password: { enabled: true }
  flows:
    login:
      ui_url: $DASH/auth/login
    registration:
      enabled: true
      ui_url: $DASH/auth/sign-up
      after:
        password:
          hooks: [{ hook: session }]
    error:
      ui_url: $DASH/auth/error
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

echo "-> 3/14 bex-api (issuer + resource configured)..."
if ! curl -sf -o /dev/null "http://localhost:$API_PORT/healthz"; then
  (cd lego/backend && BEX_HYDRA_ADMIN_URL=$HYDRA_ADM BEX_KRATOS_URL=$KRATOS_PUB \
    BEX_OAUTH_ISSUER=$ISSUER BEX_OAUTH_RESOURCE=$RESOURCE \
    BEX_API_ADDR=":$API_PORT" nohup go run ./cmd/api >"$TMP/bexapi.log" 2>&1 & echo $! >"$TMP/api.pid")
  API_PID="$(cat "$TMP/api.pid")"
  wait_http "http://localhost:$API_PORT/healthz" 60
fi

echo "-> 4/14 dashboard dev server (consent route) on :$DASH_PORT..."
if ! curl -sf -o /dev/null "$DASH/"; then
  (cd dashboard && VITE_KRATOS_PUBLIC_URL=$KRATOS_PUB VITE_API_URL=http://localhost:$API_PORT/graphql \
    HYDRA_ADMIN_URL=$HYDRA_ADM nohup yarn dev --port $DASH_PORT >"$TMP/dash.log" 2>&1 & echo $! >"$TMP/dash.pid")
  DASH_PID="$(cat "$TMP/dash.pid")"
  wait_http "$DASH/auth/consent" 90 || true
fi

echo "-> 5/14 RFC 9728 discovery on bex-api..."
meta="$(curl -sf "http://localhost:$API_PORT/.well-known/oauth-protected-resource")"
echo "$meta" | python3 -c "
import sys, json
m = json.load(sys.stdin)
assert m['resource'] == '$RESOURCE', m
assert '$ISSUER' in m['authorization_servers'], m
assert m.get('scopes_supported') == ['bex.read', 'bex.write', 'bex.sensitive'], m
print('  ✓ metadata advertises the issuer and granular scopes')
"
www="$(curl -s -o /dev/null -D - "http://localhost:$API_PORT/mcp" | grep -i '^www-authenticate' | tr -d '\r')"
echo "$www" | grep -q 'resource_metadata=' || { echo "error: 401 lacks resource_metadata: $www" >&2; exit 1; }
echo "  ✓ 401 WWW-Authenticate carries resource_metadata"

# --- the browser, in shell -----------------------------------------------------
# Hydra 2.x serves DCR at /oauth2/register (the pinned image above).
REG_EP="$ISSUER/oauth2/register"
VERIFIER="dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
CHALLENGE="E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

dcr() { # client_name [scope] -> client_id on stdout
  local name="$1" scope="${2:-openid offline_access bex.read}" code
  code="$(curl -s -o "$TMP/reg.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{
    "client_name": "'"$name"'",
    "redirect_uris": ["'"$CALLBACK"'"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "token_endpoint_auth_method": "none",
    "scope": "'"$scope"'",
    "audience": ["'"$RESOURCE"'"]
  }' "$REG_EP")"
  [ "$code" = "201" ] || { echo "error: DCR returned $code: $(cat "$TMP/reg.json")" >&2; return 1; }
  python3 -c "import json;print(json.load(open('$TMP/reg.json'))['client_id'])"
}

assert_reserved_dcr_metadata_rejected() {
  local code client_id registration_uri registration_token
  code="$(curl -s -o "$TMP/reserved-create.json" -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' -d '{
      "client_name": "reserved metadata create probe",
      "redirect_uris": ["'"$CALLBACK"'"],
      "grant_types": ["authorization_code"],
      "response_types": ["code"],
      "token_endpoint_auth_method": "none",
      "metadata": {"bex.co/platform-client": true}
    }' "$REG_EP")"
  [ "$code" = "400" ] || { echo "error: DCR accepted reserved metadata on create ($code)" >&2; return 1; }
  python3 -c "import json; r=json.load(open('$TMP/reserved-create.json')); assert r.get('error') == 'invalid_client_metadata', r"

  code="$(curl -s -o "$TMP/reserved-update-client.json" -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' -d '{
      "client_id": "429024F5E608930E2A65EF92591A25CC",
      "client_name": "reserved metadata update probe",
      "redirect_uris": ["'"$CALLBACK"'"],
      "grant_types": ["authorization_code"],
      "response_types": ["code"],
      "token_endpoint_auth_method": "none"
    }' "$REG_EP")"
  [ "$code" = "201" ] || { echo "error: DCR update probe registration returned $code" >&2; return 1; }
  client_id="$(python3 -c "import json; print(json.load(open('$TMP/reserved-update-client.json'))['client_id'])")"
  [ "$client_id" != "429024F5E608930E2A65EF92591A25CC" ] || { echo "error: DCR honored an operator-reserved client_id" >&2; return 1; }
  registration_uri="$(python3 -c "import json; print(json.load(open('$TMP/reserved-update-client.json'))['registration_client_uri'])")"
  registration_token="$(python3 -c "import json; print(json.load(open('$TMP/reserved-update-client.json'))['registration_access_token'])")"
  python3 -c "import json; p='$TMP/reserved-update-client.json'; r=json.load(open(p)); [r.pop(k, None) for k in ('registration_access_token','registration_client_uri','client_id_issued_at','client_secret_expires_at')]; r['metadata']={'bex.co/platform-client':True}; json.dump(r,open('$TMP/reserved-update.json','w'))"
  code="$(curl -s -o "$TMP/reserved-update-response.json" -w '%{http_code}' -X PUT \
    -H "Authorization: Bearer $registration_token" -H 'Content-Type: application/json' \
    -d @"$TMP/reserved-update.json" "$registration_uri")"
  [ "$code" = "400" ] || { echo "error: DCR accepted reserved metadata on update ($code)" >&2; return 1; }
  python3 -c "import json; r=json.load(open('$TMP/reserved-update-response.json')); assert r.get('error') == 'invalid_client_metadata', r"
  curl -sf "$HYDRA_ADM/admin/clients/$client_id" >"$TMP/reserved-update-admin.json"
  python3 -c "import json; r=json.load(open('$TMP/reserved-update-admin.json')); assert not r.get('metadata'), r"
}

qparam() { # url param -> value on stdout ("" when absent)
  python3 -c "
from urllib.parse import urlparse, parse_qs
print(parse_qs(urlparse('$1').query).get('$2', [''])[0])
"
}

# Walk redirects like a browser would, with a cookie jar, stopping at the agent's
# callback (a refused loopback — never actually fetched) or at any page that
# renders (login page, consent page). The final URL lands on stdout; the last
# fetched body in $TMP/page.html.
follow() { # url jar -> final url on stdout
  local url="$1" jar="$2" loc
  for _ in $(seq 1 8); do
    case "$url" in "$CALLBACK"*) break ;; esac
    curl -s -o "$TMP/page.html" -D "$TMP/page.head" -c "$jar" -b "$jar" "$url"
    loc="$(grep -i '^location:' "$TMP/page.head" | tr -d '\r' | cut -d' ' -f2 | tail -1)"
    [ -z "$loc" ] && break
    url="$loc"
  done
  echo "$url"
}

# The dashboard login page's exact call (use-ory-flow.ts): create a browser login
# flow bound to the challenge — return_to and all — then submit the password form.
# Success answers 422 browser_location_change_required -> Hydra's continue URL
# (Kratos accepted the login challenge natively — the w4/m9 crux).
#
# A browser that already holds a Kratos session never sees a form: Kratos accepts
# the challenge outright against that session — and answers this AJAX call with
# HTTP 200 and a body of literally `null` (v1.3.1). No flow, no redirect, nothing
# to render. The page must hand the challenge back to Kratos as a *browser*
# request (no `Accept: application/json`), which 303s straight to Hydra's continue
# URL; that is what `bootstrapViaKratos` in use-ory-flow.ts does, and what leg 13
# exercises. Before w4/m17 the page rendered the null and sat on its skeleton
# forever, so an already-signed-in user could not connect an agent at all.
#
# Records which of the three shapes Kratos answered with in $TMP/login_mode, so a
# leg can assert the path it took rather than just its outcome.
kratos_login() { # login_challenge jar -> continue url on stdout
  local challenge="$1" jar="$2" flow action csrf submit cont url
  url="$KRATOS_PUB/self-service/login/browser?login_challenge=$challenge&return_to=$DASH/"
  flow="$(curl -s -c "$jar" -b "$jar" -H 'Accept: application/json' "$url")"

  if [ "$(echo "$flow" | tr -d '[:space:]')" = "null" ]; then
    echo signed-in-shortcircuit >"$TMP/login_mode"
    cont="$(curl -s -o /dev/null -D - -c "$jar" -b "$jar" "$url" \
      | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2 | tail -1)"
    [ -n "$cont" ] || { echo "error: Kratos did not redirect the browser-shaped login: $url" >&2; return 1; }
    echo "$cont"; return 0
  fi

  cont="$(echo "$flow" | python3 -c "
import sys, json
print((json.loads(sys.stdin.read() or 'null') or {}).get('redirect_browser_to', ''))
")"
  if [ -n "$cont" ]; then echo redirect-browser-to >"$TMP/login_mode"; echo "$cont"; return 0; fi

  echo password-form >"$TMP/login_mode"
  action="$(echo "$flow" | python3 -c "import sys,json;print(json.load(sys.stdin)['ui']['action'])")"
  csrf="$(echo "$flow" | python3 -c "
import sys, json
f = json.load(sys.stdin)
print(next(n['attributes']['value'] for n in f['ui']['nodes'] if n['attributes'].get('name') == 'csrf_token'))
")"
  submit="$(curl -s -c "$jar" -b "$jar" -H 'Accept: application/json' -H 'Content-Type: application/json' \
    -d "{\"method\":\"password\",\"identifier\":\"$USER_EMAIL\",\"password\":\"$USER_PASSWORD\",\"csrf_token\":\"$csrf\"}" \
    "$action")"
  cont="$(echo "$submit" | python3 -c "import sys,json;print(json.load(sys.stdin).get('redirect_browser_to',''))")"
  [ -n "$cont" ] || { echo "error: login did not yield a continue URL: $submit" >&2; return 1; }
  echo "$cont"
}

# One authorization, driven as a browser: authorize -> (login if Hydra asks) ->
# wherever the consent route sends us. Final URL on stdout: either the agent's
# callback (consent was headless) or the consent page (a human must decide).
authorize() { # client_id state jar [scope] -> final url on stdout
  local client="$1" state="$2" jar="$3" scope="${4:-openid+offline_access+bex.read}" final challenge cont
  final="$(follow "$ISSUER/oauth2/auth?client_id=$client&response_type=code&scope=$scope&redirect_uri=$CALLBACK&state=$state&code_challenge=$CHALLENGE&code_challenge_method=S256&audience=$RESOURCE" "$jar")"
  case "$final" in
    "$DASH/auth/login"*)
      challenge="$(qparam "$final" login_challenge)"
      [ -n "$challenge" ] || { echo "error: login page without a login_challenge: $final" >&2; return 1; }
      cont="$(kratos_login "$challenge" "$jar")"
      final="$(follow "$cont" "$jar")"
      ;;
  esac
  echo "$final"
}

# Post the consent page's own form back to it, exactly as the browser does:
# same-origin POST carrying the challenge-bound CSRF token the page embedded.
form_field() { # name -> the value of that hidden input in $TMP/page.html
  python3 -c "
import re, sys
html = open('$TMP/page.html').read()
m = re.search(r'name=\"$1\"[^>]*value=\"([^\"]+)\"', html)
print(m.group(1) if m else '')
"
}

consent_decide() { # decision jar -> final url on stdout (page.html must hold the consent page)
  local decision="$1" jar="$2" challenge csrf loc
  challenge="$(form_field consent_challenge)"
  csrf="$(form_field csrf_token)"
  [ -n "$challenge" ] && [ -n "$csrf" ] || { echo "error: consent page carried no challenge/csrf form fields" >&2; return 1; }
  curl -s -o /dev/null -D "$TMP/decide.head" -c "$jar" -b "$jar" -X POST \
    -H "Origin: $DASH" \
    --data-urlencode "consent_challenge=$challenge" \
    --data-urlencode "csrf_token=$csrf" \
    --data-urlencode "decision=$decision" \
    "$DASH/auth/consent"
  loc="$(grep -i '^location:' "$TMP/decide.head" | tr -d '\r' | cut -d' ' -f2 | tail -1)"
  [ -n "$loc" ] || { echo "error: consent decision did not redirect: $(head -1 "$TMP/decide.head")" >&2; return 1; }
  follow "$loc" "$jar"
}

expect_at() { # expected-url-prefix actual-url what-went-wrong
  case "$2" in "$1"*) return 0 ;; esac
  echo "error: $3: $2" >&2
  exit 1
}

exchange() { # code client_id -> access token on stdout
  curl -s -X POST -d "grant_type=authorization_code&code=$1&redirect_uri=$CALLBACK&client_id=$2&code_verifier=$VERIFIER" \
    "$ISSUER/oauth2/token" | python3 -c "
import sys, json
t = json.loads(sys.stdin.read() or '{}')
if 'access_token' not in t:
    sys.exit('token exchange failed: ' + json.dumps(t))
print(t['access_token'])
"
}

assert_bearer_works() { # access_token
  local status
  status="$(curl -s -o "$TMP/api.out" -w '%{http_code}' -H "Authorization: Bearer $1" "http://localhost:$API_PORT/v1/services")"
  [ "$status" = "200" ] || { echo "error: bex-api with Bearer returned $status: $(cat "$TMP/api.out")" >&2; return 1; }
  # The MCP endpoint is the token's audience — it must no longer answer 401.
  status="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $1" "$RESOURCE")"
  [ "$status" != "401" ] || { echo "error: /mcp still 401s with the granted token" >&2; return 1; }
}

echo "-> 6/14 DCR: three agents self-register public PKCE clients..."
assert_reserved_dcr_metadata_rejected
TRUSTED_CLIENT="$(dcr 'claude-code-shaped agent (e2e)')"
CONSENT_CLIENT="$(dcr 'unblessed agent (e2e)')"
DENIED_CLIENT="$(dcr 'unwelcome agent (e2e)')"
echo "  ✓ reserved metadata rejected on create/update; fixed platform IDs cannot be claimed"
echo "  ✓ registered 3 clients at $REG_EP (none blessed yet)"

echo "-> 7/14 operator blesses ONE client (skip_consent) + creates the test user..."
# An operator marking an agent trusted is an admin action — the headless path.
# The other two clients stay unblessed: since w4/m16 that is no longer a dead
# end, it is the self-serve consent path.
curl -sf -X PATCH -H 'Content-Type: application/json' \
  -d '[{"op":"replace","path":"/skip_consent","value":true}]' \
  "$HYDRA_ADM/admin/clients/$TRUSTED_CLIENT" >/dev/null
curl -sf -X POST -H 'Content-Type: application/json' -d '{
  "schema_id": "default",
  "traits": { "email": "'"$USER_EMAIL"'" },
  "credentials": { "password": { "config": { "password": "'"$USER_PASSWORD"'" } } }
}' "$KRATOS_ADM/admin/identities" >/dev/null
echo "  ✓ $TRUSTED_CLIENT trusted, identity created"

echo "-> 8/14 trusted client: PKCE authorize -> login -> HEADLESS consent -> token -> Bearer..."
JAR_TRUSTED="$TMP/trusted.jar"
final="$(authorize "$TRUSTED_CLIENT" "state-trusted" "$JAR_TRUSTED")"
expect_at "$CALLBACK" "$final" "trusted flow did not reach the agent callback (consent was not headless)"
[ "$(qparam "$final" state)" = "state-trusted" ] || { echo "error: state mismatch: $final" >&2; exit 1; }
ACCESS="$(exchange "$(qparam "$final" code)" "$TRUSTED_CLIENT")"
assert_bearer_works "$ACCESS"
echo "  ✓ consent auto-accepted headlessly -> token -> bex-api 200 (unchanged by m16)"

echo "-> 9/14 unblessed client: authorize -> login -> the CONSENT PAGE renders..."
JAR_USER="$TMP/user.jar"
final="$(authorize "$CONSENT_CLIENT" "state-consent" "$JAR_USER")"
expect_at "$DASH/auth/consent" "$final" "unblessed client did not reach the consent page"
grep -q 'unblessed agent (e2e)' "$TMP/page.html" || { echo "error: consent page does not name the client" >&2; exit 1; }
grep -q 'offline_access' "$TMP/page.html" || { echo "error: consent page does not list the requested scopes" >&2; exit 1; }
grep -q 'bex.read' "$TMP/page.html" || { echo "error: consent page does not list bex.read" >&2; exit 1; }
echo "  ✓ the human sees a consent page naming the client and its scopes (no 403, no operator action)"

echo "-> 10/14 the human approves -> code -> token -> bex-api..."
final="$(consent_decide approve "$JAR_USER")"
expect_at "$CALLBACK" "$final" "approve did not reach the agent callback"
[ "$(qparam "$final" state)" = "state-consent" ] || { echo "error: state mismatch: $final" >&2; exit 1; }
ACCESS="$(exchange "$(qparam "$final" code)" "$CONSENT_CLIENT")"
assert_bearer_works "$ACCESS"
echo "  ✓ user-consented token passes bex-api introspection + audience check, exactly like a blessed client's"

echo "-> 11/14 a second agent asks; the human denies -> access_denied, no code..."
# A fresh browser (jar) per authorization, deliberately: Kratos accepts each Hydra
# login challenge without asking Hydra to remember the login, so every authorize
# re-runs login anyway — and driving each leg from a clean browser keeps the legs
# independent (leg 12 proves the *consent* remember window on its own).
JAR_DENY="$TMP/deny.jar"
final="$(authorize "$DENIED_CLIENT" "state-deny" "$JAR_DENY")"
expect_at "$DASH/auth/consent" "$final" "the denied client did not reach the consent page"
final="$(consent_decide deny "$JAR_DENY")"
expect_at "$CALLBACK" "$final" "deny did not bounce the agent back to its redirect_uri"
[ "$(qparam "$final" error)" = "access_denied" ] || { echo "error: deny did not yield error=access_denied: $final" >&2; exit 1; }
[ -z "$(qparam "$final" code)" ] || { echo "error: a denied flow still delivered an authorization code: $final" >&2; exit 1; }
echo "  ✓ Hydra rejected the request; the agent gets error=access_denied and no code"

echo "-> 12/14 remembered consent: a fresh browser, same user+client -> no consent page..."
JAR_AGAIN="$TMP/again.jar"
final="$(authorize "$CONSENT_CLIENT" "state-again" "$JAR_AGAIN")"
expect_at "$CALLBACK" "$final" "the remembered grant re-prompted for consent"
ACCESS="$(exchange "$(qparam "$final" code)" "$CONSENT_CLIENT")"
assert_bearer_works "$ACCESS"
echo "  ✓ inside the remember window the consent page is skipped -> token straight through"

echo "-> 13/14 SIGNED-IN browser (reused jar): authorize again with no re-login..."
# The most common connect-an-agent path (docs/ADR025-connect-an-agent.md) and the one
# every leg above deliberately avoids: the user is *already signed into the
# dashboard* when the agent asks. JAR_USER has held a live Kratos session since
# leg 9, so this is the real thing, not a simulation of it.
#
# Hydra still re-runs the login redirect (Kratos accepts each challenge without
# asking Hydra to remember the login), so the browser lands back on the login page
# holding a session — the state that dead-ended before w4/m17. Assert the *path*,
# not just the outcome: no password form may be submitted.
: >"$TMP/login_mode" # a stale marker from an earlier leg must not pass this
final="$(authorize "$CONSENT_CLIENT" "state-signedin" "$JAR_USER")"
mode="$(cat "$TMP/login_mode")"
[ "$mode" = "signed-in-shortcircuit" ] || {
  echo "error: a signed-in browser should never re-authenticate; login took the '$mode' path" >&2
  exit 1
}
expect_at "$CALLBACK" "$final" "the signed-in authorization did not reach the agent callback"
[ "$(qparam "$final" state)" = "state-signedin" ] || { echo "error: state mismatch: $final" >&2; exit 1; }
ACCESS="$(exchange "$(qparam "$final" code)" "$CONSENT_CLIENT")"
assert_bearer_works "$ACCESS"
echo "  ✓ already signed in -> no login form, no consent page -> token (this used to dead-end)"

echo "-> extra/w8-m27 read-only token cannot mutate..."
# ACCESS is the consented bex.read token from leg 13. A write verb must fail
# closed with INSUFFICIENT_SCOPE before OpenFGA or resource lookup.
status="$(curl -s -o "$TMP/write.out" -w '%{http_code}' -X POST \
  -H "Authorization: Bearer $ACCESS" \
  "http://localhost:$API_PORT/v1/services/does-not-exist/suspend")"
[ "$status" = "403" ] || { echo "error: read-only suspend returned $status: $(cat "$TMP/write.out")" >&2; exit 1; }
python3 -c "
import json
body = json.load(open('$TMP/write.out'))
assert body.get('code') == 'INSUFFICIENT_SCOPE', body
assert body.get('params', {}).get('required') == 'bex.write', body
print('  ✓ bex.read token refused RelCanOperate with INSUFFICIENT_SCOPE')
"

echo "-> 14/14 Settings -> Security & Compliance 'Connected agents': list -> revoke -> token dead..."
# w4/m18: the revocation surface m16's remembered consent shipped without.
# JAR_USER already carries a live Kratos session (since leg 9) — exactly the
# cookie the dashboard's /api/connected-agents route reads — and $ACCESS is
# the token leg 13 just minted for $CONSENT_CLIENT, still live.
list="$(curl -sf -b "$JAR_USER" "$DASH/api/connected-agents")"
echo "$list" | python3 -c "
import sys, json
agents = json.load(sys.stdin)
assert any(a['clientId'] == '$CONSENT_CLIENT' for a in agents), agents
print('  ✓ the authorized client is listed under Connected Agents')
"

status="$(curl -s -o "$TMP/revoke.out" -w '%{http_code}' -X POST \
  -H "Origin: $DASH" -H 'Content-Type: application/json' \
  -b "$JAR_USER" \
  -d "{\"clientId\":\"$CONSENT_CLIENT\"}" \
  "$DASH/api/connected-agents")"
[ "$status" = "204" ] || { echo "error: revoke returned $status: $(cat "$TMP/revoke.out")" >&2; exit 1; }
echo "  ✓ revoke succeeded"

status="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $ACCESS" "$RESOURCE")"
[ "$status" = "401" ] || { echo "error: /mcp still accepts the revoked client's token (status $status)" >&2; exit 1; }
echo "  ✓ the revoked client's access token is now rejected at /mcp (introspection inactive -> 401)"

list="$(curl -sf -b "$JAR_USER" "$DASH/api/connected-agents")"
echo "$list" | python3 -c "
import sys, json
agents = json.load(sys.stdin)
assert not any(a['clientId'] == '$CONSENT_CLIENT' for a in agents), agents
print('  ✓ the revoked client no longer appears in the list')
"

echo "-> extra/w8-m27 third-party bex.api umbrella is refused at consent..."
LEGACY_CLIENT="$(dcr 'legacy umbrella (e2e)' 'openid offline_access bex.api')"
JAR_LEGACY="$TMP/legacy.jar"
final="$(authorize "$LEGACY_CLIENT" "state-legacy" "$JAR_LEGACY" "openid+offline_access+bex.api")"
expect_at "$DASH/auth/consent" "$final" "legacy bex.api client did not stop at consent"
grep -q 'invalid_scope' "$TMP/page.html" || { echo "error: bex.api umbrella was not refused: $(head -c 400 "$TMP/page.html")" >&2; exit 1; }
grep -q 'bex.read' "$TMP/page.html" || { echo "error: refusal did not name bex.read: $(head -c 400 "$TMP/page.html")" >&2; exit 1; }
[ -z "$(qparam "$final" code)" ] || { echo "error: refused bex.api flow still delivered a code: $final" >&2; exit 1; }
echo "  ✓ third-party bex.api is invalid_scope at consent (no code)"

if [ -n "${HOLD:-}" ]; then
  echo
  echo "HOLD=1: stack left up — dashboard :$DASH_PORT, hydra $ISSUER, kratos $KRATOS_PUB"
  echo "  consent-page client_id=$DENIED_CLIENT  user=$USER_EMAIL / $USER_PASSWORD"
  sleep "${HOLD_SECONDS:-900}"
fi

echo
echo "✓ w4/m9 + m16 + m17 + m18 end-to-end: DCR -> PKCE -> dashboard-driven Kratos login"
echo "  (native challenge accept, from a signed-out AND an already-signed-in browser)"
echo "  -> consent (headless for blessed clients, a real user decision for everyone"
echo "  else: approve / deny / remembered) -> token -> Bearer-authorized bex-api"
echo "  -> Connected Agents list -> revoke -> the same token 401s at /mcp"
