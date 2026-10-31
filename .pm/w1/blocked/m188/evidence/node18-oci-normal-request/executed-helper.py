#!/usr/bin/env python3
"""Exactly one root-authorized Node18 normal build at the literal reviewed OCI source."""
import argparse
import concurrent.futures
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

SOURCE = "da0e56e80a1e4eb08cacabd67efde3c768a6ae81"
TAG = "node-18-bookworm"
BASE = "node:18-bookworm@sha256:c6ae79e38498325db67193d391e6ec1d224d96c693a8a4d943498556716d3783"
REPO = "bex-co/bex"
WORKFLOW = ".github/workflows/native-base-image.yml"
ADMISSION = Path("/tmp/bex-m188-da0e-root-source-admission.json")
ADMISSION_SHA = "28630810ac0b5ee6eb669660763ff7fae6c28e0c8598b38d66b72e8e9957bcce"
INSTALL_SHA = "fe880434bb8f428500be52cd626534f69c9ae089d162e3549a50a769b09e5462"


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def frozen(path, value):
    with path.open("x") as stream:
        json.dump(value, stream, indent=2)
        stream.write("\n")


def git(*args):
    return subprocess.check_output(["git", *args], stderr=subprocess.PIPE)


def source_proof(output):
    admission_raw = ADMISSION.read_bytes()
    assert sha(admission_raw) == ADMISSION_SHA
    admission = json.loads(admission_raw)
    assert admission["source"] == SOURCE and admission["admitted"] is True
    assert set(admission["roles"]) == {"producer:all11", "verification:all11"}
    sources = {}
    for path, expected in admission["publisher_profile"].items():
        raw = git("show", SOURCE + ":" + path)
        assert sha(raw) == expected
        sources[path] = {"sha256": sha(raw), "bytes": len(raw)}
    workflow = git("show", SOURCE + ":" + WORKFLOW).decode()
    install = git("show", SOURCE + ":deploy/native-base/install.sh")
    assert sha(install) == INSTALL_SHA
    catalog_raw = git("show", SOURCE + ":.github/native-base-recovery.json")
    entry = json.loads(catalog_raw)["images"][TAG]
    assert entry["base"] == BASE
    changed = git("diff", "--no-renames", "--name-only", entry["producer"], SOURCE, "--", "deploy/native-base").decode().splitlines()
    assert "deploy/native-base/install.sh" in changed and any(not path.endswith(".md") for path in changed)
    lines = workflow.splitlines()
    start_step = next(i for i, line in enumerate(lines) if "name: select a reviewed recovery artifact" in line)
    start = next(i for i in range(start_step, len(lines)) if lines[i].strip() == "python3 - <<'PY'") + 1
    end = next(i for i in range(start, len(lines)) if lines[i].strip() == "PY")
    selector = "\n".join(line[10:] for line in lines[start:end]) + "\n"
    # Read actual immutable producer/HEAD objects in an isolated temporary Git
    # directory; no shared checkout/catalog/ref edits and no network fetch.
    with tempfile.TemporaryDirectory(prefix="bex-node18-actual-selector-") as temporary:
        root = Path(temporary)
        isolated = root / "objects.git"
        subprocess.run(["git", "init", "--bare", "--quiet", str(isolated)], check=True, capture_output=True)
        objects = Path(git("rev-parse", "--git-path", "objects").decode().strip()).resolve()
        (isolated / "objects/info/alternates").write_text(str(objects) + "\n")
        subprocess.run(["git", "--git-dir=" + str(isolated), "symbolic-ref", "HEAD", "refs/heads/dispatch-source"], check=True, capture_output=True)
        subprocess.run(["git", "--git-dir=" + str(isolated), "update-ref", "refs/heads/dispatch-source", SOURCE], check=True, capture_output=True)
        (root / ".github").mkdir()
        (root / ".github/native-base-recovery.json").write_bytes(catalog_raw)
        selected = root / "outputs"
        selected.write_bytes(b"")
        env = dict(os.environ, GIT_DIR=str(isolated), GIT_WORK_TREE=str(root), MATRIX_NAME=TAG, BASE_IMAGE=BASE, NATIVE_IMAGE="ghcr.io/bex-co/bex-cnb-run", GITHUB_OUTPUT=str(selected))
        result = subprocess.run(["python3", "-B", "-c", selector], cwd=root, env=env, capture_output=True, timeout=30)
        assert result.returncode == 0 and selected.read_bytes() == b""
        assert b"Recovery recipe changed; building from source" in result.stdout
        selector_result = {"exit_code": result.returncode, "recovery_output_bytes": 0, "stdout": result.stdout.decode(), "stderr": result.stderr.decode(), "actual_immutable_selector_sha256": sha(selector.encode()), "isolated_git_objects_from_shared_repository_read_only": True}
    receipt = {"at": now(), "literal_source": SOURCE, "runtime": TAG, "base": BASE, "root_source_admission_sha256": ADMISSION_SHA, "publisher_profile": sources, "install_sha256": sha(install), "catalog_sha256": sha(catalog_raw), "historical_recovery_entry": entry, "producer_to_dispatch_recipe_changes": changed, "selector_result": selector_result, "normal_build_required": True, "recovery_catalog_edit": False}
    if output:
        frozen(output / "source-and-normal-selector-proof.json", receipt)
    return receipt


