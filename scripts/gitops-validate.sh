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
# Requires: kubectl (built-in kustomize), helm, yq v4, ssh-keygen. Optional: fga, promtool
# (steps skipped with a WARN when absent; CI installs both).
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

# Stable-edge ownership guard (w1/m41): the API-level protection keeps hcloud
# CCM's Service finalizer from deleting the adopted object, while prevent_destroy
# independently blocks Terraform from planning its destruction. Keep both.
echo "==> Terraform-owned Traefik load balancer has both deletion guards"
terraform_lb_block="$(awk '
  /^resource "hcloud_load_balancer" "traefik" \{/ { found=1 }
  found { print }
  found && /^  }$/ { exit }
' infra/terraform/main.tf)"
if [ -z "$terraform_lb_block" ]; then
  echo "FAIL: infra/terraform/main.tf has no hcloud_load_balancer.traefik resource" >&2
  fail=1
fi
if ! grep -Eq '^[[:space:]]*name[[:space:]]*=[[:space:]]*"bex-traefik"[[:space:]]*$' <<<"$terraform_lb_block"; then
  echo "FAIL: Terraform Traefik load balancer must keep the stable bex-traefik name" >&2
  fail=1
fi
if ! grep -Eq '^[[:space:]]*delete_protection[[:space:]]*=[[:space:]]*true[[:space:]]*$' <<<"$terraform_lb_block"; then
  echo "FAIL: Terraform Traefik load balancer must enable Hetzner delete_protection" >&2
  fail=1
fi
if ! grep -Eq '^[[:space:]]*prevent_destroy[[:space:]]*=[[:space:]]*true[[:space:]]*$' <<<"$terraform_lb_block"; then
  echo "FAIL: Terraform Traefik load balancer must keep lifecycle.prevent_destroy" >&2
  fail=1
fi

echo "==> Terraform is the sole owner of all production edge listeners"
terraform_target_block="$(awk '
  /^resource "hcloud_load_balancer_target" "traefik_workers" \{/ { found=1 }
  found { print }
  found && /^}$/ { exit }
' infra/terraform/main.tf)"
for required in \
  'type             = "label_selector"' \
  'label_selector   = "caph-cluster-bex=owned,machine_type=worker"' \
  'use_private_ip   = true'; do
  grep -qF "$required" <<<"$terraform_target_block" || {
    echo "FAIL: Terraform Traefik worker target lost: $required" >&2
    fail=1
  }
done
for edge in ssh:22:32207 http:80:31218 https:443:31976 postgres:5432:31056 valkey:6379:31892; do
  name="${edge%%:*}"
  ports="${edge#*:}"
  listen="${ports%%:*}"
  destination="${ports##*:}"
  block="$(awk -v name="$name" '
    $0 == "resource \"hcloud_load_balancer_service\" \"" name "\" {" { found=1 }
    found { print }
    found && /^}$/ { exit }
  ' infra/terraform/main.tf)"
  if [ -z "$block" ] ||
    ! grep -Eq "^[[:space:]]*listen_port[[:space:]]*=[[:space:]]*${listen}[[:space:]]*$" <<<"$block" ||
    ! grep -Eq "^[[:space:]]*destination_port[[:space:]]*=[[:space:]]*${destination}[[:space:]]*$" <<<"$block" ||
    ! grep -Eq "^[[:space:]]*port[[:space:]]*=[[:space:]]*${destination}[[:space:]]*$" <<<"$block"; then
    echo "FAIL: Terraform edge listener $name must map :$listen to NodePort $destination with the same health-check port" >&2
    fail=1
  fi
  expected_proxyprotocol=false
  case "$name" in
    postgres | valkey) expected_proxyprotocol=true ;;
  esac
  if ! grep -Eq "^[[:space:]]*proxyprotocol[[:space:]]*=[[:space:]]*${expected_proxyprotocol}[[:space:]]*$" <<<"$block"; then
    echo "FAIL: Terraform edge listener $name must set proxyprotocol=$expected_proxyprotocol" >&2
    fail=1
  fi
done

# Enabling PROXY protocol before header-capable proxy pods are Ready breaks both
# datastore front doors. Terraform must apply a saved plan only after the same
# commit's deploy workflow succeeds, and that workflow must wait for the exact
# freshly built digest on both proxy DaemonSets.
echo "==> datastore PROXY protocol rollout ordering"
for required in \
  'terraform plan -no-color -out=tfplan' \
  'terraform show -json tfplan' \
  'actions/workflows/deploy.yml/runs?head_sha=${GITHUB_SHA}' \
  'terraform apply -auto-approve -no-color tfplan'; do
  grep -qF "$required" .github/workflows/infra.yml || {
    echo "FAIL: infra workflow lost datastore PROXY rollout gate: $required" >&2
    fail=1
  }
