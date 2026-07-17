#!/usr/bin/env bash
# verify-substrate.sh — proves the full w1/m19 Hetzner substrate DoD:
#   1. ACCESS:   the cluster's kubeconfig is fetchable with NO mgmt cluster
#                (hcloud API -> CP node -> SSH admin.conf; scripts/fetch-app-kubeconfig.sh)
#   2. SELF-MGMT: CAPI objects live IN the cluster — machines all Running with
#                nodeRefs; KCP is 3/3 Available and not RollingOut
#   3. SHAPE:     3 tainted CP + 3 tainted platform + >=1 tenant nodes, all Ready
#                with private 10.10/16 InternalIPs
#   4. PLACEMENT: platform-selected pods stay on platform nodes; CAPI controllers
#                stay on control-plane nodes
#   5. DATA:      OpenBao is 3/3 Ready+unsealed and the four platform CNPG
#                clusters have every declared instance Ready
#   6. SCHEDULER: kube-scheduler carries the MostAllocated config
#   7. CSR:       controller-manager logs contain no CSRValidationFailed in 24h
#   8. NETWORK:   CAPH owns network bex; both LBs use private-IP targets and
#                every listener has a healthy backend; a remote node scan sees
#                only SSH on the public interface
#   9. AUTOSCALER: the Argo-managed in-cluster autoscaler is Running
#  10. NO PET:    no bootstrap pet server exists on Hetzner
#
# Env: HCLOUD_TOKEN (required; sourced from ./.env if unset)
#      BEX_SSH_KEY_PATH   SSH key for the CP fetch (default ~/.ssh/bex)
#      WL_KUBECONFIG      skip the fetch and use this kubeconfig (check 1 still
#                         runs unless SKIP_FETCH=1)
# Exit 0 on full pass; non-zero naming the failed check. Requires kubectl, jq,
# curl, and SSH access to a control-plane node.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -z "${HCLOUD_TOKEN:-}" ] && [ -f .env ]; then
  set -a; source ./.env; set +a
fi
: "${HCLOUD_TOKEN:?HCLOUD_TOKEN required (env or .env)}"
export BEX_SSH_KEY_PATH="${BEX_SSH_KEY_PATH:-$HOME/.ssh/bex}"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

# --- 1. ACCESS: mgmt-free kubeconfig fetch -----------------------------------
KUBECONFIG_FILE="${WL_KUBECONFIG:-}"
if [ "${SKIP_FETCH:-0}" != "1" ]; then
  TMP=$(mktemp)
  trap 'rm -f "$TMP"' EXIT
  bash scripts/fetch-app-kubeconfig.sh "$TMP" >/dev/null \
    || fail "access: fetch-app-kubeconfig.sh could not reach a CP node"
  KUBECONFIG_FILE="$TMP"
  pass "access: kubeconfig fetched via hcloud API + CP SSH (no mgmt cluster)"
fi
wl() { kubectl --kubeconfig "$KUBECONFIG_FILE" "$@"; }

# --- 2. SELF-MGMT: machines + healthy KCP reconcile in-cluster ----------------
MACHINES_JSON=$(wl get machines -n default -o json 2>/dev/null) \
  || fail "self-mgmt: 'kubectl get machines' failed in-cluster (CAPI absent?)"
TOTAL=$(echo "$MACHINES_JSON" | jq '.items | length')
[ "$TOTAL" -ge 1 ] || fail "self-mgmt: zero Machine objects in-cluster"
NOT_RUNNING=$(echo "$MACHINES_JSON" | jq '[.items[] | select(.status.phase != "Running")] | length')
[ "$NOT_RUNNING" -eq 0 ] || fail "self-mgmt: $NOT_RUNNING/$TOTAL machines not Running"
NO_NODE=$(echo "$MACHINES_JSON" | jq '[.items[] | select(.status.nodeRef == null)] | length')
[ "$NO_NODE" -eq 0 ] || fail "self-mgmt: $NO_NODE machines have no nodeRef"
KCP_JSON=$(wl get kcp bex-control-plane -n default -o json 2>/dev/null) \
  || fail "self-mgmt: KubeadmControlPlane bex-control-plane absent"
