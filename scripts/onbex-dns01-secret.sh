#!/usr/bin/env bash
# Install or rotate cert-manager's zone-scoped Cloudflare token without putting
# the credential in Git, argv, or logs. Invoked only by deploy.yml's protected
# production-deploy job after the app-cluster kubeconfig is available.
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

token="${BEX_ONBEX_DNS_API_TOKEN:-}"
[ -n "$token" ] || {
  echo "error: BEX_ONBEX_DNS_API_TOKEN is required in the production-deploy environment" >&2
  exit 1
}

kubectl create namespace traefik --dry-run=client -o yaml | kubectl apply -f -
apply_secret traefik onbex-dns01-cloudflare Opaque api-token "$token"
unset token
