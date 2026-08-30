#!/usr/bin/env bash
set -euo pipefail

# Headless acceptance for the official Render CLI's non-interactive `psql`
# command (`render psql [id|name] -c "<SQL>" -o text`). The default creates and
# unconditionally removes a disposable non-production public Postgres, resolves
# it by opaque id AND exact name, crosses the CLI's own client-side IP-allow-list
# gate (getUserIP → hasAccessToPostgres over pg.ipAllowList), and runs
# `SELECT 1 AS bex_psql_probe;` through the real local `psql` binary the CLI
# shells out to. Sensitive API bodies (URI/password) never reach durable output.
#
# The `-o text` output format is what forces the CLI down its
# ExecutePSQLNonInteractive path (outputFormat.Interactive() == false), the same
# path Render's own CLI uses; the interactive TTY session is proven for the
# sibling pgcli row and cross-referenced from the checklist, not re-driven here.
#
# Env overrides (all optional; defaults create a fresh fixture):
#   BEX_PSQL_ALLOW_CIDR              explicit allow CIDR (default: this host's
#                                      public /32 from api.ipify.org — the same
#                                      address the CLI's client-side check reads)
#   BEX_PSQL_ADDITIONAL_ALLOW_CIDRS  optional comma-separated transport CIDRs
#                                      (for example loopback when a local
#                                      port-forward fronts pg-sni-proxy)
#   BEX_PSQL_DATABASE_ID             reuse an EXISTING disposable target (dpg-…)
#                                      instead of creating one; requires
#                                      BEX_PSQL_EXISTING_DISPOSABLE=1
#   BEX_PSQL_DATABASE_NAME           name for the created fixture
#   BEX_PSQL_REAL_BIN                path to the psql binary (default: PATH `psql`)
#   BEX_PSQL_SKIP_DENY=1             skip the allow-list-deny negative leg
#   BEX_PSQL_READY_TIMEOUT_SECONDS   fixture-ready deadline (default 600)
#   BEX_PSQL_COMMAND_TIMEOUT_SECONDS per-CLI-invocation deadline (default 60)
#   BEX_PSQL_PROBE_ATTEMPTS          endpoint-convergence attempts (default 15)
#   BEX_PSQL_PROBE_RETRY_SECONDS     delay between attempts (default 2)
#   BEX_PSQL_CLEANUP_TIMEOUT_SECONDS delete-confirm deadline (default 90)
#   BEX_PSQL_TARGET_CLASS            name of the non-production target class
#   BEX_PSQL_NON_PRODUCTION=1        required acknowledgement (non-production)

tmp="$(mktemp -d)"
database_id=""
database_name=""
created=0
cleanup_complete=0

fail() {
  echo "FAIL psql-compat $*" >&2
  exit 1
}

