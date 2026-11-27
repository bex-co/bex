#!/usr/bin/env python3
"""Hash every page in Render's docs index and compare with the ledger (w4/m688).

    render_docs_pages.py check    <docs base URL> <ledger> [cache dir]
    render_docs_pages.py hashes   <docs base URL> [page ...]
    render_docs_pages.py validate <ledger>

Render lists its documentation in <base>/llms.txt, one markdown link per page,
and serves a markdown copy of each page at <base>/<page>.md. `check` reads the
index and every page it lists, hashes each page's normalized text, and compares
the result with docs/render-artifacts/docs-pages.md, which has one row per page:

    | Page | Disposition | Pointer | Hash |
    | [`free`](https://render.com/docs/free) | matched | ADR018 § ... | `0123456789ab` |

It fails, naming the page and its row's pointer, on a page with no row, a row
whose page llms.txt no longer lists, a page whose hash moved, a listed page it
cannot fetch, and a row that breaks the disposition rules. It never writes the
ledger: a person re-reads the page, re-checks the row's pointer, and copies the
new hash from the failure (or from `hashes`).

With a cache directory, `check` keeps each normalized page under its hash
(`<cache>/<hash>.md`), and for a changed page whose recorded hash it kept, prints
the paragraphs that changed. Only the pages the ledger or this fetch names stay.
`validate` applies the row rules alone, without the network.

The two topic bundles (`sandboxes/all`, `workflows/all`) concatenate pages the
index already lists, so they are not pages here.
"""

from __future__ import annotations

import concurrent.futures
import dataclasses
import difflib
import hashlib
import os
import re
import sys
import time
import urllib.error
import urllib.request

USER_AGENT = "Mozilla/5.0 (compatible; bex-render-docs-drift)"
MAX_BYTES = 4 * 1024 * 1024
WORKERS = 8
HASH_LENGTH = 12
# `[Title](https://render.com/docs/<page>.md)`; a bare URL in llms.txt's
# preamble (`https://render.com/docs/api.md#setup`) is not an index entry.
INDEX_LINK = re.compile(r"\[([^\]]*)\]\(https://render\.com/docs/([A-Za-z0-9][A-Za-z0-9/_-]*)\.md\)")
ROW_LINK = re.compile(r"\]\(https://render\.com/docs/([A-Za-z0-9][A-Za-z0-9/_-]*)\)")
ROW_HASH = re.compile(r"`([0-9a-f]{%d})`" % HASH_LENGTH)
DISPOSITIONS = ("matched", "partial", "gap", "non-goal", "n/a")
BOARD_ITEM = re.compile(r"\bw\d+/(?:m\d+|\d{3})\b")
# A pointer that is empty or only a placeholder word says nothing.
PLACEHOLDER = re.compile(r"[\W_]*(?:unknown|todo|tbd|unowned|n/?a)?[\W_]*", re.I)
# What a re-render changes without Render changing the page: the index banner
# under the title, image placeholders, trailing whitespace and blank-line runs.
BANNER = re.compile(r">\s*For the complete documentation index\b.*")
IMAGE = re.compile(r"\s*(?:\[image:[^\]]*\]|!\[[^\]]*\]\([^)]*\))\s*")
DIFF_PARAGRAPHS = 12
DIFF_CHARS = 600


class FetchError(Exception):
    pass


def fetch(url: str) -> str:
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    error: Exception | None = None
    for attempt in range(3):
        if attempt:
            time.sleep(2 * attempt)
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                body = response.read(MAX_BYTES + 1)
            if len(body) > MAX_BYTES:
                raise FetchError(f"{url} is larger than {MAX_BYTES} bytes")
            return body.decode("utf-8", errors="replace")
        except urllib.error.HTTPError as err:
            error = err
            if err.code < 500 and err.code != 429:
                break
        except (urllib.error.URLError, OSError) as err:
            error = err
            if url.startswith("file:"):
                break
    raise FetchError(f"could not fetch {url} ({error})")


def normalize(text: str) -> str:
    lines = []
    for line in text.replace("\r\n", "\n").split("\n"):
        line = line.rstrip()
        if BANNER.fullmatch(line) or IMAGE.fullmatch(line):
            continue
        if not line and (not lines or not lines[-1]):
            continue
        lines.append(line)
    return "\n".join(lines).strip("\n") + "\n"


def digest(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:HASH_LENGTH]


