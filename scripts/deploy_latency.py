#!/usr/bin/env python3
# Copyright 2026.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""How long has main's oldest deployable commit waited for production, and why?

A deployable commit is one deploy.yml's push filter would deploy; it is
deployed once the latest `chore(deploy): pin platform images to <sha>` commit
names a descendant of it. w6/087 recorded deploy starvation with a ">12 h
means reopen" rule nobody checked, and gaps of 15.7 h and 24.9 h went by
unnoticed (w6/m156). This reports the wait and classifies its cause from the
deploy runs that carry the commit:

  deployed               nothing is waiting
  not-triggered          no deploy run includes the commit yet
  queued                 a run waits for the bex-production-deploy slot
  waiting-for-runner     a job has waited over 5 min for a runner
  in-progress            a run is executing
  failed                 the newest run carrying it failed (job/step named)
  cancelled              the newest run carrying it was cancelled while pending
  superseded             the newest run succeeded but did not pin (superseded)

Usage: deploy_latency.py [--json] [--repo DIR] [--threshold-hours H]

Exit 0 when nothing has waited longer than H hours (default: no threshold),
1 when the oldest deployable commit has, 2 when the inputs are unusable.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import pathlib
import re
import subprocess
import sys
from dataclasses import asdict, dataclass

PIN_SUBJECT = re.compile(r"^chore\(deploy\): pin platform images to ([0-9a-f]{7,40})")
RUNNER_WAIT = dt.timedelta(minutes=5)
ACTIVE = {"in_progress", "queued", "waiting", "pending", "requested"}


@dataclass
class Wait:
    state: str
    commit: str = ""
    subject: str = ""
    age_hours: float = 0.0
    run_id: int = 0
    detail: str = ""


def push_paths(deploy_yml: str) -> tuple[list[str], list[str]]:
    """deploy.yml's on.push.paths, split into include and `!` exclude globs."""
    lines = deploy_yml.splitlines()
    try:
        start = next(i for i, line in enumerate(lines) if re.match(r"^    paths:\s*$", line))
    except StopIteration as exc:
        raise ValueError("deploy.yml has no on.push.paths") from exc
    include, exclude = [], []
    for line in lines[start + 1 :]:
        if re.match(r"^\s*#", line) or not line.strip():
            continue
        item = re.match(r'^      - "([^"]+)"\s*$', line)
        if not item:
            break
        glob = item.group(1)
        (exclude if glob.startswith("!") else include).append(glob.lstrip("!"))
    return include, exclude


def parse_time(value: str) -> dt.datetime:
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


def classify(runs: list[dict], jobs: dict[int, list[dict]], now: dt.datetime) -> tuple[str, int, str]:
    """The waiting commit's state from the deploy runs that carry it, newest first."""
    if not runs:
        return "not-triggered", 0, ""
    for run in runs:
        if run.get("status") in ACTIVE:
            run_id = run["databaseId"]
            for job in jobs.get(run_id, []):
                if job.get("status") == "queued" and not job.get("runner_name") and job.get("created_at"):
                    waited = now - parse_time(job["created_at"])
                    if waited > RUNNER_WAIT:
                        return "waiting-for-runner", run_id, f"{job.get('name')} queued {int(waited.total_seconds() // 60)} min"
            if run.get("status") == "in_progress":
                return "in-progress", run_id, ""
            if run.get("status") == "waiting":
                return "queued", run_id, "waiting on an environment protection rule"
            return "queued", run_id, "waiting for the bex-production-deploy slot"
    run = runs[0]
    run_id = run["databaseId"]
    conclusion = run.get("conclusion")
    if conclusion == "failure":
        for job in jobs.get(run_id, []):
            if job.get("conclusion") == "failure":
                steps = [s.get("name", "") for s in job.get("steps", []) if s.get("conclusion") == "failure"]
                return "failed", run_id, f"{job.get('name')}: {', '.join(steps) or 'failed'}"
        return "failed", run_id, ""
    if conclusion == "cancelled":
        return "cancelled", run_id, "cancelled while pending or superseded in the queue"
    if conclusion == "success":
        return "superseded", run_id, "the run succeeded without pinning this commit"
    return conclusion or "unknown", run_id, ""


def git(repo: pathlib.Path, *args: str) -> str:
    return subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True, text=True).stdout.strip()


def is_ancestor(repo: pathlib.Path, ancestor: str, descendant: str) -> bool:
    return subprocess.run(["git", "-C", str(repo), "merge-base", "--is-ancestor", ancestor, descendant],
                          capture_output=True).returncode == 0


def gh_json(*args: str):
    out = subprocess.run(["gh", *args], check=True, capture_output=True, text=True).stdout
    return json.loads(out or "null")


def measure(repo: pathlib.Path, now: dt.datetime | None = None) -> Wait:
    now = now or dt.datetime.now(dt.timezone.utc)
    head = "origin/main"
    pin_line = git(repo, "log", "-1", "--format=%H %s", "--grep=^chore(deploy): pin platform images to ", head)
    match = PIN_SUBJECT.match(pin_line.split(" ", 1)[1]) if pin_line else None
    if not match:
        raise ValueError("no pin commit on origin/main")
    pinned = git(repo, "rev-parse", match.group(1))
    include, exclude = push_paths((repo / ".github/workflows/deploy.yml").read_text())
    pathspec = include + [f":(exclude){glob}" for glob in exclude]
    pending = git(repo, "rev-list", "--reverse", "--no-merges", f"{pinned}..{head}", "--", *pathspec).split()
    pending = [c for c in pending if not PIN_SUBJECT.match(git(repo, "log", "-1", "--format=%s", c))]
    if not pending:
        return Wait("deployed")
    oldest = pending[0]
    committed = dt.datetime.fromtimestamp(int(git(repo, "log", "-1", "--format=%ct", oldest)), dt.timezone.utc)
    # No --branch: that filter goes through a lagging search and can miss the
    # newest runs (as in ci-red-streak.sh), and the listing order is not a
    # contract, so filter and sort here.
    runs = gh_json("run", "list", "--workflow=deploy.yml", "--limit", "60",
                   "--json", "databaseId,headSha,headBranch,status,conclusion,createdAt") or []
    runs = sorted((r for r in runs if r.get("headBranch") == "main"), key=lambda r: r["createdAt"], reverse=True)
    carrying = [r for r in runs if is_ancestor(repo, oldest, r["headSha"])]
    jobs = {}
    for run in carrying[:3]:
        data = gh_json("api", f"repos/{{owner}}/{{repo}}/actions/runs/{run['databaseId']}/jobs")
        jobs[run["databaseId"]] = (data or {}).get("jobs", [])
    state, run_id, detail = classify(carrying, jobs, now)
    return Wait(state, oldest[:12], git(repo, "log", "-1", "--format=%s", oldest),
                round((now - committed).total_seconds() / 3600, 1), run_id, detail)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--repo", default=".")
    parser.add_argument("--threshold-hours", type=float, default=None)
    args = parser.parse_args(argv)
    try:
        wait = measure(pathlib.Path(args.repo))
    except (subprocess.CalledProcessError, ValueError, OSError) as exc:
        print(f"deploy-latency: unusable: {exc}", file=sys.stderr)
        return 2
    if args.json:
        print(json.dumps(asdict(wait)))
    else:
        print(f"{wait.state} {wait.age_hours}h {wait.commit} run={wait.run_id} {wait.detail} — {wait.subject}".strip())
    late = args.threshold_hours is not None and wait.state != "deployed" and wait.age_hours > args.threshold_hours
    return 1 if late else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