done
for required in \
  'BEX_EXPECTED_PROXY_IMAGE: ${{ env.IMAGE }}@${{ steps.build_operator.outputs.digest }}' \
  'bex-pg-sni-proxy bex-kv-sni-proxy' \
  'rollout status "daemonset/${daemonset}"'; do
  grep -qF "$required" .github/workflows/deploy.yml || {
    echo "FAIL: deploy workflow lost datastore proxy digest gate: $required" >&2
    fail=1
  }
done

echo "==> SSH activation safety gates"
bash scripts/ssh-activate.test.sh || { echo "FAIL: SSH activation safety gates" >&2; fail=1; }

echo "==> SSH Cloudflare DNS reconciliation safety gates"
bash scripts/ssh-dns-cloudflare.test.sh || { echo "FAIL: SSH Cloudflare DNS reconciliation safety gates" >&2; fail=1; }

echo "==> datastore Cloudflare DNS reconciliation safety gates"
bash scripts/datastore-dns-cloudflare.test.sh || { echo "FAIL: datastore Cloudflare DNS reconciliation safety gates" >&2; fail=1; }

echo "==> SSH verifier CLI safety gates"
bash scripts/ssh-verify.test.sh || { echo "FAIL: SSH verifier CLI safety gates" >&2; fail=1; }

for dir in deploy/gitops/base deploy/gitops/overlays/*/ deploy/gitops/charts/*/; do
  [ -f "$dir/kustomization.yaml" ] || continue # e.g. charts/opensandbox-controller is a Helm chart
  echo "==> kustomize build $dir"
  kubectl kustomize "$dir" >/dev/null || { echo "FAIL: $dir does not render" >&2; fail=1; }
done

# The production kernel requires CAP_SYS_ADMIN to create the meter's private
# pin directory on Cilium's bpffs mount. Keep the complete set explicit: the
# container drops ALL first and remains non-privileged, but omitting any one of
# these capabilities leaves outbound accounting unready after rollout.
echo "==> egress-meter has the production bpffs/BPF capability set"
egress_caps="$(yq -N '.spec.template.spec.containers[] | select(.name == "egress-meter") | .securityContext.capabilities.add | sort | join(",")' lego/operator/config/egress-meter/daemonset.yaml)"
if [ "$egress_caps" != "BPF,NET_ADMIN,PERFMON,SYS_ADMIN,SYS_RESOURCE" ]; then
  echo "FAIL: egress-meter capabilities are '$egress_caps', want BPF,NET_ADMIN,PERFMON,SYS_ADMIN,SYS_RESOURCE" >&2
  fail=1
fi
egress_apparmor="$(yq -N '.spec.template.spec.securityContext.appArmorProfile.type' lego/operator/config/egress-meter/daemonset.yaml)"
egress_seccomp="$(yq -N '.spec.template.spec.containers[] | select(.name == "egress-meter") | .securityContext.seccompProfile.type' lego/operator/config/egress-meter/daemonset.yaml)"
if [ "$egress_apparmor" != "Unconfined" ] || [ "$egress_seccomp" != "RuntimeDefault" ]; then
  echo "FAIL: egress-meter needs AppArmor Unconfined + seccomp RuntimeDefault for production BPF pinning; got $egress_apparmor + $egress_seccomp" >&2
  fail=1
fi

# Platform CNPG drain-safety guard (w1/m38): the auth/control-plane databases
# must have a standby on another platform node. CNPG performs a planned
# switchover before a drain evicts a primary, but only when a replica exists;
# required anti-affinity makes that replica a real node-failure boundary rather
# than a second pod on the same node. Explicit `primaryUpdateMethod: switchover`
# also keeps later pod-template changes from restarting the primary in place.
echo "==> platform CNPG clusters have HA + required pod anti-affinity"
for db in \
  "deploy/gitops/charts/auth-dbs/kratos-db.yaml:auth:kratos-db" \
  "deploy/gitops/charts/auth-dbs/hydra-db.yaml:auth:hydra-db" \
  "deploy/gitops/charts/auth-dbs/openfga-db.yaml:auth:openfga-db" \
  "deploy/gitops/charts/bex-postgres/cluster.yaml:bex-system:bex-db"; do
  manifest="${db%%:*}"
  identity="${db#*:}"
  namespace="${identity%%:*}"
  name="${identity##*:}"
  actual_identity="$(yq -N '.metadata.namespace + ":" + .metadata.name' "$manifest")"
  instances="$(yq -N '.spec.instances' "$manifest")"
  anti_affinity="$(yq -N '.spec.affinity.podAntiAffinityType' "$manifest")"
  update_method="$(yq -N '.spec.primaryUpdateMethod' "$manifest")"

  if [ "$actual_identity" != "$namespace:$name" ]; then
    echo "FAIL: $manifest declares '$actual_identity', want '$namespace:$name'" >&2
    fail=1
  fi
  if ! [[ "$instances" =~ ^[0-9]+$ ]] || [ "$instances" -lt 2 ]; then
    echo "FAIL: $namespace/$name has spec.instances '$instances', want >=2 for drain-safe failover" >&2
    fail=1
  fi
  if [ "$anti_affinity" != "required" ]; then
    echo "FAIL: $namespace/$name has affinity.podAntiAffinityType '$anti_affinity', want required" >&2
    fail=1
  fi
  if [ "$update_method" != "switchover" ]; then
    echo "FAIL: $namespace/$name has primaryUpdateMethod '$update_method', want switchover" >&2
    fail=1
  fi
