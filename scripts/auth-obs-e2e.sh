#!/usr/bin/env bash
# End-to-end ADR088 ops-workspace consent gate check
# (docs/ADR088-platform-observability-ui.md §4-5, w5/m86): drives the Grafana
# (`bex-obs`) OAuth flow against REAL components — throwaway Hydra + Kratos +
# OpenFGA containers, the real dashboard consent acceptor (OAUTH_OPS_CLIENTS),
# and the real bex-api serving the internal ops-role verb on its DECOUPLED
# cluster-internal listener (BEX_OPS_WORKSPACE + BEX_OPS_ROLE_TOKEN with NO
# control-plane DB — the startOpsRoleServer path in cmd/api):
#
#   scripts/auth-bootstrap-client.sh (the real t003 bex-obs upsert)
#   -> authorize (PKCE S256, identity-only scopes, deliberately NO audience)
#   -> Kratos login (browser flow + login_challenge, as the dashboard drives it)
#   -> dashboard /auth/consent: the verified-email gate (w2/m168, ON as in
#      production) resolves consent.subject through bex-api's GET
#      /internal/identity-claims, then the ops gate resolves its role through
#      GET /internal/ops-role, both BEFORE the trusted headless accept (bex-obs
#      is skip_consent — a gate any later would never fire)
#   -> Hydra redirects to Grafana's byte-exact callback
#      https://obs.bex.co/login/generic_oauth — captured, NEVER fetched
#   -> token exchange (client_secret_basic + PKCE verifier) -> claims.
#
# Both verdicts (ADR088 §4):
#   * ALLOW — user A holds `user:<id> admin workspace:tea-obse2e`: the flow
#     completes headlessly and the id_token AND /userinfo carry
#     ops_role=GrafanaAdmin plus email/name stamped from the Kratos identity;
#   * DENY — user B outside the ops workspace bounces with error=access_denied
#     and no code ever issued; re-seated with a `billing` tuple (a real member
#     in a non-qualifying role) B still bounces — absence from the
#     admin/developer/viewer map IS the deny;
#   * DENY — user C, an ops-workspace admin whose email is NOT verified,
#     bounces with the verified-email gate's own description: the role never
#     outranks the gate (docs/ADR075 D8 revision). A and B hold verified
#     addresses (written through Kratos' admin API; this Kratos has no
#     courier), so their verdicts are the ops gate's alone.
#
# obs.bex.co is never resolved or fetched anywhere: the browser walk stops at
# the redirect and reads its query string — exactly what Grafana would receive.
#
# Self-contained: Hydra + Kratos + OpenFGA run as throwaway Docker containers
# (in-memory); bex-api and the dashboard dev server are ALWAYS started fresh —
# their ops env is the thing under test, so an already-running instance
# (auth-oauth21-e2e.sh's, say) must never be reused. Requires: docker, curl,
# python3, go, yarn, and a live kubectl context carrying the App CRD for
# bex-api's kube client (any cluster; no auth stack needed on it —
# scripts/mock-cluster.sh provides one).
#
# Usage: scripts/auth-obs-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."

NET=bexobs-e2e
HYDRA=bexobs-e2e-hydra
KRATOS=bexobs-e2e-kratos
OPENFGA=bexobs-e2e-openfga
# The same pinned OpenFGA image the backend CI suite runs against
# (.github/workflows/backend-test.yml); its entrypoint is the bare binary, so
# the HTTP server only starts under the `run` subcommand.
OPENFGA_IMAGE=openfga/openfga:latest@sha256:01a6000aa6040a4d0bde6ea1d3359ac3b9f21dc972ac6103a87b38659a836776
DASH_PORT=${DASH_PORT:-5197} # distinct from auth-oauth21-e2e.sh's 5199
API_PORT=${API_PORT:-8290}   # not 8090: a leftover bex-api has no ops env
CP_PORT=${CP_PORT:-8191}     # the cluster-internal listener (BEX_CP_ADDR)
FGA_PORT=${FGA_PORT:-18080}
ISSUER=http://localhost:4444
KRATOS_PUB=http://localhost:4433
KRATOS_ADM=http://localhost:4434
HYDRA_ADM=http://localhost:4445
DASH=http://localhost:$DASH_PORT
RESOURCE="http://localhost:$API_PORT/mcp"
FGA_URL="http://localhost:$FGA_PORT"
OPS_VERB="http://localhost:$CP_PORT/internal/ops-role"
CLAIMS_VERB="http://localhost:$CP_PORT/internal/identity-claims"

