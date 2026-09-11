#!/usr/bin/env bash
# Bootstrap Grafana's scoped analytics reader on the selected CNPG cluster. Reuses its
# existing password; credentials travel over stdin and never appear in argv.
set -euo pipefail
set +x
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

profile="${1:-}"
case "$profile" in cli|product) ;; *) echo "usage: analytics-bootstrap.sh cli|product" >&2; exit 2 ;; esac

system_ns="${BEX_SYSTEM_NAMESPACE:-bex-system}"
monitoring_ns="${BEX_MONITORING_NAMESPACE:-monitoring}"
if [ "${DRY_RUN:-0}" = "1" ]; then
  echo "would provision ${profile}_analytics.events and restricted bex_${profile}_analytics role in $system_ns/bex-db"
  echo "would reuse or mint $monitoring_ns/grafana-${profile}-analytics (password + CNPG CA)"
  exit 0
fi
for tool in kubectl openssl; do
  command -v "$tool" >/dev/null || { echo "error: $tool is required" >&2; exit 1; }
done

primary="$(kubectl -n "$system_ns" get clusters.postgresql.cnpg.io bex-db -o jsonpath='{.status.currentPrimary}')"
[ -n "$primary" ] || { echo "error: bex-db has no primary" >&2; exit 1; }
existing="$(kubectl -n "$monitoring_ns" get secret grafana-${profile}-analytics --ignore-not-found -o jsonpath='{.data.password}')"
password="$(printf '%s' "$existing" | base64 --decode)"
if [ -z "$password" ]; then password="$(openssl rand -hex 32)"; fi
# This Secret belongs to this bootstrap and only contains its hex password.
# Validation also keeps the psql variable assignment below unambiguous.
if [[ ! "$password" =~ ^[a-f0-9]{64}$ ]]; then
  echo "error: existing analytics password is not in the bootstrap's expected format" >&2
  exit 1
fi
ca="$(kubectl -n "$system_ns" get secret bex-db-ca -o jsonpath='{.data.ca\.crt}' | base64 --decode)"
[ -n "$ca" ] || { echo "error: bex-db CA is missing" >&2; exit 1; }

kubectl -n "$system_ns" exec -i "$primary" -c postgres -- \
  psql -X -q -v ON_ERROR_STOP=1 -U postgres -d bex < "$script_dir/${profile}-analytics.sql"
{
  printf "\\set analytics_password '%s'\n" "$password"
  printf '%s\n' "ALTER ROLE bex_${profile}_analytics LOGIN PASSWORD :'analytics_password';"
} | kubectl -n "$system_ns" exec -i "$primary" -c postgres -- \
  psql -X -q -v ON_ERROR_STOP=1 -U postgres -d bex

apply_secret "$monitoring_ns" grafana-${profile}-analytics Opaque password "$password" ca.crt "$ca"
unset password ca existing
echo "provisioned ${profile}_analytics.events and $monitoring_ns/grafana-${profile}-analytics"
echo "Grafana picks up the Secret on its next rollout; keep encrypted custody with scripts/seal-secret.sh."
