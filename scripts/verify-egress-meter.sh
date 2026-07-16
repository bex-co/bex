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

# Isolated, non-casual live traffic matrix for outbound-bandwidth accounting.
# It creates a three-node Cilium 1.19.5 kind cluster unless REUSE_CLUSTER=1,
# runs every included and excluded byte path, then exercises source resets.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CLUSTER_NAME=${CLUSTER_NAME:-bex-egress-matrix}
CONTEXT="kind-${CLUSTER_NAME}"
KEEP_CLUSTER=${KEEP_CLUSTER:-0}
REUSE_CLUSTER=${REUSE_CLUSTER:-0}
OPERATOR_IMAGE="bex/operator:egress-matrix"
FIXTURE_IMAGE="bex/egress-fixture:matrix"
# Route one real-public address entirely inside the disposable Docker network.
# Documentation/benchmark ranges are deliberately excluded by the meter.
PUBLIC_IP=8.8.8.8
PUBLIC_PORT=18000
PIDS=()
CREATED_CLUSTER=0

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  if [[ "$CREATED_CLUSTER" == 1 && "$KEEP_CLUSTER" != 1 ]]; then
    kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

for command in docker kind kubectl helm curl awk go; do
  command -v "$command" >/dev/null || {
    printf 'required command not found: %s\n' "$command" >&2
    exit 1
  }
done

wait_http() {
  local url=$1
  for _ in $(seq 1 60); do
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  printf 'timed out waiting for %s\n' "$url" >&2
  return 1
}

metric_value() {
  local url=$1 metric=$2 needle=${3:-}
  curl -fsS "$url" | awk -v metric="$metric" -v needle="$needle" '
    index($0, metric) == 1 && index($0, needle) > 0 && $1 !~ /^#/ { sum += $NF; found = 1 }
    END { if (found) print sum; else print 0 }
  '
}