OBS_CLIENT=bex-obs
# Byte-exact Grafana generic_oauth callback (ADR088 §3) —
# auth-bootstrap-client.sh registers exactly this, and Hydra will redirect to
# exactly this; the walk below stops on the prefix instead of fetching it.
OBS_REDIRECT=https://obs.bex.co/login/generic_oauth
OPS_WORKSPACE=tea-obse2e

USER_A_EMAIL=obs-operator@bex.co
USER_A_NAME='Obs Operator'
USER_B_EMAIL=obs-customer@bex.co
USER_B_NAME='Obs Customer'
USER_C_EMAIL=obs-unverified@bex.co
USER_C_NAME='Obs Unverified'
USER_PASSWORD='e2e-password-123!'

# Throwaway secrets for a throwaway stack — random per run, never printed,
# dead with the containers. The bootstrap secret must be ≥16 chars
# (auth-bootstrap-client.sh enforces it).
BOOTSTRAP_SECRET="e2e-bootstrap-$(python3 -c 'import secrets;print(secrets.token_hex(8))')"
OBS_SECRET="e2e-grafana-$(python3 -c 'import secrets;print(secrets.token_hex(8))')"
OPS_TOKEN="$(python3 -c 'import secrets;print(secrets.token_hex(16))')"

TMP="$(mktemp -d)"
DASH_PGID=""
API_PGID=""

# Start a server as its own process group (job control on for the launch), so
# cleanup stops the whole tree, yarn's node, vite and esbuild children included
# (the auth-oauth21-e2e.sh pattern, w7/065). Sets LAUNCHED to the group's id,
# its leader's PID.
launch() { # log dir command...
  local log="$1" dir="$2"
  shift 2
  set -m
  (cd "$dir" && exec "$@") </dev/null >"$log" 2>&1 &
  LAUNCHED=$!
  set +m
}

# TERM the group, TERM it again (bex-api drains for ~15s on the first, still
# holding its ports, which would trip the next run's preflight, and
# force-exits on the second), then KILL what is left. Returns once the group
# is gone, so nothing this run started still listens when the script exits.
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
  status=$?
  if [ "$status" != "0" ]; then
    for log in bootstrap bexapi dash; do
      [ -f "$TMP/$log.log" ] && { echo "--- $log log tail ---" >&2; tail -15 "$TMP/$log.log" >&2; } || true
    done
  fi
  docker rm -f "$HYDRA" "$KRATOS" "$OPENFGA" >/dev/null 2>&1 || true
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

wait_status() { # url expected-status [attempts] — for routes with no 2xx probe
  local code=000
  for _ in $(seq 1 "${3:-30}"); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)"
    [ "$code" = "$2" ] && return 0
    sleep 1
  done
  echo "error: $1 never answered $2 (last: $code)" >&2
  return 1
}

