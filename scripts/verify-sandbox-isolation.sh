#!/usr/bin/env bash
set -euo pipefail

# Adversarial verification for ADR042 / w3/m35. This intentionally uses the
# public sandbox API to create real OpenSandbox resources, then executes probes
# inside their gVisor workload containers with kubectl. It requires a
# production-equivalent containerd+Cilium+runsc cluster and four pre-existing
# identities:
#
#   BEX_WS_A_OWNER_TOKEN   ordinary member A in BEX_WORKSPACE_A
#   BEX_WS_A_MEMBER_TOKEN  different ordinary member B in BEX_WORKSPACE_A
#   BEX_WS_A_ADMIN_TOKEN   explicit admin in BEX_WORKSPACE_A
#   BEX_WS_B_OWNER_TOKEN   ordinary member in BEX_WORKSPACE_B
#
# No credential is printed. Every API-created sandbox and Kubernetes fixture is
# removed by the EXIT trap, including on a failed assertion.

required_env=(
  BEX_API_URL
  BEX_WORKSPACE_A
  BEX_WORKSPACE_B
  BEX_WS_A_OWNER_TOKEN
  BEX_WS_A_MEMBER_TOKEN
  BEX_WS_A_ADMIN_TOKEN
  BEX_WS_B_OWNER_TOKEN
)
for name in "${required_env[@]}"; do
  [ -n "${!name:-}" ] || { echo "error: $name is required" >&2; exit 2; }
done
for command in curl go jq kubectl base64 python3; do
  command -v "$command" >/dev/null || { echo "error: missing required command: $command" >&2; exit 2; }
