"""Common-image input and publication routing controls; no containers or network."""
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest


HERE = Path(__file__).resolve().parent


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


common = load("native_common_matrix", HERE / "matrix.py")
checks = load("native_common_checks", HERE / "check-common.py")
otp = load("native_otp_features", HERE / "otp-features.py")


class CommonImageTests(unittest.TestCase):
    def test_otp_donor_must_retain_beam_native_driver_and_matching_headers(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            reference, target = root / "reference", root / "target"
            for name in ("lib/wx-2.5/ebin/wx.beam", "lib/wx-2.5/priv/wxe_driver.so", "lib/wx-2.5/include/wx.hrl", "usr/include/erl_nif.h"):
                for tree in (reference, target):
                    path = tree / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(b"source-only inventory control")
            self.assertTrue(otp.complete(reference, target))
            (target / "lib/wx-2.5/priv/wxe_driver.so").unlink()
            self.assertFalse(otp.complete(reference, target))
            empty = root / "empty"
            empty.mkdir()
            with self.assertRaises(ValueError):
                otp.complete(empty, target)

    def test_real_cli_supplies_one_common_row_and_reviewed_base(self):
        result = subprocess.run(["python3", str(HERE / "matrix.py")], check=True, capture_output=True)
        rows = json.loads(result.stdout)["include"]
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["name"], "common-bookworm")
        source = json.loads((HERE / "upstream.json").read_text())
        self.assertEqual(rows[0]["base"], source["ref"])
        self.assertEqual(set(source["platforms"]), {"amd64", "arm64"})

    def test_missing_ambiguous_or_other_distribution_pin_is_refused(self):
        source = (HERE / "Dockerfile").read_text()
        for changed in ("", source.replace("buildpack-deps:bookworm@", "buildpack-deps:trixie@"),
                        source.replace("@sha256:", ":"), source + "\nARG BASE_IMAGE=extra\n"):
            with self.subTest(source=changed[:70]), self.assertRaises(ValueError):
                common.matrix(changed)

    def test_common_layout_refuses_other_distribution_and_dangling_primary(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "etc").mkdir()
            release = root / "etc/os-release"
            release.write_text("VERSION_CODENAME=trixie\n")
            with self.assertRaisesRegex(RuntimeError, "Bookworm"):
                checks.check_layout(root)
            release.write_text("VERSION_CODENAME=bookworm\n")
            checks.check_layout(root)
            primary = root / "usr/local/bin/python3"
            primary.parent.mkdir(parents=True)
            primary.symlink_to("missing-interpreter")
            with self.assertRaisesRegex(RuntimeError, "primary runtime"):
                checks.check_layout(root)

    def test_all_wheels_are_required_even_without_a_primary_interpreter(self):
        with tempfile.TemporaryDirectory() as directory:
            store = Path(directory)
            (store / "wheels").mkdir()
            artifacts = [{"id": name, "filename": name, "sha256": hashlib.sha256(name.encode()).hexdigest()}
                         for name in ("control-one.whl", "control-two.whl")]
            (store / "python-tools.json").write_text(json.dumps({"artifacts": artifacts}))
            (store / "wheels/control-one.whl").write_bytes(b"control-one.whl")
            with self.assertRaises(FileNotFoundError):
                checks.check_wheels(store)
            (store / "wheels/control-two.whl").write_bytes(b"control-two.whl")
            checks.check_wheels(store)
            (store / "wheels/control-one.whl").write_bytes(b"changed")
            with self.assertRaisesRegex(RuntimeError, "incomplete or changed"):
                checks.check_wheels(store)


if __name__ == "__main__":
    unittest.main()