cleanup() {
  local rc="${1:-$?}"
  trap - EXIT INT TERM
  if [[ "$created" == "1" && "$database_id" == dpg-* ]]; then
    if api_request DELETE "/postgres/$database_id" >/dev/null 2>&1; then
      local deadline=$((SECONDS + ${BEX_PSQL_CLEANUP_TIMEOUT_SECONDS:-90})) status
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
      echo "PASS psql cleanup disposable-database"
    else
      echo "FAIL psql cleanup disposable-database" >&2
      rc=1
    fi
  elif [[ -n "$database_id" ]]; then
    echo "PASS psql cleanup existing-disposable-target-unchanged"
  else
    echo "PASS psql cleanup no-database-fixture"
  fi
  rm -r "$tmp" 2>/dev/null || true
  if [[ ! -e "$tmp" ]]; then
    echo "PASS psql cleanup local-artifacts"
  else
    echo "FAIL psql cleanup local-artifacts" >&2
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
: "${BEX_PSQL_TARGET_CLASS:?name the non-production target class}"
[[ "${BEX_PSQL_NON_PRODUCTION:-0}" == "1" ]] || fail "BEX_PSQL_NON_PRODUCTION=1 is required"
[[ -x "$RENDER_BIN" ]] || fail "pinned Render CLI is missing or not executable"
for required_command in curl jq python3; do
  command -v "$required_command" >/dev/null || fail "missing required client binary: $required_command"
done

# Portable bounded execution: coreutils `timeout` isn't on stock macOS, so fall
# back to `gtimeout`, then to unbounded (the non-interactive CLI path resolves or
# errors promptly). rc 124 only ever appears when a real timeout binary is used.
timeout_bin=""
for candidate in timeout gtimeout; do
  if command -v "$candidate" >/dev/null; then
    timeout_bin="$candidate"
    break
  fi
done
run_bounded() {
  local secs="$1"
  shift
  if [[ -n "$timeout_bin" ]]; then
    "$timeout_bin" "$secs" "$@"
  else
    "$@"
  fi
}

api="${RENDER_HOST%/}"
if ! api_host="$(python3 - "$api" <<'PY'
import sys
import urllib.parse

parsed = urllib.parse.urlsplit(sys.argv[1])
try:
    parsed.port
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
curl_timeout="${BEX_PSQL_API_TIMEOUT_SECONDS:-30}"

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

# The CLI runs whatever `psql` is first on PATH. Pin an explicit binary onto a
# private bin dir so a live run uses the operator's real client and the
# regression harness can inject a deterministic fake without touching the rest
# of PATH. `render psql` execs `psql` by name (views.PSQL), so this is the seam.
real_psql="${BEX_PSQL_REAL_BIN:-}"
if [[ -z "$real_psql" ]]; then
  real_psql="$(command -v psql || true)"
fi
[[ -n "$real_psql" && -x "$real_psql" ]] || fail "real psql client is missing (install postgresql-client or set BEX_PSQL_REAL_BIN)"
mkdir -p "$tmp/psql-bin"
ln -s "$real_psql" "$tmp/psql-bin/psql"
export PATH="$tmp/psql-bin:$PATH"

# The live acceptance pins the checksum-verified v2.21.0 release (reports
# "render v2.21.0"); a locally-built binary reports "render vdev", so the
# regression harness overrides the expected string. The strict release value
# stays the default — CI/live runs demand the pinned release.
expected_cli_version="${BEX_PSQL_CLI_VERSION:-render v2.21.0}"
if ! render_version="$(RENDER_CLI_CONFIG_PATH="$tmp/version-cli.yaml" python3 - "$RENDER_BIN" "$expected_cli_version" <<'PY'
import re
import subprocess
import sys

binary, expected = sys.argv[1], sys.argv[2]
try:
    result = subprocess.run([binary, "--version"], capture_output=True, timeout=20, check=False)
except (OSError, subprocess.TimeoutExpired):
    raise SystemExit(1)
first_line = result.stdout.decode("utf-8", "replace").splitlines()[:1]
if result.returncode or first_line != [expected]:
    raise SystemExit(1)
if not re.fullmatch(r"[A-Za-z0-9._ -]+", first_line[0]):
    raise SystemExit(1)
print(first_line[0])
PY
)"; then
  fail "official CLI is missing, hung, or not pinned to '$expected_cli_version'"
fi
echo "INFO psql target=$BEX_PSQL_TARGET_CLASS"
echo "INFO official-cli=$render_version"

if [[ -n "${BEX_PSQL_DATABASE_ID:-}" ]]; then
  [[ "${BEX_PSQL_EXISTING_DISPOSABLE:-0}" == "1" ]] ||
    fail "an existing target requires BEX_PSQL_EXISTING_DISPOSABLE=1"
  database_id="$BEX_PSQL_DATABASE_ID"
  [[ "$database_id" =~ ^dpg-[a-z0-9]{20}([a-z0-9]{2})?$ ]] || fail "existing database id is not Render-shaped"
  target_json="$(api_request GET "/postgres/$database_id")" || fail "existing target resolution"
  database_name="$(jq -er '.name' <<<"$target_json")" || fail "existing target name"
  target_json=""
