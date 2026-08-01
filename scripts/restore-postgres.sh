#!/usr/bin/env bash
# Recover any plugin-backed CNPG archive into an isolated throwaway namespace.
# The source Cluster and ObjectStore are read-only. This script never supports
# in-place recovery or a target namespace that is not named restore-*.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/restore.sh
source "$HERE/lib/restore.sh"

usage() {
  cat <<'EOF'
Usage:
  DRY_RUN=1 scripts/restore-postgres.sh --source-namespace NS --source-cluster NAME \
    --object-store NAME --server-name NAME --target-namespace restore-NAME \
    --database DB --query SQL [--expect VALUE] [--target-time RFC3339]

  scripts/restore-postgres.sh <same options> --confirm restore-NAME \
    [--teardown-on-success]

  scripts/restore-postgres.sh --teardown restore-NAME --confirm restore-NAME

The target is always a newly-created restore-* namespace. The source ObjectStore
and its referenced S3 Secrets are copied into that namespace without printing
their values. The recovery Cluster has no backup plugin, so it cannot write to
the source archive identity.
EOF
}

SOURCE_NAMESPACE=""
SOURCE_CLUSTER=""
OBJECT_STORE=""
SERVER_NAME=""
TARGET_NAMESPACE=""
DATABASE=""
QUERY=""
EXPECT=""
EXPECT_SET=0
TARGET_TIME=""
CONFIRM=""
TEARDOWN=""
TEARDOWN_ON_SUCCESS=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --source-namespace) SOURCE_NAMESPACE="${2:-}"; shift 2 ;;
    --source-cluster) SOURCE_CLUSTER="${2:-}"; shift 2 ;;
    --object-store) OBJECT_STORE="${2:-}"; shift 2 ;;
    --server-name) SERVER_NAME="${2:-}"; shift 2 ;;
    --target-namespace) TARGET_NAMESPACE="${2:-}"; shift 2 ;;
    --database) DATABASE="${2:-}"; shift 2 ;;
    --query) QUERY="${2:-}"; shift 2 ;;
    --expect) EXPECT="${2:-}"; EXPECT_SET=1; shift 2 ;;
    --target-time) TARGET_TIME="${2:-}"; shift 2 ;;
    --confirm) CONFIRM="${2:-}"; shift 2 ;;
    --teardown) TEARDOWN="${2:-}"; shift 2 ;;
    --teardown-on-success) TEARDOWN_ON_SUCCESS=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) restore_die "unknown argument: $1" ;;
  esac
done

restore_require_command kubectl
restore_require_command jq

if [ -n "$TEARDOWN" ]; then
  [ -z "$SOURCE_NAMESPACE$SOURCE_CLUSTER$OBJECT_STORE$SERVER_NAME$TARGET_NAMESPACE$DATABASE$QUERY$TARGET_TIME" ] || \
    restore_die "--teardown cannot be combined with recovery options"
  restore_require_throwaway_namespace "$TEARDOWN"
  restore_require_confirmation "$TEARDOWN" "$CONFIRM"
  [ "${DRY_RUN:-0}" != "1" ] || restore_die "teardown is unavailable in DRY_RUN mode"
  restore_delete_namespace "$TEARDOWN"
  exit 0
fi

[ -n "$SOURCE_NAMESPACE" ] || restore_die "missing --source-namespace"
[ -n "$SOURCE_CLUSTER" ] || restore_die "missing --source-cluster"
[ -n "$OBJECT_STORE" ] || restore_die "missing --object-store"
[ -n "$SERVER_NAME" ] || restore_die "missing --server-name"
[ -n "$TARGET_NAMESPACE" ] || restore_die "missing --target-namespace"
[ -n "$DATABASE" ] || restore_die "missing --database"
[ -n "$QUERY" ] || restore_die "missing --query"
restore_validate_dns_label "$SOURCE_NAMESPACE" "source namespace"
restore_validate_dns_label "$SOURCE_CLUSTER" "source cluster"
restore_validate_dns_label "$OBJECT_STORE" "object store"
restore_validate_dns_label "$SERVER_NAME" "server name"
restore_validate_dns_label "$DATABASE" "database"
restore_require_throwaway_namespace "$TARGET_NAMESPACE"
[ "$TARGET_NAMESPACE" != "$SOURCE_NAMESPACE" ] || restore_die "target namespace must differ from source namespace"
[ -z "$TARGET_TIME" ] || restore_validate_rfc3339 "$TARGET_TIME"

source_cluster_json="$(kubectl -n "$SOURCE_NAMESPACE" get clusters.postgresql.cnpg.io "$SOURCE_CLUSTER" -o json)"
object_store_json="$(kubectl -n "$SOURCE_NAMESPACE" get objectstores.barmancloud.cnpg.io "$OBJECT_STORE" -o json)"

