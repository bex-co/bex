#!/usr/bin/env bash
# repo-artifact-guard.test.sh — repo-artifact-guard.sh against throwaway repos:
# a big blob, a .pm archive, a big blob a later commit deletes, and a
# size-filtered partial clone whose big blob was never fetched.
set -euo pipefail

guard=$(cd "$(dirname "$0")" && pwd)/repo-artifact-guard.sh
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
commit() { git -c user.name=t -c user.email=t@t commit -qm "$1"; }
passes() { bash "$guard" 2>"$tmp/err" || fail "expected pass: $(cat "$tmp/err")"; }
rejects() { # <needle> [base]
  local needle=$1
  shift
  if bash "$guard" "$@" 2>"$tmp/err"; then fail "expected rejection of $needle"; fi
  grep -qF "$needle" "$tmp/err" || fail "rejection does not name $needle: $(cat "$tmp/err")"
}

git init -q "$tmp/r"
cd "$tmp/r"
git config uploadpack.allowFilter true
mkdir -p .pm/w1 docs
echo '# note' >.pm/w1/001.md
head -c 1048575 /dev/zero >docs/just-under-1mib.txt
git add . && commit base
passes
base=$(git rev-parse HEAD)

echo x >.pm/w1/evidence.whl
git add . && commit wheel
rejects ".pm/w1/evidence.whl (binary/archive under .pm)"
git rm -q .pm/w1/evidence.whl && echo x >'.pm/w1/Report Copy.ZIP'
git add . && commit zip
rejects ".pm/w1/Report Copy.ZIP (binary/archive under .pm)"
git rm -q '.pm/w1/Report Copy.ZIP' && commit drop-zip
passes
rejects ".pm/w1/evidence.whl" "$base"

head -c 1048576 /dev/zero >lego-binary
git add . && commit big
rejects "lego-binary (1048576 bytes >= 1 MiB)"
big=$(git rev-parse HEAD:lego-binary)

git clone -q --no-checkout --no-local --filter=blob:limit=1m "file://$tmp/r" "$tmp/partial"
cd "$tmp/partial"
GIT_NO_LAZY_FETCH=1 git cat-file -e "$big" 2>/dev/null && fail "partial clone fetched the big blob"
rejects "$big (not fetched: 1 MiB or more in a size-filtered clone"
[ "$(grep -c "not fetched" "$tmp/err")" = 1 ] || fail "a blob under 1 MiB was reported missing: $(cat "$tmp/err")"
GIT_NO_LAZY_FETCH=1 git cat-file -e "$big" 2>/dev/null && fail "the guard lazily fetched the big blob"
echo "PASS: repo-artifact-guard"
