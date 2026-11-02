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
#   6. the dedicated serving pools (w2/m250) route every compute tier to a
#      pool it fits, stay tainted and cold at zero, and bootstrap in sync
#      with bex-tenant-0 (as the build pools' template must too).
#   7. the sandbox pool's gVisor runtime (codex-security round 18, CWE-494) is
#      verified against SHA-512 digests pinned in this repository — never a
#      checksum co-fetched from the release origin — and fails closed on any
#      non-x86_64 architecture.
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
SANDBOX_OVERLAY="${SANDBOX_OVERLAY_FILE:-infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml}"
PACKER="${PACKER_FILE:-infra/packer/bex-worker.pkr.hcl}"
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

# Tenant processes bind ports below 1024 without NET_BIND_SERVICE: containerd
# 2.x's built-in CRI default enables unprivileged ports, setting
# net.ipv4.ip_unprivileged_port_start=0 inside every pod, as on Render (1.7's
# default turns it off). bex-api accepts such ports (w6/m149,
# docs/render-artifacts/container-privileges.md), so the baked containerd must
# stay on 2.x and no bootstrap may override the setting (checked per template
# in the restart loop below), or every service on :80 crash-loops.
echo "==> $PACKER bakes a containerd whose CRI default enables unprivileged ports"
if [ -z "$pk_containerd" ] || [ "${pk_containerd%%.*}" -lt 2 ]; then
  echo "FAIL: packer containerd_version '$pk_containerd' is not 2.x; containerd before 2.0 disables unprivileged ports by default" >&2
  fail=1
fi

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
  if echo "$prek" | grep -q 'enable_unprivileged_ports'; then
    echo "FAIL: $kind/$name overrides containerd's enable_unprivileged_ports; tenant services bind ports below 1024 through its default" >&2
    fail=1
  fi
  # w5/171: tenants may pull from any public registry, so containerd itself
  # must never reach the instance-metadata service from the node network.
  if ! echo "$prek" | grep -q 'socket cgroupv2 level 2 "system.slice/containerd.service" ip daddr 169.254.0.0/16 counter reject' \
    || ! echo "$prek" | grep -q 'ExecStartPost=/usr/sbin/nft -f /etc/bex-containerd-egress.nft' \
    || ! echo "$prek" | grep -q "nft list table inet bex_containerd_egress"; then
    echo "FAIL: $kind/$name must deny containerd's cgroup link-local egress (w5/171) and verify the table after restarting containerd" >&2
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
# min=1 keeps one stable serving node warm; max=8 leaves room for rollout
# surge, rescheduling once the serving nodes fill up, and high-availability
# Postgres pairs on distinct nodes (docs/ADR060 § dedicated build pool; raised
# 3 -> 5 on 2026-10-24, 5 -> 8 on 2026-10-29 for w2/m171). Serving workloads
# still cannot use the tainted build pools.
if [ "$tenant_min" != "1" ] || [ "$tenant_max" != "8" ]; then
  echo "FAIL: bex-tenant-0 must retain serving rollout capacity (want min=1 max=8 per docs/ADR060, got min=$tenant_min max=$tenant_max)" >&2
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
# cx53 and ccx43 (the w2/m250 serving pools) are ESTIMATES until their first
# node exists: the cx33→cx43 measurements are linear in nominal GB (~1006415Ki
# per GB, -216488Ki), which predicts ~31988808Ki (cx53, 32 GB) and ~64194104Ki
# (ccx43, 64 GB); both are taken ~2% lower. Replace them with the measured
# allocatable at w2/m250 t003.
alloc_ki() {
  case "$1" in
    cx33) echo 7834836 ;;
    cx43) echo 15886160 ;;
    cx53) echo 31300000 ;;  # estimate, see above
    ccx43) echo 62900000 ;; # estimate, see above
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

