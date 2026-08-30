#!/usr/bin/env bash
set -euo pipefail

# Hermetic regression for scripts/psql-compat-verify.sh. A deterministic fake
# bex-api and a fake `psql` binary drive the REAL, unmodified Render CLI through
# its non-interactive psql path (resolve by id/name → client-side allow-list
# gate → connection-info → exec psql). No live cluster, database, network, or
# real psql client is required — only the pinned render binary. Sensitive
# material (a planted password) must never reach durable output.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verifier="$repo_root/scripts/psql-compat-verify.sh"
tmp="$(mktemp -d)"
server_pid=""
secret='psql-planted-secret-value'

cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
  rm -r "$tmp" 2>/dev/null || true
}
trap cleanup EXIT

fail() {
  echo "FAIL $*" >&2
  exit 1
}

assert_safe() {
  local path="$1"
  if grep -Fq "$secret" "$path"; then
    fail "planted credential reached durable output: $path"
  fi
}

render_bin="${RENDER_BIN:-}"
if [[ -z "$render_bin" ]] && command -v render >/dev/null; then
  render_bin="$(command -v render)"
fi
[[ -n "$render_bin" && -x "$render_bin" ]] || fail "set RENDER_BIN to the pinned official CLI"
render_version_output="$("$render_bin" --version 2>/dev/null)"
render_version="${render_version_output%%$'\n'*}"
render_version_output=""
[[ "$render_version" =~ ^render\ v[[:alnum:].+-]+$ ]] || fail "RENDER_BIN did not report a safe Render CLI version"

# Fake bex-api. The credential-bearing URI is assembled from an environment
# variable and never written to the request log. The allow list is stateful so
# the deny leg's PUT is reflected on the next GET the CLI reads.
cat >"$tmp/fake-api.py" <<'PY'
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit

PORT_FILE, LOG_FILE, EDGE_FILE = sys.argv[1:]
SECRET = os.environ["FAKE_SECRET"]
MAIN_ID = "dpg-0123456789abcdefghij"
state = {"deleted": False, "name": "psql-test",
         "allow": [{"cidrBlock": "0.0.0.0/0", "description": "test verifier"}]}


def detail(database_id, name=None):
    return {
        "createdAt": "2026-07-18T00:00:00Z",
        "databaseName": "psql_test",
        "databaseUser": "planted_user",
        "diskAutoscalingEnabled": False,
        "externalHost": "db.nonprod.invalid",
        "highAvailabilityEnabled": False,
        "id": database_id,
        "ipAllowList": state["allow"],
        "name": name or state["name"],
        "owner": {"id": "tea-0123456789abcdefghij", "name": "test"},
        "ownerId": "tea-0123456789abcdefghij",
        "plan": "free",
        "public": True,
        "readReplicas": [],
        "region": "oregon",
        "role": "primary",
        "status": "available",
        "suspended": "not_suspended",
        "updatedAt": "2026-07-18T00:00:00Z",
        "version": "17",
    }


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        return

    def record(self, marker):
        with open(LOG_FILE, "a", encoding="ascii") as stream:
            stream.write(marker + "\n")

    def send_json(self, status, body):
        encoded = json.dumps(body, separators=(",", ":")).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def read_body(self):
        length = int(self.headers.get("Content-Length", "0"))
        return json.loads(self.rfile.read(length) or b"{}")

    def do_POST(self):
        if self.path != "/v1/postgres":
            self.send_json(404, {"message": "not found"})
            return
        body = self.read_body()
        state["name"] = body.get("name", "psql-test")
        state["deleted"] = False
        if body.get("ipAllowList"):
            state["allow"] = body["ipAllowList"]
        self.record("POST create")
        self.send_json(201, detail(MAIN_ID))

    def do_PUT(self):
        parts = self.path.split("/")
        if len(parts) == 5 and parts[:3] == ["", "v1", "postgres"] and parts[4] == "ip-allow-list":
            body = self.read_body()
            state["allow"] = body.get("cidrs", [])
            self.record("PUT ip-allow-list")
            self.send_json(200, detail(parts[3]))
            return
        self.send_json(404, {"message": "not found"})

    def do_DELETE(self):
        if self.path == f"/v1/postgres/{MAIN_ID}":
            state["deleted"] = True
            self.record("DELETE database")
            self.send_response(204)
            self.end_headers()
            return
        self.send_json(404, {"message": "not found"})

    def do_GET(self):
        parsed = urlsplit(self.path)
        path = parsed.path
        if path == "/v1/owners/tea-0123456789abcdefghij":
            self.record("GET owner")
            self.send_json(200, {
                "id": "tea-0123456789abcdefghij", "name": "test",
                "type": "team", "email": "test@example.invalid",
            })
            return
        if path == "/v1/postgres":
            name = parse_qs(parsed.query).get("name", [""])[0]
            self.record("GET list-name")
            if name in (state["name"], MAIN_ID):
                self.send_json(200, [{"cursor": "", "postgres": detail(MAIN_ID)}])
            else:
                self.send_json(200, [])
            return

        parts = path.split("/")
        if len(parts) == 5 and parts[:3] == ["", "v1", "postgres"] and parts[4] == "connection-info":
            self.record("GET connection-info")
            if os.path.exists(EDGE_FILE):
                self.send_json(503, {
                    "id": "unavailable",
                    "message": "public datastore endpoint unavailable: configure BEX_DB_DOMAIN",
                })
                return
            uri = f"postgresql://planted_user:{SECRET}@db.nonprod.invalid:5432/psql_test?sslmode=require"
            self.send_json(200, {
                "externalConnectionString": uri,
                "internalConnectionString": "",
                "password": SECRET,
                "psqlCommand": "",
            })
            return

        if len(parts) == 4 and parts[:3] == ["", "v1", "postgres"]:
            database_id = parts[3]
            self.record("GET database")
            if database_id == MAIN_ID and state["deleted"]:
                self.send_json(404, {"message": "not found"})
            elif database_id == MAIN_ID:
                self.send_json(200, detail(database_id))
            else:
                self.send_json(404, {"message": "not found"})
            return
        self.send_json(404, {"message": "not found"})


