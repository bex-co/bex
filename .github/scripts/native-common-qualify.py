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
import sys
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


# A checked-in receipt authorizes TEST-ONLY reuse of one already reviewed image.
# It does not perform or replace the retained V11 cryptographic/blob review.
CANDIDATE_PATH = ".github/native-common-test-candidate.json"
CANDIDATE_SCHEMA = "bex.native-common-test-candidate/v1"
V11_TOOLS = {
    "loopx-w8-native-review-v11.py": "ea03d2a9c7a21fde50c0d55f6ead84910c2c431f8c7b52c490624fc5ff8bff85",
    "loopx-w8-verify-native-artifact-ranged.py": "ca4a765d547b92f567f7e08149183a8cb094249d544ae594655c92b93d09ae15",
    "common-v11-adapter.py": "4746a5022ab6df57ed95b6354d56b92eff15976d1737c2aa5a5ca15aa0080a4b",
}
# Checked by the adapter PINNED manifest; not an additional executed policy.
V11_SUPPORT_INPUTS = {
    "loopx-w8-verify-native-artifact.py": "cf5932b2c5598bc9140a4bf9b99ab8c6ae0aa9613e73ec88140f492b086f1bb2",
}
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
    "operator/internal/registry", "types/k8sname", "types/netutil", "types/v1alpha1",
)
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
    "scripts/lib/toolchain-freshness.py", "scripts/install-syft.py",
)
# These are the common publisher's actual local execution inputs, independent
# of the generator/qualification code whose baseline may need requalification.
IMAGE_POLICY_FILES = (
    ".github/workflows/native-base-image.yml", ".github/scripts/native-oci-stage.py",
    ".github/scripts/native-oci-mirror.py", "deploy/native-base/check.Dockerfile",
    "deploy/native-base/check-tools.py", "deploy/native-base/check-brotli.py",
    "deploy/native-base/check-primary-python.py", "deploy/native-base/testdata/archive.dump.b64",
    "deploy/native-base/testdata/rows.txt", "deploy/native-common/matrix.py",
    "deploy/native-common/upstream.json", "scripts/lib/toolchain-freshness.py",
    "lego/operator/internal/build/toolchain-freshness.json",
)
IMAGE_INPUT_PATHS = {"deploy/" + path for path in FACTORY_PATHS} | set(IMAGE_POLICY_FILES)


LEGACY_PUBLICATION_GATES = {
    "verify real native tool transformations without network on both architectures",
    "scan amd64 candidate", "scan arm64 candidate", "sign and attest the immutable candidate",
    "expose the fully gated immutable image",
    "Run docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a",
}
# Historical0941 still must prove every original publication/signature role.
# D3 changes future production architecture scope, never its executed provenance.
PUBLICATION_GATES = LEGACY_PUBLICATION_GATES

# One reviewed compatibility transition, not an arbitrary source-policy waiver.
# Only the exact original0941 proof may cross this exact workflow-byte change;
# the other32 image inputs, all retained proof bodies and source identities stay exact.
D3_LEGACY_0941 = {'ref': 'ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0',
 'producer': '1feba939b4697894e800ed0abb4f25a0bf415641',
 'verification': '1feba939b4697894e800ed0abb4f25a0bf415641',
 'approval_inputs_sha256': '822f75fe0b44129c5b4fefa9ec0f30fd5f6e325aaf3674f6234f24f540a20797',
 'path': '.github/workflows/native-base-image.yml',
 'before': {'sha256': '0571fa44f87342e1c90a57140d25a3022c871c3110643667e3bf1e392cd90e47',
            'bytes': 18402,
            'mode': '100644',
            'type': 'regular'},
 'after': {'sha256': 'fafb81ca5bf80cd0d4193660c0bed68da3b704f64da481c10dc6b1f071b70759',
           'bytes': 15859,
           'mode': '100644',
           'type': 'regular'}}


def image_inputs_compatible(candidate, recorded, current):
    if recorded == current:
        return True
    transition = D3_LEGACY_0941
    path = transition["path"]
    return (set(recorded) == set(current) == IMAGE_INPUT_PATHS
            and all(candidate[key] == transition[key] for key in ("ref", "producer", "verification"))
            and candidate["approval"]["inputs"]["sha256"] == transition["approval_inputs_sha256"]
            and recorded[path] == transition["before"] and current[path] == transition["after"]
            and all(recorded[name] == current[name] for name in IMAGE_INPUT_PATHS - {path}))

