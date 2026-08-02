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

echo "==> retired platform source/config paths remain absent"
bash scripts/platform-deprecations-validate.sh || fail=1

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

echo "==> production edge workflow is validation-only after target migration"
if grep -qF '/actions/remove_target' .github/workflows/infra.yml ||
  grep -qF 'remove legacy per-server edge targets' .github/workflows/infra.yml; then
  echo "FAIL: infra workflow must not retain the completed per-server target deletion loop" >&2
  fail=1
fi
for required in \
  'name: verify canonical edge target shape' \
  'select(.type == "server")' \
  '.label_selector.selector == $selector' \
  '.use_private_ip == true' \
  "'22,80,443,5432,6379'" \
  'missing_healthy'; do
  grep -qF "$required" .github/workflows/infra.yml || {
    echo "FAIL: infra workflow lost canonical edge validation: $required" >&2
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

for dir in deploy/opensandbox deploy/gitops/base deploy/gitops/overlays/*/ deploy/gitops/charts/*/; do
  [ -f "$dir/kustomization.yaml" ] || continue # e.g. charts/opensandbox-controller is a Helm chart
  echo "==> kustomize build $dir"
  kubectl kustomize "$dir" >/dev/null || { echo "FAIL: $dir does not render" >&2; fail=1; }
done

# kpack is vendored rather than fetched during reconciliation. Pin both the
# official asset bytes and the rendered compatibility contract: the v1.34 fleet
# must never regain the old KUBERNETES_MIN_VERSION=1.31 workaround, while the
# controller/webhook placement patches must survive an upstream manifest swap.
echo "==> vendored kpack v0.18.0 integrity + compatibility shape"
KPACK_ASSET="deploy/gitops/charts/kpack/upstream/release-0.18.0.yaml"
KPACK_SHA="cde8b7df8d31d6a5758ec4880eec45009f17811baf3df5a29b76a144fe200e69"
actual_kpack_sha="$(sha256sum "$KPACK_ASSET" | awk '{print $1}')"
if [ "$actual_kpack_sha" != "$KPACK_SHA" ]; then
  echo "FAIL: $KPACK_ASSET SHA-256 is $actual_kpack_sha (want $KPACK_SHA)" >&2
  fail=1
fi
kpack_render="$(kubectl kustomize deploy/gitops/charts/kpack)"
if grep -q 'KUBERNETES_MIN_VERSION' <<<"$kpack_render"; then
  echo "FAIL: rendered kpack still carries the retired KUBERNETES_MIN_VERSION override" >&2
  fail=1
fi
for component in kpack-controller kpack-webhook; do
  placement="$(yq -N \
    "select(.kind == \"Deployment\" and .metadata.name == \"$component\") |
      [.spec.template.spec.nodeSelector.\"bex.co/pool\",
       (.spec.template.spec.tolerations | length | tostring),
       .spec.template.spec.tolerations[0].key,
       .spec.template.spec.tolerations[0].value,
       .spec.template.spec.tolerations[0].effect] | join(\":\")" \
    - <<<"$kpack_render" | tr -d '\n')"
  if [ "$placement" != "platform:1:bex.co/platform:true:NoSchedule" ]; then
    echo "FAIL: $component placement is '$placement' (want platform:1:bex.co/platform:true:NoSchedule)" >&2
    fail=1
  fi
done
kpack_sa_namespace="$(yq -N \
  'select(.kind == "ServiceAccount" and .metadata.name == "bex-kpack-builder") | .metadata.namespace' \
  deploy/gitops/charts/kpack/platform.yaml | tr -d '\n')"
if [ "$kpack_sa_namespace" != "bex-system" ] \
  || ! grep -Fq 'KPACK_NS="${BEX_KPACK_NAMESPACE:-bex-system}"' scripts/registry-secrets.sh \
  || ! grep -Fq 'kubectl create secret generic bex-registry-push-kpack -n "$KPACK_NS"' scripts/registry-secrets.sh \
  || ! grep -Fq '$1 ~ /^app-/ && NF == 2' scripts/registry-secrets.sh; then
  echo "FAIL: kpack builder credential must share bex-system with its ServiceAccount and registry rotation must preserve app-* identities" >&2
  fail=1
fi

# Barman Cloud is vendored for the same reproducibility reason as kpack. Pin
# the upstream bytes, controller placement/image, CRD + TLS resources, and the
# three credential-reference-only ObjectStores. These checks deliberately inspect
# only Secret names/key names; no credential data is present or decoded.
echo "==> vendored Barman Cloud Plugin v0.13.0 + ObjectStore contracts"
BARMAN_ASSET="deploy/gitops/charts/barman-cloud-plugin/upstream/manifest-0.13.0.yaml"
BARMAN_SHA="d2e71e7b06822448f1a421f05781846cfdb9cc621e7ef32eef5e20c5133213b0"
actual_barman_sha="$(sha256sum "$BARMAN_ASSET" | awk '{print $1}')"
if [ "$actual_barman_sha" != "$BARMAN_SHA" ]; then
  echo "FAIL: $BARMAN_ASSET SHA-256 is $actual_barman_sha (want $BARMAN_SHA)" >&2
  fail=1
fi
barman_render="$(kubectl kustomize deploy/gitops/charts/barman-cloud-plugin)"
barman_deployment="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "barman-cloud") |
  [.metadata.namespace,
   .spec.template.spec.containers[0].image,
   .spec.template.spec.nodeSelector."bex.co/pool",
   .spec.template.spec.tolerations[0].key,
   .spec.template.spec.tolerations[0].value,
   .spec.template.spec.tolerations[0].effect] | join("|")' \
  - <<<"$barman_render" | tr -d '\n')"
if [ "$barman_deployment" != "cnpg-system|ghcr.io/cloudnative-pg/plugin-barman-cloud:v0.13.0|platform|bex.co/platform|true|NoSchedule" ]; then
  echo "FAIL: Barman plugin deployment contract is '$barman_deployment'" >&2
  fail=1
fi
for resource in \
  'CustomResourceDefinition:objectstores.barmancloud.cnpg.io' \
  'Certificate:barman-cloud-client' \
  'Certificate:barman-cloud-server'; do
  kind="${resource%%:*}"
  name="${resource#*:}"
  if ! yq -e "select(.kind == \"$kind\" and .metadata.name == \"$name\")" \
    - <<<"$barman_render" >/dev/null; then
    echo "FAIL: rendered Barman plugin is missing $kind/$name" >&2
    fail=1
  fi
done

objectstores_render="$(kubectl kustomize deploy/gitops/charts/barman-cloud-objectstores)"
for expected in \
  'auth|auth-dbs|s3://bex-tfstate/auth-dbs|https://s3.eu-central-2.wasabisys.com|7d|auth-dbs-backup-s3|AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY' \
  'bex-system|bex-db|s3://bex-tfstate/bex-db|https://s3.eu-central-2.wasabisys.com|7d|bex-db-backup-s3|AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY' \
  'default|bex-tenant-postgres|s3://bex-tfstate/postgres|https://s3.eu-central-2.wasabisys.com|30d|bex-db-backup-s3|AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY'; do
  namespace="${expected%%|*}"
  rest="${expected#*|}"
  name="${rest%%|*}"
  actual="$(yq -N \
    "select(.kind == \"ObjectStore\" and .metadata.namespace == \"$namespace\" and .metadata.name == \"$name\") |
      [.metadata.namespace,
       .metadata.name,
       .spec.configuration.destinationPath,
       .spec.configuration.endpointURL,
       .spec.retentionPolicy,
       .spec.configuration.s3Credentials.accessKeyId.name,
       .spec.configuration.s3Credentials.accessKeyId.key,
       .spec.configuration.s3Credentials.secretAccessKey.key] | join(\"|\")" \
    - <<<"$objectstores_render" | tr -d '\n')"
  if [ "$actual" != "$expected" ]; then
    echo "FAIL: ObjectStore $namespace/$name contract is '$actual' (want '$expected')" >&2
    fail=1
  fi
done
if yq -e 'select(.kind == "Secret")' - <<<"$objectstores_render" >/dev/null 2>&1; then
  echo "FAIL: ObjectStore manifests must reference out-of-band Secrets, never render a Secret" >&2
  fail=1
fi
plugin_wave="$(yq -N '.metadata.annotations."argocd.argoproj.io/sync-wave" + "|" + .spec.source.path' deploy/gitops/base/barman-cloud-plugin.yaml)"
stores_wave="$(yq -N '.metadata.annotations."argocd.argoproj.io/sync-wave" + "|" + .spec.source.path' deploy/gitops/base/barman-cloud-objectstores.yaml)"
postgres_wave="$(yq -N '.metadata.annotations."argocd.argoproj.io/sync-wave" + "|" + .spec.source.path' deploy/gitops/base/bex-postgres.yaml)"
if [ "$plugin_wave" != "2|deploy/gitops/charts/barman-cloud-plugin" ] \
  || [ "$stores_wave" != "3|deploy/gitops/charts/barman-cloud-objectstores" ] \
  || [ "$postgres_wave" != "4|deploy/gitops/charts/bex-postgres" ]; then
  echo "FAIL: Barman plugin/ObjectStore/bex-postgres Applications lost their ordered GitOps paths" >&2
  fail=1
fi
if kubectl kustomize deploy/gitops/overlays/local | yq -e \
  'select(.kind == "Application" and .metadata.name == "barman-cloud-objectstores")' - >/dev/null 2>&1; then
  echo "FAIL: local overlay must omit credential-backed ObjectStores" >&2
  fail=1
fi

# Every checked-in CNPG Cluster must continuously archive through the supported
# plugin. The guard resolves each namespaced ObjectStore reference and rejects a
# duplicate destination/serverName pair; its synthetic fixtures prove both the
# red and green paths so a future refactor cannot turn the scan into a no-op.
echo "==> every GitOps CNPG Cluster has a unique Barman Cloud archive identity"
bash scripts/cnpg-backup-guard.sh || fail=1
bash scripts/cnpg-backup-guard.sh --self-test || fail=1

# Physical tenant backups and the operator's logical-export/purge jobs must
# address the same transport. The values are non-secret; credential bytes remain
# exclusively in the referenced out-of-band Secret. Local must stay disabled
# because its overlay deliberately omits the credential-backed ObjectStores.
prod_operator_render="$(kubectl kustomize lego/operator/config/prod)"
tenant_backup_env="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") |
  .spec.template.spec.containers[] | select(.name == "manager") |
  [.env[] |
    select(.name == "BEX_DB_BACKUP_DESTINATION" or
           .name == "BEX_DB_BACKUP_ENDPOINT" or
           .name == "BEX_DB_BACKUP_S3_SECRET" or
           .name == "BEX_KV_BACKUP_DESTINATION" or
           .name == "BEX_KV_BACKUP_ENDPOINT" or
           .name == "BEX_KV_BACKUP_S3_SECRET") |
    .name + "=" + .value] | sort | join("|")' \
  - <<<"$prod_operator_render" | tr -d '\n')"
expected_tenant_backup_env='BEX_DB_BACKUP_DESTINATION=s3://bex-tfstate/postgres|BEX_DB_BACKUP_ENDPOINT=https://s3.eu-central-2.wasabisys.com|BEX_DB_BACKUP_S3_SECRET=bex-db-backup-s3|BEX_KV_BACKUP_DESTINATION=s3://bex-tfstate/keyvalue|BEX_KV_BACKUP_ENDPOINT=https://s3.eu-central-2.wasabisys.com|BEX_KV_BACKUP_S3_SECRET=bex-kv-backup-s3'
if [ "$tenant_backup_env" != "$expected_tenant_backup_env" ]; then
  echo "FAIL: prod tenant backup env contract is '$tenant_backup_env' (want '$expected_tenant_backup_env')" >&2
  fail=1
fi
if kubectl kustomize lego/operator/config/default | yq -e '
  select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") |
  .spec.template.spec.containers[] | select(.name == "manager") |
  .env[] | select(.name == "BEX_DB_BACKUP_DESTINATION" or
                  .name == "BEX_DB_BACKUP_ENDPOINT" or
                  .name == "BEX_DB_BACKUP_S3_SECRET" or
                  .name == "BEX_KV_BACKUP_DESTINATION" or
                  .name == "BEX_KV_BACKUP_ENDPOINT" or
                  .name == "BEX_KV_BACKUP_S3_SECRET")' - >/dev/null 2>&1; then
  echo "FAIL: default/local operator config must leave tenant datastore backups disabled" >&2
  fail=1
fi

# w7/m58: universal platform-pod hardening. bex-system is deliberately
# privileged-PSS (egress-meter needs BPF/NET_ADMIN caps + hostNetwork), so there
# is NO namespace PodSecurity backstop — the per-pod securityContext is the only
# control. Assert EVERY platform Deployment/DaemonSet carries the hardened
# baseline: pod-level runAsNonRoot:true + seccompProfile RuntimeDefault, and each
# container allowPrivilegeEscalation:false + readOnlyRootFilesystem:true +
# capabilities.drop [ALL]. Enumerated over the rendered tree (not hand-picked), so
# a newly added component is covered automatically, and fail-closed on a MISSING
# field — exactly the static-server gap this milestone closed. Sole exemption:
# bex-egress-meter (deliberately privileged for post-policy L3 metering, checked
# separately below).
platform_hardening_guard="$(cat "$(dirname "${BASH_SOURCE[0]}")/lib/platform-pod-hardening-guard.yq")"
unhardened_pods="$(yq ea "$platform_hardening_guard" - <<<"$prod_operator_render" | sed '/^$/d' | sort -u | tr '\n' ' ')"
if [ -n "$unhardened_pods" ]; then
  echo "FAIL: platform workload(s) below the securityContext hardening baseline (w7/m58): ${unhardened_pods}" >&2
  echo "      each non-exempt Deployment/DaemonSet needs pod runAsNonRoot:true + seccompProfile RuntimeDefault and" >&2
  echo "      per-container allowPrivilegeEscalation:false + readOnlyRootFilesystem:true + capabilities.drop:[ALL]" >&2
  fail=1
fi

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
  # Production sizing contract: enough disk for concurrent large uploads and
  # explicit compute requests so the registry is not admitted as BestEffort.
  echo "$vals" | yq -e '.pvc.storage == "100Gi"' >/dev/null \
    || { echo "FAIL: zot PVC must be 100Gi" >&2; fail=1; }
  echo "$vals" | yq -e '.resources.requests.cpu == "500m" and .resources.requests.memory == "1Gi"' >/dev/null \
    || { echo "FAIL: zot resource requests must be cpu=500m memory=1Gi" >&2; fail=1; }
  echo "$vals" | yq -e '.resources.limits.cpu == "2" and .resources.limits.memory == "4Gi"' >/dev/null \
    || { echo "FAIL: zot resource limits must be cpu=2 memory=4Gi" >&2; fail=1; }
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
echo "==> platform CNPG WAL scrape stamps namespace + cluster on primaries"
platform_cnpg_scrape="$(yq -N '
  .spec.source.helm.values | from_yaml |
  .serverFiles."prometheus.yml".scrape_configs[] |
  select(.job_name == "cnpg-platform-db") |
  [(.kubernetes_sd_configs[0].namespaces.names | sort | join(",")),
   (.relabel_configs[] | select(.target_label == "namespace") | .source_labels | join(",")),
   (.relabel_configs[] | select(.target_label == "cnpg_io_cluster") | .source_labels | join(",")),
   (.relabel_configs[] | select(.action == "keep" and .regex == "primary") | .source_labels | join(","))] |
  join("|")
' deploy/gitops/base/prometheus.yaml)"
expected_platform_cnpg_scrape='auth,bex-system|__meta_kubernetes_namespace|__meta_kubernetes_pod_label_cnpg_io_cluster|__meta_kubernetes_pod_label_role'
if [ "$platform_cnpg_scrape" != "$expected_platform_cnpg_scrape" ]; then
  echo "FAIL: cnpg-platform-db scrape label contract is '$platform_cnpg_scrape' (want '$expected_platform_cnpg_scrape')" >&2
  fail=1
fi

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

# bex-db plugin backup contract (w1/m56 t006/t008): Cluster WAL archiving and
# ScheduledBackup use the one declared ObjectStore and preserve the 04:00 UTC
# schedule. The exact plugin-only object shape is asserted below.
echo "==> bex-db Barman plugin backup contract"
bex_db_render="$(kubectl kustomize deploy/gitops/charts/bex-postgres)"
bex_db_plugin="$(yq -N '
  select(.kind == "Cluster" and .metadata.name == "bex-db") |
  [.metadata.namespace,
   (.spec.plugins | length | tostring),
   .spec.plugins[0].name,
   (.spec.plugins[0].isWALArchiver | tostring),
   .spec.plugins[0].parameters.barmanObjectName,
   .spec.plugins[0].parameters.serverName,
   ((.spec.backup // {}) | length | tostring)] | join("|")' \
  - <<<"$bex_db_render" | tr -d '\n')"
if [ "$bex_db_plugin" != "bex-system|1|barman-cloud.cloudnative-pg.io|true|bex-db|bex-db|0" ]; then
  echo "FAIL: bex-db Cluster plugin contract is '$bex_db_plugin'" >&2
  fail=1
fi
bex_db_schedule="$(yq -N '
  select(.kind == "ScheduledBackup" and .metadata.name == "bex-db-nightly") |
  [.metadata.namespace,
   .spec.schedule,
   .spec.backupOwnerReference,
   .spec.cluster.name,
   .spec.method,
   .spec.pluginConfiguration.name] | join("|")' \
  - <<<"$bex_db_render" | tr -d '\n')"
if [ "$bex_db_schedule" != "bex-system|0 0 4 * * *|self|bex-db|plugin|barman-cloud.cloudnative-pg.io" ]; then
  echo "FAIL: bex-db ScheduledBackup plugin contract is '$bex_db_schedule'" >&2
  fail=1
fi
local_bex_db_render="$(kubectl kustomize deploy/gitops/charts/bex-postgres-local)"
local_bex_db_shape="$(yq -N '
  select(.kind == "Cluster" and .metadata.name == "bex-db") |
  [((.spec.plugins // []) | length | tostring),
   ((.spec.backup // {}) | length | tostring)] | join("|")' \
  - <<<"$local_bex_db_render" | tr -d '\n')"
local_bex_db_schedules="$(yq -N '
  select(.kind == "ScheduledBackup" and .metadata.name == "bex-db-nightly") |
  .metadata.name' - <<<"$local_bex_db_render" | tr -d '\n')"
local_bex_db_path="$(kubectl kustomize deploy/gitops/overlays/local | yq -N '
  select(.kind == "Application" and .metadata.name == "bex-postgres") |
  .spec.source.path' - | tr -d '\n')"
if [ "$local_bex_db_shape" != "0|0" ] || [ -n "$local_bex_db_schedules" ] \
  || [ "$local_bex_db_path" != "deploy/gitops/charts/bex-postgres-local" ]; then
  echo "FAIL: local bex-db must use the backup-disabled chart overlay; shape='$local_bex_db_shape' schedules='$local_bex_db_schedules' path='$local_bex_db_path'" >&2
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

# Tenant-node egress policy guards. The deny policy must
# carve out CNPG-managed pods (cnpg.io/cluster label) so CNPG init/instance
# pods can reach the Kubernetes API (10.96.0.1:443). The workspace label stays
# on CNPG pods (same-workspace isolation needs it), so the fix is a
# DoesNotExist clause in the deny's matchExpressions, not a label removal.
# Deny-overrides-allow means silently removing this exclusion breaks managed
# Postgres on any fresh tenant node — this guard catches that regression in CI.
#
# The label-independent metadata deny selects every apps-namespace endpoint.
# It must explicitly disable default-deny so non-App platform workloads (CNPG,
# backup-purge Jobs, and the autoscaler) retain their ordinary egress. App Pods
# remain default-denied by their operator-managed Kubernetes NetworkPolicy.
EGRESS="deploy/gitops/base/tenant-node-egress.yaml"
if [ -f "$EGRESS" ]; then
  echo "==> $EGRESS CNPG exclusion from node/metadata deny"
  cnpg_excl="$(yq -N \
    'select(.kind == "CiliumNetworkPolicy" and .metadata.name == "deny-tenant-node-and-metadata-egress") |
      .spec.endpointSelector.matchExpressions[] | select(.key == "cnpg.io/cluster") | .operator' \
    "$EGRESS")"
  [ "$cnpg_excl" = "DoesNotExist" ] \
    || { echo "FAIL: deny-tenant-node-and-metadata-egress must exclude cnpg.io/cluster pods (DoesNotExist matchExpression) — CNPG init pods need k8s API reachability (w7/m33)" >&2; fail=1; }

  echo "==> $EGRESS metadata deny does not enable namespace-wide default-deny"
  metadata_deny_shape="$(yq -N \
    'select(.kind == "CiliumNetworkPolicy" and .metadata.name == "deny-metadata-egress-all-pods") |
      [.metadata.namespace,
       (.spec.enableDefaultDeny.egress | tostring),
       (.spec.egressDeny | length | tostring),
       (.spec.egressDeny[0].toCIDR | join(",")),
       ((.spec.egress // []) | length | tostring)] | join(":")' \
    "$EGRESS" | tr -d '\n')"
  [ "$metadata_deny_shape" = "default:false:1:169.254.0.0/16:0" ] \
    || { echo "FAIL: deny-metadata-egress-all-pods must deny only metadata without enabling default-deny; got '$metadata_deny_shape'" >&2; fail=1; }

  obsolete_platform_allows="$(yq -N \
    'select(.kind == "CiliumNetworkPolicy" and
      (.metadata.name == "allow-cluster-autoscaler-kube-apiserver" or .metadata.name == "allow-cnpg-kube-apiserver")) |
      .metadata.name' "$EGRESS" | tr '\n' ' ')"
  [ -z "$obsolete_platform_allows" ] \
    || { echo "FAIL: obsolete egress allows remain after disabling metadata-policy default-deny: $obsolete_platform_allows" >&2; fail=1; }

  echo "==> sandbox Cilium DNS/FQDN/SNI and lifecycle-server egress boundaries"
  grep -qF -- '--set l7Proxy=true' .github/workflows/app-cluster.yml \
    || { echo "FAIL: sandbox TLS SNI policy requires Cilium l7Proxy=true at cluster bootstrap" >&2; fail=1; }
  sandbox_selector="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-default-deny") |
      .spec.endpointSelector.matchLabels."app.bex.co/regime"' "$EGRESS" | tr -d '\n')"
  [ "$sandbox_selector" = "sandbox" ] \
    || { echo "FAIL: sandbox egress policy must select app.bex.co/regime=sandbox" >&2; fail=1; }

  structural_positive_rules="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-default-deny") |
      ((.spec.egress // []) | length)' "$EGRESS" | tr -d '\n')"
  [ "$structural_positive_rules" = "0" ] \
    || { echo "FAIL: structural sandbox default-deny regained positive egress rules" >&2; fail=1; }

  legacy_selector="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-legacy-allowlist") |
      [.spec.endpointSelector.matchLabels."app.bex.co/regime",
       (.spec.endpointSelector.matchExpressions[] | select(.key == "bex.co/agent-session") | .operator)] | join(":")' \
    "$EGRESS" | tr -d '\n')"
  [ "$legacy_selector" = "sandbox:DoesNotExist" ] \
    || { echo "FAIL: legacy sandbox egress policy does not exclude agent sessions: '$legacy_selector'" >&2; fail=1; }

  expected_sandbox_names=$'api.anthropic.com\napi.github.com\napi.openai.com\nfiles.pythonhosted.org\ngithub.com\nobjects.githubusercontent.com\npypi.org\nraw.githubusercontent.com\nregistry.npmjs.org'
  dns_names="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-legacy-allowlist") |
      .spec.egress[].toPorts[]?.rules.dns[]?.matchName' "$EGRESS" | sort)"
  fqdn_names="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-legacy-allowlist") |
      .spec.egress[].toFQDNs[]?.matchName' "$EGRESS" | sort)"
  sni_names="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-legacy-allowlist") |
      .spec.egress[].toPorts[]?.serverNames[]?' "$EGRESS" | sort)"
  for surface in dns_names fqdn_names sni_names; do
    actual="${!surface}"
    [ "$actual" = "$expected_sandbox_names" ] \
      || { echo "FAIL: sandbox $surface allowlist drifted: $actual" >&2; fail=1; }
  done
  if yq -e \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and
      (.metadata.name == "sandbox-egress-default-deny" or .metadata.name == "sandbox-egress-legacy-allowlist")) |
      .. | select(tag == "!!str" and (test("\\*") or . == "world" or . == "0.0.0.0/0"))' \
    "$EGRESS" >/dev/null 2>&1; then
    echo "FAIL: sandbox egress policy contains a wildcard/world/public-CIDR allow" >&2
    fail=1
  fi
  sandbox_deny="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-default-deny") |
      [(.spec.egressDeny[0].toEntities | sort | join(",")),
       (.spec.egressDeny[1].toCIDR | sort | join(",")),
       (.spec.egressDeny[2].toCIDR | sort | join(",")),
       .spec.egressDeny[2].toPorts[0].ports[0].port,
       .spec.egressDeny[2].toPorts[0].ports[0].protocol] | join(":")' "$EGRESS" | tr -d '\n')"
  [ "$sandbox_deny" = "host,kube-apiserver,remote-node:169.254.0.0/16,fe80::/10:10.0.0.0/8,100.64.0.0/10,172.16.0.0/12,192.168.0.0/16,fc00::/7:443:TCP" ] \
    || { echo "FAIL: sandbox host/node/metadata/private-rebinding deny is '$sandbox_deny'" >&2; fail=1; }

  sandbox_execd_ingress="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-execd-ingress") |
      [.spec.endpointSelector.matchLabels."app.bex.co/regime",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.cilium.k8s.policy.serviceaccount",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:app.kubernetes.io/name",
       .spec.ingress[0].toPorts[0].ports[0].protocol,
       .spec.ingress[0].toPorts[0].ports[0].port] | join(":")' \
    "$EGRESS" | tr -d '\n')"
  [ "$sandbox_execd_ingress" = "sandbox:opensandbox-system:opensandbox-server:opensandbox-server:TCP:44772" ] \
    || { echo "FAIL: sandbox execd ingress identity is '$sandbox_execd_ingress'" >&2; fail=1; }

  server_ingress="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-server-ingress") |
      [.spec.endpointSelector.matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.endpointSelector.matchLabels."k8s:app.kubernetes.io/name",
       (.spec.ingress | length | tostring),
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.cilium.k8s.policy.serviceaccount",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:app.kubernetes.io/name",
       (.spec.ingress[0].toPorts | length | tostring),
       .spec.ingress[0].toPorts[0].ports[0].protocol,
       .spec.ingress[0].toPorts[0].ports[0].port] | join(":")' \
    "$EGRESS" | tr -d '\n')"
  [ "$server_ingress" = "opensandbox-system:opensandbox-server:1:bex-system:bex-api:bex-api:1:TCP:8077" ] \
    || { echo "FAIL: lifecycle-server ingress identity is '$server_ingress'" >&2; fail=1; }

  exec_gateway_ingress="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-exec-gateway-ingress") |
      [.spec.endpointSelector.matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.endpointSelector.matchLabels."k8s:io.cilium.k8s.policy.serviceaccount",
       .spec.endpointSelector.matchLabels."k8s:app.kubernetes.io/name",
       (.spec.ingressDeny | length | tostring),
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.cilium.k8s.policy.serviceaccount",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:app.kubernetes.io/name",
       .spec.ingress[0].toPorts[0].ports[0].protocol,
       .spec.ingress[0].toPorts[0].ports[0].port] | join(":")' \
    "$EGRESS" | tr -d '\n')"
  [ "$exec_gateway_ingress" = "bex-system:bex-ssh-gateway:bex-ssh-gateway:4:bex-system:bex-api:bex-api:TCP:8081" ] \
    || { echo "FAIL: sandbox exec-gateway ingress identity is '$exec_gateway_ingress'" >&2; fail=1; }
  exec_gateway_deny_keys="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-exec-gateway-ingress") |
      .spec.ingressDeny[].fromEndpoints[]?.matchExpressions[]?.key' "$EGRESS" | sort)"
  [ "$exec_gateway_deny_keys" = $'k8s:app.kubernetes.io/name\nk8s:io.cilium.k8s.policy.serviceaccount\nk8s:io.kubernetes.pod.namespace' ] \
    || { echo "FAIL: sandbox exec-gateway deny complement drifted: '$exec_gateway_deny_keys'" >&2; fail=1; }

  server_selector="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-server-egress") |
      [.spec.endpointSelector.matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.endpointSelector.matchLabels."k8s:app.kubernetes.io/name"] | join(":")' "$EGRESS" | tr -d '\n')"
  server_ports="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-server-egress") |
      .spec.egress[].toPorts[]?.ports[]?.port' "$EGRESS" | sort -nu | paste -sd, -)"
  server_entities="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-server-egress") |
      .spec.egress[].toEntities[]?' "$EGRESS" | sort | paste -sd, -)"
  if [ "$server_selector" != "opensandbox-system:opensandbox-server" ] \
    || [ "$server_ports" != "53,443,6443,8091,44772" ] \
    || [ "$server_entities" != "kube-apiserver" ]; then
    echo "FAIL: lifecycle-server egress is selector=$server_selector ports=$server_ports entities=$server_entities" >&2
    fail=1
  fi

  local_cilium="$(kubectl kustomize deploy/gitops/overlays/local | yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and
      (.metadata.name == "sandbox-egress-default-deny" or
       .metadata.name == "sandbox-egress-legacy-allowlist" or
       .metadata.name == "sandbox-execd-ingress" or
       .metadata.name == "opensandbox-server-ingress" or
       .metadata.name == "sandbox-exec-gateway-ingress" or
       .metadata.name == "opensandbox-server-egress" or
       .metadata.name == "opensandbox-controller-egress")) |
      .metadata.name' - | tr '\n' ' ')"
  [ -z "$local_cilium" ] \
    || { echo "FAIL: non-Cilium local overlay retained sandbox policy resources: $local_cilium" >&2; fail=1; }
