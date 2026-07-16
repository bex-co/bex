#!/usr/bin/env bash
# Snapshot/compare the Kubernetes identities attached to one KeyValue. Values
# and Secret data are never read: evidence contains only kind/name/UID tuples.
# The KeyValue sibling of scripts/postgres-rename-verify.sh (w9/m6).
set -euo pipefail

MODE="${1:-}"
[ -n "$MODE" ] && shift || true
NAMESPACE="${BEX_API_NAMESPACE:-bex-system}"
KEYVALUE=""
NAME=""
OUTPUT=""
BEFORE=""

usage() {
  cat <<'EOF'
Usage:
  scripts/keyvalue-rename-verify.sh snapshot --namespace NS --keyvalue ID --output FILE
  scripts/keyvalue-rename-verify.sh compare  --namespace NS --keyvalue ID --before FILE [--name NEW_NAME]

Take snapshot before a rename, run the REST/GraphQL/MCP/dashboard/official-CLI
rename, then compare. Any recorded KeyValue/StatefulSet/PVC/Secret/Service/
route object that disappears or changes UID fails the comparison. Children added
by normal asynchronous provisioning are allowed.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace) NAMESPACE="$2"; shift ;;
    --keyvalue) KEYVALUE="$2"; shift ;;
    --name) NAME="$2"; shift ;;
    --output) OUTPUT="$2"; shift ;;
    --before) BEFORE="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

case "$MODE" in
  snapshot|compare) ;;
  *) usage >&2; exit 2 ;;
esac
[ -n "$KEYVALUE" ] || { echo "error: --keyvalue is required" >&2; exit 2; }
command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }

resources=(
  keyvalues.app.bex.co
  statefulsets.apps
  persistentvolumeclaims
  secrets
  services
  ingressroutetcps.traefik.io
  middlewaretcps.traefik.io
)

snapshot() {
  local resource json
  for resource in "${resources[@]}"; do
    json="$(kubectl -n "$NAMESPACE" get "$resource" -o json 2>/dev/null || true)"
    [ -n "$json" ] || continue
    jq -r --arg id "$KEYVALUE" '
      .items[]
      | select(
          .metadata.name == $id
          or (.metadata.name | startswith($id + "-"))
          or any(.metadata.ownerReferences[]?; .name == $id)
          or any(.metadata.labels[]?; . == $id)
        )
      | [.apiVersion, .kind, .metadata.namespace, .metadata.name, .metadata.uid]
      | @tsv
    ' <<<"$json"
  done | LC_ALL=C sort -u
}

if [ "$MODE" = "snapshot" ]; then
  [ -n "$OUTPUT" ] || { echo "error: snapshot needs --output" >&2; exit 2; }
  snapshot >"$OUTPUT"
  [ -s "$OUTPUT" ] || { echo "error: no objects found for $NAMESPACE/$KEYVALUE" >&2; exit 1; }
  echo "captured $(wc -l <"$OUTPUT" | tr -d ' ') identities for $NAMESPACE/$KEYVALUE"
  exit 0
fi

[ -n "$BEFORE" ] && [ -f "$BEFORE" ] || { echo "error: compare needs an existing --before file" >&2; exit 2; }
after="$(mktemp)"
trap 'rm -f "$after"' EXIT
snapshot >"$after"
missing="$(comm -23 "$BEFORE" "$after")"
if [ -n "$missing" ]; then
  echo "recorded identities missing or replaced:" >&2
  printf '%s\n' "$missing" >&2
  echo "error: Kubernetes object identity changed during key-value rename" >&2
  exit 1
fi
if [ -n "$NAME" ]; then
  actual="$(kubectl -n "$NAMESPACE" get keyvalues.app.bex.co "$KEYVALUE" -o jsonpath='{.spec.name}')"
  [ "$actual" = "$NAME" ] || { echo "error: spec.name=$actual, want $NAME" >&2; exit 1; }
fi
echo "verified: $NAMESPACE/$KEYVALUE kept every recorded object identity"
