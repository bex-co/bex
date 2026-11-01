"""Rehash only already-observed actual retained inputs; no external actions."""
from pathlib import Path
import hashlib,json
R=Path(__file__).parent
def digest(p):
 p=Path(p);assert p.is_file() and not p.is_symlink();h=hashlib.sha256();n=0
 with p.open('rb') as f:
  while b:=f.read(1<<20):h.update(b);n+=len(b)
 return h.hexdigest(),n
rows=json.loads((R/'INPUT-CUSTODY.json').read_bytes())
for path,row in rows.items():assert digest(path)==(row['sha256'],row['bytes']),path
root=Path('/private/tmp/bex-w8-common499-review-2ff8ba6f1f039942');e=root/'evidence/native-common-bookworm-1feba939b4697894e800ed0abb4f25a0bf415641'
receipt=json.loads((e/'receipt.json').read_bytes());crypto=json.loads((e/'bundles/crypto-results.json').read_bytes())
assert len(crypto['results'])==5 and set(crypto['accepted_bundles'])=={x['bundle_digest'] for x in receipt['signed_payloads']}
prior=Path('/private/tmp/bex-w8-common0941-independent-review-20261101-4ig86hts')
assert digest(prior/'REVIEW.json')[0]=='201e17c131c030d084507b2ee35208d0a3b8a1abda9707edf5b0ca9c847c459b'
assert digest(prior/'MANIFEST.json')[0]=='5330635f81fbde72dee7bf8665114b23604611d92777a615253aa688d9adc5f7'
for name,row in json.loads((prior/'MANIFEST.json').read_bytes()).items():assert digest(prior/name)==(row['sha256'],row['bytes'])
out={'PASS':True,'actual_inputs_unchanged':len(rows),'crypto_aggregate_five_exact_accepted_bundles':True,
 'prior_current_source_review':{'root':str(prior),'review_sha256':digest(prior/'REVIEW.json')[0],'manifest_sha256':digest(prior/'MANIFEST.json')[0],
 'scope':'Already independently verified current20factory+33image-critical full-byte/Git-mode/blob closure at actual1feba versus generatorf071; actualV11 tool pins and retained producer/verifier/generator22-file inventories independently rechecked in audit.py.'},
 'receipt_sha256':digest(e/'receipt.json')[0],'RESULT_sha256':digest(root/'RESULT.json')[0]}
(R/'FINAL-CONSISTENCY.json').write_text(json.dumps(out,indent=2)+'\n')
print(json.dumps({'PASS':True,'actual_inputs_rehashed_unchanged':len(rows)}))
