#!/usr/bin/env bash
# E2E for outbound event webhooks (w3/m11, docs/ADR018-render-parity.md
# "Outbound event webhooks"). It proves the milestone's Definition of Done live:
#
#   1. REGISTER: a workspace registers a webhook URL + event subscription over
#      REST; the signing secret is returned exactly once (whsec_…) and never by
#      any later read.
#   2. DELIVER: creating a store-managed service opens a real deploy row, and
#      the endpoint receives a deploy_started payload within seconds — with
#      webhook-id/webhook-timestamp/webhook-signature headers whose HMAC an
#      INDEPENDENT verifier (its own ~20 lines of crypto, not bex's code)
#      validates against the registered secret.
#   3. LIFECYCLE: suspend + resume deliver service_suspended/service_resumed.
#   4. RETRY + AUTO-DISABLE: an endpoint that answers 500 is retried on the
#      configured backoff (BEX_WEBHOOK_BACKOFF, shortened here to seconds),
#      each attempt recorded in the delivery history, and after exhausting the
#      schedule the delivery is failed, the endpoint auto-disabled with a
#      reason, and the email notice attempted (logged — SMTP is unset here).
#   5. deploy_ended: the reconciler's deploy-close write-back (gate timeout
#      without an operator, ~3 min) produces a signed deploy_ended delivery.
#      Skipped with BEX_VERIFY_FAST=1.
#
# It starts disposable Hydra + Postgres containers on the host and runs bex-api
# from source. Because production correctly requires public HTTPS destinations
# and blocks private/loopback dialing, the caller supplies a temporary HTTPS
# tunnel that forwards to this script's receiver on 127.0.0.1:19999.
#
# Usage: BEX_VERIFY_RECEIVER_URL=https://<temporary-tunnel> \
#          KUBECONFIG=$PWD/infra/local/bex.kubeconfig scripts/webhooks-verify.sh
# Env:   BEX_VERIFY_FAST=1   skip the ~3-minute deploy_ended wait
# Requires: docker, go, kubectl, curl, jq.
set -euo pipefail
cd "$(dirname "$0")/.."

KUBECONFIG="${KUBECONFIG:-$PWD/infra/local/bex.kubeconfig}"
export KUBECONFIG
[ -f "$KUBECONFIG" ] || { echo "error: KUBECONFIG $KUBECONFIG not found" >&2; exit 1; }
kubectl get crd apps.app.bex.co >/dev/null || { echo "error: App CRD missing on the target cluster (make -C lego/operator install)" >&2; exit 1; }

STAMP="$(date +%s)"
TENANT="whv$STAMP"
APP=web
SVC="$TENANT-$APP"

PG_NAME=bex-webhooks-verify-pg
HYDRA_NAME=bex-webhooks-verify-hydra
PG_PORT=25440
HYDRA_PUBLIC=127.0.0.1:24446
HYDRA_ADMIN=127.0.0.1:24447
API=127.0.0.1:18093
CP=127.0.0.1:18094
RECEIVER=127.0.0.1:19999
RECEIVER_URL="${BEX_VERIFY_RECEIVER_URL:-}"
RECEIVER_URL="${RECEIVER_URL%/}"
CP_TOKEN="webhooks-verify-$STAMP"
CLIENT_ID=webhooks-verify
CLIENT_SECRET="webhooks-verify-secret-$STAMP"

case "$RECEIVER_URL" in
  https://*) ;;
  *) echo "error: BEX_VERIFY_RECEIVER_URL must be a public HTTPS tunnel forwarding to http://$RECEIVER" >&2; exit 1 ;;
esac

bindir="$(mktemp -d)"
API_PID=""
RECV_PID=""

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }
info() { echo "  -- $*"; }

reap() {
  local pid="$1"
  [ -n "$pid" ] || return 0
  kill "$pid" 2>/dev/null || true
  for _ in $(seq 1 10); do kill -0 "$pid" 2>/dev/null || return 0; sleep 0.3; done
  kill -9 "$pid" 2>/dev/null || true
}

