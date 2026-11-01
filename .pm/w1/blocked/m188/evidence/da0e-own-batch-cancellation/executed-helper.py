#!/usr/bin/env python3
"""One explicitly authorized own-source push-batch cancel; never cancel started native work."""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import time

REPO = "bex-co/bex"
WORKFLOW = ".github/workflows/native-base-image.yml"


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def frozen(path, value):
    with path.open("x") as stream:
        json.dump(value, stream, indent=2)
        stream.write("\n")


def reference(path):
    raw = path.read_bytes()
    return {"file": str(path), "sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw)}


def valid_run(run, source):
    assert run["head_sha"] == source and run["head_branch"] == "main"
    assert run["event"] == "push" and run["path"] == WORKFLOW
    assert run["run_attempt"] == 1


def native_review(run, document, source):
    valid_run(run, source)
    jobs = document["jobs"]
    assert document["total_count"] == len(jobs) and len(jobs) <= 100
    native = []
    for job in jobs:
        assert job["run_id"] == run["id"] and job["run_attempt"] == 1 and job["head_sha"] == source
        assert isinstance(job["steps"], list)
        name = job["name"]
        if name == "matrix":
            continue
        assert name == "build" or name.startswith("build ("), "unknown workflow job"
        started_steps = [step for step in job["steps"] if step.get("started_at") or step.get("status") in ("in_progress", "completed")]
        native.append({"id": job["id"], "name": name, "status": job["status"], "runner_id": job.get("runner_id"), "runner_name": job.get("runner_name"), "started_steps": started_steps})
    safe = run["status"] != "completed" and all(
        job["status"] in ("queued", "pending", "waiting", "requested")
        and job["runner_id"] in (None, 0) and not job["runner_name"] and not job["started_steps"]
        for job in native
    )
    return {"at": now(), "source": source, "run_id": run["id"], "run_attempt": 1, "run_status": run["status"], "native_jobs": native, "safe_zero_started_native_work": safe}


class Watch:
    def __init__(self, source, output):
        self.source, self.output = source, output

    def api(self, method, endpoint, label):
        request = self.output / (label + ".request.json")
        argv = ["gh", "api", "--method", method]
        if method == "POST":
            argv.append("--include")
        argv.append(endpoint)
        intent = {"started_at": now(), "argv": argv, "source": self.source}
        frozen(request, intent)
        try:
            result = subprocess.run(argv, capture_output=True, timeout=45)
            stdout, stderr, code = result.stdout, result.stderr, result.returncode
        except subprocess.TimeoutExpired as error:
            stdout, stderr, code = error.stdout or b"", error.stderr or b"", None
        raw = self.output / (label + ".stdout")
        err = self.output / (label + ".stderr")
        with raw.open("xb") as stream:
            stream.write(stdout)
        with err.open("xb") as stream:
            stream.write(stderr)
        receipt = {**intent, "finished_at": now(), "exit_code": code, "stdout": reference(raw), "stderr": reference(err)}
        frozen(self.output / (label + ".result.json"), receipt)
        if method == "POST":
            return receipt, code == 0 and bool(re.match(rb"HTTP/\S+ 202(?:\s|$)", stdout))
        if code != 0:
            raise RuntimeError("read-only GitHub observation failed: " + label)
        return json.loads(stdout)

    def terminal(self, run_id, interval, seconds):
        deadline = time.monotonic() + seconds
        number = 0
        while time.monotonic() < deadline:
            number += 1
            try:
                run = self.api("GET", f"repos/{REPO}/actions/runs/{run_id}", f"terminal-{number}-run")
                valid_run(run, self.source)
                if run["status"] == "completed":
                    jobs = self.api("GET", f"repos/{REPO}/actions/runs/{run_id}/attempts/1/jobs?per_page=100", f"terminal-{number}-jobs")
                    review = native_review(run, jobs, self.source)
                    receipt = {"at": now(), "source": self.source, "run_id": run_id, "terminal_confirmed": True, "conclusion": run["conclusion"], "native_jobs": review["native_jobs"], "raw_run": reference(self.output / f"terminal-{number}-run.stdout"), "raw_jobs": reference(self.output / f"terminal-{number}-jobs.stdout")}
                    frozen(self.output / "terminal-confirmation.json", receipt)
                    print(json.dumps(receipt), flush=True)
                    return
            except (RuntimeError, json.JSONDecodeError):
                pass
            time.sleep(interval)
        frozen(self.output / "terminal-pending.json", {"at": now(), "run_id": run_id, "source": self.source, "terminal_confirmed": False, "no_cancel_retry": True})

    def execute(self, interval, seconds):
        deadline = time.monotonic() + seconds
        number = 0
        while time.monotonic() < deadline:
            number += 1
            try:
                document = self.api("GET", f"repos/{REPO}/actions/workflows/native-base-image.yml/runs?event=push&branch=main&head_sha={self.source}&per_page=100", f"discovery-{number}")
                assert document["total_count"] <= 100
                matches = [run for run in document["workflow_runs"] if run["head_sha"] == self.source and run["event"] == "push" and run["head_branch"] == "main" and run["path"] == WORKFLOW]
                assert len(matches) <= 1, "multiple own-source push runs; refuse selection"
                if not matches:
                    time.sleep(interval)
                    continue
                run_id = matches[0]["id"]
                run = self.api("GET", f"repos/{REPO}/actions/runs/{run_id}", f"precheck-{number}-run")
                jobs = self.api("GET", f"repos/{REPO}/actions/runs/{run_id}/attempts/1/jobs?per_page=100", f"precheck-{number}-jobs")
                review = native_review(run, jobs, self.source)
                review["raw_run"] = reference(self.output / f"precheck-{number}-run.stdout")
                review["raw_jobs"] = reference(self.output / f"precheck-{number}-jobs.stdout")
                frozen(self.output / "fresh-review.json", review)
                if not review["safe_zero_started_native_work"]:
                    print(json.dumps({"stopped": "terminal run or actual native work; no cancel", "review": review}), flush=True)
                    return
                # Exclusive intent freezes the one-POST budget before dispatch.
                frozen(self.output / "cancel-intent.json", {"at": now(), "run_id": run_id, "source": self.source, "max_posts": 1, "review": reference(self.output / "fresh-review.json"), "no_force": True, "no_retry_even_if_uncertain": True})
                receipt, accepted = self.api("POST", f"repos/{REPO}/actions/runs/{run_id}/cancel", "cancel")
                print(json.dumps({"source": self.source, "run_id": run_id, "cancel_request_http202": accepted, "terminal_confirmed": False, "result": receipt}), flush=True)
                self.terminal(run_id, interval, seconds)
                return
            except (RuntimeError, json.JSONDecodeError):
                # Only GET failures reach here before the exclusive intent exists.
                # An uncertain POST is never repeated, including on helper restart.
                if (self.output / "cancel-intent.json").exists():
                    raise
                time.sleep(interval)
        frozen(self.output / "discovery-pending.json", {"at": now(), "source": self.source, "run_not_observed": True, "cancel_posts": 0})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source")
    parser.add_argument("output", type=Path)
    parser.add_argument("--authorization", type=Path)
    parser.add_argument("--execute", action="store_true")
    parser.add_argument("--interval", type=int, default=15)
    parser.add_argument("--seconds", type=int, default=1200)
    args = parser.parse_args()
    assert re.fullmatch(r"[0-9a-f]{40}", args.source)
    assert 5 <= args.interval <= 60 and 60 <= args.seconds <= 1800
    if not args.execute:
        print(json.dumps({"prepared_only": True, "source": args.source, "remote_calls": 0, "execute_requires_explicit_root_authorization": True}))
        return
    assert args.authorization, "explicit exact-SHA root authorization required"
    approval = json.loads(args.authorization.read_text())
    assert approval["source"] == args.source and approval["authorized"] is True and approval["max_cancel_posts"] == 1
    assert approval["workflow"] == WORKFLOW and approval["event"] == "push" and approval["branch"] == "main"
    args.output.mkdir(mode=0o700, parents=False, exist_ok=False)
    shutil.copyfile(args.authorization, args.output / "authorization.json")
    shutil.copyfile(__file__, args.output / "executed-helper.py")
    Watch(args.source, args.output).execute(args.interval, args.seconds)


if __name__ == "__main__":
    main()
