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
MGMT="kind-bex-mgmt"
WL_KUBECONFIG=infra/local/bex.kubeconfig

# --- Single-writer lock (w7/m148 t001) ---------------------------------------
# Bring-up and scale are read-modify-write against ONE shared mgmt cluster:
# `kind create`, `clusterctl init` and the ClusterClass apply all mutate it.
# Two runs interleaving produced half-built clusters that still printed success.
# mkdir is the portable atomic primitive here — flock is not present on macOS —
# and the directory carries its owner pid so a crashed run can be reclaimed
# without ever breaking a live one.
LOCK_DIR=infra/local/.mock-cluster.lock
LOCK_HELD=

release_lock() {
  # Only the process that took the lock may drop it, so an interrupted run
  # cannot release a lock another session now owns.
  [ -n "$LOCK_HELD" ] || return 0
  rm -rf "$LOCK_DIR"
  LOCK_HELD=
}
trap release_lock EXIT INT TERM

# --- Required-stage failures are fatal (w7/m148 t003) ------------------------
# These waits used to end in `|| true`, so a cluster that never became ready
# carried on through CNI, storage and cert-manager and still printed its
# success banner. A required stage that fails now stops the stages that depend
# on it. Recovery is re-running this script: every step is idempotent, and it
# only ever touches the local kind/CAPD cluster.
require() {
  local stage=$1; shift
  if ! "$@"; then
    echo >&2
    echo "error: $stage did not succeed — stopping before the stages that depend on it." >&2
    echo "       Re-run: bash scripts/mock-cluster.sh   (idempotent)" >&2
    echo "       Scope: the local kind/CAPD cluster only. This never touches" >&2
    echo "       production or another workstream's resources." >&2
    exit 1
  fi
}

acquire_lock() {
  mkdir -p infra/local
  if mkdir "$LOCK_DIR" 2>/dev/null; then
    printf '%s\n' "$$" >"$LOCK_DIR/pid"
    LOCK_HELD=1
    return 0
  fi
  local owner
  owner=$(cat "$LOCK_DIR/pid" 2>/dev/null || true)
  if [ -n "$owner" ] && kill -0 "$owner" 2>/dev/null; then
    echo "error: mock-cluster run (pid $owner) already owns $LOCK_DIR" >&2
    echo "       That process is alive. Wait for it rather than racing it —" >&2
    echo "       two runs provisioning one mgmt cluster is how a cluster ends" >&2
    echo "       up half-built while still reporting success." >&2
    exit 1
  fi
  # Stale: the owner is gone. Recovery is removing a directory this script
  # created; it touches no cluster state and no other workstream's resources.
  echo "==> reclaiming stale lock from pid ${owner:-unknown} (no such process)"
  rm -rf "$LOCK_DIR"
  mkdir "$LOCK_DIR" || { echo "error: could not take $LOCK_DIR" >&2; exit 1; }
  printf '%s\n' "$$" >"$LOCK_DIR/pid"
  LOCK_HELD=1
}

