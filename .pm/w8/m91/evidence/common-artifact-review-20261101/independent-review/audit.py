"""In-place audit of completed retained results; never invokes verifier/network."""
from pathlib import Path
import base64,collections,datetime,gc,gzip,hashlib,json,urllib.parse
R=Path(__file__).resolve().parent;ROOT=Path('/private/tmp/bex-w8-common-actual-review-run-20261101-qq32k8wf');E=ROOT/'evidence/native-common-bookworm-99cdc5c5a5b53629e2a98963d43d373511a91c1f';checks=[]
def digest_file(path,compressed=False):
 p=Path(path);assert p.is_file() and not p.is_symlink();h=hashlib.sha256();n=0
 with (gzip.open(p,'rb') if compressed else p.open('rb')) as f:
  while b:=f.read(1<<20):h.update(b);n+=len(b)
 return h.hexdigest(),n
def load(p):
 p=Path(p);assert p.stat().st_size<1<<20;return json.loads(p.read_bytes())
def instant(x):return datetime.datetime.fromisoformat(x.replace('Z','+00:00'))
result=load(ROOT/'RESULT.json');scope=load(ROOT/'SCOPE.json');config=load(ROOT/'EXECUTION.json');process=load(ROOT/'PROCESS.json');receipt=load(E/'receipt.json');plan=load(config['plan'])
assert result['exit']==0 and result['stop'] is None and result['source_unchanged'] is True and result['own_group_absent'] is True and result['cancelled_signal'] is None and result['candidate_approved'] is True
assert result['minimum_free']>=1207959552 and result['peak_owned_bytes']<608<<20 and result['final_observation']['owned']==result['peak_owned_bytes']
assert digest_file(ROOT/'review.log')[0]==result['log_sha256'];assert scope['entry']==config and scope['argv']==process['argv'] and process['pid']==process['pgid'] and type(process['pid']) is int
assert not load(ROOT/'anonymous-docker-config/config.json')['auths']
for name,expected in config['pinned_sources'].items():assert digest_file(name)[0]==expected
assert config['pinned_sources'][str(ROOT/'run.py')]=='6e25fd7f69ecca980ad4990fb8ac6f0e7ac833de8af10af0a2c7672e649898cd'
assert config['pinned_sources'][config['adapter']]=='4746a5022ab6df57ed95b6354d56b92eff15976d1737c2aa5a5ca15aa0080a4b'
assert config['pinned_sources'][config['plan']]=='ffc6b6ce1173691851e1de5e5a1f744fb599fc582c70511f2b56e82ea2c0f51e'
assert scope['remaining_floor']==1207959552 and scope['own_byte_cap']==640<<20 and scope['stop_own_bytes']==608<<20 and config['layer_workers']==1
assert config['projected_required_start_bytes']==1787765459 and scope['start_free']>=1787765459
assert receipt['ref']==config['ref']==plan['ref']=='ghcr.io/bex-co/bex-cnb-run@sha256:56c58abb41abd6becfa1810834c6cba46b742703e2beffdb3a88387772b3b986'
for k in ['approved_supplier_candidate','anonymous_manifest_bytes_sha_verified','strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified','strict_actual_Rekor_v2_and_RFC3161_TSA_verified','wrong_source_and_subject_negative_checks_rejected']:assert receipt[k] is True
assert receipt['verification_workflow_sha']==receipt['producer_workflow_sha']==plan['verification_sha']==plan['producer_sha']=='99cdc5c5a5b53629e2a98963d43d373511a91c1f'
assert receipt['new_manual_layer_reads']==0 and receipt['spdx_additional_verified_root_bindings']==[] and receipt['unaccepted_attachment_results']==[]
for name,expected in receipt['evidence_tool_sha256'].items():assert digest_file(name)[0]==expected and config['pinned_sources'][name]==expected
prov=receipt['producer_verification_provenance'];assert prov==load(E/'producer-verification-provenance.json') and prov['mode']=='fresh-build' and prov['generator_sha']==plan['generator_sha']=='8ae50548300ce79a06109e8b7e49dc1d19cf80ce'
assert prov['common_review_plan_sha256']==digest_file(config['plan'])[0]
eq=load(E/'common-source-equivalence.json');assert hashlib.sha256(json.dumps(eq,sort_keys=True).encode()).hexdigest()==prov['common_factory_equivalence_sha256'] and eq['factory_equivalent'] is True
for role in ['producer','verification','generator']:
 assert eq['roles'][role]=={'sha':plan[role+'_sha'],'factory_inputs':plan['factory_inputs']}
 index=load(E/'common-source'/(role+'-source-files.json'));assert index['sha']==plan[role+'_sha']
 for name,desc in index['files'].items():
  sha,size=digest_file(E/'common-source'/role/name);assert sha==desc['sha256'] and size==desc['bytes']
  if name in plan['factory_inputs']:assert desc==plan['factory_inputs'][name]
  if name in plan['selection_inputs']:assert desc==plan['selection_inputs'][name]
