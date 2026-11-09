#!/usr/bin/env bash
# Contract tests for scripts/mock-cluster.sh (w7/m148 t006).
#
# These cover the two behaviours that used to fail silently and destructively:
# a second run racing the first, and a failed kubeconfig generation truncating
# the shared config every other session reads. Both are exercised against the
# real functions, with clusterctl/docker/kubectl stubbed — provisioning a CAPD
# cluster is not something a test suite should do.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fails=0
ok()   { printf '  ok    %s\n' "$1"; }
bad()  { printf '  FAIL  %s\n' "$1"; fails=$((fails + 1)); }
check(){ if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want '$3', got '$2')"; fi; }

# Source definitions only: the guard returns before any provisioning runs.
BEX_MOCK_CLUSTER_LIB=1
export BEX_MOCK_CLUSTER_LIB
# shellcheck source=/dev/null
. scripts/mock-cluster.sh

LOCK_DIR="$tmp/lock"
WL_KUBECONFIG="$tmp/bex.kubeconfig"

echo "lock ownership:"

acquire_lock >/dev/null
check "acquiring an unheld lock succeeds" "$LOCK_HELD" "1"
check "the lock records the owning pid" "$(cat "$LOCK_DIR/pid")" "$$"

# A live owner must not be displaced. The subshell inherits the held lock dir
# but pretends to be a different process by re-running acquire_lock, which must
# refuse because our own pid is alive.
set +e
( LOCK_HELD=; acquire_lock >/dev/null 2>&1 ) ; rc=$?
set -e
check "a second run refuses while a live owner holds it" "$rc" "1"
check "the refusal left the original owner in place" "$(cat "$LOCK_DIR/pid")" "$$"

# A crashed run leaves a lock whose pid is gone; that must be reclaimable, or
# one crash wedges every future run.
printf '%s\n' "999999" >"$LOCK_DIR/pid"
LOCK_HELD=
acquire_lock >/dev/null
check "a stale lock (dead owner) is reclaimed" "$(cat "$LOCK_DIR/pid")" "$$"

# release_lock must be ownership-aware: not holding it means not removing it.
LOCK_HELD=
release_lock
check "release does nothing when this process does not hold it" \
  "$([ -d "$LOCK_DIR" ] && echo present || echo gone)" "present"
LOCK_HELD=1
release_lock
check "release removes the lock this process holds" \
  "$([ -d "$LOCK_DIR" ] && echo present || echo gone)" "gone"

# A TERM must stop the run AND release the lock. The old `trap release_lock
# INT TERM` released it and then let the run continue without it.
set +e
out="$( (BEX_MOCK_CLUSTER_LIB=1; . scripts/mock-cluster.sh; LOCK_DIR="$tmp/term-lock"; LOCK_HELD=
  acquire_lock >/dev/null; kill -TERM "$BASHPID"; echo "continued after TERM") 2>&1)"; rc=$?
set -e
check "a TERM ends the run" "$(grep -c 'continued after TERM' <<<"$out" || true)" "0"
check "a TERM exits 143" "$rc" "143"
check "a TERM releases the lock" "$([ -d "$tmp/term-lock" ] && echo held || echo released)" "released"

echo
echo "kubeconfig publication:"

printf 'PRIOR-CONFIG\n' >"$WL_KUBECONFIG"
prior_sum="$(shasum "$WL_KUBECONFIG" | awk '{print $1}')"

