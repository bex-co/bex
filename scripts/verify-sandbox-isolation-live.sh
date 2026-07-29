#!/usr/bin/env bash
set -euo pipefail

# Provision disposable production-equivalent principals/workspaces for the
# w3/m35 adversarial sandbox verifier, run the verifier, then remove every
# credential, authorization tuple, workspace, and local secret file.
#
# The core verifier intentionally accepts pre-provisioned bearer tokens. This
# wrapper creates four distinct machine principals through the real Hydra,
# tenant store, and OpenFGA paths so the owner/member/admin/cross-workspace
# matrix can be repeated without keeping long-lived test users or credentials.
#
# Required:
#   BEX_LIVE_VERIFY=1
#   BEX_API_URL=https://api.example.com
#   KUBECONFIG=/path/to/app-cluster-admin.kubeconfig
#
# Cluster defaults can be overridden for non-production installations:
#   BEX_AUTH_NAMESPACE=auth
#   BEX_API_NAMESPACE=bex-system
#   BEX_OPENFGA_TOKEN_SECRET=bex-openfga
#   BEX_OPENFGA_TOKEN_KEY=token

cd "$(dirname "$0")/.."

[ "${BEX_LIVE_VERIFY:-0}" = 1 ] || {
  echo "error: set BEX_LIVE_VERIFY=1 to authorize disposable live verification fixtures" >&2
  exit 2
}
: "${BEX_API_URL:?set BEX_API_URL to the bex-api origin}"
: "${KUBECONFIG:?set KUBECONFIG to the target app-cluster kubeconfig}"

for command in bash base64 curl jq kubectl openssl python3; do
  command -v "$command" >/dev/null || {
    echo "error: missing required command: $command" >&2
    exit 2
  }
done

auth_namespace="${BEX_AUTH_NAMESPACE:-auth}"
api_namespace="${BEX_API_NAMESPACE:-bex-system}"
fga_secret="${BEX_OPENFGA_TOKEN_SECRET:-bex-openfga}"
fga_secret_key="${BEX_OPENFGA_TOKEN_KEY:-token}"
api_url="${BEX_API_URL%/}"
run_id="$(date -u +%Y%m%d%H%M%S)-$$"
workspace_name_a="m35-a-$(date -u +%m%d%H%M%S)-$((BASHPID % 10000))"
workspace_name_b="m35-b-$(date -u +%m%d%H%M%S)-$((BASHPID % 10000))"
bootstrap_client="m35-bootstrap-$run_id"
bootstrap_secret="$(openssl rand -hex 32)"
fixture_dir="$(mktemp -d)"
umask 077

forward_pids=()
bootstrap_created=false
fga_tuple_written=false
workspace_a=""
workspace_b=""
key_owner_a=""
key_member_b=""
key_admin_a=""
key_owner_b=""

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

free_port() {
  python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()'
}

hydra_public_port="$(free_port)"
hydra_admin_port="$(free_port)"
openfga_port="$(free_port)"
hydra_public="http://127.0.0.1:$hydra_public_port"
hydra_admin="http://127.0.0.1:$hydra_admin_port"
openfga="http://127.0.0.1:$openfga_port"

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

graphql_call() {
  local auth_file="$1" query="$2" variables="$3" output="$4"
  local payload code
  payload="$(jq -nc --arg query "$query" --argjson variables "$variables" \
    '{query:$query,variables:$variables}')"
  code="$(api_call "$auth_file" POST /graphql "$output" \
    --header 'Content-Type: application/json' --data-binary "$payload")"
  [ "$code" = 200 ] || fail "GraphQL returned HTTP $code"
  jq -e '((.errors // []) | length) == 0' "$output" >/dev/null \
    || fail "GraphQL returned an application error"
}

fga_call() {
  local method="$1" path="$2" output="$3"
  shift 3
  curl --config "$fixture_dir/openfga.curl" --silent --show-error \
    --request "$method" --output "$output" --write-out '%{http_code}' \
    "$openfga$path" "$@"
}

