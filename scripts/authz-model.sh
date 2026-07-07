#!/usr/bin/env bash
# Apply the platform authorization model to OpenFGA, idempotently, and seed the
# bootstrap tuples (docs/auth.md) — the same out-of-band deploy step pattern as
# auth-secrets.sh / auth-bootstrap-client.sh:
#   1. ensure store `bex` exists (create by name if absent),
#   2. write deploy/gitops/authz/model.json as a new authorization model ONLY
#      when it differs from the latest applied one (models are append-only),
#   3. write the seed tuples (bex-bootstrap -> admin of tenant:default),
#      tolerating already-exists.
#
# The preshared key comes from OPENFGA_PRESHARED_KEY (explicit env wins; .env is
# the local fallback) and is never printed.
#
# Usage: scripts/authz-model.sh          # port-forwards auth/openfga
#        OPENFGA_URL=http://... ...      # use an already-reachable URL
#        DRY_RUN=1 ...                   # print intent, change nothing
# Requires: curl, yq v4; kubectl unless OPENFGA_URL is set.
set -euo pipefail
cd "$(dirname "$0")/.."

STORE_NAME=bex
MODEL_FILE=deploy/gitops/authz/model.json
NS=auth
PF_PID=""

if [ -z "${OPENFGA_PRESHARED_KEY:-}" ] && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
key="${OPENFGA_PRESHARED_KEY:-}"
[ -n "$key" ] || { echo "error: OPENFGA_PRESHARED_KEY is missing or empty (.env or environment)" >&2; exit 1; }

cleanup() {
  if [ -n "$PF_PID" ]; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

url="${OPENFGA_URL:-}"
if [ -z "$url" ]; then
  kubectl -n "$NS" port-forward service/openfga 38080:8080 >/dev/null 2>&1 &
  PF_PID=$!
  url=http://127.0.0.1:38080
  for _ in $(seq 1 30); do
    curl -s -o /dev/null "$url/stores" -H "Authorization: Bearer $key" && break
    sleep 2
  done
fi

fga() { # METHOD PATH [JSON_BODY]
  local args=(-s -X "$1" "$url$2" -H "Authorization: Bearer $key")
  [ "${3:-}" != "" ] && args+=(-H 'Content-Type: application/json' -d "$3")
  curl "${args[@]}"
}

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would ensure store $STORE_NAME, apply $MODEL_FILE if changed, seed bootstrap tuples"
  exit 0
fi

# --- 1. store ----------------------------------------------------------------
store_id="$(fga GET "/stores?page_size=100" | yq ".stores[] | select(.name == \"$STORE_NAME\") | .id" -)"
if [ -z "$store_id" ] || [ "$store_id" = "null" ]; then
  store_id="$(fga POST /stores "{\"name\":\"$STORE_NAME\"}" | yq '.id' -)"
  [ -n "$store_id" ] && [ "$store_id" != "null" ] || { echo "error: creating store failed" >&2; exit 1; }
  echo "created store $STORE_NAME ($store_id)"
else
  echo "store $STORE_NAME exists ($store_id)"
fi

# --- 2. model (append only when changed) ---------------------------------------
# normalize: OpenFGA's GET decorates models with server defaults (module: "",
# source_info: null, empty maps/arrays) — prune empties from BOTH sides (a
# symmetric transform, so real differences still surface), then sort keys.
# Pruning leaves can create newly-empty parents, so run to a fixpoint.
normalize() {
  local cur next
  cur="$(yq -o=json 'sort_keys(..)' -)"
  while next="$(printf '%s' "$cur" | yq -o=json '
      del(.. | select((tag == "!!null") or (. == "") or ((tag == "!!map") and length == 0) or ((tag == "!!seq") and length == 0)))' -)"       && [ "$next" != "$cur" ]; do
    cur="$next"
  done
  printf '%s' "$cur"
}
latest="$(fga GET "/stores/$store_id/authorization-models?page_size=1" \
  | yq -o=json '.authorization_models[0] | {"schema_version": .schema_version, "type_definitions": .type_definitions}' - | normalize)"
wanted="$(normalize <"$MODEL_FILE")"
if [ "$latest" = "$wanted" ]; then
  echo "model unchanged"
else
  model_id="$(fga POST "/stores/$store_id/authorization-models" "$(cat "$MODEL_FILE")" | yq '.authorization_model_id' -)"
  [ -n "$model_id" ] && [ "$model_id" != "null" ] || { echo "error: writing model failed" >&2; exit 1; }
  echo "applied model $model_id"
fi

# --- 3. seed tuples -------------------------------------------------------------
# bex-bootstrap (the platform operator's / CI's client, docs/bex-api.md#auth)
# administers the default tenant. Writes of existing tuples fail — tolerate that
# one error shape and nothing else.
seed='{"writes":{"tuple_keys":[{"user":"user:bex-bootstrap","relation":"admin","object":"workspace:default"}]}}'
resp="$(fga POST "/stores/$store_id/write" "$seed")"
if [ "$(printf '%s' "$resp" | yq '.code // ""' -)" = "write_failed_due_to_invalid_input" ]; then
  echo "seed tuples already present"
elif [ "$(printf '%s' "$resp" | yq 'has("code")' -)" = "true" ]; then
  echo "error: seeding tuples failed: $(printf '%s' "$resp" | yq '.code' -)" >&2
  exit 1
else
  echo "seeded bootstrap tuples"
fi
