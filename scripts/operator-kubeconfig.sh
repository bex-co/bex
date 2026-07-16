#!/usr/bin/env bash
# Mint a scoped (non-system:masters) operator kubeconfig for day-to-day ops.
#
# The full admin kubeconfig (CN=kubernetes-admin, O=system:masters) is reserved
# for break-glass only (docs/ADR019-infra-credentials.md). This script generates a
# token-based kubeconfig for the bex-operator ServiceAccount (ClusterRole
# bex-operator-day-to-day, deployed by Argo from
# deploy/gitops/base/operator-daytoday-rbac.yaml): read + exec + one-shot-job
# access without cluster-admin.
#
# Usage:
#   KUBECONFIG=/path/to/admin.kubeconfig scripts/operator-kubeconfig.sh [output-path]
#
# output-path defaults to ~/.kube/bex-operator.kubeconfig
# Token TTL: 1 year (same lifecycle as the admin cert; re-run annually).
# After generation, use the new kubeconfig for day-to-day access:
#   export KUBECONFIG=~/.kube/bex-operator.kubeconfig
# For break-glass, revert to the admin kubeconfig fetched by fetch-app-kubeconfig.sh.
#
# Requires: kubectl >=1.24 (pointing at the target cluster via KUBECONFIG or the
# default context). The bex-operator SA must already exist (deployed by Argo).
set -euo pipefail

OUT="${1:-$HOME/.kube/bex-operator.kubeconfig}"

# Verify the SA exists — it is deployed by Argo, not created here.
kubectl -n kube-system get serviceaccount bex-operator >/dev/null

# Mint a 1-year ServiceAccount token. Token-based kubeconfigs are immediately
# revocable (delete the SA to invalidate), unlike the client-cert admin.conf.
TOKEN=$(kubectl create token bex-operator -n kube-system --duration=8760h)

# Extract server URL and cluster CA from the currently active kubeconfig.
SERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
CA_DATA=$(kubectl config view --minify --raw \
  -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

mkdir -p "$(dirname "$OUT")"
cat > "$OUT" <<EOF
apiVersion: v1
kind: Config
clusters:
  - name: bex
    cluster:
      server: ${SERVER}
      certificate-authority-data: ${CA_DATA}
contexts:
  - name: bex-operator@bex
    context:
      cluster: bex
      user: bex-operator
current-context: bex-operator@bex
users:
  - name: bex-operator
    user:
      token: ${TOKEN}
EOF
chmod 600 "$OUT"

# Smoke-test: list is permitted, admin-level writes are denied.
KUBECONFIG="$OUT" kubectl -n bex-system get pods >/dev/null
if KUBECONFIG="$OUT" kubectl auth can-i create deployments --all-namespaces 2>/dev/null \
    | grep -q '^yes$'; then
  echo "ERROR: scoped credential can create Deployments — RBAC misconfigured" >&2
  rm -f "$OUT"
  exit 1
fi
if KUBECONFIG="$OUT" kubectl auth can-i create clusterroles 2>/dev/null \
    | grep -q '^yes$'; then
  echo "ERROR: scoped credential can create ClusterRoles — RBAC misconfigured" >&2
  rm -f "$OUT"
  exit 1
fi

echo "Scoped operator kubeconfig -> $OUT"
echo "Day-to-day: export KUBECONFIG=$OUT"
echo "Break-glass: export KUBECONFIG=/path/to/admin.kubeconfig  (from fetch-app-kubeconfig.sh)"
