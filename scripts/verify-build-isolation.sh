#!/usr/bin/env bash
# Live, non-secret-printing verification for the w2/m59 untrusted-execution
# boundary. Inputs are Kubernetes object/image names only; credential values are
# mounted from an existing Secret and are never read or printed by this script.
#
# Required environment:
#   PROOF_JOB               completed adversarial BuildKit Job in bex-build
#   PROOF_APP               Running App produced by that Job
#   REGISTRY_AUTH_SECRET    build-namespace docker config (key config.json)
#   OWN_IMAGE               image that credential may read
#   OTHER_IMAGE             existing repository that credential must not read
#   UNSIGNED_IMAGE          unsigned image in the tenant registry
#   TAMPERED_IMAGE          signed tag whose manifest was replaced after signing
#
# Optional:
#   BUILD_NS=bex-build APPS_NS=default REGISTRY_HOST=zot.bex-registry.svc
#   REGISTRY_PORT=5000 PROBE_IMAGE=nicolaka/netshoot
#   METADATA_FIXTURE=0      set to 1 only on a disposable cluster; temporarily
#                           maps 169.254.169.254 to a live Service so the
#                           metadata denial is provably non-vacuous

set -euo pipefail

for required in PROOF_JOB PROOF_APP REGISTRY_AUTH_SECRET OWN_IMAGE OTHER_IMAGE UNSIGNED_IMAGE TAMPERED_IMAGE; do
  if [ -z "${!required:-}" ]; then
    echo "ERROR: $required must name an existing proof resource" >&2
    exit 2
  fi
done
command -v kubectl >/dev/null || { echo "ERROR: kubectl is required" >&2; exit 2; }
command -v yq >/dev/null || { echo "ERROR: yq v4 is required" >&2; exit 2; }

BUILD_NS="${BUILD_NS:-bex-build}"
APPS_NS="${APPS_NS:-default}"
REGISTRY_HOST="${REGISTRY_HOST:-zot.bex-registry.svc}"
REGISTRY_PORT="${REGISTRY_PORT:-5000}"
PLATFORM_HOST="${PLATFORM_HOST:-bex-api.bex-system.svc}"
PLATFORM_PORT="${PLATFORM_PORT:-8090}"
PROBE_IMAGE="${PROBE_IMAGE:-nicolaka/netshoot}"
METADATA_FIXTURE="${METADATA_FIXTURE:-0}"
SUFFIX="${RANDOM}${RANDOM}"
TARGET_NS="bex-m59-probe-${SUFFIX}"
SOURCE_POD="m59-source-${SUFFIX}"
REGISTRY_POD="m59-registry-${SUFFIX}"
METADATA_POD="m59-metadata-${SUFFIX}"
POLICY="m59-same-workspace-${SUFFIX}"
WS_A="tea-m59-allow-${SUFFIX}"
WS_B="tea-m59-deny-${SUFFIX}"
PASS=0
FAIL=0
FAILED=()

log() { echo "  [m59] $*"; }
pass() { log "PASS  $1"; PASS=$((PASS + 1)); }
fail() { log "FAIL  $1"; FAIL=$((FAIL + 1)); FAILED+=("$1"); }

assert_eq() {
  local name="$1" actual="$2" expected="$3"
  if [ "$actual" = "$expected" ]; then
    pass "$name"
  else
    fail "$name (got '$actual', want '$expected')"
  fi
}

allow_nc() {
  local name="$1" ns="$2" pod="$3" host="$4" port="$5"
  if kubectl exec -n "$ns" "$pod" -- timeout 5 nc -zw5 "$host" "$port" >/dev/null 2>&1; then
    pass "$name"
  else
    fail "$name (expected reachable)"
  fi
}

deny_nc() {
  local name="$1" host="$2" port="$3" control="$4"
  if kubectl exec -n "$BUILD_NS" "$SOURCE_POD" -- timeout 5 nc -zw5 "$host" "$port" >/dev/null 2>&1; then
    fail "$name (connected; positive control: $control)"
  else
    pass "$name [positive=$control]"
  fi
}

