#!/usr/bin/env bash
# Provision/verify/clean the paid · excluded · comp m53 workspace set.
# This is production infrastructure with Stripe TEST objects only.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib/stripe-billing.sh
source scripts/lib/stripe-billing.sh

action="${1:-}"
case "$action" in plan|apply|verify|cleanup) ;; *) echo "usage: $0 plan|apply|verify|cleanup" >&2; exit 2 ;; esac

bex_stripe_require_test_key "${BEX_STRIPE_SECRET_KEY:-}"
[ -n "${BEX_CP_TOKEN:-}" ] || { echo "error: BEX_CP_TOKEN is required" >&2; exit 2; }
[ -n "${BEX_BILLING_FIXTURE_DB_URI:-}" ] || { echo "error: BEX_BILLING_FIXTURE_DB_URI is required" >&2; exit 2; }

cp_url="${BEX_CP_URL:-http://127.0.0.1:8091}"
state_file="${BEX_BILLING_FIXTURE_STATE:-infra/local/stripe-billing-m53-fixtures.json}"
actor="${BEX_BILLING_FIXTURE_ACTOR:-stripe-m53-test-operator}"

tmp_dir="$(mktemp -d)"
auth_config="$tmp_dir/curl-auth"
pg_service="$tmp_dir/pg_service.conf"
cleanup_tmp() {
  unlink "$auth_config" "$pg_service" 2>/dev/null || true
  find "$tmp_dir" -type f -delete
  rmdir "$tmp_dir" 2>/dev/null || true
}
trap cleanup_tmp EXIT
umask 077
printf 'header = "Authorization: Bearer %s"\n' "$BEX_CP_TOKEN" >"$auth_config"
printf '[bex_fixture]\nconninfo=%s\n' "$BEX_BILLING_FIXTURE_DB_URI" >"$pg_service"
export PGSERVICEFILE="$pg_service" STRIPE_API_KEY="$BEX_STRIPE_SECRET_KEY"
unset BEX_CP_TOKEN BEX_BILLING_FIXTURE_DB_URI BEX_STRIPE_SECRET_KEY

cp_curl() { curl -fsS --config "$auth_config" "$@"; }
stripe_json() {
  local output
  output="$(stripe "$@" --color off)" || { echo "error: Stripe test-mode request failed" >&2; return 1; }
  [ "$(printf '%s' "$output" | jq -r '.livemode // false')" = false ] || { echo "error: live Stripe object encountered" >&2; return 1; }
  [ "$(printf '%s' "$output" | jq -r 'has("error")')" = false ] || { echo "error: Stripe test-mode request returned an error" >&2; return 1; }
  printf '%s' "$output"
}

if [ "$action" = plan ]; then
  echo "DRY RUN: would create three m53-* workspaces, exclude one before export, comp one, seed four bounded sealed dimensions per workspace, and record non-secret ids in $state_file"
  exit 0
fi

command -v psql >/dev/null || { echo "error: psql is required" >&2; exit 2; }
command -v stripe >/dev/null || { echo "error: stripe CLI is required" >&2; exit 2; }

if [ "$action" = apply ]; then
  [ ! -e "$state_file" ] || { echo "error: fixture state already exists at $state_file; verify or cleanup it first" >&2; exit 1; }
  run_id="$(date -u +%m%d%H%M)"
  window="$(python3 - <<'PY'
import datetime
now = datetime.datetime.now(datetime.timezone.utc).replace(minute=0, second=0, microsecond=0)
print((now - datetime.timedelta(hours=2)).isoformat().replace('+00:00', 'Z'))
PY
)"
  expires="$(python3 - <<'PY'
import datetime
print((datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(hours=24)).isoformat().replace('+00:00', 'Z'))
PY
)"
  mkdir -p "$(dirname "$state_file")"
  python3 - "$state_file" "$window" "$actor" "$expires" <<'PY'
import datetime, json, os, sys
path, window, actor, expires = sys.argv[1:]
created = datetime.datetime.now(datetime.timezone.utc)
body = {
    "purpose": "disposable Stripe m53 production test-mode fixtures",
    "owner": actor,
    "createdAt": created.isoformat().replace("+00:00", "Z"),
    "expiresAt": expires,
    "windowStart": window,
    "workspaces": {},
}
fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as handle:
    json.dump(body, handle, sort_keys=True, indent=2)
    handle.write("\n")
