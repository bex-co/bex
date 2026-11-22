#!/usr/bin/env bash
# rt-pre-pr-check.sh — fast deterministic pre-PR gate for rt-* routine runs.
# Catches low-signal diffs (ledger-only PRs) and formatting/lint issues locally
# before pushing a branch or opening a draft PR, avoiding avoidable CI rounds.
# Modules are discovered dynamically from nearest go.mod / package.json files so
# newly added modules need no edits here.
#
# Usage: bash scripts/rt-pre-pr-check.sh [<base-ref>]   (default: origin/main)
set -euo pipefail

base="${1:-origin/main}"
root="$(git rev-parse --show-toplevel)"
cd "$root"

collect_changed_files() {
  {
    if git rev-parse --verify --quiet "$base" >/dev/null 2>&1; then
      git diff --name-only --diff-filter=ACMRTUXB "$base...HEAD" 2>/dev/null || true
    fi
    git diff --name-only --diff-filter=ACMRTUXB --cached 2>/dev/null || true
    git diff --name-only --diff-filter=ACMRTUXB 2>/dev/null || true
    git ls-files --others --exclude-standard 2>/dev/null || true
  } | awk 'NF' | LC_ALL=C sort -u
}

# Walk up from a repo-relative file path to find the nearest ancestor directory
# (within $root) containing the marker file (e.g. go.mod or package.json).
find_nearest_root() {
  local path="$1" marker="$2"
  local dir
  dir="$(dirname "$path")"
  while true; do
    if [ -f "$root/$dir/$marker" ]; then
      printf '%s\n' "$dir"
      return 0
    fi
    if [ "$dir" = "." ] || [ "$dir" = "/" ]; then
      return 1
    fi
    dir="$(dirname "$dir")"
  done
}

# Skip non-production example/fixture trees when dispatching per-module linters.
is_non_prod_fixture() {
  local p="$1"
  local ex_prefix="examples"
  case "$p" in
    "$ex_prefix/"* | */testdata/* | testdata/* | */fixtures/* | fixtures/*) return 0 ;;
    *) return 1 ;;
  esac
}

changed="$(collect_changed_files)"
if [ -z "$changed" ]; then
  echo "rt-pre-pr-check: no changed files relative to $base — nothing to ship (valid empty run)." >&2
  exit 1
fi

non_ledger=""
while IFS= read -r path; do
  case "$path" in
    .claude/skills/*/ledger.md | .agents/skills/*/ledger.md | */rt-shared/ledger.md) ;;
    *) non_ledger="${non_ledger}${path}"$'\n' ;;
  esac
done <<<"$changed"

if [ -z "$non_ledger" ]; then
  echo "rt-pre-pr-check: FAIL — diff modifies only routine ledger.md files. Never commit or open a PR for ledger-only updates (even Deferred entries)." >&2
  exit 1
fi

fail=0

# 1. Go formatting & per-package golangci-lint (modules discovered via nearest go.mod).
go_files=()
while IFS= read -r path; do
  case "$path" in
    *.go)
      if [ -f "$path" ] && ! is_non_prod_fixture "$path"; then
        go_files+=("$path")
      fi
      ;;
  esac
done <<<"$changed"

if [ "${#go_files[@]}" -gt 0 ]; then
  unformatted="$(gofmt -l "${go_files[@]}")"
  if [ -n "$unformatted" ]; then
    echo "rt-pre-pr-check: FAIL — gofmt needed on:" >&2
    printf '  %s\n' "$unformatted" >&2
    fail=1
  fi

  lint_bin="${BEX_GOLANGCI_LINT:-$root/lego/operator/bin/golangci-lint}"
  if [ -x "$lint_bin" ]; then
    go_mods=()
    while IFS= read -r mod_dir; do
      [ -n "$mod_dir" ] && go_mods+=("$mod_dir")
    done < <(
      for f in "${go_files[@]}"; do
        find_nearest_root "$f" "go.mod" || true
      done | LC_ALL=C sort -u
    )

    for mod_dir in "${go_mods[@]}"; do
      pkgs=()
      while IFS= read -r pkg; do
        [ -n "$pkg" ] && pkgs+=("./$pkg")
      done < <(
        for f in "${go_files[@]}"; do
          f_mod="$(find_nearest_root "$f" "go.mod" || true)"
          [ "$f_mod" = "$mod_dir" ] || continue
          if [ "$mod_dir" = "." ]; then
            rel="$f"
          else
            rel="${f#"$mod_dir/"}"
          fi
          dir="$(dirname "$rel")"
          if [ "$dir" = "." ]; then
            printf '.\n'
          else
            printf '%s\n' "$dir"
          fi
        done | LC_ALL=C sort -u
      )
      if [ "${#pkgs[@]}" -gt 0 ]; then
        if ! (cd "$root/$mod_dir" && GOWORK=off "$lint_bin" run "${pkgs[@]}"); then
          echo "rt-pre-pr-check: FAIL — golangci-lint failed in $mod_dir (${pkgs[*]})" >&2
          fail=1
        fi
      fi
    done
  fi
