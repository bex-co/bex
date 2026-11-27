#!/usr/bin/env python3
"""Inventory Render's public dashboard bundle: every GraphQL operation and
every feature-flag gate (w1/m645), and every dashboard URL (w3/m704).

The changelog ledger (render-changelog-drift.sh) sees only what Render
announces. A feature Render ships in its dashboard alone, or keeps behind a
per-workspace experiment flag, changes no API contract and no changelog
entry. The dashboard's JavaScript is public and readable without signing in,
and every GraphQL operation it can send is a literal in it, so this reads the
whole bundle and lists them:

  render_dashboard_ops.py list  <dashboard URL>
      the inventory as JSON: entry chunk, chunk count, operations (kind, name,
      root fields, chunk families), flags (name, chunk families), URLs
      (pattern, sources, chunk families, query parameters) and the URL
      expressions it could not read
  render_dashboard_ops.py check <dashboard URL> <ledger.md> <routes.md>
      compare with docs/render-artifacts/dashboard-operations.md and the URL
      table of docs/render-artifacts/dashboard-routes.md, and fail on an
      operation, flag or URL with no row, or a row whose operation, flag or
      URL is gone from the bundle

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

A URL comes from the `urls-*.js` chunk's builders, from the route tables'
`path` values (a child joined to its parent), from `path` values elsewhere
that evaluate to a dashboard path (create menus, sidebars) and from side
navigation's `hrefPatterns`; see the Dashboard URLs section below. A route or
navigation expression it cannot read is listed under `unresolved_urls` and
printed as a notice: it never fails a check, because the factories and
descendant `<Routes>` it cannot follow are stable from week to week, but it is
never dropped silently. The URL chunk missing, no builder or no route table
read fails like the rest.

Exit 0 when the ledger matches, 1 on drift or an unreadable bundle, 2 on bad
usage.
"""

from __future__ import annotations

import concurrent.futures
import json
import re
import sys
import time
from typing import NamedTuple
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


# Dashboard URLs (w3/m704) ----------------------------------------------------
#
# Render builds its dashboard links in one chunk, `urls-*.js`: one builder per
# page, such as Qe=e=>`/env-group/${e.id}`. Its routers add the paths only a
# router knows, in route objects ({path:…, Component:…, children:[…]}) whose
# path is often a builder call (path:na(`:serviceTypeSlug`)) or a constant
# imported from the urls chunk under a local name, and its side navigation adds
# `hrefPatterns`. Reading them takes a little JavaScript: a lexer that knows
# strings, template literals, regular expressions and comments; each chunk's
# top-level declarations, imports and exports; and an evaluator for what a path
# is built from (template literals, `+`, conditionals, arrow-function calls,
# member access, TypeScript string enums). Nothing is executed. A value known
# only at run time becomes `:param`, and an expression the evaluator cannot
# read is reported by name, never dropped.

URLS_CHUNK = "urls"
# A route object has a path and one of these keys; a pathless layout route has
# children and an element; an app table ({app:…, routes:[…]}) roots a tree.
ROUTE_KEYS = frozenset({"Component", "element", "children", "loader", "lazy", "action", "ErrorBoundary", "errorElement"})
SCANNED_KEYS = ROUTE_KEYS | {"path", "routes", "hrefPatterns"}
PARAM = "\x00"  # a part of a URL known only at run time
MAX_ALTERNATIVES = 64
_LEX = re.compile(r"\s*(?:([A-Za-z_$][\w$]*)|(\d[\w.]*)|(\.\.\.|=>|===|!==|==|!=|<=|>=|&&|\|\||\?\?|\?\.(?!\d)|\S))")
_TEMPLATE_STOP = re.compile(r"[`\\]|\$\{")
# A `/` after one of these starts a regular expression, not a division.
_REGEX_AFTER = frozenset(
    "( , = : [ ! & | ? { } ; + - * % < > ~ ^ => === !== == != <= >= && || ?? ...".split()
    + "return typeof case do else in of new delete void throw instanceof yield await".split()
    + [""]
)
IMPORT = re.compile(r"\bimport\s*(\{[^}]*\}|\*\s*as\s+[\w$]+)\s*from\s*[\"']\./(" + CHUNK + r")[\"']")
EXPORT = re.compile(r"\bexport\s*\{([^}]*)\}")
# A TypeScript string enum: function(e){return e.web=`web`,…,e}({}).
ENUM_BODY = re.compile(r"\{\s*return\s*((?:[\w$]+\.[\w$]+\s*=\s*(?:`[^`$\\]*`|\"[^\"\\]*\"),\s*)+)[\w$]+\s*\}")
ENUM_MEMBER = re.compile(r"[\w$]+\.([\w$]+)\s*=\s*(?:`([^`]*)`|\"([^\"]*)\")")
# Globals whose value a URL can carry only at run time (`?next=${window.location.pathname}`).
RUNTIME_GLOBALS = frozenset({"window", "location", "document", "globalThis"})
IDENTITY_GLOBALS = frozenset({"encodeURIComponent", "encodeURI", "String"})
ASSET_SEGMENT = re.compile(r"\.[A-Za-z0-9]{1,5}$")
NOT_PAGES = ("assets", "api", "graphql")


