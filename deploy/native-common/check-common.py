"""Offline checks for the language-free common image and its complete wheel store."""
import ctypes
import ctypes.util
import hashlib
import json
from pathlib import Path
import subprocess


PRIMARY_PATHS = ("usr/local/bin/node", "usr/local/bin/python3", "usr/local/bin/ruby",
                 "usr/local/go", "usr/local/cargo", "usr/local/lib/erlang", "usr/local/lib/elixir")


def check_layout(root):
    os_release = dict(line.split("=", 1) for line in (root / "etc/os-release").read_text().splitlines()
                      if "=" in line and not line.startswith("#"))
    if os_release.get("VERSION_CODENAME", "").strip('"') != "bookworm":
        raise RuntimeError("common native image must remain Bookworm")
    for name in PRIMARY_PATHS:
        path = root / name
        if path.exists() or path.is_symlink():
            raise RuntimeError("common native image contains a primary runtime: " + name)


def check_wheels(store):
    manifest = json.loads((store / "python-tools.json").read_text())
    for artifact in manifest["artifacts"]:
        content = (store / "wheels" / artifact["filename"]).read_bytes()
        if hashlib.sha256(content).hexdigest() != artifact["sha256"]:
            raise RuntimeError("common native wheel store is incomplete or changed: " + artifact["id"])


def main():
    check_layout(Path("/"))
    check_wheels(Path("/opt/bex/python-tools"))
    # Genuine coinstalled packages retain dpkg/SBOM/scan identity. Distinct
    # SONAMEs supply old runtimes without replacing Bookworm's current ABIs.
    for library in ("libssl.so.1.1", "libcrypto.so.1.1", "libffi.so.7", "libssl.so.3", "libffi.so.8"):
        ctypes.CDLL(library)
    # Importing the system interpreter's development modules cannot prove a
    # subsequently selected primary's matching headers; that remains its own gate.
    for library in ("vips", "pq", "ssl", "ffi", "tcl8.6", "tk8.6", "uuid", "zstd", "jemalloc", "odbc", "sctp"):
        name = ctypes.util.find_library(library)
        if not name:
            raise RuntimeError("common native runtime dependency missing: " + library)
        # jemalloc uses initial-exec TLS and must be linked at process startup.
        # Its actual Ruby interpreter gate remains required; late Python dlopen
        # is not a valid test of that startup contract.
        if library != "jemalloc":
            ctypes.CDLL(name)
    subprocess.run(["/opt/bex/native-node-tools/node", "-e",
                    "if(process.versions.node!=='24.22.1')process.exit(1)"], check=True)
    print("common Bookworm layout, runtime libraries and complete offline wheel store verified")


if __name__ == "__main__":
    main()
