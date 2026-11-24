#!/usr/bin/env bash
# Docs link gate. The documents an agent starts from (docs/, every AGENTS.md
# and README.md, the .pm conventions at the top of the board and each
# workstream's README.md) link to each other and to the board with relative paths, and nothing checked that those
# paths still exist: a purge of build artifacts, a `/pm` move and a renamed ADR
# each left links pointing at nothing.
#
# Fails when a relative Markdown link, image or reference definition points at
# a file or directory that does not exist, and names each as file:line -> target.
#
# Checked: docs/**/*.md, every AGENTS.md and README.md, .pm/*.md at the top
# level, .pm/wN/README.md (the board's own index, which a `/pm` move breaks
# first) and the closed lines scripts/pm-roll-closed.py rolled out of it into
# .pm/wN/done/closed-K.md. Not checked: the rest of the workstream history under .pm/wN/
# (milestone READMEs, tasks, notes, done/ and blocked/), the vendored and generated trees .prettierignore
# lists, fenced code and inline code, external URLs (any scheme), and absolute
# paths. A target must exist with exactly the case written, since CI is
# case-sensitive and a developer's disk may not be, and must lie inside the
# repository.
#
# #anchors are checked only into the Render parity ledger
# (docs/ADR018-render-parity.md and docs/ADR018/, w10/m607): its sections move
# between files, and a link must name a heading the file it points at has, by
# GitHub's slug rules (lower case, punctuation dropped, spaces to hyphens, -1,
# -2 for a repeated heading) or an <a id>/<a name>. Elsewhere only the file
# must exist.
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
        # The conventions at the top of the board and each workstream's own
        # README with the closed lines rolled out of it, not the rest of the
        # wN history.
        return (rel.count("/") == 1 and base.endswith(".md")) or re.fullmatch(
            r"\.pm/w\d+/(?:README|done/closed-\d+)\.md", rel
        ) is not None
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


def prose(text: str):
    """Yield (line number, line) for each line outside fenced code."""
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
        yield number, line


def targets(text: str):
    """Yield (line number, raw target) for each link outside code."""
    for number, line in prose(text):
        line = without_code_spans(line)
        for match in INLINE.finditer(line):
            yield number, match.group(1)
        reference = REFERENCE.match(line)
        if reference:
            yield number, reference.group(1)


# Files whose #anchors are checked: the parity ledger (w10/m607).
ANCHORED = ("docs/ADR018-render-parity.md", "docs/ADR018/")
HEADING = re.compile(r"^ {0,3}#{1,6}[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$")
HTML_ANCHOR = re.compile(r"""<a\s[^>]*\b(?:id|name)\s*=\s*["']([^"']+)["']""", re.IGNORECASE)
LINK_TEXT = re.compile(r"!?\[([^\]]*)\]\([^)]*\)")


def anchored(rel: str) -> bool:
    return rel == ANCHORED[0] or rel.startswith(ANCHORED[1])


def slug(heading: str) -> str:
    """GitHub's anchor for a heading: its text without link targets, tags and
    emphasis or code markers, lower-cased, with every character that is not a
    letter, digit, underscore, hyphen or space dropped and spaces made hyphens."""
    text = re.sub(r"<[^>]*>", "", LINK_TEXT.sub(r"\1", heading)).replace("`", "").replace("*", "")
    return re.sub(r"[^\w\- ]", "", text.strip().lower()).replace(" ", "-")


_anchors: dict[str, set[str]] = {}


def anchors(rel: str) -> set[str]:
    """The #fragments a document answers: each heading's slug, a repeated one
    suffixed -1, -2 as GitHub does, and every <a id> or <a name>."""
    if rel not in _anchors:
        found: set[str] = set()
        repeats: dict[str, int] = {}
        with open(os.path.join(root, rel), encoding="utf-8") as handle:
            text = handle.read()
        for _, line in prose(text):
            heading = HEADING.match(line)
            if heading:
                base = slug(heading.group(1))
                count = repeats.get(base, 0)
                repeats[base] = count + 1
                found.add(f"{base}-{count}" if count else base)
            found.update(HTML_ANCHOR.findall(line))
        _anchors[rel] = found
    return _anchors[rel]


def missing_anchor(rel: str, target: str) -> bool:
    """Whether target's #fragment names no anchor of rel, a ledger file."""
    if not anchored(rel):
        return False
    fragment = urllib.parse.unquote(target.partition("#")[2])
    return bool(fragment) and fragment not in anchors(rel)


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
    if target.startswith("#"):
        return f"{rel}:{number} -> {target} (no such heading)" if missing_anchor(rel, target) else None
    if not target or target.startswith("/") or SCHEME.match(target):
        return None
    path = urllib.parse.unquote(re.split(r"[#?]", target, maxsplit=1)[0])
    if not path:
        return None
    resolved = os.path.normpath(os.path.join(root, os.path.dirname(rel), path))
    inside = resolved == root or resolved.startswith(root + os.sep)
    if inside and exists_exactly(resolved):
        dest = os.path.relpath(resolved, root)
        if os.path.isfile(resolved) and missing_anchor(dest, target):
            return f"{rel}:{number} -> {target} (no such heading in {dest})"
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
        "file is gone for good (purged evidence, a removed item). A #anchor into the parity "
        "ledger (docs/ADR018-render-parity.md, docs/ADR018/) must name a heading of the file "
        "it points at: point it at the file its section moved to.",
        file=sys.stderr,
    )
    sys.exit(1)
print(f"docs-links-validate: ok ({len(files)} files)")
PY
