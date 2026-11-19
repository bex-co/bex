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
"""Keep `/loopx` running while its workstream still has actionable work.

The rule — never stop while a next pick exists — is prose in
`.claude/skills/loopx/SKILL.md`, and prose cannot enforce itself: a loop that
writes an end-of-run summary treats writing it as a turn boundary, naming the
next pick in the same breath. This can enforce it.

This script answers questions of fact and keeps time; it never judges. What
"actionable" means, in every mode:

- An open inbox note (`.pm/<wN>/NNN.md`, a name that starts with a digit) is
  actionable. A working document beside it (`builder-issues.md`,
  `RESEARCH-*.md`) is not an item, and neither is `dev-N/` or `routine/`.
- An open milestone (`.pm/<wN>/mN/`) is actionable when at least one of its
  open tasks has every written `depends_on` done. A milestone whose tasks all
  wait on unfinished work elsewhere is *waiting*: it neither holds the loop
  nor wakes the listener, and becomes actionable by itself once the work it
  names is done. Anything under `done/` or `blocked/` never counts.

Each mark belongs to the Claude Code session that armed it: `start` reads the
caller's `CLAUDE_CODE_SESSION_ID`, and the Stop hook consults only the mark of
the session that is stopping. Several sessions can therefore share one
checkout, each running its own loop or none, without blocking each other.

A parked item whose gate is checkable is *ready* once that gate clears, with
no board change: its workstream line says ``pin must carry `<sha>` `` and the
newest `chore(deploy): pin platform images to <sha>` commit on origin/main
pins a commit that contains every named one, or it says `date gate: <date>`
and that instant has passed, or both (`scripts/pm_gates.py`). `board` lists
such items under `blocked.ready` and makes the first one `next`, after an own
CI failure and before ready board items. Once one is dispatched, the gate it
was dispatched with is remembered: an item its worker parks again with the
same line is not offered again until the line changes.

A drained loop listens by default: `wait` polls `origin/main` with `git fetch`
and exponential backoff, costs no model tokens, and exits when the workstream
gains actionable work or a ready parked item, a routine falls due, one of the
loop's own pushes turned CI red, the idle limit passes, or fetch has failed
for two hours straight. Sessions share this checkout, so it also wakes for an
item a teammate filed here and has not pushed yet. One listener runs per workstream;
a second `wait` exits at once. While a listener is alive the Stop hook lets
the turn end and keeps the mark; a drained loop that is *not* listening is
held until it starts one. With `--no-listen`, drained means done.

While the orchestrator waits for a background worker, `dispatch <label>` marks
the loop in flight and the Stop hook lets turns end. For a board item the hook
checks the facts itself: once the item has left the open tree and everything
is pushed, the worker is finished and the loop is held again, whether or not
anyone ran `settle`. Marks without a board signal (routines, CI fixes) expire
on a timer.

`dispatch` also records which paths are already uncommitted. A teammate's
board filing or a foreign edit that was in the tree before the worker started
is not that worker's leftover: `outcome` and the Stop hook count only what
appeared since.

A drained workstream may also run routines listed in
`.pm/<wN>/routine/README.md` on origin/main. A routine is a named row holding
any command: a slash command with its arguments, a shell command, or a task in
plain words; the guard schedules it and never interprets it. `wait` exits 5
when one is due: its interval has passed, and the day's routine budget is not
spent. A row runs on its own schedule: `when: always` (the default) means
every interval, whatever changed; `when: changed` also requires one of the
row's `paths` (any path when it lists none) to have changed since the last
run, and `routine-done` doubles such a row's interval after an empty run
(capped at 14 days), resetting it after a run that shipped something. A row
that fails three runs in a row drops to one try a day until a run succeeds.
`wait` never sleeps past the next routine's due time, so a schedule holds even
while the board poll is backed off; board work still wins whenever both are
ready. Run state stays in `.git/loopx/`, never on the public board.

`/ship` ends at the push and nobody reads CI for that commit afterwards, so
the guard does: `outcome` records the commits a worker pushed, and `board` and
`wait` ask `gh` whether one of those commits is where a workflow on main first
went red. Only push-triggered runs are read: a scheduled run's head is
whatever main was at the time, which says nothing about who broke it. Red
that started on someone else's commit is not reported here; a worker meets it
at ship time (`scripts/ship-gates.sh` names it). Without `gh` the check is
skipped and says so.

Modes:

    loopx-guard.py start <wN> [--no-listen]
                                arm a loop for the calling Claude Code session
    loopx-guard.py end          disarm the calling session's loop (a genuine Exit)
    loopx-guard.py board <wN>   the facts for the next pick (JSON): own CI
                                failures, ready and waiting items, parked
                                items whose gate cleared, blocked items
                                changed since they were last listed, the due
                                routine, and the default next step
    loopx-guard.py dispatch <label>
                                mark the calling session's loop as waiting on
                                a background worker (Stop hook lets turns end)
    loopx-guard.py outcome <wN> <id> [<sha>...]
                                verify a worker's result (JSON): where <id>
                                sits on origin/main, whether the listed
                                commits are there, nothing unpushed, nothing
                                left in the tree that was not there at
                                dispatch; clears the in-flight mark when it holds.
                                <id> is an item, `routine:<name>`, or `ci:<slug>`
    loopx-guard.py settle       clear the in-flight mark by hand
    loopx-guard.py routine-due <wN>
                                the next due routine on origin/main (JSON)
    loopx-guard.py routine-done <wN> <name> found|empty|failed
                                record a routine run and reschedule it
    loopx-guard.py wait <wN> [--min S] [--max S] [--give-up S] [--chunk S]
                     [--fetch-give-up S]
                                block until origin/main (or an unpushed filing
                                in this checkout) gives <wN> something to
                                do, or a parked item's gate clears; exit 0
                                board work, 5 routine due, 6 own CI
                                failure, 3 idle limit, 4 fetch failing,
                                10 chunk elapsed (call again), 11 another
                                listener is already running
    loopx-guard.py              Stop hook: reads the hook payload on stdin and
                                blocks the stop while this session's loop has work
    loopx-guard.py forget       SessionEnd hook: drop the ending session's mark

Outside Claude Code there is no session id and no Stop hook (Codex has
neither), so `start` arms nothing and says so; `board`, `outcome`, `wait` and
the routine modes work the same everywhere, and the skill's exit rules still
apply.
"""

from __future__ import annotations

import hashlib
import json
import os
import pathlib
import random
import re
import shutil
import subprocess
import sys
import time

import pm_gates

# A stop attempt that makes no progress may repeat; bound it rather than spin.
MAX_BLOCKS_WITHOUT_PROGRESS = 25
# A mark this old belongs to a session that ended without SessionEnd (a crash).
STALE_SECONDS = 7 * 24 * 3600

# `wait` defaults: 2 min first poll, doubling, capped at 30 min, 24 h idle limit.
WAIT_MIN = 120.0
WAIT_MAX = 1800.0
WAIT_GIVE_UP = 24 * 3600.0
# Fetch may fail this long in a row (a sleeping laptop, a flaky network) before
# the listener gives up; it keeps backing off normally meanwhile.
WAIT_FETCH_GIVE_UP = 2 * 3600.0
EXIT_WORK, EXIT_GAVE_UP, EXIT_FETCH, EXIT_ROUTINE, EXIT_CI, EXIT_CHUNK, EXIT_BUSY = 0, 3, 4, 5, 6, 10, 11

