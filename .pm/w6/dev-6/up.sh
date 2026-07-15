#!/usr/bin/env bash
# Bring up the dev-6 isolated environment (workstream w6): its own Kratos +
# Hydra + Mailpit (namespace dev-6-auth) on the shared local kind/CAPD cluster
# ("bex"), reusing that cluster's CNPG operator and the bex operator (already
# watching all namespaces) — only the identity stack + a dedicated app
# namespace (dev-6) are duplicated, so this never collides with the shared
# cluster's "auth" namespace Kratos on :5173/:4433, nor with any other
# workstream's dev-N environment.
#
# Then builds+runs a local bex-api pointed at this stack, and port-forwards
# kratos-public + hydra-admin + bex-db to the ports in ports.env.
#
# Usage: bash .pm/w6/dev-6/up.sh
# Idempotent: safe to re-run (helm upgrade --install, kubectl apply, PIDs
# re-checked before re-forking). Requires: kind, kubectl, helm, go, openssl.
set -euo pipefail
cd "$(dirname "$0")/../../.." # repo root
ENVDIR=".pm/w6/dev-6"
# shellcheck disable=SC1091
source "$ENVDIR/ports.env"

mkdir -p "$ENVDIR/.pids" "$ENVDIR/logs" "$ENVDIR/bin"
KUBECONFIG_FILE="$ENVDIR/.kubeconfig"

echo "==> refreshing kubeconfig for kind cluster 'bex'"
kind get kubeconfig --name bex > "$KUBECONFIG_FILE"
export KUBECONFIG="$PWD/$KUBECONFIG_FILE"

echo "==> namespaces"
kubectl create namespace "$DEV_AUTH_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl create namespace "$DEV_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

echo "==> operator RBAC for the dev-6 apps namespace (secrets CRUD — see rbac-dev-6.yaml)"
kubectl apply -f "$ENVDIR/rbac-dev-6.yaml" >/dev/null

echo "==> CNPG Clusters (kratos-db, hydra-db, bex-db)"
kubectl apply -f "$ENVDIR/db/kratos-db.yaml" -f "$ENVDIR/db/hydra-db.yaml" -f "$ENVDIR/db/bex-db.yaml" >/dev/null
for cluster in kratos-db hydra-db bex-db; do
  echo "    waiting for $cluster-app credentials secret..."
  for _ in $(seq 1 60); do
    kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" >/dev/null 2>&1 && break
    sleep 5
  done
  kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" >/dev/null 2>&1 \
    || { echo "error: $cluster never produced its -app credentials secret"; exit 1; }
done

echo "==> mailpit"
kubectl apply -f "$ENVDIR/mailpit/deployment.yaml" >/dev/null
kubectl -n "$DEV_AUTH_NS" rollout status deployment/mailpit --timeout=120s

