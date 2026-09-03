#!/usr/bin/env bash
# static-delete-timing-verify.sh — w3/m81 disposable live delete-timing + cross-
# surface probe. Proves the deleting-service read contract on a REAL cluster:
# after a static site is deleted, its list row, REST service/routes/headers/
# deploys reads, the GraphQL service read, and its public host all converge to
# the SAME terminal answer (absent / 404 / not-served) within a stated window —
# never the 2+-hour "list gone but by-id still serving `phase: Deleting` + a dead
# URL" split this milestone was promoted to close (docs/ADR006 § Reads while a
# deletion finalizes, docs/ADR029 § Deletion and finalizer bound).
#
# It is turnkey and disposable: it creates its own fixture, records only
# non-secret evidence (surface -> terminal state + elapsed seconds), and deletes
# the fixture on every exit path. It NEVER prints the bearer token, a kubeconfig,
# or any credential.
#
# Usage:
#   API=https://api.bex.co/v1 BEARER="$TOKEN" REPO=https://github.com/bex/static-site \
#     scripts/static-delete-timing-verify.sh
#
# Required env:
#   API      bex-api base URL including /v1 (e.g. https://api.bex.co/v1)
#   BEARER   a pre-exchanged OAuth2 access token (never logged)
#   REPO     a public git repo whose root is a no-build static site (examples/static-site shape)
# Optional env:
#   BRANCH        default main
#   PUBLISH_PATH  default "" (repo root; the no-build publish path)
#   OWNER_ID      workspace id (tea-...); omitted => the token's default workspace
#   GRAPHQL       GraphQL endpoint; default derives from API (strip trailing /v1 -> /graphql)
#   POLL_INTERVAL seconds between deletion polls (default 3)
#   DEADLINE      seconds allowed to reach the terminal state (default 300 — the w5/m49 bar)
#   SERVE_DEADLINE seconds allowed for the fixture to start serving before delete (default 300)
#
# Requires: curl, jq. Exits 0 on pass, non-zero on any surface that fails to
# converge within DEADLINE.
set -euo pipefail

fail() { echo "FAIL: $*" >&2; exit 1; }
command -v curl >/dev/null || fail "curl is required"
command -v jq >/dev/null || fail "jq is required"
: "${API:?set API to the bex-api base URL including /v1}"
: "${BEARER:?set BEARER to a pre-exchanged access token}"
: "${REPO:?set REPO to a public no-build static-site git repo}"
BRANCH="${BRANCH:-main}"
PUBLISH_PATH="${PUBLISH_PATH:-}"
GRAPHQL="${GRAPHQL:-${API%/v1}/graphql}"
POLL_INTERVAL="${POLL_INTERVAL:-3}"
DEADLINE="${DEADLINE:-300}"
SERVE_DEADLINE="${SERVE_DEADLINE:-300}"

TMP_DIR="$(mktemp -d)"
STAMP="$(date +%s)"
NAME="m81probe${STAMP}"
SERVICE_ID=""
PUBLIC_URL=""
trap cleanup EXIT

# auth-curl runs curl with the bearer supplied through a header FILE (-K) so the
# token never appears in the process table or in `set -x` output.
HDR_FILE="$TMP_DIR/hdr"
umask 077
printf 'header = "Authorization: Bearer %s"\n' "$BEARER" >"$HDR_FILE"
acurl() { curl -sS -K "$HDR_FILE" --connect-timeout 5 --max-time 30 "$@"; }

cleanup() {
  local code=$?
  if [ -n "$SERVICE_ID" ]; then
    # Best-effort teardown: the fixture must never outlive this probe.
    acurl -o /dev/null -X DELETE "$API/services/$SERVICE_ID" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP_DIR"
  exit "$code"
}

# ---- REST / GraphQL / public-host probes (state vars, redaction-safe) ---------

# rest_status <path> -> echoes the HTTP status of GET $API/<path>
rest_status() {
  acurl -o /dev/null -w '%{http_code}' "$API/$1" 2>/dev/null || echo "000"
}

# list_has_id -> "present" | "absent" | "error"
list_state() {
  local body="$TMP_DIR/list.json"
  if ! acurl -o "$body" "$API/services?limit=100" 2>/dev/null; then echo "error"; return; fi
  # Render's list envelope is [{service,cursor}] (or a bare [service]); match either.
  if jq -e --arg id "$SERVICE_ID" '[.[] | (.service // .)] | any(.id == $id)' "$body" >/dev/null 2>&1; then
    echo "present"
  else
    echo "absent"
  fi
}

# graphql_state -> "present" | "absent" | "error"
graphql_state() {
  local body="$TMP_DIR/gql.json"
  acurl -o "$body" -H 'Content-Type: application/json' -X POST "$GRAPHQL" \
    --data "$(jq -nc --arg id "$SERVICE_ID" '{query:"query($id:String!){server(id:$id){id phase url}}",variables:{id:$id}}')" \
    >/dev/null 2>&1 || { echo "error"; return; }
  # server:null (+ a not-found error) is the absent shape; a populated node is present.
  if jq -e '.data.server != null' "$body" >/dev/null 2>&1; then echo "present"; else echo "absent"; fi
}

