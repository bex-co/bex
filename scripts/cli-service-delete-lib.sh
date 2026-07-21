#!/usr/bin/env bash
# Shared bounded deletion checks for the official Render CLI service verifier.
# The caller supplies API, RENDER_API_KEY, RENDER_BIN, TMP_DIR, and the
# CREATED_IDS/CREATED_NAMES arrays. Probe functions set state variables instead
# of returning authenticated response bodies, keeping diagnostics redaction-safe.

assert_service_fixture_names() {
  local name max_len="${BEX_CLI_SERVICE_NAME_MAX:-30}"
  for name in "$@"; do
    if [ -z "$name" ] || [ "${#name}" -gt "$max_len" ] || [[ ! "$name" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]]; then
      echo "FAIL: verifier fixture name '$name' violates the ${max_len}-character DNS-label contract" >&2
      return 1
    fi
  done
}

probe_service_get() {
  local id="$1" response_file="$TMP_DIR/service-get-probe.json" status
  status="$(curl -sS -o "$response_file" -w '%{http_code}' \
    --connect-timeout 3 --max-time 10 \
    -H "Authorization: Bearer $RENDER_API_KEY" "$API/services/$id" 2>/dev/null || true)"
  SERVICE_GET_STATE="$status"
  SERVICE_GET_PHASE="unknown"
  if [ "$status" = "200" ]; then
    SERVICE_GET_PHASE="$(jq -r '.phase // .status // "visible"' "$response_file" 2>/dev/null || echo unknown)"
  fi
  [ ! -f "$response_file" ] || unlink "$response_file"
}

classify_service_list_response() {
  local response_file="$1" id="$2"
  if ! jq -e '. == null or type == "array"' "$response_file" >/dev/null 2>&1; then
    SERVICE_LIST_STATE="error"
  elif jq -e --arg id "$id" '(. // []) | any(.id == $id)' "$response_file" >/dev/null 2>&1; then
    SERVICE_LIST_STATE="present"
  else
    SERVICE_LIST_STATE="absent"
  fi
}

probe_service_list() {
  local id="$1" response_file="$TMP_DIR/service-list-probe.json"
  if ! "$RENDER_BIN" services -o json >"$response_file" 2>/dev/null; then
    SERVICE_LIST_STATE="error"
  else
    classify_service_list_response "$response_file" "$id"
  fi
  [ ! -f "$response_file" ] || unlink "$response_file"
}

wait_service_gone() {
  local id="$1" name="$2" timeout="${3:-${BEX_CLI_SERVICES_DELETE_TIMEOUT_SECONDS:-300}}"
  local deadline=$((SECONDS + timeout))
  while true; do
    probe_service_get "$id"
    probe_service_list "$id"
    if [ "$SERVICE_GET_STATE" = "404" ] && [ "$SERVICE_LIST_STATE" = "absent" ]; then
      echo "PASS: service $name reached GET 404 and official-CLI list absence"
      return 0
    fi
    if (( SECONDS >= deadline )); then
      echo "FAIL: service $name deletion did not converge within ${timeout}s (GET=$SERVICE_GET_STATE phase=$SERVICE_GET_PHASE list=$SERVICE_LIST_STATE)" >&2
      return 1
    fi
    sleep "${BEX_CLI_SERVICES_DELETE_POLL_SECONDS:-2}"
  done
}

delete_service_fixture() {
  "$RENDER_BIN" services delete "$1" --confirm -o json >/dev/null 2>&1
}

cleanup_created_services() {
  local index failed=0
  # Acknowledge every deletion first so finalizers can converge concurrently.
  for index in "${!CREATED_IDS[@]}"; do
    if ! delete_service_fixture "${CREATED_IDS[$index]}"; then
      echo "WARN: cleanup DELETE failed for ${CREATED_NAMES[$index]} (${CREATED_IDS[$index]})" >&2
      failed=1
    fi
  done
  for index in "${!CREATED_IDS[@]}"; do
    if ! wait_service_gone "${CREATED_IDS[$index]}" "${CREATED_NAMES[$index]}"; then
      failed=1
    fi
  done
  return "$failed"
}
