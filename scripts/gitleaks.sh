#!/usr/bin/env bash
# gitleaks.sh — the one pinned Gitleaks CLI behind every secret scan of this
# public repository (ADR019 §6).
#
#   bash scripts/gitleaks.sh <git-log-range>    # e.g. origin/main..HEAD
#
# Scans the patches of the range's commits against the default rules plus the
# .gitleaks.toml committed at the range's tip, so commits are judged by the
# config they ship with, never by an uncommitted edit. Findings print
# redacted. Exit 0: clean or no commits; 1: a finding; 2: no scan.
#
# The binary is the release below, checksum-verified, downloaded once into
# ${BEX_GITLEAKS_CACHE:-~/.cache/bex-gitleaks}. BEX_GITLEAKS_BIN runs another
# binary instead (the tests' stub). The official open-source CLI, not
# gitleaks-action: the action needs a separate license for organization
# repositories.
set -euo pipefail
# Any unexpected failure means no scan, never a finding.
trap 'exit 2' ERR

readonly version=8.30.1
range=${1:?usage: $0 <git-log-range>}
here=$(cd "$(dirname "$0")" && pwd)
root=$(git rev-parse --show-toplevel)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

[ "$(git -C "$root" rev-list --count "$range")" != 0 ] || exit 0
# Gitleaks reports a git error as a clean scan, so git reads the range first:
# a bad range, or a blob a partial clone cannot fetch, fails here instead.
git -C "$root" log -p --format= "$range" >/dev/null
git -C "$root" show "${range##*..}:.gitleaks.toml" >"$tmp/config.toml" 2>/dev/null \
  || printf '[extend]\nuseDefault = true\n' >"$tmp/config.toml"

bin=${BEX_GITLEAKS_BIN:-}
if [ -z "$bin" ]; then
  case "$(uname -s)_$(uname -m)" in
    Darwin_arm64) platform=darwin_arm64 sha=b40ab0ae55c505963e365f271a8d3846efbc170aa17f2607f13df610a9aeb6a5 ;;
    Darwin_x86_64) platform=darwin_x64 sha=dfe101a4db2255fc85120ac7f3d25e4342c3c20cf749f2c20a18081af1952709 ;;
    Linux_aarch64 | Linux_arm64) platform=linux_arm64 sha=e4a487ee7ccd7d3a7f7ec08657610aa3606637dab924210b3aee62570fb4b080 ;;
    Linux_x86_64) platform=linux_x64 sha=551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb ;;
    *)
      echo "gitleaks.sh: no pinned Gitleaks build for $(uname -sm)" >&2
      exit 2
      ;;
  esac
  cache="${BEX_GITLEAKS_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/bex-gitleaks}/$version"
  bin="$cache/gitleaks"
  if [ ! -x "$bin" ]; then
    if ! bash "$here/ci-fetch-verified.sh" \
      "https://github.com/gitleaks/gitleaks/releases/download/v${version}/gitleaks_${version}_${platform}.tar.gz" \
      "$sha" "$tmp/gitleaks.tar.gz" 4 >&2; then
      echo "gitleaks.sh: could not fetch Gitleaks $version for $platform" >&2
      exit 2
    fi
    tar -xzf "$tmp/gitleaks.tar.gz" -C "$tmp" gitleaks
    mkdir -p "$cache"
    # Renamed into place, so a concurrent scan never runs a half-written file.
    mv "$tmp/gitleaks" "$bin.$$"
    chmod +x "$bin.$$"
    mv -f "$bin.$$" "$bin"
  fi
fi

# Findings exit 3, so a scanner error (exit 1) is never mistaken for a leak.
rc=0
"$bin" git --no-banner --redact --exit-code 3 --config "$tmp/config.toml" --log-opts="$range" "$root" || rc=$?
case "$rc" in
  0) exit 0 ;;
  3) exit 1 ;;
  *)
    echo "gitleaks.sh: the scan itself failed (exit $rc)" >&2
    exit 2
    ;;
esac