done

# The bex-db instance manager lives behind bex-system's default-deny ingress
# policy. Without an explicit cnpg-system peer, the operator cannot extract
# status, create a standby, or coordinate a switchover even when the Cluster
# manifest itself is HA-shaped.
cnpg_bex_ingress="$(yq -N '. | select(.kind == "NetworkPolicy" and .metadata.namespace == "bex-system" and .metadata.name == "allow-cnpg-bex-db-management") | [.spec.podSelector.matchLabels["cnpg.io/cluster"], .spec.ingress[0].from[0].namespaceSelector.matchLabels["kubernetes.io/metadata.name"], .spec.ingress[0].ports[0].protocol, .spec.ingress[0].ports[0].port] | join(":")' deploy/gitops/base/network-policies.yaml | tr -d '\n')"
if [ "$cnpg_bex_ingress" != "bex-db:cnpg-system:TCP:8000" ]; then
  echo "FAIL: bex-db must allow cnpg-system instance management on TCP 8000 only" >&2
  fail=1
fi

# App SSH gateway trust-boundary guard (w2/m39, docs/ADR035-ssh.md): only the
# namespace Role may grant pods/exec, and it grants create only. The bex-api
# Role must stay read-only for pods and must never acquire exec.
echo "==> SSH gateway pods/exec isolation"
SSH_RBAC="deploy/gitops/base/bex-ssh-apps-rbac.yaml"
exec_verbs="$(yq -N '. | select(.kind == "Role") | .rules[] | select(.resources[] == "pods/exec") | .verbs | join(",")' "$SSH_RBAC")"
if [ "$exec_verbs" != "create" ]; then
  echo "FAIL: SSH gateway pods/exec verbs are '$exec_verbs', want exactly create" >&2
  fail=1
fi
if yq -N '. | select(.kind == "Role" or .kind == "ClusterRole") | .rules[]? | .resources[]?' \
    deploy/gitops/base/bex-api-apps-rbac.yaml lego/operator/config/api/rbac.yaml \
    | grep -qx 'pods/exec'; then
  echo "FAIL: bex-api RBAC gained pods/exec — keep exec isolated to the SSH gateway" >&2
  fail=1
fi
ssh_metrics_port="$(yq -N '.spec.ports[] | select(.name == "metrics") | .port' lego/operator/config/ssh/service.yaml)"
if [ "$ssh_metrics_port" != "9090" ] || ! grep -q 'bex-ssh-gateway.bex-system.svc:9090' deploy/gitops/base/prometheus.yaml; then
  echo "FAIL: SSH gateway metrics must remain internal on :9090 and Prometheus-scraped" >&2
  fail=1
fi
if yq -N '. | select(.kind == "Role" or .kind == "ClusterRole") | .rules[]? | .resources[]?' \
    "$SSH_RBAC" | grep -qx 'secrets'; then
  echo "FAIL: SSH gateway RBAC must never grant Secret access" >&2
  fail=1
fi
bex_api_ingress_verbs="$(yq -N '. | select(.kind == "Role" and .metadata.name == "bex-api-apps") | .rules[] | select(.apiGroups[] == "networking.k8s.io" and .resources[] == "ingresses") | .verbs | sort | join(",")' deploy/gitops/base/bex-api-apps-rbac.yaml)"
if [ "$bex_api_ingress_verbs" != "get,list" ]; then
  echo "FAIL: bex-api tenant Role needs exactly get,list on Ingresses for exact-router egress accounting; got '$bex_api_ingress_verbs'" >&2
  fail=1
fi
ssh_cluster_binding="$(kubectl kustomize lego/operator/config/default | yq -N '. | select(.kind == "ClusterRoleBinding") | .subjects[]? | select(.kind == "ServiceAccount" and .name == "bex-ssh-gateway") | .name')"
if [ -n "$ssh_cluster_binding" ]; then
  echo "FAIL: SSH gateway must not have cluster-wide RBAC" >&2
  fail=1
fi
ssh_namespace="$(yq -N '. | select(.kind == "Role") | .metadata.namespace' "$SSH_RBAC")"
ssh_namespaced_rules="$(yq -N '. | select(.kind == "Role") | .rules[] | [.apiGroups | join(","), .resources | join(","), .verbs | sort | join(",")] | join("|")' "$SSH_RBAC")"
if [ "$ssh_namespace" != 'default' ] || [ "$ssh_namespaced_rules" != $'app.bex.co|apps|get,list\n|pods|get,list\n|pods/exec|create' ]; then
  echo "FAIL: SSH gateway tenant Role must remain default-only App/pod get/list + pods/exec create" >&2
  fail=1
