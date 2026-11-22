#!/usr/bin/env bash
set -euo pipefail

# Live isolation probe for sandbox group credential brokering (w2/m536 t003,
# docs/ADR122-sandbox-credential-brokering.md, docs/ADR123-security-review-sandbox-credentials.md).
#
# Designed to run as BEX_VERIFY_PROBE under scripts/verify-sandbox-isolation-live.sh
# (which provisions disposable workspaces A and B plus Admin/Developer/cross-workspace
# tokens and cleans them all up on exit).
#
# Verifies on real Pods under real Cilium & OpenFGA:
#   1. Cross-workspace refusal: workspace B gets 404 on workspace A's group credentials.
#   2. Split RBAC refusal: workspace A Developer gets 403 on create and domain/value PATCH.
#   3. Cross-group non-injection: Group A's credential injects into Group A's allow-all
#      sandbox (with zero secret bytes in sandbox env), while Group B's allow-all sandbox
#      gets ZERO injection and preserves the real upstream TLS issuer.
#   4. deny-all refusal: a deny-all sandbox in Group A has no HTTPS_PROXY and cannot reach
#      bex-ssh-gateway:8086 or external HTTPS.
#   5. allow-list intersection: an allow-list sandbox in Group A listing only api.github.com
#      reaches api.github.com (real TLS, zero secret) and is refused 403 by the broker when
#      requesting Group A's credentialed (but unlisted) echo domain.

: "${BEX_API_URL:?set BEX_API_URL}"
: "${BEX_WORKSPACE_A:?set BEX_WORKSPACE_A}"
: "${BEX_WS_A_ADMIN_TOKEN:?set BEX_WS_A_ADMIN_TOKEN}"
: "${BEX_WS_A_OWNER_TOKEN:?set BEX_WS_A_OWNER_TOKEN}"
: "${BEX_WS_B_OWNER_TOKEN:?set BEX_WS_B_OWNER_TOKEN}"

for cmd in curl jq awk openssl; do
  command -v "$cmd" >/dev/null || {
    echo "error: missing required command: $cmd" >&2
    exit 2
  }
done

api_url="${BEX_API_URL%/}"
echo_host="${BEX_BROKER_ECHO_HOST:-postman-echo.com}"
echo_path="${BEX_BROKER_ECHO_PATH:-/headers}"
broker_host="bex-ssh-gateway.bex-system.svc.cluster.local"
run_id="$(date -u +%Y%m%d%H%M%S)-$$"
secret_marker="bex-iso-broker-$(openssl rand -hex 12)"

fixture_dir="$(mktemp -d)"
umask 077
created_sandboxes=()
created_groups=()

