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
operator_registry_credentials() { echo regcreds >>"$steps"; }
deploy_operator >/dev/null
check "operator steps run in order" \
  "$(grep -vE '^wl (--request-timeout|-n bex-system get deploy,ds)' "$steps" |
     sed -e 's/^wl apply -f deploy\/gitops\/base\/operator-apps-rbac.yaml$/rbac/' \
         -e 's/^wl apply -f deploy\/gitops\/base\/operator-registry-rbac.yaml$/regrbac/' | tr '\n' ' ')" \
  "prereqs build import deploy rbac regrbac regcreds roll wait "
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

unset -f operator_registry_credentials
eval "$(awk '/^operator_registry_credentials\(\)/,/^}$/' scripts/mock-cluster.sh)"

echo "platform registry: Zot from the production pin (w2/m293 t001/t002):"
# 24. The archive, values and Service are production's.
check "the Zot pin is production's lock line" \
  "$(helm_pin zot | awk '{print $1, $2}')" "$(awk -F'|' '$1=="zot"{print $2, $3}' deploy/helm-artifacts.lock)"
check "production's Argo zot Application runs the same locked archive" \
  "$(grep -c "targetRevision: \"$(awk -F'|' '$1=="zot"{print $5}' deploy/helm-artifacts.lock)\"" deploy/gitops/base/zot.yaml)" "1"
zvals="$(zot_values)"
check "the values are production's (NodePort 30500 behind :5000)" \
  "$(yq '.service.type + " " + (.service.port|tostring) + " " + (.service.nodePort|tostring)' <<<"$zvals")" "NodePort 5000 30500"
check "the config and htpasswd come from the external Secrets the operator manages" \
  "$(yq '[.mountConfig, (.externalSecrets[].secretName)] | join(" ")' <<<"$zvals")" "false zot-config zot-htpasswd"
check "the containerd hosts.toml on every local-capd pool points the registry name at that NodePort" \
  "$(grep -c "printf 'server = \"http://127.0.0.1:30500\"" infra/clusterapi/overlays/local-capd/cluster.yaml)" "3"
