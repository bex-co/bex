"""Exercise the actual export scripts and their failure protocol without a server."""
import json
import importlib.util
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent


class ArtifactNamespaceTests(unittest.TestCase):
    def test_operand_tags_do_not_become_native_supplier_inputs(self):
        spec = importlib.util.spec_from_file_location("native_matrix", ROOT.parent / "native-base/matrix.py")
        native = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(native)

        def source(image):
            return 'var nativeToolchains = {\n{line: "18", image: "' + image + '"},\n}\n// nativeBun'

        official = "node:18.20.8@sha256:" + "a" * 64
        self.assertEqual(native.matrix(source(official))["include"][0]["base"], official)
        for major, recipe in json.loads((ROOT / "matrix.json").read_text()).items():
            version = recipe["base"].split(":", 1)[1].split("-", 1)[0]
            image = "ghcr.io/bex-co/bex-operator:" + version + "-postgres-" + "b" * 40 + "@sha256:" + "c" * 64
            with self.subTest(major=major):
                self.assertNotIn(":native-", image)
                self.assertNotIn(":latest", image)
                with self.assertRaises(ValueError):
                    native.matrix(source(image))


# Simulate the database protocol only. The actual export and snapshot-dump
# scripts execute unchanged; real pg_dump errors must prevent a final archive.
# Runtime SQL/snapshot semantics remain the separate per-image PostgreSQL gate.
PSQL = r'''#!/usr/bin/env python3
import os,pathlib,subprocess,sys
root=pathlib.Path(os.environ['TEST_ROOT'])
sql=sys.stdin.read()
assert sql.index('BEGIN ISOLATION LEVEL REPEATABLE READ') < sql.index('pg_export_snapshot()') < sql.index('pgivm.get_restore_immv_commands()') < sql.index('\\! bex-postgres-dump-snapshot') < sql.index('COMMIT;')
assert 'SELECT 1 / 0;' in sql
work=pathlib.Path(os.environ['BEX_EXPORT_WORK'])
(work/'snapshot').write_text('00000001-00000005-1\n')
metadata=pathlib.Path(next(x.split('=',1)[1] for x in sys.argv if x.startswith('metadata_file=')))
if os.environ.get('METADATA_FAIL')=='1': sys.exit(3)
if os.environ.get('HAS_IVM')=='1':metadata.write_text("SELECT pgivm.restore_immv('public.owned_view');\n")
(root/'transaction-open').touch()
result=subprocess.run(['bex-postgres-dump-snapshot']).returncode
(root/'transaction-open').unlink()
sys.exit(0 if result==0 and (work/'dump.complete').exists() else 3)
'''

DUMP = r'''#!/usr/bin/env python3
import json,os,pathlib,sys
root=pathlib.Path(os.environ['TEST_ROOT'])
(root/'dump-argv.json').write_text(json.dumps({'argv':sys.argv[1:],'transaction_open':(root/'transaction-open').exists()}))
destination=pathlib.Path(next(x.split('=',1)[1] for x in sys.argv if x.startswith('--file=')))
destination.mkdir()
(destination/'toc.dat').write_text('partial' if os.environ.get('DUMP_FAIL') else 'complete')
sys.exit(31 if os.environ.get('DUMP_FAIL') else 0)
'''


class ExportProtocolTests(unittest.TestCase):
    def export(self, root, **controls):
        binaries = root / "bin"
        binaries.mkdir()
        for name, body in (("psql", PSQL), ("pg_dump", DUMP)):
            path = binaries / name
            path.write_text(body)
            path.chmod(0o700)
        # The script is installed executable in the image. Use an executable
        # wrapper here so tests don't depend on checkout executable modes.
        wrapper = binaries / "bex-postgres-dump-snapshot"
        wrapper.write_text('#!/bin/sh\nexec sh "$OPERAND_DUMP_SCRIPT"\n')
        wrapper.chmod(0o700)
        environment = dict(os.environ, PATH=str(binaries) + os.pathsep + os.environ["PATH"],
                           TEST_ROOT=str(root), BEX_POSTGRES_EXPORT_ROOT=str(root / "work"),
                           OPERAND_DUMP_SCRIPT=str(ROOT / "dump-snapshot.sh"), PGDATABASE="orders_prod",
                           EXPORT_DIRECTORY="2026-10-31T00:00Z", EXPORT_FILENAME="owned.dir.tar.gz", **controls)
        return subprocess.run(["sh", str(ROOT / "export.sh")], env=environment, capture_output=True, timeout=10)

    def test_complete_export_carries_same_snapshot_metadata_and_cleans_transaction_files(self):
        for has_ivm in ("0", "1"):
            with self.subTest(has_ivm=has_ivm), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                result = self.export(root, HAS_IVM=has_ivm)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                observed = json.loads((root / "dump-argv.json").read_text())
                self.assertTrue(observed["transaction_open"])
                self.assertIn("--snapshot=00000001-00000005-1", observed["argv"])
                with tarfile.open(root / "work/owned.dir.tar.gz") as archive:
                    self.assertIn("2026-10-31T00:00Z/orders_prod/toc.dat", archive.getnames())
                    metadata = archive.extractfile("2026-10-31T00:00Z/pg_ivm_restore.sql").read()
                    self.assertEqual(bool(metadata), has_ivm == "1")
                self.assertEqual(list((root / "work").glob(".bex-postgres-export.*")), [])

    def test_dump_or_metadata_failure_never_produces_a_success_archive(self):
        for control in ("DUMP_FAIL", "METADATA_FAIL"):
            with self.subTest(control=control), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                result = self.export(root, **{control: "1"})
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((root / "work/owned.dir.tar.gz").exists())
                self.assertEqual((root / "dump-argv.json").exists(), control == "DUMP_FAIL")
                self.assertEqual(list((root / "work").glob(".bex-postgres-export.*")), [])


