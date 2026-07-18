#!/usr/bin/env bash
# scripts/samples-lifecycle.sh — drive one examples/ sample (or `all`) through the
# full Render lifecycle against a bex deployment and prove each leg:
#
#   deploy → verify-up → suspend → verify-down → resume → verify-up → delete → verify-gone
#
# CLI-FIRST: every verb the official Render CLI (render-oss/cli, v2.x) has is run
# through it verbatim — `services create`, `deploys create`, `services delete`,
# `postgres create/suspend/resume/delete`, `logs`. The two verbs the CLI lacks —
# **service** suspend/resume — are dashboard-only in Render's CLI, so those legs
# fall back to raw Render-compatible REST (`POST /v1/services/{id}/suspend|resume`,
# 202, docs/ADR007-restart-suspend-and-resume.md). Each leg prints whether it used
# CLI or REST, so the CLI's gaps stay visible (docs/cli-compatibility-checklist.md).
#
# This is an ON-DEMAND guard (like `scripts/cli-compat.sh verify`), NOT CI — it
# creates and deletes real services on whatever bex the CLI points at. Point it at
# prod only when you mean to.
#
# Usage:
#   scripts/samples-lifecycle.sh <sample>       # one sample: hello-go|hello-node|
#                                               #   hello-python|whoami|static-site|
#                                               #   cron-demo|stack-demo
#   scripts/samples-lifecycle.sh all            # every sample, sequentially
#   scripts/samples-lifecycle.sh list           # print the known samples
#
# Auth (same overrides as scripts/cli-compat.sh):
#   RENDER_BIN        render binary, default `render` (must be logged in, or set
#                     RENDER_HOST/RENDER_API_KEY)
#   RENDER_HOST       REST base incl. /v1/, e.g. https://api.bex.co/v1/ — used by
#                     the REST-only legs; default: `api.host` from ~/.render/cli.yaml
#   RENDER_API_KEY    bearer for the REST-only legs; default: `api.key` from
#                     ~/.render/cli.yaml (re-read per call so CLI token refreshes
#                     propagate). Set both to target a bex-api the CLI isn't
#                     logged into (they are then also exported to the CLI).
#
# Knobs:
#   BASE_DOMAIN       app URL wildcard, default onbex.co (URL = <name>.<BASE_DOMAIN>)
#   REPO / BRANCH     repo-backed create source, default https://github.com/bex-co/bex, main
#   PLAN              instance plan for created services, default free
#   POLL_TIMEOUT      per-leg convergence budget in seconds, default 600
#   POLL_INTERVAL     poll cadence in seconds, default 10
#   DOWN_CONFIRM      consecutive "not serving" checks required to call a suspended
#                     service down, default 3 (rejects a transient 502 as "down")
#   CRON_SCHEDULE     cron-demo's schedule, default "*/5 * * * *"; the sweep runs it
#                     with "* * * * *" so a run fires within ~a minute (observable)
#   CRON_WINDOW       seconds to watch a suspended cron NOT fire, default 330 (one
#                     5-min window); pair with a faster CRON_SCHEDULE to shorten
#   NAME_SUFFIX       appended to each sample's service name (avoid collisions)
#   PRECLEAN=1        delete a pre-existing service of the same name before deploy
#   KEEP=1            skip the delete/verify-gone legs (leave the service running)
#   DRY_RUN=1         print each CLI/REST call instead of executing
#   RESIDUE_KUBE=1    opt in to the cluster residue check in verify-gone (leftover
#                     pods, reg-pull-<name> Secret, app-<name> htpasswd user). Only
#                     set it with kubectl/KUBECONFIG pointed at the SAME bex cluster
#                     the services were created on — a wrong context would falsely
#                     report "no residue". Unset ⇒ API-only checks (documented
#                     degrade, per t002).
#   APPS_NAMESPACE    tenant App namespace for the kubeconfig residue checks, default default
#   REGISTRY_NS       Zot registry namespace for the htpasswd residue check, default bex-registry
set -euo pipefail
cd "$(dirname "$0")/.."

# --------------------------------------------------------------------------- #
# config
# --------------------------------------------------------------------------- #
RENDER_BIN="${RENDER_BIN:-render}"
BASE_DOMAIN="${BASE_DOMAIN:-onbex.co}"
REPO="${REPO:-https://github.com/bex-co/bex}"
BRANCH="${BRANCH:-main}"
PLAN="${PLAN:-free}"
POLL_TIMEOUT="${POLL_TIMEOUT:-600}"
POLL_INTERVAL="${POLL_INTERVAL:-10}"
DOWN_CONFIRM="${DOWN_CONFIRM:-3}"
CRON_SCHEDULE="${CRON_SCHEDULE:-*/5 * * * *}"   # cron-demo schedule (sweep may use "* * * * *" to observe runs fast)
CRON_WINDOW="${CRON_WINDOW:-330}"               # seconds to watch for a suspended cron NOT firing (one+ schedule window)
NAME_SUFFIX="${NAME_SUFFIX:-}"
APPS_NAMESPACE="${APPS_NAMESPACE:-default}"
REGISTRY_NS="${REGISTRY_NS:-bex-registry}"
CLI_YAML="${RENDER_CLI_CONFIG_PATH:-$HOME/.render/cli.yaml}"

