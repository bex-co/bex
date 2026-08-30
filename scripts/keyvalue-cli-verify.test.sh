#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verify="$repo_root/scripts/keyvalue-cli-verify.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fake_bin="$tmp/bin"
mkdir -p "$fake_bin" "$tmp/config"

write_fakes() {
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'if [[ "${1:-}" == "--version" ]]; then echo "render v2.21.0"; exit 0; fi' \
    'if [[ "${1:-}" == "keyvalues" && "${2:-}" == "create" ]]; then' \
    '  name=""; allow=""; args=("$@")' \
    '  for ((i=0; i<${#args[@]}; i++)); do' \
    '    [[ "${args[$i]}" == "--name" ]] && name="${args[$((i+1))]}"' \
    '    [[ "${args[$i]}" == "--ip-allow-list" ]] && allow="${args[$((i+1))]}"' \
    '    [[ "${args[$i]}" != "--public" ]] || exit 91' \
    '  done' \
    '  [[ -n "$name" && "$allow" == cidr=192.0.2.10/32,* ]] || exit 92' \
    '  printf "CLI create %s\n" "$name" >>"$KV_VERIFY_CALLS"' \
    '  printf "{\"data\":{\"id\":\"red-fixture\",\"name\":\"%s\"}}\n" "$name"' \
    '  exit 0' \
    'fi' \
    'exit 93' >"$fake_bin/render"

  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'exit 0' >"$fake_bin/redis-cli"

  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'echo 192.0.2.10' >"$fake_bin/dig"

  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'exit 0' >"$fake_bin/nc"

  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'if [[ "${1:-}" == "version" && "${2:-}" == "-m" ]]; then' \
    '  if [[ "${KV_VERIFY_BAD_MODULE:-0}" == "1" ]]; then printf "\tpath\texample.invalid/modified-cli\n"; else printf "\tpath\tgithub.com/render-oss/cli\n"; fi' \
    '  exit 0' \
    'fi' \
    'printf "go id=%s name=%s family=%s\n" "$BEX_TEST_KV_CLI_ID" "$BEX_TEST_KV_CLI_NAME" "$BEX_TEST_KV_CLI_FAMILY" >>"$KV_VERIFY_CALLS"' \
    'if [[ "${KV_VERIFY_GO_FAIL:-0}" == "1" ]]; then' \
    '  echo "safe simulated probe failure" >&2' \
    '  exit 1' \
    'fi' >"$fake_bin/go"

  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'method=GET' \
    'write_code=0' \
    'data=""' \
    'url=""' \
    'while (($#)); do' \
    '  case "$1" in' \
    '    -X) method="$2"; shift 2 ;;' \
    '    -w) write_code=1; shift 2 ;;' \
    '    -d) data="$2"; shift 2 ;;' \
    '    -H|--connect-timeout|--max-time|-o) shift 2 ;;' \
    '    http*) url="$1"; shift ;;' \
    '    *) shift ;;' \
    '  esac' \
    'done' \
    'path="${url#https://api.example.test/v1}"' \
    'printf "%s %s\n" "$method" "$path" >>"$KV_VERIFY_CALLS"' \
    'case "$method $path" in' \
    '  "GET /key-value") [[ "${KV_VERIFY_API_PREFLIGHT_FAIL:-0}" != "1" ]] || exit 22; echo "[]" ;;' \
    '  "GET /key-value/red-fixture")' \
    '    if [[ -f "$KV_VERIFY_DELETED" ]]; then' \
    '      [[ "$write_code" == "1" ]] && printf 404' \
    '    elif [[ "${KV_VERIFY_RESOURCE_PENDING:-0}" == "1" ]]; then' \
    '      echo "{\"status\":\"creating\"}"' \
    '    else' \
    '      echo "{\"status\":\"available\",\"externalHost\":\"kv.example.test\"}"' \
    '    fi ;;' \
    '  "GET /key-value/red-fixture/connection-info")' \
    '    if [[ "${KV_VERIFY_MISSING_EDGE:-0}" == "1" ]]; then' \
    '      [[ "$write_code" == "1" ]] && printf 503' \
    '    elif [[ "$write_code" == "1" ]]; then printf 200' \
    '    else echo "{\"externalConnectionString\":\"rediss://default:synthetic@kv.example.test:6379\"}"; fi ;;' \
    '  "DELETE /key-value/red-fixture") touch "$KV_VERIFY_DELETED" ;;' \
    '  *) echo "unexpected fake curl call: $method $path" >&2; exit 90 ;;' \
    'esac' >"$fake_bin/curl"

  chmod +x "$fake_bin/render" "$fake_bin/redis-cli" "$fake_bin/dig" "$fake_bin/nc" "$fake_bin/go" "$fake_bin/curl"
}

