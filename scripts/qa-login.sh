#!/usr/bin/env bash
# Sign the QA user in and hand the Playwright MCP browser a live session —
# without QA_PASSWORD ever passing through the agent's context.
#
#   bash scripts/qa-login.sh [OUT]           write a storage-state file
#   bash scripts/qa-login.sh --serve         serve the state once over loopback
#   bash scripts/qa-login.sh --logout PATH   revoke the Kratos session in PATH
#   bash scripts/qa-login.sh --login-challenge CHALLENGE [--cookie-jar PATH] [OUT]
#                                           finish a CLI device OAuth login
#
# Reads QA_EMAIL/QA_PASSWORD from the environment, .env, or muse.env, completes the
# Kratos password login against $KRATOS_PUB, and writes OUT: a Playwright
# storage-state file (cookies only, mode 600) that /qa-find-bugs restores with
# the browser_set_storage_state MCP tool. Credentials go to curl by file, never
# in argv (ps leaks argv), and stdout is exactly two lines — "ok <path-or-url>"
# then "logout <jar>" — so no secret is ever printed. Consumed by
# .claude/skills/qa-find-bugs/SKILL.md.
#
# Every successful login also writes this run's own 0600 Netscape cookie jar at
# .playwright-mcp/qa-session-<run>.jar (gitignored), printed on the logout line,
# so the hunt can revoke the session later without putting cookies in the agent
# transcript. Per-run paths keep concurrent hunts in one checkout from
# overwriting or revoking each other's sessions (w4/343), which is also why
# --logout takes the handle explicitly instead of guessing a shared default. The
# session must outlive this script (the browser uses it for the whole hunt), so
# revocation is NOT in the EXIT trap — callers must run --logout when the hunt
# ends (Phase 8).
#
# OUT defaults to .playwright-mcp/qa-storage-state-<run>.json. Delete OUT and
# the jar after --logout: they carried a live session cookie.
# --cookie-jar seeds a private copy of a jar or storage state. KRATOS_PUB, DASH,
# and OAUTH_PUB configure the only origins allowed during OAuth continuation.
set -euo pipefail
cd "$(dirname "$0")/.."

KRATOS_PUB="${KRATOS_PUB:-https://auth.bex.co}"
DASH="${DASH:-https://dashboard.bex.co}"
OAUTH_PUB="${OAUTH_PUB:-https://oauth.bex.co}"
RUN_ID="$(python3 -c 'import secrets; print(secrets.token_hex(6))')"
DEFAULT_OUT=".playwright-mcp/qa-storage-state-$RUN_ID.json"
RUN_JAR=".playwright-mcp/qa-session-$RUN_ID.jar"

qa_curl() {
  curl --connect-timeout 10 --max-time 30 "$@"
}

MODE=login
SERVE=0
OUT=""
LOGOUT_PATH=""
LOGIN_CHALLENGE=""
SEED_JAR=""

while [ $# -gt 0 ]; do
  case "$1" in
    --cookie-jar)
      if [ -n "$SEED_JAR" ] || [ $# -lt 2 ] || [ -z "$2" ] || [[ "$2" == --* ]]; then
        echo "error: --cookie-jar requires one session-file path" >&2
        exit 2
      fi
      SEED_JAR="$2"
      shift 2
      ;;
    --login-challenge)
      if [ -n "$LOGIN_CHALLENGE" ] || [ $# -lt 2 ] || [ -z "$2" ] || [[ "$2" == --* ]]; then
        echo "error: --login-challenge requires one nonempty challenge" >&2
        exit 2
      fi
      LOGIN_CHALLENGE="$2"
      shift 2
      ;;
    --logout)
      MODE=logout
      shift
      if [ $# -gt 0 ] && [[ "$1" != --* ]]; then
        LOGOUT_PATH="$1"
        shift
      fi
      ;;
    --serve)
      SERVE=1
      shift
      ;;
    -h | --help)
      sed -n '2,/^set -euo pipefail/{ /^set -euo pipefail/d; s/^# \{0,1\}//; p; }' "$0"
      exit 0
      ;;
    -*)
      echo "error: unknown flag $1 (try --help)" >&2
      exit 2
      ;;
    *)
      if [ -n "$OUT" ]; then
        echo "error: unexpected argument $1" >&2
        exit 2
      fi
      OUT="$1"
      shift
      ;;
  esac