fi
# Traefik reaches the gateway on 2222 (native SSH) + 8080 (Browser Web Shell
# WebSocket, w2/m55); monitoring scrapes 9090. Enumerate every port per rule so a
# stray added port can't slip past a ports[0]-only check.
ssh_ingress="$(yq -N '.spec.ingress[] | [.from[0].namespaceSelector.matchLabels."kubernetes.io/metadata.name", (.ports | map(.port | tostring) | join(","))] | join(":")' lego/operator/config/ssh/networkpolicy.yaml)"
if [ "$ssh_ingress" != $'traefik:2222,8080\nmonitoring:9090' ]; then
  echo "FAIL: SSH gateway ingress must remain Traefik SSH+shell + monitoring metrics only" >&2
  fail=1
fi
ssh_rendered="$(kubectl kustomize lego/operator/config/default)"
ssh_service_name="$(yq -N 'select(.kind == "Service" and .metadata.name == "bex-ssh-gateway") | .metadata.name' <<<"$ssh_rendered")"
ssh_route_service="$(yq -N 'select(.kind == "IngressRouteTCP" and .metadata.name == "bex-ssh-gateway") | .spec.routes[0].services[0].name' <<<"$ssh_rendered")"
if [ -z "$ssh_service_name" ] || [ "$ssh_route_service" != "$ssh_service_name" ]; then
  echo "FAIL: rendered SSH TCP route targets '$ssh_route_service', want rendered Service '$ssh_service_name'" >&2
  fail=1
fi
ssh_prod_values="deploy/gitops/overlays/prod/values/traefik.values.yaml"
ssh_prod_port="$(yq -N '[.ports.ssh.port, .ports.ssh.exposedPort, .ports.ssh.expose.default, .ports.ssh.protocol] | join(":")' "$ssh_prod_values")"
if [ "$ssh_prod_port" != '2222:22:true:TCP' ]; then
  echo "FAIL: production Traefik SSH port must expose TCP/22 to container entrypoint 2222; got '$ssh_prod_port'" >&2
  fail=1
fi

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
  # Per-App pull credentials (w7/m36): the operator manages the full Zot config via
  # the zot-config Secret, so mountConfig must be false (chart must not override it).
  echo "$vals" | yq -e '.mountConfig == false' >/dev/null \
    || { echo "FAIL: zot mountConfig must be false (operator manages config via zot-config Secret, w7/m36)" >&2; fail=1; }
  # Both externalSecrets (zot-config and zot-htpasswd) must be present.
  echo "$vals" | grep -q 'zot-config' \
    || { echo "FAIL: zot externalSecrets must reference zot-config Secret (operator-managed config, w7/m36)" >&2; fail=1; }
  echo "$vals" | grep -q 'zot-htpasswd' \
    || { echo "FAIL: zot externalSecrets must reference zot-htpasswd Secret" >&2; fail=1; }
  # The bex-puller shared credential must NOT appear in the Zot chart values —
  # it is absent from the operator-managed per-App scheme (ADR022:204 closed).
  if echo "$vals" | grep -q 'bex-puller'; then
    echo "FAIL: zot values must not reference bex-puller (shared credential removed in w7/m36)" >&2; fail=1
  fi
  # Kubernetes defaults apiVersion/kind on StatefulSet PVC templates. Once Argo
  # removes the ignored immutable storage request, it also has to ignore the
  # whole requests map or the resulting empty-map shape self-heals forever.
  zot_ignores="$(yq -r '.spec.ignoreDifferences[] | select(.group == "apps" and .kind == "StatefulSet" and .name == "zot") | .jsonPointers[]' "$ZOT")"
  for pointer in \
    /spec/volumeClaimTemplates/0/apiVersion \
    /spec/volumeClaimTemplates/0/kind \
    /spec/volumeClaimTemplates/0/spec/resources/requests; do
    printf '%s\n' "$zot_ignores" | grep -qxF "$pointer" \
      || { echo "FAIL: zot ignoreDifferences is missing $pointer (prevents StatefulSet PVC normalization drift)" >&2; fail=1; }
  done
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

# The platform databases can fail over only if the services above them remain
# reachable. Render the production layers and pin every synchronous request-path
# Deployment to two node-separated replicas plus a one-available PDB. This also
# proves the production-only bex-api overlay renders without making the one-node
# local CAPD cluster permanently Progressing.
echo "==> production auth/control-plane request path is drain-safe"
helm template hydra "$tmp/hydra-$(yq '.spec.sources[0].targetRevision' deploy/gitops/base/hydra.yaml).tgz" -n auth \
  -f deploy/gitops/base/values/hydra.values.yaml \
  -f deploy/gitops/overlays/prod/values/hydra.values.yaml >"$tmp/hydra-prod.yaml"
