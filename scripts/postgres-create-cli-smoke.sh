#!/usr/bin/env bash
# Prove Render's unmodified CLI custom databaseName/databaseUser create contract
# against one isolated dev-N CNPG stack. The script creates only disposable
# databases, never prints credentials/kubeconfig contents, and cleans up on exit.
set -Eeuo pipefail
cd "$(dirname "$0")/.."

DEV_NUMBER="${1:-6}"
if [[ ! "$DEV_NUMBER" =~ ^([1-9]|10)$ ]]; then
  echo "usage: scripts/postgres-create-cli-smoke.sh [1-10]" >&2
  exit 2
fi

ENV_DIR=".pm/w$DEV_NUMBER/dev-$DEV_NUMBER"
# shellcheck disable=SC1090
source "$ENV_DIR/ports.env"
KUBECONFIG_FILE="$PWD/$ENV_DIR/.kubeconfig"
RENDER_BIN="${RENDER_BIN:-$PWD/.pm/w9/dev-9/bin/render}"
HYDRA_PUBLIC_PORT=$((59000 + DEV_NUMBER * 10))
WORK_DIR="$(mktemp -d)"
DATABASE_ID=""
PF_PID=""
PHASE="preflight"

cleanup() {
  if [ -n "$DATABASE_ID" ]; then
    KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" delete databases.app.bex.co "$DATABASE_ID" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  fi
  if [ -n "$PF_PID" ]; then
    kill "$PF_PID" >/dev/null 2>&1 || true
    wait "$PF_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT
trap 'echo "error: dev-$DEV_NUMBER postgres-create smoke failed during $PHASE" >&2' ERR

[ -f "$KUBECONFIG_FILE" ] || { echo "error: $ENV_DIR is not up (missing generated kubeconfig)" >&2; exit 1; }
[ -x "$RENDER_BIN" ] || { echo "error: render binary missing at $RENDER_BIN" >&2; exit 1; }
for command_name in curl jq kubectl openssl; do
  command -v "$command_name" >/dev/null || { echo "error: $command_name is required" >&2; exit 1; }
done

PHASE="Kratos and bex-api readiness"
for _ in $(seq 1 30); do
  if curl -sf "http://localhost:$KRATOS_PUBLIC_PORT/health/ready" >/dev/null 2>&1 &&
    curl -sf "http://localhost:$BEX_API_PORT/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -sf "http://localhost:$KRATOS_PUBLIC_PORT/health/ready" >/dev/null
curl -sf "http://localhost:$BEX_API_PORT/healthz" >/dev/null

KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_AUTH_NS" port-forward service/hydra-public "$HYDRA_PUBLIC_PORT:4444" >"$WORK_DIR/hydra-public.log" 2>&1 &
PF_PID=$!
PHASE="Hydra public readiness"
for _ in $(seq 1 30); do
  curl -sf "http://localhost:$HYDRA_PUBLIC_PORT/health/ready" >/dev/null 2>&1 && break
  sleep 1
done
curl -sf "http://localhost:$HYDRA_PUBLIC_PORT/health/ready" >/dev/null

password="$(openssl rand -hex 18)"
PHASE="Kratos registration"
registration=""
for attempt in $(seq 1 5); do
  email="postgres-create-dev-$DEV_NUMBER-$(date +%s)-$$-$attempt@example.com"
  if flow="$(curl -sf "http://localhost:$KRATOS_PUBLIC_PORT/self-service/registration/api" | jq -er '.id')" &&
    registration="$(curl -sf -X POST "http://localhost:$KRATOS_PUBLIC_PORT/self-service/registration?flow=$flow" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc --arg email "$email" --arg password "$password" '{method:"password",traits:{email:$email},password:$password}')")" &&
    jq -e '.session_token | strings | length > 0' <<<"$registration" >/dev/null; then
    break
  fi
  registration=""
  sleep 2
done
[ -n "$registration" ] || { echo "error: Kratos registration did not become ready" >&2; exit 1; }
session_token="$(jq -er '.session_token' <<<"$registration")"

PHASE="workspace and API-key bootstrap"
curl -sf -H "X-Session-Token: $session_token" "http://localhost:$BEX_API_PORT/v1/services" >/dev/null
key="$(curl -sf -X POST "http://localhost:$BEX_API_PORT/v1/api-keys" \
  -H "X-Session-Token: $session_token" -H 'Content-Type: application/json' \
  -d '{"name":"postgres-create-smoke"}')"
client_id="$(jq -er '.id' <<<"$key")"
client_secret="$(jq -er '.secret' <<<"$key")"
workspace_id="$(curl -sf -H "X-Session-Token: $session_token" "http://localhost:$BEX_API_PORT/v1/owners" | jq -er '.[0].owner.id')"
access_token="$(curl -sf -X POST "http://localhost:$HYDRA_PUBLIC_PORT/oauth2/token" \
  -d "grant_type=client_credentials&client_id=$client_id&client_secret=$client_secret" | jq -er '.access_token')"

