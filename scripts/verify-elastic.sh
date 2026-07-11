#!/usr/bin/env bash
# verify-elastic.sh — proves the w1/m3 elastic-substrate DoD (bin-pack + autoscale):
#   1. SCALE-UP:   an unschedulable pod makes the cluster-autoscaler add a machine
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

if [ -z "${WL_KUBECONFIG:-}" ]; then
  WL_KUBECONFIG=$(mktemp)
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
}
trap cleanup EXIT

workers() { wl get nodes --no-headers -l "$TENANT_SEL" 2>/dev/null | grep -cw Ready || true; }

# pod <name> <cpu> — a pause pod requesting (and limited to) <cpu>
pod() {
  wl -n "$NS" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata: { name: $1, labels: { app: elastic-verify } }
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.10
    resources: { requests: { cpu: $2 }, limits: { cpu: $2 } }
EOF
}

# node_req <node> — CPU requests (millicores) of non-terminal pods on <node>,
# read from the one cluster-wide dump in $PODS_JSON
node_req() {
  jq --arg n "$1" '[.items[]
    | select(.spec.nodeName == $n and .status.phase != "Succeeded" and .status.phase != "Failed")
    | .spec.containers[].resources.requests.cpu // "0"
    | if test("m$") then (rtrimstr("m") | tonumber) else (tonumber * 1000) end] | add // 0' <<<"$PODS_JSON"
}

echo "==> preflight"
capi get deploy -n default cluster-autoscaler-clusterapi-cluster-autoscaler >/dev/null \
  || fail "cluster-autoscaler not found (self-managed: Argo app 'cluster-autoscaler'; mock: scripts/install-autoscaler.sh on \$MGMT_CTX)"
MD=$(capi get machinedeployments -n default -o json \
  | jq -r --arg f "$MD_FILTER" '.items[]
      | select(.metadata.annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"])
      | select(.metadata.name | test($f))
      | .metadata.name' | head -1)
[ -n "$MD" ] || fail "no annotated MachineDeployment matches /$MD_FILTER/ (the elastic tenant pool)"
echo "    autoscaler up; elastic MachineDeployment: $MD"

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
ALLOC=$(jq -r '.items[0].status.allocatable.cpu
  | if test("m$") then rtrimstr("m") | tonumber else tonumber * 1000 end' <<<"$NODES_JSON")
BIG=$(( ALLOC * 65 / 100 ))m; MID=$(( ALLOC * 45 / 100 ))m
echo "    tenant workers=$N0 allocatable=${ALLOC}m -> pods: big=$BIG mid=$MID"

wl create ns "$NS" >/dev/null 2>&1 || true

echo "==> 1. scale-up: big pod fills worker, mid pod overflows -> new machine"
pod big "$BIG"
pod mid "$MID"
# big (65%) + mid (45%) can't co-locate, so mid Running requires a new machine
wl -n "$NS" wait pod/mid --for=jsonpath='{.status.phase}'=Running --timeout=600s >/dev/null \
  || fail "scale-up: mid pod not Running after 10m (workers=$(workers), started $N0)"
n=$(workers)
[ "$n" -gt "$N0" ] || fail "scale-up: mid Running but tenant workers=$n did not grow (started $N0)"
echo "    PASS: tenant workers $N0 -> $n; mid pod scheduled on the new machine"

echo "==> 2. pack: a small probe lands on the FULLEST worker (MostAllocated)"
probe_req=$(( ALLOC * 5 / 100 ))
pod probe "${probe_req}m"
wl -n "$NS" wait pod/probe --for=condition=Ready --timeout=180s >/dev/null || fail "pack: probe never became Ready"
probe_node=$(wl -n "$NS" get pod probe -o jsonpath='{.spec.nodeName}')
PODS_JSON=$(wl get pods -A -o json)   # one dump; node_req reads from it
fullest=""; fullest_req=-1
for node in $(wl get nodes -o name -l "$TENANT_SEL" | cut -d/ -f2); do
  [ "$node" = "$probe_node" ] && continue  # compare against the others' pre-probe load
  r=$(node_req "$node"); [ "$r" -gt "$fullest_req" ] && { fullest_req=$r; fullest=$node; }
done
probe_node_req=$(( $(node_req "$probe_node") - probe_req ))
[ "$probe_node_req" -ge "$fullest_req" ] \
  || fail "pack: probe landed on $probe_node (${probe_node_req}m pre-probe) but $fullest had ${fullest_req}m — scheduler is spreading, not packing"
echo "    PASS: probe packed onto $probe_node (${probe_node_req}m) vs $fullest (${fullest_req}m)"

echo "==> 3. scale-down: load removed -> surplus machine drained (min-size floor)"
wl -n "$NS" delete pod --all --wait=false >/dev/null
sleep 240  # values guarantee no scale-down before unneeded-time (5m); skip futile polls
deadline=$(( $(date +%s) + SCALE_DOWN_WAIT ))
while :; do
  n=$(workers)
  [ "$n" -le "$N0" ] && break
  [ "$(date +%s)" -gt "$deadline" ] && fail "scale-down: still $n tenant workers after ${SCALE_DOWN_WAIT}s (started $N0)"
  sleep 20
done
echo "    PASS: tenant workers back to $n"

echo "PASS: elastic substrate verified — scale-up, MostAllocated pack, scale-down"