server_dry_run() {
  local name="$1" image="$2" verify_label="$3" expect="$4" output rc=0
  output="$(kubectl create --dry-run=server -f - >/dev/null 2>&1 <<EOF
apiVersion: v1
kind: Pod
metadata:
  generateName: m59-admission-
  namespace: $APPS_NS
  labels:
    app.bex.co/app: m59-admission
    app.bex.co/workspace: $WS_A
$verify_label
spec:
  automountServiceAccountToken: false
  hostUsers: false
  restartPolicy: Never
  securityContext:
    seccompProfile: {type: RuntimeDefault}
  containers:
    - name: probe
      image: $image
      command: ["sh", "-c", "true"]
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: ["ALL"]}
        runAsNonRoot: true
        runAsUser: 65532
EOF
  )" || rc=$?
  if [ "$expect" = allow ] && [ "$rc" -eq 0 ]; then
    pass "$name"
  elif [ "$expect" = deny ] && [ "$rc" -ne 0 ]; then
    pass "$name"
  else
    fail "$name (server dry-run outcome did not match $expect)"
  fi
  unset output
}

cleanup() {
  kubectl delete pod "$METADATA_POD" -n "$BUILD_NS" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete pod "$SOURCE_POD" "$REGISTRY_POD" -n "$BUILD_NS" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  kubectl delete networkpolicy "$POLICY" -n "$BUILD_NS" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  kubectl delete namespace "$TARGET_NS" --ignore-not-found --wait=false >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> m59 rendered/live workload shape"
kubectl get namespace "$BUILD_NS" >/dev/null
kubectl get job "$PROOF_JOB" -n "$BUILD_NS" >/dev/null
kubectl get app "$PROOF_APP" -n "$APPS_NS" >/dev/null
kubectl get secret "$REGISTRY_AUTH_SECRET" -n "$BUILD_NS" >/dev/null

JOB_JSON="$(kubectl get job "$PROOF_JOB" -n "$BUILD_NS" -o json)"
assert_eq "proof-job-completed" "$(yq -r '.status.conditions[]? | select(.type == "Complete") | .status' <<<"$JOB_JSON")" "True"
assert_eq "proof-job-no-api-token" "$(yq -r '.spec.template.spec.automountServiceAccountToken' <<<"$JOB_JSON")" "false"
assert_eq "proof-job-pod-user-namespace" "$(yq -r '.spec.template.spec.hostUsers' <<<"$JOB_JSON")" "false"
assert_eq "proof-job-tenant-selector" "$(yq -r '.spec.template.spec.nodeSelector."bex.co/pool"' <<<"$JOB_JSON")" "tenant"

if yq -e '.spec.template.spec.volumes[]? | select(.projected.sources[]?.serviceAccountToken)' <<<"$JOB_JSON" >/dev/null 2>&1; then
  fail "proof-job-has-no-projected-api-token"
else
  pass "proof-job-has-no-projected-api-token"
fi
PHASES="$(yq -r '[.spec.template.spec.initContainers[].name, .spec.template.spec.containers[].name] | join(",")' <<<"$JOB_JSON")"
assert_eq "credential-phases-are-serial" "$PHASES" "clone,buildkit,push,sign"
if yq -e '.spec.template.spec.initContainers[] | select(.name == "buildkit") | .env[]? | select(.name == "BUILDKITD_FLAGS" and (.value | contains("no-process-sandbox")))' <<<"$JOB_JSON" >/dev/null 2>&1; then
  fail "buildkit-default-process-sandbox"
else
  pass "buildkit-default-process-sandbox"
fi
if yq -e '.spec.template.spec.initContainers[] | select(.name == "buildkit") | .volumeMounts[]? | select(.name == "push-registry-cred" or .name == "cosign-key")' <<<"$JOB_JSON" >/dev/null 2>&1; then
  fail "tenant-run-cannot-mount-push-or-signing-credentials"
else
  pass "tenant-run-cannot-mount-push-or-signing-credentials"
fi
if yq -e '([.spec.template.spec.initContainers[] | select(.name != "clone") | .env[]? | select(.name == "GIT_AUTH_TOKEN")] | length) > 0' <<<"$JOB_JSON" >/dev/null 2>&1; then
  fail "clone-token-is-clone-phase-only"
else
  pass "clone-token-is-clone-phase-only"
fi

PROOF_POD="$(kubectl get pod -n "$BUILD_NS" -l job-name="$PROOF_JOB" -o jsonpath='{.items[0].metadata.name}')"
PROOF_NODE="$(kubectl get pod "$PROOF_POD" -n "$BUILD_NS" -o jsonpath='{.spec.nodeName}')"
assert_eq "proof-pod-landed-on-tenant-pool" "$(kubectl get node "$PROOF_NODE" -o jsonpath='{.metadata.labels.bex\.co/pool}')" "tenant"
if kubectl get node -l bex.co/pool=platform -o name | grep -q .; then
  pass "platform-pool-positive-control-exists"
else
  fail "platform-pool-positive-control-exists"
fi
assert_eq "adversarial-app-running" "$(kubectl get app "$PROOF_APP" -n "$APPS_NS" -o jsonpath='{.status.phase}')" "Running"
APP_POD="$(kubectl get pod -n "$APPS_NS" -l app.bex.co/app="$PROOF_APP" --field-selector=status.phase=Running --sort-by=.metadata.creationTimestamp -o name | tail -1)"
if [ -n "$APP_POD" ] && kubectl exec -n "$APPS_NS" "$APP_POD" -- test -f /m59-proof >/dev/null 2>&1; then
  pass "adversarial-dockerfile-executed-and-image-ran"
else
  fail "adversarial-dockerfile-executed-and-image-ran"
fi

echo "==> m59 non-vacuous network matrix"
kubectl create namespace "$TARGET_NS" >/dev/null
kubectl label namespace "$TARGET_NS" pod-security.kubernetes.io/enforce=restricted --overwrite >/dev/null

kubectl apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata: {name: same, namespace: $TARGET_NS, labels: {app.bex.co/workspace: $WS_A, probe: same}}
spec:
  automountServiceAccountToken: false
  hostUsers: false
  securityContext: {seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: server
      image: nginxinc/nginx-unprivileged:1.27-alpine
      ports: [{containerPort: 8080}]
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: ["ALL"]}, runAsNonRoot: true}
---
apiVersion: v1
kind: Pod
metadata: {name: cross, namespace: $TARGET_NS, labels: {app.bex.co/workspace: $WS_B, probe: cross}}
spec:
  automountServiceAccountToken: false
  hostUsers: false
  securityContext: {seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: server
      image: nginxinc/nginx-unprivileged:1.27-alpine
      ports: [{containerPort: 8080}]
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: ["ALL"]}, runAsNonRoot: true}
---
apiVersion: v1
kind: Pod
metadata: {name: platform, namespace: $TARGET_NS, labels: {plane: platform, probe: platform}}
spec:
  automountServiceAccountToken: false
  hostUsers: false
  securityContext: {seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: server
      image: nginxinc/nginx-unprivileged:1.27-alpine
      ports: [{containerPort: 8080}]
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: ["ALL"]}, runAsNonRoot: true}
---
apiVersion: v1
kind: Pod
metadata: {name: control, namespace: $TARGET_NS}
spec:
  automountServiceAccountToken: false
  hostUsers: false
  securityContext: {seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: control
      image: $PROBE_IMAGE
      command: ["sh", "-c", "sleep 900"]
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: ["ALL"]}, runAsNonRoot: true, runAsUser: 1000}
---
apiVersion: v1
kind: Service
metadata: {name: same, namespace: $TARGET_NS}
spec: {selector: {probe: same}, ports: [{port: 8080, targetPort: 8080}]}
---
apiVersion: v1
kind: Service
metadata: {name: cross, namespace: $TARGET_NS}
spec: {selector: {probe: cross}, ports: [{port: 8080, targetPort: 8080}]}
---
apiVersion: v1
kind: Service
metadata: {name: platform, namespace: $TARGET_NS}
spec: {selector: {probe: platform}, ports: [{port: 8080, targetPort: 8080}]}
EOF