source_instance() {
  curl -fsS "$1" | awk '
    /^bex_egress_meter_healthy\{/ {
      line = $0
      sub(/^.*source_instance="/, "", line)
      sub(/".*$/, "", line)
      print line
      exit
    }
  '
}

wait_metric() {
  local url=$1 metric=$2 needle=$3 expected=${4:-}
  local value
  for _ in $(seq 1 60); do
    value=$(metric_value "$url" "$metric" "$needle" 2>/dev/null || true)
    if [[ -n "$value" && ( -z "$expected" || "$value" == "$expected" ) ]]; then
      printf '%s\n' "$value"
      return
    fi
    sleep 1
  done
  printf 'timed out waiting for metric %s %s expected=%s\n' "$metric" "$needle" "$expected" >&2
  return 1
}

wait_positive_metric() {
  local url=$1 metric=$2 needle=$3
  local value
  for _ in $(seq 1 60); do
    value=$(metric_value "$url" "$metric" "$needle" 2>/dev/null || true)
    if [[ -n "$value" ]] && awk -v value="$value" 'BEGIN { exit !(value > 0) }'; then
      printf '%s\n' "$value"
      return
    fi
    sleep 1
  done
  printf 'timed out waiting for positive metric %s %s\n' "$metric" "$needle" >&2
  return 1
}

assert_equal() {
  local label=$1 actual=$2 expected=$3
  awk -v a="$actual" -v e="$expected" 'BEGIN { exit !(a == e) }' || {
    printf '%s: got %s, want %s\n' "$label" "$actual" "$expected" >&2
    exit 1
  }
}

assert_between() {
  local label=$1 actual=$2 minimum=$3 maximum=$4
  awk -v a="$actual" -v lo="$minimum" -v hi="$maximum" 'BEGIN { exit !(a >= lo && a <= hi) }' || {
    printf '%s: got %s, want [%s,%s]\n' "$label" "$actual" "$minimum" "$maximum" >&2
    exit 1
  }
}

start_forward() {
  local namespace=$1 resource=$2
  shift 2
  local log="/tmp/${CLUSTER_NAME}-port-forward-${#PIDS[@]}.log" pid
  for _ in $(seq 1 30); do
    kubectl --context "$CONTEXT" -n "$namespace" port-forward "$resource" "$@" >"$log" 2>&1 &
    pid=$!
    sleep 1
    if kill -0 "$pid" >/dev/null 2>&1; then
      PIDS+=("$pid")
      return
    fi
  done
  printf 'failed to establish port-forward for %s/%s: ' "$namespace" "$resource" >&2
  tail -1 "$log" >&2
  return 1
}

if ! kubectl config get-contexts "$CONTEXT" >/dev/null 2>&1; then
  [[ "$REUSE_CLUSTER" != 1 ]] || {
    printf 'REUSE_CLUSTER=1 but context %s does not exist\n' "$CONTEXT" >&2
    exit 1
  }
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
  API_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${CLUSTER_NAME}-control-plane")
  helm repo add cilium https://helm.cilium.io/ --force-update >/dev/null
  helm repo update cilium >/dev/null
  helm --kube-context "$CONTEXT" upgrade --install cilium cilium/cilium \
    --namespace kube-system --version 1.19.5 \
    --set "k8sServiceHost=${API_IP}" --set k8sServicePort=6443 \
    --set kubeProxyReplacement=true --set ipam.mode=kubernetes \
    --set routingMode=tunnel --set tunnelProtocol=vxlan \
    --set encryption.enabled=true --set encryption.type=wireguard \
    --set bpf.hostLegacyRouting=true --set operator.replicas=1 --wait --timeout 10m
  kubectl --context "$CONTEXT" wait node --all --for=condition=Ready --timeout=5m
fi

# REUSE_CLUSTER is intentionally supported for development, but it must not
# weaken the production-equivalent netfilter path selected by this matrix. A
# desktop VM restart can also reassign the kind control-plane IP, so refresh
# the explicit kube-proxy-free API endpoint on every reuse.
API_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${CLUSTER_NAME}-control-plane")
helm repo add cilium https://helm.cilium.io/ --force-update >/dev/null
helm --kube-context "$CONTEXT" upgrade cilium cilium/cilium \
  --namespace kube-system --version 1.19.5 --reuse-values \
  --set "k8sServiceHost=${API_IP}" --set k8sServicePort=6443 \
  --set bpf.hostLegacyRouting=true --wait --timeout 10m
kubectl --context "$CONTEXT" wait node --all --for=condition=Ready --timeout=5m
kubectl --context "$CONTEXT" -n kube-system rollout status daemonset/cilium --timeout=5m
sleep 5

WORKER="${CLUSTER_NAME}-worker"
PLATFORM="${CLUSTER_NAME}-worker2"
kubectl --context "$CONTEXT" label node "$WORKER" meter-test-enabled=true --overwrite
kubectl --context "$CONTEXT" label node "$PLATFORM" bex.co/pool=platform --overwrite

docker build -t "$OPERATOR_IMAGE" "$ROOT/lego"
docker build -t "$FIXTURE_IMAGE" "$ROOT/scripts/fixtures/egress-matrix"
kind load docker-image --name "$CLUSTER_NAME" "$OPERATOR_IMAGE" "$FIXTURE_IMAGE"
go build -o "/tmp/${CLUSTER_NAME}-egress-fixture" "$ROOT/scripts/fixtures/egress-matrix/main.go"
CLIENT="/tmp/${CLUSTER_NAME}-egress-fixture"

kubectl --context "$CONTEXT" create namespace system --dry-run=client -o yaml | kubectl --context "$CONTEXT" apply -f -
kubectl --context "$CONTEXT" create namespace traefik --dry-run=client -o yaml | kubectl --context "$CONTEXT" apply -f -
kubectl --context "$CONTEXT" apply -f "$ROOT/lego/operator/config/crd/bases/app.bex.co_databases.yaml"
kubectl --context "$CONTEXT" apply -f "$ROOT/lego/operator/config/crd/bases/app.bex.co_keyvalues.yaml"
kubectl --context "$CONTEXT" apply -k "$ROOT/lego/operator/config/egress-meter"
kubectl --context "$CONTEXT" apply -k "$ROOT/lego/operator/config/pg-sni-proxy"
kubectl --context "$CONTEXT" apply -k "$ROOT/lego/operator/config/kv-sni-proxy"
kubectl --context "$CONTEXT" -n system set image daemonset/egress-meter "egress-meter=${OPERATOR_IMAGE}"
kubectl --context "$CONTEXT" -n system set image daemonset/pg-sni-proxy "pg-sni-proxy=${OPERATOR_IMAGE}"
kubectl --context "$CONTEXT" -n system set image daemonset/kv-sni-proxy "kv-sni-proxy=${OPERATOR_IMAGE}"
kubectl --context "$CONTEXT" -n system patch daemonset egress-meter --type=strategic -p \
  '{"spec":{"template":{"spec":{"nodeSelector":{"meter-test-enabled":"true"},"containers":[{"name":"egress-meter","imagePullPolicy":"IfNotPresent"}]}}}}'
kubectl --context "$CONTEXT" -n system patch daemonset pg-sni-proxy --type=strategic -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"pg-sni-proxy","imagePullPolicy":"IfNotPresent"}]}}}}'
kubectl --context "$CONTEXT" -n system patch daemonset kv-sni-proxy --type=strategic -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"kv-sni-proxy","imagePullPolicy":"IfNotPresent"}]}}}}'

