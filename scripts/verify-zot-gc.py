#!/usr/bin/env python3
"""Reproduce last-manifest deletion followed by a metadata rebuild on restart.

Runs only disposable Docker containers. Pass an image to check an older release:
  python3 scripts/verify-zot-gc.py ghcr.io/project-zot/zot:v2.1.18
The old release must fail; the production-pinned image must pass.
"""

import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


def docker(*args, check=True):
    return subprocess.run(
        ["docker", *args], check=check, text=True, capture_output=True, timeout=90
    )


def main():
    image = sys.argv[1] if len(sys.argv) > 1 else (
        "ghcr.io/project-zot/zot:v2.1.22@"
        "sha256:96cda11459ce6f8c60da3b03f03e9f67fd11b3ae2d82c4c04c92b3c608996080"
    )
    name = "bex-w7-gc-" + uuid.uuid4().hex[:10]
    with tempfile.TemporaryDirectory(prefix="bex-zot-gc-") as directory:
        root = Path(directory)
        data_directory = root / "data"
        data_directory.mkdir(mode=0o777)
        data_directory.chmod(0o777)
        config = {
            "distSpecVersion": "1.1.0",
            "storage": {
                "rootDirectory": "/var/lib/registry",
                "dedupe": True,
                "gc": True,
                "gcDelay": "30s",
                "gcInterval": "5s",
                "retention": {"policies": [{
                    "repositories": ["**"],
                    "deleteUntagged": True,
                    "keepTags": [{"patterns": [".*"], "mostRecentlyPushedCount": 5}],
                }]},
            },
            "http": {"address": "0.0.0.0", "port": "5000"},
            "extensions": {"search": {"enable": True}},
            "log": {"level": "debug"},
        }
        (root / "config.json").write_text(json.dumps(config))
        try:
            def start(version):
                docker(
                    "run", "-d", "--name", name, "-p", "127.0.0.1::5000",
                    "-v", f"{root / 'config.json'}:/etc/zot/config.json:ro",
                    "-v", f"{data_directory}:/var/lib/registry", version,
                )
                port = docker("port", name, "5000/tcp").stdout.strip().split(":")[-1]
                return "http://127.0.0.1:" + port

            origin = start("ghcr.io/project-zot/zot:v2.1.18")

            def request(path, method="GET", data=None, content_type=None):
                headers = {"Content-Type": content_type} if content_type else {}
                req = urllib.request.Request(
                    urllib.parse.urljoin(origin, path), data=data,
                    method=method, headers=headers,
                )
                try:
                    with urllib.request.urlopen(req, timeout=5) as response:
                        return response.status, response.headers, response.read()
                except urllib.error.HTTPError as error:
                    return error.code, error.headers, error.read()

            def ready():
                deadline = time.monotonic() + 45
                while time.monotonic() < deadline:
                    try:
                        if request("/v2/")[0] == 200:
                            return
                    except (OSError, urllib.error.URLError):
                        pass
                    time.sleep(0.25)
                raise AssertionError("registry did not become ready")

            def push(repo, payload):
                def blob(data):
                    digest = "sha256:" + hashlib.sha256(data).hexdigest()
                    code, headers, _ = request(f"/v2/{repo}/blobs/uploads/", "POST", b"")
                    assert code == 202, ("start blob upload", code)
                    location = headers["Location"]
                    separator = "&" if "?" in location else "?"
                    code, _, _ = request(
                        location + separator + urllib.parse.urlencode({"digest": digest}),
                        "PUT", data, "application/octet-stream",
                    )
                    assert code == 201, ("finish blob upload", code)
                    return {"digest": digest, "size": len(data)}

                manifest = json.dumps({
                    "schemaVersion": 2,
                    "mediaType": "application/vnd.oci.image.manifest.v1+json",
                    "config": {**blob(b"{}"), "mediaType": "application/vnd.oci.image.config.v1+json"},
                    "layers": [{**blob(payload), "mediaType": "application/vnd.oci.image.layer.v1.tar"}],
                }).encode()
                code, _, _ = request(
                    f"/v2/{repo}/manifests/live", "PUT", manifest,
                    "application/vnd.oci.image.manifest.v1+json",
                )
                assert code == 201, ("manifest push", code)
                return "sha256:" + hashlib.sha256(manifest).hexdigest(), manifest

            ready()
            live_digest, live_manifest = push("fixture/live", b"retained live image")
            retained = json.loads(live_manifest)
            retained_descriptors = [retained["config"], *retained["layers"]]
            dead_digest, _ = push("fixture/empty", b"unreferenced bytes to collect")
            assert request(f"/v2/fixture/empty/manifests/{dead_digest}", "DELETE")[0] == 202
            orphan_directory = data_directory / "fixture/empty"
            assert list(orphan_directory.glob("blobs/sha256/*")), "no orphan blobs to collect"
            docker("rm", "-f", name)
            # Only the disposable fixture's metadata is removed. Rebuilding it
            # from manifests preserves the live repo but omits the empty repo.
            (data_directory / "meta.db").unlink()
            origin = start(image)
            ready()

            deadline = time.monotonic() + 100
            cycles = 0
            collected = False
            while time.monotonic() < deadline:
                logs = docker("logs", name).stdout
                cycles = sum(
                    "gc successfully completed for /var/lib/registry/fixture/live" in line
                    for line in logs.splitlines()
                )
                assert request(f"/v2/fixture/live/manifests/{live_digest}")[2] == live_manifest
                for descriptor in retained_descriptors:
                    code, _, body = request(f"/v2/fixture/live/blobs/{descriptor['digest']}")
                    assert code == 200 and len(body) == descriptor["size"]
                    assert "sha256:" + hashlib.sha256(body).hexdigest() == descriptor["digest"]
                collected = not orphan_directory.exists()
                if collected and cycles >= 2:
                    print(f"PASS: {image}: restart then {cycles} GC cycles, orphan repo removed, live manifest intact")
                    return
                time.sleep(2)
            errors = [
                line for line in logs.splitlines()
                if '"repository":"fixture/empty"' in line and "failed to get repoMeta" in line
            ]
            raise AssertionError(
                f"orphan repo collected={collected}, successful GC cycles={cycles}, "
                f"missing-metadata errors={len(errors)}"
            )
        except Exception:
            logs = docker("logs", "--tail", "20", name, check=False)
            print(logs.stdout + logs.stderr, file=sys.stderr)
            raise
        finally:
            docker("rm", "-f", name, check=False)


if __name__ == "__main__":
    main()
