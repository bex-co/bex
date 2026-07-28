#!/usr/bin/env bash
# install-autoscaler.sh — the ONE installer for the cluster-autoscaler (w1/m3),
# used by prod CI (.github/workflows/app-cluster.yml, infra cluster) and the
# local mock (scripts/mock-cluster.sh, kind mgmt). Owns the chart version pin,
# release name, and namespace so the two environments cannot drift.
# Why it installs on the MANAGEMENT cluster, not via Argo: see the header of
# infra/clusterapi/autoscaler-values.yaml (the decision record).
#
# Usage: [CA_TAG=v1.34.5] scripts/install-autoscaler.sh [kube-context]
#
# CA_TAG pins the autoscaler IMAGE to the workload cluster's k8s minor (the
# chart default tracks the latest minor). This matters below the chart level:
# a newer CA's DRA informers watch resource.k8s.io/v1, which older apiservers
# (e.g. prod v1.31) don't serve — the informer cache never syncs and the main
# loop silently never starts (learned on prod, 2026-07-10). Unset ⇒ chart
# default, fine when the workload tracks recent k8s (the CAPD mock).
set -euo pipefail
cd "$(dirname "$0")/.."

CTX="${1:-}"
CA_TAG="${CA_TAG:-}"

# This script is the BOOTSTRAP-phase installer (management cluster, pre-pivot):
# CAPI objects in-cluster, workload reached via the CAPI-generated
# bex-kubeconfig secret. Post-pivot the autoscaler is Argo-managed in-cluster
# with clusterAPIMode=incluster-incluster (deploy/gitops/base/autoscaler.yaml);
# uninstall this release at pivot time (w1/m19 t007) so two autoscalers never
# manage the same MachineDeployments.
#
# --repo avoids `helm repo add` state (and a duplicate index fetch in CI);
# --wait replaces a rollout wait on the chart's derived deployment name.
helm upgrade --install cluster-autoscaler cluster-autoscaler \
  --repo https://kubernetes.github.io/autoscaler \
  ${CTX:+--kube-context=$CTX} \
  ${CA_TAG:+--set image.tag=$CA_TAG} \
  --set clusterAPIMode=kubeconfig-incluster \
  --set clusterAPIKubeconfigSecret=bex-kubeconfig \
  --version 9.58.0 --namespace default \
  -f infra/clusterapi/autoscaler-values.yaml \
  --wait --timeout 3m