PY
  record_workspace() {
    local mode="$1" id="$2"
    python3 - "$state_file" "$mode" "$id" <<'PY'
import json, os, sys, tempfile
path, mode, workspace_id = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    body = json.load(handle)
body["workspaces"][mode] = workspace_id
directory = os.path.dirname(path) or "."
fd, temporary = tempfile.mkstemp(prefix=".stripe-m53-", dir=directory)
try:
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, "w") as handle:
        json.dump(body, handle, sort_keys=True, indent=2)
        handle.write("\n")
    os.replace(temporary, path)
except BaseException:
    try:
        os.close(fd)
    except OSError:
        pass
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
    raise
PY
  }
  create_workspace() {
    local mode="$1"
    jq -cn --arg name "m53-$mode-$run_id" '{name:$name,plan:"hobby"}' |
      cp_curl -H 'Content-Type: application/json' -X POST --data-binary @- "$cp_url/v1/tenants"
  }
  paid="$(create_workspace paid | jq -r .id)"
  case "$paid" in tea-*) record_workspace paid "$paid" ;; *) echo "error: control plane returned an invalid workspace id" >&2; exit 1 ;; esac
  excluded="$(create_workspace excluded | jq -r .id)"
  case "$excluded" in tea-*) record_workspace excluded "$excluded" ;; *) echo "error: control plane returned an invalid workspace id" >&2; exit 1 ;; esac
  comp="$(create_workspace comp | jq -r .id)"
  case "$comp" in tea-*) record_workspace comp "$comp" ;; *) echo "error: control plane returned an invalid workspace id" >&2; exit 1 ;; esac
  jq -cn --arg actor "$actor" '{excluded:true,actor:$actor}' |
    cp_curl -H 'Content-Type: application/json' -X PATCH --data-binary @- "$cp_url/v1/tenants/$excluded/billing-excluded" >/dev/null
  jq -cn --arg actor "$actor" '{action:"provision",actor:$actor,reason:"disposable m53 paid test fixture; expires in 24h"}' |
    cp_curl -H 'Content-Type: application/json' -X POST --data-binary @- "$cp_url/v1/tenants/$paid/billing-override" >/dev/null
  jq -cn --arg actor "$actor" '{action:"comp",actor:$actor,reason:"disposable m53 test fixture; expires in 24h"}' |
    cp_curl -H 'Content-Type: application/json' -X POST --data-binary @- "$cp_url/v1/tenants/$comp/billing-override" >/dev/null

  for id in "$paid" "$comp"; do
    mapping="$(psql service=bex_fixture -v ON_ERROR_STOP=1 -At -F '|' -v id="$id" -c "SELECT customer_id, subscription_id FROM billing_provider_mappings WHERE workspace_id=:'id'")"
    customer="${mapping%%|*}"
    subscription="${mapping#*|}"
    [ -n "$customer" ] && [ -n "$subscription" ] || { echo "error: fixture provider mapping is incomplete" >&2; exit 1; }
    stripe_json post "/v1/customers/$customer" -d 'metadata[bex_fixture]=m53' -d "metadata[bex_fixture_owner]=$actor" -d "metadata[bex_fixture_expires_at]=$expires" >/dev/null
    stripe_json post "/v1/subscriptions/$subscription" -d 'metadata[bex_fixture]=m53' -d "metadata[bex_fixture_owner]=$actor" -d "metadata[bex_fixture_expires_at]=$expires" >/dev/null
  done

  psql service=bex_fixture -v ON_ERROR_STOP=1 -v paid="$paid" -v excluded="$excluded" -v comp="$comp" -v window="$window" >/dev/null <<'SQL'
