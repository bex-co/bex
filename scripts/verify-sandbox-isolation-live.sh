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
# Optional after deploying w3/m37:
#   BEX_VERIFY_AGENT_DRIVER=1
#   BEX_VERIFY_AGENT_RETENTION=1   (with the line above: a stopped turn keeps
#                                   its disk, ADR047 D12, w22/m413)
#
# Optional: run another probe against the same disposable fixtures instead of the
# adversarial matrix (sandbox-measure.yml runs the measurement legs this way,
# w22/m313, w22/m314, w22/m321, w22/002, w22/008):
#   BEX_VERIFY_PROBE=scripts/measure-sandbox-kubelet.sh
#
# Cluster defaults can be overridden for non-production installations:
#   BEX_AUTH_NAMESPACE=auth
#   BEX_API_NAMESPACE=bex-system
#   BEX_OPENFGA_TOKEN_SECRET=bex-openfga
#   BEX_OPENFGA_TOKEN_KEY=token
#
# Before it provisions, the wrapper removes what an earlier run left behind:
# m35-* Hydra clients and m35-a-/m35-b- workspaces older than
#   BEX_VERIFY_STALE_FIXTURE_SECONDS=21600
# (six hours, longer than any GitHub-hosted job can run), w9/119.

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
stale_after="${BEX_VERIFY_STALE_FIXTURE_SECONDS:-21600}"
[[ "$stale_after" =~ ^[0-9]+$ ]] || {
  echo "error: BEX_VERIFY_STALE_FIXTURE_SECONDS must be a whole number of seconds" >&2
  exit 2
}
run_id="$(date -u +%Y%m%d%H%M%S)-$$"
workspace_name_a="m35-a-$(date -u +%m%d%H%M%S)-$((BASHPID % 10000))"
workspace_name_b="m35-b-$(date -u +%m%d%H%M%S)-$((BASHPID % 10000))"
bootstrap_client="m35-bootstrap-$run_id"
bootstrap_secret="$(openssl rand -hex 32)"
fixture_dir="$(mktemp -d)"
umask 077

forward_pids=()
bootstrap_created=false
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

# Mint a disposable workspace principal the same way CreateAPIKey does
# (Hydra client + tenant_members row + OpenFGA membership), without calling
# POST /v1/api-keys — AuthorizeMintClass refuses client_credentials callers
# (codex round-7 F3), and this wrapper's bootstrap is exactly that class.
# relation is the OpenFGA workspace relation / tenant_members.role
# (developer for ordinary members, admin for the elevated principal).
# creator, when given, is stamped as the key's bex.co/created-by: since w9/m481
# an API key is never allowed what its creator is not, and a key with no creator
# never acts above a Developer, so the elevated principal names the bootstrap
# client, an admin of every disposable workspace, as the person behind it.
create_bound_principal() {
  local workspace="$1" name="$2" relation="$3" output="$4" creator="${5:-}"
  local client_id client_secret client_body code
  case "$relation" in developer | admin) ;; *)
    fail "create_bound_principal: relation must be developer or admin, got $relation"
    ;;
  esac
  client_id="m35-$(openssl rand -hex 10)"
  client_secret="$(openssl rand -hex 32)"
  client_body="$fixture_dir/hydra-$client_id.json"
  jq -nc --arg id "$client_id" --arg secret "$client_secret" --arg name "$name" --arg creator "$creator" \
    '{client_id:$id,client_name:$name,client_secret:$secret,
      grant_types:["client_credentials"],token_endpoint_auth_method:"client_secret_post",
      metadata:({"bex.co/api-key":true} + (if $creator == "" then {} else {"bex.co/created-by":$creator} end))}' >"$client_body"
  code="$(curl --silent --show-error --output "$fixture_dir/hydra-create-$client_id.json" \
    --write-out '%{http_code}' --request POST --header 'Content-Type: application/json' \
    --data-binary "@$client_body" "$hydra_admin/admin/clients")"
  [ "$code" = 201 ] || fail "creating Hydra client for $name returned HTTP $code"

  bind_subject "$workspace" "$client_id" "$relation" "$name"
  jq -nc --arg id "$client_id" --arg secret "$client_secret" '{id:$id,secret:$secret}' >"$output"
}

