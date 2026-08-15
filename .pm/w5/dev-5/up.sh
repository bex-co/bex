#!/usr/bin/env bash
# Bring up the dev-5 isolated environment (workstream w5): its own Kratos +
# Hydra + Mailpit (namespace dev-5-auth) on the shared local kind/CAPD cluster
# ("bex"), reusing that cluster's CNPG operator and the bex operator (already
# watching all namespaces) — only the identity stack + a dedicated app
# namespace (dev-5) are duplicated, so this never collides with the shared
# cluster's "auth" namespace Kratos on :5173/:4433, nor with any other
# workstream's dev-N environment.
#
# Then builds+runs a local bex-api pointed at this stack, and port-forwards
# kratos-public + hydra-admin + bex-db to the ports in ports.env.
#
# Usage: bash .pm/w5/dev-5/up.sh
# Idempotent: safe to re-run (helm upgrade --install, kubectl apply, PIDs
# re-checked before re-forking). Requires: kind, kubectl, helm, go, openssl.
set -euo pipefail
cd "$(dirname "$0")/../../.." # repo root
ENVDIR=".pm/w5/dev-5"
# shellcheck disable=SC1091
source "$ENVDIR/ports.env"

mkdir -p "$ENVDIR/.pids" "$ENVDIR/logs" "$ENVDIR/bin"
KUBECONFIG_FILE="$ENVDIR/.kubeconfig"

# --- preflight (w5/m62): fail fast with an actionable message instead of a
# cryptic mid-run error. The header once claimed dev-5 "reuses the shared
# cluster's CNPG operator" — a precondition that silently stopped holding, so
# up.sh died opaquely at the Cluster apply ("no matches for kind Cluster").
echo "==> preflight"
for tool in kind kubectl helm go openssl; do
  command -v "$tool" >/dev/null 2>&1 \
    || { echo "error: '$tool' not found on PATH — dev-5 needs kind/kubectl/helm/go/openssl" >&2; exit 1; }
done
kind get clusters 2>/dev/null | grep -qx bex \
  || { echo "error: kind cluster 'bex' not found — run 'bash scripts/mock-cluster.sh' first" >&2; exit 1; }

echo "==> refreshing kubeconfig for kind cluster 'bex'"
kind get kubeconfig --name bex > "$KUBECONFIG_FILE"
export KUBECONFIG="$PWD/$KUBECONFIG_FILE"
kubectl cluster-info >/dev/null 2>&1 \
  || { echo "error: cluster 'bex' is unreachable via $KUBECONFIG_FILE — is the kind cluster running?" >&2; exit 1; }

# A default StorageClass is a hard precondition: every stateful workload dev-5
# creates needs a PVC — the three CNPG databases (kratos/hydra/bex) AND Loki. A
# cluster with none leaves them all Pending ("unbound immediate PVC"), which
# looks like a dev-5 bug but is a broken shared cluster. Catch it here with a
# fix pointer instead (w5/m62 — this was a real dev-5 block: the CAPD `bex`
# cluster came up storage-less).
kubectl get storageclass -o jsonpath='{range .items[*]}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{"\n"}{end}' 2>/dev/null | grep -qx true \
  || { echo "error: cluster 'bex' has no default StorageClass — CNPG databases and Loki cannot bind their PVCs. Reprovision the cluster ('bash scripts/mock-cluster.sh') so its default StorageClass (local-path on CAPD) is present." >&2; exit 1; }