# tolerates_taint <file> <yq expression yielding the tolerations array> <key>
# <value> <effect>
# Implements the Kubernetes toleration match: a keyless Exists toleration
# matches every taint, an Exists toleration on the key ignores the value, and an
# empty effect matches every effect.
tolerates_taint() {
  local file="$1" expr="$2" key="$3" value="$4" effect="$5" n
  n="$(yq -N "[${expr} // [] | .[] | select(
        (((.key // \"\") == \"\" and (.operator // \"Equal\") == \"Exists\") or .key == \"$key\")
        and ((.operator // \"Equal\") == \"Exists\" or (.value // \"\") == \"$value\")
        and ((.effect // \"\") == \"\" or .effect == \"$effect\")
      )] | length" "$file" 2>/dev/null)"
  [ "${n:-0}" -gt 0 ]
}
tolerates_build_taint() { tolerates_taint "$1" "$2" "$BUILD_TAINT_KEY" "$BUILD_TAINT_VALUE" "$BUILD_TAINT_EFFECT"; }

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

# ── Dedicated serving pools (w2/m250) ────────────────────────────────────────
# The compute tiers no bex-tenant-0 node can hold run on their own pools
# (lego/types/tiers/tiers.yaml servingPool → bex.co/serving-pool=<pool>). Every
# way this breaks is silent: a pool that loses its taint fills with small
# tiers and is pinned above zero; a pool that loses min 0 is billed forever; a
# tier that outgrows its pool — or one left on bex-tenant-0 that never fit
# it — sits Pending while cluster-autoscaler declines to scale, with no error.
# So the routing, the bootstrap and the arithmetic are all guarded here.
TIERS_FILE="${TIERS_FILE:-lego/types/tiers/tiers.yaml}"
SERVING_KEY="$(go_const ServingPoolKey "$EXECUTION_SOURCE")"
if [ -z "$SERVING_KEY" ]; then
  echo "FAIL: could not read ServingPoolKey from $EXECUTION_SOURCE" >&2
  fail=1
fi

# The user-approved autoscaler ceiling per pool (2026-11-02, .pm/w2/m250). A
# new pool, or a new ceiling, is a spend decision: record it here with it.
serving_pool_max() {
  case "$1" in
    xl) echo 4 ;;
    xxl) echo 2 ;;
    *) return 1 ;;
  esac
}

# vCPUs per Hetzner server type (Hetzner API /server_types). Allocatable CPU
# equals capacity: the kubelet reserves none (no system/kube-reserved flag).
server_vcpus() {
  case "$1" in
    cx33 | cpx32) echo 4 ;;
    cx43) echo 8 ;;
    cx53 | ccx43) echo 16 ;;
    *) return 1 ;;
  esac
}

# cpu_milli converts a Kubernetes CPU quantity to millicores.
cpu_milli() {
  local q="$1" whole frac
  case "$q" in
    *m) echo "${q%m}" ;;
    *.*)
      whole="${q%%.*}"; frac="${q#*.}000"; frac="${frac:0:3}"
      echo $((10#${whole:-0} * 1000 + 10#$frac)) ;;
    *[!0-9]* | "") return 1 ;;
    *) echo $((q * 1000)) ;;
  esac
}

# daemonset_floor <taint key> <taint value>: the memory (bytes) and CPU
# (millicores) every DaemonSet that lands on a tenant node carrying that taint
# requests — "" for an untainted node, where every tenant-eligible DaemonSet
# lands. A DaemonSet pinned to another pool (bex.co/pool != tenant) never runs
# there. Sets ds_mem, ds_cpu and ds_names.
ds_pool_exprs=()
for i in "${!ds_files[@]}"; do
  case "${ds_tol_exprs[$i]}" in
    .spec.source.helm.values*) ds_pool_exprs+=('[select(.spec.source.helm.values != null) | .spec.source.helm.values | from_yaml | .controller.nodeSelector."bex.co/pool"] | .[0] // "tenant"') ;;
    *) ds_pool_exprs+=('[select(.kind == "DaemonSet") | .spec.template.spec.nodeSelector."bex.co/pool"] | .[0] // "tenant"') ;;
  esac
