"""Compact one completed strict V8 receipt; preserve full public payloads privately."""
from pathlib import Path
import gzip
import hashlib
import json
import shutil

ROOT = Path('/Users/tianpan/projects/bex7')
CONTROL = Path(__file__).resolve().parent
HANDOFF = json.loads((CONTROL / 'handoff.json').read_text())
RAW = Path(HANDOFF['output_root']) / ('native-' + HANDOFF['name'] + '-' + HANDOFF['source'])
COMPACT = ROOT / '.pm/w8/m91/evidence/published-ruby30-20261030'


def sha(path):
    value = hashlib.sha256()
    with path.open('rb') as stream:
        while block := stream.read(1 << 20):
            value.update(block)
    return value.hexdigest()


def decompressed_sha(path):
    value = hashlib.sha256()
    size = 0
    with gzip.open(path, 'rb') as stream:
        while block := stream.read(1 << 20):
            value.update(block)
            size += len(block)
    return value.hexdigest(), size


def dump(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


receipt_path = RAW / 'receipt.json'
receipt = json.loads(receipt_path.read_text())
assert receipt['approved_supplier_candidate'] is True
assert receipt['ref'] == HANDOFF['ref']
assert receipt['verification_workflow_sha'] == HANDOFF['source']
assert receipt['producer_workflow_sha'] == HANDOFF['producer']
assert receipt['ci_runtime_scan_signing_evidence']['job_id'] == HANDOFF['job_id']
assert receipt['producer_verification_provenance']['mode'] == 'fresh-build'
assert receipt['producer_workflow_sha'] == receipt['verification_workflow_sha']
source_context = json.loads((CONTROL / 'fresh-source-recipe-inputs.json').read_text())
assert source_context['non_documentation_recipe_files'] == 22
assert source_context['source'] == receipt['verification_workflow_sha']
assert source_context['official_base'] == receipt['official_base']
assert len(receipt['signed_payloads']) == 5
assert sum(row['kind'] == 'image-signature' for row in receipt['signed_payloads']) == 1
for kind in ('sbom', 'vulnerability'):
    assert {row['architecture'] for row in receipt['signed_payloads'] if row['kind'] == kind} == {'amd64', 'arm64'}
assert all(sha(Path(path)) == expected for path, expected in receipt['evidence_tool_sha256'].items())
assert receipt['wrong_source_and_subject_negative_checks_rejected'] is True
assert receipt['anonymous_pull']['all_unique_config_and_layer_bytes_fully_hashed'] is True
assert receipt['anonymous_pull']['bytes_reused_from_prior_verified_full_read'] == 0
assert not (RAW / 'failed-review.json').exists()
shutil.copyfile(receipt_path, CONTROL / 'frozen-final-receipt.json')

payloads = []
for row in receipt['signed_payloads']:
    bundle = Path(row['full_bundle_gzip'])
    predicate = Path(row['full_predicate_gzip'])
    assert sha(bundle) == row['full_bundle_gzip_sha256']
    assert bundle.stat().st_size == row['full_bundle_gzip_bytes']
    bundle_sha, bundle_size = decompressed_sha(bundle)
    assert 'sha256:' + bundle_sha == row['bundle_digest'] and bundle_size == row['bundle_bytes']
    assert sha(predicate) == row['full_predicate_gzip_sha256']
    predicate_sha, predicate_size = decompressed_sha(predicate)
    assert predicate_sha == row['decoded_predicate_sorted_json_sha256']
    assert predicate_size == row['decoded_predicate_sorted_json_bytes']
    payloads.append({key: row[key] for key in ('kind', 'architecture', 'manifest_digest', 'bundle_digest', 'bundle_bytes', 'full_bundle_gzip', 'full_bundle_gzip_sha256', 'full_bundle_gzip_bytes', 'full_predicate_gzip', 'full_predicate_gzip_sha256', 'decoded_predicate_sorted_json_sha256', 'decoded_predicate_sorted_json_bytes')})

COMPACT.mkdir(exist_ok=False)
custody = []
for source in sorted(RAW.rglob('*')):
    if not source.is_file():
        continue
    relative = source.relative_to(RAW)
    if 'owned-layer-files' in relative.parts or source.name.endswith(('.bundle.json', '.bundle.json.gz', '.predicate.json.gz')):
        continue
    assert not source.is_symlink() and source.stat().st_size <= 4 << 20, str(source)
    if source.suffix == '.log':
        relative = relative.with_suffix('.txt')
    destination = COMPACT / relative
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, destination)
    assert sha(source) == sha(destination)
    custody.append({'source': str(source), 'compact': str(relative), 'bytes': source.stat().st_size, 'sha256': sha(source)})

