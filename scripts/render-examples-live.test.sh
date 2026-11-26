#!/usr/bin/env bash
# Self-test for scripts/render-examples-live.sh (w6/m644): its outcome and
# stage reporting, the plan guard that keeps it from touching a resource it did
# not create, and the cleanup that must run after every failure. A fake curl
# answers as bex-api and a template's web service would, from a scenario; no
# network and no credential is used.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

mkdir -p "$tmp/bin"
# public_log.py asks kubectl for its key; keep the test off any real cluster.
printf '#!/bin/sh\nexit 1\n' >"$tmp/bin/kubectl"
cat >"$tmp/bin/curl" <<'CURL'
#!/usr/bin/env python3
"""Fake curl: answers from $SCENARIO, records "METHOD URL" in $STATE/calls."""
import json, os, sys

args = sys.argv[1:]
method, out, url, data, write_code, config = "GET", None, None, None, False, False
i = 0
while i < len(args):
    a = args[i]
    if a == "-X":
        method = args[i + 1]; i += 1
    elif a == "-o":
        out = args[i + 1]; i += 1
    elif a == "-w":
        write_code = True; i += 1
    elif a == "--data":
        data = args[i + 1]; i += 1
    elif a in ("-H", "--max-time"):
        i += 1
    elif a == "-K":
        config = True; i += 1
    elif not a.startswith("-"):
        url = a
    i += 1

state, scenario = os.environ["STATE"], os.environ["SCENARIO"]
token = os.environ.get("BEX_API_TOKEN", "")
if token and token in " ".join(args):
    sys.exit("the API key reached curl's argv")
cfg = sys.stdin.read() if config else ""
if url.startswith("https://api.test/") and f"Bearer {token}" not in cfg:
    sys.exit("an API request carried no key on stdin")
with open(os.path.join(state, "calls"), "a") as f:
    f.write(f"{method} {url}\n")
path = url.replace("https://api.test", "").split("?")[0]

def gone(rid):
    return os.path.exists(os.path.join(state, "deleted-" + rid))

def count(name):
    p = os.path.join(state, "count-" + name)
    n = int(open(p).read()) + 1 if os.path.exists(p) else 1
    open(p, "w").write(str(n))
    return n

resources = [{"id": "srv-web", "name": "web", "type": "web_service"}]
if scenario == "worker":
    resources.append({"id": "srv-wrk", "name": "wrk", "type": "background_worker"})
if scenario == "shared-name":
    resources.insert(0, {"id": "dpg-web", "name": "web", "type": "postgres"})
code, body = 404, {"message": "not found"}

if method == "DELETE":
    rid = path.rstrip("/").split("/")[-1]
    open(os.path.join(state, "deleted-" + rid), "w").close()
    code, body = 204, ""
elif path == "/v1/owners":
    owners = [{"owner": {"id": "tea-test"}, "cursor": "tea-test"}]
    if scenario == "two-owners":
        owners.append({"owner": {"id": "tea-other"}, "cursor": "tea-other"})
    code, body = 200, owners
elif path == "/v1/projects":
    code, body = 200, []
elif path == "/v1/blueprints" and method == "POST":
    if scenario == "refused":
        code, body = 400, {"message": "services[0].plan: plan pro-ultra is not available"}
    elif scenario == "plan-error":
        code, body = 201, {"blueprint": {"id": "exs-1"}, "planningOutcome": "error",
                           "notices": [{"severity": "error", "message": "startCommand: Too long"}], "sync": None}
    else:
        op = "update" if scenario == "adopt" else "create"
        code, body = 201, {"blueprint": {"id": "exs-1"}, "planningOutcome": "planned", "notices": [],
                           "sync": {"id": "exe-1", "requiredManualEnvVars": [],
                                    "rawPlan": {"actions": [{"operation": op, "kind": "service", "name": "web"}]}}}
elif path == "/v1/blueprints/exs-1/approve":
    open(os.path.join(state, "approved"), "w").close()
    code, body = 200, {"id": "exe-1", "state": "success"}
