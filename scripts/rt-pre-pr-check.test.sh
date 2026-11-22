#!/usr/bin/env bash
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/rt-pre-pr-check.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

init_repo() {
  local dir="$1"
  mkdir -p "$dir/scripts" "$dir/.claude/skills/rt-fix-logic-bugs" "$dir/lego/types"
  cp "$script" "$dir/scripts/rt-pre-pr-check.sh"
  git -C "$dir" init -q -b main
  git -C "$dir" config user.email test@example.com
  git -C "$dir" config user.name test
  printf '# ledger\n' >"$dir/.claude/skills/rt-fix-logic-bugs/ledger.md"
  printf 'module example.com/types\n\ngo 1.24\n' >"$dir/lego/types/go.mod"
  printf 'package types\n\nconst X = 1\n' >"$dir/lego/types/x.go"
  git -C "$dir" add .
  git -C "$dir" commit -q -m "base"
  git -C "$dir" checkout -q -b feature
}

assert_rc() {
  local name="$1" want="$2" dir="$3" needle="${4:-}"
  local out rc=0
  out="$(cd "$dir" && bash scripts/rt-pre-pr-check.sh main 2>&1)" || rc=$?
  if [ "$rc" -ne "$want" ]; then
    echo "FAIL [$name]: got rc=$rc want rc=$want; output: $out" >&2
    exit 1
  fi
  if [ -n "$needle" ] && [[ "$out" != *"$needle"* ]]; then
    echo "FAIL [$name]: output missing '$needle': $out" >&2
    exit 1
  fi
  echo "PASS [$name]"
}

# 1. Empty diff is rejected (nothing to ship).
d1="$tmp/empty"
init_repo "$d1"
assert_rc "empty diff rejected" 1 "$d1" "no changed files"

# 2. Ledger-only diff is rejected (even when committed).
d2="$tmp/ledger-only"
init_repo "$d2"
printf -- '- 2026-11-22 foo — deferred\n' >>"$d2/.claude/skills/rt-fix-logic-bugs/ledger.md"
git -C "$d2" commit -q -am "chore(ledger): deferred only"
assert_rc "ledger-only rejected" 1 "$d2" "ledger.md"

# 3. Unformatted Go file is rejected.
d3="$tmp/bad-gofmt"
init_repo "$d3"
printf 'package types\n\nconst   X =   2\n' >"$d3/lego/types/x.go"
assert_rc "unformatted go rejected" 1 "$d3" "gofmt needed"

# 4. Formatted Go file + ledger update passes.
d4="$tmp/good-go"
init_repo "$d4"
printf 'package types\n\nconst X = 2\n' >"$d4/lego/types/x.go"
printf -- '- 2026-11-22 foo — skipped\n' >>"$d4/.claude/skills/rt-fix-logic-bugs/ledger.md"
git -C "$d4" commit -q -am "fix(types): update X"
assert_rc "formatted go + ledger passes" 0 "$d4" "rt-pre-pr-check: OK"

# 5. Any newly added Go module is discovered automatically via its go.mod.
d5="$tmp/new-module"
init_repo "$d5"
mkdir -p "$d5/lego/newmod/sub" "$d5/bin"
printf 'module example.com/newmod\n\ngo 1.24\n' >"$d5/lego/newmod/go.mod"
printf 'package sub\n\nconst Y = 1\n' >"$d5/lego/newmod/sub/y.go"
cat >"$d5/bin/fake-lint" <<'LINT'
#!/usr/bin/env bash
printf '%s %s\n' "$PWD" "$*" >"$FAKE_LINT_LOG"
LINT
chmod +x "$d5/bin/fake-lint"
lint_log="$d5/lint.log"
(cd "$d5" && FAKE_LINT_LOG="$lint_log" BEX_GOLANGCI_LINT="$d5/bin/fake-lint" bash scripts/rt-pre-pr-check.sh main >/dev/null)
if ! grep -q "lego/newmod run ./sub" "$lint_log"; then
  echo "FAIL [dynamic Go module discovery]: unexpected lint invocation: $(cat "$lint_log")" >&2
  exit 1
fi
echo "PASS [dynamic Go module discovery]"
