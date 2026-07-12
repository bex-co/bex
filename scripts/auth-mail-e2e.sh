#!/usr/bin/env bash
# End-to-end email-flow check (docs/ADR012-auth.md §11, w4/m7): proves Kratos's courier
# actually delivers, so account recovery + address verification work start to
# finish — the flows the dashboard advertises but that dead-ended with no SMTP.
#
# It stands up the SAME machinery prod runs — a real Kratos with the courier
# enabled and `code` recovery/verification (exactly the base kratos.values.yaml
# shape) — plus a Mailpit catcher in place of SendGrid, then drives the flows
# over Kratos's public API and reads the delivered mail back through Mailpit's
# HTTP API:
#
#   RECOVERY:     admin-create identity → submit recovery → pull the code from
#                 Mailpit → complete it → change the password in the privileged
#                 session → the NEW password logs in and the OLD one is rejected.
#   VERIFICATION: register a fresh identity → the verification mail arrives →
#                 submit its code → the identity's address flips to verified.
#   NEGATIVE:     a garbage recovery code is rejected (no session); recovery for
#                 an unknown email returns the same "sent" state but sends NO mail
#                 (anti-enumeration) — proving the flow never leaks who exists.
#
# Same self-contained-Docker shape as scripts/auth-oidc-e2e.sh (in-memory Kratos,
# throwaway Mailpit) so it runs in CI without the CAPD/Argo/CNPG substrate — and
# so it sidesteps the mock-cluster pool-label precondition entirely. The cluster
# Mailpit (deploy/gitops/overlays/local) is the same image; the dashboard's
# forgot-password → reset-password click-through stays a manual browser check (t005).
#
# Self-contained: Kratos + Mailpit as throwaway Docker containers. Requires:
# docker, curl, python3.
#
# Usage: scripts/auth-mail-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."

KRATOS=bexmail-e2e-kratos
MAILPIT=bexmail-e2e-mailpit
KRATOS_PUB=http://localhost:4433
KRATOS_ADM=http://localhost:4434
MAILPIT_API=http://localhost:8025
TMP="$(mktemp -d)"

cleanup() {
  docker rm -f "$MAILPIT" "$KRATOS" >/dev/null 2>&1 || true
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

echo "-> 1/5 mailpit (SMTP :1025 + HTTP API :8025), kratos joins its netns..."
docker rm -f "$MAILPIT" >/dev/null 2>&1 || true
# Mailpit owns the shared netns and publishes BOTH its own HTTP API (:8025) and
# Kratos's public/admin ports (:4433/:4434) — so `localhost:1025` is Mailpit from
# the host, from inside the Mailpit container, and (crucially) from Kratos's
# courier, which shares this namespace. MP_SMTP_AUTH_ACCEPT_ANY lets the courier
# connect with or without credentials over plaintext (no TLS locally).
docker run -d --name "$MAILPIT" \
  -p 8025:8025 -p 4433:4433 -p 4434:4434 \
  -e MP_SMTP_AUTH_ACCEPT_ANY=1 -e MP_SMTP_AUTH_ALLOW_INSECURE=1 \
  axllent/mailpit:v1.21.8 >/dev/null
wait_http "$MAILPIT_API/readyz"
echo "  ✓ mailpit up"

echo "-> 2/5 kratos (in-memory) — courier enabled, code recovery + verification..."
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
            "verification": { "via": "email" },
            "recovery": { "via": "email" }
          }
        }
      },
      "required": ["email"],
      "additionalProperties": false
    }
  }
}
JSON

# The courier + code recovery/verification wiring mirrors base kratos.values.yaml
# (docs/ADR012-auth.md §11). connection_uri points at Mailpit over the shared netns;
# disable_starttls is the plaintext-relay switch (Mailpit offers no TLS locally —
# the prod SendGrid URI is smtps:// instead).
cat >"$TMP/kratos.yaml" <<YAML
dsn: memory
serve:
  public:
    base_url: $KRATOS_PUB/
  admin:
    base_url: $KRATOS_ADM/
courier:
  smtp:
    connection_uri: smtp://localhost:1025/?disable_starttls=true
    from_address: no-reply@bex.co
    from_name: bex
