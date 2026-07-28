#!/usr/bin/env bash
# Fail closed if any w1/m56 production-only migration path returns to active
# source or rendered configuration. Historical docs/drills are intentionally
# outside this scan; behavioral tests separately prove the supported shape.
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

for retired in \
  scripts/ipallowlist-normalize.sh \
  scripts/ipallowlist-normalize.test.sh \
  scripts/keyvalue-name-migrate.sh \
  scripts/keyvalue-name-migrate.test.sh \
  scripts/postgres-name-migrate.sh \
  scripts/postgres-name-migrate.test.sh; do
  if [ -e "$retired" ]; then
    echo "FAIL: retired one-time migration script returned: $retired" >&2
    fail=1
  fi
done

if grep -REn --exclude='*_test.go' 'barmanObjectStore' \
  lego/operator lego/backend deploy/gitops; then
  echo "FAIL: active code or GitOps contains the retired in-tree Barman field" >&2
  fail=1
fi

if grep -REn --exclude='README.md' 'KUBERNETES_MIN_VERSION' \
  deploy/gitops/charts/kpack; then
  echo "FAIL: active kpack configuration contains the retired minimum-version override" >&2
  fail=1
fi

if grep -En 'v?1\.31(\.[0-9]+)?' \
  infra/clusterapi/overlays/hetzner-caph/cluster.yaml \
  infra/packer/bex-worker.pkr.hcl \
  .github/workflows/snapshot.yml; then
  echo "FAIL: an active provisioning path returned to Kubernetes 1.31" >&2
  fail=1
fi

if grep -En 'IngressRouteTCP|MiddlewareTCP|cleanupLegacyKeyValueRoutes|traefik(IngressRouteTCP|MiddlewareTCP)GVK' \
  lego/operator/internal/controller/database_controller.go \
  lego/operator/internal/controller/keyvalue_controller.go; then
  echo "FAIL: datastore controllers contain the retired Traefik route path" >&2
  fail=1
fi

if grep -REn --exclude='*_test.go' \
  'legacyAppArtifactOwned|DeleteAppArtifacts|BackfillReport|RepairDriftedEnvironmentIDs|ListAllEnvironments|func \([^)]*ArtifactIdentity\) Adopt\(' \
  lego/operator/internal/build \
  lego/operator/internal/execution \
  lego/backend/internal/environments \
  lego/backend/internal/store; then
  echo "FAIL: a retired metadata/datastore backfill abstraction returned" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "PASS: retired platform migration paths remain absent"
