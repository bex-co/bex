#!/usr/bin/env bash
# Stand up the local CAPD mock of the Hetzner substrate, entirely in Docker:
#   kind infra cluster -> Cluster API + Docker provider (CAPD) -> an app
#   cluster whose "machines" are Docker-container nodes. Add/remove a machine by
#   scaling the worker pool. Swap CAPD -> CAPH for Hetzner; bex is unchanged.
#
#   bash scripts/mock-cluster.sh            # bring it up (substrate + Traefik edge + Zot)
#   bash scripts/mock-cluster.sh operator   # build bex-operator:dev from this tree and deploy it
#   bash scripts/mock-cluster.sh scale 3    # set worker machines = 3 (add/remove)
set -euo pipefail
cd "$(dirname "$0")/.."
export CLUSTER_TOPOLOGY=true                 # CAPD's flavor uses ClusterClass/topology
MGMT="kind-bex-mgmt"
WL_KUBECONFIG=infra/local/bex.kubeconfig
# --- No global kube context (w2/m289 t003) -----------------------------------
# Every mgmt-cluster call names its kubeconfig AND its context. This script
# used to `kubectl config use-context` and then run context-less kubectl and
# clusterctl against ~/.kube/config, so another session clearing or switching
# the global current-context broke `clusterctl get kubeconfig` mid-bring-up
# (w2, 2026-11-07). kind writes the mgmt context into this private file, never
# into ~/.kube/config, and nothing here reads or changes the global one.
MGMT_KUBECONFIG=infra/local/bex-mgmt.kubeconfig
mgmt_kubectl() { kubectl --kubeconfig "$MGMT_KUBECONFIG" --context "$MGMT" "$@"; }
mgmt_clusterctl() { clusterctl --kubeconfig "$MGMT_KUBECONFIG" --kubeconfig-context "$MGMT" "$@"; }
# The workload kubeconfig names its own context (clusterctl writes
# current-context into it), so KUBECONFIG=$WL_KUBECONFIG never falls back to
# the global file either.
wl_kubectl() { KUBECONFIG="$WL_KUBECONFIG" kubectl "$@"; }

# helm with isolated registry credentials (w2/m289 t004): a locked macOS
# keychain must not hang an OCI chart pull. See scripts/lib/helm-isolated.sh.
# shellcheck source=lib/helm-isolated.sh
. scripts/lib/helm-isolated.sh

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
# INT/TERM must END the run, not just drop the lock: a bare `trap release_lock
# TERM` ran the handler and then resumed the script, which carried on
# provisioning with the lock already released (w2/m289, seen live). Exiting
# runs the EXIT trap, which releases it once.
trap release_lock EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

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

  if ! mgmt_clusterctl get kubeconfig bex >"$tmp" 2>/dev/null; then
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
  mgmt_kubectl patch cluster bex --type merge \
    -p "{\"spec\":{\"topology\":{\"workers\":{\"machineDeployments\":[{\"name\":\"worker-0\",\"class\":\"default-worker\",\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size\":\"1\",\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size\":\"1\"}}},{\"name\":\"tenant-0\",\"class\":\"tenant-worker\",\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size\":\"$n\",\"cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size\":\"$max\"}}}]}}}}"
  echo "tenant worker floor -> $n machine(s) (min-size $n / max-size $max; platform stays at 1)"
  echo "watch: docker ps --format '{{.Names}}' | grep bex-tenant-0"
}
# Every node Ready — and at least one node. `! kubectl get nodes | grep -qv
# Ready` alone passes when kubectl fails and prints nothing, which reported a
# dead cluster as healthy; an empty node list is not "all ready".
nodes_ready() {
  local out
  out=$(wl_kubectl --request-timeout=20s get nodes --no-headers 2>/dev/null) || return 1
  [ -n "$out" ] || return 1
  ! printf '%s\n' "$out" | grep -qv ' Ready'
}

# wait_nodes_ready SECONDS polls nodes_ready rather than `kubectl wait node
# --all`: under load CAPI's MachineHealthCheck replaces a slow tenant machine,
# and `wait --all` then fails on the Node that vanished ("nodes ... not
# found") although its replacement goes Ready moments later (w2/m289, seen on
# a fresh bring-up).
wait_nodes_ready() {
  local deadline=$((SECONDS + $1))
  until nodes_ready; do
    [ "$SECONDS" -lt "$deadline" ] || return 1
    sleep "${BEX_NODE_POLL_SECONDS:-5}"
  done
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
  check "build boundary (bex-build RBAC for the operator)" wl kubectl -n bex-build get rolebinding bex-build-credentials
  check "Traefik CRDs established (traefik.io)" traefik_crds_established
  check "Traefik edge Available (traefik)" traefik_available
  check "CNPG operator installed (postgresql.cnpg.io)" cnpg_installed
  check "Zot registry on every node (401 anonymous, 200 bex-builder)" zot_serving
  check "cluster-autoscaler Available (mgmt)" mgmt_kubectl -n kube-system wait deploy --all --for=condition=Available --timeout=15s

  if [ "$failures" -ne 0 ]; then
    echo >&2
    echo "error: $failures required check(s) failed — NOT reporting the cluster as up." >&2
    echo "       Re-run: bash scripts/mock-cluster.sh   (idempotent)" >&2
    return 1
  fi
}

# argo_manages APP: the local Argo app-of-apps (deploy/gitops/overlays/local)
# runs on this cluster and owns APP. A bootstrap Helm install over an
# Argo-owned release fails on field ownership and aborts the bring-up (w1/127),
# so each component the overlay also ships defers to Argo when it is there.
argo_manages() {
  KUBECONFIG="$WL_KUBECONFIG" kubectl -n argocd get application "$1" >/dev/null 2>&1 || return 1
  echo "$1 is Argo-managed (overlays/local); skipping its bootstrap Helm install"
}

