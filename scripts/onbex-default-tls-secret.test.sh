#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
mock_bin="$tmp/bin"
trace="$tmp/kubectl.trace"
kind_trace="$tmp/kind.trace"
mkdir -p "$mock_bin"

make_leaf() {
  local name="$1" san="$2" days="$3"
  cat >"$tmp/$name.cnf" <<EOF
[extensions]
subjectAltName = DNS:$san
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
EOF
  openssl req -new -key "$tmp/valid.key" -subj "/CN=$san" \
    -out "$tmp/$name.csr" >/dev/null 2>&1
  openssl x509 -req -days "$days" -in "$tmp/$name.csr" \
    -CA "$tmp/root.crt" -CAkey "$tmp/root.key" -CAcreateserial \
    -extfile "$tmp/$name.cnf" -extensions extensions -out "$tmp/$name.crt" \
    >/dev/null 2>&1
  printf '\n' >>"$tmp/$name.crt"
  sed -n '1,$p' "$tmp/root.crt" >>"$tmp/$name.crt"
}

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
  -out "$tmp/valid.key" >/dev/null 2>&1
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
  -out "$tmp/root.key" >/dev/null 2>&1
openssl req -x509 -new -key "$tmp/root.key" -days 3650 -subj '/CN=m85 test root' \
  -addext 'basicConstraints=critical,CA:true' -addext 'keyUsage=critical,keyCertSign,cRLSign' \
  -out "$tmp/root.crt" >/dev/null 2>&1
make_leaf valid '*.onbex.co' 60
make_leaf wrong_san 'onbex.co' 60
make_leaf expiring '*.onbex.co' 1
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
  -out "$tmp/mismatch.key" >/dev/null 2>&1
openssl req -x509 -new -key "$tmp/mismatch.key" -days 3650 -subj '/CN=untrusted m85 test root' \
  -addext 'basicConstraints=critical,CA:true' -addext 'keyUsage=critical,keyCertSign,cRLSign' \
  -out "$tmp/untrusted-root.crt" >/dev/null 2>&1

cat >"$mock_bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$TEST_KUBECTL_TRACE"

if [[ "$*" == "create namespace traefik --dry-run=client -o yaml" ]]; then
  printf '%s\n' 'apiVersion: v1' 'kind: Namespace' 'metadata:' '  name: traefik'
elif [[ "$*" == "apply -f -" ]]; then
  input="$(cat)"
  kind="$(sed -n 's/^kind: //p' <<<"$input")"
  printf '%s\n' "$kind" >>"$TEST_KIND_TRACE"
  case "$kind" in
    Namespace)
      grep -qF '  name: traefik' <<<"$input"
      ;;
    Secret)
      expected_cert="$(printf '%s' "$TEST_EXPECTED_CERT" | base64 | tr -d '\n')"
      expected_key="$(printf '%s' "$TEST_EXPECTED_KEY" | base64 | tr -d '\n')"
      grep -qF '  name: onbex-default-wildcard-tls' <<<"$input"
      grep -qF '  namespace: traefik' <<<"$input"
      grep -qF 'type: kubernetes.io/tls' <<<"$input"
      grep -qF "  tls.crt: $expected_cert" <<<"$input"
      grep -qF "  tls.key: $expected_key" <<<"$input"
      ;;
    *)
      echo "unexpected object kind: $kind" >&2
      exit 1
      ;;
  esac
else
  echo "unexpected kubectl invocation: $*" >&2
  exit 1
fi
MOCK
chmod +x "$mock_bin/kubectl"

expect_rejected() {
  local label="$1" certificate="$2" key="$3" expected="$4"
  : >"$trace"
  if PATH="$mock_bin:$PATH" \
    TEST_KUBECTL_TRACE="$trace" TEST_KIND_TRACE="$kind_trace" \
    SSL_CERT_FILE="$tmp/root.crt" \
    BEX_ONBEX_TLS_CERT="$certificate" BEX_ONBEX_TLS_KEY="$key" \
    bash scripts/onbex-default-tls-secret.sh >"$tmp/rejected.out" 2>"$tmp/rejected.err"; then
    echo "FAIL: $label was accepted" >&2
    exit 1
  fi
  grep -qF "$expected" "$tmp/rejected.err"
  [ ! -s "$trace" ] || {
    echo "FAIL: $label reached kubectl before validation failed" >&2
    exit 1
  }
}