def index(base: str) -> dict[str, str]:
    """Every page llms.txt lists, in its order: page -> title."""
    source = base + "/llms.txt"
    pages: dict[str, str] = {}
    for title, page in INDEX_LINK.findall(fetch(source)):
        pages.setdefault(page, title.strip())
    if not pages:
        raise FetchError(f"{source} lists no [title](https://render.com/docs/<page>.md) link; its markup changed")
    return pages


def read_pages(base: str, pages: list[str]) -> tuple[dict[str, str], dict[str, str]]:
    """page -> normalized text, and page -> fetch error."""
    texts: dict[str, str] = {}
    errors: dict[str, str] = {}
    with concurrent.futures.ThreadPoolExecutor(max_workers=WORKERS) as pool:
        futures = {pool.submit(fetch, f"{base}/{page}.md"): page for page in pages}
        for future in concurrent.futures.as_completed(futures):
            page = futures[future]
            try:
                texts[page] = normalize(future.result())
            except FetchError as err:
                errors[page] = str(err)
    return texts, errors


@dataclasses.dataclass
class Row:
    number: int
    disposition: str
    pointer: str
    hash: str

    def summary(self) -> str:
        return f"{self.disposition}: {self.pointer}"


def read_ledger(path: str) -> tuple[dict[str, Row], list[str]]:
    rows: dict[str, Row] = {}
    problems: list[str] = []
    with open(path, encoding="utf-8") as handle:
        for number, line in enumerate(handle, 1):
            line = line.strip()
            if not line.startswith("|") or re.fullmatch(r"\|[\s:|-]+\|", line):
                continue
            cells = [cell.strip() for cell in re.split(r"(?<!\\)\|", line.strip("|"))]
            if cells[0] == "Page":
                continue
            where = f"{path}:{number}"
            if len(cells) != 4:
                problems.append(f"{where}: {len(cells)} cells, want 4 (Page | Disposition | Pointer | Hash)")
                continue
            page_cell, disposition, pointer, hash_cell = cells
            links = ROW_LINK.findall(page_cell)
            if len(set(links)) != 1:
                problems.append(f"{where}: the Page cell must link exactly one https://render.com/docs/<page> URL")
                continue
            page = links[0]
            if page in rows:
                problems.append(f"{where}: second row for {page} (first at line {rows[page].number})")
                continue
            hashed = ROW_HASH.fullmatch(hash_cell)
            rows[page] = Row(number, disposition, pointer, hashed.group(1) if hashed else "")
            if not hashed:
                problems.append(f"{where}: hash {hash_cell!r} is not {HASH_LENGTH} lowercase hex digits in backticks")
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
    if not rows and not problems:
        problems.append(f"{path}: found no rows")
    return rows, problems


def clip(paragraph: str) -> str:
    flat = " ".join(paragraph.split())
    return flat if len(flat) <= DIFF_CHARS else flat[: DIFF_CHARS - 1] + "…"


def changed_paragraphs(old: str, new: str) -> list[str]:
    before = [p for p in old.split("\n\n") if p.strip()]
    after = [p for p in new.split("\n\n") if p.strip()]
    lines: list[str] = []
    for tag, i1, i2, j1, j2 in difflib.SequenceMatcher(None, before, after, autojunk=False).get_opcodes():
        if tag == "equal":
            continue
        lines += ["- " + clip(p) for p in before[i1:i2]]
        lines += ["+ " + clip(p) for p in after[j1:j2]]
    if len(lines) > DIFF_PARAGRAPHS:
        lines = lines[:DIFF_PARAGRAPHS] + [f"… {len(lines) - DIFF_PARAGRAPHS} more changed paragraph(s)"]
    return lines


def cached(cache: str | None, hash_: str) -> str | None:
    """The page text an earlier run kept under hash_, if any."""
    if not cache:
        return None
    try:
        with open(os.path.join(cache, hash_ + ".md"), encoding="utf-8") as handle:
            return handle.read()
    except FileNotFoundError:
        return None


def update_cache(cache: str, texts: dict[str, str], keep: set[str]) -> None:
    os.makedirs(cache, exist_ok=True)
    for text in texts.values():
        with open(os.path.join(cache, digest(text) + ".md"), "w", encoding="utf-8") as handle:
            handle.write(text)
    for name in os.listdir(cache):
        if name.endswith(".md") and name[:-3] not in keep:
            os.remove(os.path.join(cache, name))


