#!/usr/bin/env bash
# Real-CNPG regression for w4/m179: a deleted Postgres login must stop
# authenticating even when a grant or an owned object references it.
#
# Creates a throwaway CNPG Cluster in its own namespace, grants an extra login
# SELECT on an owner table, then applies the managed-role shape the operator
# projects for a spec.deletedUsers tombstone and checks, with fresh connections:
#   - the retired login is refused, NOLOGIN, with a NULL password
#   - the owner still reads the table and the grant survives
#   - same-name reissue accepts the new password and refuses the old one
#   - a retired role that owns a table keeps the table
# With LEGACY=1 it applies the pre-m179 ensure:absent shape instead and expects
# the bug: CNPG reports cannotReconcile and the login still reads data.
#
# Usage: scripts/pg-role-retirement-check.sh   (KUBECONFIG defaults to the local
# CAPD cluster; needs the CNPG operator). Passwords never reach stdout.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/secret-install.sh
source scripts/lib/secret-install.sh
export KUBECONFIG="${KUBECONFIG:-infra/local/bex.kubeconfig}"
NS="pg-role-retire-$(openssl rand -hex 3)"
trap 'kubectl delete ns "$NS" --wait=false >/dev/null 2>&1 || true' EXIT

# The tombstone projection from managedRoles (database_controller.go).
RETIRED='{"name":"qa_extra","ensure":"present","login":false,"disablePassword":true}'

fail() { echo "FAIL: $*" >&2; exit 1; }
secret() { apply_secret "$NS" "$1" kubernetes.io/basic-auth username qa_extra password "pw-$(openssl rand -hex 12)"; }
password() { kubectl -n "$NS" get secret "$1" -o jsonpath='{.data.password}' | base64 -d; }
sql() { kubectl -n "$NS" exec probe-1 -c postgres -- psql -X -A -t -v ON_ERROR_STOP=1 -d qa_data -c "$1"; }
# login PASSWORD QUERY: last output line of a fresh TCP connection as qa_extra.
login() { kubectl -n "$NS" exec probe-1 -c postgres -- env PGPASSWORD="$1" \
  psql -X -A -t -h probe-rw -U qa_extra -d qa_data -c "$2" 2>&1 | tail -1 || true; }
set_role() {
  kubectl -n "$NS" patch cluster probe --type=json \
    -p="[{\"op\":\"replace\",\"path\":\"/spec/managed/roles/1\",\"value\":$1}]" >/dev/null
}
# await_login PASSWORD WANT(ok|refused): poll up to 60s for a fresh connection.
await_login() {
  for _ in $(seq 1 20); do
    got=$(login "$1" "select 'ok'")
    [ "$2" = ok ] && [ "$got" = ok ] && return 0
    [ "$2" = refused ] && [ "$got" != ok ] && return 0
    sleep 3
  done
  return 1
}

kubectl create ns "$NS" >/dev/null
secret extra-pw
kubectl apply -f - >/dev/null <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: probe, namespace: $NS}
spec:
  instances: 1
  storage: {size: 1Gi}
  bootstrap: {initdb: {database: qa_data, owner: qa_owner}}
  managed:
    roles:
    - {name: qa_owner, ensure: present, login: true}
    - {name: qa_extra, ensure: present, login: true, passwordSecret: {name: extra-pw}}
EOF
echo "waiting for CNPG cluster in $NS..."
kubectl -n "$NS" wait cluster/probe --for=condition=Ready --timeout=300s >/dev/null
OLD=$(password extra-pw)
sql "SET ROLE qa_owner; CREATE TABLE probe(id int PRIMARY KEY, marker text);
     INSERT INTO probe VALUES (15, 'persist'); GRANT SELECT ON probe TO qa_extra;" >/dev/null
await_login "$OLD" ok || fail "baseline login"
[ "$(login "$OLD" 'select marker from probe')" = persist ] || fail "baseline granted read"

if [ "${LEGACY:-}" = 1 ]; then
  set_role '{"name":"qa_extra","ensure":"absent"}'
  for _ in $(seq 1 20); do
    kubectl -n "$NS" get cluster probe -o jsonpath='{.status.managedRolesStatus.cannotReconcile}' | grep -q qa_extra && break
    sleep 3
  done
  kubectl -n "$NS" get cluster probe -o jsonpath='{.status.managedRolesStatus.cannotReconcile}' | grep -q qa_extra ||
    fail "legacy shape: expected CNPG cannotReconcile for qa_extra"
  [ "$(login "$OLD" 'select marker from probe')" = persist ] || fail "legacy shape: login no longer reads"
  echo "LEGACY reproduced: DROP ROLE refused, deleted login still reads the granted table"
  exit 0
fi

set_role "$RETIRED"
await_login "$OLD" refused || fail "retired login still authenticates"
[ "$(sql "select rolcanlogin::text || (rolpassword is null)::text from pg_authid where rolname='qa_extra'")" = falsetrue ] ||
  fail "retired role is not NOLOGIN with a NULL password"
[ "$(sql "select has_table_privilege('qa_extra','probe','SELECT')")" = t ] || fail "grant was dropped"
[ "$(sql "SET ROLE qa_owner; select marker from probe" | tail -1)" = persist ] || fail "owner lost data"
echo "ok: granted login retired, grant and owner data kept"

secret extra-pw2
NEW=$(password extra-pw2)
set_role '{"name":"qa_extra","ensure":"present","login":true,"passwordSecret":{"name":"extra-pw2"}}'
await_login "$NEW" ok || fail "reissued password refused"
[ "$(login "$OLD" "select 'ok'")" != ok ] || fail "prior password accepted after reissue"
echo "ok: same-name reissue accepts the new password, refuses the old"

sql "GRANT CREATE ON SCHEMA public TO qa_extra" >/dev/null
[ "$(login "$NEW" "create table extra_owned(x int); insert into extra_owned values (7); select 'ok'")" = ok ] ||
  fail "could not create owned table"
set_role "$RETIRED"
await_login "$NEW" refused || fail "retired owner-of-objects login still authenticates"
[ "$(sql "select tableowner || ':' || (select x from extra_owned) from pg_tables where tablename='extra_owned'")" = qa_extra:7 ] ||
  fail "owned table lost"
echo "ok: retired role that owns a table keeps the table"
echo PASS
