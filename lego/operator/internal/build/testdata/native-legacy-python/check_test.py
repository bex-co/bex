"""Fixture-oracle controls only; no legacy Python, compiler, or Poetry run."""
import ast
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("legacy_check", HERE / "check.py")
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


def identity(project=False):
    prefix = "/project/environment" if project else "/usr/local"
    return {"python": "3.7.17", "python_env": "3.7.17", "base_prefix": "/usr/local",
            "prefix": prefix, "executable": prefix + "/bin/python", "private_imports": [],
            "sys_path": ["/project", "/usr/local/lib/python3.7"], "headers": "3.7.17",
            "libpython": "3.7.17", "number": 107, "label": "Crème brûlée & 李",
            "ffi_bytes": len("Crème brûlée & 李".encode("utf-8"))}


class LegacyOracleTests(unittest.TestCase):
    def test_wrong_version_loader_headers_or_private_imports_cannot_pass(self):
        for project in (False, True):
            check.validate_identity(identity(project), "3.7.17", project)
            for key, value in (("python", "3.11.15"), ("python_env", "3.7"), ("base_prefix", "/usr"),
                               ("headers", "3.8.20"), ("libpython", "3.8.20"), ("ffi_bytes", 0),
                               ("number", 0), ("label", "wrong"), ("private_imports", ["virtualenv"]),
                               ("sys_path", ["/opt/bex/poetry/lib/python3.11/site-packages"])):
                with self.subTest(project=project, key=key), self.assertRaises(RuntimeError):
                    data = identity(project)
                    data[key] = value
                    check.validate_identity(data, "3.7.17", project)

    def test_project_must_be_distinct_and_cannot_use_tool_or_system_interpreter(self):
        for changes in ({"prefix": "/usr/local"}, {"executable": "/usr/bin/python3"},
                        {"prefix": "/opt/bex/poetry", "executable": "/opt/bex/poetry/bin/python"}):
            with self.subTest(changes=changes), self.assertRaises(RuntimeError):
                data = identity(True)
                data.update(changes)
                check.validate_identity(data, "3.7.17", True)

    def test_actual_entrypoint_requests_unforced_poetry_run_and_records_result(self):
        calls = []

        def logged(command, *_):
            calls.append(command)
            return subprocess.CompletedProcess(command, 0, json.dumps(identity(True)), "")

        with tempfile.TemporaryDirectory() as directory:
            previous = Path.cwd()
            try:
                os.chdir(directory)
                with mock.patch.dict(os.environ, {}, clear=True), mock.patch.object(sys, "argv", ["check.py", "build", "3.7.17"]), \
                        mock.patch.object(check, "observe", return_value=identity()), mock.patch.object(check, "logged", side_effect=logged), \
                        contextlib.redirect_stdout(io.StringIO()):
                    check.main()
                self.assertEqual(calls[-1], ["poetry", "run", "python", "check.py", "project", "3.7.17"])
                self.assertEqual(len(calls), 3)
                self.assertEqual(json.loads(Path("qa-output/legacy-build.json").read_text())["project"], identity(True))
            finally:
                os.chdir(previous)

    def test_failed_child_output_is_retained_before_failure(self):
        with tempfile.TemporaryDirectory() as directory, contextlib.redirect_stderr(io.StringIO()):
            output = Path(directory) / "failure.log"
            with self.assertRaises(subprocess.CalledProcessError):
                check.logged([sys.executable, "-c", "import sys; print('controlled child failure'); sys.exit(17)"], output)
            self.assertIn("controlled child failure", output.read_text())

    def test_application_scripts_parse_with_python37_syntax(self):
        for name in ("check.py", "setup.py"):
            ast.parse((HERE / name).read_text(), feature_version=(3, 7))


if __name__ == "__main__":
    unittest.main()
