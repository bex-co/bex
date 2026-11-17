#!/usr/bin/env python3
"""Refuse a push that adds a backend migration numbered below its base's highest.

golang-migrate records one schema version per database and `Migrate()`'s
`m.Up()` applies only the migrations numbered above it. A migration that
reaches main with a number at or below one main already carries is applied by
every fresh database (CI, each dev-N), so every suite stays green, and skipped
for good by every database already past that number, production included.
0283_audit_source_ip_user_agent reached main after 0284_cron_minimum had been
pinned, production never ran it, and the workspace audit log answered 500
while every audit row was dropped (w4/701). A branch that rebases over a newer
migration keeps its old number, so nothing else catches this.

Every `NNNN_*.up.sql` under lego/backend/internal/store/migrations that <head>
has and <base> does not must number above the highest migration on <base>.

    python3 -B scripts/migration_order_guard.py <base> [<head>]

Exits 0 when every added migration numbers above the base's highest, 1 naming
each one that does not, and 2 when a revision cannot be read. It reads git
objects only, so it never depends on the working tree.
"""

from __future__ import annotations

import re
import subprocess
import sys

MIGRATIONS = "lego/backend/internal/store/migrations/"
UP = re.compile(r"^(\d+)_.*\.up\.sql$")


def migrations(rev: str) -> dict[str, int]:
    """The up migrations at rev, file name -> version."""
    out = subprocess.run(
        ["git", "ls-tree", "--name-only", rev, "--", MIGRATIONS],
        check=True, capture_output=True, text=True,
    ).stdout
    found = {}
    for path in out.splitlines():
        name = path.rsplit("/", 1)[-1]
        match = UP.match(name)
        if match:
            found[name] = int(match.group(1))
    return found


def check(base: str, head: str = "HEAD") -> tuple[str | None, list[str]]:
    """(the base's highest migration, the added migrations numbered at or below it)."""
    before = migrations(base)
    if not before:
        return None, []
    highest = max(before, key=lambda name: (before[name], name))
    after = migrations(head)
    low = sorted(name for name, version in after.items() if name not in before and version <= before[highest])
    return highest, low


def main(argv: list[str]) -> int:
    if len(argv) not in (2, 3):
        print("usage: migration_order_guard.py <base> [<head>]", file=sys.stderr)
        return 2
    base, head = argv[1], argv[2] if len(argv) == 3 else "HEAD"
    try:
        highest, low = check(base, head)
    except subprocess.CalledProcessError as err:
        print(f"migration_order_guard: cannot read {base} or {head}: {err.stderr.strip()}", file=sys.stderr)
        return 2
    if not low:
        return 0
    number = UP.match(highest).group(1)
    for name in low:
        print(f"migration_order_guard: {MIGRATIONS}{name} numbers at or below {highest}, the highest migration on {base}", file=sys.stderr)
    print(
        f"migration_order_guard: renumber above {number}; a lower number is skipped on any database already past it "
        "(golang-migrate applies only versions above the one it recorded, w4/701)",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
