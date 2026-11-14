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

A deployable commit is one that touches .github/deploy-paths; it is
deployed once the latest `chore(deploy): pin platform images to <sha>` commit
names a descendant of it. w6/087 recorded deploy starvation with a ">12 h
means reopen" rule nobody checked, and gaps of 15.7 h and 24.9 h went by
unnoticed (w6/m156). This reports the wait and classifies its cause from the
deploy runs that carry the commit:

  deployed               nothing is waiting
  not-triggered          no deploy run includes the commit yet
  queued                 a run waits for the bex-production-deploy slot
  waiting-for-runner     a job has waited over 5 min for a runner
  in-progress            a run is executing (its running jobs and minutes named)
  failed                 the newest run carrying it failed or timed out (job/step named)
  cancelled              the newest run carrying it was cancelled while pending
  superseded             the newest run succeeded but did not pin (superseded)

Usage: deploy_latency.py [--json] [--repo DIR] [--threshold-hours H]
       deploy_latency.py --changed [--repo DIR]

Exit 0 when nothing has waited longer than H hours (default: no threshold),
1 when the oldest deployable commit has, 2 when the inputs are unusable.

--changed is deploy.yml's gate (block-eden-mono w1/m25): git only, no API.
Exit 0 when a deployable commit is waiting or no pin is in reach (deploy), 1
when production already runs every deployable commit (skip), 2 when git
cannot answer. With --min-pin-age-hours H it also skips while the production
pin is younger than H hours, so push-triggered runs deploy at most every H
hours.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import pathlib
import re
import subprocess
import sys
import time
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


DEPLOY_PATHS = ".github/deploy-paths"


def deploy_paths(text: str) -> tuple[list[str], list[str]]:
    """.github/deploy-paths, split into include and `!` exclude globs."""
    include, exclude = [], []
    for line in text.splitlines():
        glob = line.strip()
        if not glob or glob.startswith("#"):
            continue
        (exclude if glob.startswith("!") else include).append(glob.lstrip("!"))
    if not include:
        raise ValueError(f"{DEPLOY_PATHS} lists no deployable path")
    return include, exclude


def parse_time(value: str) -> dt.datetime:
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


FAILED = {"failure", "timed_out", "startup_failure"}


def classify(runs: list[dict], jobs, now: dt.datetime) -> tuple[str, int, str]:
    """The waiting commit's state from the deploy runs that carry it, newest first.

    jobs maps a run id to its jobs; a missing id reads as no jobs."""
    if not runs:
        return "not-triggered", 0, ""
    active = [r for r in runs if r.get("status") in ACTIVE]
    # The newest carrying run is usually the one pending behind the slot; the
    # run that holds the slot is older, so look at every active run.
    for run in active:
        for job in jobs.get(run["databaseId"], []):
            if job.get("status") == "queued" and not job.get("runner_name") and job.get("created_at"):
                waited = now - parse_time(job["created_at"])
                if waited > RUNNER_WAIT:
                    return "waiting-for-runner", run["databaseId"], f"{job.get('name')} queued {int(waited.total_seconds() // 60)} min"
    for run in active:
        if run.get("status") == "in_progress":
            running = [j for j in jobs.get(run["databaseId"], []) if j.get("status") == "in_progress" and j.get("started_at")]
            detail = ", ".join(f"{j.get('name')} running {int((now - parse_time(j['started_at'])).total_seconds() // 60)} min" for j in running)
            return "in-progress", run["databaseId"], detail
    if active:
        run = active[0]
        if run.get("status") == "waiting":
            return "queued", run["databaseId"], "waiting on an environment protection rule"
        return "queued", run["databaseId"], "waiting for the bex-production-deploy slot"
    run = runs[0]
    run_id = run["databaseId"]
    conclusion = run.get("conclusion")
    if conclusion in FAILED:
        for job in jobs.get(run_id, []):
            if job.get("conclusion") in FAILED:
                steps = [s.get("name", "") for s in job.get("steps", []) if s.get("conclusion") in FAILED]
                return "failed", run_id, f"{job.get('name')}: {', '.join(steps) or conclusion}"
        return "failed", run_id, conclusion
    if conclusion == "cancelled":
        return "cancelled", run_id, "cancelled while pending or superseded in the queue"
    if conclusion == "success":
        return "superseded", run_id, "the run succeeded without pinning this commit"
    return conclusion or "unknown", run_id, ""


class RunJobs(dict):
    """Run id → jobs, fetched from the API only when classify asks."""

    def get(self, run_id, default=None):
        if run_id not in self:
            data = gh_json("api", f"repos/{{owner}}/{{repo}}/actions/runs/{run_id}/jobs")
            self[run_id] = (data or {}).get("jobs", [])
        return self[run_id]


def git(repo: pathlib.Path, *args: str) -> str:
    return subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True, text=True).stdout.strip()


