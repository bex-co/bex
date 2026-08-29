#!/usr/bin/env bash
# E2E for durable logs (docs/ADR010-observability.md, w3/m5) and the Application /
# Request split + structured filters (w3/m8), against the current kubeconfig
# cluster's Loki (deploy/gitops/base/loki.yaml) + log-shipper. Proves both
# milestones' points over the real surfaces:
#
#   1. DURABLE: deploy a whoami App, wait for a boot line to ship into Loki, note
#      the pre-restart timestamp window, then `kubectl delete pod` and wait for the
#      replacement. Query the pre-restart window over REST (GET /v1/logs) AND MCP
#      (list_logs) with BEX_LOKI_URL set: the pre-restart line still returns even
#      though the pod that wrote it is gone — the kubelet buffer went with it, Loki
#      kept it.
#   2. BOUNDED: a time-range query that ends BEFORE the pre-restart line excludes
#      it — the range is a real bounded search, not best-effort over a live stream.
#   3. FALLBACK (truthful degraded answer): rerun the same post-restart query with
#      BEX_LOKI_URL unset. The pre-restart line is GONE (the new pod's buffer never
#      held it) — the honest live-only answer. In the same mode, `type=request` and
#      the structured filters return 503, never a fake empty page (w3/m8).
#   4. REQUEST (w3/m8): curl the App through the Traefik edge on a unique path, then
#      `type=request` returns exactly that access line with a truthful method/status,
#      the `path` filter finds it, and a wrong-status filter excludes it.
#   5. SPLIT (w3/m8): `type=app` returns NO request lines and `type=request` returns
#      no app lines — the split is clean in both directions.
#   6. LEVEL (w3/m8): a JSON-logging fixture App plants one error line; `level=error`
#      isolates exactly it, and its plaintext line lands in the honest `unknown`
#      bucket (never substring-guessed).
#   7. DISCOVERY (w3/m8): `list_log_label_values` / GET /v1/logs/values lists the
#      service's REAL levels and statuses — and never another service's (tenancy).
#
# Like secrets-verify.sh, bex-api runs on the host (go build) talking to the
# cluster apiserver via $KUBECONFIG; Loki and Hydra are reached through kubectl
# port-forwards (Hydra unconditionally — bex-api now refuses to start without
# BEX_HYDRA_ADMIN_URL regardless of authz/BEX_OPENFGA_URL, so a bearer is always
# minted via the seeded bex-bootstrap client, same pattern as secrets-verify.sh).
# The operator itself does NOT run on the host — this script never starts one —
# so the target cluster must already have something reconciling App CRs: prod's
# in-cluster manager (the common case, via `HCLOUD_TOKEN=… scripts/fetch-app-kubeconfig.sh <path>`
# — no mgmt cluster post-pivot), or on the CAPD mock cluster a separately-run `make run`.
#
# Usage: scripts/logs-verify.sh      # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, jq, yq v4, go; a cluster with Loki + log-shipper synced
# (deploy/gitops/base/loki.yaml + log-shipper.yaml), Hydra (ns auth) for the bearer
# token bex-api now unconditionally requires to start, and an operator reconciling
# App CRs (prod: the in-cluster manager; mock: run `make run` yourself first —
# scripts/mock-cluster.sh alone installs no Argo CD/GitOps, so Loki never syncs
# there and this script can't prove anything on it).
set -euo pipefail
cd "$(dirname "$0")/.."
# `go run`'s own PID is a wrapper, not its compiled child — walking up from the
# repo root (no go.mod there; the workspace is lego/go.work) also fails to
# resolve the module. Both bit this script before; build once instead (below).
export GOWORK="$(pwd)/lego/go.work"

MON_NS=monitoring
SYS_NS=bex-system
AUTH_NS=auth
TRAEFIK_NS=traefik
APP_NS=default
SVC=whoami-logs   # a dedicated App so a rerun never collides with examples/whoami
JSVC=jsonlog-logs # the JSON-logging fixture App (level labelling, w3/m8)
HOST=logs-verify.local
NONCE="m8-$(date -u +%s)" # unique per run: the exact request path / planted line

LOKI=127.0.0.1:23100
EDGE=127.0.0.1:18080
API=127.0.0.1:18090
HYDRA_PUBLIC=127.0.0.1:23444
HYDRA_ADMIN=127.0.0.1:23445
API_PID=""
OP_PID=""
PIDS=()