# --- preflight -----------------------------------------------------------------
docker info >/dev/null 2>&1 || { echo "error: docker is not running" >&2; exit 1; }
# bex-api's startup lists Apps (deploy-hook index backfill), so its kube client
# needs a LIVE context with the App CRD — any cluster, nothing running on it.
kubectl get crd apps.app.bex.co >/dev/null 2>&1 || {
  echo "error: no live kubectl context with the App CRD — run scripts/mock-cluster.sh (or refresh a stale kind kubeconfig: kind export kubeconfig --name <cluster>)" >&2
  exit 1
}
# Every component starts fresh; an already-listening port means a stale run or
# a clash, never something to silently reuse.
for port in 4444 4445 4433 4434 "$FGA_PORT" "$API_PORT" "$CP_PORT" "$DASH_PORT"; do
  lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1 && {
    echo "error: port $port is already in use" >&2
    exit 1
  }
done

echo "-> 1/13 hydra (in-memory, login/consent -> dashboard :$DASH_PORT)..."
# v26, not v2.2.0: auth-bootstrap-client.sh asserts the per-client
# device-grant lifespan round-trips, a field neither v2.2.0 nor v2.3.0 has
# (the assertion exits "hydra too old") — the script itself
# documents its redirect behavior as "verified on Hydra v26", the production
# line.
docker network create "$NET" >/dev/null 2>&1 || true
docker rm -f "$HYDRA" >/dev/null 2>&1 || true
docker run -d --name "$HYDRA" --network "$NET" --network-alias hydra \
  -p 4444:4444 -p 4445:4445 \
  -e DSN=memory \
  -e URLS_SELF_ISSUER=$ISSUER \
  -e URLS_LOGIN="$DASH/auth/login" \
  -e URLS_CONSENT="$DASH/auth/consent" \
  -e SECRETS_SYSTEM=e2e-only-system-secret-32-chars-x \
  oryd/hydra:v26.2.0 serve all --dev >/dev/null
wait_http "$HYDRA_ADM/health/ready"

echo "-> 2/13 kratos (in-memory, native oauth2_provider -> hydra admin)..."
# The auth-oauth21-e2e.sh schema plus a `name` trait: the ops-role verb reads
# email AND display name from Kratos' admin API (workspaces.KratosIdentities),
# and the consent acceptor stamps both into the id_token — an identity without
# a name would still pass (name stamps as ""), but a named one proves the
# whole claim path. The email is a verification address, as in production's
# schema, so each identity carries the verifiable address the identity-claims
# verb reads `email_verified` from.
cat >"$TMP/identity.schema.json" <<'JSON'
{
  "$id": "https://schemas.bex.co/obs-e2e.schema.json",
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
          "ory.sh/kratos": {
            "credentials": { "password": { "identifier": true } },
            "verification": { "via": "email" }
          }
        },
        "name": { "type": "string", "title": "Name" }
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
# Kratos itself accepts Hydra login challenges (the w4/m9 bridge ADR088 §3
# rides: "login rides the existing Kratos-native bridge").
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

echo "-> 3/13 openfga (in-memory) + authz model + bootstrap tuple..."
docker rm -f "$OPENFGA" >/dev/null 2>&1 || true
docker run -d --name "$OPENFGA" --network "$NET" --network-alias openfga \
  -e OPENFGA_DATASTORE_ENGINE=memory \
  -p "$FGA_PORT:8080" \
  "$OPENFGA_IMAGE" run >/dev/null
wait_http "$FGA_URL/healthz"

fga() { # METHOD PATH [JSON_BODY] — scripts/authz-model.sh's call shape, minus
  # the preshared key (the throwaway runs open; it dies with the run).
  local args=(-s -X "$1" "$FGA_URL$2")
  [ "${3:-}" != "" ] && args+=(-H 'Content-Type: application/json' -d "$3")
  curl "${args[@]}"
}

write_tuple() { # user relation object — a direct tuple write, the ADR024 shape
  # the members surface produces (user:<kratos-id> <role> workspace:<tea-id>).
  local resp
  resp="$(fga POST "/stores/$STORE_ID/write" \
    '{"writes":{"tuple_keys":[{"user":"'"$1"'","relation":"'"$2"'","object":"'"$3"'"}]}}')"
  case "$resp" in
    *'"code"'*) echo "error: tuple write $1 $2 $3 failed: $resp" >&2; return 1 ;;
  esac
}

