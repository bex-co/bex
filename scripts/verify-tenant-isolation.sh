#!/usr/bin/env bash
# verify-tenant-isolation.sh — proves the w7/m1 tenant-isolation DoD matrix.
#
# Deploys two probe pods in distinct workspaces (using direct App CRs with
# explicit workspace labels) and runs a reachability matrix:
#   DENY probes: cross-workspace pod, platform services (bex-api, OpenBao,
#                Prometheus, zot) — connection must be REFUSED or TIME OUT
#   ALLOW probes: same-workspace private service, public internet — must SUCCEED
#
# Usage:
#   bash scripts/verify-tenant-isolation.sh          # runs against current kubeconfig
#   TIMEOUT=3 bash scripts/verify-tenant-isolation.sh # override per-probe timeout (s)
#
# Exit 0 on full compliance; non-zero with the failing probe name on stdout.
# Requires: kubectl, curl-capable pod image (nicolaka/netshoot or curlimages/curl).

set -euo pipefail

NS="${APPS_NS:-default}"
TIMEOUT="${TIMEOUT:-5}"
PROBE_IMAGE="${PROBE_IMAGE:-nicolaka/netshoot}"

WS_A="tea-verify-a00000000000"
WS_B="tea-verify-b00000000000"
APP_A="verify-iso-a"
APP_B="verify-iso-b"

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

# probe_deny <probe-name> <src-pod> <target-host> <target-port>
# Expects connection to be refused/timeout — a successful connect is a FAIL.
probe_deny() {
  local name="$1" pod="$2" host="$3" port="$4"
  if kubectl exec -n "$NS" "$pod" -- \
      timeout "$TIMEOUT" nc -zw"$TIMEOUT" "$host" "$port" &>/dev/null 2>&1; then
    fail "$name (CONNECTED — expected BLOCKED)"
  else
    ok "$name"
  fi
}

# probe_allow <probe-name> <src-pod> <target-url>
# Expects HTTP 2xx/3xx — a connection failure is a FAIL.
probe_allow() {
  local name="$1" pod="$2" url="$3"
  if kubectl exec -n "$NS" "$pod" -- \
      curl -sf --max-time "$TIMEOUT" -o /dev/null "$url" &>/dev/null 2>&1; then
    ok "$name"
  else
    fail "$name (BLOCKED — expected CONNECTED)"
  fi
}

# probe_allow_nc <probe-name> <src-pod> <host> <port>
# nc-based allow: port open = pass.
probe_allow_nc() {
  local name="$1" pod="$2" host="$3" port="$4"
  if kubectl exec -n "$NS" "$pod" -- \
      timeout "$TIMEOUT" nc -zw"$TIMEOUT" "$host" "$port" &>/dev/null 2>&1; then
    ok "$name"
  else
    fail "$name (BLOCKED — expected CONNECTED)"
  fi
}

#───────────────────────────────────────────────────────── setup ─────────────────

cleanup() {
  log "cleaning up test resources..."
  kubectl delete app "$APP_A" "$APP_B" -n "$NS" --ignore-not-found &>/dev/null || true
  kubectl delete pod "probe-a" "probe-b" -n "$NS" --ignore-not-found &>/dev/null || true
}
trap cleanup EXIT

log "creating test Apps with distinct workspace labels..."

# App A — workspace A
kubectl apply -n "$NS" -f - <<EOF
apiVersion: app.bex.co/v1alpha1
kind: App
metadata:
  name: $APP_A
  namespace: $NS
  labels:
    app.bex.co/workspace: $WS_A
spec:
  image: traefik/whoami
  port: 80
  expose: false
EOF

# App B — workspace B
kubectl apply -n "$NS" -f - <<EOF
apiVersion: app.bex.co/v1alpha1
kind: App
metadata:
  name: $APP_B
  namespace: $NS
  labels:
    app.bex.co/workspace: $WS_B
spec:
  image: traefik/whoami
  port: 80
  expose: false
EOF

log "waiting for App deployments to be Ready (up to 120s)..."
kubectl wait --for=condition=Available deployment/"$APP_A" -n "$NS" --timeout=120s &>/dev/null || true
kubectl wait --for=condition=Available deployment/"$APP_B" -n "$NS" --timeout=120s &>/dev/null || true

# Get pod IPs for cross-pod probes
POD_A_IP=$(kubectl get pods -n "$NS" -l "app.bex.co/app=$APP_A" -o jsonpath='{.items[0].status.podIP}' 2>/dev/null || echo "")
POD_B_IP=$(kubectl get pods -n "$NS" -l "app.bex.co/app=$APP_B" -o jsonpath='{.items[0].status.podIP}' 2>/dev/null || echo "")

log "launching probe pods..."

# Probe pod A: in workspace A (carries the workspace label)
kubectl run probe-a -n "$NS" \
  --image="$PROBE_IMAGE" --restart=Never \
  --labels="app.bex.co/workspace=$WS_A" \
  --command -- sleep 300 &>/dev/null || true

# Probe pod B: in workspace B
kubectl run probe-b -n "$NS" \
  --image="$PROBE_IMAGE" --restart=Never \
  --labels="app.bex.co/workspace=$WS_B" \
  --command -- sleep 300 &>/dev/null || true

log "waiting for probe pods to be Ready (up to 60s)..."
kubectl wait --for=condition=Ready pod/probe-a pod/probe-b -n "$NS" --timeout=60s &>/dev/null

#───────────────────────────────────────────────────────── deny probes ───────────

echo ""
echo "=== DENY probes (expected: BLOCKED) ==="

# Cross-workspace pod → pod
if [ -n "$POD_B_IP" ]; then
  probe_deny "cross-workspace-pod-to-pod" probe-a "$POD_B_IP" 80
else
  log "SKIP cross-workspace-pod-to-pod (App B pod IP not found)"
fi

# Cross-workspace pod → App B service
probe_deny "cross-workspace-service" probe-a "$APP_B.$NS.svc" 80

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

#───────────────────────────────────────────────────────── allow probes ──────────

echo ""
echo "=== ALLOW probes (expected: CONNECTED) ==="

# same-workspace: probe-a → App A service (private)
probe_allow_nc "same-workspace-private-service" probe-a "$APP_A.$NS.svc" 80

# public internet egress
probe_allow "internet-egress" probe-a "https://example.com"

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

echo "All probes passed — tenant isolation policy compliant."
exit 0
