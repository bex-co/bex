#!/usr/bin/env bash
# Migrate existing tenant workloads from the shared `default` namespace into
# per-tenant `<ws>` namespaces (ADR043 / w3/m31 t006).
#
# Context: with BEX_TENANT_NAMESPACES enabled (w3/m31 t002), the projector
# creates NEW App CRs in `<ws>` but leaves EXISTING CRs in `default` (it updates
# them in place, never moves them — a namespace is part of a CR's identity).
# This one-time script recreates each existing managed App/Database/KeyValue CR
# in its workspace's `<ws>` namespace, then removes the `default` copy.
#
# No-downtime approach (per resource): the operator reconciles the `<ws>` copy
# up to Ready (a second Deployment/Service behind the same slug) BEFORE the
# `default` copy is deleted; Ingress host routing resolves to whichever backing
# Service is Ready, and the activator/operator talk to the API only (never pod
# IPs), so wake survives the move (ADR022). Run per workspace, verify, then let
# it delete the old copy.
#
# SAFETY: defaults to DRY_RUN=1 (prints the plan, mutates nothing). This script
# has NOT been validated on a live cluster (t007 owns that); review every step
# against a staging workspace before running against production tenants.
#
# Usage:
#   DRY_RUN=1 scripts/migrate-tenant-namespaces.sh            # plan only (default)
#   DRY_RUN=0 scripts/migrate-tenant-namespaces.sh tea-abc    # migrate one workspace
#   DRY_RUN=0 scripts/migrate-tenant-namespaces.sh            # migrate ALL workspaces
set -euo pipefail

DRY_RUN="${DRY_RUN:-1}"
SRC_NS="${BEX_CP_APPS_NAMESPACE:-default}"
MANAGED_LABEL="app.kubernetes.io/managed-by=bex-controlplane"
WS_LABEL="app.bex.co/workspace"
KINDS=(apps.app.bex.co databases.app.bex.co keyvalues.app.bex.co)

run() {
  if [ "$DRY_RUN" = "1" ]; then
    echo "DRY_RUN: $*"
  else
    echo "+ $*"
    "$@"
  fi
}

# Workspaces to migrate: an explicit arg, else every distinct workspace label on
# managed CRs in the source namespace. An unlabeled legacy CR (ADR022 documents
# the gap) has no workspace and is SKIPPED with a warning — migrate those by
# hand once their owning workspace is confirmed.
workspaces() {
  if [ "$#" -gt 0 ]; then
    printf '%s\n' "$@"
    return
  fi
  kubectl get apps.app.bex.co -n "$SRC_NS" -l "$MANAGED_LABEL" \
    -o "jsonpath={range .items[*]}{.metadata.labels.app\.bex\.co/workspace}{'\n'}{end}" \
    | grep -v '^$' | sort -u
}

migrate_kind_for_ws() {
  local kind="$1" ws="$2" dst="$2"
  local names
  names="$(kubectl get "$kind" -n "$SRC_NS" -l "$MANAGED_LABEL,$WS_LABEL=$ws" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  [ -z "$names" ] && return 0
  while IFS= read -r name; do
    [ -z "$name" ] && continue
    echo "  migrate $kind/$name  ($SRC_NS -> $dst)"
    # Recreate in <ws>: strip namespace/resourceVersion/uid/status so the object
    # is admitted fresh; the operator reconciles it up alongside the old copy.
    if [ "$DRY_RUN" = "1" ]; then
      echo "  DRY_RUN: kubectl get $kind/$name -n $SRC_NS -o yaml | (rewrite namespace=$dst, drop rv/uid/status) | kubectl apply -f -"
    else
      kubectl get "$kind" "$name" -n "$SRC_NS" -o json \
        | kubectl neat 2>/dev/null || kubectl get "$kind" "$name" -n "$SRC_NS" -o json \
        | jq --arg ns "$dst" 'del(.status, .metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .metadata.managedFields) | .metadata.namespace = $ns' \
        | kubectl apply -f -
      echo "  waiting for $kind/$name in $dst to reconcile ..."
      kubectl wait --for=condition=Ready "$kind/$name" -n "$dst" --timeout=300s || {
        echo "  !! $kind/$name did not become Ready in $dst — NOT deleting the $SRC_NS copy" >&2
        return 1
      }
      run kubectl delete "$kind" "$name" -n "$SRC_NS"
    fi
  done <<<"$names"
}

main() {
  echo "== tenant namespace migration (DRY_RUN=$DRY_RUN, source ns=$SRC_NS) =="
  [ "$DRY_RUN" = "1" ] && echo "(dry run — mutating nothing; set DRY_RUN=0 to apply)"
  local ws
  while IFS= read -r ws; do
    [ -z "$ws" ] && continue
    echo "-- workspace $ws --"
    # The NamespaceReconciler (BEX_TENANT_NAMESPACES on) already provisions
    # <ws>; ensure it exists before recreating CRs into it.
    if ! kubectl get ns "$ws" >/dev/null 2>&1; then
      echo "  !! namespace $ws does not exist yet — enable BEX_TENANT_NAMESPACES and let the reconciler create it first" >&2
      continue
    fi
    for kind in "${KINDS[@]}"; do
      migrate_kind_for_ws "$kind" "$ws"
    done
  done < <(workspaces "$@")
  echo "== done =="
  echo "Verify with scripts/verify-tenant-isolation.sh (t007), then confirm $SRC_NS holds no tenant workloads."
}

main "$@"