class Token(NamedTuple):
    """kind is id, num, str, tpl, re or punct; a template literal's parts are
    its strings and the (start, end) spans of its interpolations, in order."""

    kind: str
    text: str
    start: int
    end: int
    parts: tuple = ()


def lex(src: str, i: int = 0):
    """JavaScript tokens from src[i:], skipping whitespace and comments."""
    prev = ""
    while True:
        match = _LEX.match(src, i)
        if not match:
            return
        start = match.start(match.lastindex)
        if match.group(1):
            token = Token("id", match.group(1), start, match.end())
        elif match.group(2):
            token = Token("num", match.group(2), start, match.end())
        else:
            mark = match.group(3)
            if mark in "'\"":
                end = _skip_quoted(src, start) + 1
                token = Token("str", src[start + 1 : end - 1], start, end)
            elif mark == "`":
                end, parts = _template(src, start)
                token = Token("tpl", "", start, end, parts)
            elif mark == "/" and src.startswith("//", start):
                newline = src.find("\n", start)
                i = len(src) if newline < 0 else newline
                continue
            elif mark == "/" and src.startswith("/*", start):
                close = src.find("*/", start + 2)
                i = len(src) if close < 0 else close + 2
                continue
            elif mark == "/" and prev in _REGEX_AFTER and (end := _regex_end(src, start)):
                token = Token("re", src[start:end], start, end)
            else:
                token = Token("punct", mark, start, match.end())
        prev = token.text if token.kind in ("id", "punct") else "value"
        yield token
        i = token.end


def _regex_end(src: str, i: int) -> int | None:
    """From the `/` at src[i], the index after the regular expression's flags,
    or None when no `/` closes it on the line (then it was a division)."""
    i, in_class = i + 1, False
    while i < len(src):
        c = src[i]
        if c == "\\":
            i += 2
            continue
        if c == "\n":
            return None
        if in_class:
            in_class = c != "]"
        elif c == "[":
            in_class = True
        elif c == "/":
            i += 1
            while i < len(src) and (src[i].isalnum() or src[i] in "_$"):
                i += 1
            return i
        i += 1
    return None


def _template(src: str, i: int) -> tuple[int, tuple]:
    """From the backtick at src[i], the index after the literal and its parts."""
    parts: list = []
    text, i = "", i + 1
    while True:
        stop = _TEMPLATE_STOP.search(src, i)
        if not stop:
            raise ValueError("unterminated template literal")
        text += src[i : stop.start()]
        mark = stop.group()
        if mark == "`":
            parts.append(text)
            return stop.end(), tuple(parts)
        if mark == "\\":
            text += src[stop.start() : stop.start() + 2]
            i = stop.start() + 2
            continue
        parts.append(text)
        text = ""
        close = _closing(src, stop.end())
        parts.append((stop.end(), close))
        i = close + 1


def _expression_end(src: str, i: int, separators: tuple = (",", ";")) -> int:
    """The index where the expression starting at src[i] ends: its first
    top-level separator or closing bracket, or the end of src."""
    depth = 0
    for token in lex(src, i):
        if token.kind != "punct":
            continue
        if token.text in ("(", "[", "{"):
            depth += 1
        elif token.text in (")", "]", "}"):
            if depth == 0:
                return token.start
            depth -= 1
        elif depth == 0 and token.text in separators:
            return token.start
    return len(src)


def _closing(src: str, i: int) -> int:
    """The index of the bracket that closes the group src[i:] is inside."""
    end = _expression_end(src, i, ())
    if end == len(src):
        raise ValueError("unbalanced brackets")
    return end


def _elements(src: str, i: int) -> list[int]:
    """The start of each element of the array or argument list opening at src[i]."""
    starts = []
    i += 1
    while True:
        token = next(lex(src, i), None)
        if token is None or token.text in ("]", ")"):
            return starts
        starts.append(token.start)
        i = _expression_end(src, token.start)
        if src.startswith(",", i):
            i += 1


def _bindings(clause: str) -> list[tuple[str, str]]:
    """`a as b, c` -> [(a, b), (c, c)]."""
    pairs = []
    for part in clause.strip("{} ").split(","):
        match = BINDING.fullmatch(part)
        if match and match.group(1):
            pairs.append((match.group(1), match.group(2) or match.group(1)))
    return pairs


