"""Run the real shell transfer function with deterministic curl transport faults."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SOURCE = Path(__file__).with_name("install.sh").read_text()
FUNCTION = "fetch_pgdg_key() {" + SOURCE.split("fetch_pgdg_key() {", 1)[1].split("\n}\n", 1)[0] + "\n}\n"


class KeyTransferTests(unittest.TestCase):
    def run_transfer(self, codes):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            curl = root / "curl"
            curl.write_text('''#!/usr/bin/env python3
import os, pathlib, sys
root = pathlib.Path(os.environ["FAKE_CURL_ROOT"])
log = root / "attempts"
prior = log.read_text().splitlines() if log.exists() else []
log.write_text("".join(x + "\\n" for x in prior) + "attempt\\n")
codes = os.environ["FAKE_CURL_CODES"].split(",")
code = int(codes[min(len(prior), len(codes) - 1)])
pathlib.Path(sys.argv[sys.argv.index("-o") + 1]).write_text("complete" if code == 0 else "partial")
sys.exit(code)
''')
            curl.chmod(0o700)
            sleep = root / "sleep"
            sleep.write_text('#!/bin/sh\nprintf "%s\\n" "$1" >> "$FAKE_CURL_ROOT/sleeps"\n')
            sleep.chmod(0o700)
            env = dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"], FAKE_CURL_ROOT=str(root), FAKE_CURL_CODES=",".join(map(str, codes)))
            result = subprocess.run(["sh", "-ec", FUNCTION + '\nfetch_pgdg_key https://www.postgresql.org/key "$FAKE_CURL_ROOT/key"\n'], env=env, capture_output=True, timeout=5)
            attempts = len((root / "attempts").read_text().splitlines())
            sleeps = (root / "sleeps").read_text().splitlines() if (root / "sleeps").exists() else []
            return result.returncode, attempts, sleeps, (root / "key").read_text()

    def test_tls_transport_failure_retries_with_a_fresh_complete_output(self):
        self.assertEqual(self.run_transfer([35, 0]), (0, 2, ["1"], "complete"))

    def test_exhaustion_is_bounded_and_keeps_original_failure(self):
        self.assertEqual(self.run_transfer([35] * 4), (35, 4, ["1", "2", "3"], "partial"))

    def test_http_and_certificate_failures_stop_without_retry(self):
        for status in (22, 60):
            with self.subTest(status=status):
                self.assertEqual(self.run_transfer([status]), (status, 1, [], "partial"))


if __name__ == "__main__":
    unittest.main()
