#!/usr/bin/env bash
# image-digest-exists.sh <ref@sha256:...> — does that digest exist in THAT
# repository? Exits 0 if the registry serves a manifest for it, non-zero if not.
#
# This is the check that catches a pin copy-pasted from the wrong image — the
# one failure mode a code review cannot see, because a wrong 64-hex string looks
# exactly like a right one (w7/m85 t007).
#
# It deliberately does NOT compare the digest against what the tag resolves to
# today. A moved tag is the pin doing its job; asserting otherwise would turn
# every upstream rebuild of `alpine:3.21` into a red build.
#
# Registry auth is negotiated from the WWW-Authenticate challenge rather than
# hardcoded per host: bex pins images from Docker Hub, ghcr.io, quay.io,
# registry.k8s.io, gcr.io and an Aliyun registry, and each answers a different
# way. Anonymous pull only — every reference bex pins is public.
#
# Used by scripts/image-pin-validate.sh --verify-digests, a manual network check
# rather than a merge gate.
set -euo pipefail

ref="${1:?usage: image-digest-exists.sh <name[:tag]@sha256:...>}"
case "$ref" in
  *@sha256:*) ;;
  *) echo "not a digest reference: $ref" >&2; exit 2 ;;
esac

digest="${ref##*@}"
name="${ref%@*}"
name="${name%%:*}"

# Split registry host from repository. A first component with a dot or a port is
# a hostname; anything else is a Docker Hub repository, with the implicit
# library/ namespace for single-word names.
first="${name%%/*}"
if [ "$first" != "$name" ] && [[ "$first" == *.* || "$first" == *:* ]]; then
  registry="$first"
  repo="${name#*/}"
  [ "$registry" = "docker.io" ] && registry="registry-1.docker.io"
else
  registry="registry-1.docker.io"
  repo="$name"
fi
if [ "$registry" = "registry-1.docker.io" ]; then
  case "$repo" in */*) ;; *) repo="library/$repo" ;; esac
fi

url="https://${registry}/v2/${repo}/manifests/${digest}"
# All four media types: a multi-arch reference is an index, a single-platform
# one a manifest, and both exist in OCI and Docker flavors. Omitting any turns a
# valid pin into a 404.
accept=(
  -H 'Accept: application/vnd.oci.image.index.v1+json'
  -H 'Accept: application/vnd.docker.distribution.manifest.list.v2+json'
  -H 'Accept: application/vnd.oci.image.manifest.v1+json'
  -H 'Accept: application/vnd.docker.distribution.manifest.v2+json'
)

headers="$(curl -sSL -o /dev/null -D - "${accept[@]}" "$url" 2>/dev/null || true)"
status="$(printf '%s' "$headers" | awk 'toupper($1) ~ /^HTTP/ {code=$2} END {print code}')"

if [ "$status" = "401" ]; then
  # Bearer realm="https://…/token",service="…"[,scope="…"] — take the realm and
  # service the registry itself names, and ask for pull scope on this repo.
  challenge="$(printf '%s' "$headers" | tr -d '\r' |
    awk 'tolower($1) == "www-authenticate:" { sub(/^[^ ]+ /, ""); print; exit }')"
  realm="$(printf '%s' "$challenge" | sed -n 's/.*realm="\([^"]*\)".*/\1/p')"
  service="$(printf '%s' "$challenge" | sed -n 's/.*service="\([^"]*\)".*/\1/p')"
  if [ -z "$realm" ]; then
    echo "$ref -> HTTP 401 with no usable auth challenge: $challenge" >&2
    exit 1
  fi
  token_url="${realm}?scope=repository:${repo}:pull"
  [ -n "$service" ] && token_url="${token_url}&service=${service}"
  token="$(curl -fsSL "$token_url" |
    python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("token") or d.get("access_token",""))')"
  if [ -z "$token" ]; then
    echo "$ref -> anonymous pull token refused by $realm" >&2
    exit 1
  fi
  status="$(curl -sSL -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer $token" "${accept[@]}" "$url")"
fi

[ "$status" = "200" ] || { echo "$ref -> HTTP $status" >&2; exit 1; }
