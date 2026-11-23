#!/usr/bin/env bash
# Unit tests for render-changelog-drift.sh. Network-free: the feed and the page
# are file:// URLs to fixtures shaped like Render's (scripts/fixtures/
# render-changelog/), so what is under test is the script's own logic: which
# entries count as dispositioned, which rows it refuses, and that a feed or page
# it cannot read fails with a message instead of passing.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-changelog-drift.sh"
fixtures="$repo_root/scripts/fixtures/render-changelog"
committed_table="$repo_root/docs/render-artifacts/render-changelog.md"

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

sandbox="$(mktemp -d -t bex-changelog-drift-test.XXXXXX)"
trap 'rm -rf "$sandbox"' EXIT
mkdir "$sandbox/tmp"

# run_with <feed> <page> <table>: the script against these three files, with
# its temporary directory inside the sandbox so the cleanup can be asserted.
run_with() {
  TMPDIR="$sandbox/tmp" \
    RENDER_CHANGELOG_FEED_URL="file://$1" \
    RENDER_CHANGELOG_PAGE_URL="file://$2" \
    RENDER_CHANGELOG_TABLE="$3" \
    "$script"
}

feed="$fixtures/feed.xml"
page="$fixtures/page.html"
table="$fixtures/table.md"

# A variant of the fixture table with one row replaced or added.
table_with() { # table_with <name> <sed expression>
  sed -e "$2" "$table" >"$sandbox/$1.md"
  echo "$sandbox/$1.md"
}

check "every entry dispositioned passes" 0 "all 3 entries dispositioned (2 in the feed, 3 on the page)" \
  run_with "$feed" "$page" "$table"

# A new entry in the feed with no row: the job's whole purpose.
sed 's#<entry>#<entry><title type="html"><![CDATA[Bulk suspend services]]></title><id>https://render.com/changelog/bulk-suspend-services</id><updated>2026-11-20T00:00:00.000Z</updated></entry><entry>#' \
  "$feed" >"$sandbox/feed-new.xml"
check "a feed entry with no row fails and is named" 1 \
  "no disposition: 2026-11-20 Bulk suspend services <https://render.com/changelog/bulk-suspend-services>" \
  run_with "$sandbox/feed-new.xml" "$page" "$table"

# An entry only on the page, as in a week with more than the feed's ten.
check "a page-only entry with no row fails" 1 \
  "no disposition: Assign access roles to team members <https://render.com/changelog/access-roles>" \
  run_with "$feed" "$page" "$(table_with no-access-roles '/access-roles/d')"

# A row whose entry Render removed is reported, never silently kept.
check "a row whose entry left the changelog fails" 1 \
  "https://render.com/changelog/an-entry-render-deleted is no longer on Render's changelog" \
  run_with "$feed" "$page" "$(table_with removed-entry '$a\
| 2024-01-01 | [Gone](https://render.com/changelog/an-entry-render-deleted) | n/a | Render bug fix |')"

check "a malformed feed fails with a clear message" 1 "is not well-formed XML" \
  run_with "$(head -c 400 "$feed" >"$sandbox/truncated.xml" && echo "$sandbox/truncated.xml")" "$page" "$table"

printf '<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>Service unavailable<br></body></html>\n' >"$sandbox/error-page.xml"
check "an HTML error page where the feed was fails" 1 "is not well-formed XML" \
  run_with "$sandbox/error-page.xml" "$page" "$table"

printf '<?xml version="1.0"?>\n<rss version="2.0"><channel><title>Render</title></channel></rss>\n' >"$sandbox/rss.xml"
check "a feed that is not Atom fails" 1 "is not an Atom feed" \
  run_with "$sandbox/rss.xml" "$page" "$table"

printf '<?xml version="1.0"?>\n<feed xmlns="http://www.w3.org/2005/Atom"><id>https://render.com</id></feed>\n' >"$sandbox/empty.xml"
check "an empty feed fails" 1 "has no entries" \
  run_with "$sandbox/empty.xml" "$page" "$table"

sed 's#<id>https://render.com/changelog/render-sandboxes-now-available-in-beta</id>#<id>https://example.com/post</id>#' \
  "$feed" >"$sandbox/foreign-id.xml"
check "a feed entry whose id is not a changelog URL fails" 1 "whose <id> is not a changelog URL" \
  run_with "$sandbox/foreign-id.xml" "$page" "$table"

