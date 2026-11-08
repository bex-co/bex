#!/bin/sh
# local-path volume usage, published under the kubelet's own series name so
# bex-api's production disk query reads it unchanged on the CAPD mock (w2/m255
# t009).
#
# Why this exists: the mock's only StorageClass is rancher.io/local-path, which
# provisions hostPath PersistentVolumes. The kubelet's hostPath plugin has no
# metrics provider, so it publishes NO kubelet_volume_stats_* series for them
# (checked 2026-11-07 on every CAPD node: zero series in /metrics/ and no
# pvcRef volume in /stats/summary, while CNPG's PVC-backed pods ran). Without a
# series, the disk-usage sweeper (lego/backend/internal/diskusage) sees every
# disk as "not mounted" and never fires. Production volumes (hcloud CSI) do
# report it, so the production query and series name stay as they are and this
# local DaemonSet supplies the same series from `du` of each claim's directory.
#
# local-path names every directory pvc-<uid>_<namespace>_<claim>; neither a
# namespace nor a claim name may contain "_", so the split is unambiguous.
#
#   volume-usage.sh once [ROOT]  # print one exposition to stdout (tests)
#   volume-usage.sh serve [ROOT] # refresh every 15s and serve it on :19101
set -eu

ROOT="${2:-/data}"

metrics() {
  echo '# HELP kubelet_volume_stats_used_bytes Bytes used by a local-path volume (du of its directory; dev-env stand-in for the kubelet series).'
  echo '# TYPE kubelet_volume_stats_used_bytes gauge'
  for d in "$1"/pvc-*_*_*; do
    [ -d "$d" ] || continue
    rest="${d##*/}"
    rest="${rest#*_}"
    ns="${rest%%_*}"
    claim="${rest#*_}"
    kib="$(du -sk "$d" 2>/dev/null | cut -f1)"
    [ -n "$kib" ] || continue
    echo "kubelet_volume_stats_used_bytes{namespace=\"$ns\",persistentvolumeclaim=\"$claim\"} $((kib * 1024))"
  done
}

case "${1:-}" in
once) metrics "$ROOT" ;;
serve)
  mkdir -p /tmp/www
  (
    while true; do
      metrics "$ROOT" >/tmp/www/metrics.tmp && mv /tmp/www/metrics.tmp /tmp/www/metrics
      sleep 15
    done
  ) &
  exec httpd -f -p 19101 -h /tmp/www
  ;;
*)
  echo "usage: volume-usage.sh {once|serve} [ROOT]" >&2
  exit 2
  ;;
esac
