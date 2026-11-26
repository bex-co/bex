#!/usr/bin/env python3
"""Read Render's per-language version pages into one JSON document (w3/m637).

    render_version_pages.py <docs base URL>

Render lists its version pages in <base>/llms.txt and serves a markdown copy
of each at <base>/<page>.md. Each page has a header table giving the current
default and, below it, the default by service creation date:

    | Service Creation Date | Default Node.js Version |
    |---|---|
    | 2026-10-21 and later | `24.22.1` |
    | 2026-09-17 | `24.21.0` |
    | Before 2023-11-01 | `14.17.0` |

The output, keyed by page, is what lego/operator/internal/build's
TestRenderVersionDrift compares with bex's dated tables:

    {"source": ".../llms.txt", "pages": {"node-version": {
        "url": ".../node-version.md", "defaults": {"Node.js": "24.22.1"},
        "since": "2026-10-21", "tool": "Node.js",
        "rows": [{"from": "2026-10-21", "version": "24.22.1"}, ...,
                 {"before": "2023-11-01", "version": "14.17.0"}]}}}

It fails closed, naming the page: an eighth version page in llms.txt, a page
llms.txt no longer lists or that cannot be fetched, and any markup this
parser does not recognise. A page it cannot read must never pass as one
whose rows all vanished.
"""

from __future__ import annotations

import json
import re
import sys
import time
import urllib.error
import urllib.request

# The pages this job watches, each with at least one bex table in
# render_version_drift_test.go. A page Render adds fails the run by name until
# bex has a table for it and it is listed here.
WATCHED = (
    "bun-version",
    "elixir-erlang-versions",
    "node-version",
    "poetry-version",
    "python-version",
    "ruby-version",
    "uv-version",
)
USER_AGENT = "Mozilla/5.0 (compatible; bex-render-version-drift)"
MAX_BYTES = 4 * 1024 * 1024
DATE = r"\d{4}-\d{2}-\d{2}"
VERSION = re.compile(r"\d+(?:\.\d+){1,3}")
# Poetry's oldest rows name a series, not a release.
SERIES = re.compile(r"\d+\.\d+\.x \(various\)")
PAGE_LINK = re.compile(r"\]\(https://render\.com/docs/([a-z0-9-]+-versions?)\.md\)")
CURRENT_HEADER = re.compile(r"Current default (\S.*?) version|Current defaults", re.I)
HISTORY_HEADER = re.compile(r"Default (\S.*?) Version", re.I)
# `*`24.22.1`*` on a one-tool page, `*Elixir `1.18.4`*` on the Elixir page.
DEFAULT = re.compile(r"\*(?:([A-Za-z][\w./]*) )?`([^`]*)`\*")
SINCE = re.compile(r"Services created before \*(" + DATE + r")\* have (?:a )?different default versions?")
ROW_DATE = re.compile(r"(" + DATE + r") and later|(" + DATE + r")|Before (" + DATE + r")")


class PageError(Exception):
    """A page, or llms.txt, is not what this parser reads."""

    def __init__(self, page: str, message: str):
        super().__init__(message)
        self.page = page


def fetch(url: str, page: str) -> str:
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    error: Exception | None = None
    for attempt in range(3):
        if attempt:
            time.sleep(2 * attempt)
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                body = response.read(MAX_BYTES + 1)
            if len(body) > MAX_BYTES:
                raise PageError(page, f"{url} is larger than {MAX_BYTES} bytes")
            return body.decode("utf-8", errors="replace")
        except urllib.error.HTTPError as err:
            error = err
            if err.code < 500 and err.code != 429:
                break
        except (urllib.error.URLError, OSError) as err:
            error = err
            if url.startswith("file:"):
                break
    raise PageError(page, f"could not fetch {url} ({error}); Render removed or moved the page")


def tables(text: str) -> list[list[list[str]]]:
    """Every markdown table in text: rows of cells, separator rows dropped."""
    found: list[list[list[str]]] = []
    current: list[list[str]] | None = None
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("|"):
            current = None
            continue
        if re.fullmatch(r"\|[\s:|-]+\|", line):
            continue
        if current is None:
            current = []
            found.append(current)
        current.append([cell.strip() for cell in line.strip("|").split("|")])
    return found


def version(page: str, text: str) -> str:
    if not (VERSION.fullmatch(text) or SERIES.fullmatch(text)):
        raise PageError(page, f"{text!r} is not a version this parser reads")
    return text


