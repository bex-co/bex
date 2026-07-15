#!/usr/bin/env bash
# Backfill Database.spec.name without changing metadata.name. The latter is the
# immutable API/data-plane identity, so this migration never creates, deletes,
# or renames a Kubernetes object. Safe to rerun: populated objects are skipped.
set -euo pipefail

APPLY=0
NAMESPACE="${BEX_API_NAMESPACE:-bex-system}"
ALL_NAMESPACES=0

usage() {
  cat <<'EOF'
Usage: scripts/postgres-name-migrate.sh [--apply] [--namespace NS | --all-namespaces]

Without --apply, print the idempotent backfill plan. Run once per dev/prod
cluster after the CRD is upgraded and before enabling rename traffic.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --apply) APPLY=1 ;;
    --namespace)
      [ "$#" -ge 2 ] || { echo "error: --namespace needs a value" >&2; exit 2; }
      NAMESPACE="$2"
      shift
      ;;
    --all-namespaces) ALL_NAMESPACES=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

if [ "$ALL_NAMESPACES" -eq 1 ]; then
  scope_label="all namespaces"
else
  scope_label="namespace $NAMESPACE"
fi

command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }

if [ "$ALL_NAMESPACES" -eq 1 ]; then
  objects="$(kubectl get databases.app.bex.co -A -o json)"
else
  objects="$(kubectl -n "$NAMESPACE" get databases.app.bex.co -o json)"
fi

# The upgraded CRD accepts the same canonical display-name shape as the API.
# Stop before any write if a legacy metadata.name cannot be represented.
invalid="$(jq -r '
  .items[]
  | select((.spec.name // "") == "")
  | select((.metadata.name | test("^[a-z0-9](?:[a-z0-9-]{0,28}[a-z0-9])?$")) | not)
  | [.metadata.namespace, .metadata.name] | @tsv
' <<<"$objects")"
if [ -n "$invalid" ]; then
  echo "error: legacy ids below cannot be used as display names; set a valid spec.name explicitly before rollout:" >&2
  echo "$invalid" >&2
  exit 1
fi

# Existing legacy/new objects must not already violate workspace-scoped name
# uniqueness. A blank tenant label is the standalone/dev scope.
duplicates="$(jq -r '
  [.items[] | {
    namespace: .metadata.namespace,
    id: .metadata.name,
    tenant: (.metadata.labels["bex.co/tenant"] // ""),
    name: (.spec.name // .metadata.name)
  }]
  | group_by([.namespace, .tenant, .name])[]
  | select(length > 1)
  | "\(.[0].namespace)\t\(.[0].tenant // "")\t\(.[0].name)\t\([.[].id] | join(","))"
' <<<"$objects")"
if [ -n "$duplicates" ]; then
  echo "error: duplicate database display names exist (namespace, tenant, name, ids):" >&2
  echo "$duplicates" >&2
  exit 1
fi

pending="$(jq -r '
  .items[]
  | select((.spec.name // "") == "")
  | [.metadata.namespace, .metadata.name] | @tsv
' <<<"$objects")"

if [ -z "$pending" ]; then
  echo "postgres display-name backfill: already complete in $scope_label"
  exit 0
fi

count="$(wc -l <<<"$pending" | tr -d ' ')"
echo "postgres display-name backfill: $count object(s) pending in $scope_label"
while IFS=$'\t' read -r namespace id; do
  [ -n "$id" ] || continue
  if [ "$APPLY" -eq 0 ]; then
    echo "plan: $namespace/$id spec.name <- $id"
    continue
  fi
  body="$(jq -nc --arg name "$id" '{spec:{name:$name}}')"
  kubectl -n "$namespace" patch databases.app.bex.co "$id" --type=merge -p "$body" >/dev/null
  echo "backfilled: $namespace/$id"
done <<<"$pending"

if [ "$APPLY" -eq 0 ]; then
  echo "dry run only; rerun with --apply after reviewing the plan"
else
  echo "postgres display-name backfill complete in $scope_label"
fi
