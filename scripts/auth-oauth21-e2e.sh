#!/usr/bin/env bash
# End-to-end OAuth 2.1 provider check (docs/ADR012-auth.md, w4/m9 + m16 + m17 + m18):
# drives the full agent story against REAL components — real Hydra, real Kratos
# with the native `oauth2_provider` bridge (Kratos accepts the login challenge
# itself; the login page's server accepts it only for a browser already signed
# in, w7/079), the dashboard's real login, consent, device and Connected Agents
# routes, and the real bex-api resource server:
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
# Since w7/079: a browser signed in on the dashboard's own login page, which
# Hydra has never seen, connects an agent and approves an RFC 8628 device login
# (the official Render CLI's `bex login`) with no password form: the login
# page's server answers the challenge against the live session. A client that
# asks for a fresh login (`prompt=login`) or a recent one (`max_age`) still gets
# Kratos's form.
#
# Since w7/m426: a leg after the test user exists proves an API key is a bearer
# token against a real Hydra (docs/ADR012-auth.md §8e). A signed-in person mints
# a key through bex-api, and that one string authenticates REST, GraphQL and
# MCP; a wrong key gets the answer random text gets; the key's id and secret
# still exchange for a token; and the revoked key is refused within the
# documented five seconds. It signs in through Kratos' native flow, so it needs
# neither the dashboard nor the browser legs.
#
# The consent page is a server-rendered HTML form posting back to /auth/consent,
# so curl drives it exactly as a browser would — no Playwright, no framework-
# internal RPC serialization to reverse-engineer.
#
# Self-contained: Hydra + Kratos run as throwaway Docker containers (in-memory
# DSNs); the dashboard dev server and bex-api are started if not already up.
# bex-api runs on a throwaway control-plane Postgres, as in production: leg 14's
# revoke records its marker there, and the dashboard refuses a revoke it cannot
# propagate (w7/065), so a DB-free bex-api can never pass it.
# Requires: docker, curl, python3, go, yarn, and a kubectl context for bex-api's
# kube client carrying the bex CRDs (no auth stack needed on it). The store
# provisions a namespace for the test user's workspace there, so prefer a
# throwaway cluster: `kind create cluster --name bexoauth-e2e` and
# `kubectl apply -f lego/operator/config/crd/bases`.
# Optional: BEX_BIN=<path to a built bex CLI> adds the CLI to the API-key leg.
#
# Usage: scripts/auth-oauth21-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."

NET=bexoauth-e2e
HYDRA=bexoauth-e2e-hydra
KRATOS=bexoauth-e2e-kratos
DB=bexoauth-e2e-db
# The digest the backend CI suite runs (.github/workflows/backend-test.yml).
DB_IMAGE=postgres:17@sha256:e2fddda1fea39c634dccdae58dba5448eb3efb85aaa4054cf45cbed28f35d296
DB_PORT=${DB_PORT:-25499}
CP_PORT=${CP_PORT:-8191} # bex-api's internal listener; 8091 is the dev stacks'
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
DASH_PGID=""
API_PGID=""

# Start a server as its own process group (job control on for the launch), so
# cleanup stops the whole tree, yarn's node, vite and esbuild children included,
# and no stale bex-api is left for the next run to test (w7/065). Sets LAUNCHED
# to the group's id, its leader's PID.
launch() { # log dir command...
  local log="$1" dir="$2"
  shift 2
  set -m
  (cd "$dir" && exec "$@") </dev/null >"$log" 2>&1 &
  LAUNCHED=$!
  set +m
}

# TERM the group, TERM it again (bex-api drains for ~15s on the first, still
# holding its port, and force-exits on the second), then KILL what is left.
stop_group() { # pgid
  local signal
  for signal in TERM TERM KILL; do
    kill -"$signal" -- "-$1" 2>/dev/null || return 0
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 -- "-$1" 2>/dev/null || return 0
      sleep 0.5
    done
  done
}

