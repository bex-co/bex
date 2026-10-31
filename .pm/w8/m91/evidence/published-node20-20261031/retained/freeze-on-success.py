import base64, datetime, gzip, hashlib, json, os
from pathlib import Path
R=Path(__file__).resolve().parent
O=R/'output/native-node-20-bookworm-6fd4f45536a2574c404b05539dacb6aaafb781f8'
def H(raw): return hashlib.sha256(raw).hexdigest()
def read(p): return json.loads(p.read_text())
def dump(p,d): p.write_text(json.dumps(d,indent=2)+'\n')
process=read(R/'execution-result.json');assert process['exit_code']==0
receipt=read(O/'receipt.json');assert receipt['approved_supplier_candidate'] is True
assert receipt['producer_workflow_sha']=='bfdeb62211ae5c4a875f5474a51c63c036e1d4dd'
assert receipt['verification_workflow_sha']=='6fd4f45536a2574c404b05539dacb6aaafb781f8'
assert receipt['ref']=='ghcr.io/bex-co/bex-cnb-run@sha256:2c69d3e2913703a634016cc24338f9532f24a750ddb831351e1374d274846a4d'
crypto=read(O/'bundles/crypto-results.json');positive=crypto['results'];neg=[crypto['wrong_source'],crypto['wrong_subject']]
assert len(positive)==5 and all(v['exit_code']==0 for v in positive)
assert all(v['exit_code']!=0 for v in neg)
attempts=[a for v in [*positive,*neg] for a in v['attempts']]
assert len(attempts)==7 and len({v['tuf_cache'] for v in attempts})==7
rows=receipt['signed_payloads'];assert len(rows)==5
assert sorted((v['kind'],str(v['architecture'])) for v in rows)==sorted([('image-signature','None'),('sbom','amd64'),('sbom','arm64'),('vulnerability','amd64'),('vulnerability','arm64')])
for v in rows:
 raw=Path(v['path']).read_bytes();assert H(raw)=='%s'%v['bundle_digest'].split(':')[1]
 assert gzip.decompress(Path(v['full_bundle_gzip']).read_bytes())==raw
 statement=base64.b64decode(json.loads(raw)['dsseEnvelope']['payload'],validate=True)
 assert H(statement)==v['signed_statement_sha256']
 decoded=json.loads(statement);assert any(s.get('digest',{}).get('sha256')==receipt['ref'].rsplit(':',1)[1] for s in decoded['subject'])
 canonical=json.dumps(decoded['predicate'],sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode()
 assert H(canonical)==v['decoded_predicate_sorted_json_sha256']
 assert gzip.decompress(Path(v['full_predicate_gzip']).read_bytes())==canonical
 assert all(x=={'kind':'hashedrekord','version':'0.0.2'} for x in v['tlog_kind_versions']) and v['tsa_timestamp_count']>0
blobs=receipt['anonymous_pull']['blob_receipts'];assert len(blobs)==37
assert all(not v['reused_prior_full_read'] for v in blobs)
assert sum(v['bytes_hashed'] for v in blobs)==1311620069
chains=[read(p) for p in O.glob('ranged-blob*.json')]
# Actual ranged-reader files may have a different stable prefix; discover by selected schema fields.
chains=[]
for p in O.glob('*.json'):
 d=read(p)
 if isinstance(d,dict) and {'logical_bytes_delivered','expected_size','attempts','digest'}<=d.keys():chains.append(dict(file=str(p.relative_to(R)),**d))
assert len(chains)==37
assert all(d['logical_bytes_delivered']==d['expected_size'] for d in chains)
raw_gets=sum(len(d['attempts']) for d in chains)
source=R/'loaded-freshness-at-start.py';current=Path('/Users/tianpan/projects/bex7/scripts/lib/toolchain-freshness.py')
assert H(source.read_bytes())==H(current.read_bytes())
for name,expected in receipt['evidence_tool_sha256'].items():assert H(Path(name).read_bytes())==expected
sample=read(R/'capacity-samples.json')
summary={'frozen_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'result':'ONE_COMPLETE_STRICT_SUPPLIER_ARTIFACT_VALIDATION_EXIT0','root_artifact_review_pending':True,'case_attempt':1,'ref':receipt['ref'],'official_base':receipt['official_base'],'producer_source':receipt['producer_workflow_sha'],'verification_source':receipt['verification_workflow_sha'],'producer_mode':receipt['producer_verification_provenance']['mode'],'run':receipt['ci_runtime_scan_signing_evidence']['run_id'],'job':receipt['ci_runtime_scan_signing_evidence']['job_id'],'actual_exit':0,'all_five_full_bundles_strict_PKI_tlog_TSA':True,'actual_wrong_source_and_subject_controls_rejected':True,'distinct_TUF_roots':7,'two_architecture_report_pairs_bound_independently':True,'unique_fresh_full_blob_count':len(blobs),'unique_fresh_full_blob_bytes':sum(v['bytes_hashed'] for v in blobs),'prior_receipt_reuse_bytes':0,'actual_body_GET_count':raw_gets,'actual_range_resume_cases':sum(len(d['attempts'])>1 for d in chains),'manual_decompressed_layer_reads':receipt['new_manual_layer_reads'],'all_trivy_summaries_retained':receipt['trivy_summaries_by_architecture'],'package_summaries_by_architecture':receipt['package_summaries_by_architecture'],'resource_minimum_observed_free_bytes':min(v['free_bytes'] for v in sample),'resource_maximum_observed_owned_bytes':max(v['own_allocated_bytes'] for v in sample),'full_raw_signed_bundles_preserved':True,'all_gzip_roundtrips_verified':True,'receipt_sha256':H((O/'receipt.json').read_bytes()),'process_receipt_sha256':H((R/'execution-result.json').read_bytes()),'executed_tools':receipt['evidence_tool_sha256'],'scope':'Historical recovered published supplier proof only: BF producer is distinct from 6fd verifier. Current common-base sources, catalog adoption, generated and hosted acceptance remain separate. No new source authorization, publication, repo, CI, Docker, provider, paid resource or production mutation. Root approval remains pending.'}
dump(R/'body-get-chain.json',{'at':summary['frozen_at'],'new_complete_blob_count':len(blobs),'full_bytes_hashed_fresh':sum(v['bytes_hashed'] for v in blobs),'cache_seed_or_prior_receipt_reuse':False,'body_GET_count':raw_gets,'ranged_blob_cases':[v for v in chains if len(v['attempts'])>1],'scope':'Unchanged canonical whole-SHA and size gates; all raw ranged-read metadata retained.'})
dump(R/'frozen-summary.json',summary)
files=[]
for p in sorted(R.rglob('*')):
 if p.is_file() and p.name!='frozen-manifest.json':
  assert not p.is_symlink()
  raw=p.read_bytes();files.append({'path':str(p.relative_to(R)),'bytes':len(raw),'sha256':H(raw)})
manifest={'schema':'bex.frozen-private-evidence/v1','at':summary['frozen_at'],'case':'node-20-bookworm-6fd4f45536a2574c404b05539dacb6aaafb781f8','files':files,'file_count':len(files),'total_bytes':sum(v['bytes'] for v in files),'root_review_pending':True}
dump(R/'frozen-manifest.json',manifest)
print(json.dumps({'manifest':str(R/'frozen-manifest.json'),'manifest_sha256':H((R/'frozen-manifest.json').read_bytes()),'file_count':len(files),'total_bytes':manifest['total_bytes'],'summary':summary},indent=2))