# A unique token per run so verify-up proves THIS deploy, not a cached response.
RUN_TOKEN="${RUN_TOKEN:-$(date +%Y%m%d-%H%M%S)-$$}"

# If the caller supplied both REST creds, hand them to the CLI too (targets a
# bex-api the local CLI config isn't logged into). Otherwise the CLI uses its own
# ~/.render/cli.yaml (with refresh), and the REST legs read that same file.
if [ -n "${RENDER_HOST:-}" ] && [ -n "${RENDER_API_KEY:-}" ]; then
  export RENDER_HOST RENDER_API_KEY
fi

# --------------------------------------------------------------------------- #
# output helpers
# --------------------------------------------------------------------------- #
if [ -t 1 ]; then
  C_RED=$'\033[31m'; C_GRN=$'\033[32m'; C_YEL=$'\033[33m'; C_DIM=$'\033[2m'; C_OFF=$'\033[0m'
else
  C_RED=""; C_GRN=""; C_YEL=""; C_DIM=""; C_OFF=""
fi
FAILURES=0
ts() { date -u +%H:%M:%S; }
log()  { printf '%s %s\n' "${C_DIM}[$(ts)]${C_OFF}" "$*"; }
note() { printf '%s %s\n' "${C_YEL}[$(ts)] …${C_OFF}" "$*"; }
pass() { printf '%s %s\n' "${C_GRN}[$(ts)] PASS${C_OFF}" "$*"; }
fail() { printf '%s %s\n' "${C_RED}[$(ts)] FAIL${C_OFF}" "$*"; FAILURES=$((FAILURES+1)); }
hdr()  { printf '\n\033[1m==== %s ====\033[0m\n' "$*"; }
die()  { printf '%s %s\n' "${C_RED}error:${C_OFF}" "$*" >&2; exit 2; }

# tag a leg with the surface it used (CLI vs REST), so gaps stay visible. Prefix a
# log line with these, e.g. `log "$VIA_REST suspending …"`.
VIA_CLI="${C_DIM}[via CLI ]${C_OFF}"
VIA_REST="${C_DIM}[via REST]${C_OFF}"

# --------------------------------------------------------------------------- #
# CLI + REST plumbing
# --------------------------------------------------------------------------- #
# _yaml_api_field <key>  — pull api.<key> out of the simple 2-space-indented cli.yaml
_yaml_api_field() {
  [ -f "$CLI_YAML" ] || return 1
  awk -v k="$1:" '
    /^api:/      {inapi=1; next}
    inapi && /^[^[:space:]]/ {inapi=0}
    inapi && $1==k {print $2; exit}
  ' "$CLI_YAML"
}
# rest_base — the API ORIGIN (no /v1). Both RENDER_HOST and the cli.yaml host
# carry a trailing /v1/ (the CLI convention), so strip it: paths below are /v1/... .
rest_base() {
  local h="${RENDER_HOST:-}"
  [ -z "$h" ] && h="$(_yaml_api_field host || true)"
  [ -n "$h" ] || die "no REST base: set RENDER_HOST or log the CLI in ($CLI_YAML)"
  h="${h%/}"; h="${h%/v1}"
  printf '%s' "$h"
}
rest_token() {
  if [ -n "${RENDER_API_KEY:-}" ]; then printf '%s' "$RENDER_API_KEY"; return; fi
  local k; k="$(_yaml_api_field key || true)"
  [ -n "$k" ] || die "no REST token: set RENDER_API_KEY or log the CLI in ($CLI_YAML)"
  printf '%s' "$k"
}

# cli <args...> — run the render binary (honors DRY_RUN). Dry-run traces go to
# stderr so command substitution (`$(cli …)`) never captures them.
cli() {
  if [ -n "${DRY_RUN:-}" ]; then echo "  + $RENDER_BIN $*" >&2; return 0; fi
  "$RENDER_BIN" "$@"
}

# rest <METHOD> <PATH> [body] — raw Render-compatible REST; prints ONLY the HTTP
# status to stdout so callers can capture it. PATH begins with /v1/... .
rest() {
  local method="$1" path="$2" body="${3:-}"
  if [ -n "${DRY_RUN:-}" ]; then echo "  + curl -X $method $(rest_base)$path" >&2; echo "202"; return 0; fi
  local args=(-sS -o /dev/null -w '%{http_code}' -X "$method"
    -H "Authorization: Bearer $(rest_token)" -H "Content-Type: application/json")
  [ -n "$body" ] && args+=(--data "$body")
  curl "${args[@]}" "$(rest_base)$path"
}

