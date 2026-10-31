#!/usr/bin/env bash
# Install the Cloudflare API token that cert-manager's onbex-dns01 Issuer uses
# to renew the *.onbex.co fallback wildcard over DNS-01 (docs/ADR093). The
# token is scoped to the onbex.co zone (DNS Edit + Zone Read). It reaches the
# cluster only over stdin, never argv or logs.
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

validate_only=false
case "${1:-}" in
  --validate-only) validate_only=true ;;
  '') ;;
  *)
    echo "usage: $0 [--validate-only]" >&2
    exit 2
    ;;
esac

token="${BEX_CLOUDFLARE_DNS_TOKEN:-}"
[ -n "$token" ] || {
  echo "error: BEX_CLOUDFLARE_DNS_TOKEN is required in the production-deploy environment" >&2
  exit 1
}
# Cloudflare tokens are URL-safe base64-ish strings. Refuse anything with
# whitespace or quoting so a pasted line break cannot silently break renewal.
[[ "$token" =~ ^[A-Za-z0-9_-]{32,}$ ]] || {
  echo "error: BEX_CLOUDFLARE_DNS_TOKEN is not a well-formed Cloudflare API token" >&2
  exit 1
}
if "$validate_only"; then
  echo "BEX_CLOUDFLARE_DNS_TOKEN is present and well-formed"
  exit 0
fi
command -v kubectl >/dev/null || {
  echo "error: missing required command: kubectl" >&2
  exit 1
}

kubectl create namespace traefik --dry-run=client -o yaml | kubectl apply -f - >/dev/null
apply_secret traefik cloudflare-dns-token Opaque api-token "$token"
echo "installed traefik/cloudflare-dns-token for the onbex-dns01 Issuer"
