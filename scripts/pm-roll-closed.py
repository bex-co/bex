#!/usr/bin/env python3
# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""pm-roll-closed.py — roll a workstream README's closed lines into done/.

Nearly all of a workstream README is its closed lines, each with its closing
summary. On 2026-11-23, 467 KB of w4's 493 KB README was `- [x]` lines
(w10/058). scripts/repo-artifact-guard.sh warns about any *.md at 512 KiB and
refuses one at 768 KiB. This script moves every top-level `- [x]` line, and
the indented lines under it, into new files `wN/done/closed-K.md`. Each file
keeps the README sections the lines sat in and their order. The script then
links each new file under the README's `## Closed items` heading.

- Open and parked `- [ ]` lines, headings, prose and other bullets stay, so
  scripts/loopx-guard.py board, scripts/pm_gates.py and
  scripts/pm-board-validate.py read the README as before.
- Nothing is ever appended to an archive file. Each roll starts at the next
  free K, counting the working tree and origin/main. It starts another file
  when the next line would push the current one past --max-bytes (default
  256 KiB, half the warning).
- Relative links on a moved line are rewritten so they resolve from done/.
  scripts/docs-links-validate.sh checks done/closed-K.md as it checks the
  README. A list line has no anchor, so a link into a moved line points at the
  README itself and still resolves.

  python3 scripts/pm-roll-closed.py wN [--root DIR] [--max-bytes N] [--date YYYY-MM-DD]