kubectl --context "$CONTEXT" apply -k "$ROOT/deploy/gitops/charts/traefik-plugins"
helm repo add traefik https://traefik.github.io/charts >/dev/null
helm repo update traefik >/dev/null
if [[ "$REUSE_CLUSTER" == 1 ]]; then
  # A developer may previously have patched this singleton host-port
  # Deployment. Recreate it so Helm owns the strategy field for this run.
  kubectl --context "$CONTEXT" -n traefik delete deployment traefik --ignore-not-found --wait=true
fi
helm --kube-context "$CONTEXT" upgrade --install traefik traefik/traefik \
  --namespace traefik --version 41.0.0 \
  -f "$ROOT/deploy/gitops/base/values/traefik.values.yaml" --wait --timeout 10m

PLATFORM_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$PLATFORM")
docker exec "$PLATFORM" ip address add "${PUBLIC_IP}/32" dev eth0 2>/dev/null || true
docker exec "$WORKER" ip route replace "${PUBLIC_IP}/32" via "$PLATFORM_IP" dev eth0

# The named Pods are intentionally immutable fixtures. Removing them makes a
# REUSE_CLUSTER development run as deterministic as a fresh CI cluster.
kubectl --context "$CONTEXT" -n default delete pod \
  public-egress-server direct-client dropped-client private-server \
  --ignore-not-found --wait=true
kubectl --context "$CONTEXT" -n default delete deployment \
  datastore-egress-fixtures ws-egress-fixture --ignore-not-found --wait=true
kubectl --context "$CONTEXT" -n system delete pod \
  -l 'app.bex.co/component in (pg-sni-proxy,kv-sni-proxy)' --wait=true

kubectl --context "$CONTEXT" apply -f - <<YAML
apiVersion: apps/v1
kind: Deployment
metadata: {name: datastore-egress-fixtures, namespace: default}
spec:
  replicas: 1
  selector: {matchLabels: {app: datastore-egress-fixtures}}
  template:
    metadata: {labels: {app: datastore-egress-fixtures}}
    spec:
      nodeName: ${WORKER}
      containers:
        - name: postgres
          image: ${FIXTURE_IMAGE}
          imagePullPolicy: IfNotPresent
          args: ["pg-server", ":5432"]
          ports: [{name: postgres, containerPort: 5432}]
          readinessProbe: {tcpSocket: {port: postgres}}
        - name: key-value
          image: ${FIXTURE_IMAGE}
          imagePullPolicy: IfNotPresent
          args: ["tls-server", ":6380"]
          ports: [{name: tls, containerPort: 6380}]
          readinessProbe: {tcpSocket: {port: tls}}
---
apiVersion: v1
kind: Service
metadata: {name: db-live-rw, namespace: default}
spec:
  selector: {app: datastore-egress-fixtures}
  ports: [{name: postgres, port: 5432, targetPort: postgres}]
---
apiVersion: v1
kind: Service
metadata: {name: kv-live, namespace: default}
spec:
  selector: {app: datastore-egress-fixtures}
  ports: [{name: tls, port: 6380, targetPort: tls}]
---
apiVersion: app.bex.co/v1alpha1
kind: Database
metadata: {name: db-live, namespace: default}
spec:
  public: true
  ipAllowList: [{cidr: 192.0.2.0/24}]
