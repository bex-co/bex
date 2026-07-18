#!/usr/bin/env bash
# Re-run docs/cli-compatibility-checklist.md's ✅ rows against a live bex-api
# and fail loudly on regression (the scripts/webhooks-verify.sh pattern, t006).
# Invoked via `scripts/cli-compat.sh verify` (which sets up RENDER_HOST/
# RENDER_API_KEY/RENDER_WORKSPACE first) — do not run this file directly.
#
# Side-effect-free rows re-run as-is; lifecycle checks create uniquely named
# resources and clean all of them up in a trap so failures leave no residue.
# w9/m4 extended the original keyvalues/instances legs to the full census that
# makes a bex-api call: postgres lifecycle (create+ip-allow-list, list/get-by-
# name, every update flag, rename, suspend/resume/delete), services CRUD,
# deploys list/create/cancel, logs (matching + empty window), environments, and
# logout's OAuth-revoke path — so none of RC2/RC7–RC12/RC15 can silently regress.
set -uo pipefail
RENDER_BIN="${RENDER_BIN:-.pm/w9/dev-9/bin/render}"
fail=0
INSTANCE_NAME=""
PG_NAME=""
PG_ID=""
KV_NAME=""
KV_ID=""
PROJECT_ID=""
ENV_ID=""
LOGOUT_CFG=""
EXPECTED_REGION="${BEX_EXPECTED_REGION:-local-capd}"
EXPECTED_DASHBOARD_URL="${BEX_EXPECTED_DASHBOARD_URL:-http://localhost:50090}"
API="${RENDER_HOST%/}" # e.g. http://localhost:54090/v1

cleanup() {
  [ -n "$INSTANCE_NAME" ] && "$RENDER_BIN" services delete "$INSTANCE_NAME" --confirm -o json >/dev/null 2>&1
  [ -n "$PG_NAME" ] && "$RENDER_BIN" postgres delete "$PG_NAME" --confirm -o json >/dev/null 2>&1
  [ -n "$KV_NAME" ] && "$RENDER_BIN" keyvalues delete "$KV_NAME" --confirm -o json >/dev/null 2>&1
  [ -n "$ENV_ID" ] && api DELETE "/environments/$ENV_ID" >/dev/null 2>&1
  [ -n "$PROJECT_ID" ] && api DELETE "/projects/$PROJECT_ID" >/dev/null 2>&1
  [ -n "$LOGOUT_CFG" ] && rm -f "$LOGOUT_CFG"
  return 0
}
trap cleanup EXIT

# --- reporting -------------------------------------------------------------
# pass/failx record one leg's result; every other helper funnels through them
# so a leg is a single line at the call site.
pass() { echo "PASS: $1"; }
failx() { echo "FAIL: $1"; [ -n "${2:-}" ] && echo "  $2"; fail=1; }

# --- raw REST helpers ------------------------------------------------------
# api METHOD PATH [JSON] — one authenticated curl against bex-api's REST
# surface (the same bearer the CLI uses), body echoed to stdout. The verifier
# seeds projects/environments and the throwaway logout key through this because
# the pinned CLI has no create verb for them.
api() {
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$body" ]; then
    curl -sS -X "$method" -H "Authorization: Bearer $RENDER_API_KEY" \
      -H 'Content-Type: application/json' -d "$body" "$API$path"
  else
    curl -sS -X "$method" -H "Authorization: Bearer $RENDER_API_KEY" "$API$path"
  fi
}
# api_code METHOD PATH — same call, but print only the HTTP status code.
api_code() {
  curl -sS -o /dev/null -w '%{http_code}' -X "$1" \
    -H "Authorization: Bearer $RENDER_API_KEY" "$API$2"
}
# json_field EXPR — read one field from a CLI/REST JSON blob on stdin,
# unwrapping the CLI's `{data:{…}}` envelope when present. e.g. json_field '["id"]'.
json_field() {
  python3 -c "import json,sys
d=json.load(sys.stdin)
d=d.get('data',d) if isinstance(d,dict) else d
print(d$1)"
}