done
for workspace in "$BEX_WORKSPACE_A" "$BEX_WORKSPACE_B"; do
  [[ "$workspace" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] \
    || { echo "error: unsafe workspace id: $workspace" >&2; exit 2; }
done
if [ "$BEX_WORKSPACE_A" = "$BEX_WORKSPACE_B" ]; then
  echo "error: BEX_WORKSPACE_A and BEX_WORKSPACE_B must differ" >&2
  exit 2
fi

api_url="${BEX_API_URL%/}"
namespace_a="${BEX_WORKSPACE_A}-sandbox"
namespace_b="${BEX_WORKSPACE_B}-sandbox"
run_id="m35-$(date -u +%Y%m%d%H%M%S)-$$"
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fixture_dir="$(mktemp -d)"
hosting_pod="${run_id}-hosting"
platform_pod="${run_id}-platform"
platform_allowed_pod="${run_id}-platform-allowed"
execd_spoof_pod="${run_id}-execd-spoof"
execd_allowed_pod="${run_id}-execd-allowed"
controller_probe_pod="${run_id}-controller"
peer_service="${run_id}-peer"
session_policy="agent-session-egress-0000000000000040"
session_label="${run_id}-agent-session"

sandbox_a_id=""
sandbox_b_id=""
sandbox_c_id=""
cr_a=""
cr_b=""
cr_c=""
hubble_agent_pod=""
fixtures_started=false

server_as='system:serviceaccount:opensandbox-system:opensandbox-server'
controller_as='system:serviceaccount:opensandbox-system:opensandbox-controller-manager'
bex_api_as='system:serviceaccount:bex-system:bex-api'
gateway_as='system:serviceaccount:bex-system:bex-ssh-gateway'

write_auth_config() {
  local token="$1" output="$2"
  umask 077
  printf 'header = "Authorization: Bearer %s"\n' "$token" >"$output"
}
write_auth_config "$BEX_WS_A_OWNER_TOKEN" "$fixture_dir/owner-a.curl"
write_auth_config "$BEX_WS_A_MEMBER_TOKEN" "$fixture_dir/member-b.curl"
write_auth_config "$BEX_WS_A_ADMIN_TOKEN" "$fixture_dir/admin-a.curl"
write_auth_config "$BEX_WS_B_OWNER_TOKEN" "$fixture_dir/owner-c.curl"

api_call() {
  local auth_file="$1" method="$2" path="$3" output="$4"
  shift 4
  curl --config "$auth_file" --silent --show-error \
    --request "$method" --output "$output" --write-out '%{http_code}' \
    "$api_url$path" "$@"
}

cleanup() {
  set +e
  if [ "$fixtures_started" != true ]; then
    rm -rf "$fixture_dir"
    return
  fi
  [ -n "$sandbox_a_id" ] && api_call "$fixture_dir/owner-a.curl" POST "/v1/sandboxes/$sandbox_a_id/terminate?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/cleanup-a.json" >/dev/null
  [ -n "$sandbox_b_id" ] && api_call "$fixture_dir/member-b.curl" POST "/v1/sandboxes/$sandbox_b_id/terminate?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/cleanup-b.json" >/dev/null
  [ -n "$sandbox_c_id" ] && api_call "$fixture_dir/owner-c.curl" POST "/v1/sandboxes/$sandbox_c_id/terminate?ownerId=$BEX_WORKSPACE_B" "$fixture_dir/cleanup-c.json" >/dev/null
  kubectl -n "$BEX_WORKSPACE_A" delete pod "$hosting_pod" --ignore-not-found --wait=false >/dev/null 2>&1
  kubectl -n bex-system delete pod "$platform_pod" --ignore-not-found --wait=false >/dev/null 2>&1
  kubectl -n bex-system delete pod "$platform_allowed_pod" --ignore-not-found --wait=false >/dev/null 2>&1
  kubectl -n opensandbox-system delete pod "$execd_spoof_pod" --ignore-not-found --wait=false >/dev/null 2>&1
  kubectl -n opensandbox-system delete pod "$execd_allowed_pod" --ignore-not-found --wait=false >/dev/null 2>&1
  kubectl -n opensandbox-system delete pod "$controller_probe_pod" --ignore-not-found --wait=false >/dev/null 2>&1
  kubectl -n "$namespace_a" delete service "$peer_service" --ignore-not-found >/dev/null 2>&1
  kubectl -n "$namespace_a" delete ciliumnetworkpolicy "$session_policy" --ignore-not-found >/dev/null 2>&1
  [ -n "$cr_a" ] && kubectl -n "$namespace_a" delete batchsandbox "$cr_a" --ignore-not-found --wait=false >/dev/null 2>&1
  [ -n "$cr_b" ] && kubectl -n "$namespace_a" delete batchsandbox "$cr_b" --ignore-not-found --wait=false >/dev/null 2>&1
  [ -n "$cr_c" ] && kubectl -n "$namespace_b" delete batchsandbox "$cr_c" --ignore-not-found --wait=false >/dev/null 2>&1
  rm -rf "$fixture_dir"
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
pass() { echo "PASS: $*"; }

preflight_cluster() {
  local resource namespace deployment image role_json sample_sandbox_node sample_cilium_pod

  echo "==> preflight hardened cluster before creating any fixture"
  [ "$(kubectl get runtimeclass gvisor -o jsonpath='{.handler}')" = runsc ] \
    || fail "RuntimeClass gvisor is absent or does not use runsc"
  [ "$(kubectl -n kube-system get configmap cilium-config -o json | jq -r '.data["enable-l7-proxy"]')" = true ] \
    || fail "Cilium L7 proxy enforcement is not enabled"
  sample_sandbox_node="$(kubectl get nodes -l bex.co/pool=sandbox -o json | jq -er '
    first(.items[] | select(any(.status.conditions[]?; .type == "Ready" and .status == "True")) | .metadata.name)')" \
    || fail "no Ready sandbox node is available"
  sample_cilium_pod="$(kubectl -n kube-system get pods -l k8s-app=cilium \
    --field-selector "spec.nodeName=$sample_sandbox_node" -o json | jq -er '
      first(.items[] | select(any(.status.conditions[]?; .type == "Ready" and .status == "True")) | .metadata.name)')" \
    || fail "no Ready Cilium agent is running on sandbox node $sample_sandbox_node"
  kubectl -n kube-system exec "$sample_cilium_pod" -c cilium-agent -- \
    hubble status --server unix:///var/run/cilium/hubble.sock >/dev/null \
    || fail "the sandbox node's Cilium agent cannot read its local Hubble socket"

  for resource in \
    ciliumclusterwidenetworkpolicy/sandbox-egress-default-deny \
    ciliumclusterwidenetworkpolicy/sandbox-egress-legacy-allowlist \
    ciliumclusterwidenetworkpolicy/sandbox-execd-ingress \
    ciliumclusterwidenetworkpolicy/opensandbox-server-ingress \
    ciliumclusterwidenetworkpolicy/sandbox-exec-gateway-ingress \
    ciliumclusterwidenetworkpolicy/opensandbox-server-egress \
    ciliumclusterwidenetworkpolicy/opensandbox-controller-egress \
    validatingadmissionpolicy/bex-sandbox-pods \
    validatingadmissionpolicybinding/bex-sandbox-pods \
    validatingadmissionpolicy/bex-api-tenant-namespaces \
    validatingadmissionpolicybinding/bex-api-tenant-namespaces \
    validatingadmissionpolicy/bex-api-tenant-namespace-objects \
    validatingadmissionpolicybinding/bex-api-tenant-namespace-objects \
    validatingadmissionpolicy/bex-api-session-egress \
    validatingadmissionpolicybinding/bex-api-session-egress; do
    kubectl get "$resource" >/dev/null 2>&1 \
      || fail "required hardening resource $resource is not deployed"
  done

  kubectl get validatingadmissionpolicy bex-sandbox-pods -o json | jq -e '
    .spec.failurePolicy == "Fail" and
    .spec.matchConstraints.namespaceSelector.matchLabels["app.bex.co/regime"] == "sandbox"' >/dev/null \
    || fail "sandbox Pod admission is not fail-closed on the native sandbox namespace selector"
  kubectl get validatingadmissionpolicybinding bex-sandbox-pods -o json | jq -e '
    (.spec.validationActions | sort) == ["Audit", "Deny"]' >/dev/null \
    || fail "sandbox Pod admission binding does not deny and audit violations"

  for namespace in "$namespace_a" "$namespace_b"; do
    kubectl get namespace "$namespace" -o json | jq -e --arg workspace "${namespace%-sandbox}" '
      .metadata.labels["app.kubernetes.io/managed-by"] == "bex-controlplane" and
      .metadata.labels["app.kubernetes.io/part-of"] == "bex" and
      .metadata.labels["app.bex.co/workspace"] == $workspace and
      .metadata.labels["app.bex.co/regime"] == "sandbox" and
      .metadata.labels["pod-security.kubernetes.io/enforce"] == "baseline" and
      .metadata.labels["pod-security.kubernetes.io/enforce-version"] == "latest" and
      .metadata.labels["pod-security.kubernetes.io/warn"] == "restricted" and
      .metadata.labels["pod-security.kubernetes.io/warn-version"] == "latest" and
      .metadata.labels["pod-security.kubernetes.io/audit"] == "restricted" and
      .metadata.labels["pod-security.kubernetes.io/audit-version"] == "latest"' >/dev/null \
      || fail "$namespace is not a canonical managed sandbox namespace"
    kubectl -n "$namespace" get networkpolicy default-deny >/dev/null 2>&1 \
      || fail "$namespace has no default-deny NetworkPolicy"
    for resource in allow-same-namespace allow-dns-egress; do
      if kubectl -n "$namespace" get networkpolicy "$resource" >/dev/null 2>&1; then
        fail "$namespace still has legacy broad policy $resource"
      fi
    done
    for resource in bex-tenant-sandbox-server bex-tenant-sandbox-controller bex-tenant-ssh-gateway; do
      kubectl -n "$namespace" get rolebinding "$resource" >/dev/null 2>&1 \
        || fail "$namespace is missing RoleBinding $resource"
    done
    if kubectl -n "$namespace" get rolebinding bex-tenant-operator >/dev/null 2>&1; then
      fail "$namespace still binds the Secret-bearing tenant operator role"
    fi
  done

  role_json="$(kubectl get clusterrole opensandbox-server -o json)" \
    || fail "OpenSandbox server informer ClusterRole is absent"
  jq -e '
    ([.rules[]?.verbs[]?] | any(. == "create" or . == "update" or . == "patch" or . == "delete")) | not' \
    <<<"$role_json" >/dev/null \
    || fail "OpenSandbox server still has cluster-wide mutation verbs"
  jq -e '([.rules[]?.resources[]?] | any(. == "secrets")) | not' <<<"$role_json" >/dev/null \
    || fail "OpenSandbox server informer role can read Secrets"

  [ "$(kubectl auth can-i create pods -n "$namespace_a" --as "$server_as")" = yes ] \
    || fail "OpenSandbox server lacks its sandbox-namespace mutation grant"
  [ "$(kubectl auth can-i create pods -n opensandbox-system --as "$server_as")" = no ] \
    || fail "OpenSandbox server can create Pods in opensandbox-system"
  [ "$(kubectl auth can-i create pods -n "$namespace_a" --as "$controller_as")" = yes ] \
    || fail "OpenSandbox controller lacks its sandbox-namespace mutation grant"
  [ "$(kubectl auth can-i create pods -n opensandbox-system --as "$controller_as")" = no ] \
    || fail "OpenSandbox controller can create Pods in opensandbox-system"
  [ "$(kubectl auth can-i create pods --subresource=exec -n "$namespace_a" --as "$gateway_as")" = yes ] \
    || fail "isolated SSH gateway lacks its sandbox-namespace pods/exec grant"
  [ "$(kubectl auth can-i create pods --subresource=exec -n opensandbox-system --as "$gateway_as")" = no ] \
    || fail "isolated SSH gateway has pods/exec authority in opensandbox-system"
  [ "$(kubectl auth can-i create ciliumnetworkpolicies.cilium.io -n "$namespace_a" --as "$bex_api_as")" = yes ] \
    || fail "bex-api lacks the admission-confined per-session Cilium policy grant"

  for deployment in opensandbox-server opensandbox-controller-manager; do
    kubectl -n opensandbox-system get deployment "$deployment" -o json | jq -e '
      (.spec.replicas // 1) > 0 and
      (.status.observedGeneration // 0) >= .metadata.generation and
      (.status.availableReplicas // 0) >= (.spec.replicas // 1)' >/dev/null \
      || fail "opensandbox-system/$deployment is not fully available"
    while IFS= read -r image; do
      [[ "$image" =~ @sha256:[0-9a-f]{64}$ ]] \
        || fail "opensandbox-system/$deployment uses mutable image $image"
    done < <(kubectl -n opensandbox-system get deployment "$deployment" -o json | jq -er \
      '.spec.template.spec.initContainers[]?.image, .spec.template.spec.containers[]?.image')
  done

  for resource in bex-platform-prod opensandbox-controller opensandbox-server; do
    kubectl -n argocd get application "$resource" -o json | jq -e '
      .status.sync.status == "Synced" and .status.health.status == "Healthy"' >/dev/null \
      || fail "Argo application $resource is not Synced and Healthy"
  done
  pass "hardening, immutable workloads, admission, Cilium, and GitOps ownership are live"
}

expect_status() {
  local want="$1" auth_file="$2" method="$3" path="$4" output="$5"
  shift 5
  local got
  got="$(api_call "$auth_file" "$method" "$path" "$output" "$@")"
  [ "$got" = "$want" ] || fail "$method $path returned $got, want $want"
}

create_sandbox() {
  local auth_file="$1" workspace="$2" output="$3"
  expect_status 201 "$auth_file" POST "/v1/sandboxes" "$output" \
    --header 'Content-Type: application/json' \
    --data "{\"ownerId\":\"$workspace\",\"plan\":\"starter\",\"networkPolicy\":{\"default\":\"deny-all\"}}"
  jq -er '.id' "$output"
}

preflight_cluster
if [ "${BEX_PREFLIGHT_ONLY:-0}" = 1 ]; then
  pass "preflight-only mode made no fixture or API mutation"
  exit 0
fi
fixtures_started=true

echo "==> create two sandboxes in workspace A and one in workspace B"
allow_all_status="$(api_call "$fixture_dir/owner-a.curl" POST "/v1/sandboxes" "$fixture_dir/allow-all.json" \
  --header 'Content-Type: application/json' \
  --data "{\"ownerId\":\"$BEX_WORKSPACE_A\",\"networkPolicy\":{\"default\":\"allow-all\"}}")"
if [ "$allow_all_status" != 400 ]; then
  if [[ "$allow_all_status" =~ ^2 ]] && jq -e '.id' "$fixture_dir/allow-all.json" >/dev/null 2>&1; then
    sandbox_a_id="$(jq -er '.id' "$fixture_dir/allow-all.json")"
  fi
  fail "allow-all create returned $allow_all_status, want 400"
fi
[ "$(jq -r '.code' "$fixture_dir/allow-all.json")" = "SANDBOX_NETWORK_POLICY_UNSUPPORTED" ] \
  || fail "allow-all did not return SANDBOX_NETWORK_POLICY_UNSUPPORTED"
sandbox_a_id="$(create_sandbox "$fixture_dir/owner-a.curl" "$BEX_WORKSPACE_A" "$fixture_dir/sandbox-a.json")"
sandbox_b_id="$(create_sandbox "$fixture_dir/member-b.curl" "$BEX_WORKSPACE_A" "$fixture_dir/sandbox-b.json")"
sandbox_c_id="$(create_sandbox "$fixture_dir/owner-c.curl" "$BEX_WORKSPACE_B" "$fixture_dir/sandbox-c.json")"
owner_a="$(jq -er '.owner' "$fixture_dir/sandbox-a.json")"
owner_b="$(jq -er '.owner' "$fixture_dir/sandbox-b.json")"
[ "$owner_a" != "$owner_b" ] || fail "the two workspace-A identities resolved to the same sandbox owner"
for file in sandbox-a.json sandbox-b.json sandbox-c.json; do
  [ "$(jq -r '.networkPolicy.default' "$fixture_dir/$file")" = "deny-all" ] \
    || fail "$file did not return the enforced deny-all policy"
done
pass "public create returns durable deny-all policy for three distinct principals"

echo "==> verify owner/member/admin and cross-workspace behavior"
expect_status 200 "$fixture_dir/owner-a.curl" GET "/v1/sandboxes/$sandbox_a_id?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/get-own.json"
expect_status 404 "$fixture_dir/member-b.curl" GET "/v1/sandboxes/$sandbox_a_id?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/get-cross-owner.json"
[ "$(jq -r '.code' "$fixture_dir/get-cross-owner.json")" = "SANDBOX_NOT_FOUND" ] \
  || fail "cross-owner lookup did not use SANDBOX_NOT_FOUND"
expect_status 404 "$fixture_dir/member-b.curl" POST "/v1/sandboxes/$sandbox_a_id/terminate?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/stop-cross-owner.json"
expect_status 200 "$fixture_dir/admin-a.curl" GET "/v1/sandboxes/$sandbox_a_id?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/get-admin.json"
expect_status 404 "$fixture_dir/owner-c.curl" GET "/v1/sandboxes/$sandbox_a_id?ownerId=$BEX_WORKSPACE_B" "$fixture_dir/get-cross-workspace.json"
expect_status 200 "$fixture_dir/member-b.curl" GET "/v1/sandboxes?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/list-member.json"
jq -e --arg own "$sandbox_b_id" --arg foreign "$sandbox_a_id" \
  '([.[].sandbox.id] | index($own)) != null and ([.[].sandbox.id] | index($foreign)) == null' \
  "$fixture_dir/list-member.json" >/dev/null || fail "ordinary member list crossed the owner boundary"
expect_status 200 "$fixture_dir/admin-a.curl" GET "/v1/sandboxes?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/list-admin.json"
jq -e --arg a "$sandbox_a_id" --arg b "$sandbox_b_id" \
  '([.[].sandbox.id] | index($a)) != null and ([.[].sandbox.id] | index($b)) != null' \
  "$fixture_dir/list-admin.json" >/dev/null || fail "workspace admin could not list both owned sandboxes"
pass "owner isolation fails closed and can_manage is the sole override"

find_batchsandbox() {
  local namespace="$1" id="$2" owner="$3"
  if kubectl -n "$namespace" get batchsandbox "$id" >/dev/null 2>&1; then
    printf '%s\n' "$id"
    return
  fi
  kubectl -n "$namespace" get batchsandboxes -o json | jq -er \
    --arg owner "$owner" --arg since "$started_at" '
      [.items[] |
       select(.metadata.labels["bex.co/owner"] == $owner and .metadata.creationTimestamp >= $since)] |
      sort_by(.metadata.creationTimestamp) | last | .metadata.name'
}

cr_a="$(find_batchsandbox "$namespace_a" "$sandbox_a_id" "$owner_a")"
cr_b="$(find_batchsandbox "$namespace_a" "$sandbox_b_id" "$owner_b")"
owner_c="$(jq -er '.owner' "$fixture_dir/sandbox-c.json")"
cr_c="$(find_batchsandbox "$namespace_b" "$sandbox_c_id" "$owner_c")"

find_ready_pod() {
  local namespace="$1" cr="$2" deadline=$((SECONDS + 300)) pod=""
  while ((SECONDS < deadline)); do
    pod="$(kubectl -n "$namespace" get pods -o json | jq -r --arg cr "$cr" '
      [.items[] |
       select(any(.metadata.ownerReferences[]?; .kind == "BatchSandbox" and .name == $cr)) |
       select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))] |
      sort_by(.metadata.creationTimestamp) | last | .metadata.name // empty')"
    [ -n "$pod" ] && { printf '%s\n' "$pod"; return; }
    sleep 5
  done
  return 1
}

pod_a="$(find_ready_pod "$namespace_a" "$cr_a")" || fail "sandbox A never reached Ready"
pod_b="$(find_ready_pod "$namespace_a" "$cr_b")" || fail "sandbox B never reached Ready"
pod_c="$(find_ready_pod "$namespace_b" "$cr_c")" || fail "sandbox C never reached Ready"

workload_container() {
  kubectl -n "$1" get pod "$2" -o json | jq -er '
    first(.spec.containers[] | select((.name | ascii_downcase | contains("execd")) | not) | .name) // .spec.containers[0].name'
}
container_a="$(workload_container "$namespace_a" "$pod_a")"
container_b="$(workload_container "$namespace_a" "$pod_b")"
sandbox_node="$(kubectl -n "$namespace_a" get pod "$pod_a" -o jsonpath='{.spec.nodeName}')"
sandbox_arch="$(kubectl get node "$sandbox_node" -o jsonpath='{.status.nodeInfo.architecture}')"
case "$sandbox_arch" in
  amd64 | arm64) ;;
  *) fail "unsupported sandbox node architecture $sandbox_arch" ;;
esac
probe_binary="$fixture_dir/m35-netprobe"
CGO_ENABLED=0 GOOS=linux GOARCH="$sandbox_arch" go build -trimpath -o "$probe_binary" scripts/m35-netprobe.go \
  || fail "could not build the sandbox network probe for $sandbox_arch"
kubectl -n "$namespace_a" exec -i "$pod_a" -c "$container_a" -- /bin/sh -c \
  'umask 077; /bin/busybox cat > /tmp/m35-netprobe && chmod 700 /tmp/m35-netprobe' \
  <"$probe_binary" || fail "could not install the network probe in the attacker sandbox"

for tuple in "$namespace_a:$pod_a:$container_a" "$namespace_a:$pod_b:$container_b"; do
  IFS=: read -r ns pod container <<<"$tuple"
  pod_json="$(kubectl -n "$ns" get pod "$pod" -o json)"
  runtime="$(jq -r '.spec.runtimeClassName' <<<"$pod_json")"
  [ "$runtime" = gvisor ] || fail "$ns/$pod uses RuntimeClass '$runtime', want gvisor"
  jq -e '
    .metadata.labels["app.bex.co/regime"] == "sandbox" and
    .spec.automountServiceAccountToken == false and
    .spec.nodeSelector["bex.co/pool"] == "sandbox"' <<<"$pod_json" >/dev/null \
    || fail "$ns/$pod lost its admitted sandbox identity/token/placement shape"
  jq -e '
    ([.spec.initContainers[]?, .spec.containers[]?] |
     map(select(.name | ascii_downcase | contains("execd")))) as $execd |
    ($execd | length) > 0 and
    all($execd[]; .image | test("@sha256:[0-9a-f]{64}$")) and
    ([.spec.initContainers[]?, .spec.containers[]?] |
     all(.[]; (.name | ascii_downcase | contains("egress")) | not))' <<<"$pod_json" >/dev/null \
    || fail "$ns/$pod lost the digest-pinned execd or unexpectedly gained an egress sidecar"
  kubectl -n "$ns" exec "$pod" -c "$container" -- /bin/sh -c 'true' \
    || fail "$ns/$pod workload container is not executable"
done
kubectl -n "$namespace_a" exec "$pod_a" -c "$container_a" -- /tmp/m35-netprobe resolve localhost >/dev/null \
  || fail "the installed sandbox network probe is not executable"
pass "attacker-controlled probes run inside real gVisor sandbox Pods"

echo "==> verify the shipped exec path uses the same owner boundary"
expect_status 200 "$fixture_dir/owner-a.curl" POST \
  "/v1/sandboxes/$sandbox_a_id/exec?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/exec-own.sse" \
  --header 'Content-Type: application/json' --data '{"command":"printf m35-owner"}'
grep -q 'm35-owner' "$fixture_dir/exec-own.sse" \
  || fail "owner exec returned 200 without the expected SSE output"
expect_status 404 "$fixture_dir/member-b.curl" POST \
  "/v1/sandboxes/$sandbox_a_id/exec?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/exec-cross-owner.json" \
  --header 'Content-Type: application/json' --data '{"command":"true"}'
[ "$(jq -r '.code' "$fixture_dir/exec-cross-owner.json")" = "SANDBOX_NOT_FOUND" ] \
  || fail "cross-owner exec did not use SANDBOX_NOT_FOUND"
expect_status 200 "$fixture_dir/admin-a.curl" POST \
  "/v1/sandboxes/$sandbox_a_id/exec?ownerId=$BEX_WORKSPACE_A" "$fixture_dir/exec-admin.sse" \
  --header 'Content-Type: application/json' --data '{"command":"printf m35-admin"}'
grep -q 'm35-admin' "$fixture_dir/exec-admin.sse" \
  || fail "workspace-admin exec returned 200 without the expected SSE output"
expect_status 404 "$fixture_dir/owner-c.curl" POST \
  "/v1/sandboxes/$sandbox_a_id/exec?ownerId=$BEX_WORKSPACE_B" "$fixture_dir/exec-cross-workspace.json" \
  --header 'Content-Type: application/json' --data '{"command":"true"}'
pass "owner/admin exec succeeds while cross-owner and cross-workspace exec hide existence"

exec_a() { kubectl -n "$namespace_a" exec "$pod_a" -c "$container_a" -- "$@"; }
python_tcp_probe='import socket,sys; s=socket.create_connection((sys.argv[1],int(sys.argv[2])),5); s.close()'

expect_denied() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    fail "$name unexpectedly succeeded"
  fi
  pass "$name denied"
}
expect_allowed() {
  local name="$1"
  shift
  "$@" >/dev/null 2>&1 || fail "$name unexpectedly failed"
  pass "$name allowed"
}

