#!/usr/bin/env python3
"""Inventory Render's public dashboard bundle: every GraphQL operation and
every feature-flag gate (w1/m645).

The changelog ledger (render-changelog-drift.sh) sees only what Render
announces. A feature Render ships in its dashboard alone, or keeps behind a
per-workspace experiment flag, changes no API contract and no changelog
entry. The dashboard's JavaScript is public and readable without signing in,
and every GraphQL operation it can send is a literal in it, so this reads the
whole bundle and lists them:

  render_dashboard_ops.py list  <dashboard URL>
      the inventory as JSON: entry chunk, chunk count, operations (kind, name,
      root fields, chunk families) and flags (name, chunk families)
  render_dashboard_ops.py check <dashboard URL> <ledger.md>
      compare with docs/render-artifacts/dashboard-operations.md and fail on
      an operation or flag with no row, or a row whose operation or flag is
      gone from the bundle

The crawl starts at the dashboard page's module script (`assets/index-*.js`)
and follows every static import, dynamic import and preload reference to
another chunk, recursively: a feature two dynamic imports deep (the rollback
dialog) is missed by a one-level read. Chunk file names carry content hashes
that change on every Render deploy, so everything is keyed by operation or
flag name, and chunks only by family (the name without its hash).

A flag gate is a call to one of the hooks a flag chunk exports, under
whatever local name the caller imports it, with a string literal first
argument: `import{r as jx}from"./featureflags-….js"` then
jx(`postgres-parameter-overrides`). Render gates features two ways, so there
are two flag chunks (FLAG_CHUNKS): `featureflags-*.js` reads the workspace's
or user's `featureFlags`, and `frontendFlag-*.js` reads `allFrontendFlags`,
which Render's server evaluates for the signed-in user, workspace and
organization (w1/148: the per-service Authentication setting sits behind the
frontend flag `oauth_proxy`). Rollup moves call sites between chunks from one
deploy to the next (most sit in the entry chunk), so a flag's chunk families
are reported but never fail a check.

The chunks are third-party data: they are read as text and never executed.
Run with `python3 -I`. Anything this parser no longer recognizes (no entry
script, a missing flag chunk, an export clause it cannot read, no operations
or no calls to either flag chunk's hooks) fails with a message naming it
rather than reporting every row as removed. An operation without a name fails
the same way: the ledger rows operations by name, so a nameless one cannot
take a row and must never pass silently.

Exit 0 when the ledger matches, 1 on drift or an unreadable bundle, 2 on bad
usage.
"""

from __future__ import annotations

import concurrent.futures
import json
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

USER_AGENT = "Mozilla/5.0 (compatible; dashboard-ops-inventory)"
MAX_BYTES = 32 * 1024 * 1024
# Vite names chunks `<name>-<8-char hash>.js`; the hash may hold `-` and `_`.
CHUNK = r"[A-Za-z0-9_$.-]+-[A-Za-z0-9_-]{8}\.js"
CHUNK_REF = re.compile(r"[\"'`](?:\./|/?assets/)(" + CHUNK + r")[\"'`]")
ENTRY_SCRIPT = re.compile(r"<script\b[^>]*\bsrc=[\"']([^\"']*?assets/index-[A-Za-z0-9_-]{8}\.js)[\"']")
# The name must be followed by its variables, directives, or selection set,
# so prose such as "a `next` query param" is not an operation.
OPERATION = re.compile(r"(?<![\w$.])(query|mutation|subscription)\s+([A-Za-z_]\w*)\s*(?=[(@{])")
# Named GraphQL fragments: their bodies can hold fields named `query`,
# `mutation`, or `subscription` (e.g. `fragment F on Account { subscription { id } }`)
# which must be excluded from anonymous-operation detection just like fields
# inside named operations.
FRAGMENT = re.compile(r"(?<![\w$.])fragment\s+[A-Za-z_]\w*\s+on\s+[A-Za-z_]\w*\s*(?=[@{])")
# An operation without a name (query { … }, query @client { … },
# subscription($x: …) { … }): GraphQL allows them, the ledger cannot row them
# (rows are keyed by name), and the named pattern above cannot see them, so
# each one validated below is a loud failure naming its chunk, never a silent
# miss.
ANONYMOUS_OPERATION = re.compile(r"(?<![\w$.])(query|mutation|subscription)\s*(?=[(@{])")
# GraphQL variable definitions always begin with `$name: Type`; requiring this
# after `(` rules out JS methods like `query() { return }` and `mutation(x) { foo }`.
GQL_VAR_START = re.compile(r"\s*\$[A-Za-z_]\w*\s*:")
NAME = re.compile(r"[A-Za-z_]\w*")
# One binding of an import or export clause: `name` or `name as other`.
BINDING = re.compile(r"\s*([\w$]+)(?:\s+as\s+([\w$]+))?\s*")
FLAG = re.compile(r"[a-z0-9][a-z0-9_-]*[a-z0-9]")
# Each flag chunk's family and a string its text must hold, so a chunk that
# keeps the name but no longer evaluates flags fails instead of passing empty.
FLAG_CHUNKS = (("featureflags", "featureFlags"), ("frontendFlag", "allFrontendFlags"))
LEDGER_DISPOSITIONS = ("matched", "partial", "gap", "non-goal", "n/a", "flagged")
BOARD_ITEM = re.compile(r"\bw\d+/(?:m\d+|\d{3})\b")
PLACEHOLDER = re.compile(r"[\W_]*(?:unknown|todo|tbd|unowned|n/?a)?[\W_]*", re.I)


