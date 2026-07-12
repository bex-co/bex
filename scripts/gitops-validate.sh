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

# Platform-side tenant-isolation lockdown shape (w6/m6 t004, docs/ADR022-tenant-isolation.md
# §platform-side). Each platform namespace must carry a default-deny-ingress
# NetworkPolicy (empty podSelector + policyTypes [Ingress]) and none may name the
# tenant apps namespace ("default") in an allow-list. This is the cluster-less,
# CI-runnable twin of the live reachability matrix in verify-tenant-isolation.sh —
# a regression here fails CI, not at a live penetration.
# Caveat: the namespace set below is hardcoded, so a NEW platform namespace added
# without a deny-tenant-ingress policy is not caught — extend the list when one is added.
POL="deploy/gitops/base/network-policies.yaml"
if [ -f "$POL" ]; then
  echo "==> $POL lockdown shape"
  # One row per deny-tenant-ingress policy: "<ns>:<podSelectorLen>:<policyTypes>".
  # Four invariants checked over this single pass, then the allow-list peer check
  # (a different query shape) as a second pass.
  rows="$(yq -N '. | select(.kind=="NetworkPolicy" and .metadata.name=="deny-tenant-ingress") | .metadata.namespace + ":" + ((.spec.podSelector | length) | tostring) + ":" + (.spec.policyTypes | join(","))' "$POL")"
  for ns in bex-system bex-registry secrets monitoring; do
    echo "$rows" | grep -q "^${ns}:" || { echo "FAIL: $ns has no deny-tenant-ingress NetworkPolicy" >&2; fail=1; }
  done
  # empty podSelector (default-deny all pods) — non-zero lengths are a regression.
  # awk (not grep) for field-2 comparison: BSD grep -E mishandles the [^:]+ form.
  bad="$(echo "$rows" | awk -F: '$2 != 0' || true)"
  [ -z "$bad" ] || { echo "FAIL: non-empty podSelector (must default-deny ALL pods): $bad" >&2; fail=1; }
  # policyTypes must include Ingress
  bad="$(echo "$rows" | grep -v Ingress || true)"
  [ -z "$bad" ] || { echo "FAIL: missing Ingress in policyTypes: $bad" >&2; fail=1; }
  # no allow-list peer may name the tenant apps namespace (bex-registry's build-pod
  # exception uses a podSelector gate, not a bare namespace allow, so it stays clean)
  cnt="$(yq -N '. | select(.kind=="NetworkPolicy") | .spec.ingress[].from[]? | .namespaceSelector.matchLabels["kubernetes.io/metadata.name"]' "$POL" | grep -cx default || true)"
  [ "$cnt" -eq 0 ] || { echo "FAIL: a platform NetworkPolicy allow-lists the tenant apps namespace (default)" >&2; fail=1; }
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# For each multi-source base Application with vendored values (kratos, hydra,
# openfga, openbao, traefik, ...): pull the pinned chart once from ITS repo, then
# render it with the base values alone and with each overlay's values layered on
# top (the same order Argo's valueFiles use — later wins). Globs every overlay
# dir, so a prod-only layer (e.g. openbao's server.ha.replicas: 3 or traefik's
# LoadBalancer) is rendered here, not first in prod. Namespace comes from the
# Application itself so this generalizes across components in different namespaces
# (auth vs secrets vs traefik).
for chart in kratos hydra openfga openbao traefik; do
  app="deploy/gitops/base/$chart.yaml"
  version="$(yq '.spec.sources[0].targetRevision' "$app")"
  repo="$(yq '.spec.sources[0].repoURL' "$app")"
  ns="$(yq '.spec.destination.namespace' "$app")"
  helm pull "$chart" --repo "$repo" --version "$version" -d "$tmp" \
    || { echo "FAIL: cannot pull $chart $version" >&2; fail=1; continue; }
  layerings=("deploy/gitops/base/values/$chart.values.yaml")
  for ov in deploy/gitops/overlays/*/values/$chart.values.yaml; do
    [ -f "$ov" ] && layerings+=("deploy/gitops/base/values/$chart.values.yaml -f $ov")
  done
  for values in "${layerings[@]}"; do
    echo "==> helm template $chart $version -n $ns -f $values"
    # shellcheck disable=SC2086 — $values intentionally splits into -f args
    helm template "$chart" "$tmp/$chart-$version.tgz" -n "$ns" -f $values >/dev/null \
      || { echo "FAIL: $chart values do not render against chart $version" >&2; fail=1; }
  done
done

# The authz model ships as DSL (model.fga, human-edited) + JSON (model.json,
# applied) — guard the pair against drift when the fga CLI is available.
if command -v fga >/dev/null 2>&1; then
  echo "==> fga model transform (model.fga vs model.json)"
  if ! diff <(fga model transform --file deploy/gitops/authz/model.fga | yq -o=json 'sort_keys(..)' -)             <(yq -o=json 'sort_keys(..)' deploy/gitops/authz/model.json) >/dev/null; then
    echo "FAIL: deploy/gitops/authz/model.fga and model.json have drifted — regenerate: fga model transform --file model.fga > model.json" >&2
    fail=1
  fi
else
  echo "WARN: fga CLI not installed — skipping model.fga <-> model.json drift check" >&2
fi

[ "$fail" -eq 0 ] && echo "PASS: gitops tree renders" || { echo "FAIL: see errors above" >&2; exit 1; }
