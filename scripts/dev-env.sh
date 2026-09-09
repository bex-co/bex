#!/usr/bin/env bash
# One dev-environment implementation for every workstream (w1/m72).
#
#   bash scripts/dev-env.sh <N> up        # bring dev-N up (idempotent)
#   bash scripts/dev-env.sh <N> down      # tear dev-N down
#   bash scripts/dev-env.sh <N> status    # health check + verification inventory
#   bash scripts/dev-env.sh <N> clean     # reclaim logs/ and bin/ (refuses while up)
#   bash scripts/dev-env.sh <N> env       # print the derived per-N values
#   bash scripts/dev-env.sh <N> agent-up  # add the ADR047 agent-session leg (opt-in)
#   bash scripts/dev-env.sh <N> agent-down # remove it, leaving the base stack up
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

AGENT_TEMPLATES="scripts/dev-env/agent"

usage() {
  cat >&2 <<'EOF'
usage: bash scripts/dev-env.sh <N> {up|down|status|clean|env|agent-up|agent-down|agent-netpol|agent-stub|agent-stub-off}
  N   workstream number 1-10 (dev-N under .pm/wN/dev-N)

agent-up adds the ADR047 cloud coding-agent-session leg on top of `up`:
OpenSandbox controller + lifecycle server, an in-cluster ssh-gateway, OpenFGA and
OpenBao. It is OPT-IN and EXCLUSIVE — see the agent_* functions for why only one
workstream can hold it at a time.
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
  # agent-session leg (opt-in, `agent-up`); same 1000-block-per-service scheme.
  OPENSANDBOX_PORT=$((61000 + off))
  AGENT_ATTACH_PORT=$((62000 + off))
  OPENFGA_PORT=$((63000 + off))
  OPENBAO_PORT=$((64000 + off))
  SANDBOX_EXEC_PORT=$((65000 + off))
  AGENT_HOST_API_HOST="bex-dev-host-api.kube-system.svc.cluster.local"
  AGENT_HOST_API_PORT=8091
  KUBECONFIG_FILE="$ENVDIR/.kubeconfig"
  AGENTDIR="$ENVDIR/.agent"

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
OPENSANDBOX_PORT=$OPENSANDBOX_PORT
AGENT_ATTACH_PORT=$AGENT_ATTACH_PORT
OPENFGA_PORT=$OPENFGA_PORT
OPENBAO_PORT=$OPENBAO_PORT
SANDBOX_EXEC_PORT=$SANDBOX_EXEC_PORT
AGENT_HOST_API_HOST=$AGENT_HOST_API_HOST
AGENT_HOST_API_PORT=$AGENT_HOST_API_PORT
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
    -e "s/__BEX_CP_PORT__/$BEX_CP_PORT/g" \
    -e "s/__SANDBOX_NS__/${SANDBOX_NS:-}/g" \
    -e "s|__OPENFGA_KEY__|${OPENFGA_KEY:-}|g" \
    -e "s|__BAO_DEV_TOKEN__|${BAO_DEV_TOKEN:-}|g" \
    -e "s|__HOST_DOCKER_IPV4__|${HOST_DOCKER_IPV4:-}|g" \
    -e "s|__AGENT_HOST_API_HOST__|${AGENT_HOST_API_HOST:-}|g" \
    -e "s|__AGENT_HOST_API_PORT__|${AGENT_HOST_API_PORT:-}|g" \
    -e "s|__HOST_API_REVISION__|${HOST_API_REVISION:-0}|g" \
    -e "s|__CONFIG_REVISION__|${CONFIG_REVISION:-0}|g" \
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
    # Do not race the replacement process against a listener that is still
    # draining after SIGTERM. This matters most when agent-up restarts bex-api:
    # its public listener can close before the control-plane :8091 listener,
    # making the first few replacements die with an avoidable address-in-use.
    local _
    for _ in $(seq 1 100); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.1
    done
  fi
  rm -f "$pidfile"
}

