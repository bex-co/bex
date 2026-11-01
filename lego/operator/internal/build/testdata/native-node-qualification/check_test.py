"""Output-oracle negatives only; these never execute Node, tools or images."""
import copy
import hashlib
import unittest
from unittest.mock import patch
import tempfile
from pathlib import Path
import subprocess
import json
import check
import tarfile

from check import PAYLOAD, validate_core, validate_identity, validate_lifecycle, validate_tools


class OutputOracle(unittest.TestCase):
    def setUp(self):
        self.expected = {"node": "14.21.2", "node_env": "14.21.2", "yarn": "1.22.22", "pnpm": "10.34.7"}
        self.proof = {"node": "14.21.2", "executable": "/usr/local/bin/node",
            "path_node": "/usr/local/bin/node", "node_env": "14.21.2",
            "path": "/own/node_modules/.bin:/usr/local/bin:/usr/bin",
            "modules": 83, "napi": "8", "addon": {"headers": "14.21.2", "module_abi": 83, "answer": "native Grüße λ 雪"},
            "openssl": "1.1.1", "brotli": "1.2.0", "sha256": hashlib.sha256(PAYLOAD).hexdigest(),
            "brotli_roundtrip": True, "gzip_roundtrip": True, "aes_roundtrip": True,
            "bad_tag_rejected": True, "roots": 100, "manager": "pnpm", "lifecycle": "probe",
            "NODE": "/usr/local/bin/node", "npm_node_execpath": "/usr/local/bin/node", "npm_execpath": "/selected/pnpm.cjs"}
        # Synthetic oracle input, not an assertion about the donor's npm patch.
        self.versions = {"npm": "6.14.17", "yarn": "1.22.22", "pnpm": "10.34.7"}
        self.tools = {"npm_package": {"name": "npm", "version": "6.14.17"},
            "npm_package_sha256": "a" * 64, "npm_cli": "/usr/local/lib/node_modules/npm/bin/npm-cli.js",
            "yarn_package": {"name": "yarn", "version": "1.22.22"},
            "yarn_cli": "/opt/yarn-v1.22.22/bin/yarn",
            "pnpm_package": {"name": "pnpm", "version": "10.34.7"}}

    def test_selected_runtime_headers_and_env_are_required(self):
        validate_identity(self.proof, self.expected)
        for key, value in (("node", "14.21.3"), ("executable", "/opt/private/node"),
                           ("path_node", "/opt/private/node"), ("node_env", "14"), ("modules", 127)):
            bad = copy.deepcopy(self.proof)
            bad[key] = value
            with self.subTest(key=key), self.assertRaises(AssertionError):
                validate_identity(bad, self.expected)
        bad = copy.deepcopy(self.proof)
        bad["addon"]["headers"] = "14.21.3"
        with self.assertRaises(AssertionError):
            validate_identity(bad, self.expected)

    def test_real_semantics_are_required_not_presence(self):
        validate_core(self.proof)
        for key, value in (("brotli_roundtrip", False), ("gzip_roundtrip", False),
                           ("aes_roundtrip", False), ("bad_tag_rejected", False),
                           ("sha256", "0" * 64), ("roots", 0)):
            bad = dict(self.proof, **{key: value})
            with self.subTest(key=key), self.assertRaises(AssertionError):
                validate_core(bad)

    def test_lifecycle_node_must_not_leak_private_host(self):
        validate_lifecycle(self.proof, self.expected, "pnpm", "probe")
        for key, value in (("NODE", "/opt/private/node"), ("npm_node_execpath", "/opt/pnpm/pnpm"),
                           ("npm_execpath", None), ("lifecycle", "not-a-script")):
            bad = dict(self.proof, **{key: value})
            with self.subTest(key=key), self.assertRaises(AssertionError):
                validate_lifecycle(bad, self.expected, "pnpm", "probe")

    def test_actual_npm_metadata_drives_exactpatch_expectation(self):
        validate_tools(self.tools, self.versions, self.expected)
        bad = dict(self.versions, npm="6.14.18")
        with self.assertRaises(AssertionError):
            validate_tools(self.tools, bad, self.expected)
        for manager in ("yarn", "pnpm"):
            with self.subTest(manager=manager), self.assertRaises(AssertionError):
                validate_tools(self.tools, dict(self.versions, **{manager: "0.0.0"}), self.expected)

    def test_yarn_uses_the_canonical_archive_wrapper(self):
        # Captured tools-build identity from the actual Node14 donor pilot.
        validate_tools(self.tools, self.versions, self.expected)
        for cli in ("/opt/yarn-v1.22.22/bin/yarn.js", "/other/yarn-v1.22.22/bin/yarn",
                    "/opt/yarn-v1.22.21/bin/yarn", "/usr/local/bin/yarn"):
            with self.subTest(cli=cli), self.assertRaises(AssertionError):
                validate_tools(dict(self.tools, yarn_cli=cli), self.versions, self.expected)


    def test_timeout_and_failure_outputs_survive_before_refusal(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            with patch.object(check, "ROOT", root), patch.object(check, "OUT", root), patch.object(
                    check.subprocess, "run", side_effect=subprocess.TimeoutExpired(
                        ["npm"], 90, output=b"partial actual output", stderr=b"timeout context")):
                with self.assertRaises(AssertionError):
                    check.command("timed", ["npm"], cwd=root)
            actual = json.loads((root / "timed.json").read_text())
            self.assertTrue(actual["timed_out"])
            self.assertEqual(actual["stdout"], "partial actual output")
            self.assertEqual(actual["stderr"], "timeout context")
            self.assertIsNone(actual["returncode"])


    def test_negative_is_local_dependency_and_positive_stays_dependency_free(self):
        with tempfile.TemporaryDirectory() as raw, patch.object(check, "OUT", Path(raw)):
            for manager in ("npm", "yarn"):
                positive = check.project(manager, self.expected)
                manifest = json.loads((positive / "package.json").read_text())
                self.assertNotIn("dependencies", manifest)
                self.assertEqual(manifest["engines"]["node"], "14.x")
                negative = check.project(manager, self.expected, wrong=True)
                manifest = json.loads((negative / "package.json").read_text())
                self.assertEqual(manifest["engines"]["node"], "14.x")
                self.assertEqual(manifest["dependencies"], {
                    check.ENGINE_SENTINEL: "file:./" + check.ENGINE_SENTINEL + ".tgz"})
                body = (negative / (check.ENGINE_SENTINEL + ".tgz")).read_bytes()
                self.assertLess(len(body), 4096)
                with tarfile.open(negative / (check.ENGINE_SENTINEL + ".tgz"), "r:gz") as archive:
                    self.assertEqual(archive.getnames(), ["package/package.json"])
                    dependency = json.load(archive.extractfile("package/package.json"))
                self.assertEqual(dependency, {"name": check.ENGINE_SENTINEL,
                    "version": "1.0.0", "engines": {"node": ">=15"}})
                receipt = json.loads((Path(raw) / (manager + "-engine-input.json")).read_text())
                self.assertEqual(receipt["sha256"], hashlib.sha256(body).hexdigest())
                self.assertEqual(receipt["bytes"], len(body))
                check.project(manager, self.expected, wrong=True)
                self.assertEqual((negative / (check.ENGINE_SENTINEL + ".tgz")).read_bytes(), body)

    def test_engine_rejection_requires_specific_reason_package_and_range(self):
        sentinel = check.ENGINE_SENTINEL
        cases = [
            ("npm", "6.14.17", "ENOTSUP",
             "npm ERR! code ENOTSUP\nnpm ERR! notsup Unsupported engine for " + sentinel +
             '@1.0.0: wanted: {"node":">=15"}'),
            ("npm", "11.7.0", "EBADENGINE",
             "npm error code EBADENGINE\nnpm error engine Not compatible with your version of node/npm: " +
             sentinel + '@1.0.0\nnpm error notsup Required: {"node":">=15"}'),
            ("yarn", "6.14.17", 'The engine "node" is incompatible',
             "error " + sentinel + '@1.0.0: The engine "node" is incompatible with this module. Expected version ">=15".')
        ]
        for manager, npm, diagnostic, output in cases:
            with self.subTest(manager=manager, npm=npm):
                actual = subprocess.CompletedProcess([manager], 1, "", output)
                check.validate_engine_rejection(actual, manager, ">=15", npm)
                for status, changed in ((0, output), (-9, output),
                        (1, output.replace(sentinel, "unrelated-package")),
                        (1, output.replace(">=15", ">=99")),
                        (1, output.replace(diagnostic, "UNRELATED_FAILURE")),
                        (1, "EAI_AGAIN registry unavailable for " + sentinel + " >=15")):
                    with self.subTest(status=status, changed=changed), self.assertRaises(AssertionError):
                        check.validate_engine_rejection(
                            subprocess.CompletedProcess([manager], status, "", changed),
                            manager, ">=15", npm)


    def test_pnpm_negative_uses_project_engines_and_real_cwd(self):
        with tempfile.TemporaryDirectory() as raw, patch.object(check, "OUT", Path(raw)):
            positive = check.project("pnpm", self.expected)
            manifest = json.loads((positive / "package.json").read_text())
            self.assertNotIn("dependencies", manifest)
            self.assertEqual(manifest["engines"]["node"], "14.x")
            negative = check.project("pnpm", self.expected, wrong=True)
            manifest = json.loads((negative / "package.json").read_text())
            self.assertNotIn("dependencies", manifest)
            self.assertEqual(list(negative.glob("*.tgz")), [])
            self.assertEqual(negative.name, "pnpm-wrong-engine")
            self.assertEqual(manifest["name"], "bex-profile-pnpm")
            self.assertEqual(manifest["engines"]["node"], ">=15")
            self.assertEqual((negative / "pnpm-workspace.yaml").read_text(), 'packages:\n  - "."\n')
            self.assertNotIn(check.ENGINE_SENTINEL, str(negative))
            custody = json.loads((Path(raw) / "pnpm-engine-input.json").read_text())
            self.assertEqual(custody["project_cwd"], str(negative.resolve()))
            self.assertEqual(custody["selected_node"], "14.21.2")
            self.assertEqual(custody["range"], ">=15")
            self.assertEqual(custody["project_manifest_sha256"],
                hashlib.sha256((negative / "package.json").read_bytes()).hexdigest())
            self.assertEqual(custody["workspace_sha256"],
                hashlib.sha256((negative / "pnpm-workspace.yaml").read_bytes()).hexdigest())
            self.assertIsNone(custody["dependency_tarball"])

    def test_pnpm_reporter_requires_project_current_node_and_wanted_range(self):
        # Derived from tagged10.34.7 reportEngineError; no actual CLI execution.
        cwd = Path("/opt/render/project/src/qa-output/pnpm-wrong-engine")
        for current in ("14.21.2", "v14.21.2"):
            output = "\n".join([
                "ERR_PNPM_UNSUPPORTED_ENGINE Unsupported environment (bad pnpm and/or Node.js version)",
                'Your Node version is incompatible with "' + str(cwd) + '".',
                "Expected version: >=15", "Got: " + current])
            actual = subprocess.CompletedProcess(["pnpm"], 1, "", output)
            result = check.validate_engine_rejection(
                actual, "pnpm", ">=15", "6.14.17", cwd=cwd, selected_node="14.21.2")
            self.assertEqual(result["project_cwd"], str(cwd))
            self.assertEqual(result["selected_node"], "14.21.2")
            for status, changed in (
                    (0, output), (-9, output),
                    (1, output.replace("Got: " + current, "Got: v24.22.1")),
                    (1, output.replace("Got: " + current, "Got: v14.21.3")),
                    (1, output.replace(str(cwd), str(cwd) + "-different")),
                    (1, output.replace(str(cwd), "bex-profile-pnpm")),
                    (1, output.replace(str(cwd), "file:/own/" + check.ENGINE_SENTINEL + ".tgz")),
                    (1, output.replace(">=15", ">=99")),
                    (1, output.replace("ERR_PNPM_UNSUPPORTED_ENGINE", "UNRELATED_ERROR")),
                    (1, "This version of pnpm requires at least Node.js v18.12")):
                with self.subTest(current=current, status=status, output=changed), self.assertRaises(AssertionError):
                    check.validate_engine_rejection(
                        subprocess.CompletedProcess(["pnpm"], status, "", changed),
                        "pnpm", ">=15", "6.14.17", cwd=cwd, selected_node="14.21.2")
            with self.assertRaises(AssertionError):
                check.validate_engine_rejection(actual, "pnpm", ">=15", "6.14.17")


if __name__ == "__main__":
    unittest.main()
