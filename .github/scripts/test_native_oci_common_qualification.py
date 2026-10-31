"""Local command/source controls only; no images, registry or actual SBOM claim."""
import argparse
import gzip
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
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
# Identity projection from the retained real Syft1.55.0 Docker-source report
# in exact-python-default-20261029. Package rows below are small test controls,
# not a replacement or newly generated SBOM.
IMAGE = "sha256:6902ef50f66ace40e04e14dbfac2b58f5cc79f675612b7b4f154d4048e00383d"
MANIFEST = "sha256:645172b6476137c42feb86af17c35515aaf9af122dd7b7bab57ae42f2306e287"
OWNER = "bex-nq-" + "b" * 24


def inventory():
    return {"source": {"id": MANIFEST[7:], "name": "sha256", "version": IMAGE[7:], "type": "image",
                       "metadata": {"userInput": IMAGE, "imageID": IMAGE, "manifestDigest": MANIFEST,
                                    "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
                                    "architecture": "arm64", "os": "linux"}},
            "unfilteredControlField": {"retained": ["all", "metadata"]}}


def document():
    return {"spdxVersion": "SPDX-2.3", "SPDXID": "SPDXRef-DOCUMENT", "documentDescribes": ["SPDXRef-DocumentRoot-Image-sha256"],
            "packages": [{"SPDXID": "SPDXRef-DocumentRoot-Image-sha256", "name": "sha256", "versionInfo": IMAGE[7:], "checksums": [{"algorithm": "SHA256", "checksumValue": MANIFEST[7:]}]},
                         {"name": "python", "versionInfo": "3.14.8"},
                         {"name": "libbrotli1", "versionInfo": "1.0.9-2+deb12u1", "externalRefs": [{"referenceLocator": "pkg:deb/debian/libbrotli1@1.0.9-2+deb12u1?arch=arm64"}]}],
            "unfilteredControlField": {"retained": ["all", "findings"]}}