# check DESC WANT CMD... — runs CMD, requires exit 0 AND that its output
# contains WANT (empty WANT skips the content check, exit-status-only). WANT
# is an extended-regex, not a literal — `-o json` pretty-prints with a space
# after each `:`, so JSON-field checks below use `"key":[[:space:]]*"value"`.
check() {
  local desc="$1" want="$2"; shift 2
  local got status
  got="$("$@" 2>&1)"; status=$?
  if [ "$status" != 0 ]; then
    failx "$desc (exit $status)" "got: $got"
  elif [ -n "$want" ] && ! grep -qE "$want" <<<"$got"; then
    failx "$desc" "expected to match: $want
  got: $got"
  else
    pass "$desc"
  fi
}

# checkFields DESC CMD... — runs CMD once, then asserts every pattern in
# WANT_FIELDS (space-separated, set by the caller before calling this) is
# present in the single output. Exists because `check`'s single-pattern form
# missed a real regression once: every keyvalues subcommand exited 0 and
# echoed the record's id/name back fine while bex-api silently sent `ownerId`
# empty and dropped `maxmemoryPolicy`/`persistenceMode` entirely (Render's
# real KeyValueDetail nests them under `owner`/`options`, bex-api sent them
# flat) — 2026-07-15. A single-field spot check never would have caught it.
checkFields() {
  local desc="$1"; shift
  local got status field missing=""
  got="$("$@" 2>&1)"; status=$?
  if [ "$status" != 0 ]; then
    failx "$desc (exit $status)" "got: $got"
    return
  fi
  for field in $WANT_FIELDS; do
    grep -qE "$field" <<<"$got" || missing="$missing $field"
  done
  if [ -n "$missing" ]; then
    failx "$desc — missing:$missing" "got: $got"
  else
    pass "$desc"
  fi
}

# check_absent DESC UNWANTED CMD... — runs CMD, requires exit 0 AND that its
# output does NOT match UNWANTED (an extended-regex). The "a fixed failure
# signature must not reappear" leg — e.g. RC7's masked "app not found" or RC8's
# `parsing time ""` crash — which the positive-match check/checkFields can't express.
check_absent() {
  local desc="$1" unwanted="$2"; shift 2
  local got status
  got="$("$@" 2>&1)"; status=$?
  if [ "$status" = 0 ] && ! grep -qiE "$unwanted" <<<"$got"; then
    pass "$desc"
  else
    failx "$desc (exit $status)" "unwanted match or failure: $unwanted
  got: $got"
  fi
}

# check_notfound DESC CMD... — the negative leg every family owns: an unknown
# id/name must fail NONZERO with a real, specific message — never a masked
# "unknown error" (the exact RC1 regression, where a wrong error-envelope shape
# swallowed the real 404 message) and never a silent exit 0.
check_notfound() {
  local desc="$1"; shift
  local got status
  got="$("$@" 2>&1)"; status=$?
  if [ "$status" = 0 ] || [ -z "$got" ] || grep -qi 'unknown error' <<<"$got"; then
    failx "$desc" "got (exit $status): $got"
  else
    pass "$desc"
  fi
}

# valid_instance_json reads a CLI JSON response from stdin and accepts only a
# non-empty ServiceInstance array with populated ids and parseable timestamps.
# Kept as one parser so the rollout poll and final assertion cannot drift.
valid_instance_json() {
  python3 -c '
import datetime, json, sys
rows = json.load(sys.stdin)
assert isinstance(rows, list) and rows
for row in rows:
    assert isinstance(row.get("id"), str) and row["id"]
    datetime.datetime.fromisoformat(row["createdAt"].replace("Z", "+00:00"))
'
}