done

OUT="${OUT:-$DEFAULT_OUT}"
if [ "$MODE" = logout ] && { [ -n "$LOGIN_CHALLENGE" ] || [ -n "$SEED_JAR" ]; }; then
  echo "error: --login-challenge/--cookie-jar cannot be used with --logout" >&2
  exit 2
fi

load_qa_env() {
  # Precedence: environment, then .env, then muse.env (both gitignored by *.env).
  local f
  for f in .env muse.env; do
    [ -n "${QA_EMAIL:-}" ] && [ -n "${QA_PASSWORD:-}" ] && break
    [ -f "$f" ] || continue
    set -a
    # shellcheck disable=SC1090
    . "./$f"
    set +a
  done
  [ -n "${QA_EMAIL:-}" ] && [ -n "${QA_PASSWORD:-}" ] || {
    echo "error: QA_EMAIL/QA_PASSWORD are unset or empty — fill them in .env or muse.env (names live in .env.example)" >&2
    exit 2
  }
  export QA_EMAIL QA_PASSWORD
}

# Playwright storage-state JSON -> Netscape cookie jar (curl -c/-b format).
state_to_jar() {
  python3 -c '
import json, sys
state = json.load(open(sys.argv[1]))
cookies = state.get("cookies") or []
out = open(sys.argv[2], "w")
out.write("# Netscape HTTP Cookie File\n")
for c in cookies:
    domain = c.get("domain") or ""
    flag = "TRUE" if domain.startswith(".") else "FALSE"
    path = c.get("path") or "/"
    secure = "TRUE" if c.get("secure") else "FALSE"
    expires = c.get("expires", -1)
    try:
        expires_i = int(expires)
    except (TypeError, ValueError):
        expires_i = 0
    if expires_i < 0:
        expires_i = 0
    name = c.get("name") or ""
    value = c.get("value") or ""
    if c.get("httpOnly"):
        out.write("#HttpOnly_")
    out.write("\t".join([domain, flag, path, secure, str(expires_i), name, value]) + "\n")
' "$1" "$2"
}

# Resolve PATH to a Netscape jar in $2. Accepts Playwright storage-state JSON
# or an existing Netscape jar.
resolve_jar() {
  local src="$1" dest="$2"
  if [ ! -f "$src" ]; then
    echo "error: no session file at $src — pass a storage-state JSON or cookie jar" >&2
    exit 1
  fi
  if python3 -c 'import json,sys; json.load(open(sys.argv[1])); sys.exit(0)' "$src" 2>/dev/null; then
    state_to_jar "$src" "$dest"
  else
    cp "$src" "$dest"
  fi
}

