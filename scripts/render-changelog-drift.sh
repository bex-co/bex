#!/usr/bin/env bash
# Check, but never update, that every entry on Render's changelog has a
# disposition in docs/render-artifacts/render-changelog.md (w4/m553).
#
# The other Render drift jobs watch API contracts: the Blueprint schema, the
# OpenAPI operations and webhook events (render-schema-drift.sh) and the MCP
# tool surface (render-mcp-drift.sh). A feature Render ships only in its
# dashboard changes none of them, which is how bulk suspend and resume
# (2025-08-27), Postgres admin apps (2024-06-04) and the log line detail view
# (2025-12-17) went 11 to 29 months unnoticed (w4/m550-m552). Render announces
# every feature on its changelog, so this job fails until each new entry has a
# row: matched, partial or gap with the board item that owns it, a non-goal, or
# n/a with the reason.
# What Render ships without announcing it, and what it keeps behind a flag,
# render-dashboard-ops.sh catches from the dashboard bundle itself (w1/m645).
#
# Two reads, because each sees what the other cannot. The Atom feed is the
# stable, structured source, but it carries only the ten newest entries. The
# changelog page lists every entry back to 2023-03-31: it catches a week with
# more than ten entries, and a table row whose entry Render removed. A page
# whose markup this parser no longer understands fails loudly rather than
# reporting nothing, since every row would read as removed.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
table="${RENDER_CHANGELOG_TABLE:-$repo_root/docs/render-artifacts/render-changelog.md}"
feed_url="${RENDER_CHANGELOG_FEED_URL:-https://render.com/changelog/feed.xml}"
page_url="${RENDER_CHANGELOG_PAGE_URL:-https://render.com/changelog}"

if [[ ! -f "$table" ]]; then
  echo "::error title=Render changelog table missing::$table does not exist" >&2
  exit 1
fi

# Under $TMPDIR on every platform (macOS mktemp -t ignores it), so the test can
# assert the cleanup.
work="$(mktemp -d "${TMPDIR:-/tmp}/bex-render-changelog.XXXXXX")"
trap 'rm -rf "$work"' EXIT

fetch() { # fetch <url> <dest>
  if ! curl --fail --location --retry 2 --silent --show-error --max-time 60 "$1" --output "$2"; then
    echo "::error title=Render changelog unreachable::could not fetch $1" >&2
    exit 1
  fi
}
fetch "$feed_url" "$work/feed.xml"
fetch "$page_url" "$work/page.html"

python3 -I - "$work/feed.xml" "$work/page.html" "$table" "$feed_url" "$page_url" <<'PY'
import html
import re
import sys
import xml.etree.ElementTree as ET

feed_path, page_path, table_path, feed_url, page_url = sys.argv[1:6]
ATOM = "{http://www.w3.org/2005/Atom}"
ENTRY_URL = re.compile(r"https://render\.com/changelog/[a-z0-9][a-z0-9-]*")
DISPOSITIONS = ("matched", "partial", "gap", "non-goal", "n/a")
BOARD_ITEM = re.compile(r"\bw\d+/(?:m\d+|\d{3})\b")
# A pointer that is empty or only a placeholder word says nothing.
PLACEHOLDER = re.compile(r"[\W_]*(?:unknown|todo|tbd|unowned|n/?a)?[\W_]*", re.I)


def fail(title, message):
    print(f"::error title={title}::{message}", file=sys.stderr)
    sys.exit(1)


# The feed: the newest entries, each <id> the entry's URL.
try:
    root = ET.parse(feed_path).getroot()
except ET.ParseError as err:
    fail("Render changelog feed malformed", f"{feed_url} is not well-formed XML ({err})")
if root.tag != ATOM + "feed":
    fail("Render changelog feed malformed", f"{feed_url} is not an Atom feed (root element {root.tag})")
entries = {}  # URL -> (date, title)
for entry in root.iter(ATOM + "entry"):
    url = (entry.findtext(ATOM + "id") or "").strip()
    if not ENTRY_URL.fullmatch(url):
        fail("Render changelog feed malformed", f"{feed_url} has an entry whose <id> is not a changelog URL: {url!r}")
    title = (entry.findtext(ATOM + "title") or "").strip()
    entries[url] = ((entry.findtext(ATOM + "updated") or "")[:10], title)
if not entries:
    fail("Render changelog feed malformed", f"{feed_url} has no entries")
