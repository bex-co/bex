#!/usr/bin/env bash
# repo-artifact-guard.test.sh — repo-artifact-guard.sh against throwaway repos:
# a big blob, a .pm archive, small evidence dumps, a big blob a later commit
# deletes, markdown under, in and over the soft limit's bands, and a
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

dumps=(.pm/w1/m2/evidence/run/notes.md .pm/w1/done/038-evidence/cases.txt
  .pm/w1/done/m3/receipt.json .pm/w1/m4/build.stdout .pm/w1/m5/Walkthrough-Evidence.txt
  .pm/w1/m5/diagnostic.log.txt mobile/e2e/artifacts/m1/screen.md shot.PNG)
for dump in "${dumps[@]}"; do mkdir -p "$(dirname "$dump")" && echo x >"$dump"; done
git add . && commit dumps
rejects "${dumps[0]} (evidence dump)"
for dump in "${dumps[@]}"; do grep -qF "$dump (evidence dump)" "$tmp/err" || fail "dump not rejected: $dump"; done
git rm -q "${dumps[@]}" && commit drop-dumps
mkdir -p .pm/w1/m6 lego/testdata && echo x >.pm/w1/m6/evidence-locator.md && echo x >lego/testdata/evidence.json
git add . && commit kept
passes

# Markdown soft limit (w10/m607): under 512 KiB is silent, the warning band
# passes with a warning naming the file and its size, 768 KiB or more is
# refused with the split remedy, in both forms. The range form counts only the
# markdown it adds or changes, at its size at HEAD.
mkdir -p docs/ledger
head -c $((512 * 1024 - 1)) /dev/zero >docs/ledger/under.md
git add . && commit md-under
passes
if grep -q warning "$tmp/err"; then fail "a markdown file under 512 KiB drew a warning: $(cat "$tmp/err")"; fi
md_base=$(git rev-parse HEAD)
head -c $((512 * 1024)) /dev/zero >docs/ledger/band.md
git add . && commit md-band
passes
grep -qF "warning: docs/ledger/band.md is 524288 bytes, past the 512 KiB markdown warning band" "$tmp/err" ||
  fail "the warning band does not name the file and its size: $(cat "$tmp/err")"
GITHUB_ACTIONS=true bash "$guard" "$md_base" >"$tmp/out" 2>"$tmp/err" || fail "the warning band was refused: $(cat "$tmp/err")"
grep -qF "::warning file=docs/ledger/band.md," "$tmp/out" || fail "no Actions warning annotation: $(cat "$tmp/out")"
# A workstream README in the band is rolled, not split (w10/058).
head -c $((512 * 1024)) /dev/zero >.pm/w1/README.md
git add . && commit md-board
passes
grep -qF "warning: .pm/w1/README.md is 524288 bytes, past the 512 KiB markdown warning band; roll its closed lines out with python3 scripts/pm-roll-closed.py w1 before" "$tmp/err" ||
  fail "a workstream README in the band is not pointed at the roll: $(cat "$tmp/err")"
grep -qF "warning: docs/ledger/band.md is 524288 bytes, past the 512 KiB markdown warning band; split it before" "$tmp/err" ||
  fail "a document in the band is not told to split: $(cat "$tmp/err")"
git rm -q .pm/w1/README.md && commit drop-md-board
head -c $((768 * 1024)) /dev/zero >docs/ledger/over.md
git add . && commit md-over
rejects "docs/ledger/over.md (786432 bytes >= 768 KiB)"
grep -qF "split each into files of its own" "$tmp/err" || fail "the refusal names no remedy: $(cat "$tmp/err")"
rejects "docs/ledger/over.md (786432 bytes >= 768 KiB)" "$md_base"
over_base=$(git rev-parse HEAD)
echo x >docs/ledger/other.md
git add . && commit md-other
bash "$guard" "$over_base" 2>"$tmp/err" || fail "the range form refused a markdown file it did not change: $(cat "$tmp/err")"
head -c 1000 /dev/zero >docs/ledger/over.md
git add . && commit md-split
bash "$guard" "$md_base" 2>"$tmp/err" || fail "a markdown file split later in the range was refused: $(cat "$tmp/err")"
git rm -q -r docs/ledger && commit drop-md
passes

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
