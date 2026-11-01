#!/usr/bin/env python3
"""Execute only root's ten selective POST bounds; never cancel a run here."""
import datetime
import hashlib
import json
import pathlib
import re
import subprocess
import sys
import time

ROOT = pathlib.Path(__file__).resolve().parent
OUT = ROOT / "execution"
OUT.mkdir(exist_ok=True)
BASE = "83c1e30e35eeeadab499de4d8d96c1f6c0349077"
WORKFLOW = "39b2ebac4a5b4d502db7a8245d90a9031db779ba4cea6da55588cb056abe469f"
CATALOG = "42c86c2b025b2f724b945888730d6cac042322fee62d2008160bac0a801b4d1e"
PINS = {
    "deploy/gitops/base/bex.yaml",
    "deploy/gitops/base/dashboard.yaml",
    "deploy/gitops/base/values/opensandbox-controller.values.yaml",
    "deploy/opensandbox/kustomization.yaml",
    "lego/operator/config/api/deployment.yaml",
}


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def save(path, value):
    raw = json.dumps(value, indent=2).encode() + b"\n"
    if path.exists():
        raise RuntimeError("refusing evidence overwrite: " + str(path))
    path.write_bytes(raw)
    return {"file": str(path), "sha256": sha(raw)}


def api(route, path):
    result = subprocess.run(["gh", "api", route], capture_output=True, timeout=40)
    save(path.with_suffix(".request.json"), {
        "at": now(), "route": route, "exit_code": result.returncode,
        "stderr": result.stderr.decode(errors="replace"),
    })
    if result.returncode:
        raise RuntimeError(result.stderr.decode(errors="replace"))
    if path.exists():
        raise RuntimeError("refusing raw overwrite")
    path.write_bytes(result.stdout)
    return json.loads(result.stdout)


def git(*args):
    return subprocess.check_output(["git", *args])


def ensure_object(source):
    if subprocess.run(["git", "cat-file", "-e", source + "^{commit}"],
                      capture_output=True).returncode:
        subprocess.run(["git", "fetch", "--no-tags", "origin", source], check=True,
                       stdout=subprocess.DEVNULL)


def profile(source):
    ensure_object(source)
    return sha(git("show", source + ":.github/workflows/native-base-image.yml"))


def source_predicate(source):
    ensure_object(source)
    if subprocess.run(["git", "merge-base", "--is-ancestor", BASE, source]).returncode:
        raise RuntimeError("source is not approved baseline descendant")
    if profile(source) != WORKFLOW:
        raise RuntimeError("workflow changed")
    if sha(git("show", source + ":.github/native-base-recovery.json")) != CATALOG:
        raise RuntimeError("recovery catalog changed")
    paths = git("diff", "--no-renames", "--name-only", BASE, source,
                "--", ".", ":!.pm").decode().splitlines()
    for path in paths:
        if path not in PINS:
            raise RuntimeError("non-PM source changed: " + path)
        before = git("show", BASE + ":" + path)
        after = git("show", source + ":" + path)
        normalize = lambda raw: re.sub(rb"sha256:[0-9a-f]{64}", b"sha256:DIGEST", raw)
        if normalize(before) != normalize(after):
            raise RuntimeError("deployment change is not digest-only: " + path)
        if git("ls-tree", BASE, "--", path).split()[0] != git("ls-tree", source, "--", path).split()[0]:
            raise RuntimeError("deployment file mode changed: " + path)
    return {"source": source, "baseline": BASE, "workflow_sha256": WORKFLOW,
            "catalog_sha256": CATALOG, "allowed_digest_only_paths": paths,
            "predicate_passed": True, "checked_at": now()}


def inventory(folder):
    rows = []
    page = 1
    while True:
        data = api("repos/bex-co/bex/actions/workflows/native-base-image.yml/runs"
                   f"?branch=main&per_page=100&page={page}", folder / f"runs-{page}.json")
        rows.extend(data["workflow_runs"])
        if len(rows) >= data["total_count"] or not data["workflow_runs"]:
            return rows
        page += 1


def native_jobs(data):
    return [j for j in data["jobs"] if j["name"].startswith("build")]


def tag_of(job):
    match = re.match(r"build \(([^,]+)", job["name"])
    return match.group(1) if match else None


