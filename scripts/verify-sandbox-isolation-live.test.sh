#!/usr/bin/env bash
# Self-test for scripts/verify-sandbox-isolation-live.sh cleanup and its stale
# fixture sweep (w9/119). Four production runs on 2026-11-15 leaked 16 API keys,
# 4 bootstrap clients and 8 workspaces: their legs failed after the bootstrap's
# 15-minute access token expired, the trap's first refused GraphQL call ran
# fail, and its exit ended the trap before the Hydra and OpenFGA deletes. Stub
# kubectl and curl stand in for the cluster, Hydra, OpenFGA and bex-api, and
# keep the Hydra clients and tuples as files, so each case asserts the end
# state: what this run created is gone, what an earlier run left is swept, and
# nothing else is touched.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="${SCRIPT:-$here/verify-sandbox-isolation-live.sh}"
tmp="$(mktemp -d)"
fails=0

stop_forwards() {
  local pid
  for pid in $(cat "$tmp"/state-*/forwards 2>/dev/null); do
    kill "$pid" 2>/dev/null || true
  done
}
trap 'stop_forwards; rm -rf "$tmp"' EXIT

mkdir -p "$tmp/bin"
cat >"$tmp/bin/kubectl" <<'STUB'
#!/usr/bin/env bash
# Stub kubectl: the reads the wrapper makes, port-forwards that block until
# killed (their pids go to $STATE/forwards), and psql through exec, which logs
# its variables and SQL and answers the stale-workspace query from a file.
args=" $* "
case "$args" in
  *" port-forward "*)
    for a in "$@"; do last="$a"; done
    remote="${last##*:}"
    local_port="${last%%:*}"
    [ -n "$local_port" ] || local_port=$((20000 + RANDOM % 10000))
    echo "Forwarding from 127.0.0.1:$local_port -> $remote"
    echo "$$" >>"$STATE/forwards"
    exec sleep 600
    ;;
  *" get secret bex-openfga -o "*) printf 'fga-token' | base64 ;;
  *" get secret bex-control-plane -o "*) printf 'cp-token' | base64 ;;
  *" get clusters.postgresql.cnpg.io bex-db "*) printf 'bex-db-1' ;;
  *" exec -i "*)
    sql="$(cat)"
    { printf -- '--- %s\n' "$*"; printf '%s\n' "$sql"; } >>"$STATE/sql.log"
    case "$sql" in
      *"FROM tenants t"*) cat "$STATE/stale-workspaces.txt" 2>/dev/null ;;
    esac
    ;;
esac
exit 0
STUB

cat >"$tmp/bin/curl" <<'STUB'
#!/usr/bin/env bash
# Stub curl. Hydra clients live in $STATE/clients.jsonl and OpenFGA tuples in
# $STATE/tuples.jsonl; tokens are tok-N, and a token listed in $STATE/expired is
# refused by bex-api with 401, as Hydra's introspection refuses an expired one.
method=GET out="" wfmt="" data="" config="" headers="" fail=false url=""
while [ $# -gt 0 ]; do
  case "$1" in
    --config | -K) config="$2"; shift ;;
    --request | -X) method="$2"; shift ;;
    --output | -o) out="$2"; shift ;;
    --write-out | -w) wfmt="$2"; shift ;;
    --data-binary | --data) data="$2"; shift ;;
    --dump-header | -D) headers="$2"; shift ;;
    --header | -H | --connect-timeout | --max-time) shift ;;
    --fail | -fsS | -f) fail=true ;;
    http*) url="$1" ;;
  esac
  shift
done
case "$data" in
  @-) data="$(cat)" ;;
  @*) data="$(cat "${data#@}")" ;;
esac
[ "$method" = GET ] && [ -n "$data" ] && method=POST
token=""
[ -z "$config" ] || token="$(sed -n 's/.*Bearer \([^"]*\)".*/\1/p' "$config")"
path="/${url#http*://*/}"
echo "$method $path $token" >>"$STATE/calls.log"
expired() { [ -n "$token" ] && grep -qx "$token" "$STATE/expired" 2>/dev/null; }

