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
"""Tests for loopx-guard.py, the facts and the clock behind `/loopx`.

Every case runs the real script as a subprocess, the way Claude Code runs it:
`start`/`end` through a Bash tool call that carries CLAUDE_CODE_SESSION_ID, and
the Stop and SessionEnd hooks with a JSON payload on stdin. The cases that
read origin/main build a real bare remote in a temporary directory; `gh` is a
fake on PATH. Nothing here touches the network or this repository's board.

    python3 -B -m unittest scripts/test_loopx_guard.py -v
"""

from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
import unittest

GUARD = pathlib.Path(__file__).resolve().parent / "loopx-guard.py"
A, B = "session-a", "session-b"


def reap(proc: subprocess.Popen) -> None:
    """Stop a background guard process and close its pipes."""
    proc.kill()
    proc.communicate()


class GuardFixture(unittest.TestCase):
    """A checkout with a w2 and a w5 board; no remote."""

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.repo = pathlib.Path(self._tmp.name)
        (self.repo / ".git").mkdir()
        self.board("w2", milestones=["m53"], notes=["089.md"])
        self.board("w5", notes=["779.md"])

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def board(self, workstream: str, milestones=(), notes=()) -> None:
        root = self.repo / ".pm" / workstream
        root.mkdir(parents=True, exist_ok=True)
        (root / "README.md").write_text(f"# {workstream}\n")
        for name in milestones:
            (root / name).mkdir()
        for name in notes:
            (root / name).write_text("note\n")

    def run_guard(self, *args: str, session: str | None = None, payload: dict | None = None):
        env = {k: v for k, v in os.environ.items() if k != "CLAUDE_CODE_SESSION_ID"}
        env["CLAUDE_PROJECT_DIR"] = str(self.repo)
        if session is not None:
            env["CLAUDE_CODE_SESSION_ID"] = session
        return subprocess.run(
            [sys.executable, str(GUARD), *args],
            input=json.dumps(payload) if payload is not None else "",
            capture_output=True,
            text=True,
            env=env,
            check=False,
        )

    def stop(self, session: str) -> dict:
        result = self.run_guard(payload={"session_id": session, "hook_event_name": "Stop"})
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def marks(self) -> list[str]:
        directory = self.repo / ".git" / "loopx"
        return sorted(p.stem for p in directory.glob("*.json")) if directory.is_dir() else []

    def mark(self, session: str) -> dict:
        return json.loads((self.repo / ".git" / "loopx" / f"{session}.json").read_text())

    def save_mark(self, session: str, state: dict) -> None:
        (self.repo / ".git" / "loopx" / f"{session}.json").write_text(json.dumps(state))

    def task(self, rel: str, status: str = "todo", depends_on=()) -> None:
        """Write a task file such as `w2/m60/t001.md` with real frontmatter."""
        path = self.repo / ".pm" / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(task_text(rel, status, depends_on))

    def fake_listener(self) -> int:
        """A live process whose command line reads like a running `wait`."""
        proc = subprocess.Popen(
            [sys.executable, "-c", "import time; time.sleep(120)", "loopx-guard.py", "wait", "w2"]
        )
        self.addCleanup(lambda: (proc.kill(), proc.wait()))
        return proc.pid


def task_text(rel: str, status: str = "todo", depends_on=()) -> str:
    task_id = rel.removesuffix(".md").replace("/done/", "/").replace("/blocked/", "/")
    return (
        f"---\nid: {task_id}\ntitle: \"a task\"\nworker: worker1\nstatus: {status}\n"
        f"estimate: 30m\ndepends_on: [{', '.join(depends_on)}]\n---\n\n## Objective\n\nstatus: not frontmatter\n"
    )


