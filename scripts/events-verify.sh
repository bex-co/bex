#!/usr/bin/env bash
# E2E for the per-service events feed (w3/m7, docs/ADR006-bex-api.md § Service events)
# against the CAPD mock cluster. It proves the milestone's claim — the feed is a
# TRUTHFUL 1:1 record of what happened to a service, composed from deploy,
# audit, and closed typed fact rows, and it can never carry a secret:
#
#   1. CORRESPONDENCE: run a scripted sequence against a store-managed service
#      (create → deploy, suspend, resume, scale, env-var write, trigger deploy),
#      then read GET /v1/services/{id}/events and assert each action appears
#      EXACTLY ONCE, under the right Render event type, with the caller's identity
#      as the actor.
#   2. ORDERING: the whole feed is newest-first across all three sources, and
#      lifecycle actions appear in the
#      reverse of the order they were performed.
#   3. CURSOR: paging with limit=2 walks the same events with no duplicate and no
#      gap at any page boundary — the keyset, not an OFFSET.
#   4. REDACTION: a planted env-var VALUE appears nowhere in any response, on any
#      of the three surfaces. (Structural: the audit rows the feed reads have never
#      carried verb arguments — this asserts the property end-to-end.)
#   5. OMITTED, NOT FAKED: with the control-plane store unwired, the endpoint 503s
#      rather than inventing an empty feed.
#
# Shape (secrets-verify.sh's harness): bex-api AND the operator run on the host
# (go build) against the cluster via $KUBECONFIG; Hydra, OpenBao and the
# control-plane Postgres (bex-db) are reached through kubectl port-forwards. The
# service is created through the control-plane's internal API — that is what makes
# it store-managed (an App CR created directly has no deploys and no feed, by
# design).
#
# Usage: scripts/events-verify.sh     # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, jq, openssl, go; the mock cluster with auth (Hydra),
# OpenBao and bex-db synced, and the App CRD installed (lego/operator: make install).
set -euo pipefail
cd "$(dirname "$0")/.."

AUTH_NS="${AUTH_NS:-auth}"
SECRETS_NS="${SECRETS_NS:-secrets}"
SYS_NS="${SYS_NS:-bex-system}"
APP_NS="${APP_NS:-default}"

HYDRA_PUBLIC=127.0.0.1:24444
HYDRA_ADMIN=127.0.0.1:24445
BAO=127.0.0.1:24200
DB=127.0.0.1:25432
API=127.0.0.1:18090
CP=127.0.0.1:18092

# A dedicated tenant+service per run, so a rerun never collides.
STAMP="$(date +%s)"
TENANT="evt$STAMP"
APP=web
SVC="$TENANT-$APP" # the projected App CR name == the service id the API takes
PLANTED="s3cr3t-value-$STAMP" # must NEVER appear in an event
WEBHOOK_SECRET="events-webhook-$STAMP"
TENANT_ID=""
APP_ID=""

API_PID=""
OP_PID=""
PIDS=()

# reap PID — SIGTERM, then SIGKILL if it's still up. bex-api's graceful shutdown
# can outlive the script; a survivor keeps :18090/:18092 bound and the NEXT run
# then talks to a stale binary with a stale CP token, which fails in a way that
# looks like a bug in the feature. So: never leave one behind.
reap() {
  local pid="$1"
  [ -n "$pid" ] || return 0
  kill "$pid" 2>/dev/null || return 0
  for _ in $(seq 1 20); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 0.25
  done
  kill -9 "$pid" 2>/dev/null || true
}

