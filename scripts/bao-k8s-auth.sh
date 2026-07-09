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
# These are cluster-wide raft-replicated writes, so they need only one
# unsealed endpoint — the leader, resolved by bao-endpoints.sh (shared with
# bao-init.sh). Reaching the leader directly (not the round-robin `openbao`
# Service) keeps this reliable in HA, and means a standalone run doesn't depend
# on bao-init.sh having run first in the same shell.
#
# Requires BAO_ROOT_TOKEN (from .env, written by scripts/bao-init.sh) — never
# printed.
#
# Usage: scripts/bao-k8s-auth.sh          # port-forwards each OpenBao pod, uses the leader
#        BAO_ADDR=http://... ...         # use an already-reachable OpenBao endpoint
#        DRY_RUN=1 ...                   # print intent, change nothing
# Requires: curl, yq v4; kubectl unless BAO_ADDR is set.
set -euo pipefail
# Resolve the script dir to an absolute path BEFORE cd so the source works from
# any cwd (a relative dirname would be stale after the cd).
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$here/.."
# shellcheck disable=SC1091 — defines bao_resolve_endpoints / bao_cleanup_forwards
source "$here/bao-endpoints.sh"

SA_NAME=bex-api
SA_NAMESPACE=bex-system
POLICY_NAME=tenants-rw
ROLE_NAME=bex-api

if [ -z "${BAO_ROOT_TOKEN:-}" ] && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
token="${BAO_ROOT_TOKEN:-}"
[ -n "$token" ] || { echo "error: BAO_ROOT_TOKEN is missing or empty (.env or environment) — run scripts/bao-init.sh first" >&2; exit 1; }

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would ensure kubernetes auth method, policy $POLICY_NAME, role $ROLE_NAME ($SA_NAME.$SA_NAMESPACE)"
  exit 0
fi

# Install the cleanup trap BEFORE resolving so a partway failure in
# bao_resolve_endpoints (which has already launched port-forwards) can't orphan
# them.
trap bao_cleanup_forwards EXIT
bao_resolve_endpoints secrets
url="$bao_leader"

# Refuse to run against a sealed (or unreachable) leader: every write below
# would 503, and because bao() uses `curl -s` (no -f) the 503 bodies would flow
# into the has()/write calls and the script would print success while
# configuring nothing. Fail loudly instead of silently mis-reporting.
if [ "$(curl -sf "$url/v1/sys/seal-status" | yq '.sealed' -)" != "false" ]; then
  echo "error: OpenBao at $url is sealed or unreachable — run scripts/bao-init.sh first" >&2
  exit 1
fi

bao() { # METHOD PATH [JSON_BODY]
  # Root token via a curl --config file on stdin, never argv (ps/proc leak).
  local args=(-s -X "$1" "$url/v1/$2")
  [ "${3:-}" != "" ] && args+=(-d "$3")
  printf 'header = "X-Vault-Token: %s"' "$token" | curl "${args[@]}" --config -
}

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