# Revoke the Kratos session carried by a Netscape jar via the browser logout
# flow (same shape dashboard/src/common/lib/ory/logout.ts uses).
logout_jar() {
  local jar="$1"
  local tmp_logout
  tmp_logout="$(mktemp -d)"
  # shellcheck disable=SC2064
  trap "rm -rf '$tmp_logout'" RETURN

  local code
  code="$(qa_curl -sS -o /dev/null -w '%{http_code}' -b "$jar" "$KRATOS_PUB/sessions/whoami" || true)"
  if [ "$code" = "401" ] || [ "$code" = "403" ]; then
    echo "ok already-signed-out"
    return 0
  fi
  if [ "$code" != "200" ]; then
    echo "error: whoami returned HTTP $code — cannot logout" >&2
    return 1
  fi

  qa_curl -fsS -c "$jar" -b "$jar" -H 'Accept: application/json' \
    "$KRATOS_PUB/self-service/logout/browser?return_to=$DASH/" >"$tmp_logout/flow.json" || {
    echo "error: no logout flow from $KRATOS_PUB" >&2
    return 1
  }
  local logout_url
  logout_url="$(python3 -c "
import json, sys
f = json.load(open(sys.argv[1]))
url = f.get('logout_url') or ''
if not url:
    token = f.get('logout_token') or ''
    if not token:
        sys.exit('error: logout flow missing logout_url/logout_token')
    url = sys.argv[2].rstrip('/') + '/self-service/logout?token=' + token
print(url)
" "$tmp_logout/flow.json" "$KRATOS_PUB")" || return 1

  # Kratos may 303 to return_to (dashboard); follow redirects, ignore body.
  # Ground truth is whoami afterwards (see endBrowserSession).
  qa_curl -sS -o /dev/null -c "$jar" -b "$jar" -L "$logout_url" || true

  code="$(qa_curl -sS -o /dev/null -w '%{http_code}' -b "$jar" "$KRATOS_PUB/sessions/whoami" || true)"
  if [ "$code" = "401" ] || [ "$code" = "403" ]; then
    echo "ok logged-out"
    return 0
  fi
  echo "error: logout completed but whoami still returns HTTP $code" >&2
  return 1
}

if [ "$MODE" = logout ]; then
  if [ -z "$LOGOUT_PATH" ]; then
    echo "error: --logout requires the path your login printed on its logout line" >&2
    exit 2
  fi
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  resolve_jar "$LOGOUT_PATH" "$tmp/jar"
  logout_jar "$tmp/jar"
  exit 0
fi

load_qa_env

tmp="$(mktemp -d)"
jar="$tmp/jar"
seeded_session=0

has_session_cookie() {
  python3 -c '
import sys
try:
    for line in open(sys.argv[1]):
        parts = line.rstrip("\n").split("\t")
        if len(parts) == 7 and parts[5].startswith("ory_kratos_session") and parts[6]:
            sys.exit(0)
except OSError:
    pass
sys.exit(1)
' "$1"
}

cleanup_login() {
  local status=$?
  trap - EXIT
  # A failed challenge may already have created a session. Revoke that session
  # before discarding its only logout handle, while leaving seeded sessions alone.
  if [ "$status" -ne 0 ] && [ -n "$LOGIN_CHALLENGE" ] && [ "$seeded_session" = 0 ] && has_session_cookie "$jar"; then
    if logout_jar "$jar" >"$tmp/cleanup-result" 2>/dev/null; then
      echo "error: revoked the temporary QA session after failed OAuth completion" >&2
    else
      echo "error: could not revoke the temporary QA session after failed OAuth completion" >&2
    fi
  fi
  rm -rf "$tmp"
  exit "$status"
}
trap cleanup_login EXIT
if [ -n "$SEED_JAR" ]; then
  # Keep Hydra's authorization CSRF cookie across the device flow. Work on a
  # private copy so a failed challenge does not alter the caller's session.
  resolve_jar "$SEED_JAR" "$jar"
  if has_session_cookie "$jar"; then
    seeded_session=1
  fi
fi

# Passwords are only ever submitted to Kratos. Provider continuations are GETs
# on these three configured origins; never use curl -L on the password POST
# (a 307/308 would resend its body to the redirect target).
login_url() {
  python3 -c '
import sys
from urllib.parse import urljoin, urlsplit
candidate, base, kind, *trusted = sys.argv[1:]
url = urljoin(base, candidate)
def origin(value):
    p = urlsplit(value)
    if (p.scheme not in ("https", "http") or not p.hostname or
            p.username is not None or p.password is not None or
            any(c.isspace() or ord(c) < 32 for c in value)):
        raise ValueError()
    return p.scheme, p.hostname.lower(), p.port or (443 if p.scheme == "https" else 80)
try:
    allowed = [base] if kind == "action" else trusted
    if origin(url) not in [origin(u) for u in allowed]:
        raise ValueError()
except (ValueError, TypeError):
    sys.exit("error: refusing an untrusted login " + kind + " URL")
print(url)
' "$1" "$2" "$3" "$KRATOS_PUB" "$DASH" "$OAUTH_PUB"
}

