#!/usr/bin/env bash
# Structural guard for the CAPH overlay — pure client-side, no cluster (w1/m36 t007).
# The baked-worker-image win (w1/m36) is only preserved if a future template edit
# can't silently reintroduce the ~350MB of scale-up-hot-path downloads, and a
# staged roll (tenant pool baked before the platform pool) can't leave a pool in
# a broken half-state. This asserts, over
# infra/clusterapi/overlays/hetzner-caph/cluster.yaml + the packer bake:
#   1. per worker MachineDeployment, the node image and its preKubeadmCommands are
#      CONSISTENT — a versioned baked snapshot must carry NO runtime
#      download / images-pull / trimmed-apt line, and an ubuntu-24.04 pool MUST
#      still download its runtime at boot (else its nodes come up with no runtime).
#      Any other imageName is rejected.
#   2. the platform pool floor stays at three nodes — prod OpenBao has three
#      required-anti-affinity Raft members, so a two-node floor is not viable.
#   3. the packer bake (infra/packer/bex-worker.pkr.hcl) installs no trimmed package.
#   4. the packer version pins (k8s/containerd/runc) MATCH the overlay — the bake
#      and the download-at-boot pools must stay in lockstep.
#   5. every CAPI workload and snapshot default matches that pin, and the retired
#      Kubernetes 1.31-or-older line cannot return.
# Run locally before pushing overlay/packer changes; CI runs it via
# .github/workflows/clusterapi-validate.yml.
# Requires: yq v4.
set -euo pipefail
cd "$(dirname "$0")/.."

# Overridable so scripts/clusterapi-validate.test.sh can drive the build-pool
# guards below against mutated fixture copies and prove they turn red.
OVERLAY="${OVERLAY_FILE:-infra/clusterapi/overlays/hetzner-caph/cluster.yaml}"
GITOPS_BASE="${GITOPS_BASE_DIR:-deploy/gitops/base}"
OPERATOR_CONFIG="${OPERATOR_CONFIG_DIR:-lego/operator/config}"
BUILD_SOURCE="${BUILD_SOURCE_FILE:-lego/operator/internal/build/build.go}"
SANDBOX_OVERLAY="infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml"
PACKER="infra/packer/bex-worker.pkr.hcl"
SNAPSHOT_WORKFLOW=".github/workflows/snapshot.yml"
SNAPSHOT_RELEASE="infra/packer/release.conf"
AUTOSCALER_VALUES="infra/clusterapi/autoscaler-values.yaml"
AUTOSCALER_APP="deploy/gitops/base/autoscaler.yaml"
fail=0

pk_var() { grep -A4 "variable \"$1\"" "$PACKER" | grep -m1 -oE 'default[[:space:]]*=[[:space:]]*"[^"]+"' | grep -oE '"[^"]+"' | tr -d '"'; }
pk_k8s="$(pk_var kubernetes_version)"; pk_containerd="$(pk_var containerd_version)"; pk_runc="$(pk_var runc_version)"
pk_containerd_sha="$(pk_var containerd_amd64_sha256)"; pk_runc_sha="$(pk_var runc_amd64_sha256)"; pk_k8s_key_sha="$(pk_var kubernetes_release_key_sha256)"

echo "==> production CAPI objects live only in the locked-down namespace"
if [ "$(yq -N 'select(.kind == "Namespace" and .metadata.name == "bex-capi") | .metadata.name' "$OVERLAY")" != "bex-capi" ]; then
  echo "FAIL: $OVERLAY must declare the bex-capi Namespace" >&2
  fail=1
fi
bad_namespace="$(yq -N 'select(.kind != "Namespace" and .metadata.namespace != "bex-capi") | .kind + "/" + .metadata.name + "=" + (.metadata.namespace // "<none>")' "$OVERLAY")"
if [ -n "$bad_namespace" ]; then
  echo "FAIL: production CAPI resources escaped bex-capi:" >&2
  echo "$bad_namespace" >&2
  fail=1
fi
bad_sandbox_namespace="$(yq -N 'select(.metadata.namespace != "bex-capi") | .kind + "/" + .metadata.name + "=" + (.metadata.namespace // "<none>")' "$SANDBOX_OVERLAY")"
if [ -n "$bad_sandbox_namespace" ]; then
  echo "FAIL: sandbox CAPI resources escaped bex-capi:" >&2
  echo "$bad_sandbox_namespace" >&2
  fail=1
fi

echo "==> supported Kubernetes version is consistent across CAPI, Packer, and snapshot defaults"
if [[ ! "$pk_k8s" =~ ^([0-9]+)\.([0-9]+)\.[0-9]+$ ]]; then
  echo "FAIL: invalid Packer Kubernetes version '$pk_k8s'" >&2
  fail=1