kubectl apply -f - >/dev/null <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: $POLICY, namespace: $BUILD_NS}
spec:
  podSelector: {matchLabels: {app.bex.co/app: $SOURCE_POD}}
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: $TARGET_NS}}
          podSelector: {matchLabels: {app.bex.co/workspace: $WS_A}}
      ports: [{protocol: TCP, port: 8080}]
---
apiVersion: v1
kind: Pod
metadata:
  name: $SOURCE_POD
  namespace: $BUILD_NS
  labels: {app.bex.co/app: $SOURCE_POD, app.bex.co/component: build, app.bex.co/workspace: $WS_A}
spec:
  automountServiceAccountToken: false
  hostUsers: false
  nodeSelector: {bex.co/pool: tenant}
  securityContext: {seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: probe
      image: $PROBE_IMAGE
      command: ["sh", "-c", "sleep 900"]
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: ["ALL"]}}
EOF

kubectl wait --for=condition=Ready pod/same pod/cross pod/platform pod/control -n "$TARGET_NS" --timeout=180s >/dev/null
kubectl wait --for=condition=Ready pod/"$SOURCE_POD" -n "$BUILD_NS" --timeout=180s >/dev/null

if [ "$METADATA_FIXTURE" = 1 ]; then
  # Kubernetes rejects link-local Service externalIPs. On a disposable cluster,
  # create a bounded host-network listener on the same tenant node instead. The
  # container removes the temporary address on TERM, and cleanup waits for that
  # termination before returning.
  kubectl apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata: {name: $METADATA_POD, namespace: $BUILD_NS}
