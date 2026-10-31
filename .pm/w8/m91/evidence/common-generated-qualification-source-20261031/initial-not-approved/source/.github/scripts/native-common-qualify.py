"""Own only ephemeral common-image qualification resources and evidence."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import urllib.parse
import uuid


LABEL = "co.bex.native-qualification"
OWNER = re.compile(r"bex-nq-[a-f0-9]{24}")
IMAGE = re.compile(r"sha256:[a-f0-9]{64}")


def dump(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def sha(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def initialize():
    ref = os.environ["COMMON_IMAGE"]
    producer, verification = os.environ["PRODUCER_SHA"], os.environ["VERIFICATION_SHA"]
    platform = os.environ["TARGET_PLATFORM"]
    if (not re.fullmatch(r"ghcr.io/bex-co/bex-cnb-run@sha256:[a-f0-9]{64}", ref)
            or not all(re.fullmatch(r"[a-f0-9]{40}", x) for x in (producer, verification))
            or verification != os.environ["GITHUB_SHA"] or producer != verification
            or platform not in ("linux/amd64", "linux/arm64")):
        raise ValueError("qualification requires this source's fully gated immutable common image")
    root = Path(tempfile.mkdtemp(prefix="bex-native-qualification-", dir=os.environ["RUNNER_TEMP"]))
    owner = "bex-nq-" + uuid.uuid4().hex[:24]
    dump(root / "inputs.json", {"owner": owner, "common_image": ref, "producer": producer,
                               "verification": verification, "platform": platform,
                               "run_id": os.environ["GITHUB_RUN_ID"],
                               "run_attempt": os.environ["GITHUB_RUN_ATTEMPT"],
                               "scope": "ephemeral generated-image qualification; no catalog adoption or hosted claim"})
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        output.write("root=" + str(root) + "\nowner=" + owner + "\n")


def cleanup(root):
    owner = json.loads((root / "inputs.json").read_text())["owner"]
    if not OWNER.fullmatch(owner):
        raise ValueError("refusing cleanup without an exact qualification owner")
    def run(args, timeout):
        try:
            return subprocess.run(["docker", *args], capture_output=True, text=True, timeout=timeout)
        except (subprocess.TimeoutExpired, OSError) as error:
            return subprocess.CompletedProcess(["docker", *args], 124, "", type(error).__name__)

    # Stop this job's builder before enumerating its output images, so an
    # interrupted client cannot load another image after our final listing.
    result = run(["buildx", "rm", owner], 60)
    results = [{"resource": "builder", "identity": owner, "exit_code": result.returncode,
                "stdout": result.stdout, "stderr": result.stderr}]
    for resource, listing, removal in (
            ("container", ["ps", "-aq"], ["rm", "-f"]),
            ("image", ["image", "ls", "-q"], ["image", "rm", "-f"])):
        selected = run([*listing, "--filter", "label=" + LABEL + "=" + owner], 20)
        if selected.returncode:
            results.append({"resource": resource + "-listing", "exit_code": selected.returncode,
                            "stdout": selected.stdout, "stderr": selected.stderr})
            continue
        for identity in sorted(set(selected.stdout.split())):
            if not re.fullmatch(r"(?:sha256:)?[a-f0-9]{12,64}", identity):
                raise ValueError("unexpected owned Docker identity")
            result = run([*removal, identity], 20)
            results.append({"resource": resource, "identity": identity, "exit_code": result.returncode,
                            "stdout": result.stdout, "stderr": result.stderr})
    # No default builder selection, wildcard removal, foreign cache,
    # certificate deletion or global prune.
    dump(root / "cleanup.json", {"owner": owner, "operations": results})
    if any(row["exit_code"] for row in results):
        raise RuntimeError("owned resource cleanup failed; exact results retained")


def bind_spdx(document, image, platform):
    if not document.get("spdxVersion", "").startswith("SPDX-"):
        raise ValueError("not a real SPDX document")
    packages = document["packages"]
    roots = set(document.get("documentDescribes", []))
    roots.update(x["relatedSpdxElement"] for x in document.get("relationships", [])
                 if x.get("spdxElementId") == document.get("SPDXID") and x.get("relationshipType") == "DESCRIBES")
    described = [x for x in packages if x.get("SPDXID") in roots]
    if len(described) != 1:
        raise ValueError("SPDX must describe one generated image")
    identities = {"sha256:" + x["checksumValue"] for x in described[0].get("checksums", []) if x.get("algorithm") == "SHA256"}
    if identities != {image}:
        raise ValueError("SPDX root differs from actual generated image config")
    arches = set()
    for package in packages:
        for external in package.get("externalRefs", []):
            locator = external.get("referenceLocator", "")
            if locator.startswith("pkg:deb/"):
                arches.update(x for x in urllib.parse.parse_qs(urllib.parse.urlsplit(locator).query).get("arch", []) if x != "all")
    if arches != {platform.split("/")[1]}:
        raise ValueError("SPDX package architecture differs from actual generated image")
    python = [x for x in packages if x.get("name") == "python" and x.get("versionInfo") == "3.14.8"]
    brotli = [x for x in packages if x.get("name") == "libbrotli1"]
    if not python or not brotli:
        raise ValueError("Syft did not identify required exact Python3.14.8 and system libbrotli1; no synthetic package substitution")
    for package in brotli:
        subprocess.run(["dpkg", "--compare-versions", package["versionInfo"], "ge", "1.0.9-2+deb12u1"], check=True)
    return {"image_config_id": image, "platform": platform, "python": "3.14.8",
            "libbrotli1_versions": sorted({x["versionInfo"] for x in brotli}),
            "packages": len(packages)}


def sbom(args):
    if not IMAGE.fullmatch(args.image) or args.platform not in ("linux/amd64", "linux/arm64"):
        raise ValueError("invalid actual generated image identity/platform")
    root = Path(args.evidence)
    report = root / "generated.spdx.json"
    with tempfile.TemporaryDirectory(prefix="syft-tmp-", dir=root) as temporary, (root / "syft.stderr.log").open("wb") as stderr:
        env = dict(os.environ, TMPDIR=temporary, SYFT_CHECK_FOR_APP_UPDATE="false", GODEBUG="http2client=0")
        subprocess.run([args.syft, "scan", "docker:" + args.image, "--platform", args.platform,
                        "-o", "spdx-json=" + str(report)], env=env, stderr=stderr, check=True, timeout=1800)
    # Preserve the complete report, including a binding/cataloger failure.
    compressed = root / "generated.spdx.json.gz"
    with report.open("rb") as source, gzip.open(compressed, "wb") as destination:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            destination.write(chunk)
    with gzip.open(compressed, "rb") as restored:
        digest = hashlib.sha256()
        for chunk in iter(lambda: restored.read(1024 * 1024), b""):
            digest.update(chunk)
    if digest.hexdigest() != sha(report):
        raise ValueError("full SPDX gzip roundtrip failed")
    custody = {"raw_bytes": report.stat().st_size, "raw_sha256": sha(report),
               "gzip_bytes": compressed.stat().st_size, "gzip_sha256": sha(compressed), "roundtrip": True}
    dump(root / "generated-spdx-custody.json", custody)
    binding = bind_spdx(json.loads(report.read_text()), args.image, args.platform)
    for phase in ("build", "runtime"):
        proof = json.loads((root / "qa-output" / ("common-default-" + phase + ".json")).read_text())
        if proof.get("python") != "3.14.8" or proof.get("python_env") != "3.14.8" or proof.get("pass") is not True:
            raise ValueError("SPDX has no matching real exact-default build/runtime proof")
    dump(root / "generated-spdx-binding.json", dict(binding, custody=custody, scope="actual ephemeral exact-default image; no hosted/adoption claim"))
    # Full lossless report retained above; remove only this own raw duplicate.
    report.unlink()


def manifest(root):
    files = []
    for path in sorted(root.rglob("*")):
        if path.is_symlink():
            raise ValueError("qualification artifacts must not contain symlinks")
        if path.is_file() and path.name != "MANIFEST.json":
            files.append({"path": str(path.relative_to(root)), "bytes": path.stat().st_size, "sha256": sha(path)})
    dump(root / "MANIFEST.json", {"files": files, "scope": "actual retained evidence, including failures; not an acceptance assertion"})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("init")
    for name in ("cleanup", "manifest"):
        commands.add_parser(name).add_argument("root", type=Path)
    scan = commands.add_parser("sbom")
    for name in ("syft", "image", "platform", "evidence"):
        scan.add_argument("--" + name, required=True)
    args = parser.parse_args()
    if args.command == "init":
        initialize()
    elif args.command == "cleanup":
        cleanup(args.root)
    elif args.command == "manifest":
        manifest(args.root)
    else:
        sbom(args)


if __name__ == "__main__":
    main()
