#!/usr/bin/env bash
# verify-tenant-isolation.sh — the ADR043 tenant-isolation reachability matrix,
# run against real per-workspace namespaces.
#
# Under ADR043 the tenant boundary IS the Kubernetes namespace: a workspace id
# (tea-<xid>) is used verbatim as its hosting namespace name, and the
# NamespaceReconciler gives that namespace a default-deny NetworkPolicy plus the
# same-namespace / DNS / Traefik-ingress / internet-egress allows layered on top.
# So this harness provisions two disposable workspaces through the control-plane
# tenant API, waits for their namespaces, and plants the test Apps and probe pods
# INSIDE them — every cross-workspace cell below crosses a genuinely different
# namespace, which is the boundary production actually enforces.
#
# The matrix:
#   DENY probes: cross-workspace pod and Service, platform services (bex-api,
#                OpenBao, Prometheus, zot), cloud metadata (169.254.169.254), and
#                the node's own IP (kubelet :10250) — must be REFUSED or TIME OUT
#   ALLOW probes: same-workspace private service, public internet — must SUCCEED
# followed by the w7/m2 hardening section (PSS baseline admission, pod
# securityContext, SA-token automount, ResourceQuota/LimitRange), read off
# workspace A's namespace rather than a shared one.
#
# This shape was first proven by hand on the migrated production cluster on
# 2026-07-29 (w3/m31 t007) with these results — a mis-edited harness can pass
# when it should fail, so they were RUN, not guessed:
#
#   DENY  cross-namespace  (WS_A pod -> service in WS_B)          -> BLOCKED  ✓
#   ALLOW same-namespace   (WS_A pod -> service in WS_A)          -> HTTP 200 ✓
#   DENY  platform         (WS_A pod -> bex-api.bex-system:8090)  -> BLOCKED  ✓
#   DENY  cloud-metadata   (169.254.169.254)                      -> BLOCKED  ✓
#   ALLOW public internet  (example.com)                          -> REACHED  ✓
#
# The cross-namespace DENY is enforced by the NamespaceReconciler's per-namespace
# default-deny (there is no cross-namespace allow) and by the internet-egress
# policy's RFC1918/CGNAT except-list; the metadata DENY by that same except-list
# plus the cluster-wide CiliumClusterwideNetworkPolicy promoted in w3/m83 t005.
# The body below now runs all of it unattended, which is what the weekly
# isolation-matrix.yml workflow calls.
#
# There is no shared-namespace mode any more. The pre-ADR043 harness put both
# workspaces in BEX_APPS_NAMESPACE (default "default") and told them apart by
# label; AppReconciler.canonicalNamespace now refuses a workspace-labeled App in
# the bootstrap namespace outright (codex-security 2026-08 F11), so that shape
# creates no workloads at all and its ALLOW probes report a policy failure that
# is really a harness failure (issue #66). The old APPS_NS override is gone with
# it.
#
# Usage:
#   KUBECONFIG=/path/to/app-cluster-admin.kubeconfig bash scripts/verify-tenant-isolation.sh
#   TIMEOUT=3 … # override the per-probe timeout (seconds)
#
# Exit 0 on full compliance; 1 with the failing probe names on stdout; 2 when the
# harness was not given what it needs (missing tool, unreachable control plane) —
# an operator to-do, not an isolation breach.
#
# Requires: kubectl (cluster-admin), curl, jq, and a curl-capable probe image
# (nicolaka/netshoot). Creates two disposable hobby workspaces and deletes them
# again on every exit path.

set -euo pipefail

TIMEOUT="${TIMEOUT:-5}"
PROBE_IMAGE="${PROBE_IMAGE:-nicolaka/netshoot}"

# Workspace ids are minted by the control plane, so they are only known at run
# time; the probe/App names stay fixed so issue text stays comparable run to run.
WS_A=""
WS_B=""
APP_A="verify-iso-a"
APP_B="verify-iso-b"
# Egress-probe App: probe-a carries this App's `app.bex.co/app` label so it is
# label-indistinguishable from a real App pod. Under ADR043 the egress allows
# come from the NAMESPACE (allow-dns-egress + allow-internet-egress), not from a
# per-App policy — those were removed with ADR022 — but keeping the label is what
# makes an admission/selector regression that keys on App labels visible here.
# Its own Service is never targeted, so probe-a joining it is harmless.
APP_EG="verify-iso-eg"

