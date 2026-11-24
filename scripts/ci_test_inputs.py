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
fails too, and so does one the paths already cover, so the list only holds
live exceptions. An event without `paths` runs on every change and an absent
one never runs the suite, so only a filtered event is checked (references.yml
filters neither; s3-contract.yml has no pull_request). Directory reads and
paths composed at run time are not resolved: list those by hand (deploy.yml's
full run is the backstop).

A suite whose tests are scripts (gitops.yml: gitops-validate.sh and the
guards it runs) is scanned as shell from its workflow: every word naming a
tracked file is a read, quoted or not, and a script a scanned file runs
(`bash x.sh`, `. lib.sh`, `python3 -m unittest t.py`, `"$here/x.sh"`, or a
variable assigned that path) is scanned in turn. A path inside single quotes
is a needle, read but never run, `bash -n` only reads, and an ignored path is
never run either. The workflow's `on:` block is no read, a folded `run: >-`
block is one command line, and a `**` word is a filter pattern no script
expands. gitops.yml missed some forty files that gitops-validate.sh and its
guards read or grep (verify-static-site-security.sh among them) while the
checker scanned only Go and TypeScript tests; the six other script-running
workflows were not scanned at all.

A path under `.pm/` is no read for a suite whose checkout leaves the board out
(w6/m157; all but the `board` suites, docs.yml and references.yml, which read
it whole), so a test that names a board file names a fixture, and board items
move between `blocked/`, the queue and `done/`: a move that made a fixture's
sample path a tracked file turned the operator suite red
(`.pm/w8/m91/README.md`, 2026-11-16) on a push no gate checked.

Every suite with tests or scripts to scan runs `python3 -B
scripts/ci_test_inputs.py <its workflow>` as a step, so a push that drops a
read from its paths fails that suite's own run as well as scripts.yml's
real-tree test and /ship's gate (the seven script-running suites had no such
step, so a push that bypassed /ship drifted their lists silently). This file
is no suite's read: a change to it runs scripts.yml, whose real-tree test
checks every suite.

  python3 scripts/ci_test_inputs.py [--repo DIR] [WORKFLOW ...]   # default: every suite
  python3 scripts/ci_test_inputs.py --affected < changed-paths    # which could open a gap
  python3 scripts/ci_test_inputs.py --triggered WORKFLOW < changed-paths  # which run it on a push
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
    # The tests are the scripts the workflow's steps run: the workflow file
    # and every script it, or a script it runs, executes are scanned as shell.
    runs: bool = False
    # The workflow checks the board out, so a board path its tests name is a
    # read like any other (docs.yml and references.yml read it whole).
    board: bool = False

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
        "lego/backend/internal/apps/service.go": "a sample path for the native push-path classifier",
        ".github/scripts/native-oci-mirror.py": "a sample path for the native push-path classifier",
    }),
    ".github/workflows/dashboard-test.yml": Suite(("dashboard",), (".test.ts", ".test.tsx", ".test.mjs", ".test.js"), {
        "lego/types/tiers/tiers.go": "a tool-call input inside an agent transcript fixture",
    }),
    # Tests the pinned upstream controller with the carried patches; no test
    # source of its own to scan.
    ".github/workflows/opensandbox-controller-test.yml": Suite(("deploy/opensandbox/patches",)),
    ".github/workflows/gitops.yml": Suite(("deploy/gitops", "lego/operator/config"), ("_test.go",), {
        "README.md": "a grep --exclude pattern, not the root README",
        "docs/ADR010-observability.md": "named in a warning when promtool is missing",
        "docs/ADR011-etcd-backup-restore.md": "named in a failure message about the etcd image",
        "docs/ADR079-security-review-round18.md": "named in env-file.sh's permission error",
    }, runs=True),
    ".github/workflows/scripts.yml": Suite(("scripts",), (), {
        ".prettierignore": "read only by docs-links-validate.sh, which test_loopx_guard.py names in a fixture",
        "README.md": "a grep --exclude pattern and fixture file names, not the root README",
        "LICENSE": "a file name inside test_install_trivy.py's fake release archive",
        "dashboard/package.json": "written into the self-tests' throwaway repositories",
        "dashboard/src/routes/index.tsx": "a file deploy-superseded.test.sh commits in its throwaway repository",
        "docs/ADR013-secrets.md": "named in bao-init.sh's failure message",
        "docs/ADR018/surface-pins-and-corrections.md": "named in render-schema-drift.sh's drift report",
        "docs/ADR019-infra-credentials.md": "named in the kubeconfig and public-log helpers' messages",
        "docs/ADR049-render-yaml-parity.md": "named in render-schema-drift.sh's drift report",
        "docs/ADR060-build-worker-reliability-and-performance.md": "named in the toolchain freshness report",
        "docs/ADR079-security-review-round18.md": "named in env-file.sh's permission error",
        "docs/cli-compatibility-checklist.md": "named in render-schema-drift.sh's drift report",
        "docs/runbooks/main-history-rewrite.md": "named in the repository security drift report",
    }, runs=True),
    ".github/workflows/clusterapi-validate.yml": Suite(
        ("infra/clusterapi/overlays/hetzner-caph", "infra/packer"), (), {}, runs=True),
    ".github/workflows/docs.yml": Suite((), (), {}, runs=True, board=True),
    ".github/workflows/cli-test.yml": Suite(
        ("lego/cli", "lego/backend/internal/cliauth", "lego/backend/internal/api"), ("_test.go",), {}, runs=True),
    ".github/workflows/references.yml": Suite(("docs", ".pm"), (), {}, runs=True, board=True),
    ".github/workflows/s3-contract.yml": Suite(
        ("lego/backend/internal/s3gw", "lego/backend/internal/objectstorage", "lego/cli/internal/s3"), (), {},
        runs=True),
}