pod_a_ip="$(kubectl -n "$namespace_a" get pod "$pod_a" -o jsonpath='{.status.podIP}')"
pod_b_ip="$(kubectl -n "$namespace_a" get pod "$pod_b" -o jsonpath='{.status.podIP}')"
pod_c_ip="$(kubectl -n "$namespace_b" get pod "$pod_c" -o jsonpath='{.status.podIP}')"
lifecycle_ip="$(kubectl -n opensandbox-system get service opensandbox-server -o jsonpath='{.spec.clusterIP}')"
bex_cp_ip="$(kubectl -n bex-system get service bex-api -o jsonpath='{.spec.clusterIP}')"
exec_gateway_ip="$(kubectl -n bex-system get service bex-ssh-gateway -o jsonpath='{.spec.clusterIP}')"
api_server_ip="$(kubectl -n default get service kubernetes -o jsonpath='{.spec.clusterIP}')"
node_ip="$(kubectl get node "$sandbox_node" -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}')"

expect_allowed "own Pod loopback execd" exec_a /tmp/m35-netprobe tcp 127.0.0.1 44772
expect_denied "same-workspace peer execd" exec_a /tmp/m35-netprobe tcp "$pod_b_ip" 44772
expect_denied "cross-workspace peer execd" exec_a /tmp/m35-netprobe tcp "$pod_c_ip" 44772