KCP_INIT=$(jq -r '.status.initialization.controlPlaneInitialized // false' <<<"$KCP_JSON")
[ "$KCP_INIT" = "true" ] || fail "self-mgmt: KubeadmControlPlane not initialized in-cluster"
KCP_SPEC=$(jq -r '.spec.replicas // 0' <<<"$KCP_JSON")
KCP_READY=$(jq -r '.status.readyReplicas // 0' <<<"$KCP_JSON")
KCP_AVAILABLE=$(jq -r '[.status.conditions[]? | select(.type == "Available") | .status][0] // ""' <<<"$KCP_JSON")
KCP_ROLLING=$(jq -r '[.status.conditions[]? | select(.type == "RollingOut") | .status][0] // ""' <<<"$KCP_JSON")
[ "$KCP_SPEC" -eq 3 ] && [ "$KCP_READY" -eq 3 ] && [ "$KCP_AVAILABLE" = "True" ] && [ "$KCP_ROLLING" = "False" ] \
  || fail "self-mgmt: KCP spec/ready=$KCP_SPEC/$KCP_READY Available=$KCP_AVAILABLE RollingOut=$KCP_ROLLING (want 3/3 True/False)"
pass "self-mgmt: $TOTAL machines Running with nodeRefs; KCP 3/3 Available, not RollingOut"

# --- 3. SHAPE: target node pools + private addressing -------------------------
NODES_JSON=$(wl get nodes -o json)
NOT_READY=$(jq '[.items[] | select(any(.status.conditions[]; .type == "Ready" and .status != "True"))] | length' <<<"$NODES_JSON")
[ "$NOT_READY" -eq 0 ] || fail "shape: $NOT_READY nodes are not Ready"

CP_COUNT=$(jq '[.items[] | select(.metadata.labels["node-role.kubernetes.io/control-plane"] != null)] | length' <<<"$NODES_JSON")
BAD_CP_TAINT=$(jq '[.items[]
  | select(.metadata.labels["node-role.kubernetes.io/control-plane"] != null)
  | select((any(.spec.taints[]?; .key == "node-role.kubernetes.io/control-plane" and .effect == "NoSchedule")) | not)] | length' <<<"$NODES_JSON")
[ "$CP_COUNT" -eq 3 ] && [ "$BAD_CP_TAINT" -eq 0 ] \
  || fail "shape: control-plane count=$CP_COUNT, missing NoSchedule taint=$BAD_CP_TAINT (want 3/0)"

PLATFORM_COUNT=$(jq '[.items[] | select(.metadata.labels["bex.co/pool"] == "platform")] | length' <<<"$NODES_JSON")
BAD_PLATFORM_TAINT=$(jq '[.items[]
  | select(.metadata.labels["bex.co/pool"] == "platform")
  | select((any(.spec.taints[]?; .key == "bex.co/platform" and .effect == "NoSchedule")) | not)] | length' <<<"$NODES_JSON")
[ "$PLATFORM_COUNT" -eq 3 ] && [ "$BAD_PLATFORM_TAINT" -eq 0 ] \
  || fail "shape: platform count=$PLATFORM_COUNT, missing NoSchedule taint=$BAD_PLATFORM_TAINT (want 3/0)"

