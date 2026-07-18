#!/usr/bin/env bash
# Reconcile the DNS-only wildcard records for the shared PostgreSQL and Valkey
# raw-TCP edge. App/API/custom-domain records remain Cloudflare-proxied.
set -euo pipefail

mode="${1:-}"
if [[ -n "$mode" && "$mode" != "--check" ]]; then
  echo "usage: $0 [--check]" >&2
  exit 2
fi

: "${BEX_DB_DOMAIN:?set the public managed-Postgres domain}"
: "${BEX_KV_DOMAIN:?set the public managed-Key-Value domain}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
reconciler="${BEX_EDGE_DNS_RECONCILER:-$repo_root/scripts/ssh-dns-cloudflare.sh}"
[[ -x "$reconciler" ]] || { echo "raw-edge DNS reconciler is not executable" >&2; exit 1; }

reconcile_domain() {
  local label="$1" domain="$2" zone
  zone="${BEX_DATASTORE_DNS_ZONE:-${domain#*.}}"
  BEX_EDGE_DNS_HOST="*.$domain" \
    BEX_EDGE_DNS_ZONE="$zone" \
    BEX_EDGE_DNS_LABEL="$label datastore" \
    bash "$reconciler" "$mode"
}

reconcile_domain Postgres "$BEX_DB_DOMAIN"
reconcile_domain 'Key Value' "$BEX_KV_DOMAIN"