valid_cert="$(<"$tmp/valid.crt")"
valid_key="$(<"$tmp/valid.key")"

echo "==> missing and invalid material aborts before cluster access"
expect_rejected missing-cert '' "$valid_key" 'BEX_ONBEX_TLS_CERT is required'
expect_rejected missing-key "$valid_cert" '' 'BEX_ONBEX_TLS_KEY is required'
expect_rejected wrong-san "$(<"$tmp/wrong_san.crt")" "$valid_key" 'exact DNS SAN *.onbex.co'
expect_rejected expiring "$(<"$tmp/expiring.crt")" "$valid_key" 'expires inside the 30-day minimum validity window'
expect_rejected mismatched "$valid_cert" "$(<"$tmp/mismatch.key")" 'do not contain the same public key'

echo "==> an untrusted chain aborts before cluster access"
: >"$trace"
if PATH="$mock_bin:$PATH" \
  TEST_KUBECTL_TRACE="$trace" TEST_KIND_TRACE="$kind_trace" \
  SSL_CERT_FILE="$tmp/untrusted-root.crt" \
  BEX_ONBEX_TLS_CERT="$valid_cert" BEX_ONBEX_TLS_KEY="$valid_key" \
  bash scripts/onbex-default-tls-secret.sh >"$tmp/untrusted.out" 2>"$tmp/untrusted.err"; then
  echo "FAIL: untrusted chain was accepted" >&2
  exit 1
fi
grep -qF 'does not contain a chain trusted by the runner' "$tmp/untrusted.err"
[ ! -s "$trace" ] || {
  echo "FAIL: untrusted chain reached kubectl before validation failed" >&2
  exit 1
}

echo "==> preflight validates without cluster access"
: >"$trace"
PATH="$mock_bin:$PATH" \
  TEST_KUBECTL_TRACE="$trace" TEST_KIND_TRACE="$kind_trace" \
  SSL_CERT_FILE="$tmp/root.crt" \
  BEX_ONBEX_TLS_CERT="$valid_cert" BEX_ONBEX_TLS_KEY="$valid_key" \
  bash scripts/onbex-default-tls-secret.sh --validate-only \
  >"$tmp/preflight.out" 2>"$tmp/preflight.err"
grep -qF 'passed certificate, trust, expiry, and key checks' "$tmp/preflight.out"
[ ! -s "$trace" ] || {
  echo "FAIL: validate-only preflight reached kubectl" >&2
  exit 1
}

echo "==> validated material travels only over stdin"
: >"$trace"
: >"$kind_trace"
PATH="$mock_bin:$PATH" \
  TEST_KUBECTL_TRACE="$trace" TEST_KIND_TRACE="$kind_trace" \
  TEST_EXPECTED_CERT="$valid_cert" TEST_EXPECTED_KEY="$valid_key" \
  SSL_CERT_FILE="$tmp/root.crt" \
  BEX_ONBEX_TLS_CERT="$valid_cert" BEX_ONBEX_TLS_KEY="$valid_key" \
  bash scripts/onbex-default-tls-secret.sh >"$tmp/valid.out" 2>"$tmp/valid.err"

for output in "$trace" "$tmp/valid.out" "$tmp/valid.err"; do
  if grep -Fq -- 'BEGIN PRIVATE KEY' "$output" || grep -Fq -- 'BEGIN CERTIFICATE' "$output"; then
    echo "FAIL: TLS material leaked through $output" >&2
    exit 1
  fi
done
[ "$(paste -sd, "$kind_trace")" = 'Namespace,Secret' ] || {
  echo "FAIL: installer did not apply the expected Namespace then Secret" >&2
  exit 1
}

echo "PASS: onbex fallback TLS installation is validated, fail-closed, and stdin-only"