kubectl -n "$namespace_a" exec "$pod_b" -c "$container_b" -- sh -c \
  'nohup /bin/busybox httpd -f -p 18080 >/tmp/m35-http.log 2>&1 </dev/null &'
sleep 2
kubectl -n "$namespace_a" expose pod "$pod_b" --name "$peer_service" --port 18080 --target-port 18080 >/dev/null
peer_service_ip="$(kubectl -n "$namespace_a" get service "$peer_service" -o jsonpath='{.spec.clusterIP}')"
expect_denied "same-workspace peer Pod user port" exec_a /tmp/m35-netprobe tcp "$pod_b_ip" 18080
expect_denied "same-workspace peer Service" exec_a /tmp/m35-netprobe tcp "$peer_service_ip" 18080

python_image='python:3.12.13-slim-trixie@sha256:cab2dbf575e971934a81e4622f5aba17aa7929719bd7e31033a3a83b97fd0464'
kubectl -n "$BEX_WORKSPACE_A" run "$hosting_pod" --image "$python_image" --restart Never \
  --labels "app.bex.co/m35-fixture=$run_id" --command -- python3 -m http.server 18081 >/dev/null
kubectl -n "$BEX_WORKSPACE_A" wait --for=condition=Ready "pod/$hosting_pod" --timeout=180s >/dev/null
hosting_ip="$(kubectl -n "$BEX_WORKSPACE_A" get pod "$hosting_pod" -o jsonpath='{.status.podIP}')"
expect_denied "sandbox to same-workspace hosting namespace" exec_a /tmp/m35-netprobe tcp "$hosting_ip" 18081
expect_denied "sandbox to lifecycle API" exec_a /tmp/m35-netprobe tcp "$lifecycle_ip" 8077
expect_denied "sandbox to Kubernetes API" exec_a /tmp/m35-netprobe tcp "$api_server_ip" 443
expect_denied "sandbox to node/kubelet" exec_a /tmp/m35-netprobe tcp "$node_ip" 10250
expect_denied "sandbox to cloud metadata" exec_a /tmp/m35-netprobe tcp 169.254.169.254 80

