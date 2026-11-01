#!/usr/bin/env python3
"""Read-only Go-only metadata observer; never grants a legacy source role."""
import datetime
import hashlib
import json
import pathlib
import subprocess
import time

ROOT = pathlib.Path(__file__).resolve().parent
RUN = 41142870861
SOURCE = "8af93065a6a83299e8db7977aaf6527a5bad3e5d"
TAG = "golang-1.27-bookworm"
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
sha = lambda raw: hashlib.sha256(raw).hexdigest()
last = {}
previous = None


def get(route):
    response = subprocess.run(["gh", "api", route], capture_output=True, timeout=30)
    assert response.returncode == 0, response.stderr.decode(errors="replace")
    return response.stdout, json.loads(response.stdout)


def freeze(name, raw):
    path = ROOT / name
    assert not path.exists()
    path.write_bytes(raw)
    return {"file": str(path), "sha256": sha(raw), "bytes": len(raw)}


while not (ROOT / "stop-observer").exists():
    try:
        run_raw, run = get(f"repos/bex-co/bex/actions/runs/{RUN}")
        assert run["head_sha"] == SOURCE and run["event"] == "workflow_dispatch"
        assert run["head_branch"] == "main" and run["run_attempt"] == 1
        jobs_raw, jobs = get(f"repos/bex-co/bex/actions/runs/{RUN}/attempts/1/jobs?per_page=100")
        native = [j for j in jobs["jobs"] if j["name"].startswith("build")]
        assert len(native) <= 1
        assert not native or native[0]["name"].startswith("build (" + TAG + ",")
        state = {"observed_at": now(), "run_id": RUN, "source": SOURCE, "run_attempt": 1,
                 "status": run["status"], "conclusion": run["conclusion"],
                 "modern_metadata_only": True, "legacy_source_role_granted": False,
                 "observation_stale": False,
                 "jobs": [{"id": j["id"], "name": j["name"], "status": j["status"],
                           "conclusion": j["conclusion"], "runner_id": j.get("runner_id"),
                           "steps": [{k: step.get(k) for k in ["number", "name", "status", "conclusion", "started_at", "completed_at"]}
                                     for step in j.get("steps", [])]} for j in jobs["jobs"]]}
        if native and not (ROOT / "actual-native-binding.json").exists():
            record = {"at": now(), "run_id": RUN, "source": SOURCE, "run_attempt": 1,
                      "tag": TAG, "job_id": native[0]["id"], "metadata_only": True,
                      "run": freeze("first-native-run.json", run_raw),
                      "jobs": freeze("first-native-jobs.json", jobs_raw)}
            freeze("actual-native-binding.json", json.dumps(record, indent=2).encode() + b"\n")
        if run["status"] == "completed":
            state["completed_run"] = freeze("completed-run.json", run_raw)
            state["completed_jobs"] = freeze("completed-jobs.json", jobs_raw)
        last = state
    except Exception as error:
        state = dict(last, checked_at=now(), run_id=RUN, source=SOURCE,
                     observation_stale=True, error=str(error), modern_metadata_only=True)
    temporary = ROOT / "modern-go-status.new.json"
    temporary.write_text(json.dumps(state, indent=2) + "\n")
    temporary.replace(ROOT / "modern-go-status.json")
    phase = (state.get("status"), state.get("conclusion"),
             [(j["id"], j["status"], [s["name"] for s in j["steps"] if s["status"] == "in_progress"])
              for j in state.get("jobs", [])], state.get("error"))
    if phase != previous:
        print(json.dumps({"at": now(), "modern_go_phase": phase}), flush=True)
        previous = phase
    if state.get("status") == "completed" and not state.get("observation_stale"):
        break
    time.sleep(45)
