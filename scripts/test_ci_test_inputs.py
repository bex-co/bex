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
"""Tests for ci_test_inputs.py: each suite's trigger paths cover what its tests read."""

import contextlib
import importlib.util
import io
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("ci_test_inputs", HERE / "ci_test_inputs.py")
cti = importlib.util.module_from_spec(spec)
sys.modules["ci_test_inputs"] = cti
spec.loader.exec_module(cti)

WORKFLOW = ".github/workflows/dashboard-test.yml"


def workflow(paths, push_paths=None):
    def block(items):
        return "".join(f'      - "{p}"\n' for p in items)
    return ("name: test (dashboard)\non:\n  pull_request:\n    paths:\n" + block(paths)
            + "      # a comment\n  push:\n    branches: [main]\n    paths:\n"
            + block(paths if push_paths is None else push_paths)
            + "  workflow_call: {}\n\npermissions:\n  contents: read\n")


class Globs(unittest.TestCase):
    def test_github_filter_semantics(self):
        self.assertTrue(cti.covered("dashboard/src/a.ts", ["dashboard/**"]))
        self.assertFalse(cti.covered("lego/a.go", ["dashboard/**"]))
        self.assertTrue(cti.covered("docs/x/a.json", ["docs/x/*.json"]))
        self.assertFalse(cti.covered("docs/x/y/a.json", ["docs/x/*.json"]), "* stops at /")
        self.assertTrue(cti.covered("a/b/c.md", ["**/*.md"]))
        self.assertTrue(cti.covered("c.md", ["**/*.md"]), "**/ matches no directory too")

    def test_a_later_negation_excludes(self):
        self.assertFalse(cti.covered("lego/cli/a.go", ["lego/**", "!lego/cli/**"]))
        self.assertTrue(cti.covered("lego/cli/a.go", ["!lego/cli/**", "lego/**"]))

    def test_quantifiers_are_refused(self):
        with self.assertRaises(ValueError):
            cti.covered("a", ["a?"])


class Filters(unittest.TestCase):
    def test_paths_per_event(self):
        got = cti.path_filters(workflow(["dashboard/**", "x.json"]))
        self.assertEqual(got, {"pull_request": ["dashboard/**", "x.json"], "push": ["dashboard/**", "x.json"]})

    def test_an_unfiltered_event_is_none_and_an_absent_one_missing(self):
        # references.yml's push runs on every change; s3-contract.yml has no
        # pull_request at all, only a schedule beside its push.
        text = "on:\n  push:\n    branches: [main]\n  schedule:\n    - cron: '0 0 * * 1'\n"
        self.assertEqual(cti.path_filters(text), {"push": None})

    def test_paths_ignore_is_refused(self):
        with self.assertRaises(ValueError):
            cti.path_filters("on:\n  push:\n    paths-ignore:\n      - docs/**\n")


class Literals(unittest.TestCase):
    def test_strings_joins_and_placeholders(self):
        src = '''
        // "lego/commented/out.go" is ignored
        a := "../../../../deploy/x.yaml"
        b := filepath.Join("..", "..", "docs", "ADR1.md")
        c := filepath.Join(repoRoot(t), "examples", "y.yaml")
        d := filepath.Join("..", name, "z")
        e := `${REPO_ROOT}/lego/backend/k.go`
        '''
        got = cti.literals(src)
        self.assertIn("../../../../deploy/x.yaml", got)
        self.assertIn("../../docs/ADR1.md", got)
        self.assertIn("examples/y.yaml", got)
        self.assertIn(cti.HOLE + "/lego/backend/k.go", got)
        self.assertNotIn("lego/commented/out.go", got)
        self.assertFalse(any(v.endswith("/z") for v in got), "a variable mid-join makes the path unknowable")


