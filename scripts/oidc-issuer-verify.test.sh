#!/usr/bin/env bash
# Tests for scripts/oidc-issuer-verify.sh with kubectl, curl and the verifier
# stubbed: a healthy issuer passes, and each failure the script exists to catch
# (a token another workspace's issuer accepts, a claim off Render's template,
# discovery drifting from Render's, a malformed id answering) fails it.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/oidc-issuer-verify.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

ws="tea-d98210cbbpdc73dcrkvg"
srv="srv-c0000000000000000000"
base=https://oidc.bex.test
export OIDC_ISSUER_BASE=$base RENDER_OIDC_BASE=https://oidc.render.test
export TEST_DIR=$tmp

mkdir -p "$tmp/bin"
cat >"$tmp/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  "-n bex-system exec deploy/bex-api -c api -- /api oidc-test-token "*) echo "minted-test-token" ;;
  *) echo "unexpected kubectl $*" >&2; exit 1 ;;
esac
EOF
cat >"$tmp/bin/verify" <<'EOF'
#!/usr/bin/env bash
issuer=$2
read -r token
[ "$token" = "minted-test-token" ] || { echo "wrong token" >&2; exit 1; }
if [ "$issuer" != "$OIDC_ISSUER_BASE/tea-d98210cbbpdc73dcrkvg" ] && [ "$TEST_MODE" != cross ]; then
  echo "oidc: id token issued by a different provider" >&2
  exit 1
fi
sub="workspace:tea-d98210cbbpdc73dcrkvg:environment:default:service:srv-c0000000000000000000"
[ "$TEST_MODE" != sub ] || sub="srv-c0000000000000000000"
jq -cn --arg iss "$OIDC_ISSUER_BASE/tea-d98210cbbpdc73dcrkvg" --arg sub "$sub" \
  '{iss: $iss, sub: $sub, aud: ["bex-oidc-verify"], iat: 1000, exp: 4600, kid: "k1", jwksKids: ["k1"]}'
EOF
cat >"$tmp/bin/curl" <<'EOF'
#!/usr/bin/env bash
out="" url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift 2 ;;
    -sS | -D) shift ;;
    -) shift ;;
    *) url=$1; shift ;;
  esac
done
json() { printf 'HTTP/2 200\r\ncontent-type: application/json\r\ncache-control: public, max-age=300\r\n\r\n'; printf '%s' "$1" >"$out"; }
discovery() {
  local iss=$1
  printf '{"issuer":"%s","jwks_uri":"%s/.well-known/jwks.json","response_types_supported":["id_token"],"subject_types_supported":["public"],"id_token_signing_alg_values_supported":["ES256"]}\n' "$iss" "$iss"
}
jwks='{"keys":[{"use":"sig","kty":"EC","kid":"k1","crv":"P-256","alg":"ES256","x":"x","y":"y"}]}'
case "$url" in
  "$RENDER_OIDC_BASE/tea-c0000000000000000000/.well-known/openid-configuration") json "$(discovery "$RENDER_OIDC_BASE/tea-c0000000000000000000")" ;;
  "$RENDER_OIDC_BASE/tea-c0000000000000000000/.well-known/jwks.json") json "$jwks" ;;
  "$OIDC_ISSUER_BASE/tea-d98210cbbpdc73dcrkvg/.well-known/openid-configuration")
    if [ "$TEST_MODE" = drift ]; then
      json "$(discovery "$OIDC_ISSUER_BASE/tea-d98210cbbpdc73dcrkvg" | jq -c 'del(.subject_types_supported)')"
    else
      json "$(discovery "$OIDC_ISSUER_BASE/tea-d98210cbbpdc73dcrkvg")"
    fi ;;
  "$OIDC_ISSUER_BASE/tea-d98210cbbpdc73dcrkvg/.well-known/jwks.json") json "$jwks" ;;
  "$OIDC_ISSUER_BASE/tea-abc/.well-known/openid-configuration")
    if [ "$TEST_MODE" = malformed ]; then json "$(discovery "$OIDC_ISSUER_BASE/tea-abc")"; exit 0; fi
    printf 'HTTP/2 404\r\ncontent-type: text/plain; charset=utf-8\r\n\r\n'; printf '404 page not found\n' >"$out" ;;
  *)
    printf 'HTTP/2 404\r\ncontent-type: text/plain; charset=utf-8\r\n\r\n'; printf '404 page not found\n' >"$out" ;;
esac
EOF
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH" OIDC_VERIFY_BIN="$tmp/bin/verify"

run() { # run <mode> → exit code, output in $tmp/out
  set +e
  TEST_MODE=$1 bash "$script" "$ws" "$srv" >"$tmp/out" 2>&1
  local code=$?
  set -e
  return "$code"
}

if ! run good; then
  cat "$tmp/out" >&2
  echo "FAIL: a healthy issuer did not pass" >&2
  exit 1
fi
grep -q "every check passed for $base/$ws" "$tmp/out" || {
  cat "$tmp/out" >&2
  echo "FAIL: missing the success line" >&2
  exit 1
}

expect_failure() { # expect_failure <mode> <message fragment>
  if run "$1"; then
    cat "$tmp/out" >&2
    echo "FAIL: mode $1 passed, want a failure" >&2
    exit 1
  fi
  grep -qF "$2" "$tmp/out" || {
    cat "$tmp/out" >&2
    echo "FAIL: mode $1 failed without naming: $2" >&2
    exit 1
  }
}
expect_failure cross "accepted a token issued for $ws"
expect_failure sub "the verified claims are not Render's"
expect_failure drift "discovery differs from Render's"
expect_failure malformed "/tea-abc/.well-known/openid-configuration answers 200"

if bash "$script" tea-abc "$srv" >/dev/null 2>&1; then
  echo "FAIL: a malformed workspace argument was accepted" >&2
  exit 1
fi
echo "oidc-issuer-verify.test.sh: ok"
