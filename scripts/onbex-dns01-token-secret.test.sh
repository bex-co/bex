#!/usr/bin/env bash
# Self-test for scripts/onbex-dns01-token-secret.sh (docs/ADR093): it must fail
# closed on a missing or malformed token, and install a valid one as
# traefik/cloudflare-dns-token without the token reaching argv or output.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
mock_bin="$tmp/bin"
trace="$tmp/kubectl.trace"
kind_trace="$tmp/kind.trace"
mkdir -p "$mock_bin"
token="cfat_$(printf 'a%.0s' {1..40})-B_9"

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
  if [ "$kind" = Secret ]; then
    grep -qF '  name: cloudflare-dns-token' <<<"$input"
    grep -qF '  namespace: traefik' <<<"$input"
    grep -qF 'type: Opaque' <<<"$input"
    encoded="$(sed -n 's/^  api-token: //p' <<<"$input")"
    [ "$(printf '%s' "$encoded" | base64 --decode)" = "$TEST_EXPECTED_TOKEN" ]
  fi
else
  echo "unexpected kubectl call: $*" >&2
  exit 1
fi
MOCK
chmod +x "$mock_bin/kubectl"

run_installer() { # <token> <label> [--validate-only]
  : >"$trace"
  : >"$kind_trace"
  PATH="$mock_bin:$PATH" TEST_KUBECTL_TRACE="$trace" TEST_KIND_TRACE="$kind_trace" \
    TEST_EXPECTED_TOKEN="$token" BEX_CLOUDFLARE_DNS_TOKEN="$1" \
    bash scripts/onbex-dns01-token-secret.sh ${3:+"$3"} >"$tmp/$2.out" 2>"$tmp/$2.err"
}

for case in "missing|" "short|cfat_abc" "whitespace|${token} " "newline|${token}
extra"; do
  label="${case%%|*}"
  if run_installer "${case#*|}" "$label-preflight" --validate-only; then
    echo "FAIL: preflight accepted a $label token" >&2
    exit 1
  fi
  if run_installer "${case#*|}" "$label"; then
    echo "FAIL: installer accepted a $label token" >&2
    exit 1
  fi
  [ ! -s "$trace" ] || {
    echo "FAIL: installer touched the cluster for a $label token" >&2
    exit 1
  }
  echo "ok: $label token refused before any cluster write"
done

run_installer "$token" preflight --validate-only
[ ! -s "$trace" ] || {
  echo "FAIL: --validate-only called kubectl" >&2
  exit 1
}
echo "ok: preflight accepts a valid token without touching the cluster"

run_installer "$token" valid
for output in "$trace" "$tmp"/*.out "$tmp"/*.err; do
  if grep -qF -- "$token" "$output"; then
    echo "FAIL: token leaked through $output" >&2
    exit 1
  fi
done
[ "$(paste -sd, "$kind_trace")" = 'Namespace,Secret' ] || {
  echo "FAIL: installer did not apply the expected Namespace then Secret" >&2
  exit 1
}
echo "ok: valid token installed over stdin only"

echo "PASS: onbex DNS-01 token installation is fail-closed and stdin-only"