def parse(page: str, url: str, text: str) -> dict:
    found = tables(text)
    header = [t for t in found if t and CURRENT_HEADER.fullmatch(t[0][0])]
    history = [t for t in found if t and len(t[0]) == 2 and t[0][0] == "Service Creation Date"]
    if len(header) != 1:
        raise PageError(page, f"found {len(header)} 'Current default … version' tables, want 1; the markup changed")
    if len(history) != 1:
        raise PageError(page, f"found {len(history)} 'Service Creation Date' tables, want 1; the markup changed")
    header, history = header[0], history[0]
    if len(header) != 2:
        raise PageError(page, f"the current-default table has {len(header) - 1} rows, want 1")

    # The current default(s), from the header table's first cell.
    one_tool = CURRENT_HEADER.fullmatch(header[0][0]).group(1)
    cell = header[1][0]
    named = DEFAULT.findall(cell)
    if one_tool:
        if len(named) != 1 or named[0][0]:
            raise PageError(page, f"the current default cell {cell!r} does not name one {one_tool} version")
        defaults = {one_tool: version(page, named[0][1])}
    else:
        if not named or not all(name for name, _ in named):
            raise PageError(page, f"the current defaults cell {cell!r} does not name each tool's version")
        defaults = {name: version(page, v) for name, v in named}
    since = SINCE.search(cell)
    since = since.group(1) if since else None

    # The history table, newest first.
    match = HISTORY_HEADER.fullmatch(history[0][1])
    tool = match.group(1) if match else None
    if tool not in defaults:
        raise PageError(page, f"the history table's column {history[0][1]!r} names no tool the header gives a default for")
    rows = []
    for number, cells in enumerate(history[1:]):
        last = number == len(history) - 2
        if len(cells) != 2:
            raise PageError(page, f"history row {'|'.join(cells)!r} has {len(cells)} cells, want 2")
        date, value = cells
        dated = ROW_DATE.fullmatch(date)
        quoted = re.fullmatch(r"`([^`]+)`", value)
        if not dated or not quoted:
            raise PageError(page, f"history row {date!r} | {value!r} is not '<date> | `<version>`'")
        later, plain, before = dated.groups()
        if (later is not None) != (number == 0) or (before is not None and not last):
            raise PageError(page, f"history row {date!r} is out of place: only the first row says 'and later', only the last 'Before'")
        row = {"before": before} if before else {"from": later or plain}
        row["version"] = version(page, quoted.group(1))
        if rows and "from" in row and row["from"] >= rows[-1]["from"]:
            raise PageError(page, f"history row {row['from']} is not older than the row above it ({rows[-1]['from']})")
        rows.append(row)
    if not rows:
        raise PageError(page, "the history table has no rows")
    if rows[0]["version"] != defaults[tool]:
        raise PageError(page, f"the current default {tool} {defaults[tool]} is not the history table's newest row ({rows[0]['version']})")
    if since and since != rows[0]["from"]:
        raise PageError(page, f"the header dates the current default from {since}, the history table from {rows[0]['from']}")
    return {"url": url, "defaults": defaults, "since": since, "tool": tool, "rows": rows}


def report(source: str, problems: list[tuple[str, str]]) -> int:
    print(f"::error title=Render version pages unreadable::{len(problems)} problem(s) reading {source}", file=sys.stderr)
    for page, message in problems:
        print(f"  {page}: {message}", file=sys.stderr)
    return 1


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: render_version_pages.py <docs base URL>", file=sys.stderr)
        return 2
    base = sys.argv[1].rstrip("/")
    source = base + "/llms.txt"
    problems: list[tuple[str, str]] = []
    pages: dict[str, dict] = {}
    try:
        listed = set(PAGE_LINK.findall(fetch(source, "llms.txt")))
        if not listed:
            raise PageError("llms.txt", f"{source} lists no */docs/*-version(s).md page; its markup changed")
    except PageError as err:
        return report(source, [(err.page, str(err))])
    for page in sorted(listed - set(WATCHED)):
        problems.append((page, f"Render lists a version page this job does not watch: {base}/{page}.md. "
                               "Give it a bex table in render_version_drift_test.go and add it to WATCHED"))
    for page in WATCHED:
        url = f"{base}/{page}.md"
        if page not in listed:
            problems.append((page, f"{source} no longer lists {page}; Render removed or renamed the page"))
            continue
        try:
            pages[page] = parse(page, url, fetch(url, page))
        except PageError as err:
            problems.append((err.page, str(err)))
    if problems:
        return report(source, problems)
    json.dump({"source": source, "pages": pages}, sys.stdout, indent=2, sort_keys=True)
    print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
