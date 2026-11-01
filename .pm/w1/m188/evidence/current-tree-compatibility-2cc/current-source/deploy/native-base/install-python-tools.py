"""Restore reviewed packaging modules after an exact primary Python copy."""
from email.parser import Parser
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import sysconfig


root = Path(__file__).resolve().parent
manifest = json.loads((root / "python-tools.json").read_text())
version = sys.version_info[:2]
if version < (3, 7) or version > (3, 14):
    raise RuntimeError("native primary Python packaging requires a reviewed Python 3.7–3.14")
line = "3.7" if version == (3, 7) else "3.8" if version == (3, 8) else "3.9-3.14"
selected = [item for item in manifest["artifacts"] if item["id"] in manifest["selections"][line]]
site = Path(sysconfig.get_path("purelib"))
matches = True
for artifact in selected:
    content = (root / "wheels" / artifact["filename"]).read_bytes()
    if hashlib.sha256(content).hexdigest() != artifact["sha256"]:
        raise RuntimeError("offline Python tool checksum mismatch: " + artifact["id"])
    metadata = site / (artifact["name"] + "-" + artifact["version"] + ".dist-info") / "METADATA"
    matches = matches and metadata.is_file()
    if metadata.is_file():
        installed = Parser().parsestr(metadata.read_text())
        matches = matches and installed["Name"].lower() == artifact["name"] and installed["Version"] == artifact["version"]
probe = [sys.executable, "-I", "-c", "import setuptools, pkg_resources, wheel, distutils.sysconfig"]
if not matches or subprocess.run(probe, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
    subprocess.run([
        sys.executable, "-I", "-m", "pip", "--isolated", "install",
        "--no-index", "--no-deps", "--no-cache-dir", "--disable-pip-version-check",
        "--no-compile", "--force-reinstall",
        *[str(root / "wheels" / artifact["filename"]) for artifact in selected],
    ], check=True)
subprocess.run(probe, check=True)
record = {
    "python": sys.version.split()[0],
    "interpreter": sys.executable,
    "artifacts": [{key: artifact[key] for key in ("name", "version", "filename", "sha256")} for artifact in selected],
}
Path("/usr/share/bex-native-python-tools.json").write_text(json.dumps(record, indent=2) + "\n")
print("native primary Python packaging ready: " + ", ".join(artifact["id"] for artifact in selected))
