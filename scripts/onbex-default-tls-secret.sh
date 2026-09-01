#!/usr/bin/env bash
# Validate and install the externally issued onbex.co wildcard certificate.
# Certificate acquisition stays outside bex; this script never calls DNS APIs.
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/onbex-default-tls.sh
. "$script_dir/lib/onbex-default-tls.sh"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

certificate="${BEX_ONBEX_TLS_CERT:-}"
private_key="${BEX_ONBEX_TLS_KEY:-}"
validate_only=false
case "${1:-}" in
  --validate-only) validate_only=true ;;
  '') ;;
  *)
    echo "usage: $0 [--validate-only]" >&2
    exit 2
    ;;
esac

[ -n "$certificate" ] || {
  echo "error: BEX_ONBEX_TLS_CERT is required in the production-deploy environment" >&2
  exit 1
}
[ -n "$private_key" ] || {
  echo "error: BEX_ONBEX_TLS_KEY is required in the production-deploy environment" >&2
  exit 1
}
command -v openssl >/dev/null || {
  echo "error: missing required command: openssl" >&2
  exit 1
}

certificate_details="$(onbex_tls_certificate_details "$certificate")" || {
  echo "error: BEX_ONBEX_TLS_CERT is not a valid PEM certificate" >&2
  exit 1
}
onbex_tls_has_wildcard_san "$certificate_details" || {
  echo "error: BEX_ONBEX_TLS_CERT must contain the exact DNS SAN *.onbex.co" >&2
  exit 1
}
onbex_tls_valid_for_minimum_window "$certificate" || {
  echo "error: BEX_ONBEX_TLS_CERT expires inside the 30-day minimum validity window" >&2
  exit 1
}
onbex_tls_chain_is_trusted "$certificate" || {
  echo "error: BEX_ONBEX_TLS_CERT does not contain a chain trusted by the runner" >&2
  exit 1
}
printf '%s' "$private_key" | openssl pkey -noout >/dev/null 2>&1 || {
  echo "error: BEX_ONBEX_TLS_KEY is not a valid unencrypted PEM private key" >&2
  exit 1
}

certificate_key_digest="$(
  printf '%s' "$certificate" |
    openssl x509 -pubkey -noout 2>/dev/null |
    openssl pkey -pubin -outform DER 2>/dev/null |
    openssl dgst -sha256
)"
private_key_digest="$(
  printf '%s' "$private_key" |
    openssl pkey -pubout -outform DER 2>/dev/null |
    openssl dgst -sha256
)"
[ "$certificate_key_digest" = "$private_key_digest" ] || {
  echo "error: BEX_ONBEX_TLS_CERT and BEX_ONBEX_TLS_KEY do not contain the same public key" >&2
  exit 1
}

if "$validate_only"; then
  echo "onbex.co fallback TLS material passed certificate, trust, expiry, and key checks"
  exit 0
fi
command -v kubectl >/dev/null || {
  echo "error: missing required command: kubectl" >&2
  exit 1
}
kubectl create namespace traefik --dry-run=client -o yaml | kubectl apply -f - >/dev/null
apply_secret traefik onbex-default-wildcard-tls kubernetes.io/tls \
  tls.crt "$certificate" tls.key "$private_key"