done
daemonset_floor() {
  local key="$1" value="$2" i file q bytes milli cpu_expr name
  ds_mem=0 ds_cpu=0 ds_names=()
  for i in "${!ds_files[@]}"; do
    file="${ds_files[$i]}"
    # eval-all with the collector outside the select: one answer per file on
    # both yq 4.44 and 4.5x (see scripts/gitops-validate.sh on the `//` trap).
    [ "$(yq -N ea "${ds_pool_exprs[$i]}" "$file" 2>/dev/null)" = tenant ] || continue
    if [ -n "$key" ] && ! tolerates_taint "$file" "${ds_tol_exprs[$i]}" "$key" "$value" NoSchedule; then
      continue
    fi
    name="$(basename "$file" .yaml)"; [ "$name" != daemonset ] || name="$(basename "$(dirname "$file")")"
    ds_names+=("$name")
    while read -r q; do
      [ -n "$q" ] || continue
      if bytes="$(mem_bytes "$q")"; then ds_mem=$((ds_mem + bytes)); else
        echo "FAIL: $file declares an unparseable memory request '$q'" >&2; fail=1
      fi
    done < <(yq -N "${ds_req_exprs[$i]} | .[]" "$file" 2>/dev/null)
    # CPU mirrors the memory expressions; the chart sidecar allowance is the
    # Alloy chart's config-reloader request (10m, chart default — not measured).
    cpu_expr="${ds_req_exprs[$i]//.memory/.cpu}"
    cpu_expr="${cpu_expr//\"50Mi\"/\"10m\"}"
    while read -r q; do
      [ -n "$q" ] || continue
      if milli="$(cpu_milli "$q")"; then ds_cpu=$((ds_cpu + milli)); else
        echo "FAIL: $file declares an unparseable cpu request '$q'" >&2; fail=1
      fi
    done < <(yq -N "$cpu_expr | .[]" "$file" 2>/dev/null)
  done
}

# Single-object reads use eval-all with the collector outside the select, so
# they yield exactly one value on both yq 4.44 (CI) and 4.5x — a bare
# `select(...) | .x // ""` prints one line PER DOCUMENT on 4.44 (see
# scripts/gitops-validate.sh).
overlay_one() { yq -N ea "[select(.kind==\"$1\" and .metadata.name==\"$2\") | $3] | .[0] // ${4:-\"\"}" "$OVERLAY"; }
md_field() { overlay_one MachineDeployment "$1" "$2"; }
md_hint() { md_field "$1" ".metadata.annotations.\"$2\""; }

# tier_fits <id> <cpu> <mem> <disk> <where> <md> <server type>: the tier's pod plus the
# DaemonSet floor (ds_mem/ds_cpu, set by the caller) must fit what the CA
# capacity hint promises AND what the node really allocates.
tier_fits() {
  local id="$1" cpu="$2" mem="$3" disk="$4" where="$5" md="$6" type="$7"
  local cpu_hint mem_hint disk_hint alloc vcpus need_mem need_cpu hint_bytes cpu_m mem_b disk_b
  cpu_hint="$(md_hint "$md" capacity.cluster-autoscaler.kubernetes.io/cpu)"
  mem_hint="$(md_hint "$md" capacity.cluster-autoscaler.kubernetes.io/memory)"
  if ! cpu_m="$(cpu_milli "$cpu")" || ! mem_b="$(mem_bytes "$mem")"; then
    echo "FAIL: tier $id has an unparseable cpu '$cpu' or memory '$mem' in $TIERS_FILE" >&2; fail=1; return
  fi
  if ! alloc="$(alloc_ki "$type")" || ! vcpus="$(server_vcpus "$type")"; then
    echo "FAIL: $md runs unmeasured server type '$type' — add it to alloc_ki()/server_vcpus() before routing tier $id there" >&2; fail=1; return
  fi
  if ! hint_bytes="$(mem_bytes "$mem_hint")" || ! [[ "$cpu_hint" =~ ^[0-9]+$ ]]; then
    echo "FAIL: $md has no readable cpu/memory capacity hint (cpu='$cpu_hint' memory='$mem_hint')" >&2; fail=1; return
  fi
  need_mem=$((mem_b + ds_mem)); need_cpu=$((cpu_m + ds_cpu))
  if ((need_mem > hint_bytes || need_mem > alloc * 1024 || need_cpu > cpu_hint * 1000 || need_cpu > vcpus * 1000)); then
    echo "FAIL: tier $id ($cpu CPU / $mem) no longer fits $where $md ($type): it needs $need_cpu""m / $(mib "$need_mem")Mi with the DaemonSet floor [${ds_names[*]:-none}]," >&2
    echo "      the CA hint offers ${cpu_hint}000m / $(mib "$hint_bytes")Mi and the node allocates $((vcpus * 1000))m / $(mib $((alloc * 1024)))Mi — it would sit Pending with no scale-up (w2/m250)" >&2
    fail=1
  fi
  if [ -n "$disk" ]; then
    disk_hint="$(md_hint "$md" capacity.cluster-autoscaler.kubernetes.io/ephemeral-disk)"
    if ! disk_b="$(mem_bytes "$disk")" || [ -z "$disk_hint" ] || ! hint_bytes="$(mem_bytes "$disk_hint")" || ((disk_b > hint_bytes)); then
      echo "FAIL: tier $id requests $disk ephemeral-storage but $md's CA ephemeral-disk hint is '$disk_hint' — a scale-from-zero template node without room for it fits no pod (w2/m250)" >&2
      fail=1
    fi
  fi
}

