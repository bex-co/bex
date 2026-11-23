#!/usr/bin/env bash
# Runs check.sh on platforms of a built operand index, as the publish
# workflow gates it (ADR092 D3): each platform from its own manifest digest,
# with the actual Free-plan memory limit and no network. Every named platform
# runs even when an earlier one fails, so an arm64 failure never hides
# production's amd64 result; the gate fails at the end if any platform failed.
#
# Usage: gate.sh <registry-ref> <index-digest> <major> <carried> <admitted> <version> [arch…]
#
# The architectures default to arm64 and amd64. The workflow gates each on
# its own native runner: under QEMU the emulator's own memory pushes two
# concurrent DuckDB scans past the 256 MiB cgroup, which says nothing about
# the image.
#
# Pulling the index digest twice with different --platform values fails on
# the runner's image store ("cannot overwrite digest", run 41188121287), so
# each platform runs from its own manifest digest.
set -euo pipefail

ref="$1"
index_digest="$2"
major="$3"
carried="$4"
admitted="$5"
version="$6"
shift 6
[ "$#" -gt 0 ] || set -- arm64 amd64
here="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
index="$(oras manifest fetch --plain-http "$ref@$index_digest")"
failed=""
for arch in "$@"; do
  digest="$(jq -er --arg a "$arch" \
    '[.manifests[] | select(.platform.os == "linux" and .platform.architecture == $a) | .digest] | if length == 1 then .[0] else error("want one \($a) manifest") end' <<<"$index")"
  echo "::group::PG${major} linux/${arch}"
  if docker run --rm --pull missing --platform "linux/$arch" --network none --memory 256m --memory-swap 256m \
    -v "$here:/check:ro" "$ref@$digest" bash /check/check.sh "$major" "$carried" "$admitted" "$version"; then
    verdict=PASS
  else
    verdict=FAIL
    failed="$failed $arch"
  fi
  echo "::endgroup::"
  echo "gate_${arch}=${verdict}"
done
if [ -n "$failed" ]; then
  echo "PG${major}: the operand gate failed on${failed}" >&2
  exit 1
fi
