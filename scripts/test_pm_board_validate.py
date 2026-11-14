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

    def run(self, base: str = "origin/main") -> tuple[int, str]:
        done = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root), "--base", base],
            capture_output=True,
            text=True,
        )
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

    def test_the_repositorys_own_board_passes(self):
        done = subprocess.run([sys.executable, str(SCRIPT), "--root", str(REPO)], capture_output=True, text=True)
        self.assertEqual(done.returncode, 0, done.stderr)


if __name__ == "__main__":
    unittest.main()
