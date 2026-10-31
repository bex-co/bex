#!/usr/bin/env python3
"""One normal E20 dispatch under root's complete non-PM source predicate."""
import concurrent.futures
import datetime
import hashlib
import json
import pathlib
import subprocess

OUT = pathlib.Path('/tmp/bex-m188-elixir120-normal-dispatch')
OUT.mkdir(exist_ok=False)
BASE = '8af93065a6a83299e8db7977aaf6527a5bad3e5d'
TAG = 'elixir-1.20'
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
sha = lambda raw: hashlib.sha256(raw).hexdigest()


def save(name, value):
    path = OUT / name
    assert not path.exists()
    path.write_text(json.dumps(value, indent=2) + '\n')
    return {'file': str(path), 'sha256': sha(path.read_bytes())}


def api(route, stem):
    p = subprocess.run(['gh', 'api', route], capture_output=True, timeout=40)
    path = OUT / (stem + '.json')
    assert not path.exists()
    path.write_bytes(p.stdout)
    save(stem + '-request.json', {'at': now(), 'route': route, 'exit_code': p.returncode,
                                'stderr': p.stderr.decode(errors='replace'), 'raw_sha256': sha(p.stdout)})
    assert p.returncode == 0, p.stderr.decode(errors='replace')
    return json.loads(p.stdout)


def predicate(source, name):
    if subprocess.run(['git', 'cat-file', '-e', source + '^{commit}'], capture_output=True).returncode:
        subprocess.run(['git', 'fetch', '--no-tags', 'origin', source], check=True)
    assert subprocess.run(['git', 'merge-base', '--is-ancestor', BASE, source]).returncode == 0
    paths = subprocess.check_output(['git', 'diff', '--no-renames', '--name-only', BASE, source,
                                     '--', '.', ':!.pm'], text=True).splitlines()
    assert paths == [], 'non-PM source changed: ' + str(paths)
    workflow = subprocess.check_output(['git', 'show', source + ':.github/workflows/native-base-image.yml'])
    install = subprocess.check_output(['git', 'show', source + ':deploy/native-base/install.sh'])
    catalog_raw = subprocess.check_output(['git', 'show', source + ':.github/native-base-recovery.json'])
    assert sha(workflow) == '39b2ebac4a5b4d502db7a8245d90a9031db779ba4cea6da55588cb056abe469f'
    assert sha(install) == 'fe880434bb8f428500be52cd626534f69c9ae089d162e3549a50a769b09e5462'
    assert catalog_raw == subprocess.check_output(['git', 'show', BASE + ':.github/native-base-recovery.json'])
    assert TAG not in json.loads(catalog_raw)['images']
    return save(name, {'at': now(), 'literal_source': source, 'base': BASE,
                      'root_policy_sha256': POLICY_HASH, 'modern_scope': TAG,
                      'source_admitted_by_exact_root_family_predicate': True,
                      'entire_non_pm_diff': paths, 'workflow_sha256': sha(workflow),
                      'install_sha256': sha(install), 'catalog_sha256': sha(catalog_raw),
                      'recovery_entry_absent': True, 'normal_build_required': True,
                      'legacy_source_role_or_guard_context_append': False})


POLICY_HASH = '5dd8405b1d0c4fe42ea6e5af371a0c7eaf8a06818d3abd97c9731835935f445a'
policy = pathlib.Path('/tmp/bex-m188-elixir120-9cb-source-and-dispatch-policy.json')
assert sha(policy.read_bytes()) == POLICY_HASH and json.loads(policy.read_text())['root_approved']
old_preflight = pathlib.Path('/tmp/bex-m188-elixir120-exact800-preflight/preflight.json')
assert not json.loads(old_preflight.read_text())['dispatch_performed']
assert sha(pathlib.Path('/tmp/bex-m188-elixir120-sign-failure-124104286411-yi3jo7jb/failure-intake.json').read_bytes()) == '0a7ccd8225413457cd503a24d9af3af1e45a5385e7e4e1e882da51238ee0e292'
head = api('repos/bex-co/bex/git/ref/heads/main', 'initial-main')['object']['sha']
predicate(head, 'initial-source-predicate.json')
rows = []
page = 1
while True:
    listing = api('repos/bex-co/bex/actions/workflows/native-base-image.yml/runs'
                  f'?branch=main&per_page=100&page={page}', f'all-source-runs-{page}')
    rows.extend(listing['workflow_runs'])
    if len(rows) >= listing['total_count'] or not listing['workflow_runs']:
        break
    page += 1
by_id = {r['id']: r for r in rows}
known = json.loads(pathlib.Path('/tmp/bex-m188-known-publishers.json').read_text())
known['41142870861'] = BASE
for rid in known:
    if int(rid) not in by_id:
        by_id[int(rid)] = api(f'repos/bex-co/bex/actions/runs/{rid}', 'buried-run-' + rid)


def read_jobs(run):
    data = api(f"repos/bex-co/bex/actions/runs/{run['id']}/jobs?per_page=100", f"dedupe-{run['id']}-jobs")
    native = [j for j in data['jobs'] if j['name'].startswith('build')]
    assert native, 'nonterminal native workflow input unknown: ' + str(run['id'])
    return {'run_id': run['id'], 'source': run['head_sha'], 'event': run['event'],
            'native_jobs': [{'id': j['id'], 'name': j['name'], 'status': j['status']} for j in native],
            'matches_e20': any(j['name'].startswith('build (' + TAG + ',') for j in native)}


active = [r for r in by_id.values() if r['status'] != 'completed']
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
    observations = list(pool.map(read_jobs, active))
duplicates = [r for r in observations if r['matches_e20']]
save('dedupe-summary.json', {'at': now(), 'all_recent_runs': len(rows), 'all_known_union': len(by_id),
                            'nonterminal_observations': observations, 'duplicates': duplicates})
assert not duplicates, 'existing nonterminal E20 request; no POST'
head = api('repos/bex-co/bex/git/ref/heads/main', 'immediate-prepost-main')['object']['sha']
source_receipt = predicate(head, 'actual-prepost-source-role-receipt.json')
payload = OUT / 'dispatch-input.json'
save(payload.name, {'ref': 'main', 'inputs': {'runtime': TAG}})
began = now()
save('dispatch-intent.json', {'at': began, 'maximum_total_e20_posts': 1,
                            'earlier_exact800_posts': 0, 'source': head, 'source_role': source_receipt,
                            'root_policy_sha256': POLICY_HASH})
try:
    post = subprocess.run(['gh', 'api', '--include', '-X', 'POST',
                           'repos/bex-co/bex/actions/workflows/native-base-image.yml/dispatches',
                           '--input', str(payload)], capture_output=True, timeout=40)
except subprocess.TimeoutExpired as error:
    save('post-uncertain.json', {'at': now(), 'potential_post_sent': True, 'no_retry': True, 'error': str(error)})
    raise
receipt = save('dispatch-result.json', {'started_at': began, 'finished_at': now(), 'requested_source': head,
           'tag': TAG, 'dispatch_posts': 1, 'exit_code': post.returncode,
           'stdout': post.stdout.decode(errors='replace'), 'stderr': post.stderr.decode(errors='replace'),
           'no_blind_retry': True, 'normal_build_required': True, 'no_recovery_authorization_added': True,
           'no_cancellation': True, 'no_legacy_guard_context_write': True})
print(json.dumps({'dispatch_result': receipt, 'source': head, 'exit_code': post.returncode,
                  'http_status_line': post.stdout.decode(errors='replace').splitlines()[0] if post.stdout else None,
                  'actual_binding_pending': True}), flush=True)