BEX_API_NAMESPACE="${BEX_API_NAMESPACE:-bex-system}"
CP_SECRET="${BEX_CONTROL_PLANE_SECRET:-bex-control-plane}"

BEX_SYSTEM_NS="${BEX_SYSTEM_NS:-bex-system}"
OPENBAO_NS="${OPENBAO_NS:-secrets}"
MONITORING_NS="${MONITORING_NS:-monitoring}"
REGISTRY_NS="${REGISTRY_NS:-bex-registry}"

PASS=0
FAIL=0
FAILED_PROBES=()

#───────────────────────────────────────────────────────────── helpers ──────────

log()  { echo "  [verify] $*"; }
ok()   { log "PASS  $1"; PASS=$((PASS+1)); }
fail() { log "FAIL  $1"; FAIL=$((FAIL+1)); FAILED_PROBES+=("$1"); }

# unusable reports a harness prerequisite failure (exit 2) — deliberately NOT a
# probe failure, so a missing tool never reads as a tenant escaping its namespace.
# It writes to stderr because some callers run inside a command substitution,
# where stdout is being captured as a value.
unusable() { echo "  [verify] UNUSABLE  $*" >&2; exit 2; }

# SRC_NS is the namespace every probe pod exec runs in — workspace A's namespace,
# set once the workspace exists. All probes originate from probe-a.
SRC_NS=""

# probe_deny <probe-name> <src-pod> <target-host> <target-port>
# Expects connection to be refused/timeout — a successful connect is a FAIL.
probe_deny() {
  local name="$1" pod="$2" host="$3" port="$4"
  if kubectl exec -n "$SRC_NS" "$pod" -- \
      timeout "$TIMEOUT" nc -zw"$TIMEOUT" "$host" "$port" &>/dev/null 2>&1; then
    fail "$name (CONNECTED — expected BLOCKED)"
  else
    ok "$name"
  fi
}

# http_reachable <src-pod> <target-url> — true if an HTTP fetch succeeds.
http_reachable() {
  kubectl exec -n "$SRC_NS" "$1" -- \
    curl -sf --max-time "$TIMEOUT" -o /dev/null "$2" &>/dev/null 2>&1
}

# probe_allow <probe-name> <src-pod> <target-url>
# Expects HTTP 2xx/3xx — a connection failure is a FAIL.
probe_allow() {
  local name="$1" pod="$2" url="$3"
  if http_reachable "$pod" "$url"; then
    ok "$name"
  else
    fail "$name (BLOCKED — expected CONNECTED)"
  fi
}

# probe_deny_url <probe-name> <src-pod> <target-url>
# curl-based deny: a successful HTTP fetch is a FAIL (the target must be
# unreachable). Used for the cloud-metadata endpoint, where reachability means
# a live SSRF hole, not just an open port.
probe_deny_url() {
  local name="$1" pod="$2" url="$3"
  if http_reachable "$pod" "$url"; then
    fail "$name (CONNECTED — expected BLOCKED)"
  else
    ok "$name"
  fi
}

# probe_allow_nc <probe-name> <src-pod> <host> <port>
# nc-based allow: port open = pass.
probe_allow_nc() {
  local name="$1" pod="$2" host="$3" port="$4"
  if kubectl exec -n "$SRC_NS" "$pod" -- \
      timeout "$TIMEOUT" nc -zw"$TIMEOUT" "$host" "$port" &>/dev/null 2>&1; then
    ok "$name"
  else
    fail "$name (BLOCKED — expected CONNECTED)"
  fi
}

#──────────────────────────────────────── disposable workspace fixtures ─────────

: "${KUBECONFIG:?set KUBECONFIG to the target app-cluster kubeconfig}"
for tool in kubectl curl jq base64; do
  command -v "$tool" >/dev/null || unusable "missing required command: $tool"
done

work_dir="$(mktemp -d)"
chmod 700 "$work_dir"
umask 077
forward_pid=""