# install_kubelet_csr_approver installs the bare-mock kubelet-serving CSR
# approver unless Argo owns it (overlays/local carries the same CAPD values,
# guarded by scripts/gitops-validate.sh). Same chart and tight matching rules
# as the GitOps Application, with one deviation: bypassDnsResolution=true,
# because a CAPD node's hostname resolves only through the host Docker
# daemon's DNS, not from inside a pod, so the default forward-lookup check
# would deny every legitimate CSR. providerRegex and the always-on "SAN IP must
# be one of the Node's own addresses" check still apply. Control-plane
# placement for the same OrbStack+Calico apiserver-reachability reason as
# coredns.
install_kubelet_csr_approver() {
  if argo_manages kubelet-csr-approver; then
    return 0
  fi
  KUBECONFIG="$WL_KUBECONFIG" helm upgrade --install kubelet-csr-approver \
    oci://ghcr.io/postfinance/charts/kubelet-csr-approver --version 1.2.7 \
    -n kube-system \
    --set providerRegex='^bex(-[a-z0-9]+)+$' \
    --set allowedDnsNames=1 \
    --set bypassDnsResolution=true \
    --set-string 'nodeSelector.node-role\.kubernetes\.io/control-plane=' \
    --set 'tolerations[0].key=node-role.kubernetes.io/control-plane' \
    --set 'tolerations[0].effect=NoSchedule' >/dev/null
}

# install_metrics_server installs the bare-mock metrics-server unless Argo
# owns it (its overlays/local Application keeps base placement on a worker,
# which serves `kubectl top` on this mock as verified 2026-10-15). Same
# --kubelet-certificate-authority path as deploy/gitops/base/metrics-server.yaml
# (no --kubelet-insecure-tls), pinned to the control-plane node like coredns.
# The address-types commas are BACKSLASH-ESCAPED: helm treats a comma in --set
# as a list separator, so the bare form parsed as three keys and helm refused
# with `key "Hostname" has no value`, aborting every bring-up.
install_metrics_server() {
  if argo_manages metrics-server; then
    return 0
  fi
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
}

# Definitions end here. scripts/mock-cluster.test.sh sources this file to
# exercise the lock and publication contracts without provisioning anything.
# apply_build_boundary applies deploy/gitops/base/build-boundary.yaml (the
# bex-build namespace and RBAC) idempotently, minus its Cilium policies: this
# cluster runs Calico and has no cilium.io CRDs (the local GitOps overlay deletes
# every Cilium object for the same reason), so applying them failed every fresh
# bring-up; a cluster already carrying dev-env's inert CRD shim only hid that.
apply_build_boundary() {
  yq 'select(.kind != "CiliumNetworkPolicy" and .kind != "CiliumClusterwideNetworkPolicy")' \
    deploy/gitops/base/build-boundary.yaml \
    | KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f -
}

# refresh_mgmt_kubeconfig writes the kind mgmt cluster's kubeconfig to the
# private $MGMT_KUBECONFIG (t003). `kind get kubeconfig` reads the cluster
# itself, so this works whatever ~/.kube/config holds, and it re-reads the
# apiserver port Docker may have changed on a restart. Written privately and
# moved into place, like publish_kubeconfig: it is a client credential.
refresh_mgmt_kubeconfig() {
  local tmp
  mkdir -p "$(dirname "$MGMT_KUBECONFIG")"
  tmp=$(mktemp "${TMPDIR:-/tmp}/bex-mgmt-kubeconfig.XXXXXX")
  chmod 600 "$tmp"
  if ! kind get kubeconfig --name bex-mgmt >"$tmp" 2>/dev/null || [ ! -s "$tmp" ]; then
    rm -f "$tmp"
    echo "error: kind could not produce a kubeconfig for cluster 'bex-mgmt'" >&2
    return 1
  fi
  mv "$tmp" "$MGMT_KUBECONFIG"
}

# --- Traefik edge (w2/m289 t001) ---------------------------------------------
# The operator writes traefik.io objects for every web service (Middleware,
# IngressRoute) and `make deploy` ships bex-system IngressRoutes, so a mock
# without Traefik's CRDs failed both: web services stuck on `no matches for
# kind "Middleware"`, and the operator deploy aborted part-way. Production
# installs Traefik — CRDs included — from the chart deploy/helm-artifacts.lock
# pins (deploy/gitops/base/traefik.yaml mirrors that exact archive), so this
# installs the same archive, verified against the same sha256, with the same
# base values the local GitOps overlay uses. That gives the mock a real edge
# too: dev-N web services answer through Traefik exactly as in production.
TRAEFIK_VALUES=deploy/gitops/base/values/traefik.values.yaml
TRAEFIK_CRDS="ingressroutes.traefik.io ingressroutetcps.traefik.io middlewares.traefik.io tlsstores.traefik.io"

# helm_pin CHART prints "VERSION REPO SHA256" for CHART from
# deploy/helm-artifacts.lock — the one place production's chart pins live.
helm_pin() {
  awk -F'|' -v chart="$1" '$1 == chart { print $2, $3, $4; exit }' deploy/helm-artifacts.lock
}

sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }

# fetch_pinned_chart CHART DIR pulls CHART's locked archive into DIR, refuses
# it unless its sha256 matches the lock, and prints the archive's path.
fetch_pinned_chart() {
  local chart=$1 dir=$2 version repo sha got
  read -r version repo sha <<<"$(helm_pin "$chart")" || true
  if [ -z "${sha:-}" ]; then
    echo "error: deploy/helm-artifacts.lock has no $chart|VERSION|REPO|SHA256 line" >&2
    return 1
  fi
  if ! helm pull "$chart" --repo "$repo" --version "$version" --destination "$dir" >/dev/null; then
    echo "error: could not fetch the $chart $version chart from $repo" >&2
    return 1
  fi
  got=$(sha256_of "$dir/$chart-$version.tgz")
  if [ "$got" != "$sha" ]; then
    echo "error: $chart $version archive sha256 $got does not match deploy/helm-artifacts.lock ($sha)" >&2
    return 1
  fi
  printf '%s\n' "$dir/$chart-$version.tgz"
}

