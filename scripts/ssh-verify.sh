#!/usr/bin/env bash
set -euo pipefail

# Redacting public-edge acceptance for docs/ADR035-ssh.md. The default mode
# creates and cleans up a paid two-replica nginx fixture. Supplying
# BEX_SSH_VERIFY_SERVICE uses an existing disposable service instead; it must
# have BEX_SSH_SMOKE_VALUE=w2-m39-runtime and may be restarted/suspended,
# resized, and temporarily switched to a shell-less image by this script.
#
# BEX_SSH_VERIFY_FULL_MATRIX=1 additionally requires pre-arranged viewer and
# foreign-workspace identities plus existing static/cron targets. That setup is
# intentionally out of band because the acceptance token must not be able to
# manufacture its own weaker workspace role or foreign workspace.

for command in curl go jq ssh ssh-keygen ssh-keyscan; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

: "${BEX_API_URL:?set BEX_API_URL, e.g. https://api.bex.co}"
: "${BEX_API_TOKEN:?set a bearer token without echoing it}"
: "${BEX_SSH_VERIFY_PRIVATE_KEY_FILE:?set a disposable private-key file path}"
: "${BEX_SSH_EXPECTED_HOST_FINGERPRINT:?set the published SHA256 host-key fingerprint}"

api="${BEX_API_URL%/}"
private_key="$BEX_SSH_VERIFY_PRIVATE_KEY_FILE"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
real_ssh="$(command -v ssh)"
public_key="$(ssh-keygen -y -f "$private_key")"
fingerprint="$(printf '%s\n' "$public_key" | ssh-keygen -lf - | awk '{print $2}')"
tmp="$(mktemp -d)"
key_id=""
viewer_key_id=""
service_id=""
service_name=""
service_created=0
session_pid=""
agent_started=0

cleanup() {
  if [[ -n "$session_pid" ]]; then
    kill "$session_pid" >/dev/null 2>&1 || true
    wait "$session_pid" >/dev/null 2>&1 || true
  fi
  if [[ -n "$viewer_key_id" && -n "${BEX_SSH_VERIFY_VIEWER_TOKEN:-}" ]]; then
    curl -fsS -X DELETE -H "Authorization: Bearer $BEX_SSH_VERIFY_VIEWER_TOKEN" \
      "$api/v1/ssh-keys/$viewer_key_id" >/dev/null 2>&1 || true
  fi
  if [[ -n "$key_id" ]]; then
    curl -fsS -X DELETE -H "Authorization: Bearer $BEX_API_TOKEN" \
      "$api/v1/ssh-keys/$key_id" >/dev/null 2>&1 || true
  fi
  if [[ "$service_created" == "1" && -n "$service_id" ]]; then
    curl -fsS -X DELETE -H "Authorization: Bearer $BEX_API_TOKEN" \
      "$api/v1/services/$service_id" >/dev/null 2>&1 || true
  fi
  if [[ "$agent_started" == "1" ]]; then
    ssh-agent -k >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

auth=(-H "Authorization: Bearer $BEX_API_TOKEN")

api_json() {
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -fsS -X "$method" "${auth[@]}" -H 'Content-Type: application/json' -d "$body" "$api$path"
  else
    curl -fsS -X "$method" "${auth[@]}" "$api$path"
  fi
}

fail() {
  echo "FAIL $*" >&2
  exit 1
}

assert_rejected() {
  local address="$1" key_file="${2:-$private_key}"
  if ssh "${ssh_opts[@]}" -i "$key_file" "$address" 'exit 0' >/dev/null 2>&1; then
    fail "$3"
  fi
}

wait_ready() {
  local require_new="${1:-}" deadline=$((SECONDS + ${BEX_SSH_VERIFY_READY_TIMEOUT_SECONDS:-300}))
  while ((SECONDS < deadline)); do
    service_json="$(api_json GET "/v1/services/$service_id")"
    instances="$(api_json GET "/v1/services/$service_id/instances")"
    ssh_address="$(jq -r '.serviceDetails.sshAddress // empty' <<<"$service_json")"
    instance_id="$(jq -r '.[0].id // empty' <<<"$instances")"
    old_gone=1
    if [[ -n "$require_new" ]] && ! jq -e --arg old "$require_new" 'all(.[]; .id != $old)' <<<"$instances" >/dev/null; then
      old_gone=0
    fi
    if [[ "$(jq -r '.phase // empty' <<<"$service_json")" == "Running" && -n "$ssh_address" && \
      "$(jq 'length' <<<"$instances")" -ge 2 && -n "$instance_id" && "$old_gone" == "1" ]]; then
      return 0
    fi
    sleep 3
  done
  fail "service did not reach Running with two Ready current-image instances"
}

if [[ -n "${BEX_SSH_VERIFY_SERVICE:-}" ]]; then
  services="$(api_json GET "/v1/services?name=$(jq -rn --arg v "$BEX_SSH_VERIFY_SERVICE" '$v|@uri')")"
  service_json="$(jq -cer '.[0].service' <<<"$services")"
  service_id="$(jq -er '.id' <<<"$service_json")"
  service_name="$(jq -er '.name' <<<"$service_json")"
else
  service_name="ssh-verify-$(date +%s)-$$"
  create_payload="$(jq -n --arg name "$service_name" '{
    type:"web_service", name:$name, image:{imagePath:"nginxinc/nginx-unprivileged:1.27-alpine"}, port:8080,
    serviceDetails:{plan:"starter",numInstances:2,healthCheckPath:"/"},
    envVars:[{key:"BEX_SSH_SMOKE_VALUE",value:"w2-m39-runtime"}]
  }')"
  created_service="$(api_json POST /v1/services "$create_payload")"
  service_id="$(jq -er '.service.id' <<<"$created_service")"
  service_created=1
  echo "created disposable two-replica service id=$service_id"