# delete_disposable_tenants removes the two disposable tenant rows, which is what
# makes the NamespaceReconciler prune their namespaces — pruneOrphans keys off the
# `tenants` table, so deleting the namespace alone would just be re-created.
#
# The internal control-plane API has no DELETE /v1/tenants, and GraphQL
# deleteWorkspace needs an authenticated workspace admin (a whole disposable
# Hydra client + OpenFGA grant, as scripts/verify-sandbox-isolation-live.sh
# mints), so this goes to the bex-db primary directly. Every dependent row
# cascades from the tenants FK.
#
# Fail closed on the id shape. The ids come from this script's own create
# responses, but a partial/garbled capture must never widen the statement: only
# ids matching the minted workspace format (ADR020: tea-<20-char xid>) are
# spliced in, and the DELETE is pinned to exactly those.
delete_disposable_tenants() {
  local ws list=""
  for ws in "$WS_A" "$WS_B"; do
    [ -n "$ws" ] || continue
    if [[ ! "$ws" =~ ^tea-[0-9a-v]{20}$ ]]; then
      log "REFUSING to delete a workspace row whose id is not a well-formed tea-* id"
      continue
    fi
    list="${list:+$list,}'$ws'"
  done
  [ -n "$list" ] || return 0
  local primary
  primary="$(kubectl -n "$BEX_API_NAMESPACE" get clusters.postgresql.cnpg.io bex-db \
    -o jsonpath='{.status.currentPrimary}' 2>/dev/null || true)"
  if [ -z "$primary" ]; then
    log "WARNING: bex-db has no primary — delete disposable workspaces $list by hand"
    return 0
  fi
  if printf 'DELETE FROM tenants WHERE id IN (%s);\n' "$list" \
    | kubectl -n "$BEX_API_NAMESPACE" exec -i "$primary" -c postgres -- \
        psql -X -q -v ON_ERROR_STOP=1 -U postgres -d bex >/dev/null 2>&1; then
    log "deleted disposable workspace rows $list (namespaces prune on the next reconcile)"
  else
    log "WARNING: deleting disposable workspaces $list failed — remove them by hand"
  fi
}

# purge runs one cleanup deletion, reporting failure instead of swallowing it.
# A silent cleanup is how three verify-iso-* Apps sat in production tenant
# namespaces from 2026-09-14 to 2026-09-16: they carry no bex.co/tenant label, so
# analytics correctly skips them and nothing else was ever going to notice.
purge() {
  local what="$1"
  shift
  local out
  if ! out=$(kubectl delete "$@" 2>&1); then
    log "WARNING: cleanup of $what failed — remove it by hand: $out"
  fi
}

cleanup() {
  local rc=$?
  set +e
  log "cleaning up test resources..."
  if [ -n "$WS_A" ]; then
    purge "probe pods in $WS_A" pod probe-a pss-hostile-test -n "$WS_A" --ignore-not-found
    purge "apps $APP_A/$APP_EG in $WS_A" app "$APP_A" "$APP_EG" -n "$WS_A" --ignore-not-found --timeout=90s
  fi
  if [ -n "$WS_B" ]; then
    purge "probe pod in $WS_B" pod probe-b -n "$WS_B" --ignore-not-found
    purge "app $APP_B in $WS_B" app "$APP_B" -n "$WS_B" --ignore-not-found --timeout=90s
  fi
  delete_disposable_tenants
  if [ -n "$forward_pid" ]; then
    kill "$forward_pid" 2>/dev/null
    wait "$forward_pid" 2>/dev/null
  fi
  # work_dir is an exact mktemp-created directory holding the control-plane
  # bearer in a curl config file.
  find "$work_dir" -type f -delete 2>/dev/null
  rmdir "$work_dir" 2>/dev/null
  return "$rc"
}
# EXIT alone is not enough: an interrupted or killed run (Ctrl-C, a harness
# timeout, a terminated CI step) never reaches it, which is exactly how the
# 2026-09-14 fixtures leaked — their Apps had no deletionTimestamp at all, so
# cleanup had never been attempted. Trapping the signals runs cleanup and then
# exits, and EXIT stays for the ordinary paths.
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

