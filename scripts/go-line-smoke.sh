#!/usr/bin/env bash
# go-line-smoke.sh <golang:<line>-bookworm@sha256:…> <go release>
#
# Before go-line-bump.yml pins a new native Go image (w4/719), prove on this
# runner's architecture that the image is the release the plan read from its
# config, and that its own toolchain builds a module declaring that release
# with the network off: the image sets GOTOOLCHAIN=local, which is what makes
# a go.mod newer than the line fail, so nothing may be downloaded instead.
# The workflow runs it on ubuntu-24.04-arm and on ubuntu-latest, the two
# architectures the official index publishes and native builds run on.
set -euo pipefail

usage="usage: go-line-smoke.sh <golang:<line>-bookworm@sha256:…> <go release>"
image="${1:?$usage}"
release="${2:?$usage}"
if ! [[ "$image" =~ ^golang:[0-9]+\.[0-9]+-bookworm@sha256:[0-9a-f]{64}$ ]]; then
  echo "FAIL: $image is not a digest-pinned golang:<line>-bookworm image" >&2
  exit 1
fi
if ! [[ "$release" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "FAIL: $release is not a Go release (want major.minor.patch)" >&2
  exit 1
fi
line="${image#golang:}"
line="${line%%-bookworm@*}"
if [ "${release%.*}" != "$line" ]; then
  echo "FAIL: Go $release is not on the image's $line line" >&2
  exit 1
fi
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "FAIL: no official golang image for host $(uname -m)" >&2
    exit 1
    ;;
esac

# Docker Hub pulls stall now and then; the run itself must not be the retry.
for attempt in 1 2 3; do
  docker pull --quiet --platform "linux/$arch" "$image" >/dev/null && break
  if [ "$attempt" -eq 3 ]; then
    echo "FAIL: could not pull $image for linux/$arch" >&2
    exit 1
  fi
  sleep $((attempt * ${GO_SMOKE_RETRY_PAUSE:-10}))
done

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
printf 'module example.com/smoke\n\ngo %s\n' "$release" >"$work/go.mod"
printf 'package main\n\nimport "fmt"\n\nfunc main() { fmt.Println("smoke ok") }\n' >"$work/main.go"
set +e
out="$(docker run --rm --platform "linux/$arch" --network none -v "$work:/src:ro" -w /src "$image" \
  sh -c 'go version && go env GOTOOLCHAIN && go build -o /tmp/smoke . && /tmp/smoke' 2>&1)"
rc=$?
set -e
want="go version go$release linux/$arch
local
smoke ok"
if [ "$rc" -ne 0 ] || [ "$out" != "$want" ]; then
  echo "FAIL: $image on linux/$arch (exit $rc) printed:" >&2
  printf '%s\n' "$out" >&2
  echo "want:" >&2
  printf '%s\n' "$want" >&2
  exit 1
fi
echo "ok: $image is Go $release on linux/$arch and builds a go $release module offline"
