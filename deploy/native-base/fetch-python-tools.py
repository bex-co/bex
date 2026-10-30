"""Fetch only the reviewed pure-Python wheels while publishing a native base."""
from email.parser import Parser
import hashlib
import io
import json
from pathlib import Path
import urllib.parse
import urllib.request
import zipfile


root = Path(__file__).resolve().parent
manifest = json.loads((root / "python-tools.json").read_text())
destination = root / "wheels"
destination.mkdir(exist_ok=True)
for artifact in manifest["artifacts"]:
    url = urllib.parse.urlsplit(artifact["url"])
    if url.scheme != "https" or url.netloc != "files.pythonhosted.org":
        raise RuntimeError("Python tool artifact must come from public PyPI")
    with urllib.request.urlopen(artifact["url"], timeout=60) as response:
        content = response.read(8 * 1024 * 1024 + 1)
    if len(content) > 8 * 1024 * 1024 or hashlib.sha256(content).hexdigest() != artifact["sha256"]:
        raise RuntimeError("Python tool checksum mismatch: " + artifact["id"])
    with zipfile.ZipFile(io.BytesIO(content)) as wheel:
        prefix = artifact["name"].replace("-", "_") + "-" + artifact["version"] + ".dist-info/"
        metadata = Parser().parsestr(wheel.read(prefix + "METADATA").decode())
        wheel_info = Parser().parsestr(wheel.read(prefix + "WHEEL").decode())
        if (metadata["Name"].lower() != artifact["name"] or
                metadata["Version"] != artifact["version"] or
                metadata["Requires-Python"] != artifact["requires_python"] or
                wheel_info["Root-Is-Purelib"] != "true" or
                wheel_info["Tag"] != "py3-none-any"):
            raise RuntimeError("Python tool metadata differs from its review: " + artifact["id"])
    (destination / artifact["filename"]).write_bytes(content)
    print(artifact["sha256"] + "  " + artifact["filename"])