# Render changed the page's markup: every row would read as removed, so the
# script says what broke instead.
sed 's#<h2#<h3#g; s#</h2>#</h3>#g' "$page" >"$sandbox/redesigned.html"
check "a page with no entry headings fails" 1 "found no entry headings" \
  run_with "$feed" "$sandbox/redesigned.html" "$table"

check "an unreachable feed fails" 1 "could not fetch file://$sandbox/absent.xml" \
  run_with "$sandbox/absent.xml" "$page" "$table"

check "a gap row with no board item fails" 1 "gap row names no board item" \
  run_with "$feed" "$page" "$(table_with gap-unowned 's#| matched | `w9/m496` |#| gap | nobody yet |#')"

check "a gap row naming its owner passes" 0 "all 3 entries dispositioned" \
  run_with "$feed" "$page" "$(table_with gap-owned 's#| matched | `w9/m496` |#| gap | `w4/711` |#')"

check "a placeholder pointer fails" 1 "matched row has no pointer" \
  run_with "$feed" "$page" "$(table_with unknown 's#| matched | ADR024 |#| matched | unknown |#')"

check "an UNOWNED marker fails" 1 "partial row has no pointer" \
  run_with "$feed" "$page" "$(table_with unowned-marker 's#the rest is `w2/m263`#UNOWNED#')"

check "a non-goal row must cite DO_NOT_DO or ADR018" 1 "non-goal row cites neither" \
  run_with "$feed" "$page" "$(table_with non-goal 's#| matched | ADR024 |#| non-goal | we decided |#')"

check "a partial row must name what owns the rest" 1 "partial row names neither" \
  run_with "$feed" "$page" "$(table_with partial 's#the rest is `w2/m263`#the rest later#')"

check "an unknown disposition fails" 1 "disposition 'done' is not one of" \
  run_with "$feed" "$page" "$(table_with done-word 's#| matched | ADR024 |#| done | ADR024 |#')"

check "a duplicate row fails" 1 "second row for https://render.com/changelog/access-roles" \
  run_with "$feed" "$page" "$(table_with duplicate '$a\
| 2023-10-12 | [Assign access roles to team members](https://render.com/changelog/access-roles) | matched | ADR024 |')"

check "a row with a missing cell fails" 1 "3 cells, want 4" \
  run_with "$feed" "$page" "$(table_with short-row 's#| matched | ADR024 |#| ADR024 |#')"

check "a malformed date fails" 1 "date 'Oct 12, 2023' is not YYYY-MM-DD" \
  run_with "$feed" "$page" "$(table_with date 's#| 2023-10-12 |#| Oct 12, 2023 |#')"

# Every failure above ran with TMPDIR in the sandbox: none may leave its
# downloads behind.
if [[ -n "$(ls -A "$sandbox/tmp")" ]]; then
  echo "FAIL: the script left temporary files behind: $(ls -A "$sandbox/tmp")" >&2
  failures=$((failures + 1))
else
  echo "ok: no temporary files left behind, failures included"
fi

# The committed table itself, offline: every row is well formed, and the page
# built from its rows' URLs reads back as all dispositioned. The live comparison
# is the weekly job's; this catches a bad row on the push that adds it.
{
  echo '<html><body>'
  grep -E '^\|' "$committed_table" | grep -oE 'https://render\.com/changelog/[a-z0-9-]+' | sort -u |
    sed -E 's#https://render\.com(/changelog/[a-z0-9-]+)#<h2><a class="inline-block" href="\1">entry</a></h2>#'
  echo '</body></html>'
} >"$sandbox/committed-page.html"
# sed reads to the end: a reader that stops early (grep -m1) SIGPIPEs the
# pipeline under pipefail.
newest="$(grep -E '^\|' "$committed_table" | grep -oE 'https://render\.com/changelog/[a-z0-9-]+' | sed -n 1p)"
printf '<?xml version="1.0"?>\n<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>%s</id></entry></feed>\n' \
  "$newest" >"$sandbox/committed-feed.xml"
check "the committed table is well formed" 0 "entries dispositioned" \
  run_with "$sandbox/committed-feed.xml" "$sandbox/committed-page.html" "$committed_table"

if [[ "$failures" -gt 0 ]]; then
  echo "$failures render-changelog-drift test(s) failed" >&2
  exit 1
fi
echo "all render-changelog-drift tests passed"
