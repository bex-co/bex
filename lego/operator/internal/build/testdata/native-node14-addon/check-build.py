"""An additional dependency-free addon control; no original fixture changes."""
import json
import os
from pathlib import Path
import subprocess
import sys


def output(*args):
    return subprocess.check_output(args, text=True).strip()


version = sys.argv[1]
before = {"node": output("node", "-p", "process.versions.node"), "npm": output("npm", "--version"),
          "python": output("python3", "-c", "import sys; print(sys.version.split()[0])"), "PATH": os.environ["PATH"]}
assert before["node"] == version
assert before["npm"] == "6.14.18"  # Selected Node14.21.3's unchanged bundled npm.
assert before["python"] == output("/usr/bin/python3", "-c", "import sys; print(sys.version.split()[0])")
assert not any(os.environ.get(key) for key in ("PYTHON", "npm_config_python", "NODE_GYP_FORCE_PYTHON"))
result = subprocess.run(["npm", "install", "--offline", "--build-from-source", "--nodedir=/usr/local",
    "--unsafe-perm", "--no-audit", "--package-lock=false", "--loglevel=verbose"], capture_output=True, text=True)
log = result.stdout + result.stderr
Path("qa-output").mkdir(exist_ok=True)
Path("qa-output/npm-build.log").write_text(log)
print(log)
assert result.returncode == 0, result.returncode
assert "/opt/bex/native-node-python/bin/python3.9" in log
assert before["node"] == output("node", "-p", "process.versions.node")
assert before["npm"] == output("npm", "--version")
assert before["python"] == output("python3", "-c", "import sys; print(sys.version.split()[0])")
assert before["PATH"] == os.environ["PATH"]
subprocess.run(["node", "check.js", version], check=True)
Path("qa-output/addon-build.json").write_text(json.dumps({"pass": True, "before": before,
    "scope": "actual offline npm/node-gyp compilation and import with matching selected Node headers"}, indent=2))
