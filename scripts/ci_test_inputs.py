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
"""ci_test_inputs.py — a change to any file a suite's tests read runs that suite.

A test workflow runs on a push or pull request only when the change touches
its `paths`, but its tests also read files outside its own tree: the
dashboard's drift guards read lego/backend sources and testdata, the backend
reads deploy/gitops manifests, docs and CLI fixtures, the operator reads
deploy/ manifests and native-image inputs. A path missing from the list means
a change that breaks the suite never runs it, until the next batched deploy
runs every suite (dashboard-test.yml listed only dashboard/** while about
twenty of its tests read backend files).

This finds the references in each suite's test sources that name a tracked
file outside the suite's tree and fails when the workflow's push or
pull_request paths miss one, or when the two lists differ. A reference to a
file the test never reads (a path inside a fixture) goes in the suite's
`ignored` map with the reason; an ignored entry nothing references any more
fails too, so the list only holds live exceptions. Directory reads and paths
composed at run time are not resolved: list those by hand (deploy.yml's full
run is the backstop).

  python3 scripts/ci_test_inputs.py [--repo DIR] [WORKFLOW ...]   # default: every suite
  python3 scripts/ci_test_inputs.py --affected < changed-paths    # which could open a gap
  python3 scripts/ci_test_inputs.py --suites                      # the suite workflows

Exit 0 covered, 1 a gap (each printed), 2 unusable.
"""

from __future__ import annotations

import argparse
import dataclasses
import os
import pathlib
import re
import subprocess
import sys


@dataclasses.dataclass(frozen=True)
class Suite:
    roots: tuple[str, ...]  # the suite's own trees
    tests: tuple[str, ...] = ()  # suffixes of the test sources to scan
    # Out-of-tree paths a test names but never reads: path -> why.
    ignored: dict[str, str] = dataclasses.field(default_factory=dict)

    def inside(self, path: str) -> bool:
        return any(path == root or path.startswith(root + "/") for root in self.roots)

    def is_test(self, path: str) -> bool:
        return bool(self.tests) and path.endswith(self.tests) and self.inside(path)


SUITES = {
    ".github/workflows/backend-test.yml": Suite(
        ("lego/backend", "lego/types", "lego/agent-image"), ("_test.go", ".test.ts", ".test.mjs"), {
            "examples/cron-demo/render.yaml": "a Blueprint path inside an in-memory fixture repository",
            "examples/static-site/render.yaml": "a Blueprint path inside an in-memory fixture repository",
        }),
    ".github/workflows/operator-test.yml": Suite(("lego/operator", "lego/types"), ("_test.go",), {
        "lego/backend/internal/apps/service.go": "a sample path for the native custody path classifier",
        ".github/scripts/native-oci-mirror.py": "a sample path for the native custody path classifier",
    }),
    ".github/workflows/dashboard-test.yml": Suite(("dashboard",), (".test.ts", ".test.tsx", ".test.mjs", ".test.js"), {
        "lego/types/tiers/tiers.go": "a tool-call input inside an agent transcript fixture",
    }),
    # Tests the pinned upstream controller with the carried patches; no test
    # source of its own to scan.
    ".github/workflows/opensandbox-controller-test.yml": Suite(("deploy/opensandbox/patches",)),
}

_STRING = re.compile(r'"((?:[^"\\\n]|\\.)*)"|\'((?:[^\'\\\n]|\\.)*)\'|`([^`]*)`')
_JOIN = re.compile(r"\b(?:filepath\.Join|path\.Join|path\.join|path\.resolve|join|resolve)\(")
_PLACEHOLDER = re.compile(r"\$\{[^}]*\}")
_COMMENT = re.compile(r"^\s*(//|\*|/\*)")
HOLE = "\0"  # a ${…} placeholder: REPO_ROOT, process.cwd(), …


def path_filters(text: str) -> dict[str, list[str]]:
    """The `paths` list of each event under `on:` that has one."""
    filters: dict[str, list[str]] = {}
    in_on = in_paths = False
    event = ""
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        indent = len(line) - len(line.lstrip())
        if indent == 0:
            in_on = stripped == "on:"
        elif in_on and indent == 2:
            event, in_paths = stripped.split(":", 1)[0], False
        elif in_on and indent == 4:
            in_paths = stripped == "paths:"
            if in_paths:
                filters[event] = []
        elif in_on and in_paths and stripped.startswith("- "):
            filters[event].append(stripped[2:].split(" #", 1)[0].strip().strip("\"'"))
    return filters