selfservice:
  default_browser_return_url: $KRATOS_PUB/
  allowed_return_urls: [$KRATOS_PUB]
  methods:
    password: { enabled: true }
    code: { enabled: true }
  flows:
    login:
      ui_url: $KRATOS_PUB/login-stub
    registration:
      enabled: true
      ui_url: $KRATOS_PUB/registration-stub
      after:
        password:
          hooks: [{ hook: session }]
    recovery:
      enabled: true
      use: code
      ui_url: $KRATOS_PUB/recovery-stub
    verification:
      enabled: true
      use: code
      ui_url: $KRATOS_PUB/verification-stub
    settings:
      ui_url: $KRATOS_PUB/settings-stub
    error:
      ui_url: $KRATOS_PUB/error-stub
identity:
  default_schema_id: default
  schemas: [{ id: default, url: file:///etc/config/kratos/identity.schema.json }]
secrets:
  cookie: [e2e-only-cookie-secret-32-chars-xx]
  cipher: [e2e-only-cipher-secret-32-chars!]
YAML

docker rm -f "$KRATOS" >/dev/null 2>&1 || true
# --watch-courier makes `serve` ALSO run the courier dispatch loop in-process, so
# no separate `kratos courier watch` container is needed for the test.
docker run -d --name "$KRATOS" --network "container:$MAILPIT" \
  -v "$TMP:/etc/config/kratos" \
  oryd/kratos:v1.3.1 serve --dev --watch-courier \
  -c /etc/config/kratos/kratos.yaml >/dev/null
wait_http "$KRATOS_PUB/health/ready"
echo "  ✓ kratos up with the courier watching"

echo "-> 3/5 recovery: reset a forgotten password end-to-end via the emailed code..."
echo "-> 4/5 verification: a fresh signup's address is confirmed via its emailed code..."
echo "-> 5/5 negatives: bad code rejected; unknown email sends no mail (anti-enum)..."
python3 - "$KRATOS_PUB" "$KRATOS_ADM" "$MAILPIT_API" <<'PY'
import http.cookiejar, json, re, sys, time, urllib.error, urllib.parse, urllib.request

kratos_pub, kratos_adm, mailpit = sys.argv[1:4]

# One cookie jar for the whole run: browser recovery/verification flows set the
# csrf + continuity cookies (and, on recovery success, the privileged session
# cookie) that the settings handoff needs — exactly how the dashboard's browser
# rides these flows. API login calls simply ignore the jar.
_jar = http.cookiejar.CookieJar()
_opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(_jar))
# A cookie-FREE opener for the login checks: a stale session cookie in the jar
# would short-circuit an API login to session_already_available and mask the
# password check.
_nocookie = urllib.request.build_opener()

def _parse(raw):
    if not raw:
        return {}
    try:
        return json.loads(raw)
    except Exception:
        return raw

def req(method, url, body=None, headers=None, opener=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header('Accept', 'application/json')
    if data is not None:
        r.add_header('Content-Type', 'application/json')
    for k, v in (headers or {}).items():
        r.add_header(k, v)
    try:
        with (opener or _opener).open(r) as resp:
            return resp.status, _parse(resp.read().decode())
    except urllib.error.HTTPError as e:
        return e.code, _parse(e.read().decode())

def fail(msg):
    sys.exit('  ✗ ' + msg)

def action_of(flow):
    return flow['ui']['action']

def csrf_of(flow):
    for n in flow.get('ui', {}).get('nodes', []):
        a = n.get('attributes', {})
        if a.get('name') == 'csrf_token':
            return a.get('value')
    return None

# --- Mailpit helpers -------------------------------------------------------
def mp_clear():
    req('DELETE', f'{mailpit}/api/v1/messages')

def mp_messages_to(addr):
    _, body = req('GET', f'{mailpit}/api/v1/messages')
    out = []
    for m in body.get('messages', []):
        tos = [t.get('Address', '').lower() for t in m.get('To', [])]
        if addr.lower() in tos:
            out.append(m)
    return out

def mp_wait_code(addr, attempts=30):
    """Poll Mailpit for a message to addr; return (code, message_id)."""
    for _ in range(attempts):
        msgs = mp_messages_to(addr)
        if msgs:
            mid = msgs[0]['ID']
            _, full = req('GET', f'{mailpit}/api/v1/message/{mid}')
            text = (full.get('Text') or '') + ' ' + (full.get('Snippet') or '')
            m = re.search(r'\b(\d{6})\b', text)
            if m:
                return m.group(1), mid
        time.sleep(1)
    fail(f'no 6-digit code mail delivered to {addr} within {attempts}s')

def admin_create_identity(email, password):
    code, body = req('POST', f'{kratos_adm}/admin/identities', {
        'schema_id': 'default',
        'traits': {'email': email},
        'credentials': {'password': {'config': {'password': password}}},
    })
    if code not in (200, 201):
        fail(f'admin create identity failed: {code} {json.dumps(body)[:400]}')
    return body['id']

def password_login(email, password):
    """Log in over the API with the cookie-free opener. Returns (status, token)."""
    _, flow = req('GET', f'{kratos_pub}/self-service/login/api', opener=_nocookie)
    code, body = req('POST', action_of(flow), {
        'method': 'password', 'identifier': email, 'password': password,
    }, opener=_nocookie)
    tok = body.get('session_token') if isinstance(body, dict) else None
    return code, tok

def start_recovery(email):
    """Init a browser recovery flow and submit the email. Returns (flow, csrf,
    status, body); the emailed code (if any) is read from Mailpit by the caller."""
    _, flow = req('GET', f'{kratos_pub}/self-service/recovery/browser')
    csrf = csrf_of(flow)
    st, body = req('POST', action_of(flow),
                   {'method': 'code', 'email': email, 'csrf_token': csrf})
    return flow, csrf, st, body

# ===========================================================================
# RECOVERY — driven as a BROWSER flow (what the dashboard does): the code
# handoff to settings rides the privileged session cookie, not an API token.
# ===========================================================================
mp_clear()
_jar.clear()
rec_email = 'recover-me@bex.co'
old_pw, new_pw = 'OldPassw0rd!123', 'NewPassw0rd!456'
admin_create_identity(rec_email, old_pw)

# Sanity: the old password logs in before the reset.
st, _ = password_login(rec_email, old_pw)
if st != 200:
    fail(f'precondition: old password should log in, got {st}')

# Init a browser recovery flow, submit the email → Kratos mails a code.
flow, csrf, st, body = start_recovery(rec_email)
if st not in (200, 400):  # 200/400 both carry the "enter the code" state
    fail(f'recovery submit-email unexpected status {st}: {json.dumps(body)[:400]}')
code, _ = mp_wait_code(rec_email)
print(f'  ✓ recovery code delivered to Mailpit ({code[:2]}****)')

# Submit the code against the SAME flow → on success Kratos sets the privileged
# session cookie and answers 422 browser_location_change_required pointing at the
# settings flow (the mandatory password-change handoff).
st, body = req('POST', action_of(flow),
               {'method': 'code', 'code': code, 'csrf_token': csrf})
redirect = body.get('redirect_browser_to') if isinstance(body, dict) else None
if st != 422 or not redirect or 'settings' not in redirect:
    fail(f'valid code should hand off to the settings flow, got {st}: {json.dumps(body)[:500]}')
settings_flow_id = urllib.parse.parse_qs(urllib.parse.urlparse(redirect).query).get('flow', [None])[0]
if not settings_flow_id:
    fail(f'no settings flow id in the recovery redirect: {redirect}')
print('  ✓ valid code accepted → privileged session + settings handoff')

# Change the password inside that privileged session (cookie carried by the jar).
_, sflow = req('GET', f'{kratos_pub}/self-service/settings/flows?id={settings_flow_id}')
st, body = req('POST', action_of(sflow),
               {'method': 'password', 'password': new_pw, 'csrf_token': csrf_of(sflow)})
if st != 200:
    fail(f'password update in the recovery session failed: {st} {json.dumps(body)[:400]}')
print('  ✓ new password set in the privileged session')

# The payoff: new password logs in, old password is rejected.
st, tok = password_login(rec_email, new_pw)
if st != 200 or not tok:
    fail(f'new password should log in, got {st}')
st, _ = password_login(rec_email, old_pw)
if st == 200:
    fail('old password STILL logs in after reset — recovery did not rotate it')
print(f'  ✓ new password logs in; old password rejected ({st})')

# ===========================================================================
# VERIFICATION
# ===========================================================================
mp_clear()
_jar.clear()
ver_email = 'verify-me@bex.co'
ver_pw = 'VerifyPassw0rd!789'

# Register a fresh identity via the public API; verification is enabled, so
# Kratos mails a code and hands back a verification flow to continue.
_, flow = req('GET', f'{kratos_pub}/self-service/registration/api')
st, body = req('POST', action_of(flow), {
    'method': 'password', 'password': ver_pw, 'traits': {'email': ver_email},
})
if st != 200:
    fail(f'registration failed: {st} {json.dumps(body)[:400]}')
ident_id = body['identity']['id']
cont = body.get('continue_with', [])
ver_flow_id = next((c['flow']['id'] for c in cont
                    if c.get('action') == 'show_verification_ui'), None)
if not ver_flow_id:
    fail(f'registration did not start a verification flow: {json.dumps(cont)[:400]}')

code, _ = mp_wait_code(ver_email)
print(f'  ✓ verification code delivered to Mailpit ({code[:2]}****)')

# Submit the code against the verification flow the registration started.
_, vflow = req('GET',
               f'{kratos_pub}/self-service/verification/flows?id={ver_flow_id}')
st, body = req('POST', action_of(vflow), {'method': 'code', 'code': code})
if st != 200:
    fail(f'verification submit-code failed: {st} {json.dumps(body)[:400]}')

# Confirm via the admin API that the address is now verified.
_, ident = req('GET', f'{kratos_adm}/admin/identities/{ident_id}')
addrs = ident.get('verifiable_addresses', [])
if not addrs or not addrs[0].get('verified'):
    fail(f'address not marked verified: {json.dumps(addrs)[:400]}')
print('  ✓ identity address is now verified: true (admin API)')

# ===========================================================================
# NEGATIVES
# ===========================================================================
# (a) A garbage code must NOT complete recovery: no settings handoff (a VALID
# code answers 422 → settings; a wrong one must not), and the flow surfaces an error.
mp_clear()
_jar.clear()
neg_email = 'neg-code@bex.co'
admin_create_identity(neg_email, 'NegPassw0rd!123')
flow, csrf, _, _ = start_recovery(neg_email)
mp_wait_code(neg_email)  # a real code was mailed; we deliberately submit a wrong one
st, body = req('POST', action_of(flow), {'method': 'code', 'code': '000000', 'csrf_token': csrf})
redirect = body.get('redirect_browser_to') if isinstance(body, dict) else None
if st == 422 or redirect:
    fail(f'a garbage recovery code was accepted (status {st}, redirect {redirect}) — must be rejected')
msgs = ' '.join(m.get('text', '') for m in (body.get('ui', {}).get('messages', []) if isinstance(body, dict) else []))
if 'invalid' not in msgs.lower() and 'submitted' not in msgs.lower():
    fail(f'garbage code did not surface an error message: {json.dumps(body)[:400]}')
print(f'  ✓ garbage recovery code rejected (no handoff, error surfaced, status {st})')

# (b) Anti-enumeration: recovery for an unknown email returns the same "sent"
# state but delivers NO mail (Kratos default: notify_unknown_recipients=false).
mp_clear()
_jar.clear()
unknown = 'nobody-here@bex.co'
_, _, st, body = start_recovery(unknown)
state = body.get('state') if isinstance(body, dict) else None
if state != 'sent_email':
    fail(f'unknown-email recovery should report state=sent_email (anti-enum), got {state!r}')
time.sleep(3)  # give any (erroneous) courier dispatch a chance to land
if len(mp_messages_to(unknown)) != 0:
    fail('recovery for an unknown email delivered mail — leaks which accounts exist')
print('  ✓ unknown-email recovery: same "sent_email" state, zero mail delivered')

print('\n✓ email flows end-to-end: courier delivers; recovery resets a password;')
print('  verification confirms an address; invalid codes and unknown recipients')
print('  are handled without leaking. Same wiring prod runs (docs/ADR012-auth.md §11).')
PY

echo
echo "PASS: Kratos courier + recovery + verification verified end-to-end via Mailpit"