# The scripts/authz-model.sh sequence against a fresh store ("ensure by name"
# collapses to create): store `bex` — bex-api resolves the store BY NAME
# (internal/authz), so the name is load-bearing — then the committed model,
# then the bootstrap seed tuple.
STORE_ID="$(fga POST /stores '{"name":"bex"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')"
[ -n "$STORE_ID" ] || { echo "error: creating the bex store failed" >&2; exit 1; }
model_id="$(fga POST "/stores/$STORE_ID/authorization-models" "$(cat deploy/gitops/authz/model.json)" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin).get("authorization_model_id",""))')"
[ -n "$model_id" ] || { echo "error: writing the authz model failed" >&2; exit 1; }
write_tuple user:bex-bootstrap admin workspace:default
echo "  ✓ store bex ($STORE_ID) seeded with deploy/gitops/authz/model.json"

echo "-> 4/13 platform clients via scripts/auth-bootstrap-client.sh (the real t003 path)..."
HYDRA_ADMIN_URL=$HYDRA_ADM \
  BEX_BOOTSTRAP_CLIENT_SECRET="$BOOTSTRAP_SECRET" \
  BEX_OBS_OAUTH_CLIENT_SECRET="$OBS_SECRET" \
  bash scripts/auth-bootstrap-client.sh >"$TMP/bootstrap.log" 2>&1
grep -q "OAuth2 client $OBS_CLIENT" "$TMP/bootstrap.log" \
  || { echo "error: bootstrap did not provision $OBS_CLIENT" >&2; exit 1; }
# The script asserts its own round-trips; re-assert the stored posture here so
# a drifted registration fails THIS script with a pointed message: exact
# redirect, confidential client_secret_basic, skip_consent (the headless path
# the gate must intercept), identity-only scope, and NO audience.
curl -sf "$HYDRA_ADM/admin/clients/$OBS_CLIENT" >"$TMP/obs-client.json"
python3 -c "
import json
c = json.load(open('$TMP/obs-client.json'))
assert c.get('redirect_uris') == ['$OBS_REDIRECT'], c.get('redirect_uris')
assert c.get('token_endpoint_auth_method') == 'client_secret_basic', c
assert c.get('skip_consent') is True, c
assert set(c.get('scope', '').split()) == {'openid', 'profile', 'email'}, c.get('scope')
assert not c.get('audience'), c.get('audience')
print('  ✓ bex-obs registered: exact Grafana callback, confidential, skip_consent, zero audience')
"

echo "-> 5/13 bex-api: ops-role + identity-claims verbs on :$CP_PORT with NO control-plane DB..."
(cd lego/backend && go build -o "$TMP/bex-api" ./cmd/api)
# No BEX_CP_DB_URI on purpose: with BEX_OPS_WORKSPACE + BEX_OPS_ROLE_TOKEN set
# the verbs get their own minimal internal listener (startOpsRoleServer) —
# exactly the decoupling under test. BEX_KRATOS_ADMIN_URL feeds both verbs'
# identity lookup (email/email_verified/name); BEX_OPENFGA_URL feeds the
# ops-role verb's role reads.
launch "$TMP/bexapi.log" . env BEX_HYDRA_ADMIN_URL="$HYDRA_ADM" BEX_KRATOS_URL="$KRATOS_PUB" \
  BEX_KRATOS_ADMIN_URL="$KRATOS_ADM" \
  BEX_OAUTH_ISSUER="$ISSUER" BEX_OAUTH_RESOURCE="$RESOURCE" \
  BEX_API_ADDR=":$API_PORT" \
  BEX_OPS_WORKSPACE="$OPS_WORKSPACE" BEX_OPS_ROLE_TOKEN="$OPS_TOKEN" \
  BEX_CP_ADDR=":$CP_PORT" \
  BEX_OPENFGA_URL="$FGA_URL" \
  "$TMP/bex-api"
