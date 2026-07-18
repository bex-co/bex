#!/usr/bin/env bash
set -euo pipefail

# Headless acceptance for the official Render CLI's interactive pgcli command.
# Raw PTY bytes and sensitive API bodies never reach durable output. The default
# creates and unconditionally removes a disposable non-production Postgres.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pty_runner="$repo_root/scripts/render-cli-pty.py"
shim="$repo_root/scripts/pgcli-verify-client.sh"
tmp="$(mktemp -d)"
database_id=""
database_name=""
created=0
cleanup_complete=0

fail() {
  echo "FAIL pgcli-compat $*" >&2
  exit 1
}

cleanup() {
  local rc="${1:-$?}"
  trap - EXIT INT TERM
  if [[ "$created" == "1" && "$database_id" == dpg-* ]]; then
    if api_request DELETE "/postgres/$database_id" >/dev/null 2>&1; then
      local deadline=$((SECONDS + ${BEX_PGCLI_CLEANUP_TIMEOUT_SECONDS:-90})) status
      while ((SECONDS < deadline)); do
        status="$(api_status GET "/postgres/$database_id" 2>/dev/null || true)"
        if [[ "$status" == "404" ]]; then
          cleanup_complete=1
          break
        fi
        sleep 1
      done
    fi
    if [[ "$cleanup_complete" == "1" ]]; then
      echo "PASS pgcli cleanup disposable-database"
    else
      echo "FAIL pgcli cleanup disposable-database" >&2
      rc=1
    fi
  elif [[ -n "$database_id" ]]; then
    echo "PASS pgcli cleanup existing-disposable-target-unchanged"
  else
    echo "PASS pgcli cleanup no-database-fixture"
  fi
  rm -r "$tmp" 2>/dev/null || true
  if [[ ! -e "$tmp" ]]; then
    echo "PASS pgcli cleanup local-artifacts"
  else
    echo "FAIL pgcli cleanup local-artifacts" >&2
    rc=1
  fi
  exit "$rc"
}
trap 'cleanup $?' EXIT
trap 'cleanup 130' INT
trap 'cleanup 143' TERM

: "${RENDER_HOST:?set the non-production bex REST base ending in /v1/}"
: "${RENDER_API_KEY:?set a short-lived bearer token without echoing it}"
: "${RENDER_BIN:?set the pinned unmodified Render CLI executable}"
: "${BEX_PGCLI_TARGET_CLASS:?name the non-production target class}"
[[ "${BEX_PGCLI_NON_PRODUCTION:-0}" == "1" ]] || fail "BEX_PGCLI_NON_PRODUCTION=1 is required"
[[ -x "$RENDER_BIN" ]] || fail "pinned Render CLI is missing or not executable"
[[ -x "$pty_runner" && -x "$shim" ]] || fail "PTY runner or pgcli shim is not executable"
for required_command in curl jq python3; do
  command -v "$required_command" >/dev/null || fail "missing required client binary: $required_command"
done

api="${RENDER_HOST%/}"
if ! api_host="$(python3 - "$api" <<'PY'
import sys
import urllib.parse

parsed = urllib.parse.urlsplit(sys.argv[1])
try:
    port = parsed.port
except ValueError:
    raise SystemExit(1)
if parsed.scheme not in {"http", "https"} or not parsed.hostname or parsed.path != "/v1":
    raise SystemExit(1)
if parsed.username is not None or parsed.password is not None or parsed.query or parsed.fragment:
    raise SystemExit(1)
if any(ord(character) < 33 or ord(character) == 127 for character in sys.argv[1]):
    raise SystemExit(1)
print(parsed.hostname.lower())
PY
)"; then
  fail "RENDER_HOST must be a credential-free HTTP(S) base ending exactly in /v1/"
fi
case "$api_host" in
  api.bex.co | api.render.com)
    fail "production APIs are forbidden"
    ;;
esac
api_host=""
curl_timeout="${BEX_PGCLI_API_TIMEOUT_SECONDS:-30}"

