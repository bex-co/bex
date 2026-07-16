#!/usr/bin/env bash
# verify-elastic.sh — proves the w1/m3 elastic-substrate DoD (bin-pack + autoscale):
#   1. SCALE-UP:   N+1 mutually anti-affine tenant pods make the
#                  cluster-autoscaler add a machine from any N<max baseline
#   2. PACK:       with an asymmetric load, a new pod lands on the FULLEST node
#                  (kube-scheduler NodeResourcesFit MostAllocated, w1/m3 t002)
#   3. SCALE-DOWN: with the load gone, the surplus machine is drained + removed
#                  (back to the MachineDeployment's min-size annotation)
#
# Self-managed by default (w1/m19: CAPI objects live IN the workload cluster).
#   WL_KUBECONFIG   workload kubeconfig (default: fetched via
#                   scripts/fetch-app-kubeconfig.sh — prod; for the CAPD mock
#                   pass infra/local/bex.kubeconfig)
#   MGMT_CTX        ONLY for the never-pivoted CAPD mock: kubecontext holding
#                   the CAPI objects (e.g. kind-bex-mgmt). Unset = self-managed.
#   MD_FILTER       regex naming the ELASTIC (tenant) MachineDeployment
#                   (default: tenant; mock's pool matches worker)
#   SCALE_DOWN_WAIT max seconds for scale-down (default 900)
#
# Tenant nodes are the untainted pool: NOT control-plane and NOT bex.co/pool=platform.
# App-bearing tenant nodes are guarded with scale-down-disabled during the run —
# app images are node-local ctr imports (w1/m19.1); a scale-down would strand them.
# Exit 0 on full pass; non-zero naming the failed check. Requires kubectl, jq.
set -euo pipefail
cd "$(dirname "$0")/.."

SCALE_DOWN_WAIT="${SCALE_DOWN_WAIT:-900}"
MD_FILTER="${MD_FILTER:-tenant|worker}"
NS=elastic-verify
TENANT_SEL='!node-role.kubernetes.io/control-plane,bex.co/pool!=platform'
KUBECONFIG_TMP=""

if [ -z "${WL_KUBECONFIG:-}" ]; then
  KUBECONFIG_TMP=$(mktemp)
  trap 'rm -f "$KUBECONFIG_TMP"' EXIT
  WL_KUBECONFIG="$KUBECONFIG_TMP"
  bash scripts/fetch-app-kubeconfig.sh "$WL_KUBECONFIG" >/dev/null
fi

wl() { KUBECONFIG="$WL_KUBECONFIG" kubectl "$@"; }
# capi(): where the Cluster API objects + autoscaler live — the workload cluster
# itself (self-managed) unless MGMT_CTX points at the mock's mgmt kind cluster.
if [ -n "${MGMT_CTX:-}" ]; then
  capi() { kubectl --context "$MGMT_CTX" "$@"; }
else
  capi() { wl "$@"; }
fi
fail() { echo "FAIL: $*" >&2; cleanup; exit 1; }

GUARDED_NODES=""
cleanup() {
  wl delete ns "$NS" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  for n in $GUARDED_NODES; do
    wl annotate node "$n" cluster-autoscaler.kubernetes.io/scale-down-disabled- >/dev/null 2>&1 || true
  done
  [ -z "$KUBECONFIG_TMP" ] || rm -f "$KUBECONFIG_TMP"
}
trap cleanup EXIT

workers() {
  wl get nodes -l "$TENANT_SEL" -o json 2>/dev/null \
    | jq '[.items[] | select(any(.status.conditions[]; .type == "Ready" and .status == "True"))] | length' \
    || echo 0
}

# pod <name> <cpu> <memory> — a tenant-only pause pod with equal requests/limits.
pod() {
  wl -n "$NS" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata: { name: $1, labels: { app: elastic-verify } }
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: DoesNotExist
          - key: bex.co/pool
            operator: DoesNotExist
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.10
    resources:
      requests: { cpu: $2, memory: $3 }
      limits: { cpu: $2, memory: $3 }
EOF
}

# pinned_pod <name> <node> <cpu> <memory> — make a deliberate asymmetry before
# asking the real scheduler to place the probe.
pinned_pod() {
  wl -n "$NS" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata: { name: $1, labels: { app: elastic-verify } }
spec:
  nodeName: $2
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.10
    resources:
      requests: { cpu: $3, memory: $4 }
      limits: { cpu: $3, memory: $4 }
EOF
}

# Effective-enough request accounting for the steady tenant workloads used by
# this proof. The scheduler scores CPU+memory equally; the anchor creates a wide
# margin so init-container edge cases cannot decide the winner.
node_cpu_req() {
  jq --arg n "$1" '[.items[]
    | select(.spec.nodeName == $n and .status.phase != "Succeeded" and .status.phase != "Failed")
    | .spec.containers[].resources.requests.cpu // "0"
    | if test("m$") then (rtrimstr("m") | tonumber) else (tonumber * 1000) end] | add // 0' <<<"$PODS_JSON"
}