class Module:
    """One chunk: its top-level declarations, its imports and exports, and
    the object keys route tables are made of."""

    def __init__(self, name: str, src: str):
        self.name, self.src = name, src
        self.decls: dict[str, int] = {}  # name -> where its value (or `function`) starts
        self.objects: dict[int, dict[str, int]] = {}  # object start -> key -> where its value starts
        self.local: set[int] = set()  # objects inside a function body, where a short name may be a local
        self.imports: dict[str, tuple[str, str]] = {}  # local name -> (chunk, exported name)
        self.namespaces: dict[str, str] = {}  # local name -> chunk
        self.exports = {exported: local for clause in EXPORT.findall(src) for local, exported in _bindings(clause)}
        for spec, chunk in IMPORT.findall(src):
            if spec.startswith("*"):
                self.namespaces[spec.split()[-1]] = chunk
            else:
                self.imports.update({local: (chunk, exported) for exported, local in _bindings(spec)})
        self._scan()

    def _scan(self) -> None:
        stack: list[int] = []
        bodies: list[bool] = []  # per open bracket: is it a function body or a block
        in_var = False
        before = last = Token("punct", ";", 0, 0)
        for token in lex(self.src):
            kind, text = token.kind, token.text
            if kind == "punct":
                if text in ("(", "[", "{"):
                    stack.append(token.start)
                    bodies.append(last.text == "=>" or (text == "{" and last.text in (")", "else", "try", "finally", "do")))
                elif text in (")", "]", "}"):
                    if not stack:
                        raise ValueError(f"an unmatched {text} at offset {token.start}")
                    stack.pop()
                    bodies.pop()
                elif not stack and text == ";":
                    in_var = False
                elif text == ":" and last.kind == "id" and before.text in ("{", ","):
                    if stack and self.src[stack[-1]] == "{" and last.text in SCANNED_KEYS:
                        self.objects.setdefault(stack[-1], {})[last.text] = token.end
                        if any(bodies):
                            self.local.add(stack[-1])
                elif text == "=" and in_var and not stack and last.kind == "id":
                    if before.text in ("var", "let", "const", ","):
                        self.decls[last.text] = token.end
            elif kind == "id" and not stack:
                if text in ("var", "let", "const"):
                    in_var = True
                elif last.text == "function":
                    self.decls[text] = last.start
            before, last = last, token
        if stack:
            raise ValueError(f"an unclosed {self.src[stack[-1]]} at offset {stack[-1]}")


class Unresolved(Exception):
    """An expression the evaluator cannot read; the message names what stopped it."""


class Fn:
    def __init__(self, module: Module, params: list[str], body: int, block: bool, env: dict, local: bool):
        self.module, self.params, self.body, self.block, self.env = module, params, body, block, env
        self.local = local

    def returns(self) -> int | None:
        """Where the returned expression starts: the body, or a block that is one return."""
        if not self.block:
            return self.body
        tokens = lex(self.module.src, self.body + 1)
        first = next(tokens, None)
        return first.end if first is not None and first.text == "return" else None


class Obj:
    def __init__(self, module: Module, start: int):
        self.module, self.start = module, start


class Arr:
    """An array literal: (module, start, value or Unresolved) per element, spreads flattened."""

    def __init__(self, items: list):
        self.items = items


class Missing:
    """An argument that could not be read; fails only when the callee uses it."""

    def __init__(self, reason: str):
        self.reason = reason


class Namespace:
    """`import*as N from"./chunk.js"`: N.name is the chunk's export."""

    def __init__(self, module: Module):
        self.module = module


IDENTITY = object()  # encodeURIComponent and friends: the URL keeps the argument's place
UNDEFINED = ()  # no alternatives: `void 0`
RUNTIME = (PARAM,)


def _runtime(value) -> bool:
    return isinstance(value, tuple) and bool(value) and all(alt == PARAM for alt in value)


def _union(*values) -> tuple:
    alts: list[str] = []
    for value in values:
        if not isinstance(value, tuple):
            raise Unresolved("a value that is not a string")
        alts += [alt for alt in value if alt not in alts]
    return tuple(alts[:MAX_ALTERNATIVES])


def _either(yes, no):
    """Both branches of a conditional."""
    if isinstance(yes, Arr) and isinstance(no, Arr):
        return Arr(yes.items + no.items)
    return _union(yes, no)


def _concat(left, right) -> tuple:
    if not isinstance(left, tuple) or not isinstance(right, tuple):
        raise Unresolved("a value that is not a string")
    return tuple(dict.fromkeys(a + b for a in left or ("",) for b in right or ("",)))[:MAX_ALTERNATIVES]


