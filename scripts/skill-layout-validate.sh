#!/usr/bin/env bash
# Keep Claude as the single source of truth for repository skills while exposing
# the same skill set to Codex through relative symlinks.
set -euo pipefail
cd "$(dirname "$0")/.."

canonical_root=".claude/skills"
bridge_root=".agents/skills"
retired_root=".claude/commands"
fail=0

if [ -e "$retired_root" ] || [ -L "$retired_root" ]; then
  echo "FAIL: retired command directory exists: $retired_root" >&2
  fail=1
fi

for root in "$canonical_root" "$bridge_root"; do
  if [ ! -d "$root" ] || [ -L "$root" ]; then
    echo "FAIL: expected a real directory: $root" >&2
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  exit 1
fi

list_entries() {
  find "$1" -mindepth 1 -maxdepth 1 -exec basename {} \; | LC_ALL=C sort
}

canonical_names="$(list_entries "$canonical_root")"
bridge_names="$(list_entries "$bridge_root")"

if ! diff -u \
  <(printf '%s\n' "$canonical_names") \
  <(printf '%s\n' "$bridge_names"); then
  echo "FAIL: $canonical_root and $bridge_root expose different skill names" >&2
  fail=1
fi

while IFS= read -r name; do
  [ -n "$name" ] || continue

  canonical_path="$canonical_root/$name"
  skill_file="$canonical_path/SKILL.md"
  bridge_path="$bridge_root/$name"
  expected_target="../../.claude/skills/$name"

  if [ ! -d "$canonical_path" ] || [ -L "$canonical_path" ]; then
    echo "FAIL: canonical skill must be a real directory: $canonical_path" >&2
    fail=1
  fi

  if [ ! -f "$skill_file" ] || [ -L "$skill_file" ]; then
    echo "FAIL: canonical SKILL.md must be a real file: $skill_file" >&2
    fail=1
  else
    declared_name="$(
      awk '
        NR == 1 {
          if ($0 != "---") {
            exit
          }
          next
        }
        $0 == "---" {
          exit
        }
        /^name:[[:space:]]*/ {
          sub(/^name:[[:space:]]*/, "")
          print
          exit
        }
      ' "$skill_file"
    )"
    if [ "$declared_name" != "$name" ]; then
      echo "FAIL: $skill_file declares name '$declared_name', expected '$name'" >&2
      fail=1
    fi

    if ! awk '
      NR == 1 {
        if ($0 != "---") {
          exit 1
        }
        next
      }
      $0 == "---" {
        exit found ? 0 : 1
      }
      /^description:/ {
        found = 1
      }
      END {
        if (NR > 0 && $0 != "---") {
          exit 1
        }
      }
    ' "$skill_file"; then
      echo "FAIL: $skill_file has no frontmatter description" >&2
      fail=1
    fi
  fi

  if [ ! -L "$bridge_path" ]; then
    echo "FAIL: Codex bridge must be a symlink: $bridge_path" >&2
    fail=1
  elif [ "$(readlink "$bridge_path")" != "$expected_target" ]; then
    echo "FAIL: $bridge_path must target $expected_target" >&2
    fail=1
  elif [ ! -f "$bridge_path/SKILL.md" ]; then
    echo "FAIL: broken Codex skill bridge: $bridge_path" >&2
    fail=1
  fi
done <<< "$canonical_names"

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "PASS: Claude skills are canonical and Codex bridges match one-to-one"
