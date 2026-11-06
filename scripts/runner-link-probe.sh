#!/usr/bin/env bash
# runner-link-probe.sh — measure the runner host's internet link (w6/m160).
#
# On 2026-11-06 the host's link carried ~1 MB/s in total for hours: every CI
# job stalled on downloads, runners sat busy at ~0 % CPU, and deploys took
# 2-5 h, yet nothing reported the cause. This downloads a GitHub release asset
# and a Cloudflare speed-test payload for up to RUNNER_LINK_MAX_TIME seconds
# each, prints one line per target, and classifies the link against
# RUNNER_LINK_MIN_MBPS. The pool's Docker Hub mirror (DOCKER_REGISTRY_MIRROR,
# set on the runners) is reported for reachability only.
#
# Exit: 0 ok, 1 slow (a target below the threshold or unreachable),
#       2 unusable (curl missing, or no target answered at all).
set -uo pipefail

# 3 MB/s is an alarm level, not health: w6/m158's release condition is
# >= 10 MB/s. At most 20 MB per target (a ranged read of the 96 MB GitHub asset)
# keeps a reading under ~40 MB on a fast link and under 2 x 15 s on a slow one.
MIN_MBPS="${RUNNER_LINK_MIN_MBPS:-3}"
MAX_TIME="${RUNNER_LINK_MAX_TIME:-15}"
TARGETS="${RUNNER_LINK_TARGETS:-github=https://github.com/prometheus/prometheus/releases/download/v3.13.5/prometheus-3.13.5.linux-arm64.tar.gz
cloudflare=https://speed.cloudflare.com/__down?bytes=20000000}"

command -v curl >/dev/null || { echo "runner-link-probe: curl is required" >&2; exit 2; }
[[ "$MIN_MBPS" =~ ^[0-9]+([.][0-9]+)?$ ]] || { echo "runner-link-probe: RUNNER_LINK_MIN_MBPS must be a number, got '$MIN_MBPS'" >&2; exit 2; }

slow=0
answered=0
while IFS='=' read -r name url; do
  [ -n "$name" ] || continue
  # curl exits 28 when --max-time cuts a still-running transfer: that is a
  # measurement, not a failure.
  out="$(curl -sS -L -o /dev/null --range 0-19999999 --connect-timeout 15 --max-time "$MAX_TIME" \
    -w '%{http_code} %{speed_download} %{time_connect}' "$url" 2>/dev/null)"
  rc=$?
  read -r code speed connect <<<"$out"
  if { [ "$rc" = 0 ] || [ "$rc" = 28 ]; } && [ "${code:-000}" -ge 200 ] && [ "${code:-000}" -lt 400 ]; then
    answered=$((answered + 1))
    verdict="$(awk -v s="$speed" -v min="$MIN_MBPS" 'BEGIN { printf "%.2f %s", s / 1e6, (s / 1e6 < min ? "slow" : "ok") }')"
    echo "$name ${verdict% *} MB/s connect ${connect}s ${verdict#* }"
    [ "${verdict#* }" = ok ] || slow=1
  else
    echo "$name unreachable (curl exit $rc, HTTP ${code:-none})"
    slow=1
  fi
done <<<"$TARGETS"

if [ -n "${DOCKER_REGISTRY_MIRROR:-}" ]; then
  mirror_code="$(curl -s -o /dev/null --connect-timeout 5 --max-time 10 -w '%{http_code}' "${DOCKER_REGISTRY_MIRROR%/}/v2/" 2>/dev/null)"
  echo "mirror ${DOCKER_REGISTRY_MIRROR} HTTP ${mirror_code:-none} (reachability only)"
fi

[ "$answered" -gt 0 ] || { echo "runner-link-probe: no target answered" >&2; exit 2; }
[ "$slow" = 0 ] || { echo "runner-link-probe: link below ${MIN_MBPS} MB/s"; exit 1; }
echo "runner-link-probe: ok (>= ${MIN_MBPS} MB/s)"
