#!/usr/bin/env bash
# Validate the GitOps tree renders — pure client-side, no cluster:
#   1. kustomize build of base and every kustomize dir under overlays/ and
#      charts/ (globbed, so new components are covered automatically).
#   2. helm template of checksum-locked charts against the vendored values files
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

echo "==> privileged manifests and charts have immutable source identities"
if grep -En 'repoURL: https://|chart:' deploy/gitops/base/*.yaml; then
  echo "FAIL: base Applications must not resolve live external Helm repositories" >&2
  fail=1
fi
if grep -En 'raw\.githubusercontent\.com/.+/v[0-9].+/(manifests/)?install\.yaml|helm repo add' \
  .github/workflows/deploy.yml .github/workflows/app-cluster.yml; then
  echo "FAIL: production workflows retain a mutable privileged artifact resolver" >&2
  fail=1
fi
for required in \
  'ARGO_INSTALL_COMMIT: e1becb74c728a992804d39c3ceb2e9e6ae58f0ae' \
  'ARGO_INSTALL_SHA256: 752b5a2681f2522fc78ea12ba2d23be44a4523cfa5d9a55cf1907909cc23fc5d' \
  'bash scripts/helm-artifact.sh mirror'; do
  grep -qF "$required" .github/workflows/deploy.yml || {
    echo "FAIL: deploy workflow lost immutable artifact gate: $required" >&2
    fail=1
  }
done
for required in \
  'HELM_REGISTRY_CONFIG="$anonymous_config" helm pull' \
  'del(.annotations["org.opencontainers.image.created"])' \
  'oras manifest push' \
  'public OCI digest' \
  'is not anonymously pullable'; do
  grep -qF "$required" scripts/helm-artifact.sh || {
    echo "FAIL: Helm mirror no longer proves Argo can anonymously pull reviewed packages: $required" >&2
    fail=1
  }
done
for app in deploy/gitops/base/*.yaml; do
  [ "$(yq -r '.kind // ""' "$app")" = Application ] || continue
  if [ "$(yq -r '.spec.project // ""' "$app")" != bex-platform ]; then
    echo "FAIL: $app is outside the restricted bex-platform AppProject" >&2
    fail=1
  fi
  while IFS=$'\t' read -r repo digest path; do
    [ -n "$repo" ] || continue
    chart=${repo##*/}
    locked_digest=$(awk -F '|' -v chart="$chart" '$1 == chart { print $5 }' deploy/helm-artifacts.lock)
    if [ "$repo" != "oci://ghcr.io/bex-co/bex-charts/$chart" ] \
      || [ "$path" != . ] \
      || ! [[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] \
      || [ "$digest" != "$locked_digest" ]; then
      echo "FAIL: $app OCI source does not match the reviewed lock for $chart" >&2
      fail=1
    fi
  done < <(yq -r '
    [(.spec.source // {}), (.spec.sources[]?)][] |
    select(.repoURL | test("^oci://")) |
    [.repoURL, .targetRevision, .path] | @tsv' "$app")
done
for required in \
  'git@github.com:bex-co/bex.git' \
  'oci://ghcr.io/bex-co/bex-charts/*'; do
  grep -qF "$required" deploy/gitops/base/appproject.yaml || {
    echo "FAIL: bex-platform AppProject lost source restriction: $required" >&2
    fail=1
  }
done
for required_namespace in default local opensandbox-snapshot opensandbox-system; do
  if ! REQUIRED_NAMESPACE="$required_namespace" yq -e '
    .spec.destinations[] |
    select(.server == "https://kubernetes.default.svc" and .namespace == strenv(REQUIRED_NAMESPACE))' \
    deploy/gitops/base/appproject.yaml >/dev/null; then
    echo "FAIL: bex-platform AppProject blocks required namespace: $required_namespace" >&2
    fail=1
  fi
done

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
  'BEX_EXPECTED_PROXY_IMAGE: ${{ env.IMAGE }}@${{ needs.build.outputs.operator_digest }}' \
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

echo "==> onbex fallback TLS secret installation safety gates"
bash scripts/onbex-default-tls-secret.test.sh || { echo "FAIL: onbex fallback TLS secret installation safety gates" >&2; fail=1; }

for dir in deploy/opensandbox deploy/gitops/base deploy/gitops/overlays/*/ deploy/gitops/charts/*/; do
  [ -f "$dir/kustomization.yaml" ] || continue # e.g. charts/opensandbox-controller is a Helm chart
  echo "==> kustomize build $dir"
  kubectl kustomize "$dir" >/dev/null || { echo "FAIL: $dir does not render" >&2; fail=1; }
done

# Unknown *.onbex.co SNI must get a public wildcard instead of
# Traefik's generated self-signed certificate, without adding a catch-all route.
# These checks prove the Git-owned contract only; the scheduled synthetic and
# closeout probes prove live trust, expiry, and the intentional 404.
echo "==> production onbex.co wildcard/default TLS contract"
onbex_tls_render="$(kubectl kustomize deploy/gitops/charts/onbex-default-tls)"
tlsstore_shape="$(yq -N '
  select(.kind == "TLSStore" and .metadata.namespace == "traefik" and .metadata.name == "default") |
  [.metadata.annotations."argocd.argoproj.io/sync-wave", .spec.defaultCertificate.secretName] | join("|")
' - <<<"$onbex_tls_render" | tr -d '\n')"
if [ "$tlsstore_shape" != '0|onbex-default-wildcard-tls' ]; then
  echo "FAIL: Traefik default TLSStore contract is '$tlsstore_shape'" >&2
  fail=1
fi
if yq -N 'select(.kind == "Secret" or .kind == "Certificate" or .kind == "Issuer") | .kind' \
  - <<<"$onbex_tls_render" | grep -q .; then
  echo "FAIL: onbex TLS GitOps may render only the TLSStore; certificate material and issuance stay out of band" >&2
  fail=1
fi
if grep -qiF -- '--providers.kubernetescrd.defaulttlsresourcesnamespace=' \
  deploy/gitops/base/values/traefik.values.yaml; then
  echo "FAIL: pinned Traefik v3.7.5 does not support defaultTLSResourcesNamespace" >&2
  fail=1
fi
prod_tls_app="$(yq -N '
  select(.kind == "Application" and .metadata.name == "onbex-default-tls") |
  [.metadata.annotations."argocd.argoproj.io/sync-wave", .spec.project,
   .spec.destination.namespace, .spec.source.path] | join("|")
' deploy/gitops/overlays/prod/onbex-default-tls.yaml | tr -d '\n')"
if [ "$prod_tls_app" != '3|bex-platform|traefik|deploy/gitops/charts/onbex-default-tls' ]; then
  echo "FAIL: production onbex-default-tls Application wiring is '$prod_tls_app'" >&2
  fail=1
fi
if ! grep -qF -- '- onbex-default-tls.yaml' deploy/gitops/overlays/prod/kustomization.yaml; then
  echo "FAIL: production overlay no longer includes onbex-default-tls.yaml" >&2
  fail=1
fi
if ! grep -qF -- 'kubectl apply -k deploy/gitops/charts/onbex-default-tls' .github/workflows/deploy.yml \
  || ! grep -qF -- 'bash scripts/onbex-default-tls-verify.sh notfound.onbex.co' .github/workflows/deploy.yml; then
  echo "FAIL: deploy must bootstrap and publicly verify the Git-owned fallback TLSStore" >&2
  fail=1
fi
if rg -q 'onbex-default-tls' deploy/gitops/base/kustomization.yaml deploy/gitops/overlays/local/kustomization.yaml; then
  echo "FAIL: local overlay must not require production onbex certificate material" >&2
  fail=1
fi
require_onbex_literal() {
  local contract_file="$1" required="$2"
  grep -qF -- "$required" "$contract_file" || {
    echo "FAIL: onbex TLS inventory lost '$required' in $contract_file" >&2
    fail=1
  }
}
require_onbex_literal .env.example 'BEX_ONBEX_TLS_CERT_FILE='
require_onbex_literal .env.example 'BEX_ONBEX_TLS_KEY_FILE='
require_onbex_literal scripts/gh-secrets.sh 'set_file BEX_ONBEX_TLS_CERT "${BEX_ONBEX_TLS_CERT_FILE:?set BEX_ONBEX_TLS_CERT_FILE in .env}" production-deploy'
require_onbex_literal scripts/gh-secrets.sh 'set_file BEX_ONBEX_TLS_KEY "${BEX_ONBEX_TLS_KEY_FILE:?set BEX_ONBEX_TLS_KEY_FILE in .env}" production-deploy'
deploy_tls_shape="$(yq -N '
  .jobs."deploy".steps[] |
  select(.name == "install onbex.co fallback TLS certificate") |
  [.env.BEX_ONBEX_TLS_CERT, .env.BEX_ONBEX_TLS_KEY, .run] | join("|")
' .github/workflows/deploy.yml | tr -d '\n')"
if [ "$deploy_tls_shape" != '${{ secrets.BEX_ONBEX_TLS_CERT }}|${{ secrets.BEX_ONBEX_TLS_KEY }}|bash scripts/onbex-default-tls-secret.sh' ]; then
  echo "FAIL: deploy workflow onbex TLS installation step drifted" >&2
  fail=1
fi
deploy_tls_preflight_shape="$(yq -N '
  .jobs."deploy".steps[] |
  select(.name == "validate onbex.co fallback TLS certificate") |
  [.env.BEX_ONBEX_TLS_CERT, .env.BEX_ONBEX_TLS_KEY, .run] | join("|")
' .github/workflows/deploy.yml | tr -d '\n')"
if [ "$deploy_tls_preflight_shape" != '${{ secrets.BEX_ONBEX_TLS_CERT }}|${{ secrets.BEX_ONBEX_TLS_KEY }}|bash scripts/onbex-default-tls-secret.sh --validate-only' ]; then
  echo "FAIL: deploy workflow onbex TLS preflight step drifted" >&2
  fail=1
fi
liveness_probe="$(yq -N '
  .jobs."public-edge-liveness".steps[] |
  select(.id == "tls_probe") | .run
' .github/workflows/ssh-edge-liveness.yml)"
if ! grep -qF 'bash scripts/onbex-default-tls-verify.sh "$ONBEX_PROBE_HOST"' <<<"$liveness_probe"; then
  echo "FAIL: production edge liveness lost the onbex fallback TLS probe" >&2
  fail=1
fi
if [ "$(yq -N 'select(.kind == "ClusterIssuer") | .spec.acme.solvers[0].http01.ingress.ingressClassName' \
  deploy/gitops/charts/cert-manager-issuers/clusterissuers.yaml | grep -c '^traefik$')" != 2 ]; then
  echo "FAIL: existing staging/prod HTTP-01 ClusterIssuers drifted while adding fallback TLS" >&2
  fail=1
fi
if rg -qi 'cloudflare|dns-?01|BEX_ONBEX_DNS' \
  .env.example scripts/lib/onbex-default-tls.sh \
  scripts/onbex-default-tls-secret.sh scripts/onbex-default-tls-verify.sh \
  deploy/gitops/charts/onbex-default-tls deploy/gitops/overlays/prod/onbex-default-tls.yaml; then
  echo "FAIL: onbex fallback TLS must not depend on a DNS provider" >&2
  fail=1
fi

# w2/m81 (docs/ADR072-security-review-round7.md #5 / ADR061 #11 / ADR063 #9):
# the insecure kubelet-TLS bypass must never reach a production-reachable
# render again, the CA-verification flag must stay present, and the approving
# CSR watcher must stay deployed. Assert against the rendered prod overlay
# (kustomize preserves the Helm `values:` block as literal text in the
# Application CR), not just the base file, so a future overlay patch that
# reintroduces the bypass also fails this check. Local is exempt by design —
# it is the CAPD-only environment the milestone's plan allows to differ.
echo "==> metrics-server verifies kubelet TLS in every production-reachable render"
for prod_render_dir in deploy/gitops/base deploy/gitops/overlays/prod; do
  prod_render="$(kubectl kustomize "$prod_render_dir")"
  if grep -q -- '--kubelet-insecure-tls' <<<"$prod_render"; then
    echo "FAIL: $prod_render_dir renders --kubelet-insecure-tls — kubelet TLS verification must not be bypassed outside the local overlay (w2/m81)" >&2
    fail=1
  fi
  if ! grep -q -- '--kubelet-certificate-authority=' <<<"$prod_render"; then
    echo "FAIL: $prod_render_dir metrics-server is missing --kubelet-certificate-authority (w2/m81)" >&2
    fail=1
  fi
done
if ! yq -e 'select(.kind == "Application" and .metadata.name == "kubelet-csr-approver")' \
  deploy/gitops/base/kubelet-csr-approver.yaml >/dev/null 2>&1; then
  echo "FAIL: deploy/gitops/base/kubelet-csr-approver.yaml no longer defines the kubelet-csr-approver Application" >&2
  fail=1
fi
if ! grep -qF 'kubelet-csr-approver.yaml' deploy/gitops/base/kustomization.yaml; then
  echo "FAIL: kubelet-csr-approver.yaml is no longer registered in deploy/gitops/base/kustomization.yaml" >&2
  fail=1
fi
if ! grep -q 'kind: Application' <(kubectl kustomize deploy/gitops/base) \
  || [ "$(kubectl kustomize deploy/gitops/base | yq -N 'select(.kind == "Application" and .metadata.name == "kubelet-csr-approver") | .metadata.name')" != kubelet-csr-approver ]; then
  echo "FAIL: the rendered base no longer includes the kubelet-csr-approver Application" >&2
  fail=1
fi

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
for kpack_input in \
  paketobuildpacks/builder-jammy-base \
  paketobuildpacks/build-jammy-base \
  paketobuildpacks/run-jammy-base; do
  if ! grep -qE "${kpack_input//\//\\/}(@sha256:)[a-f0-9]{64}" <<<"$kpack_render"; then
    echo "FAIL: rendered kpack input is not digest-pinned: $kpack_input" >&2
    fail=1
  fi
done
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
  || ! grep -Fq 'apply_secret "$KPACK_NS" bex-registry-push-kpack' scripts/registry-secrets.sh \
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
if [ "$barman_deployment" != "cnpg-system|ghcr.io/cloudnative-pg/plugin-barman-cloud:v0.13.0@sha256:71589dbac582333442812b07b31f7ea4d00324a8358aac7ca507dabf9f4b6c96|platform|bex.co/platform|true|NoSchedule" ]; then
  echo "FAIL: Barman plugin deployment contract is '$barman_deployment'" >&2
  fail=1
fi
barman_sidecar_image="$(yq -N '
  select(.kind == "Secret" and .metadata.namespace == "cnpg-system" and
    .metadata.name == "plugin-barman-cloud-m5m67kfh8f") |
  .data.SIDECAR_IMAGE | @base64d' - <<<"$barman_render" | tr -d '\n')"
if [ "$barman_sidecar_image" != "ghcr.io/cloudnative-pg/plugin-barman-cloud-sidecar:v0.13.0@sha256:990361af3319f9e23aafa0f6d7981f99bf1f69b4e6a85cf1bc7d71d6f09bb288" ]; then
  echo "FAIL: Barman sidecar image is not digest-pinned: '$barman_sidecar_image'" >&2
  fail=1
fi
# The Barman controller's Secret grant is load-bearing in a non-obvious way, and
# both directions are wrong (w7/m77, .pm/w7/036.md):
#
#   too much - upstream also grants create/delete on every Secret cluster-wide,
#              which the ObjectStore reconciler never uses.
#   too little - removing the rule outright broke EVERY new managed Postgres on
#              the platform. The controller creates a per-cluster Role granting
#              get/watch/list on that cluster's backup credential, and Kubernetes
#              privilege-escalation prevention requires it to HOLD those verbs to
#              delegate them. It fails silently: Roles created earlier keep
#              working, so only new databases hang in Provisioning.
#
# Pin the exact verb set. NOTE the previous version of this check used jq's
# `index(...)`, which this yq does not implement; the error was swallowed by
# `2>&1` and the non-zero exit read as "no match", so the guard silently passed
# for its whole life. Keep stderr visible and compare a rendered string.
barman_secret_verbs="$(yq ea -N '
  [.. | select(has("kind")) |
   select(.kind == "ClusterRole" and .metadata.name == "plugin-barman-cloud") |
   .rules[] | select(.apiGroups[0] == "" and .resources[0] == "secrets") |
   .verbs | join(",")] | join(";")' - <<<"$barman_render")"
if [ "$barman_secret_verbs" != "get,list,watch" ]; then
  echo "FAIL: Barman controller Secret verbs are '$barman_secret_verbs', want exactly 'get,list,watch'" >&2
  echo "      (empty => new managed Postgres cannot provision anywhere; adding create/delete => over-granted)" >&2
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
default_operator_render="$(kubectl kustomize lego/operator/config/default)"
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
if yq -e '
  select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") |
  .spec.template.spec.containers[] | select(.name == "manager") |
  .env[] | select(.name == "BEX_DB_BACKUP_DESTINATION" or
                  .name == "BEX_DB_BACKUP_ENDPOINT" or
                  .name == "BEX_DB_BACKUP_S3_SECRET" or
                  .name == "BEX_KV_BACKUP_DESTINATION" or
                  .name == "BEX_KV_BACKUP_ENDPOINT" or
                  .name == "BEX_KV_BACKUP_S3_SECRET")' - <<<"$default_operator_render" >/dev/null 2>&1; then
  echo "FAIL: default/local operator config must leave tenant datastore backups disabled" >&2
  fail=1
fi

# w2/m86: persistent-disk encryption at rest is a three-file contract — the
# LUKS StorageClass, the prod operator env that names it, and the operator's
# Secret derivation the class's ${pvc.name}-luks template resolves against.
# Each failure mode is silent at review time and fatal at mount time (a disk
# that never attaches, not a disk that quietly skips encryption), so pin all
# three sides here.
echo "==> persistent-disk LUKS class contract (ADR082 D3, w2/m86)"
DISK_SC="deploy/gitops/base/disk-storageclass.yaml"
disk_sc_name="$(yq '.metadata.name' "$DISK_SC")"
[ "$disk_sc_name" = "hcloud-volumes-luks" ] \
  || { echo "FAIL: $DISK_SC class name is '$disk_sc_name' (want hcloud-volumes-luks)" >&2; fail=1; }
# One tuple pins the rest of the class shape. The template halves MUST stay
# ${pvc.name}-luks / ${pvc.namespace}: the operator mints the Secret as
# DiskPVCName+"-luks" in the App's namespace (appv1alpha1.DiskLUKSSecretName),
# and the kubelet fetches whatever this template resolves to. Any drift
# strands every encrypted disk unmountable; a lost allowVolumeExpansion kills
# the grow-only resize story.
disk_sc_shape="$(yq -N '
  .provisioner + "|" +
  (.allowVolumeExpansion | tostring) + "|" +
  .parameters."csi.storage.k8s.io/node-publish-secret-name" + "|" +
  .parameters."csi.storage.k8s.io/node-publish-secret-namespace"' "$DISK_SC")"
expected_disk_sc_shape='csi.hetzner.cloud|true|${pvc.name}-luks|${pvc.namespace}'
[ "$disk_sc_shape" = "$expected_disk_sc_shape" ] \
  || { echo "FAIL: $DISK_SC LUKS class shape is '$disk_sc_shape' (want '$expected_disk_sc_shape' — provisioner|expansion|secret-name template|secret-namespace template)" >&2; fail=1; }
# Production provisions disks on the encrypted class; the local/default render
# must NOT name it (the class cannot exist without the hcloud CSI driver, and a
# nonexistent class strands every mock disk PVC Pending).
prod_disk_class="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") |
  .spec.template.spec.containers[] | select(.name == "manager") |
  [.env[] | select(.name == "BEX_DISK_STORAGE_CLASS") | .value] | .[0] // ""' \
  - <<<"$prod_operator_render" | tr -d '\n')"
[ "$prod_disk_class" = "$disk_sc_name" ] \
  || { echo "FAIL: prod operator BEX_DISK_STORAGE_CLASS is '$prod_disk_class' (want '$disk_sc_name' — encryption at rest must stay on)" >&2; fail=1; }
if yq -e '
  select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") |
  .spec.template.spec.containers[] | select(.name == "manager") |
  .env[] | select(.name == "BEX_DISK_STORAGE_CLASS")' - <<<"$default_operator_render" >/dev/null 2>&1; then
  echo "FAIL: default/local operator config must not set BEX_DISK_STORAGE_CLASS (the CAPD mock has no hcloud class)" >&2
  fail=1
fi

# w7/026: BEX_BASE_DOMAIN is one conceptual setting read by three separately
# configured Deployments — the operator decides whether a platform-host Ingress
# exists at all, bex-api decides whether the API advertises/pre-reports the URL,
# and the static-server resolves host → site. Nothing else ties them together,
# and divergence fails silently in both directions (an advertised URL nothing
# serves, or a served host the API reports as null). Assert each render keeps
# the three effective values identical; an absent env entry and an explicit ""
# are equivalent (both disable platform hosts — the w7/m54 security posture).
echo "==> BEX_BASE_DOMAIN agrees across operator + bex-api + static-server"
check_base_domain_agreement() {
  local render_name="$1" render="$2" triples
  triples="$(yq -N '
    select(.kind == "Deployment" and
           (.metadata.name == "bex-controller-manager" or
            .metadata.name == "bex-api" or
            .metadata.name == "bex-static-server")) |
    .metadata.name + "=" +
    ([.spec.template.spec.containers[].env[]? | select(.name == "BEX_BASE_DOMAIN") | .value] | .[0] // "")' \
    - <<<"$render")"
  if [ "$(wc -l <<<"$triples" | tr -d ' ')" != 3 ]; then
    echo "FAIL: $render_name render must contain the three BEX_BASE_DOMAIN consumers, got: $(tr '\n' ' ' <<<"$triples")" >&2
    fail=1
    return
  fi
  if [ "$(cut -d= -f2- <<<"$triples" | sort -u | wc -l | tr -d ' ')" != 1 ]; then
    echo "FAIL: BEX_BASE_DOMAIN diverges across the $render_name render: $(tr '\n' ' ' <<<"$triples")" >&2
    fail=1
  fi
}
check_base_domain_agreement "config/prod" "$prod_operator_render"
check_base_domain_agreement "config/default" "$default_operator_render"

# Agreement alone is not enough: three EMPTY values agree perfectly, and that is
# exactly how platform hosting has been deleted twice (e0468cf2 2026-08-08,
# 815e003b 2026-08-16 — the second took production down for hours, since every
# App that reconciled afterwards silently lost its "<slug>.<domain>" Ingress
# while the already-created ones lingered, so nothing looked broken at first).
# Both removals were security passes acting on the onbex.co Public Suffix List
# finding. That gap is an ACCEPTED risk (`.pm/DO_NOT_DO.md` #PSL) — unsetting the
# domain does not narrow it, it just removes the hosting product. So prod must
# additionally carry a NON-EMPTY value.
#
# Deliberately domain-agnostic: this asserts the property "this installation
# declares a platform suffix", never the literal onbex.co. A downstream operator
# renders their own overlay with their own domain and this still holds. Local
# (config/default) may stay empty — no wildcard DNS, no platform hosts.
echo "==> prod render declares a non-empty BEX_BASE_DOMAIN"
prod_base_domain="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") |
  [.spec.template.spec.containers[].env[]? | select(.name == "BEX_BASE_DOMAIN") | .value] | .[0] // ""' \
  - <<<"$prod_operator_render")"