# kct_core <name>: a KubeadmConfigTemplate with the per-pool divergences (taints,
# node-labels) removed, as canonical JSON — what must equal bex-tenant-0's.
kct_core() {
  yq -o=json -I=0 -N "select(.kind==\"KubeadmConfigTemplate\" and .metadata.name==\"$1\") | .spec
    | del(.template.spec.joinConfiguration.nodeRegistration.taints)
    | del(.template.spec.joinConfiguration.nodeRegistration.kubeletExtraArgs.\"node-labels\")" "$OVERLAY"
}
tenant_core="$(kct_core bex-tenant-0)"

echo "==> serving pools: every tier lands on a pool it fits, each pool tainted, cold at zero, and in sync with bex-tenant-0"
serving_pools="$(yq -N '[.compute.tiers[] | .servingPool // ""] | unique | .[] | select(. != "")' "$TIERS_FILE")"
for pool in $serving_pools; do
  want_label="$SERVING_KEY=$pool"
  mds="$(yq -N "select(.kind==\"MachineDeployment\") | select((.metadata.annotations.\"capacity.cluster-autoscaler.kubernetes.io/labels\" // \"\") | split(\",\") | contains([\"$want_label\"])) | .metadata.name" "$OVERLAY")"
  if [ "$(printf '%s\n' "$mds" | grep -c .)" -ne 1 ]; then
    echo "FAIL: tiers.yaml routes tiers to serving pool '$pool', but $(printf '%s\n' "$mds" | grep -c .) MachineDeployments advertise $want_label (want exactly one) — those tiers would never schedule" >&2
    fail=1
    continue
  fi
  md="$mds"
  min="$(md_hint "$md" cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size)"
  max="$(md_hint "$md" cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size)"
  if [ "$min" != "0" ]; then
    echo "FAIL: serving pool $md min-size is '$min' (want 0 — a node is billed only while a $pool-tier service runs, w2/m250)" >&2
    fail=1
  fi
  if ! want_max="$(serving_pool_max "$pool")"; then
    echo "FAIL: serving pool '$pool' ($md) has no user-approved max in serving_pool_max() — a new pool is a spend decision (w2/m250)" >&2
    fail=1
  elif [ "$max" != "$want_max" ]; then
    echo "FAIL: serving pool $md max-size is '$max' (want $want_max, the user-approved ceiling, w2/m250)" >&2
    fail=1
  fi
  taint_hint="$(md_hint "$md" capacity.cluster-autoscaler.kubernetes.io/taints)"
  if [ "$taint_hint" != "$SERVING_KEY=$pool:NoSchedule" ]; then
    echo "FAIL: serving pool $md scale-from-zero taints hint is '$taint_hint' (want $SERVING_KEY=$pool:NoSchedule)" >&2
    fail=1
  fi
  labels_hint="$(md_hint "$md" capacity.cluster-autoscaler.kubernetes.io/labels)"
  if [ "$labels_hint" != "bex.co/pool=tenant,$want_label" ]; then
    echo "FAIL: serving pool $md scale-from-zero labels hint is '$labels_hint' (want bex.co/pool=tenant,$want_label)" >&2
    fail=1
  fi
  # The bootstrap registers the same taint + label at kubelet join; it is the
  # real node, the hints above are only CA's picture of it.
  cfg="$(md_field "$md" .spec.template.spec.bootstrap.configRef.name)"
  kct_taints="$(yq -o=json -I=0 -N ea "[select(.kind==\"KubeadmConfigTemplate\" and .metadata.name==\"$cfg\") | .spec.template.spec.joinConfiguration.nodeRegistration.taints] | .[0] // []" "$OVERLAY")"
  kct_labels="$(overlay_one KubeadmConfigTemplate "$cfg" '.spec.template.spec.joinConfiguration.nodeRegistration.kubeletExtraArgs."node-labels"')"
  if [ "$kct_taints" != "[{\"key\":\"$SERVING_KEY\",\"value\":\"$pool\",\"effect\":\"NoSchedule\"}]" ]; then
    echo "FAIL: $md bootstraps from KubeadmConfigTemplate '$cfg' whose registered taints are $kct_taints (want exactly $SERVING_KEY=$pool:NoSchedule) — without it every tier, build and datastore lands on, and pins, this pool" >&2
    fail=1
  fi
  if [ "$kct_labels" != "bex.co/pool=tenant,$want_label" ]; then
    echo "FAIL: KubeadmConfigTemplate '$cfg' node-labels are '$kct_labels' (want bex.co/pool=tenant,$want_label — the label the tiers' node affinity requires)" >&2
    fail=1
  fi
  if [ -z "$tenant_core" ] || [ "$(kct_core "$cfg")" != "$tenant_core" ]; then
    echo "FAIL: KubeadmConfigTemplate '$cfg' ($md) differs from bex-tenant-0 in more than its taint and node-labels — keep files/preKubeadmCommands/kubeletExtraArgs in sync" >&2
    fail=1
  fi
  infra="$(md_field "$md" .spec.template.spec.infrastructureRef.name)"
  type="$(overlay_one HCloudMachineTemplate "$infra" .spec.template.spec.type)"
  # Own spread group: Hetzner caps one at 10 servers, and tenant-0's pools
  # alone can already reach 11 (w2/m250).
  pg="$(overlay_one HCloudMachineTemplate "$infra" .spec.template.spec.placementGroupName)"
  if [ "$pg" != "tenant-large" ]; then
    echo "FAIL: serving pool $md ($infra) uses placement group '$pg' (want tenant-large: a shared spread group would pass Hetzner's 10-server cap)" >&2
    fail=1
  elif ! yq -e -N 'select(.kind=="HetznerCluster") | .spec.hcloudPlacementGroups[] | select(.name=="tenant-large" and .type=="spread")' "$OVERLAY" >/dev/null 2>&1; then
    echo "FAIL: HetznerCluster does not declare the tenant-large spread placement group that $md uses" >&2
    fail=1
  fi
  mem_hint="$(md_hint "$md" capacity.cluster-autoscaler.kubernetes.io/memory)"
  if alloc="$(alloc_ki "$type")" && hint_bytes="$(mem_bytes "$mem_hint")" && ((hint_bytes > alloc * 1024)); then
    echo "FAIL: $md ($type) CA memory hint $mem_hint EXCEEDS its allocatable $(mib $((alloc * 1024)))Mi — scale-from-zero simulation must stay conservative" >&2
    fail=1
  fi
  daemonset_floor "$SERVING_KEY" "$pool"
  echo "    $md ($type): DaemonSet floor $(mib "$ds_mem")Mi / ${ds_cpu}m [${ds_names[*]:-none}]"
  while read -r id cpu mem disk; do
    [ -n "$id" ] || continue
    tier_fits "$id" "$cpu" "$mem" "$disk" "its serving pool" "$md" "$type"
  done < <(yq -N ".compute.tiers[] | select(.servingPool == \"$pool\") | .id + \" \" + .cpu + \" \" + .memory + \" \" + .ephemeralStorage" "$TIERS_FILE")
done

# A pool no tier names would only be idle spend headroom; a serving-tainted
# pool a tier does name but no label advertises is caught above.
while read -r md; do
  [ -n "$md" ] || continue
  pool="$(md_hint "$md" capacity.cluster-autoscaler.kubernetes.io/taints | sed -n "s#^$SERVING_KEY=\([^:]*\):NoSchedule\$#\1#p")"
  if [ -z "$pool" ] || ! printf '%s\n' $serving_pools | grep -qx "$pool"; then
    echo "FAIL: $md carries the $SERVING_KEY taint but no tier in $TIERS_FILE routes to its pool ('$pool')" >&2
    fail=1
  fi
done < <(yq -N "select(.kind==\"MachineDeployment\") | select((.metadata.annotations.\"capacity.cluster-autoscaler.kubernetes.io/taints\" // \"\") | test(\"$SERVING_KEY\")) | .metadata.name" "$OVERLAY")

# Every other tier stays on bex-tenant-0, so it must fit one: a new or grown
# tier that no longer does needs a servingPool, not a Pending service.
tenant0_infra="$(md_field bex-tenant-0 .spec.template.spec.infrastructureRef.name)"
tenant0_type="$(overlay_one HCloudMachineTemplate "$tenant0_infra" .spec.template.spec.type)"
daemonset_floor "" ""
echo "    bex-tenant-0 ($tenant0_type): DaemonSet floor $(mib "$ds_mem")Mi / ${ds_cpu}m [${ds_names[*]:-none}]"
while read -r id cpu mem disk; do
  [ -n "$id" ] || continue
  tier_fits "$id" "$cpu" "$mem" "" "the standard serving pool" bex-tenant-0 "$tenant0_type"
done < <(yq -N '.compute.tiers[] | select((.servingPool // "") == "") | .id + " " + .cpu + " " + .memory + " " + .ephemeralStorage' "$TIERS_FILE")

# The build pools' bootstrap is held to the same sync contract.
if [ -z "$tenant_core" ] || [ "$(kct_core bex-tenant-build)" != "$tenant_core" ]; then
  echo "FAIL: KubeadmConfigTemplate bex-tenant-build differs from bex-tenant-0 in more than its taint and node-labels — keep files/preKubeadmCommands/kubeletExtraArgs in sync (docs/ADR060)" >&2
  fail=1
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

# bex-drain-guard hands single-instance Apps off a node during the delay CA
# sleeps between its ToBeDeletedByClusterAutoscaler taint and its drain
# (w1/m187). Upstream's 5s default would quietly bring the eviction back.
echo "==> autoscaler leaves drain-guard a window before it drains"
delete_delay="$(yq -N '.extraArgs.node-delete-delay-after-taint // ""' "$AUTOSCALER_VALUES")"
case "$delete_delay" in
  [2-9]m | [1-9][0-9]m) ;;
  *)
    echo "FAIL: autoscaler node-delete-delay-after-taint is '$delete_delay' (want whole minutes, at least 2m: drain-guard's window to start a replacement before CA evicts)" >&2
    fail=1
    ;;