node_mem_req() {
  jq --arg n "$1" '
    def mem_mi:
      if test("Ki$") then (rtrimstr("Ki") | tonumber) / 1024
      elif test("Mi$") then (rtrimstr("Mi") | tonumber)
      elif test("Gi$") then (rtrimstr("Gi") | tonumber) * 1024
      elif test("Ti$") then (rtrimstr("Ti") | tonumber) * 1048576
      elif test("K$") then (rtrimstr("K") | tonumber) / 1048.576
      elif test("M$") then (rtrimstr("M") | tonumber) / 1.048576
      elif test("G$") then (rtrimstr("G") | tonumber) * 953.674316
      else (tonumber / 1048576) end;
    [.items[]
      | select(.spec.nodeName == $n and .status.phase != "Succeeded" and .status.phase != "Failed")
      | .spec.containers[].resources.requests.memory // "0"
      | mem_mi] | add // 0 | floor' <<<"$PODS_JSON"
}

node_alloc_cpu() {
  wl get node "$1" -o json \
    | jq -r '.status.allocatable.cpu | if test("m$") then (rtrimstr("m") | tonumber) else (tonumber * 1000) end'
}

node_alloc_mem() {
  wl get node "$1" -o json | jq -r '
    .status.allocatable.memory
    | if test("Ki$") then ((rtrimstr("Ki") | tonumber) / 1024)
      elif test("Mi$") then (rtrimstr("Mi") | tonumber)
      elif test("Gi$") then ((rtrimstr("Gi") | tonumber) * 1024)
      else (tonumber / 1048576) end
    | floor'
}

echo "==> preflight"
capi get deploy -n default cluster-autoscaler-clusterapi-cluster-autoscaler >/dev/null \
  || fail "cluster-autoscaler not found (self-managed: Argo app 'cluster-autoscaler'; mock: scripts/install-autoscaler.sh on \$MGMT_CTX)"