def gh_json(*args: str):
    # Three attempts, as scripts/lib/gh-retry.sh: one api.github.com TLS
    # timeout would otherwise make the hourly watchdog unusable (exit 2).
    delay = float(os.environ.get("GH_RETRY_DELAY", "10"))
    for attempt in range(3):
        try:
            out = subprocess.run(["gh", *args], check=True, capture_output=True, text=True).stdout
            return json.loads(out or "null")
        except subprocess.CalledProcessError:
            if attempt == 2:
                raise
            time.sleep(delay * (attempt + 1))


HEAD = "origin/main"


def pin(repo: pathlib.Path) -> tuple[str, int]:
    """The production pin's producer commit and the pin commit's time (epoch
    seconds). ValueError when no pin is in reach."""
    pin_line = git(repo, "log", "-1", "--format=%ct %s", "--grep=^chore(deploy): pin platform images to ", HEAD)
    pinned_at, _, subject = pin_line.partition(" ")
    match = PIN_SUBJECT.match(subject) if pin_line else None
    if not match:
        raise ValueError("no pin commit on origin/main")
    return git(repo, "rev-parse", match.group(1)), int(pinned_at)


def pending(repo: pathlib.Path) -> list[list[str]]:
    """Deployable commits after the production pin's producer, oldest first,
    as [sha, commit time, subject]. ValueError when no pin is in reach."""
    pinned, _ = pin(repo)
    include, exclude = deploy_paths((repo / DEPLOY_PATHS).read_text())
    # glob magic: `*` stops at `/`, as in GitHub's path filters.
    pathspec = [f":(glob){g}" for g in include] + [f":(exclude,glob){g}" for g in exclude]
    log = git(repo, "log", "--reverse", "--no-merges", "--format=%H%x09%ct%x09%s", f"{pinned}..{HEAD}", "--", *pathspec)
    entries = [line.split("\t", 2) for line in log.splitlines()]
    # The pin write-back itself edits deploy/gitops; it is not new work.
    return [entry for entry in entries if not PIN_SUBJECT.match(entry[2])]


def changed(repo: pathlib.Path, min_pin_age_hours: float = 0, now: float | None = None) -> int:
    """deploy.yml's gate: 0 deploy, 1 skip, 2 unusable."""
    try:
        waiting = pending(repo)
        _, pinned_at = pin(repo)
    except ValueError as exc:
        print(f"deploy-latency: {exc}; deploying")
        return 0
    except (subprocess.CalledProcessError, OSError) as exc:
        print(f"deploy-latency: unusable: {exc}", file=sys.stderr)
        return 2
    if not waiting:
        print("deploy-latency: production runs every deployable commit; nothing to deploy")
        return 1
    print(f"deploy-latency: {len(waiting)} deployable commit(s) since the pin, oldest {waiting[0][0][:12]} — {waiting[0][2]}")
    age_hours = ((time.time() if now is None else now) - pinned_at) / 3600
    if age_hours < min_pin_age_hours:
        print(f"deploy-latency: the production pin is {age_hours:.1f} h old, under {min_pin_age_hours:g} h; the next run deploys them")
        return 1
    return 0


def measure(repo: pathlib.Path, now: dt.datetime | None = None) -> Wait:
    now = now or dt.datetime.now(dt.timezone.utc)
    head = HEAD
    waiting = pending(repo)
    if not waiting:
        return Wait("deployed")
    oldest, committed_at, subject = waiting[0]
    committed = dt.datetime.fromtimestamp(int(committed_at), dt.timezone.utc)
    carrier_shas = set(git(repo, "rev-list", "--ancestry-path", f"{oldest}..{head}").split()) | {oldest}
    # No --branch: that filter goes through a lagging search and can miss the
    # newest runs (as in ci-red-streak.sh), and the listing order is not a
    # contract, so filter and sort here.
    runs = gh_json("run", "list", "--workflow=deploy.yml", "--limit", "60",
                   "--json", "databaseId,headSha,headBranch,status,conclusion,createdAt") or []
    runs = sorted((r for r in runs if r.get("headBranch") == "main"), key=lambda r: r["createdAt"], reverse=True)
    carrying = [r for r in runs if r["headSha"] in carrier_shas]
    state, run_id, detail = classify(carrying, RunJobs(), now)
    return Wait(state, oldest[:12], subject, round((now - committed).total_seconds() / 3600, 1), run_id, detail)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--repo", default=".")
    parser.add_argument("--threshold-hours", type=float, default=None)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--changed", action="store_true", help="exit 0 to deploy, 1 to skip (deploy.yml's gate)")
    mode.add_argument("--pinned", action="store_true", help="print the production pin's producer commit")
    parser.add_argument("--min-pin-age-hours", type=float, default=0, help="with --changed, also skip while the pin is younger")
    args = parser.parse_args(argv)
    repo = pathlib.Path(args.repo)
    if args.changed:
        return changed(repo, args.min_pin_age_hours)
    try:
        if args.pinned:
            pinned, _ = pin(repo)
            print(pinned)
            return 0
        wait = measure(repo)
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