UTILITY_GATE = "actual generated offline builds, original replays and exact-default SPDX"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate receipt key")
        result[key] = value
    return result


def json_file(path):
    require(path.stat().st_size <= 4 * 1024 * 1024, "receipt metadata exceeds bound")
    return json.loads(path.read_text(), object_pairs_hook=unique_object)


def checkout_packet(path):
    """One literal packet, never an input-supplied Git pattern or shell word."""
    require(isinstance(path, str) and not re.search(r"[\x00-\x1f\x7f]", path), "invalid checkout evidence path")
    parts = path.split("/")
    require(len(parts) >= 5 and parts[:4] == [".pm", "w8", "m91", "evidence"]
            and all(part not in ("", ".", "..") for part in parts), "checkout evidence must be inside a packet")
    # Gitignore-style non-cone rules: escape literal metacharacters rather than
    # imposing a new packet naming convention on otherwise valid evidence.
    packet = "".join("\\" + char if char in "\\!*?[]# " else char for char in parts[4])
    return "/.pm/w8/m91/evidence/" + packet + "/**"


def checkout_descriptor(descriptor):
    require(isinstance(descriptor, dict) and set(descriptor) == {"path", "sha256", "bytes"}
            and isinstance(descriptor["sha256"], str) and re.fullmatch(r"[a-f0-9]{64}", descriptor["sha256"])
            and type(descriptor["bytes"]) is int and descriptor["bytes"] >= 0, "bad checkout evidence descriptor")
    return checkout_packet(descriptor["path"])


def checkout_evidence_packets(record, custody):
    """Materialization scope only; existing full validators still decide reuse."""
    require(isinstance(record, dict) and set(record) == {"schema", "candidate"}
            and record["schema"] == CANDIDATE_SCHEMA and isinstance(record["candidate"], dict),
            "missing checkout candidate")
    candidate = record["candidate"]
    descriptors = list(candidate["approval"].values()) + [candidate["publication"]["metadata"]]
    if candidate["source"] is not None:
        descriptors.append(candidate["source"]["manifest"])
    packets = {checkout_descriptor(row) for row in descriptors}
    require(candidate["utilities"] is None or isinstance(candidate["utilities"], dict), "invalid checkout utilities")
    for utility in (candidate["utilities"] or {}).values():
        packets.add(checkout_packet(utility["root"]))
        packets.update(checkout_descriptor(utility[key]) for key in ("metadata", "artifact_metadata", "manifest"))
    require(isinstance(custody, dict) and isinstance(custody.get("files"), dict), "missing checkout full custody")
    packets.update(checkout_descriptor(row) for row in custody["files"].values())
    return sorted(packets)


def checkout_evidence(mode):
    require(mode in ("fresh", "reviewed-candidate"), "unknown checkout source")
    if mode == "fresh":
        return  # No prior approval or family baseline is needed to publish fresh.
    root = Path(__file__).resolve().parents[2]
    record = json_file(regular_path(root, CANDIDATE_PATH))
    checkout_evidence_packets(record, {"files": {}})  # Validate every direct path before a Git read.
    descriptor = record["candidate"]["approval"]["custody"]
    checkout_descriptor(descriptor)
    require(descriptor["bytes"] <= 4 * 1024 * 1024, "checkout custody metadata exceeds bound")
    source = os.environ.get("GITHUB_SHA", "")
    require(re.fullmatch(r"[a-f0-9]{40}", source), "invalid checkout source SHA")
    specification = source + ":" + descriptor["path"]

    def git_object(operation, limit, expected_sha256=None):
        # Retain lossless client diagnostics before evidence is hydrated.
        result, stdout, stderr = pull_buildkit(
            ["timeout", "--kill-after=5s", "30s", "git", "-C", str(root), "cat-file", operation, specification],
            dict(os.environ))
        complete = (result["exit_code"] == 0 and result["bound"] is None and result["cancel_signal"] is None
                    and result["direct_child_reaped"] and result["process_group_absent"]
                    and len(stdout) <= limit and len(stderr) <= 1024 * 1024)
        if operation == "-s":
            complete = complete and stdout.strip() == str(descriptor["bytes"]).encode()
        if expected_sha256 is not None:
            complete = complete and len(stdout) == descriptor["bytes"] and hashlib.sha256(stdout).hexdigest() == expected_sha256
        if not complete:
            print("BEX_CANDIDATE_GIT_READ_FAILURE " + json.dumps({
                "source": source, "path": descriptor["path"], "operation": operation, "result": result,
                "stdout_base64": base64.b64encode(stdout).decode("ascii"),
                "stderr_base64": base64.b64encode(stderr).decode("ascii"),
            }), file=sys.stderr, flush=True)
            raise ValueError("bounded checkout custody read failed")
        return stdout

    git_object("-s", 64)
    raw = git_object("blob", descriptor["bytes"], descriptor["sha256"])
    custody = json.loads(raw, object_pairs_hook=unique_object)
    print("\n".join(checkout_evidence_packets(record, custody)))


