#!/usr/bin/env bash
# install-autoscaler.sh — the ONE installer for the cluster-autoscaler (w1/m3),
# used by prod CI (.github/workflows/app-cluster.yml, infra cluster) and the
# local mock (scripts/mock-cluster.sh, kind mgmt). Owns the chart version pin,
# release name, and namespace so the two environments cannot drift.
# Why it installs on the MANAGEMENT cluster, not via Argo: see the header of
# infra/clusterapi/autoscaler-values.yaml (the decision record).
#
# Usage: scripts/install-autoscaler.sh [kube-context]
set -euo pipefail
cd "$(dirname "$0")/.."

CTX="${1:-}"

# --repo avoids `helm repo add` state (and a duplicate index fetch in CI);
# --wait replaces a rollout wait on the chart's derived deployment name.
helm upgrade --install cluster-autoscaler cluster-autoscaler \
  --repo https://kubernetes.github.io/autoscaler \
  ${CTX:+--kube-context=$CTX} \
  --version 9.58.0 --namespace default \
  -f infra/clusterapi/autoscaler-values.yaml \
  --wait --timeout 3m
