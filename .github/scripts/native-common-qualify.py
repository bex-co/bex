"""Own only ephemeral common-image qualification resources and evidence."""
import argparse
import base64
import datetime
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import signal
import shutil
import subprocess
import tempfile
import time
import urllib.parse
import uuid


LABEL = "co.bex.native-qualification"
OWNER = re.compile(r"bex-nq-[a-f0-9]{24}")
IMAGE = re.compile(r"sha256:[a-f0-9]{64}")
BOOTSTRAP_OUTPUT_LIMIT = 4 * 1024 * 1024
BOOTSTRAP_WALL_SECONDS = 570
# GNU kill-after10 + direct wait1 + group-absence wait5.
BOOTSTRAP_CLEANUP_RESERVE_SECONDS = 16


def dump(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def sha(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()



# These are existing Go subtests, not an arbitrary regex/timeout surface. Each
# manual run measures one AMD64 case after the full original AMD64
# cohort. Bounds come from existing command contexts, not measured capacity.
PROFILES = {
    # 30m wheel export +45m build +30m runtime contexts, with15m overhead.
    # This one-route limit is not an actual footprint or duration estimate.
    "node-addon": ("TestLegacyNodeOfflineAddons", "BEX_NATIVE_TEST_NODE_ADDON_PROFILE",
                   ("common-node14-primary", "common-node16-primary", "common-python313-node16",
                    "historical-python313-node16"), 120, 150),
    "node": ("TestCommonNodeGeneratedImages", "BEX_NATIVE_TEST_NODE_PROFILE",
             ("node-14", "node-16", "node-18", "node-20", "node-22", "node-26",
              "node-exact14212", "ruby-node-exact14212"), 90, 120),
    "node-donors": ("TestCommonNodeGeneratedImages", "BEX_NATIVE_TEST_NODE_DONORS",
                    ("node-14", "node-16", "node-18", "node-20",
                     "node-exact14213", "ruby-node-exact14213",
                     "node-exact14212", "ruby-node-exact14212"), 90, 120),
    "modern-python": ("TestNativeModernPythonGeneratedImages", "BEX_NATIVE_TEST_MODERN_PYTHON_PROFILE",
                      ("3.9", "3.10", "3.11", "3.12", "3.14", "3.15"), 110, 140),
    "legacy-python": ("TestNativeLegacyPythonGeneratedImages", "BEX_NATIVE_LEGACY_PYTHON_INTEGRATION",
                      ("3.7", "3.8"), 110, 140),
    "ruby": ("TestCommonRubyGeneratedImages", "BEX_NATIVE_TEST_RUBY_PROFILE",
             ("ruby30", "ruby31", "ruby32", "ruby33", "ruby34", "ruby40",
              "ruby344-creation-default", "ruby3413-creation-default", "ruby320-locked-bullseye",
              "node-static-ruby34"), 75, 105),
    "go-rust": ("TestNativeGoRustGeneratedImages", "BEX_NATIVE_TEST_GORUST_PROFILE",
                ("go", "rust"), 75, 105),
    "elixir": ("TestCommonElixirOfflineQualification", "BEX_NATIVE_ELIXIR_INTEGRATION",
               ("elixir-1.18", "elixir-1.17", "elixir-1.19", "elixir-1.20",
                "elixir-exact-otp280"), 90, 120),
    "native-init": ("TestCommonNativeInitQualification", "BEX_NATIVE_TEST_INIT_PROFILE",
                    ("all-launches",), 60, 90),
}


# One dispatch runs every case below, as a matrix, on the image that run
# published: one digest is then shown to pass all of them (w8/m91). Production
# Node 14/16/18/20 run on the published donor trees, so those lines go through
# node-donors; node keeps only its lines that are not backports. A historical
# route builds on the official images without the common base, so it says
# nothing about the digest; it stays a single case.
FAMILIES = "families"


def family_cases():
    donors = set(PROFILES["node-donors"][2])
    return [(profile, case) for profile, (_, _, cases, _, _) in PROFILES.items()
            for case in cases if not (profile == "node" and case in donors) and not case.startswith("historical-")]


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


FACTORY_PATHS = (
    "native-base/bex-native-python-tools", "native-base/fetch-python-tools.py",
    "native-base/install-python-tools.py", "native-base/install.sh", "native-base/node-brotli/check.js",
    "native-base/node-brotli/patch.sh", "native-base/package-lock.json", "native-base/package.json",
    "native-base/patch-system-brotli.sh", "native-base/python-tools.json", "native-base/tsc", "native-base/webpack",
    "native-common/Dockerfile", "native-common/Dockerfile.dockerignore", "native-common/build-otp",
    "native-common/check-common.py", "native-common/legacy-libs.preferences", "native-common/legacy-libs.sources",
    "native-common/otp-features.py", "native-common/prepare-node14-tools.py",
)
SOURCE_PACKAGES = (
    "operator/cmd/native-init", "operator/internal/execution", "operator/internal/execution/executiontest",
    "operator/internal/httpclient", "operator/internal/identity", "operator/internal/publish",
    "operator/internal/registry", "types/deploybudgets", "types/k8sname", "types/netutil", "types/v1alpha1",
)
# Inside the recorded build tree, and also named: bootstrap reads its BuildKit pin.
BUILDKIT_POLICY = "lego/operator/internal/build/toolchain-freshness.json"
SOURCE_FILES = (
    "lego/operator/go.mod", "lego/operator/go.sum", "lego/types/go.mod", "lego/types/go.sum",
    "lego/.dockerignore", "lego/Dockerfile", "deploy/native-base/testdata/archive.dump.b64",
    "deploy/native-base/testdata/archive.sql", "deploy/native-base/testdata/rows.txt",
    "deploy/native-common/check-runtime.py", "deploy/native-base/check-primary-python.py",
    "deploy/native-base/check-brotli.py", "deploy/native-base/check.Dockerfile", "deploy/native-base/check-tools.py",
    ".github/workflows/native-base-image.yml", "deploy/native-common/check-local.Dockerfile",
    ".github/workflows/native-common-image.yml", ".github/scripts/native-common-qualify.py",
    ".github/scripts/test_native_oci_common_qualification.py", ".github/scripts/native-oci-stage.py",
    ".github/scripts/native-oci-mirror.py", "deploy/native-common/matrix.py", "deploy/native-common/upstream.json",
    "scripts/lib/toolchain-freshness.py", "scripts/install-syft.py", BUILDKIT_POLICY,
)
FIXED_SOURCE_PATHS = frozenset(SOURCE_FILES) | {"deploy/" + path for path in FACTORY_PATHS}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def regular_path(root, relative):
    require(isinstance(relative, str) and relative and not relative.startswith("/")
            and all(x not in ("", ".", "..") for x in relative.split("/")), "invalid source path")
    path = root
    for part in relative.split("/"):
        path /= part
        require(not path.is_symlink(), "qualification source symlink")
    require(path.is_file(), "missing qualification source: " + relative)
    return path


def file_identity(path):
    return {"sha256": sha(path), "bytes": path.stat().st_size,
            "mode": "100755" if path.stat().st_mode & 0o111 else "100644",
            "type": "regular", "permissions": path.stat().st_mode & 0o7777}


def source_inventory(root):
    """The exact functional source set qualification-source.json records; PM prose is excluded."""
    paths = set(FIXED_SOURCE_PATHS)
    build = root / "lego/operator/internal/build"
    require(build.is_dir() and not build.is_symlink(), "missing build source root")
    paths.update(str(p.relative_to(root)) for p in build.rglob("*") if not p.is_dir() or p.is_symlink())
    for package in SOURCE_PACKAGES:
        directory = root / "lego" / package
        require(directory.is_dir() and not directory.is_symlink(), "missing local source package")
        files = list(directory.glob("*.go"))
        require(files, "empty local source package")
        paths.update(str(p.relative_to(root)) for p in files)
    return {path: file_identity(regular_path(root, path)) for path in sorted(paths)}


def plan():
    event = os.environ["GITHUB_EVENT_NAME"]
    profile = os.environ.get("REQUESTED_PROFILE", "utilities")
    case = os.environ.get("REQUESTED_CASE", "pilot")
    if event not in ("push", "workflow_dispatch") or (event == "push" and profile != "utilities"):
        raise ValueError("extra profiles require an explicit manual dispatch")
    if profile == FAMILIES:
        require(case in ("pilot", "all"), "families always runs every family case")
        case, legs = "all", family_cases()
    else:
        selection = profile_selection(profile, case)
        case = selection["case"]
        legs = [] if profile == "utilities" else [(profile, case)]
    # Validate everything before opening any output or creating any resources.
    matrix = [{"profile": leg_profile, "case": leg_case,
               "job_minutes": int(profile_selection(leg_profile, leg_case)["job_minutes"])}
              for leg_profile, leg_case in legs]
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        output.write("profile=" + profile + "\ncase=" + case + "\nmatrix=" + json.dumps(matrix, separators=(",", ":")) + "\n")


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
            or platform != "linux/amd64"):
        raise ValueError("qualification requires this source's fully gated immutable common image")
    captured_source = {"source": os.environ["GITHUB_SHA"],
                       "files": source_inventory(Path(__file__).resolve().parents[2])}
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
    dump(root / "qualification-source.json", captured_source)
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


def bounded_pull_output(process, deadline, cancellation, limit):
    """Preserve every captured byte, with at most limit+1 bytes in both streams."""
    buffers = {"stdout": bytearray(), "stderr": bytearray()}
    total, bound, complete = 0, None, False
    try:
        with selectors.DefaultSelector() as ready:
            for label in ("stdout", "stderr"):
                stream = getattr(process, label)
                os.set_blocking(stream.fileno(), False)
                ready.register(stream, selectors.EVENT_READ, label)
            while ready.get_map() or process.poll() is None:
                if cancellation["signal"] is not None:
                    bound = "cancelled"
                    break
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    bound = "python-timeout"
                    break
                if not ready.get_map():
                    time.sleep(min(.05, remaining))
                    continue
                for key, _ in ready.select(min(1, remaining)):
                    try:
                        body = os.read(key.fd, min(64 * 1024, limit + 1 - total))
                    except BlockingIOError:
                        continue
                    if not body:
                        ready.unregister(key.fileobj)
                        continue
                    buffers[key.data].extend(body)
                    total += len(body)
                    if total > limit:
                        bound = "output-byte-limit"
                        break
                if bound is not None:
                    break
            complete = bound is None and not ready.get_map()
    except BaseException as error:
        # Keep the already captured prefix even on a read/selector error.
        bound = "output-read-" + type(error).__name__
    return bytes(buffers["stdout"]), bytes(buffers["stderr"]), bound, complete


def pull_buildkit(command, environment, cancellation=None, output_limit=None, deadline=None):
    """Bound this pre-pull; signals only flag cancellation until its group is gone."""
    cancellation = cancellation if cancellation is not None else {"signal": None}
    if output_limit is not None and (type(output_limit) is not int or output_limit <= 0):
        raise ValueError("positive internal pull output limit required")
    capture_complete = False
    previous = {}
    process = None
    bound = None
    stdout, stderr = b"", b""
    reaped, group_absent = True, True

    def cancel(signum, frame):
        if cancellation["signal"] is None:
            cancellation["signal"] = signum

    try:
        for signum in (signal.SIGINT, signal.SIGTERM):
            previous[signum] = signal.getsignal(signum)
            signal.signal(signum, cancel)
        if cancellation["signal"] is None:
            # A nonraising handler cannot interrupt Popen's PID registration.
            process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                       env=environment, start_new_session=True)
            if deadline is None:
                deadline = time.monotonic() + 195
            if output_limit is not None:
                stdout, stderr, bound, capture_complete = bounded_pull_output(
                    process, deadline, cancellation, output_limit)
            else:
                while cancellation["signal"] is None:
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        bound = "python-timeout"
                        break
                    try:
                        stdout, stderr = process.communicate(timeout=min(1, remaining))
                        break
                    except subprocess.TimeoutExpired as error:
                        stdout, stderr = error.output or stdout, error.stderr or stderr
    finally:
        try:
            if process is not None:
                if output_limit is not None:
                    # A stopped capture does not drain unbounded unread bytes.
                    # All bytes already read remain in stdout/stderr and receipts.
                    for stream in (process.stdout, process.stderr):
                        try:
                            stream.close()
                        except OSError as error:
                            bound = bound or "output-close-" + type(error).__name__
                try:
                    os.killpg(process.pid, 0)
                    alive = True
                except ProcessLookupError:
                    alive = False
                except PermissionError:
                    alive = True
                if alive:
                    bound = bound or "remaining-process-group"
                    try:
                        os.killpg(process.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    except PermissionError:
                        bound = "process-group-permission"
                if output_limit is None and (bound is not None or process.returncode is None):
                    try:
                        stdout, stderr = process.communicate(timeout=5)
                    except subprocess.TimeoutExpired as error:
                        bound = "incomplete-output-after-group-stop"
                        stdout, stderr = error.output or stdout, error.stderr or stderr
                        process.stdout.close()
                        process.stderr.close()
                try:
                    process.wait(timeout=1)
                    reaped = True
                except subprocess.TimeoutExpired:
                    reaped = False
                deadline = time.monotonic() + 5
                while True:
                    try:
                        os.killpg(process.pid, 0)
                    except ProcessLookupError:
                        group_absent = True
                        break
                    except PermissionError:
                        pass
                    if time.monotonic() >= deadline:
                        group_absent = False
                        break
                    time.sleep(0.05)
        finally:
            restoration_errors = []
            for signum, handler in reversed(list(previous.items())):
                try:
                    signal.signal(signum, handler)
                except (OSError, ValueError, RuntimeError) as error:
                    restoration_errors.append(type(error).__name__)
            if restoration_errors:
                raise RuntimeError("BuildKit pull signal-handler restoration failed")
    if cancellation["signal"] is not None:
        bound = "cancelled"
    result = {"exit_code": process.returncode if process is not None else None, "bound": bound,
              "cancel_signal": cancellation["signal"],
              "direct_child_reaped": reaped, "process_group_absent": group_absent}
    if output_limit is not None:
        result.update(output_limit_bytes=output_limit, captured_output_bytes=len(stdout) + len(stderr),
                      output_capture_complete=capture_complete)
    return result, stdout, stderr


def bootstrap(root):
    """Pre-pull the committed production BuildKit before bounded builder setup."""
    if root.is_symlink() or not root.is_dir() or root.stat().st_uid != os.getuid() or root.stat().st_mode & 0o077:
        raise ValueError("bootstrap evidence ownership differs from qualification")
    inputs_path = root / "inputs.json"
    if inputs_path.is_symlink() or not inputs_path.is_file():
        raise ValueError("bootstrap requires initialized qualification inputs")
    inputs = json.loads(inputs_path.read_text())
    owner = inputs.get("owner")
    if not isinstance(owner, str) or not OWNER.fullmatch(owner):
        raise ValueError("bootstrap requires an exact qualification owner")
    docker_config = owned_docker_config(root, inputs)
    if docker_config is None or not docker_config.is_dir():
        raise ValueError("bootstrap requires the existing owned Docker configuration")
    policy_path = Path(__file__).resolve().parents[2] / BUILDKIT_POLICY
    policy_bytes = policy_path.read_bytes()
    entries = [row for row in json.loads(policy_bytes)["images"] if row.get("id") == "helper-buildkit"]
    if len(entries) != 1 or entries[0].get("kind") != "helper":
        raise ValueError("missing or ambiguous production BuildKit pin")
    image = entries[0].get("committed", "")
    if not isinstance(image, str) or not re.fullmatch(r"moby/buildkit:v[0-9]+\.[0-9]+\.[0-9]+@sha256:[a-f0-9]{64}", image):
        raise ValueError("production BuildKit pin must be an immutable moby image")
    output_path = Path(os.environ["GITHUB_OUTPUT"])
    began = time.monotonic()
    deadline = began + BOOTSTRAP_WALL_SECONDS
    command = None  # Actual first argv is recorded only when enough budget remains.
    receipt = {"owner": owner, "image": image,
               "policy_path": BUILDKIT_POLICY,
               "policy_sha256": hashlib.sha256(policy_bytes).hexdigest(),
               "command": command, "maximum_attempts": 3, "retry_delay_seconds": 15,
               "whole_timeout_seconds": BOOTSTRAP_WALL_SECONDS,
               "cleanup_reserve_seconds": BOOTSTRAP_CLEANUP_RESERVE_SECONDS,
               "started_monotonic": began, "deadline_monotonic": deadline,
               "whole_budget_stop": None, "output_drain_seconds": 0,
               "output_drain_policy": "EOF under shared deadline minus direct/group cleanup6s; no unbounded drain",
               "direct_child_wait_seconds": 1, "group_absence_wait_seconds": 5,
               "attempts": [], "image_output_emitted": False,
               "combined_output_limit_bytes": BOOTSTRAP_OUTPUT_LIMIT,
               "output_limit_scope": "all bootstrap attempts combined; at most limit+1 captured bytes",
               "incomplete_capture_means": "all captured bytes retained; unread bytes after stop are not captured",
               "captured_output_bytes": 0, "output_budget_exhausted": False,
               "scope": "bounded setup input only; no generated-runtime or acceptance claim"}
    receipt_path = root / "bootstrap.json"
    if receipt_path.exists() or receipt_path.is_symlink():
        raise ValueError("refusing to overwrite an earlier bootstrap receipt")
    dump(receipt_path, receipt)
    environment = dict(os.environ, DOCKER_CONFIG=str(docker_config))
    cancellation = {"signal": None}
    previous = {}
    succeeded = False

    def cancel(signum, frame):
        if cancellation["signal"] is None:
            cancellation["signal"] = signum

    try:
        for signum in (signal.SIGINT, signal.SIGTERM):
            previous[signum] = signal.getsignal(signum)
            signal.signal(signum, cancel)
        for number in range(1, 4):
            if cancellation["signal"] is not None:
                break
            remaining = deadline - time.monotonic()
            command_seconds = int(remaining - BOOTSTRAP_CLEANUP_RESERVE_SECONDS)
            if command_seconds <= 0:
                receipt["whole_budget_stop"] = "insufficient-cleanup-reserved-pull-budget"
                break
            command = ["timeout", "--kill-after=10s", str(command_seconds) + "s", "docker", "pull", image]
            if receipt["command"] is None:
                receipt["command"] = command
            remaining_output = BOOTSTRAP_OUTPUT_LIMIT - receipt["captured_output_bytes"]
            if remaining_output <= 0:
                raise RuntimeError("BuildKit pull output budget exhausted; no retry or image output")
            started = datetime.datetime.now(datetime.timezone.utc).isoformat()
            try:
                result, stdout, stderr = pull_buildkit(
                    command, environment, cancellation, output_limit=remaining_output,
                    deadline=deadline - 6)  # Preserve1s directwait +5s group proof inside whole budget.
                attempt = {"number": number, "started_at": started,
                           "finished_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                           **result, "timeout_exit": result["exit_code"] in (124, 137)}
            except OSError as error:
                attempt = {"number": number, "started_at": started,
                           "finished_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                           "exit_code": None, "launch_error": type(error).__name__, "timeout_exit": False}
                # No subprocess stream exists on launch failure; keep its class above.
                stdout, stderr = b"", b""
            attempt.update(command=command, command_timeout_seconds=command_seconds,
                           whole_remaining_before_seconds=remaining,
                           python_deadline_monotonic=deadline - 6,
                           whole_elapsed_after_seconds=time.monotonic() - began)
            for label, raw in (("stdout", stdout), ("stderr", stderr)):
                path = root / ("bootstrap-" + str(number) + "." + label + ".log")
                path.write_bytes(raw)
                attempt[label] = {"path": path.name, "bytes": len(raw), "sha256": sha(path)}
            attempt["cancel_signal"] = cancellation["signal"]
            receipt["attempts"].append(attempt)
            receipt["captured_output_bytes"] += len(stdout) + len(stderr)
            receipt["output_budget_exhausted"] = receipt["captured_output_bytes"] >= BOOTSTRAP_OUTPUT_LIMIT
            dump(receipt_path, receipt)
            # Keep every captured bounded-command byte if artifact upload fails.
            # Never emit the environment or the isolated Docker config contents.
            print("BEX_BUILDKIT_PULL_ATTEMPT " + json.dumps({
                "owner": owner, "image": image, "command": command,
                "policy_sha256": receipt["policy_sha256"], "attempt": attempt,
                "stdout_base64": base64.b64encode(stdout).decode("ascii"),
                "stderr_base64": base64.b64encode(stderr).decode("ascii"),
            }), flush=True)
            if cancellation["signal"] is not None:
                break
            if time.monotonic() >= deadline:
                receipt["whole_budget_stop"] = "whole-bootstrap-deadline"
                break
            if (attempt.get("bound") or "").startswith("output-"):
                raise RuntimeError("bounded BuildKit pull output capture stopped; no retry or image output")
            if (type(attempt["exit_code"]) is int and attempt["exit_code"] == 0
                    and attempt.get("bound") is None and attempt.get("direct_child_reaped") is True
                    and attempt.get("process_group_absent") is True):
                succeeded = True
                break
            if attempt.get("process_group_absent") is False or attempt.get("direct_child_reaped") is False:
                raise RuntimeError("bounded BuildKit pull cleanup incomplete; exact result retained")
            if receipt["output_budget_exhausted"]:
                raise RuntimeError("BuildKit pull output budget exhausted; no retry or image output")
            if number < 3:
                # Never spend the cleanup reserve or reset the shared deadline on a retry.
                if deadline - time.monotonic() <= 15 + BOOTSTRAP_CLEANUP_RESERVE_SECONDS:
                    receipt["whole_budget_stop"] = "insufficient-budget-for-retry-delay-and-pull"
                    break
                for _ in range(15):
                    if cancellation["signal"] is not None:
                        break
                    if deadline - time.monotonic() <= BOOTSTRAP_CLEANUP_RESERVE_SECONDS:
                        receipt["whole_budget_stop"] = "shared-budget-during-retry-delay"
                        break
                    time.sleep(1)
    finally:
        restoration_errors = []
        for signum, handler in reversed(list(previous.items())):
            try:
                signal.signal(signum, handler)
            except (OSError, ValueError, RuntimeError) as error:
                restoration_errors.append(type(error).__name__)
        receipt["elapsed_seconds"] = time.monotonic() - began
        if time.monotonic() >= deadline:
            receipt["whole_budget_stop"] = "whole-bootstrap-deadline"
        dump(receipt_path, receipt)
        if cancellation["signal"] is not None:
            receipt["cancel_signal"] = cancellation["signal"]
            if receipt["attempts"]:
                receipt["attempts"][-1]["cancel_signal"] = cancellation["signal"]
            dump(receipt_path, receipt)
        if restoration_errors:
            raise RuntimeError("BuildKit bootstrap signal-handler restoration failed")
    if cancellation["signal"] is not None:
        raise RuntimeError("BuildKit bootstrap cancelled; exact result retained without retry or image output")
    if time.monotonic() >= deadline:
        receipt["whole_budget_stop"] = "whole-bootstrap-deadline"
        dump(receipt_path, receipt)
    if receipt["whole_budget_stop"] is not None:
        raise RuntimeError("shared BuildKit bootstrap budget exhausted; exact results retained")
    if not succeeded:
        raise RuntimeError("all three bounded production BuildKit pulls failed; exact results retained")
    with output_path.open("a") as output:
        output.write("image=" + image + "\n")
    receipt["image_output_emitted"] = True
    dump(receipt_path, receipt)


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
        probes = []
        # The fixed workflow has not invoked setup-buildx in this one branch.
        # Reuse the reviewed docker-container driver's exact node/state names;
        # an error or an unobserved resource is never an absence proof.
        if os.environ.get("QUALIFICATION_BUILDX_SETUP_OUTCOME") == "skipped":
            container = "buildx_buildkit_" + owner + "0"
            for kind, name, missing in (
                    ("container", container, (
                        "Error response from daemon: No such container: " + container,
                        "Error: No such container: " + container,
                        "Error: No such object: " + container)),
                    ("volume", container + "_state", (
                        "Error response from daemon: get " + container + "_state: no such volume",
                        "Error response from daemon: No such volume: " + container + "_state"))):
                command = [kind, "inspect", "--format", "{{json .Name}}", name]
                result = run(command, 10)
                absent = (type(result.returncode) is int and result.returncode == 1
                          and isinstance(result.stdout, str) and result.stdout.strip() in ("", "[]")
                          and isinstance(result.stderr, str) and result.stderr.strip() in missing)
                probes.append({"resource": kind, "identity": name, "command": ["docker", *command],
                               "exit_code": result.returncode, "stdout": result.stdout,
                               "stderr": result.stderr, "absence_proved": absent})
        if len(probes) == 2 and all(row["absence_proved"] for row in probes):
            results.append({"resource": "builder", "identity": owner, "exit_code": 0,
                            "stdout": "", "stderr": "", "removal": "not-invoked",
                            "state": "daemon-confirmed-absent-before-setup", "state_probes": probes})
        else:
            result = run(["buildx", "rm", owner], 60)
            removal = {"resource": "builder", "identity": owner, "exit_code": result.returncode,
                       "stdout": result.stdout, "stderr": result.stderr}
            if probes:
                removal["state_probes"] = probes
            results.append(removal)
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
        # These fixed owned builder/list/remove results contain no credentials.
        # Logout remains quiet; expose only its status, never auth paths/content.
        print("BEX_NATIVE_CLEANUP_RESULT " + json.dumps({
            "owner": owner,
            "operations": [
                {key: value for key, value in row.items()
                 if row["resource"] != "docker-auth" or key in
                 ("resource", "exit_code", "logout_exit_code", "removed", "removal_error")}
                for row in results
            ],
        }), flush=True)
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
    commands.add_parser("source-paths", help="print the fixed files of the qualification source inventory, one per line (scripts/ship-gates.sh)")
    for name in ("bootstrap", "cleanup", "manifest"):
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
    elif args.command == "source-paths":
        print("\n".join(sorted(FIXED_SOURCE_PATHS)))
    elif args.command == "init":
        initialize()
    elif args.command == "bootstrap":
        bootstrap(args.root)
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