# valid_raw_metadata KIND ID ROUTE — reads one raw bex-api REST object and
# validates the metadata that some CLI output adapters intentionally flatten or
# omit (Service has ownerId rather than nested owner; KeyValueOut drops
# dashboardUrl). The unmodified CLI still performs the HTTP decode first, while
# this assertion prevents its output projection from hiding a server regression.
# ID must be the resource's opaque id (srv-…/dpg-…), not its display name — the
# GET-by-id route does not resolve display names (RC5 decoupled the two).
valid_raw_metadata() {
  local kind="$1" id="$2" route="$3"
  curl -sf -H "Authorization: Bearer $RENDER_API_KEY" "$API/$kind/$id" | \
    EXPECTED_OWNER="$RENDER_WORKSPACE" EXPECTED_REGION="$EXPECTED_REGION" \
    EXPECTED_DASHBOARD="$EXPECTED_DASHBOARD_URL/$route/$id" python3 -c '
import datetime, json, os, sys
d = json.load(sys.stdin)
owner = d.get("owner")
assert isinstance(owner, dict) and owner.get("id") == os.environ["EXPECTED_OWNER"]
assert isinstance(owner.get("name"), str) and owner["name"]
assert owner.get("type") == "team"
assert d.get("dashboardUrl") == os.environ["EXPECTED_DASHBOARD"]
region = d.get("region")
if region is None:
    region = d.get("serviceDetails", {}).get("region")
assert region == os.environ["EXPECTED_REGION"]
created = datetime.datetime.fromisoformat(d["createdAt"].replace("Z", "+00:00"))
updated = datetime.datetime.fromisoformat(d["updatedAt"].replace("Z", "+00:00"))
assert updated >= created
'
}

# raw_meta_check KIND ID ROUTE DESC — valid_raw_metadata wrapped as a reported leg.
raw_meta_check() {
  if valid_raw_metadata "$1" "$2" "$3"; then pass "$4"; else failx "$4"; fi
}

raw_updated_at() {
  curl -sf -H "Authorization: Bearer $RENDER_API_KEY" "$API/$1/$2" | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["updatedAt"])'
}

timestamp_advanced() {
  BEFORE="$1" AFTER="$2" python3 -c '
import datetime, os
parse = lambda s: datetime.datetime.fromisoformat(s.replace("Z", "+00:00"))
assert parse(os.environ["AFTER"]) > parse(os.environ["BEFORE"])
'
}

# updatedat_advances KIND ID BEFORE DESC — assert the raw updatedAt moved past
# BEFORE (the shared "a mutation really touched the record" leg).
updatedat_advances() {
  local after; after="$(raw_updated_at "$1" "$2" 2>/dev/null || true)"
  if [ -n "$3" ] && [ -n "$after" ] && timestamp_advanced "$3" "$after"; then
    pass "$4"
  else
    failx "$4" "updatedAt did not advance ($3 -> $after)"
  fi
}

# ===========================================================================
# Session / identity rows (side-effect free)
# ===========================================================================
check "login recognizes RENDER_API_KEY" \
  "Success: CLI is already authenticated." \
  "$RENDER_BIN" login --confirm -o json

# whoami (RC6 route + the BEX_KRATOS_ADMIN_URL harness wiring): an API-key
# caller reports the key-minting user's email via ownerEmail's Kratos-admin
# lookup. Exact match when cli-compat.sh exported CLI_COMPAT_EMAIL; any
# populated email otherwise (an empty `Email:` line must fail either way).
check "whoami reports the key-minting user's email (RC6)" \
  "Email:[[:space:]]*${CLI_COMPAT_EMAIL:-[^[:space:]]+@[^[:space:]]+}" \
  "$RENDER_BIN" whoami -o json

check "workspace current returns the active tenant" \
  "$RENDER_WORKSPACE" \
  "$RENDER_BIN" workspace current -o json

check "workspaces lists the active tenant" \
  "$RENDER_WORKSPACE" \
  "$RENDER_BIN" workspaces -o json

check "projects responds without error" \
  "" \
  "$RENDER_BIN" projects -o json

# ===========================================================================
# Services family (RC2/RC9 envelopes, RC13 metadata) + deploys (RC2/RC10) +
# logs (RC7/RC8). One self-created image-backed web service carries all of them
# so the legs share a single rollout; the trap removes it on every exit path.
# ===========================================================================
INSTANCE_NAME="verify-instances-$$"
instance_create=$(
  "$RENDER_BIN" services create --name "$INSTANCE_NAME" --type web_service \
    --image traefik/whoami:v1.10.3 --plan free --num-instances 1 --confirm -o json 2>&1
)
if [ $? != 0 ]; then
  failx "services create (RC9 serviceAndDeploy envelope)" "got: $instance_create"
