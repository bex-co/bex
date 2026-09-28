#!/usr/bin/env bash
# Bootstrap Grafana's scoped analytics reader on the selected CNPG cluster. Reuses its
# existing password; credentials travel over stdin and never appear in argv.
#
# Custody contract (w7/m153): a Secret owned by a SealedSecret is authoritative and
# is never rewritten here — the role password follows it, and a drifted CNPG CA fails
# closed until resealed. An unowned Secret is applied and marked adoptable by the
# committed SealedSecret. SEAL_TO=<file> writes the desired Secret, sealed, to that
# manifest (requires kubeseal) so Git custody can be created or refreshed.
set -euo pipefail
set +x
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

profile="${1:-}"
case "$profile" in cli|product) ;; *) echo "usage: analytics-bootstrap.sh cli|product" >&2; exit 2 ;; esac

system_ns="${BEX_SYSTEM_NAMESPACE:-bex-system}"
monitoring_ns="${BEX_MONITORING_NAMESPACE:-monitoring}"
secret="grafana-${profile}-analytics"
seal_to="${SEAL_TO:-}"
if [ "${DRY_RUN:-0}" = "1" ]; then
  echo "would provision ${profile}_analytics.events and restricted bex_${profile}_analytics role in $system_ns/bex-db"
  echo "would reuse or mint $monitoring_ns/$secret (password + CNPG CA); a SealedSecret-owned Secret is left unchanged"
  [ -z "$seal_to" ] || echo "would seal $monitoring_ns/$secret into $seal_to"
  exit 0
fi
tools=(kubectl openssl)
[ -z "$seal_to" ] || tools+=(kubeseal)
for tool in "${tools[@]}"; do
  command -v "$tool" >/dev/null || { echo "error: $tool is required" >&2; exit 1; }
done

primary="$(kubectl -n "$system_ns" get clusters.postgresql.cnpg.io bex-db -o jsonpath='{.status.currentPrimary}')"
[ -n "$primary" ] || { echo "error: bex-db has no primary" >&2; exit 1; }
sealed_owner="$(kubectl -n "$monitoring_ns" get secret "$secret" --ignore-not-found -o jsonpath='{.metadata.ownerReferences[?(@.kind=="SealedSecret")].name}')"
existing="$(kubectl -n "$monitoring_ns" get secret "$secret" --ignore-not-found -o jsonpath='{.data.password}')"
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
if [ -n "$sealed_owner" ] && [ -z "$seal_to" ]; then
  sealed_ca="$(kubectl -n "$monitoring_ns" get secret "$secret" -o jsonpath='{.data.ca\.crt}' | base64 --decode)"
  if [ "$sealed_ca" != "$ca" ]; then
    echo "error: $monitoring_ns/$secret is SealedSecret-managed and its ca.crt no longer matches bex-db-ca;" >&2
    echo "       rerun with SEAL_TO=<its committed sealedsecret.yaml> and ship the reseal" >&2
    exit 1
  fi
  unset sealed_ca
fi

kubectl -n "$system_ns" exec -i "$primary" -c postgres -- \
  psql -X -q -v ON_ERROR_STOP=1 -U postgres -d bex < "$script_dir/${profile}-analytics.sql"
{
  printf "\\set analytics_password '%s'\n" "$password"
  printf '%s\n' "ALTER ROLE bex_${profile}_analytics LOGIN PASSWORD :'analytics_password';"
} | kubectl -n "$system_ns" exec -i "$primary" -c postgres -- \
  psql -X -q -v ON_ERROR_STOP=1 -U postgres -d bex

if [ -n "$sealed_owner" ]; then
  echo "kept SealedSecret-managed $monitoring_ns/$secret unchanged"
else
  apply_secret "$monitoring_ns" "$secret" Opaque password "$password" ca.crt "$ca"
  # Lets the committed SealedSecret adopt this Secret instead of failing its sync.
  kubectl -n "$monitoring_ns" annotate --overwrite secret "$secret" sealedsecrets.bitnami.com/managed=true >/dev/null
fi
if [ -n "$seal_to" ]; then
  secret_manifest "$monitoring_ns" "$secret" Opaque password "$password" ca.crt "$ca" |
    kubeseal --controller-namespace kube-system --controller-name sealed-secrets --format yaml >"$seal_to"
  echo "sealed $monitoring_ns/$secret into $seal_to; ship it so the controller applies it"
fi
unset password ca existing
echo "provisioned ${profile}_analytics.events and $monitoring_ns/$secret"
echo "Grafana reads the Secret at startup: roll monitoring/deployment/grafana after its data changes."