db_primary() {
  kubectl -n "$api_namespace" get clusters.postgresql.cnpg.io bex-db \
    -o jsonpath='{.status.currentPrimary}' 2>/dev/null
}

# bind_subject gives one subject a role in one workspace in BindClient's shape:
# its only tenant_members row, then the matching OpenFGA tuple.
bind_subject() {
  local workspace="$1" subject="$2" relation="$3" name="$4" primary grant_body code
  primary="$(db_primary)"
  [ -n "$primary" ] || fail "bex-db has no primary for binding $name"
  kubectl -n "$api_namespace" exec -i "$primary" -c postgres -- \
    psql -X -q -v ON_ERROR_STOP=1 -U postgres -d bex \
    -v client="$subject" -v tenant="$workspace" -v role="$relation" <<'SQL' >/dev/null \
    || fail "binding $name into tenant_members failed"
DELETE FROM tenant_members WHERE subject = :'client';
INSERT INTO tenant_members (tenant_id, subject, role)
VALUES (:'tenant', :'client', :'role');
SQL

  grant_body="$(jq -nc --arg user "user:$subject" --arg rel "$relation" --arg ws "workspace:$workspace" \
    '{writes:{tuple_keys:[{user:$user,relation:$rel,object:$ws}]}}')"
  code="$(fga_call POST "/stores/$store_id/write" "$fixture_dir/fga-bind-$subject.json" \
    --header 'Content-Type: application/json' --data-binary "$grant_body")"
  [ "$code" = 200 ] || fail "granting $relation on $workspace for $name returned HTTP $code"
}

# delete_workspace tears one fixture workspace down through deleteWorkspace,
# the product path: it revokes the people's tuples, runs the sandbox, billing and
# secret purgers, and cascades the rows. It runs from the cleanup trap and the
# stale sweep, so it reports a failure and returns: graphql_call's fail, run
# here directly, would exit the whole trap (w9/119).
delete_workspace() {
  local id="$1" name="$2"
  [ -n "$id" ] || return 0
  if (graphql_call "$fixture_dir/bootstrap.curl" \
    'mutation($id:String!,$confirmation:String!){deleteWorkspace(id:$id,confirmation:$confirmation)}' \
    "$(jq -nc --arg id "$id" --arg confirmation "sudo delete workspace $name" \
      '{id:$id,confirmation:$confirmation}')" \
    "$fixture_dir/delete-workspace-$id.json") >/dev/null 2>&1; then
    return 0
  fi
  echo "WARNING: deleting fixture workspace $id ($name) failed; the next run's stale sweep retries it" >&2
  return 1
}

