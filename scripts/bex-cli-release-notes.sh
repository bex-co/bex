#!/usr/bin/env bash
# Render the polished release message for a bex CLI release.
#
#   VERSION=X.Y.Z bash scripts/bex-cli-release-notes.sh [--tap] [<new-ref>]
#
# Prints the GitHub release body to stdout — or, with --tap, the plain-text
# Homebrew tap commit message. The range is the previous bex-cli/v* release,
# never GitHub's auto-generated base: `--generate-notes` compares against the
# previous tag of any prefix, so a CLI release cut after a mobile or kubelet
# tag compared against the wrong base, and an idle one rendered a bare
# compare link. Only commits touching the CLI's own inputs are rightful here:
# lego/cli/**, scripts/bex-cli-*, scripts/install-bex*, and
# .github/workflows/cli-release.yml (the same set scripts/cli-release-staleness.sh
# watches). Backend, dashboard, and operator churn ships continuously and is
# never a CLI change.
#
# Within that set, feat/fix/perf (and an upstream render-oss/cli pin bump,
# whatever its type — it changes the binary users run) are meaningful to a
# reader and listed under What's new / Fixes; the rest is counted, not
# bulleted. Subjects print verbatim: the script groups, it never rewrites.
set -euo pipefail

tap=0
if [ "${1:-}" = "--tap" ]; then
  tap=1
  shift
fi

: "${VERSION:?set VERSION to the release version (X.Y.Z)}"
# VERSION is interpolated into Markdown, a commit message, and URLs; reject
# anything that is not a plain (optionally pre-release-suffixed) semver.
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] ||
  { echo "error: VERSION '$VERSION' is not a plain semver" >&2; exit 1; }

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# A shallow clone has no previous tag and no range to list, so it would
# render "no user-facing changes" for any release; refuse it instead.
if [ "$(git rev-parse --is-shallow-repository)" = true ]; then
  echo "error: shallow clone; the notes need every bex-cli/v* tag and the history between them (checkout fetch-depth: 0)" >&2
  exit 1
fi

new_ref="${1:-bex-cli/v${VERSION}}"
new="$(git rev-parse --verify --quiet "${new_ref}^{commit}")" ||
  { echo "error: no such ref '${new_ref}'" >&2; exit 1; }

# The previous release is the highest bex-cli/v* tag strictly below VERSION.
# Inserting VERSION into the sorted list and taking its predecessor skips an
# abandoned higher orphan tag (it reserves its version but shipped nothing)
# and never consults another component's prefix.
prev="$(git tag -l 'bex-cli/v*' | sed 's|^bex-cli/v||' |
  { printf '%s\n' "$VERSION"; cat; } | sort -V -u |
  awk -v ver="$VERSION" '$0 == ver { print prev; exit } { prev = $0 }')"
prev_tag=""
base=""
if [ -n "${prev:-}" ]; then
  prev_tag="bex-cli/v${prev}"
  if git merge-base --is-ancestor "$prev_tag" "$new" 2>/dev/null; then
    base="$prev_tag"
  else
    # A rewritten history leaves the tag off the new ref's ancestry (the same
    # case scripts/cli-release-staleness.sh handles); count from the fork
    # point instead of an unrelated merge-base's whole past. With no common
    # history at all, fall back to the first-release mode below.
    base="$(git merge-base "$prev_tag" "$new" 2>/dev/null)" || base=""
  fi
fi

# The CLI's own inputs — the same set the staleness reminder watches.
paths=(
  "lego/cli"
  "scripts/bex-cli-*"
  "scripts/install-bex*"
  ".github/workflows/cli-release.yml"
)
pin_paths=(
  "lego/cli/go.mod"
  "lego/cli/go.sum"
  "lego/cli/UPSTREAM_RENDER_CLI.md"
)

if [ -z "$base" ]; then
  range_spec="$new"
else
  range_spec="${base}..${new}"
fi

all="$(git log --reverse --format='%H%x09%s' "$range_spec" -- "${paths[@]}")"
pin_hashes="$(git log --format='%H' "$range_spec" -- "${pin_paths[@]}" 2>/dev/null || true)"

whats_new=()
fixes=()
internal=0
while IFS="$(printf '\t')" read -r hash subject; do
  [ -n "$hash" ] || continue
  lower="$(printf '%s' "$subject" | tr '[:upper:]' '[:lower:]')"
  type="${lower%%:*}"
  type="${type%%(*}"
  entry="${subject} ($(git rev-parse --short "$hash"))"
  if printf '%s' "$pin_hashes" | grep -qxF "$hash"; then
    whats_new+=("$entry")
  elif [ "$type" = "feat" ] || [ "$type" = "perf" ]; then
    whats_new+=("$entry")
  elif [ "$type" = "fix" ] || [ "$type" = "revert" ]; then
    fixes+=("$entry")
  else
    internal=$((internal + 1))
  fi
done <<<"$all"

compare_url="https://github.com/bex-co/bex/releases/tag/bex-cli%2Fv${VERSION}"
if [ -n "$prev_tag" ]; then
  compare_url="https://github.com/bex-co/bex/compare/${prev_tag}...bex-cli/v${VERSION}"
fi

if [ "$tap" = 1 ]; then
  printf 'bex %s\n' "$VERSION"
  printf '\n'
  if [ "${#whats_new[@]}" -eq 0 ] && [ "${#fixes[@]}" -eq 0 ]; then
    printf 'No user-facing CLI changes in this release.\n'
  else
    if [ "${#whats_new[@]}" -gt 0 ]; then
      printf 'What'"'"'s new:\n'
      printf -- '- %s\n' "${whats_new[@]}"
    fi
    if [ "${#fixes[@]}" -gt 0 ]; then
      printf 'Fixes:\n'
      printf -- '- %s\n' "${fixes[@]}"
    fi
  fi
  printf '\nFull notes: %s\n' "$compare_url"
  exit 0
fi

if [ "${#whats_new[@]}" -gt 0 ]; then
  printf "## What's new\n\n"
  printf -- '- %s\n' "${whats_new[@]}"
  printf '\n'
fi
if [ "${#fixes[@]}" -gt 0 ]; then
  printf '## Fixes\n\n'
  printf -- '- %s\n' "${fixes[@]}"
  printf '\n'
fi
if [ "${#whats_new[@]}" -eq 0 ] && [ "${#fixes[@]}" -eq 0 ]; then
  printf 'No user-facing CLI changes in this release.\n\n'
elif [ "$internal" -gt 0 ]; then
  printf 'Also in this release: %d further CLI-internal change(s) (tests, docs, refactors).\n\n' "$internal"
fi

cat <<EOF
## Install or upgrade

- Fresh install: \`curl -fsSL https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh | sh\`
- In place: \`bex upgrade\` (a raw-binary install) or \`brew upgrade bex\` (a Homebrew install)
- Pin this version: \`BEX_VERSION=${VERSION} sh scripts/install-bex.sh\`

## Verify

\`\`\`bash
cosign verify-blob checksums.txt \\
  --bundle checksums.txt.sigstore.json \\
  --certificate-identity-regexp 'github.com/bex-co/bex/.github/workflows/cli-release.yml' \\
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
\`\`\`

**Full changelog**: ${compare_url}
EOF
