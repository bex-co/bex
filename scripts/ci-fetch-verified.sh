#!/usr/bin/env bash
# Download URL to OUT and verify it against SHA256, resuming across attempts.
#
#   bash scripts/ci-fetch-verified.sh URL SHA256 OUT [ATTEMPTS]
#
# CI runners on a slow link time out large release downloads (curl exit 28 on
# the 95 MB Prometheus tarball, six times in deploy run 41585192817), and
# curl --retry restarts each attempt from byte zero. Here every attempt resumes
# (-C -) from what is already on disk, so progress accumulates. A complete file
# with the wrong digest is discarded and fetched again; the script exits
# non-zero unless OUT ends with exactly SHA256.
set -euo pipefail

url="$1" want="$2" out="$3" attempts="${4:-8}"
case "$want" in
  *[!0-9a-f]* | "") echo "ci-fetch-verified: SHA256 must be 64 lowercase hex digits" >&2; exit 2 ;;
esac
[ "${#want}" -eq 64 ] || { echo "ci-fetch-verified: SHA256 must be 64 lowercase hex digits" >&2; exit 2; }

verified() { [ -f "$out" ] && [ "$(sha256sum "$out" | cut -d' ' -f1)" = "$want" ]; }

for attempt in $(seq 1 "$attempts"); do
  if verified; then
    echo "ci-fetch-verified: $out verified after $((attempt - 1)) attempt(s)"
    exit 0
  fi
  have=$(stat -c %s "$out" 2>/dev/null || stat -f %z "$out" 2>/dev/null || echo 0)
  echo "ci-fetch-verified: attempt $attempt/$attempts, resuming at byte $have"
  rc=0
  curl -fsSL -C - --connect-timeout 30 --max-time 300 -o "$out" "$url" || rc=$?
  # 33: the server cannot resume (or the range is past the end). Start over.
  if [ "$rc" -eq 33 ] || { [ "$rc" -eq 0 ] && ! verified; }; then
    rm -f "$out"
  fi
  if [ "$rc" -ne 0 ] && [ "$attempt" -lt "$attempts" ]; then
    sleep 5
  fi
done
if verified; then
  echo "ci-fetch-verified: $out verified"
  exit 0
fi
echo "ci-fetch-verified: $url did not verify against $want after $attempts attempts" >&2
exit 1