cleanup() {
  if [ "${BEX_VERIFY_KEEP:-}" = "1" ]; then
    echo "BEX_VERIFY_KEEP=1 — leaving the stack up for inspection (bindir: $bindir, api pid $API_PID)"
    return 0
  fi
  reap "$API_PID"
  reap "$RECV_PID"
  docker rm -f "$PG_NAME" "$HYDRA_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# debug_dump — on failure, show what the store actually holds so a broken run
# explains itself.
debug_dump() {
  echo "--- receiver log ---"; cat "$RECV_LOG" 2>/dev/null || true
  echo "--- audit_events ---"
  docker exec "$PG_NAME" psql -U postgres -c "SELECT verb, workspace_id, target, outcome, at FROM audit_events ORDER BY at" 2>/dev/null || true
  echo "--- webhook_watermark ---"
  docker exec "$PG_NAME" psql -U postgres -c "SELECT * FROM webhook_watermark" 2>/dev/null || true
  echo "--- webhook_deliveries ---"
  docker exec "$PG_NAME" psql -U postgres -c "SELECT endpoint_id, event_type, attempt_count, last_status, next_attempt_at, delivered_at, failed_at FROM webhook_deliveries ORDER BY created_at" 2>/dev/null || true
  echo "--- api log tail ---"; tail -20 "$bindir/api.log" 2>/dev/null || true
}

wait_http() {
  for _ in $(seq 1 60); do
    curl -sf -o /dev/null "$1" && return 0
    sleep 1
  done
  fail "$1 never became ready"
}

echo "==> disposable Postgres + Hydra (docker)"
docker rm -f "$PG_NAME" "$HYDRA_NAME" >/dev/null 2>&1 || true
docker run -d --rm --name "$PG_NAME" -e POSTGRES_PASSWORD=verify -p "$PG_PORT:5432" postgres:16-alpine >/dev/null
docker run -d --rm --name "$HYDRA_NAME" -p 24446:4444 -p 24447:4445 -e DSN=memory \
  -e "SECRETS_SYSTEM=webhooks-verify-system-secret-$STAMP" \
  oryd/hydra:v2.2.0 serve all --dev >/dev/null
for _ in $(seq 1 30); do docker exec "$PG_NAME" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
wait_http "http://$HYDRA_ADMIN/health/ready"
DB_URI="postgres://postgres:verify@127.0.0.1:$PG_PORT/postgres?sslmode=disable"

echo "==> OAuth2 client + token"
curl -sf -X POST "http://$HYDRA_ADMIN/admin/clients" -H 'Content-Type: application/json' -d "{
  \"client_id\": \"$CLIENT_ID\", \"client_secret\": \"$CLIENT_SECRET\",
  \"grant_types\": [\"client_credentials\"], \"token_endpoint_auth_method\": \"client_secret_post\"
}" >/dev/null
TOKEN="$(curl -sf -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
  -d "grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET" | jq -r .access_token)"
[ -n "$TOKEN" ] && [ "$TOKEN" != null ] || fail "no access token from Hydra"

echo "==> mock receiver (records headers + body; /ok answers 200, /fail 500)"
cat > "$bindir/receiver.go" <<'EOF'
package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
)

func main() {
	f, _ := os.Create(os.Args[1])
	var mu sync.Mutex
	h := func(status int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			json.NewEncoder(f).Encode(map[string]string{
				"path": r.URL.Path,
				"id":   r.Header.Get("webhook-id"),
				"ts":   r.Header.Get("webhook-timestamp"),
				"sig":  r.Header.Get("webhook-signature"),
				"body": base64.StdEncoding.EncodeToString(body),
			})
			f.Sync()
			mu.Unlock()
			w.WriteHeader(status)
		}
	}
	http.HandleFunc("/ok", h(200))
	http.HandleFunc("/fail", h(500))
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	if err := http.ListenAndServe(":19999", nil); err != nil {
		panic(err)
	}
}
EOF
# A previous run's receiver would silently absorb this run's deliveries — the
# port must be free (go build + exec, never `go run`: reaping go run's wrapper
# PID orphans the compiled child, which keeps the port).
lsof -nP -iTCP:19999 -sTCP:LISTEN >/dev/null 2>&1 && fail "port 19999 already bound (a previous run's receiver?)"
RECV_LOG="$bindir/received.jsonl"