else
  INSTANCE_ID=$(json_field '["id"]' <<<"$instance_create" 2>/dev/null || true)
  if [ -z "$INSTANCE_ID" ]; then
    failx "services create returned no service id" "got: $instance_create"
  else
    pass "services create returns the RC9 {service,deployId} record"
    raw_meta_check services "$INSTANCE_ID" web \
      "service raw REST metadata has real owner/region/dashboard/timestamps"

    service_updated_before=$(raw_updated_at services "$INSTANCE_ID" 2>/dev/null || true)
    check "services update mutates through official CLI" \
      '"healthCheckPath":[[:space:]]*"/metadata-ready"' \
      "$RENDER_BIN" services update "$INSTANCE_NAME" --health-check-path /metadata-ready --confirm -o json
    updatedat_advances services "$INSTANCE_ID" "$service_updated_before" \
      "service updatedAt advances after update"

    # services instances: poll until the operator materializes a decodable Pod.
    instance_json=""; instance_status=1
    for _ in $(seq 1 30); do
      instance_json=$("$RENDER_BIN" services instances "$INSTANCE_ID" -o json 2>&1)
      instance_status=$?
      if [ "$instance_status" = 0 ] && valid_instance_json <<<"$instance_json" 2>/dev/null; then break; fi
      sleep 2
    done
    if [ "$instance_status" != 0 ] || ! valid_instance_json <<<"$instance_json" 2>/dev/null; then
      failx "services instances JSON decodes live id + createdAt" "got: $instance_json"
    else
      pass "services instances JSON decodes live id + createdAt"
      first_instance_id=$(json_field '[0]["id"]' <<<"$instance_json")
      check "services instances text mode renders the real instance" \
        "$first_instance_id" \
        "$RENDER_BIN" services instances "$INSTANCE_ID" -o text
    fi

    # deploys create (RC10 clearCache string enum): the CLI always sends
    # clearCache explicitly; a bool-typed field here 400s every call.
    deploy_create=$("$RENDER_BIN" deploys create "$INSTANCE_ID" --confirm -o json 2>&1)
    if [ $? != 0 ]; then
      failx "deploys create (RC10 clearCache enum)" "got: $deploy_create"
    else
      DEPLOY_ID=$(json_field '["id"]' <<<"$deploy_create" 2>/dev/null || true)
      if [ -z "$DEPLOY_ID" ]; then
        failx "deploys create returned no deploy id" "got: $deploy_create"
      else
        pass "deploys create returns a deploy id (RC10)"
        # cancel the just-created deploy while it is still open (FinishedAt nil).
        check "deploys cancel resolves the open deploy" \
          "$DEPLOY_ID|canceled" \
          "$RENDER_BIN" deploys cancel "$INSTANCE_ID" "$DEPLOY_ID" --confirm -o json
        # deploys list (RC2 Deploy.image object, not a bare string): the created
        # deploy must appear with Render's nested {ref,…} image shape.
        WANT_FIELDS="\"id\":[[:space:]]*\"$DEPLOY_ID\" \"image\": \"ref\":"
        checkFields "deploys list carries the RC2 nested image object" \
          "$RENDER_BIN" deploys list "$INSTANCE_ID" -o json
      fi
    fi

    # logs (RC7 name resolution + RC8 empty-window cursors). Both regressions
    # manifested as a NONZERO failure — a masked "app not found" (RC7) or a
    # `parsing time ""` crash (RC8). dev-9 has no Loki, so logs read live pod
    # logs; poll until the container is past ContainerCreating (a transient 500
    # that is neither RC7 nor RC8) so the assertions see the real steady state.
    logs_hit=""; logs_status=1
    for _ in $(seq 1 30); do
      logs_hit=$("$RENDER_BIN" logs --resources "$INSTANCE_NAME" --limit 5 -o json 2>&1)
      logs_status=$?
      [ "$logs_status" = 0 ] && break
      grep -qi 'ContainerCreating\|waiting to start' <<<"$logs_hit" || break
      sleep 2
    done
    # RC7 keeps its own readiness poll above; assert the resolved output here.
    if [ "$logs_status" = 0 ] && ! grep -qiE 'app not found|not found' <<<"$logs_hit"; then
      pass "logs resolves a service by name (RC7)"
    else
      failx "logs resolves a service by name (RC7)" "got: $logs_hit"
    fi
    check_absent "logs on an empty window returns cursors, no parse crash (RC8)" \
      'parsing time|cannot parse' \
      "$RENDER_BIN" logs --resources "$INSTANCE_NAME" --text zzzz-no-such-log-line-zzzz --limit 5 -o json

    # Suspend through the REST lifecycle verb, then prove the SAME instances
    # path returns [] before delete.
    suspend_code=$(api_code POST "/services/$INSTANCE_ID/suspend")
    if [ "$suspend_code" != 202 ]; then
      failx "services suspend setup returned HTTP $suspend_code"
    else
      empty_json=$("$RENDER_BIN" services instances "$INSTANCE_ID" -o json 2>&1)
      if [ $? = 0 ] && python3 -c 'import json,sys; r=json.load(sys.stdin); assert isinstance(r,list) and not r' <<<"$empty_json" 2>/dev/null; then
        pass "services instances suspended service returns []"
      else
        failx "services instances suspended service returns []" "got: $empty_json"
      fi
    fi

    # services delete (RC2/RC9): returns the deleted record; clear the name so
    # the trap does not double-delete.
    check "services delete returns the deleted record" \
      '"deleted":[[:space:]]*true' \
      "$RENDER_BIN" services delete "$INSTANCE_NAME" --confirm -o json
    INSTANCE_NAME=""
  fi
