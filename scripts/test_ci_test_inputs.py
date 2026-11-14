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


class Repo(unittest.TestCase):
    """End to end on a throwaway repository."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = pathlib.Path(self.tmp.name)
        subprocess.run(["git", "init", "-q"], cwd=self.repo, check=True)
        self.write("dashboard/package.json", "{}")
        self.write("lego/backend/pricing.yaml", "x")
        self.write("lego/backend/fixture.go", "x")
        self.write("dashboard/src/__tests__/pricing.test.ts",
                   'read(`${process.cwd()}/../lego/backend/pricing.yaml`);\n'
                   'const sample = "lego/backend/fixture.go";\n'
                   'const filter = "lego/**/*.go";\n')

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, path, text):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def run_check(self, ignored=None):
        subprocess.run(["git", "add", "-A"], cwd=self.repo, check=True)
        tracked = set(subprocess.run(["git", "ls-files"], cwd=self.repo, capture_output=True, text=True,
                                     check=True).stdout.split())
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

    def test_the_own_tree_and_workflow_must_be_listed(self):
        self.write(WORKFLOW, workflow(["lego/backend/**"]))
        problems = self.run_check({"lego/backend/fixture.go": "x"})
        self.assertIn(f"{WORKFLOW}: paths miss the suite's own tree dashboard/**", problems)
        self.assertIn(f"{WORKFLOW}: paths miss the workflow file itself", problems)


class Affected(unittest.TestCase):
    def test_only_what_could_open_a_gap(self):
        changed = ["lego/backend/internal/x/a_test.go", "lego/backend/internal/x/a.go", "dashboard/src/b.test.tsx",
                   "dashboard/src/b.tsx", ".github/workflows/dashboard-test.yml", ".github/workflows/cli-test.yml",
                   "scripts/ci_test_inputs.py", "docs/a.md"]
        self.assertEqual(cti.affected(changed), ["lego/backend/internal/x/a_test.go", "dashboard/src/b.test.tsx",
                                                 ".github/workflows/dashboard-test.yml", "scripts/ci_test_inputs.py"])


class RealTree(unittest.TestCase):
    def test_every_suite_covers_its_reads(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(out):
            rc = cti.main(["--repo", str(HERE.parent)])
        self.assertEqual(rc, 0, out.getvalue())


if __name__ == "__main__":
    unittest.main()