# www.github.com is publicly resolvable but is intentionally absent from the
# exact allowlist, so failure demonstrates DNS policy rather than NXDOMAIN.
expect_denied "unapproved DNS name resolution" exec_a /tmp/m35-netprobe resolve www.github.com
expect_denied "unique DNS-exfiltration query" exec_a /tmp/m35-netprobe resolve "${run_id}.attacker.test"
expect_allowed "approved FQDN with TLS SNI" exec_a /tmp/m35-netprobe tls api.github.com 443 api.github.com
expect_denied "unapproved public FQDN" exec_a /tmp/m35-netprobe tls example.com 443 example.com
approved_ip="$(exec_a /tmp/m35-netprobe resolve api.github.com | tail -1)"
[ -n "$approved_ip" ] || fail "could not resolve api.github.com inside the sandbox"
expect_denied "approved destination by literal IP/wrong SNI" exec_a /tmp/m35-netprobe tls "$approved_ip" 443 example.com
expect_allowed "learned approved public IP with approved SNI" exec_a /tmp/m35-netprobe tls "$approved_ip" 443 api.github.com
expect_denied "private Kubernetes API target with approved SNI" exec_a /tmp/m35-netprobe tls "$api_server_ip" 443 api.github.com

echo "==> verify agent-session setup/agent egress phase split"
render_session_policy() {
  local phase="$1" include_registry="$2" registry_dns="" registry_fqdn="" registry_sni=""
  if [ "$include_registry" = true ]; then
    registry_dns='              - matchName: registry.npmjs.org'
    registry_fqdn='        - matchName: registry.npmjs.org'
    registry_sni='            - registry.npmjs.org'
  fi
  cat <<YAML
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: $session_policy
  namespace: $namespace_a
  labels:
    app.kubernetes.io/managed-by: bex-session-egress
    bex.co/agent-session: $session_label
  annotations:
    bex.co/egress-phase: $phase
    bex.co/model-endpoint: api.github.com
    bex.co/egress-allowlist: '["example.com"]'
    bex.co/egress-allowlist-hash: 100680ad546ce6a577f42f52df33b4cf1f2459085590b33bd215076c513b1782
spec:
  endpointSelector:
    matchLabels:
      bex.co/agent-session: $session_label
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s:k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: ANY
          rules:
            dns:
              - matchName: api.github.com
              - matchName: example.com
              - matchName: bex-ssh-gateway.bex-system.svc.cluster.local
$registry_dns
    - toFQDNs:
        - matchName: api.github.com
        - matchName: example.com
$registry_fqdn
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
          serverNames:
            - api.github.com
            - example.com
$registry_sni
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: bex-system
            k8s:io.cilium.k8s.policy.serviceaccount: bex-ssh-gateway
            k8s:app.kubernetes.io/name: bex-ssh-gateway
      toPorts:
        - ports:
            - port: "8082"
              protocol: TCP
YAML
}

# The exact writer shape must pass the bex-api admission boundary, while a CIDR
# escape hatch must fail even in dry-run. Apply the real fixture as cluster
# admin, then label the Pod so it atomically leaves the legacy allowlist.
render_session_policy setup true \
  | kubectl create --dry-run=server --as "$bex_api_as" -f - >/dev/null \
  || fail "bex-api admission rejected the exact session egress policy"
if render_session_policy setup true \
  | yq '.spec.egress += [{"toCIDR":["0.0.0.0/0"]}]' - \
  | kubectl create --dry-run=server --as "$bex_api_as" -f - >"$fixture_dir/session-cidr-deny.txt" 2>&1; then
  fail "session egress admission allowed a public CIDR escape hatch"
fi
grep -Eq 'Forbidden|exact FQDN or approved endpoint rules' "$fixture_dir/session-cidr-deny.txt" \
  || fail "session CIDR policy failed without admission-boundary evidence"
render_session_policy setup true | kubectl apply -f - >/dev/null
kubectl -n "$namespace_a" label pod "$pod_a" "bex.co/agent-session=$session_label" --overwrite >/dev/null