code=200 body='{}' link=""
case "$method $path" in
  "GET /.well-known/openid-configuration" | "GET /health/ready") ;;
  "GET /stores?page_size=100") body='{"stores":[{"id":"store-1","name":"bex"}]}' ;;
  "POST /stores/store-1/write")
    echo "$data" | jq -c '.writes.tuple_keys[]?' >>"$STATE/tuples.jsonl"
    echo "$data" | jq -c '.deletes.tuple_keys[]?' | while read -r key; do
      jq -c --argjson k "$key" 'select(. != $k)' "$STATE/tuples.jsonl" >"$STATE/tuples.next"
      mv "$STATE/tuples.next" "$STATE/tuples.jsonl"
    done
    ;;
  "POST /stores/store-1/read")
    body="$(jq -c --slurpfile all "$STATE/tuples.jsonl" '
      .tuple_key as $q
      | {tuples: [$all[] | select(.user == $q.user and (.object | startswith($q.object))) | {key: .}],
         continuation_token: ""}' <<<"$data")"
    ;;
  "POST /admin/clients")
    code=201
    echo "$data" | jq -c --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      '{client_id, client_name, created_at: $at}' >>"$STATE/clients.jsonl"
    ;;
  "GET /admin/clients?page_size=500"*)
    if [ -e "$STATE/hydra-list-fails" ]; then
      code=500
    else
      # Two clients a page, linked the way Hydra v26 does (relative, first then next).
      page="${path##*page_token=p}"
      case "$path" in *page_token=*) ;; *) page=1 ;; esac
      body="$(jq -sc --argjson p "$page" '.[(($p - 1) * 2):($p * 2)]' "$STATE/clients.jsonl")"
      link="Link: </admin/clients?page_size=500&page_token=p1>; rel=\"first\""
      if [ "$(jq -s length "$STATE/clients.jsonl")" -gt $((page * 2)) ]; then
        link="$link,</admin/clients?page_size=500&page_token=p$((page + 1))>; rel=\"next\""
      fi
    fi
    ;;
  "DELETE /admin/clients/"*)
    id="${path#/admin/clients/}"
    if jq -e --arg id "$id" 'select(.client_id == $id)' "$STATE/clients.jsonl" >/dev/null; then
      code=204
      jq -c --arg id "$id" 'select(.client_id != $id)' "$STATE/clients.jsonl" >"$STATE/clients.next"
      mv "$STATE/clients.next" "$STATE/clients.jsonl"
    else
      code=404
    fi
    echo "$id" >>"$STATE/hydra-deleted"
    ;;
  "POST /oauth2/token")
    n=$(($(wc -l <"$STATE/tokens" 2>/dev/null || echo 0) + 1))
    client="$(sed -n 's/.*client_id=\([^&]*\).*/\1/p' <<<"$data")"
    echo "tok-$n $client" >>"$STATE/tokens"
    body="{\"access_token\":\"tok-$n\"}"
    ;;
  "GET /") code=404 ;;
  "POST /v1/tenants")
    n=$(($(cat "$STATE/tenant-count" 2>/dev/null || echo 0) + 1))
    echo "$n" >"$STATE/tenant-count"
    code=201 body="{\"id\":\"tea-0000000000000000000$n\"}"
    ;;
  "PATCH /v1/tenants/"*) ;;
  "DELETE /v1/api-keys/"*) if expired; then code=401; else code=204; fi ;;
  "POST /graphql")
    if [ -e "$STATE/api-down" ]; then
      code=503 body='upstream unavailable'
    elif expired; then
      code=401 body='{"errors":[{"message":"unauthorized"}]}'
    else
      echo "$data" | jq -c '.variables' >>"$STATE/deleted-workspaces"
      body="$(echo "$data" | jq -c '{data:{deleteWorkspace:.variables.id}}')"
    fi
    ;;
esac
[ -z "$headers" ] || printf 'HTTP/1.1 %s\r\n%s\r\n\r\n' "$code" "$link" >"$headers"
if [ "$fail" = true ] && [ "$code" -ge 400 ]; then
  echo "curl: (22) The requested URL returned error: $code" >&2
  exit 22
fi
if [ -z "$out" ] || [ "$out" = - ]; then printf '%s' "$body"; else printf '%s' "$body" >"$out"; fi
[ -z "$wfmt" ] || printf '%s' "$code"
exit 0
STUB
# The wrapper's fixture directory goes into the case's state, so a case can see
# whether its credential files survived (BSD mktemp -d ignores TMPDIR).
cat >"$tmp/bin/mktemp" <<STUB
#!/usr/bin/env bash
if [ "\$*" = -d ]; then
  dir="\$("$(command -v mktemp)" -d "\$STATE/fixture.XXXXXX")" || exit
  echo "\$dir" >"\$STATE/fixture-dir"
  echo "\$dir"
  exit 0
fi
exec "$(command -v mktemp)" "\$@"
STUB
chmod +x "$tmp/bin/kubectl" "$tmp/bin/curl" "$tmp/bin/mktemp"