if [ -z "$prod_base_domain" ]; then
  echo "FAIL: prod BEX_BASE_DOMAIN is empty/absent — platform hosting is off, every App loses its <slug>.<domain> address" >&2
  echo "      This is almost certainly an attempt to remediate the onbex.co PSL finding. Don't: it is an accepted" >&2
  echo "      risk (.pm/DO_NOT_DO.md #PSL), and removing the suffix deletes the hosting product instead of securing it." >&2
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
# ADR043 spread tenant Apps into per-workspace <ws> namespaces, so the gateway
# resolves an incoming srv-<id> to its App + namespace with a CLUSTER-SCOPED App
# List (core.AuthorizeApp / appNamespaceByName) — the same pattern and
# justification as bex-api. That read is the gateway's ONLY permitted cluster-
# wide grant: read-only Apps (get/list/watch) and nothing else. No Secrets
# (checked above), no pods, no pods/exec — the sensitive verbs stay namespaced
# via the per-tenant bex-tenant-ssh-gateway binding and must never widen to
# cluster scope.
ssh_cluster_role_rules="$(yq -N '. | select(.kind == "ClusterRole") | .rules[] | [.apiGroups | join(","), .resources | join(","), .verbs | sort | join(",")] | join("|")' "$SSH_RBAC" | sed '/^$/d')"
if [ -n "$ssh_cluster_role_rules" ] && [ "$ssh_cluster_role_rules" != 'app.bex.co|apps|get,list,watch' ]; then
  echo "FAIL: SSH gateway ClusterRole may grant only read-only Apps (get,list,watch); got '$ssh_cluster_role_rules'" >&2
  fail=1
