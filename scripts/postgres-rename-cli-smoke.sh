#!/usr/bin/env bash
# Official render-oss/cli rename plus Kubernetes no-recreation proof. Auth and
# endpoint setup are delegated to scripts/cli-compat.sh.
set -euo pipefail
cd "$(dirname "$0")/.."

NAMESPACE="${BEX_API_NAMESPACE:-bex-system}"
DATABASE=""
NAME=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace) NAMESPACE="$2"; shift ;;
    --database) DATABASE="$2"; shift ;;
    --name) NAME="$2"; shift ;;
    -h|--help)
      echo "Usage: scripts/postgres-rename-cli-smoke.sh --namespace NS --database ID --name NEW_NAME"
      exit 0
      ;;
    *) echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

[ -n "$DATABASE" ] && [ -n "$NAME" ] || {
  echo "error: --database and --name are required" >&2
  exit 2
}

before="$(mktemp)"
trap 'rm -f "$before"' EXIT
bash scripts/postgres-rename-verify.sh snapshot \
  --namespace "$NAMESPACE" --database "$DATABASE" --output "$before"
bash scripts/cli-compat.sh postgres update "$DATABASE" --name "$NAME" --output json
resolved="$(bash scripts/cli-compat.sh postgres get "$NAME" --output json)"
resolved_id="$(jq -r '.id // .data.id // empty' <<<"$resolved")"
[ "$resolved_id" = "$DATABASE" ] || {
  echo "error: CLI resolved new name to id $resolved_id, want $DATABASE" >&2
  exit 1
}
bash scripts/postgres-rename-verify.sh compare \
  --namespace "$NAMESPACE" --database "$DATABASE" --before "$before" --name "$NAME"
