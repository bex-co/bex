#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"
cd "$script_dir/.."

# Install the out-of-band Secrets for the platform observability UI
# (docs/ADR087-platform-observability-ui.md, w5/m86):
#
#   bex-system/bex-ops        BEX_OPS_WORKSPACE + BEX_OPS_ROLE_TOKEN — bex-api's
#                             ops-role verb + workspace guards
#   dashboard/bex-ops         BEX_OPS_ROLE_TOKEN — the consent acceptor's bearer
#                             for that verb (same value)
#   monitoring/grafana-oauth  client-secret — the bex-obs Hydra client secret
#                             Grafana authenticates with
#   monitoring/grafana-admin  admin-user + admin-password — break-glass local
#                             admin; OIDC is the normal sign-in path
#
# Reads .env (see .env.example). BEX_OPS_WORKSPACE is REQUIRED (a real tea-*
# workspace id cannot be minted). BEX_OBS_OAUTH_CLIENT_SECRET is required and
# must match what scripts/auth-bootstrap-client.sh provisioned into Hydra.
# BEX_OPS_ROLE_TOKEN and GRAFANA_ADMIN_PASSWORD follow cp-token-secret.sh's
# precedence: environment value → existing in-cluster value (idempotent
# re-runs never rotate) → freshly minted random value. ROTATE=1 forces fresh
# values for both. Secret bytes never reach stdout or any argv
# (scripts/lib/secret-install.sh), and only deployments whose Secret bytes
# actually changed are rolled. DRY_RUN=1 previews without touching the cluster.

system_ns="${BEX_SYSTEM_NAMESPACE:-bex-system}"
dashboard_ns="${BEX_DASHBOARD_NAMESPACE:-dashboard}"
monitoring_ns="${BEX_MONITORING_NAMESPACE:-monitoring}"
env_file="${BEX_OBS_ENV_FILE:-.env}"

if [ -f "$env_file" ]; then
  set -a
  # shellcheck disable=SC1090,SC1091
  source "$env_file"
  set +a
fi

if [ -z "${BEX_OPS_WORKSPACE:-}" ]; then
  echo "error: BEX_OPS_WORKSPACE must be set (the designated tea-* ops workspace id)" >&2
  exit 1
fi
case "$BEX_OPS_WORKSPACE" in
  tea-*) ;;
  *) echo "error: BEX_OPS_WORKSPACE must be a tea-* workspace id, got: $BEX_OPS_WORKSPACE" >&2; exit 1 ;;
esac
if [ -z "${BEX_OBS_OAUTH_CLIENT_SECRET:-}" ]; then
  echo "error: BEX_OBS_OAUTH_CLIENT_SECRET must be set (provision the bex-obs client first: scripts/auth-bootstrap-client.sh)" >&2
  exit 1
fi

if [ "${DRY_RUN:-0}" = "1" ]; then
  echo "would apply Secret $system_ns/bex-ops (keys: BEX_OPS_WORKSPACE BEX_OPS_ROLE_TOKEN)"
  echo "would apply Secret $dashboard_ns/bex-ops (keys: BEX_OPS_ROLE_TOKEN)"
  echo "would apply Secret $monitoring_ns/grafana-oauth (keys: client-secret)"
  echo "would apply Secret $monitoring_ns/grafana-admin (keys: admin-user admin-password)"
  echo "would roll only the consumers (bex-api / dashboard / grafana) whose Secret bytes changed"
  exit 0
fi

command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }

read_existing() {
  kubectl -n "$1" get secret "$2" -o jsonpath="{.data['$3']}" 2>/dev/null | base64 --decode 2>/dev/null || true
}

mint() {
  command -v openssl >/dev/null || { echo "error: openssl is required (needed to generate $1)" >&2; exit 1; }
  openssl rand -hex 32
}

ops_token="${BEX_OPS_ROLE_TOKEN:-}"
if [ -z "$ops_token" ] && [ "${ROTATE:-0}" != "1" ]; then
  ops_token="$(read_existing "$system_ns" bex-ops BEX_OPS_ROLE_TOKEN)"
