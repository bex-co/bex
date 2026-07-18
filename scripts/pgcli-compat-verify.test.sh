#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verifier="$repo_root/scripts/pgcli-compat-verify.sh"
pty_runner="$repo_root/scripts/render-cli-pty.py"
shim="$repo_root/scripts/pgcli-verify-client.sh"
tmp="$(mktemp -d)"
server_pid=""
secret='pty-planted-secret-value'

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

# Directly lock the shim's URI and passthrough contract before involving the
# official client. Its safe JSONL record must not retain URI credentials.
shim_record="$tmp/direct-shim.jsonl"
BEX_PGCLI_EXPECT_HOST=db.nonprod.invalid \
  BEX_PGCLI_EXPECT_DATABASE=pgcli_test \
  BEX_PGCLI_EXPECT_ID=dpg-0123456789abcdefghij \
  BEX_PGCLI_EXPECT_LABEL=direct \
  BEX_PGCLI_SHIM_RECORD="$shim_record" \
  "$shim" "postgresql://planted-user:$secret@db.nonprod.invalid:5432/pgcli_test?sslmode=require" \
  --csv -q >"$tmp/direct-shim.out" 2>&1
grep -Fq 'BEX_PGCLI_SHIM_OK' "$tmp/direct-shim.out" || fail "valid shim handoff failed"
jq -e '.label == "direct" and .flags == ["--csv","-q"] and .tls == "require"' \
  "$shim_record" >/dev/null || fail "safe shim record is malformed"
assert_safe "$tmp/direct-shim.out"
assert_safe "$shim_record"

set +e
BEX_PGCLI_EXPECT_HOST=db.nonprod.invalid \
  BEX_PGCLI_EXPECT_DATABASE=pgcli_test \
  BEX_PGCLI_EXPECT_ID=dpg-0123456789abcdefghij \
  BEX_PGCLI_EXPECT_LABEL=malformed \
  BEX_PGCLI_SHIM_RECORD="$shim_record" \
  "$shim" "not-a-postgres-uri-$secret" --csv -q >"$tmp/malformed-shim.out" 2>&1
malformed_rc=$?
set -e
[[ "$malformed_rc" == "64" ]] || fail "malformed URI returned $malformed_rc instead of 64"
grep -Fq 'BEX_PGCLI_SHIM_REJECTED' "$tmp/malformed-shim.out" || fail "malformed URI rejection was not named"
assert_safe "$tmp/malformed-shim.out"
[[ "$(wc -l <"$shim_record" | tr -d ' ')" == "1" ]] || fail "rejected URI wrote a handoff record"

set +e
BEX_PGCLI_EXPECT_HOST=db.nonprod.invalid \
  BEX_PGCLI_EXPECT_DATABASE=pgcli_test \
  BEX_PGCLI_EXPECT_ID=dpg-0123456789abcdefghij \
  BEX_PGCLI_EXPECT_LABEL=flags \
  BEX_PGCLI_SHIM_RECORD="$shim_record" \
  "$shim" "postgresql://planted-user:$secret@db.nonprod.invalid:5432/pgcli_test?sslmode=require" \
  -q --csv >"$tmp/flags-shim.out" 2>&1
flags_rc=$?
set -e
[[ "$flags_rc" == "64" ]] || fail "wrong passthrough order did not fail"
assert_safe "$tmp/flags-shim.out"

# The server records only method/path markers. The credential-bearing URI is
# assembled from an environment variable and is never written to its source or
# request log.
cat >"$tmp/fake-api.py" <<'PY'
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit

PORT_FILE, LOG_FILE = sys.argv[1:]
SECRET = os.environ["FAKE_SECRET"]
MAIN_ID = "dpg-0123456789abcdefghij"
EMPTY_ID = "dpg-empty000000000000000"
BAD_URI_ID = "dpg-baduri00000000000000"
OTHER_ID = "dpg-other000000000000000"
state = {"deleted": False, "name": "pgcli-test"}


