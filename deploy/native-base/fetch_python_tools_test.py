"""Publication transfers retry transport faults without retrying integrity failures."""
import hashlib
import http.client
import importlib.util
import io
import json
from pathlib import Path
import ssl
import tempfile
import unittest
from unittest.mock import call, patch
import urllib.error
import zipfile

spec = importlib.util.spec_from_file_location("fetch_python_tools", Path(__file__).with_name("fetch-python-tools.py"))
fetcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fetcher)
URL = "https://files.pythonhosted.org/reviewed.whl"


class FetchTests(unittest.TestCase):
    def test_timeout_and_partial_response_are_discarded_before_complete_retry(self):
        interrupted = unittest.mock.MagicMock()
        interrupted.__enter__.return_value.read.side_effect = http.client.IncompleteRead(b"partial", 20)
        with patch.object(fetcher.urllib.request, "urlopen", side_effect=[
            urllib.error.URLError(TimeoutError("TLS handshake timed out")), interrupted, io.BytesIO(b"complete"),
        ]) as opened, patch.object(fetcher.time, "sleep") as sleep:
            self.assertEqual(fetcher.fetch(URL), b"complete")
        self.assertEqual(opened.call_args_list, [call(URL, timeout=60)] * 3)
        self.assertEqual(sleep.call_args_list, [call(1), call(2)])
        interrupted.__exit__.assert_called_once()

    def test_exhausted_transport_attempts_raise_and_stop(self):
        error = urllib.error.URLError(TimeoutError("TLS handshake timed out"))
        with patch.object(fetcher.urllib.request, "urlopen", side_effect=error) as opened, patch.object(fetcher.time, "sleep") as sleep:
            with self.assertRaises(urllib.error.URLError):
                fetcher.fetch(URL)
        self.assertEqual(opened.call_count, 3)
        self.assertEqual(sleep.call_count, 2)

    def test_transient_http_error_gets_a_complete_retry(self):
        error = urllib.error.HTTPError(URL, 503, "unavailable", {}, None)
        with patch.object(fetcher.urllib.request, "urlopen", side_effect=[error, io.BytesIO(b"complete")]) as opened, patch.object(fetcher.time, "sleep"):
            self.assertEqual(fetcher.fetch(URL), b"complete")
        self.assertEqual(opened.call_count, 2)

    def test_permanent_http_and_certificate_errors_do_not_retry(self):
        for error in (
            urllib.error.HTTPError(URL, 404, "missing", {}, None),
            ssl.SSLCertVerificationError("untrusted certificate"),
            urllib.error.URLError(ssl.SSLCertVerificationError("untrusted certificate")),
        ):
            with self.subTest(error=error), patch.object(fetcher.urllib.request, "urlopen", side_effect=error) as opened, patch.object(fetcher.time, "sleep") as sleep:
                with self.assertRaises(type(error)):
                    fetcher.fetch(URL)
                self.assertEqual(opened.call_count, 1)
                sleep.assert_not_called()

    def test_verified_wheel_is_written_and_incompatible_metadata_is_refused(self):
        for pure in ("true", "false"):
            with self.subTest(pure=pure), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                payload = io.BytesIO()
                with zipfile.ZipFile(payload, "w") as wheel:
                    wheel.writestr("reviewed-1.0.dist-info/METADATA", "Name: reviewed\nVersion: 1.0\nRequires-Python: >=3.7\n")
                    wheel.writestr("reviewed-1.0.dist-info/WHEEL", "Root-Is-Purelib: " + pure + "\nTag: py3-none-any\n")
                content = payload.getvalue()
                artifact = {"url": URL, "sha256": hashlib.sha256(content).hexdigest(), "id": "reviewed", "filename": "reviewed.whl",
                            "name": "reviewed", "version": "1.0", "requires_python": ">=3.7"}
                (root / "python-tools.json").write_text(json.dumps({"artifacts": [artifact]}))
                with patch.object(fetcher.urllib.request, "urlopen", return_value=io.BytesIO(content)) as opened, patch.object(fetcher.time, "sleep") as sleep, patch("builtins.print"):
                    if pure == "true":
                        fetcher.main(root)
                        self.assertEqual((root / "wheels" / "reviewed.whl").read_bytes(), content)
                    else:
                        with self.assertRaisesRegex(RuntimeError, "metadata differs"):
                            fetcher.main(root)
                        self.assertEqual(list((root / "wheels").iterdir()), [])
                self.assertEqual(opened.call_count, 1)
                sleep.assert_not_called()

    def test_checksum_or_oversized_payload_never_retries_or_writes_a_wheel(self):
        for content in (b"wrong bytes", b"x" * (8 * 1024 * 1024 + 1)):
            with self.subTest(size=len(content)), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                (root / "python-tools.json").write_text(json.dumps({"artifacts": [{
                    "url": URL, "sha256": "0" * 64, "id": "reviewed", "filename": "reviewed.whl",
                }]}))
                with patch.object(fetcher.urllib.request, "urlopen", return_value=io.BytesIO(content)) as opened, patch.object(fetcher.time, "sleep") as sleep:
                    with self.assertRaisesRegex(RuntimeError, "checksum mismatch"):
                        fetcher.main(root)
                self.assertEqual(opened.call_count, 1)
                sleep.assert_not_called()
                self.assertEqual(list((root / "wheels").iterdir()), [])


if __name__ == "__main__":
    unittest.main()
