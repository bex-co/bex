from pathlib import Path
import collections,datetime,gzip,hashlib,json,shutil
OUT=Path('/tmp/loopx-w8-native-review-node22-v11-83c1e30-20261031')
CACHE=Path('/tmp/loopx-w8-native-cache-node22-v11-83c1e30-20261031')
CTL=Path(__file__).resolve().parent
CASE=OUT/'native-node-22-bookworm-83c1e30e35eeeadab499de4d8d96c1f6c0349077'
def digest_stream(stream):
 h=hashlib.sha256();n=0
 while b:=stream.read(1<<20):h.update(b);n+=len(b)
 return h.hexdigest(),n
def digest(path):
 with path.open('rb') as s:return digest_stream(s)
def write(path,data):path.write_text(json.dumps(data,indent=2)+'\n')
now=datetime.datetime.now(datetime.timezone.utc).isoformat()
process=json.loads((OUT/'process-exit-case-attempt1.json').read_text())
assert process['actual_returncode']==0 and process['stopped_reason'] is None
r=json.loads((CASE/'receipt.json').read_text());assert r['approved_supplier_candidate']
assert r['verification_workflow_sha']==r['producer_workflow_sha']=='83c1e30e35eeeadab499de4d8d96c1f6c0349077'
assert r['producer_verification_provenance']['mode']=='fresh-build'
assert r['ci_runtime_scan_signing_evidence']['job_id']==124317237455 and r['ci_runtime_scan_signing_evidence']['run_id']=='41128855169'
assert r['official_base']=='node:22-bookworm@sha256:18a6d90fd60542269850c80ca03b29b1d9b14d6b221ac7ee8e35b0d33ef904bd'
assert r['strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified'] and r['strict_actual_Rekor_v2_and_RFC3161_TSA_verified'] and r['wrong_source_and_subject_negative_checks_rejected']
assert not (CASE/'failed-review.json').exists()
preflight=json.loads((CTL/'preflight.json').read_text())
for p,want in preflight['expected_source_hashes'].items():assert digest(Path(p))[0]==want
expected={}
for arch in ['amd64','arm64']:
 m=json.loads((CASE/(arch+'-manifest.json')).read_text())
 for d in [m['config'],*m['layers']]:
  if d['digest'] in expected:assert expected[d['digest']]==d['size']
  expected[d['digest']]=d['size']
pulls=r['anonymous_pull']['blob_receipts'];assert len(pulls)==len(expected)==35
assert {x['digest']:x['bytes_hashed'] for x in pulls}==expected
fresh=[x for x in pulls if not x['reused_prior_full_read']];reused=[x for x in pulls if x['reused_prior_full_read']]
prior_path=CTL/'copied-genuine-prior-receipts.json';reuse_custody=json.loads((CTL/'reuse-custody.json').read_text())
assert digest(prior_path)[0]==reuse_custody['source_journal_sha256']
prior=json.loads(prior_path.read_text())['blobs']
assert len(fresh)==26 and len(reused)==9
for row in reused:
 original=prior[row['digest']]
 assert {k:v for k,v in row.items() if k not in ['reused_prior_full_read','current_anonymous_head_at']}==original
 assert datetime.datetime.fromisoformat(process['started_at']) <= datetime.datetime.fromisoformat(row['current_anonymous_head_at']) <= datetime.datetime.fromisoformat(r['reviewed_at'])
for row in fresh:
 assert row['digest'] not in prior
 assert datetime.datetime.fromisoformat(row['anonymous_full_get_at']) >= datetime.datetime.fromisoformat(process['started_at'])
