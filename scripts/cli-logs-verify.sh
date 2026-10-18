#!/usr/bin/env bash
set -euo pipefail

# Durable-logs acceptance for the unmodified Render CLI (render-oss/cli) against
# a Loki-backed bex (docs/ADR010-observability.md; docs/cli-compatibility-checklist.md
# `logs` rows). The dev-9 baseline runs without Loki, so the store-only `render logs`
# flags (--level/--type request/--host/--status-code/--method/--path) could only be
# graded "503, needs the durable store" there. This supplement grades them for real:
# it creates a disposable busybox web-service fixture whose start command plants one
# JSON error line + one JSON info line + one plaintext line, drives two GET requests
# through the public edge (a 200 on `/` and a 404 on a nonce path), then asserts each
# flag through the CLI's own `-o json` output — positive match AND negative exclusion,
# never a silently-ignored filter (the same honesty rules as scripts/logs-verify.sh,
# which proves these paths over raw REST/MCP; this script proves the CLI surface).
#
#   SPLIT   --type app excludes request lines; --type request excludes app lines
#   LEVEL   --level error isolates exactly the planted JSON error line; --level
#           critical is an honest empty. (The CLI's own --level enum has no
#           `unknown`, so the plaintext line's honest bucket is REST-only.)
#   PATH    --path matches only the nonce path; an absent path is an honest empty
#   HOST    --host matches the fixture's public host; a wrong host is empty
#   METHOD  --method GET matches the probes; --method POST is empty
#   STATUS  --status-code 404 matches only the 404 probe (200 excludes it, and
#           vice versa); the 4xx class shorthand matches it too
#
# Requires: a bex with Loki + log-shipper synced (deploy/gitops/base/loki.yaml +
# log-shipper.yaml), the public app edge reachable from this runner (Traefik +
# wildcard DNS/TLS — production-shaped), and the fixture image pullable in-cluster.
# Creates and always deletes one single-replica web service. The blocked-only
# replay uses a Free fixture; the full historical matrix uses Starter.
#
#   BEX_API_URL                              e.g. https://api.bex.co
#   BEX_API_TOKEN                            bearer, never echoed
#   BEX_RENDER_CLI_BIN                       default: render
#   BEX_LOGS_VERIFY_BLOCKED_ONLY             1: only Free IPAllowList replay
#   BEX_LOGS_VERIFY_WORKSPACE_ID             optional fixture owner/selected workspace
#   BEX_LOGS_VERIFY_READY_TIMEOUT_SECONDS    default: 300
#   BEX_LOGS_VERIFY_SHIP_TIMEOUT_SECONDS     default: 240 (edge line -> Loki -> CLI)

fail() {
  echo "FAIL $*" >&2
  exit 1
}

for command in curl jq python3; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

: "${BEX_API_URL:?set BEX_API_URL, e.g. https://api.bex.co}"
: "${BEX_API_TOKEN:?set a bearer token without echoing it}"

