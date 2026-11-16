#!/usr/bin/env bash
# Shared helpers for the production measurement legs (scripts/measure-sandbox-*.sh),
# which sandbox-measure.yml runs through scripts/verify-sandbox-isolation-live.sh
# (BEX_VERIFY_PROBE): disposable, billing-excluded workspace A with one owner
# token, the app-cluster admin kubeconfig, and nothing else. Source this file;
# it installs an EXIT trap that terminates every sandbox a leg registered with
# measure_register and removes its scratch directory.
#
# Required environment (the live wrapper sets all of it):
#   BEX_API_URL BEX_WORKSPACE_A BEX_WS_A_OWNER_TOKEN KUBECONFIG
#
# Every measurement is printed as a `MEASURE <name> <json>` line, so a run's log
# is the evidence and nothing needs to be committed (DO_NOT_DO #NO-ARTIFACTS-IN-GIT).

set -Eeuo pipefail
# A leg that stops on an unhandled error says where, so a failed run is diagnosable.
trap 'echo "ERR line $LINENO of ${BASH_SOURCE[1]:-?}: $BASH_COMMAND" >&2' ERR

: "${BEX_API_URL:?set BEX_API_URL}"
: "${BEX_WORKSPACE_A:?set BEX_WORKSPACE_A}"
: "${BEX_WS_A_OWNER_TOKEN:?set BEX_WS_A_OWNER_TOKEN}"
: "${KUBECONFIG:?set KUBECONFIG}"

for command in curl jq kubectl; do
  command -v "$command" >/dev/null || {
    echo "error: missing required command: $command" >&2
    exit 2
  }
done

api_url="${BEX_API_URL%/}"
namespace_a="${BEX_WORKSPACE_A}-sandbox"
fixture_dir="$(mktemp -d)"
umask 077
measure_ids=()
measure_pf_pids=()

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
pass() { echo "PASS: $*"; }

# check <name> <true|false> <detail>: a non-fatal assertion; a leg ends with
# [ "$failures" = 0 ] || fail "...".
failures=0
check() {
  if [ "$2" = true ]; then pass "$1"; else echo "FAIL: $1 ($3)"; failures=$((failures + 1)); fi
}

# measure <name> <json>: one evidence line.
measure() { printf 'MEASURE %s %s\n' "$1" "$2"; }

measure_cleanup() {
  set +e
  local pid id
  for pid in "${measure_pf_pids[@]+"${measure_pf_pids[@]}"}"; do kill "$pid" 2>/dev/null; done
  for id in "${measure_ids[@]+"${measure_ids[@]}"}"; do
    api_call "$fixture_dir/owner.curl" POST "/v1/sandboxes/$id/terminate?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/cleanup-$id.json" >/dev/null 2>&1
  done
  # A terminated sandbox's image is reaped on the next meter poll rounds; leave
  # them time before the live wrapper deletes the workspace.
  [ "${#measure_ids[@]}" -gt 0 ] && sleep "${BEX_MEASURE_REAP_WAIT:-90}"
  rm -rf "$fixture_dir"
}
trap measure_cleanup EXIT

printf 'header = "Authorization: Bearer %s"\n' "$BEX_WS_A_OWNER_TOKEN" >"$fixture_dir/owner.curl"

# refresh_owner_token: the disposable owner's access token lives about 15
# minutes, and the storage leg waits longer than that; mint a new one from the
# client credentials the live wrapper passes (through its Hydra port-forward).
refresh_owner_token() {
  [ -n "${BEX_HYDRA_PUBLIC_URL:-}" ] && [ -n "${BEX_WS_A_OWNER_CLIENT_ID:-}" ] || return 1
  local out="$fixture_dir/token-refresh.json" code token
  code="$(curl --silent --show-error --output "$out" --write-out '%{http_code}' \
    --header 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "grant_type=client_credentials" --data-urlencode "client_id=$BEX_WS_A_OWNER_CLIENT_ID" \
    --data-urlencode "client_secret=$BEX_WS_A_OWNER_CLIENT_SECRET" "$BEX_HYDRA_PUBLIC_URL/oauth2/token")"
  [ "$code" = 200 ] || return 1
  token="$(jq -er '.access_token' "$out")" || return 1
  printf 'header = "Authorization: Bearer %s"\n' "$token" >"$fixture_dir/owner.curl"
}