---
apiVersion: app.bex.co/v1alpha1
kind: KeyValue
metadata: {name: kv-live, namespace: default}
spec:
  public: true
  ipAllowList: [{cidr: 192.0.2.0/24}]
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: ws-egress-fixture, namespace: default}
spec:
  replicas: 1
  selector: {matchLabels: {app: ws-egress-fixture}}
  template:
    metadata:
      labels:
        app: ws-egress-fixture
        app.bex.co/app: ws-egress-fixture
        bex.co/app-id: srv-live-ws
    spec:
      nodeName: ${WORKER}
      containers:
        - name: fixture
          image: ${FIXTURE_IMAGE}
          imagePullPolicy: IfNotPresent
          args: ["ws-server", ":8080"]
          ports: [{name: http, containerPort: 8080}]
          readinessProbe: {httpGet: {path: /healthz, port: http}}
---
apiVersion: v1
kind: Service
metadata: {name: ws-egress-fixture, namespace: default}
spec:
  selector: {app: ws-egress-fixture}
  ports: [{port: 80, targetPort: http}]
---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata: {name: ws-egress-meter, namespace: default}
spec:
  plugin:
    bexWebsocketEgress: {appId: srv-live-ws, metricsAddr: ":9101"}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ws-egress-fixture
  namespace: default
  annotations:
    traefik.ingress.kubernetes.io/router.middlewares: default-ws-egress-meter@kubernetescrd
spec:
  ingressClassName: traefik
  rules:
    - host: ws.test
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service: {name: ws-egress-fixture, port: {number: 80}}
---
apiVersion: v1
kind: Pod
metadata:
  name: public-egress-server
  namespace: default
spec:
  hostNetwork: true
  nodeName: ${PLATFORM}
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: ["public-server", "${PUBLIC_IP}:${PUBLIC_PORT}"]
---
apiVersion: v1
kind: Pod
metadata:
  name: direct-client
  namespace: default
  labels:
    meter-case: allowed
    app.bex.co/app: direct-client
    bex.co/app-id: srv-live-direct
spec:
  nodeName: ${WORKER}
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: ["idle"]
---
apiVersion: v1
kind: Pod
metadata:
  name: dropped-client
  namespace: default
  labels:
    meter-case: dropped
    app.bex.co/app: dropped-client
    bex.co/app-id: srv-live-dropped
spec:
  nodeName: ${WORKER}
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: ["idle"]
---
apiVersion: v1
kind: Pod
metadata: {name: private-server, namespace: default, labels: {app: private-server}}
spec:
  nodeName: ${PLATFORM}
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: ["public-server", ":18000"]
---
apiVersion: v1
kind: Service
metadata: {name: private-server, namespace: default}
spec:
  selector: {app: private-server}
  ports: [{port: 18000, targetPort: 18000}]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: drop-public-egress, namespace: default}
spec:
  podSelector: {matchLabels: {meter-case: dropped}}
  policyTypes: [Egress]
  egress: []
YAML

# The public proxy deliberately follows the operator-published status host, not
# spec.public alone. This mirrors the fail-closed production handoff after the
# TLS Certificate and backend port have reconciled.
kubectl --context "$CONTEXT" -n default patch keyvalues.app.bex.co kv-live \
  --subresource=status --type=merge \
  -p '{"status":{"externalHost":"kv-live.kv.bex.co"}}'

kubectl --context "$CONTEXT" -n default rollout status deployment/datastore-egress-fixtures --timeout=3m
kubectl --context "$CONTEXT" -n default rollout status deployment/ws-egress-fixture --timeout=3m
kubectl --context "$CONTEXT" -n default wait pod/direct-client pod/dropped-client pod/private-server pod/public-egress-server --for=condition=Ready --timeout=3m
kubectl --context "$CONTEXT" -n system rollout status daemonset/egress-meter --timeout=3m
kubectl --context "$CONTEXT" -n system rollout status daemonset/pg-sni-proxy --timeout=3m
kubectl --context "$CONTEXT" -n system rollout status daemonset/kv-sni-proxy --timeout=3m