fi

wait_ready
original_image="$(jq -er '.imagePath' <<<"$service_json")"
original_plan="$(jq -er '.serviceDetails.plan' <<<"$service_json")"
workspace_id="$(jq -er '.ownerId' <<<"$service_json")"
host="${ssh_address#*@}"
specific_address="$instance_id@$host"
echo "resolved service id=$service_id and Ready instance id=$instance_id"

ssh-keyscan -T 10 -p 22 "$host" >"$tmp/known_hosts" 2>/dev/null
observed_host_fingerprint="$(ssh-keygen -lf "$tmp/known_hosts" | awk 'NR == 1 {print $2}')"
[[ "$observed_host_fingerprint" == "$BEX_SSH_EXPECTED_HOST_FINGERPRINT" ]] || fail "public host fingerprint mismatch"
echo "PASS public TCP/22 host fingerprint=$observed_host_fingerprint"

ssh_opts=(-o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=yes \
  -o "UserKnownHostsFile=$tmp/known_hosts" -o IdentitiesOnly=yes)

# The fixture key is deliberately unknown before registration.
assert_rejected "$ssh_address" "$private_key" "unknown key authenticated"
echo "PASS unknown key rejected"

payload="$(jq -n --arg name "w2-m39-live-verify" --arg publicKey "$public_key" '{name:$name,publicKey:$publicKey}')"
created="$(api_json POST /v1/ssh-keys "$payload")"
key_id="$(jq -er '.id' <<<"$created")"
echo "registered key id=$key_id fingerprint=$fingerprint"

if ssh "${ssh_opts[@]}" -i "$private_key" "$ssh_address" \
  'test "$BEX_SSH_SMOKE_VALUE" = "w2-m39-runtime"' >/dev/null 2>&1; then
  echo "PASS raw OpenSSH any-instance and runtime environment"
else
  fail "raw OpenSSH any-instance/runtime environment"
fi

set +e
ssh "${ssh_opts[@]}" -i "$private_key" "$specific_address" 'exit 23' >/dev/null 2>&1
specific_rc=$?
set -e
[[ "$specific_rc" == "23" ]] || fail "specific-instance exit status (got $specific_rc)"
echo "PASS specific-instance and exit status"

# The opt-in Go probe keeps terminal bytes in memory and prints no content.
(
  cd "$repo_root/lego/backend"
  BEX_TEST_SSH_PUBLIC_ADDR="$host:22" \
    BEX_TEST_SSH_PUBLIC_USER="$instance_id" \
    BEX_TEST_SSH_PRIVATE_KEY_FILE="$private_key" \
    BEX_TEST_SSH_HOST_FINGERPRINT="$observed_host_fingerprint" \
    go test ./internal/sshgateway -run '^TestPublicGatewayPTYResize$' -count=1 >/dev/null
) || fail "public PTY/resize/runtime probe"
echo "PASS interactive PTY and resize"