API_PGID="$LAUNCHED"
wait_http "http://localhost:$API_PORT/healthz" 60
# A bearerless probe answering 401 (not a router 404) proves the decoupled
# listener mounted each verb — and that the bearer is checked before anything.
wait_status "$OPS_VERB?subject=probe" 401 30
wait_status "$CLAIMS_VERB?subject=probe" 401 30
echo "  ✓ decoupled internal listener up; bearerless probes of both verbs -> 401"

echo "-> 6/13 dashboard dev server (verified-email + ops-gated consent acceptor) on :$DASH_PORT..."
# The verified-email gate is on, as in production (pinned, so an exported 0
# cannot turn it off), and BEX_IDENTITY_CLAIMS_URL points it at the verb above
# (same bearer): every consent here really runs it (w7/074).
launch "$TMP/dash.log" dashboard env VITE_KRATOS_PUBLIC_URL="$KRATOS_PUB" \
  VITE_API_URL="http://localhost:$API_PORT/graphql" \
  HYDRA_ADMIN_URL="$HYDRA_ADM" \
  OAUTH_OPS_CLIENTS="$OBS_CLIENT" \
  BEX_OPS_ROLE_URL="$OPS_VERB" BEX_OPS_ROLE_TOKEN="$OPS_TOKEN" \
  BEX_REQUIRE_VERIFIED_EMAIL=1 BEX_IDENTITY_CLAIMS_URL="$CLAIMS_VERB" \
  yarn dev --port "$DASH_PORT"
DASH_PGID="$LAUNCHED"
wait_http "$DASH/auth/consent" 120 # 302-to-home counts: the route compiled

echo "-> 7/13 identities: ops operator (A) + customer (B), both verified; unverified ops admin (C)..."
create_identity() { # email name verified(true|false) -> identity id on stdout
  # Kratos derives each identity's verifiable address from the schema's
  # verification marker; an admin create may pass that address in already
  # verified, which is how a verified human looks to the identity-claims verb.
  # C passes none, so its address is created pending, as a sign-up's is.
  local addresses='[]'
  [ "$3" = "true" ] && addresses='[{"value":"'"$1"'","via":"email","verified":true,"status":"completed"}]'
  curl -sf -X POST -H 'Content-Type: application/json' -d '{
    "schema_id": "default",
    "traits": { "email": "'"$1"'", "name": "'"$2"'" },
    "verifiable_addresses": '"$addresses"',
    "credentials": { "password": { "config": { "password": "'"$USER_PASSWORD"'" } } }
  }' "$KRATOS_ADM/admin/identities" | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])'
}
IDENTITY_A="$(create_identity "$USER_A_EMAIL" "$USER_A_NAME" true)"
IDENTITY_B="$(create_identity "$USER_B_EMAIL" "$USER_B_NAME" true)"
IDENTITY_C="$(create_identity "$USER_C_EMAIL" "$USER_C_NAME" false)"
write_tuple "user:$IDENTITY_A" admin "workspace:$OPS_WORKSPACE"
write_tuple "user:$IDENTITY_C" admin "workspace:$OPS_WORKSPACE"
echo "  ✓ A=$IDENTITY_A (verified, admin of $OPS_WORKSPACE), B=$IDENTITY_B (verified, no tuple),"
echo "    C=$IDENTITY_C (unverified, admin of $OPS_WORKSPACE)"

# --- the browser, in shell -----------------------------------------------------
# RFC 7636 test-vector PKCE pair (the same fixed pair auth-oauth21-e2e.sh uses).
VERIFIER="dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
CHALLENGE="E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
REDIRECT_ENC="$(python3 -c "from urllib.parse import quote; print(quote('$OBS_REDIRECT', safe=''))")"