# A Ready Pod can briefly precede Cilium's policy regeneration. Confirm the
# selected endpoint has realized egress enforcement before proving that denied
# packets never reach the post-policy meter hook.
CILIUM_POD=$(kubectl --context "$CONTEXT" -n kube-system get pod -l k8s-app=cilium --field-selector "spec.nodeName=${WORKER}" -o jsonpath='{.items[0].metadata.name}')
DROPPED_ENDPOINT=$(kubectl --context "$CONTEXT" -n default get ciliumendpoint dropped-client -o jsonpath='{.status.id}')
for _ in $(seq 1 60); do
  if kubectl --context "$CONTEXT" -n kube-system exec "$CILIUM_POD" -- cilium-dbg endpoint get "$DROPPED_ENDPOINT" 2>/dev/null | grep -q '"policy-enabled": "egress"'; then
    break
  fi
  sleep 1
done
kubectl --context "$CONTEXT" -n kube-system exec "$CILIUM_POD" -- cilium-dbg endpoint get "$DROPPED_ENDPOINT" 2>/dev/null | grep -q '"policy-enabled": "egress"' || {
  printf 'timed out waiting for Cilium egress policy on dropped-client\n' >&2
  exit 1
}

METER_POD=$(kubectl --context "$CONTEXT" -n system get pod -l app.bex.co/component=egress-meter --field-selector "spec.nodeName=${WORKER}" -o jsonpath='{.items[0].metadata.name}')
start_forward system "pod/${METER_POD}" 19091:9091
start_forward traefik deployment/traefik 18080:80 19100:9100 19101:9101
start_forward system service/pg-sni-proxy 15432:5432 19092:9092
start_forward system service/kv-sni-proxy 16379:6379 19093:9093
wait_http http://127.0.0.1:19091/metrics
wait_http http://127.0.0.1:19100/metrics
wait_http http://127.0.0.1:19092/metrics
wait_http http://127.0.0.1:19093/metrics

