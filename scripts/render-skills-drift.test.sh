#!/usr/bin/env bash
# Unit tests for render-skills-drift.sh. Network-free: the upstream is a git
# repository built here, shaped like render-oss/skills, and the pin file is the
# committed packgen.go with only its UpstreamCommit replaced, so the script's
# parsing of the real file is under test too. What is checked: which upstream
# commits count as unreviewed, and that a pin it cannot read or find fails
# with a message instead of passing.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-skills-drift.sh"
committed_packgen="$repo_root/lego/cli/internal/skills/packgen/packgen.go"

failures=0
check() { # check <name> <expected exit> <expected text> <command...>
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -8 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF -- "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -8 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}
refute() { # refute <name> <text that must be absent> <command...>
  local name="$1" text="$2" output
  shift 2
  output="$("$@" 2>&1)" || true
  if grep -qF -- "$text" <<<"$output"; then
    echo "FAIL: $name — output contains '$text'" >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

sandbox="$(mktemp -d -t bex-skills-drift-test.XXXXXX)"
trap 'rm -rf "$sandbox"' EXIT
mkdir "$sandbox/tmp"
upstream="$sandbox/skills"

export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com
git init --quiet --initial-branch=main "$upstream"
commit() { # commit <file> <subject>: change one file of the fixture upstream
  mkdir -p "$(dirname "$upstream/$1")"
  echo "$2" >>"$upstream/$1"
  git -C "$upstream" add -A
  git -C "$upstream" -c commit.gpgsign=false commit --quiet -m "$2"
  git -C "$upstream" rev-parse --short HEAD
}

# pinned_at <sha>: the committed packgen.go pinned to a fixture commit.
pinned_at() {
  sed "s/^const UpstreamCommit = \".*\"$/const UpstreamCommit = \"$1\"/" "$committed_packgen" >"$sandbox/packgen-$1.go"
  echo "$sandbox/packgen-$1.go"
}
run_with() { # run_with <packgen.go>
  TMPDIR="$sandbox/tmp" RENDER_SKILLS_PACKGEN="$1" RENDER_SKILLS_REPO_URL="$upstream" "$script"
}

commit LICENSE "MIT" >/dev/null
pin="$(commit skills/render-deploy/SKILL.md "Initial skills")"

check "no commit after the pin passes" 0 "no commit after the pin $pin changes a skill bex ships" \
  run_with "$(pinned_at "$pin")"

# Commits that change nothing packgen reads: a dropped skill (read from the
# committed droppedSkills, so both must stay skipped), the plugin manifest,
# hooks and CI.
commit skills/render-workflows/SKILL.md "Workflows sleep guidance" >/dev/null
commit skills/render-sandboxes/SKILL.md "Sandboxes skill" >/dev/null
commit .claude-plugin/plugin.json "Plugin version" >/dev/null
commit hooks/approve.sh "Hook tweak" >/dev/null
head="$(commit .github/workflows/catalog.yml "Catalog refresh")"
check "dropped skills, plugin, hooks and CI changes pass" 0 "upstream head $head; skipped: render-workflows render-sandboxes" \
  run_with "$(pinned_at "$pin")"

deploy="$(commit skills/render-deploy/SKILL.md "Deploy Blueprints through MCP and CLI")"
check "an unreviewed skill commit fails and is named" 1 "  $deploy " run_with "$(pinned_at "$pin")"
refute "the skipped commits are not listed" "Workflows sleep guidance" run_with "$(pinned_at "$pin")"
license="$(commit LICENSE "Relicense")"
check "a LICENSE change counts (packgen copies it)" 1 "2 upstream commit(s) after the pin $pin" run_with "$(pinned_at "$pin")"
check "the reviewed head is named for the bump" 1 "reviewed head ($license)" run_with "$(pinned_at "$pin")"
check "pinning the reviewed head passes" 0 "no commit after the pin $license" run_with "$(pinned_at "$license")"

check "a pin upstream does not have fails" 1 "is not a commit of" run_with "$(pinned_at 0000000)"
sed '/^const UpstreamCommit/d' "$committed_packgen" >"$sandbox/no-pin.go"
check "a missing pin line fails" 1 "Skills pin unreadable" run_with "$sandbox/no-pin.go"
sed '/^var droppedSkills/d' "$(pinned_at "$pin")" >"$sandbox/no-dropped.go"
check "a missing droppedSkills line fails" 1 "Dropped skills unreadable" run_with "$sandbox/no-dropped.go"
check "an unreachable upstream fails" 1 "render-oss/skills unreachable" \
  env TMPDIR="$sandbox/tmp" RENDER_SKILLS_PACKGEN="$(pinned_at "$pin")" RENDER_SKILLS_REPO_URL="$sandbox/absent" "$script"

if [[ -n "$(ls -A "$sandbox/tmp")" ]]; then
  echo "FAIL: the script left files in its temporary directory" >&2
  failures=$((failures + 1))
else
  echo "ok: the temporary clone is removed"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "all render-skills-drift checks passed"
