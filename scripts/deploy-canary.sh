#!/usr/bin/env bash
# Weekly deploy canary (w3/m83/t004): exercise the product's core promise end to
# end on a schedule — a git repo becomes a running HTTPS service, and then
# disappears cleanly.
#
# This is the probe that covers what the six-hourly tenant-view canary cannot.
# tenant-view-liveness.sh watches a service that ALREADY exists; a build/deploy
# pipeline can be completely broken without moving that needle. It also
# discharges the two live probes still owed on the record — w3/m46 t008
# (static-site behaviours) and w3/m81 t004 (delete timing) — by running them on
# a schedule instead of "in a future QA run".
#
# Five stages, each with its own exit code so the workflow's issue names the
# broken step:
#
#   1 (exit 3) CREATE   POST /v1/services from this repo's examples/hello-go
#   2 (exit 4) READY    poll deploys until `live`, bounded by the build SLO
#   3 (exit 5) SERVE    GET the assigned public host → 200
#   4 (exit 6) ABSENCE  DELETE, then every read surface must converge to
#                       absence within the w5/m49 five-minute window: by-id 404,
#                       list row gone, public host no longer served. This is the
#                       w3/m81 contract — the incident it closed was "list gone
#                       but by-id still serving `phase: Deleting` with a dead
#                       URL" for two hours.
#   5 (exit 7) STATIC   the static-site variant, via
#                       scripts/static-delete-timing-verify.sh
#
# Two safety properties, both non-negotiable because this probe WRITES to
# production:
#
#   * It refuses to run at all unless the API key's visible workspaces include
#     BEX_CANARY_WORKSPACE_ID (canary_assert_workspace). A mis-set secret must
#     never create or delete anything in a real tenant's workspace.
#   * Cleanup is unconditional (trap). The fixture must never outlive the probe,
#     even when an assertion fails mid-flight.
#
# Usage:   scripts/deploy-canary.sh
# Env:     BEX_API_URL              default https://api.bex.co
#          BEX_OAUTH_ISSUER         default https://oauth.bex.co
#          BEX_CANARY_API_KEY       <key-id>:<key-secret>, canary workspace only
#          BEX_CANARY_WORKSPACE_ID  tea-… of the canary workspace (required)
#          BEX_CANARY_REPO          default https://github.com/bex-co/bex
#          BEX_CANARY_BRANCH        default main
#          BEX_CANARY_ROOT_DIR      default examples/hello-go
#          BEX_CANARY_READY_DEADLINE  seconds for build+deploy (default 900)
#          BEX_CANARY_SERVE_DEADLINE  seconds for the first 200 (default 300)
#          BEX_CANARY_ABSENCE_DEADLINE seconds to converge after DELETE (default 300)
#          BEX_CANARY_STATIC_REPO   a public no-build static-site repo; unset =>
#                       the static variant is SKIPPED with a notice (the fixture
#                       repo is still owed — see docs/ADR088 §6).
# Exit:    0 pass · 2 config error · 3 create · 4 ready · 5 serve · 6 absence · 7 static
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=scripts/lib/canary-api.sh
. scripts/lib/canary-api.sh

API="$(canary_api_url)"
REPO="${BEX_CANARY_REPO:-https://github.com/bex-co/bex}"
BRANCH="${BEX_CANARY_BRANCH:-main}"
ROOT_DIR="${BEX_CANARY_ROOT_DIR:-examples/hello-go}"
READY_DEADLINE="${BEX_CANARY_READY_DEADLINE:-900}"
SERVE_DEADLINE="${BEX_CANARY_SERVE_DEADLINE:-300}"
ABSENCE_DEADLINE="${BEX_CANARY_ABSENCE_DEADLINE:-300}"
POLL=5

canary_require_tools

TMP_DIR="$(mktemp -d)"
SERVICE_ID=""
PUBLIC_URL=""

cleanup() {
  local code=$?
  if [ -n "$SERVICE_ID" ]; then
    # Unconditional teardown. Best-effort and non-failing so it can never mask
    # the assertion that actually broke; stage 4 asserts the delete separately.
    echo "  cleanup: deleting $SERVICE_ID"
    acurl -o /dev/null -X DELETE "$API/v1/services/$SERVICE_ID" >/dev/null 2>&1 || true
  fi
  rm -rf -- "$TMP_DIR"
  exit "$code"
}
trap cleanup EXIT