# The probe stands in for a leg: "expire" marks every token issued so far as
# expired before failing, the way a leg that outlives 15 minutes leaves them.
cat >"$tmp/probe.sh" <<'PROBE'
#!/usr/bin/env bash
case "${PROBE_MODE:-pass}" in
  expire) cut -d' ' -f1 "$STATE/tokens" >>"$STATE/expired"; echo "FAIL: syscall burst 1 did not complete" >&2; exit 1 ;;
  api-down) touch "$STATE/api-down"; exit 1 ;;
  *) exit 0 ;;
esac
PROBE

iso_ago() { jq -nr --argjson s "$1" 'now - $s | floor | todate'; }

# new_state <name> — an empty cluster for one case.
new_state() {
  local state="$tmp/state-$1"
  mkdir -p "$state"
  : >"$state/clients.jsonl"
  : >"$state/tuples.jsonl"
  echo "$state"
}

# run_wrapper <state> VAR=value… — runs the wrapper against the stubs; prints its exit code.
run_wrapper() {
  local state="$1" rc=0
  shift
  env PATH="$tmp/bin:$PATH" STATE="$state" BEX_LIVE_VERIFY=1 \
    BEX_API_URL=https://api.test KUBECONFIG=/dev/null BEX_VERIFY_PROBE="$tmp/probe.sh" "$@" \
    bash "$SCRIPT" >"$state/out.txt" 2>&1 || rc=$?
  echo "$rc"
}

check() {
  local label="$1"
  shift
  if "$@"; then
    echo "ok   $label"
  else
    echo "FAIL $label"
    fails=$((fails + 1))
  fi
}

# --- 1. A leg that outlives the bootstrap's token still leaves nothing behind.
s="$(new_state expired)"
rc="$(run_wrapper "$s" PROBE_MODE=expire)"
check "expired token: the leg's failure is the exit code (1)" [ "$rc" = 1 ]
check "expired token: cleanup mints a fresh bootstrap token" \
  [ "$(grep -c ' m35-bootstrap-' "$s/tokens")" = 2 ]
check "expired token: both workspaces are deleted through deleteWorkspace" \
  [ "$(jq -s 'map(.id) | sort == ["tea-00000000000000000001","tea-00000000000000000002"]' "$s/deleted-workspaces" 2>/dev/null)" = true ]
check "expired token: every Hydra client the run minted is deleted" \
  [ "$(jq -s length "$s/clients.jsonl")" = 0 ]
check "expired token: no tuple is left naming an m35 principal" \
  [ "$(jq -s 'map(select(.user | startswith("user:m35-"))) | length' "$s/tuples.jsonl")" = 0 ]
check "expired token: the port-forwards are stopped" \
  bash -c 'for p in $(cat "$1/forwards"); do ! kill -0 "$p" 2>/dev/null || exit 1; done' _ "$s"
check "expired token: no credential file is left on disk" \
  bash -c '[ -s "$1/fixture-dir" ] && [ -z "$(find "$(cat "$1/fixture-dir")" -type f 2>/dev/null)" ]' _ "$s"

# --- 2. The sweep removes an earlier run's leftovers and nothing else.
s="$(new_state sweep)"
old="$(iso_ago 172800)" young="$(iso_ago 60)"
cat >"$s/clients.jsonl" <<EOF
{"client_id":"m35-0123456789abcdef0123","client_name":"m35-admin-a-20261115073618-2374","created_at":"$old"}
{"client_id":"m35-bootstrap-20261115073618-2374","client_name":"m35 disposable verifier","created_at":"$old"}
{"client_id":"m35-fedcba9876543210fedc","client_name":"m35-owner-a-$(date -u +%Y%m%d%H%M%S)-1","created_at":"$young"}
{"client_id":"5f0c2e8e-0d7e-4b1a-9a43-3c2f6f0e9b11","client_name":"terraform","created_at":"$old"}
{"client_id":"m35-imposter","client_name":"someone else's client","created_at":"$old"}
EOF
cat >"$s/tuples.jsonl" <<'EOF'
{"user":"user:m35-0123456789abcdef0123","relation":"admin","object":"workspace:tea-dbsm1uuoou4c73fdrj6g"}
{"user":"user:m35-bootstrap-20261115073618-2374","relation":"admin","object":"workspace:default"}
{"user":"user:m35-fedcba9876543210fedc","relation":"developer","object":"workspace:tea-dbsn2pmoou4c73fdrlo0"}
{"user":"user:5f0c2e8e-0d7e-4b1a-9a43-3c2f6f0e9b11","relation":"admin","object":"workspace:tea-dbsm1uuoou4c73fdrj6g"}
{"user":"user:m35-imposter","relation":"viewer","object":"org:org-1"}
EOF
echo "tea-dbsm1uuoou4c73fdrj6g m35-a-1115073618-2374" >"$s/stale-workspaces.txt"
rc="$(run_wrapper "$s" PROBE_MODE=pass)"
check "sweep: the run passes (0)" [ "$rc" = 0 ]
check "sweep: the stale workspace is deleted with its confirmation phrase" \
  grep -qx '{"id":"tea-dbsm1uuoou4c73fdrj6g","confirmation":"sudo delete workspace m35-a-1115073618-2374"}' "$s/deleted-workspaces"
