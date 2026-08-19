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

# w1/m71 folded 30 per-field MCP setters into five patch-shaped update_* tools
# (docs/render-artifacts/mcp-setter-fold-2026-08-18.md). The names may still be
# NAMED in prose — that is how a caller finds the replacement — so this checks
# only that none of them is REGISTERED as a tool again.
if grep -REn 'Name: *"set_(display_name|branch|registry_credential|root_directory|build_command|start_command|dockerfile_path|health_check_path|pre_deploy_command|max_shutdown_delay|auto_deploy|build_filter|notify_on_fail|notifications_to_send|maintenance_mode|subdomain_policy|service_ip_allow_list|autoscaling|postgres_ip_allow_list|postgres_parameter_overrides|key_value_ip_allow_list|key_value_maxmemory_policy|environment_acl|environment_services|environment_databases|environment_keyvalues|environment_env_groups|project_services|project_databases|project_keyvalues)"' \
  lego/backend/internal; then
  echo "FAIL: a retired per-field MCP setter is registered again — fold it into the resource's update_* tool" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "PASS: retired platform migration paths remain absent"
