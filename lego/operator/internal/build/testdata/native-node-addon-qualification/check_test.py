"""Supplied-record/command controls only; no target gyp/interpreter execution."""
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("node_addon_check", HERE / "check.py")
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class AddonControls(unittest.TestCase):
    def setUp(self):
        self.expected = {"case": "controlled", "node_contract": {"node": "16.20.2", "node_env": "16.20.2"},
                         "private_python": "3.9.25", "app_python": None, "common": False, "arch": "arm64", "yarn": "1.22.22", "pnpm": "10.34.7"}
        self.metadata = {"npm": "8.19.4", "gyp": "9.1.0"}  # Supplied oracle data, not donor proof.

    def record(self, version=None, executable=None):
        text = "gyp verb cli '" + check.GYP + "/bin/node-gyp.js'\ngyp info using node-gyp@" + self.metadata["gyp"] + "\n"
        text += 'gyp info find Python using Python version ' + (version or self.expected["private_python"]) + ' found at "' + (executable or check.PRIVATE) + '"\n'
        text += check.GYP + "/gyp/gyp_main.py\n-Dnode_root_dir=/usr/local\n"
        return {"returncode": 0, "stdout": text, "stderr": "", "timed_out": False}

    def test_positive_requires_real_selected_gyp_python_and_local_headers(self):
        valid = self.record()
        check.validate_install(valid, "success", {}, self.expected, self.metadata, True)
        for mutate in (lambda r: r.update(returncode=-9), lambda r: r.update(returncode=None),
                       lambda r: r.update(timed_out=True), lambda r: r.update(returncode=1),
                       lambda r: r.update(stdout=r["stdout"].replace(check.GYP, "/global/node-gyp")),
                       lambda r: r.update(stdout=r["stdout"].replace("node-gyp@9.1.0", "node-gyp@11.0.0")),
                       lambda r: r.update(stdout=r["stdout"].replace(check.PRIVATE, "/usr/bin/python3")),
                       lambda r: r.update(stdout=r["stdout"].replace("-Dnode_root_dir=/usr/local", "-Dnode_root_dir=/cached")),
                       lambda r: r.update(stdout=r["stdout"].replace("3.9.25", "3.9.24"))):
            wrong = copy.deepcopy(valid)
            mutate(wrong)
            with self.subTest(record=wrong), self.assertRaises(AssertionError):
                check.validate_install(wrong, "success", {}, self.expected, self.metadata, True)
        with self.assertRaises(AssertionError):
            check.validate_install(valid, "success", {}, self.expected, self.metadata, False)

    def test_forced_invalid_needs_precise_finder_failure_and_cannot_fallback(self):
        selectors = {"NODE_GYP_FORCE_PYTHON": "/invalid-forced"}
        log = "gyp verb cli '" + check.GYP + "/bin/node-gyp.js'\ngyp info using node-gyp@9.1.0\n"
        log += "NODE_GYP_FORCE_PYTHON /invalid-forced\nCould not find any Python installation to use\n"
        record = {"returncode": 1, "stdout": log, "stderr": "", "timed_out": False}
        check.validate_install(record, "forced-error", selectors, self.expected, self.metadata, False)
        for change in (dict(returncode=0), dict(returncode=-9), dict(timed_out=True),
                       dict(stdout=log.replace("Could not find any Python installation", "EAI_AGAIN registry unavailable")),
                       dict(stdout=log + self.record()["stdout"])):
            with self.subTest(change=change), self.assertRaises(AssertionError):
                check.validate_install(dict(record, **change), "forced-error", selectors, self.expected, self.metadata, False)
        with self.assertRaises(AssertionError):
            check.validate_install(record, "forced-error", selectors, self.expected, self.metadata, True)

    def test_accepted_explicit_app_python_failure_is_not_silent_default(self):
        expected = dict(self.expected, app_python="3.13.77")  # Deliberately noncatalog oracle input.
        record = self.record("3.13.77", "/usr/local/bin/python3")
        record.update(returncode=1, stderr="ModuleNotFoundError: No module named 'distutils'\n")
        check.validate_install(record, "distutils-error", {"PYTHON": "/usr/local/bin/python3"}, expected, self.metadata, False)
        for change in (dict(returncode=-9), dict(timed_out=True), dict(stderr="pnpm unrelated failure"),
                       dict(stdout=self.record()["stdout"]), dict(stdout=record["stdout"].replace("3.13.77", "3.13.76"))):
            with self.subTest(change=change), self.assertRaises(AssertionError):
                check.validate_install(dict(record, **change), "distutils-error", {}, expected, self.metadata, False)

    def test_common_explicit_app_python_success_is_distinct_from_historical_error(self):
        common = dict(self.expected, common=True, app_python="3.13.77")
        self.assertEqual(check.variants(common)[-1][2], "app-success")
        historical = dict(common, common=False)
        self.assertEqual(check.variants(historical)[-1][2], "distutils-error")
        app = self.record("3.13.77", "/usr/local/bin/python3")
        check.validate_install(app, "app-success", {"PYTHON": "/usr/local/bin/python3"}, common, self.metadata, True)
        for record, exists in ((self.record(), True), (dict(app, returncode=1), False), (dict(app, timed_out=True), True)):
            with self.assertRaises(AssertionError):
                check.validate_install(record, "app-success", {"PYTHON": "/usr/local/bin/python3"}, common, self.metadata, exists)
        with self.assertRaises(AssertionError):
            check.validate_install(app, "app-success", {"PYTHON": "/usr/local/bin/python3"}, historical, self.metadata, True)

    def test_explicit_controls_cover_precedence_fallback_and_app_python(self):
        rows = check.variants(self.expected)
        self.assertEqual(rows[0], ("default", {}, "success"))
        by_name = {name: (env, kind) for name, env, kind in rows}
        self.assertEqual(by_name["force-valid"][0]["NODE_GYP_FORCE_PYTHON"], check.EXPLICIT_PRIVATE)
        self.assertEqual(by_name["force-valid"][0]["npm_config_python"], check.PRIVATE)
        self.assertEqual(by_name["config-before-env"][0]["npm_config_python"], check.EXPLICIT_PRIVATE)
        self.assertNotEqual(by_name["config-before-env"][0]["PYTHON"], check.EXPLICIT_PRIVATE)
        self.assertEqual(by_name["env-valid"][0], {"PYTHON": check.EXPLICIT_PRIVATE})
        self.assertNotIn("NODE_GYP_FORCE_PYTHON", by_name["ordinary-invalid-fallback"][0])
        self.assertEqual(by_name["force-invalid"][1], "forced-error")
        app = check.variants(dict(self.expected, app_python="3.13.77"))
        self.assertEqual(app[-1], ("app-python-explicit", {"PYTHON": "/usr/local/bin/python3"}, "distutils-error"))
        for name, env, _ in rows:
            if name == "default":
                continue
            log = self.record(executable=check.EXPLICIT_PRIVATE if name in ("force-valid", "config-before-env", "env-valid") else check.PRIVATE)
            if "NODE_GYP_FORCE_PYTHON" in env:
                log["stdout"] += "NODE_GYP_FORCE_PYTHON\n"
            elif env.get("npm_config_python") == check.EXPLICIT_PRIVATE:
                log["stdout"] += "command line or npm configuration\n"
            elif env.get("PYTHON") == check.EXPLICIT_PRIVATE:
                log["stdout"] += "environment variable PYTHON\n"
            if name != "force-invalid":
                check.validate_install(log, "success", env, self.expected, self.metadata, True)
                if name in ("force-valid", "config-before-env", "env-valid"):
                    with self.assertRaises(AssertionError):
                        check.validate_install(dict(log, stdout=log["stdout"].replace(check.EXPLICIT_PRIVATE + '\"', check.PRIVATE + '\"')), "success", env, self.expected, self.metadata, True)

    def test_fresh_projects_preserve_implicit_install_and_frozen_sources(self):
        base = HERE.parent
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = root / "qa-output"
            output.mkdir()
            frozen = {"package-template.json": (base / "native-node14-addon/package.json").read_bytes(),
                      "binding.gyp": (base / "native-node14-addon/binding.gyp").read_bytes(),
                      "addon.c": (base / "native-node-qualification/addon.c").read_bytes()}
            for name, data in frozen.items():
                (root / name).write_bytes(data)
            with patch.object(check, "ROOT", root), patch.object(check, "OUT", output):
                for manager in check.MANAGERS:
                    work = check.project(manager, "default")
                    package = json.loads((work / "package.json").read_text())
                    self.assertEqual(package["scripts"], {"postinstall": "node ../../addon-check.js lifecycle"})
                    self.assertNotIn("dependencies", package)
                    self.assertTrue(package["gypfile"])
                    for name in ("addon.c", "binding.gyp"):
                        self.assertEqual((work / name).read_bytes(), frozen[name])
                    self.assertFalse((work / "build").exists())
                    with self.assertRaises(FileExistsError):
                        check.project(manager, "default")

    def test_actual_command_timeout_keeps_partial_log_but_is_unproved(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(check, "OUT", Path(directory)), patch.object(check.subprocess, "run", side_effect=subprocess.TimeoutExpired(["npm"], 90, b"gyp partial", b"timed out")):
            result = check.command("timeout", ["npm", "install"])
            self.assertTrue(result["timed_out"])
            self.assertEqual(result["stdout"], "gyp partial")
            self.assertEqual(json.loads((Path(directory) / "timeout.json").read_text()), result)
            with self.assertRaises(AssertionError):
                check.validate_install(result, "success", {}, self.expected, self.metadata, True)

    def test_lifecycle_identity_and_runtime_need_real_header_abi_and_app_node(self):
        proof = {"node": "16.20.2", "node_env": "16.20.2", "executable": "/usr/local/bin/node", "path_node": "/usr/local/bin/node",
                 "path": "/local/bins:/usr/local/bin:/usr/bin", "modules": 93,
                 "addon": {"headers": "16.20.2", "module_abi": 93, "answer": "native Grüße λ 雪"},
                 "cwd": "/case", "platform": "linux", "arch": "arm64", "phase": "lifecycle", "lifecycle": "postinstall",
                 "NODE": "/usr/local/bin/node", "npm_node_execpath": "/usr/local/bin/node", "npm_execpath": "/actual/pnpm"}
        with patch.object(check.os.path, "realpath", side_effect=lambda path: path):
            check.validate_proof(proof, self.expected, Path("/case"), True)
            for change in (dict(node="24.0.0"), dict(node_env=None), dict(phase="wrong"), dict(lifecycle=None),
                           dict(arch="x64"), dict(NODE="/pnpm-private/node"), dict(npm_node_execpath="/pnpm"), dict(npm_execpath=None),
                           dict(addon=dict(proof["addon"], headers="16.20.1")), dict(addon=dict(proof["addon"], module_abi=108))):
                with self.subTest(change=change), self.assertRaises(AssertionError):
                    check.validate_proof(dict(proof, **change), self.expected, Path("/case"), True)

    def test_default_failure_stops_before_any_override_or_other_manager(self):
        calls = []
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def execute(name, argv, *args):
                calls.append(name)
                return {"returncode": 0, "timed_out": False, "stdout": {"npm-version": self.metadata["npm"], "yarn-version": self.expected["yarn"], "pnpm-version": self.expected["pnpm"]}.get(name, "unrelated install failure"), "stderr": ""}
            with patch.object(check, "snapshot", return_value={}), patch.object(check, "private_closure"), patch.object(check, "metadata", return_value=self.metadata), patch.object(check, "retain_tool_packages"), patch.object(check, "command", side_effect=execute), patch.object(check, "project", return_value=root), patch.object(check, "OUT", root):
                with self.assertRaises(AssertionError):
                    check.build(self.expected)
            self.assertEqual(calls[-1], "npm-default")
            self.assertFalse(any("force" in name for name in calls))
            self.assertFalse((root / "addon-build.json").exists())

    def test_runtime_summary_cannot_omit_failed_default_or_reuse_another_row(self):
        installed = [{"manager": manager, "variant": name, "sha256": "a" * 64}
                     for manager in check.MANAGERS for name, _, kind in check.variants(self.expected) if kind == "success"]
        built = {"pass": True, "installed": installed}
        check.validate_summary(built, self.expected)
        for entries in (installed[1:], installed + [installed[0]], list(reversed(installed))):
            with self.assertRaises(AssertionError):
                check.validate_summary(dict(built, installed=entries), self.expected)
        with self.assertRaises(AssertionError):
            check.validate_summary(dict(built, **{"pass": False}), self.expected)

    def test_app_python_snapshot_uses_supplied_full_patch_and_preserves_prefix(self):
        expected = dict(self.expected, app_python="3.13.77")
        def observe(name, argv):
            value = "16.20.2" if name.endswith("-node") else json.dumps(["3.13.77", "/usr/local", "/usr/local/bin/python3"])
            return {"returncode": 0, "timed_out": False, "stdout": value, "stderr": ""}
        with patch.object(check, "command", side_effect=observe), patch.object(check.os.path, "realpath", side_effect=lambda path: path):
            self.assertEqual(check.snapshot(expected, "before")["python"][0], "3.13.77")
            with self.assertRaises(AssertionError):
                check.snapshot(dict(expected, app_python="3.13.76"), "wrong")
        changed = {"returncode": 0, "timed_out": False, "stdout": json.dumps(["3.13.77", "/private", "/private/python"]), "stderr": ""}
        with patch.object(check, "command", side_effect=[observe("before-node", []), changed]):
            with self.assertRaises(AssertionError):
                check.snapshot(expected, "wrong-prefix")

    def test_npm_and_gyp_versions_and_hashes_come_from_selected_package_files(self):
        npm = "/usr/local/lib/node_modules/npm/package.json"
        gyp = check.GYP + "/package.json"
        payloads = {npm: json.dumps({"name": "npm", "version": "8.99.4"}),
                    gyp: json.dumps({"name": "node-gyp", "version": "9.99.1"})}
        with patch.object(check.Path, "read_text", side_effect=[payloads[npm], payloads[gyp]]), patch.object(check, "digest", side_effect=["a" * 64, "b" * 64]) as hashes:
            actual = check.metadata()
            self.assertEqual(actual, {"npm": "8.99.4", "gyp": "9.99.1", "npm_sha256": "a" * 64, "gyp_sha256": "b" * 64})
            self.assertEqual([str(call.args[0]) for call in hashes.call_args_list], [npm, gyp])

    def test_manager_commands_keep_offline_and_actual_npm_major_semantics(self):
        with patch.object(check.os, "geteuid", return_value=0):
            self.assertIn("--unsafe-perm", check.manager_args("npm", "6.99.0"))
            self.assertNotIn("--unsafe-perm", check.manager_args("npm", "8.99.0"))
        for manager in check.MANAGERS:
            args = check.manager_args(manager, self.metadata["npm"])
            self.assertIn("--offline", args)
            self.assertNotIn("--ignore-scripts", args)
            self.assertFalse(any("node-gyp.js" in arg for arg in args))


if __name__ == "__main__":
    unittest.main()