token_for() {
  local client_id="$1" client_secret="$2" label="$3"
  local form="$fixture_dir/token-$label.form" output="$fixture_dir/token-$label.json" code
  printf 'grant_type=client_credentials&client_id=%s&client_secret=%s' \
    "$client_id" "$client_secret" >"$form"
  code="$(curl --silent --show-error --output "$output" --write-out '%{http_code}' \
    --header 'Content-Type: application/x-www-form-urlencoded' \
    --data-binary "@$form" "$hydra_public/oauth2/token")"
  [ "$code" = 200 ] || fail "Hydra token exchange for $label returned HTTP $code"
  jq -er '.access_token' "$output"
}

create_api_key() {
  local workspace="$1" name="$2" output="$3" code
  code="$(api_call "$fixture_dir/bootstrap.curl" POST /v1/api-keys "$output" \
    --header 'Content-Type: application/json' \
    --data-binary "$(jq -nc --arg name "$name" --arg owner "$workspace" \
      '{name:$name,ownerId:$owner}')")"
  [ "$code" = 201 ] || fail "creating disposable API key $name returned HTTP $code"
  jq -e '(.id | length > 0) and (.secret | length > 0)' "$output" >/dev/null \
    || fail "creating disposable API key $name returned no credential"
}

delete_workspace() {
  local id="$1" name="$2" label="$3"
  [ -n "$id" ] || return 0
  graphql_call "$fixture_dir/bootstrap.curl" \
    'mutation($id:String!,$confirmation:String!){deleteWorkspace(id:$id,confirmation:$confirmation)}' \
    "$(jq -nc --arg id "$id" --arg confirmation "sudo delete workspace $name" \
      '{id:$id,confirmation:$confirmation}')" \
    "$fixture_dir/delete-workspace-$label.json" >/dev/null 2>&1 || true
}

cleanup() {
  set +e

  # Keep the bootstrap authority until product-level cleanup has run. Direct
  # Hydra deletes below are a fallback for a partially failed API cleanup.
  if [ -s "$fixture_dir/bootstrap.curl" ]; then
    for tuple in \
      "$key_owner_a:$workspace_a" \
      "$key_member_b:$workspace_a" \
      "$key_admin_a:$workspace_a" \
      "$key_owner_b:$workspace_b"; do
      key_id="${tuple%%:*}"
      workspace_id="${tuple#*:}"
      if [ -n "$key_id" ] && [ -n "$workspace_id" ]; then
        api_call "$fixture_dir/bootstrap.curl" DELETE \
          "/v1/api-keys/$key_id?ownerId=$workspace_id" \
          "$fixture_dir/delete-$key_id.json" >/dev/null 2>&1 || true
      fi
    done
    delete_workspace "$workspace_a" "$workspace_name_a" a
    delete_workspace "$workspace_b" "$workspace_name_b" b
  fi

  for key_id in "$key_owner_a" "$key_member_b" "$key_admin_a" "$key_owner_b"; do
    [ -n "$key_id" ] || continue
    curl --silent --request DELETE "$hydra_admin/admin/clients/$key_id" >/dev/null 2>&1 || true
  done
  if [ "$bootstrap_created" = true ]; then
    curl --silent --request DELETE "$hydra_admin/admin/clients/$bootstrap_client" >/dev/null 2>&1 || true
  fi
  if [ "$fga_tuple_written" = true ] && [ -n "${store_id:-}" ]; then
    delete_body="$(jq -nc --arg user "user:$bootstrap_client" \
      '{deletes:{tuple_keys:[{user:$user,relation:"admin",object:"workspace:default"}]}}')"
    fga_call POST "/stores/$store_id/write" "$fixture_dir/fga-delete.json" \
      --header 'Content-Type: application/json' --data-binary "$delete_body" >/dev/null 2>&1 || true
  fi

  if [ "${#forward_pids[@]}" -gt 0 ]; then
    kill "${forward_pids[@]}" 2>/dev/null || true
    wait "${forward_pids[@]}" 2>/dev/null || true
  fi

  # fixture_dir is an exact mktemp-created directory and contains credentials.
  find "$fixture_dir" -type f -delete 2>/dev/null || true
  find "$fixture_dir" -depth -type d -empty -delete 2>/dev/null || true
}
trap cleanup EXIT

echo "==> validate live target and open private control-plane forwards"
kubectl get --raw=/readyz >/dev/null || fail "target Kubernetes API is not ready"
kubectl -n "$auth_namespace" get service hydra-public hydra-admin openfga >/dev/null \
  || fail "required auth services are absent"