fresh_bytes=sum(x['bytes_hashed'] for x in fresh);reused_bytes=sum(x['bytes_hashed'] for x in reused)
assert r['anonymous_pull']['bytes_hashed_this_finalization']==fresh_bytes==540371859
assert r['anonymous_pull']['bytes_reused_from_prior_verified_full_read']==reused_bytes==688364393
assert fresh_bytes+reused_bytes==sum(expected.values())==1228736252
crypto=json.loads((CASE/'bundles/crypto-results.json').read_text());assert len(crypto['accepted_bundles'])==5 and all(x['exit_code']==0 for x in crypto['results'])
assert crypto['wrong_source']['exit_code']!=0 and crypto['wrong_subject']['exit_code']!=0
all_commands=crypto['results']+[crypto['wrong_source'],crypto['wrong_subject']]
tuf=[a['tuf_cache'] for x in all_commands for a in x['attempts']];assert len(tuf)==len(set(tuf))
custody=[]
for row in r['signed_payloads']:
 raw=Path(row['path']);z=Path(row['full_bundle_gzip']);pd=Path(row['full_predicate_gzip'])
 rawsha,rawbytes=digest(raw);assert 'sha256:'+rawsha==row['bundle_digest'] and rawbytes==row['bundle_bytes']
 gzsha,gzbytes=digest(z);assert gzsha==row['full_bundle_gzip_sha256'] and gzbytes==row['full_bundle_gzip_bytes']
 with gzip.open(z,'rb') as s:roundsha,roundbytes=digest_stream(s)
 assert (roundsha,roundbytes)==(rawsha,rawbytes)
 predsha,predbytes=digest(pd);assert predsha==row['full_predicate_gzip_sha256']
 with gzip.open(pd,'rb') as s:decodedsha,decodedbytes=digest_stream(s)
 assert decodedsha==row['decoded_predicate_sorted_json_sha256'] and decodedbytes==row['decoded_predicate_sorted_json_bytes']
 custody.append({'kind':row['kind'],'architecture':row['architecture'],'bundle_digest':row['bundle_digest'],'raw_path':str(raw),'raw_sha256':rawsha,'raw_bytes':rawbytes,'gzip_path':str(z),'gzip_sha256':gzsha,'gzip_bytes':gzbytes,'actual_gzip_full_roundtrip_verified':True,'raw_preserved':True,'predicate_gzip':str(pd),'predicate_gzip_sha256':predsha,'predicate_gzip_bytes':predbytes,'decompressed_canonical_predicate_sha256':decodedsha,'decompressed_canonical_predicate_bytes':decodedbytes})
assert len(custody)==5
write(CTL/'full-signed-bundle-and-predicate-custody.json',{'at':now,'scope':'Five original authoritative signed bundles plus complete lossless gzip and canonical predicate gzip; every gzip actually fully decompressed and hashed, all original raw files retained. No files removed.','bundles':custody})
packages={}
for arch in ['amd64','arm64']:
 row=next(x for x in r['signed_payloads'] if x['kind']=='sbom' and x['architecture']==arch)
 with gzip.open(row['full_predicate_gzip'],'rt') as s:spdx=json.load(s)
 installed=[{k:x.get(k) for k in ['name','versionInfo','SPDXID']} for x in spdx.get('packages',[]) if x.get('name') in {'node','nodejs','libbrotli1','setuptools','wheel'}]
 env=json.loads((CASE/(arch+'-config.json')).read_text())['config']['Env']
 assert 'NODE_VERSION=22.23.4' in env
 assert any(x['name']=='node' and x['versionInfo']=='22.23.4' for x in installed)
 assert any(x['name']=='libbrotli1' and x['versionInfo']=='1.0.9-2+deb12u1' for x in installed)
 packages[arch]={'spdx_package_count':len(spdx['packages']),'primary_ENV':'NODE_VERSION=22.23.4','selected_actual_spdx_packages':installed}