qparam() { # url param -> value on stdout ("" when absent)
  python3 -c "
from urllib.parse import urlparse, parse_qs
print(parse_qs(urlparse('$1').query).get('$2', [''])[0])
"
}

# Walk redirects like a browser would, with a cookie jar, stopping at Grafana's
# callback (obs.bex.co — read from the Location header, NEVER fetched: no
# network path to that host exists or is needed) or at any page that renders.
# The final URL lands on stdout; the last fetched body in $TMP/page.html.
follow() { # url jar -> final url on stdout
  local url="$1" jar="$2" loc
  for _ in $(seq 1 8); do
    case "$url" in "$OBS_REDIRECT"*) break ;; esac
    curl -s -o "$TMP/page.html" -D "$TMP/page.head" -c "$jar" -b "$jar" "$url"
    loc="$(grep -i '^location:' "$TMP/page.head" | tr -d '\r' | cut -d' ' -f2 | tail -1)"
    [ -z "$loc" ] && break
    url="$loc"
  done
  echo "$url"
}

# The dashboard login page's exact call (use-ory-flow.ts): create a browser
# login flow bound to the challenge, then submit the password form. Success
# answers 422 browser_location_change_required -> Hydra's continue URL (Kratos
# accepted the login challenge natively). Every leg here drives a FRESH jar,
# so the already-signed-in shortcircuit (auth-oauth21-e2e.sh leg 13) must never
# appear — a `null` flow answer is a bug in this script's jar hygiene.
kratos_login() { # login_challenge jar email -> continue url on stdout
  local challenge="$1" jar="$2" email="$3" flow action csrf submit cont url
  url="$KRATOS_PUB/self-service/login/browser?login_challenge=$challenge&return_to=$DASH/"
  flow="$(curl -s -c "$jar" -b "$jar" -H 'Accept: application/json' "$url")"
  if [ "$(echo "$flow" | tr -d '[:space:]')" = "null" ]; then
    echo "error: Kratos short-circuited a login that should have rendered the password form (stale jar?)" >&2
    return 1
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
    -d "{\"method\":\"password\",\"identifier\":\"$email\",\"password\":\"$USER_PASSWORD\",\"csrf_token\":\"$csrf\"}" \
    "$action")"
  cont="$(echo "$submit" | python3 -c "import sys,json;print(json.load(sys.stdin).get('redirect_browser_to',''))")"
  [ -n "$cont" ] || { echo "error: login did not yield a continue URL: $submit" >&2; return 1; }
  echo "$cont"
}

# One authorization for the obs client, driven as a browser: authorize ->
# login -> wherever the consent acceptor sends us (the ACCEPT continue chain
# ends at the captured Grafana callback with a code; a REJECT ends there with
# error=access_denied). Identity-only scopes and no audience parameter — the
# registered ADR088 §3 shape; PKCE S256 because the consent gate refuses
# anything less (pkceSatisfied).
authorize() { # state jar email -> final url on stdout
  local state="$1" jar="$2" email="$3" final challenge cont
  final="$(follow "$ISSUER/oauth2/auth?client_id=$OBS_CLIENT&response_type=code&scope=openid+profile+email&redirect_uri=$REDIRECT_ENC&state=$state&code_challenge=$CHALLENGE&code_challenge_method=S256" "$jar")"
  case "$final" in
    "$DASH/auth/login"*)
      challenge="$(qparam "$final" login_challenge)"
      [ -n "$challenge" ] || { echo "error: login page without a login_challenge: $final" >&2; return 1; }
      cont="$(kratos_login "$challenge" "$jar" "$email")"
      final="$(follow "$cont" "$jar")"
      ;;
  esac
  echo "$final"
}

expect_at() { # expected-url-prefix actual-url what-went-wrong
  case "$2" in "$1"*) return 0 ;; esac
  echo "error: $3: $2" >&2
  exit 1
}

