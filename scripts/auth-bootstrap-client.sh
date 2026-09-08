#!/usr/bin/env bash
# Seed the permanent platform OAuth2 clients: confidential `bex-bootstrap`, the
# secretless public client hard-coded by the official Render CLI, the secretless
# first-party `bex-desktop` editor client (Zed), the secretless first-party
# `bex-mobile` native client, and the confidential `bex-obs` Grafana client
# (docs/ADR088-platform-observability-ui.md §3). None is a tenant-created API
# key. Idempotent: re-running resets every client to its intended grants without
# minting a public-client secret.
#
# The public clients are always upserted through the admin REST API (curl),
# never the in-pod `hydra` CLI: they carry fields the CLI cannot express, and
# `hydra update client` PUTs a full replacement that would silently wipe them.
#
# Secrets come from .env (gitignored) or the environment (CI: GitHub Actions
# secrets): BEX_BOOTSTRAP_CLIENT_SECRET is required, while
# BEX_OBS_OAUTH_CLIENT_SECRET gates only the obs client — when it is missing,
# that one client is refused and the rest are seeded normally. Values are never
# printed.
#
# Usage: scripts/auth-bootstrap-client.sh          # kubectl exec into hydra pod
#        HYDRA_ADMIN_URL=http://... scripts/...    # use an already-reachable admin URL
#        BEX_AUTH_NAMESPACE=dev-3-auth scripts/...  # non-default auth namespace
# Requires: kubectl, curl (for the port-forward fallback path).
set -euo pipefail
cd "$(dirname "$0")/.."

CLIENT_ID=bex-bootstrap
RENDER_CLI_CLIENT_ID=429024F5E608930E2A65EF92591A25CC
DESKTOP_CLIENT_ID=bex-desktop
MOBILE_CLIENT_ID=bex-mobile
# ACCEPTED-RISK: private-use custom-scheme callback (RFC 8252, single slash), not
# an https universal link. See mobile/src/features/auth/config.ts and ADR012 §
# "Mobile OAuth redirect (accepted risk)". Do not switch back to https without
# completing AASA/assetlinks + a dashboard /oauth2redirect bridge, or fresh
# logins 404 (regression from commit 9081fbdb).
MOBILE_REDIRECT_URI=co.bex.mobile:/oauth2redirect
MOBILE_AUDIENCE="${BEX_OAUTH_RESOURCE:-https://api.bex.co/mcp}"
OBS_CLIENT_ID=bex-obs
# Byte-exact Grafana generic_oauth callback (ADR088 §3) — the single entry.
OBS_REDIRECT_URI=https://obs.bex.co/login/generic_oauth
DEVICE_GRANT=urn:ietf:params:oauth:grant-type:device_code
NS="${BEX_AUTH_NAMESPACE:-auth}"
PF_PID=""
PF_LOG=""

# An explicitly-set env var wins; .env is only consulted as the local fallback
# (CI passes the secrets directly and has no .env). Pre-source values are
# captured so sourcing .env for one missing secret never clobbers the other.
env_bootstrap_secret="${BEX_BOOTSTRAP_CLIENT_SECRET:-}"
env_obs_secret="${BEX_OBS_OAUTH_CLIENT_SECRET:-}"
if { [ -z "$env_bootstrap_secret" ] || [ -z "$env_obs_secret" ]; } && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
secret="${env_bootstrap_secret:-${BEX_BOOTSTRAP_CLIENT_SECRET:-}}"
obs_secret="${env_obs_secret:-${BEX_OBS_OAUTH_CLIENT_SECRET:-}}"
[ -n "$secret" ] || { echo "error: BEX_BOOTSTRAP_CLIENT_SECRET is missing or empty (.env or environment)" >&2; exit 1; }
[ "${#secret}" -ge 16 ] || { echo "error: BEX_BOOTSTRAP_CLIENT_SECRET must be at least 16 characters (got ${#secret})" >&2; exit 1; }

cleanup() {
  if [ -n "$PF_PID" ]; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
  fi
  # `if`, not `[ … ] && …`: under `set -e` a trap whose LAST command returns
  # non-zero makes the whole script exit non-zero. With HYDRA_ADMIN_URL set (the
  # port-forward path callers like events-verify.sh/secrets-verify.sh use) PF_LOG
  # is empty, so the test failed and this script "failed" after succeeding.
  if [ -n "$PF_LOG" ]; then
    rm -f "$PF_LOG"
  fi
}
trap cleanup EXIT

admin="${HYDRA_ADMIN_URL:-}"
USE_EXEC=0
REST_ADMIN="$admin"

