#!/usr/bin/env bash
# Create the Kubernetes Secret bex-api consumes for the GitHub App integration
# (docs/github-integration.md), out-of-band of GitOps (no secret material in git
# or Argo-managed manifests — repo rule; the private key is a credential).
#
# Reads the repo-local .env (gitignored — never commit or print it). Keys:
#   BEX_GITHUB_APP_ID           numeric app id (General → App ID)
#   BEX_GITHUB_APP_PRIVATE_KEY  the app's RSA private key, PEM (multi-line; the
#                               whole .pem GitHub generated)
#   BEX_GITHUB_APP_SLUG         the app slug (e.g. bex-co)
#   BEX_GITHUB_WEBHOOK_SECRET   the app's webhook HMAC secret (optional but needed
#                               for zero-config push-to-deploy)
#
# The Secret's key names (app-id / private-key / slug / webhook-secret) are what
# lego/operator/config/api/deployment.yaml references via secretKeyRef.
#
# Usage: scripts/github-app-secret.sh             # create/update the Secret (idempotent)
#        DRY_RUN=1 scripts/github-app-secret.sh   # print what would be applied (names only)
# Requires: kubectl (respects $KUBECONFIG).
set -euo pipefail
cd "$(dirname "$0")/.."

NS=bex-system

# Load .env when present (local use); in CI the keys arrive as environment
# variables instead.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

# require NAME LEN — assert the .env key exists and is at least LEN chars. Never
# prints the value.
require() {
  local name="$1" len="$2" val="${!1:-}"
  [ -n "$val" ] || { echo "error: $name is missing or empty (.env or environment)" >&2; exit 1; }
  [ "${#val}" -ge "$len" ] || { echo "error: $name must be at least $len characters (got ${#val})" >&2; exit 1; }
}

require BEX_GITHUB_APP_ID 1
require BEX_GITHUB_APP_PRIVATE_KEY 100 # a PEM is always well over 100 chars
require BEX_GITHUB_APP_SLUG 1

# The webhook secret is optional (private-repo deploys work without it; only
# zero-config push-to-deploy needs it), so default it to empty rather than fail.
WEBHOOK_SECRET="${BEX_GITHUB_WEBHOOK_SECRET:-}"

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would ensure namespace $NS"
  echo "would apply secret $NS/bex-github-app (keys: app-id private-key slug webhook-secret)"
  exit 0
fi

kubectl get namespace "$NS" >/dev/null 2>&1 || kubectl create namespace "$NS" >/dev/null

kubectl create secret generic bex-github-app -n "$NS" \
  --from-literal=app-id="$BEX_GITHUB_APP_ID" \
  --from-literal=private-key="$BEX_GITHUB_APP_PRIVATE_KEY" \
  --from-literal=slug="$BEX_GITHUB_APP_SLUG" \
  --from-literal=webhook-secret="$WEBHOOK_SECRET" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "applied secret $NS/bex-github-app"