# An INDEPENDENT Standard-Webhooks verifier — deliberately not bex's own code,
# so a signature bug can't validate itself.
cat > "$bindir/sigverify.go" <<'EOF'
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// args: secret msgID timestamp signatureHeader bodyBase64
func main() {
	secret, id, ts, sigHeader, bodyB64 := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	key := []byte(secret)
	if rest, ok := strings.CutPrefix(secret, "whsec_"); ok {
		if k, err := base64.StdEncoding.DecodeString(rest); err == nil {
			key = k
		}
	}
	body, err := base64.StdEncoding.DecodeString(bodyB64)
	if err != nil {
		fmt.Println("bad body arg")
		os.Exit(1)
	}
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s.%s.%s", id, ts, body)
	want := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
	for _, sig := range strings.Fields(sigHeader) {
		if hmac.Equal([]byte(sig), []byte(want)) {
			fmt.Println("VALID")
			return
		}
	}
	fmt.Printf("INVALID (want %s got %s)\n", want, sigHeader)
	os.Exit(1)
}
EOF
( cd "$bindir" && go mod init verifyharness >/dev/null 2>&1 && \
  go build -o receiver receiver.go && go build -o sigverify sigverify.go )
"$bindir/receiver" "$RECV_LOG" & RECV_PID=$!
disown "$RECV_PID" 2>/dev/null || true # quiet job-control noise when cleanup reaps it
wait_http "http://$RECEIVER/healthz"

echo "==> building + starting bex-api (control plane on, webhook backoff 2s,3s,4s)"
( cd lego/backend && go build -o "$bindir/api" ./cmd/api )
env "KUBECONFIG=$KUBECONFIG" \
  "BEX_CP_DB_URI=$DB_URI" "BEX_CP_ADDR=:18094" "BEX_CP_TOKEN=$CP_TOKEN" "BEX_CP_RESYNC=5s" \
  "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" "BEX_API_ADDR=:18093" "BEX_API_NAMESPACE=default" \
  "BEX_WEBHOOK_BACKOFF=2s,3s,4s" \
  "$bindir/api" > "$bindir/api.log" 2>&1 & API_PID=$!
wait_http "http://$API/healthz"

request() { # METHOD PATH [BODY] -> LAST_CODE/LAST_BODY
  local args=(-s -w '\n%{http_code}' -X "$1" "http://$API$2" -H "Authorization: Bearer $TOKEN")
  [ -n "${3:-}" ] && args+=(-H 'Content-Type: application/json' -d "$3")
  local out; out="$(curl "${args[@]}")"
  LAST_CODE="${out##*$'\n'}"; LAST_BODY="${out%$'\n'*}"
}
expect() { [ "$LAST_CODE" = "$1" ] || fail "$2: got $LAST_CODE want $1 (body: $LAST_BODY)"; }
cp_post() { curl -s -X POST "http://$CP$1" -H "Authorization: Bearer $CP_TOKEN" -H 'Content-Type: application/json' -d "$2"; }

# wait_delivery TYPE PATH -> RECV_LINE (the receiver-log line for that event)
wait_delivery() {
  local type="$1" path="$2"
  for _ in $(seq 1 30); do
    RECV_LINE="$(jq -c --arg p "$path" 'select(.path==$p) | select((.body|@base64d|fromjson).type=="'"$type"'")' "$RECV_LOG" 2>/dev/null | head -1 || true)"
    [ -n "$RECV_LINE" ] && return 0
    sleep 1
  done
  debug_dump
  fail "no $type delivery arrived at $path"
}