def check(base: str, ledger: str, cache: str | None) -> int:
    url = base.rstrip("/")
    try:
        listed = index(url)
    except FetchError as err:
        print(f"::error title=Render docs index unreadable::{err}", file=sys.stderr)
        return 1
    rows, problems = read_ledger(ledger)
    texts, errors = read_pages(url, list(listed))
    hashes = {page: digest(text) for page, text in texts.items()}
    malformed = len(problems)

    new = [page for page in listed if page not in rows]
    gone = sorted((page for page in rows if page not in listed), key=lambda page: rows[page].number)
    changed = sorted(
        (page for page in listed if page in rows and page in hashes and rows[page].hash and rows[page].hash != hashes[page]),
        key=lambda page: rows[page].number,
    )
    for page, error in sorted(errors.items()):
        problems.append(f"unreadable: {page} is in llms.txt but {error}")
    for page in new:
        hash_ = hashes.get(page, "?")
        problems.append(f"new page: {listed[page]} <https://render.com/docs/{page}> (hash {hash_}) has no row")
    for page in gone:
        problems.append(f"gone: {ledger}:{rows[page].number}: {page} is no longer in llms.txt (was {rows[page].summary()})")
    no_earlier = 0
    for page in changed:
        row = rows[page]
        problems.append(
            f"changed: {ledger}:{row.number}: https://render.com/docs/{page} hash {row.hash} -> {hashes[page]} "
            f"(re-check {row.summary()})"
        )
        earlier = cached(cache, row.hash)
        if earlier is None:
            no_earlier += 1
        else:
            problems += ["    " + line for line in changed_paragraphs(earlier, texts[page])]

    if cache:
        update_cache(cache, texts, {row.hash for row in rows.values()} | set(hashes.values()))

    if problems:
        print(
            f"::error title=Render docs drift::{len(new)} new, {len(gone)} gone, {len(changed)} changed, "
            f"{len(errors)} unreadable, {malformed} malformed rows",
            file=sys.stderr,
        )
        for problem in problems:
            print("  " + problem, file=sys.stderr)
        if changed and no_earlier:
            where = "the cache had no earlier fetch of" if cache else "no cache directory was given for"
            print(f"  ({where} {no_earlier} changed page(s), so their paragraphs are not shown)", file=sys.stderr)
        print(
            f"""
Action: read each page named above and fix its row in {ledger}:
  new      add a row with its disposition, pointer and the hash printed above
  gone     delete the row, after checking whether the page moved to a new slug
  changed  re-read the page, re-check the row's disposition and pointer (and the
           ADR018 row it names), then replace the hash with the new one
Dispositions: matched (the ADR018 row, ADR or code), partial or gap (the board
item that owns the rest; file one with /pm add), non-goal (DO_NOT_DO.md or the
ADR018 "—" row), n/a (the reason). This job never writes the ledger.""",
            file=sys.stderr,
        )
        return 1
    print(f"Render docs: all {len(listed)} pages in llms.txt match their ledger rows")
    return 0


def print_hashes(base: str, pages: list[str]) -> int:
    url = base.rstrip("/")
    try:
        pages = pages or list(index(url))
    except FetchError as err:
        print(f"::error title=Render docs index unreadable::{err}", file=sys.stderr)
        return 1
    texts, errors = read_pages(url, pages)
    for page in pages:
        if page in texts:
            print(f"{page}\t{digest(texts[page])}")
    for page, error in sorted(errors.items()):
        print(f"::error title=Render docs page unreadable::{page}: {error}", file=sys.stderr)
    return 1 if errors else 0


def validate(ledger: str) -> int:
    rows, problems = read_ledger(ledger)
    if problems:
        print(f"::error title=Render docs ledger malformed::{len(problems)} malformed rows", file=sys.stderr)
        for problem in problems:
            print("  " + problem, file=sys.stderr)
        return 1
    print(f"Render docs ledger: {len(rows)} rows well formed")
    return 0


def main() -> int:
    args = sys.argv[1:]
    if len(args) in (3, 4) and args[0] == "check":
        return check(args[1], args[2], args[3] if len(args) == 4 else None)
    if len(args) >= 2 and args[0] == "hashes":
        return print_hashes(args[1], args[2:])
    if len(args) == 2 and args[0] == "validate":
        return validate(args[1])
    print(
        "usage: render_docs_pages.py check <docs base URL> <ledger> [cache dir]\n"
        "       render_docs_pages.py hashes <docs base URL> [page ...]\n"
        "       render_docs_pages.py validate <ledger>",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