# 25. The seeded zot-config is the operator's canonical base: same port,
#     htpasswd path, builder identity, platform builder repo and GC delay as
#     lego/operator/internal/registry, and anonymous access denied.
seed="$(zot_seed_config)"
go_const() { grep -hE "^\s*$1\s*=" lego/operator/internal/registry/*.go | grep -v _test | head -1 | sed -E 's/.*= *"([^"]*)".*/\1/'; }
check "the seed listens on the operator's zotHTTPPort" "$(jq -r .http.port <<<"$seed")" "$(go_const zotHTTPPort)"
check "the seed reads the htpasswd at the operator's zotHTPasswdPath" "$(jq -r .http.auth.htpasswd.path <<<"$seed")" "$(go_const zotHTPasswdPath)"
check "the seed's adminPolicy is the operator's builder identity with read/create/update/delete" \
  "$(jq -r '.http.accessControl.adminPolicy | (.users|join(",")) + " " + (.actions|join(","))' <<<"$seed")" "$(go_const zotBuilderUser) read,create,update,delete"
check "the seed denies anonymous access on **" \
  "$(jq -r '.http.accessControl.repositories["**"].defaultPolicy // [] | length' <<<"$seed")" "0"
check "the platform builder repository is read-only for authenticated users" \
  "$(jq -r --arg r "$(go_const platformBuilderRepository)" '.http.accessControl.repositories[$r].defaultPolicy | join(",")' <<<"$seed")" "read"
check "the seed's GC delay is the operator's ZotGCDelay" "$(jq -r .storage.gcDelay <<<"$seed")" "$(go_const ZotGCDelay)"
check "the seed names no retired bex-puller" "$(grep -c bex-puller <<<"$seed" || true)" "0"
# 26. Credentials: a verifying dev password is kept; anything else re-mints a
#     random one through registry-secrets.sh with .env ignored, never printed.
regcalls="$tmp/regsecrets-calls"
cat >"$tmp/fake-registry-secrets.sh" <<'FAKE'
#!/usr/bin/env bash
printf 'noenv=%s pwlen=%s pwhex=%s ns=%s registry=%s build=%s\n' "$BEX_REGISTRY_SECRETS_NO_ENV_FILE" \
  "${#BEX_REGISTRY_BUILDER_PASSWORD}" "$([[ $BEX_REGISTRY_BUILDER_PASSWORD =~ ^[0-9a-f]+$ ]] && echo yes || echo no)" \
  "$REGISTRY_NS" "$BEX_REGISTRY" "$BEX_BUILD_NAMESPACE" >>"$REGCALLS"
printf '%s\n' "$BEX_REGISTRY_BUILDER_PASSWORD" >"$REGCALLS.pw"
FAKE
export REGCALLS="$regcalls"
REGISTRY_SECRETS_SCRIPT="$tmp/fake-registry-secrets.sh"
dev_pw=0123456789abcdef0123456789abcdef
dev_hash="$(printf '%s\n' "$dev_pw" | "$HTPASSWD_BIN" -nB -i bex-builder)"
push_json() { jq -cn --arg pw "$1" '{data: {"config.json": ({auths: {"zot.bex-registry.svc:5000": {username: "bex-builder", password: $pw}}} | tojson | @base64)}}'; }
htpasswd_json() { jq -cn --arg h "$1" '{data: {htpasswd: ($h + "\napp-tea-x-web:$2y$05$abc" | @base64)}}'; }
fake_pw="$dev_pw"; fake_hash="$dev_hash"; fake_missing=
wl_kubectl() {
  case "$*" in
  *"get secret bex-registry-push -o json") push_json "$fake_pw" ;;
  *"get secret zot-htpasswd -o json") htpasswd_json "$fake_hash" ;;
  *"get secret bex-registry-push bex-registry-push-kpack bex-registry-pull") [ -z "$fake_missing" ] ;;
  *) return 0 ;;
  esac
}
: >"$regcalls"
out="$(ensure_registry_credentials; echo "rotated=$REGISTRY_CREDS_ROTATED")"
check "a password that verifies against zot-htpasswd is kept" "$(wc -l <"$regcalls" | tr -d ' ')" "0"
check "keeping it is not a rotation" "$(grep -c 'rotated=$' <<<"$out")" "1"
fake_hash="$(printf 'other\n' | "$HTPASSWD_BIN" -nB -i bex-builder)"
ensure_registry_credentials >"$tmp/regout"
check "a hash that no longer verifies re-mints through registry-secrets.sh" "$(wc -l <"$regcalls" | tr -d ' ')" "1"
check "the re-mint is a rotation (Zot must restart)" "$REGISTRY_CREDS_ROTATED" "1"
check "the minted password is 32 random hex chars, .env ignored, production names" \
  "$(cat "$regcalls")" "noenv=1 pwlen=32 pwhex=yes ns=bex-registry registry=zot.bex-registry.svc:5000 build=bex-build"
check "the minted password is never printed" "$(grep -cF "$(cat "$regcalls.pw")" "$tmp/regout" || true)" "0"
check "a second mint draws a different password" "$([ "$(cat "$regcalls.pw")" != "$dev_pw" ] && echo differs)" "differs"
fake_hash="$dev_hash"; fake_missing=1; : >"$regcalls"
ensure_registry_credentials >/dev/null
check "a missing push/pull Secret re-mints the set" "$(wc -l <"$regcalls" | tr -d ' ')" "1"
fake_missing=; fake_pw=; : >"$regcalls"
ensure_registry_credentials >/dev/null
check "no push Secret at all mints one" "$(wc -l <"$regcalls" | tr -d ' ')" "1"
unset -f wl_kubectl push_json htpasswd_json go_const
eval "$(awk '/^wl_kubectl\(\)/' scripts/mock-cluster.sh)"
# registry-secrets.sh itself: BEX_REGISTRY_SECRETS_NO_ENV_FILE=1 keeps a
# repo .env from replacing the dev password (here .env empties it).
rs="$tmp/rs-repo"; mkdir -p "$rs/scripts/lib"
cp scripts/registry-secrets.sh "$rs/scripts/"; cp scripts/lib/secret-install.sh scripts/lib/env-file.sh "$rs/scripts/lib/"
printf 'BEX_REGISTRY_BUILDER_PASSWORD=\n' >"$rs/.env"; chmod 600 "$rs/.env"
printf '#!/bin/sh\nexit 0\n' >"$fakebin/htpasswd"; chmod +x "$fakebin/htpasswd"
set +e
PATH="$fakebin:$PATH" DRY_RUN=1 BEX_REGISTRY_BUILDER_PASSWORD="$dev_pw" bash "$rs/scripts/registry-secrets.sh" >/dev/null 2>&1; rc_env=$?
PATH="$fakebin:$PATH" DRY_RUN=1 BEX_REGISTRY_SECRETS_NO_ENV_FILE=1 BEX_REGISTRY_BUILDER_PASSWORD="$dev_pw" \
  bash "$rs/scripts/registry-secrets.sh" >/dev/null 2>&1; rc_noenv=$?