kubectl -n "$api_namespace" get secret "$fga_secret" >/dev/null \
  || fail "OpenFGA credential Secret is absent"

kubectl -n "$auth_namespace" port-forward service/hydra-public \
  "$hydra_public_port:4444" >"$fixture_dir/hydra-public.log" 2>&1 &
forward_pids+=("$!")
kubectl -n "$auth_namespace" port-forward service/hydra-admin \
  "$hydra_admin_port:4445" >"$fixture_dir/hydra-admin.log" 2>&1 &
forward_pids+=("$!")
kubectl -n "$auth_namespace" port-forward service/openfga \
  "$openfga_port:8080" >"$fixture_dir/openfga.log" 2>&1 &
forward_pids+=("$!")

fga_token="$(kubectl -n "$api_namespace" get secret "$fga_secret" \
  -o "jsonpath={.data.$fga_secret_key}" | base64 -d)"
[ -n "$fga_token" ] || fail "OpenFGA credential Secret key is empty"
printf 'header = "Authorization: Bearer %s"\n' "$fga_token" >"$fixture_dir/openfga.curl"
chmod 600 "$fixture_dir/openfga.curl"

ready=false
for _ in $(seq 1 60); do
  if curl -fsS "$hydra_public/.well-known/openid-configuration" >/dev/null 2>&1 \
    && curl -fsS "$hydra_admin/health/ready" >/dev/null 2>&1 \
    && [ "$(fga_call GET '/stores?page_size=100' "$fixture_dir/fga-ready.json" 2>/dev/null)" = 200 ]; then
    ready=true
    break
  fi
  sleep 1
done
[ "$ready" = true ] || fail "private auth service forwards did not become ready"

echo "==> create a short-lived bootstrap principal and default-workspace grant"
client_body="$fixture_dir/bootstrap-client.json"
jq -nc --arg id "$bootstrap_client" --arg secret "$bootstrap_secret" \
  '{client_id:$id,client_name:"m35 disposable verifier",client_secret:$secret,
    grant_types:["client_credentials"],token_endpoint_auth_method:"client_secret_post"}' \
  >"$client_body"
code="$(curl --silent --show-error --output "$fixture_dir/bootstrap-create.json" \
  --write-out '%{http_code}' --request POST --header 'Content-Type: application/json' \
  --data-binary "@$client_body" "$hydra_admin/admin/clients")"
[ "$code" = 201 ] || fail "creating the disposable Hydra bootstrap client returned HTTP $code"
bootstrap_created=true

code="$(fga_call GET '/stores?page_size=100' "$fixture_dir/fga-stores.json")"
[ "$code" = 200 ] || fail "listing OpenFGA stores returned HTTP $code"
store_id="$(jq -er 'first(.stores[] | select(.name == "bex")) | .id' "$fixture_dir/fga-stores.json")"
grant_body="$(jq -nc --arg user "user:$bootstrap_client" \
  '{writes:{tuple_keys:[{user:$user,relation:"admin",object:"workspace:default"}]}}')"
code="$(fga_call POST "/stores/$store_id/write" "$fixture_dir/fga-grant.json" \
  --header 'Content-Type: application/json' --data-binary "$grant_body")"
[ "$code" = 200 ] || fail "granting disposable default-workspace authority returned HTTP $code"
fga_tuple_written=true

bootstrap_token="$(token_for "$bootstrap_client" "$bootstrap_secret" bootstrap)"
write_auth_config "$bootstrap_token" "$fixture_dir/bootstrap.curl"

echo "==> create two disposable workspaces through the public GraphQL surface"
create_query='mutation($name:String!,$plan:String){createWorkspace(name:$name,plan:$plan){id name plan role}}'
graphql_call "$fixture_dir/bootstrap.curl" "$create_query" \
  "$(jq -nc --arg name "$workspace_name_a" '{name:$name,plan:"hobby"}')" \
  "$fixture_dir/workspace-a.json"
workspace_a="$(jq -er '.data.createWorkspace.id' "$fixture_dir/workspace-a.json")"
graphql_call "$fixture_dir/bootstrap.curl" "$create_query" \
  "$(jq -nc --arg name "$workspace_name_b" '{name:$name,plan:"hobby"}')" \
  "$fixture_dir/workspace-b.json"
