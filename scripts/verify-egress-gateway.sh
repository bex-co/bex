#!/usr/bin/env bash
# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Isolated live matrix for stable tenant egress (docs/ADR125-stable-tenant-egress.md,
# w21/m616 t003). It creates a three-node Cilium 1.19.5 kind cluster with the
# datapath ADR125 D9 asks production for (kube-proxy replacement, BPF
# masquerading, the egress gateway, WireGuard with node encryption, legacy host
# routing) and checks, before any of it reaches production:
#
#   1. a tenant pod's public egress leaves from the egress IP, both from another
#      node and from the gateway node itself; a platform pod's does not;
#   2. the free-plan SMTP deny still refuses 587 while the same pod reaches 18001;
#   3. the egress meter counts a gateway-routed UDP datagram exactly once, summed
#      over every node's meter (ADR125 D10);
#   4. how soon a new pod's first connection leaves from the egress IP (D11).
#
# The control-plane node plays the internet: it holds 8.8.8.8/32 and runs the
# peers, and the workers route 8.8.8.8 to it. The egress IP is a spare address
# of the kind network on the gateway node's eth0, as a Floating IP is a /32 on a
# Hetzner node's eth0. The peers report the address they saw; no third-party
# echo service is used. KEEP_CLUSTER=1 keeps the cluster, REUSE_CLUSTER=1
# reuses it.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CLUSTER_NAME=${CLUSTER_NAME:-bex-egress-gw}
CONTEXT="kind-${CLUSTER_NAME}"
KEEP_CLUSTER=${KEEP_CLUSTER:-0}
REUSE_CLUSTER=${REUSE_CLUSTER:-0}
OPERATOR_IMAGE="bex/operator:egress-gw"
FIXTURE_IMAGE="bex/egress-fixture:egress-gw"
PUBLIC_IP=8.8.8.8
BYTES_PORT=18000
SOURCE_PORT=18001
SMTP_PORT=587
METRIC=bex_app_direct_egress_bytes_total
PIDS=()
CREATED_CLUSTER=0
CP="${CLUSTER_NAME}-control-plane"
TENANT_NODE="${CLUSTER_NAME}-worker"
GATEWAY_NODE="${CLUSTER_NAME}-worker2"

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  if [[ "$CREATED_CLUSTER" == 1 && "$KEEP_CLUSTER" != 1 ]]; then
    kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

for command in docker kind kubectl helm curl awk python3 go; do
  command -v "$command" >/dev/null || {
    printf 'required command not found: %s\n' "$command" >&2
    exit 1
  }
done