EVIDENCE_LOCATION = ".github/native-evidence.json"


def evidence_location(root):
    """The one digest-pinned GHCR artifact that holds the retained evidence.

    The evidence left git on 2026-11-04 (#NO-ARTIFACTS-IN-GIT); its file
    titles are the repository paths below .pm/w8/m91/evidence (w6/m164)."""
    location = json_file(regular_path(root, EVIDENCE_LOCATION))
    require(isinstance(location, dict) and set(location) == {"schema", "artifact", "packets"}
            and location["schema"] == "bex.native-evidence-location/v1"
            and isinstance(location["artifact"], str)
            and re.fullmatch(r"ghcr\.io/bex-co/bex-evidence@sha256:[a-f0-9]{64}", location["artifact"])
            and isinstance(location["packets"], list) and location["packets"]
            and all(isinstance(p, str) and checkout_packet(p + "/x") for p in location["packets"]),
            "invalid evidence location")
    return location


def materialize_evidence(mode, root=None):
    """Pull the pinned evidence artifact and place exactly the packets the
    reviewed candidate needs; the full validators still check every byte."""
    require(mode in ("fresh", "reviewed-candidate"), "unknown checkout source")
    if mode == "fresh":
        return  # No prior approval or family baseline is needed to publish fresh.
    root = Path(root) if root is not None else Path(__file__).resolve().parents[2]
    record = json_file(regular_path(root, CANDIDATE_PATH))
    checkout_evidence_packets(record, {"files": {}})  # Validate every direct path first.
    location = evidence_location(root)
    staging = Path(tempfile.mkdtemp(prefix="bex-evidence-", dir=os.environ.get("RUNNER_TEMP")))
    try:
        # --keep-old-files: a title can never replace a file that exists.
        pulled = subprocess.run(["oras", "pull", "--no-tty", "--keep-old-files", location["artifact"], "-o", str(staging)],
                                capture_output=True, text=True, timeout=1100)
        require(pulled.returncode == 0, "evidence artifact pull failed: " + pulled.stderr[-2000:])
        custody = json_file(evidence_file(staging, record["candidate"]["approval"]["custody"]))
        needed = {pattern[len("/"):-len("/**")].replace("\\", "") for pattern in checkout_evidence_packets(record, custody)}
        require(needed <= set(location["packets"]), "the evidence artifact lacks packets: " + ", ".join(sorted(needed - set(location["packets"]))))
        for packet in sorted(needed):
            source, target = staging / packet, root / packet
            require(source.is_dir() and not source.is_symlink(), "evidence artifact lacks " + packet)
            require(not target.exists(), "evidence packet already present: " + packet)
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.move(str(source), str(target))
    finally:
        shutil.rmtree(staging, ignore_errors=True)


def regular_path(root, relative):
    require(isinstance(relative, str) and relative and not relative.startswith("/")
            and all(x not in ("", ".", "..") for x in relative.split("/")), "invalid retained path")
    path = root
    for part in relative.split("/"):
        path /= part
        require(not path.is_symlink(), "retained source/evidence symlink")
    require(path.is_file(), "missing retained source/evidence: " + relative)
    return path