esac

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

# 3a. The base image is pinned, so only the bake's upgrade moves node kernels.
echo "==> $PACKER upgrades to the reviewed kernel"
if [ -f "$PACKER" ]; then
  pk_kernel="$(pk_var kernel_version)"
  if [[ ! "$pk_kernel" =~ ^[0-9]+\.[0-9]+\.[0-9]+-[0-9]+$ ]] \
    || ! grep -Fq 'full-upgrade",' "$PACKER" \
    || ! grep -Fq '"KERNEL_VERSION=${var.kernel_version}"' "$PACKER" \
    || ! grep -Fq '= \"$KERNEL_VERSION\"' "$PACKER" \
    || ! grep -Fq -- '-kernel-${var.kernel_version}' "$PACKER"; then
    echo "FAIL: $PACKER must full-upgrade, pass a strict kernel_version (got '$pk_kernel'), assert the newest installed kernel equals it, and name the snapshot by it" >&2
    fail=1
  fi
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

# codex-security round 18 (CWE-494): the sandbox pool's gVisor runtime is the
# kernel boundary for hostile tenant sandboxes, and it installs at BOOTSTRAP.
# The binaries must be verified against SHA-512 digests pinned in this
# repository — a checksum file co-fetched from the same release origin is not a
# trust anchor (a compromised bucket or publisher replaces binary + checksum
# together), so the same-origin .sha256/.sha512 download must never return.
echo "==> $SANDBOX_OVERLAY verifies gVisor against repository-pinned digests"
sandbox_prek="$(yq -N 'select(.kind=="KubeadmConfigTemplate" and .metadata.name=="bex-sandbox") | .spec.template.spec.preKubeadmCommands[]' "$SANDBOX_OVERLAY")"
if ! echo "$sandbox_prek" | grep -q 'ExecStartPost=/usr/sbin/nft -f /etc/bex-containerd-egress.nft' \
  || ! echo "$sandbox_prek" | grep -q "nft list table inet bex_containerd_egress"; then
  echo "FAIL: bex-sandbox must deny containerd's cgroup link-local egress (w5/171)" >&2
  fail=1