# A board item's worker marked in flight longer than this is presumed lost.
INFLIGHT_MAX = 12 * 3600.0
# A routine or CI-fix worker has no board signal, so only this timer ends its mark.
INFLIGHT_UNTRACKED_MAX = 3 * 3600.0
# Once an item has left the open tree, its worker has this long to finish shipping.
INFLIGHT_SHIP_GRACE = 30 * 60.0
# A routine that failed this many runs in a row retries once per paused interval.
ROUTINE_FAILURES_BEFORE_PAUSE = 3
ROUTINE_PAUSED_INTERVAL = 24 * 3600.0
# CI: how long a pushed commit is watched, and how many runs one `gh` call reads.
# The listing is unfiltered, so it must reach back past the window on a busy
# day: 400 runs covered 5 to 8 hours of this repository on 2026-11-15.
CI_WINDOW = 4 * 3600.0
CI_RUNS = 400
# A workflow that runs rarely can show a red streak with nothing before it in
# that listing; its own history (this many runs) says where the streak began.
CI_WORKFLOW_RUNS = 50
# A completed run with one of these conclusions is no verdict; any other
# conclusion but `success` is red (the same reading as scripts/ci-red-streak.sh).
CI_NO_VERDICT = ("cancelled", "skipped", "neutral")
# While a pushed commit's CI is still running, poll at least this often.
CI_WATCH_PAUSE = 300.0
# A red workflow that has not run again for this long is no longer tracked.
CI_OWNED_MAX = 7 * 24 * 3600.0
# Routines: runs allowed per rolling 24 h, and the cap for a backed-off interval.
ROUTINE_BUDGET = 4
ROUTINE_MAX_INTERVAL = 14 * 24 * 3600.0
# A routine row's name: its identity in run state and in `routine-done`.
ROUTINE_NAME = re.compile(r"[a-z0-9][a-z0-9-]{0,63}")
WHEN = ("changed", "always")
DURATION = re.compile(r"(\d+(?:\.\d+)?)\s*(m|h|d|w)")
DURATION_UNITS = {"m": 60, "h": 3600, "d": 86400, "w": 7 * 86400}

SESSION_ID = re.compile(r"[A-Za-z0-9_-]{1,128}")
WORKSTREAM = re.compile(r"w\d+")
ITEM_ID = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}")
# A worker with no board item: `routine:<name>` or `ci:<workflow slug>`.
PSEUDO_ID = re.compile(r"(?:routine|ci):[a-z0-9][a-z0-9-]{0,63}")
ITEM_LABEL = re.compile(r"(w\d+)/([A-Za-z0-9][A-Za-z0-9_.-]{0,127})")
COMMIT = re.compile(r"[0-9a-fA-F]{7,40}")
# The three names on a board. Directly under `.pm/<wN>/` only a milestone
# directory and a numbered note are items; a working document is not.
MILESTONE, TASK, NOTE = r"m\d[\w.-]*", r"t\d+[\w.-]*", r"\d[\w.-]*"
MILESTONE_NAME = re.compile(MILESTONE)
NOTE_NAME = re.compile(rf"{NOTE}\.md")
TASK_FILE = re.compile(rf"{TASK}\.md")
# A task in one of these states is finished, or is not this loop's to start.
TASK_FINISHED = ("done", "abandoned", "superseded", "moved")
TASK_HELD = ("blocked", "deferred")
DEP_TASK = re.compile(rf"(w\d+)/({MILESTONE})/({TASK})")
DEP_MILESTONE = re.compile(rf"(w\d+)/({MILESTONE})")
DEP_NOTE = re.compile(rf"(w\d+)/({NOTE})")
REMOTE_REF = "origin/main"
PULL = "git pull --rebase --autostash origin main"
# `outcome` prints at most this many leftover paths, and always their count.
LEFTOVER_SHOWN = 50


def project_dir() -> pathlib.Path:
    return pathlib.Path(os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd())


def state_dir() -> pathlib.Path:
    """Runtime state, never project content: `.git/` is inherently untracked."""
    git = project_dir() / ".git"
    if git.is_dir():
        return git / "loopx"
    # Worktrees and odd checkouts: fall back to a per-repo temp directory.
    import tempfile

    key = hashlib.sha256(str(project_dir()).encode()).hexdigest()[:16]
    return pathlib.Path(tempfile.gettempdir()) / f"loopx-{key}"


def mark_path(session: object) -> pathlib.Path | None:
    """The session's mark, or None for a missing or unsafe session id."""
    if not isinstance(session, str) or not SESSION_ID.fullmatch(session):
        return None
    return state_dir() / f"{session}.json"


def wait_path(workstream: str) -> pathlib.Path:
    """The listener's state for a workstream: idle start, pid, gave_up."""
    return state_dir() / f"wait-{workstream}.state"


def board_state_path(workstream: str) -> pathlib.Path:
    """Which blocked items `board` has already listed, by content hash."""
    return state_dir() / f"board-{workstream}.state"


def handed_path(workstream: str) -> pathlib.Path:
    """The gate line each parked item had when a worker was last dispatched for it."""
    return state_dir() / f"handed-{workstream}.state"


def ci_state_path(workstream: str) -> pathlib.Path:
    """The loop's recent pushes, and the workflows one of them turned red."""
    return state_dir() / f"ci-{workstream}.state"


def calling_session() -> pathlib.Path | None:
    return mark_path(os.environ.get("CLAUDE_CODE_SESSION_ID"))


def read_json(path: pathlib.Path) -> dict:
    try:
        data = json.loads(path.read_text())
    except Exception:
        return {}
    return data if isinstance(data, dict) else {}


def write_json(path: pathlib.Path, data: dict) -> None:
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(data))
    except OSError:
        pass


def sweep() -> None:
    """Drop marks left by sessions that ended without SessionEnd (a crash)."""
    cutoff = time.time() - STALE_SECONDS
    for path in state_dir().glob("*.json"):
        try:  # another session may remove its own mark meanwhile
            if path.stat().st_mtime < cutoff:
                path.unlink()
        except OSError:
            pass


def natural(name: str) -> list:
    """Sort key that orders m9 before m10 and 089.md before 1014.md."""
    return [int(part) if part.isdigit() else part for part in re.split(r"(\d+)", name)]


def classify(entries: list[tuple[str, bool]]) -> tuple[list[str], list[str]]:
    """Open milestones and inbox notes among (name, is_dir) top-level entries.

    `done/` and `blocked/` are directories that are not `mN`, so work that
    moved there never counts.
    """
    milestones = sorted((name for name, is_dir in entries if is_dir and MILESTONE_NAME.fullmatch(name)), key=natural)
    notes = sorted((name for name, is_dir in entries if not is_dir and NOTE_NAME.fullmatch(name)), key=natural)
    return milestones, notes


def git(*args: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git", *args],
        cwd=project_dir(),
        capture_output=True,
        text=True,
        check=False,
    )


class Tree:
    """Everything under `.pm/`: in the working tree, or at a git ref."""

    def __init__(self, ref: str | None = None) -> None:
        self.ref = ref
        self.ok = True
        self.files: set[str] = set()
        self.kids: dict[str, dict[str, bool]] = {}
        if ref is None:
            base = project_dir()
            for dirpath, dirnames, filenames in os.walk(base / ".pm"):
                rel = pathlib.Path(dirpath).relative_to(base).as_posix()
                for name in dirnames:  # an empty directory is still an item
                    self._add(f"{rel}/{name}", True)
                for name in filenames:
                    self._add(f"{rel}/{name}", False)
                # Nothing read from the working tree looks inside `done/`, and
                # it holds nine files in ten: list it, do not walk it.
                dirnames[:] = [name for name in dirnames if name != "done"]
            return
        listed = git("ls-tree", "-r", "-z", "--name-only", ref, "--", ".pm/")
        if listed.returncode != 0:
            self.ok = False
            return
        for path in listed.stdout.split("\0"):
            if path:
                self._add(path, False)

    def _add(self, path: str, is_dir: bool) -> None:
        if not is_dir:
            self.files.add(path)
        parent, _, name = path.rpartition("/")
        while parent:
            siblings = self.kids.setdefault(parent, {})
            siblings[name] = siblings.get(name, False) or is_dir
            is_dir = True
            parent, _, name = parent.rpartition("/")

    def children(self, directory: str) -> list[tuple[str, bool]]:
        """(name, is_dir) for each entry directly inside DIRECTORY."""
        return sorted(self.kids.get(directory, {}).items())

    def has_dir(self, path: str) -> bool:
        parent, _, name = path.rpartition("/")
        return self.kids.get(parent, {}).get(name) is True

    def read_many(self, paths: list[str]) -> dict[str, str]:
        """The text of each of PATHS that exists."""
        if not paths:
            return {}
        texts: dict[str, str] = {}
        if self.ref is None:
            for path in paths:
                try:
                    texts[path] = (project_dir() / path).read_text(encoding="utf-8", errors="replace")
                except OSError:
                    pass
            return texts
        batch = subprocess.run(
            ["git", "cat-file", "--batch"],
            cwd=project_dir(),
            input="".join(f"{self.ref}:{path}\n" for path in paths).encode(),
            capture_output=True,
            check=False,
        )
        data, at = batch.stdout, 0
        for path in paths:
            newline = data.find(b"\n", at)
            if newline < 0:
                break
            header = data[at:newline].split()
            at = newline + 1
            if len(header) != 3:  # "<name> missing"
                continue
            size = int(header[2])
            texts[path] = data[at : at + size].decode("utf-8", "replace")
            at += size + 1
        return texts