cleanup() {
  docker rm -f "$HYDRA" "$KRATOS" "$DB" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  if [ -n "$DASH_PGID" ]; then stop_group "$DASH_PGID"; fi
  if [ -n "$API_PGID" ]; then stop_group "$API_PGID"; fi
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
# Production's Hydra, v26.2.0 (ADR012 §8d). Not v2.2.0, the earlier pin
# (w7/065): its in-memory store answers a read or write that overlaps another
# with 400 "Unable to serialize access due to a concurrent update", which
# reached bex-api's key list as a 500 in about one run in three.
docker network create "$NET" >/dev/null 2>&1 || true
docker rm -f "$HYDRA" >/dev/null 2>&1 || true
docker run -d --name "$HYDRA" --network "$NET" --network-alias hydra \
  -p 4444:4444 -p 4445:4445 \
  -e DSN=memory \
  -e URLS_SELF_ISSUER=$ISSUER \
  -e URLS_LOGIN=$DASH/auth/login \
  -e URLS_CONSENT=$DASH/auth/consent \
  -e URLS_DEVICE_VERIFICATION=$DASH/auth/device \
  -e URLS_DEVICE_SUCCESS=$DASH/auth/device/success \
  -e OIDC_DYNAMIC_CLIENT_REGISTRATION_ENABLED=true \
  -e SECRETS_SYSTEM=e2e-only-system-secret-32-chars-x \
  oryd/hydra:v26.2.0 serve all --dev >/dev/null
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
# Production's Kratos, v26.2.0 (deploy/gitops/base/values/kratos.values.yaml,
# chart 0.62.1). It answers leg 13's signed-in shortcut differently from
# v1.3.1, the earlier pin (see kratos_login, w7/073).
docker rm -f "$KRATOS" >/dev/null 2>&1 || true
docker run -d --name "$KRATOS" --network "$NET" --network-alias kratos \
  -p 4433:4433 -p 4434:4434 \
  -v "$TMP:/etc/config/kratos" \
  oryd/kratos:v26.2.0 serve -c /etc/config/kratos/kratos.yaml --dev --watch-courier=false >/dev/null
wait_http "$KRATOS_PUB/health/ready"

echo "-> 3/14 bex-api (issuer + resource configured)..."
if curl -sf -o /dev/null "http://localhost:$API_PORT/healthz"; then
  # Deliberate: a bex-api already on the port (one with the control-plane store,
  # say) is the one under test. Say so, since nothing here configured it.
  echo "  ! reusing the bex-api already listening on :$API_PORT (not started by this run)"
else
  docker rm -f "$DB" >/dev/null 2>&1 || true
  docker run -d --name "$DB" -e POSTGRES_PASSWORD=pw -p "$DB_PORT:5432" "$DB_IMAGE" >/dev/null
  # Built meanwhile, so the readiness wait below times the start, not a compile.
  (cd lego/backend && go build -o "$TMP/bex-api" ./cmd/api)
  for _ in $(seq 1 30); do docker exec "$DB" pg_isready -U postgres -h 127.0.0.1 >/dev/null 2>&1 && break; sleep 1; done
  # BEX_REQUIRE_VERIFIED_EMAIL=0: this Kratos has no courier, so the test user's
  # address is never verified, and the gate would refuse every human caller.
  # BEX_ALLOW_INSECURE_AUTHZ=1: no OpenFGA here, and the store refuses to start
  # without it otherwise; the one test user is its workspace's only member.
  launch "$TMP/bexapi.log" lego/backend env BEX_HYDRA_ADMIN_URL="$HYDRA_ADM" BEX_KRATOS_URL="$KRATOS_PUB" \
    BEX_OAUTH_ISSUER="$ISSUER" BEX_OAUTH_RESOURCE="$RESOURCE" BEX_REQUIRE_VERIFIED_EMAIL=0 \
    BEX_CP_DB_URI="postgres://postgres:pw@127.0.0.1:$DB_PORT/postgres?sslmode=disable" \
    BEX_CP_ADDR=":$CP_PORT" BEX_CP_TOKEN="e2e-cp-$(python3 -c 'import secrets; print(secrets.token_hex(16))')" \
    BEX_ALLOW_INSECURE_AUTHZ=1 BEX_API_ADDR=":$API_PORT" "$TMP/bex-api"
  API_PGID="$LAUNCHED"
  wait_http "http://localhost:$API_PORT/healthz" 60
fi

echo "-> 4/14 dashboard dev server (consent route) on :$DASH_PORT..."
if curl -sf -o /dev/null "$DASH/"; then
  echo "  ! reusing the dashboard already serving $DASH (not started by this run)"
else
  # BEX_REQUIRE_VERIFIED_EMAIL=0 for the same unverified user: the consent
  # route runs the same gate before any accept, trusted clients included, and
  # answers it with error=access_denied on the agent's callback (w7/065).
  launch "$TMP/dash.log" dashboard env VITE_KRATOS_PUBLIC_URL="$KRATOS_PUB" \
    VITE_API_URL="http://localhost:$API_PORT/graphql" HYDRA_ADMIN_URL="$HYDRA_ADM" \
    HYDRA_PUBLIC_URL="$ISSUER" BEX_REQUIRE_VERIFIED_EMAIL=0 yarn dev --port "$DASH_PORT"
  DASH_PGID="$LAUNCHED"
  wait_http "$DASH/auth/consent" 90 || true
fi

echo "-> 5/14 RFC 9728 discovery on bex-api..."
meta="$(curl -sf "http://localhost:$API_PORT/.well-known/oauth-protected-resource")"
echo "$meta" | python3 -c "
import sys, json
m = json.load(sys.stdin)
assert m['resource'] == '$RESOURCE', m
assert '$ISSUER' in m['authorization_servers'], m
assert {'bex.read', 'bex.write', 'bex.sensitive'} <= set(m.get('scopes_supported', [])), m
print('  ✓ metadata advertises the issuer and granular scopes')
"
www="$(curl -s -o /dev/null -D - "http://localhost:$API_PORT/mcp" | grep -i '^www-authenticate' | tr -d '\r')"
echo "$www" | grep -q 'resource_metadata=' || { echo "error: 401 lacks resource_metadata: $www" >&2; exit 1; }
echo "  ✓ 401 WWW-Authenticate carries resource_metadata"

# --- the browser, in shell -----------------------------------------------------
# Hydra serves DCR at /oauth2/register (the pinned image above).
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
# fetched body in $TMP/page.html. Every URL fetched is appended to
# $TMP/follow.chain, so a leg can assert a hop the browser passed through.
follow() { # url jar -> final url on stdout
  local url="$1" jar="$2" loc
  for _ in $(seq 1 8); do
    case "$url" in "$CALLBACK"*) break ;; esac
    echo "$url" >>"$TMP/follow.chain"
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
# A browser that already holds a Kratos session, and that Hydra remembers (Hydra's
# login request says `skip`), never sees a form: Kratos accepts the challenge
# outright against that session before it answers. v26.2.0 then answers this
# AJAX call with HTTP 400 `session_already_available` (v1.3.1 answered 200 and a
# body of literally `null`). No flow, no redirect, nothing to render. The page
# must hand the challenge back to Kratos as a *browser* request (no `Accept:
# application/json`), which 303s straight to Hydra's continue URL; that is what
# use-ory-flow.ts does with both answers (`bootstrapViaKratos`), and what leg 13
# exercises. Before w4/m17 the page rendered the null and sat on its skeleton
# forever, so an already-signed-in user could not connect an agent at all.
#
# Hydra remembers a browser from its first login challenge on: Kratos accepts
# every challenge with `remember` for the session's lifespan. A browser with a
# live session Hydra does not remember never gets here: the login page's server
# accepts its challenge first (w7/079). Kratos's re-authentication form
# (`refresh`) is left for a client that asked for a fresh login (`prompt=login`)
# or a recent one (`max_age`).
#
# Records which shape Kratos answered with in $TMP/login_mode, so a leg can assert
# the path it took rather than just its outcome.
kratos_login() { # login_challenge jar -> continue url on stdout
  local challenge="$1" jar="$2" flow mode submit cont url
  url="$KRATOS_PUB/self-service/login/browser?login_challenge=$challenge&return_to=$DASH/"
  flow="$(curl -s -c "$jar" -b "$jar" -H 'Accept: application/json' "$url")"
  mode="$(echo "$flow" | python3 -c '
import sys, json
f = json.load(sys.stdin)
if (f.get("error") or {}).get("id") == "session_already_available":
    print("signed-in-shortcircuit")
else:
    print("reauth-form" if f.get("refresh") else "password-form")
')"
  echo "$mode" >"$TMP/login_mode"

  if [ "$mode" = signed-in-shortcircuit ]; then
    cont="$(curl -s -o /dev/null -D - -c "$jar" -b "$jar" "$url" \
      | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2 | tail -1)"
    [ -n "$cont" ] || { echo "error: Kratos did not redirect the browser-shaped login: $url" >&2; return 1; }
    echo "$cont"; return 0
  fi

  submit="$(password_submit "$flow" "$jar")"
  cont="$(echo "$submit" | python3 -c "import sys,json;print(json.load(sys.stdin).get('redirect_browser_to',''))")"
  [ -n "$cont" ] || { echo "error: login did not yield a continue URL: $submit" >&2; return 1; }
  echo "$cont"
}

# Submit a Kratos login flow's password form with the test user's credentials,
# as Ory Elements does. The response body lands on stdout.
password_submit() { # flow_json jar
  local action csrf
  action="$(echo "$1" | python3 -c "import sys,json;print(json.load(sys.stdin)['ui']['action'])")"
  csrf="$(echo "$1" | python3 -c "
import sys, json
f = json.load(sys.stdin)
print(next(n['attributes']['value'] for n in f['ui']['nodes'] if n['attributes'].get('name') == 'csrf_token'))
")"
  curl -s -c "$2" -b "$2" -H 'Accept: application/json' -H 'Content-Type: application/json' \
    -d "{\"method\":\"password\",\"identifier\":\"$USER_EMAIL\",\"password\":\"$USER_PASSWORD\",\"csrf_token\":\"$csrf\"}" \
    "$action"
}

# Sign a fresh browser in the way a person signs in at the dashboard: the login
# page's own flow, with no login challenge, so Hydra never sees it (w7/079).
dashboard_sign_in() { # jar
  local flow submit
  flow="$(curl -s -c "$1" -b "$1" -H 'Accept: application/json' "$KRATOS_PUB/self-service/login/browser?return_to=$DASH/")"
  submit="$(password_submit "$flow" "$1")"
  echo "$submit" | python3 -c "import sys,json; assert json.load(sys.stdin)['session']['active']" ||
    { echo "error: the dashboard sign-in did not yield a session: $(echo "$submit" | head -c 300)" >&2; return 1; }
  grep -q ory_hydra_session "$1" && { echo "error: a dashboard sign-in left a Hydra login cookie" >&2; return 1; }
  return 0
}

# One authorization, driven as a browser: authorize -> (login if Hydra asks) ->
# wherever the consent route sends us. Final URL on stdout: either the agent's
# callback (consent was headless) or the consent page (a human must decide).
# `extra` is appended to the authorize query as given (`&prompt=login`).
authorize() { # client_id state jar [scope] [extra] -> final url on stdout
  local client="$1" state="$2" jar="$3" scope="${4:-openid+offline_access+bex.read}" extra="${5:-}" final challenge cont
  final="$(follow "$ISSUER/oauth2/auth?client_id=$client&response_type=code&scope=$scope&redirect_uri=$CALLBACK&state=$state&code_challenge=$CHALLENGE&code_challenge_method=S256&audience=$RESOURCE$extra" "$jar")"
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

# Hydra answers a refused consent on the same callback (error=access_denied), so
# a callback without a code fails here with its reason, not at Hydra as
# invalid_grant for an empty code (w7/065).
exchange() { # callback_url client_id -> access token on stdout
  local code
  code="$(qparam "$1" code)"
  [ -n "$code" ] || {
    echo "error: the callback carries no code: error=$(qparam "$1" error) ($(qparam "$1" error_description))" >&2
    return 1
  }
  curl -s -X POST -d "grant_type=authorization_code&code=$code&redirect_uri=$CALLBACK&client_id=$2&code_verifier=$VERIFIER" \
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

echo "-> extra/w7-m426 an API key is a bearer token: mint -> REST, GraphQL, MCP -> wrong key -> exchange -> revoke..."
API="http://localhost:$API_PORT"
# with_bearer <name> <bearer> <curl args...> -> the status code; the body lands in $TMP/key.<name>
with_bearer() {
  local name="$1" bearer="$2"
  shift 2
  curl -s -o "$TMP/key.$name" -w '%{http_code}' -H "Authorization: Bearer $bearer" "$@"
}
# surfaces <bearer> -> "REST=<status> GraphQL=<status> MCP=<status>", each reading the key list.
# A key owns no key, so its own list is empty (w9/m539): it reads the workspace's
# list (allMembers), which its creator, the workspace's Admin, may.
surfaces() {
  local rest gql mcp
  rest="$(with_bearer rest "$1" "$API/v1/api-keys?allMembers=true")"
  gql="$(with_bearer gql "$1" -H 'Content-Type: application/json' -d '{"query":"{ apiKeys(allMembers: true) { id } }"}' "$API/graphql")"
  mcp="$(with_bearer mcp "$1" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_api_keys","arguments":{"allMembers":true}}}' "$API/mcp")"
  echo "REST=$rest GraphQL=$gql MCP=$mcp"
}
# A person mints the key (a machine credential cannot). Their session comes
# from Kratos' native login flow, so this leg needs no browser and no leg below.
login_flow="$(curl -sf "$KRATOS_PUB/self-service/login/api" | python3 -c 'import sys,json; print(json.load(sys.stdin)["ui"]["action"])')"
SESSION_TOKEN="$(curl -sf -X POST -H 'Content-Type: application/json' \
  -d '{"method":"password","identifier":"'"$USER_EMAIL"'","password":"'"$USER_PASSWORD"'"}' "$login_flow" |
  python3 -c 'import sys,json; print(json.load(sys.stdin)["session_token"])')" ||
  { echo "error: the native Kratos login for the key's owner failed" >&2; exit 1; }
