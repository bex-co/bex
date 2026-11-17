"""Tests for scripts/migration_order_guard.py against throwaway git repositories (w4/701)."""

import contextlib
import io
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import migration_order_guard as guard

DIR = guard.MIGRATIONS


class MigrationOrderGuardTest(unittest.TestCase):
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
        (self.repo / "README").write_text("before the first migration\n")
        self.empty = self.commit("init")
        # main as production last pinned it: 0284 is live, 0283 not yet written.
        self.add("0282_sandbox_snapshots", "0284_cron_minimum")
        self.base = self.commit("main")

    def git(self, *args):
        return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout

    def add(self, *names):
        for name in names:
            for half in ("up", "down"):
                target = self.repo / DIR / f"{name}.{half}.sql"
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("SELECT 1;\n")

    def commit(self, message):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", message)
        return self.git("rev-parse", "HEAD").strip()

    def run_main(self, *args):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            rc = guard.main(["migration_order_guard.py", *args])
        return rc, err.getvalue()

    def test_a_migration_below_the_bases_highest_is_refused(self):
        # w4/701's shape: a branch rebased over 0284 keeps its 0283.
        self.add("0283_audit_source_ip_user_agent")
        head = self.commit("audit origin")
        self.assertEqual(guard.check(self.base, head), ("0284_cron_minimum.up.sql", ["0283_audit_source_ip_user_agent.up.sql"]))
        rc, err = self.run_main(self.base, head)
        self.assertEqual(rc, 1)
        self.assertIn(f"{DIR}0283_audit_source_ip_user_agent.up.sql numbers at or below 0284_cron_minimum.up.sql", err)
        self.assertIn("renumber above 0284; a lower number is skipped on any database already past it", err)

    def test_a_migration_above_the_bases_highest_passes(self):
        self.add("0285_audit_render_event_rows")
        head = self.commit("next")
        self.assertEqual(self.run_main(self.base, head), (0, ""))

    def test_a_number_main_already_holds_is_refused(self):
        # A rebase that leaves two migrations sharing the base's highest number.
        self.add("0284_other_feature")
        head = self.commit("duplicate")
        self.assertEqual(guard.check(self.base, head)[1], ["0284_other_feature.up.sql"])

    def test_every_added_migration_is_judged_over_a_multi_commit_push(self):
        self.add("0285_first")
        self.commit("first")
        self.add("0281_late", "0286_second")
        head = self.commit("second")
        self.assertEqual(guard.check(self.base, head)[1], ["0281_late.up.sql"])

    def test_the_branch_need_not_hold_the_bases_newest_migration(self):
        # The check is against the base's tree: a pull request's merge base can
        # lack 0284, and its 0283 still lands below it.
        self.git("checkout", "-q", "-b", "old", self.base)
        self.git("rm", "-q", f"{DIR}0284_cron_minimum.up.sql", f"{DIR}0284_cron_minimum.down.sql")
        self.add("0283_audit_source_ip_user_agent")
        head = self.commit("branch without 0284")
        self.assertEqual(guard.check(self.base, head)[1], ["0283_audit_source_ip_user_agent.up.sql"])

    def test_changes_to_existing_migrations_and_other_files_pass(self):
        (self.repo / DIR / "0284_cron_minimum.up.sql").write_text("SELECT 2;\n")
        (self.repo / DIR / "README").write_text("not a migration\n")
        (self.repo / "other.txt").write_text("x\n")
        self.assertEqual(self.run_main(self.base, self.commit("edit")), (0, ""))

    def test_a_base_without_migrations_passes(self):
        self.assertEqual(self.run_main(self.empty, self.base), (0, ""))

    def test_an_unreadable_revision_is_reported(self):
        rc, err = self.run_main("no-such-ref", "HEAD")
        self.assertEqual(rc, 2)
        self.assertIn("cannot read no-such-ref", err)


if __name__ == "__main__":
    unittest.main()