def file_identity(path):
    return {"sha256": sha(path), "bytes": path.stat().st_size,
            "mode": "100755" if path.stat().st_mode & 0o111 else "100644",
            "type": "regular", "permissions": path.stat().st_mode & 0o7777}


def source_inventory(root):
    """Exact functional set; receipt/evidence/PM prose are deliberately excluded."""
    paths = set(SOURCE_FILES) | {"deploy/" + path for path in FACTORY_PATHS}
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


def image_source_inventory(root):
    """Current checked-out image-critical bytes/type/Git mode; no generator gate."""
    inventory = {}
    for path in sorted(IMAGE_INPUT_PATHS):
        observed = file_identity(regular_path(root, path))
        inventory[path] = {key: observed[key] for key in ("sha256", "bytes", "mode", "type")}
    return inventory


def evidence_file(root, descriptor):
    require(isinstance(descriptor, dict) and set(descriptor) == {"path", "sha256", "bytes"}, "bad evidence descriptor")
    require(descriptor["path"].startswith(".pm/w8/m91/evidence/")
            and re.fullmatch(r"[a-f0-9]{64}", descriptor["sha256"] or "")
            and type(descriptor["bytes"]) is int and descriptor["bytes"] >= 0, "bad evidence identity")
    path = regular_path(root, descriptor["path"])
    require(path.stat().st_size == descriptor["bytes"] and sha(path) == descriptor["sha256"], "retained evidence differs")
    return path


def evidence_json(root, descriptor):
    return json_file(evidence_file(root, descriptor))


def completed_job(root, descriptor, identity, source, name, gates):
    metadata = evidence_json(root, descriptor)
    jobs = metadata.get("jobs", [metadata])
    jobs = [row for row in jobs if row.get("id") == identity["job_id"]]
    require(len(jobs) == 1, "missing or ambiguous actual job")
    job = jobs[0]
    require(all(type(identity[key]) is int and identity[key] > 0 for key in ("run_id", "run_attempt", "job_id")), "bad job identity")
    require(job.get("run_id") == identity["run_id"] and job.get("run_attempt") == identity["run_attempt"]
            and job.get("head_sha") == source and job.get("head_branch") == "main"
            and job.get("name") == name and job.get("status") == "completed" and job.get("conclusion") == "success"
            and job.get("run_url") == "https://api.github.com/repos/bex-co/bex/actions/runs/" + str(identity["run_id"]),
            "actual job/source/status differs")
    rows = job.get("steps", [])
    steps = {row["name"]: row for row in rows}
    require(len(steps) == len(rows) and gates <= steps.keys()
            and all(steps[name].get("status") == "completed" and steps[name].get("conclusion") == "success" for name in gates),
            "actual required job gate incomplete")
    return job


