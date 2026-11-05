#!/usr/bin/env bash
# Real-CNPG check for w5/m265: Render's credential rotation.
#
# Creates a throwaway CNPG Cluster in its own namespace, lets the owner create a
# table, then applies the managed-role shapes the operator projects
# (managedRoles in database_controller.go) and checks, with fresh connections:
#   - a credential with ownerPrivileges (inRoles: [owner]) reads and writes
#     the owner's existing table and can create new objects
#   - retiring the original owner (login:false, NULL password) refuses the
#     owner's password while the new default keeps working and the owner still
#     owns its objects
#   - a second rotation: the next credential joins the owner, the retired
#     owner and the earlier credential, so after that credential is retired
#     it still reaches the table the earlier one created
#
# Usage: scripts/pg-credential-rotation-check.sh   (KUBECONFIG defaults to the
# local CAPD cluster; needs the CNPG operator). Passwords never reach stdout.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/secret-install.sh
source scripts/lib/secret-install.sh
export KUBECONFIG="${KUBECONFIG:-infra/local/bex.kubeconfig}"
NS="pg-cred-rotate-$(openssl rand -hex 3)"
trap 'kubectl delete ns "$NS" --wait=false >/dev/null 2>&1 || true' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
secret() { apply_secret "$NS" "$1" kubernetes.io/basic-auth username "$2" password "pw-$(openssl rand -hex 12)"; }
password() { kubectl -n "$NS" get secret "$1" -o jsonpath='{.data.password}' | base64 -d; }
sql() { kubectl -n "$NS" exec probe-1 -c postgres -- psql -X -A -t -v ON_ERROR_STOP=1 -d qa_data -c "$1"; }
# login USER PASSWORD QUERY: last output line of a fresh TCP connection.
login() { kubectl -n "$NS" exec probe-1 -c postgres -- env PGPASSWORD="$2" \
  psql -X -A -t -h probe-rw -U "$1" -d qa_data -c "$3" 2>&1 | tail -1 || true; }
roles() {
  kubectl -n "$NS" patch cluster probe --type=json \
    -p="[{\"op\":\"replace\",\"path\":\"/spec/managed/roles\",\"value\":$1}]" >/dev/null
}
# await USER PASSWORD WANT(ok|refused): poll up to 60s for a fresh connection.
await() {
  for _ in $(seq 1 20); do
    got=$(login "$1" "$2" "select 'ok'")
    [ "$3" = ok ] && [ "$got" = ok ] && return 0
    [ "$3" = refused ] && [ "$got" != ok ] && return 0
    sleep 3
  done
  return 1
}

kubectl create ns "$NS" >/dev/null
secret owner-pw qa_owner
secret rotated-pw rotated
secret third-pw third
kubectl apply -f - >/dev/null <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: probe, namespace: $NS}
spec:
  instances: 1
  storage: {size: 1Gi}
  bootstrap: {initdb: {database: qa_data, owner: qa_owner, secret: {name: owner-pw}}}
  managed:
    roles:
    - {name: qa_owner, ensure: present, login: true, inRoles: [pg_monitor]}
EOF
echo "waiting for CNPG cluster in $NS..."
kubectl -n "$NS" wait cluster/probe --for=condition=Ready --timeout=300s >/dev/null
OWNER=$(password owner-pw)
ROTATED=$(password rotated-pw)
await qa_owner "$OWNER" ok || fail "owner baseline login"
[ "$(login qa_owner "$OWNER" "create table probe(id int primary key, marker text); insert into probe values (1, 'persist'); select 'ok'")" = ok ] ||
  fail "owner could not create its table"

# A Render credential: a login that is a member of the owner role.
roles '[{"name":"qa_owner","ensure":"present","login":true,"inRoles":["pg_monitor"]},
        {"name":"rotated","ensure":"present","login":true,"passwordSecret":{"name":"rotated-pw"},"inRoles":["qa_owner"]}]'
await rotated "$ROTATED" ok || fail "new default credential cannot log in"
[ "$(login rotated "$ROTATED" "select marker from probe")" = persist ] || fail "new default cannot read the owner's table"
[ "$(login rotated "$ROTATED" "insert into probe values (2, 'rotated'); create table after_rotation(x int); select 'ok'")" = ok ] ||
  fail "new default cannot write or create objects"
await qa_owner "$OWNER" ok || fail "owner stopped working before it was deleted"
echo "ok: the new default inherits the owner's privileges; both credentials work"

# Render "deletes" the original user by revoking its login.
roles '[{"name":"qa_owner","ensure":"present","login":false,"disablePassword":true,"inRoles":["pg_monitor"]},
        {"name":"rotated","ensure":"present","login":true,"passwordSecret":{"name":"rotated-pw"},"inRoles":["qa_owner"]}]'
await qa_owner "$OWNER" refused || fail "retired owner still authenticates"
[ "$(login rotated "$ROTATED" "select count(*) from probe")" = 2 ] || fail "new default lost access after owner retirement"
[ "$(sql "select tableowner from pg_tables where tablename='probe'")" = qa_owner ] || fail "owner lost its table"
[ "$(sql "select rolcanlogin::text from pg_roles where rolname='qa_owner'")" = false ] || fail "owner is not NOLOGIN"
echo "ok: the retired owner is refused, keeps its objects, and the new default keeps working"

# Second rotation, then retire the credential that created after_rotation.
THIRD=$(password third-pw)
roles '[{"name":"qa_owner","ensure":"present","login":false,"disablePassword":true,"inRoles":["pg_monitor"]},
        {"name":"rotated","ensure":"present","login":true,"passwordSecret":{"name":"rotated-pw"},"inRoles":["qa_owner"]},
        {"name":"third","ensure":"present","login":true,"passwordSecret":{"name":"third-pw"},"inRoles":["qa_owner","rotated"]}]'
await third "$THIRD" ok || fail "third credential cannot log in"
roles '[{"name":"qa_owner","ensure":"present","login":false,"disablePassword":true,"inRoles":["pg_monitor"]},
        {"name":"third","ensure":"present","login":true,"passwordSecret":{"name":"third-pw"},"inRoles":["qa_owner","rotated"]},
        {"name":"rotated","ensure":"present","login":false,"disablePassword":true}]'
await rotated "$ROTATED" refused || fail "retired second credential still authenticates"
[ "$(login third "$THIRD" "insert into after_rotation values (3); select count(*) from after_rotation")" = 1 ] ||
  fail "third credential cannot reach the table the retired credential created"
[ "$(login third "$THIRD" "select count(*) from probe")" = 2 ] || fail "third credential cannot reach the owner's table"
echo "ok: after a second rotation and retiring the earlier credential, its tables stay reachable"
echo PASS