canary_login "$TMP_DIR"
canary_assert_workspace

NAME="canary$(date -u +%m%d%H%M)"

echo "== w3/m83 deploy canary: $REPO ($ROOT_DIR) =="

# --- Stage 1: create from git --------------------------------------------------
echo "==> 1/5 create: POST /v1/services (free web service, autoDeploy off)"
# autoDeploy off: this fixture must deploy exactly once, on create. A git push
# to the canary repo is not the event under test here.
body="$(jq -nc \
  --arg owner "$BEX_CANARY_WORKSPACE_ID" \
  --arg name "$NAME" \
  --arg repo "$REPO" \
  --arg branch "$BRANCH" \
  --arg root "$ROOT_DIR" '
  {ownerId: $owner, type: "web_service", name: $name, repo: $repo,
   branch: $branch, rootDir: $root, autoDeploy: "no", plan: "free",
   serviceDetails: {plan: "free", runtime: "docker", numInstances: 1,
                    healthCheckPath: "/"}}')"
if ! acurl -o "$TMP_DIR/create.json" -H 'Content-Type: application/json' \
  -X POST "$API/v1/services" --data "$body"; then
  echo "FAIL(1/5 create): POST /v1/services failed"
  exit 3
fi
SERVICE_ID="$(jq -r '(.service // .).id // empty' "$TMP_DIR/create.json")"
if [ -z "$SERVICE_ID" ]; then
  echo "FAIL(1/5 create): no service id in the response: $(jq -c '.error // .message // .' "$TMP_DIR/create.json" 2>/dev/null)"
  exit 3
fi
echo "  ok: created $SERVICE_ID ($NAME)"