class Bundle:
    """The chunks, scanned on demand, and the value of each top-level binding."""

    def __init__(self, chunks: dict[str, str]):
        self.chunks = chunks
        self.modules: dict[str, Module] = {}
        self.values: dict[tuple[str, int], object] = {}
        self.busy: set[tuple[str, int]] = set()

    def module(self, name: str) -> Module:
        if name not in self.modules:
            if name not in self.chunks:
                raise Unresolved(f"chunk {family(name)} is not in the bundle")
            try:
                self.modules[name] = Module(name, self.chunks[name])
            except ValueError as err:
                raise BundleError(
                    "Render dashboard chunk unreadable", f"{family(name)}: {err}; update the lexer"
                ) from None
        return self.modules[name]

    def locate(self, module: Module, name: str) -> tuple[Module, int]:
        """Where a top-level name's value starts, following imports."""
        for _ in range(8):
            if name in module.decls:
                return module, module.decls[name]
            if name not in module.imports:
                break
            chunk, exported = module.imports[name]
            module = self.module(chunk)
            if exported not in module.exports:
                raise Unresolved(f"{exported} is not exported by {family(chunk)}")
            name = module.exports[exported]
        raise Unresolved(name)

    def binding(self, module: Module, name: str):
        if name in module.namespaces:
            return Namespace(self.module(module.namespaces[name]))
        if name not in module.decls and name not in module.imports:
            if name in RUNTIME_GLOBALS:
                return RUNTIME
            if name in IDENTITY_GLOBALS:
                return IDENTITY
            raise Unresolved(name)
        module, start = self.locate(module, name)
        key = (module.name, start)
        if key not in self.values:
            if key in self.busy:
                raise Unresolved(f"{name} refers to itself")
            self.busy.add(key)
            try:
                self.values[key] = Parser(self, module, start, {}).value()
            except Unresolved as err:
                self.values[key] = err
            finally:
                self.busy.discard(key)
        value = self.values[key]
        if isinstance(value, Unresolved):
            raise value
        return value

    def call(self, fn, args: list):
        if fn is IDENTITY:
            return args[0] if args else RUNTIME
        if _runtime(fn):
            return RUNTIME  # a method of a run-time value: e.split(`-`).at(-1)
        if not isinstance(fn, Fn):
            raise Unresolved("a call of something that is not a function")
        if fn.block:
            enum = ENUM_BODY.match(fn.module.src, fn.body)
            if enum:
                return {m.group(1): (m.group(2) if m.group(2) is not None else m.group(3),) for m in ENUM_MEMBER.finditer(enum.group(1))}
        start = fn.returns()
        if start is None:
            return RUNTIME  # a helper with statements computes its value at run time (a query string)
        env = dict(fn.env)
        for i, param in enumerate(fn.params):
            env[param] = args[i] if i < len(args) else RUNTIME
        return Parser(self, fn.module, start, env, fn.local).value()

    def member(self, value, name: str):
        if _runtime(value):
            return RUNTIME
        if isinstance(value, dict):
            if name in value:
                return value[name]
            raise Unresolved(f".{name}")
        if isinstance(value, Namespace):
            module = value.module
            if name not in module.exports:
                raise Unresolved(f"{name} is not exported by {family(module.name)}")
            return self.binding(module, module.exports[name])
        if isinstance(value, Obj):
            start = self.property(value, name)
            return Parser(self, value.module, start, {}).value()
        raise Unresolved(f".{name}")

    def property(self, obj: Obj, name: str) -> int:
        src, i = obj.module.src, obj.start + 1
        while True:
            token = next(lex(src, i), None)
            if token is None or token.text == "}":
                raise Unresolved(f".{name}")
            after = next(lex(src, token.end), None)
            if token.kind in ("id", "str") and after is not None and after.text == ":":
                if token.text == name:
                    return after.end
            elif token.kind == "id" and token.text == name and after is not None and after.text in (",", "}"):
                return token.start  # shorthand: {name}
            i = _expression_end(src, token.start)
            if src.startswith(",", i):
                i += 1