# The two descriptions rejectConsent sends (hydra-consent.ts): the ops gate's
# deny, and the verified-email gate's.
OPS_DENIED="The user denied the request"
UNVERIFIED_DENIED="Verify your email address before signing in to other apps"

assert_denied() { # final-url what [description] — a consent-stage reject, not a transport failure
  local want="${3:-$OPS_DENIED}"
  expect_at "$OBS_REDIRECT" "$1" "$2 did not end at the Grafana redirect"
  [ "$(qparam "$1" error)" = "access_denied" ] || { echo "error: $2 did not yield error=access_denied: $1" >&2; exit 1; }
  # The description is rejectConsent's own string riding through Hydra — proof
  # the deny came from the named gate of the acceptor, not some upstream failure
  # (nor the other gate: B is verified, so its deny is the ops gate's alone).
  [ "$(qparam "$1" error_description)" = "$want" ] \
    || { echo "error: $2 was not the consent acceptor's \"$want\" reject: $1" >&2; exit 1; }
  [ -z "$(qparam "$1" code)" ] || { echo "error: $2 still delivered an authorization code: $1" >&2; exit 1; }
}

echo "-> 8/13 ALLOW: A's PKCE authorize -> login -> verified-email + ops-gated HEADLESS consent -> callback..."
JAR_A="$TMP/allow.jar"
: >"$TMP/login_mode"
final="$(authorize state-allow "$JAR_A" "$USER_A_EMAIL")"
# Assert the path, not just the outcome: a fresh browser must have really
# authenticated (password form), and the whole consent hop must have stayed
# headless — bex-obs is skip_consent, so a rendered consent page here would
# mean the trusted path broke.
[ "$(cat "$TMP/login_mode")" = "password-form" ] \
  || { echo "error: the allow leg did not authenticate via the password form ($(cat "$TMP/login_mode"))" >&2; exit 1; }
expect_at "$OBS_REDIRECT" "$final" "the allow flow did not end at the Grafana redirect (consent was not headless?)"
[ "$(qparam "$final" state)" = "state-allow" ] || { echo "error: state mismatch: $final" >&2; exit 1; }
CODE="$(qparam "$final" code)"
[ -n "$CODE" ] || { echo "error: no authorization code on the callback: $final" >&2; exit 1; }
echo "  ✓ headless gated consent accepted; code captured at the (never-fetched) obs.bex.co redirect"

echo "-> 9/13 token exchange (client_secret_basic + PKCE verifier) -> id_token claims + userinfo..."
curl -s -u "$OBS_CLIENT:$OBS_SECRET" -X POST \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  --data-urlencode "redirect_uri=$OBS_REDIRECT" \
  --data-urlencode "code_verifier=$VERIFIER" \
  "$ISSUER/oauth2/token" >"$TMP/token.json"
python3 -c "
import base64, json
tok = json.load(open('$TMP/token.json'))
assert 'access_token' in tok and 'id_token' in tok, tok
seg = tok['id_token'].split('.')[1]
claims = json.loads(base64.urlsafe_b64decode(seg + '=' * (-len(seg) % 4)))
assert claims.get('ops_role') == 'GrafanaAdmin', claims
assert claims.get('email') == '$USER_A_EMAIL', claims
assert claims.get('name') == '$USER_A_NAME', claims
assert claims.get('sub') == '$IDENTITY_A', claims
print('  ✓ id_token: ops_role=GrafanaAdmin (admin mapped), email + name stamped, subject bound')
"
ACCESS="$(python3 -c "import json;print(json.load(open('$TMP/token.json'))['access_token'])")"
# Grafana reads the userinfo endpoint too (generic_oauth api_url), so the
# claims must surface there as well, not only inside the JWT.
curl -sf -H "Authorization: Bearer $ACCESS" "$ISSUER/userinfo" >"$TMP/userinfo.json"
python3 -c "
import json
u = json.load(open('$TMP/userinfo.json'))
assert u.get('ops_role') == 'GrafanaAdmin', u
assert u.get('email') == '$USER_A_EMAIL', u
print('  ✓ /userinfo mirrors ops_role + email for Grafana api_url')
"