helm template kratos "$tmp/kratos-$(yq '.spec.sources[0].targetRevision' deploy/gitops/base/kratos.yaml).tgz" -n auth \
  -f deploy/gitops/base/values/kratos.values.yaml \
  -f deploy/gitops/overlays/prod/values/kratos.values.yaml >"$tmp/kratos-prod.yaml"
helm template openfga "$tmp/openfga-$(yq '.spec.sources[0].targetRevision' deploy/gitops/base/openfga.yaml).tgz" -n auth \
  -f deploy/gitops/base/values/openfga.values.yaml \
  -f deploy/gitops/overlays/prod/values/openfga.values.yaml >"$tmp/openfga-prod.yaml"
helm template traefik "$tmp/traefik-$(yq '.spec.sources[0].targetRevision' deploy/gitops/base/traefik.yaml).tgz" -n traefik \
  -f deploy/gitops/base/values/traefik.values.yaml \
  -f deploy/gitops/overlays/prod/values/traefik.values.yaml >"$tmp/traefik-prod.yaml"
kubectl kustomize deploy/gitops/overlays/prod >"$tmp/prod-apps.yaml"
kubectl kustomize lego/operator/config/prod >"$tmp/bex-operator-prod.yaml"

check_fixed_edge_nodeport() {
  local label="$1" manifest="$2" service="$3" port_name="$4" expected="$5"
  local service_type traffic_policy node_port lb_name
  service_type="$(yq -N "select(.kind == \"Service\" and .metadata.name == \"$service\") | .spec.type" "$manifest" | tr -d '\n')"
  traffic_policy="$(yq -N "select(.kind == \"Service\" and .metadata.name == \"$service\") | .spec.externalTrafficPolicy" "$manifest" | tr -d '\n')"
  node_port="$(yq -N "select(.kind == \"Service\" and .metadata.name == \"$service\") | .spec.ports[] | select(.name == \"$port_name\") | .nodePort" "$manifest" | tr -d '\n')"
  lb_name="$(yq -N "select(.kind == \"Service\" and .metadata.name == \"$service\") | .metadata.annotations.\"load-balancer.hetzner.cloud/name\"" "$manifest" | tr -d '\n')"
  if [ "$service_type" != NodePort ] || [ "$traffic_policy" != Local ] || [ "$node_port" != "$expected" ] || { [ -n "$lb_name" ] && [ "$lb_name" != null ]; }; then
    echo "FAIL: $label must render NodePort $expected with externalTrafficPolicy=Local and no hcloud LB adoption annotation (got type=$service_type policy=$traffic_policy nodePort=$node_port annotation=$lb_name)" >&2
    fail=1
  fi
}

echo "==> production edge Services expose fixed local-policy NodePorts"
check_fixed_edge_nodeport Traefik-SSH "$tmp/traefik-prod.yaml" traefik ssh 32207
check_fixed_edge_nodeport Traefik-HTTP "$tmp/traefik-prod.yaml" traefik web 31218
check_fixed_edge_nodeport Traefik-HTTPS "$tmp/traefik-prod.yaml" traefik websecure 31976
check_fixed_edge_nodeport PostgreSQL-proxy "$tmp/bex-operator-prod.yaml" bex-pg-sni-proxy-public postgres 31056
check_fixed_edge_nodeport Valkey-proxy "$tmp/bex-operator-prod.yaml" bex-kv-sni-proxy-public valkey 31892

