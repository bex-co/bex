#!/usr/bin/env bash
# Stand up the local CAPD mock of the Hetzner substrate, entirely in Docker:
#   kind infra cluster -> Cluster API + Docker provider (CAPD) -> an app
#   cluster whose "machines" are Docker-container nodes. Add/remove a machine by
#   scaling the worker pool. Swap CAPD -> CAPH for Hetzner; bex is unchanged.
#
#   bash scripts/mock-cluster.sh            # bring it up
#   bash scripts/mock-cluster.sh scale 3    # set worker machines = 3 (add/remove)
set -euo pipefail
cd "$(dirname "$0")/.."
export CLUSTER_TOPOLOGY=true                 # CAPD's flavor uses ClusterClass/topology
MGMT=kind-bex-mgmt
WL_KUBECONFIG=infra/local/bex.kubeconfig

# The cluster-autoscaler (w1/m3) owns the worker count, so `scale N` raises the
# tenant pool's min-size floor to N (and max if N exceeds it) instead of setting
# replicas — a replicas write would be a manual override the topology controller
# enforces against the autoscaler. The array patch also clears any stale
# replicas field, handing ownership back to the autoscaler.
scale() {
  local n=$1 max=5; [ "$n" -gt 5 ] && max=$n
  kubectl --context "$MGMT" patch cluster bex --type merge \
    -p "{\"spec\":{\"topology\":{\"workers\":{\"machineDeployments\":[{\"name\":\"worker-0\",\"class\":\"default-worker\",\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size\":\"1\",\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size\":\"1\"}}},{\"name\":\"tenant-0\",\"class\":\"tenant-worker\",\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size\":\"$n\",\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size\":\"$max\"}}}]}}}}"
  echo "tenant worker floor -> $n machine(s) (min-size $n / max-size $max; platform stays at 1)"
  echo "watch: docker ps --format '{{.Names}}' | grep bex-tenant-0"
}
if [ "${1:-}" = scale ]; then scale "${2:?usage: scale N}"; exit 0; fi

# 1. infra cluster (kind) with the docker socket mounted (CAPD needs it)
kind get clusters 2>/dev/null | grep -qx bex-mgmt || kind create cluster --config infra/local/kind-mgmt.yaml
kubectl config use-context "$MGMT" >/dev/null

# 2. Cluster API core + Docker provider (topology enabled from the start).
# clusterctl init returns before the webhooks serve; wait or the apply below
# fails with "failed calling webhook ... connection refused".
kubectl get ns capd-system >/dev/null 2>&1 || clusterctl init --infrastructure docker
for ns in capi-system capd-system capi-kubeadm-bootstrap-system capi-kubeadm-control-plane-system; do
  kubectl -n "$ns" wait deploy --all --for=condition=Available --timeout=300s
done

# 3. the app cluster (Cluster + ClusterClass + MachineDeployment, machines = containers)
kubectl apply -f infra/clusterapi/overlays/local-capd/cluster.yaml

echo "waiting for the app cluster to provision..."
kubectl --context "$MGMT" wait --for=condition=Available cluster/bex --timeout=600s || true
for i in $(seq 1 60); do
  [ "$(kubectl --context "$MGMT" get machines --no-headers 2>/dev/null | grep -c Running)" -ge 2 ] && break; sleep 8
done

# 4. app-cluster kubeconfig — rewrite the server to the lb's host-published port
#    (CAPD's internal API IP isn't reachable from the host), then install a CNI.
clusterctl get kubeconfig bex > "$WL_KUBECONFIG"
LBPORT=$(docker port bex-lb 6443/tcp | head -1 | sed 's/.*://')
sed -i '' "s#server: https://[0-9.]*:6443#server: https://127.0.0.1:$LBPORT#" "$WL_KUBECONFIG"
KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f \
  https://raw.githubusercontent.com/projectcalico/calico/v3.28.2/manifests/calico.yaml >/dev/null
KUBECONFIG="$WL_KUBECONFIG" kubectl wait --for=condition=Ready node --all --timeout=300s || true
# The fixed platform worker joins with bex.co/pool=platform; the scalable tenant
# pool joins with bex.co/pool=tenant. The split mirrors production and lets live
# isolation checks prove tenant execution cannot land on the platform pool.
# Keep cluster DNS on the control-plane node: worker-node pods can't reach the
# apiserver / cross-node services under OrbStack+Calico (docs/ADR004-deployment.md), so
# coredns scheduled onto a worker silently kills DNS for the whole cluster.
KUBECONFIG="$WL_KUBECONFIG" kubectl -n kube-system patch deploy coredns --type merge -p \
  '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
   "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"},
   {"key":"CriticalAddonsOnly","operator":"Exists"}]}}}}' >/dev/null

# 5. cluster-autoscaler beside CAPI (w1/m3) — same installer as prod CI.
#    Why on the mgmt cluster: infra/clusterapi/autoscaler-values.yaml.
bash scripts/install-autoscaler.sh "$MGMT"

echo
echo "app cluster 'bex' up. kubeconfig: $WL_KUBECONFIG"
echo "  nodes:        KUBECONFIG=$WL_KUBECONFIG kubectl get nodes"
echo "  add machine:  bash scripts/mock-cluster.sh scale 3"
echo "  remove:       bash scripts/mock-cluster.sh scale 1"