MD_JSON=$(capi get machinedeployments -n default -o json)
MD=$(jq -r --arg f "$MD_FILTER" '.items[]
      | select(.metadata.annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"])
      | select(.metadata.name | test($f))
      | .metadata.name' <<<"$MD_JSON" | head -1)
[ -n "$MD" ] || fail "no annotated MachineDeployment matches /$MD_FILTER/ (the elastic tenant pool)"
MAX=$(jq -r --arg md "$MD" '.items[] | select(.metadata.name == $md) | .metadata.annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"] | tonumber' <<<"$MD_JSON")
MIN=$(jq -r --arg md "$MD" '.items[] | select(.metadata.name == $md) | .metadata.annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"] | tonumber' <<<"$MD_JSON")
echo "    autoscaler up; elastic MachineDeployment: $MD (min=$MIN max=$MAX)"

# scheduler must be running with the MostAllocated --config (t002)
wl get pods -n kube-system -l component=kube-scheduler -o json \
  | jq -e '.items[0].spec.containers[0].command | index("--config=/etc/kubernetes/scheduler-config.yaml")' >/dev/null \
  || fail "kube-scheduler is not running with --config=/etc/kubernetes/scheduler-config.yaml (t002 not rolled out)"
echo "    kube-scheduler carries the MostAllocated config"

# guard app-bearing tenant nodes: their ctr-imported images must not be scale-down casualties
for n in $(wl get pods -n default -o json 2>/dev/null \
  | jq -r '[.items[] | select(.status.phase == "Running") | .spec.nodeName] | unique | .[]'); do
  if wl get node "$n" -l "$TENANT_SEL" --no-headers >/dev/null 2>&1 && [ -n "$(wl get node "$n" -l "$TENANT_SEL" --no-headers 2>/dev/null)" ]; then
    wl annotate node "$n" cluster-autoscaler.kubernetes.io/scale-down-disabled=true --overwrite >/dev/null
    GUARDED_NODES="$GUARDED_NODES $n"
  fi
done
[ -n "$GUARDED_NODES" ] && echo "    scale-down guard on:$GUARDED_NODES"

NODES_JSON=$(wl get nodes -l "$TENANT_SEL" -o json)
N0=$(jq '[.items[].status.conditions[] | select(.type == "Ready" and .status == "True")] | length' <<<"$NODES_JSON")
[ "$N0" -ge 1 ] || fail "need >=1 Ready tenant worker to start (have $N0)"
[ "$N0" -lt "$MAX" ] || fail "scale-up proof needs tenant workers below max (have $N0, max $MAX)"
BEFORE_NODES=$(jq -r '.items[].metadata.name' <<<"$NODES_JSON" | sort)
echo "    tenant workers=$N0 (max=$MAX)"

wl create ns "$NS" >/dev/null 2>&1 || true

echo "==> 1. scale-up: N+1 tenant pods with required hostname anti-affinity"
SCALE_REPLICAS=$((N0 + 1))
wl -n "$NS" apply -f - >/dev/null <<EOF
apiVersion: apps/v1
kind: Deployment
metadata: { name: scale-up }
spec:
  replicas: $SCALE_REPLICAS
  selector: { matchLabels: { app: elastic-scale-up } }
  template:
    metadata: { labels: { app: elastic-scale-up } }
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/control-plane
                operator: DoesNotExist
              - key: bex.co/pool
                operator: DoesNotExist
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector: { matchLabels: { app: elastic-scale-up } }
            topologyKey: kubernetes.io/hostname
      containers:
      - name: pause
        image: registry.k8s.io/pause:3.10
        resources:
          requests: { cpu: 10m, memory: 16Mi }
          limits: { cpu: 10m, memory: 16Mi }
EOF
wl -n "$NS" rollout status deployment/scale-up --timeout=600s >/dev/null \
  || fail "scale-up: $SCALE_REPLICAS anti-affine pods not Ready after 10m (workers=$(workers), started $N0)"
n=$(workers)
[ "$n" -gt "$N0" ] || fail "scale-up: anti-affine pods Ready but tenant workers=$n did not grow (started $N0)"
AFTER_NODES=$(wl get nodes -l "$TENANT_SEL" -o json | jq -r '.items[].metadata.name' | sort)
NEW_NODE=$(comm -13 <(printf '%s\n' "$BEFORE_NODES") <(printf '%s\n' "$AFTER_NODES") | head -1)
[ -n "$NEW_NODE" ] || fail "scale-up: could not identify the new tenant node"
echo "    PASS: tenant workers $N0 -> $n; new node $NEW_NODE"

echo "==> 2. pack: a small probe lands on the FULLEST worker (MostAllocated)"
wl -n "$NS" delete deployment scale-up --wait=true >/dev/null

ALLOC_CPU=$(node_alloc_cpu "$NEW_NODE")
ALLOC_MEM=$(node_alloc_mem "$NEW_NODE")
ANCHOR_CPU=$((ALLOC_CPU * 50 / 100))
ANCHOR_MEM=$((ALLOC_MEM * 50 / 100))
PROBE_CPU=$((ALLOC_CPU * 5 / 100))
PROBE_MEM=$((ALLOC_MEM * 5 / 100))
pinned_pod anchor "$NEW_NODE" "${ANCHOR_CPU}m" "${ANCHOR_MEM}Mi"
wl -n "$NS" wait pod/anchor --for=condition=Ready --timeout=180s >/dev/null \
  || fail "pack: anchor did not become Ready on $NEW_NODE"

PODS_JSON=$(wl get pods -A -o json)
best_node=""; best_score=-1; best_count=0
for node in $(wl get nodes -o name -l "$TENANT_SEL" | cut -d/ -f2); do
  alloc_cpu=$(node_alloc_cpu "$node")
  alloc_mem=$(node_alloc_mem "$node")
  req_cpu=$(node_cpu_req "$node")
  req_mem=$(node_mem_req "$node")
  if [ "$((req_cpu + PROBE_CPU))" -gt "$alloc_cpu" ] || [ "$((req_mem + PROBE_MEM))" -gt "$alloc_mem" ]; then
    continue
  fi
  score=$(( ((req_cpu + PROBE_CPU) * 10000 / alloc_cpu + (req_mem + PROBE_MEM) * 10000 / alloc_mem) / 2 ))
  if [ "$score" -gt "$best_score" ]; then
    best_node="$node"; best_score="$score"; best_count=1
  elif [ "$score" -eq "$best_score" ]; then
    best_count=$((best_count + 1))
  fi
done
[ -n "$best_node" ] || fail "pack: no tenant worker has room for the probe"
[ "$best_count" -eq 1 ] || fail "pack: pre-probe MostAllocated score has $best_count equal winners; anchor did not create a decisive asymmetry"

pod probe "${PROBE_CPU}m" "${PROBE_MEM}Mi"
wl -n "$NS" wait pod/probe --for=condition=Ready --timeout=180s >/dev/null || fail "pack: probe never became Ready"
probe_node=$(wl -n "$NS" get pod probe -o jsonpath='{.spec.nodeName}')
[ "$probe_node" = "$best_node" ] \
  || fail "pack: probe landed on $probe_node, MostAllocated score winner was $best_node ($best_score)"
echo "    PASS: probe packed onto unique MostAllocated winner $probe_node (score=$best_score)"

echo "==> 3. scale-down: load removed -> surplus machine drained (min-size floor)"
wl -n "$NS" delete pod --all --wait=false >/dev/null
deadline=$(( $(date +%s) + SCALE_DOWN_WAIT ))
while :; do
  n=$(workers)
  [ "$n" -le "$N0" ] && break
  [ "$(date +%s)" -gt "$deadline" ] && fail "scale-down: still $n tenant workers after ${SCALE_DOWN_WAIT}s (started $N0)"
  sleep 20
done
echo "    PASS: tenant workers back to $n"

echo "PASS: elastic substrate verified — scale-up, MostAllocated pack, scale-down"
