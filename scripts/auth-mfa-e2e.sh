#!/usr/bin/env bash
# E2E for second-factor MFA (w4/m11, docs/auth.md § MFA) against the current
# kubeconfig cluster's Ory Kratos — the whole TOTP + recovery-code lifecycle,
# no browser (WebAuthn ceremonies can't be scripted over curl; that path is the
# manual check in t002). Everything here is Kratos-native self-service; there is
# no custom auth code to exercise.
#
#   1. Register a fresh identity (API flow) → aal1 session token.
#   2. Enroll TOTP + lookup_secret recovery codes via the settings flow — the
#      just-registered session stays privileged, so both enroll at aal1 (Kratos
#      gates privileged submits by session recency, not by an aal2 flow; a
#      *fresh* login below is what the AAL policy forces up to aal2).
#   3. Password-only login yields an aal1 token whose whoami 403s
#      (`highest_available` policy) — the second factor is owed.
#   4. aal2 step-up: a wrong TOTP code is rejected; the right one upgrades the
#      session (whoami → aal2).
#   5. Recovery: a burned lookup_secret code logs in "without the device"; the
#      same code a second time is rejected (single-use).
#
# The `highest_available` AAL policy + the three methods must be live in the
# Kratos config (base/values/kratos.values.yaml) — this script asserts the
# behavior that config produces, so it fails loudly if any of it regresses.
#
# Talks to kratos-public via `kubectl port-forward` by default (works on the
# CAPD mock cluster with no ingress), OR set KRATOS_PUBLIC_URL to hit a running
# endpoint directly (e.g. https://auth.bex.co for the prod smoke in t004) — in
# which case no port-forward/kubectl is used.
#
# Usage: scripts/auth-mfa-e2e.sh        # respects $KUBECONFIG; exits 0 on pass
#        KRATOS_PUBLIC_URL=https://auth.bex.co scripts/auth-mfa-e2e.sh
# Requires: curl, yq v4, node (TOTP computation); kubectl unless KRATOS_PUBLIC_URL is set.
set -euo pipefail
cd "$(dirname "$0")/.."

NS=auth
PF_PIDS=()
TOTP_JS="$(mktemp)"

