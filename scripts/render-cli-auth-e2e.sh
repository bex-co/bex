#!/usr/bin/env bash
# End-to-end proof for the official, unmodified Render CLI's browser login.
#
# The CLI itself initiates and polls the device grant. A real Chrome process
# completes Kratos login + Hydra consent. The script then proves human tenancy,
# forced refresh-token rotation, per-user logout, second-user isolation, shared
# public-client preservation, and Settings/API-key-list separation.
#
# Local dev-3 example:
#   CREATE_TEST_IDENTITIES=1 \
#   KRATOS_ADMIN_URL=http://localhost:57030 \
#   BEX_API_URL=http://localhost:54030 \
#   HYDRA_ADMIN_URL=http://localhost:52030 \
#   scripts/render-cli-auth-e2e.sh
#
# Production requires two disposable human accounts supplied out of band:
#   CLI_USER_A_EMAIL=... CLI_USER_A_PASSWORD=... \
#   CLI_USER_B_EMAIL=... CLI_USER_B_PASSWORD=... \
#   BEX_API_URL=https://api.bex.co HYDRA_ADMIN_URL=<private-admin-forward> \
#   scripts/render-cli-auth-e2e.sh
#
# Credentials and tokens are never printed. playwright-core is installed into
# the throwaway directory unless PLAYWRIGHT_NODE_MODULES points at an existing
# installation. CHROME_BIN overrides the browser executable when needed.
set -euo pipefail
cd "$(dirname "$0")/.."

readonly EXPECTED_CLI_COMMIT=c23438e
readonly RENDER_CLI_CLIENT_ID=429024F5E608930E2A65EF92591A25CC

BEX_API_URL="${BEX_API_URL:-http://localhost:54030}"
HYDRA_ADMIN_URL="${HYDRA_ADMIN_URL:-http://localhost:52030}"
KRATOS_ADMIN_URL="${KRATOS_ADMIN_URL:-http://localhost:57030}"
CLI_USER_A_EMAIL="${CLI_USER_A_EMAIL:-render-cli-a@bex.test}"
CLI_USER_A_PASSWORD="${CLI_USER_A_PASSWORD:-render-cli-a-password-123!}"
CLI_USER_B_EMAIL="${CLI_USER_B_EMAIL:-render-cli-b@bex.test}"
CLI_USER_B_PASSWORD="${CLI_USER_B_PASSWORD:-render-cli-b-password-123!}"
RENDER_BIN="${RENDER_BIN:-}"
TMP="$(mktemp -d)"
LOGIN_PIDS=()

cleanup() {
  local pid
  for pid in "${LOGIN_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
  rm -rf "$TMP"
}
trap cleanup EXIT

fail() {
  echo "error: $*" >&2
  exit 1
}

wait_for_file() {
  local path="$1"
  for _ in $(seq 1 60); do
    [ -s "$path" ] && return 0
    sleep 1
  done
  fail "timed out waiting for $path"
}

create_identity() {
  local email="$1" password="$2" existing body
  existing="$(curl -sf "$KRATOS_ADMIN_URL/admin/identities?credentials_identifier=$email" | jq -r '.[0].id // empty')"
  if [ -n "$existing" ]; then
    curl -sf -X DELETE "$KRATOS_ADMIN_URL/admin/identities/$existing" >/dev/null
  fi
  body="$(jq -nc --arg email "$email" --arg password "$password" '{schema_id:"default",traits:{email:$email},credentials:{password:{config:{password:$password}}}}')"
  curl -sf -X POST -H 'Content-Type: application/json' -d "$body" \
    "$KRATOS_ADMIN_URL/admin/identities" | jq -e '.id' >/dev/null
}

is_local_url() {
  case "$1" in
    http://localhost:* | http://127.0.0.1:*) return 0 ;;
    *) return 1 ;;
  esac
}

if [ -z "$RENDER_BIN" ]; then
  [ -d cli ] || fail "./cli checkout is missing"
  actual_commit="$(git -C cli rev-parse --short HEAD)"
  [ "$actual_commit" = "$EXPECTED_CLI_COMMIT" ] || fail "official CLI is $actual_commit, want $EXPECTED_CLI_COMMIT"
  RENDER_BIN="$TMP/render"
  (cd cli && go build -o "$RENDER_BIN" .)
fi
RENDER_BIN="$(cd "$(dirname "$RENDER_BIN")" && pwd)/$(basename "$RENDER_BIN")"
[ -x "$RENDER_BIN" ] || fail "RENDER_BIN is not executable"

if [ "${CREATE_TEST_IDENTITIES:-0}" = "1" ]; then
  is_local_url "$KRATOS_ADMIN_URL" && is_local_url "$BEX_API_URL" \
    || fail "CREATE_TEST_IDENTITIES requires both Kratos admin and bex-api to be local"
  create_identity "$CLI_USER_A_EMAIL" "$CLI_USER_A_PASSWORD"
  create_identity "$CLI_USER_B_EMAIL" "$CLI_USER_B_PASSWORD"
