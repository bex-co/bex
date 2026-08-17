#!/usr/bin/env bash
# Live device-flow acceptance for the imported bex executable. It deliberately
# supplies only BEX_* inputs, so a successful run proves the bridge defaults
# the upstream CLI to ~/.bex/cli.yaml rather than ~/.render/cli.yaml.
#
# Local example:
#   CREATE_TEST_IDENTITY=1 scripts/bex-cli-auth-e2e.sh
#
# Production callers must provide a disposable human identity out of band:
#   CLI_USER_EMAIL=... CLI_USER_PASSWORD=... BEX_API_URL=https://api.bex.co \
#   HYDRA_ADMIN_URL=<private-admin-forward> scripts/bex-cli-auth-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."

BEX_API_URL="${BEX_API_URL:-http://localhost:54030}"
HYDRA_ADMIN_URL="${HYDRA_ADMIN_URL:-http://localhost:52030}"
KRATOS_ADMIN_URL="${KRATOS_ADMIN_URL:-http://localhost:57030}"
CLI_USER_EMAIL="${CLI_USER_EMAIL:-bex-cli@bex.test}"
CLI_USER_PASSWORD="${CLI_USER_PASSWORD:-bex-cli-password-123!}"
BEX_BIN="${BEX_BIN:-}"
TMP="$(mktemp -d)"
LOGIN_PID=""

cleanup() {
  if [ -n "$LOGIN_PID" ]; then
    kill "$LOGIN_PID" 2>/dev/null || true
    wait "$LOGIN_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

fail() {
  echo "error: $*" >&2
  exit 1
}

is_local_url() {
  case "$1" in
    http://localhost:* | http://127.0.0.1:*) return 0 ;;
    *) return 1 ;;
  esac
}

wait_for_file() {
  local path="$1"
  for _ in $(seq 1 60); do
    [ -s "$path" ] && return 0
    sleep 1
  done
  fail "timed out waiting for $path"
}

file_mode() {
  case "$(uname -s)" in
    Darwin) stat -f '%Lp' "$1" ;;
    *) stat -c '%a' "$1" ;;
  esac
}

create_identity() {
  local existing body
  existing="$(curl -sf "$KRATOS_ADMIN_URL/admin/identities?credentials_identifier=$CLI_USER_EMAIL" | jq -r '.[0].id // empty')"
  if [ -n "$existing" ]; then
    curl -sf -X DELETE "$KRATOS_ADMIN_URL/admin/identities/$existing" >/dev/null
  fi
  body="$(jq -nc --arg email "$CLI_USER_EMAIL" --arg password "$CLI_USER_PASSWORD" '{schema_id:"default",traits:{email:$email},credentials:{password:{config:{password:$password}}}}')"
  curl -sf -X POST -H 'Content-Type: application/json' -d "$body" \
    "$KRATOS_ADMIN_URL/admin/identities" | jq -e '.id' >/dev/null
}

if [ -z "$BEX_BIN" ]; then
  BEX_BIN="$TMP/bex"
  (cd lego/cli && go build -o "$BEX_BIN" .)
fi
BEX_BIN="$(cd "$(dirname "$BEX_BIN")" && pwd)/$(basename "$BEX_BIN")"
[ -x "$BEX_BIN" ] || fail "BEX_BIN is not executable"

if [ "${CREATE_TEST_IDENTITY:-0}" = "1" ]; then
  is_local_url "$KRATOS_ADMIN_URL" && is_local_url "$BEX_API_URL" \
    || fail "CREATE_TEST_IDENTITY requires local Kratos admin and Bex API URLs"
  create_identity
fi

PLAYWRIGHT_NODE_MODULES="${PLAYWRIGHT_NODE_MODULES:-$TMP/browser/node_modules}"
if [ ! -d "$PLAYWRIGHT_NODE_MODULES/playwright-core" ]; then
  npm install --prefix "$TMP/browser" --no-save --no-package-lock \
    playwright-core@1.61.1 >/dev/null
fi

mkdir -p "$TMP/bin" "$TMP/home/.render"
printf 'render-state-must-not-change\n' >"$TMP/home/.render/cli.yaml"
for opener in open xdg-open rundll32; do
  printf '#!/usr/bin/env sh\nexit 0\n' >"$TMP/bin/$opener"
  chmod +x "$TMP/bin/$opener"
done

run_bex() { # workspace command...
  local workspace="$1"
  shift
  PATH="$TMP/bin:$PATH" env \
    -u RENDER_API_KEY -u RENDER_HOST -u RENDER_CLI_CONFIG_PATH -u RENDER_WORKSPACE -u RENDER_OUTPUT \
    -u BEX_CLI_CONFIG_DIR -u BEX_CLI_CONFIG_PATH -u BEX_ACCESS_TOKEN \
    HOME="$TMP/home" \
    BEX_HOST="${BEX_API_URL%/}/v1/" \
    BEX_WORKSPACE="$workspace" \
    "$BEX_BIN" "$@"
}

login_log="$TMP/login.log"
login_status="$TMP/login.status"
(
  set +e
  run_bex "" login >"$login_log" 2>&1
  printf '%s\n' "$?" >"$login_status"
) &
LOGIN_PID=$!