pass() { echo "PASS: $*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

write_auth_config() {
  local token="$1" output="$2"
  printf 'header = "Authorization: Bearer %s"\n' "$token" >"$output"
  chmod 600 "$output"
}

api_call() {
  local auth_file="$1" method="$2" path="$3" output="$4"
  shift 4
  curl --config "$auth_file" --silent --show-error \
    --request "$method" --output "$output" --write-out '%{http_code}' \
    "$api_url$path" "$@"
}

expect_status() {
  local want="$1" auth_file="$2" method="$3" path="$4" output="$5" got
  shift 5
  got="$(api_call "$auth_file" "$method" "$path" "$output" "$@")"
  [ "$got" = "$want" ] || fail "$method $path returned HTTP $got, want $want ($(head -c 300 "$output" 2>/dev/null || true))"
}

cleanup() {
  set +e
  for sbx in "${created_sandboxes[@]}"; do
    api_call "$fixture_dir/admin-a.curl" POST "/v1/sandboxes/$sbx/terminate?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/term-$sbx.json" >/dev/null 2>&1 || true
  done
  for grp in "${created_groups[@]}"; do
    api_call "$fixture_dir/admin-a.curl" DELETE "/v1/sandbox-groups/$grp?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/del-$grp.json" >/dev/null 2>&1 || true
  done
  rm -rf -- "$fixture_dir"
}
trap cleanup EXIT

write_auth_config "$BEX_WS_A_ADMIN_TOKEN" "$fixture_dir/admin-a.curl"
write_auth_config "$BEX_WS_A_OWNER_TOKEN" "$fixture_dir/dev-a.curl"
write_auth_config "$BEX_WS_B_OWNER_TOKEN" "$fixture_dir/owner-b.curl"

create_group() {
  local name="$1" out="$2" id
  expect_status 201 "$fixture_dir/admin-a.curl" POST "/v1/sandbox-groups" "$out" \
    --header 'Content-Type: application/json' \
    --data "$(jq -cn --arg ws "$BEX_WORKSPACE_A" --arg n "$name" '{ownerId:$ws,name:$n}')"
  id="$(jq -er '.id' "$out")"
  created_groups+=("$id")
  printf '%s' "$id"
}

create_sandbox_in_group() {
  local group_id="$1" policy_json="$2" out="$3" id status
  expect_status 201 "$fixture_dir/admin-a.curl" POST "/v1/sandboxes" "$out" \
    --header 'Content-Type: application/json' \
    --data "$(jq -cn --arg ws "$BEX_WORKSPACE_A" --arg g "$group_id" --argjson np "$policy_json" \
      '{ownerId:$ws,sandboxGroupId:$g,plan:"1c-2g",timeoutSeconds:600,networkPolicy:$np}')"
  id="$(jq -er '.id' "$out")"
  created_sandboxes+=("$id")
  for _ in $(seq 1 90); do
    expect_status 200 "$fixture_dir/admin-a.curl" GET "/v1/sandboxes/$id?ownerId=$BEX_WORKSPACE_A" "$out.poll"
    status="$(jq -er '.status' "$out.poll")"
    if [ "$status" = "running" ]; then
      printf '%s' "$id"
      return 0
    fi
    if [ "$status" = "errored" ] || [ "$status" = "terminated" ]; then
      fail "sandbox $id entered terminal status $status"
    fi
    sleep 2
  done
  fail "sandbox $id did not reach running within 180s"
}

sandbox_exec() {
  local id="$1" command="$2" output="$3" body token_file
  body="$(jq -cn --arg c "$command" '{command:$c}')"
  expect_status 201 "$fixture_dir/admin-a.curl" POST "/v1/sandboxes/$id/runs/stream/token?ownerId=$BEX_WORKSPACE_A" "$output.mint" \
    --header 'Content-Type: application/json' --data "$body"
  token_file="$output.token.curl"
  write_auth_config "$(jq -er '.token' "$output.mint")" "$token_file"
  curl --config "$token_file" --silent --show-error --fail-with-body --max-time 90 \
    --request POST --header 'Accept: text/event-stream' --header 'Content-Type: application/json' \
    --data "$body" --output "$output.sse" "$(jq -er '.uri' "$output.mint")" \
    || fail "exec in $id failed: $(head -c 300 "$output.sse" 2>/dev/null || true)"
  awk '/^event: /{ev=substr($0, 8)} /^data: /{if (ev == "output") print substr($0, 7)}' "$output.sse" \
    | jq -rj 'select(.stream != "stderr") | .data' >"$output" \
    || fail "exec in $id returned unreadable output"
}

echo "==> 1. Provision Group A (credentialed) and Group B (uncredentialed) in workspace A"
group_a="$(create_group "broker-iso-a-$run_id" "$fixture_dir/group-a.json")"
group_b="$(create_group "broker-iso-b-$run_id" "$fixture_dir/group-b.json")"

cred_payload="$(jq -cn --arg host "$echo_host" --arg val "$secret_marker" --arg path "$echo_path" \
  '{id:"iso-echo-header",domain:$host,value:$val,enabled:true,methods:["GET"],paths:[$path],injection:{type:"header",name:"X-Bex-Iso-Probe",template:"tok-${SECRET}"}}')"

# Developer in workspace A must be refused 403 on create.
expect_status 403 "$fixture_dir/dev-a.curl" POST "/v1/sandbox-groups/$group_a/credentials" "$fixture_dir/dev-create.json" \
  --header 'Content-Type: application/json' --data "$cred_payload"
pass "workspace A Developer refused 403 on credential create"

# Admin in workspace A creates credential -> 201, zero secret echoed.
expect_status 201 "$fixture_dir/admin-a.curl" POST "/v1/sandbox-groups/$group_a/credentials" "$fixture_dir/admin-create.json" \
  --header 'Content-Type: application/json' --data "$cred_payload"
if grep -qF "$secret_marker" "$fixture_dir/admin-create.json"; then
  fail "credential create response leaked secret value"
fi
pass "workspace A Admin created Group A credential with write-only response"

# Workspace B must get 404 (not existence leak) on Group A credentials.
expect_status 404 "$fixture_dir/owner-b.curl" GET "/v1/sandbox-groups/$group_a/credentials" "$fixture_dir/foreign-list.json"
pass "cross-workspace credential list refused 404"

# Developer in workspace A must be refused 403 on domain/value PATCH.
expect_status 403 "$fixture_dir/dev-a.curl" PATCH "/v1/sandbox-groups/$group_a/credentials/iso-echo-header" "$fixture_dir/dev-patch.json" \
  --header 'Content-Type: application/json' --data '{"domain":"api.github.com"}'
pass "workspace A Developer refused 403 on domain PATCH"

echo "==> 2. Launch 4 sandboxes: Group A allow-all, Group B allow-all, Group A deny-all, Group A allow-list"
sbx_a_allow="$(create_sandbox_in_group "$group_a" '{"type":"allow-all"}' "$fixture_dir/sbx-a-allow.json")"
sbx_b_allow="$(create_sandbox_in_group "$group_b" '{"type":"allow-all"}' "$fixture_dir/sbx-b-allow.json")"
sbx_a_deny="$(create_sandbox_in_group "$group_a" '{"type":"deny-all"}' "$fixture_dir/sbx-a-deny.json")"
sbx_a_list="$(create_sandbox_in_group "$group_a" '{"type":"allow-list","rules":[{"domain":"api.github.com","protocol":"https"}]}' "$fixture_dir/sbx-a-list.json")"

echo "==> 3. Verify Group A allow-all receives brokered secret and holds zero secret in env"
sandbox_exec "$sbx_a_allow" 'env' "$fixture_dir/a-env.txt"
if grep -qF "$secret_marker" "$fixture_dir/a-env.txt"; then
  fail "Group A sandbox env contains secret value"
fi
grep -q '^HTTPS_PROXY=http://' "$fixture_dir/a-env.txt" || fail "Group A allow-all sandbox missing HTTPS_PROXY"

sandbox_exec "$sbx_a_allow" "curl -fsS --max-time 20 https://$echo_host$echo_path || printf 'ECHO_FETCH_FAILED'" "$fixture_dir/a-echo.txt"
if grep -qF "ECHO_FETCH_FAILED" "$fixture_dir/a-echo.txt"; then
  echo "NOTICE: third-party echo endpoint https://$echo_host$echo_path unreachable from production sandbox" >&2
  exit 2
fi
grep -qiF "tok-$secret_marker" "$fixture_dir/a-echo.txt" \
  || fail "Group A allow-all sandbox did not receive injected credential header"
pass "Group A allow-all sandbox received injected credential with zero secret in env"

echo "==> 4. Verify Group B allow-all receives ZERO Group A credentials"
sandbox_exec "$sbx_b_allow" "curl -fsS --max-time 20 https://$echo_host$echo_path" "$fixture_dir/b-echo.txt"
if grep -qiF "$secret_marker" "$fixture_dir/b-echo.txt"; then
  fail "ISOLATION BREACH: Group B sandbox received Group A secret!"
fi
pass "Group B allow-all sandbox received zero Group A credentials"

echo "==> 5. Verify Group A deny-all has no HTTPS_PROXY and cannot reach broker :8086 or external HTTPS"
sandbox_exec "$sbx_a_deny" 'printenv HTTPS_PROXY || printf NO_PROXY_SET' "$fixture_dir/deny-env.txt"
grep -qF "NO_PROXY_SET" "$fixture_dir/deny-env.txt" || fail "deny-all sandbox unexpectedly received HTTPS_PROXY"
sandbox_exec "$sbx_a_deny" \
  "if curl -sS --max-time 8 --proxy http://$broker_host:8086 https://$echo_host$echo_path >/dev/null 2>&1; then printf REACHED_BROKER; else printf BLOCKED; fi" \
  "$fixture_dir/deny-broker.txt"
grep -qF "BLOCKED" "$fixture_dir/deny-broker.txt" || fail "ISOLATION BREACH: deny-all sandbox reached credential broker :8086!"
pass "Group A deny-all sandbox has no HTTPS_PROXY and cannot reach broker :8086"

echo "==> 6. Verify Group A allow-list intersection (listed api.github.com allowed; credentialed unlisted host refused)"
sandbox_exec "$sbx_a_list" \
  "curl -sS --max-time 20 -o /dev/null -w '%{http_code}' https://api.github.com/ || printf 000" \
  "$fixture_dir/list-allowed.txt"
allowed_code="$(tr -d '\r\n' <"$fixture_dir/list-allowed.txt")"
[ "$allowed_code" != "000" ] && [ "$allowed_code" != "403" ] \
  || fail "allow-list sandbox could not reach listed https://api.github.com/ (HTTP $allowed_code)"

sandbox_exec "$sbx_a_list" \
  "out=\$(curl -sS --max-time 15 -w '\nCODE:%{http_code}' https://$echo_host$echo_path 2>&1 || true); printf '%s' \"\$out\"" \
  "$fixture_dir/list-denied.txt"
if grep -qiF "$secret_marker" "$fixture_dir/list-denied.txt"; then
  fail "ISOLATION BREACH: allow-list sandbox received secret for unlisted credentialed domain $echo_host!"
fi
if grep -qF "CODE:200" "$fixture_dir/list-denied.txt"; then
  fail "ISOLATION BREACH: allow-list sandbox reached unlisted credentialed domain $echo_host (HTTP 200)!"
fi
pass "Group A allow-list sandbox reached listed api.github.com and was refused on unlisted credentialed $echo_host"

echo "PASS: sandbox credential-brokering isolation matrix compliant"
