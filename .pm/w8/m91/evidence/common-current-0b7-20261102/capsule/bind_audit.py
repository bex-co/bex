"""Data-only binding after both actual V2 extractions, never image/baseline approval."""
from pathlib import Path
import hashlib, json

R = Path(__file__).resolve().parent
inputs = json.loads((R / 'intake/INPUTS.json').read_bytes())['inputs']
result = {'scope': 'Original MANIFEST identity derivation from actual approved GitHub ZIP plus V2 full CRC/size/SHA extraction; no baseline admission', 'manifest_sha256': {}, 'extractions': {}}
for arch in ('amd64', 'arm64'):
    case = R / 'intake' / arch
    download = json.loads((case / 'download.json').read_bytes())
    extraction = json.loads((case / 'extracted/extraction.json').read_bytes())
    assert download['complete'] is True and download['observed_bytes'] == inputs[arch][1] and download['observed_sha256'] == inputs[arch][2]
    assert extraction['complete'] is True and extraction['zip_bytes'] == inputs[arch][1] and extraction['zip_sha256'] == inputs[arch][2]
    body = (case / 'extracted/artifact/MANIFEST.json').read_bytes()
    digest = hashlib.sha256(body).hexdigest()
    assert digest == extraction['manifest_sha256']
    result['manifest_sha256'][arch] = digest
    result['extractions'][arch] = {'bytes': len(body), 'sha256': digest, 'completed_members': extraction['completed_members']}
path = R / 'ACTUAL-EXTRACTION-BOUND.json'
assert not path.exists()
path.write_text(json.dumps(result, indent=2) + '\n')