class Parser:
    """Evaluate the expression starting at module.src[start] in env."""

    COMPARE = frozenset({"===", "!==", "==", "!=", "<", ">", "<=", ">=", "in", "instanceof"})

    def __init__(self, bundle: Bundle, module: Module, start: int, env: dict, local: bool = False):
        # local: the expression sits in a function body, where a one-character
        # name is a minified local or parameter, not the top-level binding it
        # shadows.
        self.bundle, self.module, self.src, self.env, self.local = bundle, module, module.src, env, local
        self.arguments_open = 0
        self.seek(start)

    def seek(self, i: int) -> None:
        self.tokens, self.ahead = lex(self.src, i), []

    def peek(self, k: int = 0):
        while len(self.ahead) <= k:
            self.ahead.append(next(self.tokens, None))
        return self.ahead[k]

    def take(self):
        token = self.peek()
        if token is None:
            raise Unresolved("the end of the chunk")
        self.ahead.pop(0)
        return token

    def at(self, *texts: str) -> bool:
        token = self.peek()
        return token is not None and token.kind in ("punct", "id") and token.text in texts

    def expect(self, text: str) -> None:
        token = self.take()
        if token.text != text:
            raise Unresolved(f"{token.text!r} where {text!r} was expected")

    def value(self):
        test = self.logical()
        if not self.at("?"):
            return test
        self.take()
        yes = self.value()
        self.expect(":")
        return _either(yes, self.value())

    def logical(self):
        value = self.compare()
        while self.at("||", "??", "&&"):
            op = self.take().text
            right = self.compare()
            value = right if op == "&&" else _union(value if isinstance(value, tuple) else (), right)
        return value

    def compare(self):
        value = self.additive()
        if self.at(*self.COMPARE):
            self.take()
            self.additive()
            return RUNTIME
        return value

    def additive(self):
        value = self.unary()
        while self.at("+"):
            self.take()
            value = _concat(value, self.unary())
        return value

    def unary(self):
        if self.at("!", "typeof"):
            self.take()
            self.unary()
            return RUNTIME
        if self.at("void"):
            self.take()
            self.unary()
            return UNDEFINED
        return self.postfix(self.primary())

    def postfix(self, value):
        while True:
            if self.at(".", "?."):
                self.take()
                value = self.bundle.member(value, self.take().text)
            elif self.at("("):
                value = self.bundle.call(value, self.arguments())
            elif self.at("["):
                self.take()
                self.value()
                self.expect("]")
                if not _runtime(value):
                    raise Unresolved("an index into a value known before run time")
            else:
                return value

    def arguments(self) -> list:
        self.expect("(")
        args: list = []
        self.arguments_open += 1
        while not self.at(")"):
            start = self.peek().start
            try:
                args.append(self.value())
            except Unresolved as err:
                args.append(Missing(str(err)))
                self.seek(_expression_end(self.src, start))
            if self.at(","):
                self.take()
        self.take()
        self.arguments_open -= 1
        return args

    def primary(self):
        token = self.take()
        kind, text = token.kind, token.text
        if kind == "tpl":
            return self.template(token)
        if kind in ("str", "num"):
            return (text,)
        if kind == "id":
            if self.at("=>"):
                return self.arrow([text])
            if text == "function":
                if self.peek().kind == "id":
                    self.take()
                return self.arrow(self.params(), arrow=False)
            if text in ("true", "false", "null"):
                return RUNTIME
            if text == "undefined":
                return UNDEFINED
            if text in self.env:
                bound = self.env[text]
                if isinstance(bound, Missing):
                    raise Unresolved(bound.reason)
                return bound
            if self.local and len(text) == 1 and not self.at("("):
                # A local passed to a builder, dereferenced (`g.id`) or
                # tested is a run-time value; a local used as a path, or as
                # part of one, is a path this parser cannot see.
                if self.arguments_open or self.at(".", "?.", "?", "||", "??", "&&", *self.COMPARE):
                    return RUNTIME
                raise Unresolved(f"the local variable {text}")
            return self.bundle.binding(self.module, text)
        if text == "(":
            close = _closing(self.src, token.end)
            after = next(lex(self.src, close + 1), None)
            if after is not None and after.text == "=>":
                self.ahead.insert(0, token)
                return self.arrow(self.params())
            value = self.value()
            self.expect(")")
            return value
        if text == "{":
            close = _closing(self.src, token.end)
            self.seek(close + 1)
            return Obj(self.module, token.start)
        if text == "[":
            items: list = []
            for element in _elements(self.src, token.start):
                spread = self.src.startswith("...", element)
                try:
                    item = Parser(self.bundle, self.module, element + 3 * spread, self.env, self.local).value()
                except Unresolved as err:
                    item = err
                if spread and isinstance(item, Arr):
                    items += item.items
                else:
                    items.append((self.module, element, item))
            self.seek(_closing(self.src, token.end) + 1)
            return Arr(items)
        raise Unresolved(f"{text!r}")

    def params(self) -> list[str]:
        self.expect("(")
        names, depth = [], 0
        while True:
            token = self.take()
            if token.text in ("(", "[", "{"):
                depth += 1
            elif token.text in (")", "]", "}"):
                if depth == 0:
                    return names
                depth -= 1
            elif token.kind == "id" and depth == 0:
                names.append(token.text)

    def arrow(self, params: list[str], arrow: bool = True):
        if arrow:
            self.expect("=>")
        body = self.peek()
        if body is None:
            raise Unresolved("a function without a body")
        block = body.text == "{"
        end = _closing(self.src, body.end) + 1 if block else _expression_end(self.src, body.start)
        self.seek(end)
        return Fn(self.module, params, body.start, block, self.env, self.local)

    def template(self, token):
        value: tuple = ("",)
        for part in token.parts:
            if isinstance(part, str):
                value = _concat(value, (part,))
                continue
            try:
                inner = Parser(self.bundle, self.module, part[0], self.env, self.local).value()
            except Unresolved:
                if not any(mark in alt for alt in value for mark in ("?", "#", "://")):
                    raise
                # A query or hash built at run time (only its names are
                # recorded), or the rest of an external URL (never a row).
                inner = RUNTIME
            if not isinstance(inner, tuple):
                raise Unresolved("an interpolation that is not a string")
            value = _concat(value, inner or ("",))
        return value


def normalize_url(url: str) -> tuple[str, list[str]] | None:
    """A dashboard path pattern and its query parameter names, or None for what
    is not a dashboard page: a relative or external URL, an asset, an API path.

    A segment known only at run time, or a router's `:name`, becomes `:param`;
    a trailing `*` stays; the query is split off; a hash stays in the pattern,
    because an anchor is a link of its own (`/billing#free-usage`).
    """
    if not url.startswith("/") or url.startswith("//"):
        return None
    url, _, fragment = url.partition("#")
    path, _, query = url.partition("?")
    segments = [":param" if PARAM in seg or seg.startswith(":") else seg for seg in path.split("/") if seg]
    if segments and (segments[0] in NOT_PAGES or ASSET_SEGMENT.search(segments[-1])):
        return None
    pattern = "/" + "/".join(segments)
    if fragment:
        pattern += "#" + (":param" if PARAM in fragment else fragment)
    params = sorted({pair.partition("=")[0] for pair in query.split("&") if pair and PARAM not in pair.partition("=")[0]})
    return pattern, params


