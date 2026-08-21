#!/usr/bin/env bash
# One dev-environment implementation for every workstream (w1/m72).
#
#   bash scripts/dev-env.sh <N> up      # bring dev-N up (idempotent)
#   bash scripts/dev-env.sh <N> down    # tear dev-N down
#   bash scripts/dev-env.sh <N> status  # health check + verification inventory
#   bash scripts/dev-env.sh <N> clean   # reclaim logs/ and bin/ (refuses while up)
#   bash scripts/dev-env.sh <N> env     # print the derived per-N values
#
# dev-N is workstream wN's isolated stack on the shared local kind/CAPD cluster
# ("bex"): its own Kratos + Hydra + Mailpit in namespace dev-N-auth, its own app
# namespace dev-N, reusing the shared cluster's CNPG and bex operators, plus a
# locally-built bex-api on ports derived from N. Nothing here touches the shared
# cluster's own `auth`/`bex-system` namespaces or another workstream's dev-N.
#
# This replaces ten copied harnesses (~7,000 tracked LOC) whose up.sh had already
# drifted into eight distinct variants — see .pm/w1/done/m72/evidence/variants.md
# for the union this implements and what was dropped.
#
# Requires: kind, kubectl, helm, go, openssl.
set -euo pipefail
cd "$(dirname "$0")/.." # repo root

TEMPLATES="scripts/dev-env"

usage() {
  cat >&2 <<'EOF'
usage: bash scripts/dev-env.sh <N> {up|down|status|clean|env}
  N   workstream number 1-10 (dev-N under .pm/wN/dev-N)
EOF
  exit 2
}

# --- per-N derivation -------------------------------------------------------
# Every per-N value comes from N alone. ports.env is a generated record of this
# derivation for humans (and what the values templates document); the script
# never reads it, so the two can no longer disagree the way the copies did.
#
# BEX_CP_IDENTITY is the highest-consequence value here: two harnesses sharing
# the CAPD cluster under one identity delete each other's tenant namespaces
# (.pm/w3/017.md, ControlPlaneLabel in lego/backend/internal/store/reconciler.go).
derive() {
  local n="$1"
  case "$n" in
  '' | *[!0-9]*) echo "error: N must be a number 1-10, got '${n:-<empty>}'" >&2; exit 2 ;;
  esac
  if [ "$n" -lt 1 ] || [ "$n" -gt 10 ]; then
    echo "error: N must be 1-10 (one per workstream), got '$n'" >&2
    exit 2
  fi
  N="$n"
  ENVDIR=".pm/w$N/dev-$N"
  DEV_NS="dev-$N"
  DEV_AUTH_NS="dev-$N-auth"
  local off=$((N * 10))
  DASHBOARD_PORT=$((50000 + off))
  KRATOS_PUBLIC_PORT=$((51000 + off))
  HYDRA_ADMIN_PORT=$((52000 + off))
  MAILPIT_HTTP_PORT=$((53000 + off))
  BEX_API_PORT=$((54000 + off))
  BEX_DB_PORT=$((55000 + off))
  BEX_CP_PORT=$((56000 + off))
  KRATOS_ADMIN_PORT=$((57000 + off))
  HYDRA_PUBLIC_PORT=$((58000 + off))
  LOKI_PORT=$((59000 + off))
  MAILPIT_SMTP_PORT=$((60000 + off))
  KUBECONFIG_FILE="$ENVDIR/.kubeconfig"

  # Optional per-workstream override, sourced after derivation so it can only
  # add to or adjust the shared defaults — never silently replace the identity
  # or namespace scheme (asserted below).
  if [ -f "$ENVDIR/override.env" ]; then
    # shellcheck disable=SC1091
    source "$ENVDIR/override.env"
  fi
  if [ "$DEV_NS" != "dev-$N" ] || [ "$DEV_AUTH_NS" != "dev-$N-auth" ]; then
    echo "error: $ENVDIR/override.env changed DEV_NS/DEV_AUTH_NS — the namespace scheme is not overridable (cross-N isolation depends on it)" >&2
    exit 2
  fi
}