write_fakes
export PATH="$fake_bin:$PATH"
export RENDER_HOST="https://api.example.test/v1/"
export RENDER_API_KEY="synthetic-token"
export RENDER_WORKSPACE="tea-synthetic"
export RENDER_CLI_CONFIG_PATH="$tmp/config/cli.yaml"
export RENDER_BIN="$fake_bin/render"
export BEX_KV_VERIFY_READY_TIMEOUT_SECONDS=2
export BEX_KV_VERIFY_CLEANUP_TIMEOUT_SECONDS=2
export KV_VERIFY_CALLS="$tmp/calls"
export KV_VERIFY_DELETED="$tmp/deleted"

# Required live inputs are checked before even the read-only API preflight.
: >"$KV_VERIFY_CALLS"
set +e
preflight_output="$(env -u BEX_KV_VERIFY_ALLOW_CIDR bash "$verify" 2>&1)"
preflight_status=$?
set -e
[[ "$preflight_status" != 0 && "$preflight_output" == *"BEX_KV_VERIFY_ALLOW_CIDR"* ]] || {
  echo "missing-CIDR preflight did not fail precisely: $preflight_output" >&2
  exit 1
}
[[ ! -s "$KV_VERIFY_CALLS" ]] || {
  echo "preflight touched the API before validating inputs" >&2
  exit 1
}

# A failed API/workspace read must stop before fixture creation.
: >"$KV_VERIFY_CALLS"
set +e
api_output="$(BEX_KV_VERIFY_ALLOW_CIDR=192.0.2.10/32 KV_VERIFY_API_PREFLIGHT_FAIL=1 bash "$verify" 2>&1)"
api_status=$?
set -e
[[ "$api_status" != 0 && "$api_output" == *"API/workspace preflight failed"* ]] || {
  echo "API/workspace preflight did not fail precisely: $api_output" >&2
  exit 1
}
if grep -q '^POST /key-value$' "$KV_VERIFY_CALLS"; then
  echo "API/workspace preflight failure created a fixture" >&2
  exit 1
fi

# A binary that reports the right version but is not built from the upstream
# module must fail before the API preflight or fixture mutation.
: >"$KV_VERIFY_CALLS"
set +e
module_output="$(BEX_KV_VERIFY_ALLOW_CIDR=192.0.2.10/32 KV_VERIFY_BAD_MODULE=1 bash "$verify" 2>&1)"
module_status=$?
set -e
[[ "$module_status" != 0 && "$module_output" == *"official upstream module"* && ! -s "$KV_VERIFY_CALLS" ]] || {
  echo "modified-module preflight did not fail before API access: $module_output" >&2
  exit 1
}

run_probe_failure() {
  rm -f "$KV_VERIFY_DELETED"
  : >"$KV_VERIFY_CALLS"
  export BEX_KV_VERIFY_ALLOW_CIDR="192.0.2.10/32"
  export KV_VERIFY_GO_FAIL=1
  set +e
  failure_output="$(bash "$verify" 2>&1)"
  failure_status=$?
  set -e
  [[ "$failure_status" != 0 && "$failure_output" == *"safe simulated probe failure"* ]] || {
    echo "probe failure did not propagate safely: $failure_output" >&2
    exit 1
  }
  [[ "$failure_output" == *"PASS disposable Key Value cleanup"* ]] || {
    echo "probe failure did not report cleanup: $failure_output" >&2
    exit 1
  }
  grep -Fxq 'DELETE /key-value/red-fixture' "$KV_VERIFY_CALLS" || {
    echo "probe failure skipped exact-id cleanup" >&2
    exit 1
  }
  [[ "$(grep -c '^DELETE /key-value/' "$KV_VERIFY_CALLS")" == "1" ]] || {
    echo "probe failure deleted an unexpected number of targets" >&2
    exit 1
  }
  grep -Eq '^go id=red-fixture name=kvcli-m57-[0-9]+-[0-9]+ family=-4$' "$KV_VERIFY_CALLS" || {
    echo "probe did not receive both targets and the CIDR-matched address family" >&2
    exit 1
  }
  grep -Fxq 'GET /key-value/red-fixture/connection-info' "$KV_VERIFY_CALLS" || {
    echo "probe skipped public connection-info readiness" >&2
    exit 1
  }
}