fi

# OpenSandbox lifecycle-server GitOps shape (w3/m35): gVisor + immutable
# dependencies, no shared API key, least-privilege cluster reads, exact ingress,
# and an out-of-band control-plane bearer rendered into the TOML at pod start.
echo "==> hardened OpenSandbox lifecycle server GitOps shape"
opensandbox_render="$(kubectl kustomize deploy/opensandbox)"
opensandbox_runtime="$(yq -N \
  'select(.kind == "RuntimeClass" and .metadata.name == "gvisor") |
    [.handler, .scheduling.nodeSelector."bex.co/pool",
     .scheduling.tolerations[0].key, .scheduling.tolerations[0].value,
     .scheduling.tolerations[0].effect] | join(":")' - <<<"$opensandbox_render" | tr -d '\n')"
[ "$opensandbox_runtime" = "runsc:sandbox:bex.co/sandbox:true:NoSchedule" ] \
  || { echo "FAIL: gVisor RuntimeClass shape is '$opensandbox_runtime'" >&2; fail=1; }

opensandbox_pod="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-server") |
    [.spec.template.spec.serviceAccountName,
     .spec.template.spec.nodeSelector."bex.co/pool",
     .spec.template.spec.securityContext.runAsNonRoot,
     .spec.template.spec.securityContext.runAsUser,
     .spec.template.spec.securityContext.seccompProfile.type,
     .spec.template.spec.initContainers[0].env[0].valueFrom.secretKeyRef.name,
     .spec.template.spec.initContainers[0].env[0].valueFrom.secretKeyRef.key] | join(":")' \
  - <<<"$opensandbox_render" | tr -d '\n')"
