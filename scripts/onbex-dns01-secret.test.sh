#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
trace="$tmp/kubectl.trace"
kind_trace="$tmp/kind.trace"
mock_bin="$tmp/bin"
mkdir -p "$mock_bin"

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
      expected_b64="$(printf '%s' "$TEST_EXPECTED_TOKEN" | base64 | tr -d '\n')"
      grep -qF '  name: onbex-dns01-cloudflare' <<<"$input"
      grep -qF '  namespace: traefik' <<<"$input"
      grep -qF 'type: Opaque' <<<"$input"
      grep -qF "  api-token: $expected_b64" <<<"$input"
      ;;
    *)
      echo "unexpected object kind: $kind" >&2
      exit 1
      ;;
  esac
  printf '%s\n' applied
else
  echo "unexpected kubectl invocation: $*" >&2
  exit 1
fi
MOCK
chmod +x "$mock_bin/kubectl"

echo "==> missing token aborts before any cluster call"
if env -u BEX_ONBEX_DNS_API_TOKEN \
  PATH="$mock_bin:$PATH" TEST_KUBECTL_TRACE="$trace" TEST_KIND_TRACE="$kind_trace" TEST_EXPECTED_TOKEN=unused \
  bash scripts/onbex-dns01-secret.sh >"$tmp/missing.out" 2>"$tmp/missing.err"; then
  echo "FAIL: missing BEX_ONBEX_DNS_API_TOKEN did not abort" >&2
  exit 1
fi
grep -qF 'BEX_ONBEX_DNS_API_TOKEN is required in the production-deploy environment' "$tmp/missing.err"
[ ! -s "$trace" ] || { echo "FAIL: missing token reached kubectl" >&2; exit 1; }

echo "==> token travels only over stdin"
test_token='dns-token-must-not-appear-in-output-or-argv'
PATH="$mock_bin:$PATH" \
  TEST_KUBECTL_TRACE="$trace" TEST_KIND_TRACE="$kind_trace" TEST_EXPECTED_TOKEN="$test_token" \
  BEX_ONBEX_DNS_API_TOKEN="$test_token" \
  bash scripts/onbex-dns01-secret.sh >"$tmp/present.out" 2>"$tmp/present.err"

for output in "$trace" "$tmp/present.out" "$tmp/present.err"; do
  if grep -Fq "$test_token" "$output"; then
    echo "FAIL: DNS token leaked through $output" >&2
    exit 1
  fi
done
grep -qF -- 'create namespace traefik --dry-run=client -o yaml' "$trace"
[ "$(grep -c '^apply -f -$' "$trace")" = 2 ] || {
  echo "FAIL: expected Namespace and Secret apply calls" >&2
  exit 1
}
[ "$(paste -sd, "$kind_trace")" = 'Namespace,Secret' ] || {
  echo "FAIL: helper did not apply the expected Namespace then Secret" >&2
  exit 1
}

echo "PASS: onbex DNS-01 secret installation is fail-closed and stdin-only"