recovery_json="$(printf '%s' "$source_cluster_json" | jq \
  --arg namespace "$TARGET_NAMESPACE" \
  --arg name "restore-pg" \
  --arg source "source-archive" \
  --arg objectStore "$OBJECT_STORE" \
  --arg serverName "$SERVER_NAME" \
  --arg targetTime "$TARGET_TIME" '
  {
    apiVersion: "postgresql.cnpg.io/v1",
    kind: "Cluster",
    metadata: {
      name: $name,
      namespace: $namespace,
      labels: {"bex.co/restore-target": "true"},
      annotations: {"argocd.argoproj.io/sync-options": "SkipDryRunOnMissingResource=true"}
    },
    spec: {
      instances: 1,
      imageName: .spec.imageName,
      affinity: .spec.affinity,
      storage: .spec.storage,
      resources: .spec.resources,
      bootstrap: {recovery: ({source: $source} + (if $targetTime == "" then {} else {recoveryTarget: {targetTime: $targetTime}} end))},
      externalClusters: [{
        name: $source,
        plugin: {
          name: "barman-cloud.cloudnative-pg.io",
          parameters: {barmanObjectName: $objectStore, serverName: $serverName}
        }
      }]
    }
  }
  | .spec |= with_entries(select(.value != null))')"

echo "Postgres recovery plan"
echo "  source: $SOURCE_NAMESPACE/$SOURCE_CLUSTER via ObjectStore $OBJECT_STORE, server $SERVER_NAME"
echo "  target: $TARGET_NAMESPACE/restore-pg (new namespace and new PVC only)"
[ -z "$TARGET_TIME" ] || echo "  PITR target: $TARGET_TIME"
echo "  verification database: $DATABASE (query text/result are not printed)"
printf '%s\n' "$recovery_json" | jq .
restore_print_dry_run
if [ "${DRY_RUN:-0}" = "1" ]; then
  exit 0
fi

restore_require_confirmation "$TARGET_NAMESPACE" "$CONFIRM"
if kubectl get namespace "$TARGET_NAMESPACE" >/dev/null 2>&1; then
  restore_die "target namespace already exists; choose a new restore-* namespace"
fi

restore_create_namespace "$TARGET_NAMESPACE" postgres

# Copy the ObjectStore and every referenced S3 credential Secret as a stream;
# Secret values never become argv, stdout, or a persisted local file.
printf '%s' "$object_store_json" | jq --arg namespace "$TARGET_NAMESPACE" '
  del(.metadata.uid, .metadata.resourceVersion, .metadata.creationTimestamp,
      .metadata.generation, .metadata.managedFields, .status)
  | .metadata.namespace = $namespace
  | .metadata.labels = ((.metadata.labels // {}) + {"bex.co/restore-target": "true"})
  | .metadata.annotations = {"argocd.argoproj.io/sync-options": "SkipDryRunOnMissingResource=true"}
' | kubectl apply -f - >/dev/null

mapfile -t credential_secrets < <(printf '%s' "$object_store_json" | jq -r '
  [.spec.configuration.s3Credentials.accessKeyId.name,
   .spec.configuration.s3Credentials.secretAccessKey.name,
   .spec.configuration.s3Credentials.sessionToken.name] | .[]? // empty' | sort -u)
[ "${#credential_secrets[@]}" -gt 0 ] || restore_die "ObjectStore has no referenced S3 credential Secrets"
for secret in "${credential_secrets[@]}"; do
  kubectl -n "$SOURCE_NAMESPACE" get secret "$secret" -o json | jq --arg namespace "$TARGET_NAMESPACE" '
    del(.metadata.uid, .metadata.resourceVersion, .metadata.creationTimestamp,
        .metadata.generation, .metadata.managedFields, .metadata.ownerReferences)
    | .metadata.namespace = $namespace
    | .metadata.labels = ((.metadata.labels // {}) + {"bex.co/restore-target": "true"})
    | .metadata.annotations = {}
  ' | kubectl apply -f - >/dev/null
done

printf '%s\n' "$recovery_json" | kubectl apply -f - >/dev/null
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "recovery submitted at $started_at; waiting for CNPG Ready"
kubectl -n "$TARGET_NAMESPACE" wait --for=condition=Ready \
  cluster.postgresql.cnpg.io/restore-pg --timeout="${RESTORE_READY_TIMEOUT:-20m}" >/dev/null
ready_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

result="$(kubectl -n "$TARGET_NAMESPACE" exec restore-pg-1 -c postgres -- \
  psql -X -v ON_ERROR_STOP=1 -U postgres -d "$DATABASE" -Atqc "$QUERY")"
if [ "$EXPECT_SET" -eq 1 ]; then
  [ "$result" = "$EXPECT" ] || restore_die "verification result did not match --expect"
else
  [ -n "$result" ] || restore_die "verification query returned no rows"
fi
echo "verification passed (query output suppressed)"
echo "recovery Ready at $ready_at"

if [ "$TEARDOWN_ON_SUCCESS" -eq 1 ]; then
  restore_delete_namespace "$TARGET_NAMESPACE"
else
  echo "target retained for review; teardown with: scripts/restore-postgres.sh --teardown $TARGET_NAMESPACE --confirm $TARGET_NAMESPACE"
fi