Exit 0 when it rolled or had nothing to roll, 2 when it cannot run.
"""

from __future__ import annotations

import argparse
import bisect
import datetime
import os
import posixpath
import re
import subprocess
import sys

CLOSED = re.compile(r"[-*+] \[[xX] ?\](?:\s|$)")  # `[x ]` too: a typo w4's board carried
FENCE = re.compile(r"^ {0,3}(`{3,}|~{3,})")
HEADING = "## "
ARCHIVE = re.compile(r"closed-(\d+)\.md")
CLOSED_ITEMS = "## Closed items"
GAP = object()  # where a closed item left the README
LISTISH = re.compile(r"[-*+] |\d+[.)] |[ \t]")
# The same link grammar scripts/docs-links-validate.sh reads.
INLINE = re.compile(r"(\]\(\s*)(<[^>\n]*>|(?:[^()\s]|\([^()\s]*\))*)")
SCHEME = re.compile(r"^[a-zA-Z][a-zA-Z0-9+.-]*:")
BACKTICKS = re.compile(r"`+")


def code_spans(line: str) -> list[tuple[int, int]]:
    """Inline code spans on LINE: a run of n backticks closed by the next run of exactly n."""
    runs = [(m.start(), m.end()) for m in BACKTICKS.finditer(line)]
    by_length: dict[int, list[int]] = {}
    for index, (start, end) in enumerate(runs):
        by_length.setdefault(end - start, []).append(index)
    spans, index = [], 0
    while index < len(runs):
        start, end = runs[index]
        later = by_length[end - start]
        nxt = bisect.bisect_right(later, index)
        if nxt == len(later):
            index += 1
            continue
        closer = later[nxt]
        spans.append((start, runs[closer][1]))
        index = closer + 1
    return spans


def rebase_target(target: str, src: str, dst: str) -> str:
    """TARGET, a link written in directory SRC, as written from directory DST."""
    bracketed = target.startswith("<") and target.endswith(">")
    inner = target[1:-1] if bracketed else target
    if not inner or inner.startswith(("#", "/")) or SCHEME.match(inner):
        return target
    path, sep, fragment = inner.partition("#")
    moved = posixpath.relpath(posixpath.normpath(posixpath.join(src, path)), dst)
    if path.endswith("/"):
        moved += "/"
    moved += sep + fragment
    return f"<{moved}>" if bracketed else moved


def rebase_links(line: str, src: str, dst: str) -> str:
    spans = code_spans(line) if "`" in line else []

    def swap(found: re.Match) -> str:
        if any(start <= found.start() < end for start, end in spans):
            return found.group(0)
        return found.group(1) + rebase_target(found.group(2), src, dst)

    return INLINE.sub(swap, line)


def split_closed(lines: list[str]) -> tuple[list[object], list[tuple[str, list[str]]]]:
    """The README with each closed item replaced by GAP, and the closed items with their section."""
    kept: list[object] = []
    moved: list[tuple[str, list[str]]] = []
    section, fence, index = "", "", 0
    while index < len(lines):
        line = lines[index]
        opened = FENCE.match(line)
        if fence:
            if opened and opened.group(1)[0] == fence[0] and len(opened.group(1)) >= len(fence):
                fence = ""
        elif opened:
            fence = opened.group(1)
        elif line.startswith(HEADING):
            section = line
        elif CLOSED.match(line):
            end = index + 1
            while end < len(lines):
                if lines[end].strip() and lines[end][0] in " \t":
                    end += 1
                    continue
                after = end
                while after < len(lines) and not lines[after].strip():
                    after += 1
                if after > end and after < len(lines) and lines[after][0] in " \t":
                    end = after  # a blank line inside a loose item, then its next indented line
                    continue
                break
            moved.append((section, lines[index:end]))
            kept.append(GAP)
            index = end
            continue
        kept.append(line)
        index += 1
    return kept, moved


def join_kept(kept: list[object]) -> list[str]:
    """Kept lines with each run of gaps closed up: a blank line where it now separates two blocks, one where two meet."""
    out: list[str] = []
    index = 0
    while index < len(kept):
        if kept[index] is not GAP:
            out.append(kept[index])
            index += 1
            continue
        while index < len(kept) and kept[index] is GAP:
            index += 1
        nxt = kept[index] if index < len(kept) else ""
        prev = out[-1] if out else ""
        if prev and nxt and not (LISTISH.match(prev) and LISTISH.match(nxt)):
            out.append("")
        elif not prev and not nxt and index < len(kept):
            index += 1  # blank on both sides of the gap: keep one
    return out


def git(root: str, *args: str) -> str | None:
    done = subprocess.run(["git", "-C", root, *args], capture_output=True, text=True)
    return done.stdout if done.returncode == 0 else None


def next_number(root: str, workstream: str) -> int:
    done_dir = os.path.join(root, ".pm", workstream, "done")
    names = set(os.listdir(done_dir)) if os.path.isdir(done_dir) else set()
    listed = git(root, "ls-tree", "--name-only", "origin/main", f".pm/{workstream}/done/")
    for path in (listed or "").splitlines():
        names.add(posixpath.basename(path))
    used = [int(found.group(1)) for found in map(ARCHIVE.fullmatch, names) if found]
    return max(used, default=0) + 1


def title(section: str) -> str:
    return section[len(HEADING) :].strip() if section else "(top)"


def pack(
    moved: list[tuple[str, list[str]]], workstream: str, first: int, date: str, max_bytes: int
) -> list[tuple[str, list[tuple[str, list[str]]], str]]:
    """Archive files (name, items, text), filled in order, each under MAX_BYTES unless one item alone is larger."""

    def head(number: int) -> str:
        return (
            f"# {workstream} — closed items, part {number}\n\n"
            f"Closed lines rolled out of [the {workstream} README](../README.md) on {date} by "
            "`scripts/pm-roll-closed.py`, in the sections and order they had there. "
            "Nothing is appended to this file later: the next roll writes a new part.\n"
        )

    def lead(where: str) -> str:
        return f"\n{where}\n\n" if where else "\n"

    files: list[tuple[str, list[tuple[str, list[str]]], str]] = []
    number, items, parts, section = first, [], [head(first)], None
    size = len(parts[0].encode())
    for where, block in moved:
        chunk = "\n".join(block) + "\n"
        piece = chunk if items and where == section else lead(where) + chunk
        if items and size + len(piece.encode()) > max_bytes:
            files.append((f"closed-{number}.md", items, "".join(parts)))
            number += 1
            items, parts, piece = [], [head(number)], lead(where) + chunk
            size = len(parts[0].encode())
        parts.append(piece)
        size += len(piece.encode())
        items.append((where, block))
        section = where
    if items:
        files.append((f"closed-{number}.md", items, "".join(parts)))
    return files


def add_index(readme: list[str], workstream: str, entries: list[str]) -> list[str]:
    """README lines with ENTRIES appended to its `## Closed items` list, the section added when missing."""
    try:
        at = readme.index(CLOSED_ITEMS)
    except ValueError:
        end = len(readme)
        while end and readme[end - 1] == "":
            end -= 1
        intro = (
            f"Closed lines leave this README for `done/closed-K.md` files once it passes 384 KiB "
            f"(`python3 scripts/pm-roll-closed.py {workstream}`, [.pm/AGENTS.md](../AGENTS.md)); "
            "each file keeps the sections and order they had here."
        )
        return [*readme[:end], "", CLOSED_ITEMS, "", intro, "", *entries]
    end = at + 1
    while end < len(readme) and not readme[end].startswith(HEADING):
        end += 1
    last = end
    while last > at + 1 and readme[last - 1] == "":
        last -= 1
    return [*readme[:last], *entries, *readme[last:]]


def roll(root: str, workstream: str, max_bytes: int, date: str) -> int:
    rel = f".pm/{workstream}/README.md"
    path = os.path.join(root, rel)
    try:
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
    except OSError as err:
        print(f"pm-roll-closed: {err}", file=sys.stderr)
        return 2
    kept, moved = split_closed(text.split("\n"))
    if not moved:
        print(f"pm-roll-closed: {rel} has no closed lines to roll")
        return 0
    src, dst = f".pm/{workstream}", f".pm/{workstream}/done"
    moved = [(where, [rebase_links(line, src, dst) for line in block]) for where, block in moved]
    files = pack(moved, workstream, next_number(root, workstream), date, max_bytes)
    os.makedirs(os.path.join(root, dst), exist_ok=True)
    entries = []
    for name, items, body in files:
        with open(os.path.join(root, dst, name), "w", encoding="utf-8") as handle:
            handle.write(body)
        sections = ", ".join(dict.fromkeys(title(where) for where, _ in items))
        entries.append(f"- [done/{name}](done/{name}), rolled {date}: {len(items)} closed lines from {sections}")
        print(f"pm-roll-closed: {dst}/{name}: {len(items)} closed lines, {len(body.encode())} bytes")
    body = "\n".join(add_index(join_kept(kept), workstream, entries)).rstrip("\n") + "\n"
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(body)
    print(f"pm-roll-closed: {rel}: {len(text.encode())} -> {len(body.encode())} bytes")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("workstream", help="the workstream, e.g. w4")
    parser.add_argument("--root", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    parser.add_argument("--max-bytes", type=int, default=256 * 1024)
    parser.add_argument("--date", default=datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d"))
    args = parser.parse_args()
    if not re.fullmatch(r"w\d+", args.workstream):
        print(f"pm-roll-closed: {args.workstream!r} is not a workstream like w4", file=sys.stderr)
        return 2
    if args.max_bytes < 1024:
        print("pm-roll-closed: --max-bytes must be at least 1024", file=sys.stderr)
        return 2
    return roll(os.path.abspath(args.root), args.workstream, args.max_bytes, args.date)


if __name__ == "__main__":
    sys.exit(main())
