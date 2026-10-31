"""Local command/source controls only; no images, registry or actual SBOM claim."""
import argparse
import gzip
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


HERE = Path(__file__).resolve().parent


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


qualification = load("native_common_qualification", HERE / "native-common-qualify.py")
publisher = load("native_publisher_controls", HERE / "test_native_oci_workflow.py")
IMAGE = "sha256:" + "a" * 64
OWNER = "bex-nq-" + "b" * 24


def document():
    return {"spdxVersion": "SPDX-2.3", "SPDXID": "SPDXRef-DOCUMENT", "documentDescribes": ["image"],
            "packages": [{"SPDXID": "image", "name": "actual-test-image", "checksums": [{"algorithm": "SHA256", "checksumValue": IMAGE[7:]}]},
                         {"name": "python", "versionInfo": "3.14.8"},
                         {"name": "libbrotli1", "versionInfo": "1.0.9-2+deb12u1", "externalRefs": [{"referenceLocator": "pkg:deb/debian/libbrotli1@1.0.9-2+deb12u1?arch=amd64"}]}],
            "unfilteredControlField": {"retained": ["all", "findings"]}}


class QualificationControls(unittest.TestCase):
    def test_output_is_after_full_signing_and_never_always_on_failure(self):
        workflow = publisher.WORKFLOW_TEXT
        block = workflow.split("      - id: published\n", 1)[1].split("      - name: remove GHCR credentials", 1)[0]
        self.assertNotIn("if:", block)
        self.assertLess(workflow.index("name: sign and attest the immutable candidate"), workflow.index("      - id: published"))
        shell = publisher.run_block("id: published")
        for mode in ("happy", "last_attest_fail"):
            with self.subTest(mode=mode):
                controls = publisher.PublisherShellControls()
                self.addCleanup(controls.doCleanups)
                process, root, _ = controls.run_case(mode)
                output = root / "published-output"
                # GitHub's normal successful-step gate, not an always hook.
                if process.returncode == 0:
                    env = dict(os.environ, DIGEST=IMAGE, PRODUCER="c" * 40, GITHUB_SHA="c" * 40,
                               NATIVE_IMAGE=publisher.REPOSITORY, GITHUB_OUTPUT=str(output))
                    subprocess.run(["bash", "-euo", "pipefail", "-c", shell], env=env, check=True)
                self.assertEqual(output.exists(), mode == "happy")
                if output.exists():
                    self.assertIn("ref=" + publisher.REPOSITORY + "@" + IMAGE, output.read_text())
        common = (publisher.ROOT / ".github/workflows/native-common-image.yml").read_text()
        for fragment in ("needs: publish", "needs.publish.outputs.ref != ''", "arch: [amd64, arm64]", "max-parallel: 1", "use: false", "cleanup: false", "BEX_NATIVE_TEST_PREPARE_WHEELS:", "BEX_NATIVE_TEST_SYFT:", "if: always() && steps.owned.outputs.root != ''"):
            self.assertIn(fragment, common)

    def test_binding_rejects_wrong_image_arch_runtime_or_missing_component(self):
        with patch.object(qualification.subprocess, "run") as command:
            bound = qualification.bind_spdx(document(), IMAGE, "linux/amd64")
            self.assertEqual(bound["python"], "3.14.8")
            self.assertEqual(command.call_args.args[0], ["dpkg", "--compare-versions", "1.0.9-2+deb12u1", "ge", "1.0.9-2+deb12u1"])
        for wrong in ("image", "arch", "runtime", "missing"):
            data = document()
            if wrong == "image":
                data["packages"][0]["checksums"][0]["checksumValue"] = "f" * 64
            elif wrong == "arch":
                data["packages"][2]["externalRefs"][0]["referenceLocator"] = "pkg:deb/debian/libbrotli1@1?arch=arm64"
            elif wrong == "runtime":
                data["packages"][1]["versionInfo"] = "3.14.10"
            else:
                data["packages"].pop()
            with self.subTest(wrong=wrong), self.assertRaises(ValueError):
                qualification.bind_spdx(data, IMAGE, "linux/amd64")
        with patch.object(qualification.subprocess, "run", side_effect=subprocess.CalledProcessError(1, "dpkg")), self.assertRaises(subprocess.CalledProcessError):
            qualification.bind_spdx(document(), IMAGE, "linux/amd64")

    def test_full_capture_keeps_failure_report_and_gzip_without_success_binding(self):
        for valid in (True, False):
            with self.subTest(valid=valid), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                data = document()
                if not valid:
                    data["packages"][1]["versionInfo"] = "3.14.10"
                proof = root / "qa-output"
                proof.mkdir()
                for phase in ("build", "runtime"):
                    qualification.dump(proof / ("common-default-" + phase + ".json"), {"python": "3.14.8", "python_env": "3.14.8", "pass": True})

                def execute(command, **kwargs):
                    if command[0] == "controlled-syft":
                        self.assertEqual(command[:5], ["controlled-syft", "scan", "docker:" + IMAGE, "--platform", "linux/amd64"])
                        self.assertEqual(kwargs["env"]["SYFT_CHECK_FOR_APP_UPDATE"], "false")
                        qualification.dump(root / "generated.spdx.json", data)
                    else:
                        self.assertEqual(command[0], "dpkg")
                    return subprocess.CompletedProcess(command, 0)

                args = argparse.Namespace(syft="controlled-syft", image=IMAGE, platform="linux/amd64", evidence=str(root))
                with patch.object(qualification.subprocess, "run", side_effect=execute):
                    if valid:
                        qualification.sbom(args)
                    else:
                        with self.assertRaises(ValueError):
                            qualification.sbom(args)
                self.assertEqual(json.loads(gzip.decompress((root / "generated.spdx.json.gz").read_bytes())), data)
                self.assertEqual((root / "generated-spdx-binding.json").exists(), valid)
                self.assertEqual((root / "generated.spdx.json").exists(), not valid)
                self.assertTrue(json.loads((root / "generated-spdx-custody.json").read_text())["roundtrip"])

    def test_cleanup_stops_only_owned_builder_then_cleans_labels_despite_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            qualification.dump(root / "inputs.json", {"owner": OWNER})
            calls = []

            def execute(command, **kwargs):
                calls.append(command)
                if command[:3] == ["docker", "ps", "-aq"]:
                    self.assertEqual(command[-1], "label=" + qualification.LABEL + "=" + OWNER)
                    return subprocess.CompletedProcess(command, 0, "a" * 64 + "\n", "")
                if command[:4] == ["docker", "image", "ls", "-q"]:
                    return subprocess.CompletedProcess(command, 17, "", "controlled listing failure")
                self.assertIn(command, [["docker", "rm", "-f", "a" * 64], ["docker", "buildx", "rm", OWNER]])
                return subprocess.CompletedProcess(command, 0, "", "")

            with patch.object(qualification.subprocess, "run", side_effect=execute), self.assertRaises(RuntimeError):
                qualification.cleanup(root)
            self.assertIn(["docker", "buildx", "rm", OWNER], calls)
            self.assertEqual(calls[0], ["docker", "buildx", "rm", OWNER])
            self.assertFalse(any("prune" in c or "use" in c for c in calls))
            qualification.dump(root / "inputs.json", {"owner": "default"})
            with patch.object(qualification.subprocess, "run") as command, self.assertRaises(ValueError):
                qualification.cleanup(root)
            command.assert_not_called()

    def test_cleanup_timeout_is_recorded_and_does_not_skip_later_owned_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            qualification.dump(root / "inputs.json", {"owner": OWNER})

            def execute(command, **kwargs):
                if command[:3] == ["docker", "buildx", "rm"]:
                    raise subprocess.TimeoutExpired(command, 120)
                return subprocess.CompletedProcess(command, 0, "", "")

            with patch.object(qualification.subprocess, "run", side_effect=execute) as command, self.assertRaises(RuntimeError):
                qualification.cleanup(root)
            self.assertEqual(command.call_count, 3)
            receipt = json.loads((root / "cleanup.json").read_text())
            self.assertEqual(receipt["operations"][0]["exit_code"], 124)

    def test_init_refuses_mutable_or_foreign_source_before_resource_creation(self):
        env = dict(COMMON_IMAGE="ghcr.io/bex-co/bex-cnb-run:latest", PRODUCER_SHA="a" * 40,
                   VERIFICATION_SHA="a" * 40, GITHUB_SHA="a" * 40, TARGET_PLATFORM="linux/amd64")
        with patch.dict(os.environ, env), self.assertRaises(ValueError):
            qualification.initialize()
        env["COMMON_IMAGE"] = "ghcr.io/bex-co/bex-cnb-run@" + IMAGE
        env["PRODUCER_SHA"] = "b" * 40
        with patch.dict(os.environ, env), self.assertRaises(ValueError):
            qualification.initialize()


if __name__ == "__main__":
    unittest.main()