# api_call <auth file> <method> <path> <output> [curl args]: prints the status. A
# 401 on the owner's own credentials refreshes the token once and repeats the call.
api_call() {
  local auth_file="$1" method="$2" path="$3" output="$4" code
  shift 4
  code="$(curl --config "$auth_file" --silent --show-error \
    --request "$method" --output "$output" --write-out '%{http_code}' \
    "$api_url$path" "$@")"
  if [ "$code" = 401 ] && [ "$auth_file" = "$fixture_dir/owner.curl" ] && refresh_owner_token; then
    code="$(curl --config "$auth_file" --silent --show-error \
      --request "$method" --output "$output" --write-out '%{http_code}' \
      "$api_url$path" "$@")"
  fi
  printf '%s' "$code"
}

expect_status() {
  local want="$1" method="$2" path="$3" output="$4" got
  shift 4
  got="$(api_call "$fixture_dir/owner.curl" "$method" "$path" "$output" "$@")"
  [ "$got" = "$want" ] || fail "$method $path returned $got, want $want: $(head -c 300 "$output" 2>/dev/null)"
}

# create_sandbox <output> <networkPolicy JSON> [extra JSON fields]: prints the
# new sandbox's id and registers it for termination on exit.
create_sandbox() {
  local output="$1" policy="$2" extra="${3:-}" id
  expect_status 201 POST "/v1/sandboxes" "$output" \
    --header 'Content-Type: application/json' \
    --data "{\"ownerId\":\"$BEX_WORKSPACE_A\",\"networkPolicy\":$policy${extra:+,$extra}}"
  id="$(jq -er '.id' "$output")"
  measure_ids+=("$id")
  printf '%s\n' "$id"
}

# sandbox_status <id>: the sandbox's status, or "unknown" when the API still
# does not answer after a minute and a half: a bex-api rollout drops calls for
# about that long, and a leg that runs ten minutes can overlap one.
sandbox_status() {
  local out="$fixture_dir/status-$1.json" try
  for try in 1 2 3 4 5 6 7 8 9; do
    if [ "$(api_call "$fixture_dir/owner.curl" GET "/v1/sandboxes/$1?ownerId=$BEX_WORKSPACE_A" "$out")" = 200 ]; then
      jq -r '.status // "unknown"' "$out"
      return
    fi
    sleep 10
  done
  echo unknown
}

# wait_status <id> <status> <seconds>
wait_status() {
  local id="$1" want="$2" deadline=$((SECONDS + $3)) got=""
  while ((SECONDS < deadline)); do
    got="$(sandbox_status "$id")"
    [ "$got" = "$want" ] && return 0
    sleep 5
  done
  fail "sandbox $id is $got after $3 s, want $want"
}

# sandbox_exec <id> <command> <output>: run one command through bex's public
# exec path (mint a run token, redeem it for the SSE stream) and write its
# stdout to <output>. Returns non-zero, with the SSE body kept, when the stream
# fails or the command's exit code is not 0.
sandbox_exec() {
  local id="$1" command="$2" output="$3" body token_file
  body="$(jq -cn --arg c "$command" '{command: $c}')"
  [ "$(api_call "$fixture_dir/owner.curl" POST "/v1/sandboxes/$id/runs/stream/token?ownerId=$BEX_WORKSPACE_A" "$output.mint" \
    --header 'Content-Type: application/json' --data "$body")" = 201 ] || return 1
  token_file="$output.token.curl"
  printf 'header = "Authorization: Bearer %s"\n' "$(jq -er '.token' "$output.mint")" >"$token_file"
  curl --config "$token_file" --silent --show-error --max-time "${BEX_EXEC_MAX_TIME:-300}" \
    --request POST --header 'Accept: text/event-stream' --header 'Content-Type: application/json' \
    --data "$body" --output "$output.sse" "$(jq -er '.uri' "$output.mint")" || return 1
  awk '/^event: /{ev=substr($0, 8)} /^data: /{if (ev == "output") print substr($0, 7)}' "$output.sse" \
    | jq -rj 'select(.stream != "stderr") | .data' >"$output" || return 1
  grep -q '"exit_code":0' "$output.sse"
}

