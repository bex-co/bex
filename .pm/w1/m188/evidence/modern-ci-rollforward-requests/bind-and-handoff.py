#!/usr/bin/env python3
"""One bounded pass: bind actual replacements, then cancel only exact old zero-starts."""
import datetime
import hashlib
import json
import pathlib
import re
import subprocess

ROOT = pathlib.Path(__file__).resolve().parent
OUT = ROOT / "handoffs"
OUT.mkdir(exist_ok=True)
SOURCE = "83c1e30e35eeeadab499de4d8d96c1f6c0349077"


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def save(path, value):
    assert not path.exists(), str(path)
    path.write_text(json.dumps(value, indent=2) + "\n")
    return {"file": str(path), "sha256": sha(path.read_bytes())}


def api(route, path):
    result = subprocess.run(["gh", "api", route], capture_output=True, timeout=40)
    save(path.with_suffix(".request.json"), {"at": now(), "route": route,
         "exit_code": result.returncode, "stderr": result.stderr.decode(errors="replace")})
    assert result.returncode == 0, result.stderr.decode(errors="replace")
    assert not path.exists()
    path.write_bytes(result.stdout)
    return json.loads(result.stdout)


def native(data):
    return [job for job in data["jobs"] if job["name"].startswith("build")]


def tag_of(job):
    match = re.match(r"build \(([^,]+)", job["name"])
    return match.group(1) if match else None


approval_raw = (ROOT / "root-execution-approval.json").read_bytes()
assert sha(approval_raw) == "6ed57d9e983e879881f4c7acf9966cd06a54645a9d3288e3ecbce118208ec6a6"
approval = json.loads(approval_raw)
admission_raw = (ROOT / "root-source-admission.json").read_bytes()
assert sha(admission_raw) == "9e108c964380c624c442d04596b066b9ed29ff91764d2aae86be1fdb3b34a809"
assert json.loads(admission_raw)["admitted"]
heads_raw = (ROOT / "request-heads.json").read_bytes()
assert sha(heads_raw) == "16593f0073e7e2e909fa2d589459b9ea211301d789eeaac74353071a9f963fc1"
heads = {row["tag"]: row for row in json.loads(heads_raw)["unique_run_heads"]}
status = json.loads(pathlib.Path("/tmp/bex-m188-publisher-status.json").read_text())
observed = {row["run_id"]: row for row in status["publishers"]}
aggregate_path = pathlib.Path("/tmp/bex-m188-published-acceptance.json")
accepted_before = json.loads(aggregate_path.read_text())["verified"]

for target in approval["exact_targets"]:
    tag = target["tag"]
    folder = OUT / tag
    if folder.exists():
        continue
    replacement = heads[tag]
    assert replacement["source"] == SOURCE and replacement["run_attempt"] == 1
    cached = observed.get(replacement["run_id"])
    if not cached or cached.get("observation_stale"):
        continue
    cached_native = [job for job in cached["jobs"] if job.get("tag") == tag]
    if len(cached_native) != 1:
        continue
    folder.mkdir(exist_ok=False)
    result = {"began_at": now(), "tag": tag, "old_run_id": target["old_run_id"],
              "old_job_id": target["old_job_id"], "replacement_run_id": replacement["run_id"],
              "cancel_post_performed": False, "replacement_source": SOURCE}
    try:
        run = api(f"repos/bex-co/bex/actions/runs/{replacement['run_id']}", folder / "replacement-run.json")
        jobs = api(f"repos/bex-co/bex/actions/runs/{replacement['run_id']}/jobs?per_page=100",
                   folder / "replacement-jobs.json")
        new_jobs = native(jobs)
        assert run["head_sha"] == SOURCE and run["run_attempt"] == 1
        assert run["event"] == "workflow_dispatch" and run["head_branch"] == "main"
        assert len(new_jobs) == 1 and tag_of(new_jobs[0]) == tag
        new_job = new_jobs[0]
        assert new_job["id"] == cached_native[0]["id"]
        assert new_job.get("conclusion") in (None, "success")
        assert new_job.get("run_attempt", 1) == 1
        result.update(replacement_job_id=new_job["id"], replacement_status=new_job["status"],
                      replacement_run_attempt=1, replacement_binding_observed_at=now())
        save(folder / "replacement-binding.json", result)
        old_run = api(f"repos/bex-co/bex/actions/runs/{target['old_run_id']}", folder / "old-run.json")
        old_jobs = api(f"repos/bex-co/bex/actions/runs/{target['old_run_id']}/jobs?per_page=100",
                       folder / "old-jobs.json")
        old_native = native(old_jobs)
        assert old_run["head_sha"] == target["old_source"] and old_run["run_attempt"] == 1
        assert old_run["event"] == "workflow_dispatch" and old_run["head_branch"] == "main"
        assert len(old_native) == 1 and old_native[0]["id"] == target["old_job_id"]
        old_job = old_native[0]
        assert tag_of(old_job) == tag
        result.update(old_fresh_status=old_job["status"], old_fresh_conclusion=old_job["conclusion"],
                      old_fresh_runner_id=old_job.get("runner_id"),
                      old_fresh_steps_count=len(old_job.get("steps", [])), old_fresh_observed_at=now())
        if old_job["status"] == "completed" and old_job["conclusion"] == "cancelled":
            result["outcome"] = "old_already_terminal_cancelled_no_post"
        elif not (old_run["status"] == "queued" and old_job["status"] == "queued"
                  and old_job.get("runner_id") == 0 and not old_job.get("steps")):
            result["outcome"] = "old_not_zero_start_preserved_replacement_kept_as_fallback"
        else:
            save(folder / "old-zero-start-review.json", result)
            save(folder / "cancel-intent.json", {"at": now(), "old_run_id": target["old_run_id"],
                 "maximum_cancel_posts_for_old_run": 1, "root_approval_sha256": sha(approval_raw)})
            try:
                post = subprocess.run(["gh", "api", "--include", "-X", "POST",
                                       f"repos/bex-co/bex/actions/runs/{target['old_run_id']}/cancel"],
                                      capture_output=True, timeout=40)
            except subprocess.TimeoutExpired as error:
                result.update(outcome="cancel_post_uncertain_no_retry", cancel_post_performed=True,
                              error=str(error), cancel_finished_at=now())
                raise
            result.update(cancel_post_performed=True, cancel_exit_code=post.returncode,
                          cancel_stdout=post.stdout.decode(errors="replace"),
                          cancel_stderr=post.stderr.decode(errors="replace"), cancel_finished_at=now())
            if post.returncode == 0 and re.search(rb"HTTP/\S+ 202", post.stdout):
                result["outcome"] = "cancel_requested_terminal_not_yet_confirmed"
            else:
                result["outcome"] = "cancel_response_requires_readonly_followup_no_retry"
        save(folder / "outcome.json", result)
        print(json.dumps({k: result.get(k) for k in ["tag", "replacement_run_id", "replacement_job_id",
              "old_run_id", "outcome", "cancel_exit_code"]}), flush=True)
    except Exception as error:
        save(folder / "halt.json", dict(result, error=str(error), at=now(), no_retry=True))
        print(json.dumps({"tag": tag, "halt": str(error)}), flush=True)

assert json.loads(aggregate_path.read_text())["verified"] == accepted_before
assert len(list(OUT.glob("*/cancel-intent.json"))) <= 10
