import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest

ROOT = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location('exact_adapter', ROOT / 'exact-node-v11-adapter.py')
a = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(a)
REPO = Path(os.environ.get('BEX_EXACT_REVIEW_REPO_ROOT', Path.cwd())).resolve()


class ExactAdapterControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.plan = a.make_plan(REPO)
        cls.workflow = a.source(REPO, a.WORKFLOW)

    def test_actual_shipped_source_preparation_is_not_artifact_verification(self):
        result = a.validate_plan(self.plan, REPO)
        self.assertFalse(result['candidate_verified'])
        self.assertEqual(a.singleton_row(self.workflow), {'name': a.NAME, 'base': a.BASE})
        self.assertIsNone(self.plan['run_id'])
        self.assertIsNone(self.plan['job_id'])
        self.assertIsNone(self.plan['ref'])
        self.assertEqual(self.plan['source_inputs'][a.PATCH]['sha256'], a.PATCH_SHA)

    def test_wrong_plan_source_name_version_workflow_base_and_support_are_rejected(self):
        for field, value in [('source_sha', '0' * 40), ('name', 'node-14-bullseye'),
                             ('version', '14.21.3'), ('workflow_sha256', '0' * 64),
                             ('base', a.BASE.replace('14.21.2', '14.21.3')),
                             ('patch_sha256', '0' * 64), ('freshness_sha256', '0' * 64)]:
            with self.subTest(field=field):
                plan = dict(self.plan, **{field: value})
                with self.assertRaises(ValueError):
                    a.validate_plan(plan, REPO)

    def test_source_closure_cannot_drop_the_patch_or_change_a_mode(self):
        for change in ('delete', 'mode'):
            plan = copy.deepcopy(self.plan)
            if change == 'delete':
                del plan['source_inputs'][a.PATCH]
            else:
                plan['source_inputs'][a.PATCH]['mode'] = '100755'
            with self.subTest(change=change), self.assertRaises(ValueError):
                a.validate_plan(plan, REPO)

    def test_actual_literal_selector_rejects_wrong_selector_row_base_and_duplicates(self):
        branch = ('          elif [ "$TARGET_RUNTIME" = ' + a.NAME + ' ]; then\n'
                  '            images=\'{"include":[{"name":"' + a.NAME + '","base":"' + a.BASE + '"}]}\'\n')
        self.assertIn(branch.encode(), self.workflow)
        bad = {
            'selector': self.workflow.replace(('= ' + a.NAME + ' ];').encode(), b'= node-14-bullseye ];'),
            'row': self.workflow.replace(('"name":"' + a.NAME + '"').encode(), b'"name":"node-14-bullseye"'),
            'base': self.workflow.replace(a.BASE.encode(), a.BASE.replace('14.21.2', '14.21.3').encode()),
            'duplicate': self.workflow.replace(branch.encode(), (branch + branch).encode()),
            'extra-row': self.workflow.replace(('"base":"' + a.BASE + '"}]}').encode(),
                         ('"base":"' + a.BASE + '"},{"name":"foreign","base":"foreign"}]}').encode()),
        }
        for label, raw in bad.items():
            with self.subTest(label=label), self.assertRaises(ValueError):
                a.singleton_row(raw)

    def test_selector_reads_immutable_workflow_and_rejects_foreign_requests_or_bytes(self):
        requests = []
        def source(revision, path):
            requests.append((revision, path))
            return self.workflow
        selector = a.matrix_selector(SimpleNamespace(source_bytes=source), self.plan)
        self.assertEqual(selector(a.SOURCE, a.NAME), a.BASE)
        self.assertEqual(requests, [(a.SOURCE, a.WORKFLOW)])
        for revision, name in [('0' * 40, a.NAME), (a.SOURCE, 'node-14-bullseye')]:
            with self.subTest(revision=revision, name=name), self.assertRaises(ValueError):
                selector(revision, name)
        selector = a.matrix_selector(SimpleNamespace(source_bytes=lambda *_: self.workflow + b'\n'), self.plan)
        with self.assertRaisesRegex(ValueError, 'workflow hash differs'):
            selector(a.SOURCE, a.NAME)

    def test_actual_run_job_and_ref_are_each_mandatory_before_review(self):
        # Synthetic scalar values exercise validation only; never persisted as a publication plan.
        complete_scalars = dict(self.plan, draft=False, actual_dispatch_source_confirmed=True, run_id=1, job_id=2, ref=a.REPO + '@sha256:' + 'a' * 64)
        for field in ('run_id', 'job_id', 'ref'):
            plan = dict(complete_scalars, **{field: None})
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, 'actual'):
                a.validate_plan(plan, REPO, executing=True)
        for field, value in [('run_id', True), ('job_id', '2'), ('ref', 'ghcr.io/foreign@sha256:' + 'a' * 64)]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                a.validate_plan(dict(complete_scalars, **{field: value}), REPO, executing=True)

    def test_review_cli_refuses_unknown_actual_metadata_before_creating_output_or_cache(self):
        with tempfile.TemporaryDirectory(dir=ROOT, prefix='controlled-cli-') as folder:
            folder = Path(folder)
            plan = folder / 'plan.json'
            plan.write_text(json.dumps(self.plan))
            output, cache = folder / 'output', folder / 'cache'
            process = subprocess.run([sys.executable, str(ROOT / 'exact-node-v11-adapter.py'), 'review',
                                      '--plan', str(plan), '--repo-root', str(REPO),
                                      '--output-root', str(output), '--cache-root', str(cache)],
                                     capture_output=True, text=True, timeout=20)
            self.assertNotEqual(process.returncode, 0)
            self.assertIn('draft only: fresh actual-run source review required', process.stderr)
            self.assertFalse(output.exists())
            self.assertFalse(cache.exists())

    def test_retained_v11_methods_are_inherited_byte_pinned_and_unreplaced(self):
        driver = a.import_pinned('loopx-w8-native-review-v11.py')
        policy = a.import_pinned('loopx-w8-verify-native-artifact-ranged.py')
        cls = a.review_class(driver, self.plan, REPO, ROOT / 'PLAN-PENDING.json')
        for method in ('run', 'fetch_bundles', 'crypto_bundles', 'verify_command', 'signed_statements',
                       'derive_docker_roots_if_needed', 'pull_blobs', 'finish'):
            self.assertIs(getattr(cls, method), getattr(driver.Review, method), method)
        original = {name: getattr(policy, name) for name in ('matrix_base', 'recipe_tree', 'github_candidate_job',
                    'recovery_provenance', 'spdx_binding', 'trivy_binding')}
        policy.matrix_base = a.matrix_selector(policy, self.plan)
        for name, function in original.items():
            if name != 'matrix_base':
                self.assertIs(getattr(policy, name), function, name)


if __name__ == '__main__':
    unittest.main(verbosity=2)
