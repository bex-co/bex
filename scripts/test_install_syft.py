"""Controlled failures exercise the real installer without downloads or tools."""
import hashlib
import http.client
import importlib.util
import io
from pathlib import Path
import ssl
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import urllib.error
import urllib.request

SPEC = importlib.util.spec_from_file_location("install_syft", Path(__file__).with_name("install-syft.py"))
syft = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(syft)
URL = "https://github.com/anchore/syft/releases/download/v1.51.1/syft_1.51.1_linux_arm64.tar.gz"


class Response(io.BytesIO):
    def geturl(self):
        return URL


def archive_bytes(symlink=False):
    result = io.BytesIO()
    with tarfile.open(fileobj=result, mode="w:gz") as archive:
        member = tarfile.TarInfo("syft")
        if symlink:
            member.type = tarfile.SYMTYPE
            member.linkname = "/private-canary"
            archive.addfile(member)
        else:
            binary = b"owned fixture binary"
            member.size = len(binary)
            archive.addfile(member, io.BytesIO(binary))
        # A second member must never be extracted, even with an escaping path.
        extra = tarfile.TarInfo("../private-canary")
        extra.size = 7
        archive.addfile(extra, io.BytesIO(b"private"))
    return result.getvalue()


class InstallerTests(unittest.TestCase):
    def test_interrupted_reads_use_a_fresh_complete_request(self):
        class InterruptedResponse(Response):
            def read(self, size):
                raise http.client.IncompleteRead(b"partial")

        partial = InterruptedResponse()
        with patch.object(syft, "open_https", side_effect=[partial, Response(b"complete")]) as opened, patch.object(syft.time, "sleep") as sleep:
            self.assertEqual(syft.download(URL, 8), b"complete")
            self.assertEqual(opened.call_count, 2)
            self.assertTrue(partial.closed)
            sleep.assert_called_once_with(1)

    def test_tls_eof_retries_but_never_relaxes_verification(self):
        error = urllib.error.URLError(ssl.SSLEOFError("interrupted handshake"))
        with patch.object(syft, "open_https", side_effect=[error, Response(b"complete")]), patch.object(syft.time, "sleep") as sleep:
            self.assertEqual(syft.download(URL, 8), b"complete")
            sleep.assert_called_once_with(1)

    def test_response_read_is_bounded_even_when_server_sends_excess(self):
        with patch.object(syft, "open_https", return_value=Response(b"x" * 100)):
            self.assertEqual(len(syft.download(URL, 8)), 9)

    def test_timeout_exhaustion_is_four_attempts_and_123_backoff(self):
        with patch.object(syft, "open_https", side_effect=TimeoutError()) as opened, patch.object(syft.time, "sleep") as sleep:
            with self.assertRaisesRegex(RuntimeError, "TimeoutError"):
                syft.download(URL, 8)
            self.assertEqual(opened.call_count, 4)
            self.assertEqual([call.args[0] for call in sleep.call_args_list], [1, 2, 3])

    def test_permanent_http_and_certificate_errors_do_not_retry(self):
        for error in (urllib.error.HTTPError(URL, 404, "absent", {}, None),
                      urllib.error.URLError(ssl.SSLCertVerificationError("bad certificate"))):
            with self.subTest(error=type(error)), patch.object(syft, "open_https", side_effect=error) as opened, patch.object(syft.time, "sleep") as sleep:
                with self.assertRaises(RuntimeError):
                    syft.download(URL, 8)
                self.assertEqual(opened.call_count, 1)
                sleep.assert_not_called()

    def test_transient_http_recovers_and_closes_failed_response(self):
        error = urllib.error.HTTPError(URL, 503, "unavailable", {}, io.BytesIO(b"partial"))
        with patch.object(syft, "open_https", side_effect=[error, Response(b"complete")]), patch.object(syft.time, "sleep"):
            self.assertEqual(syft.download(URL, 8), b"complete")
            self.assertTrue(error.closed)

    def test_https_downgrade_is_refused_before_following_redirect(self):
        request = urllib.request.Request(URL)
        with self.assertRaisesRegex(RuntimeError, "outside HTTPS"):
            syft.HTTPSRedirect().redirect_request(request, None, 302, "redirect", {}, "http://example.org/payload")

    def install_fixture(self, folder, payload, content=None, path_file=None):
        root = Path(folder)
        entry = ("arm64", len(payload), hashlib.sha256(payload).hexdigest())
        with patch.object(syft.platform, "system", return_value="Linux"), patch.object(syft.platform, "machine", return_value="aarch64"), patch.dict(syft.RELEASES, {"aarch64": entry}), patch.object(syft, "download", return_value=payload if content is None else content) as download:
            syft.install(root, path_file or root / "github-path")
            download.assert_called_once_with(URL, len(payload))

    def test_success_publishes_only_regular_binary_after_integrity_check(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            self.install_fixture(folder, archive_bytes())
            destination = Path((root / "github-path").read_text().strip())
            self.assertEqual((destination / "syft").read_bytes(), b"owned fixture binary")
            self.assertEqual((destination / "syft").stat().st_mode & 0o777, 0o755)
            self.assertEqual(sorted(p.name for p in root.iterdir()), sorted([destination.name, "github-path"]))

    def test_same_size_corruption_is_final_and_never_publishes_path(self):
        payload = archive_bytes()
        corrupted = bytes([payload[0] ^ 1]) + payload[1:]
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(RuntimeError, "checksum mismatch"):
                self.install_fixture(folder, payload, corrupted)
            self.assertEqual(list(Path(folder).iterdir()), [])

    def test_truncation_and_excess_data_are_final(self):
        payload = archive_bytes()
        for content in (payload[:-1], payload + b"extra"):
            with tempfile.TemporaryDirectory() as folder:
                with self.assertRaisesRegex(RuntimeError, "size/checksum mismatch"):
                    self.install_fixture(folder, payload, content)
                self.assertEqual(list(Path(folder).iterdir()), [])

    def test_symlink_archive_is_never_installed(self):
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(RuntimeError, "regular binary"):
                self.install_fixture(folder, archive_bytes(symlink=True))
            self.assertEqual(list(Path(folder).iterdir()), [])

    def test_path_publication_failure_cleans_owned_partial_install(self):
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaises(FileNotFoundError):
                self.install_fixture(folder, archive_bytes(), path_file=Path(folder) / "absent/github-path")
            self.assertEqual(list(Path(folder).iterdir()), [])


if __name__ == "__main__":
    unittest.main()
