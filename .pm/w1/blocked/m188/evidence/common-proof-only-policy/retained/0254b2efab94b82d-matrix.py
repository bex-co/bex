#!/usr/bin/env python3
"""Publish one common system image; runtime versions never become base rows."""
import json
from pathlib import Path
import re


def matrix(source):
    pins = re.findall(r"^ARG BASE_IMAGE=(\S+)$", source, re.MULTILINE)
    if len(pins) != 1 or not re.fullmatch(r"buildpack-deps:bookworm@sha256:[0-9a-f]{64}", pins[0]):
        raise ValueError("common native base must be one pinned Bookworm buildpack-deps image")
    return {"include": [{"name": "common-bookworm", "base": pins[0]}]}


if __name__ == "__main__":
    print(json.dumps(matrix(Path(__file__).with_name("Dockerfile").read_text()), separators=(",", ":")))