# Read either a normal HTTP redirect or Kratos's JSON browser-navigation
# response. URLs and response bodies stay private, including on refusal.
login_redirect() {
  python3 -c '
import json, sys
from email.parser import Parser
headers = open(sys.argv[1]).read().replace("\r\n", "\n")
for block in reversed(headers.strip().split("\n\n")):
    if block.startswith("HTTP/"):
        location = Parser().parsestr(block.partition("\n")[2]).get("Location")
        if location and sys.argv[3] in ("301", "302", "303", "307", "308"):
            print(location)
            sys.exit(0)
        break
try:
    body = json.load(open(sys.argv[2]))
except (ValueError, OSError):
    body = None
if isinstance(body, dict) and sys.argv[3] in ("200", "201", "422"):
    target = body.get("redirect_browser_to")
    if not target:
        target = next((c.get("redirect_browser_to") for c in body.get("continue_with", [])
                       if isinstance(c, dict) and c.get("action") == "redirect_browser_to"), None)
    if isinstance(target, str):
        print(target)
' "$1" "$2" "$3"
}

describe_login_endpoint() {
  python3 -c '
import sys
from urllib.parse import parse_qsl, urlsplit
p = urlsplit(sys.argv[1])
origins = {urlsplit(value).netloc: label for value, label in zip(sys.argv[2:], ("Kratos", "dashboard", "OAuth"))}
paths = {"/auth/device/success", "/auth/device", "/auth/consent", "/auth/login",
         "/oauth2/device/verify", "/oauth2/auth", "/self-service/login/browser", "/self-service/login"}
names = {"login_challenge", "login_verifier", "consent_challenge", "consent_verifier",
         "device_challenge", "device_error", "user_code", "flow", "error", "error_description",
         "state", "code", "scope", "client_id", "redirect_uri", "response_type", "return_to"}
keys = sorted({key if key in names else "other" for key, _ in parse_qsl(p.query, keep_blank_values=True)})
print("endpoint " + origins.get(p.netloc, "unexpected origin") + " " +
      (p.path if p.path in paths else "unexpected path") + "; query keys: " + (", ".join(keys) or "none"))
' "$1" "$KRATOS_PUB" "$DASH" "$OAUTH_PUB"
}

complete_device_login() {
  local current="$1" code="$2" headers="$3" body="$4" target
  for _ in $(seq 1 20); do
    target="$(login_redirect "$headers" "$body" "$code")"
    if [ -z "$target" ]; then
      # A live Kratos session alone does not prove that OAuth consent finished.
      # Refuse login/consent/error HTML instead of reporting a false success.
      if [[ "$code" == 2?? ]] && python3 -c '
import sys
from urllib.parse import parse_qsl, urlsplit
actual, dash = (urlsplit(v) for v in sys.argv[1:])
query = parse_qsl(actual.query, keep_blank_values=True)
# Hydra adds client_id to its configured post-device URL after accepting the
# grant (oauth2/handler.go, DeviceDoneURL). No error or arbitrary query is valid.
safe_query = not query or query == [("client_id", "429024F5E608930E2A65EF92591A25CC")]
sys.exit(0 if (actual.scheme, actual.netloc) == (dash.scheme, dash.netloc) and
         actual.path == dash.path.rstrip("/") + "/auth/device/success" and
         safe_query and not actual.fragment else 1)
' "$current" "$DASH"; then
        return 0
      fi
      echo "error: OAuth login did not reach the device success page (HTTP $code; $(describe_login_endpoint "$current"))" >&2
      return 1
    fi
    # Redirecting a credential-bearing POST uses a fresh GET, even for 307/308.
    current="$(login_url "$target" "$current" redirect)" || return 1
    code="$(qa_curl -sS -o "$tmp/continue-body" -D "$tmp/continue-headers" \
      -w '%{http_code}' -c "$jar" -b "$jar" "$current")" || return 1
    headers="$tmp/continue-headers"
    body="$tmp/continue-body"
  done
  echo "error: OAuth login exceeded the redirect limit" >&2
  return 1
}