# start_port_forward: make the admin API REST-reachable at $REST_ADMIN via a
# port-forward, with full diagnostic output captured. No-op when REST_ADMIN is
# already set (HYDRA_ADMIN_URL given, or a forward is already up).
start_port_forward() {
  [ -n "$REST_ADMIN" ] && return 0
  PF_LOG=$(mktemp)
  kubectl -n "$NS" port-forward service/hydra-admin 34445:4445 > "$PF_LOG" 2>&1 &
  PF_PID=$!
  REST_ADMIN=http://127.0.0.1:34445
  ready=0
  for _ in $(seq 1 45); do
    curl -sf -o /dev/null "$REST_ADMIN/health/ready" && ready=1 && break
    sleep 2
  done
  if [ "$ready" != "1" ]; then
    echo "error: hydra admin port-forward not ready after 90s — diagnostics:" >&2
    cat "$PF_LOG" >&2
    echo "--- hydra-admin service endpoints ---" >&2
    kubectl -n "$NS" get endpoints hydra-admin >&2 || true
    echo "--- hydra pod status ---" >&2
    kubectl -n "$NS" get pods -l "app.kubernetes.io/name=hydra" >&2 || true
    exit 1
  fi
}

if [ -z "$admin" ]; then
  # Preferred path: kubectl exec into the Hydra pod and use the hydra CLI.
  # This avoids the port-forward TCP tunnel entirely — the hydra binary reaches
  # its own admin API at 127.0.0.1:4445 without needing a forwarded port.
  HYDRA_POD=$(kubectl -n "$NS" get pod \
    -l "app.kubernetes.io/name=hydra" \
    --field-selector=status.phase=Running \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null | head -1)

  if [ -n "$HYDRA_POD" ]; then
    admin="http://127.0.0.1:4445"
    USE_EXEC=1
    echo "seeding client via kubectl exec into pod $HYDRA_POD"
  else
    start_port_forward
    admin="$REST_ADMIN"
  fi
fi

if [ "$USE_EXEC" = "1" ]; then
  # Upsert via Hydra CLI inside the pod (update if exists, create if not).
  exec_hydra() { kubectl -n "$NS" exec "$HYDRA_POD" -- hydra "$@"; }

  if exec_hydra update client "$CLIENT_ID" \
      --endpoint "$admin" \
      --secret "$secret" \
      --grant-type client_credentials \
      --token-endpoint-auth-method client_secret_post \
      --name "bex bootstrap (platform operator + CI)" > /dev/null 2>&1; then
    echo "updated OAuth2 client $CLIENT_ID"
  else
    exec_hydra create client \
      --endpoint "$admin" \
      --id "$CLIENT_ID" \
      --secret "$secret" \
      --grant-type client_credentials \
      --token-endpoint-auth-method client_secret_post \
      --name "bex bootstrap (platform operator + CI)" > /dev/null
    echo "created OAuth2 client $CLIENT_ID"
  fi

else
  # Port-forward path: use curl.
  body="$(printf '{"client_id":"%s","client_name":"bex bootstrap (platform operator + CI)","client_secret":"%s","grant_types":["client_credentials"],"token_endpoint_auth_method":"client_secret_post"}' "$CLIENT_ID" "$secret")"

  # Upsert: PUT updates an existing client (and resets its secret); 404 => create.
  code="$(printf '%s' "$body" | curl -s -o /dev/null -w '%{http_code}' -X PUT \
    -H 'Content-Type: application/json' -d @- "$admin/admin/clients/$CLIENT_ID")"
  if [ "$code" = "404" ]; then
    code="$(printf '%s' "$body" | curl -s -o /dev/null -w '%{http_code}' -X POST \
      -H 'Content-Type: application/json' -d @- "$admin/admin/clients")"
    [ "$code" = "201" ] || { echo "error: creating $CLIENT_ID failed (HTTP $code)" >&2; exit 1; }
    echo "created OAuth2 client $CLIENT_ID"
  elif [ "$code" = "200" ]; then
    echo "updated OAuth2 client $CLIENT_ID"
  else
    echo "error: upserting $CLIENT_ID failed (HTTP $code)" >&2
    exit 1
  fi

fi

# ---- Render CLI client: always via the admin REST API (see header comment) ----
start_port_forward

