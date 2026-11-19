#!/usr/bin/env python3
"""scripts/github-actions-validate.sh check 11: a production job's log is public.

This repository is public, so every Actions log is too (w5/m501, ADR019
§Decision 5). In a job that holds production credentials (a production-*
environment, or the admin.conf fetch), a kubectl read (get, describe, logs,
top, events, wait) whose output reaches the log fails unless it
  - is captured ($(...), <(...), backticks) or redirected to a file,
  - ends its pipeline in scripts/lib/public_log.py, or in grep -q, wc, a
    `while read` loop or a redirect,
  - reads one platform namespace by literal -n, with no -A, no -o wide, and
    neither nodes, Machines, `describe pod` nor logs (they name nodes and
    addresses, or carry whatever tenants sent), or
  - sits under a reviewed `# public-log: <reason>` comment.
A script that offers --public-log is called with it. Scripts a job runs are
reviewed, not parsed: their output goes through public_log.py.

Usage: public_log_check.py <workflows dir>; prints one violation per line.
"""
from __future__ import annotations

import pathlib
import re
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from public_log import COMPONENT_NAMESPACES  # noqa: E402

READ_VERBS = {"get", "describe", "logs", "top", "events", "wait"}
# Options that take the next word as their value.
VALUE_OPTS = {
    "-n", "--namespace", "--context", "--kubeconfig", "--request-timeout", "--as",
    "-l", "--selector", "-o", "--output", "--field-selector", "-c", "--container",
    "--sort-by", "--tail", "--since", "-f", "--filename", "--timeout", "--for",
}
NODE_RESOURCES = re.compile(r"^(nodes?|no|machines?(\.cluster\.x-k8s\.io)?|hcloudmachines?\S*)(/|$)")
POD_RESOURCES = re.compile(r"^(pods?|po)(/|$)")
SILENT_SINKS = re.compile(r"^(python3 scripts/lib/public_log\.py|grep -[a-zA-Z]*[qc]|wc\b|while\b|read\b|true\b|:\s*$)")
KUBECTL = re.compile(r"(?<![\w./-])kubectl(?![\w.-])")
SCRIPT = re.compile(r"scripts/([\w.-]+\.sh)\b")
FETCHERS = ("scripts/fetch-app-kubeconfig.sh", "scripts/verify-substrate.sh")
MARKER = "# public-log:"


def public_log_scripts() -> set[str]:
    return {p.name for p in pathlib.Path("scripts").glob("*.sh") if "--public-log)" in p.read_text(errors="replace")}


def pipeline(line: str, start: int) -> list[str]:
    """Stages of the pipeline that starts at `start`, up to the first ;, &&,
    ||, a lone &, a closing ) or a backtick. A quoted word becomes the
    placeholder Q, so separators and redirects inside quotes do not count and a
    quoted namespace is never a literal platform one."""
    stages, cur, quote, i = [], [], None, start
    while i < len(line):
        c = line[i]
        if quote:
            if c == quote:
                quote = None
            i += 1
            continue
        if c in "'\"":
            quote = c
            if not cur or cur[-1] != "Q":
                cur.append("Q")
            i += 1
            continue
        two = line[i:i + 2]
        lone_amp = c == "&" and two != "&>" and line[i - 1:i] != ">"
        if two in ("&&", "||") or c in ";)`" or lone_amp:
            break
        if c == "|":
            stages.append("".join(cur).strip())
            cur = []
            i += 1
            continue
        cur.append(c)
        i += 1
    stages.append("".join(cur).strip())
    return stages


def captured(line: str, pos: int) -> bool:
    stack, i = [], 0
    while i < pos:
        if line.startswith(("$(", "<("), i):
            stack.append("cmd")
            i += 2
            continue
        if line[i] == "(":
            stack.append("(")
        elif line[i] == ")" and stack:
            stack.pop()
        i += 1
    return "cmd" in stack or line[:pos].count("`") % 2 == 1


def redirected(stage: str) -> bool:
    return re.search(r"(?<![0-9&>])>{1,2}\s*[^&\s]|&>", stage) is not None


