#!/usr/bin/env bash
# Reports whether the CLI has unreleased commits: any commit touching
# lego/cli/**, scripts/bex-cli-*, scripts/install-bex*, or
# .github/workflows/cli-release.yml
# newer than the latest bex-cli/v* tag (w4/032.md piece 1 — releasing depends
# on this reminder, not on memory). Exit 0 with no output when up to date;
# exit 1 with the stale commit list on stdout when a release is owed; exit 2
# when the tag is off HEAD's history with no rewritten copy to count from. Requires
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

# A history rewrite leaves a release tag on
# a commit main no longer contains, and `tag..HEAD` then counts back to an old
# merge-base. Count from main's rewritten copy instead: the same author time
# and subject, which a rewrite preserves.
base="$latest_tag"
if ! git merge-base --is-ancestor "$latest_tag" HEAD; then
  tagged="$(git log -1 --format='%at%x09%s' "${latest_tag}^{commit}")"
  base="$(git log --format='%H%x09%at%x09%s' HEAD |
    awk -v want="$tagged" '!found && substr($0, index($0, "\t") + 1) == want { print substr($0, 1, 40); found = 1 }')"
  if [[ -z "$base" ]]; then
    echo "${latest_tag} is not on HEAD's history and no rewritten copy of it was found;" \
      "cannot tell which CLI commits are unreleased" >&2
    exit 2
  fi
  echo "note: ${latest_tag} is not on HEAD's history; counting from its rewritten copy ${base:0:12}" >&2
fi

stale="$(git log --oneline "${base}..HEAD" -- "${paths[@]}")"

if [[ -z "$stale" ]]; then
  exit 0
fi

echo "CLI has unreleased commits since ${latest_tag}:"
echo "$stale"
exit 1
