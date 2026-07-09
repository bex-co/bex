#!/usr/bin/env bash
# Seed the platform's bootstrap OAuth2 client, `bex-bootstrap` — the first API
# key (docs/auth.md). bex-api has no shared static token: every caller holds an
# OAuth2 client, and this is the client the platform operator + CI use to reach
# the API (and to mint further keys via /v1/api-keys). Idempotent: re-running
# resets the client to the configured secret/grants.
#
# The secret comes from BEX_BOOTSTRAP_CLIENT_SECRET in .env (gitignored) or the
# environment (CI: GitHub Actions secret). Values are never printed.
#
# Usage: scripts/auth-bootstrap-client.sh          # port-forwards hydra-admin (auth ns)
#        HYDRA_ADMIN_URL=http://... scripts/...    # use an already-reachable admin URL
# Requires: kubectl (unless HYDRA_ADMIN_URL is set), curl.
set -euo pipefail
cd "$(dirname "$0")/.."

CLIENT_ID=bex-bootstrap
NS=auth
PF_PID=""

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
}
trap cleanup EXIT

admin="${HYDRA_ADMIN_URL:-}"
if [ -z "$admin" ]; then
  kubectl -n "$NS" port-forward service/hydra-admin 34445:4445 >/dev/null 2>&1 &
  PF_PID=$!
  admin=http://127.0.0.1:34445
  ready=0
  for _ in $(seq 1 45); do
    curl -sf -o /dev/null "$admin/health/ready" && ready=1 && break
    sleep 2
  done
  [ "$ready" = "1" ] || { echo "error: hydra admin port-forward not ready after 90s — is the port-forward established?" >&2; exit 1; }
fi

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
echo "token: POST <hydra-public>/oauth2/token  grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=***"