[ "$opensandbox_pod" = "opensandbox-server:platform:true:10001:RuntimeDefault:opensandbox-server-auth:cp_token" ] \
  || { echo "FAIL: lifecycle-server pod hardening is '$opensandbox_pod'" >&2; fail=1; }
opensandbox_config_ref="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-server") |
    .spec.template.spec.volumes[] | select(.name == "config-template") | .configMap.name' \
  - <<<"$opensandbox_render" | tr -d '\n')"
case "$opensandbox_config_ref" in
  opensandbox-config-*) ;;
  *) echo "FAIL: OpenSandbox config has no content hash to trigger rollout: '$opensandbox_config_ref'" >&2; fail=1 ;;
esac
opensandbox_config_mounts="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-server") |
    .spec.template.spec.containers[] | select(.name == "server") |
    .volumeMounts[] | select(.name == "rendered-config" or .name == "config-template") |
    [.name, .mountPath, .subPath, .readOnly] | join(":")' \
  - <<<"$opensandbox_render" | sed '/^[[:space:]]*$/d' | sort)"
expected_opensandbox_config_mounts=$'config-template:/etc/opensandbox/batchsandbox-template.yaml:batchsandbox-template.yaml:true\nrendered-config:/etc/opensandbox/sandbox-cluster.toml:sandbox-cluster.toml:true'
[ "$opensandbox_config_mounts" = "$expected_opensandbox_config_mounts" ] \
  || { echo "FAIL: OpenSandbox server config/template mounts can mask packaged files: '$opensandbox_config_mounts'" >&2; fail=1; }