class BundleError(Exception):
    """The bundle cannot be read as this parser expects."""

    def __init__(self, title: str, message: str):
        super().__init__(message)
        self.title = title


def family(chunk: str) -> str:
    """`metrics-stream-y-IZN_8E.js` -> `metrics-stream`."""
    return chunk[: -len(".js") - 9]


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
                raise BundleError("Render dashboard bundle unreadable", f"{url} is larger than {MAX_BYTES} bytes")
            return body.decode("utf-8", errors="replace")
        except urllib.error.HTTPError as err:
            error = err
            if err.code < 500 and err.code != 429:
                break
        except (urllib.error.URLError, OSError) as err:
            error = err
            if url.startswith("file:"):
                break
    raise BundleError("Render dashboard unreachable", f"could not fetch {url} ({error})")


def crawl(page_url: str) -> tuple[str, dict[str, str]]:
    """Return the entry chunk's name and every chunk reachable from it."""
    page = fetch(page_url)
    match = ENTRY_SCRIPT.search(page)
    if not match:
        raise BundleError(
            "Render dashboard entry chunk not found",
            f"no <script src=…assets/index-*.js> on {page_url}; the page changed, so update ENTRY_SCRIPT",
        )
    entry_url = urllib.parse.urljoin(page_url, match.group(1))
    base = entry_url.rsplit("/", 1)[0] + "/"
    entry = entry_url.rsplit("/", 1)[1]
    chunks: dict[str, str] = {}
    wave = {entry}
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        while wave:
            names = sorted(wave)
            for name, text in zip(names, pool.map(lambda n: fetch(base + n), names)):
                chunks[name] = text
            wave = {ref for name in names for ref in CHUNK_REF.findall(chunks[name])} - chunks.keys()
    return entry, chunks


def _skip_space(src: str, i: int) -> int:
    while i < len(src):
        if src[i] in " \t\r\n,":
            i += 1
        elif src[i] == "#":
            while i < len(src) and src[i] != "\n":
                i += 1
        elif src.startswith("\\n", i) or src.startswith("\\t", i):
            i += 2  # escaped in the source, as Render's globalSearchStatuses is
        else:
            break
    return i


def _skip_quoted(src: str, i: int) -> int:
    """From the quote at src[i], return the index of its closing quote."""
    quote, i = src[i], i + 1
    while i < len(src) and src[i] != quote:
        i += 2 if src[i] == "\\" else 1
    return i


