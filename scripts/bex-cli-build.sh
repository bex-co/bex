#!/usr/bin/env bash
# Build all supported native-launcher targets with the pinned upstream version.
set -euo pipefail
cd "$(dirname "$0")/.."

output_dir="${1:?usage: scripts/bex-cli-build.sh <output-directory>}"
readonly upstream_version=2.22.0
readonly version_flag="-X github.com/render-oss/cli/pkg/cfg.Version=$upstream_version"

mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  (
    cd cli
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
      -ldflags "$version_flag" -o "$output_dir/bex-${os}-${arch}" .
  )
done