echo "-> 10/13 the verbs themselves: wrong bearer -> 401; A -> the pinned member shape; A/C -> their verification..."
for verb in "$OPS_VERB" "$CLAIMS_VERB"; do
  status="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer not-$OPS_TOKEN" "$verb?subject=$IDENTITY_A")"
  [ "$status" = "401" ] || { echo "error: a wrong bearer at $verb answered $status, not 401" >&2; exit 1; }
done
curl -sf -H "Authorization: Bearer $OPS_TOKEN" "$OPS_VERB?subject=$IDENTITY_A" >"$TMP/opsrole.json"
curl -sf -H "Authorization: Bearer $OPS_TOKEN" "$CLAIMS_VERB?subject=$IDENTITY_A" >"$TMP/claims-a.json"
curl -sf -H "Authorization: Bearer $OPS_TOKEN" "$CLAIMS_VERB?subject=$IDENTITY_C" >"$TMP/claims-c.json"
python3 -c "
import json
body = json.load(open('$TMP/opsrole.json'))
assert body == {'member': True, 'role': 'admin', 'email': '$USER_A_EMAIL', 'name': '$USER_A_NAME'}, body
a = json.load(open('$TMP/claims-a.json'))
assert a == {'email': '$USER_A_EMAIL', 'email_verified': True, 'name': '$USER_A_NAME'}, a
c = json.load(open('$TMP/claims-c.json'))
assert c == {'email': '$USER_C_EMAIL', 'email_verified': False, 'name': '$USER_C_NAME'}, c
print('  ✓ verbs: wrong bearer 401; member answer carries role/email/name (the t004 wire contract);')
print('    identity-claims reads A verified and C unverified from Kratos')
"

echo "-> 11/13 DENY: B (authenticated, verified customer, NO ops tuple) -> access_denied, no code..."
# A fresh jar per leg, deliberately (the model script's pattern): the deny
# path never remembers a consent, but clean jars keep the legs independent.
JAR_B="$TMP/deny.jar"
final="$(authorize state-deny "$JAR_B" "$USER_B_EMAIL")"
assert_denied "$final" "the non-member flow"
echo "  ✓ authentication alone is not access: the customer bounces with access_denied"

echo "-> 12/13 DENY: B re-seated with a NON-QUALIFYING role (billing) -> still access_denied..."
write_tuple "user:$IDENTITY_B" billing "workspace:$OPS_WORKSPACE"
JAR_B2="$TMP/deny-billing.jar"
final="$(authorize state-deny-billing "$JAR_B2" "$USER_B_EMAIL")"
assert_denied "$final" "the billing-role flow"
echo "  ✓ billing is real membership but no observability role — absence from the map IS the deny"

echo "-> 13/13 DENY: C (ops-workspace ADMIN, email NOT verified) -> the verified-email gate's access_denied..."
JAR_C="$TMP/deny-unverified.jar"
final="$(authorize state-deny-unverified "$JAR_C" "$USER_C_EMAIL")"
assert_denied "$final" "the unverified-admin flow" "$UNVERIFIED_DENIED"
echo "  ✓ the admin role does not outrank the verified-email gate: C bounces before the ops gate"

echo
echo "✓ ADR088 §4-5 end-to-end: auth-bootstrap-client.sh registered bex-obs ->"
echo "  PKCE code flow through Kratos login and the dashboard's verified-email +"
echo "  ops-gated HEADLESS consent (claims and role resolved via bex-api's decoupled"
echo "  /internal/identity-claims and /internal/ops-role, no CP DB)"
echo "  -> id_token + userinfo carry ops_role/email/name for the verified admin;"
echo "  non-members, non-qualifying roles and an unverified admin are rejected with"
echo "  access_denied, no code"