minted="$(curl -sf -H "X-Session-Token: $SESSION_TOKEN" -H 'Content-Type: application/json' -d '{"name":"e2e bearer key"}' "$API/v1/api-keys")" ||
  { echo "error: minting an API key with the signed-in session failed" >&2; exit 1; }
read -r KEY_ID KEY_SECRET API_KEY < <(echo "$minted" |
  python3 -c 'import sys,json; m = json.load(sys.stdin); print(m["id"], m["secret"], m.get("key", ""))')
[ -n "$API_KEY" ] && [ "$API_KEY" = "bex_${KEY_ID}_${KEY_SECRET}" ] ||
  { echo "error: create did not answer the key as bex_<id>_<secret>" >&2; exit 1; }
echo "  ✓ create answers one key string made of the id and the secret"

got="$(surfaces "$API_KEY")"
[ "$got" = "REST=200 GraphQL=200 MCP=200" ] || { echo "error: the key as a bearer: $got" >&2; exit 1; }
for surface in rest gql mcp; do
  grep -q "$KEY_ID" "$TMP/key.$surface" || { echo "error: $surface did not list the key it authenticated with: $(head -c 300 "$TMP/key.$surface")" >&2; exit 1; }
done
if grep -q '"errors"' "$TMP/key.gql" || grep -q '"isError":true' "$TMP/key.mcp"; then
  echo "error: GraphQL or MCP answered an error for the key: $(head -c 300 "$TMP/key.gql") $(head -c 300 "$TMP/key.mcp")" >&2
  exit 1
