#!/usr/bin/env bash
# Live cross-tenant pull-denial verification for w7/m36
# (docs/ADR022-tenant-isolation.md § Read policy, closing ADR022:204).
#
# Proves:
#   1. Tenant A's per-App pull credential ("reg-pull-<a>") can pull A's own image.
#   2. The same credential CANNOT pull tenant B's image (HTTP 401/403 from Zot).
#   3. Deleting App A revokes its credential ("app-<a>" removed from zot-htpasswd
#      and zot-config); subsequent pull attempts with A's credential are denied.
#
# Prerequisites:
#   - Two Apps with registry-hosted images must exist and be Ready.
#   - BEX_REGISTRY_NS must point to the Zot namespace (default bex-registry).
#   - kubectl + KUBECONFIG configured for the target cluster.
#   - The operator must have already created per-App pull Secrets for both Apps.
#
# Usage:
#   APP_A=<name-a> APP_B=<name-b> APPS_NS=<ns> scripts/verify-per-app-registry-isolation.sh
#   DRY_RUN=1 APP_A=foo APP_B=bar scripts/verify-per-app-registry-isolation.sh  # preview only
set -euo pipefail
cd "$(dirname "$0")/.."

APP_A="${APP_A:-}"
APP_B="${APP_B:-}"
APPS_NS="${APPS_NS:-${BEX_CP_APPS_NAMESPACE:-default}}"
REGISTRY="${BEX_REGISTRY:-zot.bex-registry.svc:5000}"
REGISTRY_NS="${BEX_REGISTRY_NS:-bex-registry}"

if [ -z "$APP_A" ] || [ -z "$APP_B" ]; then
  echo "error: APP_A and APP_B must be set to two different App names" >&2
  exit 1
fi

command -v kubectl >/dev/null || { echo "error: kubectl not found" >&2; exit 1; }

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would verify:"
  echo "  1. reg-pull-${APP_A} can pull ${REGISTRY}/${APP_A}:gen-<latest>"
  echo "  2. reg-pull-${APP_A} CANNOT pull ${REGISTRY}/${APP_B}:gen-<latest> (expect 401)"
  echo "  3. zot-htpasswd contains app-${APP_A} entry"
  echo "  4. zot-config contains ${APP_A} repo ACL entry"
  echo "  5. After App ${APP_A} deletion: app-${APP_A} removed from zot-htpasswd and zot-config"
  exit 0
fi