print_env() {
  cat <<EOF
N=$N
ENVDIR=$ENVDIR
DEV_NS=$DEV_NS
DEV_AUTH_NS=$DEV_AUTH_NS
BEX_CP_IDENTITY=$DEV_NS
KUBECONFIG_FILE=$KUBECONFIG_FILE
DASHBOARD_PORT=$DASHBOARD_PORT
KRATOS_PUBLIC_PORT=$KRATOS_PUBLIC_PORT
KRATOS_ADMIN_PORT=$KRATOS_ADMIN_PORT
HYDRA_ADMIN_PORT=$HYDRA_ADMIN_PORT
HYDRA_PUBLIC_PORT=$HYDRA_PUBLIC_PORT
MAILPIT_HTTP_PORT=$MAILPIT_HTTP_PORT
MAILPIT_SMTP_PORT=$MAILPIT_SMTP_PORT
BEX_API_PORT=$BEX_API_PORT
BEX_DB_PORT=$BEX_DB_PORT
BEX_CP_PORT=$BEX_CP_PORT
LOKI_PORT=$LOKI_PORT
EOF
}

# render TEMPLATE — substitute the per-N values into a manifest template.
# __PLACEHOLDER__ rather than {{...}} so nothing collides with Helm/River
# templating inside the values files.
render() {
  sed -e "s/__N__/$N/g" \
    -e "s/__DEV_NS__/$DEV_NS/g" \
    -e "s/__DEV_AUTH_NS__/$DEV_AUTH_NS/g" \
    -e "s/__DASHBOARD_PORT__/$DASHBOARD_PORT/g" \
    -e "s/__KRATOS_PUBLIC_PORT__/$KRATOS_PUBLIC_PORT/g" \
    -e "s/__KRATOS_ADMIN_PORT__/$KRATOS_ADMIN_PORT/g" \
    -e "s/__HYDRA_ADMIN_PORT__/$HYDRA_ADMIN_PORT/g" \
    -e "s/__HYDRA_PUBLIC_PORT__/$HYDRA_PUBLIC_PORT/g" \
    -e "s/__MAILPIT_HTTP_PORT__/$MAILPIT_HTTP_PORT/g" \
    -e "s/__BEX_API_PORT__/$BEX_API_PORT/g" \
    -e "s/__LOKI_PORT__/$LOKI_PORT/g" \
    "$1"
}