class QualificationControls(unittest.TestCase):
    def initialize_owned(self, directory):
        env = dict(COMMON_IMAGE="ghcr.io/bex-co/bex-cnb-run@" + IMAGE,
                   PRODUCER_SHA="a" * 40, VERIFICATION_SHA="a" * 40, GITHUB_SHA="a" * 40,
                   TARGET_PLATFORM="linux/arm64", RUNNER_TEMP=str(directory),
                   GITHUB_RUN_ID="123", GITHUB_RUN_ATTEMPT="1",
                   GITHUB_OUTPUT=str(directory / "outputs"), GITHUB_ENV=str(directory / "environment"))
        with patch.dict(os.environ, env):
            qualification.initialize()
        outputs = dict(line.split("=", 1) for line in (directory / "outputs").read_text().splitlines())
        root = Path(outputs["root"])
        inputs = json.loads((root / "inputs.json").read_text())
        return root, inputs, Path(inputs["docker_config"])

    def test_scoped_login_uses_package_read_token_stdin_and_owned_config_outside_evidence(self):
        workflow = (publisher.ROOT / ".github/workflows/native-common-image.yml").read_text()
        qualify = workflow.split("  qualify:\n", 1)[1]
        self.assertIn("    permissions:\n      contents: read\n      packages: read\n", qualify)
        marker = "      - name: log in to GHCR using this qualification's isolated credentials\n"
        self.assertLess(qualify.index("id: owned"), qualify.index(marker))
        self.assertLess(qualify.index(marker), qualify.index("uses: docker/setup-buildx-action@"))
        login = qualify.split(marker, 1)[1].split("      - uses:", 1)[0]
        self.assertIn("GHCR_TOKEN: ${{ secrets.GITHUB_TOKEN }}", login)
        self.assertIn("timeout-minutes: 2", login)
        shell = "\n".join(line[10:] for line in login.split("        run: |\n", 1)[1].splitlines())
        with tempfile.TemporaryDirectory() as directory:
            temporary = Path(directory).resolve()
            root, inputs, config = self.initialize_owned(temporary)
            self.assertEqual(config.parent, root.parent)
            self.assertNotEqual(config, root)
            self.assertEqual(config.stat().st_mode & 0o777, 0o700)
            self.assertEqual(json.loads((config / ".qualification-owner.json").read_text()),
                             {"owner": inputs["owner"], "evidence_root": str(root)})
            exported = dict(line.split("=", 1) for line in (temporary / "environment").read_text().splitlines())
            self.assertEqual(exported, {"DOCKER_CONFIG": str(config)})
            binary = temporary / "bin"
            binary.mkdir()
            stub = r'''import json,os,pathlib,sys
if pathlib.Path(sys.argv[0]).name == 'timeout':
 assert sys.argv[1:3] == ['--kill-after=10s', '60s']
 os.execvpe(sys.argv[3], sys.argv[3:], os.environ)
assert sys.argv[1:] == ['login','ghcr.io','--username','controlled-actor','--password-stdin']
assert sys.stdin.read() == 'CONTROL-ONLY-FAKE-TOKEN'
config=pathlib.Path(os.environ['DOCKER_CONFIG'])
assert config == pathlib.Path(os.environ['EXPECTED_CONFIG'])
assert (config/'.qualification-owner.json').is_file()
(config/'config.json').write_text(json.dumps({'auths':{'ghcr.io':{'auth':'CONTROL-ONLY-FAKE-TOKEN'}}}))
print('controlled login succeeded')
'''
            for name in ("docker", "timeout"):
                executable = binary / name
                executable.write_text("#!" + sys.executable + "\n" + stub)
                executable.chmod(0o755)
            env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ["PATH"],
                       GHCR_TOKEN="CONTROL-ONLY-FAKE-TOKEN", GITHUB_ACTOR="controlled-actor",
                       EXPECTED_CONFIG=str(config), **exported)
            process = subprocess.run(["bash", "-euo", "pipefail", "-c", shell], env=env,
                                     capture_output=True, text=True, timeout=10)
            self.assertEqual(process.returncode, 0, process.stderr)
            self.assertNotIn(env["GHCR_TOKEN"], process.stdout + process.stderr)
            self.assertTrue((config / "config.json").is_file())
            self.assertFalse(any(env["GHCR_TOKEN"] in path.read_text() for path in root.rglob("*") if path.is_file()))
            qualification.manifest(root)
            self.assertNotIn("config.json", (root / "MANIFEST.json").read_text())

    def test_initialization_failure_keeps_exact_owner_scope_for_always_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            temporary = Path(directory).resolve()
            with patch.object(qualification, "capacity", side_effect=OSError("controlled late init failure")):
                with self.assertRaises(OSError):
                    self.initialize_owned(temporary)
            outputs = dict(line.split("=", 1) for line in (temporary / "outputs").read_text().splitlines())
            root = Path(outputs["root"])
            inputs = json.loads((root / "inputs.json").read_text())
            config = qualification.owned_docker_config(root, inputs)
            self.assertTrue(config.is_dir())
            with patch.object(qualification.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, "", "")):
                qualification.cleanup(root)
            self.assertFalse(config.exists())

    def test_partial_initialization_removes_incomplete_scope_and_cleanup_stays_scoped(self):
        with tempfile.TemporaryDirectory() as directory:
            temporary = Path(directory).resolve()
            original_dump = qualification.dump

            def write(path, data):
                if path.name == ".qualification-owner.json":
                    raise OSError("controlled initial marker failure")
                original_dump(path, data)

            with patch.object(qualification, "dump", side_effect=write), self.assertRaises(OSError):
                self.initialize_owned(temporary)
            outputs = dict(line.split("=", 1) for line in (temporary / "outputs").read_text().splitlines())
            root = Path(outputs["root"])
            inputs = json.loads((root / "inputs.json").read_text())
            config = Path(inputs["docker_config"])
            self.assertFalse(config.exists())
            foreign = temporary / "foreign-config"
            foreign.mkdir()
            (foreign / "config.json").write_text("foreign sentinel")

            def execute(command, **kwargs):
                self.assertEqual(kwargs["env"]["DOCKER_CONFIG"], str(config))
                self.assertTrue((config / ".qualification-owner.json").is_file())
                return subprocess.CompletedProcess(command, 0, "", "")

            with patch.dict(os.environ, DOCKER_CONFIG=str(foreign)), patch.object(qualification.subprocess, "run", side_effect=execute):
                qualification.cleanup(root)
            self.assertFalse(config.exists())
            self.assertEqual((foreign / "config.json").read_text(), "foreign sentinel")

    def test_auth_cleanup_is_finally_scoped_and_drops_credentials_after_prior_failures(self):
        for failure in ("builder", "listing", "removal", "identity", "logout-timeout", "evidence-write"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                temporary = Path(directory).resolve()
                root, inputs, config = self.initialize_owned(temporary)
                secret = "CONTROL-ONLY-FAKE-TOKEN"
                (config / "config.json").write_text(secret)
                foreign = temporary / "foreign-config"
                foreign.mkdir()
                (foreign / "config.json").write_text("foreign sentinel")
                calls = []

                def execute(command, **kwargs):
                    calls.append(command)
                    self.assertEqual(kwargs["env"]["DOCKER_CONFIG"], str(config))
                    if command[1] == "logout":
                        self.assertEqual(command, ["docker", "logout", "ghcr.io"])
                        self.assertEqual(kwargs["stdout"], subprocess.DEVNULL)
                        self.assertEqual(kwargs["stderr"], subprocess.DEVNULL)
                        if failure == "logout-timeout":
                            raise subprocess.TimeoutExpired(command, 20)
                        return subprocess.CompletedProcess(command, 0, secret, secret)
                    if command[1:3] == ["ps", "-aq"]:
                        return subprocess.CompletedProcess(command, 17 if failure == "listing" else 0,
                                                           "invalid-identity" if failure == "identity" else "a" * 64, "")
                    code = 17 if ((failure == "builder" and command[1] == "buildx")
                                  or (failure == "removal" and command[1] == "rm")) else 0
                    return subprocess.CompletedProcess(command, code, "", "")

                original_dump = qualification.dump

                def write(path, data):
                    if failure == "evidence-write" and path.name == "cleanup.json":
                        raise OSError("controlled evidence failure")
                    original_dump(path, data)

                expected = ValueError if failure == "identity" else OSError if failure == "evidence-write" else RuntimeError
                with patch.dict(os.environ, DOCKER_CONFIG=str(foreign)), patch.object(qualification.subprocess, "run", side_effect=execute), patch.object(qualification, "dump", side_effect=write):
                    with self.assertRaises(expected):
                        qualification.cleanup(root)
                self.assertEqual(calls[-1], ["docker", "logout", "ghcr.io"])
                self.assertFalse(config.exists())
                self.assertEqual((foreign / "config.json").read_text(), "foreign sentinel")
                if failure != "evidence-write":
                    receipt = (root / "cleanup.json").read_text()
                    self.assertNotIn(secret, receipt)
                    auth = json.loads(receipt)["operations"][-1]
                    self.assertTrue(auth["removed"])
                    self.assertEqual(auth["logout_exit_code"], 124 if failure == "logout-timeout" else 0)

    def test_cleanup_refuses_foreign_symlinked_or_changed_credential_ownership(self):
        for invalid in ("path", "directory-symlink", "marker-symlink", "marker-owner", "permissions"):
            with self.subTest(invalid=invalid), tempfile.TemporaryDirectory() as directory:
                temporary = Path(directory).resolve()
                root, inputs, config = self.initialize_owned(temporary)
                foreign = temporary / "foreign"
                foreign.mkdir()
                sentinel = foreign / "sentinel"
                sentinel.write_text("foreign sentinel")
                if invalid == "path":
                    inputs["docker_config"] = str(foreign)
                    qualification.dump(root / "inputs.json", inputs)
                elif invalid == "directory-symlink":
                    (config / ".qualification-owner.json").unlink()
                    config.rmdir()
                    config.symlink_to(foreign, target_is_directory=True)
                elif invalid == "marker-symlink":
                    (config / ".qualification-owner.json").unlink()
                    (config / ".qualification-owner.json").symlink_to(sentinel)
                elif invalid == "marker-owner":
                    qualification.dump(config / ".qualification-owner.json", {"owner": "foreign", "evidence_root": str(root)})
                else:
                    config.chmod(0o755)
                with patch.object(qualification.subprocess, "run") as command, self.assertRaises(ValueError):
                    qualification.cleanup(root)
                command.assert_not_called()
                self.assertEqual(sentinel.read_text(), "foreign sentinel")

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

    def test_real_report_shape_binds_distinct_config_and_manifest(self):
        self.assertNotEqual(IMAGE, MANIFEST)
        with patch.object(qualification.subprocess, "run") as command:
            bound = qualification.bind_spdx(document(), inventory(), IMAGE, "linux/arm64")
            self.assertEqual(bound["image_config_id"], IMAGE)
            self.assertEqual(bound["syft_manifest_digest"], MANIFEST)
            self.assertEqual(bound["python"], "3.14.8")
            self.assertEqual(command.call_args.args[0], ["dpkg", "--compare-versions", "1.0.9-2+deb12u1", "ge", "1.0.9-2+deb12u1"])

    def test_binding_rejects_wrong_config_platform_root_runtime_or_missing_component(self):
        for wrong in ("config", "platform", "root", "package-arch", "runtime", "missing"):
            data, source = document(), inventory()
            if wrong == "config":
                source["source"]["metadata"]["imageID"] = "sha256:" + "f" * 64
            elif wrong == "platform":
                source["source"]["metadata"]["architecture"] = "amd64"
            elif wrong == "root":
                # This was the old bug: a config digest is not the SPDX root.
                data["packages"][0]["checksums"][0]["checksumValue"] = IMAGE[7:]
            elif wrong == "package-arch":
                data["packages"][2]["externalRefs"][0]["referenceLocator"] = "pkg:deb/debian/libbrotli1@1?arch=amd64"
            elif wrong == "runtime":
                data["packages"][1]["versionInfo"] = "3.14.10"
            else:
                data["packages"].pop()
            with self.subTest(wrong=wrong), self.assertRaises(ValueError):
                qualification.bind_spdx(data, source, IMAGE, "linux/arm64")
        with patch.object(qualification.subprocess, "run", side_effect=subprocess.CalledProcessError(1, "dpkg")), self.assertRaises(subprocess.CalledProcessError):
            qualification.bind_spdx(document(), inventory(), IMAGE, "linux/arm64")

    def test_full_capture_keeps_failure_report_and_gzip_without_success_binding(self):
        for valid in (True, False):
            with self.subTest(valid=valid), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                data, source = document(), inventory()
                if not valid:
                    data["packages"][1]["versionInfo"] = "3.14.10"
                proof = root / "qa-output"
                proof.mkdir()
                for phase in ("build", "runtime"):
                    qualification.dump(proof / ("common-default-" + phase + ".json"), {"python": "3.14.8", "python_env": "3.14.8", "pass": True})

                def execute(command, **kwargs):
                    if command[0] == "controlled-syft":
                        self.assertEqual(command, ["controlled-syft", "scan", "docker:" + IMAGE, "--platform", "linux/arm64",
                                                   "-o", "syft-json=" + str(root / "generated.syft.json"),
                                                   "-o", "spdx-json=" + str(root / "generated.spdx.json")])
                        self.assertEqual(kwargs["env"]["SYFT_CHECK_FOR_APP_UPDATE"], "false")
                        qualification.dump(root / "generated.spdx.json", data)
                        qualification.dump(root / "generated.syft.json", source)
                    else:
                        self.assertEqual(command[0], "dpkg")
                    return subprocess.CompletedProcess(command, 0)

                args = argparse.Namespace(syft="controlled-syft", image=IMAGE, platform="linux/arm64", evidence=str(root))
                with patch.object(qualification.subprocess, "run", side_effect=execute):
                    if valid:
                        qualification.sbom(args)
                    else:
                        with self.assertRaises(ValueError):
                            qualification.sbom(args)
                self.assertEqual(json.loads(gzip.decompress((root / "generated.spdx.json.gz").read_bytes())), data)
                self.assertEqual((root / "generated-spdx-binding.json").exists(), valid)
                self.assertTrue((root / "generated.spdx.json").exists())
                self.assertTrue((root / "generated.syft.json").exists())
                self.assertEqual(json.loads(gzip.decompress((root / "generated.syft.json.gz").read_bytes())), source)
                custody = json.loads((root / "generated-sbom-custody.json").read_text())
                self.assertTrue(all(custody[kind]["roundtrip"] for kind in ("syft", "spdx")))

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

    def test_memory_observation_uses_visible_remaining_limit(self):
        host = "MemTotal: 16384 kB\nMemAvailable: 8192 kB\n"
        measured = qualification.memory_capacity(host, "2097152", "1572864")
        self.assertEqual(measured["host_mem_available_bytes"], 8388608)
        self.assertEqual(measured["observed_headroom_hint_bytes"], 524288)
        self.assertEqual(qualification.memory_capacity(host, "max", "100")["observed_headroom_hint_bytes"], 8388608)
        self.assertEqual(qualification.memory_capacity(host, "100", "101")["observed_headroom_hint_bytes"], 0)
        self.assertIsNone(qualification.memory_capacity("", "", "")["observed_headroom_hint_bytes"])
        with self.assertRaises(ValueError):
            qualification.memory_capacity(host, "-1", "0")

    def test_capacity_snapshot_is_read_only_and_not_a_reservation(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "inputs.json").write_text(json.dumps({"owner": OWNER}))
            with patch.object(subprocess, "run", side_effect=AssertionError("capacity must not create or inspect Docker resources")):
                qualification.capacity(root, "before-builder")
            report = json.loads((root / "capacity-before-builder.json").read_text())
            self.assertEqual(report["owner"], OWNER)
            self.assertEqual({row["name"] for row in report["filesystems"]}, {"evidence", "workspace", "temporary"})
            self.assertTrue(all(0 <= row["free_bytes"] <= row["total_bytes"] for row in report["filesystems"]))
            self.assertIn("not additive", report["scope"])
            self.assertIn("not observed", report["daemon_filesystem_and_builder_cache"])
            (root / "inputs.json").write_text(json.dumps({"owner": "foreign"}))
            with self.assertRaises(ValueError):
                qualification.capacity(root, "after-tests")
            self.assertFalse((root / "capacity-after-tests.json").exists())


if __name__ == "__main__":
    unittest.main()
