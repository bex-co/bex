#!/usr/bin/env bash
# verify-tenant-isolation.sh — proves the w7/m1 tenant-isolation DoD matrix.
#
# Deploys probe pods in distinct workspaces (using direct App CRs with explicit
# workspace labels) and runs a reachability matrix:
#   DENY probes: cross-workspace pod, platform services (bex-api, OpenBao,
#                Prometheus, zot), cloud metadata (169.254.169.254), and the
#                node's own IP (kubelet :10250) — must be REFUSED or TIME OUT
#   ALLOW probes: same-workspace private service, public internet — must SUCCEED
#
# The egress-probe pod (probe-a) borrows a per-App policy (via the app label of
# a dedicated egress-probe App) so it faithfully models a real tenant pod: the
# platform node/metadata CiliumNetworkPolicy (w7/m4) puts every workspace pod
# into egress default-deny, and only a pod that also has the per-App *allow*
# policy keeps its legitimate egress (DNS, same-workspace, internet). A bare
# workspace-labeled pod would lose all egress and mis-report the ALLOW probes.
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
# Egress-probe App: exists only to donate its operator-generated per-App
# NetworkPolicy to probe-a (which carries this app label). Its own Service is
# never targeted, so probe-a joining it is harmless.
APP_EG="verify-iso-eg"

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