def _join(parent: str, child: str) -> str:
    """A child route's path under its parent's, as React Router matches it: a
    parent's trailing splat (`/web/:id/*`) does not take part."""
    if child.startswith("/"):
        return child
    if parent.endswith("/*"):
        parent = parent[:-2]
    if not child:
        return parent or "/"
    return parent.rstrip("/") + "/" + child


class UrlInventory:
    def __init__(self, chunks: dict[str, str]):
        self.bundle = Bundle(chunks)
        self.urls: dict[str, dict[str, set[str]]] = {}
        self.unresolved: list[dict] = []

    def add(self, value, source: str, fam: str) -> int:
        added = 0
        for alt in value if isinstance(value, tuple) else ():
            found = normalize_url(alt)
            if found:
                pattern, params = found
                entry = self.urls.setdefault(pattern, {"sources": set(), "families": set(), "query": set()})
                entry["sources"].add(source)
                entry["families"].add(fam)
                entry["query"].update(params)
                added += 1
        return added

    def key_value(self, module: Module, start: int, key: str):
        """The value of the scanned object at start's key, in the object's scope."""
        return Parser(self.bundle, module, module.objects[start][key], {}, start in module.local).value()

    def report(self, source: str, module: Module, start: int, reason: str) -> None:
        end = min(_expression_end(module.src, start), start + 80)
        self.unresolved.append(
            {"source": source, "family": family(module.name), "expression": module.src[start:end], "missing": reason}
        )

    def builders(self) -> None:
        names = sorted(name for name in self.bundle.chunks if family(name) == URLS_CHUNK)
        if len(names) != 1:
            raise BundleError(
                "Render dashboard URL chunk not found",
                f"want one {URLS_CHUNK}-*.js chunk in the import graph, found {len(names) or 'none'}"
                + (f" ({', '.join(names)})" if names else "")
                + "; the URL builders moved, so update this parser",
            )
        module = self.bundle.module(names[0])
        if not module.exports:
            raise BundleError(
                "Render dashboard URL chunk unreadable",
                f"{URLS_CHUNK} has no export{{…}} clause this parser can read; update UrlInventory.builders",
            )
        built = 0
        for local in sorted(set(module.exports.values())):
            if local not in module.decls:
                continue
            try:
                value = self.bundle.binding(module, local)
                if isinstance(value, Fn):
                    # A helper with statements (a URL check, a hostname
                    # parser) returns a run-time value, which is no path.
                    value = self.bundle.call(value, [RUNTIME] * len(value.params))
            except Unresolved as err:
                self.report("builder", module, module.decls[local], str(err))
                continue
            built += self.add(value, "builder", URLS_CHUNK)
        if not built:
            raise BundleError(
                "Render dashboard URL builders not found",
                f"no export of {names[0]} evaluates to a dashboard path; the builders' shape changed, so update Parser",
            )

    def routes(self, module: Module) -> int:
        objects = module.objects
        routes = {
            start
            for start, keys in objects.items()
            if ("path" in keys and keys.keys() & ROUTE_KEYS)
            or ("children" in keys and keys.keys() & {"Component", "element"})
            or "routes" in keys
        }
        children: dict[int, list[int]] = {}
        for start in routes:
            keys = objects[start]
            key = "children" if "children" in keys else "routes" if "routes" in keys else None
            if key:
                # A pathless object with children and an element may be a JSX
                # props object, so only a route with a path or an app table
                # reports children it cannot read.
                strict = "path" in keys or key == "routes"
                children[start] = [kid for kid in self.route_array(module, keys[key], strict) if kid in routes]
        claimed = {kid for kids in children.values() for kid in kids}
        found = 0

        def walk(start: int, prefixes: tuple, seen: frozenset, root: bool) -> None:
            nonlocal found
            keys = objects[start]
            if "path" in keys:
                try:
                    values = self.key_value(module, start, "path")
                    if not isinstance(values, tuple):
                        raise Unresolved("a path that is not a string")
                except Unresolved as err:
                    self.report("route", module, keys["path"], str(err))
                    return
                if _runtime(values):
                    self.report("route", module, keys["path"], "a path known only at run time")
                    return
                if root and any(not v.startswith("/") for v in values):
                    self.report("route", module, keys["path"], "a relative path with no parent route")
                    return
                prefixes = tuple(dict.fromkeys(_join(p, v) for p in prefixes for v in values))
                found += self.add(prefixes, "route", family(module.name))
            elif root:
                prefixes = ("/",)
            for kid in children.get(start, ()):
                if kid not in seen:
                    walk(kid, prefixes, seen | {kid}, False)

        for start in sorted(routes - claimed):
            walk(start, ("",), frozenset({start}), True)
        return found

    def route_array(self, module: Module, start: int, strict: bool, depth: int = 0) -> list[int]:
        """The object starts a `children:` or `routes:` value lists: an array
        literal, a name bound to one, or `table.routes`, with spreads."""

        def problem(reason: str) -> list[int]:
            if strict:
                self.report("route", module, start, reason)
            return []

        src = module.src
        token = next(lex(src, start), None)
        if token is None or depth > 8:
            return problem("route children nested too deep")
        if token.text == "[":
            kids: list[int] = []
            for element in _elements(src, token.start):
                if src.startswith("{", element):
                    kids.append(element)
                    continue
                at = element + 3 if src.startswith("...", element) else element
                kids += self.route_array(module, at, strict, depth + 1)
            return kids
        if token.kind != "id":
            return problem("route children built by an expression")
        after = next(lex(src, token.end), None)
        if after is not None and after.text == "(":
            return problem(f"route objects built by a call of {token.text}")
        member = next(lex(src, after.end), None) if after is not None and after.text == "." else None
        try:
            owner, at = self.bundle.locate(module, token.text)
        except Unresolved as err:
            return problem(str(err))
        if owner is not module:
            return problem(f"a route table imported from {family(owner.name)}")
        if member is not None:
            table = next(lex(src, at), None)
            keys = module.objects.get(table.start, {}) if table is not None else {}
            if member.text not in keys:
                return problem(f".{member.text} of something that is not a route table")
            at = keys[member.text]
        return self.route_array(module, at, strict, depth + 1)

    def links(self, module: Module) -> None:
        """The `path` of an object that is not a route: a create-menu or a
        sidebar entry ({title:`Web Service`, path:na(al.web)}). The key also
        holds form-field paths, log filters and locals, so only a value that
        evaluates to an absolute dashboard path counts, and nothing else is
        reported."""
        for start, keys in module.objects.items():
            if "path" not in keys or keys.keys() & ROUTE_KEYS:
                continue
            try:
                value = self.key_value(module, start, "path")
            except Unresolved:
                continue
            if isinstance(value, tuple):
                self.add(tuple(alt for alt in value if not alt.startswith(PARAM)), "link", family(module.name))

    def navigation(self, module: Module) -> None:
        """A side-navigation item's hrefPatterns: the paths that select it."""
        for start, keys in module.objects.items():
            if "hrefPatterns" not in keys:
                continue
            at = keys["hrefPatterns"]
            try:
                value = self.key_value(module, start, "hrefPatterns")
            except Unresolved as err:
                self.report("nav", module, at, str(err))
                continue
            items = value.items if isinstance(value, Arr) else [(module, at, value)]
            for owner, element, item in items:
                if isinstance(item, Unresolved):
                    self.report("nav", owner, element, str(item))
                elif not isinstance(item, tuple):
                    self.report("nav", owner, element, "a pattern that is not a string")
                elif any(alt.startswith(PARAM) for alt in item):
                    self.report("nav", owner, element, "a pattern that starts with a run-time value")
                else:
                    self.add(item, "nav", family(owner.name))