api_request() {
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -fsS --connect-timeout 5 --max-time "$curl_timeout" -X "$method" -H "Authorization: Bearer $RENDER_API_KEY" \
      -H 'Content-Type: application/json' --data-binary "$body" "$api$path"
  else
    curl -fsS --connect-timeout 5 --max-time "$curl_timeout" -X "$method" \
      -H "Authorization: Bearer $RENDER_API_KEY" "$api$path"
  fi
}

api_status() {
  local method="$1" path="$2"
  curl -sS --connect-timeout 5 --max-time "$curl_timeout" -o /dev/null -w '%{http_code}' \
    -X "$method" -H "Authorization: Bearer $RENDER_API_KEY" "$api$path"
}

if ! render_version="$(RENDER_CLI_CONFIG_PATH="$tmp/version-cli.yaml" python3 - "$RENDER_BIN" <<'PY'
import re
import subprocess
import sys

try:
    result = subprocess.run([sys.argv[1], "--version"], capture_output=True, timeout=20, check=False)
except (OSError, subprocess.TimeoutExpired):
    raise SystemExit(1)
first_line = result.stdout.decode("utf-8", "replace").splitlines()[:1]
if result.returncode or first_line != ["render v2.21.0"]:
    raise SystemExit(1)
if not re.fullmatch(r"[A-Za-z0-9._ -]+", first_line[0]):
    raise SystemExit(1)
print(first_line[0])
PY
)"; then
  fail "official CLI is missing, hung, or not pinned to v2.21.0"
fi
echo "INFO pgcli target=$BEX_PGCLI_TARGET_CLASS"
echo "INFO official-cli=$render_version"

# Prove the control path is the real client guard, not a forced interactive
# flag. Its raw diagnostic stays in memory and is reduced to one named marker.
if ! RENDER_CLI_CONFIG_PATH="$tmp/non-tty-cli.yaml" python3 - "$RENDER_BIN" <<'PY'
import os
import subprocess
import sys

environment = os.environ.copy()
environment["TERM"] = "xterm-256color"
environment.pop("CI", None)
environment.pop("RENDER_OUTPUT", None)
try:
    result = subprocess.run(
        [sys.argv[1], "pgcli", "dpg-00000000000000000000"],
        stdin=subprocess.DEVNULL,
        capture_output=True,
        env=environment,
        timeout=10,
        check=False,
    )
except (OSError, subprocess.TimeoutExpired):
    raise SystemExit(1)
output = result.stdout + result.stderr
if result.returncode == 0 or b"`render pgcli` can only be used in interactive mode" not in output:
    raise SystemExit(1)
PY
then
  fail "non-TTY control missed the official interactive-only guard or deadline"
fi
echo "PASS pgcli official-non-tty-guard"

if [[ -n "${BEX_PGCLI_DATABASE_ID:-}" ]]; then
  [[ "${BEX_PGCLI_EXISTING_DISPOSABLE:-0}" == "1" ]] ||
    fail "an existing target requires BEX_PGCLI_EXISTING_DISPOSABLE=1"
  database_id="$BEX_PGCLI_DATABASE_ID"
  [[ "$database_id" =~ ^dpg-[a-z0-9]{20}([a-z0-9]{2})?$ ]] || fail "existing database id is not Render-shaped"
  target_json="$(api_request GET "/postgres/$database_id")" || fail "existing target resolution"
  database_name="$(jq -er '.name' <<<"$target_json")" || fail "existing target name"
  target_json=""
else
  database_name="${BEX_PGCLI_DATABASE_NAME:-pgcli-v-$(date +%s)-$$}"
  allow_cidr="${BEX_PGCLI_ALLOW_CIDR:-}"
  if [[ -z "$allow_cidr" ]]; then
    verifier_ip="$(curl -fsS --max-time 10 https://api.ipify.org)" || fail "public source-IP discovery"
    allow_cidr="$(python3 -c 'import ipaddress,sys; print(ipaddress.ip_network(sys.argv[1], strict=False))' \
      "${verifier_ip}/$( [[ "$verifier_ip" == *:* ]] && echo 128 || echo 32 )")" || fail "source-IP CIDR"
    verifier_ip=""
  fi
  if ! allow_list="$(python3 - "$allow_cidr" "${BEX_PGCLI_ADDITIONAL_ALLOW_CIDRS:-}" <<'PY'