submit_password_login() {
  local code="$1" action csrf
  if [[ "$code" != 2?? ]]; then
    echo "error: no login flow from $KRATOS_PUB (HTTP $code)" >&2
    return 1
  fi
  action="$(python3 -c '
import json, sys
try:
    print(json.load(open(sys.argv[1]))["ui"]["action"])
except (ValueError, KeyError, TypeError):
    sys.exit("error: login flow is missing its form action")
' "$tmp/flow.json")"
  action="$(login_url "$action" "$KRATOS_PUB" action)"
  csrf="$(python3 -c "
import sys, json
f = json.load(open(sys.argv[1]))
print(next(n['attributes']['value'] for n in f['ui']['nodes'] if n['attributes'].get('name') == 'csrf_token'))
" "$tmp/flow.json")"

  # 2. Submit the password. Body is built in a file (mode 700 tmpdir) and fed to
  #    curl with @file, so neither the password nor the token reaches argv.
  QA_CSRF="$csrf" python3 -c "
import json, os, sys
json.dump({'method': 'password', 'identifier': os.environ['QA_EMAIL'],
           'password': os.environ['QA_PASSWORD'], 'csrf_token': os.environ['QA_CSRF']},
          open(sys.argv[1], 'w'))
" "$tmp/body.json"
  code="$(qa_curl -sS -o "$tmp/resp.json" -D "$tmp/resp-headers" -w '%{http_code}' -c "$jar" -b "$jar" \
    -H 'Accept: application/json' -H 'Content-Type: application/json' \
    --data-binary "@$tmp/body.json" "$action")"
  if [ -n "$LOGIN_CHALLENGE" ]; then
    complete_device_login "$action" "$code" "$tmp/resp-headers" "$tmp/resp.json"
  fi
}

# 1. Browser-shaped login flow: gives us the form action + the CSRF token, and
#    seeds the jar with Kratos's CSRF cookie.
bootstrap="$KRATOS_PUB/self-service/login/browser?return_to=$DASH/"
if [ -n "$LOGIN_CHALLENGE" ]; then
  bootstrap="$(python3 -c '
import sys
from urllib.parse import urlencode
print(sys.argv[1].rstrip("/") + "/self-service/login/browser?" +
      urlencode({"return_to": sys.argv[2].rstrip("/") + "/", "login_challenge": sys.argv[3]}))
' "$KRATOS_PUB" "$DASH" "$LOGIN_CHALLENGE")"
fi
code="$(qa_curl -sS -c "$jar" -b "$jar" -H 'Accept: application/json' \
  -D "$tmp/flow-headers" -o "$tmp/flow.json" -w '%{http_code}' "$bootstrap")" || {
  echo "error: no login flow from $KRATOS_PUB — is it reachable?" >&2
  exit 1
}

if [ -n "$LOGIN_CHALLENGE" ] && ! python3 -c '
import json, sys
try:
    f = json.load(open(sys.argv[1]))
    sys.exit(0 if isinstance(f, dict) and isinstance(f.get("ui"), dict) else 1)
except (ValueError, OSError):
    sys.exit(1)
' "$tmp/flow.json"; then
  # Kratos can satisfy a challenge without a form. Its JSON answer is `null`,
  # or `400 session_already_available` for a live session Hydra remembers (the
  # id may be top-level or under `error`). Either way a browser-shaped request
  # obtains the provider redirect, as the dashboard hands the challenge back
  # (use-ory-flow.ts, w8/124).
  if python3 -c '
import json, sys
try:
    f = json.load(open(sys.argv[1]))
except (ValueError, OSError):
    sys.exit(1)
error = f.get("error") if isinstance(f, dict) and isinstance(f.get("error"), dict) else f
sys.exit(0 if (sys.argv[2] == "200" and f is None) or (sys.argv[2] == "400" and
         isinstance(error, dict) and error.get("id") == "session_already_available") else 1)
