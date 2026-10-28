#!/usr/bin/env bash
# Regenerates lego/cli/internal/skills/pack from render-oss/skills at the
# commit pinned in internal/skills/packgen (w8/m143). Re-run after changing the
# pin or the rewrite rules, then review the diff.
set -euo pipefail
cd "$(dirname "$0")/.."
commit="$(sed -n 's/^const UpstreamCommit = "\(.*\)"$/\1/p' internal/skills/packgen/packgen.go)"
src="$(mktemp -d)"
trap 'rm -rf "$src"' EXIT
git clone -q https://github.com/render-oss/skills.git "$src"
git -C "$src" checkout -q "$commit"
go run ./hack/skillspack "$src" internal/skills/pack
