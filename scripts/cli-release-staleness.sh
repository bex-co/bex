#!/usr/bin/env bash
# Reports whether the CLI has unreleased commits: any commit touching
# lego/cli/**, scripts/bex-cli-*, scripts/install-bex*, or
# .github/workflows/cli-release.yml
# newer than the latest bex-cli/v* tag (w4/032.md piece 1 — releasing depends
# on this reminder, not on memory). Exit 0 with no output when up to date;
# exit 1 with the stale commit list on stdout when a release is owed. Requires
# no network access — pure `git log`/`git tag` against the checked-out history
# (the workflow's checkout must use fetch-depth: 0).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

latest_tag="$(git tag -l 'bex-cli/v*' --sort=-v:refname | head -n1)"

if [[ -z "$latest_tag" ]]; then
  echo "no bex-cli/v* tag exists yet — nothing to compare against" >&2
  exit 0
fi

paths=(
  "lego/cli/**"
  "scripts/bex-cli-*"
  "scripts/install-bex*"
  ".github/workflows/cli-release.yml"
)

stale="$(git log --oneline "${latest_tag}..HEAD" -- "${paths[@]}")"

if [[ -z "$stale" ]]; then
  exit 0
fi

echo "CLI has unreleased commits since ${latest_tag}:"
echo "$stale"
exit 1