TENANT_COUNT=$(jq '[.items[]
  | select(.metadata.labels["node-role.kubernetes.io/control-plane"] == null)
  | select((.metadata.labels["bex.co/pool"] // "") != "platform")] | length' <<<"$NODES_JSON")
[ "$TENANT_COUNT" -ge 1 ] || fail "shape: no Ready tenant node"
BAD_PRIVATE_IP=$(jq '[.items[]
  | select(([.status.addresses[] | select(.type == "InternalIP" and (.address | test("^10\\.10\\.")))] | length) != 1)] | length' <<<"$NODES_JSON")
[ "$BAD_PRIVATE_IP" -eq 0 ] || fail "shape: $BAD_PRIVATE_IP nodes lack one 10.10/16 InternalIP"
pass "shape: 3 tainted CP + 3 tainted platform + $TENANT_COUNT tenant nodes, all Ready on 10.10/16"

# --- 4. PLACEMENT: platform pods + CAPI controllers ---------------------------
CP_NODES=$(wl get nodes -l node-role.kubernetes.io/control-plane -o jsonpath='{.items[*].metadata.name}')
for ns in capi-system caph-system; do
  POD_NODE=$(wl get pods -n "$ns" -o jsonpath='{.items[0].spec.nodeName}' 2>/dev/null) \
    || fail "placement: no pods in $ns"
  case " $CP_NODES " in
    *" $POD_NODE "*) ;;
    *) fail "placement: $ns pod on '$POD_NODE' (not a CP node)" ;;
  esac
done
PLATFORM_NODES=$(jq '[.items[] | select(.metadata.labels["bex.co/pool"] == "platform") | .metadata.name]' <<<"$NODES_JSON")
PODS_JSON=$(wl get pods -A -o json)
PLATFORM_SELECTED=$(jq '[.items[] | select(.spec.nodeSelector["bex.co/pool"] == "platform")] | length' <<<"$PODS_JSON")
BAD_PLATFORM_PLACEMENT=$(jq --argjson nodes "$PLATFORM_NODES" '[.items[]
  | select(.spec.nodeSelector["bex.co/pool"] == "platform")
  | .spec.nodeName as $node
  | select($node == null or ($nodes | index($node)) == null)] | length' <<<"$PODS_JSON")
BAD_PLATFORM_SELECTOR=$(jq '[.items[]
  | select(.metadata.namespace | test("^(argocd|auth|bex-registry|cert-manager|cnpg-system|dashboard|kpack|opensandbox-system|secrets|traefik)$"))
  | select(.spec.nodeSelector["bex.co/pool"] != "platform")] | length' <<<"$PODS_JSON")
[ "$PLATFORM_SELECTED" -gt 0 ] && [ "$BAD_PLATFORM_PLACEMENT" -eq 0 ] && [ "$BAD_PLATFORM_SELECTOR" -eq 0 ] \
  || fail "placement: selected=$PLATFORM_SELECTED misplaced=$BAD_PLATFORM_PLACEMENT missing-platform-selector=$BAD_PLATFORM_SELECTOR"
pass "placement: $PLATFORM_SELECTED platform-selected pods on platform nodes; CAPI/CAPH controllers on CP"

# --- 5. DATA: OpenBao quorum + platform CNPG HA -------------------------------
BAO_JSON=$(wl get pods -n secrets -l app.kubernetes.io/name=openbao -o json)
BAO_COUNT=$(jq '.items | length' <<<"$BAO_JSON")
BAO_READY=$(jq '[.items[] | select(.status.phase == "Running") | select(all(.status.containerStatuses[]?; .ready == true))] | length' <<<"$BAO_JSON")
BAO_NODES=$(jq '[.items[].spec.nodeName] | unique | length' <<<"$BAO_JSON")
[ "$BAO_COUNT" -eq 3 ] && [ "$BAO_READY" -eq 3 ] && [ "$BAO_NODES" -eq 3 ] \
  || fail "data: OpenBao pods total/Ready/distinct-nodes=$BAO_COUNT/$BAO_READY/$BAO_NODES (want 3/3/3)"
for pod in $(jq -r '.items[].metadata.name' <<<"$BAO_JSON"); do
  wl exec -n secrets "$pod" -- bao status -format=json 2>/dev/null \
    | jq -e '.sealed == false and .ha_enabled == true' >/dev/null \
    || fail "data: $pod is sealed or HA-disabled"
done

for db in auth/hydra-db auth/kratos-db auth/openfga-db bex-system/bex-db; do
  ns="${db%/*}"
  name="${db#*/}"
  db_json=$(wl get clusters.postgresql.cnpg.io -n "$ns" "$name" -o json 2>/dev/null) \
    || fail "data: CNPG $db absent"
  instances=$(jq -r '.spec.instances // 0' <<<"$db_json")
  ready=$(jq -r '.status.readyInstances // 0' <<<"$db_json")
  [ "$instances" -ge 2 ] && [ "$ready" -eq "$instances" ] \
    || fail "data: CNPG $db ready/spec=$ready/$instances (want every declared instance Ready, spec >=2)"
done
pass "data: OpenBao 3/3 unsealed on distinct nodes; four platform CNPG clusters fully Ready"

# --- 6. SCHEDULER: live static pods carry MostAllocated config ----------------
wl get pods -n kube-system -l component=kube-scheduler -o json \
  | jq -e '.items | length == 3 and all(.[]; .spec.containers[0].command | index("--config=/etc/kubernetes/scheduler-config.yaml"))' >/dev/null \
  || fail "scheduler: three kube-scheduler pods do not all carry the MostAllocated config"
pass "scheduler: all three kube-scheduler pods carry scheduler-config.yaml"

# --- 7. CSR: no serving-cert validation failures over controller log window ---
CSR_HITS=0
for pod in $(wl get pods -n kube-system -l component=kube-controller-manager -o name); do
  logs=$(wl logs -n kube-system "$pod" --since=24h 2>/dev/null || true)
  if grep -q 'CSRValidationFailed' <<<"$logs"; then
    CSR_HITS=$((CSR_HITS + 1))
  fi
done
[ "$CSR_HITS" -eq 0 ] || fail "csr: CSRValidationFailed present in $CSR_HITS controller-manager logs over 24h"
pass "csr: no CSRValidationFailed in the three controller-manager 24h log windows"

# --- 8. NETWORK: CAPH network, private LB targets, public node firewall --------
hc() { curl --fail --silent --show-error -H "Authorization: Bearer $HCLOUD_TOKEN" "https://api.hetzner.cloud/v1$1"; }
NETWORKS_JSON=$(hc '/networks?name=bex')
NETWORK_COUNT=$(jq '.networks | length' <<<"$NETWORKS_JSON")
NETWORK_ID=$(jq -r '.networks[0].id // 0' <<<"$NETWORKS_JSON")
NETWORK_SHAPE=$(jq -r '.networks[0] | [.name, .ip_range, .labels["caph-cluster-bex"]] | join(":")' <<<"$NETWORKS_JSON")
[ "$NETWORK_COUNT" -eq 1 ] && [ "$NETWORK_SHAPE" = "bex:10.10.0.0/16:owned" ] \
  || fail "network: CAPH network count/shape=$NETWORK_COUNT/$NETWORK_SHAPE"

SERVERS_JSON=$(hc '/servers?label_selector=caph-cluster-bex')
SERVER_COUNT=$(jq '.servers | length' <<<"$SERVERS_JSON")
BAD_SERVER_NETWORK=$(jq --argjson id "$NETWORK_ID" '[.servers[] | select((any(.private_net[]?; .network == $id)) | not)] | length' <<<"$SERVERS_JSON")
[ "$SERVER_COUNT" -eq "$TOTAL" ] && [ "$BAD_SERVER_NETWORK" -eq 0 ] \
  || fail "network: Hetzner servers/machines=$SERVER_COUNT/$TOTAL, missing bex network=$BAD_SERVER_NETWORK"

LBS_JSON=$(hc '/load_balancers')
TRAEFIK_COUNT=$(jq '[.load_balancers[] | select(.name == "bex-traefik")] | length' <<<"$LBS_JSON")
APISERVER_COUNT=$(jq '[.load_balancers[] | select(.name | startswith("bex-kube-apiserver"))] | length' <<<"$LBS_JSON")
[ "$TRAEFIK_COUNT" -eq 1 ] && [ "$APISERVER_COUNT" -eq 1 ] \
  || fail "network: Traefik/API LB counts=$TRAEFIK_COUNT/$APISERVER_COUNT (want 1/1)"
TRAEFIK_PORTS=$(jq -r '[.load_balancers[] | select(.name == "bex-traefik") | .services[].listen_port] | sort | join(",")' <<<"$LBS_JSON")
APISERVER_PORTS=$(jq -r '[.load_balancers[] | select(.name | startswith("bex-kube-apiserver")) | .services[].listen_port] | sort | join(",")' <<<"$LBS_JSON")
BAD_LB=$(jq --argjson id "$NETWORK_ID" '[.load_balancers[]
  | select(.name == "bex-traefik" or (.name | startswith("bex-kube-apiserver")))
  | (.targets // []) as $targets
  | ([.services[].listen_port] | unique) as $ports
  # An explicit server target carries health_status directly. A label-selector
  # target carries its resolved server targets (and their health) under targets.
  # With externalTrafficPolicy=Local, matched workers without a local endpoint
  # are expected to be unhealthy; availability requires one healthy backend for
  # every listener, not every selector-matched worker healthy for every port.
  | ([$targets[]
      | if .type == "label_selector" then (.targets // [])[] else . end
      | .health_status[]?
      | select(.status == "healthy")
      | .listen_port] | unique) as $healthy_ports
  | select(((any(.private_net[]?; .network == $id)) | not)
      or (($targets | length) == 0)
      or any($targets[]?; .use_private_ip != true)
      or (($ports - $healthy_ports) | length) > 0)] | length' <<<"$LBS_JSON")
[ "$TRAEFIK_PORTS" = "22,80,443,5432,6379" ] && [ "$APISERVER_PORTS" = "443" ] && [ "$BAD_LB" -eq 0 ] \
  || fail "network: LB ports traefik/api=$TRAEFIK_PORTS/$APISERVER_PORTS, unavailable-or-public-target LBs=$BAD_LB"

SOURCE_IP=$(jq -r '[.servers[] | select(.labels.machine_type == "control_plane")][0].public_net.ipv4.ip // ""' <<<"$SERVERS_JSON")
TARGET_IP=$(jq -r --arg source "$SOURCE_IP" '[.servers[] | select(.public_net.ipv4.ip != $source)][0].public_net.ipv4.ip // ""' <<<"$SERVERS_JSON")
[ -n "$SOURCE_IP" ] && [ -n "$TARGET_IP" ] || fail "network: need two public node IPs for the remote firewall scan"
SCAN=$(ssh -o BatchMode=yes -o ConnectTimeout=8 -o StrictHostKeyChecking=accept-new -i "$BEX_SSH_KEY_PATH" "root@$SOURCE_IP" \
  "for port in 22 80 443 2379 2380 5000 6443 8472 10250 30080; do (if timeout 2 bash -c '</dev/tcp/$TARGET_IP/'\"\$port\" 2>/dev/null; then echo \"\$port\"; fi) & done; wait") \
  || fail "network: could not scan $TARGET_IP from control-plane node $SOURCE_IP"
[ "$SCAN" = "22" ] || fail "network: public node scan saw open ports '${SCAN//$'\n'/,}' (want only 22)"
pass "network: CAPH-owned 10.10/16; private healthy LB targets; remote node scan only :22"

# --- 9. AUTOSCALER: in-cluster, Running ---------------------------------------
CA_PHASE=$(wl get pods -n default -l app.kubernetes.io/instance=cluster-autoscaler \
  -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
[ "$CA_PHASE" = "Running" ] || fail "autoscaler: in-cluster autoscaler pod not Running (${CA_PHASE:-absent})"
pass "autoscaler: in-cluster cluster-autoscaler Running"

# --- 10. NO PET: bootstrap server retired -------------------------------------
INFRA_COUNT=$(hc '/servers?name=bex-bootstrap' | jq '.servers | length')
[ "$INFRA_COUNT" -eq 0 ] || fail "no-pet: bex-bootstrap server still exists"
pass "no-pet: no bex-bootstrap server on Hetzner"

echo "verify-substrate: ALL PASS"
