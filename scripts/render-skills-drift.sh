#!/usr/bin/env bash
# Check, but never update, that bex's agent skills pack is derived from the
# newest render-oss/skills commit that changes a skill bex ships (w4/m630).
#
# `bex skills install` installs a pack embedded in the CLI and generated from
# render-oss/skills at packgen.UpstreamCommit (w8/m143). Nothing watched that
# repository, so the pin fell five commits behind, one of them Render's
# rewrite of the deploy skill around Blueprint plan and approve (#81). This
# job fails while upstream has a commit after the pin that touches what packgen
# reads: skills/** outside the skills bex drops, or LICENSE. Changes to the
# plugin manifests, hooks or CI are not part of the pack and pass.
#
# To clear it: review the listed commits, bump UpstreamCommit in packgen.go,
# run lego/cli/hack/sync-skills.sh and review the regenerated pack.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
packgen="${RENDER_SKILLS_PACKGEN:-$repo_root/lego/cli/internal/skills/packgen/packgen.go}"
upstream="${RENDER_SKILLS_REPO_URL:-https://github.com/render-oss/skills.git}"

if [[ ! -f "$packgen" ]]; then
  echo "::error title=Skills pin missing::$packgen does not exist" >&2
  exit 1
fi
pin="$(sed -n 's/^const UpstreamCommit = "\([0-9a-f]\{7,40\}\)"$/\1/p' "$packgen")"
if [[ -z "$pin" ]]; then
  echo "::error title=Skills pin unreadable::no 'const UpstreamCommit = \"<sha>\"' line in $packgen" >&2
  exit 1
fi
# The skills packgen drops (droppedSkills) change nothing bex ships.
dropped=()
while IFS= read -r skill; do
  dropped+=("$skill")
done < <(grep '^var droppedSkills = ' "$packgen" | grep -o '"[a-z0-9-]*": *true' | cut -d'"' -f2)
if [[ ${#dropped[@]} -eq 0 ]]; then
  echo "::error title=Dropped skills unreadable::no 'var droppedSkills = map[string]bool{...}' line in $packgen" >&2
  exit 1
fi

# Under $TMPDIR on every platform (macOS mktemp -t ignores it), so the test can
# assert the cleanup.
work="$(mktemp -d "${TMPDIR:-/tmp}/bex-render-skills.XXXXXX")"
trap 'rm -rf "$work"' EXIT

if ! git clone --quiet --bare "$upstream" "$work/skills.git" 2>"$work/clone.err"; then
  echo "::error title=render-oss/skills unreachable::could not clone $upstream: $(head -c 300 "$work/clone.err")" >&2
  exit 1
fi
if ! git -C "$work/skills.git" rev-parse --quiet --verify "$pin^{commit}" >/dev/null; then
  echo "::error title=Skills pin not upstream::$pin (packgen.UpstreamCommit) is not a commit of $upstream; was upstream rewritten?" >&2
  exit 1
fi

pathspec=(skills LICENSE)
for skill in "${dropped[@]}"; do
  pathspec+=(":(exclude)skills/$skill")
done
unreviewed="$(git -C "$work/skills.git" log --format='%h %ad %s' --date=short "$pin..HEAD" -- "${pathspec[@]}")"
upstream_head="$(git -C "$work/skills.git" rev-parse --short HEAD)"

if [[ -n "$unreviewed" ]]; then
  count="$(wc -l <<<"$unreviewed" | tr -d ' ')"
  echo "::error title=render-oss/skills drift::$count upstream commit(s) after the pin $pin change skills bex ships" >&2
  sed 's/^/  /' <<<"$unreviewed" >&2
  cat >&2 <<EOF

Action: review each commit (git -C <checkout> show <sha> -- skills LICENSE), then
set UpstreamCommit in lego/cli/internal/skills/packgen/packgen.go to the
reviewed head ($upstream_head), run lego/cli/hack/sync-skills.sh, and fix what
'cd lego/cli && go test ./internal/skills/...' then refuses (a stale patch, a
Render name or version floor) with packgen rules. Skipped: ${dropped[*]}.
This job never edits the pin.
EOF
  exit 1
fi
echo "render-oss/skills: no commit after the pin $pin changes a skill bex ships (upstream head $upstream_head; skipped: ${dropped[*]})"