def dashboard_urls(chunks: dict[str, str]) -> tuple[list[dict], list[dict]]:
    """Every dashboard path pattern the bundle builds or routes, and what it
    could not read."""
    inventory = UrlInventory(chunks)
    inventory.builders()
    routed = 0
    for name, text in sorted(chunks.items()):
        if "path:" in text or "hrefPatterns:" in text:
            module = inventory.bundle.module(name)
            routed += inventory.routes(module)
            inventory.links(module)
            inventory.navigation(module)
    if not routed:
        raise BundleError(
            "Render dashboard routes not found",
            f"found no route object ({{path:…, Component:…}}) in {len(chunks)} chunks; the router changed, so update "
            "UrlInventory.routes",
        )
    urls = [
        {
            "pattern": pattern,
            "sources": sorted(entry["sources"]),
            "families": sorted(entry["families"]),
            "query": sorted(entry["query"]),
        }
        for pattern, entry in sorted(inventory.urls.items())
    ]
    return urls, inventory.unresolved


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
    urls, unresolved_urls = dashboard_urls(chunks)
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
        "urls": urls,
        "unresolved_urls": unresolved_urls,
    }


def _table_cells(line: str) -> list[str] | None:
    """A markdown table row's cells, or None for its separator row."""
    if re.fullmatch(r"\|[\s:|-]+\|", line):
        return None
    return [cell.strip() for cell in re.split(r"(?<!\\)\|", line.strip("|"))]


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
            cells = _table_cells(line)
            if cells is None:
                continue
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


URL_HEADER = ["Render URL", "bex", "Disposition", "Pointer"]
NO_ROUTE = ("", "—", "-")