WITH fixture(workspace_id) AS (VALUES (:'paid'), (:'excluded'), (:'comp'))
INSERT INTO usage_hourly (workspace_id, resource_kind, service_id, kind, tier, window_start, quantity)
SELECT workspace_id, 'service', replace(workspace_id, 'tea-', 'srv-'), kind, tier, :'window'::timestamptz, quantity
FROM fixture CROSS JOIN (VALUES
  ('instance_seconds','starter',3600::bigint),
  ('egress_bytes','',1073741824::bigint),
  ('build_seconds','',30::bigint),
  ('storage_gb_seconds','',1800::bigint)
) AS dimensions(kind,tier,quantity);
SQL
  echo "created paid=$paid excluded=$excluded comp=$comp window=$window; state=$state_file"
  exit 0
fi

[ -f "$state_file" ] || { echo "error: fixture state not found at $state_file" >&2; exit 1; }
paid="$(jq -r '.workspaces.paid // empty' "$state_file")"
excluded="$(jq -r '.workspaces.excluded // empty' "$state_file")"
comp="$(jq -r '.workspaces.comp // empty' "$state_file")"
window="$(jq -r .windowStart "$state_file")"

if [ "$action" = verify ]; then
  for mode in paid excluded comp; do
    id="$(jq -r --arg mode "$mode" '.workspaces[$mode] // empty' "$state_file")"
    case "$id" in tea-*) ;; *) echo "FAIL: fixture state is incomplete; run cleanup" >&2; exit 1 ;; esac
  done
  end="$(python3 - "$window" <<'PY'
import datetime, sys
value = datetime.datetime.fromisoformat(sys.argv[1].replace('Z', '+00:00'))
print((value + datetime.timedelta(hours=1)).isoformat().replace('+00:00', 'Z'))
PY
)"
  for mode in paid excluded comp; do
    id="$(jq -r --arg mode "$mode" '.workspaces[$mode]' "$state_file")"
    report="$(cp_curl --get --data-urlencode "start=$window" --data-urlencode "end=$end" "$cp_url/v1/tenants/$id/billing-export")"
    if [ "$mode" = excluded ]; then
      [ -z "$(printf '%s' "$report" | jq -r '.customerId // empty')" ] || { echo "FAIL: excluded fixture has a Stripe Customer" >&2; exit 1; }
      [ "$(printf '%s' "$report" | jq '[.rows[] | select(.state != "pending")] | length')" = 0 ] || { echo "FAIL: excluded usage left pending state" >&2; exit 1; }
    else
      [ "$(printf '%s' "$report" | jq -r .livemode)" = false ] || { echo "FAIL: $mode mapping is not test mode" >&2; exit 1; }
      [ "$(printf '%s' "$report" | jq '[.rows[] | select(.state == "emitted")] | length')" = 4 ] || { echo "FAIL: $mode does not have four emitted dimensions" >&2; exit 1; }
    fi
  done
  echo "PASS: paid/excluded/comp fixture isolation and four-dimension export state"
  exit 0
fi

# cleanup: resolve exact mapped test Customers first, then delete only the three
# state-file workspace ids. Stripe catalog, invoices, meters, and audit evidence
# are intentionally preserved.
for id in "$paid" "$comp"; do
  case "$id" in tea-*) ;; *) continue ;; esac
  customer="$(psql service=bex_fixture -v ON_ERROR_STOP=1 -At -v id="$id" -c "SELECT customer_id FROM billing_provider_mappings WHERE workspace_id=:'id'")"
  if [ -n "$customer" ]; then
    object="$(stripe_json get "/v1/customers/$customer")"
    [ "$(printf '%s' "$object" | jq -r .livemode)" = false ] || { echo "error: refusing to delete live customer" >&2; exit 1; }
    stripe_json delete "/v1/customers/$customer" --confirm >/dev/null
  fi
done
for id in "$paid" "$excluded" "$comp"; do
  case "$id" in tea-*) ;; *) continue ;; esac
  psql service=bex_fixture -v ON_ERROR_STOP=1 -v id="$id" -c "DELETE FROM tenants WHERE id=:'id'" >/dev/null
  remaining="$(psql service=bex_fixture -v ON_ERROR_STOP=1 -At -v id="$id" -c "SELECT count(*) FROM tenants WHERE id=:'id'")"
  [ "$remaining" = 0 ] || { echo "FAIL: fixture tenant remains: $id" >&2; exit 1; }
done
unlink "$state_file"
echo "PASS: deleted exact paid/excluded/comp bex fixtures and their test Customers; retained provider billing evidence"
