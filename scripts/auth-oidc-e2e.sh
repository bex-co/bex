#!/usr/bin/env bash
# End-to-end social-login check (docs/ADR012-auth.md § Social login, w4/003): drives
# "Sign in with an OIDC provider" through the SAME machinery prod uses — the
# Kratos `oidc` method, configured by a SECOND `--config` file merged over the
# main config (exactly how scripts/auth-secrets.sh injects the GitHub provider
# out-of-band of git), with the claims→traits mapper inlined via `base64://`.
#
#   Kratos login (method=oidc) -> provider authorize -> provider login
#   -> callback -> Kratos exchanges the code, runs the mapper, creates the
#   identity + a first-party session -> /sessions/whoami returns the mapped email
#
# The provider is Dex (dexidp.io), a throwaway stand-in OIDC IdP — GitHub can't
# redirect to localhost, and pinning the *mechanism* (not GitHub's servers) is
# what a local check can prove. The GitHub-specific bits (`provider: github`,
# `scope: [user:email]`) are config-only and documented in docs/ADR012-auth.md; every
# other moving part — two-file config merge, the mapper, session issuance, the
# provider button Ory Elements renders from the flow's oidc node — is real here.
#
# Kratos and Dex SHARE a network namespace (Dex joins `container:kratos`) so
# `localhost:5556` is Dex from the host, from inside the Kratos container (its
# OIDC back-channel), and in Dex's own issuer URL — one URL, no split-horizon.
#
# Self-contained: both run as throwaway Docker containers (in-memory). Requires:
# docker, curl, python3.
#
# Usage: scripts/auth-oidc-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."

KRATOS=bexoidc-e2e-kratos
DEX=bexoidc-e2e-dex
KRATOS_PUB=http://localhost:4433
KRATOS_ADM=http://localhost:4434
DEX_ISSUER=http://localhost:5556/dex
CLIENT_ID=kratos-oidc-e2e
CLIENT_SECRET=kratos-oidc-e2e-secret
USER_EMAIL=dev@bex.co
USER_PASS=password
TMP="$(mktemp -d)"

cleanup() {
  docker rm -f "$DEX" "$KRATOS" >/dev/null 2>&1 || true
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

echo "-> 1/4 kratos (in-memory) — main config + a SECOND oidc config file..."
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
          "ory.sh/kratos": { "credentials": { "password": { "identifier": true } } }
        }
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
  flows:
    login:
      ui_url: $KRATOS_PUB/login-stub
    registration:
      enabled: true
      ui_url: $KRATOS_PUB/registration-stub
      after:
        password:
          hooks: [{ hook: session }]
    error:
      ui_url: $KRATOS_PUB/error-stub
