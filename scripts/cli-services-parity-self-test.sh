#!/usr/bin/env bash
# Dependency-light mutation tests for the verifier's bounded deletion logic.
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=cli-service-delete-lib.sh
source scripts/cli-service-delete-lib.sh

TMP_DIR="$(mktemp -d)"
API="https://redacted.invalid/v1"
RENDER_API_KEY="planted-secret-must-never-appear"
RENDER_BIN="render-not-called"

cleanup_self_test() {
  [ ! -d "$TMP_DIR" ] || rmdir "$TMP_DIR"
}
trap cleanup_self_test EXIT

TEST_GET_STATE=404
TEST_GET_PHASE=absent
TEST_LIST_STATE=absent
probe_service_get() {
  SERVICE_GET_STATE="$TEST_GET_STATE"
  SERVICE_GET_PHASE="$TEST_GET_PHASE"
}
probe_service_list() {
  SERVICE_LIST_STATE="$TEST_LIST_STATE"
}

wait_service_gone srv-test fixture 0 >/dev/null

LIST_FIXTURE="$TMP_DIR/list.json"
printf '%s' '{"id":"srv-test"}' >"$LIST_FIXTURE"
classify_service_list_response "$LIST_FIXTURE" srv-test
[ "$SERVICE_LIST_STATE" = error ] || { echo "FAIL: object-shaped CLI list was treated as absence" >&2; exit 1; }
printf '%s' '[{"id":"srv-test"}]' >"$LIST_FIXTURE"
classify_service_list_response "$LIST_FIXTURE" srv-test
[ "$SERVICE_LIST_STATE" = present ] || { echo "FAIL: CLI list fixture did not detect the service" >&2; exit 1; }
printf '%s' '[]' >"$LIST_FIXTURE"
classify_service_list_response "$LIST_FIXTURE" srv-test
[ "$SERVICE_LIST_STATE" = absent ] || { echo "FAIL: empty CLI list was not treated as absence" >&2; exit 1; }
unlink "$LIST_FIXTURE"

expect_wait_failure() {
  local description="$1"
  local output status
  set +e
  output="$(wait_service_gone srv-test fixture 0 2>&1)"
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    echo "FAIL: self-test accepted $description" >&2
    exit 1
  fi
  if grep -q "$RENDER_API_KEY" <<<"$output"; then
    echo "FAIL: verifier diagnostics leaked the planted bearer sentinel" >&2
    exit 1
  fi
}

TEST_GET_STATE=200
TEST_GET_PHASE=Building
TEST_LIST_STATE=absent
expect_wait_failure "stuck GET visibility"

TEST_GET_STATE=404
TEST_GET_PHASE=absent
TEST_LIST_STATE=present
expect_wait_failure "stuck official-CLI list visibility"

TEST_GET_STATE=000
TEST_GET_PHASE=unknown
TEST_LIST_STATE=error
expect_wait_failure "failed GET/list probes"

THIRTY_CHARS="$(printf 'a%.0s' {1..30})"
assert_service_fixture_names short-fixture "$THIRTY_CHARS"
if assert_service_fixture_names this-fixture-name-is-deliberately-over-thirty >/dev/null 2>&1; then
  echo "FAIL: self-test accepted an overlong fixture name" >&2
  exit 1
fi

CREATED_IDS=(srv-one srv-two)
CREATED_NAMES=(one two)
DELETE_CALLS=()
WAIT_CALLS=()
delete_service_fixture() {
  DELETE_CALLS+=("$1")
}
wait_service_gone() {
  WAIT_CALLS+=("$1:$2")
}
cleanup_created_services
if [ "${DELETE_CALLS[*]}" != "srv-one srv-two" ] || [ "${WAIT_CALLS[*]}" != "srv-one:one srv-two:two" ]; then
  echo "FAIL: interrupted-cleanup simulation did not delete and await every fixture" >&2
  exit 1
fi

if ! grep -q '^trap cleanup EXIT$' scripts/cli-services-parity-verify.sh ||
  ! grep -q "trap 'exit 130' INT" scripts/cli-services-parity-verify.sh ||
  ! grep -q "trap 'exit 143' TERM" scripts/cli-services-parity-verify.sh; then
  echo "FAIL: verifier lost its EXIT/INT/TERM cleanup traps" >&2
  exit 1
fi

echo "PASS: verifier rejects stuck GET/list, probe errors, overlong names, leaks, and incomplete trap cleanup"