fi
assert_bearer_works "$API_KEY"
echo "  ✓ the key alone authenticates REST, GraphQL and MCP as its own client (real Hydra exchange + introspection)"

# The bex CLI with BEX_BIN set: one variable holding the key, no login and no
# exchange. A stack with the control-plane store lists the key's workspace's
# services; without it the CLI still authenticates and stops at the store.
if [ -n "${BEX_BIN:-}" ]; then
  workspace="$(with_bearer owners "$API_KEY" "$API/v1/owners" >/dev/null && python3 -c 'import sys,json
owners = json.load(open(sys.argv[1]))
print(owners[0]["owner"]["id"] if isinstance(owners, list) and owners else "")' "$TMP/key.owners" 2>/dev/null || true)"
  cli_out="$(env -u RENDER_API_KEY -u BEX_ACCESS_TOKEN -u RENDER_HOST -u RENDER_WORKSPACE HOME="$TMP/clihome" \
    BEX_HOST="$API/v1/" BEX_WORKSPACE="${workspace:-none}" BEX_CLI_DISABLE_ANALYTICS=1 BEX_API_KEY="$API_KEY" \
    "$BEX_BIN" services -o json 2>&1)" && cli_status=0 || cli_status=$?
  if [ -n "$workspace" ]; then
    [ "$cli_status" = "0" ] && echo "$cli_out" | python3 -c 'import sys,json; assert isinstance(json.load(sys.stdin), list)' ||
      { echo "error: BEX_API_KEY=<key> bex services -o json failed ($cli_status): $cli_out" >&2; exit 1; }
    echo "  ✓ BEX_API_KEY=<key> bex services -o json lists workspace $workspace with no login and no exchange"
  else
    echo "$cli_out" | grep -q 'store not configured' ||
      { echo "error: the bex CLI did not authenticate with BEX_API_KEY ($cli_status): $cli_out" >&2; exit 1; }
    echo "  ✓ the bex CLI authenticates with BEX_API_KEY alone (this stack has no control-plane store to list from)"
  fi
