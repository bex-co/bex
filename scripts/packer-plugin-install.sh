#!/usr/bin/env bash
# Install the exact hcloud Packer plugin infra/packer/bex-worker.pkr.hcl pins,
# verifying the downloaded artifact against the repository-pinned SHA-256 in
# infra/packer/plugin-checksums.txt BEFORE it is installed or executed (w1/m66
# F12, from the 2026-08-10 codex-security scan).
#
# Replaces a bare `packer init` in the credentialed snapshot workflow: `init`
# resolves a version constraint over the network and — since Packer 1.14 dropped
# .packer.lock.hcl — records no checksum anywhere, so identical repository
# revisions could execute different plugin bytes in a job holding HCLOUD_TOKEN.
#
# Usage:  bash scripts/packer-plugin-install.sh [version]
#         PACKER_HCLOUD_PLUGIN_VERSION=1.7.2 bash scripts/packer-plugin-install.sh
#
# Exits non-zero (installing nothing) if the artifact is missing from the pin
# file or its checksum does not match.
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CHECKSUMS="$REPO_ROOT/infra/packer/plugin-checksums.txt"
TEMPLATE="$REPO_ROOT/infra/packer/bex-worker.pkr.hcl"

# The plugin API version Packer encodes in every artifact name.
PLUGIN_API="x5.0"
SOURCE="github.com/hetznercloud/hcloud"

VERSION="${1:-${PACKER_HCLOUD_PLUGIN_VERSION:-}}"
if [ -z "$VERSION" ]; then
  # Default to whatever the template pins, so the two can never disagree.
  VERSION=$(sed -n 's/^[[:space:]]*version[[:space:]]*=[[:space:]]*"=[[:space:]]*\([0-9.]*\)".*/\1/p' "$TEMPLATE" | head -1)
fi
if [ -z "$VERSION" ]; then
  echo "cannot determine the pinned plugin version; pass it explicitly" >&2
  exit 1
fi

# The template must pin this EXACT version — an open constraint would let
# `packer build` load something other than what was just verified here.
if ! grep -qE "version[[:space:]]*=[[:space:]]*\"=[[:space:]]*${VERSION//./\\.}\"" "$TEMPLATE"; then
  echo "::error::$TEMPLATE does not pin the hcloud plugin to exactly ${VERSION}" >&2
  exit 1
fi

case "$(uname -s)" in
Linux) OS=linux ;;
Darwin) OS=darwin ;;
*)
  echo "unsupported OS: $(uname -s)" >&2
  exit 1
  ;;
esac
case "$(uname -m)" in
x86_64 | amd64) ARCH=amd64 ;;
arm64 | aarch64) ARCH=arm64 ;;
*)
  echo "unsupported architecture: $(uname -m)" >&2
  exit 1
  ;;
esac

ARTIFACT="packer-plugin-hcloud_v${VERSION}_${PLUGIN_API}_${OS}_${ARCH}.zip"
BINARY="packer-plugin-hcloud_v${VERSION}_${PLUGIN_API}_${OS}_${ARCH}"

WANT=$(awk -v want="$ARTIFACT" '$1 !~ /^#/ && $2 == want { print $1 }' "$CHECKSUMS")
if [ -z "$WANT" ]; then
  echo "::error::no pinned checksum for ${ARTIFACT} in ${CHECKSUMS} — add the reviewed digest before installing" >&2
  exit 1
fi

TMP=$(mktemp -d)
cleanup() { rm -rf -- "$TMP"; }
trap cleanup EXIT

URL="https://github.com/hetznercloud/packer-plugin-hcloud/releases/download/v${VERSION}/${ARTIFACT}"
echo "downloading ${ARTIFACT}"
curl -fsSLo "$TMP/$ARTIFACT" "$URL"

if command -v sha256sum >/dev/null 2>&1; then
  GOT=$(sha256sum "$TMP/$ARTIFACT" | awk '{print $1}')
else
  GOT=$(shasum -a 256 "$TMP/$ARTIFACT" | awk '{print $1}')
fi

if [ "$GOT" != "$WANT" ]; then
  echo "::error::checksum mismatch for ${ARTIFACT}" >&2
  echo "  pinned:   $WANT" >&2
  echo "  received: $GOT" >&2
  exit 1
fi
echo "checksum OK (${WANT})"

if ! python3 - "$TMP/$ARTIFACT" "$TMP/$BINARY" "$BINARY" <<'PY'
import shutil
import sys
from zipfile import ZipFile

artifact, target, member = sys.argv[1:]
try:
    with ZipFile(artifact) as archive:
        with archive.open(member) as source, open(target, "wb") as output:
            shutil.copyfileobj(source, output)
except (KeyError, OSError):
    raise SystemExit(1)
PY
then
  echo "::error::archive did not contain the expected binary ${BINARY}" >&2
  exit 1
fi
if [ ! -f "$TMP/$BINARY" ]; then
  echo "::error::archive did not contain the expected binary ${BINARY}" >&2
  ls -la "$TMP" >&2
  exit 1
fi
chmod +x "$TMP/$BINARY"

packer plugins install --path "$TMP/$BINARY" "$SOURCE"
packer plugins installed
