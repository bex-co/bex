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
"""pm-board-validate.py — the .pm board's forward-looking invariants.

Every worker has its own checkout, so the board's numbering is handed out
without coordination: milestone numbers have already collided across
workstreams (m348 is used by three), and a bare `m348` in a commit message or a
note names no one milestone. Old tasks also carry an `id:` that no longer
matches their path after re-homings. Repairing that history is churn, and the
`wN/` prefix disambiguates it; this gate only stops the board getting worse.

  1. A milestone directory a change ADDS (a new `mN/`, or one moved into `done/`
     or `blocked/`) must not carry a number another workstream already uses,
     counting the base branch's tip and the working tree, less what the change
     itself deletes. Moving a milestone within its own workstream is not a
     duplicate; having it in two places at once is. Adding a task or a note to
     an existing milestone adds no directory, so a historical duplicate stays
     workable.
  2. An open or blocked task (outside `done/`) carries `id: wN/mN/tNNN`, the
     same as its path.

  python3 scripts/pm-board-validate.py [--root DIR] [--base REV]

--base is the branch the change is measured against (default origin/main; CI
passes the PR's base branch, or the commit before a push). A change is the
working tree against the merge base of --base and HEAD, so staged, unstaged and
untracked board files count before they are committed. When --base or the
merge base is missing (a shallow clone, a new branch) the duplicate check is
skipped and the output says so; the id check needs no history.

Exit 0 clean, 1 a violation (each printed), 2 unusable.
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys

MILESTONE_DIR = re.compile(r"^\.pm/(w\d+)/(?:(?:done|blocked)/)?(m\d+)/")
# An open or blocked task: directly under wN/mN/ or wN/blocked/mN/, never under done/.
OPEN_TASK = re.compile(r"^\.pm/(w\d+)/(?:blocked/)?(m\d+)/(t\d+)\.md$")
ID_LINE = re.compile(r"""^id:\s*["']?([^\s"'#]+)["']?\s*(?:#.*)?$""")

Milestone = tuple[str, str, str]  # (number, workstream, directory)


def git(root: str, *args: str) -> str:
    done = subprocess.run(["git", "-C", root, *args], capture_output=True, text=True)
    if done.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)}: {done.stderr.strip()}")
    return done.stdout


def resolves(root: str, rev: str) -> bool:
    return subprocess.run(
        ["git", "-C", root, "rev-parse", "--verify", "--quiet", f"{rev}^{{commit}}"],
        capture_output=True,
    ).returncode == 0


def working_tree_files(root: str) -> list[str]:
    found = []
    for dirpath, _, filenames in os.walk(os.path.join(root, ".pm")):
        for name in filenames:
            found.append(os.path.relpath(os.path.join(dirpath, name), root).replace(os.sep, "/"))
    return sorted(found)


def milestone_dirs(paths: list[str]) -> set[Milestone]:
    found = set()
    for path in paths:
        match = MILESTONE_DIR.match(path)
        if match:
            ws, number = match.groups()
            found.add((number, ws, path[: match.end() - 1]))
    return found


def tree_files(root: str, rev: str) -> list[str]:
    return git(root, "ls-tree", "-r", "--name-only", rev, "--", ".pm").split("\n")


def check_duplicates(root: str, base: str, tree: list[str]) -> tuple[list[str], list[str]]:
    """Problems, and a note when the check could not run."""
    if not resolves(root, base):
        return [], [f"base {base!r} does not resolve; the duplicate check is skipped"]
    try:
        merge_base = git(root, "merge-base", base, "HEAD").strip()
    except RuntimeError:
        return [], [f"{base!r} shares no history with HEAD; the duplicate check is skipped"]
    before = milestone_dirs(tree_files(root, merge_base))
    current = milestone_dirs(tree)
    added, deleted = current - before, before - current
    # What the base branch holds now, less what this change moves or removes.
    taken = current | (milestone_dirs(tree_files(root, base)) - deleted)
    problems = []
    for number, ws, directory in sorted(added):
        elsewhere = sorted({other for n, other, _ in taken if n == number and other != ws})
        if elsewhere:
            problems.append(
                f"{directory} adds milestone {number}, already used by {', '.join(elsewhere)}; "
                "take the next free number (git fetch origin main, then the highest in the tree and "
                "`git ls-tree -r --name-only origin/main -- .pm`, plus one)"
            )
        here = sorted(d for n, w, d in current if n == number and w == ws)
        if len(here) > 1:
            problems.append(f"milestone {ws}/{number} exists in {len(here)} places at once: {', '.join(here)}")
    return sorted(set(problems)), []


def frontmatter_id(path: str) -> str | None:
    with open(path, encoding="utf-8-sig") as handle:
        lines = handle.read().splitlines()
    if not lines or lines[0].strip() != "---":
        return None
    for line in lines[1:]:
        if line.strip() == "---":
            return None
        match = ID_LINE.match(line)
        if match:
            return match.group(1)
    return None


def check_task_ids(root: str, tree: list[str]) -> list[str]:
    problems = []
    for path in tree:
        match = OPEN_TASK.match(path)
        if not match:
            continue
        want = "/".join(match.groups())
        got = frontmatter_id(os.path.join(root, path))
        if got != want:
            problems.append(f"{path} has id: {got if got else '(none)'}, want {want}")
    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    parser.add_argument("--base", default="origin/main")
    args = parser.parse_args()
    root = os.path.abspath(args.root)
    if not os.path.isdir(os.path.join(root, ".pm")):
        print(f"pm-board-validate: no .pm directory under {root}", file=sys.stderr)
        return 2
    tree = working_tree_files(root)
    try:
        duplicates, notes = check_duplicates(root, args.base, tree)
    except RuntimeError as err:
        print(f"pm-board-validate: {err}", file=sys.stderr)
        return 2
    problems = duplicates + check_task_ids(root, tree)
    for note in notes:
        print(f"note: {note}", file=sys.stderr)
    for problem in problems:
        print(f"FAIL: {problem}", file=sys.stderr)
    if problems:
        print(f"pm-board-validate: {len(problems)} violation(s)", file=sys.stderr)
        return 1
    print("pm-board-validate: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