for name in ('preflight.json', 'handoff.json', 'green-job-handoff.json', 'original-guard-tests.py', 'private-guard-tests.py', 'guard-tests-result.json', 'guard-tests.log', 'review.log', 'archive.py', 'fresh-source-recipe-inputs.json'):
    source = CONTROL / name
    relative = Path('peer-controls') / (name[:-4] + '.txt' if name.endswith('.log') else name)
    destination = COMPACT / relative
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, destination)
    custody.append({'source': str(source), 'compact': str(relative), 'bytes': source.stat().st_size, 'sha256': sha(source)})

dump(COMPACT / 'PRIVATE_PAYLOADS.json', {'scope': 'Complete public signed bundles and predicates stay outside Git. Gzip round trips and immutable SHA/size checks passed; public OCI descriptors permit independent retrieval.', 'raw_output': str(RAW), 'frozen_receipt_sha256': sha(receipt_path), 'payloads': payloads})
dump(COMPACT / 'LOG-CUSTODY.json', {'scope': 'Copied bytes are exact; .log names become .txt to retain them through repository ignore rules.', 'files': custody})
summary = {
    'supplier': HANDOFF['name'], 'approved_supplier_candidate': True,
    'ref': receipt['ref'], 'verification_source': HANDOFF['source'], 'producer_source': HANDOFF['producer'],
    'official_base': receipt['official_base'], 'actual_ci': receipt['ci_runtime_scan_signing_evidence'],
    'strict_review': 'Unchanged V8/policy, all5 exact-source/issuer/workflow/subject/Rekor-v2/RFC3161-TSA bundle checks, wrong-source/subject negative controls,22 immutable fresh-source recipe/mode inputs, both-architecture SPDX/Trivy binding, every anonymous config/layer descriptor SHA/size.',
    'guard_cases': 43, 'guard_qualification': 'Known43 guards reused from prior Py310 run because exact V8/policy hashes are unchanged; no rerun or write to that frozen capsule.',
    'anonymous_blob_count': receipt['anonymous_pull']['unique_blob_count'],
    'anonymous_bytes_hashed': receipt['anonymous_pull']['bytes_hashed_this_finalization'],
    'prior_verified_bytes_reused': receipt['anonymous_pull']['bytes_reused_from_prior_verified_full_read'],
    'new_manual_layer_reads': receipt['new_manual_layer_reads'],
    'docker_root_qualification': 'Direct actual registry-root SPDX binding. No Stereoscope alias or manual decompressed diffID proof was needed or claimed.' if not receipt['spdx_additional_verified_root_bindings'] else receipt['spdx_additional_verified_root_bindings'],
    'package_summaries': receipt['package_summaries_by_architecture'],
    'vulnerability_summaries': receipt['trivy_summaries_by_architecture'],
    'findings_policy': 'All findings retained in full verified private gzip/public OCI; no zero-CVE claim or security waiver.',
    'limits': 'Supplier review only. No catalog/recovery policy/pin/production resource change or hosted/generated-image/tool-family acceptance.',
    'full_payload_custody': 'PRIVATE_PAYLOADS.json', 'raw_review_root': str(RAW),
    'private_fresh_cache': HANDOFF['cache_root'], 'tool_sha256': receipt['evidence_tool_sha256'],
}
dump(COMPACT / 'README.json', summary)
files = {str(path.relative_to(COMPACT)): {'bytes': path.stat().st_size, 'sha256': sha(path)} for path in sorted(COMPACT.rglob('*')) if path.is_file()}
dump(COMPACT / 'MANIFEST.json', {'schema': 'bex.native-compact-evidence-manifest/v1', 'scope': 'Small exact-source/log/metadata custody only. Full signed report/bundle gzip and image layers are not in Git.', 'file_count': len(files), 'file_bytes': sum(value['bytes'] for value in files.values()), 'files': files})
print(json.dumps({'archive': str(COMPACT), 'files': len(files), 'bytes': sum(value['bytes'] for value in files.values()), 'frozen_receipt_sha256': sha(receipt_path)}, indent=2))