# 1. clusterctl fails -> the shared file must be untouched, not truncated.
clusterctl() { return 1; }
docker() { echo "0.0.0.0:6443"; }
kubectl() { return 0; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "clusterctl failure is reported" "$rc" "1"
check "clusterctl failure leaves the prior kubeconfig byte-identical" \
  "$(shasum "$WL_KUBECONFIG" | awk '{print $1}')" "$prior_sum"

# 2. A config that disables TLS verification must never be published.
clusterctl() { printf 'server: https://10.0.0.1:6443\ninsecure-skip-tls-verify: true\n'; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "a TLS-skipping kubeconfig is refused" "$rc" "1"
check "the refusal leaves the prior kubeconfig byte-identical" \
  "$(shasum "$WL_KUBECONFIG" | awk '{print $1}')" "$prior_sum"

# 3. An unreachable apiserver must not be published either.
clusterctl() { printf 'server: https://10.0.0.1:6443\n'; }
kubectl() { return 1; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "an unreachable apiserver is refused" "$rc" "1"
check "the unreachable refusal leaves the prior kubeconfig byte-identical" \
  "$(shasum "$WL_KUBECONFIG" | awk '{print $1}')" "$prior_sum"

# 4. The happy path publishes, and rewrites the server to the published port.
docker() { echo "127.0.0.1:34567"; }
kubectl() { return 0; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "a valid kubeconfig publishes" "$rc" "0"
check "the server is rewritten to the host-published port" \
  "$(grep -c 'server: https://127.0.0.1:34567' "$WL_KUBECONFIG")" "1"

# 5. No credential material may reach stdout/stderr.
clusterctl() { printf 'server: https://10.0.0.1:6443\nclient-key-data: SUPERSECRETKEYMATERIAL\n'; }
out="$(publish_kubeconfig 2>&1 || true)"
check "publication never prints credential material" \
  "$(printf '%s' "$out" | grep -c 'SUPERSECRETKEYMATERIAL' || true)" "0"

echo "components the local Argo also ships (w1/127):"
# 6. Argo owns the approver once the local app-of-apps runs: a bootstrap Helm
#    install over it fails on field ownership, so it must not run.
helm_calls="$tmp/helm-calls"
: >"$helm_calls"
helm() { echo "helm $*" >>"$helm_calls"; }
kubectl() { [ "$*" = "-n argocd get application kubelet-csr-approver" ]; }
install_kubelet_csr_approver >/dev/null
check "an Argo-managed approver gets no Helm install" "$(wc -l <"$helm_calls" | tr -d ' ')" "0"
# 7. A bare mock has no Argo: the approver is installed, with the DNS bypass.
kubectl() { return 1; }
install_kubelet_csr_approver >/dev/null
check "a bare mock installs the approver once" "$(wc -l <"$helm_calls" | tr -d ' ')" "1"
check "the bare-mock install carries the CAPD DNS bypass" "$(grep -c 'bypassDnsResolution=true' "$helm_calls")" "1"
# 8. metrics-server follows the same rule.
: >"$helm_calls"
kubectl() { [ "$*" = "-n argocd get application metrics-server" ]; }
install_metrics_server >/dev/null
check "an Argo-managed metrics-server gets no Helm install" "$(wc -l <"$helm_calls" | tr -d ' ')" "0"
kubectl() { return 1; }
install_metrics_server >/dev/null
check "a bare mock installs metrics-server once" "$(grep -c 'install metrics-server' "$helm_calls")" "1"
unset -f helm kubectl

echo "build boundary on a Calico mock:"
# 9. A fresh CAPD mock has no cilium.io CRDs: the boundary must apply without
#    its Cilium policies, keeping the namespace, RBAC and NetworkPolicies.
applied="$tmp/applied.yaml"
kubectl() { cat >"$applied"; }
apply_build_boundary
check "no Cilium object reaches the Calico mock" "$(grep -c '^kind: Cilium' "$applied" || true)" "0"
check "both bex-build RoleBindings are applied" "$(grep -c '^kind: RoleBinding' "$applied")" "2"
unset -f kubectl

echo "no global kube context (w2/m289 t003):"
# 10. Every mgmt-cluster call names the private kubeconfig AND the context, so
#     a cleared or switched ~/.kube/config current-context cannot redirect it.
calls="$tmp/ctx-calls"
: >"$calls"
kubectl() { echo "kubectl $*" >>"$calls"; }
clusterctl() { echo "clusterctl $*" >>"$calls"; printf 'server: https://10.0.0.1:6443\n'; }
docker() { echo "127.0.0.1:34567"; }
mgmt_kubectl get ns capd-system
scale 2 >/dev/null
publish_kubeconfig >/dev/null 2>&1 || true
mgmt_clusterctl init --infrastructure docker >/dev/null
check "every mgmt kubectl call names --kubeconfig and --context" \
  "$(grep '^kubectl' "$calls" | grep -v -- '--request-timeout' | grep -vc -- "--kubeconfig $MGMT_KUBECONFIG --context kind-bex-mgmt" || true)" "0"
check "the mgmt kubectl calls were exercised" "$(grep -c "^kubectl --kubeconfig $MGMT_KUBECONFIG --context kind-bex-mgmt" "$calls")" "2"
check "every clusterctl call names --kubeconfig and --kubeconfig-context" \
  "$(grep '^clusterctl' "$calls" | grep -vc -- "--kubeconfig $MGMT_KUBECONFIG --kubeconfig-context kind-bex-mgmt" || true)" "0"
check "clusterctl get kubeconfig goes through the explicit context" \
  "$(grep -c "^clusterctl --kubeconfig $MGMT_KUBECONFIG --kubeconfig-context kind-bex-mgmt get kubeconfig bex" "$calls")" "1"
unset -f kubectl clusterctl docker
# 11. No command in the script runs kubectl/clusterctl context-less or
#     switches the global context. Comments are stripped before scanning.
code="$(sed -e 's/^[[:space:]]*#.*//' scripts/mock-cluster.sh)"
check "the script never runs kubectl config use-context/set-context" \
  "$(printf '%s\n' "$code" | grep -cE 'kubectl[^|;]*config (use-context|set-context|unset)' || true)" "0"
check "no bare kubectl/clusterctl in command position" \
  "$(printf '%s\n' "$code" | grep -cE '(^|[;&|(]|\$\(|\|\||&&)[[:space:]]*(kubectl|clusterctl)[[:space:]]' || true)" "0"
check "kind create writes only the private mgmt kubeconfig" \
  "$(printf '%s\n' "$code" | grep -c 'kind create cluster --config infra/local/kind-mgmt.yaml --kubeconfig "$MGMT_KUBECONFIG"')" "1"
# 12. The private mgmt kubeconfig comes from kind itself, is 0600, and a
#     failure leaves the previous one in place; ~/.kube/config is never read.
MGMT_KUBECONFIG="$tmp/mgmt/bex-mgmt.kubeconfig"
kind() { [ "$*" = "get kubeconfig --name bex-mgmt" ] && printf 'current-context: kind-bex-mgmt\n'; }
HOME="$tmp/no-home" refresh_mgmt_kubeconfig
check "refresh writes the kind mgmt kubeconfig privately" "$(cat "$MGMT_KUBECONFIG")" "current-context: kind-bex-mgmt"
check "the mgmt kubeconfig is 0600" "$(stat -c '%a' "$MGMT_KUBECONFIG" 2>/dev/null || stat -f '%Lp' "$MGMT_KUBECONFIG")" "600"
check "refresh never touches \$HOME/.kube" "$([ -e "$tmp/no-home/.kube" ] && echo touched || echo untouched)" "untouched"
kind() { return 1; }
set +e
refresh_mgmt_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "a kind failure is reported" "$rc" "1"
check "a kind failure keeps the previous mgmt kubeconfig" "$(cat "$MGMT_KUBECONFIG")" "current-context: kind-bex-mgmt"
unset -f kind

echo "helm registry-credential isolation (w2/m289 t004):"
# 13. helm v4 asks docker-credential-osxkeychain whenever its registry config
#     and Docker config hold no auths — so an EMPTY override still hit a locked
#     keychain. The wrapper must point both at one private file that is
#     "configured" (a placeholder auths entry) and names no credential store.
fakebin="$tmp/fakebin"
mkdir -p "$fakebin"
cat >"$fakebin/helm" <<'FAKE'
#!/usr/bin/env bash
{ echo "HELM_REGISTRY_CONFIG=${HELM_REGISTRY_CONFIG:-}"; echo "DOCKER_CONFIG=${DOCKER_CONFIG:-}"; echo "ARGS=$*"; } >>"$HELM_ENV_LOG"
FAKE
chmod +x "$fakebin/helm"
export HELM_ENV_LOG="$tmp/helm-env"
: >"$HELM_ENV_LOG"
# shellcheck source=lib/helm-isolated.sh
. scripts/lib/helm-isolated.sh
BEX_HELM_ISOLATED_DIR="$tmp/helm-registry"
kubectl() { return 1; }
PATH="$fakebin:$PATH" install_kubelet_csr_approver >/dev/null
unset -f kubectl
reg="$(sed -n 's/^HELM_REGISTRY_CONFIG=//p' "$HELM_ENV_LOG" | head -1)"
dcfg="$(sed -n 's/^DOCKER_CONFIG=//p' "$HELM_ENV_LOG" | head -1)"
check "the OCI chart install ran through the isolated helm" "$(grep -c 'ARGS=upgrade --install kubelet-csr-approver oci://' "$HELM_ENV_LOG")" "1"
check "helm's registry config is the private file" "$reg" "$tmp/helm-registry/config.json"
check "helm's Docker-config fallback is the same private file" "$dcfg/config.json" "$reg"
check "the private config names no credential store or helper" \
  "$(jq -r '[has("credsStore"), has("credHelpers")] | any' "$reg")" "false"
check "the private config counts as configured (blocks native-store detection)" \
  "$(jq -r '.auths | length > 0' "$reg")" "true"
check "the private config dir is owner-only" "$(stat -c '%a' "$dcfg" 2>/dev/null || stat -f '%Lp' "$dcfg")" "700"
check "the caller's own environment keeps no DOCKER_CONFIG" "${DOCKER_CONFIG:-unset}" "unset"
check "the autoscaler installer's helm is isolated too" \
  "$(grep -c 'KUBECONFIG="$MGMT_KUBECONFIG" helm_isolated bash scripts/install-autoscaler.sh "$MGMT"' scripts/mock-cluster.sh)" "1"
# A directory another user owns is refused rather than trusted.
if [ "$(id -u)" != 0 ] && [ -d /var/empty ] && [ ! -O /var/empty ]; then
  set +e
  # shellcheck disable=SC2034 # read by helm_isolated_dir
  ( BEX_HELM_ISOLATED_DIR=/var/empty; helm_isolated true ) >/dev/null 2>&1; rc=$?
  set -e
  check "a config dir owned by someone else is refused" "$rc" "1"
fi

echo "Traefik edge from the production pin (w2/m289 t001):"
# 14. The pin is read from deploy/helm-artifacts.lock, and it is the archive
#     production's Argo Application mirrors (same OCI digest).
read -r t_version t_repo t_sha <<<"$(helm_pin traefik)"
check "the Traefik version comes from the lock" "$t_version" "$(awk -F'|' '$1=="traefik"{print $2}' deploy/helm-artifacts.lock)"
check "the Traefik repo comes from the lock" "$t_repo" "https://traefik.github.io/charts"
check "the archive sha256 is pinned" "${#t_sha}" "64"
check "production's Argo traefik Application runs the same locked archive" \
  "$(grep -c "targetRevision: \"$(awk -F'|' '$1=="traefik"{print $5}' deploy/helm-artifacts.lock)\"" deploy/gitops/base/traefik.yaml)" "1"
# 15. An archive that does not match the pinned sha256 is never installed.
tcalls="$tmp/traefik-calls"
: >"$tcalls"
helm() {
  echo "helm $*" >>"$tcalls"
  if [ "$1" = pull ]; then
    local dest="" ver=""
    while [ $# -gt 0 ]; do
      case $1 in --destination) dest=$2; shift ;; --version) ver=$2; shift ;; esac
      shift
    done
    printf 'fake-chart-bytes' >"$dest/traefik-$ver.tgz"
  fi
}
kubectl() { echo "kubectl $*" >>"$tcalls"; [ "$*" != "-n argocd get application traefik" ]; }
set +e
install_traefik >/dev/null 2>&1; rc=$?
set -e
check "a sha256 mismatch is refused" "$rc" "1"
check "a mismatched archive never reaches helm upgrade" "$(grep -c '^helm upgrade' "$tcalls" || true)" "0"
# 16. The verified archive installs with the production base values, after
#     the namespace exemption and the plugin ConfigMaps.
printf 'fake-chart-bytes' >"$tmp/fake.tgz"
helm_pin() { echo "41.0.0 https://traefik.github.io/charts $(sha256_of "$tmp/fake.tgz")"; }
: >"$tcalls"
install_traefik >/dev/null
check "the verified archive is installed into namespace traefik" \
  "$(grep -cE '^helm upgrade --install traefik .*/traefik-41\.0\.0\.tgz -n traefik -f deploy/gitops/base/values/traefik.values.yaml --wait' "$tcalls")" "1"
check "the edge namespace is exempted from baseline PodSecurity" \
  "$(grep -c 'label namespace traefik pod-security.kubernetes.io/enforce=privileged' "$tcalls")" "1"
check "the plugin ConfigMaps are applied before the release" \
  "$(awk '/apply -k deploy\/gitops\/charts\/traefik-plugins/{p=NR} /^helm upgrade/{h=NR} END{print (p && h && p<h) ? "ordered" : "unordered"}' "$tcalls")" "ordered"
check "every traefik.io CRD the operator writes is waited for" \
  "$(grep -c 'wait --for=condition=established crd/.*traefik.io' "$tcalls")" "4"
# 17. Argo owns Traefik once the local app-of-apps runs: no bootstrap install.
kubectl() { echo "kubectl $*" >>"$tcalls"; [ "$*" = "-n argocd get application traefik" ]; }
: >"$tcalls"
install_traefik >/dev/null
check "an Argo-managed Traefik gets no Helm call" "$(grep -c '^helm' "$tcalls" || true)" "0"
unset -f helm kubectl
unset -f helm_pin
eval "$(awk '/^helm_pin\(\)/,/^}$/' scripts/mock-cluster.sh)"
# 17a. The CNPG operator the manager depends on comes from production's pin
#      too, and is skipped when present or Argo-managed.
check "the CNPG pin is production's cloudnative-pg line" \
  "$(helm_pin cloudnative-pg | awk '{print $1, $2}')" "$(awk -F'|' '$1=="cloudnative-pg"{print $2, $3}' deploy/helm-artifacts.lock)"
ccalls="$tmp/cnpg-calls"
: >"$ccalls"
helm() { echo "helm $*" >>"$ccalls"; }
kubectl() { echo "kubectl $*" >>"$ccalls"; case "$*" in "get crd clusters.postgresql.cnpg.io") return 0 ;; *"get deploy -l app.kubernetes.io/name=cloudnative-pg -o name") echo deployment.apps/cnpg-cloudnative-pg ;; *) return 1 ;; esac; }
install_cnpg
check "an installed CNPG operator is left alone" "$(grep -c '^helm' "$ccalls" || true)" "0"
fetch_pinned_chart() { echo "$tmp/cloudnative-pg-0.29.1.tgz"; }
kubectl() { echo "kubectl $*" >>"$ccalls"; case "$*" in *"wait --for=condition=established crd/clusters.postgresql.cnpg.io"*) return 0 ;; *) return 1 ;; esac; }
: >"$ccalls"
install_cnpg
check "a missing CNPG operator is installed as release cnpg in cnpg-system" \
  "$(grep -c "^helm upgrade --install cnpg $tmp/cloudnative-pg-0.29.1.tgz -n cnpg-system --create-namespace --wait" "$ccalls")" "1"
