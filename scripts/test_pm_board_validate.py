import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "pm-board-validate.py"
REPO = Path(__file__).resolve().parent.parent


def task(path_id: str, extra: str = "") -> str:
    return f"---\nid: {path_id}\ntitle: t\nstatus: todo\n---\n{extra}\n"


class Board:
    """A throwaway git repository holding a .pm board, with origin/main at its first commit."""

    def __init__(self, files: dict[str, str]):
        self._dir = tempfile.TemporaryDirectory()
        self.root = Path(self._dir.name)
        self.git("init", "-q", "-b", "main")
        self.git("config", "user.email", "t@example.com")
        self.git("config", "user.name", "t")
        self.git("config", "commit.gpgsign", "false")
        self.git("config", "core.hooksPath", os.devnull)
        self.write(files)
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "base")
        self.git("update-ref", "refs/remotes/origin/main", "HEAD")

    def close(self):
        self._dir.cleanup()

    def git(self, *args: str) -> str:
        return subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True, text=True).stdout

    def write(self, files: dict[str, str]):
        for rel, text in files.items():
            path = self.root / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text, encoding="utf-8")

    def move(self, old: str, new: str):
        (self.root / new).parent.mkdir(parents=True, exist_ok=True)
        os.rename(self.root / old, self.root / new)

    def run(self, base: str = "origin/main", head: str | None = None) -> tuple[int, str]:
        cmd = [sys.executable, str(SCRIPT), "--root", str(self.root), "--base", base]
        if head is not None:
            cmd.extend(["--head", head])
        done = subprocess.run(cmd, capture_output=True, text=True)
        return done.returncode, done.stderr + done.stdout


BASE = {
    ".pm/w1/README.md": "# w1\n",
    ".pm/w1/m5/README.md": "# m5\n",
    ".pm/w1/m5/t001.md": task("w1/m5/t001"),
    ".pm/w1/done/m3/README.md": "# done\n",
    ".pm/w1/done/m3/done/t001.md": task("w9/m9/t999"),  # history: a stale id in done/ is fine
    ".pm/w2/README.md": "# w2\n",
    ".pm/w2/blocked/m7/README.md": "# blocked\n",
    ".pm/w2/blocked/m7/t001.md": task("w2/m7/t001"),
}