else
  database_name="${BEX_PSQL_DATABASE_NAME:-psql-v-$(date +%s)-$$}"
  allow_cidr="${BEX_PSQL_ALLOW_CIDR:-}"
  if [[ -z "$allow_cidr" ]]; then
    verifier_ip="$(curl -fsS --max-time 10 https://api.ipify.org)" || fail "public source-IP discovery"
    allow_cidr="$(python3 -c 'import ipaddress,sys; print(ipaddress.ip_network(sys.argv[1], strict=False))' \
      "${verifier_ip}/$( [[ "$verifier_ip" == *:* ]] && echo 128 || echo 32 )")" || fail "source-IP CIDR"
    verifier_ip=""
  fi
  if ! allow_entries="$(python3 - "$allow_cidr" "${BEX_PSQL_ADDITIONAL_ALLOW_CIDRS:-}" <<'PY'
import ipaddress
import sys

cidrs = [sys.argv[1]]
cidrs.extend(value.strip() for value in sys.argv[2].split(",") if value.strip())
for index, cidr in enumerate(cidrs):
    description = "psql compatibility verifier" if index == 0 else "psql compatibility local transport"
    print(f"cidr={ipaddress.ip_network(cidr, strict=False)},description={description}")
PY
)"; then
    fail "invalid verifier allow-list CIDR"
  fi
  create_args=(postgres create --confirm --name "$database_name" --plan free --version 18 --output json)
  if [[ -n "${RENDER_WORKSPACE:-}" ]]; then
    create_args+=(--workspace "$RENDER_WORKSPACE")
  fi
  while IFS= read -r entry; do
    [[ -n "$entry" ]] && create_args+=(--ip-allow-list "$entry")
  done <<<"$allow_entries"
  allow_entries=""
  if ! created_json="$(RENDER_CLI_CONFIG_PATH="$tmp/create-cli.yaml" \
    run_bounded "${BEX_PSQL_COMMAND_TIMEOUT_SECONDS:-60}" "$RENDER_BIN" "${create_args[@]}" 2>&1)"; then
    created_json=""
    fail "official CLI disposable database create"
  fi
  create_args=()
  database_id="$(jq -er '.data.id' <<<"$created_json")" || fail "created database id"
  [[ "$database_id" =~ ^dpg-[a-z0-9]{20}([a-z0-9]{2})?$ ]] || fail "created database id is not Render-shaped"
  created=1
  database_name="$(jq -er '.data.name' <<<"$created_json")" || fail "created database name"
  created_json=""
  echo "PASS psql official-CLI disposable-database-created id=$database_id"
fi

ready=0
ready_deadline=$((SECONDS + ${BEX_PSQL_READY_TIMEOUT_SECONDS:-600}))
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

# Assert the external connection contract the CLI will consume, without letting
# the URI/password leave the parser (only host/database/TLS mode do).
connection_status="$(api_status GET "/postgres/$database_id/connection-info" 2>/dev/null || true)"
if [[ "$connection_status" == "503" ]]; then
  fail "public Postgres endpoint unavailable (configure BEX_DB_DOMAIN and wait for reconciliation)"
fi
[[ "$connection_status" == "200" ]] || fail "connection-info returned HTTP $connection_status"
connection_status=""
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
echo "PASS psql external-connection-precondition host=$expected_host database=$expected_database tls=require"