log "opening a private forward to the control-plane internal API..."
kubectl -n "$BEX_API_NAMESPACE" get deploy bex-api >/dev/null \
  || unusable "bex-api is not deployed in $BEX_API_NAMESPACE"
kubectl -n "$BEX_API_NAMESPACE" get secret "$CP_SECRET" >/dev/null \
  || unusable "control-plane token Secret $CP_SECRET is absent from $BEX_API_NAMESPACE"

# An unpinned local port (":8091") lets the kernel pick, so two operators — or a
# run overlapping some other forward — can never collide on a fixed number.
kubectl -n "$BEX_API_NAMESPACE" port-forward deploy/bex-api ":8091" \
  >"$work_dir/cp-forward.log" 2>&1 &
forward_pid="$!"

cp_port=""
for _ in $(seq 1 30); do
  cp_port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9][0-9]*\).*/\1/p' \
    "$work_dir/cp-forward.log" | head -1)"
  [ -n "$cp_port" ] && break
  sleep 1
done
[ -n "$cp_port" ] || unusable "the control-plane port-forward never reported a local port"
cp_base="http://127.0.0.1:$cp_port"

# The bearer lives only in a 0600 curl config file, never in argv or the log.
cp_token="$(kubectl -n "$BEX_API_NAMESPACE" get secret "$CP_SECRET" \
  -o 'jsonpath={.data.token}' | base64 -d)"
[ -n "$cp_token" ] || unusable "the control-plane token Secret has an empty token key"
printf 'header = "Authorization: Bearer %s"\n' "$cp_token" >"$work_dir/cp.curl"
chmod 600 "$work_dir/cp.curl"
unset cp_token

cp_ready=false
for _ in $(seq 1 30); do
  if [ "$(curl -sS -o /dev/null -w '%{http_code}' "$cp_base/" 2>/dev/null)" != 000 ]; then
    cp_ready=true
    break
  fi
  sleep 1
done
[ "$cp_ready" = true ] || unusable "the control-plane internal API forward did not become ready"

cp_call() {
  local method="$1" path="$2" output="$3"
  shift 3
  curl -sS --config "$work_dir/cp.curl" --output "$output" --write-out '%{http_code}' \
    -X "$method" -H 'Content-Type: application/json' "$@" "$cp_base$path"
}

# create_workspace mints one disposable hobby workspace and echoes its id. Plan
# hobby is deliberate: the harness runs three small Apps and two probe pods,
# which fit the hobby ResourceQuota, and the quota/LimitRange assertions below
# then check the tier a real hobby tenant gets.
create_workspace() {
  local name="$1" output="$2" code
  code="$(jq -nc --arg name "$name" '{name:$name,plan:"hobby"}' \
    | cp_call POST /v1/tenants "$output" --data-binary @-)"
  [ "$code" = 201 ] || unusable "control-plane createTenant $name returned HTTP $code"
  jq -er '.id' "$output"
}

log "creating two disposable workspaces through the control-plane tenant API..."
run_suffix="$(date -u +%m%d%H%M%S)"
WS_A="$(create_workspace "iso-verify-a-$run_suffix" "$work_dir/ws-a.json")"
WS_B="$(create_workspace "iso-verify-b-$run_suffix" "$work_dir/ws-b.json")"
[ "$WS_A" != "$WS_B" ] || unusable "workspace creation returned duplicate ids"
SRC_NS="$WS_A"
log "workspace A=$WS_A  workspace B=$WS_B"

# Billing-exclude both immediately: a disposable fixture must never reach Stripe
# or be payment-gated (ADR040 §7 Mode A, the same treatment the sandbox leg's
# fixtures get).
for ws in "$WS_A" "$WS_B"; do
  code="$(jq -nc '{excluded:true,actor:"tenant-isolation-verify"}' \
    | cp_call PATCH "/v1/tenants/$ws/billing-excluded" "$work_dir/exclude-$ws.json" \
        --data-binary @-)"
  [ "$code" = 200 ] || unusable "billing-excluding disposable workspace $ws returned HTTP $code"
done

