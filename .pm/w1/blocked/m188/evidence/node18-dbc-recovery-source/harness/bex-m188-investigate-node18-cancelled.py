"""One authorized read-only cancellation and all-source Node18 dedupe snapshot."""
import concurrent.futures
import datetime
import hashlib
import json
from pathlib import Path
import re
import subprocess
import urllib.parse

REPO = 'bex-co/bex'
WORKFLOW = '.github/workflows/native-base-image.yml'
output = Path('/tmp/bex-m188-node18-cancellation-dedupe-' + datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S'))
output.mkdir(mode=0o700)
(output / 'executed-helper.py').write_bytes(Path(__file__).read_bytes())
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()


def reference(path):
    raw = path.read_bytes()
    return {'file': str(path), 'sha256': hashlib.sha256(raw).hexdigest(), 'bytes': len(raw)}


def save(name, value):
    with (output / name).open('x') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')


def api(endpoint, label, optional=False):
    started = now()
    try:
        result = subprocess.run(['gh', 'api', endpoint], capture_output=True, timeout=45)
        code, stdout, stderr = result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired as error:
        code, stdout, stderr = None, error.stdout or b'', error.stderr or b''
    raw = output / (label + '.raw.json')
    raw.write_bytes(stdout)
    save(label + '.request.json', {'started_at': started, 'finished_at': now(), 'method': 'GET', 'endpoint': endpoint, 'exit_code': code, 'stderr': stderr.decode(errors='replace'), 'raw': reference(raw)})
    if code != 0:
        if optional:
            return {'observation_error': True, 'exit_code': code, 'stderr': stderr.decode(errors='replace')}
        raise RuntimeError('Read failed; no mutation or blind retry: ' + label)
    return json.loads(stdout)


def safe_text(value):
    if not value:
        return value
    text = re.sub(r'https?://\S+', '[URL omitted]', str(value))
    return re.sub(r'(?i)(authorization\s*[:=]\s*|bearer\s+|token\s*[:=]\s*)[^\s,]+', r'\1[omitted]', text)


def describe_run(run):
    return {key: run.get(key) for key in ['id', 'head_sha', 'head_branch', 'path', 'event', 'run_attempt', 'created_at', 'status', 'conclusion']}


def describe_job(job):
    return {**{key: job.get(key) for key in ['id', 'run_id', 'head_sha', 'run_attempt', 'name', 'status', 'conclusion', 'started_at', 'completed_at', 'runner_id', 'runner_name']}, 'started_steps': [{'name': step['name'], 'started_at': step.get('started_at'), 'status': step['status']} for step in job['steps'] if step.get('started_at') or step['status'] in ('in_progress', 'completed')]}


with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
    requests = {
        'da0_run': pool.submit(api, f'repos/{REPO}/actions/runs/41159591513', 'da0-run'),
        'da0_job': pool.submit(api, f'repos/{REPO}/actions/jobs/124414471403', 'da0-job'),
        'run2cc': pool.submit(api, f'repos/{REPO}/actions/runs/41184126238', '2cc-run'),
        'listing': pool.submit(api, f'repos/{REPO}/actions/workflows/native-base-image.yml/runs?branch=main&per_page=100', 'latest-main-runs'),
    }
    initial = {key: future.result() for key, future in requests.items()}
da0, cancelled, second, listing = (initial[key] for key in ['da0_run', 'da0_job', 'run2cc', 'listing'])
assert da0['head_sha'] == 'da0e56e80a1e4eb08cacabd67efde3c768a6ae81' and da0['path'] == WORKFLOW
assert cancelled['id'] == 124414471403 and cancelled['run_id'] == 41159591513 and cancelled['head_sha'] == da0['head_sha']
assert second['head_sha'] == '2cc8723df139bc4684d8d2a4d1eebc0c9dca004c' and second['path'] == WORKFLOW
assert len(listing['workflow_runs']) <= 100
all_runs = {run['id']: run for run in listing['workflow_runs']}
all_runs[da0['id']], all_runs[second['id']] = da0, second
# If history has exceeded one page, cover older active requests explicitly,
# while keeping each response bounded to100 and refusing incomplete inventories.
if listing['total_count'] > len(listing['workflow_runs']):
    for status in ['queued', 'in_progress', 'pending', 'waiting', 'requested']:
        page = api(f'repos/{REPO}/actions/workflows/native-base-image.yml/runs?branch=main&status={status}&per_page=100', 'older-active-' + status)
        assert page['total_count'] == len(page['workflow_runs']) <= 100
        all_runs.update({run['id']: run for run in page['workflow_runs']})
active = [run for run in all_runs.values() if run['status'] != 'completed']


def read_jobs(listed):
    run = listed if listed['id'] in (41159591513, 41184126238) else api(f"repos/{REPO}/actions/runs/{listed['id']}", f"active-{listed['id']}-run")
    assert run['path'] == WORKFLOW and run['head_branch'] == 'main'
    document = api(f"repos/{REPO}/actions/runs/{run['id']}/attempts/{run['run_attempt']}/jobs?per_page=100", f"run-{run['id']}-jobs")
    jobs = document['jobs']
    assert document['total_count'] == len(jobs) <= 100
    assert all(job['head_sha'] == run['head_sha'] and job['run_id'] == run['id'] and job['run_attempt'] == run['run_attempt'] for job in jobs)
    return {'run': describe_run(run), 'jobs': [describe_job(job) for job in jobs]}


targets = {run['id']: run for run in active}
targets[second['id']] = second
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
    observations = list(pool.map(read_jobs, targets.values()))
check = None
annotations = []
url = urllib.parse.urlsplit(cancelled['check_run_url'])
prefix = '/repos/' + REPO + '/check-runs/'
assert url.hostname == 'api.github.com' and url.path.startswith(prefix) and url.path[len(prefix):].isdigit()
check = api(url.path.lstrip('/'), 'da0-check-run', optional=True)
if not check.get('observation_error') and check.get('output', {}).get('annotations_count', 0):
    assert check['output']['annotations_count'] <= 100
    annotations = api(url.path.lstrip('/') + '/annotations?per_page=100', 'da0-check-annotations', optional=True)

node18 = []
unknown = []
compact = []
for observation in observations:
    run = observation['run']
    if run['status'] == 'completed':
        continue
    native = [job for job in observation['jobs'] if job['name'].startswith('build (')]
    ongoing = [job for job in native if job['status'] != 'completed']
    compact.append({'run': run, 'ongoing_native_jobs': ongoing})
    for job in ongoing:
        if job['name'].startswith('build (node-18-bookworm,'):
            node18.append({'run': run, 'job': job})
    if not native:
        unknown.append({'run': run, 'jobs': observation['jobs'], 'risk': 'No expanded runtime matrix available; selective/full input not established.'})
check_output = check.get('output', {}) if not check.get('observation_error') else {}
reason = {'check_id': check.get('id'), 'check_status': check.get('status'), 'check_conclusion': check.get('conclusion'), 'title': safe_text(check_output.get('title')), 'summary': safe_text(check_output.get('summary')), 'text': safe_text(check_output.get('text')), 'annotations': [{key: safe_text(item.get(key)) for key in ['annotation_level', 'message', 'title', 'raw_details']} for item in annotations] if isinstance(annotations, list) else annotations, 'check_api_error': check if check.get('observation_error') else None, 'cancelling_actor_not_inferred_from_dispatch_actor': True}
summary = {'schema': 'bex.m188.Node18-cancellation-and-all-source-dedupe/v1', 'finished_at': now(), 'DA0_run': describe_run(da0), 'DA0_job': describe_job(cancelled), 'DA0_cancellation_output': reason, 'run2cc': next(row for row in observations if row['run']['id'] == 41184126238), 'latest_list_total': listing['total_count'], 'listed_and_active_union_count': len(all_runs), 'nonterminal_run_count': len(compact), 'current_Node18_jobs': node18, 'unknown_matrix_input_risks': unknown, 'current_active_native_runs': compact, 'remote_mutations': 0, 'artifact_source_Q_guard_or_aggregate_written': False, 'limits': ['Exact job/check output is evidence of cancellation text, not an attribution from triggering actor.', 'Read-only snapshots are not atomic. No CI action or source admission has been performed.']}
save('summary.json', summary)
files = [reference(path) for path in sorted(output.iterdir()) if path.is_file()]
save('MANIFEST.json', {'files': files, 'file_count': len(files), 'total_bytes': sum(item['bytes'] for item in files)})
print(json.dumps({'summary': reference(output / 'summary.json'), 'manifest': reference(output / 'MANIFEST.json'), 'DA0_cancellation_output': reason, '2cc_run_state': describe_run(second), 'current_Node18_jobs': node18, 'unknown_matrix_input_risks': unknown, 'nonterminal_runs': [{'id': row['run']['id'], 'source': row['run']['head_sha'], 'status': row['run']['status'], 'tags': [job['name'].split(',')[0].removeprefix('build (') for job in row['ongoing_native_jobs']]} for row in compact], 'remote_mutations': 0}), flush=True)