DIRECT_METRIC=bex_app_direct_egress_bytes_total
direct_before=$(wait_metric http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
kubectl --context "$CONTEXT" exec direct-client -- /egress-fixture direct-client udp "${PUBLIC_IP}:${PUBLIC_PORT}" 512 128
direct_udp=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
udp_delta=$(awk -v a="$direct_udp" -v b="$direct_before" 'BEGIN { print a-b }')
assert_equal direct_udp_delta "$udp_delta" 544
kubectl --context "$CONTEXT" exec direct-client -- /egress-fixture direct-client tcp "${PUBLIC_IP}:${PUBLIC_PORT}" 4096 1024
direct_tcp=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
tcp_delta=$(awk -v a="$direct_tcp" -v b="$direct_udp" 'BEGIN { print a-b }')
assert_between direct_tcp_delta "$tcp_delta" 4104 6500

private_before=$direct_tcp
kubectl --context "$CONTEXT" exec direct-client -- /egress-fixture direct-client tcp private-server.default.svc.cluster.local:18000 2048 512
private_after=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
assert_equal private_and_cluster_dns_delta "$private_after" "$private_before"
dropped_before=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-dropped"')
if kubectl --context "$CONTEXT" exec dropped-client -- /egress-fixture direct-client udp "${PUBLIC_IP}:${PUBLIC_PORT}" 512 128; then
  printf 'Cilium-denied UDP unexpectedly completed\n' >&2
  exit 1
fi
dropped=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-dropped"')
assert_equal dropped_delta "$dropped" "$dropped_before"

http_before=$(metric_value http://127.0.0.1:19100/metrics traefik_router_responses_bytes_total 'router="default-ws-egress-fixture-ws-test@kubernetes"')
curl -fsS -H 'Host: ws.test' 'http://127.0.0.1:18080/bytes?n=4096' >/dev/null
http_after=$(metric_value http://127.0.0.1:19100/metrics traefik_router_responses_bytes_total 'router="default-ws-egress-fixture-ws-test@kubernetes"')
http_delta=$(awk -v a="$http_after" -v b="$http_before" 'BEGIN { print a-b }')
assert_equal http_router_delta "$http_delta" 4096
"$CLIENT" ws-client 127.0.0.1:18080 ws.test 2048 4096
ws_value=$(wait_positive_metric http://127.0.0.1:19101/metrics bex_websocket_egress_bytes_total 'app_id="srv-live-ws"')
assert_equal websocket_downstream_wire "$ws_value" 4100
ws_direct=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-ws"')
assert_equal websocket_private_backend_hop "$ws_direct" 0

pg_before=$(metric_value http://127.0.0.1:19092/metrics bex_pg_proxy_egress_bytes_total 'resource_id="db-live"')
kv_before=$(metric_value http://127.0.0.1:19093/metrics bex_kv_proxy_egress_bytes_total 'resource_id="kv-live"')
if "$CLIENT" pg-client 127.0.0.1:15432 db-live.db.bex.co 65536 4096; then
  printf 'Postgres allowlist denial unexpectedly completed\n' >&2
  exit 1
fi
if "$CLIENT" tls-client 127.0.0.1:16379 kv-live.kv.bex.co 65536 4096; then
  printf 'Key Value allowlist denial unexpectedly completed\n' >&2
  exit 1
fi
assert_equal denied_postgres_delta "$(metric_value http://127.0.0.1:19092/metrics bex_pg_proxy_egress_bytes_total 'resource_id="db-live"')" "$pg_before"
assert_equal denied_key_value_delta "$(metric_value http://127.0.0.1:19093/metrics bex_kv_proxy_egress_bytes_total 'resource_id="kv-live"')" "$kv_before"
kubectl --context "$CONTEXT" patch database db-live --type=merge -p '{"spec":{"ipAllowList":[]}}'
kubectl --context "$CONTEXT" patch keyvalue kv-live --type=merge -p '{"spec":{"ipAllowList":[]}}'
sleep 2
"$CLIENT" pg-client 127.0.0.1:15432 db-live.db.bex.co 65536 4096
"$CLIENT" tls-client 127.0.0.1:16379 kv-live.kv.bex.co 65536 4096
pg_value=$(wait_positive_metric http://127.0.0.1:19092/metrics bex_pg_proxy_egress_bytes_total 'resource_id="db-live"')
kv_value=$(wait_positive_metric http://127.0.0.1:19093/metrics bex_kv_proxy_egress_bytes_total 'resource_id="kv-live"')
assert_between postgres_downstream_wire "$pg_value" 4096 12000
assert_between key_value_downstream_wire "$kv_value" 4096 12000

# Churn the App Pod, then prove the stable resource counter continues instead
# of following the deleted Pod UID or a potentially reused Pod IP.
pod_churn_before=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
kubectl --context "$CONTEXT" delete pod direct-client --wait=true
kubectl --context "$CONTEXT" apply -f - <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: direct-client
  namespace: default
  labels:
    meter-case: allowed
    app.bex.co/app: direct-client
    bex.co/app-id: srv-live-direct
spec:
  nodeName: ${WORKER}
  containers:
    - name: fixture
      image: ${FIXTURE_IMAGE}
      imagePullPolicy: IfNotPresent
      args: ["idle"]
YAML
kubectl --context "$CONTEXT" wait pod/direct-client --for=condition=Ready --timeout=2m
# The exporter deliberately reconciles Pod UID/IP attribution every five
# seconds. Do not send the first packet from the replacement into that bounded
# observation gap.
sleep 6
pod_churn_retained=$(wait_metric http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
assert_equal pod_churn_retained "$pod_churn_retained" "$pod_churn_before"
kubectl --context "$CONTEXT" exec direct-client -- /egress-fixture direct-client udp "${PUBLIC_IP}:${PUBLIC_PORT}" 512 128
pod_churn_after=$(metric_value http://127.0.0.1:19091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
assert_equal pod_churn_increment "$pod_churn_after" "$(awk -v a="$pod_churn_before" 'BEGIN { print a+544 }')"

source_before=$(source_instance http://127.0.0.1:19091/metrics)
counter_before_restart=$pod_churn_after
kubectl --context "$CONTEXT" -n system delete pod "$METER_POD" --wait=true
sleep 2
kill "${PIDS[0]}" >/dev/null 2>&1 || true
METER_POD=$(kubectl --context "$CONTEXT" -n system get pod -l app.bex.co/component=egress-meter --field-selector "spec.nodeName=${WORKER}" -o jsonpath='{.items[0].metadata.name}')
kubectl --context "$CONTEXT" -n system wait "pod/${METER_POD}" --for=condition=Ready --timeout=2m
start_forward system "pod/${METER_POD}" 29091:9091
wait_http http://127.0.0.1:29091/metrics
source_after=$(source_instance http://127.0.0.1:29091/metrics)
assert_equal process_restart_source_instance "$source_after" "$source_before"
assert_equal process_restart_counter "$(metric_value http://127.0.0.1:29091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')" "$counter_before_restart"
loss_events_before=$(metric_value http://127.0.0.1:29091/metrics bex_egress_meter_counter_loss_events_total 'node=')

# Stop the node meter, remove only its pinned maps/links, and restart. The
# node-local checkpoint must restore the prior monotonic resource counter.
kubectl --context "$CONTEXT" label node "$WORKER" meter-test-enabled-
kubectl --context "$CONTEXT" -n system wait --for=delete "pod/${METER_POD}" --timeout=2m
docker exec "$WORKER" rm -rf /sys/fs/bpf/bex-egress-meter
kubectl --context "$CONTEXT" label node "$WORKER" meter-test-enabled=true
kubectl --context "$CONTEXT" -n system rollout status daemonset/egress-meter --timeout=3m
METER_POD=$(kubectl --context "$CONTEXT" -n system get pod -l app.bex.co/component=egress-meter --field-selector "spec.nodeName=${WORKER}" -o jsonpath='{.items[0].metadata.name}')
start_forward system "pod/${METER_POD}" 39091:9091
wait_http http://127.0.0.1:39091/metrics
assert_equal map_reset_source_instance "$(source_instance http://127.0.0.1:39091/metrics)" "$source_before"
map_restored=$(metric_value http://127.0.0.1:39091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
assert_between map_reset_restored_counter "$map_restored" 1 "$counter_before_restart"
reset_gap=$(awk -v before="$counter_before_restart" -v restored="$map_restored" 'BEGIN { print before-restored }')
loss_events=$(metric_value http://127.0.0.1:39091/metrics bex_egress_meter_counter_loss_events_total 'node=')
assert_equal map_reset_loss_signal "$loss_events" "$(awk -v before="$loss_events_before" 'BEGIN { print before+1 }')"
loss_time=$(metric_value http://127.0.0.1:39091/metrics bex_egress_meter_last_counter_loss_time_seconds 'node=')
assert_between map_reset_loss_timestamp "$loss_time" 1 9999999999

# A Docker-node restart models a tenant node reboot. The source identity and
# restored resource total remain node-persistent; readiness catches any attach
# delay while Cilium and bpffs return.
docker restart "$WORKER" >/dev/null
kubectl --context "$CONTEXT" wait "node/${WORKER}" --for=condition=Ready --timeout=5m
kubectl --context "$CONTEXT" -n kube-system wait pod -l k8s-app=cilium --for=condition=Ready --timeout=5m
kubectl --context "$CONTEXT" -n system rollout status daemonset/egress-meter --timeout=5m
METER_POD=$(kubectl --context "$CONTEXT" -n system get pod -l app.bex.co/component=egress-meter --field-selector "spec.nodeName=${WORKER}" -o jsonpath='{.items[0].metadata.name}')
kubectl --context "$CONTEXT" -n system wait "pod/${METER_POD}" --for=condition=Ready --timeout=5m
start_forward system "pod/${METER_POD}" 49091:9091
wait_http http://127.0.0.1:49091/readyz
assert_equal node_restart_source_instance "$(source_instance http://127.0.0.1:49091/metrics)" "$source_before"
node_counter=$(metric_value http://127.0.0.1:49091/metrics "$DIRECT_METRIC" 'app_id="srv-live-direct"')
assert_equal node_restart_counter "$node_counter" "$map_restored"
node_loss_events=$(metric_value http://127.0.0.1:49091/metrics bex_egress_meter_counter_loss_events_total 'node=')
assert_between node_restart_loss_events "$node_loss_events" "$loss_events" "$(awk -v before="$loss_events" 'BEGIN { print before+1 }')"
node_loss_time=$(metric_value http://127.0.0.1:49091/metrics bex_egress_meter_last_counter_loss_time_seconds 'node=')
assert_between node_restart_loss_timestamp "$node_loss_time" "$loss_time" 9999999999

printf 'PASS HTTP=%s WebSocket=%s direct_udp=%s direct_tcp=%s Postgres=%s KeyValue=%s private=0 dropped=0 loss_events=%s reset_gap=%s\n' \
	"$http_delta" "$ws_value" "$udp_delta" "$tcp_delta" "$pg_value" "$kv_value" "$node_loss_events" "$reset_gap"
