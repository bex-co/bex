#!/usr/bin/env bash
# Offline tests for scripts/terraform-provider-verify.sh (w9/m474): argument
# and work-directory refusals, the per-test result table, and the resource
# sweep against a fake bex API. No network, Terraform or provider clone.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
verify="$repo_root/scripts/terraform-provider-verify.sh"
helper="$repo_root/scripts/terraform-provider-verify/helper.py"
tmp="$(mktemp -d)"
server_pid=""
cleanup() {
  [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT

pass=0
fail() {
  echo "FAIL: $*" >&2
  exit 1
}
ok() {
  pass=$((pass + 1))
  echo "ok - $*"
}

# --- arguments and the work directory ------------------------------------

set +e
out="$(bash "$verify" 2>&1)"
code=$?
set -e
[ "$code" = 2 ] && grep -q "usage:" <<<"$out" || fail "no arguments should print usage and exit 2 (got $code)"
ok "no arguments: usage, exit 2"

set +e
out="$(bash "$verify" webservice bogus 2>&1)"
code=$?
set -e
[ "$code" = 2 ] && grep -q "unknown provider package 'bogus'" <<<"$out" || fail "an unknown package should be refused (got $code: $out)"
ok "unknown package refused before anything runs"

set +e
out="$(env -u BEX_API_URL bash "$verify" webservice 2>&1)"
code=$?
set -e
[ "$code" != 0 ] && grep -q "BEX_API_URL" <<<"$out" || fail "a missing BEX_API_URL should be refused (got $code)"
ok "missing BEX_API_URL refused"

inside="$repo_root/.tf-verify-test-$$/work"
set +e
out="$(BEX_API_URL=http://127.0.0.1:9 RENDER_API_KEY=k RENDER_OWNER_ID=tea-x TF_VERIFY_WORKDIR="$inside" \
  bash "$verify" webservice 2>&1)"
code=$?
set -e
[ "$code" = 2 ] && grep -q "inside the repository" <<<"$out" || fail "a work directory in the repository should be refused (got $code: $out)"
[ ! -e "$repo_root/.tf-verify-test-$$" ] || fail "the refused work directory was created in the repository"
ok "work directory inside the repository refused, nothing created"

ln -s "$repo_root/scripts" "$tmp/into-repo"
set +e
out="$(BEX_API_URL=http://127.0.0.1:9 RENDER_API_KEY=k RENDER_OWNER_ID=tea-x TF_VERIFY_WORKDIR="$tmp/into-repo/new-dir" \
  bash "$verify" project 2>&1)"
code=$?
set -e
[ "$code" = 2 ] && grep -q "inside the repository" <<<"$out" || fail "a symlink into the repository should be refused (got $code: $out)"
[ ! -e "$repo_root/scripts/new-dir" ] || fail "the refused symlinked work directory was created"
ok "symlinked work directory resolving into the repository refused"

# --- the result table ------------------------------------------------------

src="$tmp/src"
pkg_dir="$src/internal/provider/fakepkg"
mkdir -p "$pkg_dir/resource/internal" "$src/internal/provider/emptypkg"
cat >"$pkg_dir/resource/resource_test.go" <<'GO'
package resource_test

func TestLive(t *testing.T) {
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: th.SetupRecordingProvider(t, "live_cassette")})
}

func TestBroken(t *testing.T) {
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: th.SetupRecordingProviderConfigureWait(t, "broken_cassette", true)})
}

func TestDrift(t *testing.T) {
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: th.SetupRecordingProvider(t, "drift_cassette")})
}

func TestFake(t *testing.T) {
	fakeServer := httptest.NewServer(nil)
	_ = fakeServer
}
GO
cat >"$pkg_dir/resource/internal/update_test.go" <<'GO'
package internal

func TestUnit(t *testing.T) {}
GO

listed="$(python3 -I "$helper" tests "$src" fakepkg)"
[ "$listed" = "$(printf 'resource\tTestBroken\tbex\nresource\tTestDrift\tbex\nresource\tTestFake\toffline\nresource\tTestLive\tbex\nresource/internal\tTestUnit\toffline')" ] ||
  fail "tests should list every top-level test with its directory and target: $listed"
[ -z "$(python3 -I "$helper" tests "$src" emptypkg)" ] || fail "a package with no tests lists none"
ok "tests: each test with its directory, recorded against bex or offline"