def frontmatter(text: str) -> dict[str, str]:
    """The leading `---` block's keys.

    A list becomes comma-separated, however it is written: inline, as `- item`
    rows, or as a `[...]` that prettier wrapped over several indented lines.
    """
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    meta: dict[str, str] = {}
    key = None
    for line in lines[1:80]:
        if line.strip() == "---":
            break
        # A comment ends with its line: drop it before the rows are joined.
        line = re.sub(r"\s+#.*$", "", line)
        found = re.match(r"([A-Za-z_]+):\s*(.*)$", line)
        if found:
            key = found.group(1)
            meta[key] = found.group(2).strip()
        elif key and line[:1].isspace() and line.strip():
            meta[key] += "," + line.strip().removeprefix("- ")
    return meta


def unquoted(value: str) -> str:
    return value.strip().strip("\"'")


def task_status(text: str) -> str:
    return unquoted(frontmatter(text).get("status", "todo")).lower()


def dep_ids(value: str) -> list[str]:
    return [dep for dep in map(unquoted, re.split(r"[\[\],]", value)) if dep]


def dep_unmet(tree: Tree, dep: str) -> bool:
    """True while the task, milestone, or note DEP names is still open or blocked.

    A name that resolves to nothing (dropped, renamed, mistyped) never holds
    work back: the worker's triage deals with it.
    """
    found = DEP_TASK.fullmatch(dep)
    if found:
        queue, milestone, task = found.groups()
        paths = [f".pm/{queue}/{place}{milestone}/{task}.md" for place in ("", "blocked/")]
        # A task marked finished where it stands is met, as `board` reads it.
        texts = tree.read_many([path for path in paths if path in tree.files])
        return any(task_status(text) not in TASK_FINISHED for text in texts.values())
    found = DEP_MILESTONE.fullmatch(dep)
    if found:
        queue, milestone = found.groups()
        return any(tree.has_dir(f".pm/{queue}/{place}{milestone}") for place in ("", "blocked/"))
    found = DEP_NOTE.fullmatch(dep)
    if found:
        queue, note = found.groups()
        note = note.removesuffix(".md")
        return any(f".pm/{queue}/{place}{note}.md" in tree.files for place in ("", "blocked/"))
    return False


def board(tree: Tree, workstream: str) -> dict:
    """Ready milestones and notes in pick order, and what each waiting milestone waits on."""
    root = f".pm/{workstream}"
    names, notes = classify(tree.children(root))
    milestones = {
        name: [task for task, nested in tree.children(f"{root}/{name}") if not nested and TASK_FILE.fullmatch(task)]
        for name in names
    }
    texts = tree.read_many([f"{root}/{name}/{task}" for name, tasks in milestones.items() for task in tasks])

    ready: list[str] = []
    waiting: dict[str, list[str]] = {}
    for name, tasks in milestones.items():
        held: list[str] = []
        live = 0
        workable = False
        for task in tasks:
            meta = frontmatter(texts.get(f"{root}/{name}/{task}", ""))
            status = unquoted(meta.get("status", "todo")).lower()
            if status in TASK_FINISHED:
                continue
            live += 1
            if status in TASK_HELD:
                held.append(f"{workstream}/{name}/{task.removesuffix('.md')} ({status})")
                continue
            unmet = [dep for dep in dep_ids(meta.get("depends_on", "")) if dep_unmet(tree, dep)]
            if not unmet:
                workable = True
                break
            held += unmet
        # No live task at all (no task files, or every one finished) still needs
        # a worker: to triage it, or to close it out.
        if workable or not live:
            ready.append(name)
        else:
            waiting[name] = sorted(set(held))
    return {"milestones": ready, "notes": notes, "waiting": waiting}


def actionable(workstream: str) -> tuple[list[str], list[str]]:
    """Ready milestones and open inbox notes in the working tree, in pick order."""
    try:
        facts = board(Tree(), workstream)
        return facts["milestones"], facts["notes"]
    except Exception:
        # Never let a parsing surprise release the loop: fall back to "it exists".
        root = project_dir() / ".pm" / workstream
        if not root.is_dir():
            return [], []
        return classify([(p.name, p.is_dir()) for p in root.iterdir()])


def digest(line: str) -> str:
    return hashlib.sha256(line.encode()).hexdigest()[:16]


def parked_gates(tree: Tree, workstream: str) -> dict[str, tuple[pm_gates.Gate, str]]:
    """Each parked item with a checkable gate, in pick order: the gate and a digest of its line."""
    root = f".pm/{workstream}"
    milestones, notes = classify(tree.children(f"{root}/blocked"))
    if not milestones and not notes:
        return {}
    lines = pm_gates.parked_lines(tree.read_many([f"{root}/README.md"]).get(f"{root}/README.md", ""))
    gates = {}
    for name in [*milestones, *notes]:
        text = pm_gates.gate_text(lines[name]) if name in lines else None
        gate = pm_gates.parse(text) if text else None
        if gate and (gate.commits or gate.after is not None):
            gates[name] = (gate, digest(lines[name]))
    return gates


def production_pin(ref: str = REMOTE_REF) -> str | None:
    """The commit the newest `chore(deploy): pin platform images to <sha>` on REF names."""
    listed = git("log", "-n", "20", "-E", f"--grep={pm_gates.PIN_GREP}", "--format=%s", ref, "--")
    if listed.returncode == 0:
        for subject in listed.stdout.splitlines():
            found = pm_gates.PIN_SUBJECT.match(subject)  # the grep also matches body lines
            if found:
                return found.group(1)
    return None


def gate_facts(tree: Tree, workstream: str, ref: str = REMOTE_REF) -> dict:
    """Parked items whose checkable gate holds now, in pick order.

    `ready`: every commit the gate names is an ancestor of the production pin
    on REF and its date gate (if any) has passed, and no worker was already
    dispatched with this exact line. `next_in`: seconds until the soonest
    date gate still ahead. `problems`: commits that cannot clear.
    """
    gates = parked_gates(tree, workstream)
    named = any(gate.commits for gate, _ in gates.values())
    pin = production_pin(ref) if named else None
    handed = read_json(handed_path(workstream)).get("handed")
    handed = handed if isinstance(handed, dict) else {}
    carried: dict[str, int] = {}
    ready, problems, next_in = [], [], None
    now = time.time()
    if named and pin is None:
        problems.append(f"no `chore(deploy): pin platform images to <sha>` commit on {ref}")
    for name, (gate, line) in gates.items():
        holds = True
        if gate.after is not None and now < gate.after:
            next_in = gate.after - now if next_in is None else min(next_in, gate.after - now)
            holds = False
        for sha in gate.commits if pin else ():
            if sha not in carried:
                carried[sha] = git("merge-base", "--is-ancestor", sha, pin).returncode
                if carried[sha] not in (0, 1):
                    problems.append(f"{workstream}/{name}: `{sha}` is not a commit on {ref}")
        if gate.commits and (pin is None or any(carried[sha] != 0 for sha in gate.commits)):
            holds = False
        if holds and handed.get(name) != line:
            ready.append(name)
    facts: dict = {"ready": ready, "next_in": next_in}
    if pin:
        facts["pin"] = pin
    if problems:
        facts["problems"] = problems
    return facts


