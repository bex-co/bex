#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

# Install the shared `render ea sandbox exec` HMAC secret (w3/m33,
# docs/render-artifacts/ea-sandbox.md §exec) without ever rendering its bytes to
# stdout or checking them into Git.
#
# This HMAC-SHA256 key is the single activation gate for sandbox exec: bex-api
# signs a short-lived per-exec ticket with it (binding the sandbox pod + signed
# argv) and the isolated ssh-gateway verifies the signature before running the
# command via pods/exec — both processes read the SAME value from the
# bex-sandbox-exec Secret (key `secret`). While the Secret is absent both sides
# stay disabled (the exec verb returns 503 and the gateway never starts its
# internal :8081 listener); create/list/stop are unaffected. pods/exec stays
# confined to the gateway; bex-api only authorizes + signs (Option A).
#
# Value precedence:
#   1. $BEX_SANDBOX_EXEC_SECRET from the environment (e.g. sourced from .env).
#   2. An existing in-cluster bex-sandbox-exec Secret — reused as-is (idempotent).
#   3. A freshly generated 256-bit random key (first install).
# Set ROTATE=1 to force a new random key (invalidates any in-flight tickets).

namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="bex-sandbox-exec"

for command in kubectl; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

secret_value="${BEX_SANDBOX_EXEC_SECRET:-}"

if [[ -z "$secret_value" && "${ROTATE:-0}" != "1" ]]; then
  secret_value="$(kubectl -n "$namespace" get secret "$secret_name" \
    -o jsonpath='{.data.secret}' 2>/dev/null | base64 --decode 2>/dev/null || true)"
fi

if [[ -z "$secret_value" ]]; then
  command -v openssl >/dev/null || { echo "missing required command: openssl (needed to generate a key)" >&2; exit 1; }
  secret_value="$(openssl rand -hex 32)"
  echo "generated a new 256-bit sandbox exec key"
fi

apply_secret "$namespace" "$secret_name" Opaque secret "$secret_value"
unset secret_value

# Both processes read BEX_SANDBOX_EXEC_SECRET once at startup, so roll them after
# install/rotation. On first install a Deployment may not exist yet.
for deployment in bex-api bex-ssh-gateway; do
  if kubectl -n "$namespace" get "deployment/$deployment" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout restart "deployment/$deployment" >/dev/null
    kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=180s >/dev/null
  fi
done

echo "installed $secret_name in namespace $namespace; sandbox exec enabled"