# remove_subjects deletes each fixture principal's Hydra client and every
# OpenFGA tuple that names it. deleteWorkspace revokes only the people in a
# workspace (ListTenantMembers reads kind='user'), never its API keys, and the
# bootstrap's workspace:default grant lies outside every fixture workspace.
# Its callers, the cleanup trap and the sweep, run it with errexit off.
remove_subjects() {
  local subject code object_type read_body continuation tuple delete_body
  for subject in "$@"; do
    [ -n "$subject" ] || continue
    code="$(curl --silent --output /dev/null --write-out '%{http_code}' \
      --request DELETE "$hydra_admin/admin/clients/$subject" 2>/dev/null)"
    case "$code" in 204 | 404) ;; *)
      echo "WARNING: deleting Hydra client $subject returned HTTP ${code:-none}; the next run's stale sweep retries it" >&2
      ;;
    esac
    [ -n "${store_id:-}" ] || continue
    for object_type in workspace org; do
      continuation=""
      while :; do
        read_body="$(jq -nc --arg user "user:$subject" --arg object "$object_type:" --arg token "$continuation" \
          '{tuple_key:{user:$user,object:$object},page_size:100}
            + (if $token == "" then {} else {continuation_token:$token} end)')"
        code="$(fga_call POST "/stores/$store_id/read" "$fixture_dir/fga-read.json" \
          --header 'Content-Type: application/json' --data-binary "$read_body" 2>/dev/null)"
        if [ "$code" != 200 ]; then
          echo "WARNING: reading the $object_type tuples of $subject returned HTTP ${code:-none}" >&2
          break
        fi
        while IFS= read -r tuple; do
          [ -n "$tuple" ] || continue
          delete_body="$(jq -nc --argjson key "$tuple" '{deletes:{tuple_keys:[$key]}}')"
          code="$(fga_call POST "/stores/$store_id/write" "$fixture_dir/fga-delete.json" \
            --header 'Content-Type: application/json' --data-binary "$delete_body" 2>/dev/null)"
          [ "$code" = 200 ] \
            || echo "WARNING: deleting tuple $tuple returned HTTP ${code:-none}" >&2
        done < <(jq -c '.tuples[]?.key | {user,relation,object}' "$fixture_dir/fga-read.json")
        continuation="$(jq -r '.continuation_token // ""' "$fixture_dir/fga-read.json")"
        [ -n "$continuation" ] || break
      done
    done
  done
}

