#!/usr/bin/env python3
"""One exact published Node row; unchanged retained V11 crypto/report/blob/CI methods."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
from types import SimpleNamespace

SOURCE = '6883953e58e750fb41763d1d5e4bd4103bdb74c5'
NAME = 'node-14.21.2-bullseye'
BASE = 'node:14.21.2-bullseye@sha256:cc748803fe49871926c433ae6aafbfd59b765eed6005dc0843b916eb74f67b12'
REPO = 'ghcr.io/bex-co/bex-cnb-run'
WORKFLOW = '.github/workflows/native-base-image.yml'
WORKFLOW_SHA = '0571fa44f87342e1c90a57140d25a3022c871c3110643667e3bf1e392cd90e47'
PATCH = 'deploy/native-base/node-brotli/patch.sh'
PATCH_SHA = '52aa5ce0a753d658a7c1d5423bbfd42bb9c57710079e46a12fdd98eb76fa7a63'
FRESHNESS = 'scripts/lib/toolchain-freshness.py'
FRESHNESS_SHA = '4031b8338c1c1bf7771c93cb46917cb6404379b2dabe58cd4d8f61d05bf70490'
PINNED = {
    'loopx-w8-native-review-v11.py': 'ea03d2a9c7a21fde50c0d55f6ead84910c2c431f8c7b52c490624fc5ff8bff85',
    'loopx-w8-verify-native-artifact-ranged.py': 'ca4a765d547b92f567f7e08149183a8cb094249d544ae594655c92b93d09ae15',
}
CLOSURE = ['deploy/native-base', WORKFLOW, '.github/scripts/native-oci-stage.py',
           FRESHNESS, 'lego/operator/internal/build/native.go']
ROOT = Path(__file__).resolve().parent


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args], timeout=20)


def source(repo, path):
    return git(repo, 'show', SOURCE + ':' + path)


def source_manifest(repo):
    result = {}
    for entry in git(repo, 'ls-tree', '-r', '-z', SOURCE, '--', *CLOSURE).split(b'\0'):
        if not entry:
            continue
        metadata, path = entry.decode().split('\t', 1)
        mode, kind, _ = metadata.split()
        require(kind == 'blob' and mode in ('100644', '100755'), 'nonregular source input')
        raw = source(repo, path)
        require(len(raw) <= 256 << 10, 'source input exceeds bound')
        result[path] = {'mode': mode, 'bytes': len(raw), 'sha256': sha(raw)}
    require(all(path in result for path in [WORKFLOW, PATCH, FRESHNESS]), 'source closure is incomplete')
    return result


def singleton_row(raw):
    # Parse only the actual immutable workflow's literal branch; never execute shell/YAML.
    headers = re.findall(r'^          elif \[ \"\$TARGET_RUNTIME\" = ([a-z0-9.-]+) \]; then$', raw.decode(), re.MULTILINE)
    require(headers == [NAME], 'exact singleton selector missing or ambiguous')
    matches = re.findall(r'^          elif \[ "\$TARGET_RUNTIME" = ([a-z0-9.-]+) \]; then\n'
                         r"            images='([^'\n]+)'\n          else\n", raw.decode(), re.MULTILINE)
    require(len(matches) == 1 and matches[0][0] == NAME, 'exact singleton selector missing or ambiguous')
    row = json.loads(matches[0][1])
    require(row == {'include': [{'name': NAME, 'base': BASE}]}, 'exact singleton row/base differs')
    return row['include'][0]


def make_plan(repo):
    return {'schema': 'bex.exact-node-supplier-review-plan/v1',
            'scope': 'Planned exact supplier review only; this input plan is not publication, verification or adoption evidence.',
            'draft': True, 'actual_dispatch_source_confirmed': False,
            'source_sha': SOURCE, 'name': NAME, 'base': BASE, 'version': '14.21.2',
            'workflow_sha256': WORKFLOW_SHA, 'patch_sha256': PATCH_SHA,
            'freshness_sha256': FRESHNESS_SHA, 'source_inputs': source_manifest(repo),
            'run_id': None, 'job_id': None, 'ref': None}


def validate_plan(plan, repo, executing=False):
    require(plan.get('schema') == 'bex.exact-node-supplier-review-plan/v1', 'unknown plan schema')
    require(plan.get('source_sha') == SOURCE, 'wrong explicit source')
    require(plan.get('name') == NAME and plan.get('version') == '14.21.2', 'wrong exact row/name/version')
    require(plan.get('base') == BASE, 'wrong official base')
    require(plan.get('workflow_sha256') == WORKFLOW_SHA, 'wrong explicit workflow hash')
    require(plan.get('patch_sha256') == PATCH_SHA and plan.get('freshness_sha256') == FRESHNESS_SHA,
            'wrong source support hash')
    require(plan.get('source_inputs') == source_manifest(repo), 'immutable source closure differs')
    raw = source(repo, WORKFLOW)
    require(sha(raw) == WORKFLOW_SHA and sha(source(repo, PATCH)) == PATCH_SHA, 'pinned workflow/patch bytes differ')
    singleton_row(raw)
    if executing:
        require(plan.get('draft') is False and plan.get('actual_dispatch_source_confirmed') is True,
                'draft only: fresh actual-run source review required before execution')
        for field in ('run_id', 'job_id'):
            require(type(plan.get(field)) is int and plan[field] > 0, 'actual completed ' + field + ' required')
        require(re.fullmatch(re.escape(REPO) + r'@sha256:[0-9a-f]{64}', plan.get('ref') or ''),
                'actual immutable published ref required')
    return {'source_sha': SOURCE, 'name': NAME, 'base': BASE, 'workflow_sha256': WORKFLOW_SHA,
            'source_file_count': len(plan['source_inputs']), 'candidate_verified': False}


def matrix_selector(policy, plan):
    def matrix_base(revision, name):
        require(revision == plan['source_sha'] == SOURCE and name == plan['name'] == NAME,
                'foreign source/name matrix request')
        raw = policy.source_bytes(revision, WORKFLOW)
        require(sha(raw) == plan['workflow_sha256'] == WORKFLOW_SHA, 'actual workflow hash differs')
        return singleton_row(raw)['base']
    return matrix_base


def import_pinned(name):
    path = ROOT / 'retained' / name
    require(sha(path.read_bytes()) == PINNED[name], 'retained verifier changed: ' + name)
    spec = importlib.util.spec_from_file_location('exact_' + name.replace('-', '_'), path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def review_class(driver, plan, repo, plan_path):
    original_load = driver.load
    plan_hash = sha(plan_path.read_bytes())

    def scoped_load(name, path):
        policy = original_load(name, path)
        if name == 'native_policy':
            # The ONLY existing policy function replaced; all other methods remain original.
            policy.matrix_base = matrix_selector(policy, plan)
        return policy

    class ExactReview(driver.Review):
        def __init__(self, args):
            driver.load = scoped_load
            try:
                super().__init__(args)
            finally:
                driver.load = original_load
            self.tool_hashes[str(Path(__file__).resolve())] = sha(Path(__file__).read_bytes())
            self.tool_hashes[str(plan_path)] = plan_hash
            self.tool_hashes[str(repo / FRESHNESS)] = FRESHNESS_SHA

        def metadata(self):
            super().metadata()
            require(self.ref == plan['ref'] and self.base == plan['base'], 'actual artifact/base differs from plan')
            require(int(self.ci['job_id']) == plan['job_id'], 'actual selected job differs from plan')
            require(self.ci['primary_build_conclusion'] == 'success' and
                    self.provenance == {'mode': 'fresh-build', 'producer_sha': SOURCE, 'verification_sha': SOURCE},
                    'exact donor requires fresh same-source build; recovery not authorized')
            # Tighten actual main/job/output binding without replacing V11 CI checks.
            job = self.v.gh_json(['api', 'repos/bex-co/bex/actions/jobs/' + str(plan['job_id'])])
            require(job.get('id') == plan['job_id'] and job.get('run_id') == plan['run_id'] and
                    job.get('head_sha') == SOURCE and job.get('head_branch') == 'main' and
                    job.get('status') == 'completed' and job.get('conclusion') == 'success', 'actual main job identity differs')
            steps = {step['name']: step for step in job.get('steps', [])}
            output = steps.get('expose the fully gated immutable image', {})
            require(output.get('status') == 'completed' and output.get('conclusion') == 'success',
                    'fully gated immutable output did not complete')
            for arch, platform in self.platforms.items():
                require(self.v.official_environment(platform['config']).get('NODE_VERSION') == '14.21.2',
                        'actual ' + arch + ' Node metadata differs')
            self.v.dump(self.out / 'exact-row-plan-binding.json', {'plan_sha256': plan_hash,
                        'actual_job': job, 'source_inputs': plan['source_inputs'],
                        'scope': 'Exact supplier binding; common generated runtime qualification remains separate.'})
    return ExactReview


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=('source', 'review'))
    parser.add_argument('--plan', type=Path, required=True)
    parser.add_argument('--repo-root', type=Path, required=True)
    parser.add_argument('--output-root', type=Path, required=True)
    parser.add_argument('--cache-root', type=Path)
    args = parser.parse_args()
    repo = args.repo_root.resolve()
    plan_path = args.plan.resolve()
    plan = json.loads(plan_path.read_bytes())
    result = validate_plan(plan, repo, executing=args.mode == 'review')
    require(sha((repo / FRESHNESS).read_bytes()) == FRESHNESS_SHA, 'local V11 freshness support differs')
    output = args.output_root.resolve()
    require(repo != output and repo not in output.parents and not args.output_root.is_symlink(), 'output must be owned outside repository')
    require(not output.exists(), 'output must be a new owned directory')
    if args.mode == 'source':
        output.mkdir(mode=0o700)
        (output / 'source-check.json').write_text(json.dumps(result, indent=2) + '\n')
        print(json.dumps(result))
        return
    require(args.cache_root is not None and not args.cache_root.exists(), 'new owned review cache required')
    driver = import_pinned('loopx-w8-native-review-v11.py')
    import_pinned('loopx-w8-verify-native-artifact-ranged.py')
    output.mkdir(mode=0o700)
    args.cache_root.mkdir(mode=0o700)
    cls = review_class(driver, plan, repo, plan_path)
    original_args = SimpleNamespace(name=NAME, sha=SOURCE, run=str(plan['run_id']), repo_root=str(repo),
            policy_module=str(ROOT / 'retained/loopx-w8-verify-native-artifact-ranged.py'),
            output_root=str(output), cache_root=str(args.cache_root.resolve()), layer_workers=1)
    cls(original_args).run()


if __name__ == '__main__':
    main()
