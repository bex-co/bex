#!/usr/bin/env bash
# Self-test for scripts/docs-links-validate.sh. A link gate that
# was never seen to fail proves nothing, and one that flags code samples gets
# disabled: so each rule has its own red or ignored case, the failure message
# must name file:line and the target, and the real tree must pass end to end.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/docs-links-validate.sh"
[ -x "$SCRIPT" ] || {
  echo "FAIL: $SCRIPT not executable" >&2
  exit 1
}

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# fixture <dir> — a tree whose every link resolves.
fixture() {
  local dir="$1"
  mkdir -p "$dir/docs/sub" "$dir/.pm/w9" "$dir/lego/backend"
  printf '# a\n\nSee [b](sub/b.md), the [folder](sub/), [an anchor](sub/b.md#heading), and [a query](sub/b.md?plain=1).\n' >"$dir/docs/a.md"
  printf '# b\n\nBack to [a](../a.md).\n' >"$dir/docs/sub/b.md"
  printf '# agents\n\n[docs](docs/a.md)\n' >"$dir/AGENTS.md"
  printf '# board\n\n[a](../docs/a.md)\n' >"$dir/.pm/AGENTS.md"
  printf '# backend\n\n[root](../../AGENTS.md)\n' >"$dir/lego/backend/README.md"
  # The vendored and scratch trees the gate skips are the ones .prettierignore names.
  printf '# comment\n\nexternal/\nnode_modules/\n.pm/\nvendor/pinned/\n' >"$dir/.prettierignore"
}

# run <dir> — the gate scoped to one fixture tree, stderr only.
run() { { DOCS_LINKS_ROOT="$1" "$SCRIPT" >/dev/null; } 2>&1; }