fi

# A wrong secret, an unknown id, a malformed key and random text: one answer.
refusal=""
for bad in "${API_KEY}x" "bex_00000000-0000-4000-8000-000000000000_${KEY_SECRET}" "bex_not-a-key" "not-a-key-at-all"; do
  status="$(with_bearer bad "$bad" "$API/v1/api-keys")"
  [ "$status" = "401" ] || { echo "error: a bad key answered $status, want 401" >&2; exit 1; }
  [ -z "$refusal" ] && refusal="$(cat "$TMP/key.bad")"
  [ "$(cat "$TMP/key.bad")" = "$refusal" ] || { echo "error: bad keys are refused with different bodies" >&2; exit 1; }
done
echo "  ✓ a wrong secret, an unknown id, a malformed key and random text get the same 401"

# The exchange a caller does for themselves still works with the same key.
EXCHANGED="$(curl -s -X POST -d grant_type=client_credentials --data-urlencode "client_id=$KEY_ID" \
  --data-urlencode "client_secret=$KEY_SECRET" "$ISSUER/oauth2/token" |
  python3 -c 'import sys,json; print(json.load(sys.stdin).get("access_token",""))')"
[ -n "$EXCHANGED" ] && [ "$(with_bearer exchanged "$EXCHANGED" "$API/v1/api-keys")" = "200" ] ||
  { echo "error: the client_credentials exchange with the key's id and secret no longer works" >&2; exit 1; }
