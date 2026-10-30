#!/usr/bin/env bash
# Exercise the scoped installer's fail-closed pin policy and allowed operations.
# All cluster/Git/Helm calls below are fakes; no production credentials or API.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
fixture="$work/fixture repo"
mkdir -p "$fixture/scripts" "$fixture/deploy/hcloud-csi" "$work/bin"
cp "$repo_root/scripts/"{install-hcloud-csi.sh,hcloud_csi.py,helm-artifact.sh} "$fixture/scripts/"
cp "$repo_root/deploy/hcloud-csi/official.values.json" "$fixture/deploy/hcloud-csi/"
printf 'reviewed fake chart archive\n' > "$work/chart.tgz"
digest=$(sha256sum "$work/chart.tgz" | awk '{print $1}')
printf 'hcloud-csi|2.22.0|https://charts.hetzner.cloud|%s|-\n' "$digest" > "$fixture/deploy/helm-artifacts.lock"
cat > "$work/bin/git" <<'SH'
#!/usr/bin/env bash
set -eu
shift 2
[ "${!#}" = deploy/hcloud-csi/reviewed.values.json ] || exit 96
case "$1" in
  ls-files) [ "${PIN_TRACKED:-1}" = 1 ] ;;
  diff) [ "${PIN_CLEAN:-1}" = 1 ] ;;
  *) exit 97 ;;
esac
SH
cat > "$work/bin/helm" <<'SH'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/helm-calls"
case "$1" in
  status) [ "${RELEASE_EXISTS:-1}" = 1 ] ;;
  pull) mkdir -p "${!#}"; cp "$FIXTURE/chart.tgz" "${!#}/hcloud-csi-2.22.0.tgz" ;;
  upgrade) ;;
  *) echo 'unexpected Helm operation' >&2; exit 98 ;;
esac
SH
cat > "$work/bin/kubectl" <<'SH'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/kubectl-calls"
case "$*" in
  *'get deployment,daemonset '*) cat "$FIXTURE/workloads.json" ;;
  *'get pods '*) cat "$FIXTURE/pods.json" ;;
  *'rollout status deployment/'*|*'rollout status daemonset/'*) ;;
  *) echo 'unexpected Kubernetes operation' >&2; exit 99 ;;
esac
SH
printf '#!/usr/bin/env bash\nexit 0\n' > "$work/bin/sleep"
chmod +x "$work/bin/"*
printf 'disposable fake kubeconfig\n' > "$work/test.kubeconfig"
export KUBECONFIG="$work/test.kubeconfig"
export PATH="$work/bin:$PATH" FIXTURE="$work"
export CSI_TEST_REPO="$repo_root"
prepare_ready() {
  EXPECTED_IMAGE="$1" python3 - <<'PY'
import importlib.util,json,os
from pathlib import Path
spec=importlib.util.spec_from_file_location('fixtures',Path(os.environ['CSI_TEST_REPO'])/'scripts/test_hcloud_csi.py')
fixtures=importlib.util.module_from_spec(spec);spec.loader.exec_module(fixtures)
workloads,pods=fixtures.ready_documents(os.environ['EXPECTED_IMAGE'])
p=Path(os.environ['FIXTURE'])
(p/'workloads.json').write_text(json.dumps(workloads));(p/'pods.json').write_text(json.dumps(pods))
PY
}
run() {
  rm -f "$work/helm-calls" "$work/kubectl-calls"
  bash "$fixture/scripts/install-hcloud-csi.sh" "$@" > "$work/output" 2> "$work/error"
}
expect_failure_without_cluster() {
  if run "$@"; then echo "FAIL: unexpectedly accepted $*" >&2; exit 1; fi
  [ ! -e "$work/helm-calls" ] && [ ! -e "$work/kubectl-calls" ]
}
# A manual rollout must never silently install the official fallback.
expect_failure_without_cluster validate-pin
expect_failure_without_cluster rollout
KUBECONFIG= expect_failure_without_cluster bootstrap
# Bootstrap preserves the official driver, pinned to its reviewed digest.
official=$(python3 "$repo_root/scripts/hcloud_csi.py" pin official "$repo_root/deploy/hcloud-csi/official.values.json")
prepare_ready "$official"
run bootstrap
grep -q 'upgrade --install hcloud-csi ' "$work/helm-calls"
grep -q -- '--reuse-values --values' "$work/helm-calls"
grep -q 'official.values.json' "$work/helm-calls"
grep -q 'readyNodePods' "$work/output"
# Synthetic reviewed fixture only: no real digest pin is written into the repo.
python3 - <<'PY'
import json,os
from pathlib import Path
image='ghcr.io/bex-co/bex-operator@sha256:'+'a'*64
values={role:{'image':{'hcloudCSIDriver':{'name':image,'tag':''}}} for role in ('controller','node')}
(Path(os.environ['FIXTURE'])/'fixture repo/deploy/hcloud-csi/reviewed.values.json').write_text(json.dumps(values))
PY
PIN_TRACKED=0 expect_failure_without_cluster validate-pin
PIN_CLEAN=0 expect_failure_without_cluster rollout
run validate-pin
reviewed=$(cat "$work/output")
[ "$reviewed" = "ghcr.io/bex-co/bex-operator@sha256:$(printf 'a%.0s' {1..64})" ]
[ ! -e "$work/helm-calls" ]
prepare_ready "$reviewed"
run rollout
grep -q '^status hcloud-csi -n kube-system$' "$work/helm-calls"
grep -q '^upgrade hcloud-csi ' "$work/helm-calls"
if grep -q -- '--install' "$work/helm-calls"; then echo 'FAIL: rollout bootstrapped a new release' >&2; exit 1; fi
grep -q 'reviewed.values.json' "$work/helm-calls"
grep -q 'rollout status deployment/custom-csi-controller' "$work/kubectl-calls"
grep -q 'rollout status daemonset/custom-csi-node' "$work/kubectl-calls"
RELEASE_EXISTS=0 run rollout && { echo 'FAIL: accepted missing release' >&2; exit 1; }
# A completed Helm operation is insufficient when a node still uses old code.
prepare_ready "$official"
if run rollout; then echo 'FAIL: accepted old driver pods' >&2; exit 1; fi
# A broken reviewed pin must fail closed instead of falling back at bootstrap.
rm "$fixture/deploy/hcloud-csi/reviewed.values.json"
ln -s "$work/missing-pin.json" "$fixture/deploy/hcloud-csi/reviewed.values.json"
expect_failure_without_cluster bootstrap
printf 'PASS: CSI installer pins, preserves bootstrap, scopes upgrade, verifies all pods and refuses unreviewed input\n'
