#!/usr/bin/env bash
# Unit tests for render-docs-drift.sh (w4/m688). Network-free: the docs base
# URL is a file:// URL to a synthetic llms.txt and three synthetic pages written
# below, shaped like Render's (no Render page text is committed). Each kind of
# change Render makes must fail naming the page and its row's pointer: a new
# page, a gone page, a rewritten page and a page it cannot fetch; a re-render
# that only moves the index banner, images or whitespace must pass; and the
# committed ledger must follow the row rules on every push.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-docs-drift.sh"
committed_ledger="$repo_root/docs/render-artifacts/docs-pages.md"

failures=0
check() { # check <name> <expected exit> <expected text> <command...>
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF -- "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

tmp="${TMPDIR:-/tmp}"
sandbox="$(mktemp -d "${tmp%/}/bex-render-docs-drift-test.XXXXXX")"
trap 'rm -rf "$sandbox"' EXIT

# The docs: an index with three pages (one under a nested slug), a topic bundle
# and a bare preamble URL, neither of which is a page.
docs="$sandbox/docs"
mkdir -p "$docs/nested"
cat >"$docs/llms.txt" <<'EOF'
# Render

- Public documentation retrieval as Markdown: https://render.com/docs/api.md#setup

## Documentation

- [Render Sandboxes — all articles](https://render.com/docs/sandboxes/all)

#### Start

- [Alpha Page](https://render.com/docs/alpha.md): The first page.
- [Beta Page](https://render.com/docs/beta.md)
- [Alpha Page again](https://render.com/docs/alpha.md)

#### SDK

- [Nested v0](https://render.com/docs/nested/v0.md): A versioned page.
EOF
cat >"$docs/alpha.md" <<'EOF'
# Alpha Page — The first page.

> For the complete documentation index, see [llms.txt](https://render.com/llms.txt).

Alpha services spin down after 15 minutes.

   [image: The alpha settings page]

## Limits

Each alpha has 512 MB.
EOF
printf '# Beta Page\n\nBeta retries a deploy twice.\n\nBeta keeps logs for 7 days.\n' >"$docs/beta.md"
printf '# Nested v0\n\nThe v0 SDK is deprecated.\n' >"$docs/nested/v0.md"

# alpha.md's normalized text, written out independently of the script: the
# banner and image lines dropped, blank-line runs collapsed, one final newline.
alpha_hash="$(printf '# Alpha Page — The first page.\n\nAlpha services spin down after 15 minutes.\n\n## Limits\n\nEach alpha has 512 MB.\n' |
  shasum -a 256 | cut -c1-12)"

run_with() { # run_with <docs dir> <ledger> [mode…]
  local dir="$1" ledger="$2"
  shift 2
  RENDER_DOCS_URL="file://$dir" RENDER_DOCS_LEDGER="$ledger" "$script" "$@"
}

check "hashes prints alpha's normalized hash" 0 "$(printf 'alpha\t%s' "$alpha_hash")" run_with "$docs" /dev/null hashes
hashes="$(run_with "$docs" /dev/null hashes)"
hash_of() { awk -F'\t' -v page="$1" '$1 == page { print $2 }' <<<"$hashes"; }
check "hashes lists each page once, bundles and preamble URLs excluded" 0 "" \
  test "$(cut -f1 <<<"$hashes" | tr '\n' ' ')" = "alpha beta nested/v0 "

ledger="$sandbox/ledger.md"
cat >"$ledger" <<EOF
# Fixture ledger

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [\`alpha\`](https://render.com/docs/alpha) | matched | ADR018 § Services & lifecycle: Suspend / Resume | \`$(hash_of alpha)\` |
| [\`beta\`](https://render.com/docs/beta) | gap | log retention is \`w9/m123\` | \`$(hash_of beta)\` |
| [\`nested/v0\`](https://render.com/docs/nested/v0) | n/a | a deprecated SDK version | \`$(hash_of nested/v0)\` |
EOF

check "every page matching its row passes" 0 "all 3 pages in llms.txt match their ledger rows" run_with "$docs" "$ledger"
check "the committed ledger's shape is accepted offline" 0 "3 rows well formed" run_with "$docs" "$ledger" validate

# docs_with <name> <file> <sed expression>: a copy of the docs with one file
# edited; prints the copy's directory.
docs_with() {
  local dir="$sandbox/$1"
  cp -R "$docs" "$dir"
  sed -e "$3" "$docs/$2" >"$dir/$2"
  if cmp -s "$docs/$2" "$dir/$2"; then
    echo "docs_with $1: the edit changed nothing in $2" >&2
    exit 1
  fi
  echo "$dir"
}

# A re-render: a new banner link, another image, trailing spaces, a blank-line
# run and CRLF line ends.
rerendered="$(docs_with rerender alpha.md 's#https://render.com/llms.txt#https://render.com/docs/llms.txt#; s#^Each alpha#![diagram](https://render.com/x.png)\
\
\
Each alpha#; s#minutes\.$#minutes.   #')"
sed -i.bak 's/$/\r/' "$rerendered/alpha.md" && rm "$rerendered/alpha.md.bak"
check "a banner, image and whitespace re-render passes" 0 "all 3 pages in llms.txt match" run_with "$rerendered" "$ledger"

new_docs="$(docs_with new-page llms.txt 's#^- \[Beta Page\].*#&\
- [Gamma Page](https://render.com/docs/gamma.md)#')"
printf '# Gamma\n' >"$new_docs/gamma.md"
check "a new page fails naming it and its hash" 1 \
  "new page: Gamma Page <https://render.com/docs/gamma> (hash $(printf '# Gamma\n' | shasum -a 256 | cut -c1-12)) has no row" \
  run_with "$new_docs" "$ledger"

check "a gone page fails naming its row and pointer" 1 \
  "gone: $ledger:6: beta is no longer in llms.txt (was gap: log retention is \`w9/m123\`)" \
  run_with "$(docs_with gone-page llms.txt '/beta\.md/d')" "$ledger"

changed="$(docs_with changed-page beta.md 's/7 days/30 days/')"
new_beta="$(printf '# Beta Page\n\nBeta retries a deploy twice.\n\nBeta keeps logs for 30 days.\n' | shasum -a 256 | cut -c1-12)"
check "a changed page fails naming its row, pointer and both hashes" 1 \
  "changed: $ledger:6: https://render.com/docs/beta hash $(hash_of beta) -> $new_beta (re-check gap: log retention is \`w9/m123\`)" \
  run_with "$changed" "$ledger"
check "without a cache the changed paragraphs are not shown" 1 "no cache directory was given for 1 changed page(s)" \
  run_with "$changed" "$ledger"

# The cache: a green run keeps each page under its hash; a later run that finds
# a page changed quotes the paragraphs that changed since the recorded hash.
cache="$sandbox/cache"
mkdir "$cache"
echo stale >"$cache/000000000000.md"
check "a green run with a cache passes" 0 "match their ledger rows" env RENDER_DOCS_CACHE="$cache" \
  RENDER_DOCS_URL="file://$docs" RENDER_DOCS_LEDGER="$ledger" "$script"
check "the cache keeps each page under its hash" 0 "" test -f "$cache/$(hash_of beta).md"
check "the cache drops what neither the ledger nor the fetch names" 1 "" test -e "$cache/000000000000.md"
check "a changed page quotes the old paragraph" 1 "    - Beta keeps logs for 7 days." env RENDER_DOCS_CACHE="$cache" \
  RENDER_DOCS_URL="file://$changed" RENDER_DOCS_LEDGER="$ledger" "$script"
check "a changed page quotes the new paragraph" 1 "    + Beta keeps logs for 30 days." env RENDER_DOCS_CACHE="$cache" \
  RENDER_DOCS_URL="file://$changed" RENDER_DOCS_LEDGER="$ledger" "$script"
check "the recorded fetch survives a failing run" 0 "" test -f "$cache/$(hash_of beta).md"

unfetchable="$sandbox/unfetchable"
cp -R "$docs" "$unfetchable" && rm "$unfetchable/nested/v0.md"
check "a listed page that cannot be fetched fails naming it" 1 \
  "unreadable: nested/v0 is in llms.txt but could not fetch file://$unfetchable/nested/v0.md" \
  run_with "$unfetchable" "$ledger"

check "an llms.txt with no page link fails as changed markup" 1 "lists no [title](https://render.com/docs/<page>.md) link; its markup changed" \
  run_with "$(docs_with no-links llms.txt 's#\.md)#.html)#')" "$ledger"
check "an unreachable llms.txt fails" 1 "could not fetch file://$sandbox/absent/llms.txt" run_with "$sandbox/absent" "$ledger"
check "a missing ledger fails" 1 "Render docs ledger missing" run_with "$docs" "$sandbox/absent.md"

# Row rules, as in the changelog and dashboard-operations ledgers.
ledger_with() { # ledger_with <name> <sed expression>
  sed -e "$2" "$ledger" >"$sandbox/$1.md"
  if cmp -s "$ledger" "$sandbox/$1.md"; then
    echo "ledger_with $1: the edit changed nothing" >&2
    exit 1
  fi
  echo "$sandbox/$1.md"
}
check "a gap naming no board item fails" 1 "gap row names no board item" \
  run_with "$docs" "$(ledger_with gap-unowned 's#log retention is `w9/m123`#log retention is unowned#')" validate
check "a non-goal citing neither DO_NOT_DO nor ADR018 fails" 1 "non-goal row cites neither DO_NOT_DO.md nor an ADR018 row" \
  run_with "$docs" "$(ledger_with non-goal 's#| n/a | a deprecated SDK version |#| non-goal | we decided |#')" validate
check "a partial naming neither an item nor ADR018 fails" 1 "partial row names neither" \
  run_with "$docs" "$(ledger_with partial 's#| n/a | a deprecated SDK version |#| partial | half of it |#')" validate
check "a placeholder pointer fails" 1 "n/a row has no pointer" \
  run_with "$docs" "$(ledger_with placeholder 's#a deprecated SDK version#TBD#')" validate
check "an unknown disposition fails" 1 "disposition 'done' is not one of" \
  run_with "$docs" "$(ledger_with disposition 's#| matched |#| done |#')" validate
check "a malformed hash fails" 1 "is not 12 lowercase hex digits in backticks" \
  run_with "$docs" "$(ledger_with bad-hash "s#\`$(hash_of alpha)\`#$(hash_of alpha)#")" validate
check "a second row for a page fails" 1 "second row for alpha (first at line 5)" \
  run_with "$docs" "$(ledger_with duplicate '5p')" validate
check "a row with a missing cell fails" 1 "3 cells, want 4" \
  run_with "$docs" "$(ledger_with short-row 's#| matched | ADR018 § Services & lifecycle: Suspend / Resume |#| matched |#')" validate
check "a Page cell without a docs link fails" 1 "the Page cell must link exactly one" \
  run_with "$docs" "$(ledger_with no-link 's#\[`alpha`\](https://render.com/docs/alpha)#alpha#')" validate
check "a malformed row fails check too" 1 "1 malformed rows" \
  run_with "$docs" "$(ledger_with check-malformed 's#| matched |#| done |#')"

check "an unknown mode is refused" 2 "usage:" "$script" list

# The committed ledger follows the same rules; the weekly job compares it with
# the live pages.
check "the committed ledger is well formed" 0 "rows well formed" \
  env RENDER_DOCS_LEDGER="$committed_ledger" "$script" validate

if [[ "$failures" -gt 0 ]]; then
  echo "$failures render-docs-drift test(s) failed" >&2
  exit 1
fi
echo "all render-docs-drift tests passed"