traefik_crds_established() {
  local crd
  for crd in $TRAEFIK_CRDS; do
    wl_kubectl wait --for=condition=established "crd/$crd" --timeout=60s >/dev/null || return 1
  done
}

traefik_available() {
  wl_kubectl -n traefik wait deploy --all --for=condition=Available --timeout=15s
}

install_traefik() {
  if argo_manages traefik; then
    return 0
  fi
  local dir chart
  dir=$(mktemp -d "${TMPDIR:-/tmp}/bex-traefik-chart.XXXXXX")
  if ! chart=$(fetch_pinned_chart traefik "$dir"); then
    rm -rf "$dir"
    return 1
  fi
  # hostNetwork + NET_BIND_SERVICE is outside this cluster's default
  # PodSecurity baseline, so the edge namespace is exempted like
  # local-path-storage. The plugin ConfigMaps must exist first: the values
  # mount them by their content-hashed names.
  if ! { wl_kubectl create namespace traefik --dry-run=client -o yaml | wl_kubectl apply -f - >/dev/null &&
    wl_kubectl label namespace traefik pod-security.kubernetes.io/enforce=privileged --overwrite >/dev/null &&
    wl_kubectl apply -k deploy/gitops/charts/traefik-plugins >/dev/null &&
    KUBECONFIG="$WL_KUBECONFIG" helm upgrade --install traefik "$chart" -n traefik \
      -f "$TRAEFIK_VALUES" --wait --timeout 5m >/dev/null; }; then
    rm -rf "$dir"
    echo "error: installing traefik (namespace, plugins or helm release) failed" >&2
    return 1
  fi
  rm -rf "$dir"
  traefik_crds_established
}

# --- CNPG operator (w2/m289 t002) --------------------------------------------
# The bex manager watches postgresql.cnpg.io Clusters and exits when that
# cache cannot sync, so on a fresh mock the operator crashlooped until some
# dev-N `up` happened to install CNPG. The shared cluster now carries it
# from the start, from production's lock pin, under the release name
# dev-env.sh's ensure_cnpg looks for (`cnpg`), which then only pins and waits.
cnpg_installed() {
  wl_kubectl get crd clusters.postgresql.cnpg.io >/dev/null 2>&1 &&
    [ -n "$(wl_kubectl -n cnpg-system get deploy -l app.kubernetes.io/name=cloudnative-pg -o name 2>/dev/null)" ]
}

install_cnpg() {
  if argo_manages cnpg-operator || cnpg_installed; then
    return 0
  fi
  local dir chart
  dir=$(mktemp -d "${TMPDIR:-/tmp}/bex-cnpg-chart.XXXXXX")
  if ! chart=$(fetch_pinned_chart cloudnative-pg "$dir"); then
    rm -rf "$dir"
    return 1
  fi
  if ! KUBECONFIG="$WL_KUBECONFIG" helm upgrade --install cnpg "$chart" -n cnpg-system --create-namespace \
    --wait --timeout 5m >/dev/null; then
    rm -rf "$dir"
    echo "error: installing the CNPG operator (release cnpg, namespace cnpg-system) failed" >&2
    return 1
  fi
  rm -rf "$dir"
  wl_kubectl wait --for=condition=established crd/clusters.postgresql.cnpg.io --timeout=60s >/dev/null
}

# --- Platform registry: Zot (w2/m293 t001/t002) ------------------------------
# The operator runs with BEX_REGISTRY=zot.bex-registry.svc:5000 and
# BEX_REGISTRY_NS=bex-registry (config/manager), so on a mock with no Zot every
# repo-backed App failed at create (PerAppRegistryCredsFailed) and every App
# deletion hung restoring its per-App credential. Production installs Zot from
# the lock's archive with deploy/gitops/base/zot.yaml's values; this installs
# that same archive (sha-checked) with those same values, so the Service is
# `zot` on :5000 (plain HTTP) plus NodePort 30500, which the local-capd
# overlay's containerd hosts.toml already points zot.bex-registry.svc:5000 and
# zot.local:5000 at on every node.
#
# The credentials are production's shape from scripts/registry-secrets.sh:
# zot-htpasswd with bex-builder, and bex-builder's docker configs in bex-build
# (bex-registry-push, -push-kpack, bex-registry-pull) and bex-system
# (bex-registry-push-kpack). There is no bex-puller: production retired it for
# the per-App app-* users the operator mints. The password is random per
# cluster, lives only in those Secrets and is never printed; a re-run keeps it
# while the htpasswd still verifies it, so it does not churn.
ZOT_NS=bex-registry
ZOT_REGISTRY=zot.bex-registry.svc:5000
ZOT_BUILD_NS=bex-build
ZOT_APP_SOURCE=deploy/gitops/base/zot.yaml
REGISTRY_SECRETS_SCRIPT=scripts/registry-secrets.sh
HTPASSWD_BIN="$(command -v htpasswd || echo /usr/sbin/htpasswd)"

# zot_values prints the Helm values production's Argo Application carries.
zot_values() { yq '.spec.source.helm.values' "$ZOT_APP_SOURCE"; }