ci=receipt['ci_runtime_scan_signing_evidence'];assert ci==load(E/'ci-runtime-scan-signing-gates.json') and ci['run_id']=='41249441151' and ci['job_id']==124689058047 and ci['job_conclusion']=='success' and ci['primary_build_conclusion']=='success' and ci['recovery_selection_conclusion']=='skipped'
assert set(ci['verified_steps'])=={'expose the fully gated immutable image','scan amd64 candidate','scan arm64 candidate','sign and attest the immutable candidate','verify real native tool transformations without network on both architectures'}
assert digest_file(E/'index.json')[0]==receipt['ref'].split(':')[-1] and digest_file(E/'official-index.json')[0]==plan['base'].split(':')[-1]
actual_index=load(E/'index.json');expected_blobs={};configs={};identities={}
for arch,p in receipt['platforms'].items():
 m=load(E/(arch+'-manifest.json'));c=load(E/(arch+'-config.json'));configs[arch]=c
 assert digest_file(E/(arch+'-manifest.json'))[0]==p['manifest_digest'].split(':')[1] and digest_file(E/(arch+'-config.json'))[0]==p['config_digest'].split(':')[1]
 assert any(x['digest']==p['manifest_digest'] and x['platform']=={'architecture':arch,'os':'linux'} for x in actual_index['manifests'])
 assert c['architecture']==arch and c['os']=='linux' and m['config']['digest']==p['config_digest'] and len(c['rootfs']['diff_ids'])==len(m['layers'])
 oc=load(E/(arch+'-official-config.json'));om=load(E/(arch+'-official-manifest.json'));assert len(om['layers'])==len(oc['rootfs']['diff_ids']) and c['rootfs']['diff_ids'][:len(om['layers'])]==oc['rootfs']['diff_ids']
 assert p['official_base_ancestry']['verified_config_diff_id_prefix_matches'] is True and p['official_base_ancestry']['manual_decompressed_layer_hashing_performed'] is False
 for d in [m['config']]+m['layers']:
  if d['digest'] in expected_blobs:assert expected_blobs[d['digest']]==d['size']
  expected_blobs[d['digest']]=d['size']
 identities[p['manifest_digest']]=arch;identities[p['config_digest']]=arch
pull=receipt['anonymous_pull'];assert pull['unique_blob_count']==len(expected_blobs)==41 and pull['bytes_hashed_this_finalization']==sum(expected_blobs.values())==1522239095 and pull['bytes_reused_from_prior_verified_full_read']==0
observed={};body_attempts=collections.Counter()
for row in pull['blob_receipts']:
 assert row['schema']=='bex.native-anonymous-blob-pull/v2' and row['registry_repository']=='ghcr.io/bex-co/bex-cnb-run' and row['reused_prior_full_read'] is False
 assert row['digest'] not in observed and expected_blobs[row['digest']]==row['bytes_hashed']
 assert instant(scope['at'])<=instant(row['anonymous_full_get_at'])<=instant(result['at']);observed[row['digest']]=row['bytes_hashed']
 proof=load(E/('ranged-body-'+row['digest'].split(':')[1]+'.json'));assert proof['digest']==row['digest'] and proof['expected_size']==proof['logical_bytes_delivered']==row['bytes_hashed'] and proof['attempts'][-1]['outcome']=='complete'
 body_attempts[len(proof['attempts'])]+=1