elif ((BASH_REMATCH[1] < 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] <= 31))); then
  echo "FAIL: Kubernetes $pk_k8s is on the retired 1.31-or-older line" >&2
  fail=1
fi
while read -r desired; do
  [ -n "$desired" ] || continue
  desired="${desired#v}"
  if [ "$desired" != "$pk_k8s" ]; then
    echo "FAIL: CAPI workload version '$desired' differs from Packer '$pk_k8s'" >&2
    fail=1
  fi
done < <(yq -N 'select(.kind == "KubeadmControlPlane" or .kind == "MachineDeployment") | (.spec.version // .spec.template.spec.version)' "$OVERLAY")
snapshot_k8s="$(sed -n 's/^KUBERNETES_VERSION=//p' "$SNAPSHOT_RELEASE")"
if [ "$snapshot_k8s" != "$pk_k8s" ]; then
  echo "FAIL: reviewed snapshot release '$snapshot_k8s' differs from Packer '$pk_k8s'" >&2
  fail=1
fi
snapshot_image="$(sed -n 's/^IMAGE_NAME=//p' "$SNAPSHOT_RELEASE")"
if [[ ! "$snapshot_image" =~ ^bex-worker-k8s-[0-9]+-[0-9]+$ ]] \
  || ! grep -Fq "imageName: $snapshot_image" "$OVERLAY"; then
  echo "FAIL: reviewed snapshot image '$snapshot_image' is not the production CAPH selector" >&2
  fail=1
fi

control_plane_infra="$(yq -N 'select(.kind=="KubeadmControlPlane") | .spec.machineTemplate.infrastructureRef.name' "$OVERLAY")"
control_plane_image="$(yq -N "select(.kind==\"HCloudMachineTemplate\" and .metadata.name==\"$control_plane_infra\") | .spec.template.spec.imageName" "$OVERLAY")"
if [ "$control_plane_image" != "$snapshot_image" ]; then
  echo "FAIL: control-plane template '$control_plane_infra' must use reviewed snapshot '$snapshot_image', got '$control_plane_image'" >&2
  fail=1
fi
if grep -q 'workflow_dispatch.inputs' "$SNAPSHOT_WORKFLOW" \
  || grep -q '\${{ inputs\.' "$SNAPSHOT_WORKFLOW"; then
  echo "FAIL: credentialed snapshot selection must come only from $SNAPSHOT_RELEASE, never dispatch input" >&2
  fail=1
fi

# Trimmed packages (w1/m36 t003) — shared by the overlay + packer apt checks.
TRIMMED_PKGS='(\bat\b|\bjq\b|\bunzip\b|\bmtr\b|apt-transport-https)'
# A baked pool must never carry any of these again.
FORBIDDEN="wget .*(runc|containerd)|kubeadm config images pull|pkgs\.k8s\.io|apt.*install.*$TRIMMED_PKGS"
# An ubuntu pool MUST still fetch its runtime at boot (proof it can come up).
BUILDS_RUNTIME='wget .*(runc|containerd)|pkgs\.k8s\.io'

# 1. Per worker MD: image (infraRef) ⟺ preKube (bootstrap.configRef KCT) consistency.
echo "==> $OVERLAY worker image ⟺ preKubeadmCommands consistency"
while read -r md infra cfg; do
  [ -n "$md" ] || continue
  img="$(yq -N "select(.kind==\"HCloudMachineTemplate\" and .metadata.name==\"$infra\") | .spec.template.spec.imageName" "$OVERLAY")"
  prek="$(yq -N "select(.kind==\"KubeadmConfigTemplate\" and .metadata.name==\"$cfg\") | .spec.template.spec.preKubeadmCommands[]" "$OVERLAY")"
  if [ -z "$img" ] || [ "$img" = "null" ]; then
    echo "FAIL: MachineDeployment $md points at HCloudMachineTemplate '$infra' which is not defined in $OVERLAY" >&2; fail=1; continue
  fi
  case "$img" in
    bex-worker-k8s-*)
      if echo "$prek" | grep -qE "$FORBIDDEN"; then
        echo "FAIL: baked pool $md (KubeadmConfigTemplate $cfg) still has a download/pull/trimmed-apt line — the snapshot already carries the runtime:" >&2
        echo "$prek" | grep -nE "$FORBIDDEN" | sed 's/^/    /' >&2; fail=1
      fi
      if ! echo "$prek" | grep -Fq "/usr/local/bin/containerd --version | grep -q ' v$pk_containerd '"; then
        echo "FAIL: baked pool $md does not fail closed when its snapshot containerd differs from v$pk_containerd" >&2; fail=1
      fi
      ;;
    ubuntu-24.04)
      if ! echo "$prek" | grep -qE "$BUILDS_RUNTIME"; then
        echo "FAIL: ubuntu pool $md (KubeadmConfigTemplate $cfg) has imageName ubuntu-24.04 but its preKubeadmCommands no longer download the runtime — nodes would boot with no container runtime. Flip imageName to bex-worker or restore the downloads." >&2; fail=1
      fi
      ;;
    *)
      echo "FAIL: worker pool $md has unexpected imageName '$img' (want a versioned 'bex-worker-k8s-*' snapshot or 'ubuntu-24.04')" >&2; fail=1
      ;;
  esac