opensandbox_images="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-server") |
    .spec.template.spec | [.initContainers[].image, .containers[].image] | .[]' \
  - <<<"$opensandbox_render")"
if [ "$(wc -l <<<"$opensandbox_images" | tr -d ' ')" -ne 2 ] \
  || grep -q ':latest' <<<"$opensandbox_images" \
  || grep -Evq '^ghcr.io/bex-co/opensandbox-server@sha256:[a-f0-9]{64}$' <<<"$opensandbox_images"; then
  echo "FAIL: lifecycle-server images must use a CI-written digest or fail-closed bootstrap digest: $opensandbox_images" >&2
  fail=1
fi
if ! grep -qE '^    digest: sha256:[a-f0-9]{64}$' deploy/opensandbox/kustomization.yaml \
  || grep -qE '^    newTag:' deploy/opensandbox/kustomization.yaml; then
  echo "FAIL: OpenSandbox GitOps must never expose a mutable bootstrap tag" >&2
  fail=1
fi

opensandbox_cluster_verbs="$(yq -N \
  'select(.kind == "ClusterRole" and .metadata.name == "opensandbox-server") |
    .rules[].verbs[]' - <<<"$opensandbox_render" | sort -u | paste -sd, -)"
[ "$opensandbox_cluster_verbs" = "get,list,watch" ] \
  || { echo "FAIL: lifecycle server cluster verbs are '$opensandbox_cluster_verbs', want read-only" >&2; fail=1; }
opensandbox_runtimeclass_rbac="$(yq -N \
  'select(.kind == "ClusterRole" and .metadata.name == "opensandbox-server") |
    .rules[] | select(.apiGroups[]? == "node.k8s.io") |
    [(.resources | join(",")), (.resourceNames | join(",")), (.verbs | join(","))] | join(":")' \
  - <<<"$opensandbox_render" | tr -d '\n')"
[ "$opensandbox_runtimeclass_rbac" = "runtimeclasses:gvisor:get" ] \
  || { echo "FAIL: lifecycle server RuntimeClass read is '$opensandbox_runtimeclass_rbac'" >&2; fail=1; }
if yq -e \
  'select(.kind == "ClusterRole" and .metadata.name == "opensandbox-server") |
    .rules[] | select(.resources[]? == "namespaces")' \
  - <<<"$opensandbox_render" >/dev/null 2>&1; then
  echo "FAIL: lifecycle server does not need cluster-wide Namespace reads" >&2
  fail=1
fi
if yq -e \
  'select(.kind == "ClusterRoleBinding" and .roleRef.name == "bex-tenant-sandbox-server")' \
  - <<<"$opensandbox_render" >/dev/null 2>&1; then
  echo "FAIL: mutation-capable tenant sandbox role must never receive a ClusterRoleBinding" >&2
  fail=1
fi

opensandbox_ingress="$(yq -N \
  'select(.kind == "NetworkPolicy" and .metadata.name == "admit-only-bex-api") |
    [.metadata.namespace,
     .spec.podSelector.matchLabels."app.kubernetes.io/name",
     (.spec | has("ingress") | tostring),
     (.spec.ingress | length | tostring)] | join(":")' - <<<"$opensandbox_render" | tr -d '\n')"
[ "$opensandbox_ingress" = "opensandbox-system:opensandbox-server:true:0" ] \
  || { echo "FAIL: lifecycle-server ingress is '$opensandbox_ingress'" >&2; fail=1; }

batchsandbox_shape="$(yq -N \
  '[.metadata.labels."app.bex.co/regime",
    .spec.template.metadata.labels."app.bex.co/regime",
    .spec.template.spec.runtimeClassName,
    .spec.template.spec.automountServiceAccountToken,
    .spec.template.spec.nodeSelector."bex.co/pool",
    .spec.template.spec.tolerations[0].key,
    .spec.template.spec.tolerations[0].value,
    .spec.template.spec.tolerations[0].effect] | join(":")' \
  deploy/opensandbox/batchsandbox-template.yaml | tr -d '\n')"
[ "$batchsandbox_shape" = "sandbox:sandbox:gvisor:false:sandbox:bex.co/sandbox:true:NoSchedule" ] \
  || { echo "FAIL: BatchSandbox security shape is '$batchsandbox_shape'" >&2; fail=1; }

for required in \
  '[secure_runtime]' \
  'type = "gvisor"' \
  'k8s_runtime_class = "gvisor"' \
  'execd_image = "opensandbox/execd:v1.0.16@sha256:af7b55c861926c1304371c4578007fbaa424538219154a6a49a5d636217d2a3a"' \
  'auth_token = "Bearer __BEX_CP_TOKEN__"'; do
  grep -qF "$required" deploy/opensandbox/sandbox-cluster.toml \
    || { echo "FAIL: sandbox-cluster.toml lost: $required" >&2; fail=1; }
done
if grep -Eq '^[[:space:]]*(\[egress\]|OPENSANDBOX_(INSECURE_SERVER|SERVER_API_KEY)|api_key[[:space:]]*=)' \
  deploy/opensandbox/sandbox-cluster.toml deploy/opensandbox/server-in-cluster.yaml; then
  echo "FAIL: OpenSandbox config restored the gVisor-incompatible sidecar or a shared/insecure server credential" >&2
  fail=1
