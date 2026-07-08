#!/usr/bin/env bash
# E2E for the dashboard's Environment tab (w4/m6.5) against the current kubeconfig
# cluster. It drives the exact GraphQL operations the dashboard uses
# (dashboard/src/features/services/api/env-vars.graphql — Render dashboard-shaped:
# env vars nested under the service, keys-only list + per-key value fetch) against
# a live bex-api, so it validates two things at once:
#
#   1. the operations parse against bex-api's REAL schema (envVarKeys/envVar under
#      the Service type, service(id) alias, setEnvVar/deleteEnvVar mutations) —
#      i.e. the dashboard's hand-generated definitions.ts matches the backend, and
#   2. the full path still works through GraphQL: setEnvVar -> OpenBao ->
#      materialized <svc>-env Secret -> pods roll -> the running app serves it.
#
# The app is deployed as `beancount-cms` (traefik/whoami, which echoes WHOAMI_NAME
# as its `Name:` line — a stand-in that makes an env value observable over HTTP).
# Same host-run stack as scripts/secrets-verify.sh (bex-api + operator on the host,
# OpenBao/Hydra reached via port-forwards); uses a Hydra API-key bearer, so no
# browser/Kratos-login flow is needed to prove the data path.
#
# Usage: scripts/dashboard-env-verify.sh   # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, yq v4, go; a cluster with m5 (OpenBao) + auth (Hydra).
set -euo pipefail
cd "$(dirname "$0")/.."

AUTH_NS=auth
SECRETS_NS=secrets
SYS_NS=bex-system
APP_NS=default
SVC=beancount-cms

HYDRA_PUBLIC=127.0.0.1:24444
HYDRA_ADMIN=127.0.0.1:24445
BAO=127.0.0.1:24200
API=127.0.0.1:18090
PIDS=()
API_PID=""
OP_PID=""

cleanup() {
  [ -n "$OP_PID" ] && kill "$OP_PID" 2>/dev/null || true
  [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null || true
  if [ "${#PIDS[@]}" -gt 0 ]; then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
  kubectl -n "$APP_NS" delete app.app.bex.co "$SVC" --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
wait_http() {
  for _ in $(seq 1 30); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)"
    [ "$code" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
[ -n "${BAO_ROOT_TOKEN:-}" ] || fail "BAO_ROOT_TOKEN missing (.env) — run scripts/bao-init.sh first"

echo "==> ensuring the App CRD is installed"
make -C lego/operator install >/dev/null

echo "==> port-forwards (hydra, openbao) + building bex-api and the operator"
kubectl -n "$AUTH_NS" port-forward service/hydra-public 24444:4444 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$AUTH_NS" port-forward service/hydra-admin 24445:4445 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$SECRETS_NS" port-forward service/openbao 24200:8200 >/dev/null 2>&1 & PIDS+=($!)
bindir="$(mktemp -d)"
(cd lego/backend && go build -o "$bindir/bex-api" ./cmd/api)
(cd lego/operator && go build -o "$bindir/manager" ./cmd/manager)
wait_http "http://$HYDRA_ADMIN/health/ready"
wait_http "http://$BAO/v1/sys/seal-status"

echo "==> scoping the bex-api ServiceAccount in OpenBao (idempotent)"
kubectl create namespace "$SYS_NS" >/dev/null 2>&1 || true
kubectl -n "$SYS_NS" create serviceaccount bex-api >/dev/null 2>&1 || true
BAO_ADDR="http://$BAO" bash scripts/bao-k8s-auth.sh >/dev/null
jwt_file="$bindir/sa-token"
kubectl -n "$SYS_NS" create token bex-api --duration=1h > "$jwt_file"

echo "==> seeding the bootstrap OAuth2 client + minting an API-key token"
export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-verify-bootstrap-$(date +%s)}"
HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" bash scripts/auth-bootstrap-client.sh >/dev/null
token="$({ curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
  -d "grant_type=client_credentials&client_id=bex-bootstrap&client_secret=$BEX_BOOTSTRAP_CLIENT_SECRET" || true; } | yq '.access_token // ""' -)"
[ -n "$token" ] || fail "no access_token for the bootstrap client"

echo "==> starting the operator and bex-api (OpenBao wired, CORS for the dashboard)"
BEX_RUNTIME=kubernetes "$bindir/manager" >/dev/null 2>&1 & OP_PID=$!
env "BEX_OPENBAO_URL=http://$BAO" "BEX_OPENBAO_JWT_PATH=$jwt_file" \
  "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" "BEX_API_CORS_ORIGIN=http://localhost:5173" \
  BEX_API_ADDR=":18090" BEX_API_NAMESPACE="$APP_NS" "$bindir/bex-api" >/dev/null 2>&1 & API_PID=$!
wait_http "http://$API/healthz"

echo "==> deploying the $SVC App (traefik/whoami echoes WHOAMI_NAME)"
kubectl apply -f - >/dev/null <<YAML
apiVersion: app.bex.co/v1alpha1
kind: App
metadata: { name: $SVC, namespace: $APP_NS }
spec: { image: traefik/whoami, port: 80, replicas: 1 }
YAML
# Give the operator a moment to reconcile the App into a Deployment before
# waiting on its condition (kubectl wait errors on a not-yet-created resource).
for _ in $(seq 1 30); do
  kubectl -n "$APP_NS" get deployment/"$SVC" >/dev/null 2>&1 && break
  sleep 2
done
kubectl -n "$APP_NS" wait --for=condition=Available --timeout=120s deployment/"$SVC" \
  || fail "$SVC Deployment never became Available"

