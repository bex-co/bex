#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
activate="$repo_root/scripts/ssh-activate.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

ssh-keygen -q -t ed25519 -N '' -f "$tmp/host" >/dev/null
export BEX_SSH_HOST=ssh.example.test
export BEX_SSH_HOST_KEY_FILE="$tmp/host"
export TEST_PUBLIC_KEY_FILE="$tmp/host.pub"
export TEST_TRACE="$tmp/kubectl.trace"
export TEST_SCAN_TRACE="$tmp/keyscan.trace"
# Hetzner also reports the LB's private-network ingress. It must not become a
# public-DNS requirement or appear in the activation mutation path.
export TEST_EDGE_ADDRESSES=$'192.0.2.10\n2001:db8::10\n10.10.0.7\nfd00::7'
export TEST_A_ADDRESSES=192.0.2.10
export TEST_AAAA_ADDRESSES=2001:db8::10

kubectl() {
  printf '%s\n' "$*" >>"$TEST_TRACE"
  if [[ "$*" == *"get service traefik"* ]]; then
    printf '%s\n' "$TEST_EDGE_ADDRESSES"
  elif [[ "$*" == *"create configmap bex-ssh"* ]]; then
    printf '%s\n' 'apiVersion: v1' 'kind: ConfigMap' 'metadata:' '  name: bex-ssh'
  elif [[ "$*" == *"apply -f -"* ]]; then
    command cat >/dev/null
  fi
}

dig() {
  case "$*" in
    *" A "*) printf '%s\n' "$TEST_A_ADDRESSES" ;;
    *" AAAA "*) printf '%s\n' "$TEST_AAAA_ADDRESSES" ;;
    *) return 2 ;;
  esac
}

ssh-keyscan() {
  printf '%s\n' called >>"$TEST_SCAN_TRACE"
  printf '[%s]:22 %s\n' "$BEX_SSH_HOST" "$(<"$TEST_PUBLIC_KEY_FILE")"
}

export -f kubectl dig ssh-keyscan

assert_contains() {
  local needle="$1"
  local file="$2"
  grep -Fq -- "$needle" "$file" || {
    echo "expected $file to contain: $needle" >&2
    exit 1
  }
}

: >"$TEST_TRACE"
: >"$TEST_SCAN_TRACE"
output="$(bash "$activate" --check)"
[[ "$output" == PASS\ app\ SSH\ activation\ preflight* ]]
[[ "$(wc -l <"$TEST_TRACE" | tr -d ' ')" == 1 ]]
assert_contains 'get service traefik' "$TEST_TRACE"
[[ "$(wc -l <"$TEST_SCAN_TRACE" | tr -d ' ')" == 1 ]]
[[ "$output" != *'10.10.0.7'* && "$output" != *'fd00::7'* ]]

: >"$TEST_TRACE"
: >"$TEST_SCAN_TRACE"
export TEST_AAAA_ADDRESSES=2001:db8::99
if bash "$activate" --check >"$tmp/mismatch.out" 2>&1; then
  echo 'expected mismatched DNS to fail' >&2
  exit 1
fi
assert_contains 'public A/AAAA records must equal' "$tmp/mismatch.out"
[[ ! -s "$TEST_SCAN_TRACE" ]]
[[ "$(wc -l <"$TEST_TRACE" | tr -d ' ')" == 1 ]]

: >"$TEST_TRACE"
: >"$TEST_SCAN_TRACE"
export TEST_AAAA_ADDRESSES=2001:db8::10
bash "$activate" >"$tmp/activate.out"
assert_contains 'create configmap bex-ssh' "$TEST_TRACE"
assert_contains 'apply -f -' "$TEST_TRACE"
assert_contains 'rollout restart deployment/bex-api' "$TEST_TRACE"
assert_contains 'rollout status deployment/bex-api' "$TEST_TRACE"
assert_contains 'activated app SSH' "$tmp/activate.out"

echo 'PASS ssh-activate safety gates'
