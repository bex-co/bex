"""Show current DBC Node18 metadata without granting an artifact-source role."""
import json
from pathlib import Path
import subprocess

RUN = 41187749759
SOURCE = 'dbc493ba688c19758c9a489635761419fb2c1af6'
JOB = 124498652791
legacy = json.loads(subprocess.check_output(['python3', '/tmp/bex-m188-read-status.py']))
primary = json.loads(Path('/tmp/bex-m188-publisher-status.json').read_text())
row = next((item for item in primary['publishers'] if item['run_id'] == RUN), None)
origin = 'primary_metadata_observer'
if row is None:
    row = json.loads(Path('/tmp/bex-m188-node18-dbc-metadata-observer/node18-metadata-status.json').read_text())
    origin = 'isolated_metadata_only_observer'
assert row['source'] == SOURCE and row['run_attempt'] == 1
job = next(item for item in row['jobs'] if item['id'] == JOB)
steps = job.get('active_steps')
if steps is None:
    steps = [{'name': step['name'], 'started_at': step.get('started_at')} for step in job['steps'] if step['status'] == 'in_progress']
pending = SOURCE not in json.loads(Path('/tmp/bex-m188-publisher-quarantine.json').read_text())['approved_sources']
current = {'run_id': RUN, 'source': SOURCE, 'run_attempt': 1, 'job_id': JOB, 'tag': 'node-18-bookworm', 'run_status': row['status'], 'job_status': job['status'], 'conclusion': job.get('conclusion'), 'observed_at': row['observed_at'], 'stale': row.get('observation_stale', False), 'active_steps': steps, 'artifact_admission_pending': pending, 'metadata_only': True, 'observation_origin': origin}
phase = {key: value for key, value in current.items() if key not in ['observed_at', 'observation_origin']}
seen = Path('/tmp/bex-m188-core-stage-seen-v4.json')
before = json.loads(seen.read_text()) if seen.exists() else None
changes = [current] if before != phase else []
temporary = seen.with_suffix('.new')
temporary.write_text(json.dumps(phase, indent=2) + '\n')
temporary.replace(seen)
print(json.dumps({'at': primary['checked_at'], 'current_node18': current, 'core_stage_changes': changes, 'new_outcomes': legacy['new_completed_jobs'], 'unknown': legacy['unknown'], 'errors': legacy['errors']}))
