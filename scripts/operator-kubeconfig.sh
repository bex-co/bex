#!/usr/bin/env bash
# Mint a scoped (non-system:masters) operator kubeconfig for day-to-day ops.
#
# The full admin kubeconfig (CN=kubernetes-admin, O=system:masters) is reserved
# for break-glass only (docs/ADR019-infra-credentials.md). This script generates a
# token-based kubeconfig for the bex-operator ServiceAccount (ClusterRole
# bex-operator-day-to-day, deployed by Argo from
# deploy/gitops/base/operator-daytoday-rbac.yaml): read + exec + job-cleanup
# access without cluster-admin. Job CREATION is break-glass since codex-security
# round-9 #2: `kubectl create job --from=cronjob/...` (backup/rekey runbooks)
# requires the admin kubeconfig.
#
# Usage:
#   KUBECONFIG=/path/to/admin.kubeconfig scripts/operator-kubeconfig.sh [output-path]
#
# output-path defaults to ~/.kube/bex-operator.kubeconfig
# Token TTL: 12h by default (codex-security round-6 #5 — the old 1-year token
# made theft of this exec-capable credential durable for a year). Re-run the
# script to re-mint; override with BEX_OPERATOR_TOKEN_TTL for a longer window
# when a task genuinely needs one (audited, deliberate choice).
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

# Mint a short-lived ServiceAccount token (12h default). Token-based
# kubeconfigs are immediately revocable (delete the SA to invalidate), unlike
# the client-cert admin.conf — and a short TTL bounds theft of this
# cluster-wide exec-capable credential to hours, not a year.
TTL="${BEX_OPERATOR_TOKEN_TTL:-12h}"
TOKEN=$(kubectl create token bex-operator -n kube-system --duration="$TTL")

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
# codex round-9 #2: a cluster-wide jobs:create is arbitrary pod-template
# authoring (privileged/host-mounted/any node) — the routine credential must
# never hold it; manual one-shot CronJob triggers are break-glass.
if KUBECONFIG="$OUT" kubectl auth can-i create jobs --all-namespaces 2>/dev/null \
    | grep -q '^yes$'; then
  echo "ERROR: scoped credential can create Jobs — RBAC misconfigured" >&2
  rm -f "$OUT"
  exit 1
fi

echo "Scoped operator kubeconfig -> $OUT"
echo "Day-to-day: export KUBECONFIG=$OUT"
echo "Break-glass: export KUBECONFIG=/path/to/admin.kubeconfig  (from fetch-app-kubeconfig.sh)"
