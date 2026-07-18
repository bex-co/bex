#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
wrapper="$repo_root/scripts/datastore-dns-cloudflare.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fake="$tmp/reconcile"
trace="$tmp/trace"

printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'printf "%s|%s|%s|%s\n" "$BEX_EDGE_DNS_HOST" "$BEX_EDGE_DNS_ZONE" "$BEX_EDGE_DNS_LABEL" "${1:-}" >>"$BEX_EDGE_TEST_TRACE"' \
  >"$fake"
chmod +x "$fake"

export BEX_EDGE_DNS_RECONCILER="$fake"
export BEX_EDGE_TEST_TRACE="$trace"
export BEX_DB_DOMAIN=db.example.test
export BEX_KV_DOMAIN=kv.example.test

bash "$wrapper" --check
grep -Fxq '*.db.example.test|example.test|Postgres datastore|--check' "$trace"
grep -Fxq '*.kv.example.test|example.test|Key Value datastore|--check' "$trace"
[[ "$(wc -l <"$trace" | tr -d ' ')" == "2" ]]

: >"$trace"
set +e
missing_output="$(env -u BEX_KV_DOMAIN bash "$wrapper" --check 2>&1)"
missing_status=$?
set -e
[[ "$missing_status" != 0 && "$missing_output" == *"BEX_KV_DOMAIN"* && ! -s "$trace" ]]

echo 'PASS datastore Cloudflare DNS exact-wildcard dispatch'