fi
if ! grep -qF 'ARG OPENSANDBOX_SERVER_VERSION=0.2.2' deploy/opensandbox/server.Dockerfile \
  || ! grep -qF 'COPY requirements.lock' deploy/opensandbox/server.Dockerfile \
  || ! grep -qF 'opensandbox-server==${OPENSANDBOX_SERVER_VERSION}' deploy/opensandbox/server.Dockerfile \
  || ! grep -qxF 'opensandbox-server==0.2.2' deploy/opensandbox/requirements.lock \
  || grep -qE '^FROM .*:latest' deploy/opensandbox/server.Dockerfile; then
  echo "FAIL: OpenSandbox server package/base image must stay deliberately pinned" >&2
  fail=1
fi
if grep -Ev '^[[:space:]]*(#.*|$|[A-Za-z0-9_.-]+==[^[:space:]]+)$' deploy/opensandbox/requirements.lock >/dev/null; then
  echo "FAIL: OpenSandbox requirements.lock contains an unpinned dependency" >&2
  fail=1
fi
for legacy_config in deploy/opensandbox/sandbox.toml deploy/opensandbox/sandbox-k8s.toml; do
  for required in \
    'LEGACY LOCAL-DEV ONLY' \
    'execd_image = "opensandbox/execd:v1.0.16@sha256:af7b55c861926c1304371c4578007fbaa424538219154a6a49a5d636217d2a3a"' \
    'image = "opensandbox/egress:v1.0.12@sha256:ddf92e8f303c5715c8bbe8f346af80d7f14efaef6d760add92bf50e558b06c2a"'; do
    grep -qF "$required" "$legacy_config" \
      || { echo "FAIL: $legacy_config lost legacy-path pin/guard: $required" >&2; fail=1; }
  done
done
for legacy_start in scripts/start-opensandbox.sh scripts/start-opensandbox-k8s.sh; do
  grep -qF 'uvx --from opensandbox-server==0.2.2' "$legacy_start" \
    || { echo "FAIL: $legacy_start must pin opensandbox-server" >&2; fail=1; }
done

opensandbox_app="$(yq -N \
  'select(.kind == "Application" and .metadata.name == "opensandbox-server") |
    [.metadata.annotations."argocd.argoproj.io/sync-wave", .spec.source.path] | join(":")' \
  deploy/gitops/base/opensandbox-server.yaml | tr -d '\n')"
[ "$opensandbox_app" = "2:deploy/opensandbox" ] \
  || { echo "FAIL: OpenSandbox GitOps Application is '$opensandbox_app'" >&2; fail=1; }

opensandbox_controller_render="$(helm template opensandbox-controller \
  deploy/gitops/charts/opensandbox-controller -n opensandbox-system \
  -f deploy/gitops/base/values/opensandbox-controller.values.yaml \
  -f deploy/gitops/overlays/prod/values/opensandbox-controller.values.yaml)"
controller_image="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-controller-manager") |
    .spec.template.spec.containers[0].image' - <<<"$opensandbox_controller_render" | tr -d '\n')"
# The bootstrap stays on the known-good upstream digest until CI has produced
# the carried m42 image once. A follow-up activation commit pins that real digest.
[ "$controller_image" = "opensandbox/controller:v0.2.0@sha256:a9a5f73c1785ebd955336ffa313973a35c1a1b662cb7afc4ea82d92021b3532a" ] \
  || { echo "FAIL: OpenSandbox bootstrap controller image is '$controller_image'" >&2; fail=1; }
controller_args="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-controller-manager") |
    .spec.template.spec.containers[0].args[]' - <<<"$opensandbox_controller_render")"
for required in \
  '--image-committer-image=sandbox-registry.cn-zhangjiakou.cr.aliyuncs.com/opensandbox/image-committer:v0.1.0@sha256:d72cce22ff1ea248e86620e945b7cf12615db74c8a8402fcc01dbfa4a09e7442'; do
  grep -qFx -- "$required" <<<"$controller_args" \
    || { echo "FAIL: production OpenSandbox controller lost: $required" >&2; fail=1; }
done
if grep -qFx -- '--snapshot-job-namespace=opensandbox-snapshot' <<<"$controller_args"; then
  echo "FAIL: bootstrap enabled the patched snapshot job namespace before pinning its image" >&2
  fail=1
fi
if grep -Eq '^--snapshot-(registry|registry-insecure|push-secret)|^--resume-pull-secret' <<<"$controller_args"; then
  echo "FAIL: production OpenSandbox controller enabled unavailable snapshot transport/credentials" >&2
  fail=1
fi
controller_cluster_verbs="$(yq -N \
  'select(.kind == "ClusterRole") | .rules[].verbs[]' - <<<"$opensandbox_controller_render" | sort -u | paste -sd, -)"
[ "$controller_cluster_verbs" = "get,list,watch" ] \
  || { echo "FAIL: OpenSandbox controller cluster verbs are '$controller_cluster_verbs', want informer-only" >&2; fail=1; }
controller_system_mutations="$(yq -N \
  'select(.kind == "Role" and .metadata.namespace == "opensandbox-system") |
    .rules[].resources[]' - <<<"$opensandbox_controller_render" | sort -u | paste -sd, -)"
[ "$controller_system_mutations" = "events,leases" ] \
  || { echo "FAIL: controller system-namespace mutation resources are '$controller_system_mutations'" >&2; fail=1; }
if yq -e \
  'select(.kind == "ClusterRole" and
    (.metadata.name == "opensandbox-manager-role" or .metadata.name == "bex-tenant-sandbox-controller")) |
    .rules[] | select(.resources[]? == "secrets")' \
  - <<<"$(printf '%s\n%s' "$opensandbox_controller_render" "$(kubectl kustomize deploy/gitops/base)")" >/dev/null 2>&1; then
  echo "FAIL: OpenSandbox controller has Secret access while snapshot credential flags are disabled" >&2
  fail=1
fi
if yq -e \
  'select(.kind == "ClusterRoleBinding" and
    (.roleRef.name == "bex-tenant-sandbox-controller" or .roleRef.name == "bex-tenant-sandbox-server"))' \
  - <<<"$(printf '%s\n%s' "$opensandbox_controller_render" "$opensandbox_render")" >/dev/null 2>&1; then
  echo "FAIL: an OpenSandbox mutation role received a ClusterRoleBinding" >&2
  fail=1
fi
if yq -e \
  'select(.kind == "RoleBinding" and .metadata.namespace == "opensandbox-system" and
    (.roleRef.name == "bex-tenant-sandbox-controller" or .roleRef.name == "bex-tenant-sandbox-server"))' \
  - <<<"$opensandbox_render" >/dev/null 2>&1; then
  echo "FAIL: OpenSandbox mutation roles must not create Pods in the credential-bearing system namespace" >&2
  fail=1
fi
sandbox_bind_roles="$(yq -N \
  'select(.kind == "ClusterRole" and .metadata.name == "bex-api-namespaces") |
    .rules[] | select(.verbs[]? == "bind") | .resourceNames[]' \
  deploy/gitops/base/bex-api-namespace-rbac.yaml | sort)"
expected_sandbox_bind_roles=$'bex-tenant-api\nbex-tenant-operator\nbex-tenant-sandbox-controller\nbex-tenant-sandbox-server\nbex-tenant-ssh-gateway'
[ "$sandbox_bind_roles" = "$expected_sandbox_bind_roles" ] \
  || { echo "FAIL: namespace reconciler bind allowlist is '$sandbox_bind_roles'" >&2; fail=1; }
namespace_rolebinding_verbs="$(yq -N \
  'select(.kind == "ClusterRole" and .metadata.name == "bex-api-namespaces") |
    .rules[] | select(.resources[]? == "rolebindings") | .verbs[]' \
  deploy/gitops/base/bex-api-namespace-rbac.yaml | sort | paste -sd, -)"
[ "$namespace_rolebinding_verbs" = "create,delete,get,list,update" ] \
  || { echo "FAIL: namespace reconciler RoleBinding verbs are '$namespace_rolebinding_verbs'" >&2; fail=1; }
namespace_object_verbs="$(yq -N \
  'select(.kind == "ClusterRole" and .metadata.name == "bex-api-namespaces") |
    .rules[] | select(.verbs[]? != "bind") |
    [(.resources | sort | join("+")), (.verbs | sort | join(","))] | join(":")' \
  deploy/gitops/base/bex-api-namespace-rbac.yaml | sed '/^$/d' | sort)"
expected_namespace_object_verbs=$'ciliumnetworkpolicies:create,delete,get,update\nlimitranges+resourcequotas:create,get,update\nnamespaces:create,delete,get,list,update\nnetworkpolicies:create,delete,get,list,update\nrolebindings:create,delete,get,list,update'
[ "$namespace_object_verbs" = "$expected_namespace_object_verbs" ] \
  || { echo "FAIL: namespace reconciler object verbs drifted: '$namespace_object_verbs'" >&2; fail=1; }
namespace_admission_render="$(kubectl kustomize deploy/gitops/base | yq \
  'select((.kind == "ValidatingAdmissionPolicy" or .kind == "ValidatingAdmissionPolicyBinding") and
    (.metadata.name == "bex-api-tenant-namespaces" or .metadata.name == "bex-api-tenant-namespace-objects" or
     .metadata.name == "bex-api-session-egress"))')"
