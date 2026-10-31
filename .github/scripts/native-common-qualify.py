"""Own only ephemeral common-image qualification resources and evidence."""
import argparse
import datetime
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
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



# These are existing Go subtests, not an arbitrary regex/timeout surface. Each
# manual run measures one case on both architectures after the full original
# cohort. Bounds come from existing command contexts, not measured capacity.
PROFILES = {
    "node": ("TestCommonNodeGeneratedImages", "BEX_NATIVE_TEST_NODE_PROFILE",
             ("node-14", "node-16", "node-18", "node-20", "node-22", "node-26",
              "node-exact14212", "ruby-node-exact14212"), 90, 120),
    "modern-python": ("TestNativeModernPythonGeneratedImages", "BEX_NATIVE_TEST_MODERN_PYTHON_PROFILE",
                      ("3.9", "3.10", "3.11", "3.12", "3.14"), 110, 140),
    "legacy-python": ("TestNativeLegacyPythonGeneratedImages", "BEX_NATIVE_LEGACY_PYTHON_INTEGRATION",
                      ("3.7", "3.8"), 110, 140),
    "ruby": ("TestCommonRubyGeneratedImages", "BEX_NATIVE_TEST_RUBY_PROFILE",
             ("ruby30", "ruby31", "ruby32", "ruby33", "ruby34", "ruby40",
              "ruby344-creation-default", "node-static-ruby34"), 75, 105),
    "go-rust": ("TestNativeGoRustGeneratedImages", "BEX_NATIVE_TEST_GORUST_PROFILE",
                ("go", "rust"), 75, 105),
    "elixir": ("TestCommonElixirOfflineQualification", "BEX_NATIVE_ELIXIR_INTEGRATION",
               ("elixir-1.18", "elixir-1.17", "elixir-1.19", "elixir-1.20",
                "elixir-exact-otp280"), 90, 120),
    "native-init": ("TestCommonNativeInitQualification", "BEX_NATIVE_TEST_INIT_PROFILE",
                    ("all-launches",), 60, 90),
}


def profile_selection(profile, case):
    """Resolve one fixed case, with the original full utility gate unchanged."""
    if profile == "utilities":
        if case not in ("pilot", "all"):
            raise ValueError("utilities always requires its complete original cohort")
        return {"profile": profile, "case": "all", "flag": "",
                "selector": "^(TestNativeUtilityGeneratedImages|TestNativeOriginalReplayHTTPOracles)$",
                "go_timeout": "150m", "job_minutes": "180",
                "scope": "complete original utility cohort; no hosted or adoption claim"}
    if profile not in PROFILES:
        raise ValueError("unknown common qualification profile")
    test, flag, cases, minutes, job_minutes = PROFILES[profile]
    selected = cases[0] if case == "pilot" else case
    if selected not in cases:
        raise ValueError("unknown case for the selected common qualification profile")
    selector = "^" + test + "$"
    if profile != "native-init":
        selector += "/^" + re.escape(selected) + "$"
    return {"profile": profile, "case": selected, "flag": flag, "selector": selector,
            "go_timeout": str(minutes) + "m", "job_minutes": str(job_minutes),
            "scope": "single-case pilot measurement only; not full-family acceptance or a capacity guarantee"}


def plan():
    event = os.environ["GITHUB_EVENT_NAME"]
    profile = os.environ.get("REQUESTED_PROFILE", "utilities")
    case = os.environ.get("REQUESTED_CASE", "pilot")
    if event not in ("push", "workflow_dispatch") or (event == "push" and profile != "utilities"):
        raise ValueError("extra profiles require an explicit manual dispatch")
    selection = profile_selection(profile, case)
    # Validate everything before opening any output or creating any resources.
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        for key in ("profile", "case", "flag", "selector", "go_timeout", "job_minutes"):
            output.write(key + "=" + selection[key] + "\n")


def profile_environment():
    selection = profile_selection(os.environ["QUALIFICATION_PROFILE"], os.environ["QUALIFICATION_CASE"])
    if selection["profile"] == "utilities":
        raise ValueError("manual profile environment cannot replace the original utility step")
    environment = {entry[1]: "0" for entry in PROFILES.values()}
    environment[selection["flag"]] = "1"
    environment.update(BEX_BUILDKIT_INTEGRATION="1", QUALIFICATION_PROFILE=selection["profile"],
                       QUALIFICATION_CASE=selection["case"], QUALIFICATION_SELECTOR=selection["selector"],
                       QUALIFICATION_GO_TIMEOUT=selection["go_timeout"])
    with Path(os.environ["GITHUB_ENV"]).open("a") as output:
        for key, value in environment.items():
            output.write(key + "=" + value + "\n")