class Shell(unittest.TestCase):
    def test_words_reads_and_runs(self):
        src = """
        # bash scripts/commented.sh
        bash scripts/a.sh --flag
        . "$here/lib/b.sh"
        bash -n scripts/c.sh
        grep -qF 'bash scripts/needle.sh' deploy/x.yaml
        out=$(python3 -B -m unittest scripts/t1.py scripts/t2.py -v)
        runner="$repo_root/scripts/d.sh"
        bash "$runner" --check
        echo "see docs/e.md" >&2
        """
        got = cti.shell_words(src)
        for word in ["scripts/a.sh", cti.HOLE + "/lib/b.sh", "scripts/t1.py", "scripts/t2.py",
                     cti.HOLE + "/scripts/d.sh"]:
            self.assertIn((word, True), got)
        for word in ["scripts/c.sh", "scripts/needle.sh", "deploy/x.yaml", "docs/e.md"]:
            self.assertIn((word, False), got, "named, so read, but not run")
        self.assertFalse(any("commented" in word for word, _ in got))

    def test_a_workflow_trigger_filter_is_no_read_and_a_folded_run_is_one_line(self):
        text = ("on:\n  push:\n    paths:\n      - \"!docs/old/**\"\n      - \"scripts/x.sh\"\n"
                "jobs:\n  t:\n    steps:\n      - run: >-\n          shellcheck scripts/a.sh\n"
                "          scripts/b.sh\n      - run: bash scripts/c.sh\n")
        got = cti.shell_words(cti._jobs(text))
        self.assertNotIn(("scripts/x.sh", False), got, "a trigger path is not read")
        self.assertNotIn(("scripts/x.sh", True), got)
        for word in ["scripts/a.sh", "scripts/b.sh"]:
            self.assertIn((word, False), got, "shellcheck reads its arguments; it runs none")
        self.assertIn(("scripts/c.sh", True), got)

    def test_paths_resolve_holes_and_repository_globs(self):
        tracked = {"scripts/lib/b.sh", "scripts/x.sh", "deploy/a.yaml", "deploy/b.yaml", "README.md"}
        self.assertEqual(cti.shell_paths(cti.HOLE + "/lib/b.sh", "scripts/t.sh", tracked), ["scripts/lib/b.sh"])
        self.assertEqual(cti.shell_paths(cti.HOLE + "/x.sh", "scripts/t.sh", tracked), ["scripts/x.sh"])
        self.assertEqual(cti.shell_paths("deploy/*.yaml", "scripts/t.sh", tracked), ["deploy/a.yaml", "deploy/b.yaml"])
        # `deploy/**` is a trigger filter or a checker's pattern: no script
        # sets globstar, so none expands it.
        for scratch in ["*", "*/*", cti.HOLE + "/deploy/*.yaml", "../x.sh", "deploy/**"]:
            self.assertEqual(cti.shell_paths(scratch, "scripts/t.sh", tracked), [], scratch)