verify_line_signature() { # RECV_LINE SECRET
  local line="$1" secret="$2"
  local id ts sig body
  id="$(echo "$line" | jq -r .id)"; ts="$(echo "$line" | jq -r .ts)"
  sig="$(echo "$line" | jq -r .sig)"; body="$(echo "$line" | jq -r .body)"
  [ -n "$id" ] && [ -n "$ts" ] && [ -n "$sig" ] || fail "delivery missing webhook-id/-timestamp/-signature headers: $line"
  "$bindir/sigverify" "$secret" "$id" "$ts" "$sig" "$body" | grep -q VALID \
    || fail "signature did not verify independently (id=$id ts=$ts)"
  # webhook-id must equal the payload's data.id (Render's documented contract).
  local data_id
  data_id="$(echo "$line" | jq -r '.body|@base64d|fromjson|.data.id')"
  [ "$id" = "$data_id" ] || fail "webhook-id ($id) != data.id ($data_id)"
}

echo "==> 1. register: workspace + webhook endpoint (secret shown once)"
TENANT_ID="$(cp_post /v1/tenants "{\"name\":\"$TENANT\",\"plan\":\"pro\",\"admin\":\"$CLIENT_ID\"}" | jq -r '.id // empty')"
[ -n "$TENANT_ID" ] || fail "could not create tenant via the control-plane API"
request POST "/v1/webhooks" "{\"ownerId\":\"$TENANT_ID\",\"name\":\"ok-hook\",\"url\":\"$RECEIVER_URL/ok\",\"enabled\":true,\"eventFilter\":[\"deploy_started\",\"deploy_ended\",\"service_suspended\",\"service_resumed\"]}"
expect 201 "create webhook endpoint"
EP1="$(echo "$LAST_BODY" | jq -r .id)"
SECRET1="$(echo "$LAST_BODY" | jq -r .secret)"
case "$SECRET1" in whsec_*) ;; *) fail "create did not return a whsec_… secret: $SECRET1" ;; esac
request GET "/v1/webhooks?ownerId=$TENANT_ID" ''
expect 200 "list webhook endpoints"
echo "$LAST_BODY" | jq -e '.[0].webhook | has("secret") | not' >/dev/null || fail "list response carries a secret field"
echo "$LAST_BODY" | grep -q "$SECRET1" && fail "list response leaked the secret"
request GET "/v1/webhooks/$EP1?ownerId=$TENANT_ID" ''
expect 200 "get webhook endpoint"
echo "$LAST_BODY" | grep -q "$SECRET1" && fail "get response leaked the secret"
pass "endpoint $EP1 registered; secret returned once and never re-readable"

echo "==> 2. deliver: create a store-managed service (opens a real deploy)"
APP_ID="$(cp_post /v1/apps "{\"tenantId\":\"$TENANT_ID\",\"name\":\"$APP\",\"image\":\"traefik/whoami\",\"port\":80,\"replicas\":1,\"tier\":\"starter\"}" | jq -r '.id // empty')"
[ -n "$APP_ID" ] || fail "could not create app row via the control-plane API"
wait_delivery deploy_started /ok
verify_line_signature "$RECV_LINE" "$SECRET1"
got_svc="$(echo "$RECV_LINE" | jq -r '.body|@base64d|fromjson|.data.serviceId')"
[ "$got_svc" = "$SVC" ] || fail "deploy_started serviceId = $got_svc, want $SVC"
pass "deploy_started delivered within seconds, HMAC independently verified, serviceId=$SVC"

echo "==> 3. lifecycle: suspend + resume"
request POST "/v1/services/$SVC/suspend" '' ; expect 202 suspend
request POST "/v1/services/$SVC/resume" ''  ; expect 202 resume
wait_delivery service_suspended /ok ; verify_line_signature "$RECV_LINE" "$SECRET1"
wait_delivery service_resumed /ok   ; verify_line_signature "$RECV_LINE" "$SECRET1"
pass "service_suspended + service_resumed delivered and verified"

request GET "/v1/webhooks/$EP1/events?ownerId=$TENANT_ID" ''
expect 200 "delivery history"
delivered="$(echo "$LAST_BODY" | jq '[.[] | select(.webhookEvent.statusCode >= 200 and .webhookEvent.statusCode < 300)] | length')"
[ "$delivered" -ge 3 ] || fail "delivery history shows $delivered delivered, want >= 3 (body: $LAST_BODY)"
echo "$LAST_BODY" | jq -e '.[0] | (.cursor | length > 0) and (.webhookEvent.sentAt | length > 0)' >/dev/null || fail "history cursor/sentAt missing"
pass "Render-shaped history records $delivered delivered entries with stable sentAt/cursors"