# rest_json <METHOD> <PATH> — REST call returning the response body (for GETs).
rest_json() {
  local method="$1" path="$2"
  if [ -n "${DRY_RUN:-}" ]; then echo "  + curl -X $method $(rest_base)$path" >&2; echo '{}'; return 0; fi
  curl -sS -X "$method" -H "Authorization: Bearer $(rest_token)" "$(rest_base)$path"
}

# curl_status <url> — GET a public app URL, echo "<http_code>\t<body-first-line>"
# (no auth; -k tolerated so a not-yet-ready cert doesn't mask an app 200).
app_get() { curl -s -m 15 -o /dev/null -w '%{http_code}' "$1" 2>/dev/null || echo "000"; }
app_body() { curl -s -m 15 "$1" 2>/dev/null || true; }

# json_pick <python-expr over `d`> — parse stdin JSON, print an extracted value.
json_pick() { python3 -c 'import sys,json
d=json.load(sys.stdin)
'"$1" 2>/dev/null || true; }

# json_find_id <prefix> — parse stdin JSON and print the first `id` at any depth
# whose value starts with <prefix> (e.g. srv-/dpg-). Empty if none/unparseable.
json_find_id() { python3 -c '
import sys,json
prefix=sys.argv[1]
def walk(o):
    if isinstance(o,dict):
        i=o.get("id")
        if isinstance(i,str) and i.startswith(prefix): print(i); raise SystemExit
        for v in o.values(): walk(v)
    elif isinstance(o,list):
        for v in o: walk(v)
try: walk(json.load(sys.stdin))
except (SystemExit,ValueError): pass' "$1" 2>/dev/null; }

# find the created service id anywhere in a create/list JSON blob by matching name.
service_id_by_name() {
  local name="$1"
  if [ -n "${DRY_RUN:-}" ]; then echo "srv-DRYRUN"; return 0; fi
  cli services --output json 2>/dev/null | python3 -c '
import sys,json
target=sys.argv[1]
def walk(o):
    if isinstance(o,dict):
        if o.get("name")==target and isinstance(o.get("id"),str) and o["id"].startswith("srv-"):
            print(o["id"]); raise SystemExit
        for v in o.values(): walk(v)
    elif isinstance(o,list):
        for v in o: walk(v)
try: walk(json.load(sys.stdin))
except SystemExit: pass
' "$name"
}

# --------------------------------------------------------------------------- #
# per-sample spec — sets globals: SVC_NAME SVC_TYPE HAS_URL HEALTH_PATH
#                    EXPECTED_BODY CREATE_ARGS[] IS_CRON IS_STACK
# --------------------------------------------------------------------------- #
SAMPLES="hello-go hello-node hello-python whoami static-site cron-demo stack-demo"

load_spec() {
  local sample="$1"
  SVC_TYPE="web_service"; HAS_URL=1; HEALTH_PATH="/"; IS_CRON=0; IS_STACK=0
  EXPECTED_BODY="$RUN_TOKEN"
  local msg="bex-lifecycle $sample $RUN_TOKEN"
  SVC_NAME="${sample}${NAME_SUFFIX}"
  CREATE_ARGS=(services create --name "$SVC_NAME" --type "$SVC_TYPE" --confirm --output json)

  case "$sample" in
    hello-go)
      HEALTH_PATH="/"
      CREATE_ARGS+=(--repo "$REPO" --root-directory examples/hello-go --branch "$BRANCH"
                    --runtime docker --plan "$PLAN" --health-check-path "$HEALTH_PATH"
                    --env-var "MESSAGE=$msg") ;;
    hello-node)
      HEALTH_PATH="/"
      CREATE_ARGS+=(--repo "$REPO" --root-directory examples/hello-node --branch "$BRANCH"
                    --runtime docker --plan "$PLAN" --health-check-path "$HEALTH_PATH"
                    --env-var "MESSAGE=$msg") ;;
    hello-python)
      HEALTH_PATH="/healthz"
      CREATE_ARGS+=(--repo "$REPO" --root-directory examples/hello-python --branch "$BRANCH"
                    --runtime docker --plan "$PLAN" --health-check-path "$HEALTH_PATH"
                    --env-var "MESSAGE=$msg") ;;
    whoami)
      # Prebuilt image (skips the build plane). traefik/whoami has no MESSAGE knob,
      # so verify-up asserts its own signature line instead of the run token.
      # It ignores $PORT and defaults to :80 — which bex can't serve: tenant pods
      # drop ALL caps incl. NET_BIND_SERVICE (high ports only, docs/ADR022 / w7/m2),
      # and bex routes an image service to its default 3000. So point whoami at 3000
      # via its own env knob (bex has no Render-style listening-port auto-detection).
      HEALTH_PATH="/"; EXPECTED_BODY="Hostname:"
      CREATE_ARGS+=(--image traefik/whoami --plan "$PLAN" --env-var "WHOAMI_PORT_NUMBER=3000") ;;
    static-site)
      # ADR029: build → object-store origin → shared static-server. No MESSAGE;
      # verify-up asserts the shipped index.html content.
      SVC_TYPE="static_site"; HEALTH_PATH="/"; EXPECTED_BODY="Hello from bex"
      CREATE_ARGS=(services create --name "$SVC_NAME" --type static_site --confirm --output json
                   --repo "$REPO" --root-directory examples/static-site --branch "$BRANCH"
                   --publish-directory .) ;;
    cron-demo)
      # No URL — up = scheduled CronJob; a run emits MESSAGE to logs.
      SVC_TYPE="cron_job"; HAS_URL=0; IS_CRON=1; EXPECTED_BODY="$RUN_TOKEN"
      # The Render CLI requires BOTH --cron-schedule AND --cron-command (client-side
      # validation), even though cron-demo's image already has its own ENTRYPOINT
      # (/cron-demo). Pass that same binary as the command (documented CLI gap: an
      # image-entrypoint cron still needs an explicit command via the CLI).
      CREATE_ARGS=(services create --name "$SVC_NAME" --type cron_job --confirm --output json
                   --repo "$REPO" --root-directory examples/cron-demo --branch "$BRANCH"
                   --runtime docker --plan "$PLAN"
                   --cron-schedule "$CRON_SCHEDULE" --cron-command "/cron-demo"
                   --env-var "MESSAGE=$msg") ;;
    stack-demo)
      # web + worker + postgres — composed leg-by-leg (the CLI has no blueprint
      # apply). Handled by the stack_* leg overrides below; the web service serves
      # "db ok: SELECT 1 = 1" through the injected DATABASE_URL.
      IS_STACK=1; HEALTH_PATH="/healthz"; EXPECTED_BODY="db ok: SELECT 1 = 1"
      SVC_NAME="stack-web${NAME_SUFFIX}" ;;
    *) die "unknown sample: $sample (known: $SAMPLES)" ;;
  esac
  SVC_URL="https://${SVC_NAME}.${BASE_DOMAIN}"
}

