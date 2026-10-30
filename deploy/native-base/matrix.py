#!/usr/bin/env python3
"""Use the reviewed official pins as publishing inputs; never move a pin."""
import json
import re
from pathlib import Path

root = Path(__file__).resolve().parents[2]
source = (root / "lego/operator/internal/build/native.go").read_text()
primaries = source.split("var nativeToolchains =", 1)[1].split("// nativeBun", 1)[0]
images = re.findall(r'image: "([^"]+)"', primaries)
if not images or any(not re.fullmatch(r"[a-z0-9./:-]+@sha256:[0-9a-f]{64}", image) for image in images):
    raise SystemExit("native toolchain matrix contains an unpinned or malformed image")
include = []
for image in images:
    tag = image.split("@", 1)[0].replace(":", "-")
    include.append({"name": tag, "base": image})
print(json.dumps({"include": include}, separators=(",", ":")))