# Generated privately, validated, and only then published over the shared path.
# `clusterctl get kubeconfig bex > "$WL_KUBECONFIG"` truncated the shared file
# BEFORE clusterctl produced a byte, so any failure here destroyed the working
# config of every other session pointing at it (w7/m148 t002).
publish_kubeconfig() {
  local tmp lbport
  tmp=$(mktemp "${TMPDIR:-/tmp}/bex-kubeconfig.XXXXXX")
  chmod 600 "$tmp"

  # Every refusal has the same two obligations: drop the half-built file, and
  # say that the shared config was left alone. Saying it in one place is what
  # keeps the promise true if another refusal is added later.
  refuse() {
    rm -f "$tmp"
    echo "error: $1" >&2
    echo "       $WL_KUBECONFIG is unchanged." >&2
  }

  if ! clusterctl get kubeconfig bex >"$tmp" 2>/dev/null; then
    refuse "clusterctl could not produce a kubeconfig for cluster 'bex'"
    return 1
  fi

  # CAPD's internal API IP is not reachable from the host; rewrite to the lb's
  # published port. Portable rewrite — BSD and GNU sed disagree on -i.
  if ! lbport=$(docker port bex-lb 6443/tcp 2>/dev/null | head -1 | sed 's/.*://') \
     || [ -z "$lbport" ]; then
    refuse "container bex-lb publishes no 6443 port — is the cluster up?"
    return 1
  fi
  sed "s#server: https://[0-9.]*:6443#server: https://127.0.0.1:$lbport#" \
    "$tmp" >"$tmp.rewritten" && mv "$tmp.rewritten" "$tmp"

  # Refuse to publish a config that skips TLS verification: a kubeconfig that
  # trusts anything is worse than none, because nothing downstream will notice.
  if grep -q 'insecure-skip-tls-verify: *true' "$tmp"; then
    refuse "generated kubeconfig disables TLS verification; refusing to publish"
    return 1
  fi

  # Prove it reaches the intended cluster before it becomes the shared file.
  # Output is discarded: this file holds a client credential and must never be
  # echoed into a log.
  if ! KUBECONFIG="$tmp" kubectl --request-timeout=30s get --raw /readyz >/dev/null 2>&1; then
    refuse "generated kubeconfig does not reach a ready apiserver at 127.0.0.1:$lbport"
    return 1
  fi

  # Atomic within the same filesystem, so a concurrent reader sees either the
  # old config or the new one — never a partial write.
  mv "$tmp" "$WL_KUBECONFIG"
}

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
# --- Verify before claiming success (w7/m148 t004) ---------------------------
# The banner below used to print unconditionally, so a cluster missing its CNI,
# storage class or metrics still read as "up" — and the next script to fail got
# the blame. Each check covers a component THIS script installs. Nothing here
# waits: the waits already happened above, so this is a read of the end state.
verify_substrate() {
  local failures=0
  # Run a command against the workload cluster without exporting KUBECONFIG,
  # which would break the mgmt-context check below.
  wl() { KUBECONFIG="$WL_KUBECONFIG" "$@"; }

  check() {
    local what=$1; shift
    if "$@" >/dev/null 2>&1; then
      printf '  ok    %s\n' "$what"
    else
      printf '  FAIL  %s\n' "$what"
      failures=$((failures + 1))
    fi
  }

  # Every node Ready — and at least one node. `! kubectl get nodes | grep -qv
  # Ready` alone passes when kubectl fails and prints nothing, which reported a
  # dead cluster as healthy; an empty node list is not "all ready".
  nodes_ready() {
    local out
    out=$(wl kubectl --request-timeout=20s get nodes --no-headers 2>/dev/null) || return 1
    [ -n "$out" ] || return 1
    ! printf '%s\n' "$out" | grep -qv ' Ready'
  }

  default_storageclass() {
    wl kubectl get storageclass -o jsonpath='{.items[*].metadata.annotations.storageclass\.kubernetes\.io/is-default-class}' 2>/dev/null \
      | grep -q true
  }

  echo
  echo "verifying the substrate this script installed:"
  check "apiserver reachable (TLS verified)" wl kubectl --request-timeout=20s get --raw /readyz
  check "every node Ready (and at least one)" nodes_ready
  check "CNI (calico-node) rolled out" wl kubectl -n kube-system rollout status ds/calico-node --timeout=15s
  check "a default StorageClass exists" default_storageclass
  check "cert-manager Available" wl kubectl -n cert-manager wait deploy --all --for=condition=Available --timeout=15s
  check "metrics API serving (metrics.k8s.io)" wl kubectl get --raw /apis/metrics.k8s.io/v1beta1
  check "cluster-autoscaler Available (mgmt)" kubectl --context "$MGMT" -n kube-system wait deploy --all --for=condition=Available --timeout=15s

  if [ "$failures" -ne 0 ]; then
    echo >&2
    echo "error: $failures required check(s) failed — NOT reporting the cluster as up." >&2
    echo "       Re-run: bash scripts/mock-cluster.sh   (idempotent)" >&2
    return 1
  fi
}

# Definitions end here. scripts/mock-cluster.test.sh sources this file to
# exercise the lock and publication contracts without provisioning anything.
if [ -n "${BEX_MOCK_CLUSTER_LIB:-}" ]; then return 0 2>/dev/null || exit 0; fi

if [ "${1:-}" = scale ]; then acquire_lock; scale "${2:?usage: scale N}"; exit 0; fi

acquire_lock

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
for _ in $(seq 1 60); do
  [ "$(kubectl --context "$MGMT" get machines --no-headers 2>/dev/null | grep -c Running)" -ge 2 ] && break; sleep 8
