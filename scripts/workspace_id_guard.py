#!/usr/bin/env python3
"""Warn when a push names a workspace on the public board that is not bex's own.

This repository is public, `.pm/` and `docs/` included, and git keeps every
line after it is edited. Only bex's own fixtures may be named there; another
tenant's ids, service or app names, emails and metering are hashed with
scripts/lib/public_log.py or left out (.pm/AGENTS.md, "The board is public").

Every `tea-<xid>` id on a line the push adds under .pm/ or docs/ is listed,
unless it is in scripts/lib/first_party_workspaces.txt (read at <head>), the
same line marks it `(disposable)`, or the push also removes a line naming it
(an edited or moved line adds nothing new). Most ids a push adds are
disposable QA or drill workspaces, so this only warns: /ship's gate prints
the list and ships anyway.

    python3 -B scripts/workspace_id_guard.py <base> [<head>]

Exits 0 when no id needs a look, 1 listing each one, and 2 when a revision
cannot be read. It reads git objects only, so it never depends on the working
tree.
"""

from __future__ import annotations

import re
import subprocess
import sys

PATHS = (".pm", "docs")
ALLOWLIST = "scripts/lib/first_party_workspaces.txt"
# A workspace id is "tea-" and a 20-character xid (lego/backend/internal/id),
# alone or leading a name built on it (tea-<xid>-<service>).
WORKSPACE_ID = re.compile(r"(?<![0-9A-Za-z])tea-[0-9a-v]{20}(?![0-9A-Za-z])")
DISPOSABLE = re.compile(r"\(disposable\)", re.IGNORECASE)
HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@")


def git(*args: str) -> str:
    return subprocess.run(
        ["git", "-c", "core.quotePath=false", *args], check=True, capture_output=True, text=True
    ).stdout


def first_party(head: str) -> set[str]:
    """The ids scripts/lib/first_party_workspaces.txt lists at head: the first
    word of each line, after `#` comments are dropped. A head without the file
    lists none."""
    result = subprocess.run(["git", "show", f"{head}:{ALLOWLIST}"], capture_output=True, text=True)
    if result.returncode != 0:
        return set()
    ids = set()
    for line in result.stdout.splitlines():
        words = line.split("#", 1)[0].split()
        if words and WORKSPACE_ID.fullmatch(words[0]):
            ids.add(words[0])
    return ids


def added_ids(base: str, head: str) -> list[tuple[str, int, str]]:
    """(path, line, id) for each id the push adds under .pm/ or docs/ that is
    neither listed nor marked disposable on its line, in diff order."""
    diff = git("diff", "--no-color", "--no-ext-diff", "--no-textconv", "--find-renames", "--unified=0",
               "--src-prefix=a/", "--dst-prefix=b/", base, head, "--", *PATHS)
    added: list[tuple[str, int, str, str]] = []
    removed: set[str] = set()
    path, number, header = "", 0, False
    for line in diff.splitlines():
        if line.startswith("diff --git "):
            header = True
            continue
        hunk = HUNK.match(line)
        if hunk:
            header, number = False, int(hunk.group(1))
            continue
        if header:
            # File header lines; only the new side's path matters.
            if line.startswith("+++ "):
                target = line[4:]
                path = target[2:] if target.startswith("b/") else ""
            continue
        if line.startswith("+"):
            for found in WORKSPACE_ID.findall(line):
                added.append((path, number, found, line))
            number += 1
        elif line.startswith("-"):
            removed.update(WORKSPACE_ID.findall(line))
    listed = first_party(head)
    seen = set()
    report = []
    for path, number, found, line in added:
        if found in listed or found in removed or DISPOSABLE.search(line) or (path, number, found) in seen:
            continue
        seen.add((path, number, found))
        report.append((path, number, found))
    return report


def main(argv: list[str]) -> int:
    if len(argv) not in (2, 3):
        print("usage: workspace_id_guard.py <base> [<head>]", file=sys.stderr)
        return 2
    base, head = argv[1], argv[2] if len(argv) == 3 else "HEAD"
    try:
        report = added_ids(base, head)
    except subprocess.CalledProcessError as err:
        print(f"workspace_id_guard: cannot read {base} or {head}: {err.stderr.strip()}", file=sys.stderr)
        return 2
    if not report:
        return 0
    for path, number, found in report:
        print(f"workspace_id_guard: warning: {path}:{number} adds workspace {found}, which {ALLOWLIST} "
              "does not list and the line does not mark (disposable)", file=sys.stderr)
    print(
        "workspace_id_guard: this repository is public: mark a QA or drill workspace `(disposable)` on its line, "
        f"add a new bex fixture to {ALLOWLIST}, and hash another tenant's id with scripts/lib/public_log.py "
        "or leave it out (.pm/AGENTS.md); shipping anyway",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