elif path == "/v1/blueprints/exs-1":
    if not gone("exs-1"):
        code, body = 200, {"id": "exs-1", "resources": resources if os.path.exists(os.path.join(state, "approved")) else []}
elif path.endswith("/deploys"):
    rid = path.split("/")[3]
    status = "live"
    if scenario == "build-failed":
        status = "build_failed"
    elif scenario == "deploy-failed":
        status = "update_failed"
    elif scenario == "timeout":
        status = "build_in_progress"
    elif count("deploys-" + rid) < 2:
        status = "build_in_progress"
    code, body = 200, [{"deploy": {"id": "dep-1", "status": status}}]
elif path.startswith("/v1/postgres/"):
    rid = path.split("/")[3]
    if not gone(rid):
        code, body = 200, {"id": rid, "status": "available" if count(rid) > 1 else "creating"}
elif path.endswith("/events"):
    code, body = 200, [{"event": {"type": "deploy_ended",
                                  "details": {"failureReason": "container exited shortly after start (last exit code 1)"}}}]
elif path.startswith("/v1/services/"):
    rid = path.split("/")[3]
    if rid.startswith("srv-") and (not gone(rid) or scenario == "stuck"):
        code, body = 200, {"id": rid, "slug": rid, "serviceDetails": {"url": "https://web.test"}}
elif path == "/v1/logs":
    if "type=build" in url:
        code, body = 200, {"logs": [{"message": "npm ERR! missing script: build"}, {"message": "==> Cloning"}]}
    else:
        logs = {"worker": ["worker ready."], "deploy-failed": ["== Sinatra has ended his set (crowd applauds)"]}
        code, body = 200, {"logs": [{"message": m} for m in logs.get(scenario, [])]}
elif url == "https://web.test/":
    code, body = (500, "oops") if scenario == "check-failed" else (200, "<h1>Hello from Render!</h1>")

text = body if isinstance(body, str) else json.dumps(body)
if out:
    open(out, "w").write(text)
else:
    sys.stdout.write(text)
if write_code:
    sys.stdout.write(str(code))
CURL
chmod +x "$tmp/bin/curl"

# A copy of the script beside public_log.py, as in scripts/. fixture-* is no
# template in the check table, so it gets the default check unless a test sets
# RENDER_EXAMPLES_LIVE_CHECKS.
cp "$here/render-examples-live.sh" "$tmp/live.sh"
mkdir -p "$tmp/lib" && cp "$here/lib/public_log.py" "$tmp/lib/"

# run <scenario> <repo> [flags...]: sets $out, $rc and $calls.
run() {
  local scenario="$1" repo="$2"
  shift 2
  rm -rf "$tmp/state" && mkdir -p "$tmp/state"
  set +e
  out="$(PATH="$tmp/bin:$PATH" STATE="$tmp/state" SCENARIO="$scenario" BEX_API_URL=https://api.test \
    BEX_API_TOKEN=bex_test_secret BEX_OWNER_ID="${OWNER-tea-test}" RENDER_EXAMPLES_LIVE_POLL_SECONDS=0 \
    RENDER_EXAMPLES_LIVE_CLEANUP_SECONDS=2 RENDER_EXAMPLES_LIVE_TIMEOUT_SECONDS="${TIMEOUT:-30}" \
    RENDER_EXAMPLES_LIVE_CHECK_SECONDS=1 RENDER_EXAMPLES_LIVE_CHECKS="${CHECKS:-}" \
    bash "$tmp/live.sh" "$@" "$repo" 2>&1)"
  rc=$?
  set -e
  calls="$(cat "$tmp/state/calls" 2>/dev/null || true)"
}

# expect <label> <want-rc> <first-line-prefix> [<needle in output>...]
expect() {
  local label="$1" want="$2" prefix="$3" needle
  shift 3
  if [ "$rc" -ne "$want" ] || [[ "$(printf '%s' "$out" | head -n1)" != "$prefix"* ]]; then
    echo "FAIL: $label — exit $rc (want $want): $out" >&2
    fails=$((fails + 1))
    return
  fi
  for needle; do
    if ! printf '%s\n%s' "$out" "$calls" | grep -qF -- "$needle"; then
      echo "FAIL: $label — no \"$needle\" in: $out / $calls" >&2
      fails=$((fails + 1))
      return
    fi
  done
  echo "ok: $label"
}