approval_raw = (ROOT / "root-execution-approval.json").read_bytes()
assert sha(approval_raw) == "6ed57d9e983e879881f4c7acf9966cd06a54645a9d3288e3ecbce118208ec6a6"
approval = json.loads(approval_raw)
assert approval["authorized"] and len(approval["exact_targets"]) == 10
assert sha((ROOT / "plan-final.json").read_bytes()) == "eaccae1ce5880226190a49b04350fdd608e15b6a632de2cfc77ecf421264f2fd"
assert sha((ROOT / "root-source-admission.json").read_bytes()) == "9e108c964380c624c442d04596b066b9ed29ff91764d2aae86be1fdb3b34a809"
aggregate = pathlib.Path("/tmp/bex-m188-published-acceptance.json")
aggregate_raw_before = aggregate.read_bytes()
aggregate_before = sha(aggregate_raw_before)
requests = {}

for target in approval["exact_targets"]:
    tag = target["tag"]
    folder = OUT / tag
    if folder.exists():
        assert tag == "node-22-bookworm"
        binding_raw = (folder / "run-head-binding.json").read_bytes()
        assert sha(binding_raw) == "c0bbe819a955d2d860771a894d65c549826b30f23a425086bf49ad985d941187"
        binding = json.loads(binding_raw)
        prior = json.loads((folder / "result.json").read_text())
        assert prior["dispatch_performed"] and prior["post_exit_code"] == 0
        assert binding["tag"] == tag and binding["source"] == BASE
        requests[tag] = dict(prior, actual_run_id=binding["run_id"], actual_source=binding["source"],
                             actual_run_attempt=binding["run_attempt"], outcome="prior_accepted_request_bound")
        print(json.dumps({"tag": tag, "outcome": "resume_existing_no_post", "run_id": binding["run_id"]}), flush=True)
        continue
    folder.mkdir(exist_ok=False)
    result = {"tag": tag, "began_at": now(), "old_run_id": target["old_run_id"],
              "old_job_id": target["old_job_id"], "dispatch_performed": False}
    try:
        head = api("repos/bex-co/bex/git/ref/heads/main", folder / "remote-main.json")["object"]["sha"]
        result["source_predicate"] = source_predicate(head)
        save(folder / "source-predicate.json", result["source_predicate"])
        # Dedupe every recent main run, regardless of our crypto source allowlist.
        existing = []
        unresolved = []
        before_runs = inventory(folder)
        for run in before_runs:
            if run["event"] != "workflow_dispatch":
                continue
            if profile(run["head_sha"]) != WORKFLOW:
                continue
            if git("diff", "--no-renames", "--name-only", BASE, run["head_sha"],
                   "--", "deploy/native-base", "lego/operator/internal/build/native.go").strip():
                raise RuntimeError("fixed-profile run has different factory: " + str(run["id"]))
            jobs = api(f"repos/bex-co/bex/actions/runs/{run['id']}/jobs?per_page=100",
                       folder / f"dedupe-run-{run['id']}-jobs.json")
            native = native_jobs(jobs)
            names = [tag_of(job) for job in native]
            known_request = next((t for t, req in requests.items()
                                  if req.get("actual_run_id") == run["id"]), None)
            if tag in names or known_request == tag:
                existing.append({"run_id": run["id"], "source": run["head_sha"],
                                 "run_attempt": run["run_attempt"], "native_jobs": native})
            elif not names and known_request is None:
                unresolved.append(run["id"])
        if unresolved:
            raise RuntimeError("unknown fixed-profile selective inputs: " + str(unresolved))
        if existing:
            result.update(outcome="reused_existing_fixed_request", existing=existing)
            save(folder / "result.json", result)
            print(json.dumps({"tag": tag, "outcome": result["outcome"],
                              "existing_run_ids": [r["run_id"] for r in existing]}), flush=True)
            continue
        run = api(f"repos/bex-co/bex/actions/runs/{target['old_run_id']}", folder / "old-run.json")
        jobs = api(f"repos/bex-co/bex/actions/runs/{target['old_run_id']}/jobs?per_page=100",
                   folder / "old-jobs.json")
        native = native_jobs(jobs)
        if not (run["head_sha"] == target["old_source"] and run["run_attempt"] == 1
                and run["event"] == "workflow_dispatch" and run["head_branch"] == "main"
                and len(native) == 1 and native[0]["id"] == target["old_job_id"]
                and tag_of(native[0]) == tag and native[0]["status"] == "queued"
                and native[0].get("runner_id") == 0 and not native[0].get("steps")):
            result.update(outcome="old_not_zero_start_preserved", native_jobs=native)
            save(folder / "result.json", result)
            print(json.dumps({"tag": tag, "outcome": result["outcome"]}), flush=True)
            continue
        payload = folder / "dispatch-input.json"
        save(payload, {"ref": "main", "inputs": {"runtime": tag}})
        began = now()
        save(folder / "post-intent.json", {"at": began, "tag": tag, "requested_source": head,
                                          "maximum_posts_for_this_tag": 1})
        try:
            post = subprocess.run(["gh", "api", "--include", "-X", "POST",
                                   "repos/bex-co/bex/actions/workflows/native-base-image.yml/dispatches",
                                   "--input", str(payload)], capture_output=True, timeout=45)
        except subprocess.TimeoutExpired as error:
            save(folder / "post-uncertain-timeout.json", {"at": now(), "tag": tag,
                 "potential_post_sent": True, "no_retry": True, "error": str(error)})
            raise RuntimeError("POST timed out; discover before any retry") from error
        result.update(dispatch_performed=True, dispatch_started_at=began,
                      dispatch_finished_at=now(), requested_source=head,
                      post_exit_code=post.returncode,
                      post_stdout=post.stdout.decode(errors="replace"),
                      post_stderr=post.stderr.decode(errors="replace"))
        save(folder / "dispatch-result.json", result)
        if post.returncode or not re.search(rb"HTTP/\S+ 204", post.stdout):
            result["outcome"] = "uncertain_post_no_retry"
            save(folder / "result.json", result)
            raise RuntimeError("POST uncertain; discover before any retry")
        # Associate only a unique new same-event/source run after our request.
        previous_ids = {r["id"] for r in before_runs}
        candidates = []
        for discovery_attempt in range(1, 4):
            after = api(
                "repos/bex-co/bex/actions/workflows/native-base-image.yml/runs?branch=main&per_page=100",
                folder / f"after-runs-{discovery_attempt}.json")
            candidates = [r for r in after["workflow_runs"] if r["event"] == "workflow_dispatch"
                          and r["id"] not in previous_ids and r["created_at"] >= began[:19] + "Z"]
            if candidates:
                break
            time.sleep(2)
        if len(candidates) == 1:
            actual_predicate = source_predicate(candidates[0]["head_sha"])
            result.update(actual_run_id=candidates[0]["id"], actual_source=candidates[0]["head_sha"],
                          actual_run_attempt=candidates[0]["run_attempt"],
                          actual_source_predicate=actual_predicate, outcome="request_bound_run_head")
        else:
            result.update(outcome="request_accepted_run_binding_pending", possible_runs=[r["id"] for r in candidates])
        requests[tag] = result
        save(folder / "result.json", result)
        print(json.dumps({k: result.get(k) for k in ["tag", "outcome", "actual_run_id", "actual_source", "dispatch_finished_at"]}), flush=True)
        if result["outcome"] != "request_bound_run_head":
            raise RuntimeError("accepted request requires unique discovery before more POSTs")
    except Exception as error:
        save(folder / "halt.json", {"at": now(), "error": str(error), "tag": tag,
                                   "remaining_mutations_held": True})
        print(json.dumps({"halted": tag, "error": str(error)}), flush=True)
        break

aggregate_raw_after = aggregate.read_bytes()
before_fields = json.loads(aggregate_raw_before)
after_fields = json.loads(aggregate_raw_after)
before_fields.pop("accepted_sources", None)
after_fields.pop("accepted_sources", None)
assert before_fields == after_fields
save(OUT / "dispatch-summary-continued.json", {"at": now(), "requests": requests,
    "aggregate_before_sha256": aggregate_before,
    "aggregate_after_sha256": sha(aggregate_raw_after),
    "all_fields_except_external_source_policy_unchanged": True,
    "old_cancel_posts": 0, "legacy_requests_untouched": approval["preserve_legacy"]})
