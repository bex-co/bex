#!/usr/bin/env bash
# opensandbox-server-image.sh <image> <server version> <platform> <signer identity>
#
# Decides whether a deploy can pin the OpenSandbox lifecycle server image an
# earlier deploy built, and prints the answer as GitHub step outputs:
#
#   tag=inputs-<key>         the tag that names this image's build inputs
#   digest=sha256:<64 hex>   the image to reuse; empty when one must be built
#
# Why (w8/254): the server is one Recreate replica on a ReadWriteOnce store, so
# a new digest restarts it, and for that window every sandbox call in every
# workspace answers 503. The build is not reproducible (its first layer runs
# apt against a moving mirror), so building on every deploy pinned a new digest
# every time, with nothing in the image changed.
#
# The key covers what the image is built from: the Dockerfile (which pins the
# base image by digest), the dependency lock, the server version, the platform
# and the ISO week. The week keeps Debian's security upgrades arriving: the
# first deploy of a week builds, the later ones reuse. A Dockerfile that copies
# anything but the lock has an input the key does not cover, so it always
# builds.
#
# An image is reused only when this workflow's own run on main signed it (the
# signer identity, verified with cosign as the deploy signs, keyless): the
# inputs tag is predictable, and a tag any other writer pushed must never reach
# production. It must also still pass the deploy's own CRITICAL CVE gate; one
# that fails is rebuilt on the spot, so a fix Debian has published reaches
# production with that deploy and not at the turn of the week. Any failure to
# look the image up, verify it or scan it also means "build": the cost is one
# restart, never a stale, foreign or unscanned image.
#
# Needs docker (logged in to the registry), cosign, and trivy with its database
# already downloaded. Messages go to stderr; stdout carries only the outputs.
set -euo pipefail

usage="usage: opensandbox-server-image.sh <image> <server version> <platform> <signer identity>"
image="${1:?$usage}"
version="${2:?$usage}"
platform="${3:?$usage}"
signer="${4:?$usage}"
context="${BEX_OPENSANDBOX_CONTEXT:-$(cd "$(dirname "$0")/.." && pwd)/deploy/opensandbox}"
stamp="${BEX_IMAGE_REFRESH_STAMP:-$(date -u +%G-W%V)}"

# A key computed without one of its inputs would name a different image.
for input in "$context/server.Dockerfile" "$context/requirements.lock"; do
  [ -s "$input" ] || { echo "::error::OpenSandbox server build input $input is missing or empty" >&2; exit 1; }
done
key="$({
  sha256sum <"$context/server.Dockerfile"
  sha256sum <"$context/requirements.lock"
  printf 'version=%s\nplatform=%s\nrefresh=%s\n' "$version" "$platform" "$stamp"
} | sha256sum | cut -c1-16)"
tag="inputs-$key"
echo "tag=$tag"

if grep -Ei '^[[:space:]]*(COPY|ADD)[[:space:]]' "$context/server.Dockerfile" | grep -qv 'requirements\.lock'; then
  echo "::warning::server.Dockerfile copies files the reuse key does not cover; building" >&2
  echo "digest="
  exit 0
fi

manifest="$(timeout 60 docker buildx imagetools inspect "$image:$tag" --format '{{json .Manifest}}')" || manifest=""
digest="$(printf '%s' "$manifest" |
  python3 -c 'import json, sys; print(json.load(sys.stdin).get("digest", ""))' 2>/dev/null)" || digest=""
if ! [[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "no image is published as $image:$tag (refresh $stamp); building one" >&2
  echo "digest="
  exit 0
fi

if ! timeout 120 cosign verify --certificate-identity "$signer" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com "$image@$digest" >/dev/null 2>&1; then
  echo "::warning::$image:$tag ($digest) is not signed by $signer; building" >&2
  echo "digest="
  exit 0
fi

# Exit 3 is a CRITICAL finding; any other failure is the scan itself.
rc=0
trivy image --exit-code 3 --severity CRITICAL --ignore-unfixed "$image@$digest" >&2 || rc=$?
case "$rc" in
  0)
    echo "reusing $image@$digest, built from the same inputs ($tag, refresh $stamp)" >&2
    echo "digest=$digest"
    ;;
  3)
    echo "::warning::$image@$digest no longer passes the CRITICAL CVE gate; rebuilding it" >&2
    echo "digest="
    ;;
  *)
    echo "::warning::scanning $image@$digest failed (trivy exit $rc); building" >&2
    echo "digest="
    ;;
esac