fi

check_notfound "services instances missing service preserves a not-found failure" \
  "$RENDER_BIN" services instances srv-00000000000000000000 -o json

# ===========================================================================
# Postgres lifecycle (RC3 cursor envelope, RC7 name filter, RC11 non-plan
# update flags, RC12 ipAllowList wire shape, RC5 rename, RC13 metadata).
# ===========================================================================
PG_NAME="verify-pg-$$"
pg_create=$("$RENDER_BIN" postgres create --name "$PG_NAME" --plan free --version 16 \
  --ip-allow-list "cidr=10.0.0.0/8,description=verify" --confirm -o json 2>&1)
if [ $? != 0 ]; then
  failx "postgres create --ip-allow-list (RC12)" "got: $pg_create"
else
  PG_ID=$(json_field '["id"]' <<<"$pg_create" 2>/dev/null || true)
  # create round-trips the id, the dpg-… opaque id (RC5), and the RC12 wire
  # shape — including the entry's description, which persists since w4/m24
  # (asserting only the cidrBlock is the single-field-check trap RC14 taught).
  WANT_FIELDS="\"name\":[[:space:]]*\"$PG_NAME\" \"id\":[[:space:]]*\"dpg-[a-z0-9]+\" \"cidrBlock\":[[:space:]]*\"10\\.0\\.0\\.0/8\" \"description\":[[:space:]]*\"verify\""
  checkFields "postgres create: id/name/ipAllowList wire shape all correct (RC12, w4/m24 description)" \
    echo "$pg_create"

  raw_meta_check postgres "$PG_ID" d \
    "postgres raw REST metadata has real owner/region/dashboard/timestamps"

  # list + get-by-name (RC3 envelope + RC7 ?name= filter) with whole-shape asserts.
  WANT_FIELDS="\"id\":[[:space:]]*\"$PG_ID\" \"name\":[[:space:]]*\"$PG_NAME\" \"plan\": \"cidrBlock\":[[:space:]]*\"10\\.0\\.0\\.0/8\" \"description\":[[:space:]]*\"verify\""
  checkFields "postgres list: record resolves through the cursor envelope (RC3)" \
    "$RENDER_BIN" postgres list -o json
  checkFields "postgres get: resolves by name (RC3/RC7) with every field intact" \
    "$RENDER_BIN" postgres get "$PG_NAME" -o json

  check "postgres text renders Workspace" \
    "Workspace:[[:space:]]+.*$RENDER_WORKSPACE" \
    "$RENDER_BIN" postgres get "$PG_NAME" -o text
  check "postgres text renders configured Region" \
    "Region:[[:space:]]*$EXPECTED_REGION" \
    "$RENDER_BIN" postgres get "$PG_NAME" -o text
  check "postgres text renders dashboard route" \
    "Dashboard:[[:space:]]*$EXPECTED_DASHBOARD_URL/d/$PG_ID" \
    "$RENDER_BIN" postgres get "$PG_NAME" -o text

  # update --plan (RC3): the response reflects the new plan.
  pg_updated_before=$(raw_updated_at postgres "$PG_ID" 2>/dev/null || true)
  check "postgres update --plan mutates through official CLI" \
    "\"plan\":[[:space:]]*\"basic-1gb\"" \
    "$RENDER_BIN" postgres update "$PG_NAME" --plan basic-1gb -o json
  updatedat_advances postgres "$PG_ID" "$pg_updated_before" \
    "postgres updatedAt advances after update"

  # update --disk-size-gb (RC11): every non-plan flag previously 400'd because
  # the handler required plan on every update. diskSizeGB reads from spec, so
  # the response reflects it directly.
  check "postgres update --disk-size-gb applies without plan (RC11)" \
    "\"diskSizeGB\":[[:space:]]*5" \
    "$RENDER_BIN" postgres update "$PG_NAME" --disk-size-gb 5 -o json

  # update --high-availability (RC11): the read-back highAvailabilityEnabled is
  # operator-observed (needs 2 ready replicas), so the guard is that the flag
  # applies without a 400 and the record still resolves by its id.
  check "postgres update --high-availability applies without plan (RC11)" \
    "\"id\":[[:space:]]*\"$PG_ID\"" \
    "$RENDER_BIN" postgres update "$PG_NAME" --high-availability=true -o json

  # update --ip-allow-list (RC12) replaces the list; the new CIDR AND its
  # description round-trip (descriptions persist since w4/m24).
  WANT_FIELDS="\"cidrBlock\":[[:space:]]*\"203\\.0\\.113\\.0/24\" \"description\":[[:space:]]*\"office\""
  checkFields "postgres update --ip-allow-list replaces the list, description returns (RC12, w4/m24)" \
    "$RENDER_BIN" postgres update "$PG_NAME" --ip-allow-list "cidr=203.0.113.0/24,description=office" -o json

  # update --clear-ip-allow-list (RC11/RC12): the list empties. Assert the
  # ipAllowList field itself is empty rather than grepping the whole output —
  # the CLI's update diff echoes the removed CIDR, which a raw grep would catch.
  pg_cleared=$("$RENDER_BIN" postgres update "$PG_NAME" --clear-ip-allow-list -o json 2>&1)
  if [ $? = 0 ] && python3 -c 'import json,sys