# --------------------------------------------------------------------------- #
# polling
# --------------------------------------------------------------------------- #
# poll <desc> <fn...> — call fn until it returns 0 or POLL_TIMEOUT elapses.
poll() {
  local desc="$1"; shift
  if [ -n "${DRY_RUN:-}" ]; then echo "  ${C_DIM}(dry-run) would poll until: $desc${C_OFF}"; return 0; fi
  local deadline=$(( $(date +%s) + POLL_TIMEOUT ))
  while :; do
    if "$@"; then return 0; fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
      note "timed out after ${POLL_TIMEOUT}s waiting for: $desc"
      return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}
_body_has_token() { app_body "$1" | grep -qF "$EXPECTED_BODY"; }
_body_lacks_token() { ! app_body "$1" | grep -qF "$EXPECTED_BODY"; }

# --------------------------------------------------------------------------- #
# lifecycle legs (a plain web/static service; cron + stack override where noted)
# --------------------------------------------------------------------------- #
CREATED_ID=""       # set by leg_deploy; used by suspend/resume/delete
DELETED=0

leg_deploy() {
  hdr "$SVC_NAME — deploy  (verb: services create)"
  log "$VIA_CLI creating: $SVC_NAME (type=$SVC_TYPE)"
  if [ -n "${PRECLEAN:-}" ]; then
    local pre; pre="$(service_id_by_name "$SVC_NAME" || true)"
    [ -n "$pre" ] && { note "PRECLEAN: deleting pre-existing $SVC_NAME ($pre)"; cli services delete "$pre" --confirm >/dev/null 2>&1 || true; sleep 5; }
  fi
  local out; out="$(cli "${CREATE_ARGS[@]}" 2>&1)" || { fail "create failed: $out"; return 1; }
  [ -n "${DRY_RUN:-}" ] && { CREATED_ID="srv-DRYRUN"; pass "deploy (dry-run)"; return 0; }
  # The create output already carries the new service id; fall back to a by-name
  # list scan only if the JSON shape surprises us.
  CREATED_ID="$(printf '%s' "$out" | json_find_id srv-)"
  [ -z "$CREATED_ID" ] && CREATED_ID="$(service_id_by_name "$SVC_NAME" || true)"
  [ -z "$CREATED_ID" ] && { fail "created but could not resolve service id from output"; return 1; }
  log "service id: $CREATED_ID"
  # Wait for the deploy to reach live (repo samples build first; the CLI's own
  # `deploys create --wait` gates on the same terminal state, but create already
  # kicked the first deploy — poll the service until its URL serves, below).
  pass "deploy accepted ($CREATED_ID)"
}

