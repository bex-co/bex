#!/usr/bin/env bash
# ADR075 open-signup funnel counters (.pm/w6/024). Activation is a three-step
# funnel (verify email -> bind card -> first deploy) and these three numbers
# are a launch precondition for open signup — no new framework, just a
# scripted read (docs/ADR075-user-onboarding.md, Consequences):
#
#   1. identities unverified >24h        — Kratos admin API, identities whose
#      verifiable_addresses still carry verified=false 24h after creation.
#   2. workspaces unbound >24h           — tenants LEFT JOIN
#      billing_provider_mappings with payment_method_bound_at IS NULL, minus
#      the two operator exemptions that bypass the bind-card gate exactly as
#      store.PaymentEligibility.AllowsPaidIntent does: tenants.billing_comped
#      and tenants.billing_excluded (ADR040 §7 Mode A).
#   3. workspaces at zero resources      — tenants with no apps row and no
#      Database/KeyValue CR labeled bex.co/tenant=<id> (datastores live only
#      as CRs, not in the control-plane DB).
#
# Config mirrors the sibling scripts, no new mechanism:
#   BEX_CP_DB_URI        direct psql connection string; unset -> kubectl exec
#                        into the CNPG primary (ssh-gateway-db-role.sh's modes)
#   BEX_SYSTEM_NAMESPACE (bex-system), BEX_CP_DB_CLUSTER (bex-db),
#   BEX_CP_DB_NAME (bex) CNPG exec-mode coordinates
#   KRATOS_ADMIN_URL     direct Kratos admin origin (dev-env forwards it to
#                        http://localhost:570N0); unset -> kubectl port-forward
#                        service/kratos-admin in $BEX_AUTH_NAMESPACE (auth),
#                        as auth-verify.sh does
#
# The cluster CR read (counter 3) always needs kubectl against the current
# kubeconfig; export KUBECONFIG first (e.g. infra/local/bex.kubeconfig).
set -euo pipefail

namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
cluster="${BEX_CP_DB_CLUSTER:-bex-db}"
database="${BEX_CP_DB_NAME:-bex}"
auth_ns="${BEX_AUTH_NAMESPACE:-auth}"
kratos_local=127.0.0.1:14454 # distinct from auth-verify.sh's 14434 so both can run

fail() { echo "FAIL: $*" >&2; exit 1; }

for cmd in kubectl curl jq; do
  command -v "$cmd" >/dev/null || fail "missing required command: $cmd"
done
if [[ -n "${BEX_CP_DB_URI:-}" ]]; then
  command -v psql >/dev/null || fail "BEX_CP_DB_URI is set but psql is not installed"
fi
kubectl config current-context >/dev/null 2>&1 \
  || fail "no kubectl context — export KUBECONFIG (counter 3 reads Database/KeyValue CRs from the cluster; the DB and Kratos reads also fall back to it unless BEX_CP_DB_URI and KRATOS_ADMIN_URL are set)"

PF_PID=""
cleanup() {
  if [[ -n "$PF_PID" ]]; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true # reap so bash doesn't print "Terminated"
  fi
}
trap cleanup EXIT

# wait_http URL — poll until the endpoint answers 2xx (port-forward warmup).
wait_http() {
  for _ in $(seq 1 30); do
    curl -sf -o /dev/null "$1" && return 0
    sleep 2
  done
  fail "$1 did not become ready"
}

# run_sql — SQL on stdin, unaligned tuple rows on stdout. Same two admin modes
# as ssh-gateway-db-role.sh: a direct URI, else kubectl exec into the CNPG
# primary as the local postgres superuser.
run_sql() {
  if [[ -n "${BEX_CP_DB_URI:-}" ]]; then
    psql "$BEX_CP_DB_URI" -v ON_ERROR_STOP=1 -qAt
  else
    local primary
    primary="$(kubectl -n "$namespace" get pod \
      -l "cnpg.io/cluster=${cluster},cnpg.io/instanceRole=primary" \
      -o jsonpath='{.items[0].metadata.name}')"
    [[ -n "$primary" ]] \
      || fail "no CNPG primary for cluster $cluster in namespace $namespace (set BEX_CP_DB_URI for a direct connection)"
    kubectl -n "$namespace" exec -i "$primary" -- psql -U postgres -d "$database" -v ON_ERROR_STOP=1 -qAt
  fi
}

# --- 1. identities unverified >24h (Kratos admin API) ------------------------
if [[ -z "${KRATOS_ADMIN_URL:-}" ]]; then
  kubectl -n "$auth_ns" port-forward service/kratos-admin "${kratos_local#*:}:80" >/dev/null 2>&1 &
  PF_PID=$!
  KRATOS_ADMIN_URL="http://$kratos_local"
  wait_http "$KRATOS_ADMIN_URL/admin/health/ready"
fi
KRATOS_ADMIN_URL="${KRATOS_ADMIN_URL%/}"

cutoff="$(date -u -v-24H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ)"
unverified=0
headers="$(mktemp)"
url="$KRATOS_ADMIN_URL/admin/identities?page_size=250"
while [[ -n "$url" ]]; do
  body="$(curl -sf -D "$headers" "$url")" || { rm -f "$headers"; fail "Kratos admin list failed: $url"; }
  page_count="$(jq --arg cutoff "$cutoff" \
    '[ .[] | select(.created_at < $cutoff and any(.verifiable_addresses[]?; .verified == false)) ] | length' \
    <<<"$body")" || { rm -f "$headers"; fail "unparseable Kratos identities response from $url"; }
  unverified=$((unverified + page_count))
  # Token pagination (Kratos v1): follow the Link header's rel="next" target.
  next="$(tr -d '\r' <"$headers" | grep -i '^link:' | tr ',' '\n' | grep 'rel="next"' \
    | sed -E 's/.*<([^>]*)>.*/\1/' | head -n1 || true)"
  if [[ -z "$next" ]]; then
    url=""
  elif [[ "$next" == http* ]]; then
    url="$next"
  else
    url="$KRATOS_ADMIN_URL$next"
  fi
done
rm -f "$headers"

# --- 2. workspaces with no payment method bound >24h after creation ----------
unbound="$(run_sql <<'SQL'
SELECT count(*)
FROM tenants t
LEFT JOIN billing_provider_mappings m ON m.workspace_id = t.id
WHERE t.created_at < now() - interval '24 hours'
  AND m.payment_method_bound_at IS NULL
  AND NOT t.billing_comped
  AND NOT t.billing_excluded;
SQL
)"
[[ "$unbound" =~ ^[0-9]+$ ]] || fail "unexpected unbound-workspaces query result: $unbound"

# --- 3. workspaces at zero resources -----------------------------------------
zero_app_tenants="$(run_sql <<'SQL'
SELECT t.id
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM apps a WHERE a.tenant_id = t.id)
ORDER BY t.id;
SQL
)"
datastore_tenants="$(kubectl get databases.app.bex.co,keyvalues.app.bex.co --all-namespaces -o json \
  | jq -r '.items[].metadata.labels["bex.co/tenant"] // empty' | sort -u)"
zero_resources="$(comm -23 <(sort -u <<<"$zero_app_tenants") <(printf '%s\n' "$datastore_tenants") \
  | grep -c . || true)"

echo "== ADR075 open-signup funnel counters =="
echo "identities unverified >24h:                        $unverified"
echo "workspaces unbound >24h (not comped/excluded):     $unbound"
echo "workspaces at zero resources (apps/db/keyvalue):   $zero_resources"