set -e
check "registry-secrets.sh still loads .env by default (production path unchanged)" "$rc_env" "1"
check "BEX_REGISTRY_SECRETS_NO_ENV_FILE=1 skips .env" "$rc_noenv" "0"
rm -f "$fakebin/htpasswd"
# 27. Every node must answer 401 anonymously and 200 for bex-builder on
#     127.0.0.1:30500; the credential reaches curl on stdin, never argv.
dcalls="$tmp/docker-calls"; : >"$dcalls"
registry_builder_password() { printf '%s\n' "$dev_pw"; }
wl_node_names() { printf 'bex-cp\nbex-worker-0-a\n'; }
docker() {
  echo "docker $*" >>"$dcalls"
  if [ "$2" = -i ]; then grep -q "^user = \"bex-builder:$dev_pw\"$" && printf 200 || printf 401; else printf 401; fi
}
zot_serving; rc=$?
check "401 anonymous + 200 authenticated on every node is serving" "$rc" "0"
check "both probes ran on each node" "$(grep -c '127.0.0.1:30500/v2/' "$dcalls")" "4"
check "the password never appears in docker's argv" "$(grep -cF "$dev_pw" "$dcalls" || true)" "0"
docker() { [ "$2" = -i ] && cat >/dev/null; printf 200; }
set +e; zot_serving; rc=$?; set -e
check "a registry that answers anonymous pulls is not serving" "$rc" "1"
docker() { [ "$2" = -i ] && cat >/dev/null; printf 401; }
set +e; zot_serving; rc=$?; set -e
check "a registry that refuses bex-builder is not serving" "$rc" "1"
unset -f docker registry_builder_password wl_node_names
eval "$(awk '/^registry_builder_password\(\)/,/^}$/' scripts/mock-cluster.sh)"
eval "$(awk '/^wl_node_names\(\)/' scripts/mock-cluster.sh)"
# 28. install_zot: Argo-managed means hands off; a mismatched archive never
#     installs; the verified one installs as release zot with production's
#     values, seeding zot-config only when the operator has not written one.
zcalls="$tmp/zot-calls"; : >"$zcalls"
kubectl() { echo "kubectl $*" >>"$zcalls"; [ "$*" = "-n argocd get application zot" ]; }
helm() { echo "helm $*" >>"$zcalls"; }
install_zot >/dev/null
check "an Argo-managed Zot gets no Helm call and no Secret writes" "$(grep -cE '^helm|create secret' "$zcalls" || true)" "0"
zcfg_exists=; zsts_absent=
kubectl() {
  case "$*" in
  "apply -f -") echo "kubectl apply -f - $(jq -r '.metadata.name + ":" + (.data["config.json"] // "" | @base64d | fromjson? | .http.port // "")' 2>/dev/null)" >>"$zcalls"; return 0 ;;
  *"create secret generic zot-config"*) jq -Rs '{metadata: {name: "zot-config"}, data: {"config.json": @base64}}' ;;
  *"create namespace"*) printf '{}' ;;
  esac
  echo "kubectl $*" >>"$zcalls"
  case "$*" in
  "-n argocd get application zot") return 1 ;;
  *"get secret zot-config") [ -n "$zcfg_exists" ] ;;
  *"get statefulset zot") [ -z "$zsts_absent" ] ;;
  *) return 0 ;;
  esac
}
ensure_registry_credentials() { echo ensure-creds >>"$zcalls"; REGISTRY_CREDS_ROTATED=; }
zot_serving() { echo serving >>"$zcalls"; }
helm() {
  echo "helm $*" >>"$zcalls"
  if [ "$1" = pull ]; then
    local dest="" ver=""
    while [ $# -gt 0 ]; do
      case $1 in --destination) dest=$2; shift ;; --version) ver=$2; shift ;; esac
      shift
    done
    printf 'fake-zot-bytes' >"$dest/zot-$ver.tgz"
  fi
}
set +e; install_zot >/dev/null 2>&1; rc=$?; set -e
check "a Zot archive that misses the lock's sha256 is refused" "$rc" "1"
check "a mismatched Zot archive never reaches helm upgrade" "$(grep -c '^helm upgrade' "$zcalls" || true)" "0"
printf 'fake-zot-bytes' >"$tmp/fake-zot.tgz"
helm_pin() { echo "0.1.121 https://zotregistry.dev/helm-charts $(sha256_of "$tmp/fake-zot.tgz")"; }
: >"$zcalls"
install_zot >/dev/null
check "the verified archive installs as release zot in bex-registry with values, waiting" \
  "$(grep -cE '^helm upgrade --install zot .*/zot-0\.1\.121\.tgz -n bex-registry -f .*/values\.yaml --wait' "$zcalls")" "1"
