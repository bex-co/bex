#!/usr/bin/env bash
# End-to-end social-login check (docs/ADR012-auth.md § Social login, w4/003,
# w7/m465, w7/m468): drives "Sign in with Google", "Sign in with GitLab" and
# "Sign in with Bitbucket" through the SAME machinery prod uses — the Kratos
# `oidc` method, configured by a SECOND `--config` file merged over the main
# config, written by the very function scripts/auth-secrets.sh calls
# (scripts/lib/kratos-oidc-fragment.sh: all four providers, the claims→traits
# mappers inlined via `base64://`, and the workspace Google-login settings
# guard), on the Kratos version production runs.
#
#   Kratos login (method=oidc, provider=google) -> provider authorize ->
#   provider login -> callback -> Kratos exchanges the code, runs the mapper,
#   creates the identity + a first-party session -> /sessions/whoami returns
#   the mapped, verified email and the google sign-in -> the settings flow
#   asks the guard before saving an email change or an unlink
#
# The Google and GitLab providers are Dex (dexidp.io), a throwaway stand-in
# OIDC IdP registered under the provider ids `google` and `gitlab`
# (KRATOS_OIDC_GOOGLE_ISSUER, KRATOS_OIDC_GITLAB_ISSUER) — neither can redirect
# to localhost, and pinning the *mechanism* (not their servers) is what a local
# check can prove. Kratos's built-in `provider: google` and `provider: gitlab`
# differ only in where they fetch their endpoints and claims. Bitbucket is
# closer still: production's `bitbucket` IS a generic provider at Dex, so here
# Dex runs production's own config (dex_bitbucket_config, the static client
# Kratos signs in as and its callback) with only the bitbucket-cloud connector
# swapped for Dex's password database, at the issuer DEX_ISSUER_URL names.
# Each provider signs a different person up and back in. Every other
# moving part is real here: the two-file config merge, every provider button,
# the mapper marking a provider-verified email verified, the session's
# `oidc`/`google` authentication method (what bex-api's gate reads), the linked
# account's stored ID token (what the gate reads the Google email from), and
# the settings guard interrupting a profile change and an unlink, letting
# other changes through, and failing the flow closed when unreachable. The
# guard is a stub on the host standing in for bex-api's
# /internal/kratos/settings-guard (whose own decisions internal/settingsguard's
# tests pin).
#
# Kratos and Dex SHARE a network namespace (Dex joins `container:kratos`) so
# `localhost:5556` is Dex from the host, from inside the Kratos container (its
# OIDC back-channel), and in Dex's own issuer URL — one URL, no split-horizon.
# Kratos reaches the host's guard stub as host.docker.internal.
#
# Self-contained: Kratos and Dex run as throwaway Docker containers
# (in-memory). Requires: docker, curl, python3, yq (mikefarah v4).
#
# Usage: scripts/auth-oidc-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/kratos-oidc-fragment.sh
. scripts/lib/kratos-oidc-fragment.sh

KRATOS_IMAGE=oryd/kratos:v26.2.0 # production's version (chart 0.62.1)
KRATOS=bexoidc-e2e-kratos
DEX=bexoidc-e2e-dex
KRATOS_PUB=http://localhost:4433
KRATOS_ADM=http://localhost:4434
# Production's Dex, by the digest its manifest pins, from Docker Hub's copy
# of the same image (ghcr.io/dexidp/dex and dexidp/dex publish one index).
DEX_IMAGE="$(yq -r '.spec.template.spec.containers[0].image' deploy/gitops/charts/dex/deployment.yaml)"
DEX_IMAGE="dexidp/dex:${DEX_IMAGE#ghcr.io/dexidp/dex:}"
DEX_ISSUER=http://localhost:5556/dex
CLIENT_ID=kratos-oidc-e2e
CLIENT_SECRET=kratos-oidc-e2e-secret
DEX_KRATOS_SECRET=kratos-dex-e2e-secret
# One person per provider: a second provider signing the same email up would
# be Kratos's duplicate-identifier linking flow, not a sign-up.
USER_EMAIL=dev@bex.co
GITLAB_EMAIL=gitlab-dev@bex.co
BITBUCKET_EMAIL=bitbucket-dev@bex.co
USER_PASS=password
GUARD_PORT=4499
GUARD_TOKEN=e2e-guard-token
TMP="$(mktemp -d)"
GUARD_PID=""

