"""Tests for scripts/workspace_id_guard.py against throwaway git repositories.

The guard warns when a push adds a workspace id to the public board that is not
one of bex's own fixtures; /ship prints the list and ships anyway.
"""

import contextlib
import io
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import workspace_id_guard as guard

# Synthetic ids in the xid alphabet; none is a real workspace.
FIXTURE = "tea-d0f1d000000000000001"
OTHER = "tea-d0tenant000000000002"
THIRD = "tea-d0tenant000000000003"
NOTE = ".pm/w1/001.md"
DOC = "docs/example-record.md"


class WorkspaceIdGuardTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        self.cwd = os.getcwd()
        os.chdir(self.repo)
        self.addCleanup(os.chdir, self.cwd)
        self.git("init", "-q", "-b", "main")
        self.git("config", "user.email", "test@example.invalid")
        self.git("config", "user.name", "test")
        self.git("config", "commit.gpgsign", "false")
        self.write(guard.ALLOWLIST, f"# fixtures\n{FIXTURE} # bex's own\n")
        self.write(NOTE, "# 001\n\nWhy: a record.\n")
        self.base = self.commit("base")

    def git(self, *args):
        return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout

    def write(self, path, text):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def commit(self, message):
        self.git("add", "-A")
        self.git("commit", "-q", "--allow-empty", "-m", message)
        return self.git("rev-parse", "HEAD").strip()

    def run_main(self, base, head):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            rc = guard.main(["workspace_id_guard.py", base, head])
        return rc, err.getvalue()

    def test_an_added_unlisted_id_is_reported(self):
        self.write(NOTE, f"# 001\n\nWhy: a record.\n\nRead in workspace `{OTHER}`.\n")
        head = self.commit("names a tenant")
        self.assertEqual(guard.added_ids(self.base, head), [(NOTE, 5, OTHER)])
        rc, err = self.run_main(self.base, head)
        self.assertEqual(rc, 1)
        self.assertIn(f"{NOTE}:5 adds workspace {OTHER}", err)
        self.assertIn("shipping anyway", err)

    def test_an_id_leading_a_built_name_and_one_in_docs_are_reported(self):
        self.write(DOC, f"Certificate `{OTHER}-web-tls` errored.\nOwner `workspace:{THIRD}`.\n")
        head = self.commit("docs")
        self.assertEqual(guard.added_ids(self.base, head), [(DOC, 1, OTHER), (DOC, 2, THIRD)])

    def test_a_fixture_id_passes(self):
        self.write(NOTE, f"# 001\n\nWhy: a record.\n\nRead in `bex` (`{FIXTURE}`).\n")
        self.assertEqual(self.run_main(self.base, self.commit("fixture")), (0, ""))

    def test_an_id_marked_disposable_on_its_line_passes(self):
        self.write(NOTE, f"# 001\n\nWhy: a record.\n\nDrill workspace `{OTHER}` (disposable), deleted after.\n"
                         f"Then `{THIRD}`.\n")
        head = self.commit("drill")
        # The mark covers its own line only.
        self.assertEqual(guard.added_ids(self.base, head), [(NOTE, 6, THIRD)])

    def test_a_removal_only_diff_passes(self):
        self.write(NOTE, f"# 001\n\nWhy: a record.\n\nRead in workspace `{OTHER}`.\n")
        named = self.commit("names a tenant")
        self.write(NOTE, "# 001\n\nWhy: a record.\n")
        self.assertEqual(self.run_main(named, self.commit("redact")), (0, ""))
        self.git("rm", "-q", NOTE)
        self.assertEqual(self.run_main(named, self.commit("delete")), (0, ""))

    def test_an_edited_or_moved_line_adds_nothing_new(self):
        self.write(NOTE, f"# 001\n\nWhy: a record.\n\nRead in workspace `{OTHER}`.\n")
        named = self.commit("names a tenant")
        self.write(NOTE, f"# 001\n\nWhy: a record.\n\nRead in workspace `{OTHER}`, twice.\n")
        self.assertEqual(self.run_main(named, self.commit("edit")), (0, ""))
        (self.repo / ".pm/w1/done").mkdir()
        self.git("mv", NOTE, ".pm/w1/done/001.md")
        self.assertEqual(self.run_main(named, self.commit("move")), (0, ""))

    def test_the_list_is_read_at_head(self):
        self.write(guard.ALLOWLIST, f"{FIXTURE}\n{OTHER} # a new bex fixture\n")
        self.write(NOTE, f"# 001\n\nWhy: a record.\n\nRead in `{OTHER}`.\n")
        self.assertEqual(self.run_main(self.base, self.commit("new fixture")), (0, ""))

    def test_files_outside_the_board_and_docs_are_not_read(self):
        self.write("lego/backend/fixture_test.go", f'const ws = "{OTHER}"\n')
        self.assertEqual(self.run_main(self.base, self.commit("code")), (0, ""))

    def test_an_unreadable_revision_is_reported(self):
        rc, err = self.run_main("no-such-ref", "HEAD")
        self.assertEqual(rc, 2)
        self.assertIn("cannot read no-such-ref", err)


if __name__ == "__main__":
    unittest.main()