def validate_image_candidate(root=None):
    """Existing strict artifact approval; does not authorize a family baseline."""
    root = Path(root) if root is not None else Path(__file__).resolve().parents[2]
    record = json_file(regular_path(root, CANDIDATE_PATH))
    require(set(record) == {"schema", "candidate"} and record["schema"] == CANDIDATE_SCHEMA
            and isinstance(record["candidate"], dict), "no reviewed test candidate authorization")
    require(re.fullmatch(r"[a-f0-9]{40}", os.environ.get("GITHUB_SHA", "")), "invalid executing generator SHA")
    c = record["candidate"]
    require(set(c) == {"ref", "base", "producer", "verification", "publication", "platforms", "approval", "source", "utilities"},
            "unknown or missing test candidate fields")
    require(re.fullmatch(r"ghcr.io/bex-co/bex-cnb-run@sha256:[a-f0-9]{64}", c["ref"])
            and re.fullmatch(r"buildpack-deps:bookworm@sha256:[a-f0-9]{64}", c["base"])
            and re.fullmatch(r"[a-f0-9]{40}", c["producer"])
            and c["producer"] == c["verification"], "invalid immutable common provenance")
    require(set(c["platforms"]) == {"amd64", "arm64"}, "both actual platforms required")
    for platform in c["platforms"].values():
        require(set(platform) == {"manifest_digest", "config_digest"}
                and all(IMAGE.fullmatch(value) for value in platform.values()), "bad common platform identity")
    approval = c["approval"]
    require(set(approval) == {"receipt", "custody", "plan", "inputs"}, "bad retained approval fields")
    review, custody, fixed = (evidence_json(root, approval[key]) for key in ("receipt", "custody", "plan"))
    image_inputs = evidence_json(root, approval["inputs"])
    require(set(image_inputs) == {"source", "files"} and image_inputs["source"] == c["verification"]
            and image_inputs_compatible(c, image_inputs["files"], image_source_inventory(root)), "stale or wrong verifier-source image-critical inputs")
    require(review.get("ref") == fixed.get("ref") == c["ref"] and review.get("official_base") == fixed.get("base") == c["base"]
            and review.get("producer_workflow_sha") == fixed.get("producer_sha") == c["producer"]
            and review.get("verification_workflow_sha") == fixed.get("verification_sha") == c["verification"], "V11 subject/source/base differs")
    require(fixed.get("schema") == "bex.native-common-review-plan/v1" and fixed.get("name") == "common-bookworm", "not a common V11 review")
    factory = {"deploy/" + path: {key: image_inputs["files"]["deploy/" + path][key] for key in ("sha256", "bytes", "mode")} for path in FACTORY_PATHS}
    selection = {path: {key: image_inputs["files"][path][key] for key in ("sha256", "bytes", "mode")}
                 for path in ("deploy/native-common/matrix.py", "deploy/native-common/upstream.json")}
    require(fixed.get("factory_inputs") == factory and fixed.get("selection_inputs") == selection
            and fixed.get("freshness_sha256") == image_inputs["files"]["scripts/lib/toolchain-freshness.py"]["sha256"], "V11 factory/selection policy differs")
    require(all(review.get(key) is True for key in ("approved_supplier_candidate", "anonymous_manifest_bytes_sha_verified",
            "strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified",
            "strict_actual_Rekor_v2_and_RFC3161_TSA_verified", "wrong_source_and_subject_negative_checks_rejected"))
            and review.get("anonymous_pull", {}).get("all_unique_config_and_layer_bytes_fully_hashed") is True,
            "retained strict V11 review incomplete")
    provenance = review.get("producer_verification_provenance", {})
    require(provenance.get("producer_sha") == c["producer"] and provenance.get("verification_sha") == c["verification"]
            and provenance.get("mode") == "fresh-build", "retained producer/verifier relabeled")
    require(set(review.get("platforms", {})) == {"amd64", "arm64"}
            and all(all(review["platforms"][arch].get(key) == value for key, value in c["platforms"][arch].items()) for arch in c["platforms"]),
            "V11 platform identities differ")
    require(isinstance(custody, dict) and set(custody) == {"files", "support_inputs", "image_inputs"}, "missing portable full custody manifest")
    files = custody["files"]
    for descriptor in files.values():
        evidence_file(root, descriptor)
    require(approval["inputs"] in files.values(), "image input manifest has no portable custody")
    retained_image_inputs = custody["image_inputs"]
    require(isinstance(retained_image_inputs, dict) and set(retained_image_inputs) == IMAGE_INPUT_PATHS,
            "image-critical source custody set differs")
    for path, original in retained_image_inputs.items():
        require(isinstance(original, str) and original in files
                and all(files[original][key] == image_inputs["files"][path][key] for key in ("sha256", "bytes")),
                "image-critical source body has no exact portable custody")
    tools = review.get("evidence_tool_sha256", {})
    require(isinstance(tools, dict), "missing executed-tool identities")
    for path, digest in tools.items():
        require(isinstance(path, str) and re.fullmatch(r"[a-f0-9]{64}", digest or "")
                and path in files and files[path]["sha256"] == digest, "executed tool/plan has no exact portable custody")
    for name, digest in V11_TOOLS.items():
        matches = [path for path in tools if Path(path).name == name]
        require(len(matches) == 1 and tools[matches[0]] == digest, "retained V11 tool differs")
    # The actual common adapter reports its fixed plan as an executed input and
    # records the same byte hash in producer_verification_provenance.
    plan_entries = [path for path, digest in tools.items()
                    if digest == approval["plan"]["sha256"] and files[path] == approval["plan"]]
    require(len(plan_entries) == 1 and provenance.get("common_review_plan_sha256") == approval["plan"]["sha256"],
            "executed fixed review plan linkage differs")
    support = custody["support_inputs"]
    require(isinstance(support, dict) and set(support) == set(V11_SUPPORT_INPUTS), "review support manifest differs")
    for name, digest in V11_SUPPORT_INPUTS.items():
        path = support[name]
        require(isinstance(path, str) and Path(path).name == name and path in files
                and files[path]["sha256"] == digest and path not in tools,
                "review support identity differs or is mislabeled as executed")
    payloads = review.get("signed_payloads", [])
    required_pairs = {("image-signature", None), ("sbom", "amd64"), ("sbom", "arm64"),
                      ("vulnerability", "amd64"), ("vulnerability", "arm64")}
    require(isinstance(payloads, list) and all(isinstance(row, dict) for row in payloads), "invalid signed payload records")
    pairs = {(row.get("kind"), row.get("architecture")) for row in payloads}
    require(required_pairs <= pairs and pairs <= required_pairs,
            "complete supported signed image/SBOM/Trivy set required")
    for row in payloads:
        for key in ("full_bundle_gzip", "full_predicate_gzip"):
            require(row.get(key) in files and files[row[key]]["sha256"] == row.get(key + "_sha256"), "full signed report custody missing")
    pub = c["publication"]
    require(set(pub) == {"run_id", "run_attempt", "job_id", "metadata"}, "bad publication identity")
    ci = review.get("ci_runtime_scan_signing_evidence", {})
    require(str(pub["run_id"]) == ci.get("run_id") and pub["job_id"] == ci.get("job_id")
            and fixed.get("run_id") == pub["run_id"] and fixed.get("job_id") == pub["job_id"]
            and ci.get("job_name", "").startswith("publish / build (common-bookworm, ")
            and ci.get("job_conclusion") == "success" and LEGACY_PUBLICATION_GATES - {next(x for x in LEGACY_PUBLICATION_GATES if x.startswith("Run "))} <= set(ci.get("verified_steps", [])),
            "V11 actual publication provenance differs")
    publication_job = completed_job(root, pub["metadata"], pub, c["verification"], ci["job_name"], LEGACY_PUBLICATION_GATES)
    recovery = [step for step in publication_job["steps"] if step["name"] == "select a reviewed recovery artifact when its recipe is unchanged"]
    require(ci.get("recovery_selection_conclusion") in (None, "skipped")
            and all(step.get("conclusion") == "skipped" for step in recovery), "common recovery is unsupported")
    return dict(c, receipt_sha256=sha(root / CANDIDATE_PATH), generator_source=os.environ["GITHUB_SHA"],
                baseline_status="image-only-requalification")