spec:
  nodeName: $PROOF_NODE
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  automountServiceAccountToken: false
  restartPolicy: Never
  containers:
    - name: metadata
      image: $PROBE_IMAGE
      command: ["sh", "-c"]
      args:
        - >-
          set -eu;
          ! ip address show | grep -q '169.254.169.254/32';
          ip address add 169.254.169.254/32 dev lo;
          socat TCP-LISTEN:80,bind=169.254.169.254,reuseaddr,fork EXEC:/bin/true & child=\$!;
          trap 'kill "\$child" 2>/dev/null || true; ip address del 169.254.169.254/32 dev lo' EXIT TERM INT;
          wait "\$child"
      readinessProbe:
        tcpSocket: {host: 169.254.169.254, port: 80}
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: ["ALL"], add: ["NET_ADMIN", "NET_BIND_SERVICE"]}
        seccompProfile: {type: RuntimeDefault}
EOF
  kubectl wait --for=condition=Ready pod/"$METADATA_POD" -n "$BUILD_NS" --timeout=90s >/dev/null
fi

allow_nc "control-same-workspace-target-is-live" "$TARGET_NS" control same."$TARGET_NS".svc 8080
allow_nc "control-cross-workspace-target-is-live" "$TARGET_NS" control cross."$TARGET_NS".svc 8080
allow_nc "control-platform-fixture-is-live" "$TARGET_NS" control platform."$TARGET_NS".svc 8080
if kubectl get endpointslice -n bex-system -l kubernetes.io/service-name=bex-api -o jsonpath='{.items[0].endpoints[0].conditions.ready}' | grep -qx true; then
  pass "control-platform-api-endpoint-is-ready"
else
  fail "control-platform-api-endpoint-is-ready"
fi
NODE_IP="$(kubectl get node "$PROOF_NODE" -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}')"
if [ "$METADATA_FIXTURE" = 1 ]; then
  # The host-network fixture sits outside the execution Cilium/NetworkPolicy
  # selection and therefore supplies non-vacuous controls for real API/node
  # listeners that other tenant-node policies may also deny to ordinary pods.
  allow_nc "control-api-target-is-live" "$BUILD_NS" "$METADATA_POD" kubernetes.default.svc 443
  allow_nc "control-kubelet-target-is-live" "$BUILD_NS" "$METADATA_POD" "$NODE_IP" 10250
else
  fail "control-api-target-is-live (requires disposable metadata fixture)"
  fail "control-kubelet-target-is-live (requires disposable metadata fixture)"
fi

if kubectl exec -n "$BUILD_NS" "$SOURCE_POD" -- nslookup kubernetes.default.svc >/dev/null 2>&1; then
  pass "allow-dns"
else
  fail "allow-dns"
fi
if kubectl exec -n "$BUILD_NS" "$SOURCE_POD" -- curl -fsS --max-time 10 -o /dev/null https://github.com; then
  pass "allow-public-git-https"
else
  fail "allow-public-git-https"
