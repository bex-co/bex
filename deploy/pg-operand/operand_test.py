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


if __name__ == "__main__":
    unittest.main()
