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
#
# A github.com release URL (/OWNER/REPO/releases/download/TAG/NAME) has a
# second source: the same asset through api.github.com. On 2026-11-08
# github.com answered 504 to every release download for over an hour while
# the API served the identical bytes, so later attempts alternate between the
# two. GH_TOKEN (or GITHUB_TOKEN), when set, authenticates the API calls. The
# digest check is the same whichever source answered.
set -euo pipefail

url="$1" want="$2" out="$3" attempts="${4:-8}"
case "$want" in
  *[!0-9a-f]* | "") echo "ci-fetch-verified: SHA256 must be 64 lowercase hex digits" >&2; exit 2 ;;
esac
[ "${#want}" -eq 64 ] || { echo "ci-fetch-verified: SHA256 must be 64 lowercase hex digits" >&2; exit 2; }

verified() { [ -f "$out" ] && [ "$(sha256sum "$out" | cut -d' ' -f1)" = "$want" ]; }

token="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
auth=()
[ -n "$token" ] && auth=(-H "Authorization: Bearer $token")
fetch() { curl -fsSL --connect-timeout 30 --max-time 300 "$@"; }
api() { fetch ${auth[@]+"${auth[@]}"} "$@"; }
# api_asset_url prints the api.github.com URL of URL's release asset, or fails.
api_asset_url() {
  [[ "$url" =~ ^https://github\.com/([^/]+/[^/]+)/releases/download/([^/]+)/([^/]+)$ ]] || return 1
  local repo="${BASH_REMATCH[1]}" tag="${BASH_REMATCH[2]}" name="${BASH_REMATCH[3]}" id
  id=$(api "https://api.github.com/repos/$repo/releases/tags/$tag" |
    jq -er --arg name "$name" '.assets[] | select(.name == $name) | .id') || return 1
  echo "https://api.github.com/repos/$repo/releases/assets/$id"
}
fallback=""

for attempt in $(seq 1 "$attempts"); do
  if verified; then
    echo "ci-fetch-verified: $out verified after $((attempt - 1)) attempt(s)"
    exit 0
  fi
  have=$(stat -c %s "$out" 2>/dev/null || stat -f %z "$out" 2>/dev/null || echo 0)
  echo "ci-fetch-verified: attempt $attempt/$attempts, resuming at byte $have"
  rc=0
  if [ $((attempt % 2)) -eq 0 ] && { [ -n "$fallback" ] || fallback=$(api_asset_url); }; then
    echo "ci-fetch-verified: trying $fallback"
    api -C - -H "Accept: application/octet-stream" -o "$out" "$fallback" || rc=$?
  else
    fetch -C - -o "$out" "$url" || rc=$?
  fi
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