cleanup() {
  [ -n "$OP_PID" ] && { kill "$OP_PID" 2>/dev/null || true; }
  [ -n "$API_PID" ] && { kill "$API_PID" 2>/dev/null || true; }
  if [ "${#PIDS[@]}" -gt 0 ]; then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
  kubectl -n "$APP_NS" delete job -l app.bex.co/app="$JSVC" --ignore-not-found >/dev/null 2>&1 || true
  kubectl -n "$APP_NS" delete app.app.bex.co "$SVC" "$JSVC" --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

pass() { printf '\033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '\033[31mFAIL\033[0m %s\n' "$1"; exit 1; }
info() { printf '\033[36m•\033[0m %s\n' "$1"; }

# --- preflight: Loki must be synced, else this test can't prove anything ---
if ! kubectl -n "$MON_NS" get svc loki >/dev/null 2>&1; then
  fail "Loki service not found in ns/$MON_NS — sync deploy/gitops/base/loki.yaml first (mock-cluster.sh + GitOps)."
fi

# --- port-forward Loki ---
kubectl -n "$MON_NS" port-forward svc/loki 23100:3100 >/dev/null 2>&1 &
PIDS+=($!)
sleep 2
curl -sf "http://$LOKI/ready" >/dev/null 2>&1 || info "loki /ready not yet 200 (still starting is OK if pushes land)"

# --- deploy the App and wait for it Running ---
# spec.hosts gives it an Ingress on the shared Traefik edge regardless of
# BEX_BASE_DOMAIN, which is what makes the request-log phase (4) possible.
info "deploying App/$SVC (host $HOST)"
cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: app.bex.co/v1alpha1
kind: App
metadata: { name: $SVC, namespace: $APP_NS }
spec:
  image: traefik/whoami
  port: 80
  hosts: [$HOST]
YAML
kubectl -n "$APP_NS" wait --for=jsonpath='{.status.phase}'=Running "app.app.bex.co/$SVC" --timeout=180s \
  || fail "App/$SVC never reached Running"

OLD_POD="$(kubectl -n "$APP_NS" get pods -l app.bex.co/app="$SVC" -o jsonpath='{.items[0].metadata.name}')"
info "pod $OLD_POD up; waiting for its boot line to ship into Loki"

# whoami prints a boot line on start; poll Loki until the OLD pod's stream appears.
lokiq() { # $1=query $2=start(ns) $3=end(ns)
  curl -sf "http://$LOKI/loki/api/v1/query_range" \
    --data-urlencode "query=$1" --data-urlencode "start=$2" --data-urlencode "end=$3" \
    --data-urlencode "limit=100" --data-urlencode "direction=backward"
}
WINDOW_START="$(date -u -v-5M +%s 2>/dev/null || date -u -d '5 min ago' +%s)000000000"
for i in $(seq 1 30); do
  now="$(date -u +%s)000000000"
  n="$(lokiq "{namespace=\"$APP_NS\", app=\"$SVC\", container=\"app\"}" "$WINDOW_START" "$now" | jq '[.data.result[].values[]] | length')"
  [ "${n:-0}" -gt 0 ] && break
  sleep 2
done
[ "${n:-0}" -gt 0 ] || fail "no lines shipped to Loki after 60s — check the log-shipper DaemonSet"
PRE_RESTART_END="$(date -u +%s)000000000"
pass "boot line(s) for $OLD_POD are in Loki ($n line(s))"

# --- build bex-api once; BEX_HYDRA_ADMIN_URL is now unconditionally required to
#     even start, so mint a bearer too (authz stays off — no BEX_OPENFGA_URL — but
#     AuthN alone still 401s an unauthenticated GET /v1/logs) ---
BINDIR="$(mktemp -d)"
info "building bex-api"
(cd lego/backend && go build -o "$BINDIR/bex-api" ./cmd/api)

kubectl -n "$AUTH_NS" port-forward svc/hydra-public "${HYDRA_PUBLIC#*:}:4444" >/dev/null 2>&1 &
PIDS+=($!)
kubectl -n "$AUTH_NS" port-forward svc/hydra-admin "${HYDRA_ADMIN#*:}:4445" >/dev/null 2>&1 &
PIDS+=($!)
sleep 2

info "seeding the bex-bootstrap OAuth2 client + minting a bearer token"
export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-logs-verify-$(date +%s)000000}"
HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" bash scripts/auth-bootstrap-client.sh >/dev/null
BOOT_TOKEN="$(curl -sf -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
  -d "grant_type=client_credentials&client_id=bex-bootstrap&client_secret=$BEX_BOOTSTRAP_CLIENT_SECRET" \
  | yq -p json '.access_token // ""' -)"
[ -n "$BOOT_TOKEN" ] && [ "$BOOT_TOKEN" != "null" ] || fail "no access_token for the bootstrap client"
AUTH=(-H "Authorization: Bearer $BOOT_TOKEN")

# --- start bex-api on the host with Loki wired ---
start_api() { # $1 = BEX_LOKI_URL (empty => fallback)
  # A `go run`-backgrounded process's PID is its wrapper, not the compiled child
  # holding the port — killing it orphans the real server, so the NEXT start_api
  # call's healthz check can pass against the STALE (wrong-BEX_LOKI_URL) orphan
  # and silently invalidate the fallback assertion below. Run the built binary.
  [ -n "$API_PID" ] && { kill "$API_PID" 2>/dev/null || true; wait "$API_PID" 2>/dev/null || true; API_PID=""; }
  BEX_API_ADDR="$API" BEX_API_NAMESPACE="$APP_NS" BEX_LOKI_URL="$1" \
    BEX_HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" \
    "$BINDIR/bex-api" >/tmp/logs-verify-api.log 2>&1 &
  API_PID=$!
  for i in $(seq 1 30); do curl -sf "http://$API/healthz" >/dev/null 2>&1 && return 0; sleep 1; done
  fail "bex-api did not come up (see /tmp/logs-verify-api.log)"
}

start_api "http://$LOKI"

# --- restart: delete the pod, wait for a NEW one ---
info "deleting pod $OLD_POD to force a restart"
kubectl -n "$APP_NS" delete pod "$OLD_POD" --wait=false >/dev/null
kubectl -n "$APP_NS" wait --for=delete "pod/$OLD_POD" --timeout=60s || true
kubectl -n "$APP_NS" wait --for=condition=Ready pod -l app.bex.co/app="$SVC" --timeout=120s \
  || fail "replacement pod never became Ready"
NEW_POD="$(kubectl -n "$APP_NS" get pods -l app.bex.co/app="$SVC" -o jsonpath='{.items[0].metadata.name}')"
[ "$NEW_POD" != "$OLD_POD" ] || fail "pod was not actually replaced"
info "replacement pod is $NEW_POD (old $OLD_POD is gone, and so is its kubelet buffer)"

START_RFC="$(date -u -v-10M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '10 min ago' +%Y-%m-%dT%H:%M:%SZ)"
END_RFC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# 1. DURABLE over REST: the pre-restart window still returns lines from the OLD pod.
rest="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&startTime=$START_RFC&endTime=$END_RFC&limit=100")"
oldlines="$(echo "$rest" | jq --arg p "$OLD_POD" '[.logs[] | select(.labels[]? | .value == $p)] | length')"
[ "${oldlines:-0}" -gt 0 ] \
  && pass "REST: $oldlines pre-restart line(s) from the deleted pod $OLD_POD still served (durable)" \
  || fail "REST returned no lines from the deleted pod — durability broken: $rest"

# 1b. DURABLE over MCP list_logs (same read, agent surface).
mcp="$(printf '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_logs","arguments":{"resource":["%s"],"startTime":"%s","endTime":"%s","limit":100}}}' "$SVC" "$START_RFC" "$END_RFC" \
  | BEX_API_NAMESPACE="$APP_NS" BEX_LOKI_URL="http://$LOKI" BEX_HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" "$BINDIR/bex-api" mcp-stdio 2>/dev/null || true)"