class GuardTest(GuardFixture):

    def test_start_binds_to_the_calling_session_and_blocks_its_stop(self) -> None:
        self.assertIn("armed for w2", self.run_guard("start", "w2", session=A).stdout)
        decision = self.stop(A)
        self.assertEqual(decision["decision"], "block")
        self.assertIn("the next pick is w2/m53", decision["reason"])

    def test_another_session_in_the_same_checkout_is_never_blocked(self) -> None:
        # The bug this fixes: B stopped first and was held to A's loop.
        self.run_guard("start", "w2", session=A)
        self.assertEqual(self.stop(B), {})
        self.assertEqual(self.marks(), [A])
        self.assertEqual(self.stop(A)["decision"], "block")

    def test_two_sessions_run_independent_loops(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("start", "w5", session=B)
        self.assertIn("w2/m53", self.stop(A)["reason"])
        self.assertIn("w5/779.md", self.stop(B)["reason"])

    def test_start_outside_claude_code_arms_nothing(self) -> None:
        result = self.run_guard("start", "w2")
        self.assertEqual(result.returncode, 0)
        self.assertIn("not armed", result.stdout)
        self.assertEqual(self.marks(), [])
        self.assertEqual(self.stop(A), {})

    def test_end_clears_only_the_calling_session(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("start", "w5", session=B)
        self.assertIn("cleared", self.run_guard("end", session=A).stdout)
        self.assertEqual(self.stop(A), {})
        self.assertEqual(self.marks(), [B])
        self.assertIn("was not armed", self.run_guard("end", session=A).stdout)

    def test_session_end_hook_forgets_that_session(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("start", "w5", session=B)
        result = self.run_guard("forget", payload={"session_id": A, "hook_event_name": "SessionEnd"})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.marks(), [B])

    def drain_w2(self) -> None:
        (self.repo / ".pm" / "w2" / "m53").rmdir()
        (self.repo / ".pm" / "w2" / "089.md").unlink()

    def write_listener(self, workstream: str, **state) -> None:
        directory = self.repo / ".git" / "loopx"
        directory.mkdir(exist_ok=True)
        (directory / f"wait-{workstream}.state").write_text(json.dumps(state))

    def test_drained_no_listen_workstream_lets_the_stop_through_and_clears_the_mark(self) -> None:
        self.assertIn("ends when drained", self.run_guard("start", "w2", "--no-listen", session=A).stdout)
        self.drain_w2()
        self.assertEqual(self.stop(A), {})
        self.assertEqual(self.marks(), [])

    def test_drained_listening_loop_is_held_until_a_wait_runs(self) -> None:
        self.assertIn("listens when drained", self.run_guard("start", "w2", session=A).stdout)
        self.drain_w2()
        decision = self.stop(A)
        self.assertEqual(decision["decision"], "block")
        self.assertIn("loopx-guard.py wait w2", decision["reason"])
        self.assertEqual(self.marks(), [A])

    def test_live_wait_lets_the_stop_through_and_keeps_the_mark(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.drain_w2()
        self.write_listener("w2", since=time.time(), pid=self.fake_listener())
        for _ in range(30):  # never counted toward the stand-down limit
            self.assertEqual(self.stop(A), {})
        self.assertEqual(self.marks(), [A])

    def test_a_live_pid_that_is_not_a_listener_does_not_count(self) -> None:
        # After a reboot the recorded pid belongs to some other process.
        self.run_guard("start", "w2", session=A)
        self.drain_w2()
        self.write_listener("w2", since=time.time(), pid=os.getpid())
        self.assertEqual(self.stop(A)["decision"], "block")

    def test_dead_wait_pid_does_not_count_as_listening(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.drain_w2()
        dead = subprocess.Popen([sys.executable, "-c", "pass"])
        dead.wait()
        self.write_listener("w2", since=time.time(), pid=dead.pid)
        self.assertEqual(self.stop(A)["decision"], "block")

    def test_expired_listen_window_ends_the_loop(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.drain_w2()
        self.write_listener("w2", since=0, gave_up=True)
        self.assertEqual(self.stop(A), {})
        self.assertEqual(self.marks(), [])
        self.assertFalse((self.repo / ".git" / "loopx" / "wait-w2.state").exists())

    def test_start_resets_the_idle_window(self) -> None:
        self.write_listener("w2", since=0, gave_up=True)
        self.run_guard("start", "w2", session=A)
        self.assertFalse((self.repo / ".git" / "loopx" / "wait-w2.state").exists())

    def test_start_warns_when_another_session_runs_the_same_workstream(self) -> None:
        self.run_guard("start", "w2", session=B)
        result = self.run_guard("start", "w2", session=A)
        self.assertIn("warning", result.stdout)
        self.assertIn(B, result.stdout)
        self.assertNotIn("warning", self.run_guard("start", "w5", session=A).stdout)

    def test_stands_down_after_repeated_stops_without_progress(self) -> None:
        self.run_guard("start", "w2", session=A)
        for _ in range(25):
            self.assertEqual(self.stop(A)["decision"], "block")
        final = self.stop(A)
        self.assertNotIn("decision", final)
        self.assertIn("stood down", final["systemMessage"])

    def test_progress_resets_the_spin_counter(self) -> None:
        self.run_guard("start", "w2", session=A)
        for _ in range(20):
            self.stop(A)
        (self.repo / ".pm" / "w2" / "m53").rmdir()  # one item shipped
        for _ in range(20):
            self.assertEqual(self.stop(A)["decision"], "block")

    def test_unsafe_session_ids_never_touch_the_filesystem(self) -> None:
        self.assertIn("not armed", self.run_guard("start", "w2", session="../../escape").stdout)
        self.assertEqual(self.stop("../../escape"), {})
        self.assertFalse((self.repo / "escape.json").exists())
        self.assertEqual(self.marks(), [])

    def test_rejects_a_workstream_that_is_not_an_id(self) -> None:
        result = self.run_guard("start", "../w2", session=A)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(self.marks(), [])

    def test_unreadable_mark_is_cleared_rather_than_blocking(self) -> None:
        directory = self.repo / ".git" / "loopx"
        directory.mkdir()
        (directory / f"{A}.json").write_text("{not json")
        self.assertEqual(self.stop(A), {})
        self.assertEqual(self.marks(), [])

    def test_working_documents_and_harness_directories_are_not_items(self) -> None:
        # The bex board keeps w7/builder-issues.md, wN/dev-N/ and wN/routine/ beside real items.
        root = self.repo / ".pm" / "w2"
        (root / "builder-issues.md").write_text("a working document, not a numbered note\n")
        (root / "RESEARCH-workspaces.md").write_text("research\n")
        for name in ("dev-2", "routine", "mobile"):  # `mobile` starts with m and is no milestone
            (root / name).mkdir()
        (root / "routine" / "README.md").write_text("# w2 routines\n")
        self.run_guard("start", "w2", "--no-listen", session=A)
        self.assertIn("2 item(s) are ready", self.stop(A)["reason"])
        self.drain_w2()
        self.assertEqual(self.stop(A), {})  # only the documents are left: drained
        self.assertEqual(self.marks(), [])

    def test_start_sweeps_marks_left_by_crashed_sessions(self) -> None:
        self.run_guard("start", "w2", session=B)
        old = time.time() - 8 * 24 * 3600
        os.utime(self.repo / ".git" / "loopx" / f"{B}.json", (old, old))
        self.run_guard("start", "w5", session=A)
        self.assertEqual(self.marks(), [A])


class DependencyTest(GuardFixture):
    """A milestone whose tasks all wait on unfinished work is not actionable."""

    def setUp(self) -> None:
        super().setUp()
        (self.repo / ".pm" / "w2" / "m53").rmdir()
        (self.repo / ".pm" / "w2" / "089.md").unlink()
        self.task("w2/m60/t001.md", depends_on=["w5/m9/t003"])
        self.task("w2/m60/t002.md", depends_on=["w2/m60/t001"])
        self.task("w5/m9/t003.md")

    def test_a_waiting_milestone_does_not_hold_the_loop(self) -> None:
        self.run_guard("start", "w2", "--no-listen", session=A)
        self.assertEqual(self.stop(A), {})  # nothing ready: with --no-listen that is an exit
        self.assertEqual(self.marks(), [])

    def test_a_waiting_milestone_still_needs_a_listener(self) -> None:
        self.run_guard("start", "w2", session=A)
        reason = self.stop(A)["reason"]
        self.assertIn("nothing ready", reason)
        self.assertNotIn("next pick", reason)

    def test_it_becomes_actionable_when_the_work_it_names_is_done(self) -> None:
        self.run_guard("start", "w2", session=A)
        done = self.repo / ".pm" / "w5" / "m9" / "done"
        done.mkdir()
        (self.repo / ".pm" / "w5" / "m9" / "t003.md").rename(done / "t003.md")
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_a_dependency_under_blocked_is_unfinished(self) -> None:
        self.run_guard("start", "w2", "--no-listen", session=A)
        blocked = self.repo / ".pm" / "w5" / "blocked"
        blocked.mkdir()
        (self.repo / ".pm" / "w5" / "m9").rename(blocked / "m9")
        self.assertEqual(self.stop(A), {})

    def test_a_dependency_that_names_nothing_holds_nothing_back(self) -> None:
        self.run_guard("start", "w2", session=A)
        (self.repo / ".pm" / "w5" / "m9" / "t003.md").unlink()  # dropped
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_milestone_and_note_dependencies_count_too(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.task("w2/m60/t001.md", depends_on=["w5/779", '"w5/m9"'])
        self.task("w5/m9/done/t003.md", status="done")
        (self.repo / ".pm" / "w5" / "m9" / "t003.md").unlink()
        self.assertIn("nothing ready", self.stop(A)["reason"])  # note w5/779 and milestone w5/m9 are open
        (self.repo / ".pm" / "w5" / "779.md").unlink()
        self.assertIn("nothing ready", self.stop(A)["reason"])  # the milestone directory is still open
        done = self.repo / ".pm" / "w5" / "done"
        done.mkdir()
        (self.repo / ".pm" / "w5" / "m9").rename(done / "m9")
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_one_workable_task_makes_the_milestone_actionable(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.task("w2/m60/t003.md", status="blocked")
        self.assertIn("nothing ready", self.stop(A)["reason"])
        self.task("w2/m60/t004.md")  # no dependencies
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_a_milestone_with_only_finished_tasks_needs_its_closeout(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.task("w2/m60/t001.md", status="done", depends_on=["w5/m9/t003"])
        self.task("w2/m60/t002.md", status="done")
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_a_dependency_list_prettier_wrapped_over_lines_is_read(self) -> None:
        self.run_guard("start", "w2", session=A)
        (self.repo / ".pm" / "w2" / "m60" / "t001.md").write_text(
            "---\nid: w2/m60/t001\ntitle: a task\nworker: worker1\nstatus: todo\nestimate: 30m\n"
            "depends_on:\n  [\n    w5/m9/t003,\n    w5/779,\n  ]\n---\n\n## Objective\n"
        )
        self.assertIn("nothing ready", self.stop(A)["reason"])
        (self.repo / ".pm" / "w5" / "m9" / "t003.md").unlink()
        self.assertIn("nothing ready", self.stop(A)["reason"])  # w5/779 is still open
        (self.repo / ".pm" / "w5" / "779.md").unlink()
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_a_comment_on_one_dependency_row_does_not_hide_the_rows_after_it(self) -> None:
        self.run_guard("start", "w2", session=A)
        head = "---\nid: w2/m60/t001\ntitle: a task\nworker: worker1\nstatus: todo\nestimate: 30m\ndepends_on:\n"
        for rows in ("  - w5/m9/t003 # the schema\n  - w5/779\n", "  [\n    w5/m9/t003, # the schema\n    w5/779,\n  ]\n"):
            with self.subTest(rows=rows):
                (self.repo / ".pm" / "w5" / "m9" / "t003.md").unlink(missing_ok=True)
                (self.repo / ".pm" / "w2" / "m60" / "t001.md").write_text(f"{head}{rows}---\n\n## Objective\n")
                self.assertIn("nothing ready", self.stop(A)["reason"])  # w5/779 is still open

    def test_a_dependency_finished_where_it_stands_is_met(self) -> None:
        # A task closed in a parked milestone keeps its file: `status: done`, not moved to done/.
        self.run_guard("start", "w2", session=A)
        self.task("w2/m60/t001.md", depends_on=["w5/m9/t003"])
        self.assertIn("nothing ready", self.stop(A)["reason"])
        self.task("w5/m9/t003.md", status="done")
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_a_status_comment_is_not_part_of_the_status(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.task("w2/m60/t001.md", status="blocked # waits on the production deploy")
        self.assertIn("nothing ready", self.stop(A)["reason"])  # t002 waits on it
        self.task("w2/m60/t001.md", status="deferred")
        self.assertIn("nothing ready", self.stop(A)["reason"])
        self.task("w2/m60/t001.md", status="in_progress # the live re-run is next")
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_superseded_and_moved_tasks_are_finished(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.task("w2/m60/t001.md", status="superseded", depends_on=["w5/m9/t003"])
        self.task("w2/m60/t002.md", status="moved", depends_on=["w5/m9/t003"])
        # No live task is left, so the milestone needs a worker to close it out.
        self.assertIn("the next pick is w2/m60", self.stop(A)["reason"])

    def test_picks_follow_numeric_order(self) -> None:
        self.board("w2", milestones=["m100", "m9"], notes=["1014.md", "089.md"])
        self.run_guard("start", "w2", session=A)
        self.assertIn("the next pick is w2/m9", self.stop(A)["reason"])
        (self.repo / ".pm" / "w2" / "m9").rmdir()
        (self.repo / ".pm" / "w2" / "m100").rmdir()
        self.assertIn("the next pick is w2/089.md", self.stop(A)["reason"])


GIT_ENV = {
    "GIT_AUTHOR_NAME": "loopx test",
    "GIT_AUTHOR_EMAIL": "loopx@example.invalid",
    "GIT_COMMITTER_NAME": "loopx test",
    "GIT_COMMITTER_EMAIL": "loopx@example.invalid",
    "GIT_CONFIG_GLOBAL": os.devnull,
    "GIT_CONFIG_NOSYSTEM": "1",
}
# Fast timings: polls every few ms instead of minutes.
FAST = ["--min", "0.02", "--max", "0.05"]


class RemoteFixture(unittest.TestCase):
    """A checkout with a real origin, and a second clone that pushes board changes."""

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        root = pathlib.Path(self._tmp.name)
        self.origin = root / "origin.git"
        self.repo = root / "work"  # the loop's checkout
        self.other = root / "other"  # another worker pushing board changes
        self.git(root, "init", "-q", "--bare", "-b", "main", str(self.origin))
        self.git(root, "clone", "-q", str(self.origin), str(self.other))
        self.write(".pm/w2/README.md", "# w2\n")
        self.write(".pm/w2/done/001.md", "done\n")
        self.push("seed")
        self.git(root, "clone", "-q", str(self.origin), str(self.repo))

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def git(self, cwd: pathlib.Path, *args: str) -> str:
        result = subprocess.run(
            ["git", *args], cwd=cwd, capture_output=True, text=True,
            env={**os.environ, **GIT_ENV}, check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout

    def write(self, rel: str, text: str) -> None:
        path = self.other / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def push(self, message: str) -> None:
        self.git(self.other, "add", "-A")
        self.git(self.other, "commit", "-q", "-m", message)
        self.git(self.other, "push", "-q", "origin", "HEAD:main")

    def guard_env(self, session: str | None = None) -> dict:
        env = {k: v for k, v in os.environ.items() if k != "CLAUDE_CODE_SESSION_ID"}
        env.update(GIT_ENV, CLAUDE_PROJECT_DIR=str(self.repo))
        env.update(getattr(self, "extra_env", {}))
        if session is not None:
            env["CLAUDE_CODE_SESSION_ID"] = session
        return env

    def run_guard(self, *args: str, timeout: float = 30, session: str | None = None, payload: dict | None = None):
        return subprocess.run(
            [sys.executable, str(GUARD), *args], capture_output=True, text=True,
            input=json.dumps(payload) if payload is not None else "",
            env=self.guard_env(session), timeout=timeout, check=False,
        )

    def stop(self, session: str) -> dict:
        result = self.run_guard(payload={"session_id": session, "hook_event_name": "Stop"})
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def mark(self, session: str) -> dict:
        return json.loads((self.repo / ".git" / "loopx" / f"{session}.json").read_text())

    def pull(self) -> None:
        self.git(self.repo, "pull", "-q", "--rebase", "origin", "main")

    def local_commit(self, rel: str, text: str, push: bool = True) -> str:
        """A commit made in the loop's own checkout, as a worker would."""
        path = self.repo / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        self.git(self.repo, "add", "-A")
        self.git(self.repo, "commit", "-q", "-m", f"change {rel}")
        if push:
            self.git(self.repo, "push", "-q", "origin", "HEAD:main")
        return self.git(self.repo, "rev-parse", "HEAD").strip()

    def listener(self) -> dict:
        path = self.repo / ".git" / "loopx" / "wait-w2.state"
        return json.loads(path.read_text()) if path.exists() else {}


class RemoteTest(RemoteFixture):
    """`wait` and `outcome` read origin/main, so these run against a real remote."""

    def test_wait_wakes_when_a_note_lands_on_origin(self) -> None:
        self.write(".pm/w2/007.md", "new note\n")
        self.push("add note")
        result = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("next pick w2/007.md", result.stdout)
        self.assertNotIn("since", self.listener())
        self.assertNotIn("pid", self.listener())
        # It only reports; the checkout is not touched until the loop pulls.
        self.assertFalse((self.repo / ".pm" / "w2" / "007.md").exists())

    def test_wait_wakes_while_running_and_prefers_milestones(self) -> None:
        proc = subprocess.Popen(
            [sys.executable, str(GUARD), "wait", "w2", *FAST, "--give-up", "20"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
            env={**os.environ, **GIT_ENV, "CLAUDE_PROJECT_DIR": str(self.repo)},
        )
        time.sleep(0.3)
        self.assertIsNone(proc.poll(), "wait exited before any work landed")
        self.assertEqual(self.listener().get("pid"), proc.pid)
        self.write(".pm/w2/009.md", "note\n")
        self.write(".pm/w2/m4/README.md", "milestone\n")
        self.push("add work")
        out, err = proc.communicate(timeout=20)
        self.assertEqual(proc.returncode, 0, err)
        self.assertIn("next pick w2/m4", out)

    def test_moves_into_done_or_blocked_do_not_wake(self) -> None:
        self.write(".pm/w2/done/002.md", "done\n")
        self.write(".pm/w2/blocked/m3/README.md", "blocked\n")
        self.write(".pm/w5/010.md", "other workstream\n")
        self.push("unrelated board moves")
        result = self.run_guard("wait", "w2", *FAST, "--chunk", "0.2")
        self.assertEqual(result.returncode, 10, result.stdout + result.stderr)

    def test_chunks_share_one_idle_window_until_it_expires(self) -> None:
        # The chunks use a window they cannot outlast, however slow the machine is.
        first = self.run_guard("wait", "w2", *FAST, "--chunk", "0.1", "--give-up", "600")
        self.assertEqual(first.returncode, 10, first.stderr)
        since = self.listener()["since"]
        self.assertNotIn("pid", self.listener())
        second = self.run_guard("wait", "w2", *FAST, "--chunk", "0.1", "--give-up", "600")
        self.assertEqual(second.returncode, 10, second.stderr)
        self.assertEqual(self.listener()["since"], since)
        time.sleep(0.3)
        final = self.run_guard("wait", "w2", *FAST, "--give-up", "0.2")  # measured from the first chunk
        self.assertEqual(final.returncode, 3, final.stderr)
        self.assertIn("listen window is over", final.stdout)
        self.assertTrue(self.listener()["gave_up"])

    def test_wait_stops_when_fetch_keeps_failing(self) -> None:
        self.git(self.repo, "remote", "set-url", "origin", str(self.origin) + "-missing")
        result = self.run_guard("wait", "w2", *FAST, "--fetch-give-up", "0.3")
        self.assertEqual(result.returncode, 4)
        self.assertIn("has failed for", result.stdout)

    def test_a_fetch_outage_shorter_than_the_limit_does_not_end_the_listener(self) -> None:
        self.git(self.repo, "remote", "set-url", "origin", str(self.origin) + "-missing")
        proc = subprocess.Popen(
            [sys.executable, str(GUARD), "wait", "w2", *FAST, "--fetch-give-up", "30"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=self.guard_env(),
        )
        self.addCleanup(reap, proc)
        time.sleep(0.6)  # dozens of failed polls: the old count-based limit would have quit
        self.assertIsNone(proc.poll(), "the listener gave up during a short outage")
        self.git(self.repo, "remote", "set-url", "origin", str(self.origin))
        self.write(".pm/w2/012.md", "note\n")
        self.push("work lands after the outage")
        out, err = proc.communicate(timeout=20)
        self.assertEqual(proc.returncode, 0, err)
        self.assertIn("next pick w2/012.md", out)

    def test_only_one_listener_runs_per_workstream(self) -> None:
        first = subprocess.Popen(
            [sys.executable, str(GUARD), "wait", "w2", *FAST, "--give-up", "30"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=self.guard_env(),
        )
        self.addCleanup(reap, first)
        time.sleep(0.4)
        second = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(second.returncode, 11, second.stdout + second.stderr)
        self.assertIn("already running", second.stdout)
        self.assertEqual(self.listener().get("pid"), first.pid)  # the first one's state is untouched
        self.assertIsNone(first.poll())

    def test_a_waiting_milestone_does_not_wake_the_listener_until_its_dependency_is_done(self) -> None:
        self.write(".pm/w2/m7/README.md", "milestone\n")
        self.write(".pm/w2/m7/t001.md", task_text("w2/m7/t001.md", depends_on=["w9/m1/t004"]))
        self.write(".pm/w9/m1/t004.md", task_text("w9/m1/t004.md"))
        self.push("a milestone that waits on another queue")
        idle = self.run_guard("wait", "w2", *FAST, "--chunk", "0.2")
        self.assertEqual(idle.returncode, 10, idle.stdout + idle.stderr)
        (self.other / ".pm/w9/m1/done").mkdir()
        self.git(self.other, "mv", ".pm/w9/m1/t004.md", ".pm/w9/m1/done/t004.md")
        self.push("the other queue finishes it")
        woke = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(woke.returncode, 0, woke.stdout + woke.stderr)
        self.assertIn("next pick w2/m7", woke.stdout)

    def test_wait_wakes_for_an_item_filed_in_this_checkout_and_not_pushed(self) -> None:
        # Sessions share the checkout: a teammate's uncommitted filing is board work.
        (self.repo / ".pm" / "w2" / "015.md").write_text("filed by a teammate, not committed\n")
        result = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("in this checkout", result.stdout)
        self.assertIn("next pick w2/015.md", result.stdout)

    def test_an_unpushed_filing_wakes_the_listener_even_offline(self) -> None:
        self.git(self.repo, "remote", "set-url", "origin", str(self.origin) + "-missing")
        (self.repo / ".pm" / "w2" / "015.md").write_text("filed by a teammate, not committed\n")
        result = self.run_guard("wait", "w2", *FAST, "--fetch-give-up", "30")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("next pick w2/015.md", result.stdout)

    def test_wait_rejects_bad_arguments(self) -> None:
        self.assertEqual(self.run_guard("wait", "../w2").returncode, 2)
        self.assertEqual(self.run_guard("wait", "w2", "--min").returncode, 2)
        self.assertEqual(self.run_guard("wait", "w2", "--bogus", "1").returncode, 2)

    def outcome(self, item: str) -> dict:
        result = self.run_guard("outcome", "w2", item)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_outcome_reports_where_an_item_sits_on_origin(self) -> None:
        self.write(".pm/w2/m4/README.md", "open\n")
        self.write(".pm/w2/blocked/m3/README.md", "blocked\n")
        self.write(".pm/w2/008-slugged-note.md", "open note\n")
        self.push("board")
        self.assertEqual(self.outcome("m4")["where"], "open")
        self.assertEqual(self.outcome("m3")["where"], "blocked")
        self.assertEqual(self.outcome("001")["where"], "done")
        self.assertEqual(self.outcome("001.md")["where"], "done")
        self.assertEqual(self.outcome("008")["where"], "open")
        self.assertEqual(self.outcome("m9")["where"], "absent")

    def test_outcome_reports_unpushed_and_dirty_checkouts(self) -> None:
        self.write(".pm/w2/m4/README.md", "open\n")
        self.push("board")
        behind = self.outcome("m4")
        self.assertTrue(behind["fetched"])
        # Another loop pushed and origin/main moved ahead: nothing here is unpushed.
        self.assertTrue(behind["pushed"])
        sha = self.local_commit("notes.txt", "local work\n", push=False)
        unpushed = self.outcome("001")
        self.assertFalse(unpushed["pushed"])
        self.assertFalse(unpushed["ok"])
        self.git(self.repo, "pull", "-q", "--rebase", "origin", "main")
        self.git(self.repo, "push", "-q", "origin", "HEAD:main")
        self.assertTrue(self.outcome("001")["pushed"])
        self.assertTrue(self.outcome("001")["ok"])
        self.assertNotEqual(sha, "")
        (self.repo / "scratch.txt").write_text("partial work\n")
        dirty = self.outcome("001")
        self.assertFalse(dirty["clean"])
        self.assertFalse(dirty["ok"])

    def test_outcome_checks_the_commits_a_worker_lists(self) -> None:
        sha = self.local_commit("notes.txt", "shipped\n")
        ok = json.loads(self.run_guard("outcome", "w2", "001", sha[:9]).stdout)
        self.assertTrue(ok["ok"])
        self.assertEqual(ok["missing_commits"], [])
        pushes = json.loads((self.repo / ".git" / "loopx" / "ci-w2.state").read_text())["pushes"]
        self.assertEqual([p["sha"] for p in pushes], [sha])  # remembered for the CI check
        unknown = json.loads(self.run_guard("outcome", "w2", "001", "deadbeef1").stdout)
        self.assertEqual(unknown["missing_commits"], ["deadbeef1"])
        self.assertFalse(unknown["ok"])
        self.assertEqual(self.run_guard("outcome", "w2", "001", "not-a-sha").returncode, 2)

    def test_an_item_still_open_is_never_ok(self) -> None:
        self.write(".pm/w2/m4/README.md", "open\n")
        self.push("board")
        self.pull()
        self.assertFalse(self.outcome("m4")["ok"])

    def test_outcome_verifies_workers_that_have_no_board_item(self) -> None:
        sha = self.local_commit("notes.txt", "routine fix\n")
        result = json.loads(self.run_guard("outcome", "w2", "routine:cli-qa", sha).stdout)
        self.assertEqual((result["item"], result["where"], result["ok"]), ("routine:cli-qa", "n/a", True))
        self.assertTrue(json.loads(self.run_guard("outcome", "w2", "ci:test-backend").stdout)["ok"])
        self.assertEqual(self.run_guard("outcome", "w2", "routine:../x").returncode, 2)

    def test_a_verified_outcome_clears_the_matching_inflight_mark(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "w2/001.md", session=A)
        other = json.loads(self.run_guard("outcome", "w2", "m9", session=A).stdout)
        self.assertFalse(other["settled"])  # a different item: the mark stays
        self.assertIn("inflight", self.mark(A))
        verified = json.loads(self.run_guard("outcome", "w2", "001", session=A).stdout)
        self.assertTrue(verified["ok"])
        self.assertTrue(verified["settled"])
        self.assertNotIn("inflight", self.mark(A))

    def test_outcome_rejects_unsafe_ids(self) -> None:
        self.assertEqual(self.run_guard("outcome", "w2", "../m4").returncode, 2)


class InflightTest(GuardFixture):
    """`dispatch` lets turns end while a background worker runs."""

    def test_inflight_worker_lets_stops_through_until_settled(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.assertIn("in flight", self.run_guard("dispatch", "w2/m53", session=A).stdout)
        for _ in range(30):  # never counted toward the stand-down limit
            self.assertEqual(self.stop(A), {})
        self.assertEqual(self.marks(), [A])
        self.assertIn("settled w2/m53", self.run_guard("settle", session=A).stdout)
        self.assertEqual(self.stop(A)["decision"], "block")

    def test_inflight_belongs_to_the_dispatching_session(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("start", "w5", session=B)
        self.run_guard("dispatch", "w2/m53", session=A)
        self.assertEqual(self.stop(A), {})
        self.assertEqual(self.stop(B)["decision"], "block")

    def test_a_lost_worker_expires_and_the_loop_is_held_again(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "w2/m53", session=A)
        mark = self.repo / ".git" / "loopx" / f"{A}.json"
        state = json.loads(mark.read_text())
        state["inflight"]["since"] = time.time() - 13 * 3600
        mark.write_text(json.dumps(state))
        self.assertEqual(self.stop(A)["decision"], "block")
        self.assertNotIn("inflight", json.loads(mark.read_text()))

    def test_dispatch_without_a_loop_is_a_no_op(self) -> None:
        self.assertIn("no loop armed", self.run_guard("dispatch", "w2/m53", session=A).stdout)
        self.assertIn("no loop armed", self.run_guard("settle", session=A).stdout)
        self.assertEqual(self.marks(), [])

    def test_the_hook_never_asks_for_a_second_worker(self) -> None:
        # Forgetting `dispatch` used to be answered with "dispatch it to a worker".
        self.run_guard("start", "w2", session=A)
        reason = self.stop(A)["reason"]
        self.assertIn("If a worker for w2/m53 is already running", reason)
        self.assertIn("never launch a second worker", reason)

    def test_a_worker_without_a_board_item_is_released_by_a_timer(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "routine:cli-qa", session=A)
        self.assertEqual(self.stop(A), {})
        state = self.mark(A)
        self.assertFalse(state["inflight"]["was_open"])
        state["inflight"]["since"] = time.time() - 4 * 3600
        self.save_mark(A, state)
        reason = self.stop(A)["reason"]
        self.assertIn("routine:cli-qa ran out", reason)
        self.assertNotIn("inflight", self.mark(A))

    def test_an_item_that_was_not_open_at_dispatch_gives_no_board_signal(self) -> None:
        # Reviving a blocked item: its worker's first step moves it into the open tree.
        blocked = self.repo / ".pm" / "w2" / "blocked"
        blocked.mkdir()
        (blocked / "113.md").write_text("blocked note\n")
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "w2/113", session=A)
        self.assertEqual(self.stop(A), {})  # not "left the open tree": it was never in it

    def test_an_item_gone_without_a_ship_is_lost_after_the_grace_period(self) -> None:
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "w2/089.md", session=A)
        (self.repo / ".pm" / "w2" / "089.md").unlink()  # the worker moved it, then went quiet
        self.assertEqual(self.stop(A), {})  # still inside the grace period
        state = self.mark(A)
        self.assertIn("gone_since", state["inflight"])
        state["inflight"]["gone_since"] = time.time() - 3600
        self.save_mark(A, state)
        self.assertIn("w2/089.md ran out", self.stop(A)["reason"])


class FinishedWorkerTest(RemoteFixture):
    """The Stop hook sees a worker finish from the board and git, not from `settle`."""

    def setUp(self) -> None:
        super().setUp()
        self.write(".pm/w2/007.md", "a note\n")
        self.write(".pm/w2/008.md", "another note\n")
        self.push("two notes")
        self.pull()
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "w2/007.md", session=A)

    def test_the_loop_is_held_again_once_the_item_is_shipped(self) -> None:
        self.assertEqual(self.stop(A), {})  # the item is still open: the worker is running
        (self.repo / ".pm/w2/done").mkdir(exist_ok=True)
        self.git(self.repo, "mv", ".pm/w2/007.md", ".pm/w2/done/007.md")
        self.assertEqual(self.stop(A), {})  # moved but not committed: still shipping
        self.git(self.repo, "commit", "-q", "-m", "close w2/007")
        self.assertEqual(self.stop(A), {})  # committed but not pushed
        self.git(self.repo, "push", "-q", "origin", "HEAD:main")
        decision = self.stop(A)  # nobody ran `settle`
        self.assertEqual(decision["decision"], "block")
        self.assertIn("The worker for w2/007.md is finished", decision["reason"])
        self.assertIn("the next pick is w2/008.md", decision["reason"])
        self.assertNotIn("inflight", self.mark(A))

    def test_another_loop_pushing_meanwhile_does_not_hide_the_finish(self) -> None:
        (self.repo / ".pm/w2/done").mkdir(exist_ok=True)
        self.git(self.repo, "mv", ".pm/w2/007.md", ".pm/w2/done/007.md")
        self.git(self.repo, "commit", "-q", "-m", "close w2/007")
        self.git(self.repo, "push", "-q", "origin", "HEAD:main")
        self.git(self.other, "pull", "-q", "--rebase", "origin", "main")
        self.write("elsewhere.txt", "another queue's push\n")
        self.push("unrelated push")
        self.git(self.repo, "fetch", "-q", "origin", "main")  # origin/main is now ahead of HEAD
        self.assertIn("is finished", self.stop(A)["reason"])


class SharedTreeTest(RemoteFixture):
    """Sessions share one checkout: what was dirty before a worker started is not its leftover."""

    def setUp(self) -> None:
        super().setUp()
        self.write(".pm/w2/007.md", "a note\n")
        self.push("a note")
        self.pull()
        (self.repo / "teammate.txt").write_text("someone else's edit in progress\n")
        (self.repo / ".pm" / "w2" / "020.md").write_text("a teammate's filing, not committed\n")
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "w2/007.md", session=A)

    def ship_007(self) -> None:
        self.git(self.repo, "mv", ".pm/w2/007.md", ".pm/w2/done/007.md")
        self.git(self.repo, "commit", "-q", "-m", "close w2/007")
        self.git(self.repo, "push", "-q", "origin", "HEAD:main")

    def outcome(self, item: str, session: str | None = A) -> dict:
        result = self.run_guard("outcome", "w2", item, session=session)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_what_was_dirty_at_dispatch_does_not_fail_the_outcome(self) -> None:
        self.ship_007()
        verified = self.outcome("007")
        self.assertEqual((verified["ok"], verified["clean"], verified["leftover"]), (True, True, []))
        self.assertTrue(verified["settled"])
        # Checked again after the mark is settled, the same baseline still applies.
        self.assertTrue(self.outcome("007")["ok"])

    def test_the_stop_hook_sees_the_finish_despite_a_teammates_changes(self) -> None:
        self.ship_007()
        decision = self.stop(A)
        self.assertIn("The worker for w2/007.md is finished", decision["reason"])
        self.assertIn("the next pick is w2/020.md", decision["reason"])  # the unpushed filing is next
        # The hook dropped the in-flight mark; the baseline outlives it.
        self.assertTrue(self.outcome("007")["ok"])

    def test_what_appeared_during_the_run_is_a_leftover(self) -> None:
        self.ship_007()
        (self.repo / "scratch.txt").write_text("half-done work\n")
        result = self.outcome("007")
        self.assertEqual((result["ok"], result["clean"], result["leftover"]), (False, False, ["scratch.txt"]))
        self.assertEqual(self.stop(A), {})  # not finished: still inside the ship grace period

    def test_marking_the_same_worker_again_keeps_its_baseline(self) -> None:
        (self.repo / "scratch.txt").write_text("the worker's change in progress\n")
        self.run_guard("dispatch", "w2/007.md", session=A)  # the Stop hook asks for this when unsure
        self.ship_007()
        self.assertEqual(self.outcome("007")["leftover"], ["scratch.txt"])

    def test_the_same_item_spelled_without_md_keeps_its_baseline_too(self) -> None:
        # `board` and the Stop hook name a note `w2/007.md`; `outcome` and the skill say `w2/007`.
        (self.repo / "scratch.txt").write_text("the worker's change in progress\n")
        self.run_guard("dispatch", "w2/007", session=A)
        self.ship_007()
        self.assertEqual(self.outcome("007")["leftover"], ["scratch.txt"])

    def test_a_file_added_to_a_directory_that_was_already_untracked_is_a_leftover(self) -> None:
        (self.repo / "drafts").mkdir()
        (self.repo / "drafts" / "teammate.md").write_text("untracked before the worker started\n")
        self.run_guard("dispatch", "w2/001", session=A)  # a new worker: a new baseline
        (self.repo / "drafts" / "worker.md").write_text("left behind\n")
        result = self.outcome("001")
        self.assertEqual((result["leftover"], result["leftover_count"]), (["drafts/worker.md"], 1))

    def test_a_renamed_or_oddly_named_path_is_reported_as_it_is_on_disk(self) -> None:
        self.ship_007()
        self.git(self.repo, "mv", ".pm/w2/done/007.md", ".pm/w2/done/007 renamed.md")
        (self.repo / "笔记.md").write_text("a name git would quote\n")
        result = self.outcome("007")
        self.assertEqual(result["leftover"], [".pm/w2/done/007 renamed.md", "笔记.md"])

    def test_without_a_dispatch_every_uncommitted_path_counts(self) -> None:
        self.ship_007()
        result = self.outcome("007", session=None)  # Codex: no session, no baseline
        self.assertFalse(result["clean"])
        self.assertEqual(result["leftover"], [".pm/w2/020.md", "teammate.txt"])
        self.assertFalse(self.outcome("001")["clean"])  # a baseline belongs to the item it was taken for

    def test_an_item_that_exists_only_in_this_checkout_is_still_open(self) -> None:
        # Filed here by a teammate and never committed: origin/main has no trace of it.
        self.ship_007()
        self.run_guard("dispatch", "w2/020.md", session=A)
        result = self.outcome("020")
        self.assertEqual((result["where"], result["open_here"], result["ok"]), ("absent", True, False))
        (self.repo / ".pm" / "w2" / "020.md").unlink()  # dropped as invalid: nothing to ship
        self.assertTrue(self.outcome("020")["ok"])


ROUTINES = """# w2 routines

Budget: 2 runs/day

| name | command | every | when | paths |
| --- | --- | --- | --- | --- |
| cli-dead-code | `/rt-remove-dead-code go` | 24h | changed | cli |
| dashboard-flaky | `/rt-prune-useless-tests dashboard` | 3d | changed | dashboard |
"""


class RoutineTest(RemoteFixture):
    """Routine scheduling reads origin/main and keeps its state in .git/loopx/."""

    def setUp(self) -> None:
        super().setUp()
        self.write("cli/app.py", "print(1)\n")
        self.write("dashboard/app.ts", "1\n")
        self.write(".pm/w2/routine/README.md", ROUTINES)
        self.push("routines")

    def due(self) -> dict:
        result = self.run_guard("routine-due", "w2")
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def done(self, name: str, result: str):
        return self.run_guard("routine-done", "w2", name, result)

    def state(self) -> dict:
        return json.loads((self.repo / ".git" / "loopx" / "routine-w2.state").read_text())

    def save(self, state: dict) -> None:
        (self.repo / ".git" / "loopx" / "routine-w2.state").write_text(json.dumps(state))

    def fetch(self) -> None:
        self.git(self.repo, "fetch", "-q", "origin", "main")

    def schedule(self, table: str, budget: str = "") -> None:
        header = "| name | command | every | when | paths |\n| --- | --- | --- | --- | --- |\n"
        self.write(".pm/w2/routine/README.md", budget + header + table)
        self.push("schedule")
        self.fetch()

    def test_never_run_routines_are_due_in_list_order(self) -> None:
        self.fetch()
        self.assertEqual(self.due()["due"], {
            "name": "cli-dead-code", "command": "/rt-remove-dead-code go",
            "when": "changed", "paths": ["cli"], "interval_hours": 24.0,
        })
        self.done("cli-dead-code", "empty")
        self.assertEqual(self.due()["due"]["name"], "dashboard-flaky")

    def test_any_command_is_a_routine(self) -> None:
        self.schedule(
            "| links | bash scripts/docs-links-validate.sh | 1d | | |\n"
            "| triage | Read open CI failures and file each as an inbox note | 2d | always | |\n"
            "| simplify | `/simplify` | 1w | changed | |\n"
        )
        reply = self.due()
        self.assertEqual(reply["problems"], [])
        self.assertEqual(reply["due"]["command"], "bash scripts/docs-links-validate.sh")
        self.assertEqual(reply["due"]["when"], "always")
        self.done("links", "empty")
        self.assertEqual(self.due()["due"]["command"], "Read open CI failures and file each as an inbox note")

    def test_a_ran_routine_waits_for_its_interval_and_a_path_change(self) -> None:
        self.fetch()
        self.done("cli-dead-code", "found")
        self.done("dashboard-flaky", "found")
        self.assertIsNone(self.due()["due"])
        # Interval elapsed, but cli/ has not changed: still not due.
        state = self.state()
        for entry in state["routines"].values():
            entry["last_run"] -= 4 * 86400
        state["runs"] = []
        self.save(state)
        reply = self.due()
        self.assertIsNone(reply["due"])
        self.assertIn("no change since", reply["reason"])
        # A commit outside cli/ does not count; one inside does.
        self.write("mobile/app.ts", "1\n")
        self.push("mobile change")
        self.fetch()
        self.assertIsNone(self.due()["due"])
        self.write("cli/app.py", "print(2)\n")
        self.push("cli change")
        self.fetch()
        self.assertEqual(self.due()["due"]["name"], "cli-dead-code")

    def test_a_changed_row_without_paths_waits_for_any_commit(self) -> None:
        self.schedule("| any | /simplify | 1m | changed | |\n")
        self.done("any", "found")
        state = self.state()
        state["routines"]["any"]["last_run"] -= 120
        self.save(state)
        self.assertIsNone(self.due()["due"])
        self.write("mobile/app.ts", "2\n")
        self.push("any change")
        self.fetch()
        self.assertEqual(self.due()["due"]["name"], "any")

    def test_empty_runs_double_the_interval_and_a_find_resets_it(self) -> None:
        self.fetch()
        key = "cli-dead-code"
        self.done(key, "empty")
        self.assertEqual(self.state()["routines"][key]["interval"], 2 * 86400)
        self.done(key, "empty")
        self.assertEqual(self.state()["routines"][key]["interval"], 4 * 86400)
        self.assertEqual(self.state()["routines"][key]["empty_runs"], 2)
        for _ in range(6):
            self.done(key, "empty")
        self.assertEqual(self.state()["routines"][key]["interval"], 14 * 86400)  # capped
        self.done(key, "found")
        self.assertEqual(self.state()["routines"][key]["interval"], 86400)
        self.assertEqual(self.state()["routines"][key]["empty_runs"], 0)

    def test_the_daily_budget_stops_further_runs(self) -> None:
        self.fetch()
        self.done("cli-dead-code", "found")
        self.done("dashboard-flaky", "found")
        state = self.state()
        for entry in state["routines"].values():
            entry["last_run"] -= 4 * 86400
            entry["last_sha"] = ""
        self.save(state)
        reply = self.due()
        self.assertIsNone(reply["due"])
        self.assertIn("budget of 2", reply["reason"])

    def test_wait_exits_5_for_a_due_routine_and_board_work_comes_first(self) -> None:
        result = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(result.returncode, 5, result.stdout + result.stderr)
        self.assertIn("cli-dead-code (/rt-remove-dead-code go)", result.stdout)
        self.assertNotIn("since", self.listener())
        self.write(".pm/w2/011.md", "note\n")
        self.push("board work")
        result = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_wait_idles_once_every_routine_is_waiting(self) -> None:
        self.fetch()
        self.done("cli-dead-code", "found")
        self.done("dashboard-flaky", "found")
        result = self.run_guard("wait", "w2", *FAST, "--chunk", "0.2")
        self.assertEqual(result.returncode, 10, result.stdout + result.stderr)

    def test_bad_rows_are_reported_and_skipped(self) -> None:
        self.schedule(
            "| Bad_Name | /x | 24h | | |\n"
            "| often | /x | often | | |\n"
            "| nocmd |  | 1d | | |\n"
            "| gone | /x | 1d | changed | nowhere |\n"
            "| escape | /x | 1d | changed | ../etc |\n"
            "| short | /x |\n"
            "| twice | /x | 1d | | |\n"
            "| twice | /y | 1d | | |\n"
        )
        reply = self.due()
        problems = " ".join(reply["problems"])
        self.assertIn("'Bad_Name' is not a routine name", problems)
        self.assertIn("'often' is not an interval", problems)
        self.assertIn("nocmd: no command", problems)
        self.assertIn("gone: no nowhere on origin/main", problems)
        self.assertIn("escape: paths must be repository-relative", problems)
        self.assertIn("'short': a row needs name, command, and every", problems)
        self.assertIn("twice: listed twice", problems)
        self.assertEqual(reply["due"]["name"], "twice")  # the first copy still runs

    def test_no_routine_list_means_nothing_due(self) -> None:
        self.git(self.other, "rm", "-q", "-r", ".pm/w2/routine")
        self.git(self.other, "commit", "-q", "-m", "no routines")
        self.git(self.other, "push", "-q", "origin", "HEAD:main")
        self.fetch()
        self.assertEqual(self.due(), {"due": None, "reason": "no routine list", "problems": [], "next_in": None})

    def test_always_rows_run_on_a_fixed_cadence_without_a_change_gate(self) -> None:
        self.schedule("| cli-qa | /qa-find-bugs-cli w2 | 30m | always | cli |\n", budget="Budget: 48 runs/day\n\n")
        self.assertEqual(self.due()["due"], {
            "name": "cli-qa", "command": "/qa-find-bugs-cli w2",
            "when": "always", "paths": ["cli"], "interval_hours": 0.5,
        })
        for _ in range(3):  # empty runs never back off
            self.done("cli-qa", "empty")
            self.assertEqual(self.state()["routines"]["cli-qa"]["interval"], 1800)
        self.assertIsNone(self.due()["due"])  # ran just now
        state = self.state()
        state["routines"]["cli-qa"]["last_run"] -= 1801
        self.save(state)
        # Nothing under cli/ changed since the last run, yet it is due again.
        self.assertEqual(self.due()["due"]["name"], "cli-qa")

    def test_a_row_without_when_runs_on_its_schedule(self) -> None:
        self.schedule("| cli-qa | /qa-find-bugs-cli w2 | 30m |\n")
        self.assertEqual(self.due()["due"]["when"], "always")
        self.done("cli-qa", "empty")
        reply = self.due()
        self.assertIsNone(reply["due"])
        self.assertAlmostEqual(reply["next_in"], 1800, delta=60)
        self.assertEqual(self.state()["routines"]["cli-qa"]["interval"], 1800)  # no backoff

    def test_wait_wakes_on_the_routine_schedule_not_the_backed_off_poll(self) -> None:
        self.schedule("| cli-qa | /qa-find-bugs-cli w2 | 0.03m | always |\n")  # due 1.8 s after a run
        self.done("cli-qa", "empty")
        started = time.time()
        # The board poll alone would not look again for 20 s.
        result = self.run_guard("wait", "w2", "--min", "20", "--max", "20", "--give-up", "60")
        self.assertEqual(result.returncode, 5, result.stdout + result.stderr)
        self.assertLess(time.time() - started, 10)

    def test_a_scheduled_routine_outlasts_the_idle_limit(self) -> None:
        self.schedule("| cli-qa | /qa-find-bugs-cli w2 | 0.03m | always |\n")
        self.done("cli-qa", "empty")
        result = self.run_guard("wait", "w2", "--min", "20", "--max", "20", "--give-up", "0.1")
        self.assertEqual(result.returncode, 5, result.stdout + result.stderr)

    def test_board_work_wins_over_a_due_routine(self) -> None:
        self.schedule("| cli-qa | /qa-find-bugs-cli w2 | 30m | always |\n")
        self.assertIsNotNone(self.due()["due"])
        self.write(".pm/w2/m9/README.md", "milestone\n")
        self.push("board work")
        result = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("next pick w2/m9", result.stdout)

    def test_a_spent_budget_says_when_it_frees_up(self) -> None:
        self.schedule("| cli-qa | /qa-find-bugs-cli w2 | 1m |\n", budget="Budget: 1 runs/day\n\n")
        self.done("cli-qa", "found")
        reply = self.due()
        self.assertIn("budget of 1", reply["reason"])
        self.assertAlmostEqual(reply["next_in"], 86400, delta=60)

    def test_rejects_an_unknown_when(self) -> None:
        self.schedule("| cli-dead-code | /rt-remove-dead-code go | 1d | sometimes |\n")
        self.assertIn("when must be changed or always", " ".join(self.due()["problems"]))

    def test_three_failed_runs_drop_a_routine_to_one_try_a_day(self) -> None:
        self.schedule("| cli-qa | `/qa-find-bugs-cli w2` | 30m | always | cli |\n", budget="Budget: 48 runs/day\n\n")
        for count in (1, 2):
            out = self.done("cli-qa", "failed").stdout
            self.assertNotIn("/pm add", out)
            entry = self.state()["routines"]["cli-qa"]
            self.assertEqual((entry["failures"], entry["interval"]), (count, 1800))  # still on its cadence
        third = self.done("cli-qa", "failed").stdout
        self.assertIn("failed 3 runs in a row", third)
        self.assertIn("/pm add", third)  # one note, when the pause starts
        self.assertEqual(self.state()["routines"]["cli-qa"]["interval"], 86400)
        reply = self.due()
        self.assertIsNone(reply["due"])
        self.assertAlmostEqual(reply["next_in"], 86400, delta=60)
        self.assertNotIn("/pm add", self.done("cli-qa", "failed").stdout)  # not a note per failure
        # A day later it is tried again, and one good run restores the schedule.
        state = self.state()
        state["routines"]["cli-qa"]["last_run"] -= 86401
        self.save(state)
        self.assertEqual(self.due()["due"]["name"], "cli-qa")
        self.done("cli-qa", "empty")
        entry = self.state()["routines"]["cli-qa"]
        self.assertEqual((entry["failures"], entry["interval"]), (0, 1800))

    def test_a_failed_run_does_not_move_a_changed_rows_gate(self) -> None:
        self.fetch()
        self.done("cli-dead-code", "found")
        entry = self.state()["routines"]["cli-dead-code"]
        self.write("cli/app.py", "print(2)\n")
        self.push("cli change")
        self.fetch()
        self.done("cli-dead-code", "failed")  # it never looked at the new commit
        after = self.state()["routines"]["cli-dead-code"]
        self.assertEqual(after["last_sha"], entry["last_sha"])
        self.assertEqual(after["interval"], entry["interval"])  # no backoff either

    def test_recording_a_routine_run_clears_its_inflight_mark(self) -> None:
        self.fetch()
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "routine:cli-dead-code", session=A)
        self.run_guard("routine-done", "w2", "cli-dead-code", "empty", session=A)
        self.assertNotIn("inflight", self.mark(A))

    def test_routine_done_rejects_unlisted_or_malformed_runs(self) -> None:
        self.fetch()
        self.assertEqual(self.done("not-listed", "found").returncode, 2)
        self.assertEqual(self.done("cli-dead-code", "maybe").returncode, 2)
        self.assertEqual(self.done("../x", "found").returncode, 2)
        self.assertEqual(self.run_guard("routine-done", "w2", "rt-remove-dead-code", "go", "found").returncode, 2)


class BoardTest(RemoteFixture):
    """`board` hands the orchestrator the facts for a pick in one call."""

    def setUp(self) -> None:
        super().setUp()
        self.write(".pm/w2/m4/README.md", "milestone\n")
        self.write(".pm/w2/m7/README.md", "milestone\n")
        self.write(".pm/w2/m7/t001.md", task_text("w2/m7/t001.md", depends_on=["w2/m4"]))
        self.write(".pm/w2/010.md", "note\n")
        self.write(".pm/w2/blocked/113.md", "blocked: needs an upstream release\n")
        self.write(".pm/w2/blocked/m3/README.md", "blocked milestone\n")
        self.push("board")
        self.pull()

    def board(self) -> dict:
        result = self.run_guard("board", "w2")
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_lists_ready_and_waiting_items_and_the_default_next(self) -> None:
        facts = self.board()
        self.assertEqual(facts["ready"], ["m4", "010.md"])
        self.assertEqual(facts["waiting"], {"m7": ["w2/m4"]})
        self.assertEqual(facts["next"], "w2/m4")
        self.assertEqual(facts["ci"]["checked"], False)  # nothing pushed by this loop yet

    def test_blocked_items_are_listed_once_then_only_when_they_change(self) -> None:
        self.assertEqual(self.board()["blocked"], {"ready": [], "review": ["113.md", "m3"], "unchanged": 0})
        self.assertEqual(self.board()["blocked"], {"ready": [], "review": [], "unchanged": 2})
        self.write(".pm/w2/blocked/113.md", "blocked: the release shipped, unblock condition holds\n")
        self.push("the owner updates a blocker")
        self.pull()
        self.assertEqual(self.board()["blocked"], {"ready": [], "review": ["113.md"], "unchanged": 1})
        self.run_guard("start", "w2")  # a new run reviews every blocked item once
        self.assertEqual(self.board()["blocked"]["review"], ["113.md", "m3"])

    def test_a_due_routine_is_next_only_when_nothing_is_ready(self) -> None:
        self.write(".pm/w2/routine/README.md", "| name | command | every |\n| --- | --- | --- |\n| cli-qa | `/qa-find-bugs-cli w2` | 30m |\n")
        self.push("a routine")
        self.pull()
        facts = self.board()
        self.assertEqual(facts["next"], "w2/m4")  # board work first
        self.assertEqual(facts["routine"]["due"]["name"], "cli-qa")
        for path in (".pm/w2/m4", ".pm/w2/m7", ".pm/w2/010.md"):
            self.git(self.other, "rm", "-q", "-r", path)
        self.push("drain the board")
        self.pull()
        self.assertEqual(self.board()["next"], "routine:cli-qa")
        self.run_guard("routine-done", "w2", "cli-qa", "empty")
        self.assertIsNone(self.board()["next"])  # nothing to do: listen


FAKE_GH = """#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
pathlib.Path(os.environ["FAKE_GH_DIR"], "calls").open("a").write(" ".join(args) + "\\n")
runs = pathlib.Path(os.environ["FAKE_GH_DIR"], "runs.json")
if not runs.exists():
    sys.stderr.write("gh: not signed in\\n")
    sys.exit(1)
listed = json.loads(runs.read_text())
if "--workflow" in args:  # that workflow's own history, which may reach further back
    history = pathlib.Path(os.environ["FAKE_GH_DIR"], "history.json")
    workflow = args[args.index("--workflow") + 1]
    listed = [r for r in json.loads((history if history.exists() else runs).read_text()) if r["workflowName"] == workflow]
if "--limit" in args:
    listed = listed[: int(args[args.index("--limit") + 1])]
sys.stdout.write(json.dumps(listed))
"""


class CITest(RemoteFixture):
    """The loop reads CI for its own pushes, through `gh`, and only reports red it caused."""

    def setUp(self) -> None:
        super().setUp()
        self.gh_dir = pathlib.Path(self._tmp.name) / "fake-gh"
        self.gh_dir.mkdir()
        gh = self.gh_dir / "gh"
        gh.write_text(FAKE_GH)
        gh.chmod(0o755)
        self.extra_env = {"PATH": f"{self.gh_dir}{os.pathsep}{os.environ['PATH']}", "FAKE_GH_DIR": str(self.gh_dir)}
        self.theirs = self.git(self.repo, "rev-parse", "HEAD").strip()  # someone else's commit
        self.ours = self.local_commit("cli/app.py", "print(1)\n")
        self.clock = 0

    def run_on(self, workflow: str, sha: str, conclusion: str | None, event: str = "push", branch: str = "main") -> dict:
        self.clock += 1
        return {
            "databaseId": 1000 + self.clock, "workflowName": workflow, "headSha": sha, "headBranch": branch, "event": event,
            "status": "completed" if conclusion else "in_progress", "conclusion": conclusion or "",
            "createdAt": f"2026-11-14T10:{self.clock:02d}:00Z", "url": f"https://example.invalid/runs/{1000 + self.clock}",
        }

    def gh_says(self, *runs: dict) -> None:
        (self.gh_dir / "runs.json").write_text(json.dumps(list(reversed(runs))))  # gh lists newest first

    def gh_history(self, *runs: dict) -> None:
        """What `gh run list --workflow` answers: runs older than the repository-wide listing reaches."""
        (self.gh_dir / "history.json").write_text(json.dumps(list(reversed(runs))))

    def gh_calls(self, containing: str = "") -> int:
        calls = self.gh_dir / "calls"
        return sum(1 for line in calls.read_text().splitlines() if containing in line) if calls.exists() else 0

    def pushed(self, *shas: str) -> None:
        result = json.loads(self.run_guard("outcome", "w2", "routine:cli-qa", *shas).stdout)
        self.assertTrue(result["ok"], result)

    def ci(self) -> dict:
        return json.loads(self.run_guard("board", "w2").stdout)["ci"]

    def test_gh_is_not_called_until_the_loop_has_pushed_something(self) -> None:
        self.gh_says(self.run_on("test (backend)", self.ours, "failure"))
        ci = self.ci()
        self.assertEqual((ci["checked"], ci["own_failures"]), (False, []))
        self.assertEqual(self.gh_calls(), 0)

    def test_a_workflow_our_push_turned_red_is_reported_once(self) -> None:
        self.pushed(self.ours)
        self.gh_says(self.run_on("test (backend)", self.theirs, "success"), self.run_on("test (backend)", self.ours, "failure"))
        facts = json.loads(self.run_guard("board", "w2").stdout)
        failure = facts["ci"]["own_failures"][0]
        self.assertEqual((failure["workflow"], failure["label"], failure["attempt"]), ("test (backend)", "ci:test-backend", 1))
        self.assertEqual(failure["since"], self.ours[:9])
        self.assertEqual(facts["next"], "ci:test-backend")  # ahead of any board item
        again = self.ci()
        self.assertEqual(again["own_failures"], [])  # reported once
        self.assertEqual(again["still_red"], ["test (backend)"])  # but not forgotten

    def test_red_that_started_on_someone_elses_commit_is_not_ours(self) -> None:
        self.pushed(self.ours)
        self.gh_says(self.run_on("test (backend)", self.theirs, "failure"), self.run_on("test (backend)", self.ours, "failure"))
        ci = self.ci()
        self.assertTrue(ci["checked"])
        self.assertEqual((ci["own_failures"], ci["still_red"]), ([], []))

    def test_a_streak_that_began_before_the_listing_is_not_ours(self) -> None:
        # A rarely run workflow: its earlier red runs fall outside the newest-runs listing.
        self.pushed(self.ours)
        earlier = [self.run_on("pg operand", self.theirs, "failure"), self.run_on("pg operand", "f" * 40, "failure")]
        ours = self.run_on("pg operand", self.ours, "failure")
        self.gh_history(*earlier, ours)
        self.gh_says(ours)
        ci = self.ci()
        self.assertTrue(ci["checked"])
        self.assertEqual((ci["own_failures"], ci["still_red"]), ([], []))
        self.assertEqual(self.gh_calls("--workflow pg operand"), 1)

    def test_the_history_stops_at_the_last_success(self) -> None:
        self.pushed(self.ours)
        ours = self.run_on("pg operand", self.ours, "failure")
        self.gh_history(self.run_on("pg operand", self.theirs, "failure"), self.run_on("pg operand", "f" * 40, "success"), ours)
        self.gh_says(ours)
        failure = self.ci()["own_failures"][0]
        self.assertEqual((failure["workflow"], failure["since"]), ("pg operand", self.ours[:9]))

    def test_a_workflow_whose_only_runs_are_ours_and_red_is_ours(self) -> None:
        self.pushed(self.ours)
        self.gh_says(self.run_on("pg operand", self.ours, "failure"))
        failure = self.ci()["own_failures"][0]
        self.assertEqual((failure["workflow"], failure["since"], failure["attempt"]), ("pg operand", self.ours[:9], 1))
        self.assertEqual(self.ci()["still_red"], ["pg operand"])
        self.assertEqual(self.gh_calls("--workflow"), 1)  # where it began is read once, not on every poll

    def test_an_entry_an_older_guard_owned_is_dropped_when_its_streak_began_earlier(self) -> None:
        # The state an older guard left: it took the listing's first red run for the streak's start.
        self.pushed(self.ours)
        state = self.repo / ".git" / "loopx" / "ci-w2.state"
        recorded = json.loads(state.read_text())
        recorded["owned"] = {"pg operand": {"since": self.ours, "reported": [1], "at": time.time()}}
        state.write_text(json.dumps(recorded))
        earlier = self.run_on("pg operand", self.theirs, "failure")
        ours = self.run_on("pg operand", self.ours, "failure")
        fix = self.local_commit("cli/app.py", "print(2)\n")
        self.pushed(fix)
        retry = self.run_on("pg operand", fix, "failure")
        self.gh_history(earlier, ours, retry)
        self.gh_says(ours, retry)
        ci = self.ci()
        self.assertEqual((ci["own_failures"], ci["still_red"]), ([], []))

    def test_cancelled_and_running_runs_are_not_verdicts(self) -> None:
        self.pushed(self.ours)
        self.gh_says(
            self.run_on("test (backend)", self.theirs, "success"),
            self.run_on("test (backend)", self.ours, "cancelled"),
            self.run_on("test (dashboard)", self.ours, None),
        )
        ci = self.ci()
        self.assertEqual(ci["own_failures"], [])
        self.assertEqual(ci["pending"], 1)

    def test_a_failed_fix_is_reported_again_and_a_passing_one_ends_it(self) -> None:
        self.pushed(self.ours)
        history = [self.run_on("test (backend)", self.theirs, "success"), self.run_on("test (backend)", self.ours, "failure")]
        self.gh_says(*history)
        self.assertEqual(len(self.ci()["own_failures"]), 1)
        # Someone else pushes while it is red: still ours, but nothing new to report.
        history.append(self.run_on("test (backend)", "f" * 40, "failure"))
        self.gh_says(*history)
        self.assertEqual(self.ci()["own_failures"], [])
        fix = self.local_commit("cli/app.py", "print(2)\n")
        self.pushed(fix)
        history.append(self.run_on("test (backend)", fix, "failure"))
        self.gh_says(*history)
        second = self.ci()["own_failures"]
        self.assertEqual((len(second), second[0]["attempt"], second[0]["since"]), (1, 2, self.ours[:9]))
        history.append(self.run_on("test (backend)", "e" * 40, "success"))
        self.gh_says(*history)
        green = self.ci()
        self.assertEqual((green["own_failures"], green["still_red"]), ([], []))

    def test_only_push_triggered_runs_on_main_are_read(self) -> None:
        # deploy.yml also runs hourly on a schedule; that run's head says nothing about who broke it.
        self.pushed(self.ours)
        self.gh_says(
            self.run_on("deploy", self.theirs, "success"),
            self.run_on("deploy", self.ours, "failure", event="schedule"),
            self.run_on("test (backend)", self.theirs, "success"),
            self.run_on("test (backend)", self.ours, "failure", event="pull_request", branch="some-branch"),
        )
        ci = self.ci()
        self.assertTrue(ci["checked"])
        self.assertEqual((ci["own_failures"], ci["still_red"]), ([], []))

    def test_the_run_listing_is_filtered_here_and_not_by_gh(self) -> None:
        # `--branch` makes GitHub search, which can answer with a page of only old runs.
        self.pushed(self.ours)
        self.gh_says(self.run_on("test (backend)", self.ours, "success"))
        self.ci()
        call = (self.gh_dir / "calls").read_text()
        self.assertIn("run list", call)
        self.assertNotIn("--branch", call)
        self.assertNotIn("--event", call)

    def test_a_run_that_timed_out_is_red_too(self) -> None:
        self.pushed(self.ours)
        self.gh_says(self.run_on("test (backend)", self.theirs, "success"), self.run_on("test (backend)", self.ours, "timed_out"))
        self.assertEqual([f["workflow"] for f in self.ci()["own_failures"]], ["test (backend)"])

    def test_without_a_working_gh_the_check_is_skipped_and_says_so(self) -> None:
        self.pushed(self.ours)  # no runs.json: the fake gh exits 1, as when not signed in
        ci = self.ci()
        self.assertFalse(ci["checked"])
        self.assertIn("CI was not checked", ci["reason"])
        self.assertEqual(self.gh_calls(), 1)

    def test_the_listener_wakes_for_an_own_failure_until_board_has_reported_it(self) -> None:
        self.pushed(self.ours)
        self.gh_says(self.run_on("test (backend)", self.theirs, "success"), self.run_on("test (backend)", self.ours, "failure"))
        woke = self.run_guard("wait", "w2", *FAST)
        self.assertEqual(woke.returncode, 6, woke.stdout + woke.stderr)
        self.assertIn("test (backend)", woke.stdout)
        self.assertEqual(self.run_guard("wait", "w2", *FAST).returncode, 6)  # waiting does not consume it
        self.assertEqual(len(self.ci()["own_failures"]), 1)  # board reports it
        idle = self.run_guard("wait", "w2", *FAST, "--chunk", "0.2")
        self.assertEqual(idle.returncode, 10, idle.stdout + idle.stderr)


class GateTest(RemoteFixture):
    """A parked item whose checkable gate cleared is `blocked.ready`, with no board change (w5/m502)."""

    def setUp(self) -> None:
        super().setUp()
        self.before = self.head()  # a commit every fixture pin below may name
        self.write("lego/fix.go", "package fix\n")
        self.push("fix: the change m3 waits for")
        self.fix = self.head()
        self.write("lego/other.go", "package other\n")
        self.push("fix: the change 115 also waits for")
        self.other_fix = self.head()
        self.write(".pm/w2/m4/README.md", "an open milestone\n")
        self.park("m3", f"clears by itself: production's pin must carry `{self.fix[:9]}`; then replay the check")
        self.park("m8", "user decision: which plan")
        self.push("park items")
        self.pull()

    def head(self) -> str:
        return self.git(self.other, "rev-parse", "HEAD").strip()

    def park(self, name: str, gate: str) -> None:
        """Park NAME under blocked/ with GATE on its workstream line."""
        readme = self.other / ".pm" / "w2" / "README.md"
        target = f"blocked/{name}/README.md" if name.startswith("m") else f"blocked/{name}"
        self.write(f".pm/w2/{target}", "parked\n")
        lines = [line for line in readme.read_text().splitlines() if f"](blocked/{name}" not in line]
        lines.append(f"- [ ] **{name}** — **BLOCKED ({gate})** — [{name}]({target})")
        readme.write_text("\n".join(lines) + "\n")

    def pin(self, sha: str) -> None:
        """Production deploys SHA: deploy.yml writes its pin back to main."""
        self.git(self.other, "commit", "-q", "--allow-empty", "-m", f"chore(deploy): pin platform images to {sha[:12]} [skip ci]")
        self.git(self.other, "push", "-q", "origin", "HEAD:main")
        self.pull()

    def board(self) -> dict:
        result = self.run_guard("board", "w2")
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_a_release_gate_is_ready_exactly_when_the_pin_carries_its_commit(self) -> None:
        self.assertEqual(self.board()["blocked"]["ready"], [])  # no pin at all yet
        self.pin(self.before)
        facts = self.board()
        self.assertEqual(facts["blocked"]["ready"], [], "a pin behind the commit cleared its gate")
        self.assertEqual(facts["blocked"]["pin"], self.before[:12])
        self.assertEqual(facts["next"], "w2/m4")
        self.pin(self.fix)
        facts = self.board()
        self.assertEqual(facts["blocked"]["ready"], ["m3"])
        # Taken before ready board items, and not repeated under review.
        self.assertEqual(facts["next"], "w2/m3")
        self.assertNotIn("m3", facts["blocked"]["review"])
        self.assertEqual(facts["ready"], ["m4"])

    def test_every_named_commit_must_be_carried(self) -> None:
        self.park("115.md", f"pin must carry `{self.fix[:9]}`, `{self.other_fix[:9]}` and then a QA walk")
        self.push("park 115")
        self.pull()
        self.pin(self.fix)
        self.assertEqual(self.board()["blocked"]["ready"], ["m3"])
        self.pin(self.other_fix)
        self.assertEqual(self.board()["blocked"]["ready"], ["m3", "115.md"])

    def test_a_docs_commit_quoting_a_pin_subject_in_its_body_is_not_a_pin(self) -> None:
        self.pin(self.before)
        self.git(self.other, "commit", "-q", "--allow-empty", "-m", "docs(pm): park m3",
                 "-m", f"chore(deploy): pin platform images to {self.fix[:12]}")
        self.git(self.other, "push", "-q", "origin", "HEAD:main")
        self.pull()
        facts = self.board()
        self.assertEqual(facts["blocked"]["pin"], self.before[:12])
        self.assertEqual(facts["blocked"]["ready"], [])

    def test_an_unknown_commit_never_clears_and_is_reported(self) -> None:
        self.park("116.md", "pin must carry `deadbeef12345`")
        self.push("park 116 on a typo")
        self.pull()
        self.pin(self.fix)
        facts = self.board()
        self.assertEqual(facts["blocked"]["ready"], ["m3"])
        self.assertEqual(facts["blocked"]["problems"], ["w2/116.md: `deadbeef12345` is not a commit on origin/main"])

    def test_a_date_gate_is_ready_only_once_its_date_has_passed(self) -> None:
        self.park("117.md", "waits for a week of production, a date and nothing else: date gate: 2000-01-01")
        self.park("118.md", "date gate: 2999-01-01T00:00Z, then read the counters")
        self.push("park two items on dates")
        self.pull()
        self.assertEqual(self.board()["blocked"]["ready"], ["117.md"])

    def test_a_gate_with_a_pin_and_a_date_needs_both(self) -> None:
        self.park("119.md", f"pin must carry `{self.fix[:9]}`; date gate: 2999-01-01")
        self.park("120.md", f"pin must carry `{self.fix[:9]}`; date gate: 2000-01-01T12:00Z")
        self.push("park two items on a pin and a date")
        self.pull()
        self.assertEqual(self.board()["blocked"]["ready"], [])
        self.pin(self.fix)
        self.assertEqual(self.board()["blocked"]["ready"], ["m3", "120.md"])

    def test_a_dispatched_item_is_not_offered_again_until_its_gate_line_changes(self) -> None:
        self.pin(self.fix)
        self.run_guard("start", "w2", session=A)
        self.run_guard("dispatch", "w2/m3", session=A)
        # Its worker parked it again without rewriting the gate: not ready again.
        facts = self.board()
        self.assertEqual(facts["blocked"]["ready"], [])
        self.assertEqual(facts["next"], "w2/m4")
        # A rewritten gate is a new claim, offered once more.
        self.park("m3", f"pin must carry `{self.fix[:9]}`; the replay found the image still old, check again")
        self.push("the worker rewrites m3's gate")
        self.pull()
        self.assertEqual(self.board()["blocked"]["ready"], ["m3"])

    def test_wait_wakes_when_a_pin_clears_a_gate(self) -> None:
        self.git(self.other, "rm", "-q", "-r", ".pm/w2/m4")
        self.push("drain the board")
        self.pull()
        self.pin(self.before)
        proc = subprocess.Popen(
            [sys.executable, str(GUARD), "wait", "w2", *FAST, "--give-up", "20"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=self.guard_env(),
        )
        self.addCleanup(lambda: proc.poll() is None and reap(proc))
        time.sleep(0.5)
        self.assertIsNone(proc.poll(), "wait exited before the pin landed")
        self.git(self.other, "commit", "-q", "--allow-empty", "-m", f"chore(deploy): pin platform images to {self.fix[:12]}")
        self.git(self.other, "push", "-q", "origin", "HEAD:main")
        out, err = proc.communicate(timeout=20)
        self.assertEqual(proc.returncode, 0, err)
        self.assertIn("the gate of parked item w2/m3 cleared", out)

    def test_wait_does_not_wake_for_a_gate_still_closed(self) -> None:
        self.git(self.other, "rm", "-q", "-r", ".pm/w2/m4")
        self.park("118.md", "date gate: 2999-01-01")
        self.push("drain the board, park one more on a date")
        self.pull()
        self.pin(self.before)
        result = self.run_guard("wait", "w2", *FAST, "--chunk", "0.3")
        self.assertEqual(result.returncode, 10, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