d=json.load(sys.stdin); d=d.get("data",d)
assert not d.get("ipAllowList")' <<<"$pg_cleared" 2>/dev/null; then
    pass "postgres update --clear-ip-allow-list empties the list (RC11/RC12)"
  else
    failx "postgres update --clear-ip-allow-list empties the list (RC11/RC12)" "got: $pg_cleared"
  fi

  # rename (RC5): the mutable display name changes but the opaque dpg-… id does
  # not, and the CLI re-resolves the new name to the same id.
  PG_RENAMED="verify-pg-renamed-$$"
  check "postgres update --name renames (RC5)" \
    "\"name\":[[:space:]]*\"$PG_RENAMED\"" \
    "$RENDER_BIN" postgres update "$PG_NAME" --name "$PG_RENAMED" -o json
  renamed_id=$("$RENDER_BIN" postgres get "$PG_RENAMED" -o json 2>/dev/null | json_field '["id"]' 2>/dev/null || true)
  if [ "$renamed_id" = "$PG_ID" ]; then
    pass "postgres rename keeps the opaque id stable (RC5)"
  else
    failx "postgres rename keeps the opaque id stable (RC5)" "resolved new name to $renamed_id, want $PG_ID"
  fi
  PG_NAME="$PG_RENAMED"

  # suspend / resume / delete lifecycle (RC3).
  check "postgres suspend resolves and applies" \
    "\"suspended\":[[:space:]]*true" \
    "$RENDER_BIN" postgres suspend "$PG_NAME" --confirm -o json
  check "postgres resume resolves and applies" \
    "\"id\":[[:space:]]*\"$PG_ID\"" \
    "$RENDER_BIN" postgres resume "$PG_NAME" --confirm -o json
  check "postgres delete resolves and applies" \
    "\"deleted\":[[:space:]]*true" \
    "$RENDER_BIN" postgres delete "$PG_NAME" --confirm -o json
  PG_NAME=""
fi

check_notfound "postgres get missing database returns a real not-found (RC1)" \
  "$RENDER_BIN" postgres get dpg-00000000000000000000 -o json

# ===========================================================================
# KeyValue lifecycle (RC3 envelope + RC4 maxmemoryPolicy + RC14 nested shape).
# ===========================================================================
KV_NAME="verify-kv-$$"