def detail(database_id, name=None):
    return {
        "connectionPool": "none",
        "createdAt": "2026-07-18T00:00:00Z",
        "databaseName": "pgcli_test",
        "databaseUser": "planted_user",
        "diskAutoscalingEnabled": False,
        "externalHost": "db.nonprod.invalid",
        "highAvailabilityEnabled": False,
        "id": database_id,
        "ipAllowList": [{"cidrBlock": "0.0.0.0/0", "description": "test verifier"}],
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
        "suspenders": [],
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

    def do_POST(self):
        if self.path != "/v1/postgres":
            self.send_json(404, {"message": "not found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(length) or b"{}")
        state["name"] = body.get("name", "pgcli-test")
        state["deleted"] = False
        self.record("POST create")
        self.send_json(201, detail(MAIN_ID))

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
        if path == "/v1/postgres":
            name = parse_qs(parsed.query).get("name", [""])[0]
            self.record("GET list-name")
            if name.startswith("pgcli-missing-"):
                self.send_json(200, [])
            elif name == "ambiguous":
                self.send_json(200, [
                    {"cursor": "a", "postgres": detail(MAIN_ID, name)},
                    {"cursor": "b", "postgres": detail(OTHER_ID, name)},
                ])
            elif name == state["name"]:
                self.send_json(200, [{"cursor": "", "postgres": detail(MAIN_ID)}])
            else:
                self.send_json(200, [])
            return

        parts = path.split("/")
        if len(parts) == 5 and parts[:3] == ["", "v1", "postgres"] and parts[4] == "connection-info":
            database_id = parts[3]
            self.record("GET connection-info")
            if database_id == EMPTY_ID:
                uri = ""
            elif database_id == BAD_URI_ID:
                uri = "not-a-uri"
            else:
                uri = f"postgresql://planted_user:{SECRET}@db.nonprod.invalid:5432/pgcli_test?sslmode=require"
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
            elif database_id in {MAIN_ID, EMPTY_ID, BAD_URI_ID, OTHER_ID}:
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
FAKE_SECRET="$secret" python3 "$tmp/fake-api.py" "$tmp/port" "$tmp/requests.log" &
server_pid=$!
for _ in {1..100}; do
  [[ -s "$tmp/port" ]] && break
  sleep 0.02
done
[[ -s "$tmp/port" ]] || fail "fake API did not start"
api="http://127.0.0.1:$(<"$tmp/port")/v1/"

cat >"$tmp/fake-pgcli" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
record="${BEX_FAKE_PGCLI_RECORD:?}"
if [[ "${1:-}" == "--version" ]]; then
  echo version >>"$record"
  echo "Version: pgcli 4.5.0-test"
  exit 0
fi
echo runtime >>"$record"
if [[ -t 0 && -t 1 && -t 2 ]]; then
  echo all-streams-tty >>"$record"
else
  echo missing-stream-tty >>"$record"
  exit 61
fi
uri="${1:-}"
if [[ "$uri" == postgresql://* ]]; then
  echo postgresql-uri >>"$record"
else
  echo malformed-uri >>"$record"
  exit 62
fi
uri=""
printf 'BEX_PGCLI_READY> '
IFS= read -r query
[[ "$query" == 'SELECT 1 AS bex_pgcli_probe;' ]] || exit 63
printf 'bex_pgcli_probe\r\n"1"\r\nSELECT 1\r\n'
IFS= read -r quit
[[ "$quit" == '\q' ]] || exit 64
exit "${BEX_FAKE_PGCLI_EXIT:-0}"
SH
chmod +x "$tmp/fake-pgcli"

common_env=(
  "RENDER_HOST=$api"
  "RENDER_API_KEY=$secret"
  "RENDER_BIN=$render_bin"
  "RENDER_WORKSPACE=tea-0123456789abcdefghij"
  "BEX_PGCLI_TARGET_CLASS=deterministic non-production fake API"
  "BEX_PGCLI_NON_PRODUCTION=1"
  "BEX_PGCLI_ALLOW_CIDR=0.0.0.0/0"
  "BEX_PGCLI_READY_TIMEOUT_SECONDS=5"
  "BEX_PGCLI_COMMAND_TIMEOUT_SECONDS=10"
  "BEX_PGCLI_REAL_TIMEOUT_SECONDS=20"
  "BEX_PGCLI_CLEANUP_TIMEOUT_SECONDS=5"
  "BEX_FAKE_PGCLI_RECORD=$tmp/fake-pgcli-record"
)
: >"$tmp/fake-pgcli-record"

set +e
env "${common_env[@]}" RENDER_HOST=https://api.bex.co/v1/ \
  bash "$verifier" >"$tmp/production.out" 2>&1
production_rc=$?
set -e
[[ "$production_rc" != "0" ]] || fail "production API guard unexpectedly passed"
grep -Fq 'FAIL pgcli-compat production APIs are forbidden' "$tmp/production.out" ||
  fail "production API rejection was not named"
assert_safe "$tmp/production.out"

if ! env "${common_env[@]}" BEX_PGCLI_REAL_BIN="$tmp/fake-pgcli" \
  BEX_PGCLI_AMBIGUOUS_NAME=ambiguous \
  bash "$verifier" >"$tmp/full.out" 2>&1; then
  grep -E '^(PASS|FAIL|INFO|SKIP)' "$tmp/full.out" | sed "s/$secret/[redacted]/g" >&2 || true
  sed -n '1,80p' "$tmp/fake-pgcli-record" >&2
  tail -n 40 "$tmp/requests.log" >&2
  fail "full verifier failed"
fi
for marker in \
  'PASS pgcli official-non-tty-guard' \
  'PASS pgcli id-resolution-and-shim-handoff' \
  'PASS pgcli unknown-name rejected-before-child' \
  'PASS pgcli name-resolution-and-shim-handoff' \
  'PASS pgcli ambiguous-name rejected-before-child' \
  'PASS pgcli real-sql-id' \
  'PASS pgcli real-sql-name' \
  'PASS pgcli cleanup disposable-database' \
  'PASS pgcli cleanup local-artifacts'; do
  grep -Fq "$marker" "$tmp/full.out" || fail "full verifier missed marker: $marker"
done
assert_safe "$tmp/full.out"
assert_safe "$tmp/requests.log"

python3 - "$tmp/requests.log" <<'PY'
import sys

events = open(sys.argv[1], encoding="ascii").read().splitlines()
if events.count("GET connection-info") != 5:
    raise SystemExit(f"wanted five preflight/id/name connection-info reads, got {events}")
if events.count("DELETE database") != 1:
    raise SystemExit(f"cleanup delete count mismatch: {events}")
PY

for mode in empty malformed; do
  case "$mode" in
    empty) target=dpg-empty000000000000000 ;;
    malformed) target=dpg-baduri00000000000000 ;;
  esac
  set +e
  env "${common_env[@]}" \
    BEX_PGCLI_DATABASE_ID="$target" BEX_PGCLI_EXISTING_DISPOSABLE=1 BEX_PGCLI_SKIP_REAL=1 \
    bash "$verifier" >"$tmp/$mode.out" 2>&1
  mode_rc=$?
  set -e
  [[ "$mode_rc" != "0" ]] || fail "$mode external connection info unexpectedly passed"
  grep -Fq 'FAIL pgcli-compat empty or malformed external connection information' "$tmp/$mode.out" ||
    fail "$mode external connection failure was not named"
  assert_safe "$tmp/$mode.out"
done

set +e
env "${common_env[@]}" \
  BEX_PGCLI_DATABASE_ID=dpg-other000000000000000 BEX_PGCLI_EXISTING_DISPOSABLE=1 \
  BEX_PGCLI_REAL_BIN="$tmp/does-not-exist" \
  bash "$verifier" >"$tmp/missing-real.out" 2>&1
missing_real_rc=$?
set -e
[[ "$missing_real_rc" != "0" ]] || fail "missing real pgcli unexpectedly passed"
grep -Fq 'FAIL pgcli-compat real pgcli is missing' "$tmp/missing-real.out" ||
  fail "missing real pgcli failure was not named"
assert_safe "$tmp/missing-real.out"

set +e
env "${common_env[@]}" BEX_FAKE_PGCLI_EXIT=42 \
  BEX_PGCLI_DATABASE_ID=dpg-other000000000000000 BEX_PGCLI_EXISTING_DISPOSABLE=1 \
  BEX_PGCLI_REAL_BIN="$tmp/fake-pgcli" \
  bash "$verifier" >"$tmp/nonzero-real.out" 2>&1
nonzero_real_rc=$?
set -e
[[ "$nonzero_real_rc" == "42" ]] || fail "real-client exit 42 was not propagated (got $nonzero_real_rc)"
grep -Fq 'FAIL pgcli real-client-id status=42' "$tmp/nonzero-real.out" ||
  fail "non-zero real-client exit was not named"
assert_safe "$tmp/nonzero-real.out"

for output in "$tmp"/*.out; do
  assert_safe "$output"
done
if grep -R -Fq "$secret" "$tmp" --exclude='*.out'; then
  fail "planted credential reached a temporary artifact"
fi
echo "PASS pgcli compatibility regression suite"