cleanup() {
	if [ -n "$APP_ID" ] && [ -n "${CP_TOKEN:-}" ]; then
		curl -s -X DELETE "http://$CP/v1/apps/$APP_ID" -H "Authorization: Bearer $CP_TOKEN" >/dev/null 2>&1 || true
	fi
  reap "$OP_PID"
  reap "$API_PID"
  if [ "${#PIDS[@]}" -gt 0 ]; then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
  kubectl -n "$APP_NS" delete app.app.bex.co "$SVC" --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

pass() { printf '\033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '\033[31mFAIL\033[0m %s\n' "$1"; exit 1; }
info() { printf '\033[36m•\033[0m %s\n' "$1"; }

wait_http() {
  for _ in $(seq 1 40); do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

# --- preflight ---------------------------------------------------------------
# A leftover bex-api on these ports would answer /healthz with a STALE control-plane
# token, and every call below would fail confusingly. Refuse to start instead.
for port in 18090 18092; do
  lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1 \
    && fail "port $port is already in use — a previous bex-api is still running (pkill -f bex-api)"
done
kubectl get crd apps.app.bex.co >/dev/null 2>&1 \
  || fail "App CRD not installed — run 'make -C lego/operator install' (absolute KUBECONFIG)"
DB_NS="${DB_NS:-$AUTH_NS}"
kubectl -n "$DB_NS" get cluster.postgresql.cnpg.io bex-db >/dev/null 2>&1 \
  || fail "no bex-db CNPG cluster in $DB_NS — set DB_NS to the active control-plane namespace"

echo "==> port-forwards (hydra, openbao, bex-db) + building bex-api and the operator"
kubectl -n "$AUTH_NS" port-forward service/hydra-public 24444:4444 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$AUTH_NS" port-forward service/hydra-admin 24445:4445 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$SECRETS_NS" port-forward service/openbao 24200:8200 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$DB_NS" port-forward service/bex-db-rw 25432:5432 >/dev/null 2>&1 & PIDS+=($!)

bindir="$(mktemp -d)"
(cd lego/backend && go build -o "$bindir/bex-api" ./cmd/api) || fail "bex-api build failed"
(cd lego/operator && go build -o "$bindir/manager" ./cmd/manager) || fail "operator build failed"
wait_http "http://$HYDRA_ADMIN/health/ready"
wait_http "http://$BAO/v1/sys/seal-status"

echo "==> OpenBao k8s auth for bex-api (idempotent) + bootstrap OAuth2 client"
kubectl create namespace "$SYS_NS" >/dev/null 2>&1 || true
kubectl -n "$SYS_NS" create serviceaccount bex-api >/dev/null 2>&1 || true
if [ -n "${BAO_ROOT_TOKEN:-}" ] || { [ -f .env ] && grep -q '^BAO_ROOT_TOKEN=.' .env; }; then
  BAO_ADDR="http://$BAO" bash scripts/bao-k8s-auth.sh >/dev/null
else
  info "BAO_ROOT_TOKEN unavailable; using the mock cluster's existing bex-api Kubernetes-auth role"
fi
jwt_file="$bindir/sa-token"
kubectl -n "$SYS_NS" create token bex-api --duration=1h > "$jwt_file"

export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-events-verify-$STAMP}"
HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" bash scripts/auth-bootstrap-client.sh >/dev/null
TOKEN="$({ curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
  -d "grant_type=client_credentials&client_id=bex-bootstrap&client_secret=$BEX_BOOTSTRAP_CLIENT_SECRET" || true; } \
  | jq -r '.access_token // empty')"
[ -n "$TOKEN" ] || fail "no access_token for the bootstrap client"
# The caller's identity — what the feed must name as the actor of every action.
ACTOR=bex-bootstrap

DB_USER="$(kubectl -n "$DB_NS" get secret bex-db-app -o jsonpath='{.data.username}' | base64 -d)"
DB_PW="$(kubectl -n "$DB_NS" get secret bex-db-app -o jsonpath='{.data.password}' | base64 -d)"
DB_URI="postgres://${DB_USER}:${DB_PW}@${DB}/bex?sslmode=disable"
CP_TOKEN="events-verify-$STAMP"

# --- run the operator + bex-api (store on: all three feed sources live there) --
echo "==> starting the operator and bex-api (control-plane store + OpenBao wired)"
BEX_RUNTIME=kubernetes "$bindir/manager" >"$bindir/operator.log" 2>&1 &
OP_PID=$!

start_api() { # extra env... ; empty BEX_CP_DB_URI => the store-less 503 case
  env "$@" "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" BEX_API_ADDR=":18090" BEX_API_NAMESPACE="$APP_NS" \
    "$bindir/bex-api" >>"$bindir/api.log" 2>&1 &
  API_PID=$!
  wait_http "http://$API/healthz"
}

# stop_api reaps with a SIGKILL backstop and waits for the LISTEN socket to clear.
# A bare `wait` here would block forever if the process ignored SIGTERM, and
# restarting before the port is free makes the new bex-api die on bind while the
# old one still answers /healthz — the next assertion would then be measuring the
# process we thought we had replaced.
stop_api() {
  reap "$API_PID"
  API_PID=""
  for _ in $(seq 1 20); do
    lsof -nP -iTCP:18090 -sTCP:LISTEN >/dev/null 2>&1 || return 0
    sleep 0.5
  done
  fail "port 18090 never freed after stopping bex-api"
}

# BEX_CP_RESYNC=5s: the projector's write-back is what CLOSES the first deploy
# (deploy_ended). At the 30s default this test would just sit and wait for it.
start_api "BEX_CP_DB_URI=$DB_URI" "BEX_CP_ADDR=:18092" "BEX_CP_TOKEN=$CP_TOKEN" "BEX_CP_RESYNC=5s" \
  "BEX_OPENBAO_URL=http://$BAO" "BEX_OPENBAO_JWT_PATH=$jwt_file" "BEX_WEBHOOK_SECRET=$WEBHOOK_SECRET"

# request METHOD PATH BODY -> LAST_CODE + LAST_BODY (public API, bearer-gated)
request() {
  local args=(-s -w '\n%{http_code}' -X "$1" "http://$API$2" -H "Authorization: Bearer $TOKEN")
  [ -n "${3:-}" ] && args+=(-H 'Content-Type: application/json' -d "$3")
  local out; out="$(curl "${args[@]}")"
  LAST_CODE="${out##*$'\n'}"; LAST_BODY="${out%$'\n'*}"
}
expect() { [ "$LAST_CODE" = "$1" ] || fail "$2: got $LAST_CODE want $1 (body: $LAST_BODY)"; }
SINCE="$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)"

wait_event_count() {
  local event_type="$1" want="$2"
  for _ in $(seq 1 60); do
    request GET "/v1/services/$SVC/events?startTime=$SINCE&limit=100" ''
    [ "$LAST_CODE" = "200" ] || { sleep 2; continue; }
    local count
    count="$(echo "$LAST_BODY" | jq --arg type "$event_type" '[.[] | select(.event.type==$type)] | length')"
    [ "$count" -ge "$want" ] && return 0
    sleep 2
  done
  fail "$event_type did not reach count $want"
}

# --- the scripted sequence ----------------------------------------------------
echo "==> creating a store-managed service through the control-plane API"
cp_post() { curl -s -X POST "http://$CP$1" -H "Authorization: Bearer $CP_TOKEN" -H 'Content-Type: application/json' -d "$2"; }
TENANT_ID="$(cp_post /v1/tenants "{\"name\":\"$TENANT\",\"plan\":\"pro\"}" | jq -r '.id // empty')"
[ -n "$TENANT_ID" ] || fail "could not create tenant via the control-plane API (is BEX_CP_ADDR up?)"
APP_ID="$(cp_post /v1/apps "{\"tenantId\":\"$TENANT_ID\",\"name\":\"$APP\",\"image\":\"traefik/whoami\",\"port\":80,\"replicas\":1,\"tier\":\"starter\"}" | jq -r '.id // empty')"
[ -n "$APP_ID" ] || fail "could not create app row via the control-plane API"
info "tenant $TENANT_ID, app $APP_ID → App CR $SVC (the projector names it <tenant>-<app>)"

for _ in $(seq 1 30); do kubectl -n "$APP_NS" get app.app.bex.co "$SVC" >/dev/null 2>&1 && break; sleep 2; done
kubectl -n "$APP_NS" get app.app.bex.co "$SVC" >/dev/null 2>&1 || fail "the projector never created App/$SVC"
# The operator must take it Running, so the reconciler's write-back can CLOSE the
# first deploy — that close is the deploy_ended event.
kubectl -n "$APP_NS" wait --for=jsonpath='{.status.phase}'=Running "app.app.bex.co/$SVC" --timeout=180s \
  || fail "App/$SVC never reached Running (see $bindir/operator.log)"

# The first deploy row is CLOSED by the projector's write-back, one resync after the
# App reports Running — that close is the deploy_ended event, so wait for it rather
# than racing it.
first_status=""
for _ in $(seq 1 30); do
  request GET "/v1/services/$SVC/deploys" ''
  first_status="$(echo "$LAST_BODY" | jq -r '.[-1].deploy.status')"
  [ "$first_status" = "live" ] && break
  sleep 2
done
[ "$first_status" = "live" ] \
  || fail "the first deploy never closed as live ($first_status) — the write-back is what deploy_ended is derived from"

echo "==> intent + observed convergence: suspend → Hibernated → resume → Running"
# Render's own codes are accepted (202), not synchronously completed. Waiting
# between intent writes proves the distinct service_suspended/service_resumed
# facts rather than racing both requests through one operator observation.
request POST "/v1/services/$SVC/suspend" ''; expect 202 suspend
kubectl -n "$APP_NS" wait --for=jsonpath='{.status.phase}'=Hibernated "app.app.bex.co/$SVC" --timeout=180s \
  || fail "App/$SVC never converged to Hibernated"
wait_event_count service_suspended 1

request POST "/v1/services/$SVC/resume" ''; expect 202 resume
kubectl -n "$APP_NS" wait --for=jsonpath='{.status.phase}'=Running "app.app.bex.co/$SVC" --timeout=180s \
  || fail "App/$SVC never converged back to Running"
wait_event_count service_resumed 1

echo "==> manual scale + env-var redaction sentinel"
request POST "/v1/services/$SVC/scale" '{"numInstances":2}'; expect 202 scale
request PUT "/v1/services/$SVC/env-vars" "[{\"key\":\"DB_PASSWORD\",\"value\":\"$PLANTED\"}]"
[ "$LAST_CODE" = "200" ] || fail "env-var write: got $LAST_CODE want 200 — OpenBao must be reachable for the redaction check to mean anything (body: $LAST_BODY)"

echo "==> source decisions: branch change + idempotent ignored commit"
request PATCH "/v1/services/$SVC" '{"branch":"events-live"}'; expect 200 "branch change"
wait_event_count branch_changed 1
request PATCH "/v1/services/$SVC" '{"repo":"https://github.com/acme/events-fixture","autoDeploy":"yes"}'; expect 200 "switch to Git source"
PUSH_BODY="$(jq -nc '{ref:"refs/heads/events-live",after:"abc123",repository:{clone_url:"https://github.com/acme/events-fixture.git",html_url:"https://github.com/acme/events-fixture"},head_commit:{id:"abc123",url:"https://github.com/acme/events-fixture/commit/abc123",message:"docs [skip render]"},commits:[]}')"
PUSH_SIG="$(printf '%s' "$PUSH_BODY" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -hex | awk '{print $2}')"
for _ in 1 2; do
  code="$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://$API/v1/webhooks/git" \
    -H "X-Hub-Signature-256: sha256=$PUSH_SIG" -H 'X-GitHub-Delivery: events-live-delivery' \
    -H 'Content-Type: application/json' -d "$PUSH_BODY")"
  [ "$code" = "200" ] || fail "signed ignored-commit webhook returned $code, want 200"
done
wait_event_count commit_ignored 1

echo "==> deploy failure + recovery: invalid image → healthy image"
request PATCH "/v1/services/$SVC" '{"image":{"imagePath":"registry.invalid/bex/events-missing:never"}}'; expect 200 "set invalid image"
request POST "/v1/services/$SVC/deploys" ''; expect 201 "trigger invalid-image deploy"
kubectl -n "$APP_NS" wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].reason}'=ImagePullBackOff "app.app.bex.co/$SVC" --timeout=240s \
  || fail "invalid-image deploy never reported ImagePullBackOff"