# Every field Render's real KeyValueDetail carries that bex-api has
# historically dropped or zero-valued silently (RC3/RC4/owner-options-nesting/
# ipAllowList-shape) — checked together so a partial regression can't hide
# behind one passing field, per checkFields' doc comment above. The trailing
# description assertion is w4/m24: an --ip-allow-list description must come
# BACK, not merely be accepted.
kv_create=$(
  "$RENDER_BIN" keyvalues create --name "$KV_NAME" \
    --ip-allow-list "cidr=10.0.0.0/8,description=verify" --confirm -o json 2>&1
)
if [ $? != 0 ]; then
  failx "keyvalues create (RC3/RC4)" "got: $kv_create"
else
  KV_ID=$(json_field '["id"]' <<<"$kv_create" 2>/dev/null || true)
  WANT_FIELDS='"id":[[:space:]]*"red-[a-z0-9]+" "name":[[:space:]]*"'"$KV_NAME"'" "ownerId":[[:space:]]*"tea-[a-z0-9]+" "maxmemoryPolicy":[[:space:]]*"allkeys_lru" "persistenceMode":[[:space:]]*"journal_snapshot" "cidrBlock":[[:space:]]*"10\.0\.0\.0/8" "description":[[:space:]]*"verify"'
  checkFields "keyvalues create: typed id + owner/options + ipAllowList shape all correct" \
    echo "$kv_create"

  raw_meta_check key-value "$KV_ID" r \
    "key-value raw REST metadata has real owner/region/dashboard/timestamps"
fi

check "keyvalues text renders Workspace" \
  "Workspace:[[:space:]]+.*$RENDER_WORKSPACE" \
  "$RENDER_BIN" keyvalues get "$KV_NAME" -o text
check "keyvalues text renders configured Region" \
  "Region:[[:space:]]*$EXPECTED_REGION" \
  "$RENDER_BIN" keyvalues get "$KV_NAME" -o text
kv_updated_before=$(raw_updated_at key-value "$KV_ID" 2>/dev/null || true)

WANT_FIELDS='"id":[[:space:]]*"'"$KV_ID"'" "name":[[:space:]]*"'"$KV_NAME"'" "ownerId":[[:space:]]*"tea-[a-z0-9]+" "maxmemoryPolicy":[[:space:]]*"allkeys_lru" "persistenceMode":[[:space:]]*"journal_snapshot" "cidrBlock":[[:space:]]*"10\.0\.0\.0/8" "description":[[:space:]]*"verify"'
checkFields "keyvalues list: same fields survive the cursor envelope (RC3)" \
  "$RENDER_BIN" keyvalues list -o json

checkFields "keyvalues get: resolves by name (RC3) with every field intact" \
  "$RENDER_BIN" keyvalues get "$KV_NAME" -o json

check "keyvalues suspend resolves and applies" \
  "\"suspended\":[[:space:]]*true" \
  "$RENDER_BIN" keyvalues suspend "$KV_NAME" --confirm -o json

updatedat_advances key-value "$KV_ID" "$kv_updated_before" \
  "key-value updatedAt advances after suspend"

# resume's downstream `status` field is async (K8s reconciliation may still say
# "unavailable" moments after the call returns) — only exit 0 + the right id
# is asserted, not the transient status, to avoid a flaky false failure.
check "keyvalues resume resolves and applies" \
  "\"id\":[[:space:]]*\"$KV_ID\"" \
  "$RENDER_BIN" keyvalues resume "$KV_NAME" --confirm -o json

check "keyvalues delete resolves and applies" \
  "\"deleted\":[[:space:]]*true" \
  "$RENDER_BIN" keyvalues delete "$KV_NAME" --confirm -o json
KV_NAME=""

# ===========================================================================
# Environments (RC15 cursor envelope). The pinned CLI has no Project-create
# command, so seed a Project + Environment over authenticated REST, then make
# the unmodified CLI decode them.
# ===========================================================================
proj_json=$(api POST /projects "{\"name\":\"verify-proj-$$\",\"ownerId\":\"$RENDER_WORKSPACE\"}")
PROJECT_ID=$(json_field '["id"]' <<<"$proj_json" 2>/dev/null || true)
if [ -z "$PROJECT_ID" ]; then
  failx "environments setup: create project" "got: $proj_json"
