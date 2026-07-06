#!/usr/bin/env bash
# Validate the GitOps tree renders — pure client-side, no cluster:
#   1. kustomize build of base and every kustomize dir under overlays/ and
#      charts/ (globbed, so new components are covered automatically).
#   2. helm template of the pinned Ory charts against the vendored values files
#      (catches values that drift from the chart's schema), for both the base
#      values and the base+local layering the local overlay declares.
# Run locally before pushing gitops changes; CI runs it via .github/workflows/gitops.yml.
# Requires: kubectl (built-in kustomize), helm, yq v4.
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

for dir in deploy/gitops/base deploy/gitops/overlays/*/ deploy/gitops/charts/*/; do
  [ -f "$dir/kustomization.yaml" ] || continue # e.g. charts/opensandbox-controller is a Helm chart
  echo "==> kustomize build $dir"
  kubectl kustomize "$dir" >/dev/null || { echo "FAIL: $dir does not render" >&2; fail=1; }
done

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# For each Ory component: pull the pinned chart once, then render it with the
# base values alone and with the local overlay's values layered on top (the
# same order Argo's valueFiles use).
for chart in kratos hydra; do
  version="$(yq '.spec.sources[0].targetRevision' "deploy/gitops/base/$chart.yaml")"
  helm pull "$chart" --repo https://k8s.ory.sh/helm/charts --version "$version" -d "$tmp" \
    || { echo "FAIL: cannot pull $chart $version" >&2; fail=1; continue; }
  for values in \
    "deploy/gitops/base/values/$chart.values.yaml" \
    "deploy/gitops/base/values/$chart.values.yaml -f deploy/gitops/overlays/local/values/$chart.values.yaml"; do
    echo "==> helm template $chart $version -f $values"
    # shellcheck disable=SC2086 — $values intentionally splits into -f args
    helm template "$chart" "$tmp/$chart-$version.tgz" -n auth -f $values >/dev/null \
      || { echo "FAIL: $chart values do not render against chart $version" >&2; fail=1; }
  done
done

[ "$fail" -eq 0 ] && echo "PASS: gitops tree renders" || { echo "FAIL: see errors above" >&2; exit 1; }