verification_url=""
for _ in $(seq 1 30); do
  verification_url="$(rg -o 'https?://[^[:space:]]+/oauth2/device/verify\?user_code=[^[:space:]]+' "$login_log" 2>/dev/null | head -1 || true)"
  [ -n "$verification_url" ] && break
  kill -0 "$LOGIN_PID" 2>/dev/null || { sed -n '1,80p' "$login_log" >&2; fail "bex login exited before the browser URL"; }
  sleep 1
done
[ -n "$verification_url" ] || fail "bex login did not print a device verification URL"

browser_log="$TMP/browser.log"
if ! printf '%s\0%s\0' "$CLI_USER_EMAIL" "$CLI_USER_PASSWORD" | \
  NODE_PATH="$PLAYWRIGHT_NODE_MODULES" node scripts/render-cli-auth-browser.cjs \
    "$verification_url" >"$browser_log" 2>&1; then
  # Browser diagnostics redact query strings and page text, and stay in the
  # private throwaway directory unless explicitly copied on failure.
  if [ -n "${BEX_CLI_E2E_DIAGNOSTIC_LOG:-}" ]; then
    install -m 600 "$browser_log" "$BEX_CLI_E2E_DIAGNOSTIC_LOG"
  fi
  sed -n '1,120p' "$browser_log" >&2
  fail "browser device verification failed"
fi
wait_for_file "$login_status"
wait "$LOGIN_PID" || true
LOGIN_PID=""
[ "$(cat "$login_status")" = "0" ] || { sed -n '1,80p' "$login_log" >&2; fail "bex login failed"; }
grep -q 'Login successful! CLI token saved.' "$login_log" || fail "bex login did not complete"

CFG="$TMP/home/.bex/cli.yaml"
[ -f "$CFG" ] || fail "bex login did not create ~/.bex/cli.yaml"
[ "$(file_mode "$CFG")" = 600 ] || fail "Bex config permissions are $(file_mode "$CFG"), want 600"
grep -q '^render-state-must-not-change$' "$TMP/home/.render/cli.yaml" \
  || fail "bex login mutated ~/.render/cli.yaml"
[ "$(yq -r '.api.key | length' "$CFG")" -gt 20 ] || fail "Bex config stored no access token"
# Render CLI's upstream YAML struct uses `json:"refresh_token"` rather than a
# YAML tag, so yaml.v3 persists the Go field as `refreshtoken`.
[ "$(yq -r '.api.refreshtoken | length' "$CFG")" -gt 20 ] || fail "Bex config stored no refresh token"

run_bex "" workspaces -o json >"$TMP/workspaces.json"
workspace="$(jq -er '.[0].id | select(startswith("tea-"))' "$TMP/workspaces.json")"

access_before="$(yq -r '.api.key' "$CFG" | shasum -a 256 | cut -d' ' -f1)"
refresh_before="$(yq -r '.api.refreshtoken' "$CFG" | shasum -a 256 | cut -d' ' -f1)"
yq -i '.api.expires_at = 1' "$CFG"
run_bex "$workspace" services -o json >"$TMP/services.json"
[ "$access_before" != "$(yq -r '.api.key' "$CFG" | shasum -a 256 | cut -d' ' -f1)" ] \
  || fail "access token did not rotate"
[ "$refresh_before" != "$(yq -r '.api.refreshtoken' "$CFG" | shasum -a 256 | cut -d' ' -f1)" ] \
  || fail "refresh token did not rotate"

access_token="$(yq -r '.api.key' "$CFG")"
refresh_token="$(yq -r '.api.refreshtoken' "$CFG")"
run_bex "" logout -o text >"$TMP/logout.txt"
grep -q 'Successfully logged out' "$TMP/logout.txt" || fail "bex logout did not report success"
[ ! -e "$CFG" ] || fail "bex logout left ~/.bex/cli.yaml behind"

status=""
for _ in $(seq 1 40); do
  status="$(curl -s -o "$TMP/revoked.json" -w '%{http_code}' \
    -H "Authorization: Bearer $access_token" "${BEX_API_URL%/}/v1/services")"
  [ "$status" = 401 ] && break
  sleep 1
done
[ "$status" = 401 ] || fail "logged-out Bex bearer returned HTTP $status, want 401"

# The same Bex logout must revoke the refresh-token session, not just evict the
# access-token introspection cache. Keep the response in the throwaway file;
# neither token is printed.
refresh_body="$(jq -nc --arg token "$refresh_token" '{grant_type:"refresh_token",refresh_token:$token}')"
status="$(curl -s -o "$TMP/revoked-refresh.json" -w '%{http_code}' -X POST \
  -H 'Content-Type: application/x-www-form-urlencoded' --data-binary "$refresh_body" \
  "${BEX_API_URL%/}/v1/token/refresh/")"
[ "$status" = 400 ] || fail "logged-out Bex refresh token returned HTTP $status, want 400"
jq -e '.error == "invalid_grant"' "$TMP/revoked-refresh.json" >/dev/null

grep -q '^render-state-must-not-change$' "$TMP/home/.render/cli.yaml" \
  || fail "bex logout mutated ~/.render/cli.yaml"

echo "✓ bex browser login, Bex config isolation, refresh rotation, and logout revocation"
