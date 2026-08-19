#!/usr/bin/env bash
# Self-test for the build-pool guards in scripts/clusterapi-validate.sh
# (w7/m83 t003 + t004). The anti-tautology rule: a guard with no proven red case
# is decoration. Both guards exist BECAUSE their failure modes are silent —
# a stranded DaemonSet that still reports READY, and a cluster-autoscaler that
# declines to scale up without emitting an error — so "it turns red" is the only
# evidence that they work at all.
#
# Method: copy the real overlay and the two build-relevant gitops manifests into
# a fixture tree, point the validator at them with OVERLAY_FILE / GITOPS_BASE_DIR,
# mutate one fact per case, and assert the exit code and the message.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
SCRIPT="$here/clusterapi-validate.sh"

OVERLAY_SRC="$root/infra/clusterapi/overlays/hetzner-caph/cluster.yaml"
GITOPS_SRC="$root/deploy/gitops/base"

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# fixture <name> — a fresh overlay + gitops copy; echoes the fixture dir.
fixture() {
  local dir="$tmp/$1"
  mkdir -p "$dir/gitops"
  cp "$OVERLAY_SRC" "$dir/cluster.yaml"
  cp "$GITOPS_SRC/build-image-prewarm.yaml" "$GITOPS_SRC/log-shipper.yaml" "$dir/gitops/"
  echo "$dir"
}

# assert <label> <want-rc> <fixture-dir> [expected-stderr-substring]
assert() {
  local label="$1" want="$2" dir="$3" needle="${4:-}" err got
  set +e
  err="$(OVERLAY_FILE="$dir/cluster.yaml" GITOPS_BASE_DIR="$dir/gitops" "$SCRIPT" 2>&1 >/dev/null)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    printf '%s\n' "$err" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$err" | grep -qF "$needle"; then
    echo "FAIL: $label — stderr did not name '$needle'" >&2
    printf '%s\n' "$err" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label (exit $got)"
}

# ── GREEN: the canonical tree, read through the same overrides ───────────────
assert "canonical tree passes" 0 "$(fixture green)"

# ── RED (t003): the exact production regression measured 2026-08-17 ──────────
d="$(fixture no-toleration)"
yq -i 'del(.spec.template.spec.tolerations)' "$d/gitops/build-image-prewarm.yaml"
assert "prewarm without the build toleration" 1 "$d" "build-image-prewarm does not tolerate bex.co/build-only"

# A toleration for the WRONG taint must not satisfy it — the DaemonSet would
# still never reach a build node.
d="$(fixture wrong-toleration)"
yq -i '.spec.template.spec.tolerations = [{"key":"bex.co/platform","operator":"Equal","value":"true","effect":"NoSchedule"}]' \
  "$d/gitops/build-image-prewarm.yaml"
assert "prewarm tolerating the wrong taint" 1 "$d" "build-image-prewarm does not tolerate bex.co/build-only"

# A keyless Exists toleration legitimately matches every taint (log-shipper's
# shape) — the guard must accept it rather than demanding one literal spelling.
d="$(fixture exists-toleration)"
yq -i '.spec.template.spec.tolerations = [{"operator":"Exists"}]' "$d/gitops/build-image-prewarm.yaml"
assert "prewarm with a keyless Exists toleration" 0 "$d"

# ── RED (t003): a drifted prewarm image warms the wrong bytes ────────────────
d="$(fixture drifted-image)"
yq -i '(.spec.template.spec.containers[] | select(.name == "buildkit") | .image) = "moby/buildkit:v0.29.0"' \
  "$d/gitops/build-image-prewarm.yaml"
assert "prewarm image drifted from defaultBuildkitImage" 1 "$d" "but builds run"

