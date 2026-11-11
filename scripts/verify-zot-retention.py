#!/usr/bin/env python3
"""Run the registry's tag-retention rules on a real Zot (w8/m156).

A service's build is pushed as gen-<n> and, when a release runs on it, as
live-<n> for the same manifest. Zot gives each tag to the FIRST keepTags entry
whose pattern matches and applies that entry's count inside its own group, so
the order of the entries decides whether the live- rule ever sees a tag.

Pushes twelve builds that each went live, then six that never did, waits for
retention, and asserts what a running service and a rollback need:
  - the ten newest live- manifests are still there (by digest),
  - the two oldest live- manifests are gone,
  - of the builds that never went live, the five newest are there.

Runs only disposable Docker containers.
  python3 scripts/verify-zot-retention.py            # the rules as bex renders them
  python3 scripts/verify-zot-retention.py --legacy   # catch-all first: reproduces the loss

With --legacy it passes when the old order loses every build that went live,
which is the defect; any other outcome fails.
"""

import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

IMAGE = (
    "ghcr.io/project-zot/zot:v2.1.22@"
    "sha256:96cda11459ce6f8c60da3b03f03e9f67fd11b3ae2d82c4c04c92b3c608996080"
)
# lego/operator/internal/registry/zotconfig.go: defaultRetentionCount and
# liveRetentionCount, in the order buildBaseZotConfig renders them.
LIVE = {"patterns": ["^live-.*"], "mostRecentlyPushedCount": 10}
PUSHED = {"patterns": [".*"], "mostRecentlyPushedCount": 5}
LIVE_BUILDS, LATER_BUILDS = 12, 6
REPO = "tea-fixture/web"


def docker(*args, check=True):
    return subprocess.run(
        ["docker", *args], check=check, text=True, capture_output=True, timeout=120
    )