fi
if echo "$sandbox_prek" | grep -Eq 'wget.*\.sha(256|512)'; then
  echo "FAIL: sandbox bootstrap must not co-fetch gVisor checksum files — the repository-pinned digest is the trust anchor" >&2
  fail=1
fi
if ! echo "$sandbox_prek" | grep -Fq 'test "$ARCH" = x86_64'; then
  echo "FAIL: sandbox bootstrap must fail closed on a non-x86_64 architecture (the pin only covers the pool's cpx32/cx33 shape)" >&2
  fail=1
fi
if [ "$(echo "$sandbox_prek" | grep -c 'sha512sum -c')" -lt 1 ]; then
  echo "FAIL: sandbox bootstrap must verify runsc + containerd-shim-runsc-v1 with sha512sum -c against the pinned digests" >&2
  fail=1
fi
gvisor_pins="$(echo "$sandbox_prek" | grep -oE '\b[0-9a-f]{128}\b' | sort -u || true)"
if [ "$(printf '%s\n' "$gvisor_pins" | grep -c .)" -ne 2 ]; then
  echo "FAIL: sandbox bootstrap must pin exactly two gVisor SHA-512 digests (runsc + containerd-shim-runsc-v1), got:" >&2
  printf '%s\n' "$gvisor_pins" | sed 's/^/    /' >&2
  fail=1
