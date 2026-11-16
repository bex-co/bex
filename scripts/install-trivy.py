#!/usr/bin/env python3
"""Install the fixed CVE scanner from its reviewed official release."""
import importlib.util
import os
from pathlib import Path
import sys

# Transport retries, integrity checks and single-binary extraction are
# install-syft.py's; this file only names Trivy's reviewed archives.
SPEC = importlib.util.spec_from_file_location("install_syft", Path(__file__).with_name("install-syft.py"))
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)

# Reviewed releases only; trivy_<version>_checksums.txt and GitHub asset
# digests agree for every archive below. 0.70.0 is what trivy-action v0.36.0
# and deploy.yml's setup-trivy install.
# https://github.com/aquasecurity/trivy/releases/tag/v0.70.0 (checked 2026-11-16)
VERSION = "0.70.0"
RELEASES = {
    "0.70.0": {
        "x86_64": ("64bit", 47748408, "8b4376d5d6befe5c24d503f10ff136d9e0c49f9127a4279fd110b727929a5aa9"),
        "aarch64": ("ARM64", 43159403, "2f6bb988b553a1bbac6bdd1ce890f5e412439564e17522b88a4541b4f364fc8d"),
    },
}
URL = "https://github.com/aquasecurity/trivy/releases/download/v%s/trivy_%s_Linux-%s.tar.gz"


def install(temporary_root, path_file, version=VERSION):
    release.install_release(temporary_root, path_file, "Trivy", RELEASES, version, URL)


if __name__ == "__main__":
    install(Path(os.environ["RUNNER_TEMP"]), Path(os.environ["GITHUB_PATH"]),
            sys.argv[1] if len(sys.argv) > 1 else VERSION)