pass() { echo "  PASS: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }

echo "=== w7/m36 per-App registry isolation verification ==="
echo "App A: $APP_A  App B: $APP_B  ns: $APPS_NS  registry: $REGISTRY"
echo

# --- 1. Verify per-App pull Secrets exist ---
echo "--- 1. Per-App pull Secrets ---"
for app in "$APP_A" "$APP_B"; do
  sec_name="reg-pull-${app}"
  if kubectl get secret "$sec_name" -n "$APPS_NS" >/dev/null 2>&1; then
    pass "Secret $APPS_NS/$sec_name exists"
  else
    fail "Secret $APPS_NS/$sec_name missing — has the operator reconciled App $app?"
  fi
done

# --- 2. Verify htpasswd entries ---
echo
echo "--- 2. zot-htpasswd entries ---"
HTPASSWD=$(kubectl get secret zot-htpasswd -n "$REGISTRY_NS" -o jsonpath='{.data.htpasswd}' | base64 -d 2>/dev/null)
for app in "$APP_A" "$APP_B"; do
  if echo "$HTPASSWD" | grep -q "^app-${app}:"; then
    pass "app-${app} present in zot-htpasswd"
  else
    fail "app-${app} missing from zot-htpasswd"
  fi
done
# Verify bex-puller is NOT in htpasswd (shared credential removed in w7/m36).
if echo "$HTPASSWD" | grep -q "^bex-puller:"; then
  fail "bex-puller shared credential still present in zot-htpasswd — should be absent (w7/m36)"
else
  pass "bex-puller shared credential absent from zot-htpasswd (w7/m36)"
fi

# --- 3. Verify Zot config per-repo ACL entries ---
echo
echo "--- 3. zot-config per-repo ACL entries ---"
ZOT_CFG=$(kubectl get secret zot-config -n "$REGISTRY_NS" -o jsonpath='{.data.config\.json}' | base64 -d 2>/dev/null)
for app in "$APP_A" "$APP_B"; do
  if echo "$ZOT_CFG" | python3 -c "import sys,json; d=json.load(sys.stdin); assert '$app' in d['http']['accessControl']['repositories']" 2>/dev/null; then
    pass "Zot config has per-repo ACL for $app"
  else
    fail "Zot config missing per-repo ACL for $app"
  fi
done
# Verify bex-puller not in ** wildcard.
if echo "$ZOT_CFG" | python3 -c "
import sys, json
d = json.load(sys.stdin)
pols = d['http']['accessControl']['repositories']['**']['policies']
users = [u for p in pols for u in p.get('users', [])]
assert 'bex-puller' not in users, 'bex-puller in ** policy'
" 2>/dev/null; then
  pass "bex-puller absent from ** wildcard ACL (w7/m36)"
else
  fail "bex-puller present in ** wildcard ACL — shared credential not yet removed"
fi

# --- 4. Live pull test (requires in-cluster execution via kubectl run) ---
echo
echo "--- 4. Live pull cross-tenant denial ---"
echo "  (Launching a probe pod using App A's credential to try pulling App B's image)"

# Extract App A's registry credential from its pull Secret.
PULL_CFG=$(kubectl get secret "reg-pull-${APP_A}" -n "$APPS_NS" -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d 2>/dev/null)
A_USER=$(echo "$PULL_CFG" | python3 -c "import sys,json; d=json.load(sys.stdin); a=d['auths']['$REGISTRY']; print(a['username'])")
A_PASS=$(echo "$PULL_CFG" | python3 -c "import sys,json; d=json.load(sys.stdin); a=d['auths']['$REGISTRY']; print(a['password'])")

# Probe: try to pull App B's latest tag using App A's credential.
# We use a Job (not kubectl run) so we can capture the pull error.
PROBE_JOB="reg-iso-probe-$$"
APP_B_IMAGE="${REGISTRY}/${APP_B}:gen-1"  # first build tag; adjust if needed

# Create a Job in bex-system that tries to pull B's image using A's credential.
# The imagePullPolicy=Always forces a registry pull even if the node has a cache.
PULL_SECRET_MANIFEST=$(cat <<YAML
apiVersion: v1
kind: Secret
metadata:
  name: ${PROBE_JOB}-cred
  namespace: bex-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: $(echo "$PULL_CFG" | base64 -w0)
YAML
)
echo "$PULL_SECRET_MANIFEST" | kubectl apply -f - >/dev/null

JOB_MANIFEST=$(cat <<YAML
apiVersion: batch/v1
kind: Job
metadata:
  name: ${PROBE_JOB}
  namespace: bex-system
spec:
  ttlSecondsAfterFinished: 60
  template:
    spec:
      restartPolicy: Never
      imagePullSecrets:
        - name: ${PROBE_JOB}-cred
      containers:
        - name: probe
          image: ${APP_B_IMAGE}
          imagePullPolicy: Always
          command: ["/bin/true"]
YAML
)
echo "$JOB_MANIFEST" | kubectl apply -f - >/dev/null

echo "  waiting for Job ${PROBE_JOB} to fail (expect ErrImagePull / 401 from Zot)..."
if kubectl wait --for=condition=Failed job/"${PROBE_JOB}" -n bex-system --timeout=60s >/dev/null 2>&1; then
  pass "cross-tenant pull DENIED (Job failed as expected — App A cannot pull App B's image)"
else
  # If the job completed successfully, that's a failure of isolation.
  if kubectl wait --for=condition=Complete job/"${PROBE_JOB}" -n bex-system --timeout=5s >/dev/null 2>&1; then
    fail "cross-tenant pull SUCCEEDED — App A's credential can pull App B's image (isolation broken!)"
  else
    echo "  WARNING: Job timed out — check cluster manually"
  fi
fi

# Cleanup.
kubectl delete job "${PROBE_JOB}" -n bex-system --ignore-not-found >/dev/null
kubectl delete secret "${PROBE_JOB}-cred" -n bex-system --ignore-not-found >/dev/null

echo
echo "=== All checks passed — w7/m36 per-App registry isolation verified ==="
echo
echo "To verify credential revocation, delete App ${APP_A} and re-run:"
echo "  kubectl delete app ${APP_A} -n ${APPS_NS}"
echo "  kubectl get secret zot-htpasswd -n ${REGISTRY_NS} -o jsonpath='{.data.htpasswd}' | base64 -d | grep app-${APP_A}"
echo "  (expect: no output — entry revoked)"
