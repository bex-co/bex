#!/usr/bin/env python3
"""materialize-evidence places exactly the pinned artifact's needed packets (w6/m164)."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("native_common_qualify", HERE / "native-common-qualify.py")
q = importlib.util.module_from_spec(spec)
sys.modules["native_common_qualify"] = q
spec.loader.exec_module(q)

DIGEST = "ghcr.io/bex-co/bex-evidence@sha256:" + "1" * 64
PACKET = ".pm/w8/m91/evidence/packet-a"
PATTERN = "/" + PACKET + "/**"


class Materialize(unittest.TestCase):
    def setUp(self):
        self.root = Path(tempfile.mkdtemp())
        self.addCleanup(lambda: subprocess.run(["rm", "-rf", str(self.root)]))
        (self.root / ".github").mkdir()
        (self.root / q.CANDIDATE_PATH).write_text(json.dumps({"schema": "x", "candidate": {"approval": {"custody": {}}}}))
        self.location({"schema": "bex.native-evidence-location/v1", "artifact": DIGEST, "packets": [PACKET]})

    def location(self, value):
        (self.root / q.EVIDENCE_LOCATION).write_text(json.dumps(value))

    def run_materialize(self, pulled_files=("CUSTODY.json", "data.json"), exit_code=0, needed=(PATTERN,)):
        def fake_pull(argv, **_):
            self.assertIn("--keep-old-files", argv)
            self.assertIn(DIGEST, argv)
            staging = Path(argv[argv.index("-o") + 1])
            for name in pulled_files:
                (staging / PACKET).mkdir(parents=True, exist_ok=True)
                (staging / PACKET / name).write_text(name)
            return subprocess.CompletedProcess(argv, exit_code, "", "denied" if exit_code else "")
        with patch.object(q.subprocess, "run", side_effect=fake_pull), \
                patch.object(q, "checkout_evidence_packets", return_value=list(needed)), \
                patch.object(q, "evidence_file", side_effect=lambda root, d: Path(root) / PACKET / "CUSTODY.json"), \
                patch.object(q, "json_file", side_effect=lambda p: json.loads(Path(p).read_text()) if p.name != "CUSTODY.json" else {"files": {}}):
            q.materialize_evidence("reviewed-candidate", self.root)

    def test_fresh_pulls_nothing(self):
        with patch.object(q.subprocess, "run", side_effect=AssertionError("no pull")):
            q.materialize_evidence("fresh", self.root)

    def test_the_needed_packet_is_placed(self):
        self.run_materialize()
        self.assertEqual((self.root / PACKET / "data.json").read_text(), "data.json")

    def test_a_packet_the_artifact_does_not_declare_is_refused(self):
        with self.assertRaisesRegex(ValueError, "lacks packets: .pm/w8/m91/evidence/packet-b"):
            self.run_materialize(needed=(PATTERN, "/.pm/w8/m91/evidence/packet-b/**"))

    def test_an_existing_packet_is_never_replaced(self):
        (self.root / PACKET).mkdir(parents=True)
        with self.assertRaisesRegex(ValueError, "already present"):
            self.run_materialize()

    def test_a_failed_pull_fails(self):
        with self.assertRaisesRegex(ValueError, "pull failed: denied"):
            self.run_materialize(exit_code=1)

    def test_the_location_must_pin_a_digest(self):
        self.location({"schema": "bex.native-evidence-location/v1", "artifact": "ghcr.io/bex-co/bex-evidence:latest", "packets": [PACKET]})
        with self.assertRaisesRegex(ValueError, "invalid evidence location"), \
                patch.object(q, "checkout_evidence_packets", return_value=[PATTERN]):
            q.materialize_evidence("reviewed-candidate", self.root)


if __name__ == "__main__":
    unittest.main()