fi

PLAYWRIGHT_NODE_MODULES="${PLAYWRIGHT_NODE_MODULES:-$TMP/browser/node_modules}"
if [ ! -d "$PLAYWRIGHT_NODE_MODULES/playwright-core" ]; then
  npm install --prefix "$TMP/browser" --no-save --no-package-lock \
    playwright-core@1.61.1 >/dev/null
fi

# Keep the unmodified CLI's browser-open call successful while the verifier
# launches its own isolated Chrome context with the printed URL.
mkdir -p "$TMP/bin"
for opener in open xdg-open rundll32; do
  printf '#!/usr/bin/env sh\nexit 0\n' >"$TMP/bin/$opener"
  chmod +x "$TMP/bin/$opener"
done

login_user() { # label email password config-path
  local label="$1" email="$2" password="$3" config="$4"
  local log="$TMP/login-$label.log" status="$TMP/login-$label.status" pid url
  (
    set +e
    PATH="$TMP/bin:$PATH" env -u RENDER_API_KEY \
      RENDER_HOST="${BEX_API_URL%/}/v1/" \
      RENDER_CLI_CONFIG_PATH="$config" \
      "$RENDER_BIN" login >"$log" 2>&1
    printf '%s\n' "$?" >"$status"
  ) &
  pid=$!
  LOGIN_PIDS+=("$pid")

  for _ in $(seq 1 30); do
    url="$(rg -o 'https?://[^[:space:]]+/oauth2/device/verify\?user_code=[^[:space:]]+' "$log" 2>/dev/null | head -1 || true)"
    [ -n "$url" ] && break
    kill -0 "$pid" 2>/dev/null || { sed -n '1,80p' "$log" >&2; fail "CLI login $label exited before opening the browser"; }
    sleep 1
  done
  [ -n "${url:-}" ] || fail "CLI login $label did not print a device verification URL"

  NODE_PATH="$PLAYWRIGHT_NODE_MODULES" node scripts/render-cli-auth-browser.cjs \
    "$url" "$email" "$password" >/dev/null
  wait_for_file "$status"
  wait "$pid" || true
  if [ "$(cat "$status")" != "0" ]; then
    sed -n '1,80p' "$log" >&2
    fail "CLI login $label failed"
  fi
  grep -q 'Login successful! CLI token saved.' "$log" || fail "CLI login $label did not complete"
  [ "$(yq -r '.api.key | length' "$config")" -gt 20 ] || fail "CLI login $label stored no access token"
  [ "$(yq -r '.api.refreshtoken | length' "$config")" -gt 20 ] || fail "CLI login $label stored no refresh token"
}

token_hash() { # config field
  yq -r "$2" "$1" | shasum -a 256 | cut -d' ' -f1
}

echo "-> official CLI $EXPECTED_CLI_COMMIT: user A browser login"
CFG_A="$TMP/user-a.yaml"
login_user a "$CLI_USER_A_EMAIL" "$CLI_USER_A_PASSWORD" "$CFG_A"

env -u RENDER_API_KEY RENDER_HOST="${BEX_API_URL%/}/v1/" RENDER_CLI_CONFIG_PATH="$CFG_A" \
  "$RENDER_BIN" workspaces -o json >"$TMP/workspaces-a.json"
jq -e 'length == 1 and (.[0].id | startswith("tea-"))' "$TMP/workspaces-a.json" >/dev/null
TENANT_A="$(jq -r '.[0].id' "$TMP/workspaces-a.json")"

env -u RENDER_API_KEY RENDER_HOST="${BEX_API_URL%/}/v1/" RENDER_CLI_CONFIG_PATH="$CFG_A" RENDER_WORKSPACE="$TENANT_A" \
  "$RENDER_BIN" services -o json >"$TMP/services-a.json"
jq -e '. == null or type == "array"' "$TMP/services-a.json" >/dev/null

echo "-> force stored expiry; prove automatic access+refresh rotation"
old_access_hash="$(token_hash "$CFG_A" '.api.key')"
old_refresh_hash="$(token_hash "$CFG_A" '.api.refreshtoken')"
yq -i '.api.expires_at = 1' "$CFG_A"
env -u RENDER_API_KEY RENDER_HOST="${BEX_API_URL%/}/v1/" RENDER_CLI_CONFIG_PATH="$CFG_A" RENDER_WORKSPACE="$TENANT_A" \
  "$RENDER_BIN" services -o json >"$TMP/services-a-refreshed.json"