expect_eventually() {
  local mode="$1" name="$2"
  shift 2
  local deadline=$((SECONDS + 90))
  while ((SECONDS < deadline)); do
    if "$@" >/dev/null 2>&1; then
      [ "$mode" = allowed ] && { pass "$name allowed"; return; }
    else
      [ "$mode" = denied ] && { pass "$name denied"; return; }
    fi
    sleep 3
  done
  fail "$name did not become $mode"
}
expect_eventually allowed "setup-phase package registry" exec_a /tmp/m35-netprobe tls registry.npmjs.org 443 registry.npmjs.org
expect_eventually allowed "tenant allowlisted destination" exec_a /tmp/m35-netprobe tls example.com 443 example.com
expect_eventually denied "setup-phase non-allowlisted destination" exec_a /tmp/m35-netprobe tls www.github.com 443 www.github.com

render_session_policy agent false | kubectl apply -f - >/dev/null
expect_eventually denied "agent-phase package registry" exec_a /tmp/m35-netprobe tls registry.npmjs.org 443 registry.npmjs.org
expect_eventually allowed "agent-phase GitHub baseline" exec_a /tmp/m35-netprobe tls api.github.com 443 api.github.com
expect_eventually allowed "agent-phase tenant allowlisted destination" exec_a /tmp/m35-netprobe tls example.com 443 example.com
expect_eventually denied "agent-phase non-allowlisted destination" exec_a /tmp/m35-netprobe tls www.github.com 443 www.github.com
expect_denied "agent-phase same-workspace cross-sandbox isolation" exec_a /tmp/m35-netprobe tcp "$pod_b_ip" 44772
pass "setup registries narrow to the exact agent baseline plus immutable tenant widening"

kubectl -n bex-system run "$platform_pod" --image "$python_image" --restart Never \
  --labels "app.kubernetes.io/name=bex-api" \
  --overrides='{"spec":{"securityContext":{"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"'"$platform_pod"'","image":"'"$python_image"'","command":["sleep","300"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}' >/dev/null
kubectl -n bex-system wait --for=condition=Ready "pod/$platform_pod" --timeout=180s >/dev/null
expect_denied "bex-api label spoof on the default ServiceAccount" \
  kubectl -n bex-system exec "$platform_pod" -- python3 -c "$python_tcp_probe" "$lifecycle_ip" 8077
expect_denied "bex-api label spoof to sandbox exec gateway" \
  kubectl -n bex-system exec "$platform_pod" -- python3 -c "$python_tcp_probe" "$exec_gateway_ip" 8081

kubectl -n bex-system run "$platform_allowed_pod" --image "$python_image" --restart Never \
  --labels "app.kubernetes.io/name=bex-api" \
  --overrides='{"spec":{"serviceAccountName":"bex-api","automountServiceAccountToken":false,"securityContext":{"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"'"$platform_allowed_pod"'","image":"'"$python_image"'","command":["sleep","300"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}' >/dev/null
kubectl -n bex-system wait --for=condition=Ready "pod/$platform_allowed_pod" --timeout=180s >/dev/null
expect_allowed "bex-api namespace + ServiceAccount + workload identity" \
  kubectl -n bex-system exec "$platform_allowed_pod" -- python3 -c "$python_tcp_probe" "$lifecycle_ip" 8077
expect_allowed "bex-api identity to sandbox exec gateway" \
  kubectl -n bex-system exec "$platform_allowed_pod" -- python3 -c "$python_tcp_probe" "$exec_gateway_ip" 8081

kubectl -n opensandbox-system run "$execd_spoof_pod" --image "$python_image" --restart Never \
  --labels "app.kubernetes.io/name=opensandbox-server" \
  --overrides='{"spec":{"automountServiceAccountToken":false,"securityContext":{"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"'"$execd_spoof_pod"'","image":"'"$python_image"'","command":["sleep","300"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}' >/dev/null
kubectl -n opensandbox-system wait --for=condition=Ready "pod/$execd_spoof_pod" --timeout=180s >/dev/null
expect_denied "lifecycle-server label spoof on the default ServiceAccount" \
  kubectl -n opensandbox-system exec "$execd_spoof_pod" -- python3 -c "$python_tcp_probe" "$pod_a_ip" 44772

kubectl -n opensandbox-system run "$execd_allowed_pod" --image "$python_image" --restart Never \
  --labels "app.kubernetes.io/name=opensandbox-server" \
  --overrides='{"spec":{"serviceAccountName":"opensandbox-server","automountServiceAccountToken":false,"securityContext":{"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"'"$execd_allowed_pod"'","image":"'"$python_image"'","command":["sleep","300"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}' >/dev/null
kubectl -n opensandbox-system wait --for=condition=Ready "pod/$execd_allowed_pod" --timeout=180s >/dev/null
expect_allowed "lifecycle-server namespace + ServiceAccount + workload identity to execd" \
  kubectl -n opensandbox-system exec "$execd_allowed_pod" -- python3 -c "$python_tcp_probe" "$pod_a_ip" 44772
expect_allowed "lifecycle-server fixed tenant-resolver DNS" \
  kubectl -n opensandbox-system exec "$execd_allowed_pod" -- python3 -c \
    'import socket; socket.getaddrinfo("bex-api.bex-system.svc",8091)'
expect_allowed "lifecycle-server tenant-resolver connection" \
  kubectl -n opensandbox-system exec "$execd_allowed_pod" -- python3 -c "$python_tcp_probe" "$bex_cp_ip" 8091
expect_denied "lifecycle-server DNS exfiltration" \
  kubectl -n opensandbox-system exec "$execd_allowed_pod" -- python3 -c \
    'import socket,sys; socket.getaddrinfo(sys.argv[1],53)' "${run_id}.server-attacker.test"

kubectl -n opensandbox-system run "$controller_probe_pod" --image "$python_image" --restart Never \
  --labels "app.kubernetes.io/name=opensandbox" \
  --overrides='{"spec":{"serviceAccountName":"opensandbox-controller-manager","automountServiceAccountToken":false,"securityContext":{"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"'"$controller_probe_pod"'","image":"'"$python_image"'","command":["sleep","300"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}' >/dev/null
kubectl -n opensandbox-system wait --for=condition=Ready "pod/$controller_probe_pod" --timeout=180s >/dev/null
expect_allowed "controller identity to Kubernetes API" \
  kubectl -n opensandbox-system exec "$controller_probe_pod" -- python3 -c "$python_tcp_probe" "$api_server_ip" 443
expect_denied "controller DNS exfiltration" \
  kubectl -n opensandbox-system exec "$controller_probe_pod" -- python3 -c \
    'import socket,sys; socket.getaddrinfo(sys.argv[1],53)' "${run_id}.controller-attacker.test"

echo "==> verify lifecycle-server ServiceAccount least privilege"
can_i() { kubectl auth can-i "$@" --as "$server_as"; }
[ "$(can_i create batchsandboxes.sandbox.opensandbox.io -n "$namespace_a")" = yes ] \
  || fail "server SA cannot create BatchSandboxes in an authorized sandbox namespace"
[ "$(can_i delete pods -n "$namespace_a")" = yes ] \
  || fail "server SA cannot delete Pods in an authorized sandbox namespace"
