"""Run the restore consumer with command doubles, without a PostgreSQL server.

These controls prove invocation, byte custody, and failure propagation. Actual
SQL rollback, pg_ivm maintenance, and Timescale owner restore remain image gates.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent

# Include current pg_dump's psql restrictions and COPY terminator. The consumer
# must pass these bytes to psql without concatenation or its own SQL parser.
DUMP_SQL = b"""-- PostgreSQL database dump
\\restrict operand_restore_control
CREATE TABLE public.base (v integer);
COPY public.base (v) FROM stdin;
10
20
\\.
\\unrestrict operand_restore_control
"""
METADATA = b"""SELECT pgivm.restore_immv(
    immv => 'public."order totals"',
    view_definition => 'SELECT sum(v) AS s FROM public.base',
    replace => false
);
"""
TOC = """; Archive created by pg_dump
2; 3079 16384 EXTENSION - pg_ivm
3; 1259 16385 TABLE public base original_owner
4; 0 16385 TABLE DATA public base original_owner
"""

COMMAND = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ["TEST_ROOT"])
tool = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
event = {"tool": tool, "argv": args, "user": os.environ["PGUSER"],
         "database": os.environ["PGDATABASE"]}
if tool == "psql":
    files = [pathlib.Path(arg.split("=", 1)[1]) for arg in args if arg.startswith("--file=")]
    event["files"] = [path.read_text() for path in files]
    event["sql_mode"] = files[0].stat().st_mode & 0o777
with (root / "commands.jsonl").open("a") as log:
    log.write(json.dumps(event) + "\n")
if tool == "pg_restore" and "--list" in args:
    if os.environ.get("FAIL_STAGE") == "list": sys.exit(20)
    print((root / "toc.list").read_text(), end="")
elif tool == "pg_restore":
    dest = pathlib.Path(next(arg.split("=", 1)[1] for arg in args if arg.startswith("--file=")))
    dest.write_bytes((root / "dump.sql").read_bytes())
    if os.environ.get("FAIL_STAGE") == "render": sys.exit(21)
elif os.environ.get("FAIL_STAGE") == "sql":
    sys.exit(22)
'''


class RestoreProtocolTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.dump = self.root / "export dir ' $cash;" / "source database"
        self.dump.mkdir(parents=True)
        (self.dump / "toc.dat").write_bytes(b"directory dump control")
        self.metadata = self.dump.parent / "pg_ivm_restore.sql"
        self.metadata.write_bytes(METADATA)
        (self.root / "dump.sql").write_bytes(DUMP_SQL)
        (self.root / "toc.list").write_text(TOC)
        binaries = self.root / "bin"
        binaries.mkdir()
        for tool in ("pg_restore", "psql"):
            path = binaries / tool
            path.write_text(COMMAND)
            path.chmod(0o700)
        self.work = self.root / "work tmp"
        self.work.mkdir()
        self.environment = dict(
            os.environ,
            PATH=str(binaries) + os.pathsep + os.environ["PATH"],
            TEST_ROOT=str(self.root),
            TMPDIR=str(self.work),
            PGUSER="app_owner",
            PGDATABASE="target database ' $cash;",
        )

    def restore(self, *args):
        return subprocess.run(
            ["sh", str(ROOT / "restore.sh"), *map(str, args)],
            env=self.environment,
            capture_output=True,
            timeout=10,
        )

    def commands(self):
        path = self.root / "commands.jsonl"
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def assert_clean(self):
        self.assertEqual(list(self.work.iterdir()), [])
        self.assertEqual((self.dump / "toc.dat").read_bytes(), b"directory dump control")

    def test_dump_and_original_sidecar_share_one_transaction_and_selected_actor(self):
        result = self.restore(self.dump)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        listed, rendered, applied = self.commands()
        self.assertEqual(listed["argv"], ["--list", str(self.dump)])
        self.assertEqual(rendered["argv"][:2], ["--no-owner", "--no-privileges"])
        self.assertEqual(rendered["argv"][-1], str(self.dump))
        self.assertEqual(len(rendered["argv"]), 4)
        sql_file = rendered["argv"][2]
        self.assertEqual(applied["argv"], [
            "-X", "--dbname=" + self.environment["PGDATABASE"],
            "--single-transaction", "--set=ON_ERROR_STOP=1",
            sql_file, "--file=" + str(self.metadata),
        ])
        self.assertEqual(applied["files"], [DUMP_SQL.decode(), METADATA.decode()])
        self.assertEqual(applied["sql_mode"], 0o600)
        for command in (listed, rendered, applied):
            self.assertEqual(command["user"], "app_owner")
            self.assertEqual(command["database"], self.environment["PGDATABASE"])
        self.assert_clean()

    def test_empty_sidecar_from_database_without_ivm_is_valid(self):
        self.metadata.write_bytes(b"")
        result = self.restore(self.dump)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        self.assertEqual(self.commands()[-1]["files"], [DUMP_SQL.decode(), ""])
        self.assert_clean()

    def test_timescale_archive_is_rejected_before_any_target_connection(self):
        for suffix in ("", " ", " postgres"):
            with self.subTest(owner=suffix):
                (self.root / "toc.list").write_text(TOC + "5; 3079 16390 EXTENSION - timescaledb" + suffix + "\n")
                result = self.restore(self.dump)
                self.assertEqual(result.returncode, 1)
                self.assertIn(b"no database changes were made", result.stderr)
                self.assertTrue(all(command["argv"][0] == "--list" for command in self.commands()))
                self.assert_clean()

    def test_unrelated_table_name_and_extension_comment_do_not_trigger_timescale_rejection(self):
        (self.root / "toc.list").write_text(
            TOC + "5; 1259 16390 TABLE public timescaledb app_owner\n"
            "6; 0 0 COMMENT - EXTENSION timescaledb app_owner\n"
        )
        result = self.restore(self.dump)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        self.assertEqual(self.commands()[-1]["tool"], "psql")
        self.assert_clean()

    def test_large_objects_cannot_escape_the_combined_restore_transaction(self):
        for entry in ("BLOB - 16390", "BLOBS - BLOBS", "BLOB METADATA - 16390", "BLOB COMMENTS - 16390"):
            with self.subTest(entry=entry):
                (self.root / "toc.list").write_text(TOC + "5; 0 16390 " + entry + " app_owner\n")
                result = self.restore(self.dump)
                self.assertEqual(result.returncode, 1)
                self.assertIn(b"Large-object archives", result.stderr)
                self.assertTrue(all(command["argv"][0] == "--list" for command in self.commands()))
                self.assert_clean()

    def test_failure_at_each_stage_propagates_and_cleans_only_own_temporary_files(self):
        marker = self.work / "unrelated-work"
        marker.write_text("retain")
        for stage, code, tools in (
            ("list", 20, ["pg_restore"]),
            ("render", 21, ["pg_restore", "pg_restore"]),
            ("sql", 22, ["pg_restore", "pg_restore", "psql"]),
        ):
            with self.subTest(stage=stage):
                self.environment["FAIL_STAGE"] = stage
                (self.root / "commands.jsonl").unlink(missing_ok=True)
                result = self.restore(self.dump)
                self.assertEqual(result.returncode, code)
                self.assertEqual([command["tool"] for command in self.commands()], tools)
                self.assertEqual(list(self.work.iterdir()), [marker])
                self.assertEqual(marker.read_text(), "retain")
                self.assertEqual(self.metadata.read_bytes(), METADATA)

    def test_missing_sidecar_or_toc_never_invokes_database_tools(self):
        for path in (self.metadata, self.dump / "toc.dat"):
            with self.subTest(path=path.name):
                original = path.read_bytes()
                path.unlink()
                result = self.restore(self.dump)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(self.commands(), [])
                path.write_bytes(original)
                self.assert_clean()

    def test_required_target_and_single_dump_argument(self):
        for args in ((), (self.dump, "unexpected")):
            with self.subTest(args=args):
                self.assertEqual(self.restore(*args).returncode, 2)
        del self.environment["PGDATABASE"]
        self.assertNotEqual(self.restore(self.dump).returncode, 0)
        self.assertEqual(self.commands(), [])
        self.assert_clean()


if __name__ == "__main__":
    unittest.main()
