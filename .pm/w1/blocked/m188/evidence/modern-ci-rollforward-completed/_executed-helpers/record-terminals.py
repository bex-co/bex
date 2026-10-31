#!/usr/bin/env python3
"""Read-only, add-only terminal confirmations; never retry a cancellation."""
import datetime
import hashlib
import json
import pathlib
import subprocess

ROOT = pathlib.Path(__file__).resolve().parent
sha = lambda raw: hashlib.sha256(raw).hexdigest()
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
approval = json.loads((ROOT / "root-execution-approval.json").read_text())
targets = {row["tag"]: row for row in approval["exact_targets"]}
status = json.loads(pathlib.Path("/tmp/bex-m188-publisher-status.json").read_text())
observed = {row["run_id"]: row for row in status["publishers"]}

for outcome_path in sorted((ROOT / "handoffs").glob("*/outcome.json")):
    folder = outcome_path.parent
    if (folder / "terminal-confirmation.json").exists():
        continue
    outcome = json.loads(outcome_path.read_text())
    target = targets[outcome["tag"]]
    cached = observed.get(target["old_run_id"], {})
    if cached.get("status") != "completed" or cached.get("conclusion") != "cancelled":
        continue
    attempt = folder / ("terminal-read-" + datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%S%f"))
    attempt.mkdir(exist_ok=False)
    evidence = []
    try:
        records = []
        for name, route in [
            ("run", f"repos/bex-co/bex/actions/runs/{target['old_run_id']}"),
            ("jobs", f"repos/bex-co/bex/actions/runs/{target['old_run_id']}/jobs?per_page=100"),
        ]:
            response = subprocess.run(["gh", "api", route], capture_output=True, timeout=40)
            assert response.returncode == 0, response.stderr.decode(errors="replace")
            path = attempt / (name + ".json")
            path.write_bytes(response.stdout)
            evidence.append({"role": name, "file": str(path), "sha256": sha(response.stdout)})
            records.append(json.loads(response.stdout))
        run, jobs = records
        assert run["head_sha"] == target["old_source"] and run["run_attempt"] == 1
        assert run["status"] == "completed" and run["conclusion"] == "cancelled"
        native = [j for j in jobs["jobs"] if j["name"].startswith("build")]
        assert len(native) == 1 and native[0]["id"] == target["old_job_id"]
        assert native[0]["status"] == "completed" and native[0]["conclusion"] == "cancelled"
        assert native[0].get("runner_id") == 0 and not native[0].get("steps")
        result = {"at": now(), "tag": target["tag"], "old_run_id": target["old_run_id"],
                  "old_job_id": target["old_job_id"], "source": target["old_source"], "run_attempt": 1,
                  "terminal_confirmed": True, "conclusion": "cancelled", "runner_id": 0, "steps_count": 0,
                  "native_completed_at": native[0]["completed_at"], "evidence": evidence,
                  "original_outcome": {"file": str(outcome_path), "sha256": sha(outcome_path.read_bytes())},
                  "cancellation_repeated": False, "remote_mutation_performed": False}
        path = folder / "terminal-confirmation.json"
        assert not path.exists()
        path.write_text(json.dumps(result, indent=2) + "\n")
        print(json.dumps({"tag": target["tag"], "terminal": "cancelled", "receipt_sha256": sha(path.read_bytes())}), flush=True)
    except Exception as error:
        (attempt / "error.json").write_text(json.dumps({"at": now(), "error": str(error),
             "terminal_not_confirmed": True, "no_remote_mutation": True}, indent=2) + "\n")
        print(json.dumps({"tag": target["tag"], "read_error": str(error)}), flush=True)