base=terraform-provider-render/internal/provider/fakepkg
event() { jq -nc --arg p "$base/$1" --arg t "$2" --arg a "$3" --arg o "${4:-}" --argjson e "${5:-0}" \
  '{Package:$p,Test:$t,Action:$a} + (if $a=="output" then {Output:$o} else {Elapsed:$e} end)'; }
{
  event resource TestLive run
  event resource TestLive pass "" 12.4
  event resource TestLive/sub fail "" 1
  event resource TestBroken run
  event resource TestBroken output $'    resource_test.go:17: Step 1/3 error: Error running apply: exit status 1\n'
  event resource TestBroken output $'        Error: Error creating service\n'
  event resource TestBroken output $'          with render_cron_job.cron,\n'
  event resource TestBroken output $'          on main.tf line 1, in resource "render_cron_job" "cron":\n'
  event resource TestBroken output $'        Could not create service, unexpected error: a | b conflict\n'
  event resource TestBroken output $'--- FAIL: TestBroken (3.00s)\n'
  event resource TestBroken fail "" 3
  event resource TestDrift run
  event resource TestDrift output $'    resource_test.go:30: Step 2/3 error: After applying this test step, the non-refresh plan was not empty.\n'
  event resource TestDrift output $'        stdout:\n'
  event resource TestDrift output $'          ~ region = "frankfurt" -> "oregon" # forces replacement\n'
  event resource TestDrift output $'          ~ name   = "a" -> "b"\n'
  event resource TestDrift output $'          ~ plan   = "x" -> "y"\n'
  event resource TestDrift fail "" 5
  event resource TestFake fail "" 0
  event resource/internal TestUnit pass "" 0
} >"$tmp/log.json"

set +e
table="$(python3 -I "$helper" summarize "$src" fakepkg "$tmp/log.json")"
code=$?
set -e
[ "$code" = 1 ] || fail "a failing bex test should exit 1 (got $code)"
grep -qxF '| fakepkg/resource | TestLive | bex | PASS | 12 | - |' <<<"$table" || fail "TestLive row: $table"
grep -qxF '| fakepkg/resource | TestBroken | bex | FAIL | 3 | Step 1/3 error: Error running apply: exit status 1 — Error creating service Could not create service, unexpected error: a \| b conflict |' <<<"$table" ||
  fail "TestBroken row should name the step and the error, with | escaped: $table"
grep -qxF '| fakepkg/resource | TestDrift | bex | FAIL | 5 | Step 2/3 error: After applying this test step, the non-refresh plan was not empty. — ~ region = "frankfurt" -> "oregon" # forces replacement ~ name   = "a" -> "b" |' <<<"$table" ||
  fail "TestDrift row should name the first attributes the plan changes: $table"
grep -qxF '| fakepkg/resource | TestFake | offline | FAIL | 0 | - |' <<<"$table" || fail "a test without the recording provider is offline: $table"
grep -qxF '| fakepkg/resource/internal | TestUnit | offline | PASS | 0 | - |' <<<"$table" || fail "TestUnit row: $table"
! grep -q 'TestLive/sub' <<<"$table" || fail "subtests are not rows"
ok "table: one row per top-level test, bex vs offline, first error"

grep -v -e '"TestBroken"' -e '"TestDrift"' "$tmp/log.json" >"$tmp/offline-only.json"
python3 -I "$helper" summarize "$src" fakepkg "$tmp/offline-only.json" >/dev/null ||
  fail "an offline-only failure says nothing about bex and should not fail the run"
ok "an offline test's failure does not grade bex"

echo '{"Package":"terraform-provider-render/internal/provider/fakepkg/resource","Action":"output","Output":"FAIL\tterraform-provider-render/internal/provider/fakepkg/resource [build failed]\n"}' >"$tmp/build.json"
set +e
table="$(python3 -I "$helper" summarize "$src" fakepkg "$tmp/build.json")"
code=$?
set -e
[ "$code" = 1 ] && grep -q '| fakepkg/resource | (build) | - | FAIL |' <<<"$table" || fail "a build failure should be a failing row (got $code: $table)"
ok "a package that does not compile fails"

: >"$tmp/empty.json"
table="$(python3 -I "$helper" summarize "$src" emptypkg "$tmp/empty.json")"
grep -q '| emptypkg | (none) |' <<<"$table" || fail "a package with no tests should say so: $table"
ok "a package with no tests is a row"

# --- the sweep ---------------------------------------------------------------

cat >"$tmp/fake_api.py" <<'PY'
import json, sys, urllib.parse
from http.server import BaseHTTPRequestHandler, HTTPServer

# collection -> {id: name}; wrapped list items like Render's {cursor, <kind>}
state = {"services": {f"srv-old{i:03d}": f"old-{i}" for i in range(120)}, "postgres": {}, "projects": {}, "registrycredentials": {}}
kinds = {"services": "service", "postgres": "postgres", "projects": "project"}
stuck, deletes = set(), []