bootstrap="$(sed -n 's/^tok-1 //p' "$s/tokens")"
check "sweep: this run's bootstrap is seated as admin of the stale workspace first" \
  grep -q "client=$bootstrap .*tenant=tea-dbsm1uuoou4c73fdrj6g .*role=admin" "$s/sql.log"
check "sweep: the stale workspace query is bounded by the default age (21600 s)" \
  grep -q -- "-v stale=21600" "$s/sql.log"
check "sweep: the stale key and bootstrap are deleted from Hydra" \
  [ "$(grep -c -e '^m35-0123456789abcdef0123$' -e '^m35-bootstrap-20261115073618-2374$' "$s/hydra-deleted")" = 2 ]
check "sweep: their tuples, workspace:default included, are deleted" \
  [ "$(jq -s 'map(select(.user == "user:m35-0123456789abcdef0123" or .user == "user:m35-bootstrap-20261115073618-2374")) | length' "$s/tuples.jsonl")" = 0 ]
check "sweep: a young m35 client (a concurrent run's) survives with its tuple" \
  bash -c 'grep -q m35-fedcba9876543210fedc "$1/clients.jsonl" && grep -q m35-fedcba9876543210fedc "$1/tuples.jsonl"' _ "$s"
check "sweep: an old client that is not a fixture survives with its tuple" \
  bash -c 'grep -q 5f0c2e8e "$1/clients.jsonl" && grep -q 5f0c2e8e "$1/tuples.jsonl"' _ "$s"
check "sweep: an m35- id without the fixture's name survives" \
  bash -c 'grep -q m35-imposter "$1/clients.jsonl" && grep -q m35-imposter "$1/tuples.jsonl"' _ "$s"
check "sweep: the run's own fixtures are gone too" \
  [ "$(jq -s 'map(.client_id) | sort == ["5f0c2e8e-0d7e-4b1a-9a43-3c2f6f0e9b11","m35-fedcba9876543210fedc","m35-imposter"]' "$s/clients.jsonl")" = true ]
# Six clients at the sweep (these five and this run's bootstrap) are three stub pages.
check "sweep: every Hydra page is read once, following rel=next past rel=first" \
  [ "$(grep '^GET /admin/clients?page_size=500' "$s/calls.log" | sort -u | wc -l | tr -d ' ')" = 3 ]

# --- 3. A sweep that cannot list Hydra warns and the run goes on.
s="$(new_state sweep-down)"
touch "$s/hydra-list-fails"
rc="$(run_wrapper "$s" PROBE_MODE=pass)"
check "sweep down: the run still passes (0)" [ "$rc" = 0 ]
check "sweep down: the failure is reported" grep -q 'WARNING: listing Hydra clients failed' "$s/out.txt"
check "sweep down: the run's own fixtures are still removed" [ "$(jq -s length "$s/clients.jsonl")" = 0 ]

# --- 4. A workspace delete bex-api refuses does not end the trap: the Hydra
# clients, tuples and forwards are still removed, and the leftover is reported.
s="$(new_state api-down)"
rc="$(run_wrapper "$s" PROBE_MODE=api-down)"
check "api down: the leg's failure is the exit code (1)" [ "$rc" = 1 ]
check "api down: each workspace left behind is reported" \
  [ "$(grep -c 'WARNING: deleting fixture workspace tea-' "$s/out.txt")" = 2 ]
check "api down: every Hydra client the run minted is still deleted" \
  [ "$(jq -s length "$s/clients.jsonl")" = 0 ]
check "api down: no tuple is left naming an m35 principal" \
  [ "$(jq -s 'map(select(.user | startswith("user:m35-"))) | length' "$s/tuples.jsonl")" = 0 ]
check "api down: the port-forwards are stopped" \
  bash -c 'for p in $(cat "$1/forwards"); do ! kill -0 "$p" 2>/dev/null || exit 1; done' _ "$s"

if [ "$fails" -gt 0 ]; then
  for s in "$tmp"/state-*; do
    echo "---- $(basename "$s") output ----"
    cat "$s/out.txt"
  done
  echo "$fails check(s) failed"
  exit 1
fi
echo "all checks passed"
