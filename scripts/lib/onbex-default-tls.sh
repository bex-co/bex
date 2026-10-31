#!/usr/bin/env bash

# Shared certificate policy for the onbex.co fallback leaf that the public edge
# serves. cert-manager issues and renews it (docs/ADR093); this library only
# checks what is observed at the edge.
#
# 14 days matches CertificateExpiringSoon. cert-manager starts renewing at one
# third of the lifetime (about 30 days on a 90-day certificate, 15 on a 45-day
# one), so a healthy edge never drops below this; a 30-day floor would fire
# during every normal renewal.
readonly ONBEX_TLS_MIN_VALIDITY_DAYS=14
readonly ONBEX_TLS_MIN_VALIDITY_SECONDS=$((ONBEX_TLS_MIN_VALIDITY_DAYS * 86400))

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