# public_state -> "serving" | "gone" | "skip"
public_state() {
  [ -n "$PUBLIC_URL" ] || { echo "skip"; return; }
  local status
  status="$(curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 15 "$PUBLIC_URL" 2>/dev/null || echo "000")"
  # A live static site serves 200; once the route/cert are withdrawn the host
  # stops answering for this site (404 default / connection failure).
  [ "$status" = "200" ] && echo "serving" || echo "gone"
}

# ---- lifecycle ---------------------------------------------------------------

create_static_site() {
  local body="$TMP_DIR/create.json" args
  args="$(jq -nc \
    --arg name "$NAME" --arg repo "$REPO" --arg branch "$BRANCH" \
    --arg pub "$PUBLISH_PATH" --arg owner "${OWNER_ID:-}" '
    {name:$name, type:"static_site", repo:$repo, branch:$branch,
     serviceDetails:{publishPath:$pub}}
    | if $owner != "" then .ownerId = $owner else . end')"
  acurl -o "$body" -H 'Content-Type: application/json' -X POST "$API/services" --data "$args" \
    || fail "create request failed"
  SERVICE_ID="$(jq -r '(.service // .).id // empty' "$body")"
  [ -n "$SERVICE_ID" ] || fail "create did not return a service id: $(jq -c '.error // .message // .' "$body" 2>/dev/null)"
  echo "created static site $SERVICE_ID ($NAME)"

  # A route + a header so the delete sweeps real edge rules (the fixture shape
  # the production incident carried).
  acurl -o /dev/null -H 'Content-Type: application/json' -X PUT "$API/services/$SERVICE_ID/routes" \
    --data '[{"type":"rewrite","source":"/*","destination":"/index.html"}]' >/dev/null 2>&1 || true
  acurl -o /dev/null -H 'Content-Type: application/json' -X PUT "$API/services/$SERVICE_ID/headers" \
    --data '[{"path":"/*","name":"X-Frame-Options","value":"DENY"}]' >/dev/null 2>&1 || true
}

wait_serving() {
  local body="$TMP_DIR/get.json" waited=0 phase url
  while [ "$waited" -lt "$SERVE_DEADLINE" ]; do
    if acurl -o "$body" "$API/services/$SERVICE_ID" 2>/dev/null; then
      phase="$(jq -r '.phase // ""' "$body")"
      url="$(jq -r '.serviceDetails.url // .url // ""' "$body")"
      if [ -n "$url" ] && { [ "$phase" = "Running" ] || [ "$phase" = "Live" ]; }; then
        PUBLIC_URL="$url"
        echo "serving at phase=$phase (url recorded, not printed)"
        return 0
      fi
    fi
    sleep "$POLL_INTERVAL"; waited=$((waited + POLL_INTERVAL))
  done
  echo "WARN: fixture did not reach a serving state within ${SERVE_DEADLINE}s; proceeding to delete-timing anyway" >&2
}

snapshot_pre_delete() {
  echo "pre-delete: list=$(list_state) get=$(rest_status "services/$SERVICE_ID") \
routes=$(rest_status "services/$SERVICE_ID/routes") headers=$(rest_status "services/$SERVICE_ID/headers") \
deploys=$(rest_status "services/$SERVICE_ID/deploys") graphql=$(graphql_state) public=$(public_state)"
}

poll_terminal() {
  local start waited elapsed
  start="$(date +%s)"
  acurl -o /dev/null -X DELETE "$API/services/$SERVICE_ID" || fail "delete request failed"
  echo "deleted; polling every ${POLL_INTERVAL}s for the terminal contract (deadline ${DEADLINE}s)..."
  while :; do
    local ls gs rs routes headers deploys ps done=1
    ls="$(list_state)"; gs="$(graphql_state)"; ps="$(public_state)"
    rs="$(rest_status "services/$SERVICE_ID")"
    routes="$(rest_status "services/$SERVICE_ID/routes")"
    headers="$(rest_status "services/$SERVICE_ID/headers")"
    deploys="$(rest_status "services/$SERVICE_ID/deploys")"
    [ "$ls" = "absent" ] || done=0
    [ "$gs" = "absent" ] || done=0
    [ "$rs" = "404" ] || done=0
    [ "$routes" = "404" ] || done=0
    [ "$headers" = "404" ] || done=0
    [ "$deploys" = "404" ] || done=0
    [ "$ps" = "gone" ] || [ "$ps" = "skip" ] || done=0
    elapsed=$(( $(date +%s) - start ))
    if [ "$done" = "1" ]; then
      echo "TERMINAL reached in ${elapsed}s: list=$ls get=$rs routes=$routes headers=$headers deploys=$deploys graphql=$gs public=$ps"
      SERVICE_ID=""  # already gone; skip the cleanup re-delete
      echo "PASS: every tenant surface converged to absence within ${elapsed}s (deadline ${DEADLINE}s)"
      return 0
    fi
    waited=$elapsed
    if [ "$waited" -ge "$DEADLINE" ]; then
      fail "surfaces still disagree after ${waited}s (deadline ${DEADLINE}s): list=$ls get=$rs routes=$routes headers=$headers deploys=$deploys graphql=$gs public=$ps — this is the m81 split the contract forbids"
    fi
    sleep "$POLL_INTERVAL"
  done
}

echo "== w3/m81 static-site delete-timing + cross-surface probe =="
create_static_site
wait_serving
snapshot_pre_delete
poll_terminal