import ipaddress
import json
import sys

values = [sys.argv[1], *(value.strip() for value in sys.argv[2].split(",") if value.strip())]
entries = []
for index, value in enumerate(values):
    entries.append({
        "cidrBlock": str(ipaddress.ip_network(value, strict=False)),
        "description": "pgcli compatibility verifier" if index == 0 else "pgcli verifier transport",
    })
print(json.dumps(entries, separators=(",", ":")))
PY
)"; then
    fail "invalid verifier allow-list CIDR"
  fi
  create_payload="$(jq -cn \
    --arg name "$database_name" \
    --arg owner "${RENDER_WORKSPACE:-}" \
    --argjson allow "$allow_list" \
    '{name:$name,plan:"free",public:true,ipAllowList:$allow} + if $owner == "" then {} else {ownerId:$owner} end')"
  allow_list=""
  created_json="$(api_request POST /postgres "$create_payload")" || fail "disposable database create"
  create_payload=""
  database_id="$(jq -er '.id' <<<"$created_json")" || fail "created database id"
  [[ "$database_id" =~ ^dpg-[a-z0-9]{20}([a-z0-9]{2})?$ ]] || fail "created database id is not Render-shaped"
  created=1
  database_name="$(jq -er '.name' <<<"$created_json")" || fail "created database name"
  created_json=""
  echo "PASS pgcli disposable-database-created id=$database_id"
fi

ready=0
ready_deadline=$((SECONDS + ${BEX_PGCLI_READY_TIMEOUT_SECONDS:-600}))
while ((SECONDS < ready_deadline)); do
  if target_json="$(api_request GET "/postgres/$database_id" 2>/dev/null)" &&
    [[ "$(jq -r '.status // empty' <<<"$target_json")" == "available" ]] &&
    [[ "$(jq -r '.public // false' <<<"$target_json")" == "true" ]] &&
    [[ -n "$(jq -r '.externalHost // empty' <<<"$target_json")" ]]; then
    ready=1
    break
  fi
  target_json=""
  sleep 2
done
[[ "$ready" == "1" ]] || fail "database did not become available before the deadline"
expected_host="$(jq -er '.externalHost' <<<"$target_json")" || fail "external host precondition"
expected_database="$(jq -er '.databaseName' <<<"$target_json")" || fail "database-name precondition"
resolved_id="$(jq -er '.id' <<<"$target_json")" || fail "resolved database id"
[[ "$resolved_id" == "$database_id" ]] || fail "target identity changed"
[[ "$expected_host" =~ ^[A-Za-z0-9.:-]+$ && "$expected_database" =~ ^[A-Za-z0-9_.-]+$ ]] ||
  fail "external host or database name contains unsafe output characters"
target_json=""

# This sensitive response is piped directly into a parser. Only host, database,
# and TLS mode leave the parser; the URI, username, password, and body do not.
if ! connection_facts="$(api_request GET "/postgres/$database_id/connection-info" | python3 -c '
import json
import sys
import urllib.parse

body = json.load(sys.stdin)
uri = body.get("externalConnectionString", "")
parsed = urllib.parse.urlsplit(uri)
query = urllib.parse.parse_qs(parsed.query)
database = urllib.parse.unquote(parsed.path.removeprefix("/"))
if parsed.scheme not in {"postgres", "postgresql"} or not parsed.hostname:
    raise SystemExit(1)
if not parsed.username or parsed.password is None or not database:
    raise SystemExit(1)
if query.get("sslmode") != ["require"]:
    raise SystemExit(1)
print("\t".join((parsed.hostname, database, "require")))
' 2>/dev/null)"; then
  fail "empty or malformed external connection information"
fi
IFS=$'\t' read -r connection_host connection_database connection_tls <<<"$connection_facts"
connection_facts=""
[[ "$connection_host" == "$expected_host" && "$connection_database" == "$expected_database" &&
  "$connection_tls" == "require" ]] || fail "external connection contract disagrees with the resolved target"