echo "  ✓ the key's id and secret still exchange for a working access token"

# Revoked, the key is refused on every surface within the latency ADR012 §8d
# states. With the control-plane store the shared tombstone refuses it at the
# next request; a reused DB-free bex-api has none, so the bound is the 5 s
# introspection window, plus the poll.
[ "$(curl -s -o /dev/null -w '%{http_code}' -H "X-Session-Token: $SESSION_TOKEN" -X DELETE "$API/v1/api-keys/$KEY_ID")" = "204" ] ||
  { echo "error: revoking the key failed" >&2; exit 1; }
revoked_at="$(python3 -c 'import time; print(time.time())')"
refused=""
for _ in $(seq 1 16); do
  got="$(surfaces "$API_KEY")"
  if [ "$got" = "REST=401 GraphQL=401 MCP=401" ]; then
    refused="$(python3 -c "import time; print(round(time.time() - $revoked_at, 1))")"
    break
  fi
  sleep 0.5
done
[ -n "$refused" ] || { echo "error: the revoked key still works after 8 s: $got" >&2; exit 1; }
python3 -c "import sys; sys.exit(0 if $refused <= 6.0 else 1)" ||
  { echo "error: the revoked key was refused after ${refused}s, want within 5 s (plus the poll)" >&2; exit 1; }
[ "$(with_bearer exchanged "$EXCHANGED" "$API/v1/api-keys")" = "401" ] || { echo "error: the revoked key's exchanged token still works" >&2; exit 1; }
echo "  ✓ the revoked key answers 401 on REST, GraphQL and MCP ${refused}s after the revocation"

echo "-> 8/14 trusted client: PKCE authorize -> login -> HEADLESS consent -> token -> Bearer..."
JAR_TRUSTED="$TMP/trusted.jar"
final="$(authorize "$TRUSTED_CLIENT" "state-trusted" "$JAR_TRUSTED")"
expect_at "$CALLBACK" "$final" "trusted flow did not reach the agent callback (consent was not headless)"
[ "$(qparam "$final" state)" = "state-trusted" ] || { echo "error: state mismatch: $final" >&2; exit 1; }
ACCESS="$(exchange "$final" "$TRUSTED_CLIENT")"
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
ACCESS="$(exchange "$final" "$CONSENT_CLIENT")"
assert_bearer_works "$ACCESS"
echo "  ✓ user-consented token passes bex-api introspection + audience check, exactly like a blessed client's"

echo "-> 11/14 a second agent asks; the human denies -> access_denied, no code..."
# A fresh browser (jar) per authorization, deliberately: a clean browser holds no
# Kratos session and no remembered Hydra login, so it signs in with the password,
# and driving each leg from one keeps the legs independent (leg 12 proves the
# *consent* remember window on its own).
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
ACCESS="$(exchange "$final" "$CONSENT_CLIENT")"
assert_bearer_works "$ACCESS"
echo "  ✓ inside the remember window the consent page is skipped -> token straight through"