done < <(yq -N 'select(.kind=="MachineDeployment") | .metadata.name + " " + .spec.template.spec.infrastructureRef.name + " " + .spec.template.spec.bootstrap.configRef.name' "$OVERLAY")

# containerd 1.x and 2.x use different CRI plugin section names and quote styles,
# and both also emit an unrelated transfer config_path. The bootstrap must scope
# its replacement and verification to the CRI registry section only.
tenant_prek="$(yq -N 'select(.kind=="KubeadmConfigTemplate" and .metadata.name=="bex-tenant-0") | .spec.template.spec.preKubeadmCommands[]' "$OVERLAY")"
if ! echo "$tenant_prek" | grep -Fq '\[plugins\..*cri.*\.registry\]' \
  || ! echo "$tenant_prek" | grep -Fq 's#^([[:space:]]*)config_path =.*#\1config_path = "/etc/containerd/certs.d"#' \
  || ! echo "$tenant_prek" | grep -Fq 'config_path = "/etc/containerd/certs.d"'; then
  echo "FAIL: bex-tenant-0 must configure and verify only containerd's CRI registry config_path" >&2
  fail=1
fi

# containerd.io's package post-install is allowed to start the service before
# cloud-init generates the final CRI config. Every kubeadm bootstrap must use
# restart (not start, which is a no-op for an already-active service) before
# kubeadm validates the CRI RuntimeService.
echo "==> $OVERLAY kubeadm bootstraps restart containerd after config generation"
while read -r kind name; do
  [ -n "$kind" ] || continue
  prek="$(yq -N "select(.kind==\"$kind\" and .metadata.name==\"$name\") | (.spec.kubeadmConfigSpec.preKubeadmCommands // .spec.template.spec.preKubeadmCommands)[]" "$OVERLAY")"
  if ! echo "$prek" | grep -q 'systemctl restart containerd'; then
    echo "FAIL: $kind/$name must restart containerd after writing config.toml so kubeadm sees the CRI plugin" >&2
    fail=1
  fi
  if echo "$prek" | grep -q 'systemctl start containerd'; then
    echo "FAIL: $kind/$name uses 'systemctl start containerd'; an already-active package service would retain its stale non-CRI config" >&2
    fail=1
  fi
done < <(yq -N 'select(.kind=="KubeadmControlPlane" or .kind=="KubeadmConfigTemplate") | .kind + " " + .metadata.name' "$OVERLAY")

# Kubernetes 1.33 removed kube-apiserver's cloud-provider flag. The external
# CCM contract still needs the setting on kube-controller-manager and kubelet,
# but rendering it into the apiserver static Pod makes every new CP CrashLoop.
echo "==> $OVERLAY carries no removed kube-apiserver cloud-provider flag"
api_cloud_provider="$(yq -N 'select(.kind=="KubeadmControlPlane") | .spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs."cloud-provider" // ""' "$OVERLAY")"
if [ -n "$api_cloud_provider" ]; then
  echo "FAIL: kube-apiserver cloud-provider '$api_cloud_provider' is removed in Kubernetes 1.33+" >&2
  fail=1
fi

# 2. Every replacement path must use server capacity fsn1 can still create.
echo "==> $OVERLAY control-plane replacement capacity"
control_plane_type="$(yq -N "select(.kind==\"HCloudMachineTemplate\" and .metadata.name==\"$control_plane_infra\") | .spec.template.spec.type" "$OVERLAY")"
if [ "$control_plane_type" != "cx33" ]; then
  echo "FAIL: control-plane template '$control_plane_infra' uses '$control_plane_type' (want cx33, the cheaper default now that fsn1 stock returned; during an fsn1 cx stock-out flip to cpx32 here and in the overlay — docs/ADR053)" >&2
  fail=1
fi

# Autoscaler pool bounds must preserve platform quorum and route tenant
# scale-out through a server type that fsn1 can still create.
echo "==> $OVERLAY platform autoscaler floor"
platform_min="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-platform") | .metadata.annotations."cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"' "$OVERLAY")"
platform_max="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-platform") | .metadata.annotations."cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"' "$OVERLAY")"
if [ "$platform_min" != "3" ]; then
  echo "FAIL: bex-platform autoscaler min-size is '$platform_min' (want 3 for the three anti-affined OpenBao members)" >&2
  fail=1
