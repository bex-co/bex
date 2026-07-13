#!/usr/bin/env bash
# Validate the GitOps tree renders — pure client-side, no cluster:
#   1. kustomize build of base and every kustomize dir under overlays/ and
#      charts/ (globbed, so new components are covered automatically).
#   2. helm template of the pinned Ory charts against the vendored values files
#      (catches values that drift from the chart's schema), for both the base
#      values and the base+local layering the local overlay declares.
#   3. promtool check + unit-test of the platform alerting rules embedded in
#      prometheus.yaml (w3/m6), against deploy/gitops/base/rules/alerts_test.yml.
# Run locally before pushing gitops changes; CI runs it via .github/workflows/gitops.yml.
# Requires: kubectl (built-in kustomize), helm, yq v4. Optional: fga, promtool
# (steps skipped with a WARN when absent; CI installs both).
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

# RBAC least-privilege guard (w7/m7, docs/ADR028-security-review.md): the operator
# (bex-manager-role) and bex-api (bex-api-role) must NOT grant cluster-wide secrets
# read. Secrets access is scoped to the apps namespace via namespace-scoped Roles in
# deploy/gitops/base/*-apps-rbac.yaml. A regression that re-broadens either
# ClusterRole to include "secrets" get/list/watch/* fails CI before it can merge.
# Checks the source ClusterRole files directly (no kustomize needed — the prefixed
# names in the rendered output cannot add rules not present in the source).
for rbac_file in lego/operator/config/rbac/role.yaml lego/operator/config/api/rbac.yaml; do
  if yq -N '. | select(.kind=="ClusterRole") | .rules[]? | select(.resources[]? == "secrets") | .verbs[]?' "$rbac_file" 2>/dev/null \
      | grep -qE '^(get|list|watch|\*)$'; then
    echo "FAIL: $rbac_file ClusterRole grants cluster-wide secrets read — scope to a namespace Role in deploy/gitops/base/*-apps-rbac.yaml" >&2
    fail=1
  fi
done

# Zot registry auth guard (w7/m8, docs/ADR022-tenant-isolation.md § Registry access
# control): the in-cluster registry must deny anonymous access — unauthenticated
# catalog/pull/push from a tenant build pod is a cross-tenant hole. Asserts the
# committed Application values carry the auth machinery (a pinned chart, a mounted
# custom config with htpasswd auth + accessControl whose defaultPolicy is empty —
# anonymous denied). Checks zot.yaml's values string directly (the chart is never
# rendered here, so the source values are the contract). A regression that drops
# auth, widens the anonymous policy, or unpins the chart fails CI.
ZOT="deploy/gitops/base/zot.yaml"
if [ -f "$ZOT" ]; then
  echo "==> $ZOT registry-auth shape"
  # Chart pinned to an exact version (a bare * or x.* wildcard is a regression).
  rev="$(yq '.spec.source.targetRevision' "$ZOT")"
  echo "$rev" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+' \
    || { echo "FAIL: zot targetRevision is '$rev' — pin an exact chart version (no wildcard)" >&2; fail=1; }
  vals="$(yq '.spec.source.helm.values' "$ZOT")"
  # A custom config carrying auth must be shipped (not the chart's auth-off defaults).
  echo "$vals" | yq -e '.mountConfig == true' >/dev/null \
    || { echo "FAIL: zot mountConfig must be true (ship the authed config)" >&2; fail=1; }
  # The auth/accessControl tokens must all be present (htpasswd auth, the accessControl
  # block, and the two named users). One grep pass over the committed values string.
  echo "$vals" | grep -qE '"(htpasswd|accessControl|bex-builder|bex-puller)"' \
    || { echo "FAIL: zot config missing auth/accessControl tokens (htpasswd/accessControl/bex-builder/bex-puller)" >&2; fail=1; }
  # defaultPolicy must be empty — anonymous denied everything (catalog/pull/push).
  echo "$vals" | grep -q '"defaultPolicy": *\[\]' \
    || { echo "FAIL: zot accessControl.defaultPolicy must be [] (anonymous denied)" >&2; fail=1; }
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

# Platform alerting rules (w3/m6): the rule pack is the single source of truth
# embedded in prometheus.yaml's Helm values (serverFiles.alerting_rules.yml).
# Extract it and run promtool check + unit tests (deploy/gitops/base/rules/) so a
# broken expression or regressed age-math/ratio fails CI, not prod.
if command -v promtool >/dev/null 2>&1; then
  echo "==> promtool check + test rules (extracted from prometheus.yaml)"
  # helm `values:` is a block-scalar string — from_yaml re-parses it in-process so
  # we can pull the rule groups out in a single yq (no second pipe stage).
  yq '.spec.source.helm.values | from_yaml | {"groups": .serverFiles."alerting_rules.yml".groups}' \
    deploy/gitops/base/prometheus.yaml >"$tmp/alerting_rules.yml"
  cp deploy/gitops/base/rules/alerts_test.yml "$tmp/alerts_test.yml"
  # Run inside $tmp so the test file's `rule_files: [alerting_rules.yml]` resolves
  # to the freshly-extracted pack (promtool resolves rule_files from the CWD).
  ( cd "$tmp" && promtool check rules alerting_rules.yml && promtool test rules alerts_test.yml ) \
    || { echo "FAIL: alerting rules do not check/test clean — see deploy/gitops/base/prometheus.yaml + rules/alerts_test.yml" >&2; fail=1; }
else
  echo "WARN: promtool not installed — skipping alerting-rule check/test (docs/ADR010-observability.md)" >&2
fi

# bex-db backup guard (w2/m27 t009): spec.backup.barmanObjectStore must be present
# so a future edit can't silently drop the backup config. Same structural-manifest
# pattern as the network-policy and RBAC checks above.
echo "==> bex-db backup config present (deploy/gitops/charts/bex-postgres/cluster.yaml)"
barman="$(yq '.spec.backup.barmanObjectStore' deploy/gitops/charts/bex-postgres/cluster.yaml)"
if [ "$barman" = "null" ] || [ -z "$barman" ]; then
  echo "FAIL: cluster.yaml missing spec.backup.barmanObjectStore — bex-db has no backup config (see docs/ADR031-platform-data-backup.md)" >&2
  fail=1
fi

[ "$fail" -eq 0 ] && echo "PASS: gitops tree renders" || { echo "FAIL: see errors above" >&2; exit 1; }
