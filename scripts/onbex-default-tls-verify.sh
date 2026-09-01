#!/usr/bin/env bash
# Public synthetic for Traefik's unknown-host fallback certificate and 404.
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/onbex-default-tls.sh
. "$script_dir/lib/onbex-default-tls.sh"

host="${1:-notfound.onbex.co}"
[[ "$host" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?\.onbex\.co$ ]] || {
  echo "error: expected a first-level *.onbex.co hostname, got: $host" >&2
  exit 2
}

for command in curl openssl; do
  command -v "$command" >/dev/null || {
    echo "error: missing required command: $command" >&2
    exit 1
  }
done

tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
if ! openssl s_client -connect "$host:443" -servername "$host" \
  -verify_return_error -showcerts </dev/null >"$tmp/handshake" 2>"$tmp/handshake.err"; then
  echo "error: $host did not complete a publicly trusted TLS handshake" >&2
  sed -n '1,20p' "$tmp/handshake.err" >&2
  exit 1
fi
awk '/BEGIN CERTIFICATE/{show=1} show{print} /END CERTIFICATE/{exit}' \
  "$tmp/handshake" >"$tmp/leaf.crt"
openssl x509 -in "$tmp/leaf.crt" -noout >/dev/null 2>&1 || {
  echo "error: $host returned no parseable leaf certificate" >&2
  exit 1
}
openssl x509 -in "$tmp/leaf.crt" -checkhost "$host" -noout >/dev/null 2>&1 || {
  echo "error: the fallback certificate does not cover $host" >&2
  exit 1
}
leaf_certificate="$(<"$tmp/leaf.crt")"
certificate_details="$(onbex_tls_certificate_details "$leaf_certificate")"
onbex_tls_has_wildcard_san "$certificate_details" || {
  echo "error: $host is trusted but the fallback leaf lacks the exact *.onbex.co SAN" >&2
  exit 1
}
onbex_tls_valid_for_minimum_window "$leaf_certificate" || {
  echo "error: the fallback certificate expires inside 30 days" >&2
  exit 1
}

status="$(curl --silent --show-error --output "$tmp/body" --write-out '%{http_code}' \
  --max-time 20 "https://$host/")"
[ "$status" = 404 ] || {
  echo "error: https://$host/ returned HTTP $status, expected the intentional 404" >&2
  exit 1
}

printf '%s\n' "$certificate_details"
echo "HTTP 404: https://$host/"
