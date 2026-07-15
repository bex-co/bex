#!/usr/bin/env bash
# Run the official Render CLI rename smoke against one isolated dev-N stack.
# The stack must already be healthy via .pm/wN/dev-N/up.sh. Authentication is
# throwaway and kept in memory; no API key, session, password, or kubeconfig is
# printed or persisted.
set -euo pipefail
cd "$(dirname "$0")/.."

DEV_NUMBER="${1:-}"
if [[ ! "$DEV_NUMBER" =~ ^([1-9]|10)$ ]]; then
  echo "usage: scripts/postgres-rename-dev-smoke.sh <1-10>" >&2
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
  [ -z "$PF_PID" ] || kill "$PF_PID" >/dev/null 2>&1 || true
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT
trap 'echo "error: dev-$DEV_NUMBER smoke failed during $PHASE" >&2' ERR

[ -f "$KUBECONFIG_FILE" ] || { echo "error: $ENV_DIR is not up (missing generated kubeconfig)" >&2; exit 1; }
[ -x "$RENDER_BIN" ] || { echo "error: render binary missing at $RENDER_BIN" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }

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
  email="postgres-rename-dev-$DEV_NUMBER-$(date +%s)-$$-$attempt@example.com"
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

# The first authenticated request creates the caller's default workspace.
PHASE="workspace and API-key bootstrap"
curl -sf -H "X-Session-Token: $session_token" "http://localhost:$BEX_API_PORT/v1/services" >/dev/null
key="$(curl -sf -X POST "http://localhost:$BEX_API_PORT/v1/api-keys" \
  -H "X-Session-Token: $session_token" -H 'Content-Type: application/json' \
  -d '{"name":"postgres-rename-smoke"}')"
client_id="$(jq -er '.id' <<<"$key")"
client_secret="$(jq -er '.secret' <<<"$key")"
workspace_id="$(curl -sf -H "X-Session-Token: $session_token" "http://localhost:$BEX_API_PORT/v1/owners" | jq -er '.[0].owner.id')"
access_token="$(curl -sf -X POST "http://localhost:$HYDRA_PUBLIC_PORT/oauth2/token" \
  -d "grant_type=client_credentials&client_id=$client_id&client_secret=$client_secret" | jq -er '.access_token')"

export RENDER_HOST="http://localhost:$BEX_API_PORT/v1/"
export RENDER_API_KEY="$access_token"
export RENDER_WORKSPACE="$workspace_id"
export RENDER_CLI_CONFIG_PATH="$WORK_DIR/cli.yaml"

old_name="rename-dev-$DEV_NUMBER-$$"
new_name="renamed-dev-$DEV_NUMBER-$$"
PHASE="official CLI create"
created="$($RENDER_BIN postgres create --name "$old_name" --plan free --confirm --output json)"
DATABASE_ID="$(jq -er '.data.id' <<<"$created")"
[[ "$DATABASE_ID" == dpg-* ]] || { echo "error: create returned non-dpg id" >&2; exit 1; }
created_name="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get databases.app.bex.co "$DATABASE_ID" -o jsonpath='{.spec.name}')"
created_workspace="$(KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get databases.app.bex.co "$DATABASE_ID" -o jsonpath='{.metadata.labels.bex\.co/tenant}')"
[ "$created_name" = "$old_name" ] || { echo "error: created CR display name is $created_name, want $old_name" >&2; exit 1; }
[ "$created_workspace" = "$workspace_id" ] || { echo "error: created CR workspace is $created_workspace, want $workspace_id" >&2; exit 1; }

# Wait for a complete minimum data plane—not merely the Database/Cluster CRs—
# so the identity inventory proves the PVC, credential Secret, and CNPG
# connection Service survive the rename too.
PHASE="CNPG projection"
for _ in $(seq 1 120); do
  if KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get \
    "clusters.postgresql.cnpg.io/$DATABASE_ID" \
    "secrets/$DATABASE_ID-app" \
    "services/$DATABASE_ID-rw" >/dev/null 2>&1 &&
    KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get persistentvolumeclaims \
      -l "cnpg.io/cluster=$DATABASE_ID" -o name | grep -q .; then
    break
  fi
  sleep 1
done
KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get \
  "clusters.postgresql.cnpg.io/$DATABASE_ID" \
  "secrets/$DATABASE_ID-app" \
  "services/$DATABASE_ID-rw" >/dev/null
KUBECONFIG="$KUBECONFIG_FILE" kubectl -n "$DEV_NS" get persistentvolumeclaims \
  -l "cnpg.io/cluster=$DATABASE_ID" -o name | grep -q .

before="$WORK_DIR/before.tsv"
PHASE="identity snapshot"
KUBECONFIG="$KUBECONFIG_FILE" bash scripts/postgres-rename-verify.sh snapshot \
  --namespace "$DEV_NS" --database "$DATABASE_ID" --output "$before" >/dev/null
for kind in Database Cluster PersistentVolumeClaim Secret Service; do
  awk -F '\t' -v kind="$kind" '$2 == kind { found=1 } END { exit !found }' "$before" || {
    echo "error: identity snapshot did not include a $kind" >&2
    exit 1
  }
done

PHASE="old-name resolution"
for _ in $(seq 1 30); do
  if $RENDER_BIN postgres get "$old_name" --output json >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
$RENDER_BIN postgres get "$old_name" --output json >/dev/null

PHASE="official CLI rename"
$RENDER_BIN postgres update "$old_name" --name "$new_name" --output json >/dev/null
PHASE="new-name resolution"
resolved="$($RENDER_BIN postgres get "$new_name" --output json)"
resolved_id="$(jq -er '.data.id' <<<"$resolved")"
[ "$resolved_id" = "$DATABASE_ID" ] || { echo "error: renamed database resolved to a different id" >&2; exit 1; }
PHASE="identity comparison"
KUBECONFIG="$KUBECONFIG_FILE" bash scripts/postgres-rename-verify.sh compare \
  --namespace "$DEV_NS" --database "$DATABASE_ID" --before "$before" --name "$new_name"

echo "dev-$DEV_NUMBER verified: official CLI rename kept $DATABASE_ID and all recorded Kubernetes identities"
PHASE="throwaway cleanup"
$RENDER_BIN postgres delete "$DATABASE_ID" --confirm --output json >/dev/null
DATABASE_ID=""