log "waiting for the NamespaceReconciler to build both namespaces (up to 120s)..."
ns_ready=false
for _ in $(seq 1 60); do
  if kubectl get namespace "$WS_A" "$WS_B" >/dev/null 2>&1 \
    && kubectl -n "$WS_A" get networkpolicy default-deny >/dev/null 2>&1 \
    && kubectl -n "$WS_B" get networkpolicy default-deny >/dev/null 2>&1; then
    ns_ready=true
    break
  fi
  sleep 2
done
[ "$ns_ready" = true ] || unusable "the disposable workspace namespaces did not reconcile"

#───────────────────────────────────────────────────────── setup ─────────────────

log "creating test Apps in their own workspace namespaces..."

# Every App's namespace equals its app.bex.co/workspace label — the only shape
# AppReconciler.canonicalNamespace accepts outside the bootstrap namespace.

# App A — workspace A
kubectl apply -n "$WS_A" -f - <<EOF
apiVersion: app.bex.co/v1alpha1
kind: App
metadata:
  name: $APP_A
  namespace: $WS_A
  labels:
    app.bex.co/workspace: $WS_A
spec:
  image: traefik/whoami
  port: 80
  expose: false
EOF

# App B — workspace B
kubectl apply -n "$WS_B" -f - <<EOF
apiVersion: app.bex.co/v1alpha1
kind: App
metadata:
  name: $APP_B
  namespace: $WS_B
  labels:
    app.bex.co/workspace: $WS_B
spec:
  image: traefik/whoami
  port: 80
  expose: false
EOF

# App EG — workspace A, the label probe-a borrows (see the APP_EG comment above).
kubectl apply -n "$WS_A" -f - <<EOF
apiVersion: app.bex.co/v1alpha1
kind: App
metadata:
  name: $APP_EG
  namespace: $WS_A
  labels:
    app.bex.co/workspace: $WS_A
spec:
  image: traefik/whoami
  port: 80
  expose: false
EOF

log "waiting for App deployments to be Ready (up to 120s)..."
kubectl wait --for=condition=Available \
  deployment/"$APP_A" deployment/"$APP_EG" \
  -n "$WS_A" --timeout=120s &>/dev/null || true
kubectl wait --for=condition=Available \
  deployment/"$APP_B" \
  -n "$WS_B" --timeout=120s &>/dev/null || true

# App B's pod IP is the target of the raw cross-namespace pod-to-pod probe.
POD_B_IP=$(kubectl get pods -n "$WS_B" -l "app.bex.co/app=$APP_B" -o jsonpath='{.items[0].status.podIP}' 2>/dev/null || echo "")

log "launching probe pods..."

# Probe pod A: workspace A's namespace. Carries the egress-probe App's label so
# it is label-indistinguishable from a real tenant App pod — see the header note.
kubectl run probe-a -n "$WS_A" \
  --image="$PROBE_IMAGE" --restart=Never \
  --labels="app.bex.co/app=$APP_EG,app.bex.co/workspace=$WS_A" \
  --command -- sleep 300 &>/dev/null || true

# Probe pod B: workspace B's namespace — the other side of the boundary.
kubectl run probe-b -n "$WS_B" \
  --image="$PROBE_IMAGE" --restart=Never \
  --labels="app.bex.co/workspace=$WS_B" \
  --command -- sleep 300 &>/dev/null || true

# A probe pod that never starts makes every DENY cell pass for the wrong reason,
# so this is fail-closed as a harness fault (exit 2) rather than a probe result.
log "waiting for probe pods to be Ready (up to 60s)..."
kubectl wait --for=condition=Ready pod/probe-a -n "$WS_A" --timeout=60s &>/dev/null \
  || unusable "probe-a never became Ready in $WS_A"
kubectl wait --for=condition=Ready pod/probe-b -n "$WS_B" --timeout=60s &>/dev/null \
  || unusable "probe-b never became Ready in $WS_B"

#───────────────────────────────────────────────────────── deny probes ───────────

echo ""
echo "=== DENY probes (expected: BLOCKED) ==="

# Cross-workspace pod → pod (workspace A's namespace → workspace B's)
if [ -n "$POD_B_IP" ]; then
  probe_deny "cross-workspace-pod-to-pod" probe-a "$POD_B_IP" 80