feed_count = len(entries)

# The page: every entry, each an <h2> linking to it.
with open(page_path, encoding="utf-8", errors="replace") as handle:
    page = handle.read()
heading = re.compile(
    r'<h2\b[^>]*>\s*<a\b[^>]*\bhref="(?:https://render\.com)?(/changelog/[a-z0-9][a-z0-9-]*)"[^>]*>(.*?)</a>', re.S
)
page_count = 0
for path, title in heading.findall(page):
    page_count += 1
    entries.setdefault("https://render.com" + path, ("", html.unescape(re.sub(r"<[^>]+>", "", title)).strip()))
if page_count == 0:
    fail(
        "Render changelog page unreadable",
        f"found no entry headings on {page_url}; its markup changed, so update the page parser in this script",
    )

# The table: one row per entry, keyed by the entry's URL.
problems = []
rows = {}
with open(table_path, encoding="utf-8") as handle:
    for number, line in enumerate(handle, 1):
        line = line.strip()
        if not line.startswith("|") or re.fullmatch(r"\|[\s:|-]+\|", line):
            continue
        cells = [cell.strip() for cell in re.split(r"(?<!\\)\|", line.strip("|"))]
        if cells[0] == "Date":
            continue
        where = f"{table_path}:{number}"
        if len(cells) != 4:
            problems.append(f"{where}: {len(cells)} cells, want 4 (Date | Entry | Disposition | Pointer)")
            continue
        date, entry, disposition, pointer = cells
        urls = ENTRY_URL.findall(entry)
        if len(set(urls)) != 1:
            problems.append(f"{where}: the Entry cell must link exactly one changelog URL")
            continue
        url = urls[0]
        if url in rows:
            problems.append(f"{where}: second row for {url} (first at line {rows[url]})")
            continue
        rows[url] = number
        if not re.fullmatch(r"\d{4}-\d{2}-\d{2}", date):
            problems.append(f"{where}: date {date!r} is not YYYY-MM-DD")
        if disposition not in DISPOSITIONS:
            problems.append(f"{where}: disposition {disposition!r} is not one of {', '.join(DISPOSITIONS)}")
        elif PLACEHOLDER.fullmatch(pointer) or "UNOWNED" in pointer:
            problems.append(f"{where}: {disposition} row has no pointer")
        elif disposition == "gap" and not BOARD_ITEM.search(pointer):
            problems.append(f"{where}: gap row names no board item (wN/mN or wN/NNN)")
        elif disposition == "partial" and not (BOARD_ITEM.search(pointer) or "ADR018" in pointer):
            problems.append(f"{where}: partial row names neither the board item owning the rest nor an ADR018 divergence")
        elif disposition == "non-goal" and not ("DO_NOT_DO" in pointer or "ADR018" in pointer):
            problems.append(f"{where}: non-goal row cites neither DO_NOT_DO.md nor an ADR018 row")
if not rows:
    fail("Render changelog table unreadable", f"found no rows in {table_path}")

malformed = len(problems)
missing = [url for url in entries if url not in rows]
removed = sorted(url for url in rows if url not in entries)
for url in missing:
    date, title = entries[url]
    problems.append(f"no disposition: {date + ' ' if date else ''}{title} <{url}>")
for url in removed:
    problems.append(f"{table_path}:{rows[url]}: {url} is no longer on Render's changelog")

if problems:
    print(
        f"::error title=Render changelog drift::{len(missing)} undispositioned, {len(removed)} removed, "
        f"{malformed} malformed rows",
        file=sys.stderr,
    )
    for problem in problems:
        print("  " + problem, file=sys.stderr)
    print(
        f"""
Action: read each new entry and add one row to {table_path}, newest first:
  matched   bex ships it: the ADR018 row, ADR, artifact or code that says so
  partial   bex ships part: what it has, and the board item owning the rest
  gap       bex lacks it: the board item that owns it (file one with /pm add)
  non-goal  declined: the DO_NOT_DO.md line or the ADR018 "—" row
  n/a       nothing to match, and why (a Render-internal change, a Render bug fix)
A row whose entry Render removed is deleted. This job never writes the table.""",
        file=sys.stderr,
    )
    sys.exit(1)
print(f"Render changelog: all {len(entries)} entries dispositioned ({feed_count} in the feed, {page_count} on the page)")
PY