leg_verify_up() {
  hdr "$SVC_NAME — verify up"
  if [ "$HAS_URL" -eq 0 ]; then
    verify_up_cron; return
  fi
  log "$VIA_CLI polling $SVC_URL for token: $EXPECTED_BODY"
  if poll "$SVC_URL to serve the fresh deploy" _body_has_token "$SVC_URL"; then
    pass "up: $SVC_URL serves expected body ($(app_get "$SVC_URL") $EXPECTED_BODY)"
  else
    fail "up: $SVC_URL never served expected body ($EXPECTED_BODY); last status $(app_get "$SVC_URL")"
    return 1
  fi
}

leg_suspend() {
  hdr "$SVC_NAME — suspend  (verb: POST /v1/services/{id}/suspend — CLI has no service suspend)"
  log "$VIA_REST suspending $CREATED_ID"
  local code; code="$(rest POST "/v1/services/$CREATED_ID/suspend")"
  if [ "$code" = "202" ] || [ "$code" = "200" ]; then pass "suspend accepted (HTTP $code)"; else fail "suspend HTTP $code (expected 202)"; return 1; fi
}

leg_verify_down() {
  hdr "$SVC_NAME — verify down"
  if [ "$HAS_URL" -eq 0 ]; then verify_down_cron; return; fi
  if [ -n "${DRY_RUN:-}" ]; then echo "  ${C_DIM}(dry-run) would confirm $SVC_URL sustained-down${C_OFF}"; pass "down (dry-run)"; return 0; fi
  # Require the app to STOP serving the token for DOWN_CONFIRM consecutive checks
  # before calling it down — a single transient 502 during the suspend reconcile
  # (or a mid-restart blip) must not be mistaken for a settled suspended state, or
  # the following resume would race an incompletely-suspended service.
  log "$VIA_CLI polling $SVC_URL to stop serving the app (need ${DOWN_CONFIRM}x sustained, ${POLL_INTERVAL}s apart)"
  local deadline=$(( $(date +%s) + POLL_TIMEOUT )) streak=0
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if _body_lacks_token "$SVC_URL"; then
      streak=$((streak+1))
      if [ "$streak" -ge "$DOWN_CONFIRM" ]; then
        pass "down: $SVC_URL stopped serving the app for ${DOWN_CONFIRM} consecutive checks (last status $(app_get "$SVC_URL"))"
        return 0
      fi
    else
      [ "$streak" -gt 0 ] && note "down: app reappeared after $streak down check(s) — resetting (suspend not yet settled)"
      streak=0
    fi
    sleep "$POLL_INTERVAL"
  done
  fail "down: $SVC_URL never stayed down for ${DOWN_CONFIRM} consecutive checks after suspend"
  return 1
}

leg_resume() {
  hdr "$SVC_NAME — resume  (verb: POST /v1/services/{id}/resume — CLI has no service resume)"
  log "$VIA_REST resuming $CREATED_ID"
  local code; code="$(rest POST "/v1/services/$CREATED_ID/resume")"
  if [ "$code" = "202" ] || [ "$code" = "200" ]; then pass "resume accepted (HTTP $code)"; else fail "resume HTTP $code (expected 202)"; return 1; fi
}

leg_delete() {
  hdr "$SVC_NAME — delete  (verb: services delete)"
  log "$VIA_CLI deleting $CREATED_ID"
  if cli services delete "$CREATED_ID" --confirm >/dev/null 2>&1; then DELETED=1; pass "delete accepted"; else fail "delete failed"; return 1; fi
}

leg_verify_gone() {
  hdr "$SVC_NAME — verify gone"
  # 1) service absent from the list (API check, always available)
  if poll "$SVC_NAME to leave the service list" _service_absent "$SVC_NAME"; then
    pass "gone: $SVC_NAME absent from services list"
  else
    fail "gone: $SVC_NAME still present in services list"
  fi
  # 2) URL dead (only meaningful for URL-bearing samples). Poll rather than check
  #    once: a static_site keeps serving from the shared static-server's cached
  #    host→site snapshot until its next resync (~10s) after the App is deleted,
  #    and DNS/Ingress teardown can lag too — a single immediate check races that.
  if [ "$HAS_URL" -eq 1 ] && [ -z "${DRY_RUN:-}" ]; then
    if poll "$SVC_URL to stop serving (200 → dead)" _url_not_200 "$SVC_URL"; then
      pass "gone: $SVC_URL dead (status $(app_get "$SVC_URL"))"
    else
      fail "gone: $SVC_URL still serves 200 after delete"
    fi
  fi
  # 3) cluster residue — only if a kubeconfig is reachable; else API-only (degrade)
  verify_no_residue
}
_service_absent() { [ -z "$(service_id_by_name "$1" || true)" ]; }
_url_not_200() { [ "$(app_get "$1")" != "200" ]; }