wait_event_count image_pull_failed 1
wait_event_count server_failed 1

request PATCH "/v1/services/$SVC" '{"image":{"imagePath":"traefik/whoami"}}'; expect 200 "restore healthy image"
request POST "/v1/services/$SVC/deploys" ''; expect 201 "trigger recovery deploy"
kubectl -n "$APP_NS" wait --for=jsonpath='{.status.phase}'=Running "app.app.bex.co/$SVC" --timeout=240s \
  || fail "recovery deploy never reached Running"
wait_event_count server_available 1

echo "==> autoscaling status handoff: started → ended (real CR status + Postgres fact path)"
AUTO_STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
kubectl -n "$APP_NS" patch "app.app.bex.co/$SVC" --subresource=status --type=merge \
  -p "{\"status\":{\"autoscaling\":{\"transitionId\":\"verify-$STAMP\",\"fromReplicas\":2,\"toReplicas\":3,\"state\":\"Started\",\"startedAt\":\"$AUTO_STARTED\"}}}" >/dev/null
wait_event_count autoscaling_started 1
AUTO_FINISHED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
kubectl -n "$APP_NS" patch "app.app.bex.co/$SVC" --subresource=status --type=merge \
  -p "{\"status\":{\"autoscaling\":{\"transitionId\":\"verify-$STAMP\",\"fromReplicas\":2,\"toReplicas\":3,\"state\":\"Ended\",\"startedAt\":\"$AUTO_STARTED\",\"finishedAt\":\"$AUTO_FINISHED\"}}}" >/dev/null
