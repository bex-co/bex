#!/usr/bin/env bash
# One Helm installer for day-0 bootstrap and the scoped, manual CSI-only rollout.
# Reviewed derivative values are JSON (also accepted by Helm), so validation
# needs only Python's standard library, without another downloaded YAML parser.
set -euo pipefail
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
mode=${1:-bootstrap}
reviewed_values="$repo_root/deploy/hcloud-csi/reviewed.values.json"
values="$reviewed_values"
kind=reviewed
case "$mode" in
  bootstrap)
    if [ ! -e "$reviewed_values" ] && [ ! -L "$reviewed_values" ]; then
      values="$repo_root/deploy/hcloud-csi/official.values.json"
      kind=official
    fi
    ;;
  rollout|validate-pin) ;;
  *) echo 'usage: install-hcloud-csi.sh {bootstrap|rollout|validate-pin}' >&2; exit 2 ;;
esac
[ -f "$values" ] && [ ! -L "$values" ] \
  || { echo "hcloud-csi: reviewed digest values are missing; publish, verify and commit reviewed.values.json first" >&2; exit 1; }
if [ "$kind" = reviewed ]; then
  relative=deploy/hcloud-csi/reviewed.values.json
  git -C "$repo_root" ls-files --error-unmatch "$relative" >/dev/null 2>&1 \
    && git -C "$repo_root" diff --quiet HEAD -- "$relative" \
    || { echo 'hcloud-csi: derivative digest values must be committed and unchanged from HEAD' >&2; exit 1; }
fi
image=$(python3 "$repo_root/scripts/hcloud_csi.py" pin "$kind" "$values")
if [ "$mode" = validate-pin ]; then
  printf '%s\n' "$image"
  exit 0
fi
[ -n "${KUBECONFIG:-}" ] && [ -s "$KUBECONFIG" ] \
  || { echo 'hcloud-csi: an explicit fetched KUBECONFIG is required' >&2; exit 1; }
# A scoped rollout upgrades an existing release; it cannot bootstrap a new
# cluster, create provider credentials, or run CAPI/Terraform/CNI operations.
helm_args=(upgrade)
if [ "$mode" = bootstrap ]; then
  helm_args+=(--install)
else
  helm status hcloud-csi -n kube-system >/dev/null
fi
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
chart=$(bash "$repo_root/scripts/helm-artifact.sh" pull hcloud-csi "$work/chart")
helm "${helm_args[@]}" hcloud-csi "$chart" -n kube-system \
  --reuse-values --values "$values" --wait --timeout 10m \
  --set 'controller.nodeSelector.bex\.co/pool=platform' \
  --set 'controller.tolerations[0].key=bex.co/platform' \
  --set 'controller.tolerations[0].operator=Equal' \
  --set-string 'controller.tolerations[0].value=true' \
  --set 'controller.tolerations[0].effect=NoSchedule'
kubectl -n kube-system get deployment,daemonset -l app.kubernetes.io/instance=hcloud-csi -o json > "$work/workloads.json"
python3 "$repo_root/scripts/hcloud_csi.py" resources "$work/workloads.json" > "$work/resources"
while IFS= read -r resource; do
  kubectl -n kube-system rollout status "$resource" --timeout=600s
done < "$work/resources"
# Old node publishers ignore bounded-filesystem fields. Require their full
# termination, not only the new DaemonSet generation becoming Ready.
for attempt in {1..30}; do
  kubectl -n kube-system get deployment,daemonset -l app.kubernetes.io/instance=hcloud-csi -o json > "$work/workloads.json"
  kubectl -n kube-system get pods -l app.kubernetes.io/instance=hcloud-csi -o json > "$work/pods.json"
  if python3 "$repo_root/scripts/hcloud_csi.py" ready "$image" "$work/workloads.json" "$work/pods.json" 2> "$work/readiness-error"; then
    exit 0
  fi
  if [ "$attempt" -eq 30 ]; then
    cat "$work/readiness-error" >&2
    exit 1
  fi
  sleep 2
done