echo "==> 4. retry + auto-disable: a failing endpoint"
request POST "/v1/webhooks" "{\"ownerId\":\"$TENANT_ID\",\"name\":\"fail-hook\",\"url\":\"$RECEIVER_URL/fail\",\"enabled\":true,\"eventFilter\":[\"service_suspended\"]}"
expect 201 "create failing endpoint"
EP2="$(echo "$LAST_BODY" | jq -r .id)"
request POST "/v1/services/$SVC/suspend" '' ; expect 202 "suspend (for the failing endpoint)"
# 4 attempts total (initial + the 2s,3s,4s schedule) => failed after ~9s + polling.
deadline=$((SECONDS + 60))
EP2_STATE=""
while [ $SECONDS -lt $deadline ]; do
  EP2_STATE="$(docker exec "$PG_NAME" psql -U postgres -At -F, -c \
    "SELECT attempt_count, COALESCE(last_status,0), failed_at IS NOT NULL FROM webhook_deliveries WHERE endpoint_id='$EP2' AND event_type='service_suspended' ORDER BY created_at DESC LIMIT 1" 2>/dev/null || true)"
  [ "${EP2_STATE##*,}" = "t" ] && break
  sleep 2
done
[ "${EP2_STATE##*,}" = "t" ] || fail "failing delivery never reached terminal failure: $EP2_STATE"
[ "${EP2_STATE%%,*}" = "4" ] || fail "attemptCount = ${EP2_STATE%%,*}, want 4 (initial + 3 retries)"
case "$EP2_STATE" in 4,500,t) ;; *) fail "terminal delivery evidence = $EP2_STATE, want 4,500,t" ;; esac
fails="$(jq -c 'select(.path=="/fail")' "$RECV_LOG" | wc -l | tr -d ' ')"
[ "$fails" = "4" ] || fail "receiver saw $fails /fail attempts, want 4"
request GET "/v1/webhooks/$EP2?ownerId=$TENANT_ID" ''
expect 200 "get failing endpoint"
[ "$(echo "$LAST_BODY" | jq -r .enabled)" = "false" ] || fail "endpoint was not auto-disabled: $LAST_BODY"
echo "$LAST_BODY" | jq -r .disabledReason | grep -qi "automatically" || fail "disabledReason missing: $LAST_BODY"
grep -q "notice not emailed" "$bindir/api.log" || fail "no failure-notice log line (SMTP unset should log, not silently skip)"
pass "4 attempts on the 2s,3s,4s schedule, then failed + auto-disabled (reason recorded, email notice logged)"

if [ "${BEX_VERIFY_FAST:-}" != "1" ]; then
  echo "==> 5. deploy_ended (reconciler write-back; the gate timeout closes it in ~3 min without an operator)"
  RECV_LINE=""
  for _ in $(seq 1 90); do
    RECV_LINE="$(jq -c 'select(.path=="/ok") | select((.body|@base64d|fromjson).type=="deploy_ended")' "$RECV_LOG" 2>/dev/null | head -1 || true)"
    [ -n "$RECV_LINE" ] && break
    sleep 3
  done
  [ -n "$RECV_LINE" ] || fail "no deploy_ended delivery within the gate window"
  verify_line_signature "$RECV_LINE" "$SECRET1"
  ended_status="$(echo "$RECV_LINE" | jq -r '.body|@base64d|fromjson|.data.status')"
  case "$ended_status" in succeeded|failed|canceled) ;; *) fail "deploy_ended status = $ended_status" ;; esac
  pass "deploy_ended delivered and verified (status=$ended_status)"
else
  info "BEX_VERIFY_FAST=1 — skipping the deploy_ended gate-timeout wait"
fi

echo
echo "PASS: outbound webhooks verified live — register (secret once), signed delivery within"
echo "      seconds of a real deploy, lifecycle events, retry schedule, auto-disable + notice."
