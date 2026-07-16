#!/usr/bin/env bash
# Official render-oss/cli key-value rename plus Kubernetes no-recreation proof.
# Auth and endpoint setup are delegated to scripts/cli-compat.sh. The KeyValue
# sibling of scripts/postgres-rename-cli-smoke.sh (w9/m6).
set -euo pipefail
cd "$(dirname "$0")/.."

NAMESPACE="${BEX_API_NAMESPACE:-bex-system}"
KEYVALUE=""
NAME=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace) NAMESPACE="$2"; shift ;;
    --keyvalue) KEYVALUE="$2"; shift ;;
    --name) NAME="$2"; shift ;;
    -h|--help)
      echo "Usage: scripts/keyvalue-rename-cli-smoke.sh --namespace NS --keyvalue ID --name NEW_NAME"
      exit 0
      ;;
    *) echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

[ -n "$KEYVALUE" ] && [ -n "$NAME" ] || {
  echo "error: --keyvalue and --name are required" >&2
  exit 2
}

before="$(mktemp)"
trap 'rm -f "$before"' EXIT
bash scripts/keyvalue-rename-verify.sh snapshot \
  --namespace "$NAMESPACE" --keyvalue "$KEYVALUE" --output "$before"
bash scripts/cli-compat.sh keyvalues update "$KEYVALUE" --name "$NAME" --output json
resolved="$(bash scripts/cli-compat.sh keyvalues get "$NAME" --output json)"
resolved_id="$(jq -r '.id // .data.id // empty' <<<"$resolved")"
[ "$resolved_id" = "$KEYVALUE" ] || {
  echo "error: CLI resolved new name to id $resolved_id, want $KEYVALUE" >&2
  exit 1
}
bash scripts/keyvalue-rename-verify.sh compare \
  --namespace "$NAMESPACE" --keyvalue "$KEYVALUE" --before "$before" --name "$NAME"