assert observed==expected_blobs
crypto=load(E/'bundles/crypto-results.json');assert len(crypto['results'])==5 and set(crypto['accepted_bundles'])=={x['bundle_digest'] for x in receipt['signed_payloads']}
base=['/private/tmp/bex7-native-acceptance-novb586u/cosign','verify-blob-attestation']
identity='https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main';issuer='https://token.actions.githubusercontent.com'
tufs=set();command_rows=[]
def check_command(record,row,label):
 source=receipt['verification_workflow_sha'] if label!='wrong-source' else '0'*40;digest=receipt['ref'].split(':')[-1] if label!='wrong-subject' else '0'*64
 expected=base+['--bundle',row['path'],'--type',row['predicate_type'],'--digest',digest,'--digestAlg','sha256','--certificate-identity',identity,'--certificate-oidc-issuer',issuer,'--certificate-github-workflow-sha',source,'--use-signed-timestamps','--timeout','5m']
 assert record['command']==expected and record['bundle_digest']==row['bundle_digest'] and type(record['exit_code']) is int
 assert (record['exit_code']==0) is (label=='positive') and len(record['attempts'])==1
 attempt=record['attempts'][0];assert attempt['command']==expected and attempt['exit_code']==record['exit_code'];tuf=Path(attempt['tuf_cache']);assert tuf.is_relative_to(E/'bundles/cosign-tuf-cache') and tuf.is_dir() and str(tuf) not in tufs;tufs.add(str(tuf))
 err=Path(attempt['stderr']);text=err.read_text();out=err.with_name(err.name.replace('.stderr.log','.stdout.log'));assert out.is_file()
 if label=='positive':assert 'Verified OK' in text
 elif label=='wrong-source':assert 'expected GithubWorkflowSHA' in text and receipt['verification_workflow_sha'] in text
 else:assert 'provided artifact digest does not match any digest in statement' in text
 command_rows.append({'label':label,'bundle_digest':row['bundle_digest'],'exit':record['exit_code'],'command':expected,'stderr_sha256':digest_file(err)[0],'stderr_bytes':err.stat().st_size,'stdout_sha256':digest_file(out)[0],'stdout_bytes':out.stat().st_size,'tuf_root':str(tuf)})
encoder=json.JSONEncoder(sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False);payload_checks=[];findings={}
for row in receipt['signed_payloads']:
 p=Path(row['path']);raw_sha,raw_size=digest_file(p);assert 'sha256:'+raw_sha==row['bundle_digest'] and raw_size==row['bundle_bytes']
 bg=Path(row['full_bundle_gzip']);assert digest_file(bg)==(row['full_bundle_gzip_sha256'],row['full_bundle_gzip_bytes']) and digest_file(bg,True)==(raw_sha,raw_size)
 pg=Path(row['full_predicate_gzip']);assert digest_file(pg)[0]==row['full_predicate_gzip_sha256'] and digest_file(pg,True)==(row['decoded_predicate_sorted_json_sha256'],row['decoded_predicate_sorted_json_bytes'])
 with p.open('rb') as f:bundle=json.load(f)
 encoded=bundle['dsseEnvelope'].pop('payload');statement_raw=base64.b64decode(encoded,validate=True);del encoded
 assert hashlib.sha256(statement_raw).hexdigest()==row['signed_statement_sha256'];st=json.loads(statement_raw);del statement_raw
 assert st['predicateType']==row['predicate_type'] and any(x.get('digest',{}).get('sha256')==receipt['ref'].split(':')[-1] for x in st['subject'])
 vm=bundle['verificationMaterial'];assert vm['tlogEntries'] and all(x['kindVersion']=={'kind':'hashedrekord','version':'0.0.2'} for x in vm['tlogEntries']);assert len(vm['timestampVerificationData']['rfc3161Timestamps'])==row['tsa_timestamp_count']>=1
 pred=st['predicate'];hh=hashlib.sha256();n=0
 for token in encoder.iterencode(pred):b=token.encode();hh.update(b);n+=len(b)
 assert (hh.hexdigest(),n)==(row['decoded_predicate_sorted_json_sha256'],row['decoded_predicate_sorted_json_bytes'])
 manifest=load(E/'bundles'/(row['manifest_digest'].split(':')[1]+'.manifest.json'));assert digest_file(E/'bundles'/(row['manifest_digest'].split(':')[1]+'.manifest.json'))[0]==row['manifest_digest'].split(':')[1] and manifest['subject']['digest']==receipt['ref'].split('@')[1] and len(manifest['layers'])==1 and manifest['layers'][0]['digest']==row['bundle_digest'] and manifest['layers'][0]['size']==raw_size
 arch=row['architecture']
 if row['kind']=='sbom':
  roots=set(pred.get('documentDescribes',[]))|{x['relatedSpdxElement'] for x in pred.get('relationships',[]) if x.get('spdxElementId')==pred['SPDXID'] and x.get('relationshipType')=='DESCRIBES'}
  roots=[x for x in pred['packages'] if x['SPDXID'] in roots];assert len(roots)==1
  root_ids={'sha256:'+x['checksumValue'] for x in roots[0].get('checksums',[]) if x['algorithm']=='SHA256'};assert root_ids=={receipt['platforms'][arch]['manifest_digest']}
  arches=set()
  for package in pred['packages']:
   for x in package.get('externalRefs',[]):
    q=x.get('referenceLocator','')
    if q.startswith('pkg:deb/'):arches.update(a for a in urllib.parse.parse_qs(urllib.parse.urlsplit(q).query).get('arch',[]) if a in ('amd64','arm64'))
  assert arches=={arch} and len(pred['packages'])==receipt['package_summaries_by_architecture'][arch][0]['package_count']
  actual_brotli=[x.get('versionInfo') for x in pred['packages'] if x['name']=='libbrotli1'];assert actual_brotli==['1.0.9-2+deb12u1']
 elif row['kind']=='vulnerability':
  assert pred['ArtifactType']=='container_image' and pred['ArtifactName']==receipt['ref'] and pred['SchemaVersion']==2
  md=pred['Metadata'];assert md['ImageID']==receipt['platforms'][arch]['config_digest'] and md['ImageConfig']['architecture']==arch and md['ImageConfig']['os']=='linux' and md['ImageConfig']['rootfs']['diff_ids']==configs[arch]['rootfs']['diff_ids']
  counts=collections.Counter(v.get('Severity','UNKNOWN') for resultrow in pred['Results'] for v in resultrow.get('Vulnerabilities',[]));summary=receipt['trivy_summaries_by_architecture'][arch][0];assert sum(counts.values())==summary['vulnerability_count']==6887 and dict(counts)==summary['severity_counts'];findings[arch]={'count':sum(counts.values()),'severity_counts':dict(counts),'all_records_remain_in_full_signed_predicate':True}
 else:assert row['kind']=='image-signature' and arch is None
 positive=load(E/'bundles'/(row['bundle_digest'].split(':')[1]+'-positive.result.json'));check_command(positive,row,'positive')
 payload_checks.append({'kind':row['kind'],'architecture':arch,'bundle_digest':row['bundle_digest'],'bundle_bytes':raw_size,'full_bundle_gzip_sha256':row['full_bundle_gzip_sha256'],'full_predicate_gzip_sha256':row['full_predicate_gzip_sha256'],'canonical_predicate_sha256':hh.hexdigest(),'canonical_predicate_bytes':n,'signed_statement_sha256':row['signed_statement_sha256'],'subject_index':receipt['ref'],'tlog_kind_versions':row['tlog_kind_versions'],'TSA_count':row['tsa_timestamp_count'],'raw_gzip_and_payload_roundtrip_bound':True})
 del pred,st,bundle,vm;gc.collect()
