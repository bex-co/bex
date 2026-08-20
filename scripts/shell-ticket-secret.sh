#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

# Install the shared Browser Web Shell ticket secret (docs/ADR035-ssh.md § Browser
# Web Shell) without ever rendering its bytes to stdout or checking them into Git.
#
# This HMAC-SHA256 key is the single activation gate for the in-dashboard terminal:
# bex-api signs a short-lived exec ticket with it and the isolated ssh-gateway
# verifies the signature — both processes read the SAME value from the
# bex-shell-ticket Secret (key `secret`). While the Secret is absent both sides
# stay disabled (the ticket verb returns 503 "web shell transport not configured"
# and the gateway never starts its WebSocket listener); native `ssh` is unaffected.
#
# Fail-closed activation: install this only AFTER the ssh.bex.co /shell edge route,
# its TLS cert, and the 2/2 gateway are verified reachable, so a minted ticket can
# always be redeemed.
#
# Value precedence:
#   1. $BEX_SHELL_TICKET_SECRET from the environment (e.g. sourced from .env) — use
#      the same value bex-api and the gateway will read.
#   2. An existing in-cluster bex-shell-ticket Secret — reused as-is (idempotent
#      re-runs never rotate it and never invalidate live sessions).
#   3. A freshly generated 256-bit random key (first install).
# Set ROTATE=1 to force a new random key even when one already exists (invalidates
# any in-flight tickets; live sessions the gateway already accepted are unaffected).

namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="bex-shell-ticket"

for command in kubectl; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

secret_value="${BEX_SHELL_TICKET_SECRET:-}"

if [[ -z "$secret_value" && "${ROTATE:-0}" != "1" ]]; then
  # Reuse the existing key so re-runs are idempotent and don't break sessions.
  secret_value="$(kubectl -n "$namespace" get secret "$secret_name" \
    -o jsonpath='{.data.secret}' 2>/dev/null | base64 --decode 2>/dev/null || true)"
fi

if [[ -z "$secret_value" ]]; then
  command -v openssl >/dev/null || { echo "missing required command: openssl (needed to generate a key)" >&2; exit 1; }
  secret_value="$(openssl rand -hex 32)"
  echo "generated a new 256-bit web shell ticket key"
fi

apply_secret "$namespace" "$secret_name" Opaque secret "$secret_value"
unset secret_value

# Both processes read BEX_SHELL_TICKET_SECRET once at startup, so roll them after
# install/rotation. On first install a Deployment may not exist yet.
for deployment in bex-api bex-ssh-gateway; do
  if kubectl -n "$namespace" get "deployment/$deployment" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout restart "deployment/$deployment" >/dev/null
    kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=180s >/dev/null
  fi
done

echo "installed $secret_name in namespace $namespace; browser web shell enabled"