# ── The floor is composed, not a constant ───────────────────────────────────
# floor_line echoes the guard's own arithmetic line for a fixture.
floor_line() {
  OVERLAY_FILE="$1/cluster.yaml" GITOPS_BASE_DIR="$1/gitops" "$SCRIPT" 2>/dev/null | grep 'DaemonSet floor'
}
d="$(fixture floor-composition)"
canonical="$(floor_line "$d")"
case "$canonical" in
  *"DaemonSet floor 186Mi"*"build-image-prewarm log-shipper"*) echo "ok: floor sums both tolerating DaemonSets (8Mi + 128Mi + 50Mi off-repo sidecar)" ;;
  *) echo "FAIL: unexpected floor composition: $canonical" >&2; fails=$((fails + 1)) ;;
esac
# A DaemonSet that stops tolerating the taint must take its ENTIRE contribution
# with it — including the chart sidecar allowance it is the reason for. A global
# sidecar constant would leave 50Mi of a DaemonSet that is no longer there.
yq -i '.spec.source.helm.values |= (from_yaml | .controller.tolerations = [] | to_yaml)' "$d/gitops/log-shipper.yaml"
without="$(floor_line "$d")"
case "$without" in
  *"DaemonSet floor 8Mi"*) echo "ok: an untolerating DaemonSet leaves the floor entirely (sidecar allowance included)" ;;
  *) echo "FAIL: floor kept part of a DaemonSet that no longer tolerates the taint: $without" >&2; fails=$((fails + 1)) ;;
esac

# ── RED (t004): the hint erodes below the floor ──────────────────────────────
d="$(fixture low-hint)"
yq -i '(select(.kind == "MachineDeployment" and .metadata.name == "bex-tenant-burst")
        | .metadata.annotations."capacity.cluster-autoscaler.kubernetes.io/memory") = "7G"' "$d/cluster.yaml"
assert "build pool hint below the floor" 1 "$d" "is BELOW the"

# ── RED (t004): the floor grows into the hint from the DaemonSet side ────────
# This is the erosion path the milestone was written for: nothing about the
# pools changed, a DaemonSet simply got hungrier.
d="$(fixture fat-daemonset)"
yq -i '(.spec.template.spec.containers[] | select(.name == "buildkit") | .resources.requests.memory) = "2Gi"' \
  "$d/gitops/build-image-prewarm.yaml"
assert "tolerating DaemonSet request grows past the margin" 1 "$d" "is BELOW the"

# ── RED (t004): simulation must stay conservative ────────────────────────────
d="$(fixture optimistic-hint)"
yq -i '(select(.kind == "MachineDeployment" and .metadata.name == "bex-tenant-burst")
        | .metadata.annotations."capacity.cluster-autoscaler.kubernetes.io/memory") = "9G"' "$d/cluster.yaml"
assert "CA hint above measured allocatable" 1 "$d" "EXCEEDS measured allocatable"

# ── RED (t004): an unmeasured server type may not become a build pool ────────
d="$(fixture unmeasured-type)"
yq -i '(select(.kind == "HCloudMachineTemplate" and .metadata.name == "bex-tenant-burst-k134-cx33")
        | .spec.template.spec.type) = "cx53"' "$d/cluster.yaml"
assert "build pool on an unmeasured server type" 1 "$d" "unmeasured server type"

# ── RED: the dedicated build pool disappears entirely ────────────────────────
d="$(fixture no-build-pool)"
yq -i '(select(.kind == "MachineDeployment")
        | .metadata.annotations) |= with_entries(select(.key != "capacity.cluster-autoscaler.kubernetes.io/taints"))' \
  "$d/cluster.yaml"
assert "no MachineDeployment registers the build taint" 1 "$d" "the dedicated build pool"

# ── GREEN: the real tree, with no overrides at all ───────────────────────────
if "$SCRIPT" >/dev/null 2>&1; then
  echo "ok: real tree passes end-to-end"
else
  echo "FAIL: real tree — clusterapi-validate.sh is red on the canonical repo" >&2
  fails=$((fails + 1))
fi

if [ "$fails" -eq 0 ]; then
  echo "PASS: clusterapi-validate.sh build-pool guards (red and green proven)"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