class Throwaway(unittest.TestCase):
    """A throwaway repository to check end to end."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = pathlib.Path(self.tmp.name)
        subprocess.run(["git", "init", "-q"], cwd=self.repo, check=True)

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, path, text):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def tracked(self):
        subprocess.run(["git", "add", "-A"], cwd=self.repo, check=True)
        return set(subprocess.run(["git", "ls-files"], cwd=self.repo, capture_output=True, text=True,
                                  check=True).stdout.split())


class Repo(Throwaway):
    def setUp(self):
        super().setUp()
        self.write("dashboard/package.json", "{}")
        self.write("lego/backend/pricing.yaml", "x")
        self.write("lego/backend/fixture.go", "x")
        self.write("dashboard/src/__tests__/pricing.test.ts",
                   'read(`${process.cwd()}/../lego/backend/pricing.yaml`);\n'
                   'const sample = "lego/backend/fixture.go";\n'
                   'const filter = "lego/**/*.go";\n')

    def run_check(self, ignored=None):
        tracked = self.tracked()
        suite = cti.Suite(("dashboard",), (".test.ts",), ignored or {})
        with mock.patch.dict(cti.SUITES, {WORKFLOW: suite}):
            return cti.check(self.repo, WORKFLOW, tracked)

    def test_a_read_outside_the_tree_must_be_listed(self):
        self.write(WORKFLOW, workflow(["dashboard/**", WORKFLOW]))
        problems = self.run_check()
        self.assertTrue(any("miss lego/backend/pricing.yaml" in p for p in problems), problems)
        self.assertFalse(any("lego/**/*.go" in p for p in problems), "a bare glob is a fixture")

    def test_listed_reads_and_ignored_fixtures_pass(self):
        self.write(WORKFLOW, workflow(["dashboard/**", "lego/backend/pricing.yaml", WORKFLOW]))
        ignored = {"lego/backend/fixture.go": "a sample path"}
        self.assertEqual(self.run_check(ignored), [])

    def test_push_must_match_pull_request(self):
        self.write(WORKFLOW, workflow(["dashboard/**", "lego/backend/**", WORKFLOW], ["dashboard/**", WORKFLOW]))
        problems = self.run_check({"lego/backend/fixture.go": "x"})
        self.assertIn(f"{WORKFLOW}: push and pull_request paths differ; keep one list in both", problems)

    def test_a_stale_ignore_fails(self):
        self.write(WORKFLOW, workflow(["dashboard/**", "lego/backend/**", WORKFLOW]))
        problems = self.run_check({"lego/backend/gone.go": "x"})
        self.assertTrue(any("ignored lists lego/backend/gone.go" in p for p in problems), problems)

    def test_a_board_file_a_test_names_is_never_a_read(self):
        # No suite checks the board out, and an item moves between blocked/,
        # the queue and done/: the same literal is tracked one day and not
        # the next, so it can be neither a missing path nor an ignored one.
        self.write(".pm/w8/m91/README.md", "x")
        self.write("dashboard/src/__tests__/paths.test.ts", 'const sample = ".pm/w8/m91/README.md";\n')
        self.write(WORKFLOW, workflow(["dashboard/**", "lego/backend/pricing.yaml", WORKFLOW]))
        ignored = {"lego/backend/fixture.go": "a sample path"}
        self.assertEqual(self.run_check(ignored), [])
        stale = self.run_check({**ignored, ".pm/w8/m91/README.md": "a sample path"})
        self.assertTrue(any("ignored lists .pm/w8/m91/README.md" in p for p in stale), stale)

    def test_the_own_tree_and_workflow_must_be_listed(self):
        self.write(WORKFLOW, workflow(["lego/backend/**"]))
        problems = self.run_check({"lego/backend/fixture.go": "x"})
        self.assertIn(f"{WORKFLOW}: paths miss the suite's own tree dashboard/**", problems)
        self.assertIn(f"{WORKFLOW}: paths miss the workflow file itself", problems)
        self.write(WORKFLOW, workflow(["dashboard/**", "lego/backend/pricing.yaml", ".github/**"]))
        self.assertEqual(self.run_check({"lego/backend/fixture.go": "x"}), [], "a glob lists the workflow too")

    def test_only_a_filtered_event_can_miss_a_read(self):
        ignored = {"lego/backend/fixture.go": "a sample path"}
        listed = '    paths:\n      - "dashboard/**"\n      - "' + WORKFLOW + '"\n'
        # references.yml: push runs on every change, pull_request is filtered.
        self.write(WORKFLOW, "on:\n  pull_request:\n" + listed + "  push:\n    branches: [main]\n")
        self.assertEqual(self.run_check(ignored), [f"{WORKFLOW}: paths miss lego/backend/pricing.yaml, "
                                                   "read by dashboard/src/__tests__/pricing.test.ts"])
        # s3-contract.yml: push only, filtered.
        self.write(WORKFLOW, "on:\n  workflow_dispatch:\n  push:\n    branches: [main]\n" + listed)
        self.assertEqual(len(self.run_check(ignored)), 1)
        # Every event unfiltered: nothing can be missed, but a stale ignore still fails.
        self.write(WORKFLOW, "on:\n  pull_request:\n  push:\n    branches: [main]\n")
        self.assertEqual(self.run_check(ignored), [])
        self.assertEqual(self.run_check({**ignored, "lego/backend/gone.go": "x"}),
                         [f"{WORKFLOW}: ignored lists lego/backend/gone.go, which no test names any more; remove it"])
        self.write(WORKFLOW, "on:\n  schedule:\n    - cron: '0 0 * * 1'\n")
        self.assertEqual(self.run_check(ignored), [f"{WORKFLOW}: runs on neither push nor pull_request"])


class RunsRepo(Throwaway):
    """A suite whose tests are the scripts its workflow runs."""

    WORKFLOW = ".github/workflows/gitops.yml"
    LISTED = ["deploy/gitops/**", WORKFLOW, "scripts/validate.sh", "scripts/guard.sh", "infra/policy.json",
              "docs/runbook.md", "scripts/live-check.sh"]

    def setUp(self):
        super().setUp()
        self.write("deploy/gitops/base/a.yaml", "x")
        self.write("scripts/validate.sh", "bash scripts/guard.sh\n"
                   "grep -qF 'bash scripts/needle.sh' docs/runbook.md\n"
                   "grep -qF 'live scope' scripts/live-check.sh\n")
        self.write("scripts/guard.sh", 'here="$(dirname "$0")"\nyq . "$here/../infra/policy.json"\n')
        self.write("infra/policy.json", "{}")
        self.write("docs/runbook.md", "x")
        # Named only as a needle or a grep target: what they read is no input.
        self.write("scripts/needle.sh", "cat lego/never.go\n")
        self.write("scripts/live-check.sh", "cat lego/live-only.go\n")
        self.write("lego/never.go", "x")
        self.write("lego/live-only.go", "x")

    def run_check(self, ignored=None):
        tracked = self.tracked()
        suite = cti.Suite(("deploy/gitops",), (), ignored or {}, runs=True)
        with mock.patch.dict(cti.SUITES, {self.WORKFLOW: suite}, clear=True):
            return cti.check(self.repo, self.WORKFLOW, tracked)

    def steps(self, paths):
        text = workflow(paths).replace("permissions:", "jobs:\n  render:\n    steps:\n"
                                       "      - run: bash scripts/validate.sh\n\npermissions:")
        self.write(self.WORKFLOW, text)

    def test_the_scripts_run_and_what_they_read_pass_when_listed(self):
        self.steps(self.LISTED)
        self.assertEqual(self.run_check({"scripts/needle.sh": "a needle"}), [])

    def test_a_grepped_script_and_a_transitive_read_must_be_listed(self):
        self.steps([p for p in self.LISTED if p not in ("scripts/live-check.sh", "infra/policy.json")])
        problems = self.run_check({"scripts/needle.sh": "a needle"})
        self.assertIn(f"{self.WORKFLOW}: paths miss infra/policy.json, read by scripts/guard.sh", problems)
        self.assertIn(f"{self.WORKFLOW}: paths miss scripts/live-check.sh, read by scripts/validate.sh", problems)
        self.assertEqual(len(problems), 2, problems)

    def test_an_ignored_script_is_never_run(self):
        # A needle names a script: what that script reads is no input either.
        self.steps(self.LISTED)
        self.write("scripts/validate.sh", "bash scripts/guard.sh\nbash scripts/needle.sh --example\n"
                   "grep -qF 'bash scripts/needle.sh' docs/runbook.md\ngrep -qF 'live scope' scripts/live-check.sh\n")
        self.assertEqual(self.run_check({"scripts/needle.sh": "named in an example"}), [])
        followed = self.run_check()
        self.assertIn(f"{self.WORKFLOW}: paths miss lego/never.go, read by scripts/needle.sh", followed)

    def test_an_ignore_the_paths_already_cover_fails(self):
        self.steps(self.LISTED)
        problems = self.run_check({"scripts/needle.sh": "a needle", "docs/runbook.md": "a message"})
        self.assertEqual(problems, [f"{self.WORKFLOW}: ignored lists docs/runbook.md, which the paths already cover; "
                                    "remove it"])

    def test_a_board_path_is_a_read_only_where_the_board_is_checked_out(self):
        self.steps(self.LISTED)
        self.write(".pm/w1/README.md", "x")
        self.write("scripts/guard.sh", 'here="$(dirname "$0")"\nyq . "$here/../infra/policy.json"\n'
                   "grep -c open .pm/w1/README.md\n")
        self.assertEqual(self.run_check({"scripts/needle.sh": "a needle"}), [])
        tracked = self.tracked()
        suite = cti.Suite(("deploy/gitops",), (), {"scripts/needle.sh": "a needle"}, runs=True, board=True)
        with mock.patch.dict(cti.SUITES, {self.WORKFLOW: suite}, clear=True):
            problems = cti.check(self.repo, self.WORKFLOW, tracked)
        self.assertEqual(problems, [f"{self.WORKFLOW}: paths miss .pm/w1/README.md, read by scripts/guard.sh"])

    def test_the_checker_its_own_step_runs_is_no_read(self):
        # Each suite's run checks its own paths; a change to the checker runs
        # scripts.yml, whose real-tree test checks every suite, so no suite
        # lists it, and nothing it names is a read either.
        self.write(cti.CHECKER, "cat lego/never.go\n")
        self.steps(self.LISTED)
        text = (self.repo / self.WORKFLOW).read_text().replace(
            "      - run: bash scripts/validate.sh\n",
            f"      - run: python3 -B {cti.CHECKER} {self.WORKFLOW}\n      - run: bash scripts/validate.sh\n")
        self.write(self.WORKFLOW, text)
        self.assertEqual(self.run_check({"scripts/needle.sh": "a needle"}), [])

    def test_affected_counts_the_scripts_a_suite_runs(self):
        self.steps(self.LISTED)
        self.tracked()
        suite = cti.Suite(("deploy/gitops",), (), {}, runs=True)
        out = io.StringIO()
        with mock.patch.dict(cti.SUITES, {self.WORKFLOW: suite}, clear=True), \
                mock.patch("sys.stdin", io.StringIO("scripts/guard.sh\nscripts/live-check.sh\ndocs/a.md\n")), \
                contextlib.redirect_stdout(out):
            self.assertEqual(cti.main(["--repo", str(self.repo), "--affected"]), 0)
        self.assertEqual(out.getvalue().split(), ["scripts/guard.sh"])


class Affected(unittest.TestCase):
    def test_only_what_could_open_a_gap(self):
        changed = ["lego/backend/internal/x/a_test.go", "lego/backend/internal/x/a.go", "dashboard/src/b.test.tsx",
                   "dashboard/src/b.tsx", ".github/workflows/dashboard-test.yml", ".github/workflows/mobile-test.yml",
                   "scripts/ci_test_inputs.py", "docs/a.md"]
        self.assertEqual(cti.affected(changed), ["lego/backend/internal/x/a_test.go", "dashboard/src/b.test.tsx",
                                                 ".github/workflows/dashboard-test.yml", "scripts/ci_test_inputs.py"])


class RealTree(unittest.TestCase):
    def test_every_suite_covers_its_reads(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(out):
            rc = cti.main(["--repo", str(HERE.parent)])
        self.assertEqual(rc, 0, out.getvalue())

    def test_gitops_scans_what_its_validation_greps(self):
        # gitops-validate.sh greps verify-static-site-security.sh for its live
        # scope guard; the workflow listed neither it nor a dozen like it.
        repo = HERE.parent
        workflow = ".github/workflows/gitops.yml"
        found, scripts = cti.run_references(repo, workflow, cti.SUITES[workflow], cti._tracked(repo))
        self.assertIn("scripts/gitops-validate.sh", scripts)
        self.assertIn("scripts/gitops-validate.sh", found["scripts/verify-static-site-security.sh"])
        self.assertNotIn("scripts/verify-static-site-security.sh", scripts, "only grepped, never run")

    def test_the_new_suites_scan_what_their_lists_missed(self):
        # Registered after gitops.yml: each read below was missing from its
        # workflow's paths, and scripts.yml's folded shellcheck list only reads.
        repo, tracked = HERE.parent, cti._tracked(HERE.parent)

        def scan(workflow):
            return cti.run_references(repo, workflow, cti.SUITES[workflow], tracked)

        found, _ = scan(".github/workflows/clusterapi-validate.yml")
        self.assertIn("scripts/clusterapi-validate.sh", found["infra/clusterapi/autoscaler-values.yaml"])
        found, _ = scan(".github/workflows/s3-contract.yml")
        self.assertIn("scripts/s3-contract.sh", found[".github/workflows/backend-test.yml"])
        self.assertIn(".github/workflows/s3-contract.yml", found["lego/backend/go.mod"], "setup-go's version file")
        _, scripts = scan(".github/workflows/scripts.yml")
        self.assertIn("scripts/secret-argv-validate.sh", scripts)
        self.assertNotIn("scripts/adr-registry-validate.sh", scripts, "shellcheck reads it; docs.yml runs it")

    def test_every_suite_fails_its_own_run_when_its_paths_drop_a_read(self):
        # The seven script-running suites once relied on scripts.yml and the
        # ship gate alone, so a push that bypassed /ship drifted their lists.
        repo, tracked = HERE.parent, cti._tracked(HERE.parent)
        step = re.compile(r"^\s*(?:-\s+)?run:\s*python3 -B scripts/ci_test_inputs\.py (\S+)\s*$", re.M)
        real = cti.path_filters
        for workflow, suite in cti.SUITES.items():
            if not (suite.tests or suite.runs):
                continue  # opensandbox-controller-test.yml: nothing to scan, so no read to miss
            text = (repo / workflow).read_text()
            self.assertIn(workflow, step.findall(cti._jobs(text)), f"{workflow} has no step checking its own paths")
            if all(paths is None for paths in real(text).values()):
                continue  # references.yml: every event runs on every change
            reads = sorted(set(cti.reads(repo, workflow, suite, tracked)) - set(suite.ignored) - {workflow})
            self.assertTrue(reads, f"{workflow} reads nothing outside its tree")
            drop = {p for p in next(p for p in real(text).values() if p) if cti.covered(reads[0], [p])}

            def dropped(text, drop=drop):
                return {event: None if paths is None else [p for p in paths if p not in drop]
                        for event, paths in real(text).items()}

            out = io.StringIO()
            with mock.patch.object(cti, "path_filters", dropped), contextlib.redirect_stderr(out), \
                    contextlib.redirect_stdout(io.StringIO()):
                rc = cti.main(["--repo", str(repo), workflow])
            self.assertEqual(rc, 1, f"{workflow} without {sorted(drop)}")
            self.assertIn(f"{workflow}: paths miss {reads[0]}, read by", out.getvalue())

    def test_only_a_board_suite_checks_the_board_out(self):
        # The premise of BOARD: a file outside the checkout cannot be read,
        # and one inside it can (docs.yml and references.yml read it whole).
        for name, suite in cti.SUITES.items():
            text = (HERE.parent / name).read_text()
            if suite.board:
                self.assertNotIn("!/" + cti.BOARD, text, f"{name} leaves .pm/ out, so it is no board suite")
            else:
                self.assertIn("!/" + cti.BOARD, text, f"{name} checks out .pm/, so its tests could read the board")


if __name__ == "__main__":
    unittest.main()
