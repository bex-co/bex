#!/usr/bin/env bash
# Enable OpenBao's Kubernetes auth method and scope the bex-api ServiceAccount
# to tenants/* (docs/secrets.md#5), idempotently — the same out-of-band deploy
# step pattern as authz-model.sh:
#   1. ensure the `kubernetes` auth method is enabled and configured
#      (kubernetes_host is the only field OpenBao requires explicitly; the
#      reviewer JWT/CA cert default to OpenBao's own mounted ServiceAccount
#      token via server.authDelegator when left unset),
#   2. write the `tenants-rw` policy: create/read/update/delete/list on
#      tenants/*, nothing else — no sys/*, no other mount,
#   3. write the `bex-api` role, binding ServiceAccount bex-api (ns
#      bex-system) to that policy.
#
# Requires BAO_ROOT_TOKEN (from .env, written by scripts/bao-init.sh) — never
# printed.
#
# Usage: scripts/bao-k8s-auth.sh          # port-forwards secrets/openbao
#        BAO_ADDR=http://... ...         # use an already-reachable OpenBao
#        DRY_RUN=1 ...                   # print intent, change nothing
# Requires: curl, yq v4; kubectl unless BAO_ADDR is set.
set -euo pipefail
cd "$(dirname "$0")/.."

NS=secrets
SA_NAME=bex-api
SA_NAMESPACE=bex-system
POLICY_NAME=tenants-rw
ROLE_NAME=bex-api
PF_PID=""

if [ -z "${BAO_ROOT_TOKEN:-}" ] && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
token="${BAO_ROOT_TOKEN:-}"
[ -n "$token" ] || { echo "error: BAO_ROOT_TOKEN is missing or empty (.env or environment) — run scripts/bao-init.sh first" >&2; exit 1; }

cleanup() {
  if [ -n "$PF_PID" ]; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

url="${BAO_ADDR:-}"
if [ -z "$url" ]; then
  kubectl -n "$NS" port-forward service/openbao 38200:8200 >/dev/null 2>&1 &
  PF_PID=$!
  url=http://127.0.0.1:38200
  for _ in $(seq 1 30); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url/v1/sys/seal-status" || true)"
    [ "$code" != "000" ] && break
    sleep 2
  done
fi

bao() { # METHOD PATH [JSON_BODY]
  local args=(-s -X "$1" "$url/v1/$2" -H "X-Vault-Token: $token")
  [ "${3:-}" != "" ] && args+=(-d "$3")
  curl "${args[@]}"
}

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would ensure kubernetes auth method, policy $POLICY_NAME, role $ROLE_NAME ($SA_NAME.$SA_NAMESPACE)"
  exit 0
fi

# --- 1. kubernetes auth method --------------------------------------------------
if [ "$(bao GET sys/auth | yq 'has("kubernetes/")' -)" = "true" ]; then
  echo "kubernetes auth method already enabled"
else
  bao POST sys/auth/kubernetes '{"type":"kubernetes"}' >/dev/null
  echo "enabled kubernetes auth method"
fi
bao POST auth/kubernetes/config '{"kubernetes_host":"https://kubernetes.default.svc"}' >/dev/null
echo "configured kubernetes auth method (in-cluster auto-detected reviewer JWT/CA cert)"

# --- 2. policy: tenants/* only, nothing else -------------------------------------
bao PUT "sys/policies/acl/$POLICY_NAME" \
  '{"policy":"path \"tenants/*\" { capabilities = [\"create\",\"read\",\"update\",\"delete\",\"list\"] }"}' >/dev/null
echo "wrote policy $POLICY_NAME"

# --- 3. role: bind the bex-api ServiceAccount to that policy ---------------------
bao POST "auth/kubernetes/role/$ROLE_NAME" \
  "$(printf '{"bound_service_account_names":"%s","bound_service_account_namespaces":"%s","token_policies":"%s","token_ttl":"1h"}' \
    "$SA_NAME" "$SA_NAMESPACE" "$POLICY_NAME")" >/dev/null
echo "wrote role $ROLE_NAME ($SA_NAME.$SA_NAMESPACE -> $POLICY_NAME)"