fi
gvisor_release="$(echo "$sandbox_prek" | grep -oE 'gvisor/releases/release/[0-9]+\.[0-9]+' | sort -u || true)"
if [ "$(printf '%s\n' "$gvisor_release" | grep -c .)" -ne 1 ]; then
  echo "FAIL: sandbox bootstrap must download gVisor from exactly one versioned releases/release/<version> URL (never latest), got:" >&2
  printf '%s\n' "$gvisor_release" | sed 's/^/    /' >&2
  fail=1
fi


# w2/m81 (docs/ADR072-security-review-round7.md #5 / ADR061 #11 / ADR063 #9):
# every kubelet must request a CA-signed serving certificate — the
# kubelet-csr-approver Argo Application (deploy/gitops/base/
# kubelet-csr-approver.yaml) approves the resulting CSR, and metrics-server's
# --kubelet-certificate-authority (guarded in scripts/gitops-validate.sh)
# only stays safe to keep on if every node actually requests one. CAPH writes
# kubeletExtraArgs as a map (key: value); CAPD writes it as a list
# (- name/value); check both shapes rather than assume one.
echo "==> every kubeletExtraArgs block requests kubelet serving-cert rotation"
CAPD_OVERLAY="${CAPD_OVERLAY_FILE:-infra/clusterapi/overlays/local-capd/cluster.yaml}"
for kubelet_args_file in "$OVERLAY" "$SANDBOX_OVERLAY" "$CAPD_OVERLAY"; do
  [ -f "$kubelet_args_file" ] || continue
  missing="$(yq -o=json -N '[.. | select(has("kubeletExtraArgs"))] | .[].kubeletExtraArgs' "$kubelet_args_file" 2>/dev/null | jq -s '
    [.[] | if (type == "object")
      then (has("rotate-server-certificates") and .["rotate-server-certificates"] == "true")
      else (any(.[]; .name == "rotate-server-certificates" and .value == "true"))
      end] |
    length - (map(select(. == true)) | length)
  ')"
  if [ -z "$missing" ] || [ "$missing" != 0 ]; then
    echo "FAIL: $kubelet_args_file has a kubeletExtraArgs block without rotate-server-certificates: \"true\" (w2/m81) — kubelets there fall back to self-signed serving certs" >&2
    fail=1
  fi