# cleaned <label>: the service and the Blueprint were both deleted.
cleaned() {
  if [ -e "$tmp/state/deleted-srv-web" ] && [ -e "$tmp/state/deleted-exs-1" ]; then
    echo "ok: $1 cleans up"
  else
    echo "FAIL: $1 left resources: $calls" >&2
    fails=$((fails + 1))
  fi
}

run live fixture-a
expect "a template with no listed check is live when its web service answers" 0 "fixture-a live - " \
  "GET https://web.test/" "cleanup: ok, deleted the Blueprint and 1 resource(s)"
cleaned "a live run"

CHECKS=$'http web / Hello from Render!\nlog wrk worker ready.' run worker fixture-w
expect "an http and a log check pass" 0 "fixture-w live - 2 resource(s)" "type=app&text=worker%20ready."
CHECKS='log wrk never logged' run worker fixture-w
expect "a log line that never comes fails the check" 1 'fixture-w failed check wrk: the app log never showed "never logged"'
cleaned "a failed log check"

CHECKS='http web / Hello' run shared-name fixture-s
expect "a check finds the service when a database shares its name" 0 "fixture-s live - 2 resource(s)" \
  "GET https://web.test/" "DELETE https://api.test/v1/postgres/dpg-web"

OWNER='' run live fixture-a
expect "a key that reaches one workspace needs no BEX_OWNER_ID" 0 "fixture-a live - " "ownerId=tea-test"
OWNER='' run two-owners fixture-a
expect "a key that reaches two workspaces asks for BEX_OWNER_ID" 2 "error: the key reaches 2 workspaces; set BEX_OWNER_ID"
if printf '%s' "$calls" | grep -q 'POST https://api.test/v1/blueprints'; then
  echo "FAIL: a run with no workspace created a Blueprint" >&2
  fails=$((fails + 1))
fi

run refused fixture-a
expect "a refused create is stage validate" 1 "fixture-a failed validate POST /v1/blueprints answered 400: services[0].plan"
if printf '%s' "$calls" | grep -q approve; then
  echo "FAIL: a refused create was approved" >&2
  fails=$((fails + 1))
fi

run plan-error fixture-a
expect "planning errors are stage plan" 1 "fixture-a failed plan startCommand: Too long" "DELETE https://api.test/v1/blueprints/exs-1"

run adopt fixture-a
expect "a plan that changes an existing resource is not approved" 1 \
  "fixture-a failed plan the plan would change existing resources, so it was not approved: update service web"
if printf '%s' "$calls" | grep -qE 'approve|DELETE https://api.test/v1/services'; then
  echo "FAIL: an adopting plan was approved or a resource it named was deleted: $calls" >&2
  fails=$((fails + 1))
fi

run build-failed fixture-a
expect "a failed build is stage build with its error line" 1 "fixture-a failed build web: npm ERR! missing script: build"
cleaned "a failed build"

run deploy-failed fixture-a
expect "a failed release names its failure reason when its log has no error line" 1 \
  "fixture-a failed deploy web (update_failed): container exited shortly after start (last exit code 1)"
cleaned "a failed release"

TIMEOUT=1 run timeout fixture-a
expect "a build past the deadline is a timeout" 1 "fixture-a timeout build after 1s: web build_in_progress"
cleaned "a timeout"

run check-failed fixture-a
expect "a failing check is stage check" 1 "fixture-a failed check web: GET https://web.test/ answered 500"
cleaned "a failed check"

run build-failed fixture-a --keep
expect "--keep leaves a failed run's resources" 1 "fixture-a failed build" "cleanup: kept for triage, delete by hand: Blueprint exs-1 srv-web"
if printf '%s' "$calls" | grep -q DELETE; then
  echo "FAIL: --keep deleted something: $calls" >&2
  fails=$((fails + 1))
fi

run stuck fixture-a
expect "a resource that never goes away is reported" 3 "fixture-a live - " "cleanup: LEFT srv-web"

[ "$fails" -eq 0 ] || exit 1