def hand_over(workstream: str, item: str) -> None:
    """Remember the gate line of a parked ITEM a worker is dispatched for.

    `board` does not offer it again while its line is unchanged: a worker that
    parks it again must write the gate that is left, and one that forgot to
    would otherwise be dispatched for the same gate on every pick.
    """
    blocked = project_dir() / ".pm" / workstream / "blocked"
    stem = item.removesuffix(".md")
    name = stem if (blocked / stem).is_dir() else f"{stem}.md"
    if not (blocked / name).exists():
        return  # an open item: nothing parked to remember
    try:
        readme = (project_dir() / ".pm" / workstream / "README.md").read_text(encoding="utf-8", errors="replace")
    except OSError:
        return
    line = pm_gates.parked_lines(readme).get(name)
    if line is None:
        return
    path = handed_path(workstream)
    handed = read_json(path).get("handed")
    handed = {k: v for k, v in (handed if isinstance(handed, dict) else {}).items() if (blocked / k).exists()}
    handed[name] = digest(line)
    write_json(path, {"handed": handed})


def next_pick(workstream: str, milestones: list[str], notes: list[str]) -> str:
    return f"{workstream}/{milestones[0]}" if milestones else f"{workstream}/{notes[0]}"


def pid_alive(pid: object) -> bool:
    if not isinstance(pid, int) or pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    except OSError:
        return False
    return True


def listener_alive(state: dict) -> bool:
    """True when the pid in a listener's state is a running `wait`.

    A bare pid check is not enough: after a reboot or a long uptime the number
    belongs to some other process, and the loop would wait on it forever.
    """
    pid = state.get("pid")
    if not pid_alive(pid):
        return False
    try:
        shown = subprocess.run(
            ["ps", "-o", "command=", "-p", str(pid)], capture_output=True, text=True, timeout=5, check=False
        )
    except Exception:
        return True  # no `ps` here: the pid check is all there is
    return shown.returncode == 0 and "loopx-guard" in shown.stdout and " wait" in shown.stdout


def item_open_locally(workstream: str, item: str) -> bool:
    root = project_dir() / ".pm" / workstream
    stem = item.removesuffix(".md")
    return (root / stem).is_dir() or (root / f"{stem}.md").is_file() or any(root.glob(f"{stem}-*"))


def nothing_unpushed() -> bool:
    """True when every local commit is already on origin/main (as last fetched)."""
    return git("merge-base", "--is-ancestor", "HEAD", REMOTE_REF).returncode == 0


def dirty_paths() -> set[str] | None:
    """Every file with uncommitted changes, or None when git cannot say.

    Each untracked file is listed by itself: a file a worker adds to a
    directory that was already untracked must still show up as new.
    """
    # No optional locks: a status that refreshes the index would hold
    # `index.lock` against a teammate's `git add` in this shared checkout.
    status = git("--no-optional-locks", "status", "--porcelain", "-z", "--untracked-files=all")
    if status.returncode != 0:
        return None
    paths = set()
    fields = iter(status.stdout.split("\0"))
    for entry in fields:
        if len(entry) > 3:
            paths.add(entry[3:])
            if entry[0] in "RC":
                next(fields, None)  # a rename or copy is followed by its old path
    return paths


def leftovers(baseline: set[str]) -> list[str] | None:
    """Uncommitted paths that are not in BASELINE; None when git cannot say.

    Sessions share this checkout. What was in the tree before a worker started
    (a teammate's board filing, a foreign edit) is not that worker's leftover.
    """
    now = dirty_paths()
    return None if now is None else sorted(now - baseline)


def same_item(a: object, b: object) -> bool:
    """True when two labels name one item: `w2/089` and `w2/089.md` do."""
    return isinstance(a, str) and isinstance(b, str) and a.removesuffix(".md") == b.removesuffix(".md")


def dirty_at_dispatch(state: dict, label: object) -> set[str]:
    """What was already uncommitted when the session of STATE dispatched LABEL.

    It stays until the next dispatch, so it outlives the in-flight mark: the
    Stop hook drops that mark when it sees the worker finish, and `outcome`
    still measures against the same baseline afterwards. Empty for any other
    label: then everything uncommitted counts.
    """
    baseline = state.get("baseline")
    if not isinstance(baseline, dict) or not same_item(baseline.get("label"), label):
        return set()
    dirty = baseline.get("dirty")
    return set(dirty) if isinstance(dirty, list) else set()


def inflight_verdict(inflight: dict, baseline: set[str]) -> str:
    """`running`, `finished`, or `lost` for a dispatched worker.

    BASELINE is what was already uncommitted at dispatch. Mutates INFLIGHT to
    remember when its item was first seen gone.
    """
    now = time.time()
    since = inflight.get("since")
    if not isinstance(since, (int, float)):
        return "lost"
    age = now - since
    found = ITEM_LABEL.fullmatch(str(inflight.get("label", "")))
    if not found or not inflight.get("was_open"):
        # A routine, a CI fix, or an item that was not open when dispatched
        # (a blocked one being revived): no board signal, only the timer.
        limit = INFLIGHT_MAX if found else INFLIGHT_UNTRACKED_MAX
        return "running" if age < limit else "lost"
    if age >= INFLIGHT_MAX:
        return "lost"
    if item_open_locally(*found.groups()):
        return "running"
    gone = inflight.get("gone_since")
    if not isinstance(gone, (int, float)):
        gone = inflight["gone_since"] = now
    if nothing_unpushed() and leftovers(baseline) == []:
        return "finished"
    return "running" if now - gone < INFLIGHT_SHIP_GRACE else "lost"


def settle_label(label: str) -> bool:
    """Clear the calling session's in-flight mark if it is for LABEL."""
    path = calling_session()
    if path is None or not path.is_file():
        return False
    state = read_json(path)
    if not same_item((state.get("inflight") or {}).get("label"), label):
        return False
    state.pop("inflight", None)
    write_json(path, state)
    return True


def parse_duration(text: str) -> float | None:
    match = DURATION.fullmatch(text.strip().lower())
    if not match:
        return None
    seconds = float(match.group(1)) * DURATION_UNITS[match.group(2)]
    return seconds if seconds > 0 else None


def parse_routines(text: str) -> tuple[list[dict], int, list[str]]:
    """Routines, the daily budget, and problems from a routine README.

    Rows look like `| cli-qa | /qa-find-bugs-cli w4 | 30m | always | |`:
    a name, any command, an interval, an optional `when` (`always`, the
    default, or `changed`), and optional comma-separated `paths` that gate a
    `changed` row. A line like `Budget: 4 runs/day` overrides the default
    budget. The command is opaque here: the routine worker runs it.
    """
    routines, problems, seen = [], [], set()
    budget = ROUTINE_BUDGET
    for line in text.splitlines():
        found = re.match(r"\s*\**budget:?\**:?\s*(\d+)", line, re.IGNORECASE)
        if found:
            budget = int(found.group(1))
            continue
        if not line.strip().startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        name = cells[0].strip("`")
        if name.lower() == "name" or not name or set(name) <= set("-: "):
            continue  # the header or its separator
        if len(cells) < 3:
            problems.append(f"{name!r}: a row needs name, command, and every")
            continue
        command = cells[1].strip("`").strip()
        every = cells[2].strip("`")
        when = (cells[3].strip("`") if len(cells) > 3 and cells[3] else "always").lower()
        paths = [p.strip().strip("`").rstrip("/") for p in (cells[4] if len(cells) > 4 else "").split(",")]
        paths = [p for p in paths if p]
        interval = parse_duration(every)
        if not ROUTINE_NAME.fullmatch(name):
            problems.append(f"{name!r} is not a routine name like cli-qa (lowercase, digits, dashes)")
        elif not command:
            problems.append(f"{name}: no command")
        elif interval is None:
            problems.append(f"{name}: {every!r} is not an interval like 24h or 3d")
        elif when not in WHEN:
            problems.append(f"{name}: when must be changed or always, not {when!r}")
        elif any(p.startswith(("/", "-")) or ".." in p.split("/") for p in paths):
            problems.append(f"{name}: paths must be repository-relative, not {', '.join(paths)!r}")
        elif name in seen:
            problems.append(f"{name}: listed twice")
        else:
            seen.add(name)
            routines.append({"name": name, "command": command, "every": interval, "when": when, "paths": paths})
    return routines, budget, problems


