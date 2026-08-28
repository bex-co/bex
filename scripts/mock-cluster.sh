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
#
# CAPI templates (KubeadmControlPlaneTemplate, KubeadmConfigTemplate, Docker*Template)
# are IMMUTABLE in spec.template.spec. When this repo changes one of them — w2/m81
# t002 added `rotate-server-certificates` to the control-plane template — a plain
# apply onto a mgmt cluster still holding the older ClusterClass fails with
# "field is immutable", and `set -e` then skipped EVERY step below: no CNI, no
# StorageClass, no cert-manager. The result looked provisioned (machines Running)
# but was unusable, and the failure was invisible unless you read the whole log.
# Templates are pure declarations referenced by name, so recreating the drifted
# ones converges the mgmt cluster onto whatever this repo now declares.
if ! apply_err=$(kubectl apply -f infra/clusterapi/overlays/local-capd/cluster.yaml 2>&1); then
  echo "$apply_err"
  grep -q "field is immutable" <<<"$apply_err" || {
    echo "error: applying cluster.yaml failed for a reason other than template immutability" >&2
    exit 1
  }
  echo "==> ClusterClass templates drifted from this repo — recreating the immutable ones"
  # Only the template kinds are safe to recreate: they are stamped into Machines
  # at creation time, so deleting one cannot disturb a running node.
  for kind in kubeadmcontrolplanetemplate kubeadmconfigtemplate \
              dockerclustertemplate dockermachinetemplate dockermachinepooltemplate; do
    kubectl delete "$kind" --all --ignore-not-found >/dev/null 2>&1 || true
  done
  kubectl apply -f infra/clusterapi/overlays/local-capd/cluster.yaml
fi

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
# apiserver / cross-node services under OrbStack+Calico (docs/ADR004-app-deployment.md), so
# coredns scheduled onto a worker silently kills DNS for the whole cluster.
KUBECONFIG="$WL_KUBECONFIG" kubectl -n kube-system patch deploy coredns --type merge -p \
  '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
   "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"},
   {"key":"CriticalAddonsOnly","operator":"Exists"}]}}}}' >/dev/null
# calico-kube-controllers has the same apiserver dependency — on a worker node it
# crashloops forever (observed at 1903 restarts on the rotted 2026-07-26 cluster; w1/043).
KUBECONFIG="$WL_KUBECONFIG" kubectl -n kube-system patch deploy calico-kube-controllers --type merge -p \
  '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
   "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"},
   {"key":"CriticalAddonsOnly","operator":"Exists"}]}}}}' >/dev/null

# Storage: CAPD nodes ship no CSI, so PVCs (dev-N CNPG databases + Loki) can
# never bind on a fresh cluster — install local-path-provisioner and mark it the
# default StorageClass (scripts/dev-env.sh fail-fasts on exactly this; w1/043).
# Same OrbStack+Calico caveat as coredns above: the provisioner watches the
# apiserver, so keep it on the control-plane node.
KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f \
  https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.30/deploy/local-path-storage.yaml >/dev/null
# The cluster's default PodSecurity is baseline, which forbids the hostPath
# helper pods local-path uses to mkdir/rm volume dirs — exempt its namespace.
KUBECONFIG="$WL_KUBECONFIG" kubectl label ns local-path-storage \
  pod-security.kubernetes.io/enforce=privileged --overwrite >/dev/null
KUBECONFIG="$WL_KUBECONFIG" kubectl -n local-path-storage patch deploy local-path-provisioner --type merge -p \
  '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
   "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}}}}' >/dev/null
KUBECONFIG="$WL_KUBECONFIG" kubectl annotate storageclass local-path \
  storageclass.kubernetes.io/is-default-class=true --overwrite >/dev/null

# kubelet serving certificates. The control-plane template sets
# `rotate-server-certificates: "true"` (w2/m81 t002), so every kubelet asks the
# apiserver for a `kubernetes.io/kubelet-serving` cert — and kube-controller-manager
# NEVER auto-approves those. In production deploy/gitops/base/kubelet-csr-approver.yaml
# approves them, but Argo CD is not installed on this mock, so without the
# equivalent here every CSR sits Pending and the kubelets keep serving certs the
# apiserver won't trust. That breaks `kubectl port-forward`, `kubectl exec`, and
# `kubectl logs` CLUSTER-WIDE with
#   "error dialing backend: remote error: tls: internal error"
# which reads like a dev-N bug but is a broken cluster (it wedged
# scripts/dev-env.sh at its Hydra bootstrap step). Same chart and same tight
# matching rules the GitOps Application pins, with ONE local deviation:
# bypassDnsResolution=true, because a CAPD node's hostname resolves only through
# the host Docker daemon's DNS, not from inside a pod, so the approver's default
# forward-lookup check would deny every legitimate CSR here. The providerRegex
# and the always-on "SAN IP must be one of the Node's own addresses" check still
# apply. Production keeps the strict default
# (deploy/gitops/overlays/prod/values/kubelet-csr-approver.values.yaml).
KUBECONFIG="$WL_KUBECONFIG" helm upgrade --install kubelet-csr-approver \
  oci://ghcr.io/postfinance/charts/kubelet-csr-approver --version 1.2.7 \
  -n kube-system \
  --set providerRegex='^bex(-[a-z0-9]+)+$' \
  --set allowedDnsNames=1 \
  --set bypassDnsResolution=true \
  --set-string 'nodeSelector.node-role\.kubernetes\.io/control-plane=' \
  --set 'tolerations[0].key=node-role.kubernetes.io/control-plane' \
  --set 'tolerations[0].effect=NoSchedule' >/dev/null
# Certificates requested before the approver was running stay Pending forever —
# approve that startup backlog once so the cluster is usable immediately.
KUBECONFIG="$WL_KUBECONFIG" kubectl get csr -o name 2>/dev/null \
  | xargs -r env KUBECONFIG="$WL_KUBECONFIG" kubectl certificate approve >/dev/null 2>&1 || true

# cert-manager (same version deploy/gitops/base/cert-manager.yaml pins for prod):
# the operator's `make deploy` hard-requires it — config/default mounts the
# cert-manager-issued webhook-server-cert Secret, so without it the manager pod
# wedges in ContainerCreating on a missing mount (w1/043). Control-plane-pinned
# for the same OrbStack+Calico apiserver-reachability reason as coredns above.
KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f \
  https://github.com/cert-manager/cert-manager/releases/download/v1.20.3/cert-manager.yaml >/dev/null
for d in cert-manager cert-manager-cainjector cert-manager-webhook; do
  KUBECONFIG="$WL_KUBECONFIG" kubectl -n cert-manager patch deploy "$d" --type merge -p \
    '{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},
     "tolerations":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}}}}' >/dev/null
done
KUBECONFIG="$WL_KUBECONFIG" kubectl -n cert-manager wait deploy --all --for=condition=Available --timeout=300s >/dev/null || true

# 5. cluster-autoscaler beside CAPI (w1/m3) — same installer as prod CI.
#    Why on the mgmt cluster: infra/clusterapi/autoscaler-values.yaml.
bash scripts/install-autoscaler.sh "$MGMT"

echo
echo "app cluster 'bex' up. kubeconfig: $WL_KUBECONFIG"
echo "  nodes:        KUBECONFIG=$WL_KUBECONFIG kubectl get nodes"
echo "  add machine:  bash scripts/mock-cluster.sh scale 3"
echo "  remove:       bash scripts/mock-cluster.sh scale 1"