echo "PASS pgcli external-connection-precondition host=$expected_host database=$expected_database tls=require"

mkdir -p "$tmp/shim-bin" "$tmp/real-bin" "$tmp/xdg/config" "$tmp/xdg/data" "$tmp/xdg/cache"
ln -s "$shim" "$tmp/shim-bin/pgcli"
shim_record="$tmp/shim-record.jsonl"
: >"$shim_record"
chmod 600 "$shim_record"

run_shim() {
  local label="$1" selector="$2"
  BEX_PGCLI_EXPECT_HOST="$expected_host" \
    BEX_PGCLI_EXPECT_DATABASE="$expected_database" \
    BEX_PGCLI_EXPECT_ID="$database_id" \
    BEX_PGCLI_EXPECT_LABEL="$label" \
    BEX_PGCLI_SHIM_RECORD="$shim_record" \
    PATH="$tmp/shim-bin:$PATH" \
    python3 "$pty_runner" --timeout "${BEX_PGCLI_COMMAND_TIMEOUT_SECONDS:-60}" \
      --expect "handoff-$label=BEX_PGCLI_SHIM_OK" -- \
      "$RENDER_BIN" pgcli "$selector" -- --csv -q
}

run_negative_name() {
  local label="$1" selector="$2" marker="$3" before after output rc
  before="$(wc -l <"$shim_record" | tr -d ' ')"
  set +e
  output="$(PATH="$tmp/shim-bin:$PATH" python3 "$pty_runner" \
    --timeout "${BEX_PGCLI_COMMAND_TIMEOUT_SECONDS:-60}" \
    --expect "$label=$marker" --send-after "$label=$(printf '\003')" -- \
    "$RENDER_BIN" pgcli "$selector" -- --csv -q 2>&1)"
  rc=$?
  set -e
  # Bubble Tea treats Ctrl-C after rendering a user-facing error as a clean UI
  # shutdown, so the process may exit zero. The rejection proof is the exact
  # error marker plus the unchanged child-invocation count; timeout is never a
  # successful negative control.
  [[ "$rc" != "124" ]] || fail "$label did not fail boundedly"
  [[ "$output" == *"PASS pty marker $label"* ]] || fail "$label did not report its safe error marker"
  output=""
  after="$(wc -l <"$shim_record" | tr -d ' ')"
  [[ "$after" == "$before" ]] || fail "$label invoked pgcli"
  echo "PASS pgcli $label rejected-before-child"
}

run_shim id "$database_id"
[[ "$(wc -l <"$shim_record" | tr -d ' ')" == "1" ]] || fail "id path did not invoke the shim exactly once"
jq -e --arg id "$database_id" '.label == "id" and .id == $id and .flags == ["--csv","-q"] and .tls == "require"' \
  "$shim_record" >/dev/null || fail "id path safe handoff record"
echo "PASS pgcli id-resolution-and-shim-handoff"

unknown_name="pgcli-missing-${database_id#dpg-}"
run_negative_name unknown-name "$unknown_name" "No Postgres instance found with name or ID"

run_shim name "$database_name"
[[ "$(wc -l <"$shim_record" | tr -d ' ')" == "2" ]] || fail "name path did not invoke the shim exactly once"
jq -s -e --arg id "$database_id" '
  length == 2 and (map(.label) == ["id","name"]) and all(.[]; .id == $id and .tls == "require")
' "$shim_record" >/dev/null || fail "id/name safe handoff agreement"
echo "PASS pgcli name-resolution-and-shim-handoff"

if [[ -n "${BEX_PGCLI_AMBIGUOUS_NAME:-}" ]]; then
  run_negative_name ambiguous-name "$BEX_PGCLI_AMBIGUOUS_NAME" "Multiple Postgres instances found with name"
fi

if [[ "${BEX_PGCLI_SKIP_REAL:-0}" == "1" ]]; then
  echo "SKIP real pgcli SQL sessions (partial proof only)"
  exit 0
fi

real_pgcli="${BEX_PGCLI_REAL_BIN:-}"
if [[ -z "$real_pgcli" ]]; then
  real_pgcli="$(command -v pgcli || true)"
