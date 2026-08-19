#!/usr/bin/env bash
# Build-toolchain freshness inventory: validate committed pins, resolve
# upstream digests without editing the tree, and emit the open/update/close
# decision for the tracking issue.
#
# This wrapper never receives cluster, registry-push, signing, or tenant
# credentials. It only reads the repo and (for resolve) public registry metadata.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 "$repo_root/scripts/lib/toolchain-freshness.py" "$@"