_STRING = re.compile(r'"((?:[^"\\\n]|\\.)*)"|\'((?:[^\'\\\n]|\\.)*)\'|`([^`]*)`')
_JOIN = re.compile(r"\b(?:filepath\.Join|path\.Join|path\.join|path\.resolve|join|resolve)\(")
_PLACEHOLDER = re.compile(r"\$\{[^}]*\}")
_COMMENT = re.compile(r"^\s*(//|\*|/\*)")
# Shell (and the YAML and Python a workflow runs): a path is any word, quoted
# or not, and a `$var`, `${…}` or `$(…)` before a `/` is a hole.
_SHELL_HOLE = re.compile(r"(?:\$\{[^}\n]*\}|\$\((?:[^()\n]|\([^()\n]*\))*\)|\$\w+)(?=/)")
_SHELL_COMMENT = re.compile(r"(?:^|\s)#(?:\s.*)?$")
_SHELL_WORD = re.compile(r"(?<![\w.@*/-])(?:\0/)?[\w.@*-]+(?:/[\w.@*-]+)*")
# `name=path`, and a later `$name` or `${name}` that runs it.
_SHELL_ASSIGN = re.compile(r"^\s*(?:(?:local|readonly|export)\s+)?(\w+)=[\"']?((?:\0/)?[\w.@-]+(?:/[\w.@-]+)*)[\"']?\s*$")
_SHELL_VAR = re.compile(r"\$\{?(\w+)\}?")
# What runs the word after it: an interpreter (not `-n`, a syntax check, nor
# `-c`, whose program is the word), every file `unittest` is given, or the
# script itself in command position. A word inside single quotes is a needle
# or a program's text, never run.
_SHELL_RUN = re.compile(r"(?:^|[\s;&|(!{`\"'])(?:(?:bash|sh|source|\.|python3?|exec)(?P<flags>(?:\s+-[\w-]+)*)"
                        r"|python3?(?:\s+-\w+)*\s+-m\s+unittest\b[^;&|]*)\s+[\"']?\Z"
                        r"|(?:^|[;&|!`]|\$\(|\bthen|\bdo)\s*[\"']?\Z")
_SCRIPT = (".sh", ".bash", ".py")
_FOLDED = re.compile(r":\s*>[-+]?\s*$")  # a YAML folded scalar starts
# This file is no suite's read, and neither are its words. Each suite's own
# run executes it, but a change to it runs scripts.yml, whose real-tree test
# checks every suite, so no suite lists it; and its table names other suites'
# paths, which that run checks against their own lists.
CHECKER = "scripts/ci_test_inputs.py"
HOLE = "\0"  # a ${…} placeholder: REPO_ROOT, process.cwd(), …
BOARD = ".pm/"  # left out of a suite's checkout unless Suite.board says otherwise
TRIGGERS = ("push", "pull_request")  # the events a change runs


def path_filters(text: str) -> dict[str, list[str] | None]:
    """Each of push and pull_request under `on:`: its `paths` list, or None
    when it has none and so runs on every change."""
    filters: dict[str, list[str] | None] = {}
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
            if event in TRIGGERS:
                filters[event] = None
        elif in_on and indent == 4 and event in TRIGGERS:
            if stripped.startswith("paths-ignore:"):
                raise ValueError(f"unsupported filter paths-ignore under {event}: list the paths it runs on")
            in_paths = stripped == "paths:"
            if in_paths:
                filters[event] = []
        elif in_on and in_paths and stripped.startswith("- "):
            filters[event].append(stripped[2:].split(" #", 1)[0].strip().strip("\"'"))
    return filters