unset -f helm kubectl fetch_pinned_chart
eval "$(awk '/^fetch_pinned_chart\(\)/,/^}$/' scripts/mock-cluster.sh)"
check "the bring-up requires the CNPG stage" "$(grep -c '^require "CNPG operator' scripts/mock-cluster.sh)" "1"
check "the operator prerequisites include the CNPG CRD" \
  "$(awk '/^operator_prereqs\(\)/,/^}$/' scripts/mock-cluster.sh | grep -c 'crd/clusters.postgresql.cnpg.io')" "1"
check "the bring-up requires the Traefik stage" "$(grep -c '^require "Traefik edge' scripts/mock-cluster.sh)" "1"
check "the end-state verification checks Traefik CRDs and the edge" \
  "$(grep -cE 'check "Traefik (CRDs established|edge Available)' scripts/mock-cluster.sh)" "2"

echo "node readiness tolerates a replaced machine (w2/m289):"
# 17b. A Node that vanishes mid-wait (MachineHealthCheck replacement) must not
#      fail the wait once its replacement is Ready.
polls="$tmp/polls"
: >"$polls"
kubectl() {
  echo poll >>"$polls"
  case "$(wc -l <"$polls" | tr -d ' ')" in
  1) printf 'bex-cp Ready control-plane\nbex-tenant-old NotReady <none>\n' ;;
  2) printf 'bex-cp Ready control-plane\n' ;;
  *) printf 'bex-cp Ready control-plane\nbex-tenant-new Ready <none>\n' ;;
  esac
}
BEX_NODE_POLL_SECONDS=0 wait_nodes_ready 30
check "the wait passes once the replacement is Ready" "$?" "0"
kubectl() { printf 'bex-cp Ready control-plane\nbex-tenant NotReady <none>\n'; }
set +e
BEX_NODE_POLL_SECONDS=0 wait_nodes_ready 0; rc=$?
set -e
check "a node that never goes Ready times out" "$rc" "1"
kubectl() { return 1; }
set +e
BEX_NODE_POLL_SECONDS=0 wait_nodes_ready 0; rc=$?
set -e
check "an unreachable apiserver is not 'all Ready'" "$rc" "1"
unset -f kubectl
check "the bring-up waits with the replacement-tolerant poll" \
  "$(grep -c '^require "node readiness (post-CNI)" wait_nodes_ready 300$' scripts/mock-cluster.sh)" "1"