# The official CLI is intentionally interactive. Current render-oss/cli accepts
# a service name for any-instance SSH and a complete instance id for an exact
# replica. Its service-id instance-menu callback currently drops the selected
# id, so this verifier does not mistake that upstream defect for bex evidence:
# it exercises the two working public arguments directly and records only the
# destination the unmodified CLI gave OpenSSH.
if [[ "${BEX_RENDER_CLI_VERIFY:-0}" == "1" ]]; then
  command -v render >/dev/null || fail "missing render CLI"
  command -v ssh-agent >/dev/null || fail "missing ssh-agent"
  command -v ssh-add >/dev/null || fail "missing ssh-add"
  [[ -t 0 ]] || fail "official CLI verification requires a TTY"
  cli_version="$(render --version 2>&1 | head -n 1)"
  [[ -n "$cli_version" ]] || fail "official Render CLI did not report its version"
  echo "official Render CLI $cli_version"
  ssh-agent -a "$tmp/agent.sock" >"$tmp/agent.env"
  # shellcheck disable=SC1090 -- generated by the local OpenSSH ssh-agent.
  . "$tmp/agent.env" >/dev/null
  agent_started=1
  ssh-add "$private_key" >/dev/null 2>&1
  mkdir -p "$tmp/cli-bin"
  ln -s "$repo_root/scripts/ssh-verify-openssh.sh" "$tmp/cli-bin/ssh"
  cli_target_log="$tmp/cli-target"
  cli_env=(
    "PATH=$tmp/cli-bin:$PATH"
    "BEX_SSH_VERIFY_REAL_SSH=$real_ssh"
    "BEX_SSH_VERIFY_CLI_KNOWN_HOSTS=$tmp/known_hosts"
    "BEX_SSH_VERIFY_CLI_TARGET_LOG=$cli_target_log"
    "RENDER_HOST=$api/v1/"
    "RENDER_API_KEY=$BEX_API_TOKEN"
    "RENDER_WORKSPACE=$workspace_id"
  )

  echo 'CLI CHECK: resize the terminal, run `test "$BEX_SSH_SMOKE_VALUE" = "w2-m39-runtime" && exit`, and require a zero exit'
  env "${cli_env[@]}" render ssh "$service_name"
  [[ "$(<"$cli_target_log")" == "$ssh_address" ]] || fail "Render CLI service-name path selected an unexpected destination"
  echo "PASS official Render CLI by service name"

  env "${cli_env[@]}" render ssh "$instance_id" -- 'test "$BEX_SSH_SMOKE_VALUE" = "w2-m39-runtime"' >/dev/null
  [[ "$(<"$cli_target_log")" == "$specific_address" ]] || fail "Render CLI instance-id path selected an unexpected destination"
  echo "PASS official Render CLI exact instance and runtime environment"
else
  echo "SKIP official Render CLI (set BEX_RENDER_CLI_VERIFY=1 in a TTY)"
fi

# Hold a stream, restart the fixture, require closure, then prove the old
# instance id is no longer a valid target (the live non-Ready/stale case).
old_instance_id="$instance_id"
ssh "${ssh_opts[@]}" -i "$private_key" "$specific_address" 'while :; do sleep 1; done' >/dev/null 2>&1 &
session_pid=$!
sleep 2
kill -0 "$session_pid" >/dev/null 2>&1 || fail "restart probe SSH session did not stay open"
api_json POST "/v1/services/$service_id/restart" >/dev/null
restart_deadline=$((SECONDS + ${BEX_SSH_VERIFY_RESTART_TIMEOUT_SECONDS:-180}))
while kill -0 "$session_pid" >/dev/null 2>&1 && ((SECONDS < restart_deadline)); do sleep 2; done
if kill -0 "$session_pid" >/dev/null 2>&1; then fail "restart did not close attached SSH session"; fi
wait "$session_pid" >/dev/null 2>&1 || true
session_pid=""
echo "PASS restart closed attached session"
wait_ready "$old_instance_id"
assert_rejected "$old_instance_id@$host" "$private_key" "stale/non-Ready instance authenticated"
echo "PASS stale/non-Ready instance rejected"

# Suspended and free services must fail before session creation. Restore each
# state and wait for two Ready instances before continuing.
api_json POST "/v1/services/$service_id/suspend" >/dev/null
assert_rejected "$service_id@$host" "$private_key" "suspended service authenticated"
echo "PASS suspended service rejected"
api_json POST "/v1/services/$service_id/resume" >/dev/null
wait_ready

pre_free_instance="$instance_id"
api_json PATCH "/v1/services/$service_id" '{"serviceDetails":{"plan":"free"}}' >/dev/null
free_view="$(api_json GET "/v1/services/$service_id")"
[[ -z "$(jq -r '.serviceDetails.sshAddress // empty' <<<"$free_view")" ]] || fail "free service advertised sshAddress"
assert_rejected "$service_id@$host" "$private_key" "free service authenticated"
echo "PASS free service rejected and address omitted"
api_json PATCH "/v1/services/$service_id" "$(jq -n --arg plan "$original_plan" '{serviceDetails:{plan:$plan}}')" >/dev/null
wait_ready "$pre_free_instance"