fi
if ! [[ "$platform_max" =~ ^[0-9]+$ ]] || (( platform_max < 3 )); then
  echo "FAIL: bex-platform autoscaler max-size is '$platform_max' (want >=3)" >&2
  fail=1
fi

echo "==> $OVERLAY tenant baseline + burst autoscaler split"
tenant_min="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-0") | .metadata.annotations."cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"' "$OVERLAY")"
tenant_max="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-0") | .metadata.annotations."cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"' "$OVERLAY")"
burst_min="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-burst") | .metadata.annotations."cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"' "$OVERLAY")"
burst_max="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-burst") | .metadata.annotations."cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"' "$OVERLAY")"
burst_labels="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-burst") | .metadata.annotations."capacity.cluster-autoscaler.kubernetes.io/labels"' "$OVERLAY")"
burst_infra="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-burst") | .spec.template.spec.infrastructureRef.name' "$OVERLAY")"
burst_type="$(yq -N "select(.kind==\"HCloudMachineTemplate\" and .metadata.name==\"$burst_infra\") | .spec.template.spec.type" "$OVERLAY")"
# min=1 keeps one stable serving node always warm; max=2 (docs/ADR060
# § dedicated build pool, 2026-08-15) lets serving OVERFLOW grow this stable
# pool instead of leaking onto the tainted lg/burst build pools and pinning
# them (observed: single-instance CNPG PDBs kept bex-tenant-burst undrainable
# for 6 days).
if [ "$tenant_min" != "1" ] || [ "$tenant_max" != "2" ]; then
  echo "FAIL: bex-tenant-0 must stay the elastic serving baseline (want min=1 max=2 per docs/ADR060, got min=$tenant_min max=$tenant_max)" >&2
  fail=1
fi
if [ "$burst_min" != "0" ] || [ "$burst_max" != "2" ]; then
  echo "FAIL: bex-tenant-burst must preserve cold scale-out (want min=0 max=2, got min=$burst_min max=$burst_max)" >&2
  fail=1
fi
if [ "$burst_type" != "cx33" ]; then
  echo "FAIL: bex-tenant-burst uses '$burst_type' (want cx33, the cheaper default now that fsn1 stock returned; during an fsn1 cx stock-out flip to cpx32 here and in the overlay — docs/ADR053)" >&2
  fail=1
fi
# The dedicated build pools must boot from the tainted bootstrap template
# (docs/ADR060 § dedicated build pool) — a silent revert to bex-tenant-0
# would let serving pods land on (and pin) elastic build nodes again.
for build_md in bex-tenant-burst bex-tenant-lg; do
  build_cfg="$(yq -N "select(.kind==\"MachineDeployment\" and .metadata.name==\"$build_md\") | .spec.template.spec.bootstrap.configRef.name" "$OVERLAY")"
  build_taint="$(yq -N "select(.kind==\"MachineDeployment\" and .metadata.name==\"$build_md\") | .metadata.annotations.\"capacity.cluster-autoscaler.kubernetes.io/taints\"" "$OVERLAY")"
  if [ "$build_cfg" != "bex-tenant-build" ]; then
    echo "FAIL: $build_md must bootstrap from bex-tenant-build (the build-only taint), got '$build_cfg' (docs/ADR060)" >&2
    fail=1
  fi
  if [ "$build_taint" != "bex.co/build-only=true:NoSchedule" ]; then
    echo "FAIL: $build_md scale-from-zero taints hint is '$build_taint' (want bex.co/build-only=true:NoSchedule, docs/ADR060)" >&2
    fail=1
  fi
done
if [ "$burst_labels" != "bex.co/pool=tenant" ]; then
  echo "FAIL: bex-tenant-burst scale-from-zero labels are '$burst_labels' (want bex.co/pool=tenant)" >&2
  fail=1
fi

# ── Build-pool scheduling floor (w7/m83, docs/ADR060 D5/D8) ──────────────────
# Two invariants that both fail SILENTLY in production, which is why they are
# guarded here rather than trusted:
#
#   1. COMPLETENESS. D8 tainted the build pools on 2026-08-15; the BuildKit
#      prewarm DaemonSet selects bex.co/pool=tenant, which the build nodes still
#      carry, so it kept matching while the taint excluded it. Measured on
#      hetzner-prod 2026-08-17: both prewarm pods on the SERVING nodes, none on
#      the only node running builds. Nothing reported anything.
#
#   2. ARITHMETIC. A cx33 build slot works with ~300 MiB to spare (§4 of
#      .pm/w7/builder-issues.md). If a DaemonSet request grows, another
#      DaemonSet gains the build toleration, or the build request rises, the
#      build pod stops fitting the pool's cluster-autoscaler capacity hint — and
#      CA then declines to scale up WITHOUT emitting an error. Invisible by
#      construction, so the margin gets an explicit floor.
# go_const reads a Go string constant so the guard can never assert a value that
# has drifted from the code it is guarding. The taint, the prewarm image and the
# build memory request are all single-sourced this way.
go_const() { sed -n "s/.*$1 *= *\"\([^\"]*\)\".*/\1/p" "$2" | head -1; }

