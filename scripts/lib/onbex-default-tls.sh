#!/usr/bin/env bash

# Shared certificate policy for the externally issued onbex.co fallback leaf.
# Acquisition and rotation stay outside bex; this library only validates the
# certificate that bex installs or observes at the public edge.
readonly ONBEX_TLS_MIN_VALIDITY_SECONDS=2592000

onbex_tls_certificate_details() {
  printf '%s' "$1" | openssl x509 \
    -noout -subject -issuer -dates -ext subjectAltName 2>/dev/null
}

onbex_tls_has_wildcard_san() {
  grep -Eq 'DNS:\*\.onbex\.co([,[:space:]]|$)' <<<"$1"
}

onbex_tls_valid_for_minimum_window() {
  printf '%s' "$1" | openssl x509 \
    -checkend "$ONBEX_TLS_MIN_VALIDITY_SECONDS" -noout >/dev/null 2>&1
}

onbex_tls_chain_is_trusted() {
  local certificate="$1" tmp leaf chain status=0
  tmp="$(mktemp -d)" || return 1
  leaf="$tmp/leaf.pem"
  chain="$tmp/chain.pem"

  awk '
    /-----BEGIN CERTIFICATE-----/ { count++ }
    count == 1 { print }
    /-----END CERTIFICATE-----/ && count == 1 { exit }
  ' <<<"$certificate" >"$leaf"
  awk '
    /-----BEGIN CERTIFICATE-----/ { count++ }
    count >= 2 { print }
  ' <<<"$certificate" >"$chain"

  if [ -s "$chain" ]; then
    openssl verify -purpose sslserver -untrusted "$chain" "$leaf" >/dev/null 2>&1 || status=$?
  else
    openssl verify -purpose sslserver "$leaf" >/dev/null 2>&1 || status=$?
  fi
  rm -rf -- "$tmp"
  return "$status"
}