# --------------------------------------------------------------------------- #
# cron-specific up/down (no URL — prove via schedule + a run's log line)
# --------------------------------------------------------------------------- #
verify_up_cron() {
  log "$VIA_CLI cron: confirming $SVC_NAME is scheduled + a run emitted its MESSAGE"
  # Scheduled: the service exists and is not suspended.
  if _service_absent "$SVC_NAME"; then fail "cron up: $SVC_NAME not in service list"; return 1; fi
  # A run within one schedule window (5m) writes MESSAGE to app logs. Poll logs.
  if poll "a cron run to emit its MESSAGE to logs" _cron_ran; then
    pass "cron up: a run emitted MESSAGE ($EXPECTED_BODY)"
  else
    note "cron up: no run observed within ${POLL_TIMEOUT}s (schedule is */5m) — recording as partial"
    fail "cron up: no run's MESSAGE seen in logs within the poll budget"
    return 1
  fi
}
_cron_ran() { cli logs --resources "$CREATED_ID" --output json --limit 100 2>/dev/null | grep -qF "$EXPECTED_BODY"; }
verify_down_cron() {
  log "$VIA_CLI cron: confirming no NEW run fires while suspended"
  # Record the current run count marker, wait a schedule window, assert no growth.
  local before after
  # grep -c prints the count (0 on no match) but exits 1 when there are no matches;
  # `|| true` keeps set -e happy without appending a second "0".
  before="$(cli logs --resources "$CREATED_ID" --output json --limit 200 2>/dev/null | grep -cF "$EXPECTED_BODY" || true)"
  note "cron down: baseline run-line count=$before; waiting ${CRON_WINDOW}s (one+ schedule window) for NO new run"
  if [ -z "${DRY_RUN:-}" ]; then sleep "$CRON_WINDOW"; fi
  after="$(cli logs --resources "$CREATED_ID" --output json --limit 200 2>/dev/null | grep -cF "$EXPECTED_BODY" || true)"
  if [ "$after" -le "$before" ]; then pass "cron down: no new run while suspended ($before → $after)"; else fail "cron down: a run fired while suspended ($before → $after)"; return 1; fi
}

# --------------------------------------------------------------------------- #
# stack-demo — postgres + web + worker, composed leg-by-leg
# --------------------------------------------------------------------------- #
STACK_DB="stack-db${NAME_SUFFIX}"
STACK_WORKER="stack-worker${NAME_SUFFIX}"
STACK_DB_ID=""; STACK_WORKER_ID=""

