#!/usr/bin/env bash
# Sign the QA user in and hand the Playwright MCP browser a live session —
# without QA_PASSWORD ever passing through the agent's context.
#
#   bash scripts/qa-login.sh [OUT]      write a storage-state file
#   bash scripts/qa-login.sh --serve    serve the state once over loopback
#
# Reads QA_EMAIL/QA_PASSWORD from .env (or the environment), completes the
# Kratos password login against $KRATOS_PUB, and writes OUT: a Playwright
# storage-state file (cookies only, mode 600) that /qa-find-bugs restores with
# the browser_set_storage_state MCP tool. Credentials go to curl on stdin, never
# in argv (ps leaks argv), and stdout is exactly "ok <path>" — no secret is ever
# printed. Consumed by .claude/skills/qa-find-bugs/SKILL.md.
#
# OUT defaults to .playwright-mcp/qa-storage-state.json (gitignored). Delete it
# when the hunt ends: it carries a live session cookie.
set -euo pipefail
cd "$(dirname "$0")/.."

SERVE=0
if [ "${1:-}" = "--serve" ]; then
  SERVE=1
  shift
fi
OUT="${1:-.playwright-mcp/qa-storage-state.json}"
KRATOS_PUB="${KRATOS_PUB:-https://auth.bex.co}"
DASH="${DASH:-https://dashboard.bex.co}"

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

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
jar="$tmp/jar"

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
#    in memory and hands it out exactly once over loopback, so the session
#    cookie never touches the disk or the agent's transcript; otherwise it is
#    written 0600 to OUT for the browser_set_storage_state MCP tool.
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

if [ "$SERVE" = 1 ]; then
  # Hand the state out exactly once, on loopback, at an unguessable path, then
  # exit. Nothing touches the disk, so the agent can inject the session with a
  # Playwright snippet that names only a 127.0.0.1 URL.
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
echo "ok $(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"