def initialize():
    selection = profile_selection(os.environ.get("QUALIFICATION_PROFILE", "utilities"),
                                  os.environ.get("QUALIFICATION_CASE", "all"))
    ref = os.environ["COMMON_IMAGE"]
    producer, verification = os.environ["PRODUCER_SHA"], os.environ["VERIFICATION_SHA"]
    platform = os.environ["TARGET_PLATFORM"]
    if (not re.fullmatch(r"ghcr.io/bex-co/bex-cnb-run@sha256:[a-f0-9]{64}", ref)
            or not all(re.fullmatch(r"[a-f0-9]{40}", x) for x in (producer, verification))
            or verification != os.environ["GITHUB_SHA"] or producer != verification
            or platform not in ("linux/amd64", "linux/arm64")):
        raise ValueError("qualification requires this source's fully gated immutable common image")
    root = Path(tempfile.mkdtemp(prefix="bex-native-qualification-", dir=os.environ["RUNNER_TEMP"]))
    root = root.resolve(strict=True)
    owner = "bex-nq-" + uuid.uuid4().hex[:24]
    # A sibling is deliberately outside the only uploaded evidence tree.
    docker_config = root.parent / (owner + "-docker-config")
    dump(root / "inputs.json", {"owner": owner, "common_image": ref, "producer": producer,
                               "verification": verification, "platform": platform,
                               "generator_source": os.environ["GITHUB_SHA"], "selection": selection,
                               "docker_config": str(docker_config),
                               "run_id": os.environ["GITHUB_RUN_ID"],
                               "run_attempt": os.environ["GITHUB_RUN_ATTEMPT"],
                               "scope": "ephemeral generated-image qualification; no catalog adoption or hosted claim"})
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        output.write("root=" + str(root) + "\nowner=" + owner + "\n")
    # Record the exact scope before creating it or allowing any login. If a
    # later initialization operation fails, the always cleanup has its owner.
    docker_config.mkdir(mode=0o700)
    try:
        dump(docker_config / ".qualification-owner.json", {"owner": owner, "evidence_root": str(root)})
    except BaseException:
        shutil.rmtree(docker_config)
        raise
    with Path(os.environ["GITHUB_ENV"]).open("a") as environment:
        environment.write("DOCKER_CONFIG=" + str(docker_config) + "\n")
    capacity(root, "before-builder")


def memory_capacity(meminfo, maximum, current):
    """Visible kernel/cgroup observations, not an allocator reservation."""
    match = re.search(r"(?m)^MemAvailable:\s+(\d+)\s+kB\s*$", meminfo)
    host = int(match[1]) * 1024 if match else None
    limit = int(maximum) if maximum and maximum != "max" else None
    used = int(current) if current else None
    if any(value is not None and value < 0 for value in (host, limit, used)):
        raise ValueError("negative kernel capacity observation")
    remaining = max(0, limit - used) if limit is not None and used is not None else None
    observed = [value for value in (host, remaining) if value is not None]
    return {"host_mem_available_bytes": host, "visible_cgroup_limit_bytes": limit,
            "visible_cgroup_current_bytes": used, "visible_cgroup_remaining_bytes": remaining,
            "observed_headroom_hint_bytes": min(observed) if observed else None,
            "scope": "observational hint only; reclaimable pagecache, ancestor limits and future allocations are not modeled or reserved"}