namespace_admission_objects="$(yq -N \
  '[.kind, .metadata.name,
    (.metadata.annotations."argocd.argoproj.io/sync-wave" // ""),
    (.spec.failurePolicy // ""),
    ((.spec.validationActions // []) | join(","))] | join(":")' \
  - <<<"$namespace_admission_render" | sort)"
expected_namespace_admission_objects=$'ValidatingAdmissionPolicy:bex-api-session-egress:-3:Fail:\nValidatingAdmissionPolicy:bex-api-tenant-namespace-objects:-3:Fail:\nValidatingAdmissionPolicy:bex-api-tenant-namespaces:-3:Fail:\nValidatingAdmissionPolicyBinding:bex-api-session-egress:-2::Deny,Audit\nValidatingAdmissionPolicyBinding:bex-api-tenant-namespace-objects:-2::Deny,Audit\nValidatingAdmissionPolicyBinding:bex-api-tenant-namespaces:-2::Deny,Audit'
[ "$namespace_admission_objects" = "$expected_namespace_admission_objects" ] \
  || { echo "FAIL: namespace admission policy/binding shape drifted" >&2; fail=1; }
namespace_rbac_waves="$(yq -N \
  'select((.kind == "ClusterRole" or .kind == "ClusterRoleBinding") and .metadata.name == "bex-api-namespaces") |
    [.kind, .metadata.annotations."argocd.argoproj.io/sync-wave"] | join(":")' \
  deploy/gitops/base/bex-api-namespace-rbac.yaml | sort)"
expected_namespace_rbac_waves=$'ClusterRole:-1\nClusterRoleBinding:-1'
[ "$namespace_rbac_waves" = "$expected_namespace_rbac_waves" ] \
  || { echo "FAIL: NamespaceReconciler RBAC must sync only after its admission boundary" >&2; fail=1; }
for required_guard in \
  "request.userInfo.username == 'system:serviceaccount:bex-system:bex-api'" \
  "matches('^tea-[0-9a-v]{20}$')" \
  "variables.target.metadata.name == variables.workspace + '-sandbox'" \
  "metadata.?labels['pod-security.kubernetes.io/enforce'].orValue('') == 'baseline'" \
  "request.resource.resource != 'networkpolicies'" \
  "variables.target.metadata.name == 'default-deny'" \
  "variables.target.metadata.name in ['allow-same-namespace', 'allow-dns-egress', 'allow-opensandbox-server-execd']" \
  "variables.target.metadata.name.matches('^agent-session-egress-[a-f0-9]{16}$')" \
  "annotations['bex.co/model-endpoint']" \
  "!has(rule.toCIDR) && !has(rule.toCIDRSet) && !has(rule.toEntities)" \
  "rule.toPorts[0].ports[0].port == '443'" \
  "rule.toPorts[0].ports[0].port == '8082'" \
  "request.resource.resource != 'rolebindings'" \
  "request.operation == 'DELETE'" \
  "variables.target.metadata.name == variables.target.roleRef.name" \
  "variables.target.roleRef.name == 'bex-tenant-operator'" \
  "variables.target.roleRef.name == 'bex-tenant-sandbox-server'" \
  "variables.target.roleRef.name in ['bex-tenant-sandbox-server', 'bex-tenant-sandbox-controller', 'bex-tenant-ssh-gateway']" \
  "variables.target.subjects[0].name == 'opensandbox-controller-manager'"; do
  grep -qF "$required_guard" deploy/gitops/base/bex-api-namespace-admission.yaml \
    || { echo "FAIL: NamespaceReconciler admission guard lost: $required_guard" >&2; fail=1; }
done
sandbox_pod_admission_render="$(kubectl kustomize deploy/gitops/base | yq \
  'select((.kind == "ValidatingAdmissionPolicy" or .kind == "ValidatingAdmissionPolicyBinding") and
    .metadata.name == "bex-sandbox-pods")')"
sandbox_pod_admission_objects="$(yq -N \
  '[.kind, .metadata.name,
    (.metadata.annotations."argocd.argoproj.io/sync-wave" // ""),
    (.spec.failurePolicy // ""),
    ((.spec.validationActions // []) | join(","))] | join(":")' \
  - <<<"$sandbox_pod_admission_render" | sort)"
expected_sandbox_pod_admission_objects=$'ValidatingAdmissionPolicy:bex-sandbox-pods:-3:Fail:\nValidatingAdmissionPolicyBinding:bex-sandbox-pods:-2::Deny,Audit'
[ "$sandbox_pod_admission_objects" = "$expected_sandbox_pod_admission_objects" ] \
  || { echo "FAIL: sandbox Pod admission policy/binding shape drifted" >&2; fail=1; }
sandbox_pod_admission_rule="$(yq -N \
  'select(.kind == "ValidatingAdmissionPolicy" and .metadata.name == "bex-sandbox-pods") |
    [.spec.matchConstraints.resourceRules[0].resources[0],
     (.spec.matchConstraints.resourceRules[0].operations | sort | join(",")),
     .spec.matchConstraints.namespaceSelector.matchLabels."app.bex.co/regime"] | join(":")' \
  - <<<"$sandbox_pod_admission_render" | tr -d '\n')"
[ "$sandbox_pod_admission_rule" = "pods:CREATE,UPDATE:sandbox" ] \
  || { echo "FAIL: sandbox Pod admission match is '$sandbox_pod_admission_rule'" >&2; fail=1; }
for required_guard in \
  "object.metadata.?labels['app.bex.co/regime'].orValue('') == 'sandbox'" \
  "object.spec.runtimeClassName == 'gvisor'" \
  "object.spec.automountServiceAccountToken == false" \
  "object.spec.nodeSelector['bex.co/pool'] == 'sandbox'" \
  "request.operation != 'CREATE' || !has(object.spec.nodeName)"; do
  grep -qF "$required_guard" deploy/gitops/base/sandbox-pod-admission.yaml \
    || { echo "FAIL: sandbox Pod admission guard lost: $required_guard" >&2; fail=1; }
done
controller_egress_selector="$(yq -N \
  'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-controller-egress") |
    [.spec.endpointSelector.matchLabels."k8s:io.kubernetes.pod.namespace",
     .spec.endpointSelector.matchLabels."k8s:io.cilium.k8s.policy.serviceaccount",
     .spec.endpointSelector.matchLabels."k8s:app.kubernetes.io/name",
     .spec.egress[0].toEntities[0],
     (.spec.egress[0].toPorts[0].ports | map(.port) | sort | join(","))] | join(":")' \
  "$EGRESS" | tr -d '\n')"
[ "$controller_egress_selector" = "opensandbox-system:opensandbox-controller-manager:opensandbox:kube-apiserver:443,6443" ] \
  || { echo "FAIL: OpenSandbox controller egress is '$controller_egress_selector'" >&2; fail=1; }
server_policy_identities="$(yq -N \
  'select(.kind == "CiliumClusterwideNetworkPolicy" and
    (.metadata.name == "opensandbox-server-ingress" or .metadata.name == "opensandbox-server-egress")) |
    [.metadata.name,
     .spec.endpointSelector.matchLabels."k8s:io.cilium.k8s.policy.serviceaccount"] | join(":")' \
  "$EGRESS" | sed '/^$/d' | sort)"
expected_server_policy_identities=$'opensandbox-server-egress:opensandbox-server\nopensandbox-server-ingress:opensandbox-server'
[ "$server_policy_identities" = "$expected_server_policy_identities" ] \
  || { echo "FAIL: lifecycle-server Cilium policy identities drifted: '$server_policy_identities'" >&2; fail=1; }
server_callback_identity="$(yq -N \
  'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-server-egress") |
    .spec.egress[] | select(.toEndpoints[0].matchLabels."k8s:io.kubernetes.pod.namespace" == "bex-system") |
    .toEndpoints[0].matchLabels."k8s:io.cilium.k8s.policy.serviceaccount"' \
  "$EGRESS" | tr -d '\n')"
[ "$server_callback_identity" = "bex-api" ] \
  || { echo "FAIL: lifecycle-server callback lost bex-api ServiceAccount identity" >&2; fail=1; }
server_dns_names="$(yq -N \
  'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-server-egress") |
    .spec.egress[].toPorts[]?.rules.dns[]?.matchName' \
  "$EGRESS" | sed '/^$/d' | sort | paste -sd, -)"
expected_server_dns_names='bex-api.bex-system.svc.cluster.local,bex-api.bex-system.svc.opensandbox-system.svc.cluster.local,bex-api.bex-system.svc.svc.cluster.local'
[ "$server_dns_names" = "$expected_server_dns_names" ] \
  || { echo "FAIL: lifecycle-server DNS allowlist drifted: '$server_dns_names'" >&2; fail=1; }
controller_dns_rules="$(yq -N \
  'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "opensandbox-controller-egress") |
    .spec.egress[].toPorts[]? | select(has("rules")) | .rules.dns[]?.matchName' \
  "$EGRESS" | sed '/^$/d' | wc -l | tr -d ' ')"
[ "$controller_dns_rules" = "0" ] \
  || { echo "FAIL: OpenSandbox controller regained a raw DNS egress rule" >&2; fail=1; }
controller_pod_identity="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-controller-manager") |
    [.metadata.namespace, .spec.template.spec.serviceAccountName,
     .spec.template.metadata.labels."app.kubernetes.io/name"] | join(":")' \
  - <<<"$opensandbox_controller_render" | tr -d '\n')"
[ "$controller_pod_identity" = "opensandbox-system:opensandbox-controller-manager:opensandbox" ] \
  || { echo "FAIL: rendered OpenSandbox controller identity drifted: '$controller_pod_identity'" >&2; fail=1; }
if kubectl kustomize deploy/gitops/overlays/local | yq -e \
  'select(.kind == "Application" and .metadata.name == "opensandbox-server")' - >/dev/null 2>&1; then
  echo "FAIL: local overlay must omit the gVisor/Cilium-only OpenSandbox server" >&2
  fail=1
