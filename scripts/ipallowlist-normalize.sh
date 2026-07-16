#!/usr/bin/env bash
# Normalize legacy bare-CIDR-string spec.ipAllowList entries on Database and
# KeyValue CRs to the {cidr} object shape (w4/m29, the RC5 spec.name backfill
# precedent — scripts/postgres-name-migrate.sh's idempotent, non-destructive
# contract). Object-shaped entries pass through untouched, descriptions
# included; a second run finds nothing pending. Never creates, deletes, or
# renames an object. Run once per dev/prod cluster BEFORE the CRD schema is
# tightened back to structural (w4/m29/t003 — a later deploy, never this one).
set -euo pipefail

APPLY=0
NAMESPACE="${BEX_API_NAMESPACE:-bex-system}"
ALL_NAMESPACES=0

usage() {
  cat <<'EOF'
Usage: scripts/ipallowlist-normalize.sh [--apply] [--namespace NS | --all-namespaces]

Without --apply, print the idempotent normalization plan. Run once per
dev/prod cluster after w4/m24's union decoder is deployed and before the CRD
schema is tightened (w4/m29/t003).
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

# jq: entries already objects pass through unchanged (description preserved);
# bare strings become {cidr}. Anything else (number, array, object without a
# cidr key) is unexpected and aborts before any write.
normalize_filter='
  def normalized: map(if type == "string" then {cidr: .} else . end);
  def legacy_count: map(select(type == "string")) | length;
'

total_pending=0
for kind in databases.app.bex.co keyvalues.app.bex.co; do
  if [ "$ALL_NAMESPACES" -eq 1 ]; then
    objects="$(kubectl get "$kind" -A -o json)"
  else
    objects="$(kubectl -n "$NAMESPACE" get "$kind" -o json)"
  fi

  malformed="$(jq -r "$normalize_filter"'
    .items[]
    | select((.spec.ipAllowList // []) | map(type) | any(. != "string" and . != "object"))
    | [.metadata.namespace, .metadata.name] | @tsv
  ' <<<"$objects")"
  if [ -n "$malformed" ]; then
    echo "error: $kind entries below have ipAllowList items that are neither strings nor objects; fix by hand before normalizing:" >&2
    echo "$malformed" >&2
    exit 1
  fi

  pending="$(jq -r "$normalize_filter"'
    .items[]
    | select(((.spec.ipAllowList // []) | legacy_count) > 0)
    | [.metadata.namespace, .metadata.name, ((.spec.ipAllowList // []) | legacy_count | tostring)] | @tsv
  ' <<<"$objects")"

  if [ -z "$pending" ]; then
    echo "$kind: ipAllowList normalization already complete in $scope_label"
    continue
  fi

  count="$(wc -l <<<"$pending" | tr -d ' ')"
  total_pending=$((total_pending + count))
  echo "$kind: $count object(s) with legacy string entries in $scope_label"
  while IFS=$'\t' read -r namespace id legacy; do
    [ -n "$id" ] || continue
    if [ "$APPLY" -eq 0 ]; then
      echo "plan: $namespace/$id — rewrite $legacy string entr$( [ "$legacy" = 1 ] && echo y || echo ies ) to {cidr}"
      continue
    fi
    body="$(jq -c "$normalize_filter"'
      .items[] | select(.metadata.namespace == $ns and .metadata.name == $id)
      | {spec: {ipAllowList: ((.spec.ipAllowList // []) | normalized)}}
    ' --arg ns "$namespace" --arg id "$id" <<<"$objects")"
    kubectl -n "$namespace" patch "$kind" "$id" --type=merge -p "$body" >/dev/null
    echo "normalized: $namespace/$id ($legacy entr$( [ "$legacy" = 1 ] && echo y || echo ies ))"
  done <<<"$pending"
done

if [ "$APPLY" -eq 0 ] && [ "$total_pending" -gt 0 ]; then
  echo "dry run only; rerun with --apply after reviewing the plan"
elif [ "$APPLY" -eq 1 ]; then
  echo "ipAllowList normalization complete in $scope_label"
fi