workspace_b="$(jq -er '.data.createWorkspace.id' "$fixture_dir/workspace-b.json")"
[ "$workspace_a" != "$workspace_b" ] || fail "workspace creation returned duplicate ids"

echo "==> create four independent owner/member/admin principals"
create_api_key "$workspace_a" "m35-owner-a-$run_id" "$fixture_dir/key-owner-a.json"
key_owner_a="$(jq -er '.id' "$fixture_dir/key-owner-a.json")"
secret_owner_a="$(jq -er '.secret' "$fixture_dir/key-owner-a.json")"
create_api_key "$workspace_a" "m35-member-b-$run_id" "$fixture_dir/key-member-b.json"
key_member_b="$(jq -er '.id' "$fixture_dir/key-member-b.json")"
secret_member_b="$(jq -er '.secret' "$fixture_dir/key-member-b.json")"
create_api_key "$workspace_a" "m35-admin-a-$run_id" "$fixture_dir/key-admin-a.json"
key_admin_a="$(jq -er '.id' "$fixture_dir/key-admin-a.json")"
secret_admin_a="$(jq -er '.secret' "$fixture_dir/key-admin-a.json")"
create_api_key "$workspace_b" "m35-owner-b-$run_id" "$fixture_dir/key-owner-b.json"
key_owner_b="$(jq -er '.id' "$fixture_dir/key-owner-b.json")"
secret_owner_b="$(jq -er '.secret' "$fixture_dir/key-owner-b.json")"

code="$(api_call "$fixture_dir/bootstrap.curl" PATCH \
  "/v1/workspaces/$workspace_a/members/$key_admin_a" "$fixture_dir/admin-role.json" \
  --header 'Content-Type: application/json' --data-binary '{"role":"ADMIN"}')"
[ "$code" = 200 ] || fail "promoting the disposable admin principal returned HTTP $code"
[ "$(jq -r '.role' "$fixture_dir/admin-role.json")" = ADMIN ] \
  || fail "disposable admin role did not round-trip as ADMIN"

token_owner_a="$(token_for "$key_owner_a" "$secret_owner_a" owner-a)"
token_member_b="$(token_for "$key_member_b" "$secret_member_b" member-b)"
token_admin_a="$(token_for "$key_admin_a" "$secret_admin_a" admin-a)"
token_owner_b="$(token_for "$key_owner_b" "$secret_owner_b" owner-b)"

echo "==> wait for NamespaceReconciler sandbox regimes"
namespaces_ready=false
for _ in $(seq 1 120); do
  if kubectl get namespace "$workspace_a-sandbox" "$workspace_b-sandbox" >/dev/null 2>&1 \
    && kubectl -n "$workspace_a-sandbox" get rolebinding bex-tenant-sandbox-server >/dev/null 2>&1 \
    && kubectl -n "$workspace_b-sandbox" get rolebinding bex-tenant-sandbox-server >/dev/null 2>&1; then
    namespaces_ready=true
    break
  fi
  sleep 2
done
[ "$namespaces_ready" = true ] || fail "disposable sandbox namespaces did not reconcile"

# A GitOps rollout may legitimately overlap fixture provisioning. Wait for both
# shared OpenSandbox components before the verifier takes its fail-closed
# availability snapshot; the verifier still rechecks every generation/replica.
kubectl -n opensandbox-system rollout status deployment/opensandbox-server --timeout=180s
kubectl -n opensandbox-system rollout status deployment/opensandbox-controller-manager --timeout=180s

echo "==> run the complete gVisor/Cilium/OpenFGA adversarial matrix"
BEX_API_URL="$api_url" \
BEX_WORKSPACE_A="$workspace_a" \
BEX_WORKSPACE_B="$workspace_b" \
BEX_WS_A_OWNER_TOKEN="$token_owner_a" \
BEX_WS_A_MEMBER_TOKEN="$token_member_b" \
BEX_WS_A_ADMIN_TOKEN="$token_admin_a" \
BEX_WS_B_OWNER_TOKEN="$token_owner_b" \
  bash scripts/verify-sandbox-isolation.sh

echo "PASS: disposable principals/workspaces provisioned, matrix passed, cleanup armed"