else
  log "SKIP cross-workspace-pod-to-pod (App B pod IP not found)"
fi

# Cross-workspace pod → App B service in workspace B's namespace
probe_deny "cross-workspace-service" probe-a "$APP_B.$WS_B.svc" 80

# tenant pod → bex-api internal API (:8091)
BEX_API_SVC="bex-api.$BEX_SYSTEM_NS.svc"
probe_deny "tenant-to-bex-api-internal" probe-a "$BEX_API_SVC" 8091

# tenant pod → OpenBao (:8200)
OPENBAO_SVC="openbao.$OPENBAO_NS.svc"
probe_deny "tenant-to-openbao" probe-a "$OPENBAO_SVC" 8200

# tenant pod → Prometheus (:9090)
PROM_SVC="prometheus-kube-prometheus-prometheus.$MONITORING_NS.svc"
probe_deny "tenant-to-prometheus" probe-a "$PROM_SVC" 9090

# tenant pod → zot registry (:5000)
ZOT_SVC="zot.$REGISTRY_NS.svc"
probe_deny "tenant-to-zot" probe-a "$ZOT_SVC" 5000

# tenant pod → cloud instance-metadata (169.254.169.254) — the SSRF → metadata
# theft path (w7/m4). Excepted from the namespace internet-egress ipBlock AND
# egressDeny-ed by the cluster-wide CiliumClusterwideNetworkPolicy.
probe_deny_url "metadata-egress" probe-a "http://169.254.169.254/"

# tenant pod → a node's own IP on kubelet :10250 (w7/m4). Prefer a node's public
# IP (the real threat: public IPs aren't RFC1918, so the namespace egress
# except-list doesn't cover them — the Cilium host/remote-node egressDeny does).
# Fall back to the internal IP on clusters with no ExternalIP (e.g. the CAPD
# mock, where node IPs are RFC1918 and the except-list covers them anyway).
NODE_IP=$(kubectl get nodes \
  -o jsonpath='{range .items[*]}{.status.addresses[?(@.type=="ExternalIP")].address}{"\n"}{end}' 2>/dev/null \
  | grep -m1 . || true)
if [ -z "$NODE_IP" ]; then
  NODE_IP=$(kubectl get nodes \
    -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "")
fi
if [ -n "$NODE_IP" ]; then
  probe_deny "node-kubelet-10250" probe-a "$NODE_IP" 10250
else
  log "SKIP node-kubelet-10250 (no node IP discovered)"
fi

#───────────────────────────────────────────────────────── allow probes ──────────

echo ""
echo "=== ALLOW probes (expected: CONNECTED) ==="

# same-workspace: probe-a → App A service in the same namespace (private)
probe_allow_nc "same-workspace-private-service" probe-a "$APP_A.$WS_A.svc" 80

# public internet egress — the counter-assertion for the metadata + node DENY
# probes above: genuine external egress must still succeed from the same pod.
probe_allow "internet-egress" probe-a "https://example.com"

#──────────────────────────────────────── m2: pod hardening + quota checks ──────

echo ""
echo "=== m2 hardening checks (PSS · securityContext · SA token · quotas) ==="

BUILD_NS="${BUILD_NS:-bex-build}"

# PSS baseline must reject a privileged pod spec. The NamespaceReconciler stamps
# the enforce=baseline label on every workspace namespace it creates, so this
# also proves that labelling actually happened for a fresh workspace.
log "testing PSS baseline admission rejection..."
PSS_OUT=$(kubectl apply -n "$WS_A" -f - 2>&1 <<'HOSTILEEOF' || true
apiVersion: v1
kind: Pod
metadata:
  name: pss-hostile-test
spec:
  containers:
    - name: hostile
      image: busybox
      securityContext:
        privileged: true
HOSTILEEOF
)
if echo "$PSS_OUT" | grep -qiE "violates PodSecurity|forbidden|admission"; then
  ok "pss-baseline-rejects-privileged"
else
  fail "pss-baseline-rejects-privileged (expected rejection, got: $PSS_OUT)"
fi
kubectl delete pod pss-hostile-test -n "$WS_A" --ignore-not-found &>/dev/null || true