for tuple in \
  "create pods -n $BEX_WORKSPACE_A" \
  "create pods -n bex-system" \
  "create pods -n opensandbox-system" \
  "create namespaces" \
  "create clusterroles.rbac.authorization.k8s.io" \
  "get secrets -n $namespace_a"; do
  # shellcheck disable=SC2086
  [ "$(can_i $tuple)" = no ] || fail "server SA unexpectedly can $tuple"
done
[ "$(can_i list batchsandboxes.sandbox.opensandbox.io --all-namespaces)" = yes ] \
  || fail "server SA lost its required read-only informer permission"
pass "server SA mutates only provisioned sandbox namespaces and reads no Secrets"

controller_can_i() { kubectl auth can-i "$@" --as "$controller_as"; }
[ "$(controller_can_i create pods -n "$namespace_a")" = yes ] \
  || fail "controller SA cannot reconcile Pods in an authorized sandbox namespace"
[ "$(controller_can_i update batchsandboxes.sandbox.opensandbox.io/status -n "$namespace_a")" = yes ] \
  || fail "controller SA cannot update BatchSandbox status in an authorized sandbox namespace"
for tuple in \
  "create pods -n $BEX_WORKSPACE_A" \
  "create pods -n bex-system" \
  "create pods -n opensandbox-system" \
  "create namespaces" \
  "get secrets -n $namespace_a" \
  "list secrets --all-namespaces"; do
  # shellcheck disable=SC2086
  [ "$(controller_can_i $tuple)" = no ] || fail "controller SA unexpectedly can $tuple"
done
[ "$(controller_can_i list batchsandboxes.sandbox.opensandbox.io --all-namespaces)" = yes ] \
  || fail "controller SA lost required read-only informer permission"
pass "controller SA also has cluster reads plus sandbox-namespace writes only"

echo "==> verify sandbox Pod runtime admission boundary"
if kubectl create --dry-run=server --as "$controller_as" -f - \
  >"$fixture_dir/runc-pod-deny.txt" 2>&1 <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${run_id}-runc-denied
  namespace: $namespace_a
  labels:
    app.bex.co/regime: sandbox
spec:
  automountServiceAccountToken: false
  nodeSelector:
    bex.co/pool: sandbox
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "60"]
YAML
then
  fail "sandbox Pod admission allowed a controller-created runc Pod"
fi
grep -Eq 'Forbidden|sandbox Pods must retain the gVisor runtime' "$fixture_dir/runc-pod-deny.txt" \
  || fail "runc Pod failed without sandbox runtime-admission evidence"
if kubectl create --dry-run=server --as "$controller_as" -f - \
  >"$fixture_dir/node-name-deny.txt" 2>&1 <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${run_id}-node-name-denied
  namespace: $namespace_a
  labels:
    app.bex.co/regime: sandbox
spec:
  runtimeClassName: gvisor
  automountServiceAccountToken: false
  nodeName: $sandbox_node
  nodeSelector:
    bex.co/pool: sandbox
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "60"]
YAML
then
  fail "sandbox Pod admission allowed a create-time nodeName scheduler bypass"
fi
grep -Eq 'Forbidden|scheduler-enforced sandbox placement' "$fixture_dir/node-name-deny.txt" \
  || fail "nodeName Pod failed without sandbox placement-admission evidence"
if kubectl create --dry-run=server --as "$controller_as" -f - \
  >"$fixture_dir/wrong-pool-deny.txt" 2>&1 <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${run_id}-wrong-pool-denied
  namespace: $namespace_a
  labels:
    app.bex.co/regime: sandbox
spec:
  runtimeClassName: gvisor
  automountServiceAccountToken: false
  nodeSelector:
    bex.co/pool: platform
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "60"]
YAML
then
  fail "sandbox Pod admission allowed non-sandbox node placement"
fi
grep -Eq 'Forbidden|scheduler-enforced sandbox placement' "$fixture_dir/wrong-pool-deny.txt" \
  || fail "wrong-pool Pod failed without sandbox placement-admission evidence"
if kubectl create --dry-run=server --as "$controller_as" -f - \
  >"$fixture_dir/token-mount-deny.txt" 2>&1 <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${run_id}-token-mount-denied
  namespace: $namespace_a
  labels:
    app.bex.co/regime: sandbox
spec:
  runtimeClassName: gvisor
  automountServiceAccountToken: true
  nodeSelector:
    bex.co/pool: sandbox
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "60"]
YAML
then
  fail "sandbox Pod admission allowed automatic ServiceAccount-token mounting"
fi
grep -Eq 'Forbidden|disabled ServiceAccount token' "$fixture_dir/token-mount-deny.txt" \
  || fail "token-mount Pod failed without sandbox runtime-admission evidence"
if kubectl create --dry-run=server --as "$controller_as" -f - \
  >"$fixture_dir/regime-label-deny.txt" 2>&1 <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${run_id}-regime-label-denied
  namespace: $namespace_a
spec:
  runtimeClassName: gvisor
  automountServiceAccountToken: false
  nodeSelector:
    bex.co/pool: sandbox
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "60"]
YAML
then
  fail "sandbox Pod admission allowed omission of the sandbox regime identity"
fi
grep -Eq 'Forbidden|regime identity' "$fixture_dir/regime-label-deny.txt" \
  || fail "unlabeled Pod failed without sandbox identity-admission evidence"
if kubectl create --dry-run=server --as "$controller_as" -f - \
  >"$fixture_dir/host-path-deny.txt" 2>&1 <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${run_id}-host-path-denied
  namespace: $namespace_a
  labels:
    app.bex.co/regime: sandbox
spec:
  runtimeClassName: gvisor
  automountServiceAccountToken: false
  nodeSelector:
    bex.co/pool: sandbox
  volumes:
    - name: host
      hostPath:
        path: /
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "60"]
      volumeMounts:
        - name: host
          mountPath: /host
YAML
then
  fail "sandbox baseline Pod Security allowed a hostPath mount"
fi
grep -Eq 'Forbidden|violates PodSecurity.*baseline|hostPath' "$fixture_dir/host-path-deny.txt" \
  || fail "hostPath Pod failed without baseline-PSS evidence"
kubectl create --dry-run=server --as "$controller_as" -f - >/dev/null <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${run_id}-gvisor-admitted
  namespace: $namespace_a
  labels:
    app.bex.co/regime: sandbox
spec:
  runtimeClassName: gvisor
  automountServiceAccountToken: false
  nodeSelector:
    bex.co/pool: sandbox
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "60"]
YAML
pass "API server rejects sandbox runtime bypass and admits the exact gVisor Pod shape"

echo "==> verify NamespaceReconciler admission boundary"
# RBAC deliberately grants these verbs cluster-wide because tenant namespace
# names are dynamic. The API-server CEL boundary, not this can-i result, must
# prevent their use against platform namespaces.
[ "$(kubectl auth can-i delete namespaces --as "$bex_api_as")" = yes ] \
  || fail "bex-api unexpectedly lost the NamespaceReconciler namespace verb"