# --- Stage 2: the deploy reaches `live` ---------------------------------------
echo "==> 2/5 ready: poll deploys until live (deadline ${READY_DEADLINE}s)"
deploy_status=""
waited=0
while [ "$waited" -lt "$READY_DEADLINE" ]; do
  if acurl -o "$TMP_DIR/deploys.json" \
    "$API/v1/services/$SERVICE_ID/deploys?limit=5" 2>/dev/null; then
    deploy_status="$(jq -r '[.[]? | (.deploy // .)] | sort_by(.createdAt) | last | .status // empty' \
      "$TMP_DIR/deploys.json")"
    case "$deploy_status" in
      live) break ;;
      build_failed | update_failed | pre_deploy_failed | canceled)
        echo "FAIL(2/5 ready): the deploy reached terminal failure \"$deploy_status\" after ${waited}s."
        echo "      Deploy: $(jq -r '[.[]? | (.deploy // .)] | sort_by(.createdAt) | last | "\(.id) \(.failureReason // "")"' "$TMP_DIR/deploys.json")"
        echo "      Build-from-git is broken for a repo that builds by hand — the"
        echo "      product's core promise (push becomes a URL) is not being kept."
        exit 4
        ;;
    esac
  fi
  sleep "$POLL"
  waited=$((waited + POLL))
done
if [ "$deploy_status" != "live" ]; then
  echo "FAIL(2/5 ready): still \"${deploy_status:-no deploy row}\" after ${waited}s (deadline ${READY_DEADLINE}s)."
  echo "      Either the build queue is starved (see the Builds dashboard:"
  echo "      bex_build_queue_oldest_seconds) or the deploy never converged."
  exit 4
fi
echo "  ok: deploy live in ${waited}s"

# --- Stage 3: the assigned public host serves ---------------------------------
echo "==> 3/5 serve: the assigned public host must answer 200 (deadline ${SERVE_DEADLINE}s)"
status=000
waited=0
while [ "$waited" -lt "$SERVE_DEADLINE" ]; do
  if [ -z "$PUBLIC_URL" ]; then
    acurl -o "$TMP_DIR/get.json" "$API/v1/services/$SERVICE_ID" >/dev/null 2>&1 || true
    PUBLIC_URL="$(jq -r '.serviceDetails.url // .url // (.urls[0]? // "") // ""' "$TMP_DIR/get.json" 2>/dev/null || echo "")"
    PUBLIC_URL="${PUBLIC_URL%/}"
  fi
  if [ -n "$PUBLIC_URL" ] && [ "$PUBLIC_URL" != "null" ]; then
    status="$(curl -sS -o /dev/null -w '%{http_code}' \
      --connect-timeout 5 --max-time 30 "$PUBLIC_URL/" 2>/dev/null || echo 000)"
    [ "$status" = "200" ] && break
  fi
  sleep "$POLL"
  waited=$((waited + POLL))
done
if [ "$status" != "200" ]; then
  echo "FAIL(3/5 serve): the deploy is live but the public host answered $status after ${waited}s."
  echo "      A live deploy with a dead URL is the worst tenant-visible state:"
  echo "      the dashboard says healthy and the product does not work. Suspect"
  echo "      Ingress/cert issuance for the assigned onbex.co host."
  exit 5
fi
echo "  ok: 200 from the assigned host in ${waited}s (url recorded, not printed)"

# --- Stage 4: delete, then consistent absence on every surface -----------------
echo "==> 4/5 absence: DELETE, then all read surfaces converge (deadline ${ABSENCE_DEADLINE}s)"
delete_code="$(acurl -o /dev/null -w '%{http_code}' -X DELETE "$API/v1/services/$SERVICE_ID" || echo 000)"
if [ "$delete_code" != "204" ]; then
  echo "FAIL(4/5 absence): DELETE returned $delete_code, want 204"
  exit 6
fi
deleted_id="$SERVICE_ID"
SERVICE_ID="" # already deleted; keep the trap from re-issuing it
waited=0
byid=""
listed=""
served=""
while [ "$waited" -lt "$ABSENCE_DEADLINE" ]; do
  byid="$(acurl -o /dev/null -w '%{http_code}' "$API/v1/services/$deleted_id" 2>/dev/null || echo 000)"
  if acurl -o "$TMP_DIR/list.json" "$API/v1/services?limit=100" 2>/dev/null; then
    if jq -e --arg id "$deleted_id" '[.[]? | (.service // .)] | any(.id == $id)' \
      "$TMP_DIR/list.json" >/dev/null 2>&1; then
      listed=present
    else
      listed=absent
    fi
  else
    listed=error
  fi
  served="$(curl -sS -o /dev/null -w '%{http_code}' \
    --connect-timeout 5 --max-time 15 "$PUBLIC_URL/" 2>/dev/null || echo 000)"
  if [ "$byid" = "404" ] && [ "$listed" = absent ] && [ "$served" != "200" ]; then
    echo "  ok: by-id 404, list row gone, host no longer serving ($served) in ${waited}s"
    break
  fi
  sleep "$POLL"
  waited=$((waited + POLL))
done
if [ "$byid" != "404" ] || [ "$listed" != absent ] || [ "$served" = "200" ]; then
  echo "FAIL(4/5 absence): surfaces still disagree after ${waited}s — by-id=$byid list=$listed host=$served."
  echo "      This is the w3/m81 split the delete contract forbids (docs/ADR006"
  echo "      § Reads while a deletion finalizes): one surface says gone while"
  echo "      another still serves the deleted service."
  exit 6
fi

# --- Stage 5: the static-site variant ----------------------------------------
# scripts/static-delete-timing-verify.sh already does create-with-route-and-header
# → serve → delete → poll every surface to absence, and always tears down. It
# needs a public no-build static-site repo, which the canary workspace does not
# have yet (owed operator step, docs/ADR088 §6).
echo "==> 5/5 static: the static-site delete-timing variant"
if [ -z "${BEX_CANARY_STATIC_REPO:-}" ]; then
  echo "  SKIP: BEX_CANARY_STATIC_REPO is unset, so the static fixture repo does"
  echo "        not exist yet. The web variant above passed; the w3/m46 t008 +"
  echo "        w3/m81 t004 static legs stay OWED until the repo is published and"
  echo "        this variable is set on the workflow. Not a failure — an"
  echo "        explicitly recorded gap."
else
  if API="$API/v1" BEARER="$(canary_bearer)" REPO="$BEX_CANARY_STATIC_REPO" \
    OWNER_ID="$BEX_CANARY_WORKSPACE_ID" DEADLINE="$ABSENCE_DEADLINE" \
    bash scripts/static-delete-timing-verify.sh; then
    echo "  ok: static-site create → serve → delete → absence verified"
  else
    echo "FAIL(5/5 static): scripts/static-delete-timing-verify.sh failed (see its output above)"
    exit 7
  fi
fi

echo
echo "PASS: a repo became a running HTTPS service and then disappeared from every"
echo "      read surface within the delete window. The canary workspace is clean."
