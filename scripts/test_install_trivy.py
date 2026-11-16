"""Controlled releases exercise the real Trivy installer without downloads."""
import hashlib
import importlib.util
import io
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("install-trivy.py")
SPEC = importlib.util.spec_from_file_location("install_trivy", SCRIPT)
trivy = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(trivy)
release = trivy.release
URL = "https://github.com/aquasecurity/trivy/releases/download/v0.70.0/trivy_0.70.0_Linux-%s.tar.gz"


def release_archive(trivy_symlink=False):
    """The official archive's shape: docs and templates beside one binary."""
    result = io.BytesIO()
    with tarfile.open(fileobj=result, mode="w:gz") as archive:
        for name, content in (("LICENSE", b"license"), ("README.md", b"readme"),
                              ("contrib/html.tpl", b"template")):
            member = tarfile.TarInfo(name)
            member.size = len(content)
            archive.addfile(member, io.BytesIO(content))
        member = tarfile.TarInfo("trivy")
        if trivy_symlink:
            member.type = tarfile.SYMTYPE
            member.linkname = "/private-canary"
            archive.addfile(member)
        else:
            binary = b"owned fixture scanner"
            member.size = len(binary)
            member.mode = 0o700
            archive.addfile(member, io.BytesIO(binary))
    return result.getvalue()


class TrivyInstallerTests(unittest.TestCase):
    def install(self, root, payload, content=None):
        """Install `content` (default `payload`) as if `payload` were the reviewed amd64 archive."""
        entry = ("64bit", len(payload), hashlib.sha256(payload).hexdigest())
        with patch.object(release.platform, "system", return_value="Linux"), \
                patch.object(release.platform, "machine", return_value="x86_64"), \
                patch.dict(trivy.RELEASES["0.70.0"], {"x86_64": entry}), \
                patch.object(release, "download", return_value=payload if content is None else content) as download:
            trivy.install(root, root / "github-path")
        download.assert_called_once_with(URL % "64bit", len(payload), "Trivy")

    def test_each_runner_architecture_downloads_its_reviewed_archive(self):
        for machine, asset, size in (("x86_64", "64bit", 47748408), ("aarch64", "ARM64", 43159403)):
            with self.subTest(machine=machine), tempfile.TemporaryDirectory() as folder:
                with patch.object(release.platform, "system", return_value="Linux"), \
                        patch.object(release.platform, "machine", return_value=machine), \
                        patch.object(release, "download", side_effect=RuntimeError("stop")) as download:
                    with self.assertRaisesRegex(RuntimeError, "stop"):
                        trivy.install(Path(folder), Path(folder) / "github-path")
                download.assert_called_once_with(URL % asset, size, "Trivy")

    def test_success_publishes_only_the_scanner_binary(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            self.install(root, release_archive())
            destination = Path((root / "github-path").read_text().strip())
            self.assertEqual(destination.parent, root)
            self.assertTrue(destination.name.startswith("bex-trivy-"))
            # Templates, docs and archive modes are never extracted or kept.
            self.assertEqual([p.name for p in destination.iterdir()], ["trivy"])
            self.assertEqual((destination / "trivy").read_bytes(), b"owned fixture scanner")
            self.assertEqual((destination / "trivy").stat().st_mode & 0o777, 0o755)

    def test_corrupted_archive_and_symlinked_binary_publish_nothing(self):
        payload = release_archive()
        cases = (
            ("Trivy release archive size/checksum mismatch", payload, bytes([payload[0] ^ 1]) + payload[1:]),
            ("Trivy release does not contain a regular binary", release_archive(trivy_symlink=True), None),
        )
        for message, reviewed, received in cases:
            with self.subTest(message=message), tempfile.TemporaryDirectory() as folder:
                with self.assertRaisesRegex(RuntimeError, message):
                    self.install(Path(folder), reviewed, received)
                self.assertEqual(list(Path(folder).iterdir()), [])

    def test_unreviewed_version_and_platform_are_refused_before_download(self):
        for system, version, message in (("Linux", "0.76.2", "Trivy 0.76.2 is not a reviewed release"),
                                         ("Darwin", "0.70.0", "Trivy installer supports Linux")):
            with self.subTest(system=system, version=version), tempfile.TemporaryDirectory() as folder, \
                    patch.object(release.platform, "system", return_value=system), \
                    patch.object(release.platform, "machine", return_value="x86_64"), \
                    patch.object(release, "download") as download:
                with self.assertRaisesRegex(RuntimeError, message):
                    trivy.install(Path(folder), Path(folder) / "github-path", version)
                download.assert_not_called()
                self.assertEqual(list(Path(folder).iterdir()), [])

    def test_transient_failures_retry_with_the_scanner_named_in_the_log(self):
        class Response(io.BytesIO):
            def geturl(self):
                return URL % "64bit"

        stderr = io.StringIO()
        with patch.object(release, "open_https", side_effect=[TimeoutError(), Response(b"complete")]), \
                patch.object(release.time, "sleep"), patch.object(sys, "stderr", stderr):
            self.assertEqual(release.download(URL % "64bit", 8, "Trivy"), b"complete")
        self.assertEqual(stderr.getvalue(), "Trivy download retry 1/4: TimeoutError\n")
        with patch.object(release, "open_https", side_effect=TimeoutError()) as opened, \
                patch.object(release.time, "sleep"), patch.object(sys, "stderr", io.StringIO()):
            with self.assertRaisesRegex(RuntimeError, "Trivy download failed: TimeoutError"):
                release.download(URL % "64bit", 8, "Trivy")
        self.assertEqual(opened.call_count, 4)

    def test_workflow_entrypoint_loads_the_shared_core_from_any_directory(self):
        # The workflow runs the script by path; an unreviewed version must fail
        # in the real process (after the shared core loads) without a download.
        with tempfile.TemporaryDirectory() as folder:
            env = dict(os.environ, RUNNER_TEMP=folder, GITHUB_PATH=str(Path(folder) / "github-path"))
            process = subprocess.run([sys.executable, "-B", str(SCRIPT), "9.9.9"], cwd=folder, env=env,
                                     capture_output=True, text=True, timeout=30)
            self.assertNotEqual(process.returncode, 0)
            self.assertIn("RuntimeError: Trivy 9.9.9 is not a reviewed release", process.stderr)
            self.assertEqual(list(Path(folder).iterdir()), [])


if __name__ == "__main__":
    unittest.main()