k() { kubectl --context "$CONTEXT" "$@"; }
node_ip() { docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$1"; }
fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

# source_of <pod> <port>: the address the peer saw for one connection.
source_of() {
  k -n default exec "$1" -- /egress-fixture source-client "${PUBLIC_IP}:$2" 2>/dev/null | awk '$2 == "ok" { print $3 }'
}

wait_source() {
  local pod=$1 want=$2 got=""
  for _ in $(seq 1 60); do
    got=$(source_of "$pod" "$SOURCE_PORT" || true)
    [[ "$got" == "$want" ]] && return 0
    sleep 1
  done
  fail "$pod left from '${got}', want ${want}"
}

metric_sum() {
  local total=0 value port
  for port in "$@"; do
    value=$(curl -fsS "http://127.0.0.1:${port}/metrics" | awk -v metric="$METRIC" -v needle="$NEEDLE" '
      index($0, metric) == 1 && index($0, needle) > 0 { sum += $NF } END { print sum + 0 }')
    total=$(awk -v a="$total" -v b="$value" 'BEGIN { print a + b }')
  done
  printf '%s\n' "$total"
}

start_forward() {
  local pod=$1 port=$2 log pid
  log=$(mktemp)
  for _ in $(seq 1 30); do
    k -n system port-forward "pod/${pod}" "${port}:9091" >"$log" 2>&1 &
    pid=$!
    sleep 1
    if kill -0 "$pid" >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:${port}/metrics" >/dev/null 2>&1; then
      PIDS+=("$pid")
      return
    fi
    kill "$pid" >/dev/null 2>&1 || true
  done
  fail "port-forward to ${pod}: $(tail -1 "$log")"
}

# Extract one YAML document by metadata.name from a multi-document manifest,
# so the matrix applies production's own SMTP deny rather than a copy.
manifest_doc() {
  python3 -I - "$1" "$2" <<'PY'
import re, sys
path, name = sys.argv[1], sys.argv[2]
for doc in re.split(r"(?m)^---\s*$", open(path).read()):
    if re.search(r"(?m)^  name: " + re.escape(name) + r"\s*$", doc):
        print(doc.strip())
        sys.exit(0)
sys.exit(f"{name} not found in {path}")
PY
}

if ! kubectl config get-contexts "$CONTEXT" >/dev/null 2>&1; then
  [[ "$REUSE_CLUSTER" != 1 ]] || fail "REUSE_CLUSTER=1 but context ${CONTEXT} does not exist"
  kind create cluster --name "$CLUSTER_NAME" --image kindest/node:v1.36.1 --config=- <<'YAML'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true
  kubeProxyMode: none
nodes:
  - role: control-plane
  - role: worker
  - role: worker
YAML
  CREATED_CLUSTER=1
fi

API_IP=$(node_ip "$CP")
helm repo add cilium https://helm.cilium.io/ --force-update >/dev/null
helm repo update cilium >/dev/null
# ADR125 D9: production's flags (app-cluster.yml) plus the four the egress
# gateway needs. kind nodes have one NIC, so devices is left to detection.
helm --kube-context "$CONTEXT" upgrade --install cilium cilium/cilium \
  --namespace kube-system --version 1.19.5 \
  --set "k8sServiceHost=${API_IP}" --set k8sServicePort=6443 \
  --set kubeProxyReplacement=true --set ipam.mode=kubernetes \
  --set routingMode=tunnel --set tunnelProtocol=vxlan \
  --set encryption.enabled=true --set encryption.type=wireguard \
  --set encryption.nodeEncryption=true \
  --set bpf.hostLegacyRouting=true --set bpf.masquerade=true \
  --set egressGateway.enabled=true --set l7Proxy=true \
  --set operator.replicas=1 --wait --timeout 10m
k wait node --all --for=condition=Ready --timeout=5m
k -n kube-system rollout status daemonset/cilium --timeout=5m

CP_IP=$(node_ip "$CP")
TENANT_IP=$(node_ip "$TENANT_NODE")
GATEWAY_IP=$(node_ip "$GATEWAY_NODE")
EGRESS_IP=$(python3 -I -c 'import ipaddress, sys; print(list(ipaddress.ip_network(sys.argv[1]).hosts())[-6])' \
  "$(docker network inspect kind -f '{{range .IPAM.Config}}{{.Subnet}} {{end}}' | tr ' ' '\n' | grep -m1 '\.')")
docker exec "$CP" ip address replace "${PUBLIC_IP}/32" dev eth0
docker exec "$GATEWAY_NODE" ip address replace "${EGRESS_IP}/32" dev eth0
for node in "$TENANT_NODE" "$GATEWAY_NODE"; do
  docker exec "$node" ip route replace "${PUBLIC_IP}/32" via "$CP_IP" dev eth0
done
k label node "$GATEWAY_NODE" egress.bex.co/gateway=active --overwrite >/dev/null
printf 'nodes: tenant %s, gateway %s (egress IP %s), internet %s\n' "$TENANT_IP" "$GATEWAY_IP" "$EGRESS_IP" "$CP_IP"

# The matrix runs only the egress meter from the product image, so it builds
# that binary and the fixture into scratch images on the host: no BuildKit and
# no base-image pull, and minutes faster than the full product image.
case $(docker info -f '{{.Architecture}}') in
  aarch64 | arm64) GOARCH=arm64 ;;
  x86_64 | amd64) GOARCH=amd64 ;;
  *) fail "unsupported Docker architecture" ;;
esac
scratch_image() {
  local tag=$1 dir=$2 package=$3 binary=$4 build
  build=$(mktemp -d)
  (cd "$dir" && GOOS=linux GOARCH=$GOARCH CGO_ENABLED=0 GOWORK=off go build -trimpath -o "$build/$binary" "$package")
  printf 'FROM scratch\nCOPY %s /%s\nENTRYPOINT ["/%s"]\n' "$binary" "$binary" "$binary" >"$build/Dockerfile"
  docker build -q -t "$tag" "$build" >/dev/null
  rm -rf -- "$build"
}
scratch_image "$OPERATOR_IMAGE" "$ROOT/lego/operator" ./cmd/egress-meter egress-meter
scratch_image "$FIXTURE_IMAGE" "$ROOT/scripts/fixtures/egress-matrix" ./main.go egress-fixture
kind load docker-image --name "$CLUSTER_NAME" "$OPERATOR_IMAGE" "$FIXTURE_IMAGE" >/dev/null