fi
for required in \
  'build-args: OPENSANDBOX_SERVER_VERSION=0.2.2' \
  '${{ env.OPENSANDBOX_IMAGE }}@${{ steps.build_opensandbox.outputs.digest }}' \
  'group: bex-production-deploy' \
  'bash scripts/deploy-superseded.sh "$GITHUB_SHA"' \
  'refusing stale digest write-back' \
  '[[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]]' \
  'grep -qF "digest: ${OPENSANDBOX_DIGEST}"' \
  'deploy/opensandbox/kustomization.yaml' \
  'git push origin HEAD:main' \
  'bash scripts/opensandbox-server-secret.sh' \
  'wait for OpenSandbox control plane' \
  'BEX_EXPECTED_OPENSANDBOX_IMAGE' \
  'rollout restart' \
  'for deployment in opensandbox-controller-manager opensandbox-server' \
  '.status.availableReplicas'; do
  grep -qF "$required" .github/workflows/deploy.yml \
    || { echo "FAIL: deploy workflow lost OpenSandbox supply-chain/secret step: $required" >&2; fail=1; }
done
if grep -qF 'git reset --soft HEAD~1' .github/workflows/deploy.yml; then
  echo "FAIL: deploy workflow restored the broken digest write-back retry" >&2
  fail=1
fi
bash -n scripts/opensandbox-server-secret.sh \
  || { echo "FAIL: opensandbox-server-secret.sh is not valid shell" >&2; fail=1; }
grep -qF '"path":"/data/api_key"' scripts/opensandbox-server-secret.sh \
  || { echo "FAIL: OpenSandbox Secret convergence no longer removes the legacy shared api_key" >&2; fail=1; }
bash -n scripts/verify-sandbox-isolation.sh \
  || { echo "FAIL: verify-sandbox-isolation.sh is not valid shell" >&2; fail=1; }
for required_probe in \
  'api.github.com' \
  '169.254.169.254' \
  'learned approved public IP with approved SNI' \
  'private Kubernetes API target with approved SNI' \
  'sandbox to Kubernetes API' \
  'same-workspace peer execd' \
  'cross-workspace peer execd' \
  'digest-pinned execd or unexpectedly gained an egress sidecar' \
  'bex-api label spoof on the default ServiceAccount' \
  'bex-api namespace + ServiceAccount + workload identity' \
  'lifecycle-server DNS exfiltration' \
  'controller DNS exfiltration' \
  'NamespaceReconciler admission boundary' \
  'sandbox PSS downgrade' \
  'arbitrary sandbox NetworkPolicy' \
  'sandbox Pod admission allowed non-sandbox node placement' \
  'sandbox Pod admission allowed automatic ServiceAccount-token mounting' \
  'sandbox Pod admission allowed omission of the sandbox regime identity' \
  'sandbox baseline Pod Security allowed a hostPath mount' \
  'admission boundary allowed relabeling kube-system' \
  'SANDBOX_NETWORK_POLICY_UNSUPPORTED' \
  'preflight_cluster' \
  'BEX_PREFLIGHT_ONLY' \
  'pods --subresource=exec' \
  'preflight-only mode made no fixture or API mutation' \
  'hardening resource' \
  'enable-l7-proxy' \
  'unique DNS-exfiltration query' \
  'setup-phase package registry' \
  'agent-phase package registry' \
  'agent-phase tenant allowlisted destination' \
  'agent-phase non-allowlisted destination' \
  'agent-phase same-workspace cross-sandbox isolation' \
  'session egress admission allowed a public CIDR escape hatch' \
  'bex-api-session-egress' \
  'Hubble drops do not contain the unique denied DNS-exfiltration query' \
  'unix:///var/run/cilium/hubble.sock' \
  'hubble observe'; do
  grep -qF "$required_probe" scripts/verify-sandbox-isolation.sh \
    || { echo "FAIL: sandbox isolation verifier lost probe: $required_probe" >&2; fail=1; }
done

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

# Cross-namespace finalization inventories every operator-owned build artifact
# before releasing the App finalizer. Keep the extra list/delete authority
# namespaced to bex-build and pin the exact resource/verb set so it cannot drift
# into a broader Secret, ServiceAccount, or Pod grant.
build_credential_rbac="$(yq -N 'select(.kind == "Role" and .metadata.name == "bex-build-credentials" and .metadata.namespace == "bex-build") | .rules[] | [(.resources | sort | join(",")), (.verbs | sort | join(","))] | join(":")' "$BUILD_BOUNDARY" | sed '/^[[:space:]]*$/d' | sort)"
if [ "$build_credential_rbac" != $'pods:delete,get,list\nsecrets,serviceaccounts:create,delete,get,list,patch,update' ]; then
  echo "FAIL: bex-build-credentials must remain exactly scoped to Pod inventory/delete and Secret/ServiceAccount lifecycle; got '$build_credential_rbac'" >&2
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

# Static-site trust boundaries (w7/m54): Traefik must keep ExternalName support
# for two platform features, so API-server admission — not convention — pins
# every tenant alias to one of those exact operator-owned shapes. The object
# store credentials are deliberately two different names and provider policies.
echo "==> static-site alias admission and split S3 credential contracts"
BASE_RENDER="$(kubectl kustomize deploy/gitops/base)"
# w7/m57: the bex-tenant-api / bex-tenant-operator ClusterRoles grant cluster-wide
# `secrets` write — safe ONLY because they are bound per-tenant-namespace by
# RoleBinding (never a ClusterRoleBinding). A ClusterRoleBinding to either would
# make every workspace's Secrets readable/writable cluster-wide. Guard it the same
# way the sandbox mutation roles are guarded above.
if yq -e \
  'select(.kind == "ClusterRoleBinding" and
    (.roleRef.name == "bex-tenant-api" or .roleRef.name == "bex-tenant-operator"))' \
  - <<<"$BASE_RENDER" >/dev/null 2>&1; then
  echo "FAIL: a secret-bearing tenant role (bex-tenant-api/bex-tenant-operator) received a ClusterRoleBinding — bind it per-namespace only" >&2
  fail=1
fi
alias_admission_shape="$(yq -N '
  select(.kind == "ValidatingAdmissionPolicy" and .metadata.name == "bex-operator-platform-aliases") |
  [.metadata.annotations."argocd.argoproj.io/sync-wave",
   .spec.failurePolicy,
   (.spec.matchConstraints.resourceRules[0].operations | sort | join(",")),
   (.spec.matchConstraints.resourceRules[0].resources | join(",")),
   (.spec.validations | length | tostring)] | join(":")' \
  - <<<"$BASE_RENDER" | tr -d '\n')"