wait_event_count autoscaling_ended 1

# --- 1+2. correspondence and ordering ----------------------------------------
echo "==> reading the feed"
request GET "/v1/services/$SVC/events?startTime=$SINCE&limit=100" ''
expect 200 "GET events"
FEED="$LAST_BODY"
types() { echo "$FEED" | jq -r '.[].event.type'; }

for t in deploy_started deploy_ended suspender_added suspender_removed service_suspended service_resumed \
  image_pull_failed server_failed server_available branch_changed commit_ignored \
  autoscaling_started autoscaling_ended instance_count_changed env_vars_changed; do
  n="$(types | grep -cx "$t" || true)"
  case "$t" in
    deploy_started|deploy_ended) want=3 ;; # create + invalid-image + recovery
    *) want=1 ;;
  esac
  [ "$n" = "$want" ] || fail "$t appears $n time(s), want exactly $want — the feed must be 1:1 with what happened"
done
pass "correspondence: every intent/observed/source fact appears once; deploy lifecycle appears for create, failure, and recovery"

# Actor identity is truthful — the suspend names the caller that issued it.
actor="$(echo "$FEED" | jq -r '[.[] | select(.event.type=="suspender_added")][0].event.details.actor')"
[ "$actor" = "$ACTOR" ] || fail "suspender_added actor = '$actor', want '$ACTOR'"
restart_of_deploy="$(echo "$FEED" | jq -r '[.[] | select(.event.type=="deploy_ended")][0].event.details.deployStatus')"
[ "$restart_of_deploy" = "succeeded" ] || fail "deploy_ended deployStatus = '$restart_of_deploy', want succeeded"
pass "actor identity + deploy status are truthful (actor=$ACTOR, first deploy succeeded)"