k create namespace system --dry-run=client -o yaml | k apply -f - >/dev/null
k apply -k "$ROOT/lego/operator/config/egress-meter" >/dev/null
k -n system set image daemonset/egress-meter "egress-meter=${OPERATOR_IMAGE}" >/dev/null
k -n system patch daemonset egress-meter --type=strategic -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"egress-meter","imagePullPolicy":"IfNotPresent"}]}}}}' >/dev/null

k -n default delete pod -l egress-gw-matrix --ignore-not-found --wait=true >/dev/null
k delete ciliumegressgatewaypolicy tenant-egress --ignore-not-found >/dev/null

peer() {
  cat <<YAML
---
apiVersion: v1
kind: Pod
metadata: {name: $1, namespace: default, labels: {egress-gw-matrix: peer}}
spec:
  hostNetwork: true
  nodeName: ${CP}
  tolerations: [{operator: Exists}]
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: [$2, "${PUBLIC_IP}:$3"]
YAML
}

# client <name> <node> <app-id> <extra labels>: an idle fixture pod.
client() {
  cat <<YAML
---
apiVersion: v1
kind: Pod
metadata:
  name: $1
  namespace: default
  labels: {egress-gw-matrix: client, app.bex.co/app: $1, bex.co/app-id: $3$4}
spec:
  nodeName: $2
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: ["idle"]
YAML
}

{
  peer bytes-peer public-server "$BYTES_PORT"
  peer source-peer source-echo "$SOURCE_PORT"
  peer smtp-peer source-echo "$SMTP_PORT"
  client tenant-client "$TENANT_NODE" srv-gw-tenant ", app.bex.co/workspace: tea-gw-matrix"
  client gateway-client "$GATEWAY_NODE" srv-gw-local ", app.bex.co/workspace: tea-gw-matrix"
  client free-client "$TENANT_NODE" srv-gw-free ", app.bex.co/workspace: tea-gw-matrix, app.bex.co/free-plan: \"true\""
  client platform-client "$TENANT_NODE" srv-gw-platform ""
  printf -- '---\n'
  manifest_doc "$ROOT/deploy/gitops/base/tenant-node-egress.yaml" deny-free-plan-smtp-egress
} | k apply -f - >/dev/null
k -n default wait pod -l egress-gw-matrix --for=condition=Ready --timeout=3m >/dev/null
k -n system rollout status daemonset/egress-meter --timeout=3m >/dev/null

# Control: with no policy, a tenant pod leaves from its own node.
wait_source tenant-client "$TENANT_IP"
printf 'ok: without the policy, tenant egress leaves from its node (%s)\n' "$TENANT_IP"

# The policy t003 ships (ADR125 D5): workspace pods, minus datastores and
# sandboxes, to public destinations. The egress-gateway controller (t004) owns
# the gateway selection and egressIP on production.
k apply -f - >/dev/null <<YAML
apiVersion: cilium.io/v2
kind: CiliumEgressGatewayPolicy
metadata:
  name: tenant-egress
spec:
  selectors:
    - podSelector:
        matchExpressions:
          - {key: app.bex.co/workspace, operator: Exists}
          - {key: cnpg.io/cluster, operator: DoesNotExist}
          - {key: app.bex.co/keyvalue, operator: DoesNotExist}
          - {key: bex.co/sandbox-id, operator: DoesNotExist}
          - {key: bex.co/agent-session, operator: DoesNotExist}
  destinationCIDRs: ["0.0.0.0/0"]
  excludedCIDRs: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10", "169.254.0.0/16"]
  egressGateway:
    nodeSelector:
      matchLabels: {egress.bex.co/gateway: active}
    egressIP: ${EGRESS_IP}
YAML