def _skip_interpolation(src: str, i: int) -> int:
    """From `${` at src[i], return the index after its `}`: JavaScript, which
    may hold strings and template literals of its own (a query built at run
    time, as Render's per-environment statusQuery is)."""
    depth, i = 0, i + 1
    while i < len(src):
        c = src[i]
        if c in "'\"":
            i = _skip_quoted(src, i)
        elif c == "`":
            i += 1
            while i < len(src) and src[i] != "`":
                if src.startswith("${", i):
                    i = _skip_interpolation(src, i)
                    continue
                i += 2 if src[i] == "\\" else 1
        elif c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return i + 1
        i += 1
    raise ValueError("unterminated ${")


def _skip_group(src: str, i: int, close: str) -> int:
    """From the opener at src[i], return the index after its matching close."""
    opener = src[i]
    depth = 0
    while i < len(src):
        c = src[i]
        if c == '"':
            i = _skip_quoted(src, i)
        elif src.startswith("${", i):
            i = _skip_interpolation(src, i) - 1
        elif c == opener:
            depth += 1
        elif c == close:
            depth -= 1
            if depth == 0:
                return i + 1
        elif c == "`":
            break  # the template literal ended inside the group
        i += 1
    raise ValueError(f"unbalanced {opener}{close}")


def _skip_directives(src: str, i: int) -> int:
    while src.startswith("@", i):
        match = NAME.match(src, i + 1)
        if not match:
            raise ValueError("a directive without a name")
        i = _skip_space(src, match.end())
        if src.startswith("(", i):
            i = _skip_space(src, _skip_group(src, i, ")"))
    return i


def root_fields(src: str, i: int) -> tuple[list[str], int]:
    """Parse the operation whose variables or selection set start at src[i].

    Return its root fields and the index of its closing brace, so callers can
    tell a nested field shaped like an operation from a top-level one.
    """
    i = _skip_space(src, i)
    if src.startswith("(", i):
        i = _skip_space(src, _skip_group(src, i, ")"))
    i = _skip_directives(src, i)
    if not src.startswith("{", i):
        raise ValueError("no selection set")
    fields: list[str] = []
    i += 1
    while True:
        i = _skip_space(src, i)
        if src.startswith("}", i):
            return fields, i
        if src.startswith("${", i):  # root fields chosen at run time
            fields.append("${...}")
            i = _skip_interpolation(src, i)
            continue
        if src.startswith("...", i):
            i = _skip_space(src, i + 3)
            match = NAME.match(src, i)
            if match and match.group() != "on":  # a fragment spread
                fields.append("..." + match.group())
                i = _skip_directives(src, _skip_space(src, match.end()))
                continue
            if match:  # an inline fragment's type condition
                i = _skip_space(src, NAME.match(src, _skip_space(src, match.end())).end())
            i = _skip_directives(src, i)
            sub, _ = root_fields(src, i)  # its fields are root fields too
            fields.extend(sub)
            i = _skip_group(src, i, "}")
            continue
        match = NAME.match(src, i)
        if not match:
            raise ValueError(f"unexpected {src[i:i + 20]!r}")
        name, i = match.group(), _skip_space(src, match.end())
        if src.startswith(":", i):  # an alias
            match = NAME.match(src, _skip_space(src, i + 1))
            if not match:
                raise ValueError("alias without a field")
            name, i = match.group(), _skip_space(src, match.end())
        fields.append(name)
        if src.startswith("(", i):
            i = _skip_space(src, _skip_group(src, i, ")"))
        i = _skip_directives(src, i)
        if src.startswith("{", i):
            i = _skip_group(src, i, "}")


def flag_chunk_exports(chunks: dict[str, str], hooks: str, marker: str) -> tuple[str, list[str]]:
    names = sorted(name for name in chunks if family(name) == hooks)
    if len(names) != 1:
        raise BundleError(
            "Render feature-flag chunk not found",
            f"want one {hooks}-*.js chunk in the import graph, found {len(names) or 'none'}"
            + (f" ({', '.join(names)})" if names else "")
            + "; the flag hooks moved, so update this parser",
        )
    text = chunks[names[0]]
    clause = re.search(r"\bexport\s*\{([^}]*)\}", text)
    exported = []
    if clause:
        for part in clause.group(1).split(","):
            match = BINDING.fullmatch(part)
            if not match:
                exported = []
                break
            exported.append(match.group(2) or match.group(1))
    if not exported or marker not in text:
        raise BundleError(
            "Render feature-flag chunk unreadable",
            f"{hooks} has no export{{…}} clause over {marker} this parser can read; "
            "its export shape changed, so update flag_chunk_exports",
        )
    return names[0], exported