def _glob(pattern: str) -> re.Pattern[str]:
    """GitHub's filter pattern: `*` stops at `/`, `**` does not."""
    if "?" in pattern or "+" in pattern:
        raise ValueError(f"unsupported filter pattern {pattern!r}: `?` and `+` quantify the preceding character")
    out, i = [], 0
    while i < len(pattern):
        if pattern.startswith("**/", i):
            out.append("(?:.*/)?")
            i += 3
        elif pattern.startswith("**", i):
            out.append(".*")
            i += 2
        elif pattern[i] == "*":
            out.append("[^/]*")
            i += 1
        elif pattern[i] == "[":
            end = pattern.index("]", i)
            out.append(pattern[i:end + 1])
            i = end + 1
        else:
            out.append(re.escape(pattern[i]))
            i += 1
    return re.compile("".join(out) + r"\Z")


def covered(path: str, patterns: list[str]) -> bool:
    """Whether a change to path matches the filter; a later `!` pattern excludes."""
    hit = False
    for pattern in patterns:
        negated = pattern.startswith("!")
        if _glob(pattern[1:] if negated else pattern).match(path):
            hit = not negated
    return hit


def _join_args(text: str, start: int) -> list[str] | None:
    """The top-level arguments of the call whose `(` is at start - 1."""
    args, depth, quote, current = [], 0, "", []
    i = start
    while i < len(text):
        ch = text[i]
        if quote:
            current.append(ch)
            if ch == "\\" and quote != "`":
                current.append(text[i + 1])
                i += 1
            elif ch == quote:
                quote = ""
        elif ch in "\"'`":
            quote = ch
            current.append(ch)
        elif ch in "([{":
            depth += 1
            current.append(ch)
        elif ch in ")]}":
            if depth == 0:
                args.append("".join(current).strip())
                return args
            depth -= 1
            current.append(ch)
        elif ch == "," and depth == 0:
            args.append("".join(current).strip())
            current = []
        else:
            current.append(ch)
        i += 1
    return None


def _unquote(match: re.Match[str]) -> str:
    return next(g for g in match.groups() if g is not None)


def literals(text: str) -> list[str]:
    """String literals, and the literal segments of a path join as one path."""
    text = "\n".join("" if _COMMENT.match(line) else line for line in text.splitlines())
    found = [_unquote(m) for m in _STRING.finditer(text)]
    for match in _JOIN.finditer(text):
        args = _join_args(text, match.end())
        if not args:
            continue
        values = [None if (m := _STRING.fullmatch(arg)) is None else _unquote(m) for arg in args]
        # Leading variables (a root directory) are dropped; a variable after
        # the first literal makes the path unknowable.
        while values and values[0] is None:
            values.pop(0)
        if len(values) >= 2 and None not in values:
            found.append("/".join(values))
    return [_PLACEHOLDER.sub(HOLE, value) for value in found]


def _package_root(source: str, tracked: set[str]) -> str:
    """Where the test runs: the Go package directory, or the nearest package.json."""
    directory = os.path.dirname(source)
    if source.endswith(".go"):
        return directory
    probe = directory
    while probe:
        if f"{probe}/package.json" in tracked:
            return probe
        probe = os.path.dirname(probe)
    return directory


def resolve(value: str, source: str, root: str, tracked: set[str]) -> str | None:
    """The tracked file (or explicit relative glob) a literal in source names,
    if any; root is where the test runs."""
    if "/" not in value.strip(HOLE + "/") or any(ch.isspace() for ch in value):
        return None
    directory = os.path.dirname(source)
    relative = value.startswith(("../", "./"))
    if HOLE in value:
        rest = value.rpartition(HOLE)[2].lstrip("/")
        candidates = [os.path.normpath(os.path.join(base, rest)) for base in ("", root, directory)]
    elif relative:
        candidates = [os.path.normpath(os.path.join(base, value)) for base in (directory, root)]
    else:
        candidates = [os.path.normpath(value)]
    for candidate in candidates:
        if candidate.startswith(".."):
            continue
        if candidate in tracked:
            return candidate
        # A glob counts only as an explicit relative path (filepath.Glob's
        # argument); a bare one is a fixture (build filters, ignore rules).
        if relative and any(ch in candidate for ch in "*["):
            pattern = _glob(candidate)
            if any(pattern.match(f) for f in tracked):
                return candidate
    return None


