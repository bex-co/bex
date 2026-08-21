#!/usr/bin/env bash
# Seed the permanent platform OAuth2 clients: confidential `bex-bootstrap`, the
# secretless public client hard-coded by the official Render CLI, and the
# secretless first-party `bex-mobile` native client. None is a tenant-created
# API key. Idempotent: re-running resets every client to its intended grants
# without minting a public-client secret.
#
# The public clients are always upserted through the admin REST API (curl),
# never the in-pod `hydra` CLI: they carry fields the CLI cannot express, and
# `hydra update client` PUTs a full replacement that would silently wipe them.
#
# The secret comes from BEX_BOOTSTRAP_CLIENT_SECRET in .env (gitignored) or the
# environment (CI: GitHub Actions secret). Values are never printed.
#
# Usage: scripts/auth-bootstrap-client.sh          # kubectl exec into hydra pod
#        HYDRA_ADMIN_URL=http://... scripts/...    # use an already-reachable admin URL
#        BEX_AUTH_NAMESPACE=dev-3-auth scripts/...  # non-default auth namespace
# Requires: kubectl, curl (for the port-forward fallback path).
set -euo pipefail
cd "$(dirname "$0")/.."

CLIENT_ID=bex-bootstrap
RENDER_CLI_CLIENT_ID=429024F5E608930E2A65EF92591A25CC
MOBILE_CLIENT_ID=bex-mobile
# ACCEPTED-RISK: private-use custom-scheme callback (RFC 8252, single slash), not
# an https universal link. See mobile/src/features/auth/config.ts and ADR012 §
# "Mobile OAuth redirect (accepted risk)". Do not switch back to https without
# completing AASA/assetlinks + a dashboard /oauth2redirect bridge, or fresh
# logins 404 (regression from commit 9081fbdb).
MOBILE_REDIRECT_URI=co.bex.mobile:/oauth2redirect
MOBILE_AUDIENCE="${BEX_OAUTH_RESOURCE:-https://api.bex.co/mcp}"
DEVICE_GRANT=urn:ietf:params:oauth:grant-type:device_code
NS="${BEX_AUTH_NAMESPACE:-auth}"
PF_PID=""
PF_LOG=""

# An explicitly-set env var wins; .env is only consulted as the local fallback
# (CI passes the secret directly and has no .env).
if [ -z "${BEX_BOOTSTRAP_CLIENT_SECRET:-}" ] && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
secret="${BEX_BOOTSTRAP_CLIENT_SECRET:-}"
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
render_body="$(printf '{"client_id":"%s","client_name":"bex CLI","grant_types":["%s","refresh_token"],"scope":"openid offline_access bex.read bex.write bex.sensitive","token_endpoint_auth_method":"none","subject_type":"public","skip_consent":true,"metadata":{"bex.co/platform-client":true},"device_authorization_grant_access_token_lifespan":"%s","refresh_token_grant_access_token_lifespan":"%s"}' \
  "$RENDER_CLI_CLIENT_ID" "$DEVICE_GRANT" "$CLI_TOKEN_LIFESPAN" "$CLI_TOKEN_LIFESPAN")"
render_code="$(printf '%s' "$render_body" | curl -s -o /dev/null -w '%{http_code}' -X PUT \
  -H 'Content-Type: application/json' -d @- "$REST_ADMIN/admin/clients/$RENDER_CLI_CLIENT_ID")"
if [ "$render_code" = "404" ]; then
  render_code="$(printf '%s' "$render_body" | curl -s -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' -d @- "$REST_ADMIN/admin/clients")"
  [ "$render_code" = "201" ] || { echo "error: creating $RENDER_CLI_CLIENT_ID failed (HTTP $render_code)" >&2; exit 1; }
  echo "created OAuth2 client $RENDER_CLI_CLIENT_ID"
elif [ "$render_code" = "200" ]; then
  echo "updated OAuth2 client $RENDER_CLI_CLIENT_ID"
