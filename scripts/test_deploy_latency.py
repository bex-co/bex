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
"""Tests for deploy_latency.py (w6/m156)."""

import contextlib
import datetime as dt
import importlib.util
import io
import pathlib
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("deploy_latency", HERE / "deploy_latency.py")
dl = importlib.util.module_from_spec(spec)
sys.modules["deploy_latency"] = dl
spec.loader.exec_module(dl)

NOW = dt.datetime(2026, 11, 2, 12, 0, tzinfo=dt.timezone.utc)


def run(run_id, status="completed", conclusion=None):
    return {"databaseId": run_id, "status": status, "conclusion": conclusion}


class Classify(unittest.TestCase):
    def test_no_run_carries_the_commit(self):
        self.assertEqual(dl.classify([], {}, NOW)[0], "not-triggered")

    def test_a_job_waiting_over_five_minutes_for_a_runner(self):
        jobs = {7: [{"name": "build", "status": "queued", "runner_name": None, "created_at": "2026-11-02T11:40:00Z"}]}
        state, run_id, detail = dl.classify([run(7, "in_progress")], jobs, NOW)
        self.assertEqual((state, run_id), ("waiting-for-runner", 7))
        self.assertIn("build queued 20 min", detail)

    def test_a_short_runner_wait_is_still_in_progress(self):
        jobs = {7: [{"name": "build", "status": "queued", "runner_name": None, "created_at": "2026-11-02T11:58:00Z"}]}
        self.assertEqual(dl.classify([run(7, "in_progress")], jobs, NOW)[0], "in-progress")

    def test_a_run_waiting_for_the_concurrency_slot(self):
        state, _, detail = dl.classify([run(7, "pending")], {}, NOW)
        self.assertEqual((state, detail), ("queued", "waiting for the bex-production-deploy slot"))

    def test_a_run_waiting_on_an_environment_gate_names_it(self):
        self.assertEqual(dl.classify([run(7, "waiting")], {}, NOW)[2], "waiting on an environment protection rule")

    def test_the_newest_failure_names_its_job_and_step(self):
        jobs = {9: [{"name": "deploy", "conclusion": "failure",
                     "steps": [{"name": "wait for PROXY-compatible datastore proxies", "conclusion": "failure"}]}]}
        state, _, detail = dl.classify([run(9, conclusion="failure"), run(8, conclusion="success")], jobs, NOW)
        self.assertEqual(state, "failed")
        self.assertEqual(detail, "deploy: wait for PROXY-compatible datastore proxies")

    def test_cancelled_and_superseded(self):
        self.assertEqual(dl.classify([run(9, conclusion="cancelled")], {}, NOW)[0], "cancelled")
        self.assertEqual(dl.classify([run(9, conclusion="success")], {}, NOW)[0], "superseded")

    def test_an_active_run_wins_over_an_older_failure(self):
        self.assertEqual(dl.classify([run(10, "in_progress"), run(9, conclusion="failure")], {}, NOW)[0], "in-progress")

    def test_the_slot_holder_is_inspected_behind_a_pending_run(self):
        # Newest first: run 11 pends behind the slot, run 10 holds it and waits
        # for a runner. The holder's wait is the real cause.
        jobs = {10: [{"name": "build", "status": "queued", "runner_name": None, "created_at": "2026-11-02T11:00:00Z"}]}
        state, run_id, detail = dl.classify([run(11, "pending"), run(10, "in_progress")], jobs, NOW)
        self.assertEqual((state, run_id, detail), ("waiting-for-runner", 10, "build queued 60 min"))

    def test_in_progress_names_the_running_job(self):
        jobs = {10: [{"name": "test-backend", "status": "completed"},
                     {"name": "build", "status": "in_progress", "started_at": "2026-11-02T10:30:00Z"}]}
        self.assertEqual(dl.classify([run(11, "pending"), run(10, "in_progress")], jobs, NOW),
                         ("in-progress", 10, "build running 90 min"))

    def test_a_timeout_is_a_failure(self):
        jobs = {9: [{"name": "build", "conclusion": "timed_out", "steps": [{"name": "sign images + attest SBOM", "conclusion": "timed_out"}]}]}
        self.assertEqual(dl.classify([run(9, conclusion="timed_out")], jobs, NOW),
                         ("failed", 9, "build: sign images + attest SBOM"))