# Hold another stream while a real image redeploy replaces the pod. This is
# separate from the restart check above: both lifecycle paths must close their
# attached Kubernetes exec stream rather than orphaning it on the gateway.
# The new image is intentionally shell-less, so the same rollout then exercises
# the bounded exit-126 contract before the original image is restored.
pre_shellless_instance="$instance_id"
pre_shellless_address="$instance_id@$host"
ssh "${ssh_opts[@]}" -i "$private_key" "$pre_shellless_address" 'while :; do sleep 1; done' >/dev/null 2>&1 &
session_pid=$!
sleep 2
kill -0 "$session_pid" >/dev/null 2>&1 || fail "redeploy probe SSH session did not stay open"
api_json PATCH "/v1/services/$service_id" '{"image":{"imagePath":"traefik/whoami:v1.11.0"}}' >/dev/null
redeploy_deadline=$((SECONDS + ${BEX_SSH_VERIFY_REDEPLOY_TIMEOUT_SECONDS:-180}))
while kill -0 "$session_pid" >/dev/null 2>&1 && ((SECONDS < redeploy_deadline)); do sleep 2; done
if kill -0 "$session_pid" >/dev/null 2>&1; then fail "redeploy did not close attached SSH session"; fi
wait "$session_pid" >/dev/null 2>&1 || true
session_pid=""
echo "PASS redeploy closed attached session"
wait_ready "$pre_shellless_instance"
shellless_address="$instance_id@$host"
set +e
shellless_output="$(ssh "${ssh_opts[@]}" -i "$private_key" "$shellless_address" 'true' 2>&1 >/dev/null)"
shellless_rc=$?
set -e
[[ "$shellless_rc" == "126" && ${#shellless_output} -lt 256 && "$shellless_output" == *"unable to start /bin/sh in this image"* ]] \
  || fail "shell-less image did not return bounded exit 126"
shellless_output=""
echo "PASS shell-less image bounded exit 126"
shellless_instance="$instance_id"
api_json PATCH "/v1/services/$service_id" "$(jq -n --arg image "$original_image" '{image:{imagePath:$image}}')" >/dev/null
wait_ready "$shellless_instance"

# A syntactically valid but absent service id exercises the same generic denial
# shape as a hidden foreign target without claiming cross-workspace evidence.
unknown_service_id="${service_id%?}"
if [[ "${service_id: -1}" == "z" ]]; then unknown_service_id+="y"; else unknown_service_id+="z"; fi
assert_rejected "$unknown_service_id@$host" "$private_key" "unknown service authenticated"
echo "PASS unknown service rejected"

if [[ "${BEX_SSH_VERIFY_FULL_MATRIX:-0}" == "1" ]]; then
  : "${BEX_SSH_VERIFY_VIEWER_TOKEN:?full matrix requires a viewer token in the fixture workspace}"
  : "${BEX_SSH_VERIFY_VIEWER_PRIVATE_KEY_FILE:?full matrix requires a distinct viewer key}"
  : "${BEX_SSH_VERIFY_FOREIGN_SERVICE_ID:?full matrix requires a known service in a foreign workspace}"
  : "${BEX_SSH_VERIFY_STATIC_SERVICE_ID:?full matrix requires a static-site service id}"
  : "${BEX_SSH_VERIFY_CRON_SERVICE_ID:?full matrix requires a cron service id}"

  viewer_public_key="$(ssh-keygen -y -f "$BEX_SSH_VERIFY_VIEWER_PRIVATE_KEY_FILE")"
  viewer_payload="$(jq -n --arg name "w2-m39-viewer-denial" --arg publicKey "$viewer_public_key" '{name:$name,publicKey:$publicKey}')"
  viewer_created="$(curl -fsS -H "Authorization: Bearer $BEX_SSH_VERIFY_VIEWER_TOKEN" -H 'Content-Type: application/json' \
    -d "$viewer_payload" "$api/v1/ssh-keys")"
  viewer_key_id="$(jq -er '.id' <<<"$viewer_created")"
  assert_rejected "$service_id@$host" "$BEX_SSH_VERIFY_VIEWER_PRIVATE_KEY_FILE" "viewer authenticated"
  echo "PASS viewer rejected"

  assert_rejected "$BEX_SSH_VERIFY_FOREIGN_SERVICE_ID@$host" "$private_key" "foreign-workspace service authenticated"
  assert_rejected "$BEX_SSH_VERIFY_STATIC_SERVICE_ID@$host" "$private_key" "static service authenticated"
  assert_rejected "$BEX_SSH_VERIFY_CRON_SERVICE_ID@$host" "$private_key" "cron service authenticated"
  echo "PASS foreign workspace, static, and cron services rejected"
else
  echo "SKIP viewer/foreign/static/cron live denials (set BEX_SSH_VERIFY_FULL_MATRIX=1 with fixtures)"
fi

api_json DELETE "/v1/ssh-keys/$key_id" >/dev/null
key_id=""
assert_rejected "$service_id@$host" "$private_key" "deleted key authenticated"
echo "PASS deleted key rejected"

if [[ "$service_created" == "1" ]]; then
  api_json DELETE "/v1/services/$service_id" >/dev/null
  service_created=0
  echo "PASS disposable service deleted"
fi

echo "PASS SSH public-edge acceptance completed"
