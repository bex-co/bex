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
        expected = {"node": (8, 90, 120), "modern-python": (5, 110, 140),
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
        paths = {"node": "nativecommon_node_qualification_test.go", "modern-python": "nativemodern_python_test.go",
                 "legacy-python": "nativelegacy_python_test.go", "ruby": "nativecommon_ruby_test.go",
                 "go-rust": "nativecommon_gorust_test.go", "elixir": "nativecommon_elixir_qualification_test.go",
                 "native-init": "nativequalification_init_test.go"}
        for profile, name in paths.items():
            raw = (build / name).read_text()
            test, flag, cases, _, _ = qualification.PROFILES[profile]
            self.assertIn("func " + test + "(t *testing.T)", raw)
            self.assertIn('"' + flag + '"', raw)
            if profile == "node":
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
                self.assertEqual(actual, {key: expected[key] for key in ("profile", "case", "flag", "selector", "go_timeout", "job_minutes")})
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
        self.assertEqual(sum("uses: ./.github/workflows/native-base-image.yml" in body for body in jobs.values()), 1)
        plan = jobs["plan"]
        self.assertNotIn("packages:", plan)
        self.assertNotIn("docker", plan)
        self.assertIn("REQUESTED_PROFILE: ${{ inputs.profile || 'utilities' }}", plan)
        self.assertIn("REQUESTED_CASE: ${{ inputs.case || 'pilot' }}", plan)
        self.assertIn("native-common-qualify.py plan", plan)
        original = jobs["qualify"]
        self.assertIn("needs: publish\n", original)
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
        self.assertIn("timeout-minutes: ${{ fromJSON(needs.plan.outputs.job_minutes) }}", pilot)
        for job in (original, pilot):
            for unchanged in ("max-parallel: 1", "arch: [amd64, arm64]", "use: false", "cleanup: false",
                              "COMMON_IMAGE: ${{ needs.publish.outputs.ref }}", "PRODUCER_SHA: ${{ needs.publish.outputs.producer }}",
                              "VERIFICATION_SHA: ${{ needs.publish.outputs.verification }}", "packages: read"):
                self.assertIn(unchanged, job)
            for retained in ("capacity", "cleanup", "manifest"):
                self.assertIn("native-common-qualify.py " + retained, job)
            self.assertIn("if: always() && steps.owned.outputs.root != ''", job)
            self.assertIn("include-hidden-files: true", job)
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


if __name__ == "__main__":
    unittest.main()