# One upsert + read-back vocabulary for every REST-admin client below. PUT is a
# full replace (resets drifted fields, including any secret); 404 => create.
upsert_client() {
  local id="$1" body="$2" code
  code="$(printf '%s' "$body" | curl -s -o /dev/null -w '%{http_code}' -X PUT \
    -H 'Content-Type: application/json' -d @- "$REST_ADMIN/admin/clients/$id")"
  if [ "$code" = "404" ]; then
    code="$(printf '%s' "$body" | curl -s -o /dev/null -w '%{http_code}' -X POST \
      -H 'Content-Type: application/json' -d @- "$REST_ADMIN/admin/clients")"
    [ "$code" = "201" ] || { echo "error: creating $id failed (HTTP $code)" >&2; exit 1; }
    echo "created OAuth2 client $id"
  elif [ "$code" = "200" ]; then
    echo "updated OAuth2 client $id"
  else
    echo "error: upserting $id failed (HTTP $code)" >&2
    exit 1
  fi
}

# Hydra ignores unknown JSON fields, so every client's load-bearing fields are
# asserted on the stored read-back instead of trusting the PUT status alone
# (a typo'd key would be ignored, not rejected).
read_client() {
  curl -sf "$REST_ADMIN/admin/clients/$1" || {
    echo "error: reading back $1 failed" >&2
    exit 1
  }
}

assert_stored() {
  local stored="$1" needle="$2" msg="$3"
  printf '%s' "$stored" | grep -Fq "$needle" || { echo "error: $msg" >&2; exit 1; }
}

# Per-client access-token lifespan for BOTH grants that mint CLI tokens (device
# code = first login, refresh_token = every rotation). The unmodified CLI
# refreshes whenever a token is within 24h of expiry. Seven days matches
# Render's standing posture, while bex-api's replica-shared refresh
# idempotency store makes the TUI's concurrent refresh safe at any TTL instead
# of letting one rotation revoke the sibling access token. Hydra accepts Go
# duration syntax, so seven days is written as 168h.
CLI_TOKEN_LIFESPAN=168h

# skip_consent must ride every upsert: this is the operator-blessed trusted
# client (docs/ADR012-auth.md §8a) — the dashboard consent route auto-accepts on
# this flag, and the CLI device flow hard-depends on it. PUT is a full replace,
# so omitting it here silently un-blesses the client and strands every
# subsequent `render login` at a consent step that never completes.
#
# The public client id is not an authority boundary; bex-api still requires
# granular capabilities for every human OAuth token. The mobile client below
# deliberately keeps explicit consent because its HTTPS callback is public.
render_body="$(printf '{"client_id":"%s","client_name":"bex CLI","grant_types":["%s","refresh_token"],"scope":"openid offline_access bex.read bex.write bex.sensitive","token_endpoint_auth_method":"none","subject_type":"public","skip_consent":true,"device_authorization_grant_access_token_lifespan":"%s","refresh_token_grant_access_token_lifespan":"%s"}' \
  "$RENDER_CLI_CLIENT_ID" "$DEVICE_GRANT" "$CLI_TOKEN_LIFESPAN" "$CLI_TOKEN_LIFESPAN")"
upsert_client "$RENDER_CLI_CLIENT_ID" "$render_body"

# Assert BOTH grant lifespans round-trip on the stored client. Prefix match,
# not exact — Hydra may normalize the duration (168h vs 168h0m0s).
stored_render_client="$(read_client "$RENDER_CLI_CLIENT_ID")"
for lifespan_field in \
  device_authorization_grant_access_token_lifespan \
  refresh_token_grant_access_token_lifespan; do
  assert_stored "$stored_render_client" "\"$lifespan_field\":\"$CLI_TOKEN_LIFESPAN" \
    "$RENDER_CLI_CLIENT_ID $lifespan_field did not round-trip (hydra too old for per-client lifespans?)"
done

# ---- First-party desktop / editor client (bex Desktop, Zed) ----------------
# The editor is a desktop GUI app, so it uses the standard native-app flow —
# OAuth 2.0 Authorization Code + PKCE with an RFC 8252 loopback redirect — NOT
# the device grant (that is for input-constrained/browserless surfaces like the
# CLI). Its own client so token audience, telemetry, revocation, and lifespans
# are decoupled from the CLI; the client id is not an authority boundary
# (bex-api still enforces per-token granular capabilities, docs/ADR012-auth.md),
# so this grants no more than the CLI. skip_consent rides every upsert
# (first-party): the editor bounces straight back after login with no consent
# screen. Loopback redirects are registered path-only — Hydra accepts any port
# on 127.0.0.1/localhost (RFC 8252 §7.3, verified on Hydra v26), so the editor's
# ephemeral callback port always matches.
desktop_body="$(printf '{"client_id":"%s","client_name":"bex Desktop (Zed)","grant_types":["authorization_code","refresh_token"],"response_types":["code"],"redirect_uris":["http://127.0.0.1/callback","http://localhost/callback"],"scope":"openid offline_access bex.read bex.write bex.sensitive","token_endpoint_auth_method":"none","subject_type":"public","skip_consent":true,"authorization_code_grant_access_token_lifespan":"%s","refresh_token_grant_access_token_lifespan":"%s"}' \
  "$DESKTOP_CLIENT_ID" "$CLI_TOKEN_LIFESPAN" "$CLI_TOKEN_LIFESPAN")"