def flag_calls(chunks: dict[str, str]) -> dict[str, dict[str, set[str]]]:
    """Flag name -> the flag chunks whose hooks it is read through, and the
    chunk families that call them with it."""
    flags: dict[str, dict[str, set[str]]] = {}
    for hooks, marker in FLAG_CHUNKS:
        found = hook_calls(chunks, *flag_chunk_exports(chunks, hooks, marker))
        for flag, families in found.items():
            entry = flags.setdefault(flag, {"hooks": set(), "families": set()})
            entry["hooks"].add(hooks)
            entry["families"] |= families
    return flags


def hook_calls(chunks: dict[str, str], flags_chunk: str, exported: list[str]) -> dict[str, set[str]]:
    """Flag name -> chunk families that call one of flags_chunk's hooks with it."""
    imports = re.compile(r"\bimport\s*(\{[^}]*\}|\*\s*as\s+[\w$]+)\s*from\s*[\"']\./" + re.escape(flags_chunk) + r"[\"']")
    flags: dict[str, set[str]] = {}
    for name, text in chunks.items():
        if name == flags_chunk:
            continue
        callees = []
        for spec in imports.findall(text):
            if spec.startswith("*"):
                namespace = spec.split()[-1]
                callees += [re.escape(namespace) + r"\s*\.\s*" + re.escape(export) for export in exported]
                continue
            for part in spec.strip("{} ").split(","):
                match = BINDING.fullmatch(part)
                if match and match.group(1) in exported:
                    callees.append(re.escape(match.group(2) or match.group(1)))
        for callee in callees:
            for call in re.finditer(r"(?<![\w$.])" + callee + r"\s*\(\s*([`'\"])([^`'\"]*)\1", text):
                if FLAG.fullmatch(call.group(2)):
                    flags.setdefault(call.group(2), set()).add(family(name))
    if not flags:
        raise BundleError(
            "Render feature-flag calls not found",
            f"no chunk calls a {family(flags_chunk)} hook with a flag name; the call shape changed, so update hook_calls",
        )
    return flags


def inventory(page_url: str) -> dict:
    entry, chunks = crawl(page_url)
    operations: dict[tuple[str, str], dict] = {}
    anonymous: list[tuple[str, str, list[str]]] = []
    for name, text in sorted(chunks.items()):
        spans: list[tuple[int, int]] = []
        for match in OPERATION.finditer(text):
            key = (match.group(1), match.group(2))
            try:
                fields, end = root_fields(text, match.end())
            except (ValueError, AttributeError) as err:
                raise BundleError(
                    "Render dashboard operation unreadable",
                    f"{key[0]} {key[1]} in {family(name)}: {err}; update root_fields",
                ) from None
            spans.append((match.start(), end))
            op = operations.setdefault(key, {"kind": key[0], "name": key[1], "fields": [], "families": set()})
            op["fields"] = sorted(set(op["fields"]) | set(fields))
            op["families"].add(family(name))
        for match in FRAGMENT.finditer(text):
            try:
                _, end = root_fields(text, match.end())
            except (ValueError, AttributeError):
                continue
            spans.append((match.start(), end))
        for match in ANONYMOUS_OPERATION.finditer(text):
            if any(start <= match.start() < end for start, end in spans):
                continue  # a nested field of a named operation or fragment, not an operation
            pos = _skip_space(text, match.end())
            if text.startswith("(", pos) and not GQL_VAR_START.match(text, pos + 1):
                continue  # a JS method/call like `query() { return }` or `mutation(x) { foo }`
            try:
                fields, _ = root_fields(text, match.end())
            except (ValueError, AttributeError):
                continue  # a JS call or prose shaped like one; named ops carry the inventory
            anonymous.append((match.group(1), family(name), fields))
    if anonymous:
        detail = "; ".join(
            f"anonymous {kind} in {fam} (root fields {', '.join(fields) or 'none'})"
            for kind, fam, fields in anonymous
        )
        raise BundleError(
            "Render dashboard anonymous operation",
            f"{detail}: an operation without a name has no ledger row; "
            "extend this parser and the ledger to row it",
        )
    if not operations:
        raise BundleError(
            "Render dashboard operations not found",
            f"found no GraphQL operations in {len(chunks)} chunks; the bundle format changed, so update OPERATION",
        )
    flags = flag_calls(chunks)
    return {
        "entry": entry,
        "chunks": len(chunks),
        "operations": [
            {**op, "families": sorted(op["families"])} for _, op in sorted(operations.items(), key=lambda kv: kv[0][::-1])
        ],
        "flags": [
            {"name": flag, "hooks": sorted(found["hooks"]), "families": sorted(found["families"])}
            for flag, found in sorted(flags.items())
        ],
    }