if kubectl delete namespace kube-system --dry-run=server --as "$bex_api_as" \
  >"$fixture_dir/admission-deny.txt" 2>&1; then
  fail "bex-api admission boundary allowed deleting kube-system"
fi
grep -Eq 'Forbidden|canonical bex-managed tenant namespaces' "$fixture_dir/admission-deny.txt" \
  || fail "kube-system delete failed without admission-policy evidence"
if kubectl label namespace kube-system app.kubernetes.io/managed-by=bex-controlplane \
  --overwrite --dry-run=server --as "$bex_api_as" \
  >"$fixture_dir/platform-relabel-deny.txt" 2>&1; then
  fail "bex-api admission boundary allowed relabeling kube-system"
fi
grep -Eq 'Forbidden|canonical bex-managed tenant namespaces' "$fixture_dir/platform-relabel-deny.txt" \
  || fail "kube-system relabel failed without admission-policy evidence"

if kubectl create --dry-run=server --as "$bex_api_as" -f - \
  >"$fixture_dir/bind-deny.txt" 2>&1 <<YAML
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: bex-tenant-operator
  namespace: kube-system
  labels:
    app.kubernetes.io/managed-by: bex-controlplane
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: bex-tenant-operator
subjects:
  - kind: ServiceAccount
    name: bex-controller-manager
    namespace: bex-system
YAML
then
  fail "bex-api admission boundary allowed a Secret-bearing bind in kube-system"
fi
grep -Eq 'Forbidden|canonical bex-managed tenant namespaces' "$fixture_dir/bind-deny.txt" \
  || fail "platform RoleBinding failed without admission-policy evidence"

# The same policies must not block the reconciler's exact managed shapes.
kubectl get namespace "$namespace_a" -o json \
  | jq 'del(.metadata.managedFields)' \
  | kubectl replace --dry-run=server --as "$bex_api_as" -f - >/dev/null \
  || fail "admission policy rejected a canonical managed tenant Namespace update"
if kubectl get namespace "$namespace_a" -o json \
  | jq 'del(.metadata.managedFields) | .metadata.labels["pod-security.kubernetes.io/enforce"] = "privileged"' \
  | kubectl replace --dry-run=server --as "$bex_api_as" -f - \
    >"$fixture_dir/pss-downgrade-deny.txt" 2>&1; then
  fail "bex-api admission boundary allowed a sandbox PSS downgrade"
fi
grep -Eq 'Forbidden|canonical bex-managed tenant namespaces' "$fixture_dir/pss-downgrade-deny.txt" \
  || fail "sandbox PSS downgrade failed without namespace-admission evidence"
kubectl -n "$namespace_a" get networkpolicy default-deny -o json \
  | jq 'del(.metadata.managedFields)' \
  | kubectl replace --dry-run=server --as "$bex_api_as" -f - >/dev/null \
  || fail "admission policy rejected the exact sandbox default-deny NetworkPolicy"
if kubectl create --dry-run=server --as "$bex_api_as" -f - \
  >"$fixture_dir/sandbox-network-allow-deny.txt" 2>&1 <<YAML
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ${run_id}-allow-all
  namespace: $namespace_a
  labels:
    app.kubernetes.io/managed-by: bex-controlplane
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
  ingress:
    - {}
  egress:
    - {}
YAML
then
  fail "bex-api admission boundary allowed an arbitrary sandbox NetworkPolicy"
fi
grep -Eq 'Forbidden|sandbox default-deny' "$fixture_dir/sandbox-network-allow-deny.txt" \
  || fail "sandbox NetworkPolicy escalation failed without admission-policy evidence"
kubectl -n "$namespace_a" get rolebinding bex-tenant-sandbox-server -o json \
  | jq 'del(.metadata.managedFields)' \
  | kubectl replace --dry-run=server --as "$bex_api_as" -f - >/dev/null \
  || fail "admission policy rejected the exact sandbox-server RoleBinding"
kubectl -n "$namespace_a" get rolebinding bex-tenant-ssh-gateway -o json \
  | jq 'del(.metadata.managedFields)' \
  | kubectl replace --dry-run=server --as "$bex_api_as" -f - >/dev/null \
  || fail "admission policy rejected the exact sandbox exec-gateway RoleBinding"
kubectl -n "$BEX_WORKSPACE_A" delete rolebinding bex-tenant-operator \
  --dry-run=server --as "$bex_api_as" >/dev/null \
  || fail "admission policy rejected an exact hosting RoleBinding prune"
if kubectl create --dry-run=server --as "$bex_api_as" -f - \
  >"$fixture_dir/sandbox-operator-deny.txt" 2>&1 <<YAML
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: bex-tenant-operator
  namespace: $namespace_a
  labels:
    app.kubernetes.io/managed-by: bex-controlplane
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: bex-tenant-operator
subjects:
  - kind: ServiceAccount
    name: bex-controller-manager
    namespace: bex-system
YAML
then
  fail "admission boundary allowed the Secret-bearing operator role in a sandbox namespace"
fi
grep -Eq 'Forbidden|exact regime-specific tenant RoleBindings' "$fixture_dir/sandbox-operator-deny.txt" \
  || fail "sandbox operator RoleBinding failed without regime-policy evidence"
pass "NamespaceReconciler admission denies platform/regime escalation and admits exact tenant projections"

echo "==> require Hubble evidence for the denied flows"
hubble_agent_pod="$(kubectl -n kube-system get pods -l k8s-app=cilium \
  --field-selector "spec.nodeName=$sandbox_node" -o json | jq -er '
    first(.items[] | select(any(.status.conditions[]?; .type == "Ready" and .status == "True")) | .metadata.name)')" \
  || fail "no Ready Cilium agent is running on attacker node $sandbox_node"
if command -v hubble >/dev/null 2>&1 \
  && hubble status >/dev/null 2>&1 \
  && hubble observe --from-pod "$namespace_a/$pod_a" \
    --since "$started_at" --verdict DROPPED --output compact \
    >"$fixture_dir/hubble-drops.txt"; then
  : # Prefer an available relay because it includes cluster-wide flow context.
else
  kubectl -n kube-system exec "$hubble_agent_pod" -c cilium-agent -- \
    hubble observe --server unix:///var/run/cilium/hubble.sock \
      --from-pod "$namespace_a/$pod_a" \
      --since "$started_at" --verdict DROPPED --output compact \
      >"$fixture_dir/hubble-drops.txt" \
    || fail "could not collect local Hubble flows from attacker node $sandbox_node"
fi
[ -s "$fixture_dir/hubble-drops.txt" ] || fail "Hubble reported no dropped workspace-A sandbox flow"
grep -q "$pod_a" "$fixture_dir/hubble-drops.txt" \
  || fail "Hubble drops do not identify the attacker sandbox Pod $pod_a"
grep -q "${run_id}.attacker.test" "$fixture_dir/hubble-drops.txt" \
  || fail "Hubble drops do not contain the unique denied DNS-exfiltration query"
pass "Hubble attributes policy drops to the attacker sandbox"

echo "PASS: sandbox isolation, ownership, ingress, egress, and RBAC matrix"