# zot_seed_config prints the operator's canonical base config
# (lego/operator/internal/registry/zotconfig.go baseZotConfig): anonymous
# denied, bex-builder holding the global adminPolicy and the ** grant, and the
# shared kpack builder repository read-only for every authenticated user. The
# chart mounts zot-config (mountConfig: false), so Zot cannot start until it
# exists; in production the operator bootstraps it on its first App. Seeded
# only when absent: from then on the operator owns it (per-App ACL entries,
# storage migrations), exactly as in production.
zot_seed_config() {
  jq -cn '
    {users: ["bex-builder"], actions: ["read", "create", "update", "delete"]} as $builder
    | {distSpecVersion: "1.1.0",
       storage: {rootDirectory: "/var/lib/registry", dedupe: true, gc: true,
                 gcDelay: "3h", gcInterval: "3h",
                 retention: {dryRun: false, policies: [{repositories: ["**"], deleteUntagged: true,
                   keepTags: [{patterns: [".*"], mostRecentlyPushedCount: 5},
                              {patterns: ["^live-.*"], mostRecentlyPushedCount: 10}]}]}},
       http: {address: "0.0.0.0", port: "5000", compat: ["docker2s2"],
              readTimeout: "60s", writeTimeout: "60s",
              auth: {htpasswd: {path: "/secret/htpasswd"}},
              accessControl: {repositories: {"**": {policies: [$builder]},
                                             "bex-cnb-builder": {policies: [], defaultPolicy: ["read"]}},
                              adminPolicy: $builder}},
       log: {level: "info"},
       extensions: {scrub: {enable: true, interval: "24h"}}}'
}

# registry_builder_password prints bex-builder's password from the build-ns
# push Secret, or nothing. Callers keep it in a variable; it is never echoed.
registry_builder_password() {
  wl_kubectl -n "$ZOT_BUILD_NS" get secret bex-registry-push -o json 2>/dev/null |
    jq -r --arg r "$ZOT_REGISTRY" '.data["config.json"] // empty | @base64d | fromjson | .auths[$r].password // empty' 2>/dev/null
}

# registry_htpasswd_verifies PASSWORD — zot-htpasswd's bex-builder hash matches.
# The password reaches htpasswd on stdin, never argv.
registry_htpasswd_verifies() {
  local file rc=1
  file=$(mktemp "${TMPDIR:-/tmp}/bex-zot-htpasswd.XXXXXX")
  chmod 600 "$file"
  wl_kubectl -n "$ZOT_NS" get secret zot-htpasswd -o json 2>/dev/null |
    jq -r '.data.htpasswd // empty | @base64d' 2>/dev/null | grep '^bex-builder:' >"$file" || true
  if [ -s "$file" ] && printf '%s\n' "$1" | "$HTPASSWD_BIN" -v -i "$file" bex-builder >/dev/null 2>&1; then
    rc=0
  fi
  rm -f "$file"
  return "$rc"
}

# registry_secrets_present — every Secret registry-secrets.sh creates exists.
registry_secrets_present() {
  wl_kubectl -n "$ZOT_BUILD_NS" get secret bex-registry-push bex-registry-push-kpack bex-registry-pull >/dev/null 2>&1 &&
    wl_kubectl -n bex-system get secret bex-registry-push-kpack >/dev/null 2>&1
}

# ensure_registry_credentials keeps the existing dev password when it still
# verifies against zot-htpasswd and every Secret is there; otherwise it mints
# a new one and (re)applies the whole set with scripts/registry-secrets.sh,
# which preserves the operator's app-* users. Sets REGISTRY_CREDS_ROTATED=1
# when it wrote, because a running Zot reads htpasswd only at start.
REGISTRY_CREDS_ROTATED=
ensure_registry_credentials() {
  local pass
  REGISTRY_CREDS_ROTATED=
  pass=$(registry_builder_password)
  if [ -n "$pass" ] && registry_htpasswd_verifies "$pass" && registry_secrets_present; then
    echo "    registry credentials present (bex-builder verifies against zot-htpasswd)"
    return 0
  fi
  pass=$(openssl rand -hex 16) || return 1
  if ! env KUBECONFIG="$WL_KUBECONFIG" BEX_REGISTRY_SECRETS_NO_ENV_FILE=1 \
    BEX_REGISTRY_BUILDER_PASSWORD="$pass" REGISTRY_NS="$ZOT_NS" BEX_REGISTRY="$ZOT_REGISTRY" \
    BEX_KPACK_REGISTRY=zot.local:5000 BEX_BUILD_NAMESPACE="$ZOT_BUILD_NS" BEX_KPACK_NAMESPACE=bex-system \
    bash "$REGISTRY_SECRETS_SCRIPT" >/dev/null; then
    echo "error: $REGISTRY_SECRETS_SCRIPT could not apply the dev registry credentials" >&2
    return 1
  fi
  REGISTRY_CREDS_ROTATED=1
  echo "    registry credentials minted (new random dev password for bex-builder; not printed)"
}

# zot_restart_if_rotated bounces a running Zot onto credentials just rewritten.
zot_restart_if_rotated() {
  [ -n "$REGISTRY_CREDS_ROTATED" ] || return 0
  wl_kubectl -n "$ZOT_NS" get statefulset zot >/dev/null 2>&1 || return 0
  wl_kubectl -n "$ZOT_NS" rollout restart statefulset/zot >/dev/null &&
    wl_kubectl -n "$ZOT_NS" rollout status statefulset/zot --timeout=300s >/dev/null
}

# zot_node_probe NODE [--auth] prints the HTTP status Zot answers on NODE's
# 127.0.0.1:30500 — the exact path containerd's hosts.toml sends kubelet pulls
# down. With --auth, bex-builder's credential rides curl's stdin config.
zot_node_probe() {
  local node=$1 pass=
  if [ "${2:-}" = --auth ]; then
    pass=$(registry_builder_password)
    [ -n "$pass" ] || { echo 000; return 0; }
  fi
  if [ -n "$pass" ]; then
    printf 'user = "bex-builder:%s"\n' "$pass" |
      docker exec -i "$node" curl -s -o /dev/null -w '%{http_code}' -K - http://127.0.0.1:30500/v2/ 2>/dev/null || echo 000
  else
    docker exec "$node" curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:30500/v2/ 2>/dev/null || echo 000
  fi
}

