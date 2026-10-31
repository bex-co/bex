import copy
from pathlib import Path
import tempfile
import unittest
import zlib

import check


class ProfileControls(unittest.TestCase):
    def test_unicode_arithmetic_and_native_results(self):
        for runtime, pure, name, checksum, arithmetic in (
            ("go", "go-pure|8|14|33153|Bex 雪 λ🙂", "go-cgo", zlib.adler32(check.TEXT.encode()), 5346),
            ("rust", "rust-pure|8|14|65536|🙂λ 雪 xeB", "rust-native", zlib.crc32(check.TEXT.encode()), 2566),
        ):
            outputs = {"pure": pure, "native": f"{name}|{checksum:08x}|{arithmetic}|1.2.13|1.2.13"}
            check.validate_outputs(runtime, outputs)
            for bad in (
                {**outputs, "pure": pure.replace("|8|14|", "|7|14|")},
                {**outputs, "pure": pure.replace("雪", "?")},
                {**outputs, "native": outputs["native"].replace(f"{checksum:08x}", "00000000")},
                {**outputs, "native": outputs["native"].replace(str(arithmetic), "0")},
                {**outputs, "native": outputs["native"][:-1] + "4"},
                {**outputs, "native": ""},
            ):
                with self.subTest(runtime=runtime, outputs=bad), self.assertRaises(RuntimeError):
                    check.validate_outputs(runtime, bad)

    def test_primary_identity_and_environment_refusals(self):
        fixtures = [
            ({"runtime": "go", "version": "1.27.3", "env": ["GOTOOLCHAIN=local", "GOPATH=/go"]}, {"env": {"GOTOOLCHAIN": "local", "GOPATH": "/go"}, "executable": "/usr/local/go/bin/go", "version": "go1.27.3", "goroot": "/usr/local/go", "gopath": "/go", "toolchain": "local"}, ["executable", "version", "goroot", "gopath", "toolchain"]),
            ({"runtime": "rust", "version": "1.96.0", "env": ["CARGO_HOME=/usr/local/cargo", "RUSTUP_HOME=/usr/local/rustup"]}, {"env": {"CARGO_HOME": "/usr/local/cargo", "RUSTUP_HOME": "/usr/local/rustup"}, "executable": "/usr/local/cargo/bin/cargo", "version": "rustc 1.96.0 (synthetic)", "sysroot": "/usr/local/rustup/toolchains/test", "rustc": "/usr/local/rustup/toolchains/test/bin/rustc", "target_libdir": "/usr/local/rustup/toolchains/test/lib/rustlib/target/lib"}, ["executable", "version", "sysroot", "rustc", "target_libdir"]),
        ]
        for expected, actual, fields in fixtures:
            check.validate_primary(expected, actual)
            for field in fields:
                changed = copy.deepcopy(actual)
                changed[field] = "/foreign/substitute"
                with self.subTest(runtime=expected["runtime"], field=field), self.assertRaises(RuntimeError):
                    check.validate_primary(expected, changed)
            changed = copy.deepcopy(actual)
            changed["env"][next(iter(changed["env"]))] = "foreign"
            with self.assertRaises(RuntimeError):
                check.validate_primary(expected, changed)
        with self.assertRaises(RuntimeError):
            check.env_map(["X=one", "X=two"])

    def test_tree_bytes_modes_links_and_inventory(self):
        for mutation in ("bytes", "mode", "link", "missing", "extra"):
            with tempfile.TemporaryDirectory() as directory:
                reference, target = Path(directory) / "reference", Path(directory) / "target"
                for tree in (reference, target):
                    tree.mkdir()
                    (tree / "compiler").write_bytes(b"exact-donor-compiler")
                    (tree / "compiler").chmod(0o755)
                    (tree / "proxy").symlink_to("compiler")
                check.equal_trees(reference, target)
                if mutation == "bytes":
                    (target / "compiler").write_bytes(b"wrong-donor-compiler")
                elif mutation == "mode":
                    (target / "compiler").chmod(0o644)
                elif mutation == "link":
                    (target / "proxy").unlink()
                    (target / "proxy").symlink_to("foreign")
                elif mutation == "missing":
                    (target / "compiler").unlink()
                else:
                    (target / "stale-from-another-version").touch()
                with self.subTest(mutation=mutation), self.assertRaises(RuntimeError):
                    check.equal_trees(reference, target)


if __name__ == "__main__":
    unittest.main()