def capacity(root, stage):
    owner = json.loads((root / "inputs.json").read_text())["owner"]
    if not OWNER.fullmatch(owner) or stage not in ("before-builder", "after-tests"):
        raise ValueError("capacity observation requires an exact qualification owner and phase")
    paths = {"evidence": root, "workspace": Path.cwd(), "temporary": Path(tempfile.gettempdir())}
    filesystems = []
    for name, path in paths.items():
        path = path.resolve(strict=True)
        usage = shutil.disk_usage(path)
        filesystems.append({"name": name, "path": str(path), "device": path.stat().st_dev,
                            "total_bytes": usage.total, "free_bytes": usage.free})

    def read(path):
        try:
            return Path(path).read_text().strip()
        except FileNotFoundError:
            return ""

    memory = memory_capacity(read("/proc/meminfo"), read("/sys/fs/cgroup/memory.max"),
                             read("/sys/fs/cgroup/memory.current"))
    dump(root / ("capacity-" + stage + ".json"), {
        "at": datetime.datetime.now(datetime.timezone.utc).isoformat(), "owner": owner,
        "stage": stage, "filesystems": filesystems, "memory": memory,
        "daemon_filesystem_and_builder_cache": "not observed from the runner filesystem namespace",
        "scope": "read-only observations; shared device rows are not additive; no threshold, reservation or expanded-family capacity acceptance"})


def owned_docker_config(root, inputs):
    """Resolve only the exact initialized credential scope, never ambient config."""
    if "docker_config" not in inputs:  # Receipts from before scoped login.
        return None
    root = root.resolve(strict=True)
    expected = root.parent / (inputs["owner"] + "-docker-config")
    if inputs["docker_config"] != str(expected) or expected.is_symlink():
        raise ValueError("refusing foreign or symlinked Docker configuration")
    if not expected.exists():
        return expected
    marker = expected / ".qualification-owner.json"
    if (not expected.is_dir() or expected.stat().st_uid != os.getuid()
            or expected.stat().st_mode & 0o077 or marker.is_symlink()
            or json.loads(marker.read_text()) != {"owner": inputs["owner"], "evidence_root": str(root)}):
        raise ValueError("Docker configuration ownership differs from qualification")
    return expected


def cleanup(root):
    inputs = json.loads((root / "inputs.json").read_text())
    owner = inputs["owner"]
    if not OWNER.fullmatch(owner):
        raise ValueError("refusing cleanup without an exact qualification owner")
    docker_config = owned_docker_config(root, inputs)
    if docker_config is not None and not docker_config.exists():
        # An initialization failure may have removed its incomplete scope.
        # Recreate only that recorded path so Docker cannot fall back to the
        # runner's config or leave an unmarked config during cleanup.
        docker_config.mkdir(mode=0o700)
        try:
            dump(docker_config / ".qualification-owner.json", {"owner": owner, "evidence_root": str(root.resolve(strict=True))})
        except BaseException:
            shutil.rmtree(docker_config)
            raise
    environment = dict(os.environ, DOCKER_CONFIG=str(docker_config)) if docker_config is not None else dict(os.environ)

    def run(args, timeout, quiet=False):
        try:
            output = {"stdout": subprocess.DEVNULL, "stderr": subprocess.DEVNULL} if quiet else {"capture_output": True}
            return subprocess.run(["docker", *args], **output, text=True, timeout=timeout, env=environment)
        except (subprocess.TimeoutExpired, OSError) as error:
            return subprocess.CompletedProcess(["docker", *args], 124, "", type(error).__name__)

    results = []
    try:
        # Stop this job's builder before enumerating its output images, so an
        # interrupted client cannot load another image after our final listing.
        result = run(["buildx", "rm", owner], 60)
        results.append({"resource": "builder", "identity": owner, "exit_code": result.returncode,
                        "stdout": result.stdout, "stderr": result.stderr})
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
    finally:
        if docker_config is not None:
            # Do not retain logout output or config contents in evidence. Even
            # a timeout must not prevent removal of this exact credential tree.
            logout = run(["logout", "ghcr.io"], 20, quiet=True)
            auth = {"resource": "docker-auth", "identity": str(docker_config),
                    "logout_exit_code": logout.returncode, "exit_code": logout.returncode, "removed": False}
            try:
                owned_docker_config(root, inputs)
                if docker_config.exists():
                    shutil.rmtree(docker_config)
                auth["removed"] = True
            except (OSError, ValueError) as error:
                auth.update(exit_code=1, removal_error=type(error).__name__)
            results.append(auth)
        # No default builder selection, wildcard removal, foreign cache,
        # shared certificate deletion or global prune.
        dump(root / "cleanup.json", {"owner": owner, "operations": results})
    if any(row["exit_code"] for row in results):
        raise RuntimeError("owned resource cleanup failed; exact results retained")