image=next(x for x in receipt['signed_payloads'] if x['kind']=='image-signature')
for label in ['wrong-source','wrong-subject']:check_command(load(E/'bundles'/(image['bundle_digest'].split(':')[1]+'-'+label+'.result.json')),image,label)
assert len(tufs)==7 and len(payload_checks)==5 and set(findings)=={'amd64','arm64'}
policy=load(E/'local-resource-policy.json');assert all(policy[k]==v for k,v in plan['local_resource_policy'].items()) and policy['effective_layer_free_reserve']==1207959552
summary={'candidate_ref':receipt['ref'],'receipt_sha256':digest_file(E/'receipt.json')[0],'RESULT_sha256':digest_file(ROOT/'RESULT.json')[0],'SCOPE_sha256':digest_file(ROOT/'SCOPE.json')[0],'EXECUTION_sha256':digest_file(ROOT/'EXECUTION.json')[0],'pinned_sources':config['pinned_sources'],'wrapper_result':result,'CI_publication':ci,'producer_verifier_generator':prov,'official_base':receipt['official_base'],'platforms':receipt['platforms'],'anonymous_full_read_receipts':{'unique_count':41,'total_bytes':sum(observed.values()),'cache_borrowed':False,'all_digests_sizes_and_timestamps_match_actual_manifests':True,'body_attempt_counts':dict(body_attempts),'manual_decompressed_reads':0,'SPDX_registry_roots_direct':True,'note':'Layer bodies already streamed/discarded by original verifier; this audit checks actual receipts/source gates, not a new network read or independent layer rehash.'},'signed_payload_checks':payload_checks,'strict_actual_command_checks':command_rows,'findings_preserved':findings,'scope':'In-place actual-result audit only. No full-body copies, verification rerun, new network/crypto/body reads, caches or product changes.'}
raw=(json.dumps(summary,indent=2)+'\n').encode();assert len(raw)<1<<20;(R/'AUDIT.json').write_bytes(raw)
print(json.dumps({'audit_pass':True,'signed_payloads':5,'strict_commands':7,'anonymous_full_reads':41,'full_read_bytes':sum(observed.values()),'findings_per_arch':{k:v['count'] for k,v in findings.items()},'no_new_crypto_or_network':True}))
