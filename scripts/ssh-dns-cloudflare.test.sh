#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
reconcile="$repo_root/scripts/ssh-dns-cloudflare.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

export BEX_SSH_HOST=ssh.example.test
export BEX_SSH_DNS_ZONE=example.test
export CLOUDFLARE_API_TOKEN=test-token-must-not-appear-in-output
export HCLOUD_TOKEN=test-hcloud-token-must-not-appear-in-output
export TEST_CF_STATE="$tmp/records.json"
export TEST_CF_TRACE="$tmp/cloudflare.trace"

curl() {
  local method=GET
  local payload=""
  local url=""
  while (($#)); do
    case "$1" in
      --request)
        method="$2"
        shift 2
        ;;
      --header)
        shift 2
        ;;
      --data)
        payload="$2"
        shift 2
        ;;
      --silent | --show-error | --fail-with-body)
        shift
        ;;
      *)
        url="$1"
        shift
        ;;
    esac
  done

  if [[ "$url" == https://api.hetzner.cloud/v1/load_balancers* ]]; then
    jq -cn '{load_balancers: [{name: "bex-traefik", public_net: {enabled: true, ipv4: {ip: "192.0.2.10"}, ipv6: {ip: "2001:db8::10"}}}]}'
    return
  fi

  local path="${url#https://api.cloudflare.com/client/v4}"
  printf '%s %s\n' "$method" "$path" >>"$TEST_CF_TRACE"
  case "$method $path" in
    "GET /zones?"*)
      jq -cn '{success: true, result: [{id: "zone-test", name: "example.test", status: "active"}], errors: []}'
      ;;
    "GET /zones/zone-test/dns_records?"*)
      jq -cn --slurpfile records "$TEST_CF_STATE" '{success: true, result: $records[0], errors: []}'
      ;;
    "PATCH /zones/zone-test/dns_records/"*)
      local record_id="${path##*/}"
      jq --arg id "$record_id" --argjson payload "$payload" \
        'map(if .id == $id then . + $payload else . end)' \
        "$TEST_CF_STATE" >"$TEST_CF_STATE.next"
      mv "$TEST_CF_STATE.next" "$TEST_CF_STATE"
      jq -cn --argjson result "$payload" '{success: true, result: $result, errors: []}'
      ;;
    "POST /zones/zone-test/dns_records")
      local created_id
      created_id="created-$(jq -r '.type' <<<"$payload")"
      jq --arg id "$created_id" --argjson payload "$payload" \
        '. + [($payload + {id: $id})]' "$TEST_CF_STATE" >"$TEST_CF_STATE.next"
      mv "$TEST_CF_STATE.next" "$TEST_CF_STATE"
      jq -cn --arg id "$created_id" --argjson payload "$payload" \
        '{success: true, result: ($payload + {id: $id}), errors: []}'
      ;;
    "DELETE /zones/zone-test/dns_records/"*)
      local record_id="${path##*/}"
      jq --arg id "$record_id" 'map(select(.id != $id))' \
        "$TEST_CF_STATE" >"$TEST_CF_STATE.next"
      mv "$TEST_CF_STATE.next" "$TEST_CF_STATE"
      jq -cn --arg id "$record_id" '{success: true, result: {id: $id}, errors: []}'
      ;;
    *)
      echo "unexpected mock Cloudflare request: $method $path" >&2
      return 1
      ;;
  esac
}

export -f curl

write_state() {
  printf '%s\n' "$1" >"$TEST_CF_STATE"
  : >"$TEST_CF_TRACE"
}

assert_contains() {
  local needle="$1"
  local file="$2"
  grep -Fq -- "$needle" "$file" || {
    echo "expected $file to contain: $needle" >&2
    exit 1
  }
}

assert_reconciled_state() {
  jq -e '
    length == 2 and
    any(.[]; .type == "A" and .content == "192.0.2.10" and .proxied == false) and
    any(.[]; .type == "AAAA" and .content == "2001:db8::10" and .proxied == false)
  ' "$TEST_CF_STATE" >/dev/null
}

write_state '[
  {"id":"a-good","type":"A","name":"ssh.example.test","content":"192.0.2.10","ttl":300,"proxied":false},
  {"id":"aaaa-good","type":"AAAA","name":"ssh.example.test","content":"2001:db8::10","ttl":300,"proxied":false}
]'
output="$(bash "$reconcile" --check)"
[[ "$output" == "PASS Cloudflare SSH DNS host=ssh.example.test addresses=192.0.2.10,2001:db8::10" ]]
if grep -Eq '^(PATCH|POST|DELETE) ' "$TEST_CF_TRACE"; then
  echo 'Cloudflare DNS check mode performed a mutation' >&2
  exit 1
fi
if [[ "$output" == *"$CLOUDFLARE_API_TOKEN"* ]] || grep -Fq "$CLOUDFLARE_API_TOKEN" "$TEST_CF_TRACE"; then
  echo 'Cloudflare API token leaked into test output' >&2
  exit 1
fi

write_state '[
  {"id":"a-proxied","type":"A","name":"ssh.example.test","content":"192.0.2.10","ttl":1,"proxied":true},
  {"id":"aaaa-stale","type":"AAAA","name":"ssh.example.test","content":"2001:db8::99","ttl":300,"proxied":false}
]'
if bash "$reconcile" --check >"$tmp/mismatch.out" 2>&1; then
  echo 'expected proxied or stale Cloudflare records to fail check mode' >&2
  exit 1
fi
assert_contains 'do not match the Terraform edge Load Balancer addresses' "$tmp/mismatch.out"

write_state '[
  {"id":"a-primary","type":"A","name":"ssh.example.test","content":"192.0.2.10","ttl":1,"proxied":true},
  {"id":"a-duplicate","type":"A","name":"ssh.example.test","content":"198.51.100.8","ttl":300,"proxied":false},
  {"id":"aaaa-primary","type":"AAAA","name":"ssh.example.test","content":"2001:db8::99","ttl":300,"proxied":false}
]'
output="$(bash "$reconcile")"
[[ "$output" == "reconciled Cloudflare SSH DNS host=ssh.example.test addresses=192.0.2.10,2001:db8::10" ]]
assert_contains 'PATCH /zones/zone-test/dns_records/a-primary' "$TEST_CF_TRACE"
assert_contains 'PATCH /zones/zone-test/dns_records/aaaa-primary' "$TEST_CF_TRACE"
assert_contains 'DELETE /zones/zone-test/dns_records/a-duplicate' "$TEST_CF_TRACE"
assert_reconciled_state

write_state '[
  {"id":"a-only","type":"A","name":"ssh.example.test","content":"192.0.2.10","ttl":300,"proxied":false}
]'
bash "$reconcile" >"$tmp/create.out"
assert_contains 'POST /zones/zone-test/dns_records' "$TEST_CF_TRACE"
assert_reconciled_state

echo 'PASS Cloudflare SSH DNS reconciliation safety gates'