echo "operator subcommand (w2/m289 t002):"
# 18. The pull-policy patch touches only containers on the operator image.
# shellcheck disable=SC2034 # read by the sourced operator_* functions
OPERATOR_IMG=bex-operator:dev
patch="$(operator_pull_policy_patch <<'JSON'
{"spec":{"template":{"spec":{
  "initContainers":[{"name":"probe","image":"bex-operator:dev","imagePullPolicy":"Always"}],
  "containers":[{"name":"manager","image":"bex-operator:dev","imagePullPolicy":"Always"},
                {"name":"sidecar","image":"busybox:1.36","imagePullPolicy":"Always"},
                {"name":"done","image":"bex-operator:dev","imagePullPolicy":"IfNotPresent"}]}}}}
JSON
)"
check "the patch sets IfNotPresent on exactly the operator-image containers" \
  "$(jq -r '[.[] | .path] | join(",")' <<<"$patch")" \
  "/spec/template/spec/containers/0/imagePullPolicy,/spec/template/spec/initContainers/0/imagePullPolicy"
check "an already-IfNotPresent workload needs no patch" \
  "$(operator_pull_policy_patch <<<'{"spec":{"template":{"spec":{"containers":[{"image":"bex-operator:dev","imagePullPolicy":"IfNotPresent"}]}}}}')" ""