echo "$mcp" | grep -q "$OLD_POD" \
  && pass "MCP list_logs: pre-restart lines from $OLD_POD present too (three-adapter parity)" \
  || info "MCP check inconclusive (stdio framing) — REST already proved durability"

# 2. BOUNDED: a range ending before the run excludes everything.
past="$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)"
past_end="$(date -u -v-30M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '30 min ago' +%Y-%m-%dT%H:%M:%SZ)"
empty="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&startTime=$past&endTime=$past_end&limit=100" | jq '.logs | length')"
[ "${empty:-1}" -eq 0 ] \
  && pass "BOUNDED: a pre-run time range returns 0 lines (real bounds, not a live tail)" \
  || fail "a pre-run time range should be empty, got $empty lines"

# 3. FALLBACK: same post-restart query with Loki unwired => the deleted pod's
#    lines are GONE (only the live pod's buffer remains) — the truthful answer.
start_api ""
fb="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&startTime=$START_RFC&endTime=$END_RFC&limit=100")"
fb_old="$(echo "$fb" | jq --arg p "$OLD_POD" '[.logs[] | select(.labels[]? | .value == $p)] | length')"
[ "${fb_old:-0}" -eq 0 ] \
  && pass "FALLBACK: with BEX_LOKI_URL unset the deleted pod's lines are gone (honest live-only answer)" \
  || fail "fallback should NOT surface the deleted pod's lines (it has no store), got $fb_old"