# sweep_stale_fixtures removes what an earlier run left behind (w9/119): a
# killed runner, or a cleanup that failed, leaves live credentials nothing else
# ever deletes. Only what this script mints qualifies: Hydra clients m35-* named m35*, and workspaces
# m35-a-/m35-b-<10 digits>-<n> with a tea- id and no member but m35-* principals.
# Each must be older than stale_after, longer than any run lives, so a
# concurrent run's fixtures are never touched. This run's bootstrap seats itself
# as admin of each stale workspace and deletes it through the product path.
# The body is a subshell with errexit off: a failure warns and the sweep goes
# on, because a leftover is a hygiene problem, not an isolation failure, and the
# next run retries it.
sweep_stale_fixtures() (
  set +e
  local cutoff url headers next pages=0 primary rows workspace name clients=() workspaces=()
  cutoff=$(($(date -u +%s) - stale_after))

  url="$hydra_admin/admin/clients?page_size=500"
  headers="$fixture_dir/hydra-clients.headers"
  : >"$fixture_dir/hydra-clients.jsonl"
  while [ -n "$url" ] && [ "$pages" -lt 100 ]; do
    pages=$((pages + 1))
    if ! curl --silent --show-error --fail --dump-header "$headers" --output "$fixture_dir/hydra-page.json" "$url" \
      || ! jq -c '.[]' "$fixture_dir/hydra-page.json" >>"$fixture_dir/hydra-clients.jsonl"; then
      echo "WARNING: listing Hydra clients failed; skipping the stale-fixture sweep" >&2
      return 0
    fi
    next="$(tr -d '\r' <"$headers" | grep -i '^link:' | tr ',' '\n' | grep 'rel="next"' \
      | sed -n 's/.*<\([^>]*\)>.*/\1/p' | head -1)"
    case "$next" in
      "") url="" ;;
      http://* | https://*) url="$hydra_admin/${next#*://*/}" ;;
      *) url="$hydra_admin/${next#/}" ;;
    esac
  done
  mapfile -t clients < <(jq -r --arg self "$bootstrap_client" --argjson cutoff "$cutoff" '
    select((.client_id // "") | test("^m35-[0-9a-z-]+$"))
    | select(.client_id != $self and ((.client_name // "") | startswith("m35")))
    | select((try ((.created_at // "") | sub("\\.[0-9]+"; "") | sub("\\+00:00$"; "Z") | fromdateiso8601)
              catch null) as $at | $at != null and $at < $cutoff)
    | .client_id' "$fixture_dir/hydra-clients.jsonl")

  primary="$(db_primary)"
  if [ -z "$primary" ]; then
    echo "WARNING: bex-db has no primary; stale fixture workspaces are not swept this run" >&2
  elif rows="$(kubectl -n "$api_namespace" exec -i "$primary" -c postgres -- \
    psql -X -q -At -F ' ' -v ON_ERROR_STOP=1 -U postgres -d bex -v stale="$stale_after" <<'SQL'
SELECT t.id, COALESCE(NULLIF(t.display_name, ''), t.name)
FROM tenants t
WHERE t.name ~ '^m35-[ab]-[0-9]{10}-[0-9]+$'
  AND t.id ~ '^tea-[0-9a-v]{20}$'
  AND t.created_at < now() - make_interval(secs => :stale)
  AND NOT EXISTS (SELECT 1 FROM tenant_members m
                  WHERE m.tenant_id = t.id AND m.subject NOT LIKE 'm35-%')
ORDER BY t.created_at;
SQL
  )"; then
    [ -z "$rows" ] || mapfile -t workspaces <<<"$rows"
  else
    echo "WARNING: listing stale fixture workspaces failed" >&2
  fi

  if [ "${#clients[@]}" -eq 0 ] && [ "${#workspaces[@]}" -eq 0 ]; then
    echo "    none older than ${stale_after}s"
    return 0
  fi
  for workspace in "${workspaces[@]}"; do
    name="${workspace#* }"
    workspace="${workspace%% *}"
    echo "    workspace $workspace ($name)"
    (bind_subject "$workspace" "$bootstrap_client" admin "the sweep on $name") >/dev/null 2>&1 \
      || echo "WARNING: seating the bootstrap as admin of $workspace failed" >&2
    delete_workspace "$workspace" "$name"
  done
  for name in "${clients[@]}"; do
    echo "    Hydra client $name"
  done
  remove_subjects "${clients[@]}"
  return 0
)

cleanup() {
  set +e
  local token

  # The bootstrap's access token lives 15 minutes (Hydra's ttl.access_token), and
  # a leg that outlives it reached this trap with every API call refused (w9/119).
  # Mint a fresh one; the command substitution keeps token_for's fail from
  # ending the trap.
  if [ "$bootstrap_created" = true ] \
    && token="$(token_for "$bootstrap_client" "$bootstrap_secret" cleanup 2>/dev/null)"; then
    write_auth_config "$token" "$fixture_dir/bootstrap.curl"
  fi

  # Keep the bootstrap authority until product-level cleanup has run:
  # remove_subjects below then deletes the Hydra clients and every tuple left.
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
    delete_workspace "$workspace_a" "$workspace_name_a"
    delete_workspace "$workspace_b" "$workspace_name_b"
  fi

  remove_subjects "$key_owner_a" "$key_member_b" "$key_admin_a" "$key_owner_b"
  if [ "$bootstrap_created" = true ]; then
    remove_subjects "$bootstrap_client"
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

bootstrap_token="$(token_for "$bootstrap_client" "$bootstrap_secret" bootstrap)"
write_auth_config "$bootstrap_token" "$fixture_dir/bootstrap.curl"

echo "==> remove fixtures an earlier run left behind"
sweep_stale_fixtures

# Workspaces must exist before sandbox create is payment-gated (ADR046 +
# BEX_REQUIRE_PAYMENT_METHOD=all). createWorkspace on the public GraphQL surface
# refuses a Hydra bootstrap principal with no Stripe customer; the control-plane
# POST /v1/tenants path is the operator exemption for disposable fixtures and
# grants the bootstrap client admin so key mint + deleteWorkspace still work.
# Workspace A stays on the paid "pro" tier: the matrix holds three concurrent
# sandboxes there when the agent-driver leg runs, and the hobby sandbox-namespace
# ResourceQuota (limits 4 CPU / 8 Gi = two sandboxes) cannot admit a third.
# Workspace B stays hobby (one sandbox). Billing-exclude both immediately so
# sandbox create itself is not payment-gated either.
echo "==> create two disposable workspaces through the control-plane tenant API"
# Match verify-tenant-isolation.sh: let the kernel pick the local port and
# parse it out of the forward's own log. free_port()+bind races with other
# forwards on the shared runner and was what left this step red after the
# tenant leg had already proven the same API is reachable.
kubectl -n "$api_namespace" port-forward deploy/bex-api ":8091" \
  >"$fixture_dir/cp-forward.log" 2>&1 &
forward_pids+=("$!")
cp_port=""
for _ in $(seq 1 30); do
  cp_port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9][0-9]*\).*/\1/p' \
    "$fixture_dir/cp-forward.log" | head -1)"
  [ -n "$cp_port" ] && break
  sleep 1
done
[ -n "$cp_port" ] || {
  echo "---- cp-forward.log ----" >&2
  cat "$fixture_dir/cp-forward.log" >&2 || true
  fail "control-plane port-forward never reported a local port"
}
cp_token="$(kubectl -n "$api_namespace" get secret bex-control-plane -o 'jsonpath={.data.token}' | base64 -d)"
[ -n "$cp_token" ] || fail "control-plane internal API token is empty"
printf 'header = "Authorization: Bearer %s"\n' "$cp_token" >"$fixture_dir/cp.curl"
chmod 600 "$fixture_dir/cp.curl"
cp_ready=false
for _ in $(seq 1 30); do
  if [ "$(curl -sS --connect-timeout 2 --max-time 3 -o /dev/null -w '%{http_code}' \
    "http://127.0.0.1:$cp_port/" 2>/dev/null)" != 000 ]; then
    cp_ready=true
    break
  fi
  sleep 1
done
[ "$cp_ready" = true ] || {
  echo "---- cp-forward.log ----" >&2
  cat "$fixture_dir/cp-forward.log" >&2 || true
  fail "control-plane internal API forward did not become ready"
}

create_cp_tenant() {
  local name="$1" plan="$2" output="$3" code
  code="$(jq -nc --arg name "$name" --arg plan "$plan" --arg admin "$bootstrap_client" \
    '{name:$name,plan:$plan,admin:$admin}' \
    | curl -sS --config "$fixture_dir/cp.curl" --output "$output" \
        --write-out '%{http_code}' -X POST -H 'Content-Type: application/json' \
        --data-binary @- "http://127.0.0.1:$cp_port/v1/tenants")"
  [ "$code" = 201 ] || fail "control-plane createTenant $name returned HTTP $code"
  jq -er '.id' "$output"
}

workspace_a="$(create_cp_tenant "$workspace_name_a" pro "$fixture_dir/workspace-a.json")"
workspace_b="$(create_cp_tenant "$workspace_name_b" hobby "$fixture_dir/workspace-b.json")"
[ "$workspace_a" != "$workspace_b" ] || fail "workspace creation returned duplicate ids"

echo "==> billing-exclude the disposable workspaces (ADR046 paid-intent gate)"
for workspace in "$workspace_a" "$workspace_b"; do
  code="$(jq -nc '{excluded:true,actor:"sandbox-isolation-live-verify"}' \
    | curl -sS --config "$fixture_dir/cp.curl" --output "$fixture_dir/exclude-$workspace.json" \
        --write-out '%{http_code}' -X PATCH -H 'Content-Type: application/json' \
        --data-binary @- "http://127.0.0.1:$cp_port/v1/tenants/$workspace/billing-excluded")"
  [ "$code" = 200 ] || fail "billing-excluding disposable workspace $workspace returned HTTP $code"
done

echo "==> create four independent owner/member/admin principals"
# Direct Hydra+DB+FGA mint (not POST /v1/api-keys): AuthorizeMintClass forbids
# the bootstrap's client_credentials token from self-replicating.
create_bound_principal "$workspace_a" "m35-owner-a-$run_id" developer "$fixture_dir/key-owner-a.json"
key_owner_a="$(jq -er '.id' "$fixture_dir/key-owner-a.json")"
secret_owner_a="$(jq -er '.secret' "$fixture_dir/key-owner-a.json")"
create_bound_principal "$workspace_a" "m35-member-b-$run_id" developer "$fixture_dir/key-member-b.json"
key_member_b="$(jq -er '.id' "$fixture_dir/key-member-b.json")"
secret_member_b="$(jq -er '.secret' "$fixture_dir/key-member-b.json")"
create_bound_principal "$workspace_a" "m35-admin-a-$run_id" admin "$fixture_dir/key-admin-a.json" "$bootstrap_client"
key_admin_a="$(jq -er '.id' "$fixture_dir/key-admin-a.json")"
secret_admin_a="$(jq -er '.secret' "$fixture_dir/key-admin-a.json")"
create_bound_principal "$workspace_b" "m35-owner-b-$run_id" developer "$fixture_dir/key-owner-b.json"
key_owner_b="$(jq -er '.id' "$fixture_dir/key-owner-b.json")"
secret_owner_b="$(jq -er '.secret' "$fixture_dir/key-owner-b.json")"

token_owner_a="$(token_for "$key_owner_a" "$secret_owner_a" owner-a)"
token_member_b="$(token_for "$key_member_b" "$secret_member_b" member-b)"
token_admin_a="$(token_for "$key_admin_a" "$secret_admin_a" admin-a)"
token_owner_b="$(token_for "$key_owner_b" "$secret_owner_b" owner-b)"

echo "==> wait for NamespaceReconciler sandbox regimes"
namespaces_ready=false
for _ in $(seq 1 120); do
  if kubectl get namespace "$workspace_a-sandbox" "$workspace_b-sandbox" >/dev/null 2>&1 \
    && kubectl -n "$workspace_a-sandbox" get rolebinding bex-tenant-sandbox-server >/dev/null 2>&1 \
    && kubectl -n "$workspace_b-sandbox" get rolebinding bex-tenant-sandbox-server >/dev/null 2>&1 \
    && kubectl -n "$workspace_a-sandbox" get networkpolicy default-deny >/dev/null 2>&1 \
    && kubectl -n "$workspace_b-sandbox" get networkpolicy default-deny >/dev/null 2>&1; then
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

echo "==> run ${BEX_VERIFY_PROBE:-the complete gVisor/Cilium/OpenFGA adversarial matrix}"
BEX_API_URL="$api_url" \
BEX_WORKSPACE_A="$workspace_a" \
BEX_WORKSPACE_B="$workspace_b" \
BEX_WS_A_OWNER_TOKEN="$token_owner_a" \
BEX_HYDRA_PUBLIC_URL="$hydra_public" \
BEX_WS_A_OWNER_CLIENT_ID="$key_owner_a" \
BEX_WS_A_OWNER_CLIENT_SECRET="$secret_owner_a" \
BEX_WS_A_MEMBER_TOKEN="$token_member_b" \
BEX_WS_A_ADMIN_TOKEN="$token_admin_a" \
BEX_WS_B_OWNER_TOKEN="$token_owner_b" \
BEX_VERIFY_AGENT_DRIVER="${BEX_VERIFY_AGENT_DRIVER:-0}" \
BEX_VERIFY_AGENT_RETENTION="${BEX_VERIFY_AGENT_RETENTION:-0}" \
  bash "${BEX_VERIFY_PROBE:-scripts/verify-sandbox-isolation.sh}"

echo "PASS: disposable principals/workspaces provisioned, matrix passed, cleanup armed"
