"""One fixed, offline, allowlisted m144 evidence export. No auth or live calls."""
from pathlib import Path
import hashlib
import json
import re

REPO = Path('/Users/tianpan/projects/bex7')
LIVE = Path('/private/tmp/bex-w8-m144-live-20261031-cc62f8e3a1ee8b8c')
ENDED = LIVE / 'frozen-ended-attempt'
V3 = LIVE / 'v3-surface-preparation'
INITIAL = REPO / '.pm/w8/m144/evidence/paid-initial-cc62-20261031'
DEST = REPO / '.pm/w8/m144/evidence/paid-attempt-cc62-20261031'
INPUTS = {
    ENDED: '5d07ba3fe53f0182633590a55341a4ce0513a1372f03ed5e003f1559d3742c83',
    V3: '58ba9d98bdfeb4390f1178d2164516527893ee5c3a30fedce8bd9814a360128f',
    INITIAL: '4aa8920fe5d9572852ca858d4ed025f404f5cb69a1edeec9b91836ef3b79a113',
}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def cleaned(value):
    if isinstance(value, dict):
        return {key: cleaned(item) for key, item in value.items() if key.lower() != 'authorization'}
    if isinstance(value, list):
        return [cleaned(item) for item in value]
    return value


def transform(source):
    raw = source.read_bytes()
    if source.suffix == '.json':
        value = json.loads(raw)
        if 'response' in value and 'response_sha256' in value:
            assert sha(value['response'].encode()) == value['response_sha256'], source
        return (json.dumps(cleaned(value), indent=2) + '\n').encode(), 'JSON authorization metadata omitted; response bytes unchanged'
    return raw, 'verbatim noncredential source/test/failure text'


def check_bytes(raw):
    assert not re.search(rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----', raw)
    assert not re.search(rb'Bearer [A-Za-z0-9._-]{24,}', raw)
    assert not re.search(rb'eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}', raw)


for root, expected in INPUTS.items():
    manifest = root / 'MANIFEST.json'
    assert sha(manifest.read_bytes()) == expected, root
    data = json.loads(manifest.read_text())
    files = data if root == INITIAL else data['files']
    for name, proof in files.items():
        source = root / name
        assert source.is_file() and not source.is_symlink(), source
        assert source.stat().st_size == proof['bytes'] and sha(source.read_bytes()) == proof['sha256'], source

assert not DEST.exists(), 'Never overwrite an existing export'
DEST.mkdir(mode=0o755)
mapping = {}


def export(source, name):
    assert source.is_file() and not source.is_symlink(), source
    raw, rule = transform(source)
    check_bytes(raw)
    target = DEST / name
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(raw)
    mapping[name] = {'source': str(source), 'source_sha256': sha(source.read_bytes()),
                     'source_bytes': source.stat().st_size, 'transformation': rule,
                     'sha256': sha(raw), 'bytes': len(raw)}


for source in sorted((ENDED / 'http').glob('*.json')):
    value = json.loads(source.read_text())
    assert not value['request']['path'].startswith('/v1/services?ownerId='), 'No full baseline-service lists'
    export(source, 'http/' + source.name)

attempt_names = '''early-final-hashes.stdout.json early-final-hashes.result.json
early-final-surfaces.result.json early-final-surfaces.stderr.txt
early-final-billing.stdout.json early-final-billing.result.json
early-cleanup-launch.json early-cleanup.stdout.json early-cleanup-result.json
early-cleanup-check-001.stdout.json early-cleanup-check-001.result.json
post-cleanup-quota.py post-cleanup-quota-attempt1.json post-cleanup-quota.receipt.json
post-cleanup-quota.result.json auth-cleanup.stdout.json auth-cleanup-original-receipt.json
auth-cleanup-result.json cleanup-watchdog-state.json watchdog-natural-exit.json
cleanup-watchdog-1793450229831339000.txt cleanup-watchdog-1793450234486338000.txt
orchestrator-v2.diff orchestrator-v2-ast-identity.json orchestrator-v2-tests.log
test_orchestrator_v2.py observe-v2-readonly-001.stdout.json observe-v2-readonly-001.result.json
existing-local-access-source-followup.json remaining-budget-1223.json inventory-snapshot-012.json'''.split()
for name in attempt_names:
    export(ENDED / name, 'attempt/' + name)

for name in '''access-intent.json access-result.json fetch-error-classification.json
source-custody.json existing-hostpin-source-presence.json failed-fetch-private-material-state.json'''.split():
    export(ENDED / 'local-access' / name, 'local-access/' + name)
export(ENDED / 'local-access/run-existing-access.sh', 'local-access/run-existing-access.sh')
for source in sorted(V3.iterdir()):
    if source.is_file() and source.name != 'MANIFEST.json':
        export(source, 'surface-v3/' + source.name)
for name in ['original-observe-FAIL.stderr.txt', 'original-status-origin-FAIL.stderr.txt', 'v2-first-read-API-reset.stderr.txt']:
    export(INITIAL / name, 'original-failures/' + name)
export(LIVE / 'inventory-check-1259.json', 'inventory-check-1259.json')

summary = json.loads((ENDED / 'SUMMARY.json').read_text())
summary.pop('raw_http_index')
summary.pop('private_only_baseline_service_list_receipt_hashes')
summary['exported_scope'] = 'Allowlisted derivatives; exact source hashes and transformations in CUSTODY.json'
summary['latest_inventory_check'] = cleaned(json.loads((LIVE / 'inventory-check-1259.json').read_text()))
outcome = (json.dumps(cleaned(summary), indent=2) + '\n').encode()
check_bytes(outcome)
(DEST / 'OUTCOME.json').write_bytes(outcome)
(DEST / 'CUSTODY.json').write_text(json.dumps({'frozen_input_manifests': {str(k): v for k, v in INPUTS.items()}, 'files': mapping}, indent=2) + '\n')
(DEST / 'SANITIZATION.json').write_text(json.dumps({
    'scope': 'Offline export only; no credential files, raw ledger, .env, kubeconfig, HTTP or auth action used',
    'authorization_metadata': 'Omitted recursively from JSON exports; raw response strings and their hashes retained',
    'excluded': ['full baseline-service list responses', 'raw ledger', 'credential/config/session files',
                 'raw SSH host identifiers and key fingerprints', 'images/binaries/filesystem data'],
    'SSH_failure_preserved': 'Fixed error classes/counts and original stderr SHA/size; complete raw stderr stays private',
    'source_and_test_code': 'Actual frozen source/test bytes retained; credential path and variable names are code, not values',
    'synthetic_checks': 'V3 seven pure cases are local tests; not live acceptance',
    'unknowns': ['Kubernetes App/PVC/PV/Secret/Jobs absence', 'provider volume absence', 'S3 purge',
                 '1-to-5 requested-size billing', 'financial sealing/invoice', 'hosted full/grow/remount/restore/dashboard'],
    'latest_inventory_metadata': 'Selected facts transcribed from actual existing tool result; no redirected raw stdout claimed',
}, indent=2) + '\n')

# Reproduce every mapped output from its exact frozen source, then verify bytes.
for name, item in mapping.items():
    source = Path(item['source'])
    assert sha(source.read_bytes()) == item['source_sha256']
    expected, _ = transform(source)
    actual = (DEST / name).read_bytes()
    assert actual == expected and sha(actual) == item['sha256'] and len(actual) == item['bytes'], name
print(json.dumps({'export': str(DEST), 'reproduced_source_exports': len(mapping),
                  'outcome_sha256': sha(outcome), 'no_board_status_or_location_change': True}))