def main():
    legacy = "--legacy" in sys.argv[1:]
    keep_tags = [PUSHED, LIVE] if legacy else [LIVE, PUSHED]
    name = "bex-w8-retention-" + uuid.uuid4().hex[:10]
    with tempfile.TemporaryDirectory(prefix="bex-zot-retention-") as directory:
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
                "gcDelay": "5s",
                "gcInterval": "5s",
                "retention": {"policies": [{
                    "repositories": ["**"],
                    "deleteUntagged": True,
                    "keepTags": keep_tags,
                }]},
            },
            "http": {"address": "0.0.0.0", "port": "5000"},
            "log": {"level": "debug"},
        }
        (root / "config.json").write_text(json.dumps(config))
        try:
            docker(
                # As the caller, so the temporary directory can be removed.
                "run", "-d", "--name", name, "--user", f"{os.getuid()}:{os.getgid()}",
                "-p", "127.0.0.1::5000",
                "-v", f"{root / 'config.json'}:/etc/zot/config.json:ro",
                "-v", f"{data_directory}:/var/lib/registry", IMAGE,
            )
            port = docker("port", name, "5000/tcp").stdout.strip().split(":")[-1]
            origin = "http://127.0.0.1:" + port

            def request(path, method="GET", data=None, content_type=None):
                headers = {"Content-Type": content_type} if content_type else {}
                req = urllib.request.Request(
                    urllib.parse.urljoin(origin, path), data=data,
                    method=method, headers=headers,
                )
                try:
                    with urllib.request.urlopen(req, timeout=10) as response:
                        return response.status, response.headers, response.read()
                except urllib.error.HTTPError as error:
                    return error.code, error.headers, error.read()

            deadline = time.monotonic() + 60
            while True:
                try:
                    if request("/v2/")[0] == 200:
                        break
                except (OSError, urllib.error.URLError):
                    pass
                assert time.monotonic() < deadline, "registry did not become ready"
                time.sleep(0.25)

            def blob(data):
                digest = "sha256:" + hashlib.sha256(data).hexdigest()
                code, headers, _ = request(f"/v2/{REPO}/blobs/uploads/", "POST", b"")
                assert code == 202, ("start blob upload", code)
                location = headers["Location"]
                separator = "&" if "?" in location else "?"
                code, _, _ = request(
                    location + separator + urllib.parse.urlencode({"digest": digest}),
                    "PUT", data, "application/octet-stream",
                )
                assert code == 201, ("finish blob upload", code)
                return {"digest": digest, "size": len(data)}

            def build(number, live):
                """Push build <number> as gen-<n>, and as live-<n> when it went live."""
                manifest = json.dumps({
                    "schemaVersion": 2,
                    "mediaType": "application/vnd.oci.image.manifest.v1+json",
                    "config": {**blob(b"{}"), "mediaType": "application/vnd.oci.image.config.v1+json"},
                    "layers": [{
                        **blob(f"build {number}".encode()),
                        "mediaType": "application/vnd.oci.image.layer.v1.tar",
                    }],
                }).encode()
                for tag in [f"gen-{number}"] + ([f"live-{number}"] if live else []):
                    code, _, _ = request(
                        f"/v2/{REPO}/manifests/{tag}", "PUT", manifest,
                        "application/vnd.oci.image.manifest.v1+json",
                    )
                    assert code == 201, ("manifest push", tag, code)
                    time.sleep(1.1)  # distinct push times, as real builds have
                return "sha256:" + hashlib.sha256(manifest).hexdigest()

            digests = {}
            for number in range(1, LIVE_BUILDS + 1):
                digests[number] = build(number, live=True)
            for number in range(LIVE_BUILDS + 1, LIVE_BUILDS + LATER_BUILDS + 1):
                digests[number] = build(number, live=False)

            def stored(number):
                return request(f"/v2/{REPO}/manifests/{digests[number]}")[0] == 200

            want_kept = set(range(LIVE_BUILDS - LIVE["mostRecentlyPushedCount"] + 1, LIVE_BUILDS + 1))
            want_kept |= set(range(
                LIVE_BUILDS + LATER_BUILDS - PUSHED["mostRecentlyPushedCount"] + 1,
                LIVE_BUILDS + LATER_BUILDS + 1,
            ))
            want_gone = set(digests) - want_kept

            # The old order keeps the five newest pushes of any kind.
            legacy_kept = set(range(
                LIVE_BUILDS + LATER_BUILDS - PUSHED["mostRecentlyPushedCount"] + 1,
                LIVE_BUILDS + LATER_BUILDS + 1,
            ))
            settled = legacy_kept if legacy else want_kept

            # Retention has run once nothing it should drop is left; then wait
            # two more GC cycles, in which nothing else may go.
            deadline = time.monotonic() + 150
            while time.monotonic() < deadline:
                kept = {number for number in digests if stored(number)}
                if kept <= settled:
                    break
                time.sleep(3)
            time.sleep(12)
            kept = {number for number in digests if stored(number)}
            order = "catch-all first (legacy)" if legacy else "live- first"
            print(f"{order}: builds stored after retention: {sorted(kept)}")
            if legacy:
                assert kept == legacy_kept, (
                    f"{order}: expected the old order to keep only {sorted(legacy_kept)}, got {sorted(kept)}"
                )
                print(
                    f"REPRODUCED: with the catch-all first, every build that went live "
                    f"({sorted(want_kept - legacy_kept)}) is gone after {LATER_BUILDS} later pushes"
                )
                return
            missing, extra = sorted(want_kept - kept), sorted(kept - want_kept)
            assert not missing, (
                f"{order}: builds {missing} are gone; a service running one of them cannot start a new "
                "instance, and a rollback to it cannot pull"
            )
            assert not extra, f"{order}: builds {extra} should have been removed"
            print(
                f"PASS: the {LIVE['mostRecentlyPushedCount']} newest live- builds and the "
                f"{PUSHED['mostRecentlyPushedCount']} newest other builds are stored; "
                f"{sorted(want_gone)} were removed"
            )
        except Exception:
            logs = docker("logs", "--tail", "15", name, check=False)
            print((logs.stdout + logs.stderr)[-3000:], file=sys.stderr)
            raise
        finally:
            docker("rm", "-f", name, check=False)


if __name__ == "__main__":
    main()