def bind_spdx(document, inventory, image, platform):
    source = inventory.get("source", {})
    metadata = source.get("metadata", {})
    if source.get("type") != "image" or metadata.get("imageID") != image:
        raise ValueError("Syft source differs from actual generated Docker config")
    if (metadata.get("os"), metadata.get("architecture")) != tuple(platform.split("/")):
        raise ValueError("Syft source platform differs from actual generated image")
    manifest = metadata.get("manifestDigest", "")
    if not IMAGE.fullmatch(manifest):
        raise ValueError("Syft source has no actual image manifest digest")
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
    if identities != {manifest}:
        raise ValueError("SPDX root differs from the same scan's Syft manifest digest")
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
    return {"image_config_id": image, "syft_manifest_digest": manifest, "platform": platform, "python": "3.14.8",
            "libbrotli1_versions": sorted({x["versionInfo"] for x in brotli}),
            "packages": len(packages)}


def retain_report(report):
    compressed = report.with_suffix(report.suffix + ".gz")
    with report.open("rb") as source, gzip.open(compressed, "wb") as destination:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            destination.write(chunk)
    with gzip.open(compressed, "rb") as restored:
        digest = hashlib.sha256()
        for chunk in iter(lambda: restored.read(1024 * 1024), b""):
            digest.update(chunk)
    if digest.hexdigest() != sha(report):
        raise ValueError("full SBOM report gzip roundtrip failed")
    return {"raw_file": report.name, "raw_bytes": report.stat().st_size, "raw_sha256": sha(report),
            "gzip_file": compressed.name, "gzip_bytes": compressed.stat().st_size,
            "gzip_sha256": sha(compressed), "roundtrip": True}


def sbom(args):
    if not IMAGE.fullmatch(args.image) or args.platform not in ("linux/amd64", "linux/arm64"):
        raise ValueError("invalid actual generated image identity/platform")
    root = Path(args.evidence)
    report = root / "generated.spdx.json"
    inventory = root / "generated.syft.json"
    with tempfile.TemporaryDirectory(prefix="syft-tmp-", dir=root) as temporary, (root / "syft.stderr.log").open("wb") as stderr:
        env = dict(os.environ, TMPDIR=temporary, SYFT_CHECK_FOR_APP_UPDATE="false", GODEBUG="http2client=0")
        subprocess.run([args.syft, "scan", "docker:" + args.image, "--platform", args.platform,
                        "-o", "syft-json=" + str(inventory), "-o", "spdx-json=" + str(report)],
                       env=env, stderr=stderr, check=True, timeout=1800)
    # Two formats from one scan: Docker config/platform come from Syft JSON,
    # while the SPDX document root names its manifest/source digest.
    # Preserve both complete original reports even on a binding failure.
    custody = {"syft": retain_report(inventory), "spdx": retain_report(report)}
    dump(root / "generated-sbom-custody.json", custody)
    binding = bind_spdx(json.loads(report.read_text()), json.loads(inventory.read_text()), args.image, args.platform)
    for phase in ("build", "runtime"):
        proof = json.loads((root / "qa-output" / ("common-default-" + phase + ".json")).read_text())
        if proof.get("python") != "3.14.8" or proof.get("python_env") != "3.14.8" or proof.get("pass") is not True:
            raise ValueError("SPDX has no matching real exact-default build/runtime proof")
    dump(root / "generated-spdx-binding.json", dict(binding, custody=custody, scope="actual ephemeral exact-default image; no hosted/adoption claim"))


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
    commands.add_parser("plan")
    commands.add_parser("profile-env")
    for name in ("cleanup", "manifest"):
        commands.add_parser(name).add_argument("root", type=Path)
    observation = commands.add_parser("capacity")
    observation.add_argument("root", type=Path)
    observation.add_argument("stage", choices=("before-builder", "after-tests"))
    scan = commands.add_parser("sbom")
    for name in ("syft", "image", "platform", "evidence"):
        scan.add_argument("--" + name, required=True)
    args = parser.parse_args()
    if args.command == "plan":
        plan()
    elif args.command == "profile-env":
        profile_environment()
    elif args.command == "init":
        initialize()
    elif args.command == "cleanup":
        cleanup(args.root)
    elif args.command == "manifest":
        manifest(args.root)
    elif args.command == "capacity":
        capacity(args.root, args.stage)
    else:
        sbom(args)


if __name__ == "__main__":
    main()