echo "-> 13/14 SIGNED-IN browser (reused jar): authorize again with no re-login..."
# The most common connect-an-agent path (docs/ADR025-connect-an-agent.md) and the one
# every leg above deliberately avoids: the user is *already signed into the
# dashboard* when the agent asks. JAR_USER has held a live Kratos session since
# leg 9, so this is the real thing, not a simulation of it.
#
# Hydra sends every authorization to the login page, even one it can skip, so the
# browser lands back there holding a session — the state that dead-ended before
# w4/m17. Leg 9's sign-in went through a login challenge, which Kratos accepted
# with `remember`, so Hydra remembers this browser and Kratos accepts the new
# challenge against the session. Assert the *path*, not just the outcome: no
# password form may be submitted. (A browser signed in at the dashboard only,
# which Hydra does not remember yet, is answered by the login page's server
# instead: leg extra/w7-079 below.)
: >"$TMP/login_mode" # a stale marker from an earlier leg must not pass this
final="$(authorize "$CONSENT_CLIENT" "state-signedin" "$JAR_USER")"
mode="$(cat "$TMP/login_mode")"
[ "$mode" = "signed-in-shortcircuit" ] || {
  echo "error: a signed-in browser should never re-authenticate; login took the '$mode' path" >&2
  exit 1
}
expect_at "$CALLBACK" "$final" "the signed-in authorization did not reach the agent callback"
[ "$(qparam "$final" state)" = "state-signedin" ] || { echo "error: state mismatch: $final" >&2; exit 1; }
ACCESS="$(exchange "$final" "$CONSENT_CLIENT")"
assert_bearer_works "$ACCESS"
echo "  ✓ already signed in -> no login form, no consent page -> token (this used to dead-end)"

echo "-> extra/w8-m27 read-only token cannot mutate..."
# ACCESS is the consented bex.read token from leg 13. A write verb must fail
# closed with insufficient_token_scope before OpenFGA or resource lookup.
status="$(curl -s -o "$TMP/write.out" -w '%{http_code}' -X POST \
  -H "Authorization: Bearer $ACCESS" \
  "http://localhost:$API_PORT/v1/services/does-not-exist/suspend")"
[ "$status" = "403" ] || { echo "error: read-only suspend returned $status: $(cat "$TMP/write.out")" >&2; exit 1; }
python3 -c "
import json
body = json.load(open('$TMP/write.out'))
assert body.get('code') == 'insufficient_token_scope', body
assert body.get('params', {}).get('required') == 'bex.write', body
print('  ✓ bex.read token refused RelCanOperate with insufficient_token_scope')
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
# The page renders the refusal in the reader's language since 10ded69c8; the
# route's own verdict, `scope_required`, rides its loader data in every one.
grep -q '"scope_required"' "$TMP/page.html" || { echo "error: bex.api umbrella was not refused: $(head -c 400 "$TMP/page.html")" >&2; exit 1; }
grep -q 'bex.read' "$TMP/page.html" || { echo "error: refusal did not name bex.read: $(head -c 400 "$TMP/page.html")" >&2; exit 1; }
[ -z "$(qparam "$final" code)" ] || { echo "error: refused bex.api flow still delivered a code: $final" >&2; exit 1; }
echo "  ✓ third-party bex.api is refused at consent as scope_required (no code)"

echo "-> extra/w7-079 signed in at the dashboard only: the first agent asks for no password..."
# The commonest first step of all: a person signed in on the dashboard's own
# login page, which never goes through Hydra, connects their first agent. Hydra
# does not remember this browser, so its login request is not skippable, and
# Kratos alone would answer it with a re-authentication form. The login page's
# server accepts the challenge against the live session instead
# (hydra-login.ts); the consent page still decides the grant.
JAR_DASH="$TMP/dashboard.jar"
dashboard_sign_in "$JAR_DASH"
FIRST_CLIENT="$(dcr 'first agent from a signed-in browser (e2e)')"
: >"$TMP/login_mode"
: >"$TMP/follow.chain"
final="$(authorize "$FIRST_CLIENT" "state-first" "$JAR_DASH")"
[ ! -s "$TMP/login_mode" ] || { echo "error: the signed-in browser was handed to Kratos's login ($(cat "$TMP/login_mode"))" >&2; exit 1; }
grep -q "^$DASH/auth/login?login_challenge=" "$TMP/follow.chain" ||
  { echo "error: Hydra never asked this browser to log in, so the leg proves nothing: $(cat "$TMP/follow.chain")" >&2; exit 1; }
expect_at "$DASH/auth/consent" "$final" "the first agent from a signed-in browser did not reach the consent page"
grep -q 'first agent from a signed-in browser (e2e)' "$TMP/page.html" || { echo "error: consent page does not name the client" >&2; exit 1; }
final="$(consent_decide approve "$JAR_DASH")"
expect_at "$CALLBACK" "$final" "approve did not reach the agent callback"
assert_bearer_works "$(exchange "$final" "$FIRST_CLIENT")"
grep -q ory_hydra_session "$JAR_DASH" || { echo "error: Hydra was not asked to remember the login" >&2; exit 1; }
echo "  ✓ the login page answered Hydra with the dashboard session: no password, then the consent page -> token"