# 19. Only bex-system workloads running the image are selected.
kubectl() {
  cat <<'JSON'
{"items":[
 {"kind":"Deployment","metadata":{"name":"bex-controller-manager"},"spec":{"template":{"spec":{"containers":[{"image":"bex-operator:dev"}]}}}},
 {"kind":"DaemonSet","metadata":{"name":"bex-pg-sni-proxy"},"spec":{"template":{"spec":{"containers":[{"image":"bex-operator:dev"}]}}}},
 {"kind":"Deployment","metadata":{"name":"unrelated"},"spec":{"template":{"spec":{"containers":[{"image":"nginx:1"}]}}}}]}
JSON
}
check "workloads on the operator image are found" "$(operator_workloads | tr '\n' ' ')" "deployment/bex-controller-manager daemonset/bex-pg-sni-proxy "
unset -f kubectl
# 20. The image reaches every node the workload cluster reports, byte-exact.
icalls="$tmp/import-calls"
: >"$icalls"
kubectl() { printf 'bex-cp-1\nbex-worker-0-a\nbex-tenant-0-b\n'; }
docker() {
  case $1 in
  save) printf 'IMAGE-TAR' >"$4" ;;
  exec) echo "exec $3 $4 $5 $6 $7 $8 $9 $(cat)" >>"$icalls" ;;
  esac
}
operator_import_image >/dev/null
check "the image is imported on every node" "$(wc -l <"$icalls" | tr -d ' ')" "3"
check "each import feeds ctr the saved image into k8s.io" \
  "$(grep -c '^exec bex-[a-z0-9-]* ctr -n k8s.io images import - IMAGE-TAR$' "$icalls")" "3"