run_probe_failure

# A public-intent resource without a reconciled edge returns actionable config
# guidance and never launches the PTY/local redis client. Cleanup still runs.
rm -f "$KV_VERIFY_DELETED"
: >"$KV_VERIFY_CALLS"
unset KV_VERIFY_GO_FAIL
set +e
missing_edge_output="$(KV_VERIFY_MISSING_EDGE=1 bash "$verify" 2>&1)"
missing_edge_status=$?
set -e
[[ "$missing_edge_status" != 0 && "$missing_edge_output" == *"configure BEX_KV_DOMAIN"* ]] || {
  echo "missing-edge failure was not actionable: $missing_edge_output" >&2
  exit 1
}
! grep -Fq 'go id=' "$KV_VERIFY_CALLS" || {
  echo "missing edge invoked the local Key Value client" >&2
  exit 1
}
grep -Fxq 'DELETE /key-value/red-fixture' "$KV_VERIFY_CALLS"

# An interrupt after creation must take the same exact-id cleanup path.
rm -f "$KV_VERIFY_DELETED"
: >"$KV_VERIFY_CALLS"
export KV_VERIFY_RESOURCE_PENDING=1
bash "$verify" >"$tmp/interrupt.out" 2>&1 &
verify_pid=$!
fixture_observed=0
for _ in {1..50}; do
  if grep -Fxq 'GET /key-value/red-fixture' "$KV_VERIFY_CALLS"; then
    fixture_observed=1
    break
  fi
  sleep 0.05
done
[[ "$fixture_observed" == "1" ]] || {
  kill -TERM "$verify_pid" >/dev/null 2>&1 || true
  wait "$verify_pid" >/dev/null 2>&1 || true
  echo 'interrupt test did not observe the created fixture' >&2
  exit 1
}
kill -TERM "$verify_pid"
set +e
wait "$verify_pid"
interrupt_status=$?
set -e
unset KV_VERIFY_RESOURCE_PENDING
[[ "$interrupt_status" == "143" ]] || {
  echo "interrupt verifier status = $interrupt_status, want 143" >&2
  exit 1
}
assert_interrupt_cleanup="$(<"$tmp/interrupt.out")"
[[ "$assert_interrupt_cleanup" == *'PASS disposable Key Value cleanup'* ]] || {
  echo 'interrupt did not report disposable Key Value cleanup' >&2
  exit 1
}
[[ "$(grep -c '^DELETE /key-value/red-fixture$' "$KV_VERIFY_CALLS")" == "1" ]] || {
  echo 'interrupt did not delete the exact fixture once' >&2
  exit 1
}

rm -f "$KV_VERIFY_DELETED"
: >"$KV_VERIFY_CALLS"
unset KV_VERIFY_GO_FAIL
success_output="$(bash "$verify" 2>&1)"
for label in \
  'PASS official-CLI source-restricted public Key Value created' \
  'PASS public rediss endpoint ready' \
  'PASS official kv-cli by opaque id: PING and SET' \
  'PASS official kv-cli by display name: GET and DEL' \
  'PASS no attached human terminal required' \
  'PASS disposable Key Value cleanup'; do
  [[ "$success_output" == *"$label"* ]] || {
    echo "successful verifier omitted label: $label" >&2
    exit 1
  }
done
for forbidden in synthetic-token 'default:synthetic' 'rediss://'; do
  [[ "$success_output" != *"$forbidden"* ]] || {
    echo "successful verifier leaked synthetic sensitive output" >&2
    exit 1
  }
done
grep -Fxq 'DELETE /key-value/red-fixture' "$KV_VERIFY_CALLS"
grep -Eq '^CLI create kvcli-m57-[0-9]+-[0-9]+$' "$KV_VERIFY_CALLS"
if grep -Fxq 'POST /key-value' "$KV_VERIFY_CALLS"; then
  echo "verifier bypassed the official CLI for fixture creation" >&2
  exit 1
fi
echo "PASS Key Value CLI verifier preflight and exact cleanup"
