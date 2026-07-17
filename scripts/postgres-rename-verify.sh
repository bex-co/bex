#!/usr/bin/env bash
# Snapshot/compare the Kubernetes identities attached to one Database. Values
# and Secret data are never read: evidence contains only kind/name/UID tuples.
set -euo pipefail

MODE="${1:-}"
[ -n "$MODE" ] && shift || true
NAMESPACE="${BEX_API_NAMESPACE:-bex-system}"
DATABASE=""
NAME=""
OUTPUT=""
BEFORE=""

usage() {
  cat <<'EOF'
Usage:
  scripts/postgres-rename-verify.sh snapshot --namespace NS --database ID --output FILE
  scripts/postgres-rename-verify.sh compare  --namespace NS --database ID --before FILE [--name NEW_NAME]

Take snapshot before a rename, run the REST/GraphQL/MCP/dashboard/official-CLI
rename, then compare. Any recorded Database/CNPG/PVC/Secret/Service/Pooler/
route/backup/export/recovery object that disappears or changes UID fails the
comparison. Children added by normal asynchronous provisioning are allowed.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace) NAMESPACE="$2"; shift ;;
    --database) DATABASE="$2"; shift ;;
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
[ -n "$DATABASE" ] || { echo "error: --database is required" >&2; exit 2; }
command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }

resources=(
  databases.app.bex.co
  clusters.postgresql.cnpg.io
  persistentvolumeclaims
  secrets
  services
  poolers.postgresql.cnpg.io
  ingressroutetcps.traefik.io
  scheduledbackups.postgresql.cnpg.io
  backups.postgresql.cnpg.io
  jobs.batch
)

snapshot() {
  local resource json err
  err="$(mktemp)"
  # shellcheck disable=SC2064
  trap "rm -f '$err'" RETURN
  for resource in "${resources[@]}"; do
    if ! json="$(kubectl -n "$NAMESPACE" get "$resource" -o json 2>"$err")"; then
      # Only an uninstalled resource type may be skipped (e.g. Traefik CRDs on
      # a cluster without them). Any other list failure must not be silently
      # dropped: a vanished line would read as identity churn in compare.
      if grep -qiE "doesn't have a resource type|could not find the requested resource" "$err"; then
        continue
      fi
      echo "error: listing $resource in $NAMESPACE failed:" >&2
      cat "$err" >&2
      exit 1
    fi
    [ -n "$json" ] || continue
    jq -r --arg id "$DATABASE" '
      .items[]
      | select(
          .metadata.name == $id
          or (.metadata.name | startswith($id + "-"))
          or (.metadata.labels["cnpg.io/cluster"] // "") == $id
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
  [ -s "$OUTPUT" ] || { echo "error: no objects found for $NAMESPACE/$DATABASE" >&2; exit 1; }
  echo "captured $(wc -l <"$OUTPUT" | tr -d ' ') identities for $NAMESPACE/$DATABASE"
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
  echo "error: Kubernetes object identity changed during Postgres rename" >&2
  exit 1
fi
if [ -n "$NAME" ]; then
  actual="$(kubectl -n "$NAMESPACE" get databases.app.bex.co "$DATABASE" -o jsonpath='{.spec.name}')"
  [ "$actual" = "$NAME" ] || { echo "error: spec.name=$actual, want $NAME" >&2; exit 1; }
fi
echo "verified: $NAMESPACE/$DATABASE kept every recorded object identity"