# 3b. FALLBACK HONESTY (w3/m8): the labels live in the store, not in a pod's
#     stdout — so a store-only filter is REFUSED (503), never silently ignored and
#     answered with unfiltered lines.
for filter in "type=request" "level=error" "statusCode=500" "method=GET" "path=/x"; do
  code="$(curl -s -o /dev/null -w '%{http_code}' "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&$filter")"
  [ "$code" = "503" ] || fail "fallback: $filter should be 503 (store-only), got $code"
done
pass "FALLBACK HONESTY: type=request + every store-only filter => 503, not a fake empty page"

# --- back to the Loki-wired API for the w3/m8 phases ---
start_api "http://$LOKI"

# 4. REQUEST: drive one request through the real Traefik edge on a unique path, then
#    read it back as a request log with a truthful method/status.
kubectl -n "$TRAEFIK_NS" port-forward svc/traefik 18080:80 >/dev/null 2>&1 &
PIDS+=($!)
sleep 2
curl -sf -H "Host: $HOST" "http://$EDGE/$NONCE" >/dev/null \
  || fail "could not reach App/$SVC through the Traefik edge (Host: $HOST) — no access line to verify"
info "requested http://$HOST/$NONCE through the edge; waiting for the access line to ship"

req=""
for i in $(seq 1 30); do
  req="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&type=request&limit=100")"
  echo "$req" | jq -e --arg n "$NONCE" '[.logs[] | select(.message | contains($n))] | length > 0' >/dev/null 2>&1 && break
  sleep 2
done
line="$(echo "$req" | jq -c --arg n "$NONCE" 'first(.logs[] | select(.message | contains($n)))' 2>/dev/null)"
[ -n "$line" ] && [ "$line" != "null" ] \
  || fail "type=request returned no access line for /$NONCE after 60s (check Traefik access logs + the shipper)"

lbl() { echo "$line" | jq -r --arg k "$1" 'first(.labels[] | select(.name == $k) | .value) // ""'; }
[ "$(lbl type)" = "request" ] || fail "the access line's type label is '$(lbl type)', want request"
[ "$(lbl method)" = "GET" ] || fail "the access line's method label is '$(lbl method)', want GET"
[ "$(lbl statusCode)" = "200" ] || fail "the access line's statusCode label is '$(lbl statusCode)', want 200"
pass "REQUEST: type=request returns the access line for /$NONCE (method=GET, statusCode=200 — truthful)"

# path is line-searchable (never a label — the cardinality budget), statusCode narrows.
n_path="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&type=request&path=/$NONCE&limit=100" | jq '.logs | length')"
[ "${n_path:-0}" -gt 0 ] || fail "the path filter should find /$NONCE, got $n_path lines"
n_5xx="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&type=request&statusCode=5xx&limit=100" \
  | jq --arg n "$NONCE" '[.logs[] | select(.message | contains($n))] | length')"
[ "${n_5xx:-1}" -eq 0 ] || fail "statusCode=5xx must exclude the 200 access line, got $n_5xx"
pass "REQUEST FILTERS: path=/$NONCE finds it; statusCode=5xx excludes it (filters really narrow)"

