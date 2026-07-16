#!/usr/bin/env bash
# Seed both permanent platform OAuth2 clients: the confidential `bex-bootstrap`
# machine client and the secretless public client hard-coded by the official
# Render CLI. Neither is a tenant-created API key. Idempotent: re-running resets
# both clients to their intended grants without minting a Render CLI secret.
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
    # Fallback: port-forward with full diagnostic output captured.
    PF_LOG=$(mktemp)
    kubectl -n "$NS" port-forward service/hydra-admin 34445:4445 > "$PF_LOG" 2>&1 &
    PF_PID=$!
    admin=http://127.0.0.1:34445
    ready=0
    for _ in $(seq 1 45); do
      curl -sf -o /dev/null "$admin/health/ready" && ready=1 && break
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

  if exec_hydra update client "$RENDER_CLI_CLIENT_ID" \
      --endpoint "$admin" \
      --grant-type "$DEVICE_GRANT,refresh_token" \
      --scope "openid,offline_access" \
      --token-endpoint-auth-method none \
      --subject-type public \
      --metadata '{"bex.co/platform-client":true}' \
      --name "Render CLI (bex platform)" > /dev/null 2>&1; then
    echo "updated OAuth2 client $RENDER_CLI_CLIENT_ID"
  else
    exec_hydra create client \
      --endpoint "$admin" \
      --id "$RENDER_CLI_CLIENT_ID" \
      --grant-type "$DEVICE_GRANT,refresh_token" \
      --scope "openid,offline_access" \
      --token-endpoint-auth-method none \
      --subject-type public \
      --metadata '{"bex.co/platform-client":true}' \
      --name "Render CLI (bex platform)" > /dev/null
    echo "created OAuth2 client $RENDER_CLI_CLIENT_ID"
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

  render_body="$(printf '{"client_id":"%s","client_name":"Render CLI (bex platform)","grant_types":["%s","refresh_token"],"scope":"openid offline_access","token_endpoint_auth_method":"none","subject_type":"public","metadata":{"bex.co/platform-client":true}}' "$RENDER_CLI_CLIENT_ID" "$DEVICE_GRANT")"
  render_code="$(printf '%s' "$render_body" | curl -s -o /dev/null -w '%{http_code}' -X PUT \
    -H 'Content-Type: application/json' -d @- "$admin/admin/clients/$RENDER_CLI_CLIENT_ID")"
  if [ "$render_code" = "404" ]; then
    render_code="$(printf '%s' "$render_body" | curl -s -o /dev/null -w '%{http_code}' -X POST \
      -H 'Content-Type: application/json' -d @- "$admin/admin/clients")"
    [ "$render_code" = "201" ] || { echo "error: creating $RENDER_CLI_CLIENT_ID failed (HTTP $render_code)" >&2; exit 1; }
    echo "created OAuth2 client $RENDER_CLI_CLIENT_ID"
  elif [ "$render_code" = "200" ]; then
    echo "updated OAuth2 client $RENDER_CLI_CLIENT_ID"
  else
    echo "error: upserting $RENDER_CLI_CLIENT_ID failed (HTTP $render_code)" >&2
    exit 1
  fi
fi

echo "token: POST <hydra-public>/oauth2/token  grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=***"
