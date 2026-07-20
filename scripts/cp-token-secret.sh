#!/usr/bin/env bash
set -euo pipefail

# Install the internal control-plane API bearer token (w1/m53) without ever
# rendering its bytes to stdout or checking them into Git.
#
# bex-api's internal tenant API on :8091 (BEX_CP_ADDR) grants workspace-admin and
# performs cross-tenant writes; it is reachable from platform-namespace pods
# (bex-system/traefik/monitoring). bex-api reads this token from the
# bex-control-plane Secret (key `token`) and refuses to start the :8091 listener
# when it is empty (requireCPAuth), so this Secret must exist BEFORE the w1/m53
# deployment change rolls, or the pod crash-loops with a clear fatal log.
#
# Value precedence:
#   1. $BEX_CP_TOKEN from the environment (e.g. sourced from .env).
#   2. An existing in-cluster bex-control-plane Secret — reused as-is (idempotent
#      re-runs never rotate it).
#   3. A freshly generated 256-bit random token (first install).
# Set ROTATE=1 to force a new random token even when one already exists (existing
# callers must be updated with the new value).

namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="bex-control-plane"

for command in kubectl; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

secret_value="${BEX_CP_TOKEN:-}"

if [[ -z "$secret_value" && "${ROTATE:-0}" != "1" ]]; then
  # Reuse the existing token so re-runs are idempotent.
  secret_value="$(kubectl -n "$namespace" get secret "$secret_name" \
    -o jsonpath='{.data.token}' 2>/dev/null | base64 --decode 2>/dev/null || true)"
fi

if [[ -z "$secret_value" ]]; then
  command -v openssl >/dev/null || { echo "missing required command: openssl (needed to generate a token)" >&2; exit 1; }
  secret_value="$(openssl rand -hex 32)"
  echo "generated a new 256-bit control-plane token"
fi

kubectl -n "$namespace" create secret generic "$secret_name" \
  --from-literal=token="$secret_value" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
unset secret_value

# bex-api reads BEX_CP_TOKEN once at startup, so roll it after install/rotation.
# On first install the Deployment may not exist yet.
if kubectl -n "$namespace" get deployment/bex-api >/dev/null 2>&1; then
  kubectl -n "$namespace" rollout restart deployment/bex-api >/dev/null
  kubectl -n "$namespace" rollout status deployment/bex-api --timeout=180s >/dev/null
fi

echo "installed $secret_name in namespace $namespace; control-plane API now requires a bearer"