' "$tmp/flow.json" "$code"; then
    code="$(qa_curl -sS -c "$jar" -b "$jar" -D "$tmp/flow-headers" \
      -o "$tmp/flow.json" -w '%{http_code}' "$bootstrap")"
  fi
  complete_device_login "$bootstrap" "$code" "$tmp/flow-headers" "$tmp/flow.json"
else
  submit_password_login "$code"
fi

# 3. The session is real only if Kratos says so.
if [ "$(qa_curl -sS -o /dev/null -w '%{http_code}' -b "$jar" "$KRATOS_PUB/sessions/whoami")" != "200" ]; then
  # A server may echo submitted fields in a failed flow. Never print that body.
  echo "error: login failed — no session established" >&2
  exit 1
fi

# 4. Netscape jar -> Playwright storage state (cookies only). --serve keeps it
#    in memory and hands it out exactly once over loopback so cookies stay out
#    of the agent transcript; otherwise it is written 0600 to OUT. Either way
#    this run's jar is installed at RUN_JAR for Phase 8 --logout.
jar_to_state() { # jar -> {"cookies":[…],"origins":[]} on stdout
  python3 -c "
import json, sys
cookies = []
for raw in open(sys.argv[1]):
    http_only = raw.startswith('#HttpOnly_')
    line = raw[len('#HttpOnly_'):] if http_only else raw
    if line.startswith('#') or not line.strip():
        continue
    parts = line.rstrip('\n').split('\t')
    if len(parts) != 7:
        continue
    domain, _flag, path, secure, expires, name, value = parts
    cookies.append({'name': name, 'value': value, 'domain': domain, 'path': path,
                    'expires': int(expires) if expires.isdigit() and int(expires) else -1,
                    'httpOnly': http_only, 'secure': secure.upper() == 'TRUE',
                    'sameSite': 'Lax'})
if not any(c['name'].startswith('ory_kratos_session') for c in cookies):
    sys.exit('error: login reported success but produced no ory_kratos_session cookie')
json.dump({'cookies': cookies, 'origins': []}, sys.stdout)
" "$1"
}

install_logout_jar() {
  mkdir -p "$(dirname "$RUN_JAR")"
  install -m 600 "$jar" "$RUN_JAR"
}

print_logout_handle() {
  echo "logout $(cd "$(dirname "$RUN_JAR")" && pwd)/$(basename "$RUN_JAR")"
}

if [ "$SERVE" = 1 ]; then
  # Hand the state out exactly once, on loopback, at an unguessable path, then
  # exit. This run's jar is the Phase 8 logout handle.
  state="$tmp/state.json"
  jar_to_state "$jar" >"$state"
  url_file="$(mktemp)"
  nohup python3 -c "
import http.server, secrets, sys, threading
data = open(sys.argv[1], 'rb').read()
token = secrets.token_urlsafe(16)
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != '/' + token + '.json':
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)
        threading.Thread(target=self.server.shutdown, daemon=True).start()
    def log_message(self, *a):
        pass
srv = http.server.HTTPServer(('127.0.0.1', 0), H)
print('ok http://127.0.0.1:%d/%s.json' % (srv.server_address[1], token), flush=True)
threading.Timer(${QA_SERVE_TTL:-300}, srv.shutdown).start()
srv.serve_forever()
" "$state" >"$url_file" 2>/dev/null &
  for _ in $(seq 1 100); do
    [ -s "$url_file" ] && break
    sleep 0.1
  done
  [ -s "$url_file" ] || {
    # No handle was printed, so nothing could revoke this session later.
    logout_jar "$jar" >/dev/null 2>&1 || true
    echo "error: one-shot session server did not start" >&2
    rm -f "$url_file"
    exit 1
  }
  install_logout_jar
  cat "$url_file"
  rm -f "$url_file"
  print_logout_handle
  exit 0
fi

mkdir -p "$(dirname "$OUT")"
jar_to_state "$jar" >"$tmp/state.json"
install -m 600 "$tmp/state.json" "$OUT"
install_logout_jar
echo "ok $(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"
print_logout_handle