def read_url_ledger(path: str) -> tuple[dict, list[str]]:
    """Rows of the URL table, `| Render URL | bex | Disposition | Pointer |`,
    keyed by pattern, plus problems. A table is the URL table when its header
    is exactly that; the file's other tables are prose and are not read (a
    URL table whose header drifts is not read either, so the bundle's URLs
    fail as rowless)."""
    rows: dict[str, tuple[int, str, str, str]] = {}
    problems: list[str] = []
    table = None
    with open(path, encoding="utf-8") as handle:
        for number, line in enumerate(handle, 1):
            line = line.strip()
            if not line.startswith("|"):
                table = None
                continue
            cells = _table_cells(line)
            if cells is None:
                continue
            if table is None:  # a table's first row is its header
                table = "url" if cells == URL_HEADER else "other"
                continue
            if table != "url":
                continue
            where = f"{path}:{number}"
            if len(cells) != 4:
                problems.append(f"{where}: {len(cells)} cells, want 4")
                continue
            key, bex, disposition, pointer = cells
            pattern = key[1:-1] if len(key) > 2 and key[0] == key[-1] == "`" else ""
            if not pattern or normalize_url(pattern) != (pattern, []):
                problems.append(f"{where}: {key} is not a `/path` pattern as `list` prints it (`:param`, no query)")
                continue
            if pattern in rows:
                problems.append(f"{where}: second row for URL {pattern} (first at line {rows[pattern][0]})")
                continue
            rows[pattern] = (number, bex, disposition, pointer)
    return rows, problems


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


def check(page_url: str, ledger: str, routes_ledger: str) -> int:
    found = inventory(page_url)
    operations, flags, malformed = read_ledger(ledger)
    if not operations or not flags:
        raise BundleError("Render dashboard ledger unreadable", f"found no Operation or no Flag rows in {ledger}")
    urls, url_problems = read_url_ledger(routes_ledger)
    if not urls and not url_problems:
        raise BundleError("Render dashboard URL ledger unreadable", f"found no Render URL rows in {routes_ledger}")
    malformed += url_problems
    for pattern, (number, bex, disposition, pointer) in urls.items():
        problem = check_row(f"{routes_ledger}:{number}", disposition, pointer, flags)
        if not problem and disposition in ("matched", "partial") and bex in NO_ROUTE:
            problem = f"{routes_ledger}:{number}: {disposition} row for URL {pattern} names no bex route"
        if problem:
            malformed.append(problem)
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
    bundle_urls = {url["pattern"]: url for url in found["urls"]}
    missing += [
        f"no row: URL {pattern} (from {', '.join(url['sources'])} in {', '.join(url['families'])}"
        + (f"; query {', '.join(url['query'])}" if url["query"] else "")
        + ")"
        for pattern, url in bundle_urls.items()
        if pattern not in urls
    ]
    gone = [
        f"{ledger}:{number}: {key} is no longer in Render's dashboard bundle (pointer: {pointer})"
        for key, (number, _, pointer) in operations.items()
        if key not in in_bundle
    ] + [
        f"{routes_ledger}:{number}: URL {pattern} is no longer in Render's dashboard bundle (pointer: {pointer})"
        for pattern, (number, _, _, pointer) in urls.items()
        if pattern not in bundle_urls
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

    for item in found["unresolved_urls"]:
        print(
            f"::notice title=Render dashboard URL not read::{item['source']} in {item['family']}: "
            f"{item['expression']} ({item['missing']}); `list` prints every one"
        )

    summary = (
        f"{len(found['operations'])} operations, {len(found['flags'])} flags and {len(found['urls'])} URLs "
        f"in {found['chunks']} chunks (entry {found['entry']})"
    )
    if not (missing or gone or malformed):
        print(f"Render dashboard: every operation, flag and URL has a row: {summary}")
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
Action: give each new operation or flag one row in {ledger},
and each new URL one row in {routes_ledger}'s URL table:
  matched   bex ships it: the ADR018 row, ADR, artifact or code that says so
  partial   bex ships part: the board item owning the rest, or the ADR018 divergence
  gap       bex lacks it: the board item that owns it (file one with /pm add)
  non-goal  declined: the DO_NOT_DO.md line or the ADR018 "—" row
  n/a       nothing to match (Render-internal, staff-only), and why
  flagged   gated per workspace: the flag from the Flag table
Delete the row of an operation or URL that is gone. A flag gate that is gone
means Render made the feature GA or dropped it: reopen what its row names
first. A URL's matched or partial row also names the bex route in its bex cell.
`scripts/render-dashboard-ops.sh list` prints each operation's root fields and
where each URL comes from.
This job never writes the ledger.""",
        file=sys.stderr,
    )
    return 1


def main(argv: list[str]) -> int:
    if len(argv) == 2 and argv[0] == "list":
        print(json.dumps(inventory(argv[1]), indent=1))
        return 0
    if len(argv) == 4 and argv[0] == "check":
        return check(argv[1], argv[2], argv[3])
    print(__doc__.split("\n\n")[2], file=sys.stderr)
    return 2


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except BundleError as err:
        print(f"::error title={err.title}::{err}", file=sys.stderr)
        sys.exit(1)