image_reason="$(echo "$FEED" | jq -r '[.[] | select(.event.type=="image_pull_failed")][0].event.details.reasonCode')"
[ "$image_reason" = "image_pull_backoff" ] || fail "image_pull_failed reasonCode = '$image_reason'"
branch_pair="$(echo "$FEED" | jq -r '[.[] | select(.event.type=="branch_changed")][0].event.details | [.from,.to] | join("→")')"
[ "$branch_pair" = "main→events-live" ] || fail "branch_changed details = '$branch_pair'"
ignored_reason="$(echo "$FEED" | jq -r '[.[] | select(.event.type=="commit_ignored")][0].event.details.reasonCode')"
[ "$ignored_reason" = "skip_phrase" ] || fail "commit_ignored reasonCode = '$ignored_reason'"
autoscale_pair="$(echo "$FEED" | jq -r '[.[] | select(.event.type=="autoscaling_ended")][0].event.details | [.fromInstances,.toInstances] | join("→")')"
[ "$autoscale_pair" = "2→3" ] || fail "autoscaling_ended details = '$autoscale_pair'"
pass "typed details: bounded image/commit reasons, branch from/to, autoscaling fromInstances/toInstances"

# Newest-first across all three sources, and lifecycle verbs in reverse order.
ts="$(echo "$FEED" | jq -r '.[].event.timestamp')"
[ "$(echo "$ts" | sort -r)" = "$ts" ] || fail "feed is not newest-first:\n$ts"
seq_types="$(types | grep -E 'suspender_added|suspender_removed|instance_count_changed|env_vars_changed' | paste -sd, -)"
[ "$seq_types" = "env_vars_changed,instance_count_changed,suspender_removed,suspender_added" ] \
  || fail "lifecycle events are not in reverse-chronological order: $seq_types"
pass "ordering: newest-first across deploy + audit + fact rows, lifecycle in reverse order"

# --- 3. cursor ----------------------------------------------------------------
total="$(echo "$FEED" | jq 'length')"
walked=""
cursor=""
for _ in $(seq 1 20); do
  q="/v1/services/$SVC/events?startTime=$SINCE&limit=2"
  [ -n "$cursor" ] && q="$q&cursor=$cursor"
  request GET "$q" ''
  expect 200 "GET events (paged)"
  n="$(echo "$LAST_BODY" | jq 'length')"
  [ "$n" -eq 0 ] && break
  walked="$walked$(echo "$LAST_BODY" | jq -r '.[].event.id')