# A failed start can briefly leave an untracked bex-api holding one of dev-N's
# two listeners. Sweep only this environment's uniquely pathed binary before a
# replacement starts; another dev-M process cannot match this path.
kill_dev_api_processes() {
  local pids attempt
  pids="$(pgrep -f "$ENVDIR/bin/bex-api" 2>/dev/null || true)"
  [ -z "$pids" ] && return
  # shellcheck disable=SC2086 # pgrep emits a whitespace-separated PID list.
  kill $pids 2>/dev/null || true
  for attempt in $(seq 1 100); do
    pids="$(pgrep -f "$ENVDIR/bin/bex-api" 2>/dev/null || true)"
    [ -z "$pids" ] && return
    sleep 0.1
  done
  echo "error: an existing dev-$N bex-api process did not stop" >&2
  return 1
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

refresh_agent_gateway_forwards() {
  forward agent-attach bex-ssh-gateway "$AGENT_ATTACH_PORT:8083" bex-system
  forward sandbox-exec bex-ssh-gateway "$SANDBOX_EXEC_PORT:8081" bex-system

  local attempt attach_code exec_code
  for attempt in $(seq 1 50); do
    attach_code="$(curl -s -o /dev/null -w '%{http_code}' \
      "http://localhost:$AGENT_ATTACH_PORT/healthz" 2>/dev/null || true)"
    exec_code="$(curl -s -o /dev/null -w '%{http_code}' \
      "http://localhost:$SANDBOX_EXEC_PORT/healthz" 2>/dev/null || true)"
    if [ "$attach_code" = 200 ] && [ "$exec_code" = 200 ]; then
      return
    fi
    sleep 0.2
  done
  echo "error: ssh-gateway attach/exec forwards did not become healthy" >&2
  return 1
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
  # Re-derive the apiserver endpoint instead of trusting what was recorded.
  #
  # Docker assigns a published port at container START, so every CAPD port
  # CHANGES when the daemon restarts a container — after a reboot, or after the
  # laptop sleeps. The recorded kubeconfig then points at a port that is either
  # dead or, worse, now belongs to bex-lb's :8404 stats listener, which answers
  # plain HTTP and produces the thoroughly misleading
  #   "server gave HTTP response to HTTPS client"
  # That is the drift that stranded w10/003 ("the recorded kubeconfig API-server
  # port no longer matches") and cost a full session to re-diagnose.
  #
  # Prefer the CONTROL-PLANE container's own published 6443 over the load
  # balancer's. bex-lb is a single-backend haproxy in front of a single-CP mock,
  # so it buys no availability here but adds two observed failure modes: it
  # resolves the CP by container NAME, which Docker answers with an IPv6 address
  # the apiserver does not serve (haproxy then reports "Layer6 timeout" and
  # "backend has no server available" while the CP is perfectly healthy), and it
  # accumulates a session backlog from the self-healing port-forward retries.
  # Going straight to the CP sidesteps both. The lb remains the fallback.
  local apiport
  apiport="$(docker ps --filter label=io.x-k8s.kind.cluster=bex \
    --filter label=io.x-k8s.kind.role=control-plane --format '{{.Names}}' 2>/dev/null |
    head -1 | xargs -r -I{} docker port {} 6443/tcp 2>/dev/null | head -1 | sed 's/.*://')"
  [ -n "$apiport" ] ||
    apiport="$(docker port bex-lb 6443/tcp 2>/dev/null | head -1 | sed 's/.*://')"
  local lbport="$apiport"
  if [ -n "$lbport" ]; then
    sed -i '' -E "s#server: https://[0-9.]+:[0-9]+#server: https://127.0.0.1:$lbport#" \
      "$KUBECONFIG_FILE" 2>/dev/null ||
      sed -i -E "s#server: https://[0-9.]+:[0-9]+#server: https://127.0.0.1:$lbport#" \
        "$KUBECONFIG_FILE"
    # Repair the shared file too, so the next harness (and mock-cluster.sh's own
    # consumers) do not each rediscover the same drift.
    if [ -f infra/local/bex.kubeconfig ]; then
      sed -i '' -E "s#server: https://[0-9.]+:[0-9]+#server: https://127.0.0.1:$lbport#" \
        infra/local/bex.kubeconfig 2>/dev/null ||
        sed -i -E "s#server: https://[0-9.]+:[0-9]+#server: https://127.0.0.1:$lbport#" \
          infra/local/bex.kubeconfig
    fi
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

# agent_enabled — has `agent-up` provisioned the ADR047 leg for this dev-N?
# The marker is what makes `up` idempotent across the two shapes: re-running `up`
# on an agent-enabled environment must restart bex-api with the agent variables
# still set, not silently drop back to the base environment.
agent_enabled() { [ -f "$AGENTDIR/enabled" ]; }

# agent_secret NAME — a stable per-environment random secret, generated on first
# use and reused after. These are shared HMAC keys: bex-api and the in-cluster
# gateway must agree on them, so they cannot be regenerated per run.
agent_secret() {
  local name="$1"
  local f="$AGENTDIR/$name"
  if [ ! -f "$f" ]; then
    mkdir -p "$AGENTDIR"
    # OpenSandbox validates the control-plane token against
    # [A-Za-z0-9._~-]{32,256}; 48 alphanumerics satisfies that and is ample for
    # the HMAC keys.
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 48 >"$f"
    chmod 600 "$f"
  fi
  cat "$f"
}

# Resolve the host-reachable IPv4 from the CAPD control-plane container. A
# host-network proxy on that node can reach it even though ordinary Pods cannot;
# those Pods use the proxy's stable kube-system Service instead.
agent_resolve_host_docker() {
  local node
  node="$(kubectl get nodes -l node-role.kubernetes.io/control-plane \
    -o jsonpath='{.items[0].metadata.name}')"
  HOST_DOCKER_IPV4="$(docker exec "$node" getent ahostsv4 host.docker.internal 2>/dev/null \
    | awk 'NR == 1 { print $1 }')"
  [ -n "$HOST_DOCKER_IPV4" ] || {
    echo "error: could not resolve host.docker.internal to IPv4 from CAPD node $node" >&2
    exit 1
  }
}

# start_api — the single definition of how this environment runs bex-api. Both
# `up` and `agent-up` call it; agent-up only adds variables, so there is one
# place where the process environment is described.
start_api() {
  echo "==> starting bex-api on :$BEX_API_PORT (namespace $DEV_NS)"
  kill_if_running "$ENVDIR/.pids/bex-api.pid"
  kill_dev_api_processes

  # Base environment. Authz and the control-plane API are BOTH insecure here:
  # unset BEX_OPENFGA_URL means allow-all, which is fine while the only client
  # is a single-member local workspace.
  local -a env_args=(
    KUBECONFIG="$PWD/$KUBECONFIG_FILE"
    BEX_API_ADDR=":$BEX_API_PORT"
    BEX_CP_ADDR=":$BEX_CP_PORT"
    BEX_API_NAMESPACE="$DEV_NS"
    BEX_API_CORS_ORIGIN="http://localhost:$DASHBOARD_PORT"
    BEX_KRATOS_URL="http://localhost:$KRATOS_PUBLIC_PORT"
    BEX_KRATOS_ADMIN_URL="http://localhost:$KRATOS_ADMIN_PORT"
    BEX_HYDRA_ADMIN_URL="http://localhost:$HYDRA_ADMIN_PORT"
    BEX_OAUTH_ISSUER="http://localhost:$HYDRA_PUBLIC_PORT"
    BEX_CP_DB_URI="$(hostDsn bex-db bex "$BEX_DB_PORT")"
    BEX_CP_APPS_NAMESPACE="$DEV_NS"
    BEX_CP_IDENTITY="$DEV_NS"
    BEX_BASE_DOMAIN="onbex.co"
    BEX_BUILD_NAMESPACE="bex-system"
    BEX_REGION="local-capd"
    BEX_DASHBOARD_URL="http://localhost:$DASHBOARD_PORT"
    BEX_LOKI_URL="http://localhost:$LOKI_PORT"
    BEX_SMTP_ADDR="localhost:$MAILPIT_SMTP_PORT"
    BEX_SMTP_FROM="bex dev-$N <no-reply@dev-$N.local>"
    BEX_REQUIRE_VERIFIED_INVITE_EMAIL="0"
  )

  if agent_enabled; then
    # The agent-session leg replaces both insecure overrides with the real
    # thing. OpenFGA is not optional once the gateway is running: the gateway
    # calls requiredEnv("BEX_OPENFGA_URL") and re-checks the session relation at
    # ticket redemption, so bex-api must write its tuples to the SAME store or
    # every attach is denied. Likewise BEX_CP_TOKEN, because the OpenSandbox
    # tenant provider authenticates to the control-plane API with it.
    env_args+=(
      BEX_CP_TOKEN="$(agent_secret cp.token)"
      BEX_OPENFGA_URL="http://localhost:$OPENFGA_PORT"
      BEX_OPENFGA_TOKEN="$(agent_secret openfga.key)"
      BEX_OPENBAO_URL="http://localhost:$OPENBAO_PORT"
      BEX_OPENBAO_JWT_PATH="$PWD/$AGENTDIR/bex-api.jwt"
      BEX_OPENSANDBOX_URL="http://localhost:$OPENSANDBOX_PORT"
      BEX_AGENT_SESSION_IMAGE="docker.io/library/bex-agent-sandbox:dev"
      # The origin handed to the BROWSER for the attach stream, so it is the
      # host-side port-forward — not a cluster Service name.
      BEX_AGENT_SESSION_GATEWAY_URL="http://localhost:$AGENT_ATTACH_PORT"
      # The origin the SANDBOX dials, so it must be the in-cluster FQDN.
      BEX_AGENT_MODEL_PROXY_URL="http://bex-ssh-gateway.bex-system.svc.cluster.local:8084"
      BEX_SHELL_TICKET_SECRET="$(agent_secret shell-ticket.secret)"
      BEX_SANDBOX_EXEC_SECRET="$(agent_secret sandbox-exec.secret)"
      BEX_SANDBOX_EXEC_URL="http://localhost:$SANDBOX_EXEC_PORT/sandbox-exec"
    )
  else
    env_args+=(BEX_CP_INSECURE="1" BEX_ALLOW_INSECURE_AUTHZ="1")
  fi

  # GitHub App. `agent-up` can stage the real credentials (clone + draft PR need
  # installation tokens the throwaway identity cannot mint); otherwise the
  # locally minted key is enough for wireGitHubApp to construct the client,
  # which is all the ANONYMOUS public-repo blueprint fetch needs.
  if [ -f "$AGENTDIR/github-app.env" ] && [ -f "$AGENTDIR/github-app.pem" ]; then
    # shellcheck disable=SC1091
    source "$AGENTDIR/github-app.env"
    local github_var
    for github_var in BEX_GITHUB_APP_ID BEX_GITHUB_APP_SLUG \
      BEX_GITHUB_APP_CLIENT_ID BEX_GITHUB_APP_CLIENT_SECRET; do
      if [ -z "${!github_var:-}" ]; then
        echo "error: $AGENTDIR/github-app.env is missing $github_var" >&2
        return 1
      fi
    done
    env_args+=(
      BEX_GITHUB_APP_ID="$BEX_GITHUB_APP_ID"
      BEX_GITHUB_APP_SLUG="$BEX_GITHUB_APP_SLUG"
      BEX_GITHUB_APP_CLIENT_ID="$BEX_GITHUB_APP_CLIENT_ID"
      BEX_GITHUB_APP_CLIENT_SECRET="$BEX_GITHUB_APP_CLIENT_SECRET"
      BEX_GITHUB_APP_PRIVATE_KEY="$(cat "$AGENTDIR/github-app.pem")"
    )
  else
    env_args+=(
      BEX_GITHUB_APP_ID="1"
      BEX_GITHUB_APP_SLUG="dev-local"
      BEX_GITHUB_APP_PRIVATE_KEY="$(cat "$ENVDIR/.github-app-dev.pem")"
    )
  fi

  local api_started=0 attempt
  for attempt in $(seq 1 5); do
    nohup env "${env_args[@]}" \
      "./$ENVDIR/bin/bex-api" >>"$ENVDIR/logs/bex-api.log" 2>&1 &
    echo $! >"$ENVDIR/.pids/bex-api.pid"
    sleep 3
    if kill -0 "$(cat "$ENVDIR/.pids/bex-api.pid")" 2>/dev/null; then
      api_started=1
      break
    fi
    echo "    bex-api start attempt $attempt hit a transient listener/startup failure; retrying..."
    sleep 2
  done
  if [ "$api_started" -ne 1 ]; then
    echo "error: bex-api exited immediately — see $ENVDIR/logs/bex-api.log" >&2
    tail -20 "$ENVDIR/logs/bex-api.log" >&2
    exit 1
  fi
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

  # A stale dashboard/.env (written by `yarn local-bex`) silently breaks every
  # SSR GraphQL query: a .env VITE_SSR_API_URL beats an UNSET shell var, so a
  # dashboard started without the printed command's explicit VITE_SSR_API_URL
  # points SSR at a port nothing serves — every detail page cold-loads as
  # "Something went wrong" until the client self-heals (.pm/w1/080.md).
  if [ -f dashboard/.env ] && grep -q '^VITE_SSR_API_URL=' dashboard/.env; then
    local pinned_ssr
    pinned_ssr=$(sed -n 's/^VITE_SSR_API_URL=//p' dashboard/.env | tail -1)
    if [ "$pinned_ssr" != "http://localhost:$BEX_API_PORT/graphql" ]; then
      echo "WARNING: dashboard/.env pins VITE_SSR_API_URL=$pinned_ssr — not this env's http://localhost:$BEX_API_PORT/graphql."
      echo "         Start the dashboard with the exact command printed below (its explicit"
      echo "         VITE_SSR_API_URL wins over .env); omitting it leaves every SSR GraphQL"
      echo "         query failing with ECONNREFUSED and detail pages flashing an error."
    fi
  fi

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

  start_api

  cat <<EOF

dev-$N (workstream w$N) is up:
  kubeconfig:  $KUBECONFIG_FILE (KUBECONFIG=\$PWD/$KUBECONFIG_FILE kubectl -n $DEV_NS get apps.app.bex.co)
  kratos:      http://localhost:$KRATOS_PUBLIC_PORT (admin: http://localhost:$KRATOS_ADMIN_PORT)
  hydra:       admin http://localhost:$HYDRA_ADMIN_PORT | public/issuer http://localhost:$HYDRA_PUBLIC_PORT
  mailpit UI:  http://localhost:$MAILPIT_HTTP_PORT (SMTP on :$MAILPIT_SMTP_PORT)
  loki:        http://localhost:$LOKI_PORT (bex-api BEX_LOKI_URL — log store)
  bex-api:     http://localhost:$BEX_API_PORT (log: $ENVDIR/logs/bex-api.log, truncated on each up)

start the dashboard against it:
  cd dashboard && HYDRA_ADMIN_URL=http://localhost:$HYDRA_ADMIN_PORT HYDRA_PUBLIC_URL=http://localhost:$HYDRA_PUBLIC_PORT VITE_API_URL=http://localhost:$BEX_API_PORT/graphql VITE_SSR_API_URL=http://localhost:$BEX_API_PORT/graphql VITE_KRATOS_PUBLIC_URL=http://localhost:$KRATOS_PUBLIC_PORT VITE_KRATOS_SSR_URL=http://localhost:$KRATOS_PUBLIC_PORT yarn dev --port $DASHBOARD_PORT

  (VITE_SSR_API_URL is NOT optional: dashboard/.env pins it to local-bex's
   offline stub on :8099, and a .env value wins over an unset shell var — so
   omitting it leaves every SSR GraphQL query failing with ECONNREFUSED and
   each detail page cold-loading as "Something went wrong" until the client
   self-heals. Same reason VITE_KRATOS_SSR_URL is already spelled out.)

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

  if agent_enabled; then
    local openfga_body openbao_body opensandbox_body gateway_ready
    openfga_body="$(curl -fsS -H "Authorization: Bearer $(cat "$AGENTDIR/openfga.key" 2>/dev/null || true)" \
      "http://localhost:$OPENFGA_PORT/stores?page_size=100" 2>/dev/null || true)"
    if printf '%s' "$openfga_body" | grep -Eq '"name"[[:space:]]*:[[:space:]]*"bex"'; then
      ok "OpenFGA reachable with store 'bex'"
    else
      no "OpenFGA unavailable or store 'bex' missing (:$OPENFGA_PORT; re-run agent-up)"
    fi

    openbao_body="$(curl -fsS "http://localhost:$OPENBAO_PORT/v1/sys/health" 2>/dev/null || true)"
    if printf '%s' "$openbao_body" | tr -d '[:space:]' | grep -q '"sealed":false'; then
      ok "OpenBao reachable and unsealed"
    else
      no "OpenBao unavailable or sealed (:$OPENBAO_PORT; check deploy/openbao in $DEV_AUTH_NS)"
    fi

    opensandbox_body="$(curl -fsS "http://localhost:$OPENSANDBOX_PORT/health" 2>/dev/null || true)"
    if printf '%s' "$opensandbox_body" | tr -d '[:space:]' | grep -q '"status":"healthy"'; then
      ok "OpenSandbox lifecycle server healthy"
    else
      no "OpenSandbox unhealthy (:$OPENSANDBOX_PORT; check opensandbox-system logs)"
    fi

    if [ -f "$KUBECONFIG_FILE" ] \
      && kubectl get crd ciliumnetworkpolicies.cilium.io batchsandboxes.sandbox.opensandbox.io >/dev/null 2>&1; then
      ok "agent-session CRDs established (CiliumNetworkPolicy + BatchSandbox)"
    else
      no "agent-session CRDs missing (re-run agent-up)"
    fi

    if kubectl -n opensandbox-system exec deploy/opensandbox-server -- \
      python -c "import socket; s=socket.create_connection(('$AGENT_HOST_API_HOST',$AGENT_HOST_API_PORT),3); s.close()" \
      >/dev/null 2>&1; then
      ok "OpenSandbox reverse hop reaches host bex-api via Service (:$AGENT_HOST_API_PORT -> :$BEX_CP_PORT)"
    else
      no "OpenSandbox cannot reach host bex-api through kube-system/bex-dev-host-api"
    fi

    gateway_ready="$(kubectl -n bex-system get deploy bex-ssh-gateway \
      -o jsonpath='{.status.readyReplicas}' 2>/dev/null || true)"
    if [ "${gateway_ready:-0}" -ge 1 ] \
      && [ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$AGENT_ATTACH_PORT/healthz" 2>/dev/null)" = 200 ] \
      && [ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$SANDBOX_EXEC_PORT/healthz" 2>/dev/null)" = 200 ]; then
      ok "ssh-gateway ready (attach :$AGENT_ATTACH_PORT, exec :$SANDBOX_EXEC_PORT)"
    else
      no "ssh-gateway not ready or an aux forward is down (check bex-system/bex-ssh-gateway)"
    fi

    # Capabilities is tenant-authorized and intentionally cannot be probed with
    # the unbound platform bootstrap token. Reuse the live verifier's standard
    # inputs so status checks the ACTUAL public gate rather than inferring it
    # from process environment. The token is never printed.
    if [ -n "${BEX_API_TOKEN:-}" ]; then
      local caps_url caps_body
      caps_url="http://localhost:$BEX_API_PORT/v1/agent-sessions/capabilities"
      if [ -n "${BEX_VERIFY_OWNER_ID:-}" ]; then
        caps_url="$caps_url?ownerId=$BEX_VERIFY_OWNER_ID"
      fi
      caps_body="$(curl -fsS -H "Authorization: Bearer $BEX_API_TOKEN" "$caps_url" 2>/dev/null || true)"
      if printf '%s' "$caps_body" | tr -d '[:space:]' | grep -q '"enabled":true'; then
        ok "agent-session capabilities.enabled=true"
      else
        no "agent-session capabilities gate is false or unauthorized (check token/owner and bex-api agent env)"
      fi
    else
      no "capabilities check needs BEX_API_TOKEN (workspace member with can_operate; optional BEX_VERIFY_OWNER_ID)"
    fi
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

# cmd_agent_up — layer the ADR047 cloud coding-agent-session path onto a running
# dev-N (docs/ADR047-cloud-coding-agent-sessions.md, ADR042, ADR062).
#
# EXCLUSIVE, not per-workstream. Three of the four workloads below have names the
# PRODUCT hard-codes and therefore cannot be suffixed with N:
# lego/backend/internal/store/namespaces.go binds every `<ws>-sandbox` namespace's
# RoleBindings to `opensandbox-server`/`opensandbox-controller-manager` in
# `opensandbox-system` and to `bex-ssh-gateway` in `bex-system`, and the sandbox's
# Git remote defaults to that gateway's FQDN. Renaming them per N would strip the
# sandbox namespaces' RBAC and break the clone. So the leg is stamped with
# `app.bex.co/dev-env: dev-N` and refuses to steal it from another workstream.
#
# The base `up` deliberately ships without any of this — see the table in
# .pm/wN/dev-N/README.md — because the datastore CRUD flows it targets need none
# of it and it is a lot of extra load on a laptop.
cmd_agent_up() {
  preflight
  refresh_kubeconfig
  env_is_up || {
    echo "error: dev-$N is not up — run 'bash scripts/dev-env.sh $N up' first" >&2
    exit 1
  }

  local owner
  owner=$(kubectl get ns opensandbox-system -o jsonpath='{.metadata.labels.app\.bex\.co/dev-env}' 2>/dev/null || true)
  if [ -n "$owner" ] && [ "$owner" != "$DEV_NS" ]; then
    echo "error: the agent-session leg is currently held by $owner." >&2
    echo "       It is cluster-wide and exclusive (product-fixed workload names)." >&2
    echo "       Release it with: bash scripts/dev-env.sh ${owner#dev-} agent-down" >&2
    exit 1
  fi

  mkdir -p "$AGENTDIR"
  chmod 700 "$AGENTDIR"
  OPENFGA_KEY="$(agent_secret openfga.key)"
  BAO_DEV_TOKEN="$(agent_secret bao.token)"
  CONFIG_REVISION="$(date -u +%Y%m%d%H%M%S)"
  agent_resolve_host_docker
  HOST_API_REVISION="$HOST_DOCKER_IPV4-$BEX_CP_PORT"

  echo "==> building the agent-session images (arm64-native where the pinned digest is not)"
  agent_build_images

  echo "==> reconciling agent images onto every node's containerd (no registry on the mock)"
  agent_reconcile_node_images

  echo "==> tenant-namespace ClusterRoles (shared; the RoleBindings bex-api stamps point at these)"
  kubectl apply -f deploy/gitops/base/tenant-namespace-clusterroles.yaml >/dev/null

  echo "==> reverse hop Service (cluster Pods -> host bex-api :$BEX_CP_PORT)"
  render "$AGENT_TEMPLATES/host-api.yaml" | kubectl apply -f - >/dev/null
  kubectl -n kube-system rollout status deploy/bex-dev-host-api-proxy --timeout=180s

  # Inert CiliumNetworkPolicy CRD. Without it EVERY agent session fails at
  # dispatch with `no matches for kind "CiliumNetworkPolicy"`, because bex-api
  # mints a per-session egress policy. See the file header: this registers a
  # schema, it does not enforce anything on this Calico cluster.
  echo "==> CiliumNetworkPolicy CRD shim (schema only — NOT enforced here)"
  kubectl apply -f "$AGENT_TEMPLATES/cilium-crd-shim.yaml" >/dev/null

  echo "==> OpenFGA + OpenBao ($DEV_AUTH_NS)"
  render "$AGENT_TEMPLATES/authz-secrets.yaml" | kubectl apply -f - >/dev/null
  kubectl -n "$DEV_AUTH_NS" rollout status deploy/openfga --timeout=180s
  kubectl -n "$DEV_AUTH_NS" rollout status deploy/openbao --timeout=180s

  echo "==> OpenSandbox controller + lifecycle server (opensandbox-system)"
  helm upgrade --install opensandbox-controller deploy/gitops/charts/opensandbox-controller \
    -n opensandbox-system --create-namespace \
    --set controller.image.repository=opensandbox-controller \
    --set controller.image.tag=v0.2.0-bex \
    --set controller.image.pullPolicy=IfNotPresent \
    --set controller.leaderElection.enabled=false \
    --set-string 'controller.nodeSelector.node-role\.kubernetes\.io/control-plane=' \
    --set 'controller.tolerations[0].key=node-role.kubernetes.io/control-plane' \
    --set 'controller.tolerations[0].effect=NoSchedule' \
    --wait --timeout 5m >/dev/null
  kubectl label ns opensandbox-system "app.bex.co/dev-env=$DEV_NS" --overwrite >/dev/null
  # The control-plane token is substituted here rather than in render() so it
  # never reaches a template that any other manifest path renders.
  kubectl -n opensandbox-system create configmap opensandbox-config \
    --from-file=sandbox-local.toml=<(render "$AGENT_TEMPLATES/sandbox-local.toml" |
      sed "s|__BEX_CP_TOKEN__|$(agent_secret cp.token)|g") \
    --from-file=batchsandbox-template.yaml="$AGENT_TEMPLATES/batchsandbox-template.local.yaml" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  render "$AGENT_TEMPLATES/opensandbox-server.yaml" | kubectl apply -f - >/dev/null
  kubectl -n opensandbox-system rollout status deploy/opensandbox-server --timeout=300s

  echo "==> OpenBao: kubernetes auth + tenants/ KV v2 + the bex-api role"
  agent_seed_openbao

  echo "==> minting a bex-api ServiceAccount token (bex-api runs off-cluster here)"
  kubectl -n bex-system create token bex-api --duration=24h >"$AGENTDIR/bex-api.jwt"
  chmod 600 "$AGENTDIR/bex-api.jwt"

  echo "==> ssh-gateway (in-cluster: it dials sandbox Pod IPs, unroutable from macOS)"
  agent_gateway_secrets
  render "$AGENT_TEMPLATES/ssh-gateway.yaml" | kubectl apply -f - >/dev/null
  kubectl -n bex-system rollout status deploy/bex-ssh-gateway --timeout=180s

  echo "==> port-forwards"
  forward opensandbox opensandbox-server "$OPENSANDBOX_PORT:8077" opensandbox-system
  refresh_agent_gateway_forwards
  forward openfga openfga "$OPENFGA_PORT:8080"
  forward openbao openbao "$OPENBAO_PORT:8200"
  sleep 4

  echo "==> OpenFGA store + authorization model"
  OPENFGA_URL="http://127.0.0.1:$OPENFGA_PORT" OPENFGA_PRESHARED_KEY="$OPENFGA_KEY" \
    bash scripts/authz-model.sh

  touch "$AGENTDIR/enabled"
  echo "==> restarting bex-api with the agent-session environment"
  start_api

  cat <<EOF

dev-$N agent-session leg is up:
  opensandbox:  http://localhost:$OPENSANDBOX_PORT (bex-api BEX_OPENSANDBOX_URL)
  agent attach: http://localhost:$AGENT_ATTACH_PORT (browser SSE; BEX_AGENT_SESSION_GATEWAY_URL)
  openfga:      http://localhost:$OPENFGA_PORT   openbao: http://localhost:$OPENBAO_PORT

Still required before a real repository session can run (per workspace):
  1. a GitHub App connection — stage real credentials in
     $AGENTDIR/github-app.env (BEX_GITHUB_APP_ID/_SLUG/_CLIENT_ID/
     _CLIENT_SECRET) + github-app.pem, re-run 'agent-up', then connect the App;
     the throwaway identity cannot complete OAuth or mint installation tokens.
  2. the BYO model key (ADR062):
     bao kv put tenants/default/agent-sessions/<workspaceId>/model-key \\
       BEX_AGENT_MODEL_API_KEY=<key>
  3. per sandbox namespace, the local stand-in for the Cilium allows:
     bash scripts/dev-env.sh $N agent-netpol <workspaceId>

Check readiness: GET /v1/agent-sessions/capabilities?ownerId=<workspaceId>
EOF
}

# agent_build_images — build what the leg runs. Only the OpenSandbox server needs
# a local Dockerfile: deploy/opensandbox/server.Dockerfile pins its python base by
# a single-architecture DIGEST, so on Apple Silicon it yields an amd64 image a
# local arm64 containerd cannot exec. The other three build natively.
#
# An image is rebuilt when it is missing, when any file under its build source
# is newer than the image (a code change must reach the cluster — the old
# build-only-when-absent guard silently ran stale gateway/bex-api code,
# .pm/w1/081.md), or when AGENT_REBUILD=1 forces it. Node distribution no longer
# depends on an invocation-local reload list: agent_reconcile_node_images
# compares Docker config Ids to each node's crictl image Id every run, so a
# retry after a partial import converges without AGENT_REBUILD or source edits
# (w1/m137). The config-revision stamp on the Deployments then rolls the pods.

# The four images agent-up distributes into every CAPD node's containerd.
AGENT_NODE_IMAGES=(
  bex-lego:dev
  opensandbox-server:0.2.2-local
  opensandbox-controller:v0.2.0-bex
  bex-agent-sandbox:dev
)

# agent_image_ref <tag> — containerd/CRI reference for a locally-tagged image.
agent_image_ref() {
  printf 'docker.io/library/%s\n' "$1"
}

# agent_image_absent_err <stderr> — true when inspect failed because the image
# is missing (vs a real inspection failure).
agent_image_absent_err() {
  printf '%s' "$1" | grep -qiE 'No such image|No such object|not found|does not exist|NotFound'
}

# agent_local_image_identity <img> — comparable Docker config Id on stdout.
# Exit 0=ok, 2=missing, 1=inspection failure. Do not compare this to ctr's
# manifest DIGEST column — that is a different digest family (w1/m137).
agent_local_image_identity() {
  local img="$1" out
  if ! out=$(docker image inspect -f '{{.Id}}' "$img" 2>&1); then
    agent_image_absent_err "$out" && return 2
    echo "error: inspecting local image $img: $out" >&2
    return 1
  fi
  if [ -z "$out" ] || [ "$out" = "<no value>" ]; then
    echo "error: local image $img has empty identity" >&2
    return 1
  fi
  printf '%s\n' "$out"
}

# agent_node_image_identity <node> <img> — comparable CRI config Id on stdout.
# Uses crictl (status.id == Docker .Id after docker-save|ctr-import), not ctr's
# manifest digest. Exit 0=ok, 2=missing, 1=inspection failure.
agent_node_image_identity() {
  local node="$1" img="$2" ref out
  ref=$(agent_image_ref "$img")
  if ! out=$(docker exec "$node" crictl inspecti --output go-template \
    --template '{{.status.id}}' "$ref" 2>&1); then
    agent_image_absent_err "$out" && return 2
    echo "error: inspecting $img on node $node: $out" >&2
    return 1
  fi
  if [ -z "$out" ] || [ "$out" = "<no value>" ]; then
    echo "error: $img on node $node has empty identity" >&2
    return 1
  fi
  printf '%s\n' "$out"
}

# agent_image_import_needed <desired-id> <observed-id-or-empty>
# Exit 0 when the node must import; 1 when the observed identity already matches.
agent_image_import_needed() {
  local desired="$1" observed="${2:-}"
  [ -n "$desired" ] && [ -n "$observed" ] && [ "$desired" = "$observed" ] && return 1
  return 0
}

# agent_import_image_to_node <img> <node> <desired-id> — replace the node's tag
# and verify the imported identity matches before returning.
agent_import_image_to_node() {
  local img="$1" node="$2" desired="$3" ref observed
  ref=$(agent_image_ref "$img")
  docker exec "$node" ctr -n k8s.io images rm "$ref" >/dev/null 2>&1 || true
  echo "    $img -> $node (desired ${desired#sha256:})"
  if ! docker save "$img" | docker exec -i "$node" ctr -n k8s.io images import - >/dev/null; then
    echo "error: importing $img onto node $node failed" >&2
    return 1
  fi
  observed=$(agent_node_image_identity "$node" "$img") || {
    echo "error: post-import inspection of $img on node $node failed" >&2
    return 1
  }
  if [ "$observed" != "$desired" ]; then
    echo "error: post-import identity mismatch for $img on node $node" >&2
    echo "       desired=$desired" >&2
    echo "       observed=$observed" >&2
    return 1
  fi
}

# agent_reconcile_node_images — ensure every node has the intended identity for
# all four agent images. Missing or mismatched tags are imported; matching Ids
# are skipped. Any inspect/import/verify failure aborts before workload rollout.
agent_reconcile_node_images() {
  local img node desired observed rc
  local -a nodes=()
  while IFS= read -r node; do
    [ -n "$node" ] && nodes+=("$node")
  done < <(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
  if [ "${#nodes[@]}" -eq 0 ]; then
    echo "error: no cluster nodes to load agent images onto" >&2
    return 1
  fi

  for img in "${AGENT_NODE_IMAGES[@]}"; do
    desired=$(agent_local_image_identity "$img") || {
      rc=$?
      if [ "$rc" -eq 2 ]; then
        echo "error: required local image $img is missing after build" >&2
      fi
      return 1
    }
    for node in "${nodes[@]}"; do
      if ! observed=$(agent_node_image_identity "$node" "$img"); then
        rc=$?
        [ "$rc" -eq 2 ] || return 1
        observed=""
      fi
      if agent_image_import_needed "$desired" "$observed"; then
        agent_import_image_to_node "$img" "$node" "$desired" || return 1
      else
        echo "    $img @ $node already ${desired#sha256:}"
      fi
    done
  done
}

# image_stale <image> <src>... — true when the image is missing, AGENT_REBUILD=1,
# or any file under a src path is newer than the image's Created timestamp.
image_stale() {
  local img="$1"
  shift
  if [ "${AGENT_REBUILD:-0}" = "1" ]; then return 0; fi
  local created
  created=$(docker image inspect -f '{{.Created}}' "$img" 2>/dev/null) || return 0
  created="${created%%.*}Z" # trim fractional seconds for find(1)'s -newermt
  local src
  for src in "$@"; do
    if [ -n "$(find "$src" -newermt "$created" -print -quit 2>/dev/null)" ]; then
      return 0
    fi
  done
  return 1
}

agent_build_images() {
  if image_stale bex-agent-sandbox:dev lego; then
    (cd lego && docker build -q -f agent-image/Dockerfile -t bex-agent-sandbox:dev . >/dev/null)
  fi
  if image_stale bex-lego:dev lego; then
    (cd lego && docker build -q -t bex-lego:dev . >/dev/null)
  fi
  if image_stale opensandbox-server:0.2.2-local deploy/opensandbox "$AGENT_TEMPLATES/opensandbox-server.local.Dockerfile"; then
    docker build -q -f "$AGENT_TEMPLATES/opensandbox-server.local.Dockerfile" \
      -t opensandbox-server:0.2.2-local deploy/opensandbox >/dev/null
  fi
  # The chart's image helper prepends "v" to a semver-looking tag, so the built
  # tag must already carry it or the Deployment references an image that is not
  # on the node.
  if image_stale opensandbox-controller:v0.2.0-bex deploy/opensandbox; then
    docker build -q -f deploy/opensandbox/controller.Dockerfile \
      -t opensandbox-controller:v0.2.0-bex deploy/opensandbox >/dev/null
  fi
}

# agent_seed_openbao — dev-mode OpenBao needs no init/unseal, only the kubernetes
# auth method plus the tenants/ mount, policy and role bex-api logs in with.
#
# This deliberately does NOT use scripts/bao-init.sh: that script writes
# BAO_ROOT_TOKEN and the unseal quorum into the repo's .env, which holds the
# PRODUCTION OpenBao credentials. Pointing it at a disposable local cluster would
# overwrite them.
agent_seed_openbao() {
  local pod
  pod=$(kubectl -n "$DEV_AUTH_NS" get pod -l app.kubernetes.io/name=openbao \
    -o jsonpath='{.items[0].metadata.name}')
  local -a bx=(kubectl -n "$DEV_AUTH_NS" exec "$pod" -- env
    BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN="$BAO_DEV_TOKEN")
  "${bx[@]}" bao auth enable kubernetes >/dev/null 2>&1 || true
  "${bx[@]}" bao write auth/kubernetes/config \
    kubernetes_host=https://kubernetes.default.svc >/dev/null
  "${bx[@]}" bao secrets enable -path=tenants -version=2 kv >/dev/null 2>&1 || true
  "${bx[@]}" sh -c 'bao policy write tenants-rw - <<EOF
path "tenants/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
EOF' >/dev/null
  "${bx[@]}" bao write auth/kubernetes/role/bex-api \
    bound_service_account_names=bex-api \
    bound_service_account_namespaces=bex-system \
    policies=tenants-rw ttl=1h >/dev/null
}

# agent_gateway_secrets — the gateway's DB credential, host key and the HMAC keys
# it must share byte-for-byte with bex-api.
agent_gateway_secrets() {
  local user pass uri
  user=$(kubectl -n "$DEV_AUTH_NS" get secret bex-db-app -o jsonpath='{.data.username}' | base64 -d)
  pass=$(kubectl -n "$DEV_AUTH_NS" get secret bex-db-app -o jsonpath='{.data.password}' | base64 -d)
  uri="postgres://$user:$pass@bex-db-rw.$DEV_AUTH_NS.svc:5432/bex?sslmode=disable"
  [ -f "$AGENTDIR/ssh_host_ed25519_key" ] ||
    ssh-keygen -t ed25519 -N "" -C "dev-$N gateway" -f "$AGENTDIR/ssh_host_ed25519_key" >/dev/null
  kubectl -n bex-system create secret generic bex-dev-gateway \
    --from-literal=db_uri="$uri" \
    --from-literal=openfga_token="$OPENFGA_KEY" \
    --from-literal=shell_ticket_secret="$(agent_secret shell-ticket.secret)" \
    --from-literal=sandbox_exec_secret="$(agent_secret sandbox-exec.secret)" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl -n bex-system create secret generic bex-dev-gateway-hostkey \
    --from-file=ssh_host_ed25519_key="$AGENTDIR/ssh_host_ed25519_key" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
}

# cmd_agent_netpol WORKSPACE — install the local stand-in for the cluster-wide
# Cilium allows into one `<ws>-sandbox` namespace. See the header of
# scripts/dev-env/agent/sandbox-netpol.yaml for why this is needed at all and why
# it must run per workspace (bex-api creates the namespace on demand).
cmd_agent_netpol() {
  local ws="${1:-}"
  [ -n "$ws" ] || {
    echo "usage: bash scripts/dev-env.sh $N agent-netpol <workspaceId>" >&2
    exit 2
  }
  refresh_kubeconfig
  SANDBOX_NS="$ws-sandbox"
  kubectl get ns "$SANDBOX_NS" >/dev/null 2>&1 || {
    echo "error: namespace $SANDBOX_NS does not exist yet — create the workspace first" >&2
    exit 1
  }
  render "$AGENT_TEMPLATES/sandbox-netpol.yaml" | kubectl apply -f -
}

# cmd_agent_stub — stand up the local model stub and point the gateway's upstream
# hop at it, so a turn can complete without a provider credential.
#
# ⚠️  A TEST DOUBLE, NOT A MODEL — see scripts/dev-env/agent/model-stub.py.
#
# Why it has to interpose HERE. bex pins each agent profile to its registered
# provider endpoint (agentsession.RegisteredModelEndpoint rejects any other
# `agentConfig.modelEndpoint`), which is a deliberate product rule this must not
# weaken. The only seam left is the gateway's own upstream request, so the stub
# serves TLS as `api.anthropic.com` behind a hostAlias, and the gateway is given
# a CA bundle that trusts it via Go's SSL_CERT_FILE. No product code changes.
#
# The bundle is the stub CA appended to a real root store, not the CA alone, so
# every other TLS destination (GitHub, the registry) keeps verifying normally.
cmd_agent_stub() {
  preflight
  refresh_kubeconfig
  agent_enabled || {
    echo "error: run 'bash scripts/dev-env.sh $N agent-up' first" >&2
    exit 1
  }
  local d="$AGENTDIR/modelstub"
  mkdir -p "$d"
  CONFIG_REVISION="$(date -u +%Y%m%d%H%M%S)"

  if [ ! -f "$d/tls.crt" ]; then
    echo "==> minting the stub CA + api.anthropic.com server certificate"
    openssl req -x509 -newkey rsa:2048 -nodes -keyout "$d/ca.key" -out "$d/ca.crt" \
      -days 30 -subj "/CN=dev-$N model stub CA" 2>/dev/null
    openssl req -newkey rsa:2048 -nodes -keyout "$d/tls.key" -out "$d/tls.csr" \
      -subj "/CN=api.anthropic.com" 2>/dev/null
    printf 'subjectAltName=DNS:api.anthropic.com\nextendedKeyUsage=serverAuth\n' >"$d/ext.cnf"
    openssl x509 -req -in "$d/tls.csr" -CA "$d/ca.crt" -CAkey "$d/ca.key" \
      -CAcreateserial -out "$d/tls.crt" -days 30 -extfile "$d/ext.cnf" 2>/dev/null
    chmod 600 "$d"/*.key
  fi

  echo "==> model stub ($DEV_NS)"
  kubectl -n "$DEV_NS" create secret generic model-stub-tls \
    --from-file=tls.crt="$d/tls.crt" --from-file=tls.key="$d/tls.key" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl -n "$DEV_NS" create configmap model-stub-src \
    --from-file=model-stub.py="$AGENT_TEMPLATES/model-stub.py" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  render "$AGENT_TEMPLATES/model-stub.yaml" | kubectl apply -f - >/dev/null
  kubectl -n "$DEV_NS" rollout status deploy/model-stub --timeout=180s

  echo "==> teaching the gateway to trust it"
  # Take the root store from the stub's own image rather than the host: it must
  # be the bundle a Linux container expects, and this avoids a macOS/Linux skew.
  kubectl -n "$DEV_NS" exec deploy/model-stub -- cat /etc/ssl/certs/ca-certificates.crt \
    >"$d/system-ca.crt"
  cat "$d/system-ca.crt" "$d/ca.crt" >"$d/bundle.crt"
  kubectl -n bex-system create configmap model-stub-ca --from-file=bundle.crt="$d/bundle.crt" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  local stubip
  stubip=$(kubectl -n "$DEV_NS" get svc model-stub -o jsonpath='{.spec.clusterIP}')
  # STRATEGIC merge, never `--type merge`: a JSON merge patch REPLACES the env
  # and volume lists wholesale, which silently strips all 18 BEX_* variables and
  # leaves a gateway that starts but authorizes nothing.
  kubectl -n bex-system patch deploy bex-ssh-gateway --type strategic -p "{
    \"spec\": {\"template\": {\"spec\": {
      \"hostAliases\": [{\"ip\": \"$stubip\", \"hostnames\": [\"api.anthropic.com\"]}],
      \"containers\": [{
        \"name\": \"ssh-gateway\",
        \"env\": [{\"name\": \"SSL_CERT_FILE\", \"value\": \"/etc/model-stub-ca/bundle.crt\"}],
        \"volumeMounts\": [{\"name\": \"model-stub-ca\", \"mountPath\": \"/etc/model-stub-ca\", \"readOnly\": true}]
      }],
      \"volumes\": [{\"name\": \"model-stub-ca\", \"configMap\": {\"name\": \"model-stub-ca\"}}]
    }}}
  }" >/dev/null
  kubectl -n bex-system rollout status deploy/bex-ssh-gateway --timeout=180s
  # A kubectl port-forward is bound to one Pod, not the Service forever. The
  # gateway rollout above invalidates both existing tunnels; restart them now
  # so the first completer/attach call cannot be the one that discovers the
  # dead Pod and leaves a session waiting for the next reconciliation pass.
  refresh_agent_gateway_forwards

  cat <<EOF

model stub active for dev-$N. A repo-less session now completes a real turn:

  curl -X POST -H "Authorization: Bearer \$TOKEN" -H 'Content-Type: application/json' \\
    -d '{"ownerId":"<workspaceId>","agentConfig":{"agent":"claude","task":"say hi"}}' \\
    http://localhost:$BEX_API_PORT/v1/agent-sessions

⚠️  Responses are canned. This proves bex's transport, credential mint and
   transcript — NOT the provider. Undo with: dev-env.sh $N agent-stub-off
EOF
}

# cmd_agent_stub_off — drop the stub and give the gateway its normal trust store
# back, so the same environment can be pointed at a real provider key.
cmd_agent_stub_off() {
  refresh_kubeconfig
  kubectl -n "$DEV_NS" delete deploy/model-stub svc/model-stub \
    configmap/model-stub-src secret/model-stub-tls --ignore-not-found >/dev/null
  # Recreate the Deployment from its template. `kubectl apply` alone preserves
  # fields added by the strategic stub patch, which would leave a dangling CA
  # mount after its ConfigMap is removed.
  kubectl -n bex-system delete deploy bex-ssh-gateway --ignore-not-found --wait=true >/dev/null
  kubectl -n bex-system delete configmap model-stub-ca --ignore-not-found >/dev/null
  CONFIG_REVISION="$(date -u +%Y%m%d%H%M%S)"
  render "$AGENT_TEMPLATES/ssh-gateway.yaml" | kubectl apply -f - >/dev/null
  kubectl -n bex-system rollout status deploy/bex-ssh-gateway --timeout=180s
  refresh_agent_gateway_forwards
  echo "model stub removed; the gateway is back on the system trust store."
}

# cmd_agent_down — remove the leg, leave the base dev-N running. Only touches
# objects this dev-N owns (the app.bex.co/dev-env stamp), so it cannot delete
# another workstream's leg.
cmd_agent_down() {
  refresh_kubeconfig
  local owner
  owner=$(kubectl get ns opensandbox-system -o jsonpath='{.metadata.labels.app\.bex\.co/dev-env}' 2>/dev/null || true)
  if [ -n "$owner" ] && [ "$owner" != "$DEV_NS" ]; then
    echo "error: the agent-session leg belongs to $owner, not $DEV_NS — refusing" >&2
    exit 1
  fi
  local name
  for name in opensandbox agent-attach sandbox-exec openfga openbao; do
    kill_if_running "$ENVDIR/.pids/pf-$name.pid"
  done
  helm uninstall opensandbox-controller -n opensandbox-system >/dev/null 2>&1 || true
  kubectl delete ns opensandbox-system --ignore-not-found >/dev/null
  kubectl -n "$DEV_NS" delete deploy/model-stub svc/model-stub \
    configmap/model-stub-src secret/model-stub-tls --ignore-not-found >/dev/null
  kubectl -n bex-system delete deploy bex-ssh-gateway --ignore-not-found >/dev/null
  kubectl -n bex-system delete configmap model-stub-ca --ignore-not-found >/dev/null
  kubectl -n kube-system delete deploy bex-dev-host-api-proxy --ignore-not-found >/dev/null
  kubectl -n kube-system delete service bex-dev-host-api --ignore-not-found >/dev/null
  kubectl -n kube-system delete configmap bex-dev-host-api-proxy --ignore-not-found >/dev/null
  kubectl -n bex-system delete secret bex-dev-gateway bex-dev-gateway-hostkey --ignore-not-found >/dev/null
  kubectl -n "$DEV_AUTH_NS" delete deploy openfga openbao --ignore-not-found >/dev/null
  kubectl -n "$DEV_AUTH_NS" delete svc openfga openbao --ignore-not-found >/dev/null
  kubectl delete clusterrolebinding "openbao-auth-delegator-$DEV_NS" --ignore-not-found >/dev/null
  rm -f "$AGENTDIR/enabled"
  echo "agent-session leg removed. bex-api still holds its agent environment until"
  echo "you re-run: bash scripts/dev-env.sh $N up"
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
agent-up) cmd_agent_up ;;
agent-down) cmd_agent_down ;;
agent-stub) cmd_agent_stub ;;
agent-stub-off) cmd_agent_stub_off ;;
agent-netpol)
  shift
  cmd_agent_netpol "${1:-}"
  ;;
*) usage ;;
esac