fi
if [ -z "$ops_token" ]; then
  ops_token="$(mint BEX_OPS_ROLE_TOKEN)"
  echo "generated a new ops-role bearer token"
fi

admin_pw="${GRAFANA_ADMIN_PASSWORD:-}"
if [ -z "$admin_pw" ] && [ "${ROTATE:-0}" != "1" ]; then
  admin_pw="$(read_existing "$monitoring_ns" grafana-admin admin-password)"
fi
if [ -z "$admin_pw" ]; then
  admin_pw="$(mint GRAFANA_ADMIN_PASSWORD)"
  echo "generated a new break-glass Grafana admin password"
fi

secret_hash() {
  # `|| true`: a missing secret (every first install) must hash as empty, not
  # kill the script through pipefail.
  { kubectl -n "$1" get secret "$2" -o json 2>/dev/null || true; } | shasum -a 256 | cut -d' ' -f1
}

ensure_ns() {
  kubectl create namespace "$1" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
}

pre_sys="$(secret_hash "$system_ns" bex-ops)"
pre_dash="$(secret_hash "$dashboard_ns" bex-ops)"
pre_oauth="$(secret_hash "$monitoring_ns" grafana-oauth)"
pre_admin="$(secret_hash "$monitoring_ns" grafana-admin)"

ensure_ns "$system_ns"
ensure_ns "$dashboard_ns"
ensure_ns "$monitoring_ns"

apply_secret "$system_ns" bex-ops Opaque \
  BEX_OPS_WORKSPACE "$BEX_OPS_WORKSPACE" \
  BEX_OPS_ROLE_TOKEN "$ops_token"
apply_secret "$dashboard_ns" bex-ops Opaque \
  BEX_OPS_ROLE_TOKEN "$ops_token"
apply_secret "$monitoring_ns" grafana-oauth Opaque \
  client-secret "$BEX_OBS_OAUTH_CLIENT_SECRET"
apply_secret "$monitoring_ns" grafana-admin Opaque \
  admin-user admin \
  admin-password "$admin_pw"
unset ops_token admin_pw

# Restart only consumers whose Secret bytes actually changed (an idempotent
# re-run must not churn three production pods), and overlap the rollout waits
# so wall time is the slowest rollout, not the sum.
restarts=()
if [ "$(secret_hash "$system_ns" bex-ops)" != "$pre_sys" ]; then
  restarts+=("$system_ns deployment/bex-api")
fi
if [ "$(secret_hash "$dashboard_ns" bex-ops)" != "$pre_dash" ]; then
  restarts+=("$dashboard_ns deployment/dashboard")
fi
if [ "$(secret_hash "$monitoring_ns" grafana-oauth)" != "$pre_oauth" ] ||
  [ "$(secret_hash "$monitoring_ns" grafana-admin)" != "$pre_admin" ]; then
  restarts+=("$monitoring_ns deployment/grafana")
fi

waits=()
for roll in ${restarts[@]+"${restarts[@]}"}; do
  ns="${roll%% *}"; target="${roll#* }"
  kubectl -n "$ns" get "$target" >/dev/null 2>&1 || continue
  kubectl -n "$ns" rollout restart "$target" >/dev/null
  waits+=("$roll")
done
if [ "${#waits[@]}" -eq 0 ]; then
  echo "secrets unchanged (or consumers not deployed); no rollout needed"
fi
for roll in ${waits[@]+"${waits[@]}"}; do
  ns="${roll%% *}"; target="${roll#* }"
  kubectl -n "$ns" rollout status "$target" --timeout=300s >/dev/null
  echo "rolled $ns/$target"
done

echo "installed $system_ns/bex-ops, $dashboard_ns/bex-ops, $monitoring_ns/grafana-oauth, $monitoring_ns/grafana-admin"
echo "(read the break-glass admin password back with: kubectl -n $monitoring_ns get secret grafana-admin -o jsonpath=\"{.data['admin-password']}\" | base64 -d)"