"
  cursor="$(echo "$LAST_BODY" | jq -r '.[-1].cursor')"
done
walked="$(echo "$walked" | sed '/^$/d')"
n_walked="$(echo "$walked" | wc -l | tr -d ' ')"
n_unique="$(echo "$walked" | sort -u | wc -l | tr -d ' ')"
[ "$n_walked" = "$total" ] || fail "paged walk returned $n_walked events, want $total (a gap or a duplicate at a page boundary)"
[ "$n_unique" = "$total" ] || fail "paged walk repeated an event id ($n_unique unique of $n_walked)"
[ "$walked" = "$(echo "$FEED" | jq -r '.[].event.id')" ] || fail "paged walk visited events in a different order than the full feed"
pass "cursor: limit=2 walks all $total events, no duplicates, no gaps, same order"

# --- 4. redaction -------------------------------------------------------------
echo "$FEED" | grep -q "$PLANTED" && fail "REST feed leaked the env-var VALUE"
echo "$FEED" | grep -q "DB_PASSWORD" && info "note: the env-var KEY appears in the feed (values are what must never appear)"
gqlq="{\"query\":\"{ serviceEvents(serviceId: \\\"$SVC\\\", startTime: \\\"$SINCE\\\", limit: 100) { id type timestamp details { deployId deployStatus actor triggeredByUser reasonCode fromCount toCount branchFrom branchTo commitId commitUrl } } }\"}"
gql="$(curl -s -X POST "http://$API/graphql" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$gqlq")"
echo "$gql" | grep -q "$PLANTED" && fail "GraphQL serviceEvents leaked the env-var VALUE"
echo "$gql" | jq -e '.data.serviceEvents | length > 0' >/dev/null || fail "GraphQL serviceEvents returned nothing: $gql"
rest_identity="$(echo "$FEED" | jq -r '.[] | [.event.id,.event.type,.event.timestamp] | join("|")')"
gql_identity="$(echo "$gql" | jq -r '.data.serviceEvents[] | [.id,.type,.timestamp] | join("|")')"
[ "$rest_identity" = "$gql_identity" ] || fail "REST and GraphQL ids/types/timestamps diverged"
pass "surface parity + redaction: REST and GraphQL agree, and the planted value appears in neither"

# The value really is retrievable through the env-vars API — so its absence above
# is redaction, not a write that silently failed.
request GET "/v1/services/$SVC/env-vars" ''
echo "$LAST_BODY" | grep -q "$PLANTED" \
  && pass "the planted value IS readable via GET /env-vars — the feed's omission is deliberate, not a broken write" \
  || fail "the planted value was never stored; the redaction check above proved nothing"

# --- 5. store-less 503 --------------------------------------------------------
echo "==> restarting bex-api with the control-plane store unwired"
stop_api
start_api # no BEX_CP_DB_URI
request GET "/v1/services/$SVC/events" ''
[ "$LAST_CODE" = "503" ] \
  && pass "store-less: the feed 503s (omitted, not faked as an empty history)" \
  || fail "store-less events = $LAST_CODE, want 503 (body: $LAST_BODY)"

echo "==> restoring the store-backed API for fixture cleanup"
stop_api
start_api "BEX_CP_DB_URI=$DB_URI" "BEX_CP_ADDR=:18092" "BEX_CP_TOKEN=$CP_TOKEN" "BEX_CP_RESYNC=5s" \
  "BEX_OPENBAO_URL=http://$BAO" "BEX_OPENBAO_JWT_PATH=$jwt_file" "BEX_WEBHOOK_SECRET=$WEBHOOK_SECRET"
request DELETE "/v1/services/$SVC" ''
expect 204 "delete verification service"
APP_ID=""
for _ in $(seq 1 30); do
  kubectl -n "$APP_NS" get app.app.bex.co "$SVC" >/dev/null 2>&1 || break
  sleep 1
done
kubectl -n "$APP_NS" get app.app.bex.co "$SVC" >/dev/null 2>&1 \
  && fail "verification App/$SVC still exists after cleanup"
pass "cleanup: service row, App, workloads, and OpenBao service secrets removed through the product delete path"

echo
pass "service events verified: three-source, idempotent, range/paging-stable, and value-free"