# http_reachable <src-pod> <target-url> — true if an HTTP fetch succeeds.
http_reachable() {
  kubectl exec -n "$NS" "$1" -- \
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
  kubectl delete app "$APP_A" "$APP_B" "$APP_EG" -n "$NS" --ignore-not-found &>/dev/null || true
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

# App EG — workspace A, egress-probe policy donor (see APP_EG comment above).
kubectl apply -n "$NS" -f - <<EOF
apiVersion: app.bex.co/v1alpha1
kind: App
metadata:
  name: $APP_EG
  namespace: $NS
  labels:
    app.bex.co/workspace: $WS_A
spec:
  image: traefik/whoami
  port: 80
  expose: false
EOF

log "waiting for App deployments to be Ready (up to 120s)..."
kubectl wait --for=condition=Available \
  deployment/"$APP_A" deployment/"$APP_B" deployment/"$APP_EG" \
  -n "$NS" --timeout=120s &>/dev/null || true

# Get pod IPs for cross-pod probes
POD_A_IP=$(kubectl get pods -n "$NS" -l "app.bex.co/app=$APP_A" -o jsonpath='{.items[0].status.podIP}' 2>/dev/null || echo "")
POD_B_IP=$(kubectl get pods -n "$NS" -l "app.bex.co/app=$APP_B" -o jsonpath='{.items[0].status.podIP}' 2>/dev/null || echo "")

log "launching probe pods..."

# Probe pod A: workspace A. Carries the egress-probe App label too, so it
# inherits that App's operator-generated per-App NetworkPolicy (the egress
# *allow* rules) and behaves like a real tenant pod under the node/metadata
# CiliumNetworkPolicy — see the header note.
kubectl run probe-a -n "$NS" \
  --image="$PROBE_IMAGE" --restart=Never \
  --labels="app.bex.co/app=$APP_EG,app.bex.co/workspace=$WS_A" \
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

# tenant pod → cloud instance-metadata (169.254.169.254) — the SSRF → metadata
# theft path (w7/m4). Excepted from the per-App egress ipBlock AND egressDeny-ed
# by the platform CiliumNetworkPolicy.
probe_deny_url "metadata-egress" probe-a "http://169.254.169.254/"

# tenant pod → a node's own IP on kubelet :10250 (w7/m4). Prefer a node's public
# IP (the real threat: public IPs aren't RFC1918, so the per-App ipBlock doesn't
# cover them — the CiliumNetworkPolicy host/remote-node egressDeny does). Fall
# back to the internal IP on clusters with no ExternalIP (e.g. the CAPD mock,
# where node IPs are RFC1918 and the per-App ipBlock covers them anyway).
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

# same-workspace: probe-a → App A service (private)
probe_allow_nc "same-workspace-private-service" probe-a "$APP_A.$NS.svc" 80

# public internet egress — the counter-assertion for the metadata + node DENY
# probes above: genuine external egress must still succeed from the same pod.
probe_allow "internet-egress" probe-a "https://example.com"

#──────────────────────────────────────── m2: pod hardening + quota checks ──────

echo ""
echo "=== m2 hardening checks (PSS · securityContext · SA token · quotas) ==="

BUILD_NS="${BUILD_NS:-bex-system}"

# PSS baseline must reject a privileged pod spec.
log "testing PSS baseline admission rejection..."
PSS_OUT=$(kubectl apply -n "$NS" -f - 2>&1 <<'HOSTILEEOF' || true
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
kubectl delete pod pss-hostile-test -n "$NS" --ignore-not-found &>/dev/null || true

# Inspect the reconciled App A pod for hardening fields.
POD_NAME=$(kubectl get pods -n "$NS" -l "app.bex.co/app=$APP_A" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$POD_NAME" ]; then
  # AutomountServiceAccountToken=false: the default SA token projected volume
  # must be absent from the pod spec.
  SA_TOKEN=$(kubectl get pod "$POD_NAME" -n "$NS" \
    -o jsonpath='{.spec.automountServiceAccountToken}' 2>/dev/null || echo "")
  if [ "$SA_TOKEN" = "false" ]; then
    ok "tenant-pod-no-sa-token"
  else
    fail "tenant-pod-no-sa-token (automountServiceAccountToken=$SA_TOKEN, want false)"
  fi

  # RuntimeDefault seccomp.
  SECCOMP=$(kubectl get pod "$POD_NAME" -n "$NS" \
    -o jsonpath='{.spec.containers[0].securityContext.seccompProfile.type}' 2>/dev/null || echo "")
  if [ "$SECCOMP" = "RuntimeDefault" ]; then
    ok "tenant-pod-runtimedefault-seccomp"
  else
    fail "tenant-pod-runtimedefault-seccomp (got: $SECCOMP)"
  fi

  # allowPrivilegeEscalation=false.
  APE=$(kubectl get pod "$POD_NAME" -n "$NS" \
    -o jsonpath='{.spec.containers[0].securityContext.allowPrivilegeEscalation}' 2>/dev/null || echo "")
  if [ "$APE" = "false" ]; then
    ok "tenant-pod-no-privilege-escalation"
  else
    fail "tenant-pod-no-privilege-escalation (got: $APE)"
  fi

  # capabilities.drop includes ALL.
  CAPS=$(kubectl get pod "$POD_NAME" -n "$NS" \
    -o jsonpath='{.spec.containers[0].securityContext.capabilities.drop}' 2>/dev/null || echo "")
  if echo "$CAPS" | grep -q "ALL"; then
    ok "tenant-pod-drop-all-caps"
  else
    fail "tenant-pod-drop-all-caps (got: $CAPS)"
  fi
else
  log "SKIP pod hardening checks (App A pod not found in $NS)"
fi

# Build Job resource limits (jobs run in BEX_BUILD_NAMESPACE = bex-system by default).
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

# ResourceQuota must be present on the tenant apps namespace.
QUOTA=$(kubectl get resourcequota -n "$NS" -o name 2>/dev/null || echo "")
if [ -n "$QUOTA" ]; then
  ok "namespace-has-resourcequota"
else
  fail "namespace-has-resourcequota (no ResourceQuota in $NS)"
fi

# LimitRange must be present.
LIMITRANGE=$(kubectl get limitrange -n "$NS" -o name 2>/dev/null || echo "")
if [ -n "$LIMITRANGE" ]; then
  ok "namespace-has-limitrange"
else
  fail "namespace-has-limitrange (no LimitRange in $NS)"
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

echo "All probes passed — tenant isolation policy compliant."
exit 0
