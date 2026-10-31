"""Run inside default Poetry's private venv on the selected modern app Python."""
import importlib.metadata
import json
import os
import platform
import subprocess
import sys


EXPECTED_PYTHON = "@APP_VERSION@"


def validate_tool(data, expected):
    if (data["python"] != expected or data["python_env"] != expected
            or data["base_prefix"] != "/usr/local" or data["prefix"] != "/opt/bex/poetry"
            or not data["executable"].startswith("/opt/bex/poetry/bin/")
            or data["poetry"] != "2.1.3" or not data["virtualenv"]
            or data["dependency_check_succeeded"] is not True):
        raise RuntimeError("modern default Poetry must use the selected application Python privately")


def main():
    # pip checks the installed Poetry release's real dependency metadata,
    # including its virtualenv range, without adding a legacy upper bound.
    checked = subprocess.run([sys.executable, "-I", "-m", "pip", "check"],
                             capture_output=True, text=True, timeout=30)
    if checked.returncode:
        raise RuntimeError("private Poetry dependencies failed pip check: " + checked.stdout + checked.stderr)
    data = {"python": sys.version.split()[0], "python_env": os.environ.get("PYTHON_VERSION"),
            "base_prefix": sys.base_prefix, "prefix": sys.prefix, "executable": sys.executable,
            "os": platform.system(), "machine": platform.machine(),
            "poetry": importlib.metadata.version("poetry"),
            "virtualenv": importlib.metadata.version("virtualenv"),
            "dependency_check_succeeded": True, "dependency_check_output": checked.stdout.strip()}
    validate_tool(data, EXPECTED_PYTHON)
    print(json.dumps(data))


if __name__ == "__main__":
    main()
