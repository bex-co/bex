#!/usr/bin/env python3
"""Private common-only input adapter; all V11 cryptographic/report/blob gates remain inherited."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import shlex
import shutil
import subprocess
from types import SimpleNamespace

SCHEMA = "bex.native-common-review-plan/v1"
GUARD_BYTES = (1 << 30) + (128 << 20) + (2 << 20)
MAX_SOURCE_BYTES = 256 << 10
PINNED = {
    "loopx-w8-native-review-v11.py": "ea03d2a9c7a21fde50c0d55f6ead84910c2c431f8c7b52c490624fc5ff8bff85",
    "loopx-w8-verify-native-artifact-ranged.py": "ca4a765d547b92f567f7e08149183a8cb094249d544ae594655c92b93d09ae15",
    "loopx-w8-verify-native-artifact.py": "cf5932b2c5598bc9140a4bf9b99ab8c6ae0aa9613e73ec88140f492b086f1bb2",
}
COMMON_DOCKERFILE = "deploy/native-common/Dockerfile"
COMMON_IGNORE = "deploy/native-common/Dockerfile.dockerignore"
MATRIX = "deploy/native-common/matrix.py"
UPSTREAM = "deploy/native-common/upstream.json"
BUILD_STEP = "Run docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a"
ORIGINAL_JOB_GATES = {
    "verify real native tool transformations without network on both architectures",
    "scan amd64 candidate", "scan arm64 candidate", "sign and attest the immutable candidate",
}
COMMON_OUTPUT_GATE = "expose the fully gated immutable image"


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def physical_guard(path):
    free = shutil.disk_usage(path).free
    require(free >= GUARD_BYTES, "STOP_WRITES: amended source guard failed: " + str(free))
    return free


def write_json(path, value):
    raw = (json.dumps(value, indent=2, ensure_ascii=False) + "\n").encode()
    physical_guard(path.parent)
    path.write_bytes(raw)


def import_pinned(path, expected):
    require(digest(path.read_bytes()) == expected, "retained V11 source SHA differs: " + path.name)
    spec = importlib.util.spec_from_file_location("common_" + path.stem.replace("-", "_"), path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class GitSource:
    """Only local immutable Git objects; never fetch or execute source commands."""
    def __init__(self, root):
        self.root = Path(root).resolve()

    def __call__(self, revision, path):
        require(re.fullmatch(r"[0-9a-f]{40}", revision), "immutable source SHA required")
        require(path and not path.startswith("/") and ".." not in Path(path).parts, "invalid source path")
        result = subprocess.run(["git", "ls-tree", revision, "--", path], cwd=self.root,
                                capture_output=True, check=True, timeout=20)
        rows = result.stdout.decode().splitlines()
        require(len(rows) == 1, "required source input missing: " + path)
        metadata, observed = rows[0].split("\t", 1)
        mode, kind, blob = metadata.split()
        require(observed == path and kind == "blob" and mode in ("100644", "100755"),
                "required regular source input/mode differs: " + path)
        raw = subprocess.run(["git", "show", revision + ":" + path], cwd=self.root,
                             capture_output=True, check=True, timeout=20).stdout
        require(len(raw) <= MAX_SOURCE_BYTES, "source input exceeds private read bound")
        return raw, mode


def identity(raw, mode):
    return {"sha256": digest(raw), "bytes": len(raw), "mode": mode}


def checked_identity(value):
    require(isinstance(value, dict) and set(value) == {"sha256", "bytes", "mode"}, "bad fixed input identity")
    require(re.fullmatch(r"[0-9a-f]{64}", value["sha256"]) and type(value["bytes"]) is int
            and 0 <= value["bytes"] <= MAX_SOURCE_BYTES and value["mode"] in ("100644", "100755"),
            "bad fixed input digest/size/mode")


def validate_plan(plan, executing=False):
    require(plan.get("schema") == SCHEMA and plan.get("name") == "common-bookworm", "common-only fixed plan required")
    for role in ("producer", "verification", "generator"):
        require(re.fullmatch(r"[0-9a-f]{40}", plan.get(role + "_sha", "")), "missing immutable " + role + " source")
    # Current common publisher has no recovery route. Preserve V11's fresh-build semantics.
    require(plan["producer_sha"] == plan["verification_sha"], "common recovery is unsupported; do not relabel producer")
    require(re.fullmatch(r"buildpack-deps:bookworm@sha256:[0-9a-f]{64}", plan.get("base", "")), "exact common base required")
    inputs = plan.get("factory_inputs", {})
    require(isinstance(inputs, dict) and len(inputs) == 20, "fixed current common closure requires all20 inputs")
    for value in inputs.values():
        checked_identity(value)
    require(set(plan.get("selection_inputs", {})) == {MATRIX, UPSTREAM}, "fixed common selection inputs missing")
    for value in plan["selection_inputs"].values():
        checked_identity(value)
    require(re.fullmatch(r"[0-9a-f]{64}", plan.get("freshness_sha256", "")), "fixed local freshness helper SHA missing")
    if executing:
        require(type(plan.get("run_id")) is int and plan["run_id"] > 0 and type(plan.get("job_id")) is int
                and plan["job_id"] > 0, "actual completed run/job identities required")
        require(isinstance(plan.get("job_name"), str) and plan["job_name"].startswith("publish / build (common-bookworm, "),
                "exact reusable common job name required")
        require(re.fullmatch(r"ghcr.io/bex-co/bex-cnb-run@sha256:[0-9a-f]{64}", plan.get("ref") or ""),
                "actual fully gated canonical REF required")


def local_copy_inputs(dockerfile):
    paths = {COMMON_DOCKERFILE, COMMON_IGNORE}
    for line in dockerfile.decode().splitlines():
        if not line.startswith("COPY "):
            continue
        require(not line.endswith("\\"), "review multiline common COPY before adapting")
        tokens = shlex.split(line[5:])
        if any(token.startswith("--from=") for token in tokens):
            continue
        values = [token for token in tokens if not token.startswith("--")]
        require(len(values) >= 2, "invalid common COPY")
        for item in values[:-1]:
            require(not item.startswith("/") and ".." not in Path(item).parts and not any(c in item for c in "*?[$"),
                    "review nonliteral common COPY before adapting")
            paths.add("deploy/" + item)
    return paths


def common_matrix_base(reader, revision, expected_selection):
    matrix, mode = reader(revision, MATRIX)
    require(identity(matrix, mode) == expected_selection[MATRIX], "common matrix source differs from fixed plan")
    dockerfile, _ = reader(revision, COMMON_DOCKERFILE)
    # Same source-driven matrix contract as V11; only the reviewed common matrix is executed.
    namespace = {"__name__": "__reviewed_common_matrix__"}
    exec(compile(matrix, "reviewed-common-" + revision, "exec"), namespace)
    rows = namespace["matrix"](dockerfile.decode())["include"]
    require(len(rows) == 1 and rows[0].get("name") == "common-bookworm", "common matrix must select exactly one common row")
    return rows[0]["base"]


def source_equivalence(plan, reader):
    validate_plan(plan)
    roles = {}
    for role in ("producer", "verification", "generator"):
        revision = plan[role + "_sha"]
        dockerfile, _ = reader(revision, COMMON_DOCKERFILE)
        actual_paths = local_copy_inputs(dockerfile)
        require(actual_paths == set(plan["factory_inputs"]), role + " common COPY closure differs or omitted helper")
        observed = {}
        for path in sorted(actual_paths):
            observed[path] = identity(*reader(revision, path))
            require(observed[path] == plan["factory_inputs"][path], role + " common input bytes/mode stale: " + path)
        for path, expected in plan["selection_inputs"].items():
            require(identity(*reader(revision, path)) == expected, role + " selection metadata stale: " + path)
        require(common_matrix_base(reader, revision, plan["selection_inputs"]) == plan["base"], role + " exact common base changed")
        # The Dockerignore itself is pinned; these exact local COPY paths must be included.
        ignored, _ = reader(revision, COMMON_IGNORE)
        includes = set(ignored.decode().splitlines())
        require(all("!" + path.removeprefix("deploy/") in includes for path in actual_paths
                    if path not in (COMMON_DOCKERFILE, COMMON_IGNORE)), "common COPY input omitted from context allowlist")
        roles[role] = {"sha": revision, "factory_inputs": observed}
    return {"schema": "bex.native-common-factory-equivalence/v1", "base": plan["base"], "roles": roles,
            "critical_input_count": 20, "factory_equivalent": True, "candidate_approved": False,
            "scope": "Immutable source equivalence only; apt inventory/runtime/signature/report/hosted gates remain separate"}


def candidate_job(plan, run):
    validate_plan(plan, executing=True)
    require(run.get("databaseId") == plan["run_id"] and run.get("headSha") == plan["verification_sha"]
            and run.get("headBranch") == "main", "common run/source/main identity differs")
    jobs = [job for job in run.get("jobs", []) if job.get("databaseId") == plan["job_id"]]
    require(len(jobs) == 1, "exact common job identity missing or ambiguous")
    job = jobs[0]
    require(job.get("name") == plan["job_name"], "exact reusable common job name differs")
    require(job.get("status") == "completed" and job.get("conclusion") == "success", "common publication job is not complete/success")
    rows = job.get("steps", [])
    steps = {step["name"]: step for step in rows}
    require(len(steps) == len(rows), "common publication has ambiguous duplicate step names")
    expected = ORIGINAL_JOB_GATES | {COMMON_OUTPUT_GATE}
    require(expected <= steps.keys() and all(steps[name].get("status") == "completed"
            and steps[name].get("conclusion") == "success" for name in expected), "original runtime/scan/sign/output gate incomplete")
    primary = steps.get(BUILD_STEP, {})
    require(primary.get("status") == "completed" and primary.get("conclusion") == "success", "common fresh producer build did not succeed")
    recovery = steps.get("select a reviewed recovery artifact when its recipe is unchanged", {})
    require(recovery.get("conclusion") in (None, "skipped"), "common recovery is unsupported")
    return {"run_id": str(plan["run_id"]), "url": run["url"], "run_status": run["status"],
            "run_conclusion": run.get("conclusion"), "job_id": job["databaseId"], "job_url": job["url"],
            "job_name": job["name"], "job_conclusion": job["conclusion"], "verified_steps": sorted(expected),
            "primary_build_conclusion": primary["conclusion"], "recovery_selection_conclusion": recovery.get("conclusion")}


def local_resources(driver, plan, args):
    """Root-amended case resources only; the execution wrapper owns its cap/floor."""
    if "local_resource_policy" not in plan:
        return None  # Preserve the original source-only/host controls without amending resources.
    p = plan["local_resource_policy"]
    require(set(p) == {"remaining_free_bytes", "layer_workers", "layer_file_budget_bytes", "maximum_owned_bytes",
                      "outer_monitor_interval_seconds", "outer_guard_required", "ref", "sizing_provenance"}, "missing fixed local resource policy")
    require(p["ref"] == plan["ref"] and type(p["remaining_free_bytes"]) is int
            and p["remaining_free_bytes"] == (1 << 30) + (128 << 20), "unreviewed local operating reserve")
    require(type(args.layer_workers) is int and type(p["layer_workers"]) is int
            and args.layer_workers == p["layer_workers"] == 1, "local resource plan requires one layer worker")
    require(type(p["layer_file_budget_bytes"]) is int and p["layer_file_budget_bytes"] == driver.LAYER_FILE_BUDGET == 2 << 30,
            "original layer-file budget must remain unchanged")
    require(type(p["maximum_owned_bytes"]) is int and p["maximum_owned_bytes"] == 640 << 20
            and type(p["outer_monitor_interval_seconds"]) is int and p["outer_monitor_interval_seconds"] == 1
            and p["outer_guard_required"] is True, "required outer resource guard differs")
    sizing = p["sizing_provenance"]
    require(set(sizing) == {"layer_sizing_sha256", "bundle_sizing_sha256", "referrer_index_sha256"}
            and all(isinstance(value, str) and re.fullmatch(r"[a-f0-9]{64}", value) for value in sizing.values()), "missing descriptor sizing provenance")
    require(set(plan.get("local_resource_sizing", {})) == {"layers", "bundles"}
            and plan["local_resource_sizing"]["layers"]["sha256"] == sizing["layer_sizing_sha256"]
            and plan["local_resource_sizing"]["bundles"]["sha256"] == sizing["bundle_sizing_sha256"], "descriptor sizing linkage differs")
    driver.LAYER_FREE_RESERVE = p["remaining_free_bytes"]
    return dict(p, effective_layer_free_reserve=driver.LAYER_FREE_RESERVE,
                unchanged_driver_disk_reserve=driver.DISK_RESERVE, unchanged_driver_start_free=driver.START_FREE)


def review_class(driver, plan, reader, equivalence, plan_path):
    """Adapt only three source/CI selectors; retain every V11 review method unchanged."""
    original_load = driver.load
    plan_raw = plan_path.read_bytes()
    require(json.loads(plan_raw) == plan, "fixed plan bytes do not describe supplied plan")
    plan_sha = digest(plan_raw)
    equivalence_sha = digest(json.dumps(equivalence, sort_keys=True).encode())

    def common_load(name, path):
        policy = original_load(name, path)
        if name == "native_policy":
            def matrix_base(sha, runtime):
                require(runtime == "common-bookworm" and sha == plan["verification_sha"], "foreign common matrix request")
                return common_matrix_base(reader, sha, plan["selection_inputs"])

            def recipe_tree(sha):
                require(sha in {plan["producer_sha"], plan["verification_sha"], plan["generator_sha"]}, "foreign factory source")
                return {path: identity(*reader(sha, path)) for path in sorted(plan["factory_inputs"])}

            def github_candidate_job(sha, runtime, run_id):
                require(sha == plan["verification_sha"] and runtime == "common-bookworm"
                        and run_id == str(plan["run_id"]), "foreign common run request")
                run = policy.gh_json(["run", "view", run_id, "--json", "databaseId,headSha,headBranch,jobs,url,status,conclusion"])
                return candidate_job(plan, run)

            policy.matrix_base = matrix_base
            policy.recipe_tree = recipe_tree
            policy.github_candidate_job = github_candidate_job
        return policy

    class CommonReview(driver.Review):
        def __init__(self, args):
            resources = local_resources(driver, plan, args)
            physical_guard(args.output_root)
            require(digest(plan_path.read_bytes()) == plan_sha, "fixed plan changed before construction")
            driver.load = common_load
            try:
                super().__init__(args)
            finally:
                driver.load = original_load
            self.tool_hashes[str(Path(__file__).resolve())] = digest(Path(__file__).read_bytes())
            self.tool_hashes[str(plan_path)] = plan_sha
            if resources is not None:
                write_json(self.out / "local-resource-policy.json", resources)

        def metadata(self):
            super().metadata()
            require(self.ref == plan["ref"] and self.base == plan["base"], "actual canonical common REF/base differs from fixed plan")
            require(self.provenance["producer_sha"] == plan["producer_sha"]
                    and self.provenance["verification_sha"] == plan["verification_sha"], "actual producer/verifier provenance differs")
            self.provenance.update(generator_sha=plan["generator_sha"],
                                   common_factory_equivalence_sha256=equivalence_sha,
                                   common_review_plan_sha256=plan_sha,
                                   scope="Fresh common producer/verifier; separately identified current generator with equivalent factory inputs")
            write_json(self.out / "common-source-equivalence.json", equivalence)
            write_json(self.out / "producer-verification-provenance.json", self.provenance)
            # Source bodies are all locally retained Git inputs, not fetched executable payloads.
            source = self.out / "common-source"
            physical_guard(self.out)
            source.mkdir(exist_ok=True)
            for role in ("producer", "verification", "generator"):
                index = {}
                revision = plan[role + "_sha"]
                for path in sorted(set(plan["factory_inputs"]) | set(plan["selection_inputs"]) | {
                        ".github/workflows/native-base-image.yml", ".github/workflows/native-common-image.yml"}):
                    raw, mode = reader(revision, path)
                    target = source / role / path
                    physical_guard(self.out)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(raw)
                    index[path] = identity(raw, mode)
                write_json(source / (role + "-source-files.json"), {"sha": revision, "files": index})
    return CommonReview


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("source", "review"))
    parser.add_argument("--plan", type=Path, required=True)
    parser.add_argument("--repo-root", type=Path, required=True)
    parser.add_argument("--output-root", type=Path, required=True)
    parser.add_argument("--cache-root", type=Path)
    parser.add_argument("--layer-workers", type=int, default=2, choices=range(1, 5))
    args = parser.parse_args()
    physical_guard(args.output_root)
    args.output_root = args.output_root.resolve()
    args.repo_root = args.repo_root.resolve()
    require(args.output_root != args.repo_root and args.repo_root not in args.output_root.parents, "private output must stay outside repository")
    require(args.plan.stat().st_size <= MAX_SOURCE_BYTES, "fixed plan exceeds bound")
    raw = args.plan.read_bytes()
    plan = json.loads(raw)
    validate_plan(plan, executing=args.mode == "review")
    require(args.mode != "review" or "local_resource_policy" in plan, "resource adapter review requires its fixed resource plan")
    reader = GitSource(args.repo_root)
    equivalent = source_equivalence(plan, reader)
    require(digest((args.repo_root / "scripts/lib/toolchain-freshness.py").read_bytes()) == plan["freshness_sha256"],
            "local V11 freshness support helper differs from fixed plan")
    if args.mode == "source":
        write_json(args.output_root / "source-equivalence.json", equivalent)
        print(json.dumps({"source_equivalent": True, "candidate_approved": False, "generator_sha": plan["generator_sha"]}))
        return
    require(args.cache_root is not None, "explicit owned review cache root required")
    retained = Path(__file__).resolve().parent / "retained"
    for name, expected in PINNED.items():
        require(digest((retained / name).read_bytes()) == expected, "retained V11 source SHA differs: " + name)
    driver = import_pinned(retained / "loopx-w8-native-review-v11.py", PINNED["loopx-w8-native-review-v11.py"])
    cls = review_class(driver, plan, reader, equivalent, args.plan.resolve())
    original_args = SimpleNamespace(name="common-bookworm", sha=plan["verification_sha"], run=str(plan["run_id"]),
        repo_root=str(args.repo_root), policy_module=str(retained / "loopx-w8-verify-native-artifact-ranged.py"),
        output_root=str(args.output_root), cache_root=str(args.cache_root), layer_workers=args.layer_workers)
    review = None
    try:
        review = cls(original_args)
        review.run()  # Original metadata→bundles→crypto→root proof→report binding→full blob finalization.
    except Exception as error:
        if review is not None:
            write_json(review.out / "failed-review.json", {"at": review.v.now(), "candidate_approved": False,
                "error_type": type(error).__name__, "error": driver.safe_error(error)})
        raise


if __name__ == "__main__":
    main()