cleanup() {
  docker rm -f "$DEX" "$KRATOS" >/dev/null 2>&1 || true
  if [ -n "$GUARD_PID" ]; then kill "$GUARD_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

wait_http() { # url [attempts]
  for _ in $(seq 1 "${2:-45}"); do
    curl -sf -o /dev/null "$1" && return 0
    sleep 1
  done
  echo "error: $1 never became ready" >&2
  return 1
}

echo "-> 1/6 kratos (in-memory) — main config + the SECOND oidc config file auth-secrets.sh writes..."
cat >"$TMP/identity.schema.json" <<'JSON'
{
  "$id": "https://schemas.bex.co/e2e.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Person",
  "type": "object",
  "properties": {
    "traits": {
      "type": "object",
      "properties": {
        "email": {
          "type": "string",
          "format": "email",
          "ory.sh/kratos": {
            "credentials": { "password": { "identifier": true } },
            "verification": { "via": "email" }
          }
        },
        "name": { "type": "string" }
      },
      "required": ["email"],
      "additionalProperties": false
    }
  }
}
JSON

cat >"$TMP/kratos.yaml" <<YAML
dsn: memory
serve:
  public:
    base_url: $KRATOS_PUB/
  admin:
    base_url: $KRATOS_ADM/
selfservice:
  default_browser_return_url: $KRATOS_PUB/
  allowed_return_urls: [$KRATOS_PUB]
  methods:
    password: { enabled: true }
    profile: { enabled: true }
  flows:
    login:
      ui_url: $KRATOS_PUB/login-stub
    registration:
      enabled: true
      ui_url: $KRATOS_PUB/registration-stub
      after:
        password:
          hooks: [{ hook: session }]
    settings:
      ui_url: $KRATOS_PUB/settings-stub
      privileged_session_max_age: 15m
    verification:
      enabled: true
      use: code
      ui_url: $KRATOS_PUB/verification-stub
    error:
      ui_url: $KRATOS_PUB/error-stub
identity:
  default_schema_id: default
  schemas: [{ id: default, url: file:///etc/config/kratos/identity.schema.json }]
secrets:
  cookie: [e2e-only-cookie-secret-32-chars-xx]
  cipher: [e2e-only-cipher-secret-32-chars!]
YAML

# The SECOND config file, written by the same function as scripts/auth-secrets.sh
# with Dex standing in for Google, GitLab and Bitbucket's Dex, and the guard
# pointed at the host stub. The GitHub pair is set too, so the login flow must
# carry all four provider buttons. The Bitbucket pair is Dex's (unused here:
# its connector is swapped out below).
fragment() { # extra NAME=value assignments... → the fragment on stdout
  (
    export BEX_GOOGLE_OIDC_CLIENT_ID="$CLIENT_ID" BEX_GOOGLE_OIDC_CLIENT_SECRET="$CLIENT_SECRET" \
      BEX_GITHUB_OIDC_CLIENT_ID=github-unused BEX_GITHUB_OIDC_CLIENT_SECRET=github-unused \
      KRATOS_OIDC_GOOGLE_ISSUER="$DEX_ISSUER" \
      KRATOS_SETTINGS_GUARD_URL="http://host.docker.internal:$GUARD_PORT" KRATOS_SETTINGS_GUARD_TOKEN="$GUARD_TOKEN"
    [ $# -eq 0 ] || export "$@"
    kratos_oidc_fragment
  )
}
BRIDGE=(BEX_BITBUCKET_OIDC_CLIENT_ID=bitbucket-unused BEX_BITBUCKET_OIDC_CLIENT_SECRET=bitbucket-unused
  DEX_KRATOS_CLIENT_SECRET="$DEX_KRATOS_SECRET" DEX_ISSUER_URL="$DEX_ISSUER" KRATOS_PUBLIC_URL="$KRATOS_PUB")
fragment BEX_GITLAB_OIDC_CLIENT_ID="$CLIENT_ID" BEX_GITLAB_OIDC_CLIENT_SECRET="$CLIENT_SECRET" \
  KRATOS_OIDC_GITLAB_ISSUER="$DEX_ISSUER" "${BRIDGE[@]}" >"$TMP/oidc.yaml"

docker rm -f "$KRATOS" >/dev/null 2>&1 || true
# Kratos owns the shared netns and publishes its own ports AND Dex's :5556.
docker run -d --name "$KRATOS" \
  -p 4433:4433 -p 4434:4434 -p 5556:5556 \
  --add-host host.docker.internal:host-gateway \
  -v "$TMP:/etc/config/kratos" \
  "$KRATOS_IMAGE" serve --dev --watch-courier=false \
  -c /etc/config/kratos/kratos.yaml \
  -c /etc/config/kratos/oidc.yaml >/dev/null
wait_http "$KRATOS_PUB/health/ready"
echo "  ✓ kratos merged two --config files (oidc method enabled)"

echo "-> 2/6 dex (production's Bitbucket bridge config; stand-in Google and GitLab; shares the kratos netns)..."
mkdir -p "$TMP/dex"
# Production's Dex config (auth-secrets.sh writes the same function's output),
# with the bitbucket-cloud connector swapped for a password database holding
# one person per provider, and one more static client for the google and
# gitlab stand-ins. bcrypt("password"); test-only credentials, never secrets.
# shellcheck disable=SC2016 # the bcrypt hash's $ signs are literal
(export "${BRIDGE[@]}"; dex_bitbucket_config) | \
  CLIENT_ID="$CLIENT_ID" CLIENT_SECRET="$CLIENT_SECRET" KRATOS_PUB="$KRATOS_PUB" \
  USER_EMAIL="$USER_EMAIL" GITLAB_EMAIL="$GITLAB_EMAIL" BITBUCKET_EMAIL="$BITBUCKET_EMAIL" \
  yq '
    .connectors = [] |
    .enablePasswordDB = true |
    .staticPasswords = [
      {"email": strenv(USER_EMAIL), "username": "dev", "userID": "1c2f0e3a-4b5c-6d7e-8f90-abcdef012345"},
      {"email": strenv(GITLAB_EMAIL), "username": "gitlab-dev", "userID": "2c2f0e3a-4b5c-6d7e-8f90-abcdef012345"},
      {"email": strenv(BITBUCKET_EMAIL), "username": "bitbucket-dev", "userID": "3c2f0e3a-4b5c-6d7e-8f90-abcdef012345"}
    ] |
    .staticPasswords[].hash = "$2a$10$NEz7tWCzOz6fF3oboUCTsuH9OEy.fFH8mYmwUYibQorTiL6Qh.Owi" |
    .staticClients += [{"id": strenv(CLIENT_ID), "secret": strenv(CLIENT_SECRET), "name": "Kratos (e2e stand-ins)",
      "redirectURIs": [strenv(KRATOS_PUB) + "/self-service/methods/oidc/callback/google",
                       strenv(KRATOS_PUB) + "/self-service/methods/oidc/callback/gitlab"]}]
  ' >"$TMP/dex/config.yaml"
docker rm -f "$DEX" >/dev/null 2>&1 || true
docker run -d --name "$DEX" --network "container:$KRATOS" \
  -v "$TMP/dex:/etc/dex" --read-only --entrypoint dex \
  "$DEX_IMAGE" serve /etc/dex/config.yaml >/dev/null
wait_http "$DEX_ISSUER/.well-known/openid-configuration"
echo "  ✓ dex OIDC discovery is live at $DEX_ISSUER"

echo "-> 3/6 settings guard stub (stands in for bex-api's /internal/kratos/settings-guard)..."
# It answers what $TMP/guard-mode says ("refuse" or "allow") and logs each
# call, so the flow below can assert what Kratos sent and what it did with
# each answer.
cat >"$TMP/guard.py" <<'GUARD'
import http.server, json, os, sys
port, tmp = int(sys.argv[1]), sys.argv[2]
class Guard(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        with open(os.path.join(tmp, 'guard.log'), 'a') as log:
            log.write(json.dumps({'path': self.path, 'auth': self.headers.get('Authorization'), 'body': json.loads(body)}) + '\n')
        if open(os.path.join(tmp, 'guard-mode')).read().strip() == 'refuse':
            profile = 'method=profile' in self.path
            out = json.dumps({'messages': [{'instance_ptr': '#/traits/email' if profile else '#/', 'messages': [
                {'id': 4100001 if profile else 4100002, 'text': 'refused by the e2e guard', 'type': 'error',
                 'context': {'workspaces': ['Acme']}}]}]}).encode()
            self.send_response(403)
        else:
            out = b'{}'
            self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(out)))
        self.end_headers()
        self.wfile.write(out)
    def log_message(self, *args):
        pass
http.server.HTTPServer(('0.0.0.0', port), Guard).serve_forever()
GUARD
echo allow >"$TMP/guard-mode"
python3 -I "$TMP/guard.py" "$GUARD_PORT" "$TMP" &
GUARD_PID=$!
sleep 1
echo "  ✓ guard stub listening on :$GUARD_PORT"

echo "-> 4/6 drive the whole browser flow (one cookie jar end-to-end)..."
# The entire dance runs in ONE python process holding ONE cookie jar: Kratos's
# csrf + continuity cookies (both HttpOnly) and Dex's session cookie must all
# survive across the login-flow init, the oidc redirect, the provider login, and
# the callback. Splitting this across curl invocations drops the HttpOnly
# continuity cookie (curl's `#HttpOnly_` jar lines are comments to other
# parsers) and Kratos rejects the callback — so keep it in-process.
python3 - "$KRATOS_PUB" "$KRATOS_ADM" "$USER_EMAIL" "$USER_PASS" "$TMP" "$GUARD_TOKEN" "$GUARD_PID" "$GITLAB_EMAIL" "$BITBUCKET_EMAIL" <<'PY'
import sys, re, os, json, base64, signal, time, http.cookiejar, urllib.request, urllib.parse
kratos_pub, kratos_adm, email, password, tmp, guard_token, guard_pid, gitlab_email, bitbucket_email = sys.argv[1:10]

def browser():
    """A fresh browser: its own cookie jar."""
    op = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    op.addheaders = [('User-Agent', 'bex-oidc-e2e'), ('Accept', 'application/json')]
    return op

def get_json_with(op, url, data=None, ctype=None):
    req = urllib.request.Request(url, data=data)
    if ctype:
        req.add_header('Content-Type', ctype)
    try:
        with op.open(req) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, body

opener = browser()

def get_json(url, data=None, ctype=None):
    return get_json_with(opener, url, data, ctype)

# (1) Init a browser login flow; assert it offers a button per provider.
_, flow = get_json(f'{kratos_pub}/self-service/login/browser')
nodes = flow['ui']['nodes']
action = flow['ui']['action']
csrf = next(n['attributes']['value'] for n in nodes if n['attributes'].get('name') == 'csrf_token')
providers = sorted(n['attributes'].get('value') for n in nodes if n['attributes'].get('name') == 'provider')
assert providers == ['bitbucket', 'github', 'gitlab', 'google'], 'login flow provider nodes = %r, want all four' % providers
labels = sorted(n['meta']['label']['text'] for n in nodes if n['attributes'].get('name') == 'provider')
assert labels == ['Sign in with Bitbucket', 'Sign in with GitHub', 'Sign in with GitLab', 'Sign in with Google'], 'provider button labels = %r' % labels
print('  ✓ 4a login flow carries an oidc node per provider (bitbucket, github, gitlab, google) — the buttons')

# (2) Submit method=oidc. Kratos answers 422 browser_location_change_required
# whose redirect_browser_to is the stand-in Google's authorize endpoint.
body = json.dumps({'method': 'oidc', 'provider': 'google', 'csrf_token': csrf}).encode()
_, submit = get_json(action, data=body, ctype='application/json')
redir = submit.get('redirect_browser_to') if isinstance(submit, dict) else None
assert redir, 'oidc submit gave no redirect_browser_to: ' + json.dumps(submit)
print(f'  ✓ 4b Kratos -> provider authorize ({redir[:42]}...)')

# (3) Follow to Dex's local-login form, POST credentials. skipApprovalScreen
# sends the success redirect straight through /approval to the Kratos callback,
# which auto-follows — Kratos exchanges the code, runs the mapper, sets the
# session cookie. Terminal hop is Kratos's return url (its root, no route): a
# benign 404 = "completed"; landing on the error page = failure.
with opener.open(redir) as r:
    login_url, html = r.geturl(), r.read().decode()
m = re.search(r'action="([^"]+)"', html)
assert m, 'no form action on the Dex login page:\n' + html[:800]
form = urllib.parse.urljoin(login_url, m.group(1).replace('&amp;', '&'))
creds = urllib.parse.urlencode({'login': email, 'password': password}).encode()
try:
    with opener.open(urllib.request.Request(form, data=creds)) as r:
        final = r.geturl()
except urllib.error.HTTPError as e:
    final = e.url
if 'error-stub' in final:
    eid = urllib.parse.parse_qs(urllib.parse.urlparse(final).query).get('id', [''])[0]
    _, err = get_json(f'{kratos_pub}/self-service/errors?id={eid}')
    sys.exit('flow ended on the Kratos error page: ' + json.dumps(err)[:800])
print(f'  ✓ 4c provider login accepted; browser landed back on Kratos ({final[:42]}...)')

# (4) The payoff: a first-party Kratos session with the mapper-populated email,
# signed in with the google provider, the address verified because the
# provider said so.
code, who = get_json(f'{kratos_pub}/sessions/whoami')
assert code == 200 and who.get('active') is True, f'no active session: {code} {json.dumps(who)[:400]}'
got = who['identity']['traits']['email']
assert got == email, 'mapper did not set the email trait: ' + json.dumps(who['identity']['traits'])
methods = [(m.get('method'), m.get('provider')) for m in who.get('authentication_methods', [])]
assert ('oidc', 'google') in methods, 'session authentication_methods = %r, want oidc/google' % methods
verified = [a for a in who['identity'].get('verifiable_addresses', []) if a['value'] == email and a['verified']]
assert verified, 'the provider-verified email is not verified: ' + json.dumps(who['identity'].get('verifiable_addresses'))
print(f'  ✓ 4d /sessions/whoami: active session, email={got} (verified), signed in with oidc/google')

# (5) The linked account's stored ID token, which bex-api's gate reads the
# Google email from.
ident = who['identity']['id']
with urllib.request.urlopen(f'{kratos_adm}/admin/identities/{ident}?include_credential=oidc') as r:
    adm = json.loads(r.read().decode())
tokens = [p.get('initial_id_token', '') for p in adm['credentials']['oidc']['config']['providers'] if p['provider'] == 'google']
assert tokens and tokens[0].count('.') == 2, 'no stored google ID token'
payload = tokens[0].split('.')[1]
claims = json.loads(base64.urlsafe_b64decode(payload + '=' * (-len(payload) % 4)))
assert claims.get('email') == email, 'stored ID token email = %r' % claims.get('email')
print('  ✓ 4e the linked Google account stored its ID token, email', claims['email'])

# (6) Sign back in from a fresh browser: the same identity, not a second one.
def provider_login(op, provider, who):
    """Sign in (or up) through `provider` as the stand-in person `who`."""
    _, flow = get_json_with(op, f'{kratos_pub}/self-service/login/browser')
    csrf = next(n['attributes']['value'] for n in flow['ui']['nodes'] if n['attributes'].get('name') == 'csrf_token')
    _, submit = get_json_with(op, flow['ui']['action'], data=json.dumps({'method': 'oidc', 'provider': provider, 'csrf_token': csrf}).encode(), ctype='application/json')
    with op.open(submit['redirect_browser_to']) as r:
        login_url, html = r.geturl(), r.read().decode()
    form = urllib.parse.urljoin(login_url, re.search(r'action="([^"]+)"', html).group(1).replace('&amp;', '&'))
    try:
        op.open(urllib.request.Request(form, data=urllib.parse.urlencode({'login': who, 'password': password}).encode())).close()
    except urllib.error.HTTPError as e:
        if 'error-stub' in (e.url or ''):
            eid = urllib.parse.parse_qs(urllib.parse.urlparse(e.url).query).get('id', [''])[0]
            sys.exit(f'{provider}: flow ended on the Kratos error page: ' + json.dumps(get_json_with(op, f'{kratos_pub}/self-service/errors?id={eid}')[1])[:800])
    return get_json_with(op, f'{kratos_pub}/sessions/whoami')


code, again = provider_login(browser(), 'google', email)
assert code == 200 and again['identity']['id'] == ident, f'signing back in = {code}, identity {again.get("identity", {}).get("id")} want {ident}'
print('  ✓ 4f a fresh browser signed back in with Google as the same identity')

# (6b) GitLab and Bitbucket (w7/m468): each signs a new person up, the mapper
# sets and verifies the email, the session says which provider, and a fresh
# browser signs back in as the same identity. Bitbucket's leg runs through
# production's own Dex config: Kratos's static client, its secret and its
# callback.
for provider, who in (('gitlab', gitlab_email), ('bitbucket', bitbucket_email)):
    code, up = provider_login(browser(), provider, who)
    assert code == 200 and up.get('active') is True, f'{provider} sign-up = {code} {json.dumps(up)[:400]}'
    assert up['identity']['traits']['email'] == who, f'{provider}: email trait = %r' % up['identity']['traits']
    assert up['identity']['id'] != ident, f'{provider} signed up as the Google identity'
    methods = [(m.get('method'), m.get('provider')) for m in up.get('authentication_methods', [])]
    assert ('oidc', provider) in methods, f'{provider}: session authentication_methods = %r' % methods
    verified = [a for a in up['identity'].get('verifiable_addresses', []) if a['value'] == who and a['verified']]
    assert verified, f'{provider}: the provider-verified email is not verified: ' + json.dumps(up['identity'].get('verifiable_addresses'))
    linked = [p['provider'] for p in get_json(f'{kratos_adm}/admin/identities/{up["identity"]["id"]}?include_credential=oidc')[1]['credentials']['oidc']['config']['providers']]
    assert linked == [provider], f'{provider}: linked oidc providers = %r' % linked
    code, back = provider_login(browser(), provider, who)
    assert code == 200 and back['identity']['id'] == up['identity']['id'], f'{provider}: signing back in = {code}, identity {back.get("identity", {}).get("id")}'
    print(f'  ✓ 4g {provider}: signed {who} up (verified, oidc/{provider}) and a fresh browser back in as the same identity')

# (7) The settings guard: Kratos asks it before saving a profile change or an
# unlink; a refusal is shown and nothing changes; an allow saves; and a guard
# that cannot be reached fails the flow, closed.
def settings(body, mode):
    open(os.path.join(tmp, 'guard-mode'), 'w').write(mode)
    _, sf = get_json(f'{kratos_pub}/self-service/settings/browser')
    csrf = next(n['attributes']['value'] for n in sf['ui']['nodes'] if n['attributes'].get('name') == 'csrf_token')
    code, out = get_json(sf['ui']['action'], data=json.dumps(dict(body, csrf_token=csrf)).encode(), ctype='application/json')
    ids = []
    if isinstance(out, dict) and 'ui' in out:
        ids = [m['id'] for m in out['ui'].get('messages') or []]
        ids += [m['id'] for n in out['ui']['nodes'] for m in n.get('messages') or []]
    return code, ids

def current_email():
    return get_json(f'{kratos_pub}/sessions/whoami')[1]['identity']['traits']['email']

code, ids = settings({'method': 'profile', 'traits': {'email': 'personal@gmail.example'}}, 'refuse')
assert code == 400 and 4100001 in ids and current_email() == email, f'refused email change: {code} {ids} {current_email()}'
# Kratos itself never unlinks an identity's only credential, so give it a
# password first (the password method is not guarded).
code, _ = settings({'method': 'password', 'password': 'a-long-e2e-only-passphrase-42'}, 'allow')
assert code == 200, f'setting a password = {code}'
code, ids = settings({'method': 'oidc', 'unlink': 'google'}, 'refuse')
assert code == 400 and 4100002 in ids, f'refused unlink: {code} {ids}'
code, _ = settings({'method': 'profile', 'traits': {'email': email, 'name': 'Dev'}}, 'allow')
assert code == 200, f'an allowed profile change = {code}'
log = [json.loads(line) for line in open(os.path.join(tmp, 'guard.log'))]
assert [e['path'].split('method=')[1] for e in log] == ['profile', 'oidc', 'profile'], [e['path'] for e in log]
assert all(e['auth'] == 'Bearer ' + guard_token for e in log), 'the guard was not sent its bearer'
first = log[0]['body']
assert first['identity_id'] == ident and first['email'] == 'personal@gmail.example' and '/self-service/settings' in first['request_url'], first
print('  ✓ 4h the settings guard refused an email change and an unlink, and let a name change through')

os.kill(int(guard_pid), signal.SIGTERM)
time.sleep(1)
code, _ = settings({'method': 'profile', 'traits': {'email': 'personal@gmail.example', 'name': 'Dev'}}, 'allow')
assert code != 200 and current_email() == email, f'with the guard unreachable the change = {code}, email {current_email()}'
print(f'  ✓ 4i with the guard unreachable, Kratos refused the change ({code}): fail closed')
PY
GUARD_PID=""

echo "-> 5/6 confirm the minted identity is federated (oidc credential)..."
# A brand-new provider login registers the identity via the oidc strategy, so it
# carries an `oidc` credential (not password) — proof it came in through Dex.
ident="$(curl -sf "$KRATOS_ADM/admin/identities?credentials_identifier=$USER_EMAIL" || true)"
echo "$ident" | python3 -c "
import sys, json
arr = json.load(sys.stdin)
assert arr, 'no identity found for $USER_EMAIL'
methods = list(arr[0].get('credentials', {}).keys())
assert 'oidc' in methods, 'identity is missing the oidc credential: ' + json.dumps(methods)
print('  ✓ identity has an oidc (federated) credential:', methods)
"

echo "-> 6/6 without the GitLab and Bitbucket pairs, Kratos offers neither button..."
# The same fragment with only GitHub's and Google's pairs, on a fresh start of
# the same Kratos (memory DSN: nothing above survives, and Dex has lost its
# network namespace, which nothing here needs any more).
fragment >"$TMP/oidc.yaml"
docker restart "$KRATOS" >/dev/null
wait_http "$KRATOS_PUB/health/ready"
curl -sf -H 'Accept: application/json' "$KRATOS_PUB/self-service/login/browser" | python3 -c "
import sys, json
nodes = json.load(sys.stdin)['ui']['nodes']
providers = sorted(n['attributes'].get('value') for n in nodes if n['attributes'].get('name') == 'provider')
assert providers == ['github', 'google'], 'login flow provider nodes = %r, want github and google only' % providers
print('  ✓ the login flow offers github and google only')
"

echo
echo "✓ w4/003 + w7/m465 + w7/m468 end-to-end: the auth-secrets.sh fragment on Kratos v26.2.0"
echo "  -> Sign in with Google, GitLab and Bitbucket (Dex on production's bridge config)"
echo "  -> verified first-party sessions signed in with each provider, and back in"
echo "  -> the settings guard refuses an email change and an unlink, and fails closed"
echo "  when unreachable (docs/ADR012-auth.md §9b, §10)."
