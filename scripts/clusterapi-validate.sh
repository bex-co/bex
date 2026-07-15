#!/usr/bin/env bash
# Structural guard for the CAPH overlay — pure client-side, no cluster (w1/m36 t007).
# The baked-worker-image win (w1/m36) is only preserved if a future template edit
# can't silently reintroduce the ~350MB of scale-up-hot-path downloads, and a
# staged roll (tenant pool baked before the platform pool) can't leave a pool in
# a broken half-state. This asserts, over
# infra/clusterapi/overlays/hetzner-caph/cluster.yaml + the packer bake:
#   1. per worker MachineDeployment, the node image and its preKubeadmCommands are
#      CONSISTENT — a baked snapshot (imageName: bex-worker) must carry NO runtime
#      download / images-pull / trimmed-apt line, and an ubuntu-24.04 pool MUST
#      still download its runtime at boot (else its nodes come up with no runtime).
#      Any other imageName is rejected. (Control-plane uses KubeadmControlPlane, not
#      a worker MachineDeployment, so it is out of scope here.)
#   2. the platform pool floor stays at three nodes — prod OpenBao has three
#      required-anti-affinity Raft members, so a two-node floor is not viable.
#   3. the packer bake (infra/packer/bex-worker.pkr.hcl) installs no trimmed package.
#   4. the packer version pins (k8s/containerd/runc) MATCH the overlay — the bake
#      and the download-at-boot pools must stay in lockstep.
# Run locally before pushing overlay/packer changes; CI runs it via
# .github/workflows/clusterapi-validate.yml.
# Requires: yq v4.
set -euo pipefail
cd "$(dirname "$0")/.."

OVERLAY="infra/clusterapi/overlays/hetzner-caph/cluster.yaml"
PACKER="infra/packer/bex-worker.pkr.hcl"
fail=0

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
    bex-worker)
      if echo "$prek" | grep -qE "$FORBIDDEN"; then
        echo "FAIL: baked pool $md (KubeadmConfigTemplate $cfg) still has a download/pull/trimmed-apt line — the snapshot already carries the runtime:" >&2
        echo "$prek" | grep -nE "$FORBIDDEN" | sed 's/^/    /' >&2; fail=1
      fi
      ;;
    ubuntu-24.04)
      if ! echo "$prek" | grep -qE "$BUILDS_RUNTIME"; then
        echo "FAIL: ubuntu pool $md (KubeadmConfigTemplate $cfg) has imageName ubuntu-24.04 but its preKubeadmCommands no longer download the runtime — nodes would boot with no container runtime. Flip imageName to bex-worker or restore the downloads." >&2; fail=1
      fi
      ;;
    *)
      echo "FAIL: worker pool $md has unexpected imageName '$img' (want 'bex-worker' or 'ubuntu-24.04')" >&2; fail=1
      ;;
  esac
done < <(yq -N 'select(.kind=="MachineDeployment") | .metadata.name + " " + .spec.template.spec.infrastructureRef.name + " " + .spec.template.spec.bootstrap.configRef.name' "$OVERLAY")

# 2. The platform autoscaler must preserve one node per OpenBao Raft member.
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

# 4. The bake pins and the overlay must not drift.
echo "==> $PACKER version pins match the overlay"
if [ -f "$PACKER" ]; then
  pk_var() { grep -A4 "variable \"$1\"" "$PACKER" | grep -m1 -oE 'default[[:space:]]*=[[:space:]]*"[^"]+"' | grep -oE '"[^"]+"' | tr -d '"'; }
  pk_k8s="$(pk_var kubernetes_version)"; pk_containerd="$(pk_var containerd_version)"; pk_runc="$(pk_var runc_version)"
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
fi

[ "$fail" -eq 0 ] && echo "PASS: CAPH overlay is internally consistent (baked ⟺ no-download, ubuntu ⟺ download)" || { echo "FAIL: see errors above" >&2; exit 1; }
