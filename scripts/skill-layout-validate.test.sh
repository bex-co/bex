#!/usr/bin/env bash
# Self-test for scripts/skill-layout-validate.sh (w4/729). A gate that was never
# seen to fail proves nothing: each fixture is a scratch repository holding a
# copy of the validator, and every rule that keeps maintainer skills out of
# `npx skills add bex-co/bex` has its own red case that must name the skill.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/skill-layout-validate.sh"

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# skill <repo> <name> [frontmatter lines...] — a canonical skill and its Codex
# bridge. The lines go between `name:` and `description:`, where the
# repository's skills keep the marker.
skill() {
  local repo="$1" name="$2"
  shift 2
  mkdir -p "$repo/.claude/skills/$name"
  {
    printf -- '---\nname: %s\n' "$name"
    if [ "$#" -gt 0 ]; then
      printf '%s\n' "$@"
    fi
    printf -- 'description: >-\n  Does %s things.\n---\n\n# %s\n' "$name" "$name"
  } >"$repo/.claude/skills/$name/SKILL.md"
  ln -s "../../.claude/skills/$name" "$repo/.agents/skills/$name"
}

# new <label> — a repository that passes: one maintainer skill with the marker
# and one vendored skill named in skills-lock.json.
new() {
  local repo="$tmp/$1"
  mkdir -p "$repo/scripts" "$repo/.agents/skills"
  cp "$SCRIPT" "$repo/scripts/"
  skill "$repo" ship 'metadata:' '  internal: "true"' 'allowed-tools: Bash(git status:*)'
  skill "$repo" vendored-audit
  printf '{"version": 1, "skills": {"vendored-audit": {"source": "example/skills"}}}\n' >"$repo/skills-lock.json"
  printf '%s' "$repo"
}

# assert <label> <want-rc> <repo> [expected-stderr-substring]
assert() {
  local label="$1" want="$2" repo="$3" needle="${4:-}" err got
  set +e
  err="$(bash "$repo/scripts/skill-layout-validate.sh" 2>&1 >/dev/null)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    printf '%s\n' "$err" | sed 's/^/        /' >&2
    fails=$((fails + 1))
    return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$err" | grep -qF -- "$needle"; then
    echo "FAIL: $label — stderr did not name '$needle'" >&2
    printf '%s\n' "$err" | sed 's/^/        /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label (exit $got)"
}

repo="$(new pass)"
assert "a marked maintainer skill and a vendored skill pass" 0 "$repo"

repo="$(new unmarked)"
skill "$repo" new-routine
assert "a new maintainer skill without the marker fails" 1 "$repo" ".claude/skills/new-routine/SKILL.md is not in skills-lock.json"

repo="$(new false)"
skill "$repo" new-routine 'metadata:' '  internal: "false"'
assert "metadata.internal \"false\" fails" 1 "$repo" ".claude/skills/new-routine/SKILL.md"

repo="$(new top-level)"
skill "$repo" new-routine 'internal: "true"'
assert "internal outside metadata fails" 1 "$repo" ".claude/skills/new-routine/SKILL.md"

repo="$(new sibling)"
skill "$repo" new-routine 'metadata:' '  short-description: x' 'allowed-tools: Read' '  internal: "true"'
assert "internal under another key fails" 1 "$repo" ".claude/skills/new-routine/SKILL.md"

repo="$(new body)"
skill "$repo" new-routine
printf 'metadata:\n  internal: "true"\n' >>"$repo/.claude/skills/new-routine/SKILL.md"
assert "a marker in the body, past the frontmatter, fails" 1 "$repo" ".claude/skills/new-routine/SKILL.md"

repo="$(new last)"
skill "$repo" new-routine
printf -- '---\nname: new-routine\ndescription: >-\n  Does things.\n\n\nmetadata:\n  internal: "true"\n---\n' >"$repo/.claude/skills/new-routine/SKILL.md"
assert "the marker after a folded description passes" 0 "$repo"

repo="$(new nested)"
skill "$repo" new-routine 'metadata:' '  short-description: x' '  internal: "true"' 'allowed-tools: Read'
assert "the marker among other metadata keys passes" 0 "$repo"

repo="$(new unlocked)"
printf '{"version": 1, "skills": {}}\n' >"$repo/skills-lock.json"
assert "a vendored skill dropped from skills-lock.json fails" 1 "$repo" ".claude/skills/vendored-audit/SKILL.md"

repo="$(new no-lock)"
rm "$repo/skills-lock.json"
assert "no skills-lock.json leaves every skill needing the marker" 1 "$repo" ".claude/skills/vendored-audit/SKILL.md"

repo="$(new bad-lock)"
printf '{"version": 1}\n' >"$repo/skills-lock.json"
assert "a skills-lock.json with no skills object fails" 1 "$repo" "skills-lock.json has no readable"

if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails skill-layout-validate case(s)" >&2
  exit 1
fi
echo "PASS: skill-layout-validate self-test"