export RENDER_HOST="http://localhost:$BEX_API_PORT/v1/"
export RENDER_API_KEY="$access_token"
export RENDER_WORKSPACE="$workspace_id"
export RENDER_CLI_CONFIG_PATH="$WORK_DIR/cli.yaml"

verify_case() {
  local suffix="$1"
  local requested_db="$2"
  local requested_user="$3"
  local display_name="m38-$DEV_NUMBER-$suffix-$$"
  local -a args=(postgres create --name "$display_name" --plan free --confirm --output json)
  [ -z "$requested_db" ] || args+=(--database-name "$requested_db")
  [ -z "$requested_user" ] || args+=(--database-user "$requested_user")

  PHASE="official CLI create ($suffix)"
  local created
  created="$($RENDER_BIN "${args[@]}")"
  DATABASE_ID="$(jq -er '.data.id' <<<"$created")"
  [[ "$DATABASE_ID" == dpg-* ]] || { echo "error: create returned non-dpg id" >&2; exit 1; }

  local default_db="${DATABASE_ID//-/_}"
  local expected_db="${requested_db:-$default_db}"
  local expected_user="${requested_user:-${default_db}_user}"
  jq -e --arg db "$expected_db" --arg user "$expected_user" '.data.databaseName == $db and .data.databaseUser == $user' <<<"$created" >/dev/null

  PHASE="CR intent ($suffix)"
  local spec_db spec_user
  spec_db="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get databases.app.bex.co "$DATABASE_ID" -o jsonpath='{.spec.databaseName}')"
  spec_user="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get databases.app.bex.co "$DATABASE_ID" -o jsonpath='{.spec.databaseUser}')"
  [ "$spec_db" = "$requested_db" ] || { echo "error: spec.databaseName did not preserve create intent" >&2; exit 1; }
  [ "$spec_user" = "$requested_user" ] || { echo "error: spec.databaseUser did not preserve create intent" >&2; exit 1; }

  PHASE="CNPG readiness ($suffix)"
  for _ in $(seq 1 180); do
    phase="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get databases.app.bex.co "$DATABASE_ID" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    if [ "$phase" = "Ready" ] && KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get "secret/$DATABASE_ID-app" >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done
  [ "$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get databases.app.bex.co "$DATABASE_ID" -o jsonpath='{.status.phase}')" = "Ready" ]

  PHASE="SQL identity ($suffix)"
  local secret_json db_password primary_pod identity
  secret_json="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get "secret/$DATABASE_ID-app" -o json)"
  [ "$(jq -er '.data.dbname | @base64d' <<<"$secret_json")" = "$expected_db" ]
  [ "$(jq -er '.data.username | @base64d' <<<"$secret_json")" = "$expected_user" ]
  db_password="$(jq -er '.data.password | @base64d' <<<"$secret_json")"
  primary_pod="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get pods -l "cnpg.io/cluster=$DATABASE_ID,role=primary" -o jsonpath='{.items[0].metadata.name}')"
  identity="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" exec -c postgres "$primary_pod" -- env PGPASSWORD="$db_password" \
    psql -h "$DATABASE_ID-rw" -U "$expected_user" -d "$expected_db" -Atqc "select current_database(), current_user")"
  [ "$identity" = "$expected_db|$expected_user" ] || { echo "error: SQL identity mismatch" >&2; exit 1; }

  if [ "$suffix" = "both" ]; then
    PHASE="Datadog update rejection"
    if "$RENDER_BIN" postgres update "$DATABASE_ID" --datadog-api-key not-a-real-key --datadog-site US1 --output json >"$WORK_DIR/datadog-update.out" 2>&1; then
      echo "error: unsupported Datadog update unexpectedly succeeded" >&2
      exit 1
    fi
    grep -q "Postgres Datadog monitoring is not supported" "$WORK_DIR/datadog-update.out"
  fi

  PHASE="case cleanup ($suffix)"
  "$RENDER_BIN" postgres delete "$DATABASE_ID" --confirm --output json >/dev/null
  DATABASE_ID=""
}

verify_case both "m38_${DEV_NUMBER}_data_$$" "m38_${DEV_NUMBER}_owner_$$"
verify_case database-only "m38_${DEV_NUMBER}_data_only_$$" ""
verify_case user-only "" "m38_${DEV_NUMBER}_owner_only_$$"

PHASE="Datadog create rejection"
if "$RENDER_BIN" postgres create --name "m38-$DEV_NUMBER-dd-$$" --plan free --datadog-api-key not-a-real-key --datadog-site US1 --confirm --output json >"$WORK_DIR/datadog-create.out" 2>&1; then
  echo "error: unsupported Datadog create unexpectedly succeeded" >&2
  exit 1
fi
grep -q "Postgres Datadog monitoring is not supported" "$WORK_DIR/datadog-create.out"

echo "PASS: official CLI custom/default database and owner names reached real CNPG SQL identity; Datadog flags failed closed"
