"""Executed only by the private system-Python Poetry interpreter."""
import importlib.metadata
import json
import platform
import sys

from packaging.version import Version


def validate_tool(data):
    if (not data["python"].startswith("3.11.") or data["base_prefix"] != "/usr"
            or data["prefix"] != "/opt/bex/poetry" or data["poetry"] != "2.1.3"
            or not Version("20.26.6") <= Version(data["virtualenv"]) < Version("20.33.0")):
        raise RuntimeError("legacy Poetry must use its reviewed system-Python dependency solve")


def main():
    data = {"python": sys.version.split()[0], "base_prefix": sys.base_prefix,
            "prefix": sys.prefix, "executable": sys.executable, "os": platform.system(),
            "machine": platform.machine(), "poetry": importlib.metadata.version("poetry"),
            "virtualenv": importlib.metadata.version("virtualenv")}
    validate_tool(data)
    data["virtualenv_cap_satisfied"] = True
    print(json.dumps(data))


if __name__ == "__main__":
    main()
