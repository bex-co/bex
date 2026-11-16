#!/usr/bin/env python3
"""Refuse a push that adds a command-line flag the pinned image cannot parse.

Argo applies lego/operator/config from main as soon as a push lands, but the
platform images move only when deploy.yml writes a `chore(deploy): pin platform
images to <sha>` commit. A flag added to a container's args together with its
definition therefore runs on the previous binary first, whose flag.Parse()
exits: f114fdd9f's --pprof-bind-address crash-looped the operator manager for
about 35 minutes until the pin landed (w8/331).

For every manifest under lego/operator/config that the push changes, the flags
added to a bex-built container (image `controller…` or `ghcr.io/bex-co/…`) or
by a kustomize patch's args/command op must already be defined as Go flags under
lego/ at the commit the platform is pinned to. Ship the definition first and
the manifest argument once a pin carries it, or pass the setting as an env var,
which an older binary ignores.

    python3 -B scripts/manifest_flag_guard.py <base> [<head>]

Exits 0 when nothing is added or every added flag is defined, 1 naming each
undefined flag, and 0 with a warning when PyYAML or a pin commit is missing, so
the guard never blocks on its own prerequisites.
"""

from __future__ import annotations

import re
import subprocess
import sys

from deploy_latency import PIN_SUBJECT

try:
    import yaml
except ImportError:
    yaml = None

MANIFEST_ROOT = "lego/operator/config/"
FLAG_ARG = re.compile(r"^--([A-Za-z0-9][A-Za-z0-9._-]*)(?:=|$)")
# A Go flag definition: flag.BoolVar(&x, "name", ...), fs.String("name", ...),
# flag.Var(&v, "name", ...), flag.Func("name", ...).
FLAG_DEF = r'\b\w+\.(?:Bool|String|Int|Int64|Uint|Uint64|Float64|Duration|Func|BoolFunc|Var|TextVar)(?:Var)?\(.*"{name}"'
ARGS_PATH = re.compile(r"/(?:c|initC)ontainers/[^/]+/(?:args|command)(?:/|$)")


def git(*args: str) -> str:
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def show(rev: str, path: str) -> str | None:
    result = subprocess.run(["git", "show", f"{rev}:{path}"], capture_output=True, text=True)
    return result.stdout if result.returncode == 0 else None


def ours(image: object) -> bool:
    return isinstance(image, str) and (
        image == "controller" or image.startswith(("controller:", "controller@", "ghcr.io/bex-co/"))
    )


def flags_of(values: object) -> set[str]:
    if isinstance(values, str):
        values = [values]
    if not isinstance(values, list):
        return set()
    names = set()
    for value in values:
        if isinstance(value, str):
            match = FLAG_ARG.match(value.strip())
            if match:
                names.add(match.group(1))
    return names


def walk_containers(node: object):
    """Every container map under a workload document, at any depth."""
    if isinstance(node, dict):
        for key in ("containers", "initContainers"):
            for container in node.get(key) or []:
                if isinstance(container, dict):
                    yield container
        for value in node.values():
            yield from walk_containers(value)
    elif isinstance(node, list):
        for value in node:
            yield from walk_containers(value)


def manifest_flags(text: str | None) -> set[str]:
    """Flags passed to bex-built containers, plus flags kustomize patches add to args."""
    if not text:
        return set()
    try:
        documents = list(yaml.safe_load_all(text))
    except yaml.YAMLError:
        return set()
    names: set[str] = set()
    for document in documents:
        for container in walk_containers(document):
            if ours(container.get("image")):
                names |= flags_of(container.get("args")) | flags_of(container.get("command"))
        if isinstance(document, dict):
            for patch in document.get("patches") or []:
                if isinstance(patch, dict) and isinstance(patch.get("patch"), str):
                    names |= patch_flags(patch["patch"])
    return names


def patch_flags(patch: str) -> set[str]:
    try:
        ops = yaml.safe_load(patch)
    except yaml.YAMLError:
        return set()
    names: set[str] = set()
    if isinstance(ops, list):
        for op in ops:
            if isinstance(op, dict) and ARGS_PATH.search(str(op.get("path", ""))):
                names |= flags_of(op.get("value"))
    else:
        for container in walk_containers(ops):
            names |= flags_of(container.get("args")) | flags_of(container.get("command"))
    return names


def pinned_commit(head: str) -> str | None:
    subject = git("log", "-1", "--format=%s", "--grep=^chore(deploy): pin platform images to ", head).strip()
    match = PIN_SUBJECT.match(subject)
    return match.group(1) if match else None


def defined_at(commit: str, name: str) -> bool:
    # git grep's -E is POSIX (no (?:…)), so find the literal and match in Python.
    result = subprocess.run(
        ["git", "grep", "-h", "-F", f'"{name}"', commit, "--", "lego/*.go"], capture_output=True, text=True
    )
    definition = re.compile(FLAG_DEF.format(name=re.escape(name)))
    return any(definition.search(line) for line in result.stdout.splitlines())


def check(base: str, head: str = "HEAD") -> tuple[str | None, list[tuple[str, str]]]:
    """The pinned commit and each (manifest, flag) the push adds that it does not define."""
    changed = [
        path
        for path in git("diff", "--name-only", f"{base}..{head}", "--", MANIFEST_ROOT).splitlines()
        if path.endswith((".yaml", ".yml"))
    ]
    added = {(path, name) for path in changed for name in manifest_flags(show(head, path)) - manifest_flags(show(base, path))}
    if not added:
        return None, []
    pin = pinned_commit(head)
    if pin is None:
        print("manifest-flag-guard: warning: no pin commit found; skipped", file=sys.stderr)
        return None, []
    return pin, sorted((path, name) for path, name in added if not defined_at(pin, name))


def main(argv: list[str]) -> int:
    if len(argv) not in (2, 3):
        print(f"usage: {argv[0]} <base> [<head>]", file=sys.stderr)
        return 2
    if yaml is None:
        print("manifest-flag-guard: warning: PyYAML is unavailable; skipped", file=sys.stderr)
        return 0
    pin, undefined = check(*argv[1:])
    for path, name in undefined:
        print(f"manifest-flag-guard: {path} adds --{name}, which the pinned image ({pin}) does not define", file=sys.stderr)
    if not undefined:
        return 0
    print(
        "manifest-flag-guard: Argo applies the manifest at the push but the image moves only at the next pin, so the "
        "running binary would exit on the unknown flag (w8/331). Ship the flag's definition first and the manifest "
        "argument once a pin carries it, or pass the setting as an env var.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