def routine_state_path(workstream: str) -> pathlib.Path:
    return state_dir() / f"routine-{workstream}.state"


def due_routine(workstream: str, ref: str = REMOTE_REF) -> dict:
    """The first due routine at REF, or why none is: {"due": {...} | None, ...}."""
    shown = git("show", f"{ref}:.pm/{workstream}/routine/README.md")
    if shown.returncode != 0:
        return {"due": None, "reason": "no routine list", "problems": [], "next_in": None}
    routines, budget, problems = parse_routines(shown.stdout)
    if not routines:
        return {"due": None, "reason": "routine list is empty", "problems": problems, "next_in": None}

    state = read_json(routine_state_path(workstream))
    now = time.time()
    runs = [t for t in state.get("runs", []) if isinstance(t, (int, float)) and now - t < 86400]
    if len(runs) >= budget:
        return {
            "due": None,
            "reason": f"daily budget of {budget} routine runs is spent",
            "problems": problems,
            # The budget frees up as the oldest run in the window ages out.
            "next_in": max(min(runs) + 86400 - now, 0.0),
        }

    entries = state.get("routines", {}) if isinstance(state.get("routines"), dict) else {}
    waiting = []
    next_in = None  # seconds until the soonest routine's interval passes
    for routine in routines:
        name, paths = routine["name"], routine["paths"]
        missing = [p for p in paths if git("cat-file", "-e", f"{ref}:{p}").returncode != 0]
        if missing:
            problems.append(f"{name}: no {', '.join(missing)} on {ref}")
            continue
        entry = entries.get(name, {})
        interval = entry.get("interval") if isinstance(entry.get("interval"), (int, float)) else routine["every"]
        if routine["when"] == "always":
            interval = routine["every"]  # fixed cadence: no backoff
        if int(entry.get("failures", 0)) >= ROUTINE_FAILURES_BEFORE_PAUSE:
            interval = max(interval, ROUTINE_PAUSED_INTERVAL)  # it keeps failing: one try a day
        last_run, last_sha = entry.get("last_run"), entry.get("last_sha")
        if isinstance(last_run, (int, float)) and now - last_run < interval:
            remaining = interval - (now - last_run)
            next_in = remaining if next_in is None else min(next_in, remaining)
            waiting.append(f"{name}: next in {remaining / 3600:.1f} h")
            continue
        if routine["when"] == "changed" and isinstance(last_sha, str) and last_sha:
            changed = git("log", "-1", "--format=%H", f"{last_sha}..{ref}", "--", *paths)
            # An unknown SHA (history rewritten, shallow clone) counts as changed.
            if changed.returncode == 0 and not changed.stdout.strip():
                waiting.append(f"{name}: no change since {last_sha[:9]}")
                continue
        return {
            "due": {
                "name": name,
                "command": routine["command"],
                "when": routine["when"],
                "paths": paths,
                "interval_hours": round(interval / 3600, 2),
            },
            "problems": problems,
        }
    # A `changed` row waiting only for a commit has no time to wake for: the
    # board poll notices the commit.
    return {"due": None, "reason": "; ".join(waiting) or "nothing due", "problems": problems, "next_in": next_in}


def record_pushes(workstream: str, shas: list[str]) -> None:
    """Remember commits this loop pushed, so their CI results get read."""
    full = []
    for sha in shas:
        resolved = git("rev-parse", "--verify", "-q", f"{sha}^{{commit}}")
        if resolved.returncode == 0 and resolved.stdout.strip():
            full.append(resolved.stdout.strip())
    if not full:
        return
    path = ci_state_path(workstream)
    state = read_json(path)
    now = time.time()
    pushes = [p for p in state.get("pushes", []) if isinstance(p, dict) and now - p.get("at", 0) < CI_WINDOW]
    known = {p.get("sha") for p in pushes}
    pushes += [{"sha": sha, "at": now} for sha in full if sha not in known]
    state["pushes"] = pushes
    write_json(path, state)