[ "$old_access_hash" != "$(token_hash "$CFG_A" '.api.key')" ] || fail "access token did not rotate"
[ "$old_refresh_hash" != "$(token_hash "$CFG_A" '.api.refreshtoken')" ] || fail "refresh token did not rotate"
[ "$(yq -r '.api.expires_at' "$CFG_A")" -gt "$(date +%s)" ] || fail "refreshed expiry is not in the future"

echo "-> user B browser login and isolated human workspace"
CFG_B="$TMP/user-b.yaml"
login_user b "$CLI_USER_B_EMAIL" "$CLI_USER_B_PASSWORD" "$CFG_B"
env -u RENDER_API_KEY RENDER_HOST="${BEX_API_URL%/}/v1/" RENDER_CLI_CONFIG_PATH="$CFG_B" \
  "$RENDER_BIN" workspaces -o json >"$TMP/workspaces-b.json"
jq -e 'length == 1 and (.[0].id | startswith("tea-"))' "$TMP/workspaces-b.json" >/dev/null
TENANT_B="$(jq -r '.[0].id' "$TMP/workspaces-b.json")"
[ "$TENANT_A" != "$TENANT_B" ] || fail "two human identities resolved to the same personal workspace"

ACCESS_A="$(yq -r '.api.key' "$CFG_A")"
REFRESH_A="$(yq -r '.api.refreshtoken' "$CFG_A")"
ACCESS_B="$(yq -r '.api.key' "$CFG_B")"

echo "-> user A logout revokes access+refresh without affecting user B/client"
env -u RENDER_API_KEY RENDER_HOST="${BEX_API_URL%/}/v1/" RENDER_CLI_CONFIG_PATH="$CFG_A" \
  "$RENDER_BIN" logout -o text >"$TMP/logout-a.txt"
grep -q 'Successfully logged out' "$TMP/logout-a.txt" || fail "CLI logout did not report success"
[ ! -e "$CFG_A" ] || fail "CLI logout left the local config behind"

# Hydra revokes the token immediately; each bex-api replica may serve its
# cached positive introspection for up to core.PositiveTTL (30s), and only the
# replica that handled the logout invalidates its cache entry synchronously —
# so against a multi-replica deployment the 401 can lag by one cache window.
status=""
for _ in $(seq 1 40); do
  status="$(curl -s -o "$TMP/old-access.json" -w '%{http_code}' \
    -H "Authorization: Bearer $ACCESS_A" "${BEX_API_URL%/}/v1/services")"
  [ "$status" = 401 ] && break
  sleep 1
done
[ "$status" = 401 ] || fail "logged-out access token returned HTTP $status, want 401 (waited past the introspection-cache TTL)"

refresh_body="$(jq -nc --arg token "$REFRESH_A" '{grant_type:"refresh_token",refresh_token:$token}')"
status="$(curl -s -o "$TMP/old-refresh.json" -w '%{http_code}' -X POST \
  -H 'Content-Type: application/x-www-form-urlencoded' --data-binary "$refresh_body" \
  "${BEX_API_URL%/}/v1/token/refresh/")"
[ "$status" = 400 ] || fail "logged-out refresh token returned HTTP $status, want 400"
jq -e '.error == "invalid_grant"' "$TMP/old-refresh.json" >/dev/null

env -u RENDER_API_KEY RENDER_HOST="${BEX_API_URL%/}/v1/" RENDER_CLI_CONFIG_PATH="$CFG_B" RENDER_WORKSPACE="$TENANT_B" \
  "$RENDER_BIN" services -o json >"$TMP/services-b-after-a-logout.json"
jq -e '. == null or type == "array"' "$TMP/services-b-after-a-logout.json" >/dev/null

status="$(curl -s -o "$TMP/client.json" -w '%{http_code}' \
  "$HYDRA_ADMIN_URL/admin/clients/$RENDER_CLI_CLIENT_ID")"
[ "$status" = 200 ] || fail "shared Render CLI client disappeared (HTTP $status)"
jq -e --arg id "$RENDER_CLI_CLIENT_ID" '
  .client_id == $id and
  .token_endpoint_auth_method == "none" and
  .subject_type == "public" and
  .client_secret == null and
  (.grant_types | index("urn:ietf:params:oauth:grant-type:device_code")) != null and
  (.grant_types | index("refresh_token")) != null
' "$TMP/client.json" >/dev/null

status="$(curl -s -o "$TMP/api-keys.json" -w '%{http_code}' \
  -H "Authorization: Bearer $ACCESS_B" "${BEX_API_URL%/}/v1/api-keys")"
[ "$status" = 200 ] || fail "API-key list returned HTTP $status"
jq -e --arg id "$RENDER_CLI_CLIENT_ID" 'all(.[]; .id != $id)' "$TMP/api-keys.json" >/dev/null

echo "✓ official CLI browser login, human tenancy, forced refresh, scoped logout,"
echo "  two-user isolation, shared secretless client, and API-key-list separation"