check "a missing zot-config is seeded with the canonical config (port 5000)" "$(grep -c 'apply -f - zot-config:5000$' "$zcalls")" "1"
check "credentials exist before the release and serving is proven after it" \
  "$(grep -oE '^(ensure-creds|helm upgrade|serving)' "$zcalls" | tr '\n' ' ')" "ensure-creds helm upgrade serving "
check "kept credentials leave a running Zot alone" \
  "$(grep -c 'rollout restart statefulset/zot' "$zcalls" || true)" "0"
ensure_registry_credentials() { echo ensure-creds >>"$zcalls"; REGISTRY_CREDS_ROTATED=1; }
: >"$zcalls"
install_zot >/dev/null
check "a re-mint over a running Zot bounces it onto the new htpasswd" \
  "$(grep -c 'rollout restart statefulset/zot' "$zcalls")" "1"
zsts_absent=1; : >"$zcalls"
install_zot >/dev/null
check "a first install starts on the fresh credentials with no bounce" \
  "$(grep -c 'rollout restart statefulset/zot' "$zcalls" || true)" "0"
zsts_absent=; zcfg_exists=1; : >"$zcalls"
install_zot >/dev/null
check "an existing (operator-owned) zot-config is never overwritten" "$(grep -c 'zot-config:' "$zcalls" || true)" "0"
unset -f kubectl helm helm_pin ensure_registry_credentials zot_serving
eval "$(awk '/^helm_pin\(\)/,/^}$/' scripts/mock-cluster.sh)"
eval "$(awk '/^ensure_registry_credentials\(\)/,/^}$/' scripts/mock-cluster.sh)"
eval "$(awk '/^zot_serving\(\)/,/^}$/' scripts/mock-cluster.sh)"
REGISTRY_SECRETS_SCRIPT=scripts/registry-secrets.sh
# 29. Wiring: the bring-up installs Zot after the build boundary; the end
#     state checks it; the operator step needs it and grants the RBAC.
check "the bring-up requires the Zot stage after the build boundary" \
  "$(awk '/^require "build boundary/{b=NR} /^require "platform registry \(Zot/{z=NR} END{print (b && z && b<z) ? "ordered" : "unordered"}' scripts/mock-cluster.sh)" "ordered"
check "the end-state verification checks Zot on every node" "$(grep -c 'check "Zot registry on every node' scripts/mock-cluster.sh)" "1"
check "the operator prerequisites include Zot" \
  "$(awk '/^operator_prereqs\(\)/,/^}$/' scripts/mock-cluster.sh | grep -c 'statefulset zot')" "1"
check "the operator registry RBAC grants secrets and pod delete in bex-registry" \
  "$(yq -N 'select(.kind == "Role") | .metadata.namespace + " " + ([.rules[].resources[]] | join(","))' deploy/gitops/base/operator-registry-rbac.yaml)" "bex-registry secrets,pods"

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