range_rows=[json.loads(p.read_text()) for p in sorted(CASE.glob('ranged-body-*.json'))]
assert len(range_rows)==len(fresh) and all(x['logical_bytes_delivered']==x['expected_size'] for x in range_rows)
range_events=[x for x in range_rows if len(x['attempts'])>1]
write(CTL/'body-get-chain.json',{'at':now,'case_attempt':1,'all_selected_full_blob_count':len(pulls),'all_selected_full_bytes_covered':sum(expected.values()),'new_complete_blob_count':len(fresh),'new_full_bytes_hashed':fresh_bytes,'genuine_prior_full_blob_count':len(reused),'genuine_prior_full_bytes_with_current_HEAD':reused_bytes,'cache_seed_or_prior_receipt_reuse':True,'prior_custody':str(CTL/'reuse-custody.json'),'body_GET_count':sum(len(x['attempts']) for x in range_rows),'ranged_blob_cases':range_events,'scope':'All canonical full-SHA/size receipt gates passed. Any range records retain actual responses without changing policy. No manual decompressed-layer proof is implied.'})
summary={'frozen_at':now,'result':'ONE_COMPLETE_STRICT_SUPPLIER_ARTIFACT_VALIDATION_EXIT0','root_artifact_review_pending':True,'case_attempt':1,'ref':r['ref'],'official_base':r['official_base'],'source':r['verification_workflow_sha'],'producer':r['producer_workflow_sha'],'producer_mode':r['producer_verification_provenance']['mode'],'run':'41128855169','job':124317237455,'actual_exit':process['actual_returncode'],'all_five_full_bundles_strict_PKI_tlog_TSA':True,'actual_wrong_source_and_subject_controls_rejected':True,'distinct_TUF_roots':len(tuf),'two_architecture_report_pairs_bound_independently':True,'unique_verified_full_blob_count':len(pulls),'unique_verified_full_blob_bytes':sum(expected.values()),'new_complete_full_blob_count':len(fresh),'new_complete_full_blob_bytes':fresh_bytes,'genuine_prior_full_blob_count':len(reused),'prior_receipt_reuse_bytes':reused_bytes,'actual_current_anonymous_HEAD_count':len(reused),'actual_body_GET_count':sum(len(x['attempts']) for x in range_rows),'actual_range_resume_cases':len(range_events),'manual_decompressed_layer_reads':r['new_manual_layer_reads'],'actual_package_findings_by_architecture':packages,'all_trivy_summaries_retained':r['trivy_summaries_by_architecture'],'resource_minimum_observed_free_bytes':process['minimum_observed_free_bytes'],'resource_maximum_observed_owned_bytes':process['maximum_observed_owned_bytes'],'resource_stop_reason':None,'full_raw_signed_bundles_preserved':True,'all_gzip_roundtrips_verified':True,'receipt_sha256':digest(CASE/'receipt.json')[0],'process_receipt_sha256':digest(OUT/'process-exit-case-attempt1.json')[0],'scope':'Published supplier identity/runtime/report/blob evidence only. All findings retained, no zero-CVE claim/waiver. Node22 supplier proof does not establish generated-image/hosted acceptance, catalog adoption or the unresolved common-base design. No catalog/adoption/repo/CI/Docker/resources changed. Eleven prior root-approved W8 suppliers and all historical rejected attempts remain unchanged.','executed_source_hashes':preflight['expected_source_hashes']}
write(CTL/'frozen-summary.json',summary)
roots={'output':OUT,'cache':CACHE,'controls':CTL};items={}
for label,root in roots.items():
 for p in sorted(root.rglob('*')):
  if p.is_symlink():raise RuntimeError('Unexpected symlink in owned frozen proof')
  if p.is_file() and p!=CTL/'FROZEN-MANIFEST.json':
   sha,size=digest(p);items[label+'/'+str(p.relative_to(root))]={'path':str(p),'sha256':sha,'bytes':size}
write(CTL/'FROZEN-MANIFEST.json',{'frozen_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'payload_files':len(items),'payload_bytes':sum(x['bytes'] for x in items.values()),'files':items,'manifest_excludes_only_itself':True,'process_complete_no_further_writes_expected':True})
print(json.dumps({'result':summary['result'],'receipt':str(CASE/'receipt.json'),'summary':str(CTL/'frozen-summary.json'),'summary_sha256':digest(CTL/'frozen-summary.json')[0],'manifest':str(CTL/'FROZEN-MANIFEST.json'),'manifest_sha256':digest(CTL/'FROZEN-MANIFEST.json')[0],'payload_files':len(items),'payload_bytes':sum(x['bytes'] for x in items.values()),'free_bytes':shutil.disk_usage(CTL).free}))
