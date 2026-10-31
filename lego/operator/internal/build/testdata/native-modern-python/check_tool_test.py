"""Source-oracle controls only; no target Python, wheels, Poetry or compiler."""
import ast
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest
from unittest import mock


spec = importlib.util.spec_from_file_location("modern_tool", Path(__file__).with_name("check-tool.py"))
tool = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tool)
app_spec = importlib.util.spec_from_file_location("shared_app", Path(__file__).parent.parent / "native-legacy-python" / "check.py")
app = importlib.util.module_from_spec(app_spec)
app_spec.loader.exec_module(app)


def identity():
    return {"python": "3.9.25", "python_env": "3.9.25", "base_prefix": "/usr/local",
            "prefix": "/opt/bex/poetry", "executable": "/opt/bex/poetry/bin/python",
            "poetry": "2.1.3", "virtualenv": "20.35.0", "dependency_check_succeeded": True}


class ModernToolOracle(unittest.TestCase):
    def test_shared_observer_requires_each_exact_modern_app_and_project(self):
        for version in ("3.9.25", "3.10.22", "3.11.18", "3.12.16", "3.14.10"):
            for project in (False, True):
                prefix = "/project/environment" if project else "/usr/local"
                data = {"python": version, "python_env": version, "base_prefix": "/usr/local",
                        "prefix": prefix, "executable": prefix + "/bin/python", "private_imports": [],
                        "sys_path": ["/project", "/usr/local/lib"], "headers": version, "libpython": version,
                        "number": 107, "label": "Crème brûlée & 李", "ffi_bytes": len("Crème brûlée & 李".encode())}
                app.validate_identity(data, version, project)
                for field in ("python", "python_env", "headers", "libpython"):
                    with self.subTest(version=version, project=project, field=field), self.assertRaises(RuntimeError):
                        wrong = dict(data, **{field: "3.14.8"})
                        app.validate_identity(wrong, version, project)

    def test_selected_modern_tool_identity_and_dependency_check_required(self):
        for virtualenv in ("20.32.0", "20.35.0"):
            data = identity()
            data["virtualenv"] = virtualenv
            tool.validate_tool(data, "3.9.25")  # No invented modern cap or floor.
        for key, value in (("python", "3.11.15"), ("python_env", "3.9"), ("base_prefix", "/usr"),
                           ("prefix", "/usr/local"), ("executable", "/usr/local/bin/python"),
                           ("poetry", "2.2.0"), ("virtualenv", ""), ("dependency_check_succeeded", False)):
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                data = identity()
                data[key] = value
                tool.validate_tool(data, "3.9.25")

    def test_actual_entrypoint_checks_installed_metadata_without_running_a_project(self):
        output = io.StringIO()
        with mock.patch.object(tool, "EXPECTED_PYTHON", "3.9.25"), \
                mock.patch.object(sys, "version", "3.9.25 synthetic"), \
                mock.patch.object(sys, "base_prefix", "/usr/local"), \
                mock.patch.object(sys, "prefix", "/opt/bex/poetry"), \
                mock.patch.object(sys, "executable", "/opt/bex/poetry/bin/python"), \
                mock.patch.dict(os.environ, {"PYTHON_VERSION": "3.9.25"}), \
                mock.patch.object(tool.importlib.metadata, "version", side_effect=lambda name: {"poetry": "2.1.3", "virtualenv": "20.35.0"}[name]), \
                mock.patch.object(tool.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, "No broken requirements found.\n", "")) as run, \
                contextlib.redirect_stdout(output):
            tool.main()
        self.assertEqual(run.call_args.args[0], ["/opt/bex/poetry/bin/python", "-I", "-m", "pip", "check"])
        self.assertTrue(json.loads(output.getvalue())["dependency_check_succeeded"])

    def test_failed_real_dependency_check_cannot_produce_success_metadata(self):
        output = io.StringIO()
        with mock.patch.object(tool.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, "broken requirement", "")), \
                contextlib.redirect_stdout(output), self.assertRaisesRegex(RuntimeError, "broken requirement"):
            tool.main()
        self.assertEqual(output.getvalue(), "")

    def test_checker_parses_with_lowest_selected_python_syntax(self):
        ast.parse(Path(__file__).with_name("check-tool.py").read_text(), feature_version=(3, 9))


if __name__ == "__main__":
    unittest.main()