# find_pod <id>: the substrate pod of a sandbox, "<pod> <node> <cr>". The
# BatchSandbox carries the public id as the bex.co/sandbox-id label and its pod
# is <cr>-0 (ADR042 D4; the pinned controller names a non-pooled sandbox's pod
# <batchsandbox>-<index>).
find_pod() {
  local id="$1" deadline=$((SECONDS + 300)) cr="" pod=""
  while ((SECONDS < deadline)); do
    cr="$(kubectl -n "$namespace_a" get batchsandboxes -l "bex.co/sandbox-id=$id" -o json 2>/dev/null | jq -r '.items[0].metadata.name // empty')"
    if [ -n "$cr" ]; then
      pod="$(kubectl -n "$namespace_a" get pods -o json | jq -r --arg cr "$cr" '
        [.items[] | select(any(.metadata.ownerReferences[]?; .kind == "BatchSandbox" and .name == $cr)
                           and any(.status.conditions[]?; .type == "Ready" and .status == "True"))]
        | sort_by(.metadata.creationTimestamp) | last | .metadata.name // empty')"
      if [ -n "$pod" ]; then
        printf '%s %s %s\n' "$pod" "$(kubectl -n "$namespace_a" get pod "$pod" -o jsonpath='{.spec.nodeName}')" "$cr"
        return 0
      fi
    fi
    sleep 5
  done
  fail "sandbox $id has no Ready pod after 5 minutes (cr='$cr')"
}

# node_summary <node> <output>: the kubelet's stats summary for the node.
node_summary() { kubectl get --raw "/api/v1/nodes/$1/proxy/stats/summary" >"$2"; }

# node_cadvisor <node> <output>: the node's raw cAdvisor exposition.
node_cadvisor() { kubectl get --raw "/api/v1/nodes/$1/proxy/metrics/cadvisor" >"$2"; }

# prom_forward: start a port-forward to Prometheus and set $prom_url ("" when
# it did not start). The apiserver service proxy times out for the monitoring
# namespace (Cilium remote-node denial; scripts/alerting-heartbeat.sh), so use
# a port-forward as that script does.
prom_url=""
prom_forward() {
  local log="$fixture_dir/prom-pf.log" i port
  kubectl -n monitoring port-forward svc/prometheus-server :80 --request-timeout=20s >"$log" 2>&1 &
  measure_pf_pids+=("$!")
  for i in $(seq 1 40); do
    port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9]*\) .*/\1/p' "$log" 2>/dev/null | head -1 || true)" # a poll: a failed read is tried again
    [ -n "$port" ] && { prom_url="http://127.0.0.1:$port"; return 0; }
    sleep 0.5
  done
  prom_url=""
}

# prom_query <expr>: the instant-query result array as compact JSON ("[]" on any failure).
prom_query() {
  [ -n "$prom_url" ] || { echo '[]'; return; }
  curl -sS --fail --max-time 20 --get --data-urlencode "query=$1" "$prom_url/api/v1/query" 2>/dev/null \
    | jq -c '.data.result // []' 2>/dev/null || echo '[]'
}