wait_source tenant-client "$EGRESS_IP"
printf 'ok: a tenant pod on another node leaves from the egress IP\n'
wait_source gateway-client "$EGRESS_IP"
printf 'ok: a tenant pod on the gateway node leaves from the egress IP\n'
platform_source=$(source_of platform-client "$SOURCE_PORT")
[[ "$platform_source" == "$TENANT_IP" ]] || fail "platform pod left from '${platform_source}', want its node ${TENANT_IP}"
printf 'ok: a pod with no workspace label keeps its node address\n'

free_source=$(source_of free-client "$SOURCE_PORT")
[[ "$free_source" == "$EGRESS_IP" ]] || fail "free pod left from '${free_source}' on ${SOURCE_PORT}, want ${EGRESS_IP}"
if k -n default exec free-client -- /egress-fixture source-client "${PUBLIC_IP}:${SMTP_PORT}" >/dev/null 2>&1; then
  fail "free-plan pod reached ${SMTP_PORT} through the gateway"
fi
paid_smtp=$(source_of tenant-client "$SMTP_PORT")
[[ "$paid_smtp" == "$EGRESS_IP" ]] || fail "paid pod's ${SMTP_PORT} left from '${paid_smtp}', want ${EGRESS_IP}"
printf 'ok: the free-plan SMTP deny refuses %s; a paid pod reaches it from the egress IP\n' "$SMTP_PORT"

# ADR125 D10: one gateway-routed datagram, counted once across every meter.
METERS=()
port=19191
for node in "$TENANT_NODE" "$GATEWAY_NODE"; do
  pod=$(k -n system get pod -l app.bex.co/component=egress-meter --field-selector "spec.nodeName=${node}" -o jsonpath='{.items[0].metadata.name}')
  start_forward "$pod" "$port"
  METERS+=("$port")
  port=$((port + 1))
done
for app in srv-gw-tenant srv-gw-local; do
  pod=tenant-client
  [[ "$app" == srv-gw-local ]] && pod=gateway-client
  NEEDLE="app_id=\"${app}\""
  before=$(metric_sum "${METERS[@]}")
  k -n default exec "$pod" -- /egress-fixture direct-client udp "${PUBLIC_IP}:${BYTES_PORT}" 512 128 >/dev/null
  delta=0
  for _ in $(seq 1 30); do
    delta=$(awk -v a="$(metric_sum "${METERS[@]}")" -v b="$before" 'BEGIN { print a - b }')
    [[ "$delta" == 544 ]] && break
    sleep 1
  done
  per_node=$(for p in "${METERS[@]}"; do printf '%s:%s ' "$p" "$(metric_sum "$p")"; done)
  [[ "$delta" == 544 ]] || fail "meter counted ${delta} bytes for ${app}'s 544-byte datagram (per meter: ${per_node})"
  printf 'ok: the meter counts %s once (544 bytes)\n' "$app"
done

# ADR125 D11: a new pod's first connections, from the moment its container runs.
k apply -f - >/dev/null <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: fresh-client
  namespace: default
  labels: {egress-gw-matrix: client, app.bex.co/app: fresh-client, bex.co/app-id: srv-gw-fresh, app.bex.co/workspace: tea-gw-matrix}
spec:
  nodeName: ${TENANT_NODE}
  restartPolicy: Never
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: ["source-client", "${PUBLIC_IP}:${SOURCE_PORT}", "50", "50"]
YAML
k -n default wait pod/fresh-client --for=jsonpath='{.status.phase}'=Succeeded --timeout=2m >/dev/null
fresh=$(k -n default logs fresh-client)
leaked=$(awk -v ip="$EGRESS_IP" '$2 == "ok" && $3 != ip' <<<"$fresh" | wc -l | tr -d ' ')
first_ok=$(awk -v ip="$EGRESS_IP" '$2 == "ok" && $3 == ip { print $1; exit }' <<<"$fresh")
start=$(awk 'NR == 1 { print $1 }' <<<"$fresh")
printf 'new pod: %s of 50 connections left from another address; the first from the egress IP came %s ms after the first attempt\n' \
  "$leaked" "$(awk -v a="${first_ok:-0}" -v b="${start:-0}" 'BEGIN { print a - b }')"
printf '%s\n' "$fresh" | awk '{ print "  " $0 }' | head -8

printf 'egress gateway matrix: ok\n'