else
  echo "error: upserting $RENDER_CLI_CLIENT_ID failed (HTTP $render_code)" >&2
  exit 1
fi

# Guard against silent field drops (a typo'd lifespan key would be ignored, not
# rejected): assert BOTH grant lifespans round-trip on the stored client. Prefix
# match, not exact — Hydra may normalize the duration (168h vs 168h0m0s).
stored_render_client="$(curl -sf "$REST_ADMIN/admin/clients/$RENDER_CLI_CLIENT_ID")" || {
  echo "error: reading back $RENDER_CLI_CLIENT_ID failed" >&2
  exit 1
}
for lifespan_field in \
  device_authorization_grant_access_token_lifespan \
  refresh_token_grant_access_token_lifespan; do
  if ! printf '%s' "$stored_render_client" \
      | grep -q "\"$lifespan_field\":\"$CLI_TOKEN_LIFESPAN"; then
    echo "error: $RENDER_CLI_CLIENT_ID $lifespan_field did not round-trip (hydra too old for per-client lifespans?)" >&2
    exit 1
  fi
done

# ---- First-party native mobile client (ADR012 §8b) -------------------------
# A store-distributed app cannot keep a client secret. The reverse-domain
# private-use redirect is exact and single-slash per RFC 8252 (ACCEPTED-RISK:
# not an https universal link — see MOBILE_REDIRECT_URI note above); PKCE S256
# is required on every authorization. skip_consent=true: it is a first-party
# app and the token still requires granular capabilities at bex-api.
mobile_body="$(printf '{"client_id":"%s","client_name":"bex mobile (first-party native)","grant_types":["authorization_code","refresh_token"],"response_types":["code"],"redirect_uris":["%s"],"audience":["%s"],"scope":"openid offline_access bex.read bex.write","token_endpoint_auth_method":"none","subject_type":"public","skip_consent":true,"metadata":{"bex.co/platform-client":true}}' \
  "$MOBILE_CLIENT_ID" "$MOBILE_REDIRECT_URI" "$MOBILE_AUDIENCE")"
mobile_code="$(printf '%s' "$mobile_body" | curl -s -o /dev/null -w '%{http_code}' -X PUT \
  -H 'Content-Type: application/json' -d @- "$REST_ADMIN/admin/clients/$MOBILE_CLIENT_ID")"
if [ "$mobile_code" = "404" ]; then
  mobile_code="$(printf '%s' "$mobile_body" | curl -s -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' -d @- "$REST_ADMIN/admin/clients")"
  [ "$mobile_code" = "201" ] || { echo "error: creating $MOBILE_CLIENT_ID failed (HTTP $mobile_code)" >&2; exit 1; }
  echo "created OAuth2 client $MOBILE_CLIENT_ID"
elif [ "$mobile_code" = "200" ]; then
  echo "updated OAuth2 client $MOBILE_CLIENT_ID"
else
  echo "error: upserting $MOBILE_CLIENT_ID failed (HTTP $mobile_code)" >&2
  exit 1
fi

# Hydra ignores unknown JSON fields, so assert the redirect and public-client
# posture round-trip instead of trusting the PUT status alone.
mobile_stored="$(curl -sf "$REST_ADMIN/admin/clients/$MOBILE_CLIENT_ID")"
printf '%s' "$mobile_stored" | grep -Fq "\"$MOBILE_REDIRECT_URI\"" || {
  echo "error: $MOBILE_CLIENT_ID redirect URI did not round-trip" >&2
  exit 1
}
printf '%s' "$mobile_stored" | grep -Fq '"token_endpoint_auth_method":"none"' || {
  echo "error: $MOBILE_CLIENT_ID is not a public client" >&2
  exit 1
}
printf '%s' "$mobile_stored" | grep -Fq "\"$MOBILE_AUDIENCE\"" || {
  echo "error: $MOBILE_CLIENT_ID audience did not round-trip" >&2
  exit 1
}

echo "token: POST <hydra-public>/oauth2/token  grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=***"
