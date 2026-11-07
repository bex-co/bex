#!/usr/bin/env python3
"""Install the fixed platform SBOM tool from its reviewed official release."""
import hashlib
import http.client
import io
import os
from pathlib import Path
import platform
import shutil
import ssl
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.request

# Reviewed releases only; official release checksums and GitHub asset digests
# agree for every archive below. 1.51.1 is the default (download-syft's former
# default); 1.55.0 is what native-common and pg-operand pin (native-base-image.yml
# still uses download-syft: its bytes are pinned by w8/m91's D3 custody).
# https://github.com/anchore/syft/releases/tag/v1.51.1
# https://github.com/anchore/syft/releases/tag/v1.55.0 (checked 2026-11-05)
VERSION = "1.51.1"
RELEASES = {
    "1.51.1": {
        "x86_64": ("amd64", 29203595, "8fcb33017a0dc1058298c923c436d19dfa68ae93968e0b423248542e3afb9fc3"),
        "aarch64": ("arm64", 26668605, "a7fd2b784e6664acd44719270574f6cd8c6864fc2b1700bf9099bd1cccda7d7f"),
    },
    "1.55.0": {
        "x86_64": ("amd64", 31198534, "bb65249542db5fedeeb4d3f9b018b75e7264dbfbbb10486f044581a92f90490d"),
        "aarch64": ("arm64", 28485268, "929bd163db2d7cd2255d3bcb8967208079813d7b46b0c952f7b794d8e2c7d304"),
    },
}
TRANSIENT_HTTP = (408, 425, 429, 500, 502, 503, 504)


class HTTPSRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, url):
        if not url.startswith("https://"):
            raise RuntimeError("Syft release redirected outside HTTPS")
        return super().redirect_request(request, response, code, message, headers, url)


def open_https(url):
    return urllib.request.build_opener(HTTPSRedirect()).open(url, timeout=60)


def download(url, size):
    """Fresh complete requests only; do not retry certificate/integrity errors."""
    for attempt in range(4):
        try:
            with open_https(url) as response:
                # A redirect may never downgrade the release transfer to HTTP.
                if not response.geturl().startswith("https://"):
                    raise RuntimeError("Syft release redirected outside HTTPS")
                return response.read(size + 1)
        except (urllib.error.URLError, TimeoutError, ConnectionError,
                http.client.IncompleteRead, ssl.SSLError) as error:
            reason = getattr(error, "reason", error)
            category = type(reason).__name__
            if isinstance(error, urllib.error.HTTPError):
                category = "HTTP " + str(error.code)
                error.close()
                if error.code not in TRANSIENT_HTTP:
                    raise RuntimeError("Syft download failed: " + category) from None
            if isinstance(reason, ssl.SSLCertVerificationError) or attempt == 3:
                raise RuntimeError("Syft download failed: " + category) from None
            # Never print a redirect URL: release-assets URLs carry signatures.
            print("Syft download retry %d/4: %s" % (attempt + 1, category), file=sys.stderr)
            time.sleep(attempt + 1)


def install(temporary_root, path_file, version=VERSION):
    if version not in RELEASES:
        raise RuntimeError("Syft %s is not a reviewed release" % version)
    releases = RELEASES[version]
    if platform.system() != "Linux" or platform.machine() not in releases:
        raise RuntimeError("Syft installer supports Linux amd64/arm64 runners only")
    arch, size, checksum = releases[platform.machine()]
    url = "https://github.com/anchore/syft/releases/download/v%s/syft_%s_linux_%s.tar.gz" % (version, version, arch)
    content = download(url, size)
    if len(content) != size or hashlib.sha256(content).hexdigest() != checksum:
        raise RuntimeError("Syft release archive size/checksum mismatch")
    # Read one regular member into a fresh file; archive paths and symlinks
    # cannot write outside the job's own installation directory.
    with tarfile.open(fileobj=io.BytesIO(content), mode="r:gz") as archive:
        member = archive.getmember("syft")
        if not member.isfile():
            raise RuntimeError("Syft release does not contain a regular binary")
        destination = Path(tempfile.mkdtemp(prefix="bex-syft-", dir=temporary_root))
        try:
            target = destination / "syft"
            with archive.extractfile(member) as binary, target.open("wb") as output:
                shutil.copyfileobj(binary, output)
            target.chmod(0o755)
            with path_file.open("a") as output:
                output.write(str(destination) + "\n")
        except BaseException:
            shutil.rmtree(destination)
            raise
    print("Installed Syft %s linux/%s from reviewed SHA256 %s" % (version, arch, checksum))


if __name__ == "__main__":
    install(Path(os.environ["RUNNER_TEMP"]), Path(os.environ["GITHUB_PATH"]),
            sys.argv[1] if len(sys.argv) > 1 else VERSION)