[ "$alias_admission_shape" = "-3:Fail:CREATE,UPDATE:services:4" ] || {
  echo "FAIL: static-site alias admission shape is '$alias_admission_shape'" >&2
  fail=1
}
alias_binding_shape="$(yq -N '
  select(.kind == "ValidatingAdmissionPolicyBinding" and
         (.metadata.name == "bex-operator-platform-aliases" or
          .metadata.name == "bex-operator-platform-aliases-default")) |
  [.metadata.name,
   .metadata.annotations."argocd.argoproj.io/sync-wave",
   .spec.policyName,
   (.spec.validationActions | sort | join(",")),
   (.spec.matchResources.namespaceSelector.matchLabels."app.kubernetes.io/managed-by" // ""),
   (.spec.matchResources.namespaceSelector.matchLabels."app.kubernetes.io/part-of" // ""),
   (.spec.matchResources.namespaceSelector.matchLabels."app.bex.co/regime" // ""),
   ([.spec.matchResources.namespaceSelector.matchExpressions[]? |
      select(.key == "app.bex.co/workspace" and .operator == "Exists")] | length | tostring),
  (.spec.matchResources.namespaceSelector.matchLabels."kubernetes.io/metadata.name" // "")] |
  join(":")' - <<<"$BASE_RENDER" | sed '/^$/d' | sort)"
expected_alias_binding_shape=$'bex-operator-platform-aliases-default:-2:bex-operator-platform-aliases:Audit,Deny::::0:default\nbex-operator-platform-aliases:-2:bex-operator-platform-aliases:Audit,Deny:bex-controlplane:bex:hosting:1:'
[ "$alias_binding_shape" = "$expected_alias_binding_shape" ] || {
  echo "FAIL: static-site alias binding shape is '$alias_binding_shape'" >&2
  fail=1
}
for required_alias_invariant in \
  "system:serviceaccount:bex-system:bex-controller-manager" \
  "bex-static-server.bex-system.svc.cluster.local" \
  "bex-activator.bex-system.svc.cluster.local" \
  "app.bex.co/platform-alias" \
  "app.kubernetes.io/managed-by" \
  "ownerReferences"; do
  grep -qF "$required_alias_invariant" deploy/gitops/base/operator-alias-admission.yaml || {
    echo "FAIL: operator alias admission lost '$required_alias_invariant'" >&2
    fail=1
  }
done

traefik_external_names="$(yq -N '.providers.kubernetesIngress.allowExternalNameServices' \
  deploy/gitops/base/values/traefik.values.yaml | tr -d '\n')"
[ "$traefik_external_names" = "true" ] || {
  echo "FAIL: ExternalName support changed; static and maintenance aliases require it" >&2
  fail=1
}

tenant_alias_writes="$(yq -N '
  select((.kind == "ClusterRole" or .kind == "Role") and
    (.metadata.name == "bex-tenant-api" or .metadata.name == "bex-tenant-ssh-gateway" or
     .metadata.name == "bex-api-apps" or .metadata.name == "bex-ssh-apps")) |
  .rules[] | select(.resources[]? == "services" or .resources[]? == "ingresses") |
  .verbs[] | select(test("^(create|update|patch|delete|deletecollection)$"))' \
  - <<<"$BASE_RENDER" | tr '\n' ' ')"
[ -z "$tenant_alias_writes" ] || {
  echo "FAIL: a tenant-facing ClusterRole gained Service/Ingress mutation: $tenant_alias_writes" >&2
  fail=1
}

publish_secret="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "controller-manager") |
  .spec.template.spec.containers[].env[] |
  select(.name == "BEX_STATIC_PUBLISH_S3_SECRET") | .value' \
  lego/operator/config/manager/manager.yaml | tr -d '\n')"
[ "$publish_secret" = "bex-static-publish-s3" ] || {
  echo "FAIL: manager static publish Secret is '$publish_secret'" >&2
  fail=1
}
publish_compat_secret="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "controller-manager") |
  .spec.template.spec.containers[].env[] |
  select(.name == "BEX_STATIC_S3_SECRET") | .value' \
  lego/operator/config/manager/manager.yaml | tr -d '\n')"
[ "$publish_compat_secret" = "bex-static-publish-s3" ] || {
  echo "FAIL: manager rollout-compatible static publish Secret is '$publish_compat_secret'" >&2
  fail=1
}
read_secret="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "static-server") |
  .spec.template.spec.containers[].envFrom[] | select(has("secretRef")) | .secretRef |
  [.name, ((.optional // false) | tostring)] | join(":")' \
  lego/operator/config/staticserver/deployment.yaml | tr -d '\n')"
[ "$read_secret" = "bex-static-read-s3:false" ] || {
  echo "FAIL: static-server read Secret contract is '$read_secret'" >&2
  fail=1
}
metadata_fallback="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "static-server") |
  .spec.template.spec.containers[].env[] |
  select(.name == "AWS_EC2_METADATA_DISABLED") | .value' \
  lego/operator/config/staticserver/deployment.yaml | tr -d '\n')"
[ "$metadata_fallback" = "true" ] || {
  echo "FAIL: static-server must disable AWS metadata credential fallback" >&2
  fail=1
}
if rg -n 'BEX_STATIC_S3_SECRET|name: static-s3$' \
    lego/operator/cmd lego/operator/internal/publish >/dev/null || \
   rg -n 'name: static-s3$' lego/operator/config >/dev/null; then
  echo "FAIL: legacy shared static S3 Secret contract remains in runtime code/config" >&2
  fail=1
fi

# Static-server GitOps codification (w1/m57, docs/ADR029-static-sites.md): the
# three hand-applied prod fixes are now git-owned, and these render-level
# invariants are the CI twin of the drift that motivated the milestone —
# a singular Argo-managed static-server, config-complete via bex-static-config,
# whose serve origin equals the manager's publish origin, with the origin config
# kept OUT of the local base (else the credential-less CAPD static-server
# CrashLoops instead of serving the intentional degraded-503).
echo "==> static-server is singular + config-complete (prod) and origin-free (local)"
# $tmp/bex-operator-prod.yaml was rendered above (config/prod).
prod_static_count="$(yq -N 'select(.kind == "Deployment" and .metadata.name == "bex-static-server") | .metadata.name' "$tmp/bex-operator-prod.yaml" | grep -c . || true)"
if [ "$prod_static_count" != "1" ]; then
  echo "FAIL: prod render has $prod_static_count bex-static-server Deployments, want exactly 1" >&2
  fail=1
fi
static_envfrom="$(yq -N 'select(.kind == "Deployment" and .metadata.name == "bex-static-server") | .spec.template.spec.containers[].envFrom[] | select(has("configMapRef")) | .configMapRef.name + ":" + ((.configMapRef.optional // false) | tostring)' "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
if [ "$static_envfrom" != "bex-static-config:true" ]; then
  echo "FAIL: static-server envFrom ConfigMap is '$static_envfrom', want bex-static-config:true (optional so local renders without it)" >&2
  fail=1
fi
# Env completeness + serve-origin==publish-origin: the ConfigMap the server reads
# must carry exactly the four origin/base-domain keys AND match the manager env
# that dispatches the publish Job, or the server serves a different bucket/domain
# than the operator publishes to.
static_cfg_kv="$(yq -N 'select(.kind == "ConfigMap" and .metadata.name == "bex-static-config") | .data | to_entries | map(.key + "=" + .value) | sort | join(",")' "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
mgr_origin_kv="$(yq -N 'select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") | .spec.template.spec.containers[] | select(.name == "manager") | [.env[] | select(.name == "BEX_BASE_DOMAIN" or .name == "BEX_STATIC_S3_ENDPOINT" or .name == "BEX_STATIC_S3_BUCKET" or .name == "BEX_STATIC_S3_REGION") | .name + "=" + .value] | sort | join(",")' "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
expected_origin_kv='BEX_BASE_DOMAIN=onbex.co,BEX_STATIC_S3_BUCKET=bex-static,BEX_STATIC_S3_ENDPOINT=https://s3.eu-central-2.wasabisys.com,BEX_STATIC_S3_REGION=eu-central-2'
if [ "$static_cfg_kv" != "$expected_origin_kv" ]; then
  echo "FAIL: bex-static-config keys/values are '$static_cfg_kv', want '$expected_origin_kv'" >&2
  fail=1
fi
if [ "$static_cfg_kv" != "$mgr_origin_kv" ]; then
  echo "FAIL: static serve origin (bex-static-config) != publish origin (manager env): '$static_cfg_kv' vs '$mgr_origin_kv'" >&2
  fail=1
fi
kubectl kustomize lego/operator/config/default >"$tmp/bex-operator-default.yaml"
if yq -e 'select(.kind == "ConfigMap" and .metadata.name == "bex-static-config")' - <"$tmp/bex-operator-default.yaml" >/dev/null 2>&1; then
  echo "FAIL: config/default (local) renders bex-static-config — the CAPD static-server would CrashLoop on a missing Wasabi read credential" >&2
  fail=1
fi

# Static publish pull-secret custody (w1/m57, w9/012 chain #3): the publish Job's
# extract initContainer pulls the built tenant image with bex-registry-pull in
# the BUILD namespace, minted out-of-band by registry-secrets.sh as the
# bex-builder identity — the per-App reg-pull-<name> scheme dropped the shared
# bex-puller from the Zot ACLs, so a bex-puller-authed pull 403s. Pin the
# script's mint target/identity and the operator's default.
echo "==> static publish pull-secret custody (bex-registry-pull minted as bex-builder)"
grep -q 'bex-registry-pull -n "$BUILD_NS"' scripts/registry-secrets.sh \
  && grep -q 'registry_config bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD"' scripts/registry-secrets.sh \
  || { echo "FAIL: registry-secrets.sh must mint bex-registry-pull in the build ns as bex-builder" >&2; fail=1; }
if grep -q 'registry_config bex-puller' scripts/registry-secrets.sh; then
  echo "FAIL: registry-secrets.sh must never mint a credential as the retired bex-puller identity" >&2
  fail=1
fi
grep -q 'RegistryBuildPullSecret = "bex-registry-pull"' lego/operator/cmd/manager/main.go \
  || { echo "FAIL: operator must default BEX_REGISTRY_BUILD_PULL_SECRET to bex-registry-pull" >&2; fail=1; }

read_policy_shape="$(jq -r '
  [.Statement[] | .Action[]? // .Action] | flatten | sort | join(",")' \
  infra/wasabi/static-s3-read-policy.json)"
[ "$read_policy_shape" = "s3:GetBucketLocation,s3:GetObject,s3:ListBucket" ] || {
  echo "FAIL: static reader IAM actions drifted: $read_policy_shape" >&2
  fail=1
}
publish_policy_shape="$(jq -r '
  [.Statement[] | .Action[]? // .Action] | flatten | sort | join(",")' \
  infra/wasabi/static-s3-publish-policy.json)"
expected_publish_actions='s3:AbortMultipartUpload,s3:DeleteObject,s3:GetBucketLocation,s3:GetObject,s3:ListBucket,s3:ListBucketMultipartUploads,s3:ListMultipartUploadParts,s3:PutObject'
[ "$publish_policy_shape" = "$expected_publish_actions" ] || {
  echo "FAIL: static publisher IAM actions drifted: $publish_policy_shape" >&2
  fail=1
}
for policy in infra/wasabi/static-s3-read-policy.json infra/wasabi/static-s3-publish-policy.json; do
  resources="$(jq -r '[.Statement[].Resource] | flatten | sort | join(",")' "$policy")"
  [ "$resources" = "arn:aws:s3:::bex-static,arn:aws:s3:::bex-static/*" ] || {
    echo "FAIL: $policy is not confined to the static bucket: $resources" >&2
    fail=1
  }
done
for required_probe in \
  'reader write static object' \
  'reader list tfstate bucket' \
  'publisher list tfstate bucket' \
  'reader list account buckets' \
  'publisher list account buckets' \
  'reader list unrelated bucket' \
  'publisher list unrelated bucket'; do
  grep -qF "$required_probe" scripts/static-s3-credentials.sh || {
    echo "FAIL: static S3 verifier lost '$required_probe'" >&2
    fail=1
  }
done
for required_live_scope_guard in \
  'expected default plus at least one tenant hosting namespace' \
  'App namespaces outside the admission-protected hosting set' \
  'get serviceaccounts -o json' \
  'get rolebindings -o json' \
  'kubectl auth can-i'; do
  grep -qF "$required_live_scope_guard" scripts/verify-static-site-security.sh || {
    echo "FAIL: static live verifier lost exhaustive scope guard '$required_live_scope_guard'" >&2
    fail=1
  }
done

[ "$fail" -eq 0 ] && echo "PASS: gitops tree renders" || { echo "FAIL: see errors above" >&2; exit 1; }