fi
bex_api_ingress_verbs="$(yq -N '. | select(.kind == "Role" and .metadata.name == "bex-api-apps") | .rules[] | select(.apiGroups[] == "networking.k8s.io" and .resources[] == "ingresses") | .verbs | sort | join(",")' deploy/gitops/base/bex-api-apps-rbac.yaml)"
if [ "$bex_api_ingress_verbs" != "get,list" ]; then
  echo "FAIL: bex-api tenant Role needs exactly get,list on Ingresses for exact-router egress accounting; got '$bex_api_ingress_verbs'" >&2
  fail=1
fi
# Guard recovery's exact read-only grants in both the bootstrap/default Role and
# the per-tenant ClusterRole bound into ADR043 workspace namespaces.
for recovery_role_spec in \
  'Role:bex-api-apps:deploy/gitops/base/bex-api-apps-rbac.yaml' \
  'ClusterRole:bex-tenant-api:deploy/gitops/base/tenant-namespace-clusterroles.yaml'; do
  IFS=: read -r recovery_kind recovery_name recovery_file <<<"$recovery_role_spec"
  cnpg_backup_shape="$(yq -N ". | select(.kind == \"$recovery_kind\" and .metadata.name == \"$recovery_name\") | .rules[] | select(.apiGroups[] == \"postgresql.cnpg.io\" and .resources[] == \"backups\") | [(.resources | sort | join(\",\")), (.verbs | sort | join(\",\"))] | join(\"|\")" "$recovery_file" | sed '/^$/d')"
  barman_window_shape="$(yq -N ". | select(.kind == \"$recovery_kind\" and .metadata.name == \"$recovery_name\") | .rules[] | select(.apiGroups[] == \"barmancloud.cnpg.io\" and .resources[] == \"objectstores\") | [(.resources | sort | join(\",\")), (.resourceNames | sort | join(\",\")), (.verbs | sort | join(\",\"))] | join(\"|\")" "$recovery_file" | sed '/^$/d')"
  if [ "$cnpg_backup_shape" != "backups|list" ]; then
    echo "FAIL: $recovery_name needs exactly list on CNPG backups for managed-Postgres recovery; got '$cnpg_backup_shape'" >&2
    fail=1
  fi
  if [ "$barman_window_shape" != "objectstores|bex-tenant-postgres|get" ]; then
    echo "FAIL: $recovery_name needs get on only Barman ObjectStore bex-tenant-postgres; got '$barman_window_shape'" >&2
    fail=1
  fi