# gql OP_NAME QUERY VARIABLES_JSON -> sets LAST (the .data object, JSON). Fails on
# a GraphQL error (so a schema mismatch — e.g. envVarKeys not on Service — fails
# loudly instead of silently returning null).
gql() {
  local body resp
  # strenv keeps the GraphQL query + variables JSON intact (no yq parsing of {}/!/$).
  body="$(_Q="$2" _V="$3" yq -n -o=json '.query = strenv(_Q) | .variables = (strenv(_V) | fromjson)')"
  resp="$(curl -s -X POST "http://$API/graphql" \
    -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body")"
  if [ "$(printf '%s' "$resp" | yq '.errors // [] | length')" != "0" ]; then
    fail "$1: GraphQL errors: $(printf '%s' "$resp" | yq -o=json '.errors' -)"
  fi
  LAST="$(printf '%s' "$resp" | yq -o=json '.data')"
}

# Single-quoted so bash leaves the GraphQL $variables intact; the gql() helper
# expands them into the yq expression as a whole (never re-expanded by bash).
Q_KEYS='query EnvVarKeys($id: String!) { service(id: $id) { id envVarKeys { id key } } }'
Q_VALUE='query EnvVarValue($id: String!, $key: String!) { service(id: $id) { id envVar(key: $key) { id key value } } }'
M_SET='mutation SetEnvVar($serviceId: String!, $key: String!, $value: String) { setEnvVar(serviceId: $serviceId, key: $key, value: $value) }'
M_DEL='mutation DeleteEnvVar($serviceId: String!, $key: String!) { deleteEnvVar(serviceId: $serviceId, key: $key) }'

echo "==> [1/5] envVarKeys parses against the live schema and starts empty"
gql EnvVarKeys "$Q_KEYS" "{\"id\":\"$SVC\"}"
n="$(printf '%s' "$LAST" | yq '.service.envVarKeys | length')"
[ "$n" = "0" ] || echo "    note: $SVC already had $n env var(s); continuing"
echo "    ok: service(id){ envVarKeys{ id key } } resolved (schema matches the dashboard's ops)"

WANT="dash-$(date +%s)"
echo "==> [2/5] setEnvVar WHOAMI_NAME=$WANT (the dashboard's add/update mutation)"
gql SetEnvVar "$M_SET" "{\"serviceId\":\"$SVC\",\"key\":\"WHOAMI_NAME\",\"value\":\"$WANT\"}"
[ "$(printf '%s' "$LAST" | yq '.setEnvVar')" = "true" ] || fail "setEnvVar did not return true"
echo "    ok: setEnvVar -> true"

echo "==> [3/5] value present in OpenBao + readable via the dashboard's envVar(key)"
scoped="$(curl -sf -X POST "http://$BAO/v1/auth/kubernetes/login" \
  -d "{\"role\":\"bex-api\",\"jwt\":\"$(cat "$jwt_file")\"}" | yq '.auth.client_token' -)"
bao_val="$(curl -sf -H "X-Vault-Token: $scoped" \
  "http://$BAO/v1/tenants/data/$APP_NS/services/$SVC/env" | yq '.data.data.WHOAMI_NAME' -)"
[ "$bao_val" = "$WANT" ] || fail "OpenBao does not hold the value: got '$bao_val'"
gql EnvVarValue "$Q_VALUE" "{\"id\":\"$SVC\",\"key\":\"WHOAMI_NAME\"}"
[ "$(printf '%s' "$LAST" | yq '.service.envVar.value')" = "$WANT" ] || fail "envVar(key) did not return the value"
# keys-only list must NOT carry the value (Render dashboard shape).
gql EnvVarKeys "$Q_KEYS" "{\"id\":\"$SVC\"}"
printf '%s' "$LAST" | yq -e '.service.envVarKeys[] | select(.key == "WHOAMI_NAME")' - >/dev/null || fail "WHOAMI_NAME not in envVarKeys"
echo "    ok: in OpenBao, and envVar(WHOAMI_NAME).value == $WANT (keys list stays keys-only)"

echo "==> [4/5] pods roll; the running app serves the new value"
kubectl -n "$APP_NS" rollout status deployment/"$SVC" --timeout=120s >/dev/null || fail "rollout did not complete"
APP=127.0.0.1:18081
kubectl -n "$APP_NS" port-forward service/"$SVC" 18081:80 >/dev/null 2>&1 & app_pf=$!; PIDS+=("$app_pf")
wait_http "http://$APP/"
served=""
for _ in $(seq 1 20); do
  served="$(curl -s "http://$APP/" || true)"
  echo "$served" | grep -q "Name: $WANT" && break
  sleep 3
done
{ kill "$app_pf" && wait "$app_pf"; } 2>/dev/null || true
echo "$served" | grep -q "Name: $WANT" || fail "app did not serve the new value"
echo "    ok: $SVC serves 'Name: $WANT'"

echo "==> [5/5] deleteEnvVar removes it (the dashboard's delete mutation)"
gql DeleteEnvVar "$M_DEL" "{\"serviceId\":\"$SVC\",\"key\":\"WHOAMI_NAME\"}"
[ "$(printf '%s' "$LAST" | yq '.deleteEnvVar')" = "true" ] || fail "deleteEnvVar did not return true"
gql EnvVarKeys "$Q_KEYS" "{\"id\":\"$SVC\"}"
printf '%s' "$LAST" | yq -e '.service.envVarKeys[] | select(.key == "WHOAMI_NAME")' - >/dev/null 2>&1 && fail "WHOAMI_NAME still present after delete"
echo "    ok: deleteEnvVar -> true; key gone"

echo "PASS: the dashboard's env-vars GraphQL operations work end-to-end against a live beancount-cms"