def read_ledger(path: str) -> tuple[dict, dict, list[str]]:
    """Rows keyed by `kind name` (operations) and by flag name, plus problems.

    An operation row is `| Operation | Kind | Disposition | Pointer |`, a flag
    row `| Flag | Called from | Disposition | Pointer |`; the header row says
    which table a row belongs to.
    """
    operations: dict[str, tuple[int, str, str]] = {}
    flags: dict[str, tuple[int, str, str, str]] = {}
    problems: list[str] = []
    table = None
    with open(path, encoding="utf-8") as handle:
        for number, line in enumerate(handle, 1):
            line = line.strip()
            if not line.startswith("|"):
                table = None  # a table ends at its first other line
                continue
            if re.fullmatch(r"\|[\s:|-]+\|", line):
                continue
            cells = [cell.strip() for cell in re.split(r"(?<!\\)\|", line.strip("|"))]
            if cells[0] in ("Operation", "Flag"):
                table = cells[0]
                continue
            where = f"{path}:{number}"
            if table is None:
                problems.append(f"{where}: a row outside an Operation or Flag table")
                continue
            if len(cells) != 4:
                problems.append(f"{where}: {len(cells)} cells, want 4")
                continue
            key_cell, second, disposition, pointer = cells
            key = key_cell.strip("`")
            if table == "Operation":
                if second not in ("query", "mutation", "subscription") or not NAME.fullmatch(key):
                    problems.append(f"{where}: want `name` | query, mutation or subscription")
                    continue
                key = f"{second} {key}"
                seen = operations
            else:
                if not FLAG.fullmatch(key):
                    problems.append(f"{where}: flag {key!r} is not a flag name")
                    continue
                seen = flags
            if key in seen:
                problems.append(f"{where}: second row for {key} (first at line {seen[key][0]})")
                continue
            seen[key] = (number, disposition, pointer) + ((second,) if table == "Flag" else ())
    return operations, flags, problems


def check_row(where: str, disposition: str, pointer: str, flags: dict) -> str | None:
    if disposition not in LEDGER_DISPOSITIONS:
        return f"{where}: disposition {disposition!r} is not one of {', '.join(LEDGER_DISPOSITIONS)}"
    if PLACEHOLDER.fullmatch(pointer) or "UNOWNED" in pointer:
        return f"{where}: {disposition} row has no pointer"
    if disposition == "gap" and not BOARD_ITEM.search(pointer):
        return f"{where}: gap row names no board item (wN/mN or wN/NNN)"
    if disposition == "partial" and not (BOARD_ITEM.search(pointer) or "ADR018" in pointer):
        return f"{where}: partial row names neither the board item owning the rest nor an ADR018 divergence"
    if disposition == "non-goal" and not ("DO_NOT_DO" in pointer or "ADR018" in pointer):
        return f"{where}: non-goal row cites neither DO_NOT_DO.md nor an ADR018 row"
    if disposition == "flagged" and not any(f"`{flag}`" in pointer for flag in flags):
        return f"{where}: flagged row names no flag from the Flag table"
    return None