upsert_client "$DESKTOP_CLIENT_ID" "$desktop_body"

stored_desktop_client="$(read_client "$DESKTOP_CLIENT_ID")"
assert_stored "$stored_desktop_client" '"http://127.0.0.1/callback"' \
  "$DESKTOP_CLIENT_ID loopback redirect did not round-trip"
assert_stored "$stored_desktop_client" '"token_endpoint_auth_method":"none"' \
  "$DESKTOP_CLIENT_ID is not a public client"

# ---- First-party native mobile client (ADR012 §8b) -------------------------
# A store-distributed app cannot keep a client secret. The reverse-domain
# private-use redirect is exact and single-slash per RFC 8252 (ACCEPTED-RISK:
# not an https universal link — see MOBILE_REDIRECT_URI note above); PKCE S256
# is required on every authorization. skip_consent=true: it is a first-party
# app and the token still requires granular capabilities at bex-api.
mobile_body="$(printf '{"client_id":"%s","client_name":"bex mobile (first-party native)","grant_types":["authorization_code","refresh_token"],"response_types":["code"],"redirect_uris":["%s"],"audience":["%s"],"scope":"openid offline_access bex.read bex.write","token_endpoint_auth_method":"none","subject_type":"public","skip_consent":true}' \
  "$MOBILE_CLIENT_ID" "$MOBILE_REDIRECT_URI" "$MOBILE_AUDIENCE")"
upsert_client "$MOBILE_CLIENT_ID" "$mobile_body"

mobile_stored="$(read_client "$MOBILE_CLIENT_ID")"
assert_stored "$mobile_stored" "\"$MOBILE_REDIRECT_URI\"" \
  "$MOBILE_CLIENT_ID redirect URI did not round-trip"
assert_stored "$mobile_stored" '"token_endpoint_auth_method":"none"' \
  "$MOBILE_CLIENT_ID is not a public client"
assert_stored "$mobile_stored" "\"$MOBILE_AUDIENCE\"" \
  "$MOBILE_CLIENT_ID audience did not round-trip"

# ---- First-party observability client (Grafana at obs.bex.co, ADR088 §3) ----
# Grafana signs in against the platform issuer as one more first-party client
# (docs/ADR088-platform-observability-ui.md): confidential, client_secret_basic
# (Grafana generic_oauth authenticates with HTTP basic by default), the exact
# generic_oauth callback, and skip_consent=true riding every upsert (the
# headless trusted path through the consent acceptor — the ops-workspace gate
# still runs before any accept). Scope is identity-only (openid profile email)
# and there is deliberately NO audience — unlike bex-mobile above — so a
# Grafana token carries zero bex-api authority. The secret is never generated
# or defaulted here: when BEX_OBS_OAUTH_CLIENT_SECRET is missing, provisioning
# this one client is refused and every client seeded above stands untouched.
if [ -z "$obs_secret" ]; then
  echo "refusing to provision OAuth2 client $OBS_CLIENT_ID: BEX_OBS_OAUTH_CLIENT_SECRET is missing or empty (.env or environment); other clients were provisioned normally" >&2
else
  obs_body="$(printf '{"client_id":"%s","client_name":"bex observability (Grafana)","client_secret":"%s","grant_types":["authorization_code","refresh_token"],"response_types":["code"],"redirect_uris":["%s"],"scope":"openid profile email","token_endpoint_auth_method":"client_secret_basic","subject_type":"public","skip_consent":true}' \
    "$OBS_CLIENT_ID" "$obs_secret" "$OBS_REDIRECT_URI")"
  upsert_client "$OBS_CLIENT_ID" "$obs_body"

  # The identity-only posture is load-bearing: empty audience (Hydra always
  # stores the key) means an obs token carries zero bex-api authority.
  obs_stored="$(read_client "$OBS_CLIENT_ID")"
  assert_stored "$obs_stored" "\"$OBS_REDIRECT_URI\"" \
    "$OBS_CLIENT_ID redirect URI did not round-trip"
  assert_stored "$obs_stored" '"token_endpoint_auth_method":"client_secret_basic"' \
    "$OBS_CLIENT_ID is not a client_secret_basic confidential client"
  assert_stored "$obs_stored" '"audience":[]' \
    "$OBS_CLIENT_ID audience is not empty — an obs token must carry zero bex-api authority"
fi

echo "token: POST <hydra-public>/oauth2/token  grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=***"