stack_deploy() {
  hdr "stack-demo — deploy postgres + web + worker"
  if [ -n "${DRY_RUN:-}" ]; then
    cli postgres create --name "$STACK_DB" --plan "$PLAN" --confirm --output json >/dev/null
    cli services create --name "$SVC_NAME" --type web_service --repo "$REPO" --root-directory examples/stack-demo --env-var "DATABASE_URL=<from $STACK_DB>" >/dev/null
    cli services create --name "$STACK_WORKER" --type background_worker --repo "$REPO" --env-var ROLE=worker >/dev/null
    STACK_DB_ID="dpg-DRYRUN"; CREATED_ID="srv-DRYRUN-web"; STACK_WORKER_ID="srv-DRYRUN-worker"
    pass "stack deploy (dry-run)"; return 0
  fi
  # 1) Postgres (CLI postgres create). Newest CLI: `render postgres create`.
  log "$VIA_CLI creating postgres $STACK_DB"
  local dbout; dbout="$(cli postgres create --name "$STACK_DB" --plan "$PLAN" --confirm --output json 2>&1)" || { fail "postgres create failed: $dbout"; return 1; }
  STACK_DB_ID="$(printf '%s' "$dbout" | json_find_id dpg-)"
  [ -z "$STACK_DB_ID" ] && { fail "postgres created but id unresolved"; return 1; }
  log "postgres id: $STACK_DB_ID — waiting for its connection string"
  # 2) The connection string (fromDatabase in the blueprint; here, read + inject).
  local conn=""
  poll "postgres $STACK_DB to expose a connection string" _stack_db_conn || true
  conn="$STACK_CONN"
  [ -z "$conn" ] && { fail "postgres connection string never became available"; return 1; }
  # 3) web + worker (CLI services create, DATABASE_URL wired explicitly since the
  #    CLI has no fromDatabase reference — a documented compose-what-the-CLI-can gap).
  log "$VIA_CLI creating web $SVC_NAME (DATABASE_URL wired from $STACK_DB)"
  local webout; webout="$(cli services create --name "$SVC_NAME" --type web_service --confirm --output json \
    --repo "$REPO" --root-directory examples/stack-demo --branch "$BRANCH" --runtime docker --plan "$PLAN" \
    --health-check-path "$HEALTH_PATH" --env-var "DATABASE_URL=$conn" 2>&1)" || { fail "web create failed: $webout"; return 1; }
  CREATED_ID="$(printf '%s' "$webout" | json_find_id srv-)"
  log "$VIA_CLI creating worker $STACK_WORKER"
  local wkout; wkout="$(cli services create --name "$STACK_WORKER" --type background_worker --confirm --output json \
    --repo "$REPO" --root-directory examples/stack-demo --branch "$BRANCH" --runtime docker --plan "$PLAN" \
    --env-var "ROLE=worker" --env-var "DATABASE_URL=$conn" 2>&1)" || { fail "worker create failed: $wkout"; return 1; }
  STACK_WORKER_ID="$(printf '%s' "$wkout" | json_find_id srv-)"
  [ -z "$CREATED_ID" ] && { fail "web service id unresolved"; return 1; }
  pass "stack deploy accepted (db=$STACK_DB_ID web=$CREATED_ID worker=${STACK_WORKER_ID:-?})"
}
STACK_CONN=""
_stack_db_conn() {
  # Prefer the INTERNAL connection string: web+worker run in-cluster, so they reach
  # CNPG over the internal Service (the external string needs BEX_DB_DOMAIN + the
  # SNI proxy + an IP allow-list). This is the CLI-compose stand-in for the
  # blueprint's fromDatabase secretRef — the credential is injected as a plain env
  # var here (documented divergence: blueprint keeps it in the CNPG Secret).
  STACK_CONN="$(rest_json GET "/v1/postgres/$STACK_DB_ID/connection-info" | json_pick '
print(d.get("internalConnectionString") or d.get("externalConnectionString") or d.get("connectionString") or "")')"
  [ -n "$STACK_CONN" ]
}
stack_delete() {
  hdr "stack-demo — delete web + worker + postgres"
  local ok=1
  log "$VIA_CLI deleting web $CREATED_ID + worker ${STACK_WORKER_ID:-none} (verb: services delete)"
  cli services delete "$CREATED_ID" --confirm >/dev/null 2>&1 || ok=0
  [ -n "$STACK_WORKER_ID" ] && { cli services delete "$STACK_WORKER_ID" --confirm >/dev/null 2>&1 || ok=0; }
  log "$VIA_CLI deleting postgres $STACK_DB_ID (verb: postgres delete)"
  cli postgres delete "$STACK_DB_ID" --confirm >/dev/null 2>&1 || ok=0
  if [ "$ok" -eq 1 ]; then DELETED=1; pass "stack delete accepted"; else fail "stack delete had errors"; fi
}
# stack suspend/resume: web+worker via service REST, postgres via CLI (supported).
stack_suspend() {
  hdr "stack-demo — suspend web+worker (REST) + postgres (CLI)"
  log "$VIA_REST suspending web+worker; $VIA_CLI postgres suspend $STACK_DB_ID"
  rest POST "/v1/services/$CREATED_ID/suspend" >/dev/null
  [ -n "$STACK_WORKER_ID" ] && rest POST "/v1/services/$STACK_WORKER_ID/suspend" >/dev/null
  cli postgres suspend "$STACK_DB_ID" --confirm >/dev/null 2>&1 || note "postgres suspend returned non-zero"
  pass "stack suspend issued"
}
stack_resume() {
  hdr "stack-demo — resume web+worker (REST) + postgres (CLI)"
  log "$VIA_CLI postgres resume $STACK_DB_ID; $VIA_REST resuming web+worker"
  cli postgres resume "$STACK_DB_ID" --confirm >/dev/null 2>&1 || note "postgres resume returned non-zero"
  rest POST "/v1/services/$CREATED_ID/resume" >/dev/null
  [ -n "$STACK_WORKER_ID" ] && rest POST "/v1/services/$STACK_WORKER_ID/resume" >/dev/null
  pass "stack resume issued"
}

