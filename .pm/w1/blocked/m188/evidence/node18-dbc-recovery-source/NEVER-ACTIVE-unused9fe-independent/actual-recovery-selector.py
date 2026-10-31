import json, os, re, subprocess
from pathlib import Path
path = Path('.github/native-base-recovery.json')
if not path.exists():
    raise SystemExit(0)
data = json.loads(path.read_text())
if data.get('schema') != 'bex.native-publisher-recovery/v1':
    raise SystemExit('invalid recovery authorization schema')
entry = data['images'].get(os.environ['MATRIX_NAME'])
if entry is None or entry['base'] != os.environ['BASE_IMAGE']:
    raise SystemExit(0)
ref, producer = entry['ref'], entry['producer']
prefix = os.environ['NATIVE_IMAGE'] + '@'
if not ref.startswith(prefix) or not re.fullmatch(r'sha256:[0-9a-f]{64}', ref[len(prefix):]) or not re.fullmatch(r'[0-9a-f]{40}', producer):
    raise SystemExit('recovery requires a reviewed same-package digest and producer')
if subprocess.run(['git', 'cat-file', '-e', producer + '^{commit}'], capture_output=True).returncode:
    subprocess.run(['git', 'fetch', '--no-tags', '--depth=1', 'origin', producer], check=True)
changed = subprocess.check_output(['git', 'diff', '--no-renames', '--name-only', producer, 'HEAD', '--', 'deploy/native-base'], text=True).splitlines()
if any(not name.endswith('.md') for name in changed):
    print('Recovery recipe changed; building from source')
    raise SystemExit(0)
with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
    output.write('digest=' + ref[len(prefix):] + '\nproducer=' + producer + '\n')
print('Rechecking reviewed artifact ' + ref + '; producer ' + producer + '; original job ' + str(entry['job_id']))
