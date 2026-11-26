#!/usr/bin/env python3
"""Use the reviewed official pins as publishing inputs; never move a pin."""
import json
import re
import sys
from pathlib import Path


def matrix(source):
    primaries = source.split("var nativeToolchains =", 1)[1].split("// nativeBun", 1)[0]
    include = []
    rows = re.findall(r'\{\s*line:\s*"[^"\n]+"[^{}]*\}', primaries)
    if len(rows) != len(re.findall(r'\bimage\s*:\s*"', primaries)):
        raise ValueError("native toolchain matrix contains an unrecognized row")
    for row in rows:
        # Never derive a derivative from itself after adoption. The original
        # reviewed source pin remains independently tracked in the inventory.
        match = (
            re.search(r'\bupstream:\s*"([^"]+)"', row)
            or re.search(r'\bimage:\s*"([^"]+)"', row)
        )
        image = match.group(1) if match else ""
        if not re.fullmatch(r"(?:elixir|golang|node|python|ruby|rust):[a-z0-9.-]+@sha256:[0-9a-f]{64}", image):
            raise ValueError("native publishing input must be a reviewed official digest: " + image)
        tag = image.split("@", 1)[0].replace(":", "-")
        include.append({"name": tag, "base": image})
    if not include:
        raise ValueError("native toolchain matrix is empty")
    # Release this backport's eleven frozen lines before unrelated refreshes.
    # Every reviewed line remains in the matrix and passes the same gates.
    legacy = re.compile(r"^(node-(14|16|18|20)-|python-3\.(7|8|9|10)-|ruby-3\.(0|1|2)-)")
    include.sort(key=lambda image: not bool(legacy.match(image["name"])))
    return {"include": include}


# Exact releases no line pins, published only as runtime donors by explicit
# singleton dispatch: Render's dated Node 24 defaults before the line's
# Brotli fix (w3/m672). Keys equal the identities the exact resolver pins.
EXACT_DONORS = (
    "node:24.14.1-bookworm@sha256:80fc934952c8f1b2b4d39907af7211f8a9fff1a4c2cf673fb49099292c251cec",
    "node:24.21.0-bookworm@sha256:3d27e5c11e5786e309ec3e03f93ae536eb36e6e5eb3714d5eb3300a36157add0",
)


def exact_donors():
    rows = []
    for image in EXACT_DONORS:
        if not re.fullmatch(r"node:[0-9]+\.[0-9]+\.[0-9]+-bookworm@sha256:[0-9a-f]{64}", image):
            raise ValueError("exact donor input must be a reviewed official digest: " + image)
        rows.append({"name": image.split("@", 1)[0].replace(":", "-"), "base": image})
    return rows


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    source = (root / "lego/operator/internal/build/native.go").read_text()
    if len(sys.argv) == 3 and sys.argv[1] == "--changed-from":
        print(str(matrix(source) != matrix(Path(sys.argv[2]).read_text())).lower())
    elif len(sys.argv) == 1:
        rows = matrix(source)["include"] + exact_donors()
        print(json.dumps({"include": rows}, separators=(",", ":")))
    else:
        raise SystemExit("usage: matrix.py [--changed-from previous-native.go]")