def _jobs(text: str) -> str:
    """A workflow without its `on:` block, whose trigger filter names paths
    that start the suite, not files it reads; a folded `run: >-` block is one
    command line, as the runner gets it."""
    out, in_on, fold = [], False, None
    for line in text.splitlines():
        if line and not line[0].isspace() and not line.startswith("#"):
            in_on = line.rstrip() == "on:"
        indent = len(line) - len(line.lstrip())
        if in_on:
            line = ""
        elif fold is not None and line.strip() and indent > fold:
            out[-1] += " " + line.strip()
            continue
        fold = indent if _FOLDED.search(line) else None
        out.append(line)
    return "\n".join(out)


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


def _single_quoted(line: str) -> list[bool]:
    """Whether each character of a shell line is inside single quotes."""
    mask, quote, escaped = [], "", False
    for ch in line:
        mask.append(quote == "'")
        if escaped:
            escaped = False
        elif ch == "\\" and quote != "'":
            escaped = True
        elif ch in "'\"" and quote in ("", ch):
            quote = "" if quote else ch
    return mask


def shell_words(text: str) -> list[tuple[str, bool]]:
    """Each path-like word of a shell, workflow or Python source (a leading
    hole as HOLE), and whether the source runs it."""
    words, assigned = [], {}

    def runs(line: str, quoted: list[bool], start: int) -> bool:
        run = None if quoted[start] else _SHELL_RUN.search(line[:start])
        return run is not None and not {"-n", "-c"} & set((run.group("flags") or "").split())

    for line in text.replace("\\\n", " ").splitlines():
        line = _SHELL_COMMENT.sub("", _SHELL_HOLE.sub(HOLE, line))
        quoted = _single_quoted(line)
        if assignment := _SHELL_ASSIGN.match(line):
            assigned[assignment[1]] = assignment[2]
        for match in _SHELL_WORD.finditer(line):
            words.append((match.group(), runs(line, quoted, match.start())))
        for match in _SHELL_VAR.finditer(line):
            if match[1] in assigned and runs(line, quoted, match.start()):
                words.append((assigned[match[1]], True))
    return words


def shell_paths(value: str, source: str, tracked: set[str]) -> list[str]:
    """The tracked files a shell word names: relative to the repository root,
    where the steps run; after a hole also to the source's directory; a glob
    as every file it matches."""
    directory = os.path.dirname(source)
    if value.startswith(HOLE):
        rest = value[1:].lstrip("/")
        bases = ("", directory) if "/" in rest else (directory,)
    else:
        rest, bases = value, ("",)
    for base in bases:
        candidate = os.path.normpath(os.path.join(base, rest))
        if candidate.startswith(".."):
            continue
        if "*" in candidate:
            # Only a glob under a literal directory of the repository: `*`,
            # `*/*` or `$tmp/*.json` is a case arm, a regex, SQL or scratch.
            # `**` is a trigger filter or a checker's pattern (`dashboard/**`
            # in this file), never a glob a script expands: none sets globstar.
            if value.startswith(HOLE) or "*" in candidate.partition("/")[0] or "**" in candidate:
                continue
            pattern = _glob(candidate)
            matches = sorted(f for f in tracked if pattern.match(f))
            if matches:
                return matches
        elif candidate in tracked:
            return [candidate]
    return []


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
            if path and not suite.inside(path) and (suite.board or not path.startswith(BOARD)):
                found.setdefault(path, set()).add(source)
    return found


def run_references(repo: pathlib.Path, workflow: str, suite: Suite,
                   tracked: set[str]) -> tuple[dict[str, set[str]], set[str]]:
    """Out-of-tree files the workflow and the scripts it runs name, and those
    scripts: the workflow, and each script a scanned one runs, transitively.
    An ignored path is named, never read, so never run either; CHECKER is
    neither."""
    found: dict[str, set[str]] = {}
    sources, queue = {workflow}, [workflow]
    while queue:
        source = queue.pop()
        text = (repo / source).read_text(encoding="utf-8", errors="replace")
        for value, runs in shell_words(_jobs(text) if source == workflow else text):
            for path in shell_paths(value, source, tracked):
                if path == CHECKER or (path.startswith(BOARD) and not suite.board):
                    continue
                if runs and path.endswith(_SCRIPT) and path not in sources and path not in suite.ignored:
                    sources.add(path)
                    queue.append(path)
                if not suite.inside(path):
                    found.setdefault(path, set()).add(source)
    return found, sources


def reads(repo: pathlib.Path, workflow: str, suite: Suite, tracked: set[str]) -> dict[str, set[str]]:
    """Every out-of-tree file the suite reads: {path: {source, …}}."""
    found = references(repo, suite, tracked)
    if suite.runs:
        for path, sources in run_references(repo, workflow, suite, tracked)[0].items():
            found.setdefault(path, set()).update(sources)
    return found