fi
allow_nc "allow-own-registry-tcp" "$BUILD_NS" "$SOURCE_POD" "$REGISTRY_HOST" "$REGISTRY_PORT"
allow_nc "allow-same-workspace-service" "$BUILD_NS" "$SOURCE_POD" same."$TARGET_NS".svc 8080

deny_nc "deny-kubernetes-api" kubernetes.default.svc 443 control-api-target-is-live
deny_nc "deny-kubelet" "$NODE_IP" 10250 control-kubelet-target-is-live
deny_nc "deny-cross-workspace" cross."$TARGET_NS".svc 8080 control-cross-workspace-target-is-live
deny_nc "deny-platform-service" platform."$TARGET_NS".svc 8080 control-platform-fixture-is-live
deny_nc "deny-real-platform-api" "$PLATFORM_HOST" "$PLATFORM_PORT" control-platform-api-endpoint-is-ready

if [ "$METADATA_FIXTURE" = 1 ]; then
  pass "control-metadata-target-is-live"
  deny_nc "deny-cloud-metadata" 169.254.169.254 80 control-metadata-target-is-live
else
  fail "deny-cloud-metadata (set METADATA_FIXTURE=1 on a disposable cluster for a non-vacuous target)"
fi

echo "==> m59 per-repository registry authorization"
kubectl apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $REGISTRY_POD
  namespace: $BUILD_NS
  labels: {app.bex.co/app: $SOURCE_POD, app.bex.co/component: build, app.bex.co/workspace: $WS_A}
spec:
  automountServiceAccountToken: false
  hostUsers: false
  nodeSelector: {bex.co/pool: tenant}
  securityContext: {seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: skopeo
      image: quay.io/skopeo/stable:v1.22.2@sha256:64ac45c5a1c01230896fbae960b2213e32a5040e4009b83b5f5cbf31a35f61c3
      command: ["sh", "-c", "sleep 900"]
      volumeMounts: [{name: auth, mountPath: /auth, readOnly: true}]
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: ["ALL"]}}
  volumes:
    - name: auth
      secret:
        secretName: $REGISTRY_AUTH_SECRET
        items: [{key: config.json, path: config.json}]
EOF
kubectl wait --for=condition=Ready pod/"$REGISTRY_POD" -n "$BUILD_NS" --timeout=180s >/dev/null
if kubectl exec -n "$BUILD_NS" "$REGISTRY_POD" -- skopeo inspect --tls-verify=false --authfile /auth/config.json docker://"$OWN_IMAGE" >/dev/null 2>&1; then
  pass "allow-own-registry-repository"
else
  fail "allow-own-registry-repository"
fi
if kubectl exec -n "$BUILD_NS" "$REGISTRY_POD" -- skopeo inspect --tls-verify=false --authfile /auth/config.json docker://"$OTHER_IMAGE" >/dev/null 2>&1; then
  fail "deny-other-registry-repository (credential crossed repository boundary)"
else
  pass "deny-other-registry-repository [positive=allow-own-registry-repository]"
fi

echo "==> m59 tenant-image admission"
VWC_JSON="$(kubectl get validatingwebhookconfiguration bex-validating-webhook-configuration -o json)"
assert_eq "webhook-failure-policy-fail" "$(yq -r '.webhooks[] | select(.name == "vpod.kb.io") | .failurePolicy' <<<"$VWC_JSON")" "Fail"
assert_eq "webhook-selects-verified-pods" "$(yq -r '.webhooks[] | select(.name == "vpod.kb.io") | .objectSelector.matchLabels."app.bex.co/verify-image"' <<<"$VWC_JSON")" "enabled"
server_dry_run "deny-unsigned-tenant-image" "$UNSIGNED_IMAGE" "    app.bex.co/verify-image: enabled" deny
server_dry_run "deny-tampered-tenant-image" "$TAMPERED_IMAGE" "    app.bex.co/verify-image: enabled" deny
server_dry_run "exclude-unselected-platform-pod" "busybox:1.36" "" allow
pass "allow-signed-tenant-image [positive=adversarial-app-running]"

echo "==> Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -ne 0 ]; then
  printf '  - %s\n' "${FAILED[@]}"
  exit 1
fi
echo "PASS: m59 untrusted-execution boundary verified"