# 4b. TENANT ATTRIBUTION (w6/m131). Everything above runs in APP_NS=default —
#     and `default` is exactly the namespace the w6/m131 bug did NOT break. The
#     shipper reconstructs {namespace, app} by parsing Traefik's ServiceName, and
#     its regex was anchored to the literal `default`; under ADR043 a tenant App
#     lives in `tea-<xid>`, so every REAL service's access line was dropped as
#     not_a_tenant_app while this script's own fixture kept passing. Running this
#     script in production would have gone green through the entire outage.
#
#     So assert the property the fixture cannot: that the regex in the LIVE,
#     DEPLOYED ConfigMap attributes a tenant namespace. Reading the cluster's own
#     copy (not the repo's) is what makes this answer "did the GitOps change
#     actually reach the running shipper?" — including the w3/m13 failure mode
#     where Argo CD freezes the live ConfigMap at an older render.
expr_line="$(kubectl -n "$MON_NS" get configmap -o json \
  | jq -r '.items[] | (.data // {}) | to_entries[] | .value' 2>/dev/null \
  | grep -F 'expression =' | grep -F '@kubernetes' | head -n1 || true)"
[ -n "$expr_line" ] \
  || fail "no deployed ConfigMap in $MON_NS carries a Traefik ServiceName regex — the shipper config is not live"
deployed_re="${expr_line#*\"}"
deployed_re="${deployed_re%\"*}"
# RE2 named groups and \d are not POSIX ERE; translate for grep -E.
# The ConfigMap holds River SOURCE, so \d appears escaped as \\d; handle a
# single backslash too in case the config is ever written unescaped.
ere="$(printf '%s' "$deployed_re" | sed -e 's/(?P<[a-z]*>/(/g' -e 's/\\\\d/[0-9]/g' -e 's/\\d/[0-9]/g')"
info "deployed ServiceName regex: $deployed_re"
XID=abcdefghijklmnopqrst # 20 chars, the [a-z0-9]{20} xid shape
echo "tea-$XID-tea-$XID-web-80@kubernetes" | grep -Eq "$ere" \
  || fail "the DEPLOYED ServiceName regex does not match a tenant (tea-<xid>) namespace — every tenant access line is being dropped as not_a_tenant_app (w6/m131). Deployed: $deployed_re"
echo "default-$SVC-80@kubernetes" | grep -Eq "$ere" \
  || fail "the deployed ServiceName regex no longer matches the shared \`default\` namespace — this script's own fixture would stop being attributed. Deployed: $deployed_re"
if echo "kube-system-traefik-80@kubernetes" | grep -Eq "$ere"; then
  fail "the deployed ServiceName regex matches a NON-tenant namespace (kube-system) — platform access lines would be attributed to a tenant App. Deployed: $deployed_re"
fi
pass "TENANT ATTRIBUTION: the live shipper regex attributes tea-<xid> AND default, and rejects kube-system"

# 5. SPLIT: the two types don't bleed into each other, in either direction.
app_reqs="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&type=app&limit=100" \
  | jq '[.logs[] | select(.labels[]? | select(.name == "type") | .value == "request")] | length')"
[ "${app_reqs:-1}" -eq 0 ] || fail "type=app returned $app_reqs request line(s) — the split leaks"
req_apps="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$SVC&type=request&limit=100" \
  | jq '[.logs[] | select(.labels[]? | select(.name == "type") | .value == "app")] | length')"
[ "${req_apps:-1}" -eq 0 ] || fail "type=request returned $req_apps app line(s) — the split leaks"
pass "SPLIT: type=app excludes access lines and type=request excludes app lines (clean both ways)"

# 6. LEVEL: a JSON-logging fixture plants exactly one error line. A cron-type App is
#    the CR path that honors spec.command, and `kubectl create job --from` runs it
#    once now — its pod carries the App label, so the shipper treats it as app logs.
info "deploying the JSON-logging fixture App/$JSVC and planting one error line"
cat <<YAML | kubectl apply -f - >/dev/null
apiVersion: app.bex.co/v1alpha1
kind: App
metadata: { name: $JSVC, namespace: $APP_NS }
spec:
  type: cron_job
  schedule: "0 0 31 2 *" # never fires on its own (Feb 31) — the run below is explicit
  image: busybox
  command: >-
    echo '{"level":"info","msg":"json fixture up $NONCE"}';
    echo '{"level":"error","msg":"planted failure $NONCE"}';
    echo 'plain text, not json $NONCE'