EXECUTION_SOURCE="${EXECUTION_SOURCE_FILE:-lego/operator/internal/execution/security.go}"
BUILD_TAINT_KEY="$(go_const BuildPoolTaintKey "$EXECUTION_SOURCE")"
BUILD_TAINT_VALUE="$(go_const BuildPoolTaintValue "$EXECUTION_SOURCE")"
# The effect is not a Go constant — execution.TolerateBuildPool spells it with
# the typed corev1.TaintEffectNoSchedule, and the pools' CA hint (checked above)
# already pins the same string.
BUILD_TAINT_EFFECT="NoSchedule"
if [ -z "$BUILD_TAINT_KEY" ] || [ -z "$BUILD_TAINT_VALUE" ]; then
  echo "FAIL: could not read BuildPoolTaintKey/Value from $EXECUTION_SOURCE" >&2
  fail=1
fi

# Measured node allocatable memory (hetzner-prod, 2026-08-17; recorded in
# docs/ADR060 § Measured node arithmetic). Allocatable, not capacity: the
# kubelet's own reservations are already subtracted, and that is the number the
# scheduler actually compares a pod's requests against.
alloc_ki() {
  case "$1" in
    cx33) echo 7834836 ;;
    cx43) echo 15886160 ;;
    *) return 1 ;;
  esac
}

# mem_bytes converts a Kubernetes memory quantity to bytes. Binary and decimal
# suffixes are both in play on purpose — the build request is 7Gi while the CA
# hint is 8G — and conflating them is a 7% error in exactly the direction that
# would hide a shortfall.
mem_bytes() {
  local q="$1" n unit
  n="${q%%[A-Za-z]*}"
  unit="${q#"$n"}"
  case "$unit" in
    "") echo "$n" ;;
    Ki) echo $((n * 1024)) ;;
    Mi) echo $((n * 1024 * 1024)) ;;
    Gi) echo $((n * 1024 * 1024 * 1024)) ;;
    k | K) echo $((n * 1000)) ;;
    M) echo $((n * 1000000)) ;;
    G) echo $((n * 1000000000)) ;;
    *) return 1 ;;
  esac
}

mib() { echo $(($1 / 1024 / 1024)); }

