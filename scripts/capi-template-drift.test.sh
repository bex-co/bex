#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/capi-template-drift.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() {
  echo "FAIL: $*" >&2
  fails=$((fails + 1))
}

mkdir -p "$tmp/bin"
# kubectl stub: `diff -f FILE` answers by the template name in FILE; anything
# that is not a diff is a mutation and fails the test.
cat >"$tmp/bin/kubectl" <<'EOF'
#!/bin/sh
echo "$*" >>"$KUBECTL_TRACE"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --context) shift 2 ;;
    diff) verb=diff; shift ;;
    -f) file="$2"; shift 2 ;;
    *) echo "unexpected kubectl call: $*" >&2; exit 9 ;;
  esac
done
[ "$verb" = diff ] || exit 9
case "$file" in
  *drifted*) echo "-      wget latest"; echo "+      wget 20260622.0"; exit 1 ;;
  *broken*) echo "forbidden" >&2; exit 2 ;;
  *) exit 0 ;;
esac
EOF
chmod +x "$tmp/bin/kubectl"
export KUBECTL_TRACE="$tmp/trace"

template() {
  printf -- '---\napiVersion: bootstrap.cluster.x-k8s.io/v1beta1\nkind: %s\nmetadata: {name: %s, namespace: bex-capi}\nspec: {}\n' "$1" "$2"
}
{
  template KubeadmConfigTemplate pool-clean
  template HCloudMachineTemplate pool-clean
  printf -- '---\napiVersion: cluster.x-k8s.io/v1beta1\nkind: MachineDeployment\nmetadata: {name: pool-drifted}\nspec: {}\n'
} >"$tmp/clean.yaml"
template KubeadmConfigTemplate pool-drifted >"$tmp/drifted.yaml"
template HCloudMachineTemplate pool-broken >"$tmp/broken.yaml"

run() {
  set +e
  PATH="$tmp/bin:$PATH" BEX_KUBE_CONTEXT=test-ctx bash "$script" "$@" >"$tmp/out" 2>&1
  rc=$?
  set -e
}

: >"$KUBECTL_TRACE"
run "$tmp/clean.yaml"
[ "$rc" -eq 0 ] || fail "matching templates must exit 0 (got $rc)"
grep -q "no drift" "$tmp/out" || fail "clean run must say no drift"
[ "$(grep -c 'diff -f' "$KUBECTL_TRACE")" -eq 2 ] || fail "only the two templates are compared, never the MachineDeployment"
grep -q -- "--context test-ctx" "$KUBECTL_TRACE" || fail "BEX_KUBE_CONTEXT must select the context"

run "$tmp/clean.yaml" "$tmp/drifted.yaml"
[ "$rc" -eq 1 ] || fail "a drifted template must exit 1 (got $rc)"
grep -q "DRIFT  KubeadmConfigTemplate-pool-drifted" "$tmp/out" || fail "drift must name the template"
grep -q "wget 20260622.0" "$tmp/out" || fail "drift must show the diff"

run "$tmp/drifted.yaml" "$tmp/broken.yaml"
[ "$rc" -eq 2 ] || fail "a template that cannot be compared must exit 2, even beside drift (got $rc)"

printf -- '---\nkind: MachineDeployment\nmetadata: {name: x}\n' >"$tmp/none.yaml"
run "$tmp/none.yaml"
[ "$rc" -ne 0 ] || fail "a manifest without templates must not report success"

if [ "$fails" -ne 0 ]; then
  echo "capi-template-drift: $fails failure(s)" >&2
  exit 1
fi
echo "PASS capi-template-drift"