# The publish gate runs check.sh on every platform of the built index. A fake
# registry client and container runtime stand in; the actual gate.sh runs.
ORAS = r'''#!/usr/bin/env python3
import json
print(json.dumps({"manifests": [
    {"digest": "sha256:" + "a" * 64, "platform": {"os": "linux", "architecture": "arm64"}},
    {"digest": "sha256:" + "b" * 64, "platform": {"os": "linux", "architecture": "amd64"}},
    {"digest": "sha256:" + "c" * 64, "platform": {"os": "unknown", "architecture": "unknown"}},
]}))
'''

DOCKER = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
argv = sys.argv[1:]
arch = argv[argv.index("--platform") + 1].split("/")[1]
with open(pathlib.Path(os.environ["TEST_ROOT"]) / "runs.jsonl", "a") as runs:
    runs.write(json.dumps(argv) + "\n")
sys.exit(1 if arch in os.environ.get("FAIL_ARCHES", "").split(",") else 0)
'''


class GateTests(unittest.TestCase):
    def gate(self, root, fail_arches="", arches=()):
        binaries = root / "bin"
        binaries.mkdir()
        for name, body in (("oras", ORAS), ("docker", DOCKER)):
            path = binaries / name
            path.write_text(body)
            path.chmod(0o700)
        environment = dict(os.environ, PATH=str(binaries) + os.pathsep + os.environ["PATH"],
                           TEST_ROOT=str(root), FAIL_ARCHES=fail_arches)
        result = subprocess.run(["bash", str(ROOT / "gate.sh"), "127.0.0.1:5000/operand", "sha256:" + "f" * 64,
                                 "16", "postgis,timescaledb", "postgis", "16.16", *arches],
                                env=environment, capture_output=True, timeout=10)
        runs = [json.loads(line) for line in (root / "runs.jsonl").read_text().splitlines()]
        return result, runs

    def test_every_platform_runs_from_its_own_manifest_even_after_a_failure(self):
        for fail_arches in ("", "arm64", "amd64", "arm64,amd64"):
            with self.subTest(fail_arches=fail_arches), tempfile.TemporaryDirectory() as folder:
                result, runs = self.gate(Path(folder), fail_arches)
                self.assertEqual([run[run.index("--platform") + 1] for run in runs], ["linux/arm64", "linux/amd64"])
                for run, digest in zip(runs, ("a", "b")):
                    self.assertIn("127.0.0.1:5000/operand@sha256:" + digest * 64, run)
                    self.assertEqual(run[-5:], ["/check/check.sh", "16", "postgis,timescaledb", "postgis", "16.16"])
                    for flag in (["--network", "none"], ["--memory", "256m"], ["--memory-swap", "256m"]):
                        self.assertIn(flag, [run[i:i + 2] for i in range(len(run) - 1)])
                stdout = result.stdout.decode()
                for arch in ("arm64", "amd64"):
                    want = "FAIL" if arch in fail_arches else "PASS"
                    self.assertIn("gate_%s=%s" % (arch, want), stdout)
                if fail_arches:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("failed on " + fail_arches.replace(",", " "), result.stderr.decode())
                else:
                    self.assertEqual(result.returncode, 0, result.stderr.decode())

    def test_a_named_platform_runs_alone_on_its_own_runner(self):
        for fail_arches, code in (("", 0), ("amd64", 1)):
            with self.subTest(fail_arches=fail_arches), tempfile.TemporaryDirectory() as folder:
                result, runs = self.gate(Path(folder), fail_arches, ("amd64",))
                self.assertEqual([run[run.index("--platform") + 1] for run in runs], ["linux/amd64"])
                self.assertIn("127.0.0.1:5000/operand@sha256:" + "b" * 64, runs[0])
                self.assertEqual(result.returncode, code, result.stderr.decode())


if __name__ == "__main__":
    unittest.main()