done
# Required: clusterctl cannot produce a usable kubeconfig for a cluster whose
# machines never came up, and every stage below depends on that kubeconfig.
machines_running=$(kubectl --context "$MGMT" get machines --no-headers 2>/dev/null | grep -c Running || true)
if [ "${machines_running:-0}" -lt 2 ]; then
  echo >&2
  echo "error: only ${machines_running:-0} CAPI machine(s) reached Running within 8m (need 2)." >&2
  echo "       Inspect: kubectl --context $MGMT get machines" >&2
  echo "       Re-run: bash scripts/mock-cluster.sh   (idempotent)" >&2
  exit 1
fi

# 4. app-cluster kubeconfig — rewrite the server to the lb's host-published port
#    (CAPD's internal API IP isn't reachable from the host), then install a CNI.
publish_kubeconfig
KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f \
  https://raw.githubusercontent.com/projectcalico/calico/v3.28.2/manifests/calico.yaml >/dev/null
# Required, and deliberately ordered after the Calico apply above: nodes cannot
# go Ready until a CNI is installed, so this must not move earlier (t003).
require "node readiness (post-CNI)" \
  env KUBECONFIG="$WL_KUBECONFIG" kubectl wait --for=condition=Ready node --all --timeout=300s
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
# Genuinely best-effort, unlike the waits above: there may be no backlog at all,
# and the approver installed above handles everything from here on.
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
require "cert-manager availability" \
  env KUBECONFIG="$WL_KUBECONFIG" kubectl -n cert-manager wait deploy --all \
  --for=condition=Available --timeout=300s

# metrics-server — resource metrics (metrics.k8s.io) for bex-api's CPU/memory
# fallback and `kubectl top` (w5/057). Without it, Metrics-page walks on every
# `dev-N` stack see empty CPU/memory regardless of replica count. Installed
# after kubelet-csr-approver so kubelets present approved serving certs; same
# --kubelet-certificate-authority path as deploy/gitops/base/metrics-server.yaml
# (no --kubelet-insecure-tls). Pin to the control-plane node for the same
# OrbStack+Calico apiserver-reachability reason as coredns/cert-manager above;
# the platform-pool nodeSelector from the GitOps Application is a prod concern.
# The address-types commas are BACKSLASH-ESCAPED: helm treats a comma in
# --set as a list separator, so the bare form parsed as three keys and helm
# refused with `key "Hostname" has no value`, aborting every bring-up here.
# t003 is what surfaced it — before, the run carried on to its success
# banner regardless of what failed.
KUBECONFIG="$WL_KUBECONFIG" helm upgrade --install metrics-server \
  metrics-server --repo https://kubernetes-sigs.github.io/metrics-server/ \
  --version 3.12.2 \
  -n kube-system \
  --set 'args[0]=--kubelet-preferred-address-types=InternalIP\,Hostname\,ExternalIP' \
  --set 'args[1]=--kubelet-certificate-authority=/etc/kubernetes/pki/kubelet-ca/ca.crt' \
  --set 'extraVolumes[0].name=kubelet-ca' \
  --set 'extraVolumes[0].configMap.name=kube-root-ca.crt' \
  --set 'extraVolumeMounts[0].name=kubelet-ca' \
  --set 'extraVolumeMounts[0].mountPath=/etc/kubernetes/pki/kubelet-ca' \
  --set 'extraVolumeMounts[0].readOnly=true' \
  --set 'metrics.enabled=false' \
  --set-string 'nodeSelector.node-role\.kubernetes\.io/control-plane=' \
  --set 'tolerations[0].key=node-role.kubernetes.io/control-plane' \
  --set 'tolerations[0].effect=NoSchedule' >/dev/null
require "metrics-server availability" \
  env KUBECONFIG="$WL_KUBECONFIG" kubectl -n kube-system wait deploy/metrics-server \
  --for=condition=Available --timeout=180s

# 5. cluster-autoscaler beside CAPI (w1/m3) — same installer as prod CI.
#    Why on the mgmt cluster: infra/clusterapi/autoscaler-values.yaml.
bash scripts/install-autoscaler.sh "$MGMT"

verify_substrate

echo
echo "app cluster 'bex' up. kubeconfig: $WL_KUBECONFIG"
# Scope, stated plainly: this script publishes a substrate. It does NOT install
# the bex operator or the App CRD — that stays `make deploy` from lego/operator.
echo "  note:         substrate only — the bex operator and App CRD are a"
echo "                separate step (make -C lego/operator deploy)"
echo "  nodes:        KUBECONFIG=$WL_KUBECONFIG kubectl get nodes"
echo "  add machine:  bash scripts/mock-cluster.sh scale 3"
echo "  remove:       bash scripts/mock-cluster.sh scale 1"