class DeployPaths(unittest.TestCase):
    def test_the_real_deploy_paths(self):
        include, exclude = dl.deploy_paths((HERE.parent / dl.DEPLOY_PATHS).read_text())
        self.assertIn("lego/**", include)
        self.assertIn("dashboard/**", include)
        self.assertIn(".github/workflows/deploy.yml", include)
        self.assertEqual(exclude, ["lego/cli/**"])

    def test_an_empty_list_is_unusable(self):
        with self.assertRaises(ValueError):
            dl.deploy_paths("# only a comment\n\n!lego/cli/**\n")


class Measure(unittest.TestCase):
    """End to end on a throwaway repository with the GitHub API stubbed."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = pathlib.Path(self.tmp.name)
        self.sh("git", "init", "-q", "-b", "main")
        self.sh("git", "config", "user.email", "t@example.com")
        self.sh("git", "config", "user.name", "t")
        (self.repo / ".github").mkdir(parents=True)
        (self.repo / dl.DEPLOY_PATHS).write_text("# fixture\nlego/**\n!lego/cli/**\n")
        self.commit("lego/a.go", "feat: deployed")
        self.deployed = self.head()
        self.commit("deploy.pin", f"chore(deploy): pin platform images to {self.deployed[:12]} [skip ci]")
        self.sh("git", "update-ref", "refs/remotes/origin/main", "HEAD")

    def tearDown(self):
        self.tmp.cleanup()

    def sh(self, *args):
        return subprocess.run(args, cwd=self.repo, check=True, capture_output=True, text=True).stdout.strip()

    def head(self):
        return self.sh("git", "rev-parse", "HEAD")

    def commit(self, path, subject):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(subject)
        self.sh("git", "add", "-A")
        self.sh("git", "commit", "-q", "-m", subject)
        self.sh("git", "update-ref", "refs/remotes/origin/main", "HEAD")

    def test_no_pin_commit_is_unusable_not_late(self):
        fresh = tempfile.TemporaryDirectory()
        self.addCleanup(fresh.cleanup)
        repo = pathlib.Path(fresh.name)
        subprocess.run(["git", "init", "-q", "-b", "main"], cwd=repo, check=True)
        subprocess.run(["git", "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"], cwd=repo, check=True)
        subprocess.run(["git", "update-ref", "refs/remotes/origin/main", "HEAD"], cwd=repo, check=True)
        self.assertEqual(dl.main(["--repo", str(repo), "--threshold-hours", "1"]), 2)

    def test_nothing_waits_after_the_pin(self):
        self.commit("lego/cli/x.go", "feat(cli): launcher only")
        self.commit("docs/x.md", "docs: not deployable")
        with mock.patch.object(dl, "gh_json", side_effect=AssertionError("no API call needed")):
            self.assertEqual(dl.measure(self.repo, NOW).state, "deployed")

    def test_pinned_prints_the_producer_commit(self):
        self.commit("lego/b.go", "fix: waits")
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertEqual(dl.main(["--pinned", "--repo", str(self.repo)]), 0)
        self.assertEqual(out.getvalue().strip(), self.deployed)

    def test_changed_skips_when_production_runs_every_deployable_commit(self):
        self.commit("lego/cli/x.go", "feat(cli): launcher only")
        self.commit("docs/x.md", "docs: not deployable")
        self.assertEqual(dl.main(["--changed", "--repo", str(self.repo)]), 1)

    def test_changed_deploys_a_waiting_commit(self):
        self.commit("lego/b.go", "fix: waits")
        self.assertEqual(dl.main(["--changed", "--repo", str(self.repo)]), 0)

    def test_changed_ignores_the_pin_write_back_itself(self):
        # The write-back edits deployable manifests after the producer it names.
        self.commit("deploy/gitops/base/bex.yaml", f"chore(deploy): pin platform images to {self.deployed[:12]} [skip ci]")
        (self.repo / dl.DEPLOY_PATHS).write_text("lego/**\ndeploy/gitops/**\n")
        self.assertEqual(dl.main(["--changed", "--repo", str(self.repo)]), 1)

    def test_changed_waits_while_the_pin_is_younger_than_the_interval(self):
        self.commit("lego/b.go", "fix: waits for the interval")
        _, pinned_at = dl.pin(self.repo)
        self.assertEqual(dl.changed(self.repo, 6, now=pinned_at + 5 * 3600), 1)
        self.assertEqual(dl.changed(self.repo, 6, now=pinned_at + 6 * 3600), 0)

    def test_an_old_pin_with_nothing_new_still_skips(self):
        _, pinned_at = dl.pin(self.repo)
        self.assertEqual(dl.changed(self.repo, 6, now=pinned_at + 48 * 3600), 1)

    def test_changed_deploys_without_a_pin_in_reach(self):
        fresh = tempfile.TemporaryDirectory()
        self.addCleanup(fresh.cleanup)
        repo = pathlib.Path(fresh.name)
        (repo / ".github").mkdir()
        (repo / dl.DEPLOY_PATHS).write_text("lego/**\n")
        subprocess.run(["git", "init", "-q", "-b", "main"], cwd=repo, check=True)
        subprocess.run(["git", "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"], cwd=repo, check=True)
        subprocess.run(["git", "update-ref", "refs/remotes/origin/main", "HEAD"], cwd=repo, check=True)
        self.assertEqual(dl.main(["--changed", "--repo", str(repo)]), 0)

    def test_changed_is_unusable_when_git_fails(self):
        with mock.patch.object(dl, "git", side_effect=subprocess.CalledProcessError(128, ["git"])):
            self.assertEqual(dl.main(["--changed", "--repo", str(self.repo)]), 2)

    def test_the_oldest_deployable_commit_and_its_cancelled_run(self):
        self.commit("lego/b.go", "fix: waits")
        waiting = self.head()
        self.commit("lego/c.go", "fix: newer")
        newer = self.head()

        def fake_gh(*args):
            if args[0] == "run":
                # Out of order on purpose, plus a branch run that must be ignored.
                return [{"databaseId": 1, "headSha": self.deployed, "headBranch": "main", "status": "completed", "conclusion": "success", "createdAt": "2026-11-02T08:00:00Z"},
                        {"databaseId": 3, "headSha": newer, "headBranch": "topic", "status": "in_progress", "conclusion": None, "createdAt": "2026-11-02T11:00:00Z"},
                        {"databaseId": 2, "headSha": newer, "headBranch": "main", "status": "completed", "conclusion": "cancelled", "createdAt": "2026-11-02T10:00:00Z"}]
            return {"jobs": []}

        with mock.patch.object(dl, "gh_json", side_effect=fake_gh):
            wait = dl.measure(self.repo, NOW)
        self.assertEqual((wait.state, wait.commit, wait.run_id, wait.subject), ("cancelled", waiting[:12], 2, "fix: waits"))


class GhJson(unittest.TestCase):
    def test_transient_gh_failure_is_retried_then_raised(self):
        ok = subprocess.CompletedProcess(["gh"], 0, stdout='{"a": 1}', stderr="")
        fail = subprocess.CalledProcessError(1, ["gh"], stderr="net/http: TLS handshake timeout")
        with mock.patch.dict("os.environ", {"GH_RETRY_DELAY": "0"}):
            with mock.patch.object(dl.subprocess, "run", side_effect=[fail, ok]) as run:
                self.assertEqual(dl.gh_json("api", "x"), {"a": 1})
                self.assertEqual(run.call_count, 2)
            with mock.patch.object(dl.subprocess, "run", side_effect=[fail, fail, fail]) as run:
                with self.assertRaises(subprocess.CalledProcessError):
                    dl.gh_json("api", "x")
                self.assertEqual(run.call_count, 3)


if __name__ == "__main__":
    unittest.main()