# A client that asks for a fresh login, or for one newer than max_age, still
# gets Kratos's form even though the browser is signed in.
: >"$TMP/login_mode"
final="$(authorize "$FIRST_CLIENT" "state-prompt" "$JAR_DASH" "" "&prompt=login")"
[ "$(cat "$TMP/login_mode")" = "reauth-form" ] || { echo "error: prompt=login took the '$(cat "$TMP/login_mode")' path, want Kratos's re-authentication form" >&2; exit 1; }
expect_at "$CALLBACK" "$final" "the prompt=login authorization did not finish after the password"
sleep 2
: >"$TMP/login_mode"
final="$(authorize "$FIRST_CLIENT" "state-maxage" "$JAR_DASH" "" "&max_age=1")"
[ "$(cat "$TMP/login_mode")" = "reauth-form" ] || { echo "error: max_age=1 took the '$(cat "$TMP/login_mode")' path, want Kratos's re-authentication form" >&2; exit 1; }
expect_at "$CALLBACK" "$final" "the max_age authorization did not finish after the password"
echo "  ✓ prompt=login and an exceeded max_age still ask for the password"

echo "-> extra/w7-079 signed in at the dashboard only: bex login (RFC 8628) asks for no password..."
# The official Render CLI's `bex login`: the device page's Authorize click is
# the decision, and the login behind it must not ask again.
DEVICE_GRANT=urn:ietf:params:oauth:grant-type:device_code
CLI_CLIENT=429024F5E608930E2A65EF92591A25CC # hydra-device.ts RENDER_CLI_CLIENT_ID
curl -sf -X POST -H 'Content-Type: application/json' -d '{
  "client_id": "'"$CLI_CLIENT"'", "client_name": "bex CLI (e2e)",
  "grant_types": ["'"$DEVICE_GRANT"'", "refresh_token"],
  "scope": "openid offline_access bex.read", "token_endpoint_auth_method": "none",
  "skip_consent": true
}' "$HYDRA_ADM/admin/clients" >/dev/null
JAR_CLI="$TMP/cli.jar"
dashboard_sign_in "$JAR_CLI"
curl -sf -X POST -d "client_id=$CLI_CLIENT&scope=openid+offline_access+bex.read" "$ISSUER/oauth2/device/auth" >"$TMP/device.json"
read -r DEVICE_CODE VERIFY_URL < <(python3 -c "import json; d = json.load(open('$TMP/device.json')); print(d['device_code'], d['verification_uri_complete'])")
final="$(follow "$VERIFY_URL" "$JAR_CLI")"
expect_at "$DASH/auth/device?" "$final" "the verification link did not reach the device page"
[ -n "$(qparam "$final" device_challenge)" ] || { echo "error: the device page carries no device_challenge: $final" >&2; exit 1; }
curl -s -o /dev/null -D "$TMP/device.head" -c "$JAR_CLI" -b "$JAR_CLI" -X POST -H "Origin: $DASH" \
  --data-urlencode "user_code=$(qparam "$final" user_code)" \
  --data-urlencode "device_challenge=$(qparam "$final" device_challenge)" "$DASH/auth/device"
loc="$(grep -i '^location:' "$TMP/device.head" | tr -d '\r' | cut -d' ' -f2 | tail -1)"
[ -n "$loc" ] || { echo "error: Authorize on the device page did not redirect: $(head -1 "$TMP/device.head")" >&2; exit 1; }
: >"$TMP/follow.chain"
final="$(follow "$loc" "$JAR_CLI")"
grep -q "^$DASH/auth/login?login_challenge=" "$TMP/follow.chain" ||
  { echo "error: Hydra never asked the device browser to log in: $(cat "$TMP/follow.chain")" >&2; exit 1; }
expect_at "$DASH/auth/device/success" "$final" "the device login stopped short of the success page (a password form?)"
CLI_TOKEN="$(curl -s -X POST -d "grant_type=$DEVICE_GRANT&device_code=$DEVICE_CODE&client_id=$CLI_CLIENT" "$ISSUER/oauth2/token" |
  python3 -c 'import sys,json; print(json.load(sys.stdin).get("access_token",""))')"
[ -n "$CLI_TOKEN" ] || { echo "error: the approved device code did not exchange for a token" >&2; exit 1; }
assert_bearer_works "$CLI_TOKEN"
echo "  ✓ device page -> Authorize -> no password -> success page -> the CLI's token works"

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
echo "✓ w7/m426: an API key is a bearer on REST, GraphQL and MCP, and revocable"
echo "✓ w7/079: signed in at the dashboard only, the first agent and bex login ask"
echo "  for no password; prompt=login and max_age still do"