def parse(args: list[str]) -> dict:
    """The namespace, verb and resource of `kubectl <args>`, and whether it
    spans every namespace or asks for the wide output."""
    out = {"ns": None, "verb": None, "resource": None, "all": False, "wide": False}
    i = 0
    while i < len(args):
        a = args[i]
        value = args[i + 1] if i + 1 < len(args) else None
        if a in ("-A", "--all-namespaces"):
            out["all"] = True
        elif re.fullmatch(r"(-o|--output)=?wide|-owide", a) or (a in ("-o", "--output") and value == "wide"):
            out["wide"] = True
        m = re.fullmatch(r"(?:-n|--namespace)=(.+)", a) or re.fullmatch(r"-n([^-=].*)", a)
        if m:
            out["ns"] = m.group(1)
        elif a in VALUE_OPTS:
            if a in ("-n", "--namespace"):
                out["ns"] = value
            i += 2
            continue
        elif not a.startswith("-"):
            if out["verb"] is None:
                out["verb"] = a
            elif out["resource"] is None:
                out["resource"] = a
        i += 1
    return out


def platform_scoped(cmd: dict) -> bool:
    if cmd["all"] or cmd["wide"] or cmd["ns"] not in COMPONENT_NAMESPACES:
        return False
    if cmd["verb"] == "logs":
        return False
    resource = cmd["resource"] or ""
    if NODE_RESOURCES.match(resource):
        return False
    return not (cmd["verb"] == "describe" and (not resource or POD_RESOURCES.match(resource)))


def production(block: list[tuple[int, str]]) -> bool:
    text = "\n".join(line for _, line in block)
    return bool(re.search(r"^    environment:\s*\S*production-|^      name:\s*\S*production-", text, re.M)) \
        or any(f in text for f in FETCHERS)


def joined(block: list[tuple[int, str]]) -> list[tuple[int, str]]:
    """(first line number, text) with \\ and trailing-| continuations joined."""
    out, buf, first = [], "", None
    for n, raw in block:
        line = raw.rstrip()
        if first is None:
            first = n
        if line.endswith("\\"):
            buf += line[:-1] + " "
            continue
        # A trailing | continues the pipeline; `run: |` opens a YAML block.
        if line.endswith("|") and not line.endswith("||") and not re.search(r":\s*\|$", line):
            buf += line + " "
            continue
        out.append((first, buf + line))
        buf, first = "", None
    if buf:
        out.append((first, buf))
    return out


def jobs(path: pathlib.Path) -> list[tuple[str, list[tuple[int, str]]]]:
    found, in_jobs = [], False
    for n, line in enumerate(path.read_text().splitlines(), 1):
        if re.match(r"^jobs:\s*$", line):
            in_jobs = True
            continue
        if not in_jobs:
            continue
        head = re.match(r"^  ([A-Za-z0-9_-]+):\s*(#.*)?$", line)
        if head:
            found.append((head.group(1), []))
        elif found:
            found[-1][1].append((n, line))
    return found


def violations(path: pathlib.Path, scripts: set[str]) -> list[str]:
    out = []
    for name, block in jobs(path):
        if not production(block):
            continue
        marker = False  # a `# public-log:` line in the comment run just above
        for n, line in joined(block):
            stripped = line.strip()
            if stripped.startswith("#"):
                marker = marker or stripped.startswith(MARKER)
                continue
            if not stripped:
                continue
            reviewed, marker = marker or MARKER in line, False
            if reviewed:
                continue
            for m in SCRIPT.finditer(line):
                if m.group(1) in scripts and "--public-log" not in line:
                    out.append(f"{path}:{n}: job {name}: scripts/{m.group(1)} supports --public-log; a production job must pass it")
            for m in KUBECTL.finditer(line):
                stages = pipeline(line, m.start())
                cmd = parse(stages[0].split()[1:])
                if cmd["verb"] not in READ_VERBS:
                    continue
                if captured(line, m.start()) or redirected(stages[0]):
                    continue
                if len(stages) > 1 and (SILENT_SINKS.match(stages[-1]) or redirected(stages[-1])):
                    continue
                if platform_scoped(cmd):
                    continue
                out.append(
                    f"{path}:{n}: job {name}: `kubectl {cmd['verb']}` prints to this public log; "
                    "pipe it through scripts/lib/public_log.py, capture it, or read one platform namespace"
                )
    return out


def main(argv: list[str]) -> int:
    scripts = public_log_scripts()
    for wf in sorted(pathlib.Path(argv[0]).glob("*.yml")):
        for v in violations(wf, scripts):
            print(v)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
