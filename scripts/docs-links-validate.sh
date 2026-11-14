#!/usr/bin/env bash
# Docs link gate. The documents an agent starts from (docs/, every AGENTS.md
# and README.md, and the .pm conventions at the top of the board) link to each
# other and to the board with relative paths, and nothing checked that those
# paths still exist: a purge of build artifacts, a `/pm` move and a renamed ADR
# each left links pointing at nothing.
#
# Fails when a relative Markdown link, image or reference definition points at
# a file or directory that does not exist, and names each as file:line -> target.
#
# Checked: docs/**/*.md, every AGENTS.md and README.md, .pm/*.md at the top
# level. Not checked: the workstream history under .pm/wN/ (including each
# workstream's own README), the vendored and generated trees .prettierignore
# lists, fenced code and inline code, external URLs (any scheme), absolute
# paths, and #anchors (a heading slug needs GitHub's rules: the file must exist,
# the heading is not checked). A target must exist with exactly the case
# written, since CI is case-sensitive and a developer's disk may not be, and
# must lie inside the repository.
#
# DOCS_LINKS_ROOT is overridable so the self-test
# (scripts/docs-links-validate.test.sh) can run against throwaway fixture trees.
set -euo pipefail
cd "$(dirname "$0")/.."

export DOCS_LINKS_ROOT="${DOCS_LINKS_ROOT:-$PWD}"

exec python3 - <<'PY'
import bisect
import os
import re
import sys
import urllib.parse

root = os.path.realpath(os.environ["DOCS_LINKS_ROOT"])

# Always pruned while walking; they never hold documents.
SKIP_DIRS = {"node_modules", ".git", ".playwright-mcp", "dist", ".output", ".tanstack", ".turbo"}


def prettierignore() -> tuple[tuple[str, ...], set[str]]:
    """The vendored, generated and scratch trees .prettierignore lists: the one
    place they are named, so a new vendored tree is skipped here too. `.pm/` is
    handled below, not skipped. An entry with a slash inside is a path from the
    repository root; one without matches a directory of that name anywhere."""
    prefixes: list[str] = []
    names: set[str] = set()
    try:
        with open(os.path.join(root, ".prettierignore"), encoding="utf-8") as handle:
            lines = [line.strip() for line in handle]
    except FileNotFoundError:
        return (), names
    for line in lines:
        if not line or line.startswith("#") or not line.endswith("/") or line == ".pm/":
            continue
        if "/" in line[:-1]:
            prefixes.append(line.lstrip("/"))
        else:
            names.add(line[:-1])
    return tuple(prefixes), names


SKIP_PREFIXES, IGNORED_NAMES = prettierignore()


def in_scope(rel: str) -> bool:
    if rel.startswith(SKIP_PREFIXES) or IGNORED_NAMES.intersection(rel.split("/")[:-1]):
        return False
    base = os.path.basename(rel)
    if rel.startswith(".pm/"):
        # The conventions at the top of the board, not the wN history.
        return rel.count("/") == 1 and base.endswith(".md")
    return rel.startswith("docs/") or base in ("AGENTS.md", "README.md")


def markdown_files() -> list[str]:
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for name in sorted(filenames):
            if not name.endswith(".md"):
                continue
            rel = os.path.relpath(os.path.join(dirpath, name), root)
            if in_scope(rel):
                found.append(rel)
    return found


# A backtick fence's info string holds no backtick (else it is inline code), and
# a closing fence is bare: only fence characters and spaces.
FENCE_OPEN = re.compile(r"^ {0,3}(?:(`{3,})[^`]*|(~{3,}).*)$")
FENCE_CLOSE = re.compile(r"^ {0,3}(`{3,}|~{3,}) *$")
BACKTICKS = re.compile(r"`+")
# `](target)`: the text before it may hold brackets, an image or another link (a
# badge), so only the closing bracket and the opening parenthesis are matched. A
# target holds no space and at most one level of balanced parentheses.
INLINE = re.compile(r"\]\(\s*(<[^>\n]*>|(?:[^()\s]|\([^()\s]*\))*)")
# A reference definition, not a footnote (`[^1]: text`).
REFERENCE = re.compile(r"^ {0,3}\[(?!\^)[^\]\n]+\]:\s*(<[^>\n]*>|\S+)")
SCHEME = re.compile(r"^[a-zA-Z][a-zA-Z0-9+.-]*:")


def without_code_spans(line: str) -> str:
    """The line with inline code removed. A run of n backticks opens a span only
    if a later run of exactly n closes it (CommonMark); pairing runs through a
    per-length index keeps this linear in the line, where a regex backtracks."""
    if "`" not in line:
        return line
    runs = [(m.start(), m.end()) for m in BACKTICKS.finditer(line)]
    by_length: dict[int, list[int]] = {}
    for index, (start, end) in enumerate(runs):
        by_length.setdefault(end - start, []).append(index)
    kept, position, index = [], 0, 0
    while index < len(runs):
        start, end = runs[index]
        later = by_length[end - start]
        nxt = bisect.bisect_right(later, index)
        if nxt == len(later):
            index += 1
            continue
        closer = later[nxt]
        kept.append(line[position:start])
        position = runs[closer][1]
        index = closer + 1
    kept.append(line[position:])
    return "".join(kept)


def targets(text: str):
    """Yield (line number, raw target) for each link outside code."""
    fence = ""
    for number, line in enumerate(text.split("\n"), 1):
        line = line.rstrip("\r")
        if fence:
            closer = FENCE_CLOSE.match(line)
            if closer and closer.group(1)[0] == fence[0] and len(closer.group(1)) >= len(fence):
                fence = ""
            continue
        opener = FENCE_OPEN.match(line)
        if opener:
            fence = opener.group(1) or opener.group(2)
            continue
        line = without_code_spans(line)
        for match in INLINE.finditer(line):
            yield number, match.group(1)
        reference = REFERENCE.match(line)
        if reference:
            yield number, reference.group(1)


_listings: dict[str, set[str]] = {}


def exists_exactly(path: str) -> bool:
    """Whether path exists with the case written, a check that does not depend
    on the disk being case-sensitive."""
    if not os.path.exists(path):
        return False
    parent, name = os.path.split(path)
    if parent == path or not name:
        return True
    if parent not in _listings:
        try:
            _listings[parent] = set(os.listdir(parent))
        except OSError:
            return False
    return name in _listings[parent] and exists_exactly(parent)


def broken(rel: str, number: int, raw: str):
    target = raw.strip("<>").strip()
    if not target or target.startswith(("#", "/")) or SCHEME.match(target):
        return None
    path = urllib.parse.unquote(re.split(r"[#?]", target, maxsplit=1)[0])
    if not path:
        return None
    resolved = os.path.normpath(os.path.join(root, os.path.dirname(rel), path))
    inside = resolved == root or resolved.startswith(root + os.sep)
    if inside and exists_exactly(resolved):
        return None
    return f"{rel}:{number} -> {target}"


problems = []
files = markdown_files()
for rel in files:
    with open(os.path.join(root, rel), encoding="utf-8") as handle:
        text = handle.read()
    for number, raw in targets(text):
        problem = broken(rel, number, raw)
        if problem:
            problems.append(problem)

if problems:
    for problem in problems:
        print(f"FAIL: {problem}", file=sys.stderr)
    print(
        f"docs-links-validate: {len(problems)} broken relative link(s) in {len(files)} file(s). "
        "Point each at the file's current path, or write the path as plain text when the "
        "file is gone for good (purged evidence, a removed item).",
        file=sys.stderr,
    )
    sys.exit(1)
print(f"docs-links-validate: ok ({len(files)} files)")
PY