identity:
  default_schema_id: default
  schemas: [{ id: default, url: file:///etc/config/kratos/identity.schema.json }]
secrets:
  cookie: [e2e-only-cookie-secret-32-chars-xx]
  cipher: [e2e-only-cipher-secret-32-chars!]
YAML

# The SECOND config file — byte-for-byte the shape scripts/auth-secrets.sh writes
# into the `kratos` Secret's oidc.yaml key, only with a `generic` Dex provider in
# place of `github`. The mapper is inlined via base64:// (no extra file mount).
MAPPER="$(printf '%s' \
  'local claims = std.extVar('"'"'claims'"'"'); { identity: { traits: { email: claims.email } } }' \
  | base64 | tr -d '\n')"
cat >"$TMP/oidc.yaml" <<YAML
selfservice:
  methods:
    oidc:
      enabled: true
      config:
        providers:
          - id: dex
            provider: generic
            issuer_url: $DEX_ISSUER
            client_id: $CLIENT_ID
            client_secret: $CLIENT_SECRET
            mapper_url: "base64://$MAPPER"
            scope:
              - openid
              - email
  flows:
    registration:
      after:
        oidc:
          hooks:
            - hook: session
YAML

docker rm -f "$KRATOS" >/dev/null 2>&1 || true
# Kratos owns the shared netns and publishes its own ports AND Dex's :5556.
docker run -d --name "$KRATOS" \
  -p 4433:4433 -p 4434:4434 -p 5556:5556 \
  -v "$TMP:/etc/config/kratos" \
  oryd/kratos:v1.3.1 serve --dev --watch-courier=false \
  -c /etc/config/kratos/kratos.yaml \
  -c /etc/config/kratos/oidc.yaml >/dev/null
wait_http "$KRATOS_PUB/health/ready"
echo "  ✓ kratos merged two --config files (oidc method enabled)"

echo "-> 2/4 dex (stand-in OIDC IdP, shares the kratos netns)..."
mkdir -p "$TMP/dex"
cat >"$TMP/dex/config.yaml" <<YAML
issuer: $DEX_ISSUER
storage:
  type: memory
web:
  http: 0.0.0.0:5556
oauth2:
  skipApprovalScreen: true
staticClients:
  - id: $CLIENT_ID
    secret: $CLIENT_SECRET
    name: Kratos (e2e)
    redirectURIs:
      - $KRATOS_PUB/self-service/methods/oidc/callback/dex
enablePasswordDB: true
staticPasswords:
  - email: "$USER_EMAIL"
    # bcrypt("password"); test-only credential, never a secret.
    hash: "\$2a\$10\$NEz7tWCzOz6fF3oboUCTsuH9OEy.fFH8mYmwUYibQorTiL6Qh.Owi"
    username: "dev"
    userID: "1c2f0e3a-4b5c-6d7e-8f90-abcdef012345"
YAML
docker rm -f "$DEX" >/dev/null 2>&1 || true
docker run -d --name "$DEX" --network "container:$KRATOS" \
  -v "$TMP/dex:/etc/dex" \
  dexidp/dex:v2.41.1 dex serve /etc/dex/config.yaml >/dev/null
wait_http "$DEX_ISSUER/.well-known/openid-configuration"
echo "  ✓ dex OIDC discovery is live at $DEX_ISSUER"

echo "-> 3/4 drive the whole browser flow (one cookie jar end-to-end)..."
# The entire dance runs in ONE python process holding ONE cookie jar: Kratos's
# csrf + continuity cookies (both HttpOnly) and Dex's session cookie must all
# survive across the login-flow init, the oidc redirect, the provider login, and
# the callback. Splitting this across curl invocations drops the HttpOnly
# continuity cookie (curl's `#HttpOnly_` jar lines are comments to other
# parsers) and Kratos rejects the callback — so keep it in-process.
python3 - "$KRATOS_PUB" "$USER_EMAIL" "$USER_PASS" <<'PY'
import sys, re, json, http.cookiejar, urllib.request, urllib.parse
kratos_pub, email, password = sys.argv[1:4]

jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
opener.addheaders = [('User-Agent', 'bex-oidc-e2e'), ('Accept', 'application/json')]

def get_json(url, data=None, ctype=None):
    req = urllib.request.Request(url, data=data)
    if ctype:
        req.add_header('Content-Type', ctype)
    try:
        with opener.open(req) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, body

# (1) Init a browser login flow; assert it offers the provider button.
_, flow = get_json(f'{kratos_pub}/self-service/login/browser')
nodes = flow['ui']['nodes']
action = flow['ui']['action']
csrf = next(n['attributes']['value'] for n in nodes if n['attributes'].get('name') == 'csrf_token')
btn = [n for n in nodes
       if n['attributes'].get('name') == 'provider' and n['attributes'].get('value') == 'dex']
assert btn, 'login flow has no oidc provider=dex node: ' + json.dumps(nodes)
print('  ✓ 3a login flow carries an oidc node (provider=dex) — the provider button')

# (2) Submit method=oidc. Kratos answers 422 browser_location_change_required
# whose redirect_browser_to is Dex's authorize endpoint.
body = json.dumps({'method': 'oidc', 'provider': 'dex', 'csrf_token': csrf}).encode()
_, submit = get_json(action, data=body, ctype='application/json')
redir = submit.get('redirect_browser_to') if isinstance(submit, dict) else None
assert redir, 'oidc submit gave no redirect_browser_to: ' + json.dumps(submit)
print(f'  ✓ 3b Kratos -> provider authorize ({redir[:42]}...)')

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
print(f'  ✓ 3c provider login accepted; browser landed back on Kratos ({final[:42]}...)')

# (4) The payoff: a first-party Kratos session with the mapper-populated email.
code, who = get_json(f'{kratos_pub}/sessions/whoami')
assert code == 200 and who.get('active') is True, f'no active session: {code} {json.dumps(who)[:400]}'
got = who['identity']['traits']['email']
assert got == email, 'mapper did not set the email trait: ' + json.dumps(who['identity']['traits'])
print(f'  ✓ 3d /sessions/whoami: active session, email={got}, aal={who["authenticator_assurance_level"]}')
PY

echo "-> 4/4 confirm the minted identity is federated (oidc credential)..."
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

echo
echo "✓ w4/003 end-to-end: Kratos oidc method (two-file config merge + base64"
echo "  mapper) -> provider login -> callback -> first-party session. The GitHub"
echo "  provider is the same wiring with provider:github (docs/ADR012-auth.md)."