# dsn CLUSTER DB — postgres:// DSN for a CNPG cluster's app user (mirrors
# scripts/auth-secrets.sh's dsn() helper, dev-6-auth namespace).
dsn() {
  local cluster="$1" db="$2" user pass
  { read -r user; read -r pass; } < <(kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" \
    -o go-template='{{.data.username | base64decode}}{{"\n"}}{{.data.password | base64decode}}')
  printf 'postgres://%s:%s@%s-rw.%s.svc.cluster.local:5432/%s?sslmode=require' \
    "$user" "$pass" "$cluster" "$DEV_AUTH_NS" "$db"
}

echo "==> out-of-band kratos/hydra secrets (freshly generated, dev-6-auth only)"
kubectl create secret generic kratos -n "$DEV_AUTH_NS" \
  --from-literal=dsn="$(dsn kratos-db kratos)" \
  --from-literal=secretsDefault="$(openssl rand -hex 16)" \
  --from-literal=secretsCookie="$(openssl rand -hex 16)" \
  --from-literal=secretsCipher="$(openssl rand -hex 16)" \
  --from-literal=smtpConnectionURI="smtp://mailpit.$DEV_AUTH_NS.svc:1025/?disable_starttls=true" \
  --from-literal=oidc.yaml="$(printf 'selfservice:\n  methods:\n    oidc:\n      enabled: false\n')" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

kubectl create secret generic hydra -n "$DEV_AUTH_NS" \
  --from-literal=dsn="$(dsn hydra-db hydra)" \
  --from-literal=secretsSystem="$(openssl rand -hex 16)" \
  --from-literal=secretsCookie="$(openssl rand -hex 16)" \
  --from-literal=oidcPairwiseSalt="$(openssl rand -hex 8)" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

echo "==> helm repo"
helm repo add ory https://k8s.ory.sh/helm/charts >/dev/null 2>&1 || true
helm repo update ory >/dev/null

echo "==> kratos + hydra (helm upgrade --install)"
helm upgrade --install kratos ory/kratos --version 0.62.1 -n "$DEV_AUTH_NS" \
  -f deploy/gitops/base/values/kratos.values.yaml -f "$ENVDIR/values/kratos.values.yaml" --wait --timeout 5m
helm upgrade --install hydra ory/hydra --version 0.62.1 -n "$DEV_AUTH_NS" \
  -f deploy/gitops/base/values/hydra.values.yaml -f "$ENVDIR/values/hydra.values.yaml" --wait --timeout 5m

kill_if_running() {
  local pidfile="$1" pid
  if [ -f "$pidfile" ]; then
    pid="$(cat "$pidfile")"
    pkill -P "$pid" 2>/dev/null || true # the forward()'d loop's current kubectl child
    kill "$pid" 2>/dev/null || true
  fi
  rm -f "$pidfile"
}

# forward NAME SERVICE LOCALPORT:REMOTEPORT — a self-healing port-forward: kind's
# Calico CNI occasionally resets an idle port-forward's connection ("lost
# connection to pod"), which would otherwise wedge bex-api's DB pool until a
# manual restart. Loops kubectl port-forward under a supervisor instead of
# forking it directly, so a drop reconnects on its own within ~1s.
forward() {
  local name="$1" service="$2" ports="$3"
  kill_if_running "$ENVDIR/.pids/pf-$name.pid"
  nohup bash -c "while true; do kubectl -n '$DEV_AUTH_NS' port-forward service/$service $ports; sleep 1; done" \
    > "$ENVDIR/logs/pf-$name.log" 2>&1 & echo $! > "$ENVDIR/.pids/pf-$name.pid"
}

echo "==> port-forwards (self-healing)"
forward kratos kratos-public "$KRATOS_PUBLIC_PORT:80"
forward kratos-admin kratos-admin "$KRATOS_ADMIN_PORT:80"
forward hydra hydra-admin "$HYDRA_ADMIN_PORT:4445"
forward mailpit mailpit "$MAILPIT_HTTP_PORT:8025"
forward bex-db bex-db-rw "$BEX_DB_PORT:5432"
sleep 3

echo "==> building bex-api"
(cd lego/backend && go build -o "../../$ENVDIR/bin/bex-api" ./cmd/api)

# hostDsn CLUSTER DB — like dsn() above, but through the CLUSTER's host
# port-forward instead of the in-cluster service DNS: bex-api runs on the
# HOST (not in-cluster), so it can't resolve *.svc.cluster.local.
hostDsn() {
  local cluster="$1" db="$2" port="$3" user pass
  { read -r user; read -r pass; } < <(kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" \
    -o go-template='{{.data.username | base64decode}}{{"\n"}}{{.data.password | base64decode}}')
  printf 'postgres://%s:%s@localhost:%s/%s?sslmode=disable' "$user" "$pass" "$port" "$db"
}

echo "==> starting bex-api on :$BEX_API_PORT (namespace $DEV_NS)"
kill_if_running "$ENVDIR/.pids/bex-api.pid"
api_started=0
for attempt in $(seq 1 5); do
  nohup env \
    KUBECONFIG="$PWD/$KUBECONFIG_FILE" \
    BEX_API_ADDR=":$BEX_API_PORT" \
    BEX_CP_ADDR=":$BEX_CP_PORT" \
    BEX_API_NAMESPACE="$DEV_NS" \
    BEX_API_CORS_ORIGIN="http://localhost:$DASHBOARD_PORT" \
    BEX_DASHBOARD_URL="http://localhost:$DASHBOARD_PORT" \
    BEX_REGION="local-capd" \
    BEX_KRATOS_URL="http://localhost:$KRATOS_PUBLIC_PORT" \
    BEX_KRATOS_ADMIN_URL="http://localhost:$KRATOS_ADMIN_PORT" \
    BEX_HYDRA_ADMIN_URL="http://localhost:$HYDRA_ADMIN_PORT" \
    BEX_CP_DB_URI="$(hostDsn bex-db bex "$BEX_DB_PORT")" \
    BEX_CP_APPS_NAMESPACE="$DEV_NS" \
    BEX_BASE_DOMAIN="onbex.co" \
    "./$ENVDIR/bin/bex-api" > "$ENVDIR/logs/bex-api.log" 2>&1 & echo $! > "$ENVDIR/.pids/bex-api.pid"
  sleep 3
  if kill -0 "$(cat "$ENVDIR/.pids/bex-api.pid")" 2>/dev/null; then
    api_started=1
    break
  fi
  echo "    bex-api start attempt $attempt hit a transient DB-forward failure; retrying..."
  sleep 2
done
if [ "$api_started" -ne 1 ]; then
  echo "error: bex-api exited immediately — see $ENVDIR/logs/bex-api.log" >&2
  tail -20 "$ENVDIR/logs/bex-api.log" >&2
  exit 1
fi

echo
echo "dev-6 (workstream w6) is up:"
echo "  kubeconfig:  $KUBECONFIG_FILE (KUBECONFIG=\$PWD/$KUBECONFIG_FILE kubectl -n $DEV_NS get keyvalues.app.bex.co)"
echo "  kratos:      http://localhost:$KRATOS_PUBLIC_PORT (admin: http://localhost:$KRATOS_ADMIN_PORT)"
echo "  hydra admin: http://localhost:$HYDRA_ADMIN_PORT"
echo "  mailpit UI:  http://localhost:$MAILPIT_HTTP_PORT"
echo "  bex-api:     http://localhost:$BEX_API_PORT (log: $ENVDIR/logs/bex-api.log)"
echo
echo "start the dashboard against it:"
echo "  cd dashboard && VITE_API_URL=http://localhost:$BEX_API_PORT/graphql VITE_KRATOS_PUBLIC_URL=http://localhost:$KRATOS_PUBLIC_PORT yarn dev --port $DASHBOARD_PORT"
echo
echo "status: bash $ENVDIR/status.sh   |   tear down: bash $ENVDIR/down.sh"