def validate_test_candidate(root=None):
    """D3 family reuse needs the complete current AMD64 utility artifact."""
    root = Path(root) if root is not None else Path(__file__).resolve().parents[2]
    c = validate_image_candidate(root)
    baseline = c["source"]
    require(isinstance(baseline, dict) and set(baseline) == {"sha", "manifest"} and re.fullmatch(r"[a-f0-9]{40}", baseline["sha"]), "family reuse requires a complete utility baseline source")
    expected = evidence_json(root, baseline["manifest"])
    require(set(expected) == {"source", "files"} and expected["source"] == baseline["sha"] and expected["files"] == source_inventory(root), "stale qualification source set/bytes/modes")
    require(isinstance(c["utilities"], dict) and set(c["utilities"]) == {"amd64"}, "complete original AMD64 utility artifact required")
    for arch, utility in c["utilities"].items():
        require(set(utility) == {"run_id", "run_attempt", "job_id", "artifact_id", "artifact_metadata", "metadata", "root", "manifest"}
                and type(utility["artifact_id"]) is int and utility["artifact_id"] > 0, "bad utility artifact identity")
        completed_job(root, utility["metadata"], utility, baseline["sha"], "qualify (" + arch + ")", {UTILITY_GATE})
        archive = evidence_json(root, utility["artifact_metadata"])
        require(archive.get("id") == utility["artifact_id"]
                and archive.get("name") == "common-generated-" + baseline["sha"] + "-" + arch + "-" + str(utility["run_attempt"])
                and archive.get("workflow_run", {}).get("id") == utility["run_id"]
                and archive.get("workflow_run", {}).get("head_sha") == baseline["sha"], "actual utility artifact provenance differs")
        prefix = utility["root"] + "/"
        require(prefix.startswith(".pm/w8/m91/evidence/") and utility["manifest"]["path"] == prefix + "MANIFEST.json", "utility artifact scope differs")
        artifact = evidence_json(root, utility["manifest"])
        entries = artifact.get("files", [])
        retained = {row["path"]: row for row in entries}
        require(len(retained) == len(entries) and {"inputs.json", "qualification-source.json", "qualification.log", "cleanup.json"} <= retained.keys(), "incomplete original utility artifacts")
        for path, row in retained.items():
            evidence_file(root, {"path": prefix + path, "sha256": row["sha256"], "bytes": row["bytes"]})
        captured_source = retained["qualification-source.json"]
        require(captured_source["sha256"] == baseline["manifest"]["sha256"]
                and captured_source["bytes"] == baseline["manifest"]["bytes"]
                and json_file(regular_path(root, prefix + "qualification-source.json")) == expected,
                "utility job did not capture this exact baseline source")
        inputs = json_file(regular_path(root, prefix + "inputs.json"))
        require(inputs.get("common_image") == c["ref"] and inputs.get("producer") == c["producer"]
                and inputs.get("verification") == c["verification"] and inputs.get("generator_source") == baseline["sha"]
                and inputs.get("platform") == "linux/" + arch and inputs.get("run_id") == str(utility["run_id"])
                and inputs.get("run_attempt") == str(utility["run_attempt"])
                and inputs.get("selection") == profile_selection("utilities", "all"), "utility inputs/selector/source differ")
        def proof(suffix):
            paths = [path for path in retained if path.endswith("/" + suffix)]
            require(len(paths) == 1, "missing or ambiguous original utility proof: " + suffix)
            return paths[0], json_file(regular_path(root, prefix + paths[0]))

        default_config = None
        for case in ("node-compute", "node-static", "python313-original", "python3148-default"):
            _, actual = proof(case + "/receipt.json")
            require(actual.get("common_base") == c["ref"] and actual.get("image_platform") == "linux/" + arch
                    and IMAGE.fullmatch(actual.get("image_config_id", "")) and actual.get("network") == "none"
                    and actual.get("runtime_command_executed") is (case != "node-static")
                    and actual.get("static_artifacts_extracted") is (case == "node-static"), "actual generated utility identity differs")
            if case == "python3148-default":
                default_config = actual["image_config_id"]
        for name in ("jq", "rsync", "python-metadata", "python-pip", "python-headers"):
            _, omission = proof("missing-" + name + "/omission-receipt.json")
            require(omission.get("removed_supplier") == name and omission.get("common_base") == c["ref"]
                    and omission.get("healthy_independent_prerequisites_observed") is True
                    and omission.get("original_command_expected_failure_observed") is True, "original omission control did not pass")
        originals = json_file(regular_path(root, "lego/operator/internal/build/testdata/native-utilities/original-cases.json"))
        require(len(originals) == 13 and len({row["mid"] for row in originals}) == 13, "original13 source set differs")
        for case in originals:
            name = "node-compute" if case["runtime"] == "node" else "python313-original"
            mid = str(case["mid"])
            require(any(path.endswith("/" + name + "/qa-output/build-" + mid + ".log") for path in retained), "original authored build log absent")
            _, responses = proof(name + "/qa-output/runtime-" + mid + "-runtime.json")
            require(isinstance(responses, list) and len(responses) == 3 and all(row.get("pass") is True for row in responses), "original runtime HTTP proof incomplete")
        path, binding = proof("python3148-default/generated-spdx-binding.json")
        require(binding.get("platform") == "linux/" + arch and binding.get("python") == "3.14.8"
                and binding.get("image_config_id") == default_config and IMAGE.fullmatch(binding.get("syft_manifest_digest", ""))
                and binding.get("libbrotli1_versions"), "actual exact-default SPDX binding differs")
        for kind in ("syft", "spdx"):
            reports = binding.get("custody", {}).get(kind, {})
            require(reports.get("roundtrip") is True, "actual complete report custody absent")
            for form in ("raw", "gzip"):
                report_path = str(Path(path).parent / reports.get(form + "_file", ""))
                require(report_path in retained and retained[report_path]["sha256"] == reports.get(form + "_sha256")
                        and retained[report_path]["bytes"] == reports.get(form + "_bytes"), "full generated report custody differs")
    return dict(c, baseline_status="current-complete")


