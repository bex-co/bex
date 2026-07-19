#!/usr/bin/env bash
set -euo pipefail

# Opt-in full-edge acceptance for the official Render CLI's interactive-only
# `kv-cli` command. scripts/cli-compat.sh owns authentication and invokes this
# file after exporting the official CLI environment. This verifier creates one
# source-restricted public fixture and removes only the returned opaque id.

fail() {
  echo "FAIL $*" >&2
  exit 1
}

for command in awk curl dig go jq nc redis-cli sed; do
  command -v "$command" >/dev/null || fail "missing required command: $command"
done

: "${RENDER_HOST:?run through scripts/cli-compat.sh kv-cli-verify}"
: "${RENDER_API_KEY:?run through scripts/cli-compat.sh kv-cli-verify}"
: "${RENDER_WORKSPACE:?the CLI compatibility key must select a workspace}"
: "${RENDER_CLI_CONFIG_PATH:?run through scripts/cli-compat.sh kv-cli-verify}"
: "${RENDER_BIN:?run through scripts/cli-compat.sh kv-cli-verify}"
: "${BEX_KV_VERIFY_ALLOW_CIDR:?set the verifier host explicit source CIDR}"

[[ "$RENDER_HOST" == https://*/v1/ ]] || fail "RENDER_HOST must be a public HTTPS /v1/ origin"
[[ "$BEX_KV_VERIFY_ALLOW_CIDR" =~ ^[^,[:space:]]+/[0-9]+$ ]] ||
  fail "BEX_KV_VERIFY_ALLOW_CIDR must be one explicit CIDR"
address_family=-4
if [[ "$BEX_KV_VERIFY_ALLOW_CIDR" == *:* ]]; then
  address_family=-6
fi
[[ -x "$RENDER_BIN" ]] || fail "official Render CLI is not executable"
[[ -d "$(dirname "$RENDER_CLI_CONFIG_PATH")" && -w "$(dirname "$RENDER_CLI_CONFIG_PATH")" ]] ||
  fail "RENDER_CLI_CONFIG_PATH parent must be writable"

cli_version="$("$RENDER_BIN" --version 2>&1 | sed -n '1p')"
[[ "$cli_version" == "render v2.21.0" ]] || fail "expected official Render CLI v2.21.0"
cli_module="$(go version -m "$RENDER_BIN" 2>/dev/null | awk '$1 == "path" { print $2 }')"
[[ "$cli_module" == "github.com/render-oss/cli" ]] || fail "Render CLI binary is not built from the official upstream module"
redis-cli --version >/dev/null 2>&1 || fail "redis-cli is not runnable"

api="${RENDER_HOST%/}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ready_timeout="${BEX_KV_VERIFY_READY_TIMEOUT_SECONDS:-300}"
cleanup_timeout="${BEX_KV_VERIFY_CLEANUP_TIMEOUT_SECONDS:-60}"
[[ "$ready_timeout" =~ ^[1-9][0-9]*$ ]] || fail "BEX_KV_VERIFY_READY_TIMEOUT_SECONDS must be a positive integer"
[[ "$cleanup_timeout" =~ ^[1-9][0-9]*$ ]] || fail "BEX_KV_VERIFY_CLEANUP_TIMEOUT_SECONDS must be a positive integer"

auth=(-H "Authorization: Bearer $RENDER_API_KEY")

api_json() {
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -fsS --connect-timeout 10 --max-time 30 -X "$method" "${auth[@]}" \
      -H 'Content-Type: application/json' -d "$body" "$api$path"
  else
    curl -fsS --connect-timeout 10 --max-time 30 -X "$method" "${auth[@]}" "$api$path"
  fi
}

# Authenticate and validate the selected workspace before the first mutation.
api_json GET /key-value >/dev/null || fail "API/workspace preflight failed"

fixture_id=""
fixture_created=0
cleanup_complete=0

cleanup() {
  local primary_status=$? delete_status=0 poll_status="" deadline
  trap - EXIT INT TERM
  if [[ "$fixture_created" == "1" && -n "$fixture_id" && "$cleanup_complete" != "1" ]]; then
    if ! api_json DELETE "/key-value/$fixture_id" >/dev/null 2>&1; then
      delete_status=1
    else
      deadline=$((SECONDS + cleanup_timeout))
      while [[ "$SECONDS" -lt "$deadline" ]]; do
        poll_status="$(curl -sS --connect-timeout 10 --max-time 30 -o /dev/null -w '%{http_code}' -X GET "${auth[@]}" "$api/key-value/$fixture_id" 2>/dev/null || true)"
        if [[ "$poll_status" == "404" ]]; then
          cleanup_complete=1
          break
        fi
        sleep 2
      done
    fi
    if [[ "$delete_status" != "0" || "$cleanup_complete" != "1" ]]; then
      echo "FAIL disposable Key Value cleanup" >&2
      exit 1
    fi
    echo "PASS disposable Key Value cleanup"
  fi
  exit "$primary_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

fixture_name="kvcli-m57-$(date +%s)-$$"
create_payload="$(jq -cn --arg name "$fixture_name" --arg cidr "$BEX_KV_VERIFY_ALLOW_CIDR" '{
  name:$name,
  plan:"free",
  public:true,
  ipAllowList:[{cidrBlock:$cidr,description:"automated kv-cli acceptance"}]
}')"
created="$(api_json POST /key-value "$create_payload")" || fail "disposable Key Value creation failed"
fixture_id="$(jq -er '.id | select(startswith("red-"))' <<<"$created")" || fail "create response omitted an opaque Key Value id"
fixture_created=1
created_name="$(jq -er '.name' <<<"$created")" || fail "create response omitted the display name"
[[ "$created_name" == "$fixture_name" ]] || fail "create response returned an unexpected display name"
created=""
create_payload=""
created_name=""
echo "PASS source-restricted public Key Value created"

deadline=$((SECONDS + ready_timeout))
ready=0
while ((SECONDS < deadline)); do
  if resource="$(api_json GET "/key-value/$fixture_id" 2>/dev/null)" &&
    [[ "$(jq -r '.status // empty' <<<"$resource")" == "available" ]] &&
    [[ -n "$(jq -r '.externalHost // empty' <<<"$resource")" ]]; then
    ready=1
    break
  fi
  sleep 3
done
[[ "$ready" == "1" ]] || fail "public Key Value did not become available"
resource=""

connection_info="$(api_json GET "/key-value/$fixture_id/connection-info")" || fail "connection-info lookup failed"
external_host="$(jq -er '.externalConnectionString |
  select(startswith("rediss://")) |
  capture("^rediss://(?:[^@]+@)?(?<host>\\[[^]]+\\]|[^:/]+)(?::[0-9]+)?(?:/.*)?$").host' \
  <<<"$connection_info")" || fail "connection-info did not expose a public rediss endpoint"
connection_info=""
external_host="${external_host#[}"
external_host="${external_host%]}"
[[ -n "$(dig +short "$external_host" | sed -n '1p')" ]] || fail "public Key Value DNS did not resolve"
nc -z -w 10 "$external_host" 6379 >/dev/null 2>&1 || fail "public Key Value TCP/6379 did not accept a connection"
external_host=""
echo "PASS public rediss endpoint ready"

probe_key="m57:$(date +%s):$$"
probe_value="m57-value-$(date +%s)-$$"
if ! probe_output="$({
  cd "$repo_root/lego/backend"
  BEX_TEST_KV_CLI_BIN="$RENDER_BIN" \
    BEX_TEST_KV_CLI_ID="$fixture_id" \
    BEX_TEST_KV_CLI_NAME="$fixture_name" \
    BEX_TEST_KV_CLI_KEY="$probe_key" \
    BEX_TEST_KV_CLI_VALUE="$probe_value" \
    BEX_TEST_KV_CLI_FAMILY="$address_family" \
    go test ./internal/keyvalue -run '^TestOfficialCLIKeyValueAcceptance$' -count=1 -v
} 2>&1)"; then
  printf '%s\n' "$probe_output" >&2
  fail "official kv-cli PTY acceptance"
fi
probe_output=""
probe_key=""
probe_value=""
echo "PASS official kv-cli by opaque id: PING and SET"
echo "PASS official kv-cli by display name: GET and DEL"
echo "PASS no attached human terminal required"