echo "==> production raw-TCP edge proxies are admitted through default-deny"
for proxy in pg-sni-proxy kv-sni-proxy; do
  selected="$(yq -N "select(.kind == \"NetworkPolicy\" and .metadata.name == \"allow-production-edge-proxies\") | .spec.podSelector.matchExpressions[] | select(.key == \"app.bex.co/component\" and .operator == \"In\") | .values[] | select(. == \"$proxy\")" "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
  if [ "$selected" != "$proxy" ]; then
    echo "FAIL: production edge NetworkPolicy does not select $proxy" >&2
    fail=1
  fi
done
echo "==> datastore proxies trust only the fixed Terraform load balancer address"
for proxy in pg-sni-proxy kv-sni-proxy; do
  trusted_proxy_cidrs="$(yq -N "select(.kind == \"DaemonSet\" and .metadata.name == \"bex-$proxy\") | .spec.template.spec.containers[] | select(.name == \"$proxy\") | .env[] | select(.name == \"BEX_PROXY_PROTOCOL_TRUSTED_CIDRS\") | .value" "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
  if [ "$trusted_proxy_cidrs" != 10.10.0.7/32 ]; then
    echo "FAIL: $proxy trusts '$trusted_proxy_cidrs' for PROXY protocol, want only 10.10.0.7/32" >&2
    fail=1
  fi
done
for port in 5432 6379; do
  admitted="$(yq -N "select(.kind == \"NetworkPolicy\" and .metadata.name == \"allow-production-edge-proxies\") | .spec.ingress[] | select(.from == null) | .ports[] | select(.protocol == \"TCP\" and .port == $port) | .port" "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
  if [ "$admitted" != "$port" ]; then
    echo "FAIL: production edge NetworkPolicy does not publicly admit TCP/$port" >&2
    fail=1
  fi
done

check_request_path_ha() {
  local label="$1" manifest="$2" deployment="$3" pdb_manifest="$4" pdb="$5"
  local replicas required min_available
  replicas="$(yq -N "select(.kind == \"Deployment\" and .metadata.name == \"$deployment\") | .spec.replicas" "$manifest" | tr -d '\n')"
  required="$(yq -N "select(.kind == \"Deployment\" and .metadata.name == \"$deployment\") | .spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution | length" "$manifest" | tr -d '\n')"
  min_available="$(yq -N "select(.kind == \"PodDisruptionBudget\" and .metadata.name == \"$pdb\") | .spec.minAvailable" "$pdb_manifest" | tr -d '\n')"
  if ! [[ "$replicas" =~ ^[0-9]+$ ]] || (( replicas < 2 )); then
    echo "FAIL: $label renders replicas '$replicas', want >=2" >&2
    fail=1
  fi
  if ! [[ "$required" =~ ^[0-9]+$ ]] || (( required < 1 )); then
    echo "FAIL: $label has no required hostname pod anti-affinity" >&2
    fail=1
  fi
  if [ "$min_available" != "1" ]; then
    echo "FAIL: $label PDB minAvailable is '$min_available', want 1" >&2
    fail=1
  fi
}

check_request_path_ha hydra "$tmp/hydra-prod.yaml" hydra "$tmp/hydra-prod.yaml" hydra
check_request_path_ha kratos "$tmp/kratos-prod.yaml" kratos "$tmp/kratos-prod.yaml" kratos
check_request_path_ha openfga "$tmp/openfga-prod.yaml" openfga "$tmp/prod-apps.yaml" openfga
check_request_path_ha traefik "$tmp/traefik-prod.yaml" traefik "$tmp/traefik-prod.yaml" traefik
check_request_path_ha bex-api "$tmp/bex-operator-prod.yaml" bex-api "$tmp/bex-operator-prod.yaml" bex-api

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

# etcd snapshot image guard (w7/m29 drill): the CronJob's snapshot image must be ≥3.6.x.
# etcdutl (required for 'snapshot restore' in the runbook) ships only in 3.6.x+ images.
# A 3.5.x pin breaks the restore path even though the backup itself succeeds.
echo "==> etcd-backup CronJob image version ≥3.6 (deploy/gitops/charts/etcd-backup/cronjob.yaml)"
etcd_img="$(yq '.spec.jobTemplate.spec.template.spec.initContainers[] | select(.name == "snapshot") | .image' \
  deploy/gitops/charts/etcd-backup/cronjob.yaml)"
etcd_minor="$(echo "$etcd_img" | grep -oE '3\.[0-9]+' | head -1)"
if [ -z "$etcd_minor" ]; then
  echo "FAIL: could not parse etcd version from image '$etcd_img'" >&2; fail=1
else
  etcd_m="$(echo "$etcd_minor" | cut -d. -f2)"
  if [ "$etcd_m" -lt 6 ]; then
    echo "FAIL: etcd-backup CronJob uses '$etcd_img' (3.$etcd_m.x < 3.6.x) — restore requires etcdutl, which only ships in ≥3.6.x images (docs/ADR011-etcd-backup-restore.md)" >&2; fail=1
  fi
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

# log-shipper node-scope guard (w3/m13): every Alloy replica runs as a DaemonSet
# pod, one per node. Without a node-scoped field selector on the shared
# `discovery.kubernetes "pods"` block, every replica discovers and tails EVERY
# pod cluster-wide, so each log line ships once per node (N×) instead of once —
# the duplication bug this milestone fixed. Guard against it silently coming
# back: the committed Alloy River config must carry the field-selector
# expression, keyed off K8S_NODE_NAME — a downward-API env var the alloy chart
# itself injects into every replica unconditionally (containers/_agent.yaml),
# so there's no extraEnv of our own to check for. Checks the source values
# string directly (same shape as the zot auth guard above) — no chart render
# needed to catch a regression at the source.
LOGSHIP="deploy/gitops/base/log-shipper.yaml"
if [ -f "$LOGSHIP" ]; then
  echo "==> $LOGSHIP node-scoped pod discovery"
  vals="$(yq '.spec.source.helm.values' "$LOGSHIP")"
  echo "$vals" | grep -qF 'field = "spec.nodeName=" + sys.env("K8S_NODE_NAME")' \
    || { echo "FAIL: log-shipper.yaml's discovery.kubernetes \"pods\" block lost its node-scope field selector — every replica would discover every pod cluster-wide again (N× log duplication)" >&2; fail=1; }

  # Managed-Postgres attribution guard (w3/m28): only operator-marked tenant
  # Database pods may enter the pipeline, only PostgreSQL's own container is
  # public, and CNPG's immutable cluster id must become the `database` label.
  # Losing any one of these either ingests platform DBs or mixes/unscopes tenant
  # reads, so pin the four River source invariants alongside node scoping.
  echo "==> $LOGSHIP managed-Postgres attribution"
  for required in \
    '__meta_kubernetes_pod_label_app_bex_co_component' \
    'regex         = "database"' \
    '__meta_kubernetes_pod_label_cnpg_io_cluster' \
    'target_label  = "database"' \
    'regex         = "postgres"' \
    'values = { type = "postgres" }'; do
    echo "$vals" | grep -qF "$required" \
      || { echo "FAIL: log-shipper.yaml managed-Postgres pipeline lost required attribution rule: $required" >&2; fail=1; }
  done
fi

# Operator day-to-day RBAC guard (w7/m37, docs/ADR019-infra-credentials.md): the
# scoped ClusterRole must NOT grant write verbs on cluster-administrative resources
# (namespaces, ClusterRoles, ClusterRoleBindings, nodes, or CRDs). Day-to-day ops
# need read + exec + one-shot-job; admin writes are break-glass territory.
DAYTODAY_RBAC="deploy/gitops/base/operator-daytoday-rbac.yaml"
if [ -f "$DAYTODAY_RBAC" ]; then
  echo "==> $DAYTODAY_RBAC does not grant admin writes"
  dangerous_write="$(yq -N \
    '. | select(.kind == "ClusterRole" and .metadata.name == "bex-operator-day-to-day") |
      .rules[] | select(.verbs[] | test("^(create|update|patch|delete|deletecollection)$")) |
      .resources[]?' \
    "$DAYTODAY_RBAC" \
    | grep -E '^(namespaces|clusterroles|clusterrolebindings|nodes|customresourcedefinitions)$' \
    || true)"
  if [ -n "$dangerous_write" ]; then
    echo "FAIL: bex-operator-day-to-day ClusterRole grants write access to admin resources: $dangerous_write" >&2
    fail=1
  fi
fi

# CNPG bootstrap vs tenant egress deny guard (w7/m33): the deny policy must
# carve out CNPG-managed pods (cnpg.io/cluster label) so CNPG init/instance
# pods can reach the Kubernetes API (10.96.0.1:443). The workspace label stays
# on CNPG pods (same-workspace isolation needs it), so the fix is a
# DoesNotExist clause in the deny's matchExpressions, not a label removal.
# Deny-overrides-allow means silently removing this exclusion breaks managed
# Postgres on any fresh tenant node — this guard catches that regression in CI.
EGRESS="deploy/gitops/base/tenant-node-egress.yaml"
if [ -f "$EGRESS" ]; then
  echo "==> $EGRESS CNPG exclusion from node/metadata deny"
  cnpg_excl="$(yq '.spec.endpointSelector.matchExpressions[] | select(.key == "cnpg.io/cluster") | .operator' "$EGRESS")"
  [ "$cnpg_excl" = "DoesNotExist" ] \
    || { echo "FAIL: deny-tenant-node-and-metadata-egress must exclude cnpg.io/cluster pods (DoesNotExist matchExpression) — CNPG init pods need k8s API reachability (w7/m33)" >&2; fail=1; }
fi

# Untrusted-execution boundary (w2/m59, ADR039 O-01/O-02). These assertions are
# intentionally source-level as well as covered by kustomize rendering above:
# they fail if an environment value, selector, or deny rule silently drifts.
echo "==> m59 dedicated untrusted-execution boundary"
BUILD_BOUNDARY="deploy/gitops/base/build-boundary.yaml"
build_ns_shape="$(yq -N 'select(.kind == "Namespace" and .metadata.name == "bex-build") | [.metadata.labels."app.bex.co/execution-boundary", .metadata.labels."pod-security.kubernetes.io/enforce", .metadata.labels."pod-security.kubernetes.io/audit", .metadata.labels."pod-security.kubernetes.io/warn"] | join(":")' "$BUILD_BOUNDARY")"
if [ "$build_ns_shape" != "untrusted:privileged:restricted:restricted" ]; then
  echo "FAIL: bex-build namespace boundary/PSS labels drifted: '$build_ns_shape'" >&2
  fail=1
fi

for manifest in lego/operator/config/manager/manager.yaml lego/operator/config/api/deployment.yaml; do
  build_ns_env="$(yq -N 'select(.kind == "Deployment") | .spec.template.spec.containers[].env[]? | select(.name == "BEX_BUILD_NAMESPACE") | .value' "$manifest")"
  if [ "$build_ns_env" != "bex-build" ]; then
    echo "FAIL: $manifest has BEX_BUILD_NAMESPACE='$build_ns_env', want bex-build" >&2
    fail=1
  fi
done

deny_types="$(yq -N 'select(.kind == "NetworkPolicy" and .metadata.name == "default-deny" and .metadata.namespace == "bex-build") | .spec.policyTypes | sort | join(",")' "$BUILD_BOUNDARY")"
if [ "$deny_types" != "Egress,Ingress" ]; then
  echo "FAIL: bex-build default-deny must select both ingress and egress" >&2
  fail=1
fi
required_ports="$(yq -N 'select(.kind == "NetworkPolicy" and .metadata.name == "allow-required-egress") | [.spec.egress[].ports[]? | .port] | sort | join(",")' "$BUILD_BOUNDARY" | tr -d '\n')"
if [ "$required_ports" != "22,53,53,80,443,5000" ]; then
  echo "FAIL: bex-build required egress ports are '$required_ports', want 22,53,53,80,443,5000" >&2
  fail=1
fi
for blocked_cidr in 10.0.0.0/8 100.64.0.0/10 169.254.0.0/16 172.16.0.0/12 192.168.0.0/16; do
  yq -e 'select(.kind == "NetworkPolicy" and .metadata.name == "allow-required-egress") | .spec.egress[].to[].ipBlock.except[]? == "'"$blocked_cidr"'"' "$BUILD_BOUNDARY" >/dev/null || {
    echo "FAIL: bex-build public egress no longer excludes $blocked_cidr" >&2
    fail=1
  }
done
cilium_denies="$(yq -N 'select(.kind == "CiliumNetworkPolicy" and .metadata.name == "deny-build-node-and-metadata-egress") | [(.spec.egressDeny[].toEntities[]? // ""), (.spec.egressDeny[].toCIDR[]? // "")] | sort | join(",")' "$BUILD_BOUNDARY")"
for required in host remote-node 169.254.0.0/16; do
  grep -q "\(^\|,\)$required\(,\|$\)" <<<"$cilium_denies" || {
    echo "FAIL: bex-build Cilium deny lost '$required': $cilium_denies" >&2
    fail=1
  }
done

echo "==> m59 webhook selects tenant images and fails closed"
WEBHOOK="lego/operator/config/webhook/manifests.yaml"
webhook_shape="$(yq -N 'select(.kind == "ValidatingWebhookConfiguration") | .webhooks[] | select(.name == "vpod.kb.io") | [.failurePolicy, .objectSelector.matchLabels."app.bex.co/verify-image", (.namespaceSelector // "none")] | join(":")' "$WEBHOOK" | tr -d '\n')"
if [ "$webhook_shape" != "Fail:enabled:none" ]; then
  echo "FAIL: tenant-image webhook shape is '$webhook_shape', want Fail:enabled:none" >&2
  fail=1
fi
manager_rollout="$(yq -N 'select(.kind == "Deployment") | [.spec.strategy.type, .spec.strategy.rollingUpdate.maxUnavailable, .spec.strategy.rollingUpdate.maxSurge] | join(":")' lego/operator/config/manager/manager.yaml | tr -d '\n')"
if [ "$manager_rollout" != "RollingUpdate:0:1" ]; then
  echo "FAIL: webhook manager rollout must preserve one Ready endpoint; got '$manager_rollout'" >&2
  fail=1
fi

echo "==> m59 execution placement and production user namespaces"
prewarm_shape="$(yq -N '[.spec.template.spec.nodeSelector."bex.co/pool", .spec.template.spec.containers[0].image] | join(":")' deploy/gitops/base/build-image-prewarm.yaml)"
if [ "$prewarm_shape" != "tenant:moby/buildkit:v0.30.0" ]; then
  echo "FAIL: BuildKit prewarm must target tenant nodes with the supported rootful image; got '$prewarm_shape'" >&2
  fail=1
fi
userns_gates="$(grep -c 'UserNamespacesSupport=true' infra/clusterapi/overlays/hetzner-caph/cluster.yaml || true)"
if [ "$userns_gates" -lt 5 ]; then
  echo "FAIL: production control-plane/worker templates expose only $userns_gates UserNamespacesSupport gates, want at least 5" >&2
  fail=1
fi
if ! grep -q 'export CONTAINERD=2.3.3' infra/clusterapi/overlays/hetzner-caph/cluster.yaml ||
  ! grep -q 'export RUNC=1.5.1' infra/clusterapi/overlays/hetzner-caph/cluster.yaml; then
  echo "FAIL: production worker bootstrap must pin containerd 2.3.3 + runc 1.5.1 for Pod user namespaces" >&2
  fail=1
fi

[ "$fail" -eq 0 ] && echo "PASS: gitops tree renders" || { echo "FAIL: see errors above" >&2; exit 1; }