docker() {
  case $1 in
  save) printf 'IMAGE-TAR' >"$4" ;;
  exec) cat >/dev/null; [ "$3" != bex-worker-0-a ] ;;
  esac
}
out="$(operator_import_image 2>&1 || echo "rc=$?")"
check "a failed node import is fatal" "$(grep -c 'rc=1' <<<"$out")" "1"
check "the failed import names the node" "$(grep -c "into node container 'bex-worker-0-a' failed" <<<"$out")" "1"
kubectl() { :; }
docker() { [ "$1" = save ] && printf 'x' >"$4"; }
out="$(operator_import_image 2>&1 || echo "rc=$?")"
check "no nodes at all is fatal, not a silent success" "$(grep -c 'rc=1' <<<"$out")" "1"
unset -f kubectl docker
# 21. make deploy runs GOWORK=off with an ABSOLUTE workload kubeconfig (make
#     -C changes directory) and the dev image.
cat >"$fakebin/make" <<'FAKE'
#!/usr/bin/env bash
echo "KUBECONFIG=$KUBECONFIG GOWORK=$GOWORK ARGS=$*" >"$MAKE_LOG"
FAKE
chmod +x "$fakebin/make"
export MAKE_LOG="$tmp/make-log"
(WL_KUBECONFIG=infra/local/bex.kubeconfig; PATH="$fakebin:$PATH" operator_make_deploy)
check "make deploy targets the absolute workload kubeconfig, GOWORK=off, the dev image" \
  "$(cat "$MAKE_LOG")" "KUBECONFIG=$repo_root/infra/local/bex.kubeconfig GOWORK=off ARGS=-C lego/operator deploy IMG=bex-operator:dev"