# pids_dir_entries — the pid files this environment owns, if any.
pid_files() { ls "$ENVDIR"/.pids/*.pid 2>/dev/null || true; }

env_is_up() {
  local pidfile pid
  for pidfile in $(pid_files); do
    pid="$(cat "$pidfile" 2>/dev/null || true)"
    [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null && return 0
  done
  return 1
}

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
# connection ("lost connection to pod"), which would otherwise wedge bex-api's DB
# pool until a manual restart. Loops kubectl port-forward under a supervisor
# instead of forking it directly, so a drop reconnects on its own within ~1s.
# NAMESPACE defaults to the auth stack's; Loki lives in `monitoring`.
#
# The retry backs off and eventually gives up, which the ten copies did not: when
# the cluster underneath a forgotten environment dies, `while true; sleep 1` turns
# into an error-per-second log bomb. That is literally where the 3.9 GB w1/m72
# reclaimed came from — six port-forward logs of ~677 MB each, all written after
# the CAPD cluster went away. A forward that cannot connect for ~10 minutes is
# not healing; it is reporting that the environment is gone.
forward() {
  local name="$1" service="$2" ports="$3" ns="${4:-$DEV_AUTH_NS}"
  kill_if_running "$ENVDIR/.pids/pf-$name.pid"
  nohup bash -c "
    fails=0
    while true; do
      start=\$SECONDS
      kubectl -n '$ns' port-forward service/$service $ports
      # A forward that survived a while was healthy; reset the failure budget so
      # only a genuinely unreachable cluster trips the give-up path.
      if [ \$((SECONDS - start)) -ge 30 ]; then fails=0; else fails=\$((fails + 1)); fi
      if [ \$fails -ge 60 ]; then
        echo \"giving up on $service after \$fails immediate failures — the cluster looks gone; re-run 'bash scripts/dev-env.sh $N up'\"
        exit 1
      fi
      # 1s, then back off to 10s so a dead cluster costs one line per 10s.
      if [ \$fails -lt 5 ]; then sleep 1; else sleep 10; fi
    done" \
    >"$ENVDIR/logs/pf-$name.log" 2>&1 &
  echo $! >"$ENVDIR/.pids/pf-$name.pid"
}

# dsn CLUSTER DB — postgres:// DSN for a CNPG cluster's app user, in-cluster.
dsn() {
  local cluster="$1" db="$2" user pass
  {
    read -r user
    read -r pass
  } < <(kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" \
    -o go-template='{{.data.username | base64decode}}{{"\n"}}{{.data.password | base64decode}}')
  printf 'postgres://%s:%s@%s-rw.%s.svc.cluster.local:5432/%s?sslmode=require' \
    "$user" "$pass" "$cluster" "$DEV_AUTH_NS" "$db"
}

# hostDsn CLUSTER DB PORT — like dsn(), but through the host port-forward:
# bex-api runs on the HOST, so it cannot resolve *.svc.cluster.local.
hostDsn() {
  local cluster="$1" db="$2" port="$3" user pass
  {
    read -r user
    read -r pass
  } < <(kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" \
    -o go-template='{{.data.username | base64decode}}{{"\n"}}{{.data.password | base64decode}}')
  printf 'postgres://%s:%s@localhost:%s/%s?sslmode=disable' "$user" "$pass" "$port" "$db"
}

# --- preflight (from w5, the variant that had accumulated the most) ---------
preflight() {
  echo "==> preflight"
  local tool
  for tool in kind kubectl helm go openssl; do
    command -v "$tool" >/dev/null 2>&1 ||
      { echo "error: '$tool' not found on PATH — dev-$N needs kind/kubectl/helm/go/openssl" >&2; exit 1; }
  done
  kind get clusters 2>/dev/null | grep -qx bex ||
    { echo "error: kind cluster 'bex' not found — run 'bash scripts/mock-cluster.sh' first" >&2; exit 1; }
}

refresh_kubeconfig() {
  echo "==> refreshing kubeconfig for cluster 'bex'"
  if [ -f infra/local/bex.kubeconfig ]; then
    # mock-cluster.sh owns the authoritative CAPD workload-cluster kubeconfig.
    # Keep an isolated copy so down/status keep working if another harness
    # refreshes the shared file while this environment is running.
    cp infra/local/bex.kubeconfig "$KUBECONFIG_FILE"
  else
    kind get kubeconfig --name bex >"$KUBECONFIG_FILE" # legacy direct-kind path
  fi
  # kind/CAPD sometimes emit a 0.0.0.0 server address; the apiserver cert covers
  # 127.0.0.1, not 0.0.0.0, so pin it to loopback (w4/w9 both hit this).
  sed -i '' 's|https://0\.0\.0\.0:|https://127.0.0.1:|' "$KUBECONFIG_FILE" 2>/dev/null ||
    sed -i 's|https://0\.0\.0\.0:|https://127.0.0.1:|' "$KUBECONFIG_FILE"
  export KUBECONFIG="$PWD/$KUBECONFIG_FILE"
  kubectl cluster-info >/dev/null 2>&1 ||
    { echo "error: cluster 'bex' is unreachable via $KUBECONFIG_FILE — is the cluster running?" >&2; exit 1; }
  # A default StorageClass is a hard precondition: the three CNPG databases and
  # Loki all need PVCs. Without one they sit Pending, which reads like a dev-N
  # bug but is a broken shared cluster.
  kubectl get storageclass -o jsonpath='{range .items[*]}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{"\n"}{end}' 2>/dev/null | grep -qx true ||
    { echo "error: cluster 'bex' has no default StorageClass — CNPG databases and Loki cannot bind their PVCs. Reprovision it ('bash scripts/mock-cluster.sh')." >&2; exit 1; }
}

# ensure_cnpg — install the CNPG operator when the CRD is absent rather than
# assuming the shared cluster has it, then wait for it to be READY. Pins the
# same chart deploy/gitops/base/cnpg-operator.yaml does, minus the platform
# nodeSelector a disposable kind node does not carry.
ensure_cnpg() {
  echo "==> CNPG operator (self-installing if absent)"
  if ! kubectl get crd clusters.postgresql.cnpg.io >/dev/null 2>&1; then
    echo "    CNPG operator absent — installing cloudnative-pg 0.26.0"
    helm repo add cnpg https://cloudnative-pg.github.io/charts >/dev/null 2>&1 || true
    helm repo update cnpg >/dev/null
    helm upgrade --install cnpg cnpg/cloudnative-pg --version 0.26.0 \
      -n cnpg-system --create-namespace >/dev/null
  fi
  kubectl wait --for=condition=established crd/clusters.postgresql.cnpg.io --timeout=60s >/dev/null 2>&1 ||
    { echo "error: the CNPG 'clusters' CRD never established — operator install failed" >&2; exit 1; }
  # Pin the manager to the control-plane node: on the CAPD mock, worker-node pods
  # cannot reach the apiserver (OrbStack+Calico, docs/ADR004-app-deployment.md),
  # so a worker-scheduled CNPG manager crashloops.
  kubectl -n cnpg-system patch deploy cnpg-cloudnative-pg --type merge -p \
    '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
     "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}}}}' >/dev/null
  echo "    waiting for the CNPG operator to become ready..."
  if ! kubectl -n cnpg-system rollout status deploy/cnpg-cloudnative-pg --timeout=180s; then
    # A degraded local cluster can crashloop the operator WITHOUT blocking dev-N:
    # the Clusters it must create may already exist and keep running (their
    # Postgres pods do not need a live operator). Only hard-fail when they don't.
    if kubectl -n "$DEV_AUTH_NS" get cluster.postgresql.cnpg.io kratos-db hydra-db bex-db >/dev/null 2>&1; then
      echo "    warn: CNPG operator not ready, but kratos-db/hydra-db/bex-db already exist — continuing." >&2
    else
      {
        echo "error: the CNPG operator did not become ready AND the DB Clusters do not exist — dev-$N's databases cannot be created."
        echo "diagnose:"
        echo "  KUBECONFIG=$PWD/$KUBECONFIG_FILE kubectl -n cnpg-system describe pod -l app.kubernetes.io/name=cloudnative-pg"
        echo "  KUBECONFIG=$PWD/$KUBECONFIG_FILE kubectl -n cnpg-system logs deploy/cnpg-cloudnative-pg --tail=50"
      } >&2
      exit 1
    fi
  fi
}

ensure_observability() {
  # A minimal Loki + Alloy shipper, so every dev-N can prove the store-gated log
  # path (request logs, structured filters) instead of only the live-pod-log
  # fallback. Mirrors deploy/gitops/base/{loki,log-shipper}.yaml at local scale.
  # Set DEV_ENV_OBSERVABILITY=0 in override.env on a constrained machine.
  if [ "${DEV_ENV_OBSERVABILITY:-1}" != "1" ]; then
    echo "==> observability skipped (DEV_ENV_OBSERVABILITY=0)"
    return
  fi
  echo "==> observability (Loki + log-shipper)"
  helm repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
  helm repo update grafana >/dev/null
  render "$TEMPLATES/values/loki.values.yaml" >"$ENVDIR/.rendered-loki.values.yaml"
  render "$TEMPLATES/values/log-shipper.values.yaml" >"$ENVDIR/.rendered-log-shipper.values.yaml"
  helm upgrade --install loki grafana/loki --version '6.*' -n monitoring --create-namespace \
    -f "$ENVDIR/.rendered-loki.values.yaml" >/dev/null
  helm upgrade --install log-shipper grafana/alloy --version '1.*' -n monitoring \
    -f "$ENVDIR/.rendered-log-shipper.values.yaml" >/dev/null
  kubectl -n monitoring rollout status statefulset/loki --timeout=180s ||
    { echo "error: Loki did not become ready — dev-$N's log store is down ('kubectl -n monitoring describe pod loki-0'; an unbound PVC means no default StorageClass)." >&2; exit 1; }
}

# --- verbs ------------------------------------------------------------------
cmd_up() {
  mkdir -p "$ENVDIR/.pids" "$ENVDIR/logs" "$ENVDIR/bin"
  # Truncate on up: a long-lived environment's self-healing port-forwards
  # otherwise accumulate without bound (3.9 GB in one workstream before m72).
  : >"$ENVDIR/logs/bex-api.log"
  rm -f "$ENVDIR"/logs/pf-*.log

  preflight
  refresh_kubeconfig

  echo "==> control-plane platform label"
  # Base values select bex.co/pool=platform; local Ory overlays add a
  # control-plane selector because OrbStack cannot route worker pods reliably.
  kubectl label node -l node-role.kubernetes.io/control-plane \
    bex.co/pool=platform --overwrite >/dev/null

  echo "==> namespaces"
  kubectl create namespace "$DEV_AUTH_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl create namespace "$DEV_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl label namespace "$DEV_AUTH_NS" pod-security.kubernetes.io/enforce=privileged --overwrite >/dev/null

  echo "==> operator RBAC for the $DEV_NS apps namespace (secrets CRUD)"
  render "$TEMPLATES/rbac.yaml" | kubectl apply -f - >/dev/null

  ensure_cnpg

  echo "==> CNPG Clusters (kratos-db, hydra-db, bex-db)"
  local cluster
  for cluster in kratos hydra bex; do
    render "$TEMPLATES/db/$cluster-db.yaml" | kubectl apply -f - >/dev/null
  done
  for cluster in kratos-db hydra-db bex-db; do
    echo "    waiting for $cluster-app credentials secret..."
    for _ in $(seq 1 60); do
      kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" >/dev/null 2>&1 && break
      sleep 5
    done
    kubectl -n "$DEV_AUTH_NS" get secret "$cluster-app" >/dev/null 2>&1 ||
      { echo "error: $cluster never produced its -app credentials secret" >&2; exit 1; }
  done

  echo "==> mailpit"
  render "$TEMPLATES/mailpit/deployment.yaml" | kubectl apply -f - >/dev/null
  kubectl -n "$DEV_AUTH_NS" rollout status deployment/mailpit --timeout=120s

  echo "==> out-of-band kratos/hydra secrets (freshly generated, $DEV_AUTH_NS only)"
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
  render "$TEMPLATES/values/kratos.values.yaml" >"$ENVDIR/.rendered-kratos.values.yaml"
  render "$TEMPLATES/values/hydra.values.yaml" >"$ENVDIR/.rendered-hydra.values.yaml"
  helm upgrade --install kratos ory/kratos --version 0.62.1 -n "$DEV_AUTH_NS" \
    -f deploy/gitops/base/values/kratos.values.yaml \
    -f deploy/gitops/base/values/kratos-email-templates.values.yaml \
    -f "$ENVDIR/.rendered-kratos.values.yaml" --wait --timeout 5m
  helm upgrade --install hydra ory/hydra --version 0.62.1 -n "$DEV_AUTH_NS" \
    -f deploy/gitops/base/values/hydra.values.yaml -f "$ENVDIR/.rendered-hydra.values.yaml" --wait --timeout 5m

  ensure_observability

  echo "==> port-forwards (self-healing)"
  forward kratos kratos-public "$KRATOS_PUBLIC_PORT:80"
  forward kratos-admin kratos-admin "$KRATOS_ADMIN_PORT:80"
  forward hydra hydra-admin "$HYDRA_ADMIN_PORT:4445"
  forward hydra-public hydra-public "$HYDRA_PUBLIC_PORT:4444"
  forward mailpit mailpit "$MAILPIT_HTTP_PORT:8025"
  # SMTP side of the same Mailpit — bex-api's own invite mailer (BEX_SMTP_ADDR);
  # Kratos reaches Mailpit in-cluster and does not need this.
  forward mailpit-smtp mailpit "$MAILPIT_SMTP_PORT:1025"
  forward bex-db bex-db-rw "$BEX_DB_PORT:5432"
  if [ "${DEV_ENV_OBSERVABILITY:-1}" = "1" ]; then
    forward loki loki "$LOKI_PORT:3100" monitoring
  fi
  sleep 3

  echo "==> permanent platform OAuth2 clients (machine bootstrap + Render CLI)"
  export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-dev-$N-bootstrap-secret}"
  HYDRA_ADMIN_URL="http://localhost:$HYDRA_ADMIN_PORT" bash scripts/auth-bootstrap-client.sh >/dev/null

  echo "==> building bex-api"
  (cd lego/backend && go build -o "../../$ENVDIR/bin/bex-api" ./cmd/api)

  # Throwaway GitHub-App identity: a locally minted RSA key + fake id/slug make
  # wireGitHubApp construct the API client, which is all the blueprint fetcher
  # needs for ANONYMOUS public-repo fetches (token minting is never reached
  # without a stored git connection). No real credential is involved.
  if [ ! -f "$ENVDIR/.github-app-dev.pem" ]; then
    openssl genrsa -out "$ENVDIR/.github-app-dev.pem" 2048 2>/dev/null
  fi

  echo "==> starting bex-api on :$BEX_API_PORT (namespace $DEV_NS)"
  kill_if_running "$ENVDIR/.pids/bex-api.pid"
  local api_started=0 attempt
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
      BEX_OAUTH_ISSUER="http://localhost:$HYDRA_PUBLIC_PORT" \
      BEX_CP_DB_URI="$(hostDsn bex-db bex "$BEX_DB_PORT")" \
      BEX_CP_APPS_NAMESPACE="$DEV_NS" \
      BEX_CP_IDENTITY="$DEV_NS" \
      BEX_CP_INSECURE="1" \
      BEX_ALLOW_INSECURE_AUTHZ="1" \
      BEX_BASE_DOMAIN="onbex.co" \
      BEX_BUILD_NAMESPACE="bex-system" \
      BEX_REGION="local-capd" \
      BEX_DASHBOARD_URL="http://localhost:$DASHBOARD_PORT" \
      BEX_LOKI_URL="http://localhost:$LOKI_PORT" \
      BEX_SMTP_ADDR="localhost:$MAILPIT_SMTP_PORT" \
      BEX_SMTP_FROM="bex dev-$N <no-reply@dev-$N.local>" \
      BEX_REQUIRE_VERIFIED_INVITE_EMAIL="0" \
      BEX_GITHUB_APP_ID="1" \
      BEX_GITHUB_APP_PRIVATE_KEY="$(cat "$ENVDIR/.github-app-dev.pem")" \
      BEX_GITHUB_APP_SLUG="dev-local" \
      "./$ENVDIR/bin/bex-api" >>"$ENVDIR/logs/bex-api.log" 2>&1 &
    echo $! >"$ENVDIR/.pids/bex-api.pid"
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

  cat <<EOF

dev-$N (workstream w$N) is up:
  kubeconfig:  $KUBECONFIG_FILE (KUBECONFIG=\$PWD/$KUBECONFIG_FILE kubectl -n $DEV_NS get apps.app.bex.co)
  kratos:      http://localhost:$KRATOS_PUBLIC_PORT (admin: http://localhost:$KRATOS_ADMIN_PORT)
  hydra:       admin http://localhost:$HYDRA_ADMIN_PORT | public/issuer http://localhost:$HYDRA_PUBLIC_PORT
  mailpit UI:  http://localhost:$MAILPIT_HTTP_PORT (SMTP on :$MAILPIT_SMTP_PORT)
  loki:        http://localhost:$LOKI_PORT (bex-api BEX_LOKI_URL — log store)
  bex-api:     http://localhost:$BEX_API_PORT (log: $ENVDIR/logs/bex-api.log, truncated on each up)

start the dashboard against it:
  cd dashboard && HYDRA_ADMIN_URL=http://localhost:$HYDRA_ADMIN_PORT HYDRA_PUBLIC_URL=http://localhost:$HYDRA_PUBLIC_PORT VITE_API_URL=http://localhost:$BEX_API_PORT/graphql VITE_KRATOS_PUBLIC_URL=http://localhost:$KRATOS_PUBLIC_PORT VITE_KRATOS_SSR_URL=http://localhost:$KRATOS_PUBLIC_PORT yarn dev --port $DASHBOARD_PORT

status: bash scripts/dev-env.sh $N status   |   tear down: bash scripts/dev-env.sh $N down
EOF
}

cmd_down() {
  local dry_run="${1:-}"
  if [ "$dry_run" = "--dry-run" ]; then
    echo "would kill pids:"
    local pidfile
    for pidfile in $(pid_files); do echo "  $pidfile ($(cat "$pidfile"))"; done
    echo "would helm uninstall: kratos hydra (namespace $DEV_AUTH_NS)"
    echo "would delete namespaces: $DEV_AUTH_NS $DEV_NS"
    return
  fi

  echo "==> killing local processes (bex-api, port-forwards)"
  local pidfile pid
  for pidfile in $(pid_files); do
    pid="$(cat "$pidfile")"
    pkill -P "$pid" 2>/dev/null || true # forward()'s loop's current kubectl child
    kill "$pid" 2>/dev/null || true
    rm -f "$pidfile"
  done

  if [ -f "$KUBECONFIG_FILE" ]; then
    export KUBECONFIG="$PWD/$KUBECONFIG_FILE"
    echo "==> helm uninstall kratos/hydra"
    helm uninstall kratos hydra -n "$DEV_AUTH_NS" >/dev/null 2>&1 || true
    echo "==> deleting namespaces $DEV_AUTH_NS $DEV_NS"
    # Only this environment's own two namespaces, never the shared cluster's
    # auth/bex-system, never another workstream's dev-M.
    kubectl delete namespace "$DEV_AUTH_NS" "$DEV_NS" --ignore-not-found --wait=false
  else
    echo "no kubeconfig at $KUBECONFIG_FILE — nothing to delete in-cluster"
  fi

  echo "dev-$N down. Namespace deletion continues in the background (kubectl get ns to watch)."
}

cmd_status() {
  echo "== local processes =="
  local pidfile pid name
  for pidfile in $(pid_files); do
    pid="$(cat "$pidfile")"
    name="$(basename "$pidfile" .pid)"
    if kill -0 "$pid" 2>/dev/null; then echo "  $name: running (pid $pid)"; else echo "  $name: NOT running (stale pid $pid)"; fi
  done

  if [ -f "$KUBECONFIG_FILE" ]; then
    export KUBECONFIG="$PWD/$KUBECONFIG_FILE"
    echo
    echo "== $DEV_AUTH_NS pods =="
    kubectl -n "$DEV_AUTH_NS" get pods 2>&1 || true
    echo
    echo "== $DEV_NS resources =="
    kubectl -n "$DEV_NS" get apps.app.bex.co,keyvalues.app.bex.co,databases.app.bex.co 2>&1 || true
    echo
    # Control-plane identity (w6/m39). The orphan prune is cluster-scoped, so a
    # harness whose OWNER column is not $DEV_NS is pruning outside its own lane —
    # exactly the failure that let two dev-N stacks delete each other's tenants.
    echo "== tenant namespaces by control-plane owner (this harness: $DEV_NS) =="
    kubectl get ns -l app.kubernetes.io/managed-by=bex-controlplane \
      -L app.bex.co/control-plane,app.bex.co/workspace 2>&1 || true
  fi

  echo
  echo "== http checks =="
  curl -s -o /dev/null -w "  kratos    (:$KRATOS_PUBLIC_PORT): %{http_code}\n" "http://localhost:$KRATOS_PUBLIC_PORT/health/alive" || true
  curl -s -o /dev/null -w "  kratos-adm(:$KRATOS_ADMIN_PORT): %{http_code}\n" "http://localhost:$KRATOS_ADMIN_PORT/admin/health/alive" || true
  curl -s -o /dev/null -w "  hydra-adm (:$HYDRA_ADMIN_PORT): %{http_code}\n" "http://localhost:$HYDRA_ADMIN_PORT/health/ready" || true
  curl -s -o /dev/null -w "  hydra-pub (:$HYDRA_PUBLIC_PORT): %{http_code}\n" "http://localhost:$HYDRA_PUBLIC_PORT/health/ready" || true
  curl -s -o /dev/null -w "  bex-api   (:$BEX_API_PORT): %{http_code}\n" "http://localhost:$BEX_API_PORT/healthz" || true
  curl -s -o /dev/null -w "  dashboard (:$DASHBOARD_PORT): %{http_code}\n" "http://localhost:$DASHBOARD_PORT/" || true

  # == verification inventory ==
  # Unlike the loose health output above, these are ASSERTIONS: each prints
  # [ok]/[FAIL] and the section exits non-zero if any fails, so a broken
  # substrate is a hard, actionable signal rather than a status to interpret.
  echo
  echo "== verification inventory =="
  local inv_fail=0
  ok() { echo "  [ok]   $1"; }
  no() {
    echo "  [FAIL] $1"
    inv_fail=$((inv_fail + 1))
  }

  if [ -f "$KUBECONFIG_FILE" ]; then
    export KUBECONFIG="$PWD/$KUBECONFIG_FILE"
    if kubectl get storageclass -o jsonpath='{range .items[*]}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{"\n"}{end}' 2>/dev/null | grep -qx true; then
      ok "default StorageClass present"
    else
      no "no default StorageClass — CNPG DBs + Loki PVCs cannot bind (reprovision the cluster)"
    fi
    local cnpg_avail loki_ready
    cnpg_avail="$(kubectl -n cnpg-system get deploy cnpg-cloudnative-pg -o jsonpath='{.status.availableReplicas}' 2>/dev/null)"
    if kubectl get crd clusters.postgresql.cnpg.io >/dev/null 2>&1 && [ "${cnpg_avail:-0}" -ge 1 ]; then
      ok "CNPG operator ready"
    else
      no "CNPG operator not ready (absent CRD, or the manager's :9443 webhook won't bind)"
    fi
    if [ "${DEV_ENV_OBSERVABILITY:-1}" = "1" ]; then
      loki_ready="$(kubectl -n monitoring get statefulset loki -o jsonpath='{.status.readyReplicas}' 2>/dev/null)"
      if [ "${loki_ready:-0}" -ge 1 ]; then
        ok "Loki ready"
      else
        no "Loki not ready (check 'kubectl -n monitoring describe pod loki-0')"
      fi
    fi
  fi

  local api_code
  api_code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$BEX_API_PORT/healthz" 2>/dev/null)"
  if [ "$api_code" = "200" ]; then
    ok "bex-api reachable (:$BEX_API_PORT)"
  else
    no "bex-api not reachable (:$BEX_API_PORT healthz=$api_code)"
  fi

  echo
  if [ "$inv_fail" -eq 0 ]; then
    echo "verification inventory: ALL GREEN"
  else
    echo "verification inventory: $inv_fail check(s) FAILED"
    exit 1
  fi
}

cmd_clean() {
  if env_is_up; then
    echo "error: dev-$N is up — run 'bash scripts/dev-env.sh $N down' first (clean removes the logs a running environment is writing)" >&2
    exit 1
  fi
  local before
  before="$(du -sh "$ENVDIR" 2>/dev/null | cut -f1)"
  # ${ENVDIR:?} so an unset ENVDIR can never turn this into `rm -rf /bin`.
  rm -rf "${ENVDIR:?}/logs" "${ENVDIR:?}/bin"
  rm -f "${ENVDIR:?}"/.rendered-*.yaml
  echo "cleaned $ENVDIR (was ${before:-unknown}): removed logs/ and bin/"
}

# --- entrypoint -------------------------------------------------------------
[ $# -ge 2 ] || usage
derive "$1"
shift
case "$1" in
up) cmd_up ;;
down)
  shift
  cmd_down "${1:-}"
  ;;
status) cmd_status ;;
clean) cmd_clean ;;
env) print_env ;;
*) usage ;;
esac