fi

# 2. JS/TS eslint on touched files (packages discovered via nearest package.json).
js_files=()
while IFS= read -r path; do
  case "$path" in
    *.ts | *.tsx | *.js | *.jsx)
      if [ -f "$path" ] && ! is_non_prod_fixture "$path"; then
        js_files+=("$path")
      fi
      ;;
  esac
done <<<"$changed"

if [ "${#js_files[@]}" -gt 0 ]; then
  js_pkgs=()
  while IFS= read -r pkg_dir; do
    [ -n "$pkg_dir" ] && js_pkgs+=("$pkg_dir")
  done < <(
    for f in "${js_files[@]}"; do
      find_nearest_root "$f" "package.json" || true
    done | LC_ALL=C sort -u
  )

  for pkg_dir in "${js_pkgs[@]}"; do
    [ -x "$root/$pkg_dir/node_modules/.bin/eslint" ] || continue
    rel_files=()
    for f in "${js_files[@]}"; do
      f_pkg="$(find_nearest_root "$f" "package.json" || true)"
      [ "$f_pkg" = "$pkg_dir" ] || continue
      if [ "$pkg_dir" = "." ]; then
        rel_files+=("$f")
      else
        rel_files+=("${f#"$pkg_dir/"}")
      fi
    done
    if [ "${#rel_files[@]}" -gt 0 ]; then
      if ! (cd "$root/$pkg_dir" && "$root/$pkg_dir/node_modules/.bin/eslint" "${rel_files[@]}"); then
        echo "rt-pre-pr-check: FAIL — eslint failed in $pkg_dir" >&2
        fail=1
      fi
    fi
  done
fi

# 3. Shell scripts (when shellcheck is installed).
sh_files=()
while IFS= read -r path; do
  case "$path" in
    *.sh) [ -f "$path" ] && sh_files+=("$path") ;;
  esac
done <<<"$changed"

if [ "${#sh_files[@]}" -gt 0 ] && command -v shellcheck >/dev/null 2>&1; then
  if ! shellcheck "${sh_files[@]}"; then
    echo "rt-pre-pr-check: FAIL — shellcheck failed" >&2
    fail=1
  fi
fi

# 4. Skill layout validation when skills changed.
skills_touched=0
while IFS= read -r path; do
  case "$path" in
    .claude/skills/* | .agents/skills/* | .claude/commands/*) skills_touched=1 ;;
  esac
done <<<"$changed"

if [ "$skills_touched" = 1 ] && [ -f "$root/scripts/skill-layout-validate.sh" ]; then
  if ! bash "$root/scripts/skill-layout-validate.sh"; then
    echo "rt-pre-pr-check: FAIL — skill-layout-validate.sh failed" >&2
    fail=1
  fi
fi

# 5. Secret scan on committed range when gitleaks is available.
if command -v gitleaks >/dev/null 2>&1 && [ -f "$root/scripts/gitleaks.sh" ] && git rev-parse --verify --quiet "$base" >/dev/null 2>&1; then
  if [ -n "$(git rev-list "$base..HEAD" 2>/dev/null || true)" ]; then
    if ! bash "$root/scripts/gitleaks.sh" "$base..HEAD"; then
      echo "rt-pre-pr-check: FAIL — gitleaks detected potential secrets in $base..HEAD" >&2
      fail=1
    fi
  fi
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "rt-pre-pr-check: OK"