done

# w4/m200: the bake must keep installing the reviewed patched kubelet. A
# recipe that drops the step, or a release bump that leaves the patch behind,
# would silently restore the stock reader that joins stdout/stderr records.
echo "==> the node bake installs the reviewed CRI-record kubelet"
FRAMING="infra/packer/log-record-framing"
pk_kubelet_sha="$(pk_var kubelet_amd64_sha256)"
pk_kubelet_tag="$(pk_var kubelet_release_tag)"
framing_version="$(sed -n 's/^ARG KUBERNETES_VERSION=v//p' "$FRAMING/Dockerfile")"
if ! [[ "$pk_kubelet_sha" =~ ^[0-9a-f]{64}$ ]] \
  || ! grep -qF 'echo \"$KUBELET_AMD64_SHA256  /tmp/kubelet\" | sha256sum --check -' "$PACKER" \
  || ! grep -qF 'install -m 0755 /tmp/kubelet /usr/bin/kubelet' "$PACKER" \
  || [ ! -s "$FRAMING/kubelet-cri-records.patch" ]; then
  echo "FAIL: $PACKER must install the sha256-pinned patched kubelet from $FRAMING (w4/m200)" >&2
  fail=1
fi
if [ "$framing_version" != "$pk_k8s" ] || [[ "$pk_kubelet_tag" != "bex-kubelet/v$pk_k8s-"* ]]; then
  echo "FAIL: patched kubelet (Dockerfile v$framing_version, release $pk_kubelet_tag) does not match the baked Kubernetes v$pk_k8s — rebuild and re-pin it with the bump (w4/m200)" >&2
  fail=1
fi

# A full control-plane roll replaces every node; without the stable host key
# the BEX_SSH_KNOWN_HOSTS pin goes stale and every admin.conf fetch fails until
# a human re-pins (w6/m153 t004/t008, ADR019).
echo "==> $OVERLAY control planes serve the stable SSH host key"
APP_CLUSTER_WORKFLOW="${APP_CLUSTER_WORKFLOW_FILE:-.github/workflows/app-cluster.yml}"
cp_key_secret=bex-control-plane-ssh-host-key
cp_key_stage=/etc/bex/cp-ssh-host-key/ssh_host_ed25519_key
kcp_json="$(yq -o=json -N 'select(.kind == "KubeadmControlPlane") | .spec.kubeadmConfigSpec' "$OVERLAY")"
staged="$(printf '%s' "$kcp_json" | jq -r --arg s "$cp_key_secret" '[.files[]? | select(.contentFrom.secret.name == $s) | .path] | sort | join(",")')"
if [ "$staged" != "$cp_key_stage,$cp_key_stage.pub" ] \
  || ! printf '%s' "$kcp_json" | jq -e --arg c "install -m 0600 -o root -g root $cp_key_stage /etc/ssh/ssh_host_ed25519_key" '.preKubeadmCommands | any(. == $c)' >/dev/null \
  || ! grep -qF "apply_secret bex-capi $cp_key_secret" "$APP_CLUSTER_WORKFLOW"; then
  echo "FAIL: KubeadmControlPlane must stage $cp_key_secret and install it as /etc/ssh/ssh_host_ed25519_key, and $APP_CLUSTER_WORKFLOW must create that Secret (w6/m153)" >&2
  fail=1
fi

[ "$fail" -eq 0 ] && echo "PASS: CAPH overlay is internally consistent (reviewed snapshot ⟺ no privileged bootstrap downloads)" || { echo "FAIL: see errors above" >&2; exit 1; }