# Inspect the reconciled App A pod for hardening fields.
POD_NAME=$(kubectl get pods -n "$WS_A" -l "app.bex.co/app=$APP_A" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$POD_NAME" ]; then
  # AutomountServiceAccountToken=false: the default SA token projected volume
  # must be absent from the pod spec.
  SA_TOKEN=$(kubectl get pod "$POD_NAME" -n "$WS_A" \
    -o jsonpath='{.spec.automountServiceAccountToken}' 2>/dev/null || echo "")
  if [ "$SA_TOKEN" = "false" ]; then
    ok "tenant-pod-no-sa-token"
  else
    fail "tenant-pod-no-sa-token (automountServiceAccountToken=$SA_TOKEN, want false)"
  fi

  # RuntimeDefault seccomp.
  SECCOMP=$(kubectl get pod "$POD_NAME" -n "$WS_A" \
    -o jsonpath='{.spec.containers[0].securityContext.seccompProfile.type}' 2>/dev/null || echo "")
  if [ "$SECCOMP" = "RuntimeDefault" ]; then
    ok "tenant-pod-runtimedefault-seccomp"
  else
    fail "tenant-pod-runtimedefault-seccomp (got: $SECCOMP)"
  fi

  # allowPrivilegeEscalation=false.
  APE=$(kubectl get pod "$POD_NAME" -n "$WS_A" \
    -o jsonpath='{.spec.containers[0].securityContext.allowPrivilegeEscalation}' 2>/dev/null || echo "")
  if [ "$APE" = "false" ]; then
    ok "tenant-pod-no-privilege-escalation"
  else
    fail "tenant-pod-no-privilege-escalation (got: $APE)"
  fi

  # capabilities.drop includes ALL.
  CAPS=$(kubectl get pod "$POD_NAME" -n "$WS_A" \
    -o jsonpath='{.spec.containers[0].securityContext.capabilities.drop}' 2>/dev/null || echo "")
  if echo "$CAPS" | grep -q "ALL"; then
    ok "tenant-pod-drop-all-caps"
  else
    fail "tenant-pod-drop-all-caps (got: $CAPS)"
  fi
else
  log "SKIP pod hardening checks (App A pod not found in $WS_A)"
fi

# Build Job resource limits (jobs run in the dedicated BEX_BUILD_NAMESPACE).
BUILD_JOB=$(kubectl get jobs -n "$BUILD_NS" -l "app.bex.co/component=build" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$BUILD_JOB" ]; then
  CPU_LIMIT=$(kubectl get job "$BUILD_JOB" -n "$BUILD_NS" \
    -o jsonpath='{.spec.template.spec.containers[0].resources.limits.cpu}' 2>/dev/null || echo "")
  if [ -n "$CPU_LIMIT" ]; then
    ok "build-job-has-resource-limits"
  else
    fail "build-job-has-resource-limits (no cpu limit on job $BUILD_JOB in $BUILD_NS)"
  fi
else
  log "SKIP build-job-resource-limits (no build Job in $BUILD_NS — run a build-from-git deploy to test)"
fi

# ResourceQuota must be present on the workspace's own namespace (ADR043 D3 —
# this is what enforces the per-workspace service/datastore caps at admission).
QUOTA=$(kubectl get resourcequota -n "$WS_A" -o name 2>/dev/null || echo "")
if [ -n "$QUOTA" ]; then
  ok "namespace-has-resourcequota"
else
  fail "namespace-has-resourcequota (no ResourceQuota in $WS_A)"
fi

# LimitRange must be present.
LIMITRANGE=$(kubectl get limitrange -n "$WS_A" -o name 2>/dev/null || echo "")
if [ -n "$LIMITRANGE" ]; then
  ok "namespace-has-limitrange"
else
  fail "namespace-has-limitrange (no LimitRange in $WS_A)"
fi

#───────────────────────────────────────────────────────── summary ───────────────

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
  echo "FAILED probes:"
  for p in "${FAILED_PROBES[@]}"; do
    echo "  - $p"
  done
  exit 1
fi

echo "All probes passed — ADR043 tenant isolation policy compliant."
exit 0