cleanup() {
  rm -f "$TOTP_JS"
  if [ "${#PF_PIDS[@]}" -gt 0 ]; then
    kill "${PF_PIDS[@]}" 2>/dev/null || true
    wait "${PF_PIDS[@]}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

wait_http() { # url — wait until it answers at all (any HTTP status)
  for _ in $(seq 1 30); do
    code="$(curl -sk -o /dev/null -w '%{http_code}' "$1" || true)"
    [ "$code" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

# RFC 6238 TOTP (SHA1/30s/6-digit, Kratos's defaults) with zero dependencies —
# oathtool/otplib aren't guaranteed on a dev box, but node ships with the
# dashboard toolchain. `totp <base32-secret> [skew]` prints the current code;
# skew shifts the time window by N steps (used to build a guaranteed-wrong code).
cat >"$TOTP_JS" <<'EOF'
const crypto = require("crypto");
function base32ToBuf(b32) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const c of b32.replace(/=+$/, "").toUpperCase().replace(/\s+/g, "")) {
    const v = alphabet.indexOf(c);
    if (v >= 0) bits += v.toString(2).padStart(5, "0");
  }
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
  return Buffer.from(bytes);
}
function totp(secret, skew) {
  const key = base32ToBuf(secret);
  let counter = Math.floor(Date.now() / 1000 / 30) + (skew | 0);
  const buf = Buffer.alloc(8);
  for (let i = 7; i >= 0; i--) { buf[i] = counter & 0xff; counter = Math.floor(counter / 256); }
  const h = crypto.createHmac("sha1", key).update(buf).digest();
  const off = h[h.length - 1] & 0xf;
  const bin = ((h[off] & 0x7f) << 24) | ((h[off + 1] & 0xff) << 16) | ((h[off + 2] & 0xff) << 8) | (h[off + 3] & 0xff);
  return (bin % 1e6).toString().padStart(6, "0");
}
process.stdout.write(totp(process.argv[2], Number(process.argv[3] || 0)));
EOF
totp() { node "$TOTP_JS" "$1" "${2:-0}"; }

# --- reach kratos-public ------------------------------------------------------
if [ -n "${KRATOS_PUBLIC_URL:-}" ]; then
  K="${KRATOS_PUBLIC_URL%/}"
  echo "==> using KRATOS_PUBLIC_URL=$K (no port-forward)"
else
  K=http://127.0.0.1:24433
  echo "==> port-forwarding kratos-public"
  kubectl -n "$NS" port-forward service/kratos-public 24433:80 >/dev/null 2>&1 &
  PF_PIDS+=($!)
fi
wait_http "$K/health/ready"

CURL=(curl -sk) # -k: prod public TLS is fine, but a port-forward has no cert

# kratos METHOD PATH [SESSION_TOKEN] [JSON_BODY] — sets LAST_CODE + LAST_BODY.
kratos() {
  local args=("${CURL[@]}" -w '\n%{http_code}' -X "$1" "$K$2")
  [ -n "${3:-}" ] && args+=(-H "X-Session-Token: $3")
  [ -n "${4:-}" ] && args+=(-H 'Content-Type: application/json' -d "$4")
  local out
  out="$("${args[@]}")"
  LAST_CODE="${out##*$'\n'}"
  LAST_BODY="${out%$'\n'*}"
}

jqf() { printf '%s' "$LAST_BODY" | yq "$1" -; } # read a field off LAST_BODY

# Kratos builds flow `ui.action` URLs from its configured base_url (e.g.
# localhost:4433 on the mock overlay), which need not be the host we actually
# reach it on (a port-forward on :24433). Reduce any action URL to its path so
# we re-target it at $K. On prod (base_url == $K) this is a no-op.
rel() { local a="${1#*://}"; printf '/%s' "${a#*/}"; }

# --- 1. register a fresh identity (API flow) ----------------------------------
EMAIL="mfa-e2e-$(date +%s)-$RANDOM@bex.co"
PASSWORD="bex-Mfa-e2e-$RANDOM-Zq9!"
echo "==> registering $EMAIL"
kratos GET /self-service/registration/api
reg_action="$(jqf '.ui.action')"
[ -n "$reg_action" ] && [ "$reg_action" != "null" ] || fail "no registration flow action"
kratos POST "$(rel "$reg_action")" "" \
  "{\"method\":\"password\",\"password\":\"$PASSWORD\",\"traits\":{\"email\":\"$EMAIL\"}}"
[ "$LAST_CODE" = "200" ] || fail "registration failed ($LAST_CODE): $LAST_BODY"
tok="$(jqf '.session_token')"
[ -n "$tok" ] && [ "$tok" != "null" ] || fail "registration returned no session token"
echo "    ok: registered, aal1 session token issued"

# whoami helper: prints the AAL, or the HTTP code (e.g. "403") when the session
# is below required_aal. Always called in a $(...) so its kratos() call sets
# LAST_* only in the subshell, never clobbering the caller's.
whoami_aal() { # session_token -> aal string | HTTP code
  kratos GET /sessions/whoami "$1"
  [ "$LAST_CODE" = "200" ] || { echo "$LAST_CODE"; return; }
  jqf '.authenticator_assurance_level'
}

[ "$(whoami_aal "$tok")" = "aal1" ] || fail "fresh session should be aal1"

# --- 2a. enroll TOTP via the settings flow ------------------------------------
echo "==> enrolling TOTP"
kratos GET /self-service/settings/api "$tok"
[ "$LAST_CODE" = "200" ] || fail "settings flow (aal1) failed ($LAST_CODE): $LAST_BODY"
set_action="$(jqf '.ui.action')"
secret="$(jqf '.ui.nodes[] | select(.attributes.id == "totp_secret_key") | .attributes.text.text')"
[ -n "$secret" ] && [ "$secret" != "null" ] || fail "settings flow did not expose a TOTP secret"
kratos POST "$(rel "$set_action")" "$tok" "{\"method\":\"totp\",\"totp_code\":\"$(totp "$secret")\"}"
[ "$LAST_CODE" = "200" ] || fail "TOTP enrollment rejected ($LAST_CODE): $LAST_BODY"
[ "$(jqf '.state')" = "success" ] || fail "settings flow did not report success after TOTP confirm: $LAST_BODY"
echo "    ok: TOTP enrolled"

# Define the aal2 step-up helper used by the login-challenge tests below. Opens
# an aal2 login flow bound to the session and submits one second factor; leaves
# the login response in LAST_CODE/LAST_BODY (200 + upgraded token on success).
# Call directly (not in $()) so the caller can read LAST_*.
step_up() { # session_token  method  extra-json (e.g. '"totp_code":"123456"')
  kratos GET "/self-service/login/api?aal=aal2&refresh=false" "$1"
  kratos POST "$(rel "$(jqf '.ui.action')")" "$1" "{\"method\":\"$2\",$3}"
}

# --- 2b. enroll lookup_secret recovery codes (same privileged aal1 session) ---
# The just-registered session is still privileged, so it enrolls a second
# credential without stepping up — Kratos gates privileged submits by session
# recency, not by demanding an aal2 flow to open settings.
echo "==> enrolling recovery codes"
kratos GET /self-service/settings/api "$tok"
[ "$LAST_CODE" = "200" ] || fail "settings flow failed ($LAST_CODE): $LAST_BODY"
kratos POST "$(rel "$(jqf '.ui.action')")" "$tok" '{"method":"lookup_secret","lookup_secret_regenerate":true}'
[ "$LAST_CODE" = "200" ] || fail "recovery-code generation failed ($LAST_CODE): $LAST_BODY"
# Codes come back structured on the lookup_secret_codes node — one per secret.
# (while-read, not mapfile: stay compatible with macOS's stock bash 3.2.)
CODES=()
while IFS= read -r _code; do [ -n "$_code" ] && CODES+=("$_code"); done < <(
  jqf '.ui.nodes[] | select(.attributes.id == "lookup_secret_codes") | .attributes.text.context.secrets[].context.secret'
)
kratos POST "$(rel "$(jqf '.ui.action')")" "$tok" '{"method":"lookup_secret","lookup_secret_confirm":true}'
[ "$LAST_CODE" = "200" ] || fail "recovery-code confirm failed ($LAST_CODE): $LAST_BODY"
[ "${#CODES[@]}" -ge 2 ] || fail "expected >=2 recovery codes, parsed ${#CODES[@]}"
echo "    ok: recovery codes enrolled (${#CODES[@]} codes)"

# --- 3 + 4. password login is aal1; TOTP challenge gates aal2 -----------------
echo "==> login challenge: password → aal2"
# Password (first-factor) login; leaves the aal1 session response in
# LAST_CODE/LAST_BODY. Call directly, then read the token via jqf.
login_password() {
  kratos GET /self-service/login/api
  kratos POST "$(rel "$(jqf '.ui.action')")" "" \
    "{\"method\":\"password\",\"identifier\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
}
login_password
p_tok="$(jqf '.session_token')"
[ -n "$p_tok" ] && [ "$p_tok" != "null" ] || fail "password login returned no token"
[ "$(whoami_aal "$p_tok")" = "403" ] || fail "aal1 session must fail whoami under highest_available"
echo "    ok: password-only login is aal1; whoami 403 (second factor owed)"

# Wrong code rejected.
wrong="$(totp "$secret" -3)" # a code from 90s ago — outside the accept window
[ "$wrong" != "$(totp "$secret")" ] || wrong="000000"
step_up "$p_tok" totp "\"totp_code\":\"$wrong\""
[ "$LAST_CODE" != "200" ] || fail "a wrong TOTP code was accepted"
[ "$(whoami_aal "$p_tok")" = "403" ] || fail "session must stay aal1 after a wrong code"
echo "    ok: wrong TOTP code rejected, session unchanged"

# Right code upgrades to aal2.
step_up "$p_tok" totp "\"totp_code\":\"$(totp "$secret")\""
[ "$LAST_CODE" = "200" ] || fail "a valid TOTP code was rejected ($LAST_CODE): $LAST_BODY"
up_tok="$(jqf '.session_token')"
[ "$(whoami_aal "$up_tok")" = "aal2" ] || fail "valid TOTP code must produce an aal2 session"
echo "    ok: valid TOTP code upgrades the session to aal2"

# --- 5. recovery code logs in without the device; single-use ------------------
echo "==> recovery code: burn once, reject reuse"
login_password
r_tok="$(jqf '.session_token')"
[ "$(whoami_aal "$r_tok")" = "403" ] || fail "recovery-path login should start at aal1/403"
step_up "$r_tok" lookup_secret "\"lookup_secret\":\"${CODES[0]}\""
[ "$LAST_CODE" = "200" ] || fail "a valid recovery code was rejected ($LAST_CODE): $LAST_BODY"
rec_tok="$(jqf '.session_token')"
[ "$(whoami_aal "$rec_tok")" = "aal2" ] || fail "recovery code must produce an aal2 session"
echo "    ok: recovery code unlocks the account (aal2)"

# Reuse of the SAME code must fail (single-use).
login_password
r_tok2="$(jqf '.session_token')"
step_up "$r_tok2" lookup_secret "\"lookup_secret\":\"${CODES[0]}\""
[ "$LAST_CODE" != "200" ] || fail "a used recovery code was accepted a second time"
echo "    ok: used recovery code is rejected on reuse"

echo "PASS: TOTP enroll → aal2 challenge (wrong rejected) → single-use recovery codes verified end-to-end"