else
  env_json=$(api POST /environments "{\"name\":\"staging\",\"projectId\":\"$PROJECT_ID\"}")
  ENV_ID=$(json_field '["id"]' <<<"$env_json" 2>/dev/null || true)
  if [ -z "$ENV_ID" ]; then
    failx "environments setup: create environment" "got: $env_json"
  else
    # RC15: a flat []Environment silently decodes to all-zero values; the CLI
    # needs the {environment,cursor} envelope. Assert the seeded values survive.
    WANT_FIELDS="\"id\":[[:space:]]*\"$ENV_ID\" \"name\":[[:space:]]*\"staging\" \"projectId\":[[:space:]]*\"$PROJECT_ID\" \"protectedStatus\":"
    checkFields "environments decode the RC15 cursor envelope into real values" \
      "$RENDER_BIN" environments "$PROJECT_ID" -o json
  fi
fi
check_notfound "environments unknown project preserves a not-found failure" \
  "$RENDER_BIN" environments prj-00000000000000000000 -o json

# ===========================================================================
# logout — OAuth-credential revoke path (POST /v1/oauth/revoke). Mint a
# THROWAWAY key (never the harness key), drive the unmodified CLI's real logout
# against it, and prove the token is dead afterward. Revoking the harness key
# would break the rest of this run, so the throwaway is mandatory.
# ===========================================================================
lo_key=$(api POST /api-keys "{\"name\":\"verify-logout-$$\",\"ownerId\":\"$RENDER_WORKSPACE\"}")
lo_id=$(json_field '["id"]' <<<"$lo_key" 2>/dev/null || true)
lo_secret=$(json_field '["secret"]' <<<"$lo_key" 2>/dev/null || true)
if [ -z "$lo_id" ] || [ -z "$lo_secret" ]; then
  failx "logout setup: mint throwaway api key" "got: $lo_key"
else
  lo_token=$(curl -sf -X POST "${HYDRA_PUBLIC_URL:-http://localhost:59090}/oauth2/token" \
    -d "grant_type=client_credentials&client_id=$lo_id&client_secret=$lo_secret" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])' 2>/dev/null || true)
  pre_code=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $lo_token" "$API/postgres")
  if [ -z "$lo_token" ] || [ "$pre_code" != 200 ]; then
    failx "logout setup: throwaway token does not authenticate" "token http=$pre_code"
  else
    # Hand-write the OAuth config the CLI's logout path reads (config.OAuthConfig):
    # api.key is the bearer it revokes, api.host is where it POSTs /oauth/revoke.
    LOGOUT_CFG="$(mktemp -d)/logout-cli.yaml"
    printf 'version: 1\nworkspace: %s\napi:\n  key: %s\n  host: %s/\n' \
      "$RENDER_WORKSPACE" "$lo_token" "$API" > "$LOGOUT_CFG"
    logout_out=$(RENDER_CLI_CONFIG_PATH="$LOGOUT_CFG" "$RENDER_BIN" logout 2>&1)
    if [ $? = 0 ] && ! grep -qiE 'went wrong revoking' <<<"$logout_out"; then
      pass "logout drives the OAuth revoke path without error"
    else
      failx "logout drives the OAuth revoke path without error" "got: $logout_out"
    fi
    # Proof the revoke actually deleted the underlying Hydra client: a fresh
    # client_credentials grant with the same id/secret now fails. (An
    # already-issued opaque access token is not itself invalidated on client
    # deletion — it lapses at its 15m TTL — so the grant is the deterministic
    # signal that revoke did its job, not a reuse of the old bearer.)
    regrant_code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
      "${HYDRA_PUBLIC_URL:-http://localhost:59090}/oauth2/token" \
      -d "grant_type=client_credentials&client_id=$lo_id&client_secret=$lo_secret")
    if [ "$regrant_code" != 200 ]; then
      pass "logout actually revoked the credential (client_credentials grant now fails)"
    else
      failx "logout actually revoked the credential (client_credentials grant now fails)" \
        "re-grant http=$regrant_code (want non-200)"
    fi
  fi
fi

if [ "$fail" != "0" ]; then
  echo "verify: one or more ✅ rows regressed" >&2
  exit 1
fi
echo "verify: all ✅ rows still hold"