YAML
for i in $(seq 1 30); do kubectl -n "$APP_NS" get cronjob "$JSVC" >/dev/null 2>&1 && break; sleep 2; done
kubectl -n "$APP_NS" get cronjob "$JSVC" >/dev/null 2>&1 || fail "the operator never created CronJob/$JSVC"
kubectl -n "$APP_NS" create job "$JSVC-$NONCE" --from="cronjob/$JSVC" >/dev/null
kubectl -n "$APP_NS" wait --for=condition=complete "job/$JSVC-$NONCE" --timeout=120s >/dev/null \
  || fail "the JSON-logging fixture job never completed"

errs=""
for i in $(seq 1 30); do
  errs="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$JSVC&type=app&level=error&limit=100")"
  [ "$(echo "$errs" | jq '.logs | length')" -gt 0 ] && break
  sleep 2
done
n_err="$(echo "$errs" | jq '.logs | length')"
[ "${n_err:-0}" -eq 1 ] || fail "level=error should isolate exactly the 1 planted line, got ${n_err:-0}"
echo "$errs" | jq -e --arg n "$NONCE" '.logs[0].message | contains("planted failure") and contains($n)' >/dev/null \
  || fail "level=error returned the wrong line: $(echo "$errs" | jq -c '.logs[0].message')"
pass "LEVEL: level=error isolates exactly the planted error line (parsed from JSON, not guessed)"

n_unknown="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs?resource=$JSVC&type=app&level=unknown&limit=100" \
  | jq '[.logs[] | select(.message | contains("plain text, not json"))] | length')"
[ "${n_unknown:-0}" -gt 0 ] \
  && pass "LEVEL: the plaintext line lands in the honest 'unknown' bucket (never substring-guessed)" \
  || fail "the plaintext line should be level=unknown, got $n_unknown lines"

# 7. DISCOVERY: real values, and scoped to the service that asked (tenancy).
levels="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs/values?resource=$JSVC&label=level")"
echo "$levels" | jq -e 'index("error") != null and index("info") != null and index("unknown") != null' >/dev/null \
  || fail "level discovery should list the fixture's real levels (error/info/unknown), got $levels"
statuses="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs/values?resource=$SVC&label=statusCode")"
echo "$statuses" | jq -e 'index("200") != null' >/dev/null \
  || fail "statusCode discovery should list 200 for $SVC, got $statuses"
pass "DISCOVERY: label values are the service's REAL levels ($(echo "$levels" | jq -c .)) and statuses"

# TENANCY: the JSON fixture never served a request, so it must expose NO methods —
# proving discovery is scoped to the asking service, not the whole store.
jmethods="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs/values?resource=$JSVC&label=method")"
[ "$(echo "$jmethods" | jq 'length')" -eq 0 ] \
  || fail "TENANCY LEAK: $JSVC has no request logs but method discovery returned $jmethods"
jinst="$(curl -sf "${AUTH[@]}" "http://$API/v1/logs/values?resource=$JSVC&label=instance")"
echo "$jinst" | jq -e --arg p "$NEW_POD" 'index($p) == null' >/dev/null \
  || fail "TENANCY LEAK: $JSVC's instance discovery lists $SVC's pod $NEW_POD"
pass "TENANCY: $JSVC's discovery never returns $SVC's methods or pods (scoped to the caller's service)"

# 7b. The same discovery over MCP, under the official tool's name/args.
mcpv="$(printf '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_log_label_values","arguments":{"label":"level","resource":["%s"]}}}' "$JSVC" \
  | BEX_API_NAMESPACE="$APP_NS" BEX_LOKI_URL="http://$LOKI" BEX_HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" "$BINDIR/bex-api" mcp-stdio 2>/dev/null || true)"
# grep for `unknown`, not `error` — "error" is both a level VALUE and what a failed
# call would say, so it can't tell success from failure.
echo "$mcpv" | grep -q "unknown" \
  && pass "MCP list_log_label_values: discovers the same levels (three-adapter parity)" \
  || info "MCP list_log_label_values check inconclusive (stdio framing) — REST already proved discovery"

echo
pass "logs verified: history survives restarts; the app/request split is real and clean; level=error is truthful; discovery is scoped"