# assert <label> <want-rc> <dir> [expected-stderr-substring]
assert() {
  local label="$1" want="$2" dir="$3" needle="${4:-}"
  set +e
  local err
  err="$(run "$dir")"
  local got=$?
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

# refuse <label> <dir> <needle> — the gate must NOT name needle in its stderr.
refuse() {
  local label="$1" dir="$2" needle="$3" err
  set +e
  err="$(run "$dir")"
  set -e
  if printf '%s' "$err" | grep -qF -- "$needle"; then
    echo "FAIL: $label — stderr named '$needle'" >&2
    printf '%s\n' "$err" | sed 's/^/        /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label"
}

# --- green baseline --------------------------------------------------------
green="$tmp/green"
fixture "$green"
assert "a tree whose links resolve passes (file, folder, anchor, query)" 0 "$green"

# --- red: a missing file, named by file:line and target ---------------------
missing="$tmp/missing"
fixture "$missing"
printf '# a\n\nline three\n[gone](sub/gone.md)\n' >"$missing/docs/a.md"
assert "a missing target is red" 1 "$missing" "docs/a.md:4 -> sub/gone.md"

# --- red: image, reference definition, angle-bracket and encoded targets ----
kinds="$tmp/kinds"
fixture "$kinds"
printf '# a\n\n![shot](img/shot.png)\n\n[ref]: refs/missing.md\n\n[angle](<has space.md>)\n[enc](my%%20file.md)\n' >"$kinds/docs/a.md"
assert "a missing image is red" 1 "$kinds" "docs/a.md:3 -> img/shot.png"
assert "a missing reference definition is red" 1 "$kinds" "docs/a.md:5 -> refs/missing.md"
assert "a missing angle-bracket target is red" 1 "$kinds" "docs/a.md:7 -> has space.md"
assert "a missing percent-encoded target is red" 1 "$kinds" "docs/a.md:8 -> my%20file.md"

# --- red: the other scoped file kinds ----------------------------------------
agents="$tmp/agents"
fixture "$agents"
printf '# agents\n\n[gone](docs/gone.md)\n' >"$agents/AGENTS.md"
assert "an AGENTS.md link is checked" 1 "$agents" "AGENTS.md:3 -> docs/gone.md"
board="$tmp/board"
fixture "$board"
printf '# board\n\n[gone](../docs/gone.md)\n' >"$board/.pm/AGENTS.md"
assert "a top-level .pm file is checked" 1 "$board" ".pm/AGENTS.md:3 -> ../docs/gone.md"
readme="$tmp/readme"
fixture "$readme"
printf '# backend\n\n[gone](../../nope/README.md)\n' >"$readme/lego/backend/README.md"
assert "a nested README is checked" 1 "$readme" "lego/backend/README.md:3 -> ../../nope/README.md"

# --- ignored: fenced code, inline code, external, absolute, anchor ----------
ignored="$tmp/ignored"
fixture "$ignored"
cat >"$ignored/docs/a.md" <<'EOF'
# a

```md
[in a fence](nowhere/fenced.md)
```

~~~
[in a tilde fence](nowhere/tilde.md)
~~~

Inline `[in a span](nowhere/span.md)` and ``[double](nowhere/double.md)``.

[web](https://example.com/x) [mail](mailto:a@b.c) [ftp](ftp://host/p) [abs](/etc/passwd) [self](#top)

[after the fences](sub/b.md)
EOF
assert "code, external, absolute and anchor links are not checked" 0 "$ignored"

# A longer fence is not closed by a shorter one, so what follows stays code.
nested="$tmp/nested"
fixture "$nested"
printf '# a\n\n````md\n```\n[still code](nowhere/nested.md)\n```\n````\n\n[real](nowhere/real.md)\n' >"$nested/docs/a.md"
assert "a fence closes only on a fence at least as long" 1 "$nested" "docs/a.md:9 -> nowhere/real.md"
refuse "a link inside a longer fence is not reported" "$nested" "nowhere/nested.md"

# --- ignored: the workstream history and vendored trees ----------------------
history="$tmp/history"
fixture "$history"
printf '# w9\n\n[evidence](evidence/gone.log)\n' >"$history/.pm/w9/README.md"
mkdir -p "$history/external/dep" "$history/node_modules/pkg"
printf '[x](nowhere.md)\n' >"$history/external/dep/README.md"
printf '[x](nowhere.md)\n' >"$history/node_modules/pkg/README.md"
mkdir -p "$history/vendor/pinned" "$history/deep/external/dep"
printf '[x](nowhere.md)\n' >"$history/vendor/pinned/README.md"
printf '[x](nowhere.md)\n' >"$history/deep/external/dep/README.md"
assert ".pm/wN history and the .prettierignore trees are not checked" 0 "$history"

# A tree .prettierignore does not list is checked, so the list is the only source.
unlisted="$tmp/unlisted"
fixture "$unlisted"
mkdir -p "$unlisted/vendor/other"
printf '[x](nowhere.md)\n' >"$unlisted/vendor/other/README.md"
assert "a tree .prettierignore does not name is checked" 1 "$unlisted" "vendor/other/README.md:1 -> nowhere.md"

# --- parsing: what is a fence, a span, a target -------------------------------
# A backtick fence's info string holds no backtick, so this line is inline code
# and not the start of a fence that would hide the rest of the file.
spanline="$tmp/spanline"
fixture "$spanline"
printf '# a\n\n```code``` and [one](nowhere/one.md)\n\n[two](nowhere/two.md)\n' >"$spanline/docs/a.md"
assert "a line of inline code does not open a fence" 1 "$spanline" "docs/a.md:5 -> nowhere/two.md"

# A closing fence is bare: a fence with an info string inside a fence is content.
infoclose="$tmp/infoclose"
fixture "$infoclose"
printf '# a\n\n```md\n```js\n[still code](nowhere/code.md)\n```\n\n[real](nowhere/real.md)\n' >"$infoclose/docs/a.md"
assert "a fence with an info string does not close a fence" 1 "$infoclose" "docs/a.md:8 -> nowhere/real.md"
refuse "so what follows it stays code" "$infoclose" "nowhere/code.md"

parens="$tmp/parens"
fixture "$parens"
printf '# a\n\n[ok](Foo_(bar).md) [gone](Gone_(baz).md)\n' >"$parens/docs/a.md"
printf '# foo\n' >"$parens/docs/Foo_(bar).md"
assert "a target with parentheses is read whole" 1 "$parens" "docs/a.md:3 -> Gone_(baz).md"
refuse "and an existing one is not reported" "$parens" "Foo_(bar).md"

badge="$tmp/badge"
fixture "$badge"
mkdir -p "$badge/docs/img"
printf 'png' >"$badge/docs/img/ok.png"
printf '# a\n\n[![badge](img/ok.png)](sub/gone.md)\n' >"$badge/docs/a.md"
assert "a link around an image is checked as well as the image" 1 "$badge" "docs/a.md:3 -> sub/gone.md"
refuse "and an existing image is not reported" "$badge" "img/ok.png"

footnote="$tmp/footnote"
fixture "$footnote"
printf '# a\n\nText[^1].\n\n[^1]: See the docs, not a path.\n' >"$footnote/docs/a.md"
assert "a footnote definition is not a link" 0 "$footnote"

crlf="$tmp/crlf"
fixture "$crlf"
printf '# a\r\n\r\n```\r\n[code](nowhere/code.md)\r\n```\r\n\r\n[real](nowhere/real.md)\r\n' >"$crlf/docs/a.md"
assert "CRLF files are read like LF ones" 1 "$crlf" "docs/a.md:7 -> nowhere/real.md"

# A target must exist with the case written (CI is case-sensitive) ...
casing="$tmp/casing"
fixture "$casing"
printf '# a\n\n[wrong case](sub/B.md)\n' >"$casing/docs/a.md"
assert "a target of the wrong case is red, on any disk" 1 "$casing" "docs/a.md:3 -> sub/B.md"

# ... and inside the repository, even when the file it escapes to exists.
escape="$tmp/escape"
fixture "$escape"
printf '# outside\n' >"$tmp/outside.md"
printf '# a\n\n[out](../../outside.md)\n' >"$escape/docs/a.md"
assert "a target outside the repository is red" 1 "$escape" "docs/a.md:3 -> ../../outside.md"

# Pairing backtick runs and scanning brackets are linear: these lines took a minute
# or more under a regex that backtracks.
dense="$tmp/dense"
fixture "$dense"
python3 - >"$dense/docs/a.md" <<'PY'
tick = chr(96)
print("# a")
print()
# Runs of varied lengths that mostly never close, then a flood of brackets.
print("x " + " ".join(tick * (i % 50 + 1) for i in range(8000)))
print("y " + tick * 3000 + "a")
print("[" * 20000)
print("[x](nowhere/dense.md)")
PY
assert "lines of backtick runs, one 3000 long and unclosed, and of 20000 brackets are read in linear time" 1 "$dense" "docs/a.md:6 -> nowhere/dense.md"

# --- the failure names how many, and exits non-zero with every miss listed ---
many="$tmp/many"
fixture "$many"
printf '# a\n\n[x](x.md)\n[y](y.md)\n' >"$many/docs/a.md"
assert "every miss is reported" 1 "$many" "2 broken relative link(s)"

# --- the real tree must pass end to end -------------------------------------
set +e
real="$({ "$SCRIPT" >/dev/null; } 2>&1)"
rc=$?
set -e
if [ "$rc" -ne 0 ]; then
  echo "FAIL: the repository's own links are broken" >&2
  printf '%s\n' "$real" | sed 's/^/        /' >&2
  fails=$((fails + 1))
else
  echo "ok: the repository's own links resolve"
fi

if [ "$fails" -ne 0 ]; then
  echo "docs-links-validate.test: $fails failure(s)" >&2
  exit 1
fi
echo "docs-links-validate.test: all cases passed"