render_cli="${BEX_RENDER_CLI_BIN:-render}"
if [[ "$render_cli" == */* ]]; then
  [[ -x "$render_cli" ]] || fail "Render CLI is not executable: $render_cli"
else
  command -v "$render_cli" >/dev/null || fail "missing Render CLI: $render_cli"
fi

api="${BEX_API_URL%/}"
tmp="$(mktemp -d)"
nonce="qa-$(date -u +%Y%m%d)-$(date +%s)-$(( $$ % 100000 ))"
service_id=""
service_created=0
blocked_only="${BEX_LOGS_VERIFY_BLOCKED_ONLY:-0}"
[[ "$blocked_only" == "0" || "$blocked_only" == "1" ]] || fail "BEX_LOGS_VERIFY_BLOCKED_ONLY must be 0 or 1"

delete_fixture() {
  local deadline=$((SECONDS + 60)) absence=""
  request_deadline="$deadline" api_json DELETE "/v1/services/$service_id" >/dev/null || return 1
  while ((SECONDS < deadline)); do
    absence="$(request_deadline="$deadline" api_curl -sS -o /dev/null -w '%{http_code}' "$api/v1/services/$service_id")" || absence=""
    if [[ "$absence" == "404" ]]; then
      service_created=0
      echo "PASS disposable fixture deleted; GET absence verified"
      return 0
    fi
    sleep 2
  done
  echo "FAIL fixture still readable after delete (HTTP $absence)" >&2
  return 1
}

cleanup() {
  local result=$?
  if [[ "$service_created" == "1" && -n "$service_id" ]]; then
    delete_fixture || result=1
  fi
  rm -rf "$tmp"
  return "$result"
}
trap cleanup EXIT

auth=(-H "Authorization: Bearer $BEX_API_TOKEN")

request_timeout() {
  local remaining=30
  if [[ -n "${request_deadline:-}" ]]; then
    remaining=$((request_deadline - SECONDS))
    ((remaining > 0)) || return 124
    ((remaining < 30)) || remaining=30
  fi
  printf '%s\n' "$remaining"
}

api_curl() {
  local seconds
  seconds="$(request_timeout)" || return 124
  curl --connect-timeout 10 --max-time "$seconds" "${auth[@]}" "$@"
}

api_json() {
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    api_curl -fsS -X "$method" -H 'Content-Type: application/json' -d "$body" "$api$path"
  else
    api_curl -fsS -X "$method" "$api$path"
  fi
}

cli_version_output="$("$render_cli" --version 2>&1)" || fail "Render CLI did not report its version"
cli_version="${cli_version_output%%$'\n'*}"
echo "unmodified Render CLI: $cli_version -> $api"

# Every graded read goes through the unmodified CLI with only its two documented
# overrides (RENDER_HOST/RENDER_API_KEY) and an isolated config file, exactly like
# scripts/cli-compat.sh. `-o json` emits a stream of {id,labels,timestamp,message}
# objects; jq -s turns it into the array the assertions consume.
cli_logs() {
  local seconds
  seconds="$(request_timeout)" || return 124
  # The pinned upstream client uses http.Client{} without a request timeout.
  RENDER_CLI_CONFIG_PATH="$tmp/cli.yaml" RENDER_HOST="$api/v1/" RENDER_API_KEY="$BEX_API_TOKEN" \
    RENDER_WORKSPACE="$workspace_id" \
    python3 - "$seconds" "$render_cli" logs -r "$service_id" -o json --confirm "$@" <<'PY' | jq -s '.'
import subprocess, sys
try:
    result = subprocess.run(sys.argv[2:], timeout=int(sys.argv[1]))
except subprocess.TimeoutExpired:
    print("FAIL CLI log read timed out", file=sys.stderr)
    sys.exit(124)
sys.exit(result.returncode)
PY
}

# jq prelude: pull a named label off an entry ("" when absent — path/host stay in
# the line by design, so those assertions parse the access-log JSON message).
JQ_LAB='def lab(n): ([.labels[]? | select(.name == n) | .value] | first) // "";'

# The name doubles as the fixture's DNS label (30-char cap), so reuse the nonce.
service_name="$nonce"
# Boot plants the three level-fixture lines on the App's own stdout (the shipper
# labels level from the line's JSON — log-shipper.yaml), then serves a static ok
# page so `/` is a deterministic 200 and any other path a deterministic 404.
start_command="echo '{\"level\":\"error\",\"msg\":\"planted error $nonce\"}'; echo '{\"level\":\"info\",\"msg\":\"fixture up $nonce\"}'; echo 'plaintext fixture line $nonce'; mkdir -p /tmp/site && printf ok >/tmp/site/index.html && exec httpd -f -p 8080 -h /tmp/site"
fixture_plan="starter"
[[ "$blocked_only" == "0" ]] || fixture_plan="free"
create_payload="$(jq -n --arg name "$service_name" --arg start "$start_command" --arg plan "$fixture_plan" \
  --arg owner "${BEX_LOGS_VERIFY_WORKSPACE_ID:-}" '{
  type:"web_service", name:$name, image:{imagePath:"busybox:1.36.1",ownerId:""}, port:8080,
  serviceDetails:{
    plan:$plan, numInstances:1, healthCheckPath:"/", runtime:"image",
    envSpecificDetails:{startCommand:$start}
  }
} + (if $owner == "" then {} else {ownerId:$owner} end)
')"
created="$(api_json POST /v1/services "$create_payload")"
service_id="$(jq -er '.service.id' <<<"$created")"
workspace_id="$(jq -er '.service.ownerId' <<<"$created")" # the CLI refuses to run workspace-less
service_created=1
echo "created disposable fixture service id=$service_id name=$service_name"

deadline=$((SECONDS + ${BEX_LOGS_VERIFY_READY_TIMEOUT_SECONDS:-300}))
url=""
while ((SECONDS < deadline)); do
  service_json="$(api_json GET "/v1/services/$service_id")" || { sleep 3; continue; }
  url="$(jq -r '.serviceDetails.url // empty' <<<"$service_json")"
  [[ "$(jq -r '.phase // empty' <<<"$service_json")" == "Running" && -n "$url" ]] && break
  url=""
  sleep 3
done
[[ -n "$url" ]] || fail "fixture never reached Running with a public URL"
host="${url#*://}"
host="${host%%/*}"

# The edge (DNS/TLS/route) can lag Running by a beat; a 200 on `/` proves the
# whole request path this script is about to generate access lines through.
while ((SECONDS < deadline)); do
  [[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/")" == "200" ]] && break
  sleep 3
done
[[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/")" == "200" ]] || fail "public edge never served 200 on $url/"
echo "fixture Running and serving at $url"

probe_path="/missing-$nonce"
[[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url$probe_path")" == "404" ]] || fail "nonce path did not 404 through the edge"
curl -s -o /dev/null --max-time 10 "$url/" || true
echo "generated edge traffic: GET / (200), GET $probe_path (404)"

if [[ "$blocked_only" == "0" ]]; then
# Wait until both streams land in the store *through the CLI itself*: the planted
# app lines and the nonce-path access line. Everything after this is deterministic.
ship_deadline=$((SECONDS + ${BEX_LOGS_VERIFY_SHIP_TIMEOUT_SECONDS:-240}))
while ((SECONDS < ship_deadline)); do
  app_seen="$(cli_logs --type app --text "$nonce" --limit 100 | jq 'length')"
  req_seen="$(cli_logs --type request --path "$probe_path" --limit 100 | jq 'length')"
  [[ "$app_seen" -ge 3 && "$req_seen" -ge 1 ]] && break
  sleep 5
done
[[ "${app_seen:-0}" -ge 3 ]] || fail "the 3 planted app lines never shipped into the store (saw $app_seen)"
[[ "${req_seen:-0}" -ge 1 ]] || fail "the nonce-path access line never shipped into the store"
echo "both streams shipped (app fixture lines: $app_seen, nonce request lines: $req_seen)"

# SPLIT — --type app and --type request exclude each other, both directions.
app_logs="$(cli_logs --type app --limit 100)"
jq -e "$JQ_LAB"' [.[] | select(lab("type") == "request")] | length == 0' <<<"$app_logs" >/dev/null \
  || fail "--type app returned request-labelled lines"
jq -e --arg n "$nonce" '[.[] | select(.message | contains($n))] | length >= 3' <<<"$app_logs" >/dev/null \
  || fail "--type app is missing the planted fixture lines"
req_logs="$(cli_logs --type request --limit 100)"
jq -e "$JQ_LAB"' length > 0 and ([.[] | select(lab("type") != "request")] | length == 0)' <<<"$req_logs" >/dev/null \
  || fail "--type request returned non-request lines (or nothing)"
jq -e --arg n "planted error $nonce" '[.[] | select(.message | contains($n))] | length == 0' <<<"$req_logs" >/dev/null \
  || fail "--type request leaked an app line"
echo "PASS --type: app/request split clean both directions"

# LEVEL — the planted JSON error line, isolated; a level nothing logged at is an
# honest empty (exit 0, no lines), never a silently-unfiltered page.
lvl="$(cli_logs --type app --level error --limit 100)"
jq -e "$JQ_LAB"' length > 0 and ([.[] | select(lab("level") != "error")] | length == 0)' <<<"$lvl" >/dev/null \
  || fail "--level error returned non-error lines (or nothing)"
jq -e --arg n "planted error $nonce" '[.[] | select(.message | contains($n))] | length == 1' <<<"$lvl" >/dev/null \
  || fail "--level error did not isolate exactly the planted error line"
jq -e '[.[] | select(.message | test("fixture up|plaintext fixture"))] | length == 0' <<<"$lvl" >/dev/null \
  || fail "--level error leaked the info/plaintext fixture lines"
crit="$(cli_logs --type app --level critical --limit 100)"
[[ "$(jq 'length' <<<"$crit")" == "0" ]] || fail "--level critical should be an honest empty"
echo "PASS --level: error isolates the planted line; critical is an honest empty"

# PATH — line-level filter parsed out of the access-log JSON; absent path empty.
p="$(cli_logs --path "$probe_path" --limit 100)"
jq -e --arg p "$probe_path" 'length > 0 and all(.[]; .message | fromjson | .RequestPath == $p)' <<<"$p" >/dev/null \
  || fail "--path returned lines for other paths (or nothing)"
[[ "$(cli_logs --path "/absent-$nonce" --limit 100 | jq 'length')" == "0" ]] \
  || fail "--path on an absent path should be an honest empty"
echo "PASS --path: matches only $probe_path; absent path empty"

# HOST — the fixture's public host matches; a host nothing was served on is empty.
h="$(cli_logs --host "$host" --path "$probe_path" --limit 100)"
jq -e --arg h "$host" 'length > 0 and all(.[]; .message | fromjson | .RequestHost == $h)' <<<"$h" >/dev/null \
  || fail "--host returned lines for other hosts (or nothing)"
[[ "$(cli_logs --host "absent-$nonce.invalid" --limit 100 | jq 'length')" == "0" ]] \
  || fail "--host on an absent host should be an honest empty"
echo "PASS --host: matches only $host; absent host empty"

# METHOD — the GET probes match; POST (never sent) is an honest empty.
m="$(cli_logs --method GET --path "$probe_path" --limit 100)"
jq -e "$JQ_LAB"' length > 0 and all(.[]; lab("method") == "GET" and (.message | fromjson | .RequestMethod == "GET"))' <<<"$m" >/dev/null \
  || fail "--method GET returned non-GET lines (or nothing)"
[[ "$(cli_logs --method POST --path "$probe_path" --limit 100 | jq 'length')" == "0" ]] \
  || fail "--method POST should be an honest empty (no POST was sent)"
echo "PASS --method: GET matches; POST empty"

# STATUS — exact code matches exactly (200 excludes the 404 probe and vice
# versa), and Render's 4xx class shorthand matches it too.
s404="$(cli_logs --status-code 404 --path "$probe_path" --limit 100)"
jq -e "$JQ_LAB"' length > 0 and all(.[]; lab("statusCode") == "404" and (.message | fromjson | .DownstreamStatus == 404))' <<<"$s404" >/dev/null \
  || fail "--status-code 404 returned non-404 lines (or nothing)"
[[ "$(cli_logs --status-code 200 --path "$probe_path" --limit 100 | jq 'length')" == "0" ]] \
  || fail "--status-code 200 should exclude the 404 probe line"
jq -e 'length > 0' <<<"$(cli_logs --status-code 200 --path / --limit 100)" >/dev/null \
  || fail "--status-code 200 should match the 200 probe on /"
jq -e 'length > 0' <<<"$(cli_logs --status-code 4xx --path "$probe_path" --limit 100)" >/dev/null \
  || fail "--status-code 4xx class shorthand should match the 404 probe"
echo "PASS --status-code: exact 404/200 exclusion both ways; 4xx class matches"
fi

# EDGE REFUSALS — the app never sees this request. Traefik still records its
# ingress identity (OriginStatus=0, ServiceName absent), so both readers must
# expose the same two lines within two minutes. The nonce path avoids counting
# public scanner traffic or the separate readiness probe.
api_json PATCH "/v1/services/$service_id" '{"serviceDetails":{"ipAllowList":[{"cidrBlock":"203.0.113.7/32","description":"owned blocked-request fixture"}]}}' >/dev/null
edge_deadline=$((SECONDS + ${BEX_LOGS_VERIFY_READY_TIMEOUT_SECONDS:-300}))
while ((SECONDS < edge_deadline)); do
  [[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/await-deny-$nonce")" == "403" ]] && break
  sleep 3
done
[[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/await-deny-$nonce")" == "403" ]] || fail "IPAllowList never refused the client"
blocked_path="/blocked-$nonce"
for attempt in 1 2; do
  [[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url$blocked_path")" == "403" ]] || fail "blocked probe $attempt did not return 403"
done
blocked_deadline=$((SECONDS + 120))
while ((SECONDS < blocked_deadline)); do
  blocked_cli="$(request_deadline="$blocked_deadline" cli_logs --type request --status-code 403 --path "$blocked_path" --limit 100)"
  blocked_rest="$(request_deadline="$blocked_deadline" api_json GET "/v1/logs?resource=$service_id&type=request&statusCode=403&path=$blocked_path&limit=100" | jq '.logs')"
  ((SECONDS < blocked_deadline)) || fail "refusal reads exceeded the 120s deadline"
  [[ "$(jq length <<<"$blocked_cli")" == "2" && "$(jq length <<<"$blocked_rest")" == "2" ]] && break
  sleep 5
done
((SECONDS < blocked_deadline)) || fail "refusal lines did not arrive within 120s"
for reader in CLI REST; do
  if [[ "$reader" == "CLI" ]]; then lines="$blocked_cli"; else lines="$blocked_rest"; fi
  jq -e --arg p "$blocked_path" --arg h "$host" "$JQ_LAB"' length == 2 and all(.[];
    lab("type") == "request" and lab("statusCode") == "403" and
    (.message | fromjson | .RequestPath == $p and .RequestHost == $h and
      .RequestMethod == "GET" and .DownstreamStatus == 403 and .OriginStatus == 0))' <<<"$lines" >/dev/null \
    || fail "$reader did not expose both truthful edge-refusal lines within 120s"
done
[[ "$(jq -c '[.[].id] | sort' <<<"$blocked_cli")" == "$(jq -c '[.[].id] | sort' <<<"$blocked_rest")" ]] \
  || fail "CLI and raw REST returned different refused-line identities"
for status in 403 4xx; do
  [[ "$(cli_logs --type request --status-code "$status" --path "$blocked_path" --limit 100 | jq length)" == "2" ]] \
    || fail "--status-code $status did not match both refusals"
done
[[ "$(cli_logs --type request --status-code 200 --path "$blocked_path" --limit 100 | jq length)" == "0" ]] \
  || fail "--status-code 200 failed to exclude the edge 403s"
[[ "$(cli_logs --type request --path "$blocked_path" --limit 100 | jq length)" == "2" ]] \
  || fail "--path did not return both refusals without a status filter"
jq -e --arg p "$blocked_path" '[.[] | select((.message | fromjson | .RequestPath) == $p)] | length == 2' \
  <<<"$(cli_logs --type request --limit 100)" >/dev/null || fail "unfiltered request history omitted refusals"
api_json GET "/v1/logs/values?resource=$service_id&label=statusCode" | jq -e 'index("403") != null' >/dev/null \
  || fail "statusCode discovery omitted visible 403s"
echo "PASS edge IPAllowList: two truthful 403s in CLI + raw REST; exact/class/path filters, exclusion, history, discovery agree"

# The dashboard reads these same GraphQL fields. Compare records, not counts:
# two different or misattributed lines would otherwise make a false pass.
gql_payload="$(jq -n --arg resource "$service_id" --arg path "$blocked_path" '{
  query:"query BlockedLogs($resource:String!,$path:String!){logs(resource:$resource,type:\"request\",statusCode:[\"403\"],path:[$path],limit:100){logs{id timestamp message type method statusCode}} logLabelValues(resource:$resource,label:\"statusCode\",type:\"request\")}",
  variables:{resource:$resource,path:$path}}')"
blocked_gql="$(api_json POST /graphql "$gql_payload")"
jq -e '(.errors // [] | length) == 0 and (.data.logLabelValues | index("403") != null) and
  (.data.logs.logs | length == 2 and all(.[]; .type == "request" and .method == "GET" and .statusCode == "403"))' \
  <<<"$blocked_gql" >/dev/null || fail "GraphQL request logs or discovery disagree"
[[ "$(jq -c '[.data.logs.logs[].id] | sort' <<<"$blocked_gql")" == "$(jq -c '[.[].id] | sort' <<<"$blocked_rest")" ]] \
  || fail "GraphQL and REST returned different refusal records"

mcp_payload="$(jq -n --arg resource "$service_id" --arg path "$blocked_path" '{
  jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"list_logs",arguments:{
    resource:[$resource],type:["request"],statusCode:["403"],path:[$path],limit:100}}}')"
blocked_mcp="$(api_curl -fsS -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-06-18' \
  -X POST -d "$mcp_payload" "$api/mcp")"
if grep -q '^data: ' <<<"$blocked_mcp"; then
  blocked_mcp="$(sed -n 's/^data: //p' <<<"$blocked_mcp" | tail -1)"
fi
jq -e '.error == null and .result.isError != true and
  (.result.structuredContent.logs | length == 2 and all(.[]; .labels.type == "request" and .labels.statusCode == "403"))' \
  <<<"$blocked_mcp" >/dev/null || fail "MCP list_logs did not return both refusals"
[[ "$(jq -c '[.result.structuredContent.logs[] | {timestamp,message}] | sort_by(.timestamp,.message)' <<<"$blocked_mcp")" == \
   "$(jq -c '[.[] | {timestamp,message}] | sort_by(.timestamp,.message)' <<<"$blocked_rest")" ]] \
  || fail "MCP and REST returned different refusal records"
echo "PASS GraphQL + MCP: the same two refused records; dashboard GraphQL status discovery offers 403"

api_json PATCH "/v1/services/$service_id" '{"serviceDetails":{"ipAllowList":[{"cidrBlock":"0.0.0.0/0"}]}}' >/dev/null
edge_deadline=$((SECONDS + ${BEX_LOGS_VERIFY_READY_TIMEOUT_SECONDS:-300}))
while ((SECONDS < edge_deadline)); do
  [[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/")" == "200" ]] && break
  sleep 3
done
[[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/")" == "200" ]] || fail "clearing IPAllowList did not restore access"
[[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/index.html")" == "200" ]] || fail "restored probe did not forward"
while ((SECONDS < edge_deadline)); do
  [[ "$(cli_logs --type request --status-code 200 --path /index.html --limit 100 | jq length)" -gt 0 ]] && break
  sleep 5
done
[[ "$(cli_logs --type request --status-code 200 --path /index.html --limit 100 | jq length)" -gt 0 ]] \
  || fail "forwarded 200 disappeared after restoring access"

delete_fixture || fail "fixture cleanup failed"

echo "PASS CLI durable-logs acceptance completed ($cli_version against $api)"