def ci_slug(workflow: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", workflow.lower()).strip("-")[:60] or "workflow"


def gh_runs(workflow: str | None = None) -> list | None:
    """Recent push-triggered workflow runs on main, oldest first; None when `gh` cannot say.

    With WORKFLOW, only that workflow's runs, read from its own history. The
    branch and the event are filtered here, not by `gh`: `--branch` turns
    GitHub's listing into a search that can answer with a page of only old
    runs (scripts/ship-gates.sh records the incident).
    """
    if not shutil.which("gh"):
        return None
    scope = ["--workflow", workflow, "--limit", str(CI_WORKFLOW_RUNS)] if workflow else ["--limit", str(CI_RUNS)]
    try:
        listed = subprocess.run(
            [
                "gh", "run", "list", *scope,
                "--json", "databaseId,workflowName,headSha,headBranch,event,status,conclusion,createdAt,url",
            ],
            cwd=project_dir(), capture_output=True, text=True, timeout=45, check=False,
        )
        runs = json.loads(listed.stdout) if listed.returncode == 0 else None
    except Exception:
        return None
    if not isinstance(runs, list):
        return None
    pushed = (r for r in runs if isinstance(r, dict) and r.get("headBranch") == "main" and r.get("event") == "push")
    return sorted(pushed, key=lambda r: str(r.get("createdAt", "")))


def has_verdict(run: dict) -> bool:
    return run.get("status") == "completed" and run.get("conclusion") not in CI_NO_VERDICT


def streak_began(workflow: str, settled: list[dict], streak: list[dict]) -> dict | None:
    """The run that began WORKFLOW's current red streak; None when `gh` cannot say.

    The repository-wide listing holds only the newest CI_RUNS runs. When no
    success comes before the streak there, the streak may have begun long
    before the listing does, so the workflow's own history decides.
    """
    if len(streak) < len(settled):
        return streak[0]
    history = gh_runs(workflow)
    if history is None:
        return None
    first = str(streak[0].get("createdAt", ""))
    start = streak[0]
    for run in reversed([r for r in history if has_verdict(r) and str(r.get("createdAt", "")) < first]):
        if run.get("conclusion") == "success":
            break
        start = run
    return start


def own_ci(workstream: str, mark: bool = False) -> dict:
    """Workflows on main that one of this loop's pushes turned red.

    A workflow is *ours* when the first failing run of its current red streak
    is on a commit this loop pushed. It stays ours until a run passes, and is
    reported again only when another of our commits (a fix attempt) fails too.
    Red that began on someone else's commit is theirs: never reported here.
    Where the streak began is checked once per owned workflow, and once more
    for an entry recorded before that check existed.
    With MARK the reported runs are remembered, so the next call is quiet.
    """
    path = ci_state_path(workstream)
    state = read_json(path)
    now = time.time()
    pushes = [p for p in state.get("pushes", []) if isinstance(p, dict) and now - p.get("at", 0) < CI_WINDOW]
    owned = state.get("owned") if isinstance(state.get("owned"), dict) else {}
    owned = {
        workflow: entry
        for workflow, entry in owned.items()
        if isinstance(entry, dict) and isinstance(entry.get("reported"), list) and now - entry.get("at", now) < CI_OWNED_MAX
    }
    quiet = {"own_failures": [], "still_red": sorted(owned), "pending": 0, "watch": False}
    if not pushes and not owned:
        return {"checked": False, "reason": "no pushes to watch", **quiet}
    runs = gh_runs()
    if runs is None:
        return {"checked": False, "reason": "gh is missing or not signed in: CI was not checked", **quiet}

    ours = {p.get("sha") for p in pushes}
    by_workflow: dict[str, list[dict]] = {}
    for run in runs:
        by_workflow.setdefault(str(run.get("workflowName", "")), []).append(run)
    failures = []
    pending = 0
    for workflow, sequence in by_workflow.items():
        pending += sum(1 for r in sequence if r.get("status") != "completed" and r.get("headSha") in ours)
        settled = [r for r in sequence if has_verdict(r)]
        streak: list[dict] = []
        for run in reversed(settled):
            if run.get("conclusion") == "success":
                break
            streak.insert(0, run)
        if not streak:
            if settled:
                owned.pop(workflow, None)  # green again
            continue
        entry = owned.get(workflow)
        if not isinstance(entry, dict) or not entry.get("verified"):
            claim = entry.get("since") if isinstance(entry, dict) else None
            if claim is None and streak[0].get("headSha") not in ours:
                continue  # it went red on someone else's commit
            start = streak_began(workflow, settled, streak)
            if start is None:
                continue  # gh cannot say where the streak began: ask again next time
            if start.get("headSha") not in ((ours | {claim}) if claim else ours):
                owned.pop(workflow, None)
                continue  # it was already red before our push
            if not isinstance(entry, dict):
                entry = owned[workflow] = {"since": start.get("headSha", ""), "reported": [], "at": now}
            entry["verified"] = True
        fresh = [r for r in streak if r.get("headSha") in ours and r.get("databaseId") not in entry["reported"]]
        if fresh:
            failures.append(
                {
                    "workflow": workflow,
                    "label": f"ci:{ci_slug(workflow)}",
                    "since": str(entry.get("since", ""))[:9],
                    "run": fresh[-1].get("databaseId"),
                    "url": fresh[-1].get("url", ""),
                    "attempt": len(entry["reported"]) + 1,
                }
            )
            if mark:
                entry["reported"] += [r.get("databaseId") for r in fresh]
    state["pushes"] = pushes
    state["owned"] = owned
    write_json(path, state)
    recent = any(now - p.get("at", 0) < 1200 for p in pushes)
    return {
        "checked": True,
        "own_failures": failures,
        "still_red": sorted(owned),
        "pending": pending,
        "watch": bool(pending or recent),
    }


def blocked_review(workstream: str, ready: list[str] | tuple = ()) -> tuple[list[str], int]:
    """Blocked items whose content changed since `board` last listed them, and the count of the rest.

    An item in READY is listed there instead, and counted in neither.
    """
    listed = git("ls-tree", "HEAD", "--", f".pm/{workstream}/blocked/")
    current: dict[str, str] = {}
    if listed.returncode == 0:
        for line in listed.stdout.splitlines():
            meta, _, path = line.partition("\t")
            parts = meta.split()
            if len(parts) >= 3 and path:
                current[path.rsplit("/", 1)[-1]] = parts[2]
    path = board_state_path(workstream)
    seen = read_json(path).get("blocked")
    seen = seen if isinstance(seen, dict) else {}
    review = sorted((name for name, sha in current.items() if seen.get(name) != sha and name not in ready), key=natural)
    write_json(path, {"blocked": current})
    return review, sum(1 for name in current if name not in ready) - len(review)


def allow(path: pathlib.Path | None = None) -> None:
    """Let the stop through, removing PATH (the session's mark) when given."""
    if path is not None:
        try:
            path.unlink()
        except OSError:
            pass
    print("{}")
    sys.exit(0)


def read_payload() -> dict:
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return {}
    return payload if isinstance(payload, dict) else {}


def check_workstream(workstream: str) -> None:
    if not WORKSTREAM.fullmatch(workstream):
        print(f"loopx guard: {workstream!r} is not a workstream id like w2", file=sys.stderr)
        sys.exit(2)


def cmd_start(workstream: str, listen: bool) -> None:
    check_workstream(workstream)
    sweep()
    # A new run starts a fresh idle window and reviews every blocked item once.
    for stale in (wait_path(workstream), board_state_path(workstream)):
        try:
            stale.unlink()
        except OSError:
            pass
    path = calling_session()
    if path is None:
        print(
            "loopx guard not armed: no Claude Code session id here, so no Stop hook "
            "can hold this loop. Follow the skill's exit rules."
        )
        return
    others = [
        p.stem
        for p in state_dir().glob("*.json")
        if p != path and read_json(p).get("workstream") == workstream
    ] if state_dir().is_dir() else []
    if others:
        print(
            f"loopx guard warning: another session ({', '.join(sorted(others))}) has a "
            f"loop armed on {workstream}; two loops on one queue will race for the "
            "same items."
        )
    write_json(
        path,
        {
            "workstream": workstream,
            "started": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
            "listen": listen,
            "blocks": 0,
            "remaining": None,
        },
    )
    mode = "listens when drained" if listen else "ends when drained (--no-listen)"
    print(f"loopx guard armed for {workstream} in this session only; it {mode}")


def cmd_end() -> None:
    sweep()
    path = calling_session()
    if path is not None and path.is_file():
        workstream = read_json(path).get("workstream")
        path.unlink()
        if isinstance(workstream, str) and WORKSTREAM.fullmatch(workstream):
            try:
                wait_path(workstream).unlink()
            except OSError:
                pass
        print("loopx guard cleared for this session")
    else:
        print("loopx guard was not armed in this session")


def cmd_forget() -> None:
    path = mark_path(read_payload().get("session_id"))
    allow(path)


def cmd_dispatch(label: str) -> None:
    path = calling_session()
    if path is None or not path.is_file():
        print("loopx guard: no loop armed in this session; nothing marked in flight")
        return
    state = read_json(path)
    found = ITEM_LABEL.fullmatch(label)
    # Marking the same worker again mid-run keeps its baseline: by now the tree
    # holds that worker's own changes, and they must still count as leftovers.
    again = same_item((state.get("inflight") or {}).get("label"), label)
    state["inflight"] = {
        "label": label,
        "since": time.time(),
        # Only an item that is open now can later be seen leaving the open tree.
        "was_open": bool(found) and item_open_locally(*found.groups()),
    }
    if not again:
        state["baseline"] = {"label": label, "dirty": sorted(dirty_paths() or [])}
    if found:
        hand_over(*found.groups())
    state["blocks"] = 0
    write_json(path, state)
    print(f"loopx guard: {label} in flight; launch its worker, then end the turn")


def cmd_settle() -> None:
    path = calling_session()
    if path is None or not path.is_file():
        print("loopx guard: no loop armed in this session")
        return
    state = read_json(path)
    label = (state.pop("inflight", None) or {}).get("label")
    write_json(path, state)
    print(f"loopx guard: settled {label}" if label else "loopx guard: nothing was in flight")


def cmd_routine_due(workstream: str) -> None:
    check_workstream(workstream)
    print(json.dumps(due_routine(workstream)))


def cmd_routine_done(workstream: str, name: str, result: str) -> None:
    check_workstream(workstream)
    if not ROUTINE_NAME.fullmatch(name) or result not in ("found", "empty", "failed"):
        print("loopx guard: usage: routine-done <wN> <name> found|empty|failed", file=sys.stderr)
        sys.exit(2)
    shown = git("show", f"{REMOTE_REF}:.pm/{workstream}/routine/README.md")
    routines, _, _ = parse_routines(shown.stdout) if shown.returncode == 0 else ([], 0, [])
    row = next((r for r in routines if r["name"] == name), None)
    if row is None:
        print(f"loopx guard: {name} is not in .pm/{workstream}/routine/README.md", file=sys.stderr)
        sys.exit(2)

    base = row["every"]
    path = routine_state_path(workstream)
    state = read_json(path)
    now = time.time()
    entries = state.get("routines") if isinstance(state.get("routines"), dict) else {}
    entry = entries.get(name, {})
    previous = entry.get("interval") if isinstance(entry.get("interval"), (int, float)) else base
    failures = int(entry.get("failures", 0)) + 1 if result == "failed" else 0
    if result == "failed":
        # The run never really happened: keep the cadence, and the last SHA a
        # `changed` row is gated on. Three in a row drops it to one try a day.
        interval = base if row["when"] == "always" else max(previous, base)
        if failures >= ROUTINE_FAILURES_BEFORE_PAUSE:
            interval = max(interval, ROUTINE_PAUSED_INTERVAL)
        sha = entry.get("last_sha", "")
        empty_runs = int(entry.get("empty_runs", 0))
    else:
        if row["when"] == "always" or result == "found":
            interval = base
        else:
            interval = min(max(previous, base) * 2, ROUTINE_MAX_INTERVAL)
        sha = git("rev-parse", REMOTE_REF).stdout.strip()
        empty_runs = 0 if result == "found" else int(entry.get("empty_runs", 0)) + 1
    entries[name] = {
        "last_run": now,
        "last_sha": sha,
        "interval": interval,
        "empty_runs": empty_runs,
        "failures": failures,
    }
    runs = [t for t in state.get("runs", []) if isinstance(t, (int, float)) and now - t < 86400]
    write_json(path, {"routines": entries, "runs": [*runs, now]})
    settle_label(f"routine:{name}")
    if result == "failed" and failures >= ROUTINE_FAILURES_BEFORE_PAUSE:
        print(
            f"loopx guard: {name} failed {failures} runs in a row; it now retries once every "
            f"{interval / 3600:.0f} h until a run succeeds. "
            + ("File one inbox note for it with /pm add." if failures == ROUTINE_FAILURES_BEFORE_PAUSE else "")
        )
        return
    if row["when"] == "always" or result == "failed":
        gate = ""
    else:
        gate = f" and only after {', '.join(row['paths']) or 'the repository'} changes"
    print(f"loopx guard: {name} recorded as {result}; next eligible in {interval / 3600:.1f} h{gate}")


def cmd_wait(workstream: str, options: dict[str, float]) -> None:
    check_workstream(workstream)
    floor, cap = options["min"], max(options["max"], options["min"])
    give_up, chunk = options["give-up"], options["chunk"]
    fetch_give_up = options["fetch-give-up"]

    state_file = wait_path(workstream)
    state = read_json(state_file)
    if state.get("pid") != os.getpid() and listener_alive(state):
        print(
            f"loopx wait: a listener (pid {state.get('pid')}) is already running for "
            f"{workstream} and will wake the loop. Do not start another; end the turn."
        )
        sys.exit(EXIT_BUSY)
    now = time.time()  # wall clock: a sleeping laptop still ages the idle window
    since = state.get("since") if isinstance(state.get("since"), (int, float)) else now
    state = {"since": since, "pid": os.getpid()}
    write_json(state_file, state)

    def finish(code: int, message: str, **extra: object) -> None:
        latest = read_json(state_file)
        latest.pop("pid", None)
        latest.update(extra)
        if code in (EXIT_WORK, EXIT_ROUTINE, EXIT_CI):
            latest.pop("since", None)
        write_json(state_file, latest)
        print(message)
        sys.exit(code)

    def wake_for_items(where: str, milestones: list[str], notes: list[str]) -> None:
        if milestones or notes:
            finish(
                EXIT_WORK,
                f"loopx wait: {workstream} has {len(milestones) + len(notes)} actionable item(s) {where}; "
                f"next pick {next_pick(workstream, milestones, notes)}. Run `{PULL}`, then pick it.",
            )

    started = time.time()
    delay = floor
    failing_since = None
    while True:
        routine_in = None
        watch_ci = False
        gate_in = None
        tree = Tree(REMOTE_REF) if git("fetch", "-q", "origin", "main").returncode == 0 else None
        facts = board(tree, workstream) if tree is not None and tree.ok else None
        if facts is None:
            # Offline: keep backing off, and do not pace on a stale routine schedule.
            failing_since = failing_since or time.time()
            down = time.time() - failing_since
            if down >= fetch_give_up:
                finish(
                    EXIT_FETCH,
                    f"loopx wait: git fetch origin main has failed for {down / 60:.0f} min "
                    "straight; check network and push credentials.",
                )
        else:
            failing_since = None
            ci = own_ci(workstream)
            if ci["own_failures"]:
                first = ci["own_failures"][0]
                finish(
                    EXIT_CI,
                    f"loopx wait: one of this loop's pushes turned CI red: {first['workflow']} "
                    f"since {first['since']} ({first['url']}). Run `{PULL}`, then "
                    f"`board {workstream}`, and fix it before anything else.",
                )
            watch_ci = ci["watch"]
            wake_for_items(f"on {REMOTE_REF}", facts["milestones"], facts["notes"])
            gates = gate_facts(tree, workstream)
            if gates["ready"]:
                first = f"{workstream}/{gates['ready'][0]}"
                finish(
                    EXIT_WORK,
                    f"loopx wait: the gate of parked item {first} cleared on {REMOTE_REF}"
                    + (f" (production pin {gates['pin']})" if gates.get("pin") else "")
                    + f". Run `{PULL}`, then `board {workstream}` and dispatch it.",
                )
            gate_in = gates["next_in"]
        # Sessions share this checkout: an item a teammate filed here and has
        # not pushed is board work too, online or not.
        wake_for_items(f"in this checkout that {REMOTE_REF} does not list", *actionable(workstream))
        if facts is not None:
            # Board work was checked first and always wins; only then a routine.
            schedule = due_routine(workstream)
            due = schedule["due"]
            routine_in = schedule.get("next_in")
            if due:
                finish(
                    EXIT_ROUTINE,
                    f"loopx wait: routine due for {workstream}: {due['name']} ({due['command']}). "
                    f"Run `{PULL}`, recheck the board, then dispatch it.",
                )

        now = time.time()
        idle = now - since
        if idle >= give_up and routine_in is None and failing_since is None:
            finish(
                EXIT_GAVE_UP,
                f"loopx wait: {workstream} stayed idle for {idle / 3600:.1f} h; "
                "the listen window is over.",
                gave_up=True,
            )
        spent = now - started
        if chunk > 0 and spent >= chunk:
            finish(
                EXIT_CHUNK,
                f"loopx wait: no work for {workstream} after this {chunk:.0f} s chunk "
                f"(idle {idle / 60:.0f} min); run wait again.",
            )

        pause = delay * random.uniform(0.8, 1.2)
        if routine_in is not None:
            # Never sleep past a routine's due time; the backoff only paces the board poll.
            pause = min(pause, routine_in + 1.0)
        elif failing_since is None:
            pause = min(pause, give_up - idle)
        else:
            pause = min(pause, fetch_give_up - (now - failing_since))
        if gate_in is not None:
            pause = min(pause, gate_in + 1.0)  # a date gate clears then
        if watch_ci:
            pause = min(pause, CI_WATCH_PAUSE)  # a pushed commit's CI is still running
        if chunk > 0:
            pause = min(pause, chunk - spent)
        time.sleep(max(pause, 0.0))
        delay = min(delay * 2, cap)


def cmd_outcome(workstream: str, item: str, shas: list[str]) -> None:
    check_workstream(workstream)
    pseudo = bool(PSEUDO_ID.fullmatch(item))
    if not pseudo and not ITEM_ID.fullmatch(item):
        print(f"loopx guard: {item!r} is not an item id like m12 or 089, or routine:<name>, or ci:<slug>", file=sys.stderr)
        sys.exit(2)
    if any(not COMMIT.fullmatch(sha) for sha in shas):
        print("loopx guard: commits must be 7-40 hex digits", file=sys.stderr)
        sys.exit(2)
    fetched = git("fetch", "-q", "origin", "main").returncode == 0

    where = "n/a"
    if not pseudo:
        stem = item.removesuffix(".md")

        def matches(name: str) -> bool:
            return name == stem or name.removesuffix(".md") == stem or name.startswith(f"{stem}-")

        tree = Tree(REMOTE_REF)
        where = "absent"
        for place, directory in (
            ("open", f".pm/{workstream}"),
            ("done", f".pm/{workstream}/done"),
            ("blocked", f".pm/{workstream}/blocked"),
        ):
            if any(matches(name) for name, _ in tree.children(directory)):
                where = place
                break

    missing = [sha for sha in shas if git("merge-base", "--is-ancestor", sha, REMOTE_REF).returncode != 0]
    head = git("rev-parse", "HEAD").stdout.strip()
    # "Pushed" means nothing local is missing from origin/main. Another loop
    # pushing in the meantime moves origin/main ahead, and that is fine.
    pushed = bool(head) and nothing_unpushed()
    label = item if pseudo else f"{workstream}/{item}"
    mark = calling_session()
    left = leftovers(dirty_at_dispatch(read_json(mark) if mark else {}, label))
    clean = left == []
    # An item a teammate filed here and nobody committed is absent on
    # origin/main while still open in this checkout: that is not an outcome.
    open_here = not pseudo and item_open_locally(workstream, item)
    ok = fetched and pushed and clean and not missing and where != "open" and not open_here
    if shas and not missing:
        record_pushes(workstream, shas)
    print(
        json.dumps(
            {
                "item": label,
                "ok": ok,
                "where": where,
                "open_here": open_here,
                "fetched": fetched,
                "pushed": pushed,
                "clean": clean,
                "leftover": (left or [])[:LEFTOVER_SHOWN],
                "leftover_count": len(left or []),
                "missing_commits": missing,
                "head": head[:12],
                "settled": settle_label(label) if ok else False,
            }
        )
    )


def cmd_board(workstream: str) -> None:
    check_workstream(workstream)
    tree = Tree()
    facts = board(tree, workstream)
    ci = own_ci(workstream, mark=True)
    gates = gate_facts(tree, workstream)
    review, unchanged = blocked_review(workstream, gates["ready"])
    schedule = due_routine(workstream)
    due = schedule["due"]

    if ci["own_failures"]:
        nxt = ci["own_failures"][0]["label"]
    elif gates["ready"]:
        nxt = f"{workstream}/{gates['ready'][0]}"
    elif facts["milestones"] or facts["notes"]:
        nxt = next_pick(workstream, facts["milestones"], facts["notes"])
    elif due:
        nxt = f"routine:{due['name']}"
    else:
        nxt = None
    routine = {"due": due, "next_in": schedule.get("next_in")}
    if schedule.get("problems"):
        routine["problems"] = schedule["problems"]
    print(
        json.dumps(
            {
                "workstream": workstream,
                "next": nxt,
                "ci": {k: ci[k] for k in ("checked", "own_failures", "still_red", "pending") if k in ci}
                | ({"reason": ci["reason"]} if "reason" in ci else {}),
                "ready": [*facts["milestones"], *facts["notes"]],
                "waiting": facts["waiting"],
                "blocked": {"ready": gates["ready"]}
                | {k: gates[k] for k in ("pin", "problems") if k in gates}
                | {"review": review, "unchanged": unchanged},
                "routine": routine,
            }
        )
    )


def cmd_hook() -> None:
    path = mark_path(read_payload().get("session_id"))
    if path is None or not path.is_file():
        allow()  # this session runs no loop

    try:
        state = json.loads(path.read_text())
        workstream = state["workstream"]
        if not WORKSTREAM.fullmatch(workstream):
            raise ValueError(workstream)
    except Exception:
        allow(path)

    note = ""
    inflight = state.get("inflight")
    if isinstance(inflight, dict):
        verdict = inflight_verdict(inflight, dirty_at_dispatch(state, inflight.get("label")))
        if verdict == "running":
            state["blocks"] = 0
            write_json(path, state)
            print("{}")  # a background worker is running; its notification wakes the loop
            sys.exit(0)
        label = inflight.get("label", "the worker")
        state.pop("inflight", None)
        if verdict == "finished":
            note = (
                f"The worker for {label} is finished: its item left the open tree and "
                "everything is pushed. Verify it with `outcome` if you have not, then continue. "
            )
        else:
            note = (
                f"The in-flight mark for {label} ran out without a finished outcome. Check "
                "on that worker before anything else: verify with `outcome`, or dispatch again. "
            )

    listen = state.get("listen", True) is not False
    milestones, notes = actionable(workstream)
    remaining = len(milestones) + len(notes)

    if remaining == 0:
        if not listen:
            allow(path)  # genuinely Done, or only waiting and blocked work is left
        listener = read_json(wait_path(workstream))
        if listener.get("gave_up"):
            try:
                wait_path(workstream).unlink()
            except OSError:
                pass
            allow(path)  # the listen window ran out: Done
        if listener_alive(listener):
            state["blocks"] = 0
            state["remaining"] = 0
            write_json(path, state)
            print("{}")  # listening: the background wait wakes this session
            sys.exit(0)

    # Reset the spin counter whenever the queue actually got shorter.
    previous = state.get("remaining")
    blocks = 0 if previous is None or remaining < previous else int(state.get("blocks", 0))
    blocks += 1
    if blocks > MAX_BLOCKS_WITHOUT_PROGRESS:
        # Something is wrong beyond a missed pick; let the turn end and say so.
        print(
            json.dumps(
                {
                    "systemMessage": (
                        f"loopx guard stood down after {blocks} stop attempts with no "
                        f"progress on {workstream}. Clear it with "
                        "`python3 scripts/loopx-guard.py end`."
                    )
                }
            )
        )
        sys.exit(0)

    state["blocks"] = blocks
    state["remaining"] = remaining
    write_json(path, state)

    if remaining == 0:
        reason = (
            f"/loopx {workstream} has nothing ready on the board and is not listening. "
            f"Run `python3 scripts/loopx-guard.py board {workstream}` and follow its `next`: "
            "an own CI failure or a due routine comes first. If it names nothing, write the "
            f"drained report, start `python3 scripts/loopx-guard.py wait {workstream}` as a "
            "background command, and end the turn; it wakes this session when work lands. "
            "If the loop genuinely ends, run `python3 scripts/loopx-guard.py end`."
        )
    else:
        nxt = next_pick(workstream, milestones, notes)
        reason = (
            f"/loopx {workstream} is still running: {remaining} item(s) are ready "
            f"and the next pick is {nxt}. This is not one of the three exits, and a "
            f"progress summary is not an exit. If a worker for {nxt} is already running, "
            f"run `python3 scripts/loopx-guard.py dispatch {nxt}` and end the turn; never "
            "launch a second worker for the same item. Otherwise mark it with `dispatch`, "
            "launch its worker, and end the turn. An item that cannot proceed is parked "
            "in blocked/ by its worker with its gate written down; when the loop "
            "genuinely ends, run `python3 scripts/loopx-guard.py end`."
        )
    print(json.dumps({"decision": "block", "reason": note + reason}))


def parse_wait_options(args: list[str]) -> dict[str, float]:
    options = {
        "min": WAIT_MIN,
        "max": WAIT_MAX,
        "give-up": WAIT_GIVE_UP,
        "chunk": 0.0,
        "fetch-give-up": WAIT_FETCH_GIVE_UP,
    }
    it = iter(args)
    for flag in it:
        key = flag.removeprefix("--")
        if not flag.startswith("--") or key not in options:
            raise ValueError(flag)
        value = float(next(it))
        if value < 0:
            raise ValueError(flag)
        options[key] = value
    return options


def usage() -> None:
    print(__doc__, file=sys.stderr)
    sys.exit(2)


def main() -> None:
    args = sys.argv[1:]
    if not args:
        cmd_hook()
    elif args[0] == "start" and len(args) == 2:
        cmd_start(args[1], listen=True)
    elif args[0] == "start" and len(args) == 3 and args[2] == "--no-listen":
        cmd_start(args[1], listen=False)
    elif args == ["end"]:
        cmd_end()
    elif args == ["forget"]:
        cmd_forget()
    elif args[0] == "wait" and len(args) >= 2:
        try:
            options = parse_wait_options(args[2:])
        except (ValueError, StopIteration):
            usage()
        cmd_wait(args[1], options)
    elif args[0] == "outcome" and len(args) >= 3:
        cmd_outcome(args[1], args[2], args[3:])
    elif args[0] == "board" and len(args) == 2:
        cmd_board(args[1])
    elif args[0] == "dispatch" and len(args) == 2:
        cmd_dispatch(args[1])
    elif args == ["settle"]:
        cmd_settle()
    elif args[0] == "routine-due" and len(args) == 2:
        cmd_routine_due(args[1])
    elif args[0] == "routine-done" and len(args) == 4:
        cmd_routine_done(*args[1:])
    else:
        usage()


if __name__ == "__main__":
    main()