# ensure_cnpg (w5/m62): install the CloudNativePG operator ourselves when the
# CRD is absent, instead of assuming the shared cluster already has it, then
# wait for the operator to become *ready* — not just installed. Pins the same
# chart as deploy/gitops/base/cnpg-operator.yaml (0.29.0), minus the platform
# nodeSelector the disposable kind node doesn't carry. Failing here with the
# diagnostic below is the correct outcome for an unmeetable precondition (e.g.
# the CNPG admission webhook can't bind :9443 on some kind builds) — far better
# than the previous silent "no matches for kind Cluster".
ensure_cnpg() {
  if ! kubectl get crd clusters.postgresql.cnpg.io >/dev/null 2>&1; then
    echo "    CNPG operator absent — installing cloudnative-pg 0.29.0"
    helm repo add cnpg https://cloudnative-pg.github.io/charts >/dev/null 2>&1 || true
    helm repo update cnpg >/dev/null
    helm upgrade --install cnpg cnpg/cloudnative-pg --version 0.29.0 \
      -n cnpg-system --create-namespace >/dev/null
  fi
  kubectl wait --for=condition=established crd/clusters.postgresql.cnpg.io --timeout=120s >/dev/null 2>&1 \
    || { echo "error: the CNPG 'clusters' CRD never established — operator install failed" >&2; exit 1; }
  # Pin the manager to the control-plane node: on the CAPD mock, pods on worker
  # nodes can't reach the apiserver (OrbStack+Calico, docs/ADR004-app-deployment.md),
  # so a worker-scheduled CNPG manager starts, hangs on its first API call, and
  # crashloops (w1/043 — the prior cluster's cnpg CrashLoopBackOff root cause).
  kubectl -n cnpg-system patch deploy cnpg-cloudnative-pg --type merge -p \
    '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
     "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}}}}' >/dev/null
  echo "    waiting for the CNPG operator to become ready..."
  if ! kubectl -n cnpg-system rollout status deploy/cnpg-cloudnative-pg --timeout=150s; then
    # On a degraded local cluster the CNPG operator can crashloop (etcd/apiserver
    # slowness → "leader election lost", or a :9443 webhook bind failure on some
    # kind builds) WITHOUT that blocking dev-5: the three CNPG Clusters it needs
    # to create may ALREADY exist and keep running (their Postgres pods don't
    # depend on a live operator). Only hard-fail when the DB clusters are genuinely
    # absent; otherwise warn and continue so a re-run isn't wedged by a flaky
    # operator that already did its one job. The DB-secret waits below still gate
    # on the real precondition (the -app credentials existing).
    if kubectl -n "$DEV_AUTH_NS" get cluster.postgresql.cnpg.io kratos-db hydra-db bex-db >/dev/null 2>&1; then
      echo "    warn: CNPG operator not ready, but the kratos-db/hydra-db/bex-db Clusters already exist — continuing (their Postgres pods run without a live operator)." >&2
    else
      {
        echo "error: the CNPG operator did not become ready AND the DB Clusters do not exist yet — dev-5's databases cannot be created."
        echo "diagnose:"
        echo "  KUBECONFIG=$PWD/$KUBECONFIG_FILE kubectl -n cnpg-system describe pod -l app.kubernetes.io/name=cloudnative-pg"
        echo "  KUBECONFIG=$PWD/$KUBECONFIG_FILE kubectl -n cnpg-system logs deploy/cnpg-cloudnative-pg --tail=40"
        echo "(a webhook :9443 startup-probe failure means the operator's admission webhook can't bind; 'leader election lost' means the cluster's etcd/apiserver is too slow — both are CNPG-on-kind cluster issues, not a dev-5 script bug)"
      } >&2
      exit 1
    fi
  fi
}
echo "==> CNPG operator (self-installing if absent)"
ensure_cnpg

echo "==> namespaces"
kubectl create namespace "$DEV_AUTH_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl create namespace "$DEV_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

echo "==> operator RBAC for the dev-5 apps namespace (secrets CRUD — see rbac-dev-5.yaml)"
kubectl apply -f "$ENVDIR/rbac-dev-5.yaml" >/dev/null

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

# ensure_observability (w5/m62): a minimal Loki + Alloy log-shipper, so dev-5 can
# prove the store-gated log path (request logs, structured filters, host/path
# metrics) that used to only ever show its 503 state locally — Loki was never
# part of dev-5. Mirrors deploy/gitops/base/{loki,log-shipper}.yaml (the Argo
# Applications) via direct Helm installs with dev-5 values (values/loki.values.yaml,
# values/log-shipper.values.yaml). bex-api reaches Loki through a host
# port-forward below (BEX_LOKI_URL). SingleBinary Loki + a type=app shipper only
# — the datastore/request pipelines are prod scale (see the values file).
echo "==> observability (Loki + log-shipper)"
helm repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
helm repo update grafana >/dev/null
helm upgrade --install loki grafana/loki --version '6.*' -n monitoring --create-namespace \
  -f "$ENVDIR/values/loki.values.yaml" >/dev/null
helm upgrade --install log-shipper grafana/alloy --version '1.*' -n monitoring \
  -f "$ENVDIR/values/log-shipper.values.yaml" >/dev/null
kubectl -n monitoring rollout status statefulset/loki --timeout=180s \
  || { echo "error: Loki did not become ready — dev-5's log store is down. Check 'kubectl -n monitoring describe pod loki-0' (an unbound PVC means no default StorageClass)." >&2; exit 1; }

