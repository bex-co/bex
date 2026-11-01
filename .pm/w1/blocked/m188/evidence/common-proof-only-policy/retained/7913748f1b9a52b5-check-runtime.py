"""Additional exact-default proof; existing frozen utility programs stay intact."""
import ctypes
import json
import os
from pathlib import Path
import subprocess
import sys
import sysconfig


def main():
    phase, expected = sys.argv[1:]
    if phase not in ("build", "runtime"):
        raise RuntimeError("expected build or runtime phase")
    actual = sys.version.split()[0]
    if actual != expected or os.environ.get("PYTHON_VERSION") != expected:
        raise RuntimeError("default primary version/environment changed")
    if not sys.executable.startswith("/usr/local/"):
        raise RuntimeError("system Python substituted for selected primary")
    if "VERSION_CODENAME=bookworm" not in Path("/etc/os-release").read_text().splitlines():
        raise RuntimeError("generated image left the common Bookworm foundation")
    # Loading by SONAME exercises the real loader/cache, not a guessed path.
    library = sysconfig.get_config_var("LDLIBRARY")
    ctypes.CDLL(library)
    linked = subprocess.run(["ldd", sys.executable], check=True, capture_output=True, text=True).stdout
    if "not found" in linked:
        raise RuntimeError("selected primary has unresolved shared libraries")
    root = Path(__file__).resolve().parent
    if phase == "build":
        subprocess.run([sys.executable, str(root / "check-primary-python.py")], check=True)
    subprocess.run(["/usr/bin/python3", str(root / "check-brotli.py")], check=True)
    output = root / "qa-output"
    output.mkdir(exist_ok=True)
    receipt = {"phase": phase, "python": actual, "python_env": os.environ["PYTHON_VERSION"],
               "executable": sys.executable, "loaded_library": library, "ldd": linked,
               "packaging_and_c_extension_executed": phase == "build", "pass": True}
    (output / ("common-default-" + phase + ".json")).write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt))


if __name__ == "__main__":
    main()