def references(repo: pathlib.Path, suite: Suite, tracked: set[str]) -> dict[str, set[str]]:
    """Out-of-tree files the suite's tests name: {path: {test source, …}}."""
    found: dict[str, set[str]] = {}
    for source in filter(suite.is_test, tracked):
        root = _package_root(source, tracked)
        text = (repo / source).read_text(encoding="utf-8", errors="replace")
        for value in literals(text):
            path = resolve(value, source, root, tracked)
            if path and not suite.inside(path):
                found.setdefault(path, set()).add(source)
    return found


def check(repo: pathlib.Path, workflow: str, tracked: set[str]) -> list[str]:
    suite = SUITES[workflow]
    filters = path_filters((repo / workflow).read_text())
    push, pull = filters.get("push"), filters.get("pull_request")
    problems = []
    if push is None or pull is None:
        return [f"{workflow}: needs `paths` under both push and pull_request"]
    if push != pull:
        problems.append(f"{workflow}: push and pull_request paths differ; keep one list in both")
    for root in suite.roots:
        sample = next((f for f in sorted(tracked) if f.startswith(root + "/")), root + "/x")
        if not covered(sample, push):
            problems.append(f"{workflow}: paths miss the suite's own tree {root}/**")
    if workflow not in push:
        problems.append(f"{workflow}: paths miss the workflow file itself")
    found = references(repo, suite, tracked)
    for path in sorted(found):
        if path in suite.ignored:
            continue
        if not covered(path, push):
            sources = sorted(found[path])
            more = f" (+{len(sources) - 1} more)" if len(sources) > 1 else ""
            problems.append(f"{workflow}: paths miss {path}, read by {sources[0]}{more}")
    for path in suite.ignored:
        if path not in found:
            problems.append(f"{workflow}: ignored lists {path}, which no test names any more; remove it")
    return problems


def affected(paths: list[str]) -> list[str]:
    """The changed paths that could open a gap: test sources, suite workflows, this checker."""
    return [p for p in paths if p in SUITES or p == "scripts/ci_test_inputs.py"
            or any(suite.is_test(p) for suite in SUITES.values())]


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--repo", default=".")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--affected", action="store_true", help="print the stdin paths that could open a gap")
    mode.add_argument("--suites", action="store_true", help="print the suite workflow file names")
    parser.add_argument("workflows", nargs="*", default=sorted(SUITES))
    args = parser.parse_args(argv)
    if args.affected:
        print("\n".join(affected(sys.stdin.read().split())))
        return 0
    if args.suites:
        print("\n".join(os.path.basename(w) for w in sorted(SUITES)))
        return 0
    repo = pathlib.Path(args.repo)
    unknown = [w for w in args.workflows if w not in SUITES]
    if unknown:
        print(f"ci-test-inputs: unknown suite workflow {unknown[0]}; known: {', '.join(sorted(SUITES))}", file=sys.stderr)
        return 2
    try:
        tracked = set(subprocess.run(["git", "-C", str(repo), "ls-files", "-z"], capture_output=True,
                                     text=True, check=True).stdout.split("\0")) - {""}
        problems = [p for w in args.workflows for p in check(repo, w, tracked)]
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"ci-test-inputs: unusable: {exc}", file=sys.stderr)
        return 2
    for problem in problems:
        print(f"FAIL: {problem}", file=sys.stderr)
    if problems:
        print("ci-test-inputs: add each missing path to both the push and pull_request lists, "
              "or to the suite's ignored paths in scripts/ci_test_inputs.py when the test never reads it",
              file=sys.stderr)
        return 1
    print(f"ci-test-inputs: ok ({', '.join(args.workflows)})")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
