#!/usr/bin/env bash
# rt-pre-pr-check.sh — fast deterministic pre-PR gate for rt-* routine runs.
# Catches low-signal diffs (ledger-only PRs) and formatting/lint issues locally
# before pushing a branch or opening a draft PR, avoiding avoidable CI rounds.
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

# 1. Go formatting & per-package golangci-lint (when pinned binary is present).
go_files=()
while IFS= read -r path; do
  case "$path" in
    *.go) [ -f "$path" ] && go_files+=("$path") ;;
  esac
done <<<"$changed"

if [ "${#go_files[@]}" -gt 0 ]; then
  unformatted="$(gofmt -l "${go_files[@]}")"
  if [ -n "$unformatted" ]; then
    echo "rt-pre-pr-check: FAIL — gofmt needed on:" >&2
    printf '  %s\n' "$unformatted" >&2
    fail=1
  fi

  lint_bin="$root/lego/operator/bin/golangci-lint"
  if [ -x "$lint_bin" ]; then
    for mod in types operator backend cli; do
      pkgs=()
      while IFS= read -r pkg; do
        [ -n "$pkg" ] && pkgs+=("./$pkg")
      done < <(
        for f in "${go_files[@]}"; do
          case "$f" in
            "lego/$mod/"*)
              rel="${f#lego/"$mod"/}"
              dir="$(dirname "$rel")"
              if [ "$dir" = "." ]; then
                printf '.\n'
              else
                printf '%s\n' "$dir"
              fi
              ;;
          esac
        done | LC_ALL=C sort -u
      )
      if [ "${#pkgs[@]}" -gt 0 ]; then
        if ! (cd "$root/lego/$mod" && GOWORK=off "$lint_bin" run "${pkgs[@]}"); then
          echo "rt-pre-pr-check: FAIL — golangci-lint failed in lego/$mod (${pkgs[*]})" >&2
          fail=1
        fi
      fi
    done
  fi
fi

# 2. Dashboard eslint on touched TS/JS files (when node_modules is installed).
dash_dir="dashboard"
dash_files=()
while IFS= read -r path; do
  case "$path" in
    "$dash_dir/"*)
      rel="${path#"$dash_dir/"}"
      case "$rel" in
        *.ts | *.tsx | *.js | *.jsx) [ -f "$path" ] && dash_files+=("$rel") ;;
      esac
      ;;
  esac
done <<<"$changed"

if [ "${#dash_files[@]}" -gt 0 ] && [ -x "$root/$dash_dir/node_modules/.bin/eslint" ]; then
  if ! (cd "$root/$dash_dir" && yarn eslint "${dash_files[@]}"); then
    echo "rt-pre-pr-check: FAIL — dashboard eslint failed" >&2
    fail=1
  fi
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
