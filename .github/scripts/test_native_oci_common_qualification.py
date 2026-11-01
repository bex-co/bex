"""Local command/source controls only; no images, registry or actual SBOM claim."""
import argparse
import gzip
import importlib.util
import json
import os
import re
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
publisher.ROOT = HERE.parents[1]
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
        with patch.dict(os.environ, env), patch.object(qualification, "source_inventory", return_value={"CONTROL_ONLY": "host init fixture"}):
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
        for fragment in ("needs: [plan, publish]", "needs.publish.outputs.ref != ''", "arch: [amd64, arm64]", "max-parallel: 1", "use: false", "cleanup: false", "BEX_NATIVE_TEST_PREPARE_WHEELS:", "BEX_NATIVE_TEST_SYFT:", "if: always() && steps.owned.outputs.root != ''"):
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


class BuildKitBootstrapControls(unittest.TestCase):
    """Only fake pull processes and retained policy inputs; no Docker/network."""
    def fixture(self, directory):
        temporary = Path(directory).resolve()
        root, inputs, config = QualificationControls().initialize_owned(temporary)
        source = temporary / "source"
        policy = source / "lego/operator/internal/build/toolchain-freshness.json"
        policy.parent.mkdir(parents=True)
        policy.write_bytes((publisher.ROOT / "lego/operator/internal/build/toolchain-freshness.json").read_bytes())
        output = temporary / "bootstrap-output"
        return root, inputs, config, policy, source / ".github/scripts/native-common-qualify.py", output

    @staticmethod
    def result(code, bound=None, absent=True):
        return ({"exit_code": code, "bound": bound, "direct_child_reaped": True,
                 "process_group_absent": absent}, b"CONTROL stdout\xff\n", b"CONTROL stderr\n")

    def test_pinned_success_records_full_policy_and_emits_only_after_pull(self):
        with tempfile.TemporaryDirectory() as directory:
            root, inputs, config, policy, helper, output = self.fixture(directory)
            pin = next(row["committed"] for row in json.loads(policy.read_text())["images"] if row["id"] == "helper-buildkit")
            def controlled(command, environment, cancellation):
                self.assertFalse(output.exists())
                self.assertEqual(command, ["timeout", "--kill-after=10s", "180s", "docker", "pull", "--quiet", pin])
                self.assertEqual(environment["DOCKER_CONFIG"], str(config))
                self.assertEqual(json.loads((config / ".qualification-owner.json").read_text())["owner"], inputs["owner"])
                return self.result(0)
            with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                 patch.object(qualification, "pull_buildkit", side_effect=controlled) as pull, patch.object(qualification.time, "sleep") as sleep:
                qualification.bootstrap(root)
            self.assertEqual(pull.call_count, 1)
            sleep.assert_not_called()
            self.assertEqual(output.read_text(), "image=" + pin + "\n")
            receipt = json.loads((root / "bootstrap.json").read_text())
            self.assertEqual(receipt["policy_sha256"], qualification.sha(policy))
            self.assertEqual(receipt["owner"], inputs["owner"])
            self.assertTrue(receipt["image_output_emitted"])
            self.assertEqual((root / receipt["attempts"][0]["stdout"]["path"]).read_bytes(), b"CONTROL stdout\xff\n")
            qualification.manifest(root)
            self.assertIn("bootstrap-1.stdout.log", (root / "MANIFEST.json").read_text())

    def test_timeout_retries_are_preserved_before_success(self):
        with tempfile.TemporaryDirectory() as directory:
            root, inputs, config, policy, helper, output = self.fixture(directory)
            results = iter([self.result(124), self.result(137), self.result(0)])
            def controlled(*args):
                self.assertFalse(output.exists())
                return next(results)
            with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                 patch.object(qualification, "pull_buildkit", side_effect=controlled), patch.object(qualification.time, "sleep") as sleep:
                qualification.bootstrap(root)
            rows = json.loads((root / "bootstrap.json").read_text())["attempts"]
            self.assertEqual([row["exit_code"] for row in rows], [124, 137, 0])
            self.assertEqual([row["timeout_exit"] for row in rows], [True, True, False])
            self.assertEqual([call.args for call in sleep.call_args_list], [(1,)] * 30)
            for row in rows:
                for label in ("stdout", "stderr"):
                    self.assertEqual(row[label]["sha256"], qualification.sha(root / row[label]["path"]))

    def test_all_failed_attempts_refuse_output_and_remain_manifested(self):
        with tempfile.TemporaryDirectory() as directory:
            root, inputs, config, policy, helper, output = self.fixture(directory)
            with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                 patch.object(qualification, "pull_buildkit", side_effect=[self.result(1), self.result(124), self.result(137)]), \
                 patch.object(qualification.time, "sleep") as sleep:
                with self.assertRaisesRegex(RuntimeError, "all three"):
                    qualification.bootstrap(root)
            self.assertFalse(output.exists())
            receipt = json.loads((root / "bootstrap.json").read_text())
            self.assertEqual([row["exit_code"] for row in receipt["attempts"]], [1, 124, 137])
            self.assertFalse(receipt["image_output_emitted"])
            qualification.manifest(root)
            self.assertIn("bootstrap-3.stderr.log", (root / "MANIFEST.json").read_text())

    def test_outer_bound_or_unreaped_group_cannot_emit_success(self):
        for result in (self.result(0, "python-timeout"), self.result(0, "remaining-process-group"), self.result(0, absent=False), self.result(False)):
            with self.subTest(result=result[0]), tempfile.TemporaryDirectory() as directory:
                root, inputs, config, policy, helper, output = self.fixture(directory)
                with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                     patch.object(qualification, "pull_buildkit", return_value=result), patch.object(qualification.time, "sleep"):
                    with self.assertRaises(RuntimeError):
                        qualification.bootstrap(root)
                self.assertFalse(output.exists())

    def test_mutable_foreign_missing_or_ambiguous_pin_refuses_before_command(self):
        for mutation in ("mutable", "foreign", "missing", "duplicate"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                root, inputs, config, policy, helper, output = self.fixture(directory)
                value = json.loads(policy.read_text())
                row = next(row for row in value["images"] if row["id"] == "helper-buildkit")
                if mutation == "mutable":
                    row["committed"] = "moby/buildkit:buildx-stable-1"
                elif mutation == "foreign":
                    row["committed"] = "other/buildkit:v0.30.0@sha256:" + "a" * 64
                elif mutation == "missing":
                    value["images"].remove(row)
                else:
                    value["images"].append(dict(row))
                policy.write_text(json.dumps(value))
                with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                     patch.object(qualification, "pull_buildkit") as pull:
                    with self.assertRaises(ValueError):
                        qualification.bootstrap(root)
                pull.assert_not_called()
                self.assertFalse(output.exists())

    def test_invalid_owner_missing_scope_or_changed_marker_refuses_before_command(self):
        for mutation in ("owner", "missing", "foreign", "marker", "symlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                root, inputs, config, policy, helper, output = self.fixture(directory)
                if mutation == "owner":
                    inputs["owner"] = "foreign"
                elif mutation == "missing":
                    (config / ".qualification-owner.json").unlink()
                    config.rmdir()
                elif mutation == "foreign":
                    inputs["docker_config"] = str(config.parent / "foreign")
                elif mutation == "marker":
                    (config / ".qualification-owner.json").write_text('{}')
                else:
                    actual = config.with_name(config.name + "-moved")
                    config.rename(actual)
                    config.symlink_to(actual, target_is_directory=True)
                (root / "inputs.json").write_text(json.dumps(inputs))
                with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                     patch.object(qualification, "pull_buildkit") as pull:
                    with self.assertRaises(ValueError):
                        qualification.bootstrap(root)
                pull.assert_not_called()
                self.assertFalse(output.exists())

    def test_python_outer_timeout_kills_even_exited_leader_group(self):
        class ControlledProcess:
            pid = 404040
            returncode = 0  # Leader has exited; inherited output pipe remains held.
            def __init__(self):
                self.calls = []
            def communicate(self, timeout):
                self.calls.append(timeout)
                if len(self.calls) == 1:
                    raise subprocess.TimeoutExpired("CONTROL", timeout, output=b"partial stdout", stderr=b"partial stderr")
                return b"partial stdout", b"partial stderr"
            def wait(self, timeout):
                return self.returncode
        process = ControlledProcess()
        seen = []
        def group(pid, sig):
            self.assertEqual(pid, process.pid)
            seen.append(sig)
            if len(seen) >= 3:
                raise ProcessLookupError
        with patch.object(qualification.subprocess, "Popen", return_value=process) as popen, patch.object(qualification.os, "killpg", side_effect=group), \
             patch.object(qualification.time, "monotonic", side_effect=[0, 0, 196, 197]):
            actual, stdout, stderr = qualification.pull_buildkit(["CONTROL"], {"CONTROL": "fake"})
        self.assertTrue(popen.call_args.kwargs["start_new_session"])
        self.assertEqual(process.calls, [1, 5])
        self.assertEqual(seen, [0, qualification.signal.SIGKILL, 0])
        self.assertEqual(actual["bound"], "python-timeout")
        self.assertTrue(actual["process_group_absent"])
        self.assertEqual((stdout, stderr), (b"partial stdout", b"partial stderr"))

    def test_nominal_success_requires_absent_group_and_canonical_exit(self):
        class ControlledProcess:
            pid = 404041
            returncode = 0
            def communicate(self, timeout):
                return b"complete stdout", b"complete stderr"
            def wait(self, timeout):
                return 0
        with patch.object(qualification.subprocess, "Popen", return_value=ControlledProcess()), \
             patch.object(qualification.os, "killpg", side_effect=ProcessLookupError):
            actual, stdout, stderr = qualification.pull_buildkit(["CONTROL"], {})
        self.assertEqual(actual, {"exit_code": 0, "bound": None, "cancel_signal": None, "direct_child_reaped": True, "process_group_absent": True})
        self.assertEqual((stdout, stderr), (b"complete stdout", b"complete stderr"))

    def test_real_exited_leader_pipeholder_is_stopped_without_docker(self):
        # Only an owned Python fixture runs; its descendant holds inherited pipes.
        actual_popen = subprocess.Popen
        processes = []
        timed_out = [False]
        actual_clock = qualification.time.monotonic
        def shortened(*args, **kwargs):
            process = actual_popen(*args, **kwargs)
            communicate = process.communicate
            def bounded(timeout):
                try:
                    return communicate(timeout=min(timeout, 0.2))
                except subprocess.TimeoutExpired:
                    timed_out[0] = True
                    raise
            process.communicate = bounded
            processes.append(process)
            return process
        script = ("import json,os,pathlib,subprocess,sys; "
                  "child=subprocess.Popen([sys.executable,'-c','import time;time.sleep(30)']); "
                  "pathlib.Path(sys.argv[1]).write_text(json.dumps({'child':child.pid,'group':os.getpgrp()}))")
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "owned-process.json"
            try:
                with patch.object(qualification.subprocess, "Popen", side_effect=shortened), \
                     patch.object(qualification.time, "monotonic", side_effect=lambda: actual_clock() + (196 if timed_out[0] else 0)):
                    actual, stdout, stderr = qualification.pull_buildkit([sys.executable, "-c", script, str(journal)], dict(os.environ))
                recorded = json.loads(journal.read_text())
                self.assertEqual(recorded["group"], processes[0].pid)
                self.assertEqual(actual["bound"], "python-timeout")
                self.assertTrue(actual["direct_child_reaped"])
                self.assertTrue(actual["process_group_absent"])
                with self.assertRaises(ProcessLookupError):
                    os.killpg(recorded["group"], 0)
            finally:
                for process in processes:
                    try:
                        os.killpg(process.pid, qualification.signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    process.wait(timeout=2)

    def test_permission_unknown_group_is_bounded_failure(self):
        class ControlledProcess:
            pid = 404042
            returncode = 0
            def communicate(self, timeout):
                return b"complete stdout", b"complete stderr"
            def wait(self, timeout):
                return 0
        with patch.object(qualification.subprocess, "Popen", return_value=ControlledProcess()), \
             patch.object(qualification.os, "killpg", side_effect=PermissionError), \
             patch.object(qualification.time, "monotonic", side_effect=[0, 0, 0, 6]):
            actual, stdout, stderr = qualification.pull_buildkit(["CONTROL"], {})
        self.assertEqual(actual["bound"], "process-group-permission")
        self.assertFalse(actual["process_group_absent"])

    def test_real_sigint_and_sigterm_cancel_once_record_and_restore_handlers(self):
        import signal
        import time
        runner_source = r"""import importlib.util,json,os,pathlib,signal,subprocess,sys,time
helper,logical,root,journal,result,phase=sys.argv[1:]
spec=importlib.util.spec_from_file_location('controlled_bootstrap',helper)
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
m.__file__=logical
before={s:signal.getsignal(s) for s in (signal.SIGINT,signal.SIGTERM)}
real_popen=subprocess.Popen
program='import time;print("CONTROL owned pull",flush=True);time.sleep(30)'
def controlled(command,**kwargs):
 assert command[:6]==['timeout','--kill-after=10s','180s','docker','pull','--quiet']
 process=real_popen([sys.executable,'-c',program],**kwargs)
 pathlib.Path(journal).write_text(json.dumps({'pid':process.pid,'group':process.pid}))
 if phase=='registration':time.sleep(.3)
 return process
m.subprocess.Popen=controlled
caught=None
try:m.bootstrap(pathlib.Path(root))
except RuntimeError as error:caught=type(error).__name__
pathlib.Path(result).write_text(json.dumps({'caught':caught,'handlers_restored':all(signal.getsignal(s)==h for s,h in before.items())}))
"""
        for phase, signum in (("registration", signal.SIGTERM), ("communicate", signal.SIGINT)):
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as directory:
                root, inputs, config, policy, logical, output = self.fixture(directory)
                journal, result = Path(directory) / "owned-pull.json", Path(directory) / "runner-result.json"
                environment = dict(os.environ, GITHUB_OUTPUT=str(output), PYTHONDONTWRITEBYTECODE="1")
                runner = subprocess.Popen([sys.executable, "-c", runner_source, qualification.__file__, str(logical), str(root),
                                           str(journal), str(result), phase], env=environment, stdout=subprocess.PIPE,
                                          stderr=subprocess.PIPE, start_new_session=True)
                child_group = None
                try:
                    deadline = time.monotonic() + 3
                    while not journal.exists() and runner.poll() is None and time.monotonic() < deadline:
                        time.sleep(.01)
                    self.assertTrue(journal.exists(), "owned fake pull did not register")
                    child_group = json.loads(journal.read_text())["group"]
                    os.kill(runner.pid, signum)
                    stdout, stderr = runner.communicate(timeout=8)
                    self.assertEqual(runner.returncode, 0, stderr)
                    report = json.loads(result.read_text())
                    self.assertEqual(report, {"caught": "RuntimeError", "handlers_restored": True})
                    receipt = json.loads((root / "bootstrap.json").read_text())
                    self.assertEqual(receipt["cancel_signal"], signum)
                    self.assertEqual(len(receipt["attempts"]), 1)
                    self.assertEqual(receipt["attempts"][0]["cancel_signal"], signum)
                    self.assertTrue(receipt["attempts"][0]["direct_child_reaped"])
                    self.assertTrue(receipt["attempts"][0]["process_group_absent"])
                    self.assertFalse(receipt["image_output_emitted"])
                    self.assertFalse(output.exists())
                    with self.assertRaises(ProcessLookupError):os.killpg(child_group, 0)
                finally:
                    for group in (child_group, runner.pid):
                        if group is not None:
                            try:os.killpg(group, signal.SIGKILL)
                            except ProcessLookupError:pass
                    runner.communicate(timeout=2)

    def test_partial_signal_installation_restores_each_saved_handler_before_spawn(self):
        for target in ("pull", "bootstrap"):
            with self.subTest(target=target), tempfile.TemporaryDirectory() as directory:
                root, inputs, config, policy, helper, output = self.fixture(directory)
                originals = {qualification.signal.SIGINT: object(), qualification.signal.SIGTERM: object()}
                current = dict(originals)
                failed = [False]
                def install(signum, handler):
                    current[signum] = handler
                    if signum == qualification.signal.SIGTERM and handler is not originals[signum] and not failed[0]:
                        failed[0] = True
                        raise RuntimeError("CONTROL partial signal installation")
                with patch.object(qualification.signal, "getsignal", side_effect=lambda s: current[s]), \
                     patch.object(qualification.signal, "signal", side_effect=install), \
                     patch.object(qualification.subprocess, "Popen") as popen, \
                     patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)):
                    with self.assertRaisesRegex(RuntimeError, "partial"):
                        if target == "pull":qualification.pull_buildkit(["CONTROL"], {})
                        else:qualification.bootstrap(root)
                self.assertEqual(current, originals)
                popen.assert_not_called()
                self.assertFalse(output.exists())

    def test_signal_before_pid_registration_prevents_spawn_and_restores_handlers(self):
        originals = {s: qualification.signal.getsignal(s) for s in (qualification.signal.SIGINT, qualification.signal.SIGTERM)}
        actual_signal = qualification.signal.signal
        triggered = [False]
        def install(signum, handler):
            result = actual_signal(signum, handler)
            if signum == qualification.signal.SIGINT and callable(handler) and handler is not originals[signum] and not triggered[0]:
                triggered[0] = True
                handler(signum, None)
            return result
        with patch.object(qualification.signal, "signal", side_effect=install), patch.object(qualification.subprocess, "Popen") as popen:
            result, stdout, stderr = qualification.pull_buildkit(["CONTROL"], {})
        self.assertEqual(result["cancel_signal"], qualification.signal.SIGINT)
        self.assertEqual(result["bound"], "cancelled")
        self.assertTrue(result["process_group_absent"])
        popen.assert_not_called()
        self.assertEqual({s: qualification.signal.getsignal(s) for s in originals}, originals)

    def test_late_cancel_during_pull_cleanup_or_handler_restoration_blocks_output(self):
        for phase in ("cleanup", "restoration"):
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as directory:
                root, inputs, config, policy, helper, output = self.fixture(directory)
                originals = {s: qualification.signal.getsignal(s) for s in (qualification.signal.SIGINT, qualification.signal.SIGTERM)}
                actual_signal = qualification.signal.signal
                triggered = [False]
                def install(signum, handler):
                    active = qualification.signal.getsignal(signum)
                    if (phase == "restoration" and handler is originals[signum] and active is not handler
                            and not triggered[0]):
                        triggered[0] = True
                        active(signum, None)
                    return actual_signal(signum, handler)
                def controlled(command, environment, cancellation):
                    if phase == "cleanup":cancellation["signal"] = qualification.signal.SIGTERM
                    return self.result(0)
                with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                     patch.object(qualification, "pull_buildkit", side_effect=controlled) as pull, \
                     patch.object(qualification.signal, "signal", side_effect=install):
                    with self.assertRaisesRegex(RuntimeError, "cancelled"):
                        qualification.bootstrap(root)
                self.assertEqual(pull.call_count, 1)
                self.assertFalse(output.exists())
                self.assertIsNotNone(json.loads((root / "bootstrap.json").read_text())["cancel_signal"])
                self.assertEqual({s: qualification.signal.getsignal(s) for s in originals}, originals)

    def test_retry_delay_cancellation_records_last_attempt_without_retry(self):
        with tempfile.TemporaryDirectory() as directory:
            root, inputs, config, policy, helper, output = self.fixture(directory)
            def cancel_delay(seconds):
                self.assertEqual(seconds, 1)
                qualification.signal.getsignal(qualification.signal.SIGINT)(qualification.signal.SIGINT, None)
            with patch.object(qualification, "__file__", str(helper)), patch.dict(os.environ, GITHUB_OUTPUT=str(output)), \
                 patch.object(qualification, "pull_buildkit", return_value=self.result(1)) as pull, \
                 patch.object(qualification.time, "sleep", side_effect=cancel_delay):
                with self.assertRaisesRegex(RuntimeError, "cancelled"):
                    qualification.bootstrap(root)
            self.assertEqual(pull.call_count, 1)
            receipt = json.loads((root / "bootstrap.json").read_text())
            self.assertEqual(receipt["cancel_signal"], qualification.signal.SIGINT)
            self.assertEqual(len(receipt["attempts"]), 1)
            self.assertFalse(output.exists())

    def test_corrected_patterns_are_the_exact_closed_reset_and_recursive_set(self):
        workflow = (publisher.ROOT / ".github/workflows/native-common-image.yml").read_text()
        text = workflow.split("          sparse-checkout: |\n", 1)[1]
        patterns = []
        for line in text.splitlines():
            if not line.startswith("            "):break
            patterns.append(line[12:])
        raw = ("\n".join(patterns) + "\n").encode()
        self.assertEqual(qualification.hashlib.sha256(raw).hexdigest(), "574ae1c282c7e7c7cfd0908ea303b4f49ae124e080f489e5cde87f22379e79aa")
        self.assertEqual(len(patterns), 47)
        self.assertEqual(patterns[0], "!/**")
        self.assertEqual(sum(p.endswith('/**') and not p.startswith('!') for p in patterns), 14)
        self.assertIn('/lego/operator/internal/build/**', patterns)
        self.assertIn('/.pm/w8/m91/evidence/**', patterns)

    def test_both_bootstrap_stanzas_use_only_successful_output_and_preserve_auth_order(self):
        jobs = ProfileRoutingControls.jobs()
        for name in ("qualify", "qualify-profile"):
            body = jobs[name]
            prepull = body.split("      - id: buildkit\n", 1)[1].split("      - name: log in to GHCR", 1)[0]
            self.assertIn("timeout-minutes: 11", prepull)
            self.assertIn('native-common-qualify.py bootstrap "$QUALIFICATION_ROOT"', prepull)
            self.assertLess(body.index("id: owned"), body.index("id: buildkit"))
            self.assertLess(body.index("id: buildkit"), body.index("name: log in to GHCR"))
            buildx = body.split("      - uses: docker/setup-buildx-action@", 1)[1].split("      - ", 1)[0]
            self.assertIn("timeout-minutes: 5", buildx)
            self.assertIn("driver-opts: image=${{ steps.buildkit.outputs.image }}", buildx)
            self.assertNotIn("buildx-stable", body)
            self.assertNotIn("continue-on-error", body)

    def test_all_three_checkouts_use_the_same_closed_noncone_set(self):
        workflow = (publisher.ROOT / ".github/workflows/native-common-image.yml").read_text()
        blocks = re.findall(r'(?ms)^      - uses: actions/checkout@[^\n]+\n(.*?)(?=^      - |\Z)', workflow)
        self.assertEqual(len(blocks), 3)
        self.assertEqual(blocks, [blocks[0]] * 3)
        self.assertIn("sparse-checkout-cone-mode: false", blocks[0])
        self.assertIn("persist-credentials: false", blocks[0])
        for literal in ("/.github/", "/.pm/w8/m91/evidence/", "/lego/operator/internal/build/", "/lego/operator/cmd/native-init/", "/lego/types/v1alpha1/", "/lego/operator/go.mod", "/lego/types/go.sum", "/scripts/lib/toolchain-freshness.py"):
            self.assertIn("            " + literal + "\n", blocks[0])
        self.assertNotIn("fetch-depth:", blocks[0])
        self.assertNotIn("${{", blocks[0])


class ProfileRoutingControls(unittest.TestCase):
    @staticmethod
    def jobs():
        workflow = (publisher.ROOT / ".github/workflows/native-common-image.yml").read_text()
        names = list(re.finditer(r"(?m)^  ([a-z][a-z-]*):\n", workflow.split("jobs:\n", 1)[1]))
        body = workflow.split("jobs:\n", 1)[1]
        return {match[1]: body[match.end():names[index + 1].start() if index + 1 < len(names) else len(body)]
                for index, match in enumerate(names)}

    @staticmethod
    def run_block(job, name):
        tail = job.split("      - name: " + name + "\n", 1)[1].split("        run: |\n", 1)[1]
        lines = []
        for line in tail.splitlines():
            if line and not line.startswith("          "):
                break
            lines.append(line[10:])
        return "\n".join(lines) + "\n"

    def test_all_routes_are_exact_single_cases_and_bounds_are_fixed(self):
        expected = {"node-addon": (4, 120, 150), "node": (8, 90, 120), "modern-python": (5, 110, 140),
                    "legacy-python": (2, 110, 140), "ruby": (8, 75, 105),
                    "go-rust": (2, 75, 105), "elixir": (5, 90, 120), "native-init": (1, 60, 90)}
        self.assertEqual(set(qualification.PROFILES), set(expected))
        for profile, (test, flag, cases, minutes, job_minutes) in qualification.PROFILES.items():
            self.assertEqual((len(cases), minutes, job_minutes), expected[profile])
            self.assertEqual(qualification.profile_selection(profile, "pilot"), qualification.profile_selection(profile, cases[0]))
            for case in cases:
                selected = qualification.profile_selection(profile, case)
                self.assertEqual(selected["case"], case)
                self.assertEqual(selected["flag"], flag)
                self.assertEqual(selected["go_timeout"], str(minutes) + "m")
                self.assertEqual(selected["job_minutes"], str(job_minutes))
                self.assertIn("not full-family acceptance", selected["scope"])
                self.assertEqual(selected["selector"].split("/", 1)[0], "^" + test + "$")
                if profile != "native-init":
                    pattern = selected["selector"].split("/", 1)[1]
                    self.assertIsNotNone(re.fullmatch(pattern, case))
                    self.assertIsNone(re.fullmatch(pattern, case + "-neighbor"))
                    if "." in case:
                        self.assertIsNone(re.fullmatch(pattern, case.replace(".", "x")))

    def test_every_route_tracks_an_existing_go_test_and_actual_subtest_contract(self):
        build = publisher.ROOT / "lego/operator/internal/build"
        table = (build / "native.go").read_text()
        def table_lines(runtime):
            block = table.split(runtime + ": {", 1)[1].split("\n\t}},", 1)[0]
            return re.findall(r'\{line: "([^"]+)"', block)
        paths = {"node-addon": "nativecommon_nodeaddon_qualification_test.go", "node": "nativecommon_node_qualification_test.go", "modern-python": "nativemodern_python_test.go",
                 "legacy-python": "nativelegacy_python_test.go", "ruby": "nativecommon_ruby_test.go",
                 "go-rust": "nativecommon_gorust_test.go", "elixir": "nativecommon_elixir_qualification_test.go",
                 "native-init": "nativequalification_init_test.go"}
        for profile, name in paths.items():
            raw = (build / name).read_text()
            test, flag, cases, _, _ = qualification.PROFILES[profile]
            self.assertIn("func " + test + "(t *testing.T)", raw)
            self.assertIn('"' + flag + '"', raw)
            if profile == "node-addon":
                actual = re.findall(r'\{name: "([^"]+)", runtime:', raw)
                self.assertIn('t.Run(tc.name,', raw)
                self.assertNotIn('BEX_NATIVE_TEST_NODE14_TREE', raw)
            elif profile == "node":
                lines = re.search(r'for _, line := range \[\]string\{([^}]+)\}', raw)[1]
                actual = ["node-" + line for line in re.findall(r'"([^"]+)"', lines)]
                actual += re.findall(r'commonNodeCase\{"([^"]+)",', raw)
            elif profile == "modern-python":
                actual = re.findall(r'"([^"]+)"', re.search(r'var nativeModernPythonLines = \[\]string\{([^}]+)\}', raw)[1])
                self.assertIn('t.Run(name,', raw)
            elif profile in ("legacy-python", "go-rust"):
                variable = "line" if profile == "legacy-python" else "runtime"
                loop = re.search(r'for _, ' + variable + r' := range \[\]string\{([^}]+)\}', raw)[1]
                actual = re.findall(r'"([^"]+)"', loop)
                self.assertIn("t.Run(" + variable + ",", raw)
            elif profile == "ruby":
                actual = ["ruby" + line.replace(".", "") for line in table_lines("nativeRubyRuntime")]
                actual += re.findall(r'nativeRubyQualificationCase\{name: "([^"]+)",', raw)
                self.assertIn('name: "ruby" + strings.ReplaceAll(line.line, ".", "")', raw)
            elif profile == "elixir":
                actual = ["elixir-" + line for line in table_lines("nativeElixirRuntime")]
                actual += re.findall(r'commonElixirCase\{name: "([^"]+)",', raw)
                self.assertIn('name: "elixir-" + line.line', raw)
            else:
                actual = ["all-launches"]
                self.assertIn('[]string{"npm", "yarn", "pnpm", "direct"}', raw)
            self.assertEqual(set(cases), set(actual), profile)
            self.assertEqual(len(cases), len(actual), profile)

    def test_invalid_selections_fail_before_any_output_or_resource_operation(self):
        for profile, case in (("all", "pilot"), ("node", "all"), ("ruby", "node-14"),
                              ("node", "node-14/.*"), ("modern-python", "3x9"),
                              ("utilities", "node-14"), ("native-init", "npm"),
                              ("node\nother=1", "pilot"), ("ruby", "$(echo bad)"),
                              ("node", "ghcr.io/other@sha256:" + "a" * 64)):
            with self.subTest(profile=profile, case=case), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                env = dict(GITHUB_EVENT_NAME="workflow_dispatch", REQUESTED_PROFILE=profile,
                           REQUESTED_CASE=case, GITHUB_OUTPUT=str(output))
                with patch.dict(os.environ, env), patch.object(qualification.tempfile, "mkdtemp", side_effect=AssertionError("no resources")), patch.object(qualification.subprocess, "run", side_effect=AssertionError("no commands")):
                    with self.assertRaises(ValueError):
                        qualification.plan()
                self.assertFalse(output.exists())
        with patch.dict(os.environ, GITHUB_EVENT_NAME="push", REQUESTED_PROFILE="node", REQUESTED_CASE="pilot"):
            with self.assertRaises(ValueError):
                qualification.plan()

    def test_plan_cli_outputs_only_validated_values_and_push_keeps_full_utilities(self):
        for event, profile, case in (("push", "utilities", "pilot"), ("workflow_dispatch", "utilities", "all"),
                                     ("workflow_dispatch", "modern-python", "3.14")):
            with self.subTest(event=event, profile=profile), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "outputs"
                env = dict(os.environ, GITHUB_EVENT_NAME=event, REQUESTED_PROFILE=profile, REQUESTED_CASE=case,
                           GITHUB_OUTPUT=str(output), PYTHONDONTWRITEBYTECODE="1")
                process = subprocess.run([sys.executable, "-B", str(HERE / "native-common-qualify.py"), "plan"],
                                         env=env, text=True, capture_output=True, timeout=10)
                self.assertEqual(process.returncode, 0, process.stderr)
                actual = dict(line.split("=", 1) for line in output.read_text().splitlines())
                expected = qualification.profile_selection(profile, case)
                self.assertEqual(actual, dict({key: expected[key] for key in ("profile", "case", "flag", "selector", "go_timeout", "job_minutes")},
                                              source="fresh", ref="", producer="", verification="", receipt_sha256="", baseline_status="fresh-execution"))
                if profile == "utilities":
                    self.assertEqual(actual["case"], "all")
                    self.assertEqual(actual["selector"], "^(TestNativeUtilityGeneratedImages|TestNativeOriginalReplayHTTPOracles)$")

    def test_profile_environment_clears_all_other_opt_ins_and_rejects_original_gate(self):
        flags = {value[1] for value in qualification.PROFILES.values()}
        for profile in qualification.PROFILES:
            with self.subTest(profile=profile), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "environment"
                with patch.dict(os.environ, QUALIFICATION_PROFILE=profile, QUALIFICATION_CASE="pilot", GITHUB_ENV=str(output)):
                    qualification.profile_environment()
                actual = dict(line.split("=", 1) for line in output.read_text().splitlines())
                selected = qualification.profile_selection(profile, "pilot")
                self.assertEqual({name for name in flags if actual[name] == "1"}, {selected["flag"]})
                self.assertTrue(all(actual[name] == "0" for name in flags - {selected["flag"]}))
                self.assertEqual(actual["QUALIFICATION_SELECTOR"], selected["selector"])
                self.assertEqual(actual["QUALIFICATION_CASE"], selected["case"])
                self.assertEqual(actual["BEX_BUILDKIT_INTEGRATION"], "1")
        with patch.dict(os.environ, QUALIFICATION_PROFILE="utilities", QUALIFICATION_CASE="all"):
            with self.assertRaises(ValueError):
                qualification.profile_environment()

    def test_init_records_selected_measurement_bounds_and_generator_before_auth(self):
        with tempfile.TemporaryDirectory() as directory:
            selected = qualification.profile_selection("modern-python", "3.9")
            with patch.dict(os.environ, QUALIFICATION_PROFILE="modern-python", QUALIFICATION_CASE="3.9"):
                root, inputs, config = QualificationControls().initialize_owned(Path(directory).resolve())
            self.assertEqual(inputs["selection"], selected)
            self.assertEqual(inputs["generator_source"], "a" * 40)
            self.assertEqual(inputs["producer"], inputs["verification"])
            self.assertNotEqual(config, root)
        with patch.dict(os.environ, QUALIFICATION_PROFILE="node", QUALIFICATION_CASE="all"), patch.object(qualification.tempfile, "mkdtemp", side_effect=AssertionError("no resource creation")):
            with self.assertRaises(ValueError):
                qualification.initialize()

    def test_workflow_preserves_original_job_and_requires_both_architectures_before_pilot(self):
        jobs = self.jobs()
        self.assertEqual(set(jobs), {"plan", "publish", "qualify", "qualify-profile"})
        self.assertIn("needs: plan\n", jobs["publish"])
        self.assertIn("runtime: common-bookworm\n", jobs["publish"])
        self.assertIn("needs.plan.outputs.source == 'fresh'", jobs["publish"])
        self.assertEqual(sum("uses: ./.github/workflows/native-base-image.yml" in body for body in jobs.values()), 1)
        plan = jobs["plan"]
        self.assertNotIn("packages:", plan)
        self.assertNotIn("uses: docker/", plan)
        self.assertEqual(re.findall(r"(?m)^        run: (.+)$", plan),
                         ["python3 .github/scripts/native-common-qualify.py plan"])
        self.assertIn("REQUESTED_PROFILE: ${{ inputs.profile || 'utilities' }}", plan)
        self.assertIn("REQUESTED_CASE: ${{ inputs.case || 'pilot' }}", plan)
        self.assertIn("native-common-qualify.py plan", plan)
        original = jobs["qualify"]
        self.assertIn("needs: [plan, publish]\n", original)
        self.assertIn("timeout-minutes: 180", original)
        self.assertIn("BEX_NATIVE_TEST_PREPARE_WHEELS:", original)
        self.assertIn("BEX_NATIVE_TEST_SYFT:", original)
        self.assertIn("syft-version: v1.55.0", original)
        command = self.run_block(original, "actual generated offline builds, original replays and exact-default SPDX")
        self.assertIn("-timeout 150m", command)
        self.assertIn("-run '^(TestNativeUtilityGeneratedImages|TestNativeOriginalReplayHTTPOracles)$'", command)
        self.assertNotIn("QUALIFICATION_SELECTOR", command)
        pilot = jobs["qualify-profile"]
        self.assertIn("needs: [plan, publish, qualify]", pilot)
        self.assertIn("github.event_name == 'workflow_dispatch'", pilot)
        self.assertIn("needs.plan.outputs.profile != 'utilities'", pilot)
        self.assertIn("needs.qualify.result == 'success'", pilot)
        self.assertIn("needs.publish.outputs.ref != ''", pilot)
        self.assertIn("always() && !cancelled()", pilot)
        self.assertIn("needs.publish.result == 'skipped'", pilot)
        self.assertIn("needs.qualify.result == 'skipped'", pilot)
        self.assertIn("needs.plan.outputs.receipt_sha256 != ''", pilot)
        self.assertIn("timeout-minutes: ${{ fromJSON(needs.plan.outputs.job_minutes) }}", pilot)
        for job in (original, pilot):
            for unchanged in ("max-parallel: 1", "arch: [amd64, arm64]", "use: false", "cleanup: false",
                              "packages: read"):
                self.assertIn(unchanged, job)
            for retained in ("capacity", "cleanup", "manifest"):
                self.assertIn("native-common-qualify.py " + retained, job)
            self.assertIn("if: always() && steps.owned.outputs.root != ''", job)
            self.assertIn("include-hidden-files: true", job)
        for key in ("ref", "producer", "verification"):
            self.assertIn("needs.plan.outputs.source == 'reviewed-candidate' && needs.plan.outputs." + key + " || needs.publish.outputs." + key, pilot)
        self.assertIn("QUALIFICATION_SOURCE: ${{ needs.plan.outputs.source }}", pilot)
        self.assertIn("CANDIDATE_RECEIPT_SHA256: ${{ needs.plan.outputs.receipt_sha256 }}", pilot)
        # Credential handling stays the same in both resource-bearing jobs.
        marker = "      - name: log in to GHCR using this qualification's isolated credentials\n"
        login = lambda job: job.split(marker, 1)[1].split("      - uses:", 1)[0]
        self.assertEqual(login(original), login(pilot))
        self.assertIn("common-pilot-${{ needs.plan.outputs.profile }}-${{ needs.plan.outputs.case }}-", pilot)
        self.assertNotIn("BEX_NATIVE_TEST_PREPARE_WHEELS:", pilot)
        self.assertNotIn("BEX_NATIVE_TEST_SYFT:", pilot)

    def test_actual_pilot_shell_uses_validated_env_and_retains_failed_log(self):
        shell = self.run_block(self.jobs()["qualify-profile"], "measure the selected family case without claiming full-family acceptance")
        self.assertNotIn("${{", shell)
        for profile in qualification.PROFILES:
            for exit_code in (0, 17):
                with self.subTest(profile=profile, exit=exit_code), tempfile.TemporaryDirectory() as directory:
                    temporary = Path(directory)
                    exported = temporary / "environment"
                    with patch.dict(os.environ, QUALIFICATION_PROFILE=profile, QUALIFICATION_CASE="pilot", GITHUB_ENV=str(exported)):
                        qualification.profile_environment()
                    environment = dict(line.split("=", 1) for line in exported.read_text().splitlines())
                    binary = temporary / "bin"
                    binary.mkdir()
                    stub = binary / "go"
                    stub.write_text("#!" + sys.executable + "\n" + r"""import json,os,pathlib,sys
pathlib.Path(os.environ['ARGUMENTS']).write_text(json.dumps({'argv':sys.argv[1:],'flags':{k:v for k,v in os.environ.items() if k.startswith('BEX_NATIVE_')},'integration':os.environ['BEX_BUILDKIT_INTEGRATION']}))
print('controlled case stdout',flush=True)
print('controlled case stderr',file=sys.stderr,flush=True)
sys.exit(int(os.environ['CONTROL_EXIT']))
""")
                    stub.chmod(0o755)
                    env = dict(os.environ, **environment)
                    env.update(PATH=str(binary) + os.pathsep + os.environ["PATH"], QUALIFICATION_ROOT=str(temporary),
                               ARGUMENTS=str(temporary / "arguments.json"), CONTROL_EXIT=str(exit_code))
                    process = subprocess.run(["bash", "-euo", "pipefail", "-c", shell], env=env,
                                             capture_output=True, text=True, timeout=10)
                    self.assertEqual(process.returncode, exit_code, process.stderr)
                    actual = json.loads((temporary / "arguments.json").read_text())
                    selected = qualification.profile_selection(profile, "pilot")
                    self.assertEqual(actual["argv"], ["test", "-p", "1", "-count=1", "-timeout", selected["go_timeout"],
                                                       "./internal/build", "-run", selected["selector"], "-v"])
                    self.assertEqual({key for key, value in actual["flags"].items() if key in {entry[1] for entry in qualification.PROFILES.values()} and value == "1"}, {selected["flag"]})
                    self.assertEqual(actual["integration"], "1")
                    self.assertEqual((temporary / "qualification.log").read_text(), "controlled case stdout\ncontrolled case stderr\n")


class ReviewedCandidateControls(unittest.TestCase):
    """Synthetic, real-filesystem admission controls; NOT real crypto or approval."""
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.ref = "ghcr.io/bex-co/bex-cnb-run@" + IMAGE
        self.source = "a" * 40
        self.tool_bytes = {name: ("CONTROL ONLY " + name).encode() for name in qualification.V11_TOOLS}
        self.tool_hashes = {name: qualification.hashlib.sha256(raw).hexdigest() for name, raw in self.tool_bytes.items()}
        patched = patch.dict(qualification.V11_TOOLS, self.tool_hashes, clear=True)
        patched.start()
        self.addCleanup(patched.stop)
        self.support_bytes = {name: ("CONTROL ONLY SUPPORT " + name).encode() for name in qualification.V11_SUPPORT_INPUTS}
        support_hashes = {name: qualification.hashlib.sha256(raw).hexdigest() for name, raw in self.support_bytes.items()}
        support_patch = patch.dict(qualification.V11_SUPPORT_INPUTS, support_hashes, clear=True)
        support_patch.start()
        self.addCleanup(support_patch.stop)
        self.env = patch.dict(os.environ, GITHUB_SHA="b" * 40, GITHUB_REF="refs/heads/main", GITHUB_EVENT_NAME="workflow_dispatch")
        self.env.start()
        self.addCleanup(self.env.stop)
        for relative in set(qualification.SOURCE_FILES) | qualification.IMAGE_INPUT_PATHS:
            self.write(relative, b"CONTROL ONLY SOURCE\n")
        for package in qualification.SOURCE_PACKAGES:
            self.write("lego/" + package + "/control.go", b"package control\n")
        self.write("lego/operator/internal/build/native.go", b"package build\n")
        self.originals = [{"mid": n, "runtime": "node" if n < 100 else "python"} for n in range(93, 106)]
        self.write("lego/operator/internal/build/testdata/native-utilities/original-cases.json", self.originals)
        inputs = qualification.source_inventory(self.root)
        image_inputs = qualification.image_source_inventory(self.root)
        self.candidate = {"ref": self.ref, "base": "buildpack-deps:bookworm@sha256:" + "d" * 64,
                          "producer": self.source, "verification": self.source,
                          "platforms": {arch: {"manifest_digest": "sha256:" + digit * 64, "config_digest": "sha256:" + digit * 64}
                                        for arch, digit in (("amd64", "c"), ("arm64", "e"))},
                          "source": {"sha": self.source, "manifest": self.evidence("source.json", {"source": self.source, "files": inputs})}}
        job_name = "publish / build (common-bookworm, buildpack-deps:bookworm@sha256:control)"
        publication = dict(run_id=1, run_attempt=1, job_id=11)
        publication["metadata"] = self.evidence("publication.json", self.job(publication, self.source, job_name, qualification.PUBLICATION_GATES))
        self.candidate["publication"] = publication
        custody = {}
        tools = {}
        for name, raw in self.tool_bytes.items():
            original = "/retained/" + name
            custody[original] = self.evidence("tools/" + name, raw)
            tools[original] = custody[original]["sha256"]
        support_inputs = {}
        for name, raw in self.support_bytes.items():
            original = "/retained/" + name
            custody[original] = self.evidence("support/" + name, raw)
            support_inputs[name] = original
        payloads = []
        for kind, arch in (("image-signature", None), ("sbom", "amd64"), ("sbom", "arm64"),
                           ("vulnerability", "amd64"), ("vulnerability", "arm64")):
            row = dict(kind=kind, architecture=arch)
            for field in ("full_bundle_gzip", "full_predicate_gzip"):
                original = "/retained/" + kind + "-" + str(arch) + "-" + field
                custody[original] = self.evidence(original.removeprefix("/retained/"), b"CONTROL ONLY synthetic custody bytes")
                row[field], row[field + "_sha256"] = original, custody[original]["sha256"]
            payloads.append(row)
        review = dict(ref=self.ref, official_base=self.candidate["base"], producer_workflow_sha=self.source,
                      verification_workflow_sha=self.source, approved_supplier_candidate=True,
                      anonymous_manifest_bytes_sha_verified=True, strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified=True,
                      strict_actual_Rekor_v2_and_RFC3161_TSA_verified=True, wrong_source_and_subject_negative_checks_rejected=True,
                      anonymous_pull={"all_unique_config_and_layer_bytes_fully_hashed": True},
                      producer_verification_provenance={"mode": "fresh-build", "producer_sha": self.source, "verification_sha": self.source},
                      platforms=self.candidate["platforms"], evidence_tool_sha256=tools, signed_payloads=payloads,
                      ci_runtime_scan_signing_evidence={"run_id": "1", "job_id": 11, "job_name": job_name,
                                                      "job_conclusion": "success", "verified_steps": sorted(qualification.PUBLICATION_GATES)})
        fixed = dict(schema="bex.native-common-review-plan/v1", name="common-bookworm", ref=self.ref,
                     base=self.candidate["base"], producer_sha=self.source, verification_sha=self.source,
                     factory_inputs={"deploy/" + path: {key: inputs["deploy/" + path][key] for key in ("sha256", "bytes", "mode")} for path in qualification.FACTORY_PATHS},
                     selection_inputs={path: {key: inputs[path][key] for key in ("sha256", "bytes", "mode")} for path in ("deploy/native-common/matrix.py", "deploy/native-common/upstream.json")},
                     freshness_sha256=inputs["scripts/lib/toolchain-freshness.py"]["sha256"], run_id=1, job_id=11)
        plan = self.evidence("plan.json", fixed)
        tools["/retained/fixed-review-plan.json"] = plan["sha256"]
        custody["/retained/fixed-review-plan.json"] = plan
        review["producer_verification_provenance"]["common_review_plan_sha256"] = plan["sha256"]
        image_custody = {}
        for path in sorted(qualification.IMAGE_INPUT_PATHS):
            original = "/retained/verifier-source/" + path
            image_custody[path] = original
            custody[original] = self.evidence("verifier-source/" + path, (self.root / path).read_bytes())
        image_manifest = self.evidence("image-inputs.json", {"source": self.source, "files": image_inputs})
        custody["/retained/image-inputs.json"] = image_manifest
        self.candidate["approval"] = dict(receipt=self.evidence("review.json", review), plan=plan, inputs=image_manifest,
                                          custody=self.evidence("custody.json", {"files": custody, "support_inputs": support_inputs, "image_inputs": image_custody}))
        self.candidate["utilities"] = {arch: self.utility(arch, 20 + n) for n, arch in enumerate(("amd64", "arm64"))}
        self.save()

    def write(self, relative, value):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(value if isinstance(value, bytes) else (json.dumps(value) + "\n").encode())
        return path

    def evidence(self, name, value):
        path = self.write(".pm/w8/m91/evidence/CONTROL-ONLY/" + name, value)
        return {"path": str(path.relative_to(self.root)), "sha256": qualification.sha(path), "bytes": path.stat().st_size}

    def save(self):
        self.write(qualification.CANDIDATE_PATH, {"schema": qualification.CANDIDATE_SCHEMA, "candidate": self.candidate})

    def job(self, identity, source, name, gates):
        return {"id": identity["job_id"], "run_id": identity["run_id"], "run_attempt": identity["run_attempt"],
                "head_sha": source, "head_branch": "main", "name": name, "status": "completed", "conclusion": "success",
                "run_url": "https://api.github.com/repos/bex-co/bex/actions/runs/" + str(identity["run_id"]),
                "steps": [{"name": gate, "status": "completed", "conclusion": "success"} for gate in sorted(gates)]}

    def utility(self, arch, job_id):
        row = dict(run_id=2, run_attempt=1, job_id=job_id, artifact_id=100 + job_id)
        row["metadata"] = self.evidence(arch + "-job.json", self.job(row, self.source, "qualify (" + arch + ")", {qualification.UTILITY_GATE}))
        row["artifact_metadata"] = self.evidence(arch + "-artifact.json", {"id": row["artifact_id"],
            "name": "common-generated-" + self.source + "-" + arch + "-1", "workflow_run": {"id": 2, "head_sha": self.source}})
        row["root"] = ".pm/w8/m91/evidence/CONTROL-ONLY/" + arch
        records = []
        def put(path, value):
            actual = self.write(row["root"] + "/" + path, value)
            records.append({"path": path, "sha256": qualification.sha(actual), "bytes": actual.stat().st_size})
        put("inputs.json", dict(common_image=self.ref, producer=self.source, verification=self.source, generator_source=self.source,
                                platform="linux/" + arch, run_id="2", run_attempt="1", selection=qualification.profile_selection("utilities", "all")))
        put("qualification-source.json", json.loads((self.root / self.candidate["source"]["manifest"]["path"]).read_text()))
        put("qualification.log", b"CONTROL ONLY complete host fixture\n")
        put("cleanup.json", {"CONTROL_ONLY": True})
        prefix = "generated/native-utilities-control/"
        for case in ("node-compute", "node-static", "python313-original", "python3148-default"):
            put(prefix + case + "/receipt.json", dict(common_base=self.ref, image_platform="linux/" + arch, image_config_id=IMAGE,
                network="none", runtime_command_executed=case != "node-static", static_artifacts_extracted=case == "node-static"))
        for name in ("jq", "rsync", "python-metadata", "python-pip", "python-headers"):
            put(prefix + "missing-" + name + "/omission-receipt.json", dict(removed_supplier=name, common_base=self.ref,
                healthy_independent_prerequisites_observed=True, original_command_expected_failure_observed=True))
        for case in self.originals:
            name = "node-compute" if case["runtime"] == "node" else "python313-original"
            put(prefix + name + "/qa-output/build-" + str(case["mid"]) + ".log", b"CONTROL ONLY authored build log")
            put(prefix + name + "/qa-output/runtime-" + str(case["mid"]) + "-runtime.json", [{"pass": True}] * 3)
        custody = {}
        for kind in ("syft", "spdx"):
            custody[kind] = {"roundtrip": True}
            for form in ("raw", "gzip"):
                name = "generated-" + kind + "." + form
                put(prefix + "python3148-default/" + name, b"CONTROL ONLY full report")
                custody[kind].update({form + "_file": name, form + "_sha256": records[-1]["sha256"], form + "_bytes": records[-1]["bytes"]})
        put(prefix + "python3148-default/generated-spdx-binding.json", dict(platform="linux/" + arch, python="3.14.8",
            image_config_id=IMAGE, syft_manifest_digest=MANIFEST, libbrotli1_versions=["CONTROL ONLY"], custody=custody))
        path = self.write(row["root"] + "/MANIFEST.json", {"files": records, "scope": "CONTROL ONLY"})
        row["manifest"] = {"path": str(path.relative_to(self.root)), "sha256": qualification.sha(path), "bytes": path.stat().st_size}
        return row

    def rewrite_evidence(self, descriptor, mutate):
        path = self.root / descriptor["path"]
        value = json.loads(path.read_text())
        mutate(value)
        path.write_text(json.dumps(value))
        descriptor.update(sha256=qualification.sha(path), bytes=path.stat().st_size)
        self.save()

    def test_real_filesystem_positive_pm_only_and_receipt_exclusion(self):
        self.write(".pm/w8/m91/README.md", b"new PM-only commit")
        self.write("lego/go.work.sum", b"outside GOWORK=off")
        actual = qualification.validate_test_candidate(self.root)
        self.assertEqual(actual["producer"], self.source)
        self.assertEqual(actual["generator_source"], "b" * 40)
        self.assertEqual(actual["receipt_sha256"], qualification.sha(self.root / qualification.CANDIDATE_PATH))
        self.assertNotIn(qualification.CANDIDATE_PATH, qualification.source_inventory(self.root))

    def test_source_add_delete_bytes_mode_and_helper_changes_reject(self):
        path = self.root / "lego/operator/internal/build/native.go"
        before = path.read_bytes()
        changes = (lambda: path.write_bytes(b"changed"), lambda: path.chmod(0o755), lambda: path.unlink(),
                   lambda: self.write("lego/operator/internal/build/new-helper.py", b"added"))
        for change in changes:
            with self.subTest(change=change):
                path.write_bytes(before); path.chmod(0o644)
                added = self.root / "lego/operator/internal/build/new-helper.py"
                added.unlink(missing_ok=True)
                change()
                with self.assertRaises((ValueError, FileNotFoundError)):
                    qualification.validate_test_candidate(self.root)

    def test_null_and_unreviewed_ref_or_identity_reject(self):
        self.write(qualification.CANDIDATE_PATH, {"schema": qualification.CANDIDATE_SCHEMA, "candidate": None})
        with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)
        for field, value in (("ref", "ghcr.io/bex-co/bex-cnb-run:latest"), ("verification", "c" * 40)):
            old = self.candidate[field]
            self.candidate[field] = value; self.save()
            with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)
            self.candidate[field] = old

    def test_tampered_retained_evidence_rejects(self):
        for descriptor in (self.candidate["approval"]["receipt"], self.candidate["approval"]["custody"],
                           self.candidate["source"]["manifest"], self.candidate["utilities"]["amd64"]["manifest"]):
            with self.subTest(path=descriptor["path"]):
                path = self.root / descriptor["path"]; before = path.read_bytes(); path.write_bytes(before + b" ")
                with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)
                path.write_bytes(before)

    def test_wrong_v11_subject_flags_platform_and_report_custody_reject(self):
        descriptor = self.candidate["approval"]["receipt"]
        original = (self.root / descriptor["path"]).read_bytes()
        original_descriptor = dict(descriptor)
        for mutate in (lambda x: x.update(ref="ghcr.io/bex-co/bex-cnb-run@sha256:" + "f" * 64),
                       lambda x: x.update(strict_actual_Rekor_v2_and_RFC3161_TSA_verified=False),
                       lambda x: x["platforms"]["amd64"].update(config_digest="sha256:" + "f" * 64),
                       lambda x: x["signed_payloads"][0].update(full_bundle_gzip_sha256="f" * 64)):
            self.rewrite_evidence(descriptor, mutate)
            with self.assertRaises(ValueError): qualification.validate_image_candidate(self.root)
            (self.root / descriptor["path"]).write_bytes(original); descriptor.update(original_descriptor)

    def test_wrong_job_arch_artifact_and_old_source_reject(self):
        descriptor = self.candidate["utilities"]["amd64"]["metadata"]
        original = (self.root / descriptor["path"]).read_bytes(); original_descriptor = dict(descriptor)
        for mutate in (lambda x: x.update(conclusion="failure"), lambda x: x.update(status="in_progress"),
                       lambda x: x.update(head_sha="f" * 40), lambda x: x.update(name="qualify (arm64)"),
                       lambda x: x.update(run_attempt=2), lambda x: x["steps"][0].update(conclusion="skipped")):
            self.rewrite_evidence(descriptor, mutate)
            with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)
            (self.root / descriptor["path"]).write_bytes(original); descriptor.update(original_descriptor)
        self.candidate["utilities"]["amd64"]["artifact_id"] += 1; self.save()
        with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)

    def test_missing_arch_required_proof_or_full_report_reject(self):
        utility = self.candidate["utilities"].pop("arm64"); self.save()
        with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)
        self.candidate["utilities"]["arm64"] = utility
        descriptor = self.candidate["utilities"]["amd64"]["manifest"]
        self.rewrite_evidence(descriptor, lambda x: x.update(files=[row for row in x["files"] if not row["path"].endswith("/missing-jq/omission-receipt.json")]))
        with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)

    def test_plan_rechecks_real_fixed_files_before_any_output(self):
        env = dict(REQUESTED_SOURCE="reviewed-candidate", REQUESTED_PROFILE="node", REQUESTED_CASE="node-14",
                   GITHUB_OUTPUT=str(self.root / "plan-output"))
        with patch.dict(os.environ, env), patch.object(qualification, "__file__", str(self.root / ".github/scripts/native-common-qualify.py")):
            qualification.plan()
        actual = dict(line.split("=", 1) for line in (self.root / "plan-output").read_text().splitlines())
        self.assertEqual(actual["ref"], self.ref)
        self.assertEqual(actual["source"], "reviewed-candidate")
        self.assertEqual(actual["receipt_sha256"], qualification.sha(self.root / qualification.CANDIDATE_PATH))
        for event, ref, profile, source in (("push", "refs/heads/main", "node", "reviewed-candidate"),
                                            ("workflow_dispatch", "refs/heads/other", "node", "reviewed-candidate"),
                                            ("workflow_dispatch", "refs/heads/main", "utilities", "reviewed-candidate"),
                                            ("workflow_dispatch", "refs/heads/main", "node", "unknown")):
            (self.root / "plan-output").unlink(missing_ok=True)
            with patch.dict(os.environ, env, GITHUB_EVENT_NAME=event, GITHUB_REF=ref, REQUESTED_PROFILE=profile, REQUESTED_SOURCE=source), \
                 patch.object(qualification.tempfile, "mkdtemp", side_effect=AssertionError("no resources")):
                with self.assertRaises(ValueError): qualification.plan()
            self.assertFalse((self.root / "plan-output").exists())

    def test_source_policy_and_fixed_plan_omitted_helper_reject(self):
        helper = self.root / ".github/scripts/native-common-qualify.py"
        before = helper.read_bytes(); helper.write_bytes(before + b"changed helper")
        with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)
        helper.write_bytes(before)
        self.rewrite_evidence(self.candidate["approval"]["plan"], lambda x: x["factory_inputs"].pop("deploy/native-common/prepare-node14-tools.py"))
        with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)

    def test_unmatched_selector_platform_and_default_config_reject(self):
        utility = self.candidate["utilities"]["amd64"]
        descriptor = utility["manifest"]
        path = self.root / utility["root"] / "inputs.json"
        inputs = json.loads(path.read_text()); inputs["selection"]["selector"] = "^TestUnknown$"
        path.write_text(json.dumps(inputs))
        def update_manifest(value):
            for row in value["files"]:
                if row["path"] == "inputs.json":
                    row.update(bytes=path.stat().st_size, sha256=qualification.sha(path))
        self.rewrite_evidence(descriptor, update_manifest)
        with self.assertRaises(ValueError): qualification.validate_test_candidate(self.root)

    def test_old_job_without_captured_source_and_substituted_source_reject(self):
        descriptor = self.candidate["utilities"]["amd64"]["manifest"]
        self.rewrite_evidence(descriptor, lambda x: x.update(files=[row for row in x["files"] if row["path"] != "qualification-source.json"]))
        with self.assertRaisesRegex(ValueError, "incomplete original utility artifacts"):
            qualification.validate_test_candidate(self.root)

    def test_substituted_current_manifest_cannot_claim_an_old_baseline(self):
        descriptor = self.candidate["source"]["manifest"]
        self.rewrite_evidence(descriptor, lambda x: x.update(source="f" * 40))
        with self.assertRaisesRegex(ValueError, "stale qualification source"):
            qualification.validate_test_candidate(self.root)

    def add_sixth_payload(self):
        descriptor = self.candidate["approval"]["receipt"]
        review = json.loads((self.root / descriptor["path"]).read_text())
        row = dict(next(x for x in review["signed_payloads"] if x["kind"] == "sbom" and x["architecture"] == "amd64"))
        additions = {}
        for key in ("full_bundle_gzip", "full_predicate_gzip"):
            original = "/retained/extra-approved-sbom-amd64-" + key
            additions[original] = self.evidence("sixth/" + key, b"CONTROL ONLY distinct accepted sixth payload")
            row[key], row[key + "_sha256"] = original, additions[original]["sha256"]
        self.rewrite_evidence(self.candidate["approval"]["custody"], lambda x: x["files"].update(additions))
        self.rewrite_evidence(descriptor, lambda x: x["signed_payloads"].append(row))

    def test_sixth_accepted_payload_is_retained_without_filtering(self):
        self.add_sixth_payload()
        descriptor = self.candidate["approval"]["receipt"]
        before = (self.root / descriptor["path"]).read_bytes()
        qualification.validate_image_candidate(self.root)
        self.assertEqual((self.root / descriptor["path"]).read_bytes(), before)
        self.assertEqual(len(json.loads(before)["signed_payloads"]), 6)

    def test_every_extra_payload_requires_its_full_custody(self):
        self.add_sixth_payload()
        descriptor = self.candidate["approval"]["receipt"]
        self.rewrite_evidence(descriptor, lambda x: x["signed_payloads"][-1].update(full_bundle_gzip_sha256="f" * 64))
        with self.assertRaisesRegex(ValueError, "full signed report custody"):
            qualification.validate_image_candidate(self.root)

    def test_unsupported_payload_pair_is_rejected(self):
        self.add_sixth_payload()
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["signed_payloads"][-1].update(kind="unsupported"))
        with self.assertRaisesRegex(ValueError, "complete supported signed"):
            qualification.validate_image_candidate(self.root)

    def test_adapter_identity_must_match_even_when_wrong_bytes_are_custodied(self):
        original = "/retained/common-v11-adapter.py"
        custody = self.candidate["approval"]["custody"]
        entries = json.loads((self.root / custody["path"]).read_text())["files"]
        descriptor = entries[original]
        path = self.root / descriptor["path"]
        path.write_bytes(b"CONTROL ONLY wrong adapter source")
        replacement = dict(descriptor, sha256=qualification.sha(path), bytes=path.stat().st_size)
        self.rewrite_evidence(custody, lambda x: x["files"].update({original: replacement}))
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["evidence_tool_sha256"].update({original: replacement["sha256"]}))
        with self.assertRaisesRegex(ValueError, "retained V11 tool differs"):
            qualification.validate_image_candidate(self.root)

    def test_missing_reported_adapter_is_rejected(self):
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["evidence_tool_sha256"].pop("/retained/common-v11-adapter.py"))
        with self.assertRaisesRegex(ValueError, "retained V11 tool differs"):
            qualification.validate_image_candidate(self.root)

    def test_missing_adapter_custody_is_rejected(self):
        self.rewrite_evidence(self.candidate["approval"]["custody"], lambda x: x["files"].pop("/retained/common-v11-adapter.py"))
        with self.assertRaisesRegex(ValueError, "executed tool/plan has no exact portable custody"):
            qualification.validate_image_candidate(self.root)

    def test_every_additional_reported_tool_requires_exact_custody(self):
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["evidence_tool_sha256"].update({"/retained/extra.py": "f" * 64}))
        with self.assertRaisesRegex(ValueError, "executed tool/plan has no exact portable custody"):
            qualification.validate_image_candidate(self.root)

    def test_additional_reported_input_is_hash_custodied_without_execution(self):
        descriptor = self.evidence("additional-reported-input.txt", b"CONTROL ONLY an additional reported input")
        original = "/retained/additional-reported-input.txt"
        self.rewrite_evidence(self.candidate["approval"]["custody"], lambda x: x["files"].update({original: descriptor}))
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["evidence_tool_sha256"].update({original: descriptor["sha256"]}))
        qualification.validate_image_candidate(self.root)

    def test_plan_hash_is_bound_to_actual_common_provenance(self):
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["producer_verification_provenance"].update(common_review_plan_sha256="f" * 64))
        with self.assertRaisesRegex(ValueError, "executed fixed review plan linkage differs"):
            qualification.validate_image_candidate(self.root)

    def test_executed_plan_entry_cannot_be_replaced_by_another_custodied_file(self):
        replacement = self.evidence("another-plan.json", {"CONTROL_ONLY": "another fixed plan"})
        self.rewrite_evidence(self.candidate["approval"]["custody"], lambda x: x["files"].update({"/retained/fixed-review-plan.json": replacement}))
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["evidence_tool_sha256"].update({"/retained/fixed-review-plan.json": replacement["sha256"]}))
        with self.assertRaisesRegex(ValueError, "executed fixed review plan linkage differs"):
            qualification.validate_image_candidate(self.root)

    def test_support_manifest_identity_is_not_an_executed_policy(self):
        name = "loopx-w8-verify-native-artifact.py"
        path = "/retained/" + name
        custody = self.candidate["approval"]["custody"]
        entry = json.loads((self.root / custody["path"]).read_text())["files"][path]
        self.rewrite_evidence(self.candidate["approval"]["receipt"], lambda x: x["evidence_tool_sha256"].update({path: entry["sha256"]}))
        with self.assertRaisesRegex(ValueError, "review support identity differs or is mislabeled"):
            qualification.validate_image_candidate(self.root)

    def test_wrong_support_bytes_are_rejected_even_with_consistent_custody(self):
        original = "/retained/loopx-w8-verify-native-artifact.py"
        custody = self.candidate["approval"]["custody"]
        descriptor = json.loads((self.root / custody["path"]).read_text())["files"][original]
        path = self.root / descriptor["path"]; path.write_bytes(b"CONTROL ONLY wrong support source")
        replacement = dict(descriptor, sha256=qualification.sha(path), bytes=path.stat().st_size)
        self.rewrite_evidence(custody, lambda x: x["files"].update({original: replacement}))
        with self.assertRaisesRegex(ValueError, "review support identity differs"):
            qualification.validate_image_candidate(self.root)

    def test_reviewed_image_with_null_baseline_plans_full_utilities_but_not_family(self):
        self.candidate['source'] = None
        self.candidate['utilities'] = None
        self.save()
        approved = qualification.validate_image_candidate(self.root)
        self.assertEqual(approved['baseline_status'], 'image-only-requalification')
        with self.assertRaisesRegex(ValueError, 'family reuse requires'):
            qualification.validate_test_candidate(self.root)
        output = self.root / 'plan-output'
        env = dict(REQUESTED_SOURCE='reviewed-candidate', REQUESTED_PROFILE='utilities', REQUESTED_CASE='all', GITHUB_OUTPUT=str(output))
        with patch.object(qualification, '__file__', str(self.root / '.github/scripts/native-common-qualify.py')), \
             patch.object(qualification.tempfile, 'mkdtemp', side_effect=AssertionError('plan creates no resources')), patch.dict(os.environ, env):
            qualification.plan()
        planned = dict(line.split('=', 1) for line in output.read_text().splitlines())
        self.assertEqual(planned['baseline_status'], 'image-only-requalification')
        self.assertEqual(planned['ref'], self.ref)
        self.assertEqual(planned['case'], 'all')
        self.assertEqual(planned['selector'], qualification.profile_selection('utilities', 'all')['selector'])
        output.unlink()
        with patch.object(qualification, '__file__', str(self.root / '.github/scripts/native-common-qualify.py')), \
             patch.dict(os.environ, env, REQUESTED_PROFILE='node', REQUESTED_CASE='node-14'), self.assertRaises(ValueError):
            qualification.plan()
        self.assertFalse(output.exists())

    def test_generator_and_qualification_changes_allow_image_requalification_only(self):
        for path in ('lego/operator/internal/build/native.go', '.github/scripts/native-common-qualify.py',
                     '.github/workflows/native-common-image.yml', '.github/scripts/test_native_oci_common_qualification.py'):
            self.assertNotIn(path, qualification.IMAGE_INPUT_PATHS)
            self.write(path, b'CONTROL ONLY changed generator or qualification source')
        self.assertEqual(qualification.validate_image_candidate(self.root)['ref'], self.ref)
        with self.assertRaisesRegex(ValueError, 'stale qualification source'):
            qualification.validate_test_candidate(self.root)

    def test_partial_or_failed_old_utilities_never_authorize_a_family(self):
        original = self.candidate['utilities']
        self.candidate['utilities'] = {'amd64': original['amd64']}
        self.save()
        qualification.validate_image_candidate(self.root)
        with self.assertRaisesRegex(ValueError, 'both original utility architectures'):
            qualification.validate_test_candidate(self.root)
        self.candidate['utilities'] = original
        self.rewrite_evidence(original['amd64']['metadata'], lambda x: x.update(conclusion='failure'))
        qualification.validate_image_candidate(self.root)
        with self.assertRaisesRegex(ValueError, 'actual job/source/status differs'):
            qualification.validate_test_candidate(self.root)

    def test_null_or_stale_baseline_init_captures_new_source_without_relabeling_image(self):
        original_source, original_utilities = self.candidate['source'], self.candidate['utilities']
        for prior in ('absent', 'stale'):
            self.candidate['source'] = None if prior == 'absent' else original_source
            self.candidate['utilities'] = None if prior == 'absent' else original_utilities
            if prior == 'stale':
                self.write('lego/operator/internal/build/native.go', b'CONTROL ONLY new generator after prior baseline')
            self.save()
            approved = qualification.validate_image_candidate(self.root)
            env = dict(QUALIFICATION_PROFILE='utilities', QUALIFICATION_CASE='all', QUALIFICATION_SOURCE='reviewed-candidate',
                       COMMON_IMAGE=self.ref, PRODUCER_SHA=self.source, VERIFICATION_SHA=self.source, TARGET_PLATFORM='linux/arm64',
                       CANDIDATE_RECEIPT_SHA256=approved['receipt_sha256'], RUNNER_TEMP=str(self.root), GITHUB_RUN_ID='3', GITHUB_RUN_ATTEMPT='1',
                       GITHUB_OUTPUT=str(self.root / 'outputs'), GITHUB_ENV=str(self.root / 'environment'))
            with patch.object(qualification, '__file__', str(self.root / '.github/scripts/native-common-qualify.py')), patch.dict(os.environ, env):
                qualification.initialize()
            outputs = dict(line.split('=', 1) for line in (self.root / 'outputs').read_text().splitlines())
            evidence = Path(outputs['root'])
            recorded = json.loads((evidence / 'inputs.json').read_text())
            self.assertEqual((recorded['producer'], recorded['verification'], recorded['generator_source']), (self.source, self.source, 'b' * 40))
            self.assertEqual(recorded['baseline_status'], 'image-only-requalification')
            self.assertEqual(recorded['previous_baseline_state'], 'absent' if prior == 'absent' else 'unvalidated-existing')
            self.assertIsNone(recorded['utility_baseline_source'])
            self.assertIsNone(recorded['utility_jobs'])
            self.assertEqual(recorded['image_approval_inputs_sha256'], self.candidate['approval']['inputs']['sha256'])
            self.assertEqual(json.loads((evidence / 'qualification-source.json').read_text()),
                             {'source': 'b' * 40, 'files': qualification.source_inventory(self.root)})
            config = Path(recorded['docker_config'])
            self.assertEqual(config.stat().st_mode & 0o777, 0o700)
            self.assertFalse((config / 'config.json').exists())  # No login occurred.

    def test_requalification_init_revalidates_image_before_credentials(self):
        self.candidate['source'] = None
        self.candidate['utilities'] = None
        self.save()
        planned = qualification.validate_image_candidate(self.root)
        env = dict(QUALIFICATION_PROFILE='utilities', QUALIFICATION_CASE='all', QUALIFICATION_SOURCE='reviewed-candidate',
                   COMMON_IMAGE=self.ref, PRODUCER_SHA=self.source, VERIFICATION_SHA=self.source, TARGET_PLATFORM='linux/arm64',
                   CANDIDATE_RECEIPT_SHA256=planned['receipt_sha256'])
        self.write('deploy/native-common/prepare-node14-tools.py', b'changed after planning')
        with patch.object(qualification, '__file__', str(self.root / '.github/scripts/native-common-qualify.py')), \
             patch.object(qualification.tempfile, 'mkdtemp', side_effect=AssertionError('no resources')), patch.dict(os.environ, env):
            with self.assertRaisesRegex(ValueError, 'image-critical inputs'):
                qualification.initialize()

    def test_image_critical_changed_bytes_mode_and_type_reject(self):
        target = self.root / 'deploy/native-base/check-tools.py'
        original = target.read_bytes()
        permissions = target.stat().st_mode & 0o777
        for mutation in ('bytes', 'mode', 'type'):
            with self.subTest(mutation=mutation):
                if mutation == 'bytes': target.write_bytes(b'changed publisher check')
                elif mutation == 'mode': target.chmod(0o755)
                else: target.unlink(); target.mkdir()
                with self.assertRaises(ValueError): qualification.validate_image_candidate(self.root)
                if target.is_dir(): target.rmdir()
                target.write_bytes(original); target.chmod(permissions)

    def test_image_source_proof_must_be_original_verifier_and_exact_set(self):
        descriptor = self.candidate['approval']['inputs']
        self.rewrite_evidence(descriptor, lambda x: x.update(source='b' * 40))
        with self.assertRaisesRegex(ValueError, 'wrong verifier-source image-critical inputs'):
            qualification.validate_image_candidate(self.root)

    def test_omitted_image_source_path_is_not_an_equivalence_boolean(self):
        self.rewrite_evidence(self.candidate['approval']['inputs'], lambda x: x['files'].pop('.github/scripts/native-oci-mirror.py'))
        with self.assertRaisesRegex(ValueError, 'image-critical inputs'):
            qualification.validate_image_candidate(self.root)

    def test_image_source_body_and_manifest_need_portable_custody(self):
        self.rewrite_evidence(self.candidate['approval']['custody'], lambda x: x['image_inputs'].pop('.github/scripts/native-oci-stage.py'))
        with self.assertRaisesRegex(ValueError, 'image-critical source custody set differs'):
            qualification.validate_image_candidate(self.root)

    def test_image_manifest_itself_needs_portable_custody(self):
        self.rewrite_evidence(self.candidate['approval']['custody'], lambda x: x['files'].pop('/retained/image-inputs.json'))
        with self.assertRaisesRegex(ValueError, 'image input manifest has no portable custody'):
            qualification.validate_image_candidate(self.root)

    def test_image_only_requalification_still_rejects_wrong_index_and_platform(self):
        self.candidate['platforms']['amd64']['config_digest'] = 'sha256:' + 'f' * 64
        self.save()
        with self.assertRaisesRegex(ValueError, 'V11 platform identities differ'):
            qualification.validate_image_candidate(self.root)

    def test_image_only_requalification_requires_successful_original_publication(self):
        self.rewrite_evidence(self.candidate['publication']['metadata'], lambda x: x.update(conclusion='failure'))
        with self.assertRaisesRegex(ValueError, 'actual job/source/status differs'):
            qualification.validate_image_candidate(self.root)

    def test_complete_utility_command_and_resource_bounds_remain_unchanged(self):
        jobs = ProfileRoutingControls().jobs()
        original = jobs['qualify']
        for literal in ('timeout-minutes: 180', 'max-parallel: 1', 'arch: [amd64, arm64]', 'QUALIFICATION_PROFILE: utilities', 'QUALIFICATION_CASE: all'):
            self.assertIn(literal, original)
        command = ProfileRoutingControls().run_block(original, qualification.UTILITY_GATE)
        self.assertIn("-run '^(TestNativeUtilityGeneratedImages|TestNativeOriginalReplayHTTPOracles)$'", command)
        self.assertIn('-timeout 150m', command)
        self.assertNotIn('QUALIFICATION_SELECTOR', command)
        self.assertIn("needs.plan.outputs.baseline_status == 'current-complete'", jobs['qualify-profile'])

    def test_actual_utility_job_branch_table_has_no_failed_fresh_fallback(self):
        job = ProfileRoutingControls().jobs()['qualify']
        expression = ' '.join(job.split('    if: >-\n', 1)[1].split('    permissions:', 1)[0].split())
        expression = expression.removeprefix('${{ ').removesuffix(' }}')
        import itertools
        options = itertools.product(('fresh', 'reviewed-candidate', 'unknown'), ('utilities', 'node'),
                                    ('success', 'failure', 'cancelled', 'skipped'),
                                    ('image-only-requalification', 'current-complete', ''), ('push', 'workflow_dispatch'),
                                    (False, True), ('success', 'failure', 'cancelled', 'skipped'), (False, True))
        for mode, profile, published, baseline, event, cancelled, plan_result, bound in options:
            values = {'always()': True, '!cancelled()': not cancelled, 'github.ref': 'refs/heads/main',
                      'github.event_name': event, 'needs.plan.result': plan_result,
                      'needs.plan.outputs.source': mode, 'needs.plan.outputs.profile': profile,
                      'needs.plan.outputs.baseline_status': baseline, 'needs.publish.result': published,
                      'needs.publish.outputs.ref': self.ref if published == 'success' else '',
                      'needs.plan.outputs.ref': self.ref if bound else '',
                      'needs.plan.outputs.receipt_sha256': 'c' * 64 if bound else ''}
            translated = expression
            for token in sorted(values, key=len, reverse=True):
                translated = translated.replace(token, repr(values[token]))
            actual = eval(translated.replace('&&', ' and ').replace('||', ' or '), {'__builtins__': {}}, {})
            expected = not cancelled and plan_result == 'success' and ((mode == 'fresh' and published == 'success') or
                (mode == 'reviewed-candidate' and profile == 'utilities' and event == 'workflow_dispatch'
                 and baseline == 'image-only-requalification' and published == 'skipped' and bound))
            self.assertEqual(actual, expected, (mode, profile, published, baseline, event, cancelled, plan_result, bound))

    def test_image_only_status_cannot_start_a_family_even_with_bound_ref(self):
        job = ProfileRoutingControls().jobs()['qualify-profile']
        expression = ' '.join(job.split('    if: >-\n', 1)[1].split('    permissions:', 1)[0].split())
        expression = expression.removeprefix('${{ ').removesuffix(' }}')
        base = {'always()': True, '!cancelled()': True, 'github.ref': 'refs/heads/main',
                'github.event_name': 'workflow_dispatch', 'needs.plan.result': 'success',
                'needs.plan.outputs.source': 'reviewed-candidate', 'needs.plan.outputs.profile': 'node',
                'needs.publish.result': 'skipped', 'needs.qualify.result': 'skipped', 'needs.publish.outputs.ref': '',
                'needs.plan.outputs.ref': self.ref, 'needs.plan.outputs.receipt_sha256': 'c' * 64}
        for baseline, expected in (('image-only-requalification', False), ('', False), ('current-complete', True)):
            values = dict(base, **{'needs.plan.outputs.baseline_status': baseline})
            translated = expression
            for token in sorted(values, key=len, reverse=True):
                translated = translated.replace(token, repr(values[token]))
            self.assertEqual(eval(translated.replace('&&', ' and ').replace('||', ' or '), {'__builtins__': {}}, {}), expected)

    def test_init_revalidates_before_creds_outputs_or_resources(self):
        planned = qualification.validate_test_candidate(self.root)
        env = dict(QUALIFICATION_PROFILE="node", QUALIFICATION_CASE="node-14", QUALIFICATION_SOURCE="reviewed-candidate",
                   COMMON_IMAGE=self.ref, PRODUCER_SHA=self.source, VERIFICATION_SHA=self.source, TARGET_PLATFORM="linux/arm64",
                   CANDIDATE_RECEIPT_SHA256=planned["receipt_sha256"])
        # Change an actual file after successful planning. The real validator,
        # not a stubbed failure, must reject before any output/config/resources.
        self.write("lego/operator/internal/build/native.go", b"changed after plan")
        with patch.object(qualification, "__file__", str(self.root / ".github/scripts/native-common-qualify.py")), \
             patch.object(qualification.tempfile, "mkdtemp", side_effect=AssertionError("no resources")), patch.dict(os.environ, env):
            with self.assertRaisesRegex(ValueError, "stale qualification source"):
                qualification.initialize()

    def test_branch_table_never_heals_failed_fresh_and_init_records_reuse(self):
        pilot = ProfileRoutingControls().jobs()["qualify-profile"]
        expression = " ".join(pilot.split("    if: >-\n", 1)[1].split("    permissions:", 1)[0].split())
        expression = expression.removeprefix("${{ ").removesuffix(" }}")
        def actual_branch(mode, publish, utilities, valid, cancelled=False):
            values = {"always()": True, "!cancelled()": not cancelled, "github.ref": "refs/heads/main",
                      "github.event_name": "workflow_dispatch", "needs.plan.result": "success",
                      "needs.plan.outputs.profile": "node", "needs.plan.outputs.source": mode,
                      "needs.plan.outputs.baseline_status": "current-complete" if valid else "image-only-requalification",
                      "needs.publish.result": publish, "needs.qualify.result": utilities,
                      "needs.publish.outputs.ref": self.ref if publish == "success" else "",
                      "needs.plan.outputs.ref": self.ref if valid else "",
                      "needs.plan.outputs.receipt_sha256": "c" * 64 if valid else ""}
            translated = expression
            for token in sorted(values, key=len, reverse=True):
                translated = translated.replace(token, repr(values[token]))
            translated = translated.replace("&&", " and ").replace("||", " or ")
            return eval(translated, {"__builtins__": {}}, {})
        for mode in ("fresh", "reviewed-candidate", "unknown"):
            for publish in ("success", "skipped", "failure", "cancelled"):
                for utilities in ("success", "skipped", "failure", "cancelled"):
                    for valid in (False, True):
                        expected = (mode == "fresh" and publish == utilities == "success") or (mode == "reviewed-candidate" and valid and publish == utilities == "skipped")
                        self.assertEqual(actual_branch(mode, publish, utilities, valid), expected)
                        self.assertFalse(actual_branch(mode, publish, utilities, valid, cancelled=True))
        candidate = qualification.validate_test_candidate(self.root)
        env = dict(QUALIFICATION_PROFILE="node", QUALIFICATION_CASE="node-14", QUALIFICATION_SOURCE="reviewed-candidate",
                   COMMON_IMAGE=self.ref, PRODUCER_SHA=self.source, VERIFICATION_SHA=self.source, TARGET_PLATFORM="linux/arm64",
                   CANDIDATE_RECEIPT_SHA256=candidate["receipt_sha256"], RUNNER_TEMP=str(self.root), GITHUB_RUN_ID="3", GITHUB_RUN_ATTEMPT="1",
                   GITHUB_OUTPUT=str(self.root / "outputs"), GITHUB_ENV=str(self.root / "environment"))
        with patch.dict(os.environ, env), patch.object(qualification, "validate_test_candidate", return_value=candidate), \
             patch.object(qualification, "__file__", str(self.root / ".github/scripts/native-common-qualify.py")):
            qualification.initialize()
        outputs = dict(line.split("=", 1) for line in (self.root / "outputs").read_text().splitlines())
        actual = json.loads((Path(outputs["root"]) / "inputs.json").read_text())
        self.assertEqual(actual["candidate_receipt_sha256"], candidate["receipt_sha256"])
        self.assertEqual(actual["utility_baseline_source"], self.source)
        self.assertEqual(actual["generator_source"], "b" * 40)
        self.assertEqual(actual["utility_jobs"], candidate["utilities"])
        captured = json.loads((Path(outputs["root"]) / "qualification-source.json").read_text())
        self.assertEqual(captured, {"source": "b" * 40, "files": qualification.source_inventory(self.root)})


if __name__ == "__main__":
    unittest.main()