# dsn CLUSTER DB — postgres:// DSN for a CNPG cluster's app user (mirrors
# scripts/auth-secrets.sh's dsn() helper, dev-5-auth namespace).
dsn() {
  local cluster="$1" db="$2" user pass
  { read -r user; read -r pass; } < <(kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" \
    -o go-template='{{.data.username | base64decode}}{{"\n"}}{{.data.password | base64decode}}')
  printf 'postgres://%s:%s@%s-rw.%s.svc.cluster.local:5432/%s?sslmode=require' \
    "$user" "$pass" "$cluster" "$DEV_AUTH_NS" "$db"
}

echo "==> out-of-band kratos/hydra secrets (freshly generated, dev-5-auth only)"
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
  -f deploy/gitops/base/values/kratos.values.yaml -f "$ENVDIR/values/kratos.values.yaml" --wait --timeout 12m
helm upgrade --install hydra ory/hydra --version 0.62.1 -n "$DEV_AUTH_NS" \
  -f deploy/gitops/base/values/hydra.values.yaml -f "$ENVDIR/values/hydra.values.yaml" --wait --timeout 12m

kill_if_running() {
  local pidfile="$1" pid
  if [ -f "$pidfile" ]; then
    pid="$(cat "$pidfile")"
    pkill -P "$pid" 2>/dev/null || true # the forward()'d loop's current kubectl child
    kill "$pid" 2>/dev/null || true
  fi
  rm -f "$pidfile"
}

# forward NAME SERVICE LOCALPORT:REMOTEPORT [NAMESPACE] — a self-healing
# port-forward: kind's Calico CNI occasionally resets an idle port-forward's
# connection ("lost connection to pod"), which would otherwise wedge bex-api's
# DB pool until a manual restart. Loops kubectl port-forward under a supervisor
# instead of forking it directly, so a drop reconnects on its own within ~1s.
# NAMESPACE defaults to the auth stack's; Loki lives in `monitoring` (w5/m62).
forward() {
  local name="$1" service="$2" ports="$3" ns="${4:-$DEV_AUTH_NS}"
  kill_if_running "$ENVDIR/.pids/pf-$name.pid"
  nohup bash -c "while true; do kubectl -n '$ns' port-forward service/$service $ports; sleep 1; done" \
    > "$ENVDIR/logs/pf-$name.log" 2>&1 & echo $! > "$ENVDIR/.pids/pf-$name.pid"
}

echo "==> port-forwards (self-healing)"
forward kratos kratos-public "$KRATOS_PUBLIC_PORT:80"
forward kratos-admin kratos-admin "$KRATOS_ADMIN_PORT:80"
forward hydra hydra-admin "$HYDRA_ADMIN_PORT:4445"
forward mailpit mailpit "$MAILPIT_HTTP_PORT:8025"
forward bex-db bex-db-rw "$BEX_DB_PORT:5432"
forward loki loki "$LOKI_PORT:3100" monitoring
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
    BEX_KRATOS_URL="http://localhost:$KRATOS_PUBLIC_PORT" \
    BEX_KRATOS_ADMIN_URL="http://localhost:$KRATOS_ADMIN_PORT" \
    BEX_HYDRA_ADMIN_URL="http://localhost:$HYDRA_ADMIN_PORT" \
    BEX_CP_DB_URI="$(hostDsn bex-db bex "$BEX_DB_PORT")" \
    BEX_CP_INSECURE="1" \
    BEX_CP_APPS_NAMESPACE="$DEV_NS" \
    BEX_ALLOW_INSECURE_AUTHZ="1" \
    BEX_BASE_DOMAIN="onbex.co" \
    BEX_LOKI_URL="http://localhost:$LOKI_PORT" \
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
echo "dev-5 (workstream w5) is up:"
echo "  kubeconfig:  $KUBECONFIG_FILE (KUBECONFIG=\$PWD/$KUBECONFIG_FILE kubectl -n $DEV_NS get keyvalues.app.bex.co)"
echo "  kratos:      http://localhost:$KRATOS_PUBLIC_PORT (admin: http://localhost:$KRATOS_ADMIN_PORT)"
echo "  hydra admin: http://localhost:$HYDRA_ADMIN_PORT"
echo "  mailpit UI:  http://localhost:$MAILPIT_HTTP_PORT"
echo "  loki:        http://localhost:$LOKI_PORT (bex-api BEX_LOKI_URL — log store)"
echo "  bex-api:     http://localhost:$BEX_API_PORT (log: $ENVDIR/logs/bex-api.log)"
echo
echo "start the dashboard against it:"
echo "  cd dashboard && VITE_API_URL=http://localhost:$BEX_API_PORT/graphql VITE_KRATOS_PUBLIC_URL=http://localhost:$KRATOS_PUBLIC_PORT yarn dev --port $DASHBOARD_PORT"
echo
echo "status: bash $ENVDIR/status.sh   |   tear down: bash $ENVDIR/down.sh"
