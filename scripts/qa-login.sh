#!/usr/bin/env bash
# Sign the QA user in and hand the Playwright MCP browser a live session —
# without QA_PASSWORD ever passing through the agent's context.
#
#   bash scripts/qa-login.sh [OUT]           write a storage-state file
#   bash scripts/qa-login.sh --serve         serve the state once over loopback
#   bash scripts/qa-login.sh --logout [PATH] revoke the Kratos session in PATH
#
# Reads QA_EMAIL/QA_PASSWORD from .env (or the environment), completes the
# Kratos password login against $KRATOS_PUB, and writes OUT: a Playwright
# storage-state file (cookies only, mode 600) that /qa-find-bugs restores with
# the browser_set_storage_state MCP tool. Credentials go to curl on stdin, never
# in argv (ps leaks argv), and stdout is exactly "ok <path>" — no secret is ever
# printed. Consumed by .claude/skills/qa-find-bugs/SKILL.md.
#
# Every successful login also writes a 0600 Netscape cookie jar at
# .playwright-mcp/qa-session.jar (gitignored) so the hunt can revoke the session
# later without putting cookies in the agent transcript. The session must outlive
# this script (the browser uses it for the whole hunt), so revocation is NOT in
# the EXIT trap — callers must run --logout when the hunt ends (Phase 8).
#
# OUT defaults to .playwright-mcp/qa-storage-state.json. Delete OUT and the
# companion jar after --logout: they carried a live session cookie.
set -euo pipefail
cd "$(dirname "$0")/.."

KRATOS_PUB="${KRATOS_PUB:-https://auth.bex.co}"
DASH="${DASH:-https://dashboard.bex.co}"
DEFAULT_OUT=".playwright-mcp/qa-storage-state.json"
DEFAULT_JAR=".playwright-mcp/qa-session.jar"

MODE=login
SERVE=0
OUT=""
LOGOUT_PATH=""

while [ $# -gt 0 ]; do
  case "$1" in
    --logout)
      MODE=logout
      shift
      LOGOUT_PATH="${1:-}"
      [ -n "${1:-}" ] && shift || true
      ;;
    --serve)
      SERVE=1
      shift
      ;;
    -h | --help)
      sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
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

load_qa_env() {
  if [ -z "${QA_EMAIL:-}" ] || [ -z "${QA_PASSWORD:-}" ]; then
    [ -f .env ] || {
      echo "error: .env not found and QA_EMAIL/QA_PASSWORD are unset" >&2
      exit 2
    }
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
  fi
  [ -n "${QA_EMAIL:-}" ] && [ -n "${QA_PASSWORD:-}" ] || {
    echo "error: QA_EMAIL/QA_PASSWORD are empty — fill them in .env (names live in .env.example)" >&2
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
  code="$(curl -sS -o /dev/null -w '%{http_code}' -b "$jar" "$KRATOS_PUB/sessions/whoami" || true)"
  if [ "$code" = "401" ] || [ "$code" = "403" ]; then
    echo "ok already-signed-out"
    return 0
  fi
  if [ "$code" != "200" ]; then
    echo "error: whoami returned HTTP $code — cannot logout" >&2
    return 1
  fi

  curl -fsS -c "$jar" -b "$jar" -H 'Accept: application/json' \
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
  curl -sS -o /dev/null -c "$jar" -b "$jar" -L "$logout_url" || true

  code="$(curl -sS -o /dev/null -w '%{http_code}' -b "$jar" "$KRATOS_PUB/sessions/whoami" || true)"
  if [ "$code" = "401" ] || [ "$code" = "403" ]; then
    echo "ok logged-out"
    return 0
  fi
  echo "error: logout completed but whoami still returns HTTP $code" >&2
  return 1
}

if [ "$MODE" = logout ]; then
  if [ -z "$LOGOUT_PATH" ]; then
    if [ -f "$DEFAULT_OUT" ]; then
      LOGOUT_PATH="$DEFAULT_OUT"
    elif [ -f "$DEFAULT_JAR" ]; then
      LOGOUT_PATH="$DEFAULT_JAR"
    else
      echo "error: nothing to logout — expected $DEFAULT_OUT or $DEFAULT_JAR" >&2
      exit 1
    fi
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
trap 'rm -rf "$tmp"' EXIT

# 1. Browser-shaped login flow: gives us the form action + the CSRF token, and
#    seeds the jar with Kratos's CSRF cookie.
curl -fsS -c "$jar" -b "$jar" -H 'Accept: application/json' \
  "$KRATOS_PUB/self-service/login/browser?return_to=$DASH/" >"$tmp/flow.json" || {
  echo "error: no login flow from $KRATOS_PUB — is it reachable?" >&2
  exit 1
}
action="$(python3 -c "import sys,json;print(json.load(open(sys.argv[1]))['ui']['action'])" "$tmp/flow.json")"
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
curl -sS -o "$tmp/resp.json" -c "$jar" -b "$jar" \
  -H 'Accept: application/json' -H 'Content-Type: application/json' \
  --data-binary "@$tmp/body.json" "$action" >/dev/null

# 3. The session is real only if Kratos says so.
if [ "$(curl -sS -o /dev/null -w '%{http_code}' -b "$jar" "$KRATOS_PUB/sessions/whoami")" != "200" ]; then
  python3 -c "
import json, sys
try:
    f = json.load(open(sys.argv[1]))
except Exception:
    sys.exit('error: login failed and Kratos returned no JSON')
msgs = [m.get('text', '') for m in f.get('ui', {}).get('messages', [])]
for n in f.get('ui', {}).get('nodes', []):
    msgs += [m.get('text', '') for m in n.get('messages', [])]
sys.exit('error: login failed — ' + ('; '.join(t for t in msgs if t) or f.get('error', {}).get('reason', 'no session established')))
" "$tmp/resp.json" >&2
  exit 1
fi

# 4. Netscape jar -> Playwright storage state (cookies only). --serve keeps it
#    in memory and hands it out exactly once over loopback so cookies stay out
#    of the agent transcript; otherwise it is written 0600 to OUT. Either way a
#    companion jar is installed at DEFAULT_JAR for Phase 8 --logout.
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
  mkdir -p "$(dirname "$DEFAULT_JAR")"
  install -m 600 "$jar" "$DEFAULT_JAR"
}

if [ "$SERVE" = 1 ]; then
  # Hand the state out exactly once, on loopback, at an unguessable path, then
  # exit. The companion jar is the Phase 8 logout handle.
  state="$tmp/state.json"
  jar_to_state "$jar" >"$state"
  install_logout_jar
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
    echo "error: one-shot session server did not start" >&2
    rm -f "$url_file"
    exit 1
  }
  cat "$url_file"
  rm -f "$url_file"
  exit 0
fi

mkdir -p "$(dirname "$OUT")"
jar_to_state "$jar" >"$tmp/state.json"
install -m 600 "$tmp/state.json" "$OUT"
install_logout_jar
echo "ok $(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"