server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
with open(PORT_FILE, "w", encoding="ascii") as stream:
    stream.write(str(server.server_port))
server.serve_forever()
PY

: >"$tmp/requests.log"
FAKE_SECRET="$secret" python3 "$tmp/fake-api.py" "$tmp/port" "$tmp/requests.log" "$tmp/missing-edge" &
server_pid=$!
for _ in {1..100}; do
  [[ -s "$tmp/port" ]] && break
  sleep 0.02
done
[[ -s "$tmp/port" ]] || fail "fake API did not start"
api="http://127.0.0.1:$(<"$tmp/port")/v1/"

# Fake psql: the CLI execs `psql <connString> -c <sql>`. Assert the passthrough
# contract, print psql-shaped output the CLI captures verbatim, and record every
# invocation so the deny leg can prove psql was NOT reached. The credential-
# bearing connection string is received but never echoed.
cat >"$tmp/fake-psql" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
record="${BEX_FAKE_PSQL_RECORD:?}"
echo invoked >>"$record"
conn="${1:-}"
[[ "$conn" == postgresql://* || "$conn" == postgres://* ]] || { echo "bad conn" >&2; exit 71; }
[[ "${2:-}" == "-c" ]] || { echo "missing -c" >&2; exit 72; }
[[ "${3:-}" == 'SELECT 1 AS bex_psql_probe;' ]] || { echo "unexpected sql" >&2; exit 73; }
conn=""
printf ' bex_psql_probe \n----------------\n              1\n(1 row)\n'
exit "${BEX_FAKE_PSQL_EXIT:-0}"
SH
chmod +x "$tmp/fake-psql"

common_env=(
  "RENDER_HOST=$api"
  "RENDER_API_KEY=$secret"
  "RENDER_BIN=$render_bin"
  "RENDER_WORKSPACE=tea-0123456789abcdefghij"
  "BEX_PSQL_TARGET_CLASS=deterministic non-production fake API"
  "BEX_PSQL_NON_PRODUCTION=1"
  "BEX_PSQL_CLI_VERSION=$render_version"
  "BEX_PSQL_ALLOW_CIDR=0.0.0.0/0"
  "BEX_PSQL_ADDITIONAL_ALLOW_CIDRS=127.0.0.1/32,::1/128"
  "BEX_PSQL_REAL_BIN=$tmp/fake-psql"
  "BEX_PSQL_READY_TIMEOUT_SECONDS=5"
  "BEX_PSQL_COMMAND_TIMEOUT_SECONDS=20"
  "BEX_PSQL_PROBE_ATTEMPTS=1"
  "BEX_PSQL_CLEANUP_TIMEOUT_SECONDS=5"
  "BEX_FAKE_PSQL_RECORD=$tmp/fake-psql-record"
)
: >"$tmp/fake-psql-record"

# 1) production API guard fires before any fixture work.
set +e
env "${common_env[@]}" RENDER_HOST=https://api.bex.co/v1/ \
  bash "$verifier" >"$tmp/production.out" 2>&1
production_rc=$?
set -e
[[ "$production_rc" != "0" ]] || fail "production API guard unexpectedly passed"
grep -Fq 'FAIL psql-compat production APIs are forbidden' "$tmp/production.out" ||
  fail "production API rejection was not named"
assert_safe "$tmp/production.out"

# 2) missing real psql client is named, not a silent skip.
set +e
env "${common_env[@]}" BEX_PSQL_REAL_BIN="$tmp/does-not-exist" \
  bash "$verifier" >"$tmp/missing-psql.out" 2>&1
missing_psql_rc=$?
set -e
[[ "$missing_psql_rc" != "0" ]] || fail "missing psql client unexpectedly passed"
grep -Fq 'FAIL psql-compat real psql client is missing' "$tmp/missing-psql.out" ||
  fail "missing psql client failure was not named"
assert_safe "$tmp/missing-psql.out"

# 3) full green path: create → ready → precondition → id/name probes → unknown
#    reject → allow-list-deny (or a gated skip) → cleanup.

# Missing public-edge state must fail with configuration guidance before the
# local psql process is invoked with an empty target.
touch "$tmp/missing-edge"
: >"$tmp/fake-psql-record"
set +e
env "${common_env[@]}" bash "$verifier" >"$tmp/missing-edge.out" 2>&1
missing_edge_rc=$?
set -e
rm -f "$tmp/missing-edge"
[[ "$missing_edge_rc" != "0" ]] || fail "missing public edge unexpectedly passed"
grep -Fq 'configure BEX_DB_DOMAIN' "$tmp/missing-edge.out" ||
  fail "missing public edge did not return actionable guidance"
[[ ! -s "$tmp/fake-psql-record" ]] || fail "missing public edge invoked local psql"
assert_safe "$tmp/missing-edge.out"

if ! env "${common_env[@]}" bash "$verifier" >"$tmp/full.out" 2>&1; then
  grep -E '^(PASS|FAIL|INFO|SKIP)' "$tmp/full.out" | sed "s/$secret/[redacted]/g" >&2 || true
  tail -n 40 "$tmp/requests.log" >&2
  fail "full verifier failed"
fi
for marker in \
  'PASS psql official-CLI disposable-database-created' \
  'PASS psql external-connection-precondition' \
  'PASS psql probe-id' \
  'PASS psql probe-name' \
  'PASS psql unknown-name rejected' \
  'PASS psql cleanup disposable-database' \
  'PASS psql cleanup local-artifacts' \
  'PASS psql compatibility verification complete'; do
  grep -Fq "$marker" "$tmp/full.out" || fail "full verifier missed marker: $marker"
done
# The deny leg is best-effort (needs api.ipify.org); require exactly one of the
# two honest outcomes, never a silent omission.
if ! grep -Eq '^(PASS psql allow-list-deny rejected-before-connect|SKIP psql allow-list-deny)' "$tmp/full.out"; then
  fail "allow-list-deny leg neither passed nor honestly skipped"
fi
# The fake psql runs exactly twice (id + name); the deny leg must never reach it.
[[ "$(grep -c invoked "$tmp/fake-psql-record")" == "2" ]] ||
  fail "fake psql invocation count != 2 (deny leg reached the client, or a probe was skipped)"
assert_safe "$tmp/full.out"
assert_safe "$tmp/requests.log"
assert_safe "$tmp/fake-psql-record"

# 4) a non-zero psql exit is surfaced, not swallowed. The render CLI reports the
#    underlying code as `Error: exit status 42` and itself exits 1, so the
#    verifier fails the probe (exit 1) AND the real psql code reaches the log.
set +e
env "${common_env[@]}" BEX_FAKE_PSQL_EXIT=42 \
  bash "$verifier" >"$tmp/nonzero.out" 2>&1
nonzero_rc=$?
set -e
[[ "$nonzero_rc" != "0" ]] || fail "non-zero psql exit unexpectedly passed"
grep -Fq 'FAIL psql-compat psql probe-id exited 1' "$tmp/nonzero.out" ||
  fail "non-zero psql exit was not named"
grep -Fq 'exit status 42' "$tmp/nonzero.out" ||
  fail "underlying psql exit code was swallowed"
assert_safe "$tmp/nonzero.out"

for output in "$tmp"/*.out; do
  assert_safe "$output"
done
if grep -R -Fq "$secret" "$tmp" --exclude='*.out'; then
  fail "planted credential reached a temporary artifact"
fi
echo "PASS psql compatibility regression suite"