fi
[[ -n "$real_pgcli" && -x "$real_pgcli" ]] || fail "real pgcli is missing"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'set +e' \
  'env XDG_CONFIG_HOME="$BEX_PGCLI_REAL_XDG/config" XDG_DATA_HOME="$BEX_PGCLI_REAL_XDG/data" XDG_CACHE_HOME="$BEX_PGCLI_REAL_XDG/cache" "$BEX_PGCLI_REAL_EXEC" "$@"' \
  'client_rc=$?' \
  'set -e' \
  'printf "%s\\n" "$client_rc" >"$BEX_PGCLI_REAL_STATUS"' \
  'echo BEX_PGCLI_REAL_DONE' \
  'exit "$client_rc"' \
  >"$tmp/real-bin/pgcli"
chmod 700 "$tmp/real-bin/pgcli"
if ! pgcli_version="$(XDG_CONFIG_HOME="$tmp/xdg/config" XDG_DATA_HOME="$tmp/xdg/data" \
  XDG_CACHE_HOME="$tmp/xdg/cache" python3 - "$real_pgcli" <<'PY'
import os
import re
import subprocess
import sys

try:
    result = subprocess.run(
        [sys.argv[1], "--version"], capture_output=True, env=os.environ.copy(), timeout=15, check=False
    )
except (OSError, subprocess.TimeoutExpired):
    raise SystemExit(1)
first_line = (result.stdout + result.stderr).decode("utf-8", "replace").splitlines()[:1]
if result.returncode or not first_line or not re.fullmatch(r"[A-Za-z0-9._: -]+", first_line[0]):
    raise SystemExit(1)
print(first_line[0])
PY
)"; then
  fail "real pgcli version probe failed or hung"
fi
echo "INFO real-pgcli=$pgcli_version"

pgclirc="$tmp/pgclirc"
printf '%s\n' \
  '[main]' \
  'table_format = csv' \
  'prompt = BEX_PGCLI_READY> ' \
  'less_chatty = True' \
  'enable_pager = False' \
  'row_limit = 0' \
  'keyring = False' >"$pgclirc"
chmod 600 "$pgclirc"

run_real() {
  local label="$1" selector="$2" query quit status_file runner_rc client_rc
  query=$'SELECT 1 AS bex_pgcli_probe;\n'
  quit=$'\\q\n'
  status_file="$tmp/real-$label.status"
  set +e
  PATH="$tmp/real-bin:$PATH" PGCLIRC="$pgclirc" \
    BEX_PGCLI_REAL_EXEC="$real_pgcli" BEX_PGCLI_REAL_XDG="$tmp/xdg" BEX_PGCLI_REAL_STATUS="$status_file" \
    python3 "$pty_runner" --timeout "${BEX_PGCLI_REAL_TIMEOUT_SECONDS:-90}" --exit-timeout 3 \
      --expect "$label-ready=BEX_PGCLI_READY>" \
      --send-after "$label-ready=$query" \
      --expect "$label-column=bex_pgcli_probe" \
      --expect "$label-value=\"1\"" \
      --expect "$label-status=SELECT 1" \
      --send-after "$label-status=$quit" \
      --expect "$label-client-done=BEX_PGCLI_REAL_DONE" -- \
      "$RENDER_BIN" pgcli "$selector" -- --less-chatty --prompt 'BEX_PGCLI_READY> '
  runner_rc=$?
  set -e
  [[ -s "$status_file" ]] || fail "real pgcli $label did not record a child exit"
  client_rc="$(<"$status_file")"
  [[ "$client_rc" =~ ^[0-9]+$ && "$client_rc" -le 255 ]] || fail "real pgcli $label recorded an invalid child exit"
  if [[ "$client_rc" != "0" ]]; then
    echo "FAIL pgcli real-client-$label status=$client_rc" >&2
    return "$client_rc"
  fi
  [[ "$runner_rc" == "0" ]] || fail "official CLI did not exit cleanly after real pgcli $label"
  echo "PASS pgcli real-sql-$label"
}

run_real id "$database_id"
run_real name "$database_name"
echo "PASS pgcli compatibility verification complete"