def check(page_url: str, ledger: str) -> int:
    found = inventory(page_url)
    operations, flags, malformed = read_ledger(ledger)
    if not operations or not flags:
        raise BundleError("Render dashboard ledger unreadable", f"found no Operation or no Flag rows in {ledger}")
    for number, disposition, pointer in operations.values():
        problem = check_row(f"{ledger}:{number}", disposition, pointer, flags)
        if problem:
            malformed.append(problem)
    for name, (number, disposition, pointer, _) in flags.items():
        if disposition != "flagged":
            malformed.append(f"{ledger}:{number}: flag {name} has disposition {disposition!r}, want flagged")
        elif not (BOARD_ITEM.search(pointer) or "ADR018" in pointer):
            malformed.append(f"{ledger}:{number}: flag {name} names no board item or ADR018 row to reopen")

    in_bundle = {f"{op['kind']} {op['name']}": op for op in found["operations"]}
    flag_families = {flag["name"]: flag["families"] for flag in found["flags"]}
    missing = [
        f"no row: {key} (root fields {', '.join(op['fields'])}; in {', '.join(op['families'])})"
        for key, op in in_bundle.items()
        if key not in operations
    ] + [
        f"no row: flag {name} (called from {', '.join(families)}): a new experiment; mark what it gates flagged"
        for name, families in flag_families.items()
        if name not in flags
    ]
    gone = [
        f"{ledger}:{number}: {key} is no longer in Render's dashboard bundle (pointer: {pointer})"
        for key, (number, _, pointer) in operations.items()
        if key not in in_bundle
    ] + [
        f"{ledger}:{number}: flag gate {name} is gone from Render's dashboard bundle: Render made the feature GA "
        f"or dropped it; reopen {pointer}"
        for name, (number, _, pointer, _) in flags.items()
        if name not in flag_families
    ]
    for name, (number, _, _, called_from) in flags.items():
        recorded = sorted(re.findall(r"[\w$.-]+", called_from.replace("`", "")))
        if name in flag_families and recorded != flag_families[name]:
            print(
                f"::notice title=Render flag call sites moved::{ledger}:{number}: flag {name} is now called from "
                f"{', '.join(flag_families[name])} (recorded: {', '.join(recorded) or 'none'}); Rollup moves "
                "call sites between deploys, so this never fails"
            )

    summary = (
        f"{len(found['operations'])} operations and {len(found['flags'])} flags in {found['chunks']} chunks "
        f"(entry {found['entry']})"
    )
    if not (missing or gone or malformed):
        print(f"Render dashboard: every operation and flag has a row: {summary}")
        return 0
    print(
        f"::error title=Render dashboard drift::{len(missing)} without a row, {len(gone)} gone from the bundle, "
        f"{len(malformed)} malformed rows ({summary})",
        file=sys.stderr,
    )
    for problem in missing + gone + malformed:
        print("  " + problem, file=sys.stderr)
    print(
        f"""
Action: give each new operation or flag one row in {ledger}:
  matched   bex ships it: the ADR018 row, ADR, artifact or code that says so
  partial   bex ships part: the board item owning the rest, or the ADR018 divergence
  gap       bex lacks it: the board item that owns it (file one with /pm add)
  non-goal  declined: the DO_NOT_DO.md line or the ADR018 "—" row
  n/a       nothing to match (Render-internal, staff-only), and why
  flagged   gated per workspace: the flag from the Flag table
Delete the row of an operation that is gone. A flag gate that is gone means
Render made the feature GA or dropped it: reopen what its row names first.
`scripts/render-dashboard-ops.sh list` prints each operation's root fields.
This job never writes the ledger.""",
        file=sys.stderr,
    )
    return 1


def main(argv: list[str]) -> int:
    if len(argv) == 2 and argv[0] == "list":
        print(json.dumps(inventory(argv[1]), indent=1))
        return 0
    if len(argv) == 3 and argv[0] == "check":
        return check(argv[1], argv[2])
    print(__doc__.split("\n\n")[2], file=sys.stderr)
    return 2


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except BundleError as err:
        print(f"::error title={err.title}::{err}", file=sys.stderr)
        sys.exit(1)