class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def send(self, code, body=None):
        data = json.dumps(body).encode() if body is not None else b""
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)
    def do_GET(self):
        u = urllib.parse.urlparse(self.path)
        q = urllib.parse.parse_qs(u.query)
        if u.path == "/control/deletes":
            return self.send(200, deletes)
        if u.path == "/control/state":
            return self.send(200, state)
        if self.headers.get("Authorization") != "Bearer test-key":
            return self.send(401, {"message": "unauthorized"})
        coll = u.path.removeprefix("/v1/")
        if coll == "env-groups":
            return self.send(503, {"code": "SECRETS_UNAVAILABLE"})
        if coll not in state:
            return self.send(404, {"message": "not found"})
        if q.get("ownerId") != ["tea-test"]:
            return self.send(400, {"message": "ownerId"})
        ids = sorted(state[coll])
        limit = int(q.get("limit", ["20"])[0])
        start = ids.index(q["cursor"][0]) + 1 if "cursor" in q else 0
        page = ids[start:start + limit]
        if coll == "registrycredentials":
            return self.send(200, [{"id": i, "name": state[coll][i]} for i in page])
        return self.send(200, [{"cursor": i, kinds[coll]: {"id": i, "name": state[coll][i]}} for i in page])
    def do_POST(self):
        u = urllib.parse.urlparse(self.path)
        q = {k: v[0] for k, v in urllib.parse.parse_qs(u.query).items()}
        if u.path == "/control/add":
            state[q["collection"]][q["id"]] = q["name"]
            if q.get("stuck"):
                stuck.add(q["id"])
            return self.send(200, {})
        self.send(404)
    def do_DELETE(self):
        coll, _, rid = self.path.removeprefix("/v1/").partition("/")
        deletes.append(f"{coll}/{rid}")
        if rid not in stuck:
            state[coll].pop(rid, None)
        self.send(204)

srv = HTTPServer(("127.0.0.1", 0), H)
open(sys.argv[1], "w").write(str(srv.server_address[1]))
srv.serve_forever()
PY
python3 -I "$tmp/fake_api.py" "$tmp/port" &
server_pid=$!
for _ in $(seq 1 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
[ -s "$tmp/port" ] || fail "the fake API did not start"
api="http://127.0.0.1:$(cat "$tmp/port")"
add() { curl -sf -X POST "$api/control/add?collection=$1&id=$2&name=$3${4:+&stuck=1}" >/dev/null; }
export BEX_API_URL="$api/v1/" RENDER_API_KEY=test-key RENDER_OWNER_ID=tea-test TF_VERIFY_SWEEP_WAIT=0

python3 -I "$helper" snapshot "$tmp/before.json" 2>"$tmp/snapshot.err"
[ "$(jq '.services | length' "$tmp/before.json")" = 120 ] || fail "the snapshot should follow the cursor past one page"
grep -q "GET /env-groups answered 503" "$tmp/snapshot.err" || fail "an unavailable collection should be noted"
ok "snapshot pages through every collection; an unavailable one is noted, not fatal"

add services srv-new1 web
add services srv-zzz2 cron
add postgres dpg-new1 db
add projects prj-new1 first
add registrycredentials rgc-new1 creds
python3 -I "$helper" sweep "$tmp/before.json" >"$tmp/sweep.out" 2>&1 || fail "the sweep should succeed: $(cat "$tmp/sweep.out")"
deletes="$(curl -sf "$api/control/deletes" | jq -r '.[]')"
[ "$(sort <<<"$deletes" | tr '\n' ' ')" = "postgres/dpg-new1 projects/prj-new1 registrycredentials/rgc-new1 services/srv-new1 services/srv-zzz2 " ] ||
  fail "the sweep should delete exactly the resources created after the snapshot: $deletes"
[ "$(tail -1 <<<"$deletes")" = projects/prj-new1 ] || fail "projects should be deleted last: $deletes"
[ "$(curl -sf "$api/control/state" | jq '.services | length')" = 120 ] || fail "the pre-existing services should be untouched"
grep -q "swept services/srv-zzz2 (cron): DELETE 204" "$tmp/sweep.out" || fail "the sweep should report each deletion: $(cat "$tmp/sweep.out")"
ok "sweep deletes only what appeared during the run, services first and projects last"

add services srv-new3 survivor stuck
set +e
python3 -I "$helper" sweep "$tmp/before.json" >"$tmp/sweep.out" 2>&1
code=$?
set -e
[ "$code" = 1 ] && grep -q "still listed after the sweep: services/srv-new3" "$tmp/sweep.out" ||
  fail "a resource that survives its DELETE should fail the sweep (got $code: $(cat "$tmp/sweep.out"))"
ok "a resource still listed after its DELETE fails the sweep"

echo "terraform-provider-verify: $pass checks passed"
