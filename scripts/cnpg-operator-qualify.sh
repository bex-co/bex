#!/usr/bin/env bash
# cnpg-operator-qualify.sh — run a real-CloudNativePG operator test on a
# disposable kind cluster with production's CloudNativePG operator and the
# pinned bex operand of one major (w21/m606 t003). The test is
# TestApplicationDatabaseExtensionsOnCNPG (every admitted extension, the
# owner's export and restore, and the owner's pgaudit across an instance pod
# restart; w21/124) unless BEX_CNPG_QUALIFY_TEST names another, such as
# TestOwnerPgauditAcrossRestartOnCNPG, the restart step alone.
#
#   bash scripts/cnpg-operator-qualify.sh [pinned|upstream]
#
# pinned (the default) installs the operator image
# deploy/gitops/base/cnpg-operator.yaml pins: bex's build, whose instance
# manager keeps the owner's pgaudit across the pod restart the test makes.
# upstream installs the release that build patches (deploy/cnpg-operator's
# runtime base), on which the same test fails at that restart: the run that
# shows what the patch changes. Both use production's chart archive
# (deploy/helm-artifacts.lock) and bexOperandImages' operand for
# BEX_CNPG_MAJORS (default 18). The cluster is deleted at the end unless
# BEX_CNPG_QUALIFY_KEEP=1.
#
# Needs docker, kind, kubectl, helm and go. CI: .github/workflows/cnpg-operator-qualify.yml.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/helm-pinned.sh
. scripts/lib/helm-pinned.sh

die() {
  echo "cnpg-operator-qualify: $*" >&2
  exit 1
}

operator=${1:-pinned}
major=${BEX_CNPG_MAJORS:-18}
test_name=${BEX_CNPG_QUALIFY_TEST:-TestApplicationDatabaseExtensionsOnCNPG}
cluster=${BEX_CNPG_QUALIFY_CLUSTER:-bex-cnpg-qualify}
node_image=kindest/node:v1.36.1

case "$operator" in
  pinned)
    image=$(cnpg_operator_image) || exit 1
    read -r repository tag <<<"$image"
    ;;
  upstream)
    repository=ghcr.io/cloudnative-pg/cloudnative-pg
    tag=$(sed -n 's|^FROM ghcr\.io/cloudnative-pg/cloudnative-pg:||p' deploy/cnpg-operator/Dockerfile)
    [ -n "$tag" ] || die "deploy/cnpg-operator/Dockerfile names no CloudNativePG release image"
    ;;
  *) die "usage: $0 [pinned|upstream]" ;;
esac
case "$major" in
  *,*) die "BEX_CNPG_MAJORS names one major per run, not $major" ;;
esac
operand=$(sed -nE "s/^[[:space:]]*\"$major\": \"([^\"]+)\",?$/\\1/p" lego/operator/internal/controller/database_operand.go)
[ -n "$operand" ] || die "bexOperandImages pins no PG$major operand"

work=$(mktemp -d "${TMPDIR:-/tmp}/bex-cnpg-qualify.XXXXXX")
kubeconfig="$work/kubeconfig"
cleanup() {
  if [ "${BEX_CNPG_QUALIFY_KEEP:-0}" != 1 ]; then
    kind delete cluster --name "$cluster" >/dev/null 2>&1 || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

echo "==> kind cluster $cluster ($node_image)"
kind create cluster --name "$cluster" --image "$node_image" --kubeconfig "$kubeconfig" --wait 3m
export KUBECONFIG="$kubeconfig"
# The operator stamps this class on every Postgres pod, and admission refuses
# a pod naming a class that does not exist (production applies the same file).
kubectl apply -f deploy/gitops/base/datastore-priority.yaml

echo "==> CloudNativePG operator $repository:$tag"
chart=$(fetch_pinned_chart cloudnative-pg "$work") || exit 1
helm upgrade --install cnpg "$chart" -n cnpg-system --create-namespace \
  --set image.repository="$repository" --set image.tag="$tag" --wait --timeout 5m
kubectl wait --for=condition=established crd/clusters.postgresql.cnpg.io --timeout=60s
# The Deployment reports Ready a few seconds before its webhook answers; a
# server-side dry run of a Cluster goes through the webhook.
cat >"$work/probe.yaml" <<'YAML'
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: webhook-probe
  namespace: default
spec:
  instances: 1
  storage:
    size: 1Gi
YAML
for attempt in $(seq 1 30); do
  out=$(kubectl apply --dry-run=server -f "$work/probe.yaml" 2>&1) && break
  [ "$attempt" -lt 30 ] || die "the CloudNativePG webhook never answered: $out"
  sleep 5
done

# The operand is large; pull it before the test's five-minute wait for the
# cluster starts.
echo "==> PG$major operand $operand"
for attempt in 1 2 3; do
  docker exec "$cluster-control-plane" crictl pull "$operand" >/dev/null && break
  [ "$attempt" -lt 3 ] || die "could not pull $operand"
  sleep $((attempt * 20))
done

echo "==> $test_name/PG$major against $operator"
rc=0
(cd lego/operator && GOWORK=off BEX_CNPG_KUBECONFIG="$kubeconfig" BEX_PG_OPERAND_IMAGES="$major=$operand" BEX_CNPG_MAJORS="$major" \
  go test ./internal/controller -run "^$test_name$/^PG$major$" -count=1 -v -timeout 40m) || rc=$?
if [ "$rc" -ne 0 ]; then
  # The test deletes its namespace; the operator's log is what is left.
  echo "==> CloudNativePG operator log (test failed)"
  kubectl -n cnpg-system logs deploy/cnpg-cloudnative-pg --tail=300 || true
fi
exit "$rc"