wl_node_names() { wl_kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'; }

# zot_serving: on every node, anonymous /v2/ is refused (401: auth is on) and
# bex-builder is accepted (200).
zot_serving() {
  local node seen=0
  while IFS= read -r node; do
    [ -n "$node" ] || continue
    seen=1
    [ "$(zot_node_probe "$node")" = 401 ] || return 1
    [ "$(zot_node_probe "$node" --auth)" = 200 ] || return 1
  done < <(wl_node_names)
  [ "$seen" -eq 1 ]
}

install_zot() {
  if argo_manages zot; then
    return 0
  fi
  local dir chart values
  wl_kubectl create namespace "$ZOT_NS" --dry-run=client -o yaml | wl_kubectl apply -f - >/dev/null || return 1
  ensure_registry_credentials || return 1
  if ! wl_kubectl -n "$ZOT_NS" get secret zot-config >/dev/null 2>&1; then
    # A labelled Opaque Secret like the operator's own bootstrap (secretwrite.go).
    zot_seed_config | wl_kubectl -n "$ZOT_NS" create secret generic zot-config \
      --from-file=config.json=/dev/stdin --dry-run=client -o json |
      jq '.metadata.labels["app.bex.co/managed-by"] = "bex-operator"' |
      wl_kubectl apply -f - >/dev/null || return 1
  fi
  # A first install starts Zot on the credentials just written: no bounce.
  wl_kubectl -n "$ZOT_NS" get statefulset zot >/dev/null 2>&1 || REGISTRY_CREDS_ROTATED=
  dir=$(mktemp -d "${TMPDIR:-/tmp}/bex-zot-chart.XXXXXX")
  values="$dir/values.yaml"
  if ! chart=$(fetch_pinned_chart zot "$dir") || ! zot_values >"$values"; then
    rm -rf "$dir"
    return 1
  fi
  if ! KUBECONFIG="$WL_KUBECONFIG" helm upgrade --install zot "$chart" -n "$ZOT_NS" \
    -f "$values" --wait --timeout 5m >/dev/null; then
    rm -rf "$dir"
    echo "error: installing Zot (release zot, namespace $ZOT_NS) failed" >&2
    return 1
  fi
  rm -rf "$dir"
  zot_restart_if_rotated || return 1
  zot_serving || {
    echo "error: Zot is installed but a node's 127.0.0.1:30500 does not answer 401 anonymous / 200 bex-builder" >&2
    return 1
  }
}

# --- `operator`: the bex operator from this tree (w2/m289 t002) --------------
# After every reprovision the operator used to be rebuilt, side-loaded onto
# each node, deployed and patched by hand. This is that runbook as one
# command. CAPD nodes cannot pull a local tag, so the image is imported into
# every node's containerd, and every bex-system workload running it is set to
# IfNotPresent (config/ ships Always, which would try docker.io and fail).
OPERATOR_IMG="${BEX_OPERATOR_IMG:-bex-operator:dev}"

# operator_step NAME HINT CMD... — run one step; on failure name the step,
# say what to check, and stop: every later step depends on the earlier ones.
operator_step() {
  local stage=$1 hint=$2; shift 2
  echo "==> $stage"
  if ! "$@"; then
    echo >&2
    echo "error: operator step '$stage' failed." >&2
    echo "       $hint" >&2
    echo "       Re-run: bash scripts/mock-cluster.sh operator   (idempotent)" >&2
    exit 1
  fi
}

abs_path() { case $1 in /*) printf '%s\n' "$1" ;; *) printf '%s/%s\n' "$PWD" "$1" ;; esac; }

operator_prereqs() {
  traefik_crds_established &&
    wl_kubectl -n "$ZOT_NS" get statefulset zot >/dev/null &&
    wl_kubectl wait --for=condition=established crd/clusters.postgresql.cnpg.io --timeout=30s >/dev/null &&
    wl_kubectl -n cert-manager wait deploy --all --for=condition=Available --timeout=30s >/dev/null
}

operator_registry_credentials() {
  ensure_registry_credentials && zot_restart_if_rotated && zot_serving
}

operator_build() { docker build -t "$OPERATOR_IMG" -f lego/Dockerfile lego; }

# operator_import_image loads the image into containerd on every node the
# workload cluster reports. CAPD names each node container after its Node,
# so the Node list is the authoritative set — including scaled tenant nodes.
operator_import_image() {
  local tar node imported=0
  tar=$(mktemp "${TMPDIR:-/tmp}/bex-operator-image.XXXXXX")
  if ! docker save "$OPERATOR_IMG" -o "$tar"; then
    rm -f "$tar"
    echo "error: docker save $OPERATOR_IMG failed" >&2
    return 1
  fi
  while IFS= read -r node; do
    [ -n "$node" ] || continue
    if ! docker exec -i "$node" ctr -n k8s.io images import - <"$tar" >/dev/null; then
      rm -f "$tar"
      echo "error: importing $OPERATOR_IMG into node container '$node' failed" >&2
      return 1
    fi
    echo "    $OPERATOR_IMG -> $node"
    imported=$((imported + 1))
  done < <(wl_kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
  rm -f "$tar"
  if [ "$imported" -eq 0 ]; then
    echo "error: the workload cluster reported no nodes to import onto" >&2
    return 1
  fi
}

operator_make_deploy() {
  env KUBECONFIG="$(abs_path "$WL_KUBECONFIG")" GOWORK=off \
    make -C lego/operator deploy IMG="$OPERATOR_IMG"
}

# operator_workloads prints kind/name for each bex-system Deployment or
# DaemonSet with a container (or init container) running $OPERATOR_IMG.
operator_workloads() {
  wl_kubectl -n bex-system get deployments,daemonsets -o json |
    jq -r --arg img "$OPERATOR_IMG" '.items[]
      | select([.spec.template.spec.containers[]?, .spec.template.spec.initContainers[]?] | any(.image == $img))
      | "\(.kind | ascii_downcase)/\(.metadata.name)"'
}

# operator_pull_policy_patch reads one workload's JSON and prints the JSON
# patch that sets IfNotPresent on each of its $OPERATOR_IMG containers, or
# nothing when every one already has it.
operator_pull_policy_patch() {
  jq -c --arg img "$OPERATOR_IMG" '
    [ ("containers", "initContainers") as $field
      | (.spec.template.spec[$field] // []) | to_entries[]
      | select(.value.image == $img and .value.imagePullPolicy != "IfNotPresent")
      | {op: "add", path: "/spec/template/spec/\($field)/\(.key)/imagePullPolicy", value: "IfNotPresent"} ]
    | select(length > 0)'
}

# operator_roll pins IfNotPresent, then restarts every workload on the image:
# the tag is unchanged, so without a restart the pods keep the previous build.
operator_roll() {
  local w patch found=0
  while IFS= read -r w; do
    [ -n "$w" ] || continue
    found=1
    patch=$(wl_kubectl -n bex-system get "$w" -o json | operator_pull_policy_patch) || return 1
    if [ -n "$patch" ]; then
      wl_kubectl -n bex-system patch "$w" --type json -p "$patch" >/dev/null || return 1
    fi
    wl_kubectl -n bex-system rollout restart "$w" >/dev/null || return 1
    echo "    $w: imagePullPolicy IfNotPresent, restarted"
  done < <(operator_workloads)
  if [ "$found" -eq 0 ]; then
    echo "error: no bex-system workload runs $OPERATOR_IMG — did make deploy apply?" >&2
    return 1
  fi
}

operator_wait() {
  wl_kubectl -n bex-system rollout status deploy/bex-controller-manager --timeout=300s &&
    wl_kubectl wait --for=condition=established crd/apps.app.bex.co --timeout=60s >/dev/null
}

deploy_operator() {
  operator_step "workload cluster reachable" \
    "Bring the cluster up first: bash scripts/mock-cluster.sh" \
    wl_kubectl --request-timeout=20s get --raw /readyz
  operator_step "substrate prerequisites (Traefik CRDs, Zot, CNPG CRDs, cert-manager)" \
    "The manager needs all four; bash scripts/mock-cluster.sh installs them (idempotent)." \
    operator_prereqs
  operator_step "build $OPERATOR_IMG from lego/Dockerfile" \
    "The docker build output above names the failing layer; is the Docker daemon up?" \
    operator_build
  operator_step "import $OPERATOR_IMG onto every node" \
    "Check the node containers: docker ps --filter label=io.x-k8s.kind.cluster=bex" \
    operator_import_image
  operator_step "make -C lego/operator deploy IMG=$OPERATOR_IMG" \
    "The kubectl apply errors above name the object; a 'no matches for kind' means a CRD is missing." \
    operator_make_deploy
  # The manager watches Secrets in the apps namespace (default); production
  # grants that through Argo from deploy/gitops/base, outside config/, and
  # without it the manager crashloops on cache sync (w1/043).
  operator_step "operator apps-namespace RBAC (deploy/gitops/base/operator-apps-rbac.yaml)" \
    "The kubectl apply error above names the object." \
    wl_kubectl apply -f deploy/gitops/base/operator-apps-rbac.yaml
  # Per-App registry credentials (w2/m293 t002): the manager reads and patches
  # zot-htpasswd/zot-config and bounces the zot pod in bex-registry. Production
  # grants that through Argo from deploy/gitops/base (outside config/, whose
  # namespace transform would rewrite the Role into bex-system); without it
  # every create fails PerAppRegistryCredsFailed and every delete hangs.
  operator_step "operator registry RBAC (deploy/gitops/base/operator-registry-rbac.yaml)" \
    "bex-registry must exist: bash scripts/mock-cluster.sh installs Zot there (idempotent)." \
    wl_kubectl apply -f deploy/gitops/base/operator-registry-rbac.yaml
  # The build-ns push/pull Secrets the manager's BEX_REGISTRY_PUSH_SECRET (and
  # the BEX_REGISTRY_BUILD_PULL_SECRET default) name, from the same dev
  # bex-builder credential zot-htpasswd holds; re-minted if either drifted.
  operator_step "registry push/pull credentials (bex-build/bex-registry-push, bex-registry-pull)" \
    "Inspect: KUBECONFIG=$WL_KUBECONFIG kubectl -n $ZOT_NS get statefulset,secret" \
    operator_registry_credentials
  operator_step "imagePullPolicy IfNotPresent + restart onto the new image" \
    "Inspect: KUBECONFIG=$WL_KUBECONFIG kubectl -n bex-system get deploy,ds" \
    operator_roll
  operator_step "bex-controller-manager rolled out" \
    "Inspect: KUBECONFIG=$WL_KUBECONFIG kubectl -n bex-system logs deploy/bex-controller-manager --tail=50" \
    operator_wait
  echo
  echo "bex operator running $OPERATOR_IMG (built from this tree)."
  # Informational: config/default also ships in-cluster bex-api, ssh-gateway
  # and static-server, which need platform Secrets the mock does not create
  # (dev-N runs its own bex-api on the host). Only the manager is required.
  wl_kubectl -n bex-system get deploy,ds 2>/dev/null | sed 's/^/  /' || true
  echo "  next:      bash scripts/dev-env.sh <N> up"
}

# A just-initialised mgmt cluster reports the CAPI/CAPD Deployments Available a
# few seconds before their webhooks serve, so the first apply can fail with
# "failed calling webhook". That is retried (bounded); any other failure is not.
apply_cluster_yaml() {
  local attempt
  for attempt in $(seq 1 24); do
    if apply_err=$(mgmt_kubectl apply -f infra/clusterapi/overlays/local-capd/cluster.yaml 2>&1); then
      return 0
    fi
    grep -q "failed calling webhook" <<<"$apply_err" || return 1
    echo "    CAPI webhooks not serving yet (attempt $attempt/24); retrying in 5s"
    sleep 5
  done
  return 1
}

if [ -n "${BEX_MOCK_CLUSTER_LIB:-}" ]; then return 0 2>/dev/null || exit 0; fi

case "${1:-}" in
scale)
  acquire_lock
  require "mgmt kubeconfig (kind get kubeconfig)" refresh_mgmt_kubeconfig
  scale "${2:?usage: scale N}"
  exit 0
  ;;
operator)
  acquire_lock
  deploy_operator
  exit 0
  ;;
'') ;;
*)
  echo "usage: bash scripts/mock-cluster.sh [scale N | operator]" >&2
  exit 2
  ;;
esac

acquire_lock

# 1. infra cluster (kind) with the docker socket mounted (CAPD needs it).
# --kubeconfig keeps kind from writing (and switching) ~/.kube/config (t003).
kind get clusters 2>/dev/null | grep -qx bex-mgmt ||
  kind create cluster --config infra/local/kind-mgmt.yaml --kubeconfig "$MGMT_KUBECONFIG"
require "mgmt kubeconfig (kind get kubeconfig)" refresh_mgmt_kubeconfig

# 2. Cluster API core + Docker provider (topology enabled from the start).
# clusterctl init returns before the webhooks serve; wait or the apply below
# fails with "failed calling webhook ... connection refused".
mgmt_kubectl get ns capd-system >/dev/null 2>&1 || mgmt_clusterctl init --infrastructure docker
for ns in capi-system capd-system capi-kubeadm-bootstrap-system capi-kubeadm-control-plane-system; do
  mgmt_kubectl -n "$ns" wait deploy --all --for=condition=Available --timeout=300s
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
if ! apply_cluster_yaml; then
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
    mgmt_kubectl delete "$kind" --all --ignore-not-found >/dev/null 2>&1 || true
  done
  mgmt_kubectl apply -f infra/clusterapi/overlays/local-capd/cluster.yaml
fi

echo "waiting for the app cluster to provision..."
mgmt_kubectl wait --for=condition=Available cluster/bex --timeout=600s || true
for _ in $(seq 1 60); do
  [ "$(mgmt_kubectl get machines --no-headers 2>/dev/null | grep -c Running)" -ge 2 ] && break; sleep 8
done
# Required: clusterctl cannot produce a usable kubeconfig for a cluster whose
# machines never came up, and every stage below depends on that kubeconfig.
machines_running=$(mgmt_kubectl get machines --no-headers 2>/dev/null | grep -c Running || true)
if [ "${machines_running:-0}" -lt 2 ]; then
  echo >&2
  echo "error: only ${machines_running:-0} CAPI machine(s) reached Running within 8m (need 2)." >&2
  echo "       Inspect: kubectl --kubeconfig $MGMT_KUBECONFIG --context $MGMT get machines" >&2
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
require "node readiness (post-CNI)" wait_nodes_ready 300
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
# Root cause of the "worker pods can't reach the apiserver" quirk above (w1/108,
# 2026-09-30): Calico's default pool 192.168.0.0/16 CONTAINS the kind docker
# network (e.g. 192.168.158.0/24), so a pod dialing another node's IP — the
# apiserver ClusterIP DNATs to the control-plane node's :6443 — is treated as
# in-pool, skips natOutgoing, and leaves eth0 with its pod source address, which
# OrbStack's bridge drops. Masquerade pod -> node-subnet traffic on every node
# (a DaemonSet, so autoscaled/restarted nodes re-acquire it). The control-plane
# pins above stay as belt-and-braces.
node_subnet="$(docker network inspect kind -f '{{range .IPAM.Config}}{{.Subnet}} {{end}}' 2>/dev/null |
  tr ' ' '\n' | grep -E '^[0-9]+\.' | head -1)"
if [ -n "$node_subnet" ]; then
  KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f - >/dev/null <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata: {name: pod-node-snat, namespace: kube-system, labels: {app: pod-node-snat}}
spec:
  selector: {matchLabels: {app: pod-node-snat}}
  template:
    metadata: {labels: {app: pod-node-snat}}
    spec:
      hostNetwork: true
      hostPID: true
      priorityClassName: system-node-critical
      tolerations: [{operator: Exists}]
      containers:
        - name: snat
          image: busybox:1.36
          securityContext: {privileged: true}
          resources: {requests: {cpu: 1m, memory: 8Mi}}
          command: [nsenter, -t, "1", -m, -n, --, sh, -c]
          args:
            - |
              rule="POSTROUTING -s 192.168.0.0/16 -d $node_subnet -o eth0 -j MASQUERADE"
              while true; do
                iptables -t nat -C \$rule 2>/dev/null || iptables -t nat -A \$rule
                sleep 60
              done
EOF
fi

# CNPG instance managers talk to the apiserver. Production allows exactly that
# through the Cilium allow-cnpg-apiserver-egress policy, which the local overlay
# deletes because this cluster runs Calico. Without a replacement, the operator's
# default-deny tenant namespaces leave every local tenant Postgres stuck in
# "Setting up primary" (initdb: dial tcp <apiserver ClusterIP>:443 i/o timeout;
# w2/053). Calico's equivalent: allow egress from cnpg.io/cluster pods to the
# apiserver's ClusterIP and endpoints, resolved now (re-run after a control-plane
# IP change). Order 100 evaluates before the k8s NetworkPolicies (order 1000).
api_svc_ip="$(KUBECONFIG="$WL_KUBECONFIG" kubectl get svc kubernetes -n default -o jsonpath='{.spec.clusterIP}' 2>/dev/null)"
api_eps="$(KUBECONFIG="$WL_KUBECONFIG" kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes \
  -o jsonpath='{range .items[*].endpoints[*].addresses[*]}{@}{" "}{end}' 2>/dev/null)"
api_port="$(KUBECONFIG="$WL_KUBECONFIG" kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes \
  -o jsonpath='{.items[0].ports[0].port}' 2>/dev/null)"
if [ -n "$api_svc_ip" ] && [ -n "$api_eps" ] && [ -n "$api_port" ]; then
  api_nets="\"$api_svc_ip/32\""
  for ep in $api_eps; do api_nets="$api_nets, \"$ep/32\""; done
  KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f - >/dev/null <<EOF
apiVersion: crd.projectcalico.org/v1
kind: GlobalNetworkPolicy
metadata:
  name: allow-cnpg-apiserver-egress
spec:
  order: 100
  selector: has(cnpg.io/cluster)
  types: [Egress]
  egress:
    - action: Allow
      protocol: TCP
      destination:
        nets: [$api_nets]
        ports: [443, $api_port]
EOF
else
  echo "warn: could not resolve the apiserver ClusterIP/endpoints; local tenant Postgres will not initialize (w2/053)" >&2
fi

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
# apiserver for a `kubernetes.io/kubelet-serving` cert, and kube-controller-manager
# NEVER auto-approves those; without an approver every CSR sits Pending and
# `kubectl port-forward`/`exec`/`logs` fail cluster-wide with "error dialing
# backend: remote error: tls: internal error" (it wedged scripts/dev-env.sh at
# its Hydra bootstrap step).
install_kubelet_csr_approver
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
# fallback and `kubectl top` (w5/057); without it every `dev-N` Metrics page is
# empty. Installed after the CSR approver so kubelets present approved serving
# certs.
install_metrics_server
require "metrics-server availability" \
  env KUBECONFIG="$WL_KUBECONFIG" kubectl -n kube-system wait deploy/metrics-server \
  --for=condition=Available --timeout=180s

# Traefik edge + its CRDs, from the production chart pin (t001). After the
# CSR approver (the Traefik pod's logs/port-forward need approved kubelet
# serving certs) and before the operator, whose make deploy and web services
# need the traefik.io CRDs.
require "Traefik edge (traefik.io CRDs + controller)" install_traefik
# The CNPG operator: the bex manager cannot start without its CRDs.
require "CNPG operator (postgresql.cnpg.io)" install_cnpg

# The build boundary: the operator runs builds in bex-build
# (BEX_BUILD_NAMESPACE) and reclaims a deleted App's build credentials there,
# which needs the namespace and its RBAC. Production gets both from
# deploy/gitops/base/build-boundary.yaml through Argo CD, which this mock does
# not install, so without it every App deletion on a dev-N stack stalls on its
# finalizer (w5/084). See apply_build_boundary.
require "build boundary (bex-build)" apply_build_boundary

# The platform registry (w2/m293 t001): Zot in bex-registry from production's
# pin and values, with dev bex-builder credentials. After the build boundary,
# whose bex-build namespace holds the push/pull Secrets.
require "platform registry (Zot in $ZOT_NS)" install_zot

# The bex-datastore PriorityClass (w2/m247): the operator stamps it on every
# new Postgres/Key Value Pod so the workspace compute quota excludes them, and
# a Pod naming a class that does not exist is refused at admission. Production
# gets it from deploy/gitops/base/datastore-priority.yaml through Argo CD.
require "datastore PriorityClass" \
  env KUBECONFIG="$WL_KUBECONFIG" kubectl apply -f deploy/gitops/base/datastore-priority.yaml

# The render-v1 Localhost seccomp profile (w6/m150): bex-api creates every new
# Docker/image service as render-v1 by default (w6/090), and those pods ask the
# kubelet for profiles/bex-render-v1.json, so a node without the file fails
# them with CreateContainerError. Production installs it through
# deploy/gitops/base/seccomp-profiles.yaml; kustomize names the ConfigMap there,
# so this mock creates it under the plain name the DaemonSet references.
require "render-v1 seccomp profile" \
  sh -c 'env KUBECONFIG="$1" kubectl -n kube-system create configmap seccomp-profiles \
           --from-file=deploy/gitops/base/seccomp/bex-render-v1.json --dry-run=client -o yaml |
         env KUBECONFIG="$1" kubectl apply -f - &&
         env KUBECONFIG="$1" kubectl apply -f deploy/gitops/base/seccomp-profiles.yaml' _ "$WL_KUBECONFIG"

# 5. cluster-autoscaler beside CAPI (w1/m3) — same installer as prod CI.
#    Why on the mgmt cluster: infra/clusterapi/autoscaler-values.yaml.
#    Its helm reads the private mgmt kubeconfig and the isolated registry config.
KUBECONFIG="$MGMT_KUBECONFIG" helm_isolated bash scripts/install-autoscaler.sh "$MGMT"

verify_substrate

echo
echo "app cluster 'bex' up. kubeconfig: $WL_KUBECONFIG (mgmt: $MGMT_KUBECONFIG)"
# Scope, stated plainly: this run publishes a substrate plus the Traefik edge.
# The bex operator and App CRD are the separate `operator` step.
echo "  next:         bash scripts/mock-cluster.sh operator   (bex operator + App CRD)"
echo "  nodes:        KUBECONFIG=$WL_KUBECONFIG kubectl get nodes"
echo "  add machine:  bash scripts/mock-cluster.sh scale 3"
echo "  remove:       bash scripts/mock-cluster.sh scale 1"