def check(repo: pathlib.Path, workflow: str, tracked: set[str]) -> list[str]:
    suite = SUITES[workflow]
    filters = path_filters((repo / workflow).read_text())
    if not filters:
        return [f"{workflow}: runs on neither push nor pull_request"]
    # An event without `paths` runs on every change (references.yml's push),
    # and an absent one never runs the suite (s3-contract.yml has no
    # pull_request), so only a filtered event can miss a read.
    lists = [paths for paths in filters.values() if paths is not None]
    problems = []
    if len(lists) == 2 and lists[0] != lists[1]:
        problems.append(f"{workflow}: push and pull_request paths differ; keep one list in both")
    found = reads(repo, workflow, suite, tracked)
    if lists:
        patterns = lists[0]
        for root in suite.roots:
            sample = next((f for f in sorted(tracked) if f.startswith(root + "/")), root + "/x")
            if not covered(sample, patterns):
                problems.append(f"{workflow}: paths miss the suite's own tree {root}/**")
        if not covered(workflow, patterns):
            problems.append(f"{workflow}: paths miss the workflow file itself")
        for path in sorted(found):
            if path in suite.ignored:
                continue
            if not covered(path, patterns):
                sources = sorted(found[path])
                more = f" (+{len(sources) - 1} more)" if len(sources) > 1 else ""
                problems.append(f"{workflow}: paths miss {path}, read by {sources[0]}{more}")
    for path in suite.ignored:
        if path not in found:
            problems.append(f"{workflow}: ignored lists {path}, which no test names any more; remove it")
        elif lists and covered(path, lists[0]):
            problems.append(f"{workflow}: ignored lists {path}, which the paths already cover; remove it")
    return problems


def affected(paths: list[str], scripts: set[str] | frozenset[str] = frozenset()) -> list[str]:
    """The changed paths that could open a gap: test sources, the scripts a
    suite's steps run, suite workflows, this checker."""
    return [p for p in paths if p in SUITES or p in scripts or p == CHECKER
            or any(suite.is_test(p) for suite in SUITES.values())]


def triggered(paths: list[str], filters: dict[str, list[str] | None]) -> list[str]:
    """The changed paths that run the workflow on a push: every one when push
    has no `paths`, none when the workflow is not run by a push."""
    if "push" not in filters:
        return []
    patterns = filters["push"]
    return [p for p in paths if patterns is None or covered(p, patterns)]


def _tracked(repo: pathlib.Path) -> set[str]:
    return set(subprocess.run(["git", "-C", str(repo), "ls-files", "-z"], capture_output=True,
                              text=True, check=True).stdout.split("\0")) - {""}


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--repo", default=".")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--affected", action="store_true", help="print the stdin paths that could open a gap")
    mode.add_argument("--suites", action="store_true", help="print the suite workflow file names")
    mode.add_argument("--triggered", action="store_true",
                      help="print the stdin paths that run the one WORKFLOW given on a push")
    parser.add_argument("workflows", nargs="*", default=sorted(SUITES))
    args = parser.parse_args(argv)
    repo = pathlib.Path(args.repo)
    if args.affected:
        try:
            tracked = _tracked(repo)
            scripts = {path for workflow, suite in SUITES.items() if suite.runs and workflow in tracked
                       for path in run_references(repo, workflow, suite, tracked)[1]}
        except (OSError, ValueError, subprocess.CalledProcessError) as exc:
            print(f"ci-test-inputs: unusable: {exc}", file=sys.stderr)
            return 2
        print("\n".join(affected(sys.stdin.read().split(), scripts)))
        return 0
    if args.suites:
        print("\n".join(os.path.basename(w) for w in sorted(SUITES)))
        return 0
    if args.triggered:
        if len(args.workflows) != 1 or args.workflows[0] not in SUITES:
            print(f"ci-test-inputs: --triggered takes one suite workflow; known: {', '.join(sorted(SUITES))}",
                  file=sys.stderr)
            return 2
        try:
            filters = path_filters((repo / args.workflows[0]).read_text())
        except (OSError, ValueError) as exc:
            print(f"ci-test-inputs: unusable: {exc}", file=sys.stderr)
            return 2
        print("\n".join(triggered(sys.stdin.read().split(), filters)))
        return 0
    unknown = [w for w in args.workflows if w not in SUITES]
    if unknown:
        print(f"ci-test-inputs: unknown suite workflow {unknown[0]}; known: {', '.join(sorted(SUITES))}", file=sys.stderr)
        return 2
    try:
        tracked = _tracked(repo)
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