# --------------------------------------------------------------------------- #
# cluster residue (verify-gone step 3) — degrades to API-only w/o a kubeconfig
# --------------------------------------------------------------------------- #
verify_no_residue() {
  # Opt-in only: the residue check must run against the SAME cluster the services
  # were created on. A laptop's default kube context (e.g. a local kind cluster) is
  # NOT prod — checking it would falsely report "no residue" for a prod service. So
  # require the operator to assert the current context is the target bex cluster.
  if [ -z "${RESIDUE_KUBE:-}" ]; then
    note "residue check API-only (set RESIDUE_KUBE=1 with kubectl pointed at the bex cluster to also check pods/Secret/htpasswd)"
    return 0
  fi
  if [ -n "${DRY_RUN:-}" ]; then echo "  ${C_DIM}(dry-run) would kubectl-check pods/reg-pull/htpasswd for $SVC_NAME${C_OFF}"; return 0; fi
  if ! command -v kubectl >/dev/null 2>&1 || ! kubectl version >/dev/null 2>&1; then
    note "RESIDUE_KUBE set but no reachable kubeconfig — residue check limited to the API"
    return 0
  fi
  log "residue check against kube context: $(kubectl config current-context 2>/dev/null || echo '?')"
  local n="$SVC_NAME"
  # pods for the App
  local pods; pods="$(kubectl -n "$APPS_NAMESPACE" get pods -l "app.bex.co/name=$n" --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  [ "${pods:-0}" -eq 0 ] && pass "gone: no pods for $n in ns/$APPS_NAMESPACE" || fail "gone: $pods pod(s) still present for $n"
  # reg-pull-<name> Secret
  if kubectl -n "$APPS_NAMESPACE" get secret "reg-pull-$n" >/dev/null 2>&1; then fail "gone: reg-pull-$n Secret still present"; else pass "gone: no reg-pull-$n Secret"; fi
  # app-<name> htpasswd user (Zot htpasswd Secret in the registry ns)
  if kubectl -n "$REGISTRY_NS" get secret zot-htpasswd -o jsonpath='{.data.htpasswd}' 2>/dev/null | base64 -d 2>/dev/null | grep -q "^app-$n:"; then
    fail "gone: app-$n htpasswd user still present"
  else
    pass "gone: no app-$n htpasswd user"
  fi
}

# --------------------------------------------------------------------------- #
# orchestration
# --------------------------------------------------------------------------- #
cleanup_on_exit() {
  # Best-effort: never leave a created service running on prod after a failed run.
  [ -n "${KEEP:-}" ] && return
  [ "$DELETED" -eq 1 ] && return
  [ -z "$CREATED_ID" ] && return
  note "cleanup: deleting leftover $CREATED_ID (a leg failed before delete)"
  cli services delete "$CREATED_ID" --confirm >/dev/null 2>&1 || true
  [ "${IS_STACK:-0}" -eq 1 ] && {
    [ -n "$STACK_WORKER_ID" ] && cli services delete "$STACK_WORKER_ID" --confirm >/dev/null 2>&1 || true
    [ -n "$STACK_DB_ID" ] && cli postgres delete "$STACK_DB_ID" --confirm >/dev/null 2>&1 || true
  }
}

run_one() {
  local sample="$1"
  FAILURES=0; CREATED_ID=""; DELETED=0
  load_spec "$sample"
  printf '\n\033[1m######## lifecycle: %s (run %s) ########\033[0m\n' "$sample" "$RUN_TOKEN"
  log "target: $(rest_base)  |  URL: ${HAS_URL:+$SVC_URL}${HAS_URL:+}"
  trap cleanup_on_exit EXIT

  # A failed deploy leaves no service to suspend/resume/delete — running the rest
  # would emit misleading results (empty-id 405s, a false "gone" PASS). So gate the
  # lifecycle on deploy success; the EXIT trap still cleans up any partial create.
  if [ "$IS_STACK" -eq 1 ]; then
    if stack_deploy; then
      leg_verify_up  || true
      stack_suspend  || true
      leg_verify_down|| true
      stack_resume   || true
      leg_verify_up  || true
      [ -n "${KEEP:-}" ] || { stack_delete || true; leg_verify_gone || true; }
    fi
  else
    if leg_deploy; then
      leg_verify_up   || true
      leg_suspend     || true
      leg_verify_down || true
      leg_resume      || true
      leg_verify_up   || true
      [ -n "${KEEP:-}" ] || { leg_delete || true; leg_verify_gone || true; }
    fi
  fi
  trap - EXIT

  echo
  if [ "$FAILURES" -eq 0 ]; then
    pass "$sample — full lifecycle GREEN"
    return 0
  else
    fail "$sample — $FAILURES leg(s) failed"
    return 1
  fi
}

main() {
  local arg="${1:-}"
  case "$arg" in
    ""|-h|--help) sed -n '2,60p' "$0"; exit 0 ;;
    list) echo "$SAMPLES" | tr ' ' '\n'; exit 0 ;;
    all)
      local rc=0
      for s in $SAMPLES; do run_one "$s" || rc=1; done
      echo; [ "$rc" -eq 0 ] && pass "ALL samples GREEN" || fail "one or more samples had failures"
      exit "$rc" ;;
    *) run_one "$arg"; exit $?;;
  esac
}
main "$@"
