#!/usr/bin/env bash
set -euo pipefail

# Provision (idempotently) the least-privilege control-plane Postgres role the
# ssh-gateway connects as (w7/m56, docs/ADR035-ssh.md §116), and publish its
# credential to a Kubernetes Secret separate from bex-api's full-privilege
# bex-db-app. The role's privilege surface is single-sourced from
# lego/backend/internal/sshgateway/dbrole.sql — the SAME grants the CI test
# (dbrole_integration_test.go) applies — so the shipped and tested boundaries
# cannot drift. The generated password is never printed or written to disk.
#
# Two admin-connection modes (pick by env):
#   - BEX_CP_ADMIN_URI set  -> psql connects with that URI directly (local/dev,
#     or any operator with a superuser/CREATEROLE connection string).
#   - otherwise             -> kubectl exec into the CNPG primary and run psql as
#     the local `postgres` superuser (production; no superuser password needed).
#
# The scoped connection string is derived from bex-db-app's own URI with only the
# userinfo swapped, so the gateway keeps byte-identical host/db/TLS parameters and
# only its identity narrows. Set SKIP_SECRET=1 to apply the role DDL without
# touching Kubernetes (what the local smoke test does).

role="${BEX_SSH_GATEWAY_ROLE:-bex_ssh_gateway}"
namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
cluster="${BEX_CP_DB_CLUSTER:-bex-db}"
database="${BEX_CP_DB_NAME:-bex}"
app_secret="${BEX_CP_DB_APP_SECRET:-bex-db-app}"
gateway_secret="${BEX_SSH_GATEWAY_DB_SECRET:-bex-db-ssh-gateway}"
gateway_deploy="${BEX_SSH_GATEWAY_DEPLOYMENT:-bex-ssh-gateway}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
grants_sql="$script_dir/../lego/backend/internal/sshgateway/dbrole.sql"
[[ -f "$grants_sql" ]] || { echo "missing grants DDL: $grants_sql" >&2; exit 1; }

for command in psql openssl; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done
if [[ -z "${BEX_CP_ADMIN_URI:-}" || "${SKIP_SECRET:-}" != "1" ]]; then
  command -v kubectl >/dev/null || { echo "missing required command: kubectl" >&2; exit 1; }
fi

# A strong URL-safe password (no shell/URI-hostile characters).
password="$(openssl rand -base64 30 | tr -dc 'A-Za-z0-9' | head -c 32)"

# The role DDL: idempotent create (preserving no stale password), then the
# single-sourced grants. Reset the password on every run so rotation is a re-run.
role_ddl="$(cat <<SQL
DO \$\$ BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = '${role}') THEN
    EXECUTE format('ALTER ROLE ${role} LOGIN PASSWORD %L', '${password}');
  ELSE
    EXECUTE format('CREATE ROLE ${role} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD %L', '${password}');
  END IF;
END \$\$;
$(sed "s/__ROLE__/${role}/g" "$grants_sql")
SQL
)"

apply_sql() {
  if [[ -n "${BEX_CP_ADMIN_URI:-}" ]]; then
    psql "$BEX_CP_ADMIN_URI" -v ON_ERROR_STOP=1 -q
  else
    local primary
    primary="$(kubectl -n "$namespace" get pod \
      -l "cnpg.io/cluster=${cluster},cnpg.io/instanceRole=primary" \
      -o jsonpath='{.items[0].metadata.name}')"
    [[ -n "$primary" ]] || { echo "no CNPG primary found for cluster $cluster in $namespace" >&2; exit 1; }
    kubectl -n "$namespace" exec -i "$primary" -- psql -U postgres -d "$database" -v ON_ERROR_STOP=1 -q
  fi
}

printf '%s\n' "$role_ddl" | apply_sql
echo "applied least-privilege role '${role}' (create/alter + grants idempotent)"

if [[ "${SKIP_SECRET:-}" == "1" ]]; then
  echo "SKIP_SECRET=1 — role DDL applied; Kubernetes Secret not written"
  exit 0
fi

# Derive the scoped connection string from bex-db-app's own URI (same host/db/TLS,
# only the identity narrows), then publish it as the gateway's own Secret.
app_uri="$(kubectl -n "$namespace" get secret "$app_secret" -o jsonpath='{.data.uri}' | base64 --decode)"
[[ -n "$app_uri" ]] || { echo "could not read ${app_secret}.uri" >&2; exit 1; }
scoped_uri="$(BEX_ROLE="$role" BEX_PW="$password" APP_URI="$app_uri" python3 - <<'PY'
import os, urllib.parse
u = urllib.parse.urlsplit(os.environ["APP_URI"])
netloc = f'{os.environ["BEX_ROLE"]}:{urllib.parse.quote(os.environ["BEX_PW"], safe="")}@{u.hostname}'
if u.port:
    netloc += f":{u.port}"
print(urllib.parse.urlunsplit((u.scheme, netloc, u.path, u.query, u.fragment)))
PY
)"

kubectl -n "$namespace" create secret generic "$gateway_secret" \
  --from-literal=uri="$scoped_uri" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
echo "wrote gateway credential to Secret ${namespace}/${gateway_secret}"

# Roll the gateway so it reconnects under the scoped role (if it is deployed).
# SKIP_ROLLOUT=1 when the deployment's BEX_CP_DB_URI is switched to this Secret by
# a separate GitOps sync (Argo manages the bex-ssh-gateway Deployment): the role
# and Secret must exist BEFORE that sync, and restarting now — while the manifest
# still points at the old Secret — would only bounce live sessions for nothing.
if [[ "${SKIP_ROLLOUT:-}" != "1" ]] && kubectl -n "$namespace" get deployment/"$gateway_deploy" >/dev/null 2>&1; then
  kubectl -n "$namespace" rollout restart deployment/"$gateway_deploy" >/dev/null
  kubectl -n "$namespace" rollout status deployment/"$gateway_deploy" --timeout=180s >/dev/null
  echo "rolled deployment/${gateway_deploy} onto the scoped role"
fi