class BoardValidateTest(unittest.TestCase):
    def board(self, files=None) -> Board:
        board = Board({**BASE, **(files or {})})
        self.addCleanup(board.close)
        return board

    def assertRed(self, board: Board, needle: str, base: str = "origin/main"):
        code, out = board.run(base)
        self.assertEqual(code, 1, out)
        self.assertIn(needle, out)

    def assertGreen(self, board: Board):
        code, out = board.run()
        self.assertEqual(code, 0, out)

    def test_a_clean_board_passes(self):
        self.assertGreen(self.board())

    def test_a_milestone_number_added_under_another_workstream_fails(self):
        board = self.board()
        board.write({".pm/w2/m5/README.md": "# m5 again\n", ".pm/w2/m5/t001.md": task("w2/m5/t001")})
        self.assertRed(board, ".pm/w2/m5 adds milestone m5, already used by w1")

    def test_the_duplicate_is_caught_committed_staged_and_untracked(self):
        for stage in ("untracked", "staged", "committed"):
            with self.subTest(stage):
                board = self.board()
                board.write({".pm/w2/m5/README.md": "# m5 again\n"})
                if stage != "untracked":
                    board.git("add", "-A")
                if stage == "committed":
                    board.git("commit", "-q", "-m", "dup")
                self.assertRed(board, "already used by w1")

    def test_a_number_added_to_a_done_or_blocked_directory_counts_too(self):
        for where in (".pm/w2/done/m5", ".pm/w2/blocked/m5"):
            with self.subTest(where):
                board = self.board()
                board.write({f"{where}/README.md": "# m5 again\n"})
                self.assertRed(board, f"{where} adds milestone m5, already used by w1")

    def test_a_number_taken_on_the_base_branch_after_the_checkout_counts(self):
        board = self.board()
        board.git("checkout", "-q", "-b", "other")
        board.write({".pm/w3/m8/README.md": "# m8\n"})
        board.git("add", "-A")
        board.git("commit", "-q", "-m", "someone else takes m8")
        board.git("update-ref", "refs/remotes/origin/main", "HEAD")
        board.git("checkout", "-q", "main")
        board.write({".pm/w2/m8/README.md": "# m8 too\n"})
        self.assertRed(board, ".pm/w2/m8 adds milestone m8, already used by w3")

    def test_moving_a_milestone_within_its_workstream_is_not_a_duplicate(self):
        board = self.board()
        board.move(".pm/w1/m5", ".pm/w1/done/m5")
        self.assertGreen(board)

    def test_a_historical_duplicate_can_move_between_its_own_workstreams_directories(self):
        board = self.board({".pm/w2/blocked/m5/README.md": "# blocked, same number as w1's m5\n"})
        board.move(".pm/w2/blocked/m5", ".pm/w2/m5")
        self.assertGreen(board)
        board.move(".pm/w2/m5", ".pm/w2/done/m5")
        self.assertGreen(board)

    def test_a_workstream_cannot_take_a_number_another_workstream_uses_by_moving_a_new_milestone_in(self):
        board = self.board()
        board.write({".pm/w2/blocked/m5/README.md": "# new here, m5 is w1's\n"})
        self.assertRed(board, ".pm/w2/blocked/m5 adds milestone m5, already used by w1")

    def test_a_milestone_present_open_and_done_at_once_fails(self):
        board = self.board()
        board.write({".pm/w1/done/m5/README.md": "# copied, not moved\n"})
        self.assertRed(board, "milestone w1/m5 exists in 2 places at once")

    def test_a_duplicate_already_on_the_base_is_history_not_a_failure(self):
        board = self.board({".pm/w2/m5/README.md": "# m5 older duplicate\n"})
        self.assertGreen(board)

    def test_an_open_task_whose_id_differs_from_its_path_fails(self):
        board = self.board()
        board.write({".pm/w1/m5/t002.md": task("w4/m5/t002")})
        self.assertRed(board, ".pm/w1/m5/t002.md has id: w4/m5/t002, want w1/m5/t002")

    def test_a_blocked_task_is_held_to_its_id_and_a_missing_id_fails(self):
        board = self.board()
        board.write({".pm/w2/blocked/m7/t001.md": "\n## a gate note, no frontmatter\n"})
        self.assertRed(board, ".pm/w2/blocked/m7/t001.md has id: (none), want w2/m7/t001")

    def test_an_unresolvable_base_skips_the_duplicate_check_and_says_so(self):
        board = self.board()
        board.write({".pm/w2/m5/README.md": "# m5 again\n", ".pm/w1/m5/t002.md": task("w4/m5/t002")})
        code, out = board.run("origin/nowhere")
        self.assertEqual(code, 1, out)
        self.assertIn("does not resolve; the duplicate check is skipped", out)
        self.assertNotIn("adds milestone", out)
        self.assertIn("has id: w4/m5/t002", out)

    def test_adding_a_task_to_a_milestone_whose_number_is_a_historical_duplicate_passes(self):
        board = self.board({".pm/w2/m5/README.md": "# m5 older duplicate\n"})
        board.write({".pm/w2/m5/t001.md": task("w2/m5/t001"), ".pm/w2/m5/done/t002.md": task("w2/m5/t002")})
        board.write({".pm/w1/m5/done/t001.md": task("w1/m5/t001")})
        self.assertGreen(board)

    def test_re_homing_a_milestone_to_another_workstream_is_not_a_duplicate_of_its_old_place(self):
        board = self.board()
        board.move(".pm/w1/m5", ".pm/w2/m5")
        board.write({".pm/w2/m5/t001.md": task("w2/m5/t001")})
        self.assertGreen(board)

    def test_ids_are_read_through_quotes_a_comment_and_a_byte_order_mark(self):
        board = self.board()
        board.write(
            {
                ".pm/w1/m5/t002.md": '---\nid: "w1/m5/t002"\n---\n',
                ".pm/w1/m5/t003.md": "---\nid: w1/m5/t003 # re-homed\n---\n",
                ".pm/w1/m5/t004.md": "\ufeff---\nid: w1/m5/t004\n---\n",
            }
        )
        self.assertGreen(board)

    def test_a_missing_board_is_unusable(self):
        with tempfile.TemporaryDirectory() as empty:
            done = subprocess.run([sys.executable, str(SCRIPT), "--root", empty], capture_output=True, text=True)
        self.assertEqual(done.returncode, 2)

    def test_head_reads_committed_tree_and_ignores_uncommitted_working_tree_noise(self):
        board = self.board()
        # Uncommitted duplicate and mismatched task id in the working tree must not break a clean HEAD.
        board.write({".pm/w2/m5/README.md": "# uncommitted dup\n", ".pm/w1/m5/t002.md": task("w4/m5/t002")})
        code, out = board.run(head="HEAD")
        self.assertEqual(code, 0, out)
        # Once committed, --head HEAD catches both the colliding milestone and the wrong id.
        board.git("add", "-A")
        board.git("commit", "-q", "-m", "dup and bad id")
        code, out = board.run(head="HEAD")
        self.assertEqual(code, 1, out)
        self.assertIn(".pm/w2/m5 adds milestone m5, already used by w1", out)
        self.assertIn(".pm/w1/m5/t002.md has id: w4/m5/t002, want w1/m5/t002", out)

    def test_head_rejects_unresolvable_revision(self):
        board = self.board()
        code, out = board.run(head="no-such-rev")
        self.assertEqual(code, 2, out)
        self.assertIn("does not resolve", out)

    def gate(self, board: Board, gate: str, checked: str = " ") -> None:
        """Give w2's parked m7 the BLOCKED summary GATE on its workstream line."""
        board.write({".pm/w2/README.md": f"# w2\n\n- [{checked}] **m7** — **BLOCKED ({gate})** — [m7](blocked/m7/README.md)\n"})

    def test_a_release_gate_that_names_no_commit_fails(self):
        for gate in (
            "clears by itself: the next production deploy carries this work",
            "a production pin containing the fix commit",
            "waits for a production roll that carries the fix",
            "production's pin must carry the commit that adds depscache.go",
        ):
            with self.subTest(gate):
                board = self.board()
                self.gate(board, gate)
                self.assertRed(board, ".pm/w2/README.md:3: the BLOCKED summary waits for a production pin")

    def test_a_release_gate_in_the_checkable_form_passes(self):
        board = self.board()
        sha = board.git("rev-parse", "HEAD").strip()
        self.gate(board, f"clears by itself: production's pin must carry `{sha[:9]}` and `{sha[:12]}`; then a QA walk")
        self.assertGreen(board)

    def test_a_pin_gate_naming_no_commit_here_fails(self):
        board = self.board()
        self.gate(board, "pin must carry `deadbeef12345`")
        self.assertRed(board, ".pm/w2/README.md:3: `deadbeef12345` in its pin gate is not a commit in this repository")

    def test_a_date_gate_needs_its_form_and_a_real_date(self):
        for gate, needle in (
            ("waits for a week, a date and nothing else: read on or after 2026-11-23 13:20 UTC", "names a date gate"),
            ("the date gate: after the retention passes", "names a date gate"),
            ("date gate: 2026-02-30", "`date gate: 2026-02-30` is not a real date"),
        ):
            with self.subTest(gate):
                board = self.board()
                self.gate(board, gate)
                self.assertRed(board, needle)
        for gate in ("date gate: 2026-11-23", "read on or after 2026-11-23 13:20 UTC (date gate: 2026-11-23T13:20Z)"):
            with self.subTest(gate):
                board = self.board()
                self.gate(board, gate)
                self.assertGreen(board)

    def test_gates_that_need_a_person_and_closed_lines_are_free_text(self):
        for gate, checked in (
            ("user decision: which plan; upstream merge/release, then a Bex pin review and a CLI release", " "),
            ("the next production deploy carries this", "x"),
        ):
            with self.subTest(gate):
                board = self.board()
                self.gate(board, gate, checked)
                self.assertGreen(board)

    def test_head_reads_gates_from_the_committed_tree(self):
        board = self.board()
        self.gate(board, "the next production deploy carries this")
        code, out = board.run(head="HEAD")
        self.assertEqual(code, 0, out)  # uncommitted: not this push's
        board.git("add", "-A")
        board.git("commit", "-q", "-m", "park m7 on a deploy")
        code, out = board.run(head="HEAD")
        self.assertEqual(code, 1, out)
        self.assertIn(".pm/w2/README.md:3: the BLOCKED summary waits for a production pin", out)

    def test_the_repositorys_own_board_passes(self):
        done = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(REPO), "--head", "HEAD"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(done.returncode, 0, done.stderr)


if __name__ == "__main__":
    unittest.main()