# db_query [psql args]: a read-only query as the bex-db superuser, SQL on stdin
# (psql does not interpolate -v variables into -c strings). Prints unaligned,
# tuple-only rows with | between columns. Only SELECTs belong here.
db_primary=""
db_query() {
  [ -n "$db_primary" ] || db_primary="$(kubectl -n bex-system get clusters.postgresql.cnpg.io bex-db -o jsonpath='{.status.currentPrimary}')"
  [ -n "$db_primary" ] || fail "bex-db has no primary"
  kubectl -n bex-system exec -i "$db_primary" -c postgres -- \
    psql -X -qAt -F '|' -v ON_ERROR_STOP=1 -U postgres -d bex "$@"
}

# zot_forward: port-forward to the registry and set $zot_url, with a curl config
# in $fixture_dir/zot.curl holding the read-only meter user (never printed).
zot_url=""
zot_forward() {
  local log="$fixture_dir/zot-pf.log" i port user pass
  user="$(kubectl -n bex-registry get secret bex-sandbox-registry -o jsonpath='{.data.reader-username}' | base64 -d)"
  pass="$(kubectl -n bex-registry get secret bex-sandbox-registry -o jsonpath='{.data.reader-password}' | base64 -d)"
  [ -n "$user" ] && [ -n "$pass" ] || fail "the Secret bex-registry/bex-sandbox-registry has no reader credentials"
  printf 'user = "%s:%s"\n' "$user" "$pass" >"$fixture_dir/zot.curl"
  kubectl -n bex-registry port-forward svc/zot :5000 --request-timeout=20s >"$log" 2>&1 &
  measure_pf_pids+=("$!")
  for i in $(seq 1 40); do
    port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9]*\) .*/\1/p' "$log" 2>/dev/null | head -1 || true)" # a poll: a failed read is tried again
    [ -n "$port" ] && { zot_url="http://127.0.0.1:$port"; return 0; }
    sleep 0.5
  done
  fail "port-forward to the registry did not start"
}

# zot_get <path> <output>: GET /v2/... with the reader; prints the HTTP status.
zot_get() {
  curl --config "$fixture_dir/zot.curl" --silent --show-error --max-time 30 --output "$2" --write-out '%{http_code}' \
    --header 'Accept: application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json' \
    "$zot_url$1"
}

# zot_repo_tags <repo>: the repository's tags as a sorted JSON array, [] when it has none or is gone.
zot_repo_tags() {
  local out="$fixture_dir/zot-tags.json"
  if [ "$(zot_get "/v2/$1/tags/list" "$out")" = 200 ]; then jq -c '(.tags // []) | sort' "$out"; else echo '[]'; fi
}

# zot_newest_image_bytes <repo>: "<tag> <bytes>" for the highest snap-gen<N> tag,
# sized as bex-api sizes it (config + the layers of the OCI manifest), or "none".
zot_newest_image_bytes() {
  local repo="$1" tag manifest="$fixture_dir/zot-manifest.json"
  tag="$(zot_repo_tags "$repo" | jq -r '[.[] | select(test("^snap-gen[0-9]+$")) | ltrimstr("snap-gen") | tonumber] | if length == 0 then "" else "snap-gen\(max)" end')"
  [ -n "$tag" ] || { echo none; return; }
  [ "$(zot_get "/v2/$repo/manifests/$tag" "$manifest")" = 200 ] || { echo "$tag unreadable"; return; }
  printf '%s %s\n' "$tag" "$(jq -r '(.config.size // 0) + ([.layers[]?.size] | add // 0)' "$manifest")"
}

# zot_sandbox_repos: this run's workspace's snapshot repositories, one per line.
zot_sandbox_repos() {
  local out="$fixture_dir/zot-catalog.json"
  [ "$(zot_get "/v2/_catalog?n=1000" "$out")" = 200 ] || return 0
  jq -r --arg p "snapshots/${BEX_WORKSPACE_A}-sandbox/" '.repositories[]? | select(startswith($p))' "$out"
}