def plan():
    event = os.environ["GITHUB_EVENT_NAME"]
    profile = os.environ.get("REQUESTED_PROFILE", "utilities")
    case = os.environ.get("REQUESTED_CASE", "pilot")
    if event not in ("push", "workflow_dispatch") or (event == "push" and profile != "utilities"):
        raise ValueError("extra profiles require an explicit manual dispatch")
    selection = profile_selection(profile, case)
    mode = os.environ.get("REQUESTED_SOURCE", "fresh")
    require(mode in ("fresh", "reviewed-candidate"), "unknown qualification image source")
    if mode == "reviewed-candidate":
        require(event == "workflow_dispatch" and os.environ.get("GITHUB_REF") == "refs/heads/main", "candidate reuse requires explicit manual main qualification")
        candidate = validate_image_candidate() if profile == "utilities" else validate_test_candidate()
        selection.update(ref=candidate["ref"], producer=candidate["producer"], verification=candidate["verification"],
                         receipt_sha256=candidate["receipt_sha256"], baseline_status=candidate["baseline_status"])
    selection.setdefault("baseline_status", "fresh-execution")
    selection["source"] = mode
    # Validate everything before opening any output or creating any resources.
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        for key in ("profile", "case", "flag", "selector", "go_timeout", "job_minutes", "source", "ref", "producer", "verification", "receipt_sha256", "baseline_status"):
            output.write(key + "=" + selection.get(key, "") + "\n")


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
    mode = os.environ.get("QUALIFICATION_SOURCE", "fresh")
    reuse = None
    if mode == "reviewed-candidate":
        require(os.environ.get("GITHUB_EVENT_NAME") == "workflow_dispatch" and os.environ.get("GITHUB_REF") == "refs/heads/main", "reuse requires explicit manual main qualification")
        reuse = validate_image_candidate() if selection["profile"] == "utilities" else validate_test_candidate()
        require((ref, producer, verification, os.environ.get("CANDIDATE_RECEIPT_SHA256")) ==
                (reuse["ref"], reuse["producer"], reuse["verification"], reuse["receipt_sha256"]), "reuse env differs from reviewed receipt")
    require(mode in ("fresh", "reviewed-candidate"), "unknown qualification image source")
    if (not re.fullmatch(r"ghcr.io/bex-co/bex-cnb-run@sha256:[a-f0-9]{64}", ref)
            or not all(re.fullmatch(r"[a-f0-9]{40}", x) for x in (producer, verification))
            or (reuse is None and verification != os.environ["GITHUB_SHA"]) or producer != verification
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
                               "image_source": mode, "candidate_receipt_sha256": reuse["receipt_sha256"] if reuse else None,
                               "baseline_status": reuse["baseline_status"] if reuse else "fresh-execution",
                               "image_approval_inputs_sha256": reuse["approval"]["inputs"]["sha256"] if reuse else None,
                               "utility_baseline_source": reuse["source"]["sha"] if reuse and selection["profile"] != "utilities" else None,
                               "utility_jobs": reuse["utilities"] if reuse and selection["profile"] != "utilities" else None,
                               "previous_baseline_state": ("absent" if reuse["source"] is None or reuse["utilities"] is None else "unvalidated-existing") if reuse and selection["profile"] == "utilities" else None,
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
    policy_path = Path(__file__).resolve().parents[2] / "lego/operator/internal/build/toolchain-freshness.json"
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
               "policy_path": "lego/operator/internal/build/toolchain-freshness.json",
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
    commands.add_parser("checkout-evidence").add_argument("source", choices=("fresh", "reviewed-candidate"))
    commands.add_parser("materialize-evidence").add_argument("source", choices=("fresh", "reviewed-candidate"))
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
    elif args.command == "checkout-evidence":
        checkout_evidence(args.source)
    elif args.command == "materialize-evidence":
        materialize_evidence(args.source)
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