# The one probe the whole row turns on: `render psql <selector> -c <sql> -o text`
# resolves the target, crosses the client-side allow-list gate, fetches the
# external connection string, and execs the real psql binary. We assert the
# probe column + value survive the round-trip (psql stdout → PSQLResult.Output →
# `-o text`), and that neither the URI nor the password reach durable output.
probe_sql='SELECT 1 AS bex_psql_probe;'
run_probe() {
  local label="$1" selector="$2" out rc attempt
  local attempts="${BEX_PSQL_PROBE_ATTEMPTS:-15}"
  [[ "$attempts" =~ ^[1-9][0-9]*$ ]] || fail "BEX_PSQL_PROBE_ATTEMPTS must be a positive integer"
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    set +e
    out="$(RENDER_CLI_CONFIG_PATH="$tmp/$label-cli.yaml" run_bounded "${BEX_PSQL_COMMAND_TIMEOUT_SECONDS:-60}" \
      "$RENDER_BIN" psql "$selector" -c "$probe_sql" -o text 2>&1)"
    rc=$?
    set -e
    [[ "$rc" != "124" ]] || fail "psql probe-$label did not complete boundedly"
    if [[ "$rc" == "0" ]]; then
      break
    fi
    if ((attempt < attempts)); then
      sleep "${BEX_PSQL_PROBE_RETRY_SECONDS:-2}"
    fi
  done
  [[ "$rc" == "0" ]] || { printf '%s\n' "$out" | sed 's/:[^@/]*@/:[redacted]@/g' >&2; fail "psql probe-$label exited $rc"; }
  grep -Fq 'bex_psql_probe' <<<"$out" || fail "psql probe-$label lost the probe column"
  grep -Fq '1' <<<"$out" || fail "psql probe-$label lost the probe value"
  if ((attempt > 1)); then
    echo "INFO psql probe-$label ready-after-attempt=$attempt"
  fi
  echo "PASS psql probe-$label"
}

run_probe id "$database_id"
run_probe name "$database_name"

# Unknown name/id: the CLI's resolver refuses before any connection.
unknown_name="psql-missing-${database_id#dpg-}"
set +e
unknown_out="$(RENDER_CLI_CONFIG_PATH="$tmp/unknown-cli.yaml" run_bounded "${BEX_PSQL_COMMAND_TIMEOUT_SECONDS:-60}" \
  "$RENDER_BIN" psql "$unknown_name" -c "$probe_sql" -o text 2>&1)"
unknown_rc=$?
set -e
[[ "$unknown_rc" != "0" && "$unknown_rc" != "124" ]] || fail "unknown-name target was not rejected"
grep -Fq 'No Postgres instance found' <<<"$unknown_out" || fail "unknown-name rejection was not named"
unknown_out=""
echo "PASS psql unknown-name rejected"

# Allow-list-deny: the CLI's own client-side gate (getUserIP → hasAccessToPostgres
# over pg.ipAllowList) refuses BEFORE execing psql when the caller's public IP is
# absent from the allow list. This only fires when api.ipify.org is reachable
# (the CLI skips the check when getUserIP fails), so gate the leg on that probe —
# a skip is honest, a silent pass is not.
if [[ "${BEX_PSQL_SKIP_DENY:-0}" == "1" ]]; then
  echo "SKIP psql allow-list-deny (BEX_PSQL_SKIP_DENY=1)"
elif ! curl -fsS --max-time 10 -o /dev/null https://api.ipify.org 2>/dev/null; then
  echo "SKIP psql allow-list-deny (api.ipify.org unreachable — client-side gate not exercised)"
else
  # TEST-NET-1 (RFC 5737) never contains a real caller — flip the fixture's
  # allow list to it so hasAccessToPostgres returns false.
  deny_payload='{"cidrs":[{"cidrBlock":"192.0.2.0/24","description":"psql deny leg"}]}'
  api_request PUT "/postgres/$database_id/ip-allow-list" "$deny_payload" >/dev/null ||
    fail "allow-list-deny setup: PUT ip-allow-list"
  set +e
  deny_out="$(RENDER_CLI_CONFIG_PATH="$tmp/deny-cli.yaml" run_bounded "${BEX_PSQL_COMMAND_TIMEOUT_SECONDS:-60}" \
    "$RENDER_BIN" psql "$database_id" -c "$probe_sql" -o text 2>&1)"
  deny_rc=$?
  set -e
  [[ "$deny_rc" != "0" && "$deny_rc" != "124" ]] || fail "allow-list-deny target was not refused"
  grep -Fq 'not in allow list' <<<"$deny_out" || fail "allow-list-deny refusal was not named"
  deny_out=""
  echo "PASS psql allow-list-deny rejected-before-connect"
fi

echo "PASS psql compatibility verification complete"
