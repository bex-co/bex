#!/usr/bin/env bash
# Metrics retention guard (w7/m427). bex-api floors every metrics chart read at
# the workspace plan's window (core.MetricsRetention: Hobby 7 days, Pro 14,
# Scale and Enterprise 30, as on Render) and trusts the store to still hold it.
# Production Prometheus must therefore keep at least the longest window, or
# Scale and Enterprise workspaces silently see less than their plan includes.
#
# Checked on the rendered production overlay's prometheus Application:
#   - server.retention is whole days and at least 30;
#   - server.retentionSize (blocks only; Prometheus's GB is 2^30 bytes) is more
#     than 70% of server.persistentVolume.size, so MetricsStoreVolumeFillingUp
#     (70% full) fires before the cap starts dropping history, and at most 80%,
#     which leaves room for the WAL and one compaction.
#
#   bash scripts/metrics-retention-guard.sh
#   METRICS_RETENTION_APPS=<rendered overlay> bash scripts/metrics-retention-guard.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# The longest plan window, in days (metricsRetention in
# lego/backend/internal/core/plan_gate.go).
LONGEST_PLAN_DAYS=30

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "FAIL: metrics retention guard requires $1 — refusing to pass without running" >&2
    exit 2
  }
}
need yq

apps="${METRICS_RETENTION_APPS:-}"
if [ -z "$apps" ]; then
  need kubectl
  apps="$(mktemp)"
  trap 'rm -f "$apps"' EXIT
  kubectl kustomize deploy/gitops/overlays/prod >"$apps"
fi

param() {
  yq -N 'select(.kind == "Application" and .metadata.name == "prometheus") |
    .spec.source.helm.parameters[] | select(.name == "'"$1"'") | .value' "$apps"
}

fail=0
retention="$(param server.retention)"
cap="$(param server.retentionSize)"
volume="$(param server.persistentVolume.size)"

if ! [[ "$retention" =~ ^([0-9]+)d$ ]]; then
  echo "FAIL: production Prometheus server.retention is '${retention:-unset}'; set it in whole days (deploy/gitops/overlays/prod/kustomization.yaml)" >&2
  fail=1
elif [ "${BASH_REMATCH[1]}" -lt "$LONGEST_PLAN_DAYS" ]; then
  echo "FAIL: production Prometheus keeps $retention, below the ${LONGEST_PLAN_DAYS}-day metrics window of the Scale and Enterprise plans; bex-api floors reads per plan and needs the store to hold the longest (ADR010)" >&2
  fail=1
fi

cap_gib=""
volume_gib=""
[[ "$cap" =~ ^([0-9]+)GB$ ]] && cap_gib="${BASH_REMATCH[1]}"
[[ "$volume" =~ ^([0-9]+)Gi$ ]] && volume_gib="${BASH_REMATCH[1]}"
if [ -z "$cap_gib" ] || [ -z "$volume_gib" ]; then
  echo "FAIL: production Prometheus needs server.retentionSize in GB and server.persistentVolume.size in Gi (got '${cap:-unset}' and '${volume:-unset}'): without the cap a growing series count fills the volume" >&2
  fail=1
elif [ $((cap_gib * 100)) -le $((volume_gib * 70)) ] || [ $((cap_gib * 100)) -gt $((volume_gib * 80)) ]; then
  echo "FAIL: production Prometheus caps blocks at $cap on a $volume volume; keep the cap above 70% of it (MetricsStoreVolumeFillingUp must fire before history is dropped) and at most 80% (room for the WAL and a compaction)" >&2
  fail=1
fi

[ "$fail" = 0 ] && echo "PASS: production Prometheus keeps $retention, capped at $cap on $volume"
exit "$fail"