# 22. The steps run in dependency order, and the first failure stops the run
#     with that step's name.
steps="$tmp/steps"
: >"$steps"
wl_kubectl() { echo "wl $*" >>"$steps"; }
operator_prereqs() { echo prereqs >>"$steps"; }
operator_build() { echo build >>"$steps"; }
operator_import_image() { echo import >>"$steps"; }
operator_make_deploy() { echo deploy >>"$steps"; }
operator_roll() { echo roll >>"$steps"; }
operator_wait() { echo wait >>"$steps"; }
deploy_operator >/dev/null
check "operator steps run in order" \
  "$(grep -vE '^wl (--request-timeout|-n bex-system get deploy,ds)' "$steps" | sed 's/^wl apply -f deploy\/gitops\/base\/operator-apps-rbac.yaml$/rbac/' | tr '\n' ' ')" \
  "prereqs build import deploy rbac roll wait "
operator_make_deploy() { echo deploy >>"$steps"; return 1; }
: >"$steps"
set +e
out="$( (deploy_operator) 2>&1)"; rc=$?
set -e
check "a failed step stops the run" "$rc" "1"
check "the failure names its step" "$(grep -c "operator step 'make -C lego/operator deploy IMG=bex-operator:dev' failed" <<<"$out")" "1"
check "nothing after the failed step runs" "$(grep -cE '^(roll|wait)$' "$steps" || true)" "0"
unset -f wl_kubectl
# 23. Unknown subcommands are refused before the lock is taken.
set +e
out="$(env -u BEX_MOCK_CLUSTER_LIB bash scripts/mock-cluster.sh bogus 2>&1)"; rc=$?
set -e
check "an unknown subcommand exits 2" "$rc" "2"
check "the usage names the operator subcommand" "$(grep -c 'operator' <<<"$out")" "1"

echo "cluster.yaml apply on a just-initialised mgmt cluster:"
# A webhook that is not serving yet is retried; any other error is not.
calls="$tmp/apply-calls"; : >"$calls"
sleep() { :; }
mgmt_kubectl() { echo x >>"$calls"; [ "$(wc -l <"$calls")" -ge 3 ] && return 0; echo 'Internal error occurred: failed calling webhook "default.dockerclustertemplate"'; return 1; }
apply_cluster_yaml >/dev/null
check "a not-yet-serving webhook is retried until the apply lands" "$?:$(wc -l <"$calls" | tr -d ' ')" "0:3"
: >"$calls"
mgmt_kubectl() { echo x >>"$calls"; echo 'error: field is immutable'; return 1; }
set +e; apply_cluster_yaml >/dev/null; rc=$?; set -e
check "any other failure is returned at once" "$rc:$(wc -l <"$calls" | tr -d ' ')" "1:1"
unset -f mgmt_kubectl sleep

echo
if [ "$fails" -ne 0 ]; then
  echo "FAILED: $fails check(s)" >&2
  exit 1
fi
echo "mock-cluster.sh contracts: all checks passed"