# tolerates_build_taint <file> <yq expression yielding the tolerations array>
# Implements the Kubernetes toleration match: a keyless Exists toleration
# matches every taint, an Exists toleration on the key ignores the value, and an
# empty effect matches every effect.
tolerates_build_taint() {
  local file="$1" expr="$2" n
  n="$(yq -N "[${expr} // [] | .[] | select(
        (((.key // \"\") == \"\" and (.operator // \"Equal\") == \"Exists\") or .key == \"$BUILD_TAINT_KEY\")
        and ((.operator // \"Equal\") == \"Exists\" or (.value // \"\") == \"$BUILD_TAINT_VALUE\")
        and ((.effect // \"\") == \"\" or .effect == \"$BUILD_TAINT_EFFECT\")
      )] | length" "$file" 2>/dev/null)"
  [ "${n:-0}" -gt 0 ]
}

# Every DaemonSet the repository declares, as "<label>|<file>|<tolerations expr>|
# <container requests expr>". Plain manifests are discovered rather than listed
# so a NEW DaemonSet that tolerates the build taint raises the floor by itself —
# that silent-growth path is the whole failure mode being guarded.
# Parallel arrays rather than packed records: every field here is a yq
# expression, and yq expressions are made of pipes.
ds_files=()
ds_tol_exprs=()
ds_req_exprs=()
while IFS= read -r f; do
  [ -n "$f" ] || continue
  ds_files+=("$f")
  ds_tol_exprs+=("select(.kind == \"DaemonSet\") | .spec.template.spec.tolerations")
  ds_req_exprs+=("[select(.kind == \"DaemonSet\") | .spec.template.spec.containers[].resources.requests.memory // \"0\"]")
done < <(grep -rl "^kind: DaemonSet" "$GITOPS_BASE" "$OPERATOR_CONFIG" 2>/dev/null | sort)
# Chart-backed DaemonSets carry the same weight but express it in an Argo
# Application's Helm values, so they are discovered by controller.type instead.
while IFS= read -r f; do
  [ -n "$f" ] || continue
  ds_files+=("$f")
  ds_tol_exprs+=(".spec.source.helm.values | from_yaml | .controller.tolerations")
  # The chart's own main-container request PLUS the requests it adds that this
  # repository cannot express: the Alloy chart runs a config-reloader sidecar,
  # measured at 50Mi on hetzner-prod 2026-08-17 (log-shipper's node total was
  # 128Mi + 50Mi = 178Mi). Carried with the chart rather than added globally, so
  # it disappears together with the chart if the DaemonSet ever stops tolerating
  # the build taint.
  ds_req_exprs+=("[.spec.source.helm.values | from_yaml | .alloy.resources.requests.memory // \"0\", \"50Mi\"]")
done < <(yq -N 'select((.spec.source.helm.values // "x") | from_yaml | (.controller.type // "") == "daemonset") | filename' "$GITOPS_BASE"/*.yaml 2>/dev/null | sort -u)

echo "==> build pools: prewarm reaches them and the CA hint clears the scheduling floor"
ds_floor=0
tolerating=()
for i in "${!ds_files[@]}"; do
  file="${ds_files[$i]}"
  tolerates_build_taint "$file" "${ds_tol_exprs[$i]}" || continue
  tolerating+=("$(basename "$file" .yaml)")
  while read -r q; do
    [ -n "$q" ] || continue
    if ! bytes="$(mem_bytes "$q")"; then
      echo "FAIL: $file declares an unparseable memory request '$q'" >&2
      fail=1
      continue
    fi
    ds_floor=$((ds_floor + bytes))
  done < <(yq -N "${ds_req_exprs[$i]} | .[]" "$file" 2>/dev/null)
done

# The prewarm DaemonSet is not merely allowed on build nodes — it is REQUIRED
# there, because a cold build node is the only case it exists for.
prewarm="$GITOPS_BASE/build-image-prewarm.yaml"
if [ ! -f "$prewarm" ]; then
  echo "FAIL: $prewarm missing — the BuildKit prewarm DaemonSet must stay in the repo (w1/030)" >&2
  fail=1
elif ! tolerates_build_taint "$prewarm" ".spec.template.spec.tolerations"; then
  echo "FAIL: build-image-prewarm does not tolerate $BUILD_TAINT_KEY=$BUILD_TAINT_VALUE:$BUILD_TAINT_EFFECT," >&2
  echo "      so it is scheduled on the SERVING pool and never on the nodes that run builds" >&2
  echo "      (measured on hetzner-prod 2026-08-17; docs/ADR060 D8)" >&2
  fail=1
fi
# A drifted ref warms the wrong image, which looks identical to warming nothing.
if [ -f "$prewarm" ] && [ -f "$BUILD_SOURCE" ]; then
  prewarm_image="$(yq -N '.spec.template.spec.containers[] | select(.name == "buildkit") | .image' "$prewarm")"
  builder_image="$(go_const defaultBuildkitImage "$BUILD_SOURCE")"
  if [ -z "$builder_image" ]; then
    echo "FAIL: could not read defaultBuildkitImage from $BUILD_SOURCE" >&2
    fail=1
  elif [ "$prewarm_image" != "$builder_image" ]; then
    echo "FAIL: build-image-prewarm warms '$prewarm_image' but builds run '$builder_image'" >&2
    fail=1
  fi
fi

build_request="$(go_const buildMemoryRequest "$BUILD_SOURCE")"
if [ -z "$build_request" ] || ! build_req_bytes="$(mem_bytes "$build_request")"; then
  echo "FAIL: could not read buildMemoryRequest from $BUILD_SOURCE — the floor must be computed from the real request, never a copied literal" >&2
  fail=1
  build_req_bytes=0
fi
floor=$((build_req_bytes + ds_floor))
echo "    build request $build_request = $(mib "$build_req_bytes")Mi + DaemonSet floor $(mib "$ds_floor")Mi [${tolerating[*]:-none}] = $(mib "$floor")Mi"

build_pool_count=0
while read -r md; do
  [ -n "$md" ] || continue
  build_pool_count=$((build_pool_count + 1))
  hint="$(yq -N "select(.kind==\"MachineDeployment\" and .metadata.name==\"$md\") | .metadata.annotations.\"capacity.cluster-autoscaler.kubernetes.io/memory\"" "$OVERLAY")"
  infra="$(yq -N "select(.kind==\"MachineDeployment\" and .metadata.name==\"$md\") | .spec.template.spec.infrastructureRef.name" "$OVERLAY")"
  server_type="$(yq -N "select(.kind==\"HCloudMachineTemplate\" and .metadata.name==\"$infra\") | .spec.template.spec.type" "$OVERLAY")"
  if [ -z "$hint" ] || ! hint_bytes="$(mem_bytes "$hint")"; then
    echo "FAIL: $md has no readable capacity.cluster-autoscaler.kubernetes.io/memory hint (got '$hint')" >&2
    fail=1
    continue
  fi
  if ! alloc="$(alloc_ki "$server_type")"; then
    echo "FAIL: $md runs unmeasured server type '$server_type' — add its measured allocatable to alloc_ki() and docs/ADR060 before shipping it as a build pool" >&2
    fail=1
    continue
  fi
  alloc_bytes=$((alloc * 1024))
  if ((hint_bytes < floor)); then
    echo "FAIL: $md ($server_type) CA memory hint $hint = $(mib "$hint_bytes")Mi is BELOW the $(mib "$floor")Mi scheduling floor" >&2
    echo "      (build request $(mib "$build_req_bytes")Mi + DaemonSet floor $(mib "$ds_floor")Mi); short by $(mib $((floor - hint_bytes)))Mi." >&2
    echo "      cluster-autoscaler would silently decline to scale this pool up — no error, no event (docs/ADR060 D8)" >&2
    fail=1
  fi
  if ((alloc_bytes < floor)); then
    echo "FAIL: $md ($server_type) real allocatable $(mib "$alloc_bytes")Mi is BELOW the $(mib "$floor")Mi floor; a build cannot be placed on this pool at all" >&2
    fail=1
  fi
  # Simulation must stay CONSERVATIVE: the hint is what CA believes a not-yet-
  # existing node offers, so a hint above real allocatable makes it scale up for
  # pods that will then sit Pending on the node it just created.
  if ((hint_bytes > alloc_bytes)); then
    echo "FAIL: $md ($server_type) CA hint $(mib "$hint_bytes")Mi EXCEEDS measured allocatable $(mib "$alloc_bytes")Mi — scale-from-zero simulation must stay conservative" >&2
    fail=1
  fi
done < <(yq -N "select(.kind==\"MachineDeployment\") | select((.metadata.annotations.\"capacity.cluster-autoscaler.kubernetes.io/taints\" // \"\") | test(\"$BUILD_TAINT_KEY\")) | .metadata.name" "$OVERLAY")

if [ "$build_pool_count" -eq 0 ]; then
  echo "FAIL: no MachineDeployment registers the $BUILD_TAINT_KEY taint — the dedicated build pool (docs/ADR060 D8) is gone" >&2
  fail=1
fi

# D8's headline claim: bex-tenant-lg is the always-warm tier with TWO 7Gi slots.
# It is arithmetic, so it is guarded rather than asserted in prose.
lg_infra="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-lg") | .spec.template.spec.infrastructureRef.name' "$OVERLAY")"
lg_type="$(yq -N "select(.kind==\"HCloudMachineTemplate\" and .metadata.name==\"$lg_infra\") | .spec.template.spec.type" "$OVERLAY")"
if lg_alloc="$(alloc_ki "$lg_type")"; then
  lg_two=$((2 * build_req_bytes + ds_floor))
  if ((lg_alloc * 1024 < lg_two)); then
    echo "FAIL: bex-tenant-lg ($lg_type) allocatable $(mib $((lg_alloc * 1024)))Mi no longer fits TWO builds + the $(mib "$ds_floor")Mi DaemonSet floor ($(mib "$lg_two")Mi) — D8's two warm slots are gone" >&2
    fail=1
  fi
fi


# A Cilium node starts with agent-not-ready:NoSchedule and loses it only after
# the agent converges. Without this startup-taint declaration, CA copies the
# transient taint into its zero-sized node template and concludes that no
# Pending build Pod could fit, permanently defeating cold scale-out.
echo "==> autoscaler version + Cilium startup-taint contract"
autoscaler_startup_taint="$(yq -N '.extraArgs.startup-taint' "$AUTOSCALER_VALUES")"
autoscaler_tag="$(yq -N '.spec.sources[0].helm.parameters[] | select(.name == "image.tag") | .value' "$AUTOSCALER_APP")"
overlay_minor="$(yq -N 'select(.kind=="MachineDeployment" and .metadata.name=="bex-tenant-burst") | .spec.template.spec.version' "$OVERLAY" | sed -E 's/^(v[0-9]+\.[0-9]+).*/\1/')"
autoscaler_minor="$(printf '%s' "$autoscaler_tag" | sed -E 's/^(v[0-9]+\.[0-9]+).*/\1/')"
if [ "$autoscaler_startup_taint" != "node.cilium.io/agent-not-ready" ]; then
  echo "FAIL: autoscaler startup-taint is '$autoscaler_startup_taint' (want node.cilium.io/agent-not-ready for cold Cilium nodes)" >&2
  fail=1
fi
if [ -z "$autoscaler_tag" ] || [ "$autoscaler_minor" != "$overlay_minor" ]; then
  echo "FAIL: autoscaler image '$autoscaler_tag' must match workload Kubernetes minor '$overlay_minor'" >&2
  fail=1
fi

# 3. The bake must not reintroduce a trimmed package.
echo "==> $PACKER installs no trimmed apt package"
if [ -f "$PACKER" ]; then
  bad="$(grep -nE "apt-get.*install.*$TRIMMED_PKGS" "$PACKER" || true)"
  if [ -n "$bad" ]; then
    echo "FAIL: $PACKER installs a trimmed package (at/jq/unzip/mtr/apt-transport-https):" >&2
    echo "$bad" | sed 's/^/    /' >&2
    fail=1
  fi
else
  echo "FAIL: $PACKER missing — the baked-image recipe must stay in the repo (w1/m36 t001)" >&2; fail=1
fi

# 3b. CAPH requires imageName to resolve to exactly one snapshot. A bake must
# preserve old snapshots for rollback while retiring their active selector.
echo "==> $SNAPSHOT_WORKFLOW retires prior active worker snapshots"
if ! grep -Fq '"caph-image-name=$IMAGE_NAME"' "$SNAPSHOT_WORKFLOW" \
  || ! grep -Fq '"caph-image-name":$retired' "$SNAPSHOT_WORKFLOW" \
  || ! grep -Fq 'test "$active" -eq 1' "$SNAPSHOT_WORKFLOW"; then
  echo "FAIL: snapshot workflow must retire older snapshots for the requested image label and verify exactly one active image" >&2
  fail=1
fi

# 4. The bake pins and the overlay must not drift.
echo "==> $PACKER version pins match the overlay"
if [ -f "$PACKER" ]; then
  ov_k8s="$(yq -N 'select(.kind=="KubeadmControlPlane") | .spec.version' "$OVERLAY" | sed 's/^v//')"
  ov_containerd="$(grep -oE 'CONTAINERD=[0-9][0-9.]*' "$OVERLAY" | head -1 | cut -d= -f2)"
  ov_runc="$(grep -oE 'RUNC=[0-9][0-9.]*' "$OVERLAY" | head -1 | cut -d= -f2)"
  for pair in "kubernetes:$pk_k8s:$ov_k8s" "containerd:$pk_containerd:$ov_containerd" "runc:$pk_runc:$ov_runc"; do
    name="${pair%%:*}"; rest="${pair#*:}"; pk="${rest%%:*}"; ov="${rest##*:}"
    if [ -z "$pk" ] || [ -z "$ov" ]; then
      echo "FAIL: could not read $name version (packer='$pk' overlay='$ov')" >&2; fail=1
    elif [ "$pk" != "$ov" ]; then
      echo "FAIL: $name version drift — packer '$pk' != overlay '$ov'; bump both (infra/packer/README.md § Bumping)" >&2; fail=1
    fi
  done

  echo "==> node-root artifacts use repository-reviewed trust anchors"
  for pair in \
    "containerd:$pk_containerd_sha" \
    "runc:$pk_runc_sha" \
    "kubernetes-key:$pk_k8s_key_sha"; do
    name="${pair%%:*}"; digest="${pair##*:}"
    if [[ ! "$digest" =~ ^[0-9a-f]{64}$ ]]; then
      echo "FAIL: $name trust anchor is not a repository-pinned SHA-256" >&2
      fail=1
    fi
  done
  if grep -Eq '(runc\.sha256sum|\.tar\.gz\.sha256sum)' "$PACKER" "$OVERLAY"; then
    echo "FAIL: node bootstrap must not co-fetch runtime checksum files" >&2
    fail=1
  fi
  if [ "$(pk_var base_image)" != "161547269" ]; then
    echo "FAIL: Packer base image must use the reviewed immutable Hetzner image id" >&2
    fail=1
  fi
  control_plane_prek="$(yq -N 'select(.kind=="KubeadmControlPlane") | .spec.kubeadmConfigSpec.preKubeadmCommands[]' "$OVERLAY")"
  if echo "$control_plane_prek" | grep -Eq 'github\.com/(opencontainers|containerd)|pkgs\.k8s\.io|apt-get (update|install)|kubeadm config images pull'; then
    echo "FAIL: control-plane bootstrap must consume the reviewed snapshot, not download privileged node artifacts" >&2
    fail=1
  fi
  for expected in \
    "/usr/local/bin/containerd --version | grep -q \" v\$CONTAINERD \"" \
    "/usr/local/sbin/runc --version | head -1 | grep -qx \"runc version \$RUNC\"" \
    "kubeadm version -o short | grep -qx \"v\$KUBERNETES_VERSION\""; do
    if ! grep -Fq "$expected" <<<"$control_plane_prek"; then
      echo "FAIL: control-plane bootstrap lacks baked-version assertion: $expected" >&2
      fail=1
    fi
  done
fi

[ "$fail" -eq 0 ] && echo "PASS: CAPH overlay is internally consistent (reviewed snapshot ⟺ no privileged bootstrap downloads)" || { echo "FAIL: see errors above" >&2; exit 1; }