class Dispatch:
    def __init__(self, output):
        self.output = output

    def api(self, endpoint, label):
        started = now()
        process = subprocess.run(["gh", "api", endpoint], capture_output=True, timeout=45)
        raw = self.output / (label + ".raw.json")
        with raw.open("xb") as stream:
            stream.write(process.stdout)
        frozen(self.output / (label + ".request.json"), {"started_at": started, "finished_at": now(), "endpoint": endpoint, "exit_code": process.returncode, "stderr": process.stderr.decode(errors="replace"), "raw_sha256": sha(process.stdout)})
        assert process.returncode == 0, "read-only API failed; no blind dispatch retry"
        return json.loads(process.stdout)

    def active_jobs(self, listed):
        run = self.api(f"repos/{REPO}/actions/runs/{listed['id']}", f"dedupe-{listed['id']}-run")
        if run["status"] == "completed":
            return {"run_id": run["id"], "status": "completed", "duplicate": False}
        assert run["path"] == WORKFLOW and run["head_branch"] == "main"
        document = self.api(f"repos/{REPO}/actions/runs/{run['id']}/attempts/{run['run_attempt']}/jobs?per_page=100", f"dedupe-{run['id']}-jobs")
        jobs = document["jobs"]
        assert document["total_count"] == len(jobs) and len(jobs) <= 100
        native = [job for job in jobs if job["name"].startswith("build (")]
        assert native, "nonterminal workflow input unknown; preserve and stop"
        assert all(job["head_sha"] == run["head_sha"] and job["run_attempt"] == run["run_attempt"] for job in jobs)
        target = [job for job in native if job["name"].startswith("build (" + TAG + ",") and job["status"] != "completed"]
        return {"run_id": run["id"], "source": run["head_sha"], "event": run["event"], "attempt": run["run_attempt"], "native_jobs": [{"id": job["id"], "name": job["name"], "status": job["status"]} for job in native], "duplicate": bool(target)}

    def execute(self):
        initial = self.api(f"repos/{REPO}/git/ref/heads/main", "initial-main")["object"]["sha"]
        if initial != SOURCE:
            frozen(self.output / "head-changed-stop.json", {"at": now(), "expected": SOURCE, "actual": initial, "dispatch_posts": 0})
            raise SystemExit("main changed; no POST; root source review required")
        source_proof(self.output)
        runs = {}
        page = 1
        while True:
            document = self.api(f"repos/{REPO}/actions/workflows/native-base-image.yml/runs?branch=main&per_page=100&page={page}", f"all-source-runs-{page}")
            assert document["total_count"] <= 1000, "history exceeds bounded complete listing; stop"
            rows = document["workflow_runs"]
            runs.update({run["id"]: run for run in rows})
            if len(runs) >= document["total_count"]:
                break
            assert rows, "incomplete all-source listing"
            page += 1
        known = json.loads(Path("/tmp/bex-m188-known-publishers.json").read_text())
        known.update({"41142870861": None, "41144656779": None})
        for run_id in known:
            if int(run_id) not in runs:
                run = self.api(f"repos/{REPO}/actions/runs/{run_id}", "buried-run-" + run_id)
                runs[run["id"]] = run
        active = [run for run in runs.values() if run["status"] != "completed"]
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
            observations = list(pool.map(self.active_jobs, active))
        duplicates = [run for run in observations if run["duplicate"]]
        frozen(self.output / "all-source-dedupe.json", {"at": now(), "run_count": len(runs), "nonterminal_observations": observations, "duplicates": duplicates})
        assert not duplicates, "existing Node18 request; reuse, no POST"
        head = self.api(f"repos/{REPO}/git/ref/heads/main", "immediate-prepost-main")["object"]["sha"]
        if head != SOURCE:
            frozen(self.output / "head-changed-stop.json", {"at": now(), "expected": SOURCE, "actual": head, "dispatch_posts": 0})
            raise SystemExit("main changed before POST; root source review required")
        payload = self.output / "dispatch-input.json"
        frozen(payload, {"ref": "main", "inputs": {"runtime": TAG}})
        # The exclusive on-disk intent consumes the entire one-POST budget.
        frozen(self.output / "dispatch-intent.json", {"at": now(), "source": SOURCE, "runtime": TAG, "max_dispatch_posts": 1, "normal_build_required": True, "no_uncertain_retry": True})
        began = now()
        try:
            process = subprocess.run(["gh", "api", "--include", "--method", "POST", f"repos/{REPO}/actions/workflows/native-base-image.yml/dispatches", "--input", str(payload)], capture_output=True, timeout=45)
        except subprocess.TimeoutExpired as error:
            frozen(self.output / "dispatch-uncertain.json", {"at": now(), "source": SOURCE, "potential_post_sent": True, "no_retry": True, "stdout_sha256": sha(error.stdout or b""), "stderr_sha256": sha(error.stderr or b"")})
            raise
        result = {"started_at": began, "finished_at": now(), "requested_source": SOURCE, "runtime": TAG, "dispatch_posts": 1, "exit_code": process.returncode, "stdout": process.stdout.decode(errors="replace"), "stderr": process.stderr.decode(errors="replace"), "actual_run_head_pending": True, "normal_build_required": True, "no_cancel_or_retry": True}
        frozen(self.output / "dispatch-result.json", result)
        print(json.dumps(result), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=Path("/tmp/bex-m188-node18-oci-normal-dispatch"))
    parser.add_argument("--authorization", type=Path)
    parser.add_argument("--execute", action="store_true")
    parser.add_argument("--check-local", action="store_true")
    args = parser.parse_args()
    if not args.execute:
        proof = source_proof(None) if args.check_local else None
        print(json.dumps({"prepared_only": True, "remote_calls": 0, "runtime": TAG, "source": SOURCE, "local_selector_proof": proof}))
        return
    assert args.authorization, "exact root dispatch approval required after source activation"
    approval = json.loads(args.authorization.read_text())
    assert approval["authorized"] is True and approval["source"] == SOURCE and approval["runtime"] == TAG and approval["max_dispatch_posts"] == 1
    args.output.mkdir(mode=0o700, exist_ok=False)
    (args.output / "authorization.json").write_bytes(args.authorization.read_bytes())
    (args.output / "executed-helper.py").write_bytes(Path(__file__).read_bytes())
    Dispatch(args.output).execute()


if __name__ == "__main__":
    main()