done
ssh_cluster_binding="$(kubectl kustomize lego/operator/config/default | yq -N '. | select(.kind == "ClusterRoleBinding") | .subjects[]? | select(.kind == "ServiceAccount" and .name == "bex-ssh-gateway") | .name')"
if [ -n "$ssh_cluster_binding" ]; then
  echo "FAIL: SSH gateway must not have cluster-wide RBAC in its operator deployment overlay (cluster-scoped grants live only in bex-ssh-apps-rbac.yaml, policed above)" >&2
  fail=1
fi
ssh_namespace="$(yq -N '. | select(.kind == "Role") | .metadata.namespace' "$SSH_RBAC")"
ssh_namespaced_rules="$(yq -N '. | select(.kind == "Role") | .rules[] | [.apiGroups | join(","), .resources | join(","), .verbs | sort | join(",")] | join("|")' "$SSH_RBAC")"
if [ "$ssh_namespace" != 'default' ] || [ "$ssh_namespaced_rules" != $'app.bex.co|apps|get,list\n|pods|get,list\n|pods/exec|create' ]; then
  echo "FAIL: SSH gateway tenant Role must remain default-only App/pod get/list + pods/exec create" >&2
  fail=1
fi
# Traefik reaches 2222 (native SSH) + 8080 (Browser Web Shell); monitoring
# scrapes 9090; sandbox-regime namespaces reach only ADR047/ADR062's source-Pod-
# bound Git/model credential listeners on 8082/8084. Enumerate every port per
# rule so a stray added port can't slip past a ports[0]-only check.
ssh_ingress="$(yq -N '.spec.ingress[] | [(.from[0].namespaceSelector.matchLabels."kubernetes.io/metadata.name" // .from[0].namespaceSelector.matchLabels."app.bex.co/regime"), (.ports | map(.port | tostring) | join(","))] | join(":")' lego/operator/config/ssh/networkpolicy.yaml)"
if [ "$ssh_ingress" != $'traefik:2222,8080\nmonitoring:9090\nsandbox:8082,8084' ]; then
  echo "FAIL: SSH gateway ingress must remain Traefik SSH+shell + monitoring metrics + sandbox Git/model credential hops only" >&2
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
  # Chart pinned to the repository-reviewed OCI manifest digest.
  rev="$(yq '.spec.source.targetRevision' "$ZOT")"
  [ "$rev" = "$(awk -F '|' '$1 == "zot" { print $5 }' deploy/helm-artifacts.lock)" ] \
    || { echo "FAIL: zot targetRevision is '$rev' — require the reviewed OCI digest" >&2; fail=1; }
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
# openfga, openbao, traefik, ...): authenticate the locked chart once, then
# render it with the base values alone and with each overlay's values layered on
# top (the same order Argo's valueFiles use — later wins). Globs every overlay
# dir, so a prod-only layer (e.g. openbao's server.ha.replicas: 3 or traefik's
# LoadBalancer) is rendered here, not first in prod. Namespace comes from the
# Application itself so this generalizes across components in different namespaces
# (auth vs secrets vs traefik).
declare -A chart_archives
for chart in kratos hydra openfga openbao traefik; do
  app="deploy/gitops/base/$chart.yaml"
  ns="$(yq '.spec.destination.namespace' "$app")"
  chart_archives[$chart]=$(bash scripts/helm-artifact.sh pull "$chart" "$tmp/$chart") \
    || { echo "FAIL: cannot authenticate locked $chart chart" >&2; fail=1; continue; }
  layerings=("deploy/gitops/base/values/$chart.values.yaml")
  for ov in deploy/gitops/overlays/*/values/$chart.values.yaml; do
    [ -f "$ov" ] && layerings+=("deploy/gitops/base/values/$chart.values.yaml -f $ov")
  done
  for values in "${layerings[@]}"; do
    echo "==> helm template locked $chart -n $ns -f $values"
    # shellcheck disable=SC2086 — $values intentionally splits into -f args
    helm template "$chart" "${chart_archives[$chart]}" -n "$ns" -f $values >/dev/null \
      || { echo "FAIL: $chart values do not render against its locked chart" >&2; fail=1; }
  done
done

# The platform databases can fail over only if the services above them remain
# reachable. Render the production layers and pin every synchronous request-path
# Deployment to two node-separated replicas plus a one-available PDB. This also
# proves the production-only bex-api overlay renders without making the one-node
# local CAPD cluster permanently Progressing.
echo "==> production auth/control-plane request path is drain-safe"
helm template hydra "${chart_archives[hydra]}" -n auth \
  -f deploy/gitops/base/values/hydra.values.yaml \
  -f deploy/gitops/overlays/prod/values/hydra.values.yaml >"$tmp/hydra-prod.yaml"
helm template kratos "${chart_archives[kratos]}" -n auth \
  -f deploy/gitops/base/values/kratos.values.yaml \
  -f deploy/gitops/overlays/prod/values/kratos.values.yaml >"$tmp/kratos-prod.yaml"
helm template openfga "${chart_archives[openfga]}" -n auth \
  -f deploy/gitops/base/values/openfga.values.yaml \
  -f deploy/gitops/overlays/prod/values/openfga.values.yaml >"$tmp/openfga-prod.yaml"
helm template traefik "${chart_archives[traefik]}" -n traefik \
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
  local replicas required preferred min_available
  replicas="$(yq -N "select(.kind == \"Deployment\" and .metadata.name == \"$deployment\") | .spec.replicas" "$manifest" | tr -d '\n')"
  # w2/029: node-spread may be required OR preferred (weight >= 1, hostname
  # topologyKey). Required left rollout surge pods Pending (FailedScheduling)
  # on the 3-node platform pool; hydra/kratos/bex-api moved to preferred,
  # openfga/traefik keep required.
  required="$(yq -N "select(.kind == \"Deployment\" and .metadata.name == \"$deployment\") | .spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution | length" "$manifest" | tr -d '\n')"
  preferred="$(yq -N "select(.kind == \"Deployment\" and .metadata.name == \"$deployment\") | [.spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]? | select(.weight >= 1 and .podAffinityTerm.topologyKey == \"kubernetes.io/hostname\")] | length" "$manifest" | tr -d '\n')"
  min_available="$(yq -N "select(.kind == \"PodDisruptionBudget\" and .metadata.name == \"$pdb\") | .spec.minAvailable" "$pdb_manifest" | tr -d '\n')"
  if ! [[ "$replicas" =~ ^[0-9]+$ ]] || (( replicas < 2 )); then
    echo "FAIL: $label renders replicas '$replicas', want >=2" >&2
    fail=1
  fi
  if { ! [[ "$required" =~ ^[0-9]+$ ]] || (( required < 1 )); } && { ! [[ "$preferred" =~ ^[0-9]+$ ]] || (( preferred < 1 )); }; then
    echo "FAIL: $label has no hostname pod anti-affinity (required or preferred)" >&2
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

# Platform backup supply chain (w1/m66 F16, from the 2026-08-10 security scan).
# These Pods hold the raw etcd/OpenBao snapshot and the S3 upload credentials on
# one shared volume, so every stage that runs before the upload is as sensitive
# as the data itself. Two invariants:
#   1. every image is content-addressed (a moved tag must not change what runs);
#   2. no stage resolves code from a package repository at run time.
echo "==> platform backup CronJobs are digest-pinned and install nothing at runtime"
for backup_chart in deploy/gitops/charts/etcd-backup/cronjob.yaml \
  deploy/gitops/charts/openbao-backup/cronjob.yaml; do
  backup_imgs="$(yq -N '.spec.jobTemplate.spec.template.spec | [(.initContainers // [])[], (.containers // [])[]] | .[] | .image' \
    "$backup_chart")"
  while IFS= read -r img; do
    [ -n "$img" ] || continue
    case "$img" in
      *@sha256:*) ;;
      *)
        echo "FAIL: $backup_chart runs '$img' by tag — pin it by @sha256 digest (w1/m66 F16)" >&2
        fail=1
        ;;
    esac
  done <<< "$backup_imgs"
  # Inspect the executed args only: the charts mention `apk add` in a comment
  # explaining why it was removed, and that must not trip the guard.
  backup_args="$(yq -N '.spec.jobTemplate.spec.template.spec | [(.initContainers // [])[], (.containers // [])[]] | .[] | (.args // [])[]' \
    "$backup_chart")"
  if echo "$backup_args" | grep -qE '(apk[[:space:]]+add|apt-get[[:space:]]+install|yum[[:space:]]+install|pip[[:space:]]+install)'; then
    echo "FAIL: $backup_chart installs packages at run time next to snapshot data + upload credentials (w1/m66 F16)" >&2
    fail=1
  fi
  # The pinned age artifact must be checksum-verified before it executes.
  if echo "$backup_args" | grep -q 'FiloSottile/age/releases' \
    && ! echo "$backup_args" | grep -q 'sha256sum -c'; then
    echo "FAIL: $backup_chart downloads the age binary without verifying it against the pinned SHA-256 (w1/m66 F16)" >&2
    fail=1
  fi
done

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

  # Platform dashboard observability (w4/m88): namespace `dashboard` must ship
  # as type=platform (not a fake App), and Traefik must retain dashboard.bex.co
  # under bounded service=dashboard — never promoting RequestHost to a label.
  echo "==> $LOGSHIP platform dashboard retention (w4/m88)"
  for required in \
    'discovery.relabel "platform_pods"' \
    'regex         = "dashboard"' \
    'type  = "platform"' \
    'eq .host \"dashboard.bex.co\"' \
    'platform_service' \
    'drop_counter_reason = "not_a_tenant_app"'; do
    echo "$vals" | grep -qF "$required" \
      || { echo "FAIL: log-shipper.yaml platform dashboard retention lost required rule: $required" >&2; fail=1; }
  done
  # Cardinality tripwire: RequestHost may be extracted for the allowlist
  # decision, but must never appear as a stage.labels value (Loki stream key).
  if echo "$vals" | grep -A30 'stage.labels {' | grep -qE '^\s*host\s*='; then
    echo "FAIL: log-shipper.yaml must not promote request host to a Loki label (cardinality budget, docs/ADR010)" >&2
    fail=1
  fi
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

  echo "==> CNPG apiserver allow is additive (does not enable default-deny)"
  DSCTL="deploy/gitops/base/datastore-control-egress.yaml"
  cnpg_allow_shape="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "allow-cnpg-apiserver-egress") |
      [(.spec.enableDefaultDeny.egress | tostring),
       (.spec.endpointSelector.matchExpressions[] | select(.key == "cnpg.io/cluster") | .operator),
       ((.spec.egressDeny // []) | length | tostring),
       (.spec.egress[0].toEntities | join(","))] | join(":")' \
    "$DSCTL" | tr -d '\n')"
  [ "$cnpg_allow_shape" = "false:Exists:0:kube-apiserver" ] \
    || { echo "FAIL: allow-cnpg-apiserver-egress must select cnpg.io/cluster Exists and allow ONLY kube-apiserver with enableDefaultDeny.egress=false — a positive egress rule that enables default-deny would strand every un-migrated CNPG cluster in the shared namespace (ADR043 D8.3)" >&2; fail=1; }

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
      (.spec.egress | length)' "$EGRESS" | tr -d '\n')"
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
  expected_sandbox_dns_names=$'api.anthropic.com\napi.github.com\napi.openai.com\nbex-ssh-gateway.bex-system.svc.cluster.local\nfiles.pythonhosted.org\ngithub.com\nobjects.githubusercontent.com\npypi.org\nraw.githubusercontent.com\nregistry.npmjs.org'
  dns_names="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-legacy-allowlist") |
      .spec.egress[].toPorts[]?.rules.dns[]?.matchName' "$EGRESS" | sort)"
  fqdn_names="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-legacy-allowlist") |
      .spec.egress[].toFQDNs[]?.matchName' "$EGRESS" | sort)"
  sni_names="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-egress-legacy-allowlist") |
      .spec.egress[].toPorts[]?.serverNames[]?' "$EGRESS" | sort)"
  [ "$dns_names" = "$expected_sandbox_dns_names" ] \
    || { echo "FAIL: sandbox dns_names allowlist drifted: $dns_names" >&2; fail=1; }
  for surface in fqdn_names sni_names; do
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

  agent_driver_ingress="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "sandbox-agent-driver-ingress") |
      [.spec.endpointSelector.matchLabels."app.bex.co/regime",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:io.cilium.k8s.policy.serviceaccount",
       .spec.ingress[0].fromEndpoints[0].matchLabels."k8s:app.kubernetes.io/name",
       .spec.ingress[0].toPorts[0].ports[0].protocol,
       .spec.ingress[0].toPorts[0].ports[0].port] | join(":")' \
    "$EGRESS" | tr -d '\n')"
  [ "$agent_driver_ingress" = "sandbox:bex-system:bex-ssh-gateway:bex-ssh-gateway:TCP:8787" ] \
    || { echo "FAIL: agent-driver ingress identity is '$agent_driver_ingress'" >&2; fail=1; }

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

  credential_gateway_ingress="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "agent-credential-gateway-ingress") |
      [.spec.endpointSelector.matchLabels."k8s:io.kubernetes.pod.namespace",
       .spec.endpointSelector.matchLabels."k8s:io.cilium.k8s.policy.serviceaccount",
       .spec.endpointSelector.matchLabels."k8s:app.kubernetes.io/name",
       (.spec.ingressDeny | length | tostring),
       .spec.ingress[0].fromEndpoints[0].matchLabels."app.bex.co/regime",
       (.spec.ingress[0].toPorts[0].ports | map(.protocol + ":" + .port) | join(","))] | join(":")' \
    "$EGRESS" | tr -d '\n')"
  [ "$credential_gateway_ingress" = "bex-system:bex-ssh-gateway:bex-ssh-gateway:2:sandbox:TCP:8082,TCP:8084" ] \
    || { echo "FAIL: agent credential-gateway ingress identity is '$credential_gateway_ingress'" >&2; fail=1; }
  credential_gateway_deny_key="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "agent-credential-gateway-ingress") |
      .spec.ingressDeny[].fromEndpoints[]?.matchExpressions[]?.key' "$EGRESS" | tr -d '\n')"
  [ "$credential_gateway_deny_key" = "app.bex.co/regime" ] \
    || { echo "FAIL: agent credential-gateway deny complement drifted: '$credential_gateway_deny_key'" >&2; fail=1; }
  credential_gateway_deny_ports="$(yq -N \
    'select(.kind == "CiliumClusterwideNetworkPolicy" and .metadata.name == "agent-credential-gateway-ingress") |
      .spec.ingressDeny[].toPorts[].ports[].port' "$EGRESS" | sort -u | paste -sd, -)"
  [ "$credential_gateway_deny_ports" = "8082,8084" ] \
    || { echo "FAIL: agent credential-gateway deny ports drifted: '$credential_gateway_deny_ports'" >&2; fail=1; }

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
       .metadata.name == "agent-credential-gateway-ingress" or
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
# The deploy workflow rebuilds and pins this patched controller by digest on
# every rollout, so validate the trusted repository/tag and immutable shape.
if ! grep -Eq '^ghcr\.io/bex-co/opensandbox-controller:v0\.2\.0-bex-snapjobns-terminalpod@sha256:[0-9a-f]{64}$' \
    <<<"$controller_image"; then
  echo "FAIL: OpenSandbox patched controller image is '$controller_image'" >&2
  fail=1
fi
controller_args="$(yq -N \
  'select(.kind == "Deployment" and .metadata.name == "opensandbox-controller-manager") |
    .spec.template.spec.containers[0].args[]' - <<<"$opensandbox_controller_render")"
for required in \
  '--image-committer-image=sandbox-registry.cn-zhangjiakou.cr.aliyuncs.com/opensandbox/image-committer:v0.1.0@sha256:d72cce22ff1ea248e86620e945b7cf12615db74c8a8402fcc01dbfa4a09e7442' \
  '--snapshot-job-namespace=opensandbox-snapshot' \
  '--snapshot-registry=zot.bex-registry.svc:5000/snapshots' \
  '--snapshot-registry-insecure=true' \
  '--snapshot-push-secret=bex-snapshot-push' \
  '--resume-pull-secret=bex-snapshot-pull' \
  '--containerd-socket-path=/run/containerd/containerd.sock'; do
  grep -qFx -- "$required" <<<"$controller_args" \
    || { echo "FAIL: production OpenSandbox controller lost: $required" >&2; fail=1; }
done
# w3/m42 t002: snapshot transport is ENABLED with per-workspace scoping — the
# push credential lives only in the privileged job namespace and each tenant's
# resume-pull Secret is minted per `<ws>-sandbox` by the operator
# (SandboxNamespaceRegistryReconciler). The resume secret name must stay in
# lockstep with registry.SnapshotPullSecretName.
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
expected_sandbox_bind_roles=$'bex-operator-snapshot-pull\nbex-tenant-api\nbex-tenant-operator\nbex-tenant-sandbox-controller\nbex-tenant-sandbox-server\nbex-tenant-ssh-gateway'
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
  "rule.toPorts[0].ports[0].port == '8084'" \
  "variables.model in ['api.anthropic.com', 'api.openai.com', 'generativelanguage.googleapis.com']" \
  "request.resource.resource != 'rolebindings'" \
  "request.operation == 'DELETE'" \
  "variables.target.metadata.name == variables.target.roleRef.name" \
  "variables.target.roleRef.name == 'bex-tenant-operator'" \
  "variables.target.roleRef.name == 'bex-tenant-sandbox-server'" \
  "variables.target.roleRef.name in ['bex-tenant-sandbox-server', 'bex-tenant-sandbox-controller', 'bex-tenant-ssh-gateway', 'bex-operator-snapshot-pull']" \
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
  '${{ env.OPENSANDBOX_CONTROLLER_IMAGE }}@${{ steps.build_opensandbox_controller.outputs.digest }}' \
  'file: lego/agent-image/Dockerfile' \
  '${{ env.AGENT_SANDBOX_IMAGE }}@${{ steps.build_agent_sandbox.outputs.digest }}' \
  'AGENT_DIGEST: ${{ needs.build.outputs.agent_sandbox_digest }}' \
  'grep -qF "value: ${AGENT_SANDBOX_IMAGE}@${AGENT_DIGEST}"' \
  'group: bex-production-deploy' \
  'bash scripts/deploy-superseded.sh "$GITHUB_SHA"' \
  'refusing stale digest write-back' \
  '[[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]]' \
  'grep -qF "digest: ${OPENSANDBOX_DIGEST}"' \
  'grep -qF "tag: ${OPENSANDBOX_CONTROLLER_TAG}@${CONTROLLER_DIGEST}"' \
  'deploy/opensandbox/kustomization.yaml' \
  'git push origin HEAD:main' \
  'bash scripts/opensandbox-server-secret.sh' \
  'wait for OpenSandbox control plane' \
  'BEX_EXPECTED_OPENSANDBOX_IMAGE' \
  'OPENSANDBOX_CONTROLLER_TAG: v0.2.0-bex-snapjobns-terminalpod' \
  'BEX_EXPECTED_OPENSANDBOX_CONTROLLER_IMAGE: ${{ env.OPENSANDBOX_CONTROLLER_IMAGE }}:${{ env.OPENSANDBOX_CONTROLLER_TAG }}@${{ needs.build.outputs.opensandbox_controller_digest }}' \
  'rollout restart' \
  'for deployment in opensandbox-controller-manager opensandbox-server' \
  '.status.availableReplicas'; do
  grep -qF "$required" .github/workflows/deploy.yml \
    || { echo "FAIL: deploy workflow lost OpenSandbox supply-chain/secret step: $required" >&2; fail=1; }
done
controller_digest_pattern='r"(?m)^(\s+tag: [^@\s]+@sha256:)[0-9a-f]{64}$"'
for path in .github/workflows/deploy.yml scripts/deploy-superseded.sh; do
  grep -qF "$controller_digest_pattern" "$path" \
    || { echo "FAIL: $path hard-codes or omits the generated controller digest matcher" >&2; fail=1; }
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
for required_agent_probe in \
  'BEX_VERIFY_AGENT_DRIVER' \
  'BEX_VERIFY_AGENT_MODEL' \
  '\"template\":\"agent\"' \
  'peer sandbox to agent driver' \
  'gateway label spoof on the default ServiceAccount to agent driver' \
  'gateway workload identity to agent driver health endpoint' \
  'gateway workload identity to agent driver UI-message SSE endpoint' \
  'gateway workload identity cannot access a raw ACP launch route' \
  'agent: real model proof' \
  'agent driver exposes no raw ACP launcher'; do
  grep -qF "$required_agent_probe" scripts/verify-sandbox-isolation.sh \
    || { echo "FAIL: sandbox isolation verifier lost m37 probe: $required_agent_probe" >&2; fail=1; }
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

# w1/m68 F3: bex-api is deployed behind Traefik, so it MUST know which peers may
# assert a client IP. Unset, every IP-keyed budget (above all the m67 pre-auth
# failure limiter) keys on the Traefik pod IP, giving the whole internet one
# shared bucket — an anonymous flood then 429s every uncached login. The value
# must equal the cluster's pod CIDR; an empty or drifted value is the bug.
cluster_pod_cidr="$(yq -N 'select(.kind == "Cluster") | .spec.clusterNetwork.pods.cidrBlocks[]' infra/clusterapi/overlays/hetzner-caph/cluster.yaml | head -1)"
api_proxy_cidrs="$(yq -N 'select(.kind == "Deployment") | .spec.template.spec.containers[].env[]? | select(.name == "BEX_TRUSTED_PROXY_CIDRS") | .value' lego/operator/config/api/deployment.yaml)"
if [ -z "$api_proxy_cidrs" ]; then
  echo "FAIL: bex-api deployment must set BEX_TRUSTED_PROXY_CIDRS — behind Traefik an empty set collapses every client onto one shared rate-limit bucket (w1/m68 F3)" >&2
  fail=1
elif [ "$api_proxy_cidrs" != "$cluster_pod_cidr" ]; then
  echo "FAIL: bex-api BEX_TRUSTED_PROXY_CIDRS='$api_proxy_cidrs' but the cluster pod CIDR is '$cluster_pod_cidr' — a wrong trust set is worse than none (it lets the named range spoof X-Forwarded-For)" >&2
  fail=1
fi

# w6/m132: the ssh IngressRouteTCP and the gateway Deployment are two halves of
# one contract. If Traefik wraps the connection in a PROXY header (proxyProtocol
# set on the route) but the gateway is not told to strip it
# (BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS unset), the binary header is fed into the
# SSH version exchange and the handshake never sends KEXINIT — a silent, total
# loss of `ssh <svc>@ssh.bex.co`. w4/m82 shipped exactly that half-done change
# (sender on, receiver off), unnoticed for weeks. Assert the two stay in lockstep.
ssh_proxy_version="$(yq -N 'select(.kind == "IngressRouteTCP" and .metadata.name == "ssh-gateway") | .spec.routes[].services[].proxyProtocol.version' lego/operator/config/ssh/ingressroutetcp.yaml | head -1)"
ssh_proxy_cidrs="$(yq -N 'select(.kind == "Deployment") | .spec.template.spec.containers[].env[]? | select(.name == "BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS") | .value' lego/operator/config/ssh/deployment.yaml)"
if [ -n "$ssh_proxy_version" ] && [ "$ssh_proxy_version" != "null" ]; then
  if [ -z "$ssh_proxy_cidrs" ]; then
    echo "FAIL: ssh-gateway IngressRouteTCP sends PROXY protocol v$ssh_proxy_version but the gateway Deployment does not set BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS — the un-stripped header breaks the SSH handshake before KEXINIT (w6/m132)" >&2
    fail=1
  elif [ "$ssh_proxy_cidrs" != "$cluster_pod_cidr" ]; then
    echo "FAIL: ssh-gateway BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS='$ssh_proxy_cidrs' but Traefik forwards from the cluster pod CIDR '$cluster_pod_cidr' — a wrong trust set means the real client PROXY header is refused as untrusted (w6/m132)" >&2
    fail=1
  fi
fi

# w2/m90: the Web Shell Ingress and the shared bex-shell-ticket Secret wiring are
# one activation contract. The route alone is not enough (m55 shipped it; the
# edge stayed dark until the Secret existed). Keep the public host/path, the
# gateway listener addr, and both Deployments' optional Secret refs in lockstep
# so a future manifest edit cannot silently strand wss://ssh.bex.co/shell again.
shell_ingress_host="$(yq -N 'select(.kind == "Ingress" and .metadata.name == "ssh-shell") | .spec.rules[0].host' lego/operator/config/ssh/ingress-shell.yaml)"
shell_ingress_path="$(yq -N 'select(.kind == "Ingress" and .metadata.name == "ssh-shell") | .spec.rules[0].http.paths[0].path' lego/operator/config/ssh/ingress-shell.yaml)"
shell_ingress_port="$(yq -N 'select(.kind == "Ingress" and .metadata.name == "ssh-shell") | .spec.rules[0].http.paths[0].backend.service.port.number' lego/operator/config/ssh/ingress-shell.yaml)"
shell_ws_addr="$(yq -N 'select(.kind == "Deployment") | .spec.template.spec.containers[].env[]? | select(.name == "BEX_SHELL_WS_ADDR") | .value' lego/operator/config/ssh/deployment.yaml)"
shell_gw_secret="$(yq -N 'select(.kind == "Deployment") | .spec.template.spec.containers[].env[]? | select(.name == "BEX_SHELL_TICKET_SECRET") | .valueFrom.secretKeyRef.name' lego/operator/config/ssh/deployment.yaml)"
shell_api_secret="$(yq -N 'select(.kind == "Deployment") | .spec.template.spec.containers[].env[]? | select(.name == "BEX_SHELL_TICKET_SECRET") | .valueFrom.secretKeyRef.name' lego/operator/config/api/deployment.yaml)"
shell_api_ws_url="$(yq -N 'select(.kind == "Deployment") | .spec.template.spec.containers[].env[]? | select(.name == "BEX_SHELL_WS_URL") | .value' lego/operator/config/api/deployment.yaml)"
if [ "$shell_ingress_host" != "ssh.bex.co" ] || [ "$shell_ingress_path" != "/shell" ] || [ "$shell_ingress_port" != "8080" ]; then
  echo "FAIL: ssh-shell Ingress is host='$shell_ingress_host' path='$shell_ingress_path' port='$shell_ingress_port', want ssh.bex.co /shell → 8080 (w2/m90)" >&2
  fail=1
fi
if [ "$shell_ws_addr" != ":8080" ]; then
  echo "FAIL: gateway BEX_SHELL_WS_ADDR='$shell_ws_addr', want :8080 to match the ssh-shell Ingress backend (w2/m90)" >&2
  fail=1
fi
if [ "$shell_gw_secret" != "bex-shell-ticket" ] || [ "$shell_api_secret" != "bex-shell-ticket" ]; then
  echo "FAIL: Web Shell ticket Secret refs are gateway='$shell_gw_secret' api='$shell_api_secret', want both bex-shell-ticket (w2/m90)" >&2
  fail=1
fi
if [ "$shell_api_ws_url" != "wss://ssh.bex.co/shell" ]; then
  echo "FAIL: bex-api BEX_SHELL_WS_URL='$shell_api_ws_url', want wss://ssh.bex.co/shell (w2/m90)" >&2
  fail=1
fi

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
# Placement only. The prewarm image is checked by scripts/clusterapi-validate.sh
# against build.go's defaultBuildkitImage, which is a stronger check than the
# literal that used to sit here — that literal was a third copy of the digest and
# had to be bumped by hand on every BuildKit upgrade.
prewarm_pool="$(yq -N '.spec.template.spec.nodeSelector."bex.co/pool"' deploy/gitops/base/build-image-prewarm.yaml)"
if [ "$prewarm_pool" != "tenant" ]; then
  echo "FAIL: BuildKit prewarm must target tenant nodes; got pool '$prewarm_pool'" >&2
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
# The operator's cluster-wide controller verbs are safe only while admission
# confines its exact ServiceAccount to tenant/build namespace classes and a
# no-token/no-host/no-privileged Pod grammar.
operator_workload_admission_objects="$(yq -N '
  select((.kind == "ValidatingAdmissionPolicy" or .kind == "ValidatingAdmissionPolicyBinding") and
    (.metadata.name == "bex-operator-workloads" or
     .metadata.name == "bex-operator-service-namespaces" or
     .metadata.name == "bex-operator-object-namespaces" or
     .metadata.name == "bex-operator-daytoday-workloads" or
     .metadata.name == "bex-build-job-shape")) |
  [.kind, .metadata.name,
   .metadata.annotations."argocd.argoproj.io/sync-wave",
   (.spec.failurePolicy // ""),
   ((.spec.validationActions // []) | sort | join(","))] | join(":")' \
  - <<<"$BASE_RENDER" | sed '/^[[:space:]]*$/d' | sort)"
expected_operator_workload_admission_objects=$'ValidatingAdmissionPolicy:bex-build-job-shape:-3:Fail:\nValidatingAdmissionPolicy:bex-operator-daytoday-workloads:-3:Fail:\nValidatingAdmissionPolicy:bex-operator-object-namespaces:-3:Fail:\nValidatingAdmissionPolicy:bex-operator-service-namespaces:-3:Fail:\nValidatingAdmissionPolicy:bex-operator-workloads:-3:Fail:\nValidatingAdmissionPolicyBinding:bex-build-job-shape:-2::Audit,Deny\nValidatingAdmissionPolicyBinding:bex-operator-daytoday-workloads:-2::Audit,Deny\nValidatingAdmissionPolicyBinding:bex-operator-object-namespaces:-2::Audit,Deny\nValidatingAdmissionPolicyBinding:bex-operator-service-namespaces:-2::Audit,Deny\nValidatingAdmissionPolicyBinding:bex-operator-workloads:-2::Audit,Deny'
[ "$operator_workload_admission_objects" = "$expected_operator_workload_admission_objects" ] || {
  echo "FAIL: operator workload admission policy/binding shape drifted" >&2
  fail=1
}
for required_operator_workload_guard in \
  "system:serviceaccount:bex-system:bex-controller-manager" \
  "system:serviceaccount:kube-system:bex-operator" \
  "kubernetes.io/metadata.name: bex-build" \
  "app.bex.co/execution-boundary" \
  "app.bex.co/regime'].orValue('') == 'hosting'" \
  "automountServiceAccountToken == false" \
  "request.operation == 'DELETE' ? oldObject : object" \
  'resources: ["persistentvolumeclaims"]' \
  'resources: ["images", "builds"]' \
  "!has(v.hostPath)" \
  "!c.?securityContext.?privileged.orValue(false)" \
  "bex-kubeconfig" \
  "bex-ca" \
  "variables.target.metadata.name.startsWith('dskbak-')" \
  "['DAC_OVERRIDE', 'CHOWN', 'FOWNER']"; do
  grep -qF "$required_operator_workload_guard" deploy/gitops/base/operator-workload-admission.yaml || {
    echo "FAIL: operator workload admission lost '$required_operator_workload_guard'" >&2
    fail=1
  }
done
# DELETE is namespace-confined above, but must not re-validate the old Pod
# against today's CREATE/UPDATE grammar: doing so could make a legacy workload
# impossible for the operator to clean up. Each of the six shape validations
# explicitly short-circuits on DELETE.
operator_delete_shape_bypasses="$(grep -Fc "request.operation == 'DELETE' ||" deploy/gitops/base/operator-workload-admission.yaml || true)"
if [ "$operator_delete_shape_bypasses" -ne 6 ]; then
  echo "FAIL: operator workload admission has $operator_delete_shape_bypasses DELETE shape bypasses, want 6" >&2
  fail=1
fi
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
if grep -R -nE 'BEX_STATIC_S3_SECRET|name: static-s3$' \
    lego/operator/cmd lego/operator/internal/publish >/dev/null || \
   grep -R -nE 'name: static-s3$' lego/operator/config >/dev/null; then
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
# must carry exactly the three origin keys AND match the manager env that
# dispatches the publish Job. BEX_BASE_DOMAIN is intentionally absent until a
# dedicated tenant-hosting suffix is registered in the Public Suffix List.
static_cfg_kv="$(yq -N 'select(.kind == "ConfigMap" and .metadata.name == "bex-static-config") | .data | to_entries | map(.key + "=" + .value) | sort | join(",")' "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
mgr_origin_kv="$(yq -N 'select(.kind == "Deployment" and .metadata.name == "bex-controller-manager") | .spec.template.spec.containers[] | select(.name == "manager") | [.env[] | select(.name == "BEX_STATIC_S3_ENDPOINT" or .name == "BEX_STATIC_S3_BUCKET" or .name == "BEX_STATIC_S3_REGION") | .name + "=" + .value] | sort | join(",")' "$tmp/bex-operator-prod.yaml" | tr -d '\n')"
expected_origin_kv='BEX_STATIC_S3_BUCKET=bex-static,BEX_STATIC_S3_ENDPOINT=https://s3.eu-central-2.wasabisys.com,BEX_STATIC_S3_REGION=eu-central-2'
if [ "$static_cfg_kv" != "$expected_origin_kv" ]; then
  echo "FAIL: bex-static-config keys/values are '$static_cfg_kv', want '$expected_origin_kv'" >&2
  fail=1
fi
if [ "$static_cfg_kv" != "$mgr_origin_kv" ]; then
  echo "FAIL: static serve origin (bex-static-config) != publish origin (manager env): '$static_cfg_kv' vs '$mgr_origin_kv'" >&2
  fail=1
fi
# NOTE: this used to assert the OPPOSITE — that prod carries NO BEX_BASE_DOMAIN
# "until a dedicated suffix is in the PSL PRIVATE section" (815e003b). That
# inverted guard is gone: the PSL gap is an accepted risk (.pm/DO_NOT_DO.md
# #PSL), and pinning the absence made an outage the only CI-passing state —
# every App silently lost its platform host. The live assertion is now the
# non-empty check earlier in this script; do not reintroduce an absence gate.
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
grep -q 'apply_secret "$BUILD_NS" bex-registry-pull' scripts/registry-secrets.sh \
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
# Round-18: the static credential Secrets must carry the operator's
# protected-from-tenant-mount label so a tenant App naming one in a mount
# field is refused even in a co-located (BEX_BUILD_NAMESPACE unset) install.
grep -qF '"app.bex.co/protected-from-tenant-mount":"true"' scripts/static-s3-credentials.sh || {
  echo "FAIL: static-s3-credentials.sh no longer stamps the protected-from-tenant-mount label" >&2
  fail=1
}
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

echo "==> agent-session snapshot S3 contract (ADR059 / w2/m77)"
snapshot_env="$(yq -N '
  select(.kind == "Deployment" and .metadata.name == "bex-api") |
  .spec.template.spec.containers[] | select(.name == "api") |
  .env[] | select(.name | test("^BEX_AGENT_SNAPSHOT_S3_")) |
  .name + "=" + .valueFrom.secretKeyRef.name + ":" + .valueFrom.secretKeyRef.key + ":" + ((.valueFrom.secretKeyRef.optional // false) | tostring)
' "$tmp/bex-operator-prod.yaml" | sort | paste -sd, -)"
expected_snapshot_env='BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY=bex-agent-snapshot:BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY:true,BEX_AGENT_SNAPSHOT_S3_BUCKET=bex-agent-snapshot:BEX_AGENT_SNAPSHOT_S3_BUCKET:true,BEX_AGENT_SNAPSHOT_S3_ENDPOINT=bex-agent-snapshot:BEX_AGENT_SNAPSHOT_S3_ENDPOINT:true,BEX_AGENT_SNAPSHOT_S3_PREFIX=bex-agent-snapshot:BEX_AGENT_SNAPSHOT_S3_PREFIX:true,BEX_AGENT_SNAPSHOT_S3_REGION=bex-agent-snapshot:BEX_AGENT_SNAPSHOT_S3_REGION:true,BEX_AGENT_SNAPSHOT_S3_SECRET_KEY=bex-agent-snapshot:BEX_AGENT_SNAPSHOT_S3_SECRET_KEY:true'
if [ "$snapshot_env" != "$expected_snapshot_env" ]; then
  echo "FAIL: bex-api snapshot S3 env is '$snapshot_env', want all six optional secretKeyRefs on bex-agent-snapshot" >&2
  fail=1
fi
snapshot_policy_shape="$(jq -r '[.Statement[] | .Action[]? // .Action] | flatten | sort | join(",")' \
  infra/wasabi/agent-snapshot-s3-policy.json)"
expected_snapshot_actions='s3:AbortMultipartUpload,s3:DeleteObject,s3:GetBucketLocation,s3:GetObject,s3:ListBucket,s3:ListMultipartUploadParts,s3:PutObject'
[ "$snapshot_policy_shape" = "$expected_snapshot_actions" ] || {
  echo "FAIL: agent snapshot IAM actions drifted: $snapshot_policy_shape" >&2
  fail=1
}
snapshot_resources="$(jq -r '[.Statement[].Resource] | flatten | sort | join(",")' \
  infra/wasabi/agent-snapshot-s3-policy.json)"
[ "$snapshot_resources" = "arn:aws:s3:::bex-agent-snapshots,arn:aws:s3:::bex-agent-snapshots/*" ] || {
  echo "FAIL: agent-snapshot-s3-policy.json is not confined to bex-agent-snapshots: $snapshot_resources" >&2
  fail=1
}
if grep -q 'bex-tfstate' infra/wasabi/agent-snapshot-s3-policy.json; then
  echo "FAIL: agent snapshot IAM policy must never name bex-tfstate" >&2
  fail=1
fi
for required_snapshot_probe in \
  'snapshot put object' \
  'snapshot list tfstate bucket' \
  'snapshot list account buckets' \
  'snapshot list unrelated bucket' \
  'probe object AES256'; do
  grep -qF "$required_snapshot_probe" scripts/agent-snapshot-secret.sh || {
    echo "FAIL: agent snapshot verifier lost '$required_snapshot_probe'" >&2
    fail=1
  }
done

[ "$fail" -eq 0 ] && echo "PASS: gitops tree renders" || { echo "FAIL: see errors above" >&2; exit 1; }
