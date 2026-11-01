"""In-place CURRENT0941 retained result audit; no external verifier/actions."""
from pathlib import Path
import base64,collections,datetime,gc,gzip,hashlib,json,urllib.parse
R=Path(__file__).resolve().parent
ROOT=Path('/private/tmp/bex-w8-common499-review-2ff8ba6f1f039942')
SOURCE=Path('/private/tmp/bex-w8-common0941-final-source-20261101-kxl6d0xc')
PLAN=Path('/private/tmp/bex-w8-common0941-populated-plan-20261101-2g823lha/PLAN.json')
E=ROOT/'evidence/native-common-bookworm-1feba939b4697894e800ed0abb4f25a0bf415641'
REF='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
SHA='1feba939b4697894e800ed0abb4f25a0bf415641'
BINARY=Path('/private/tmp/bex-w8-cosign-linux-verified-20261101-iqdmo3bn/cosign-linux-arm64')
BINARY_SHA='ade83b7aac62e131cacfe96d958948b2150cc9efec46a570a2649b8e27dedad2'
IMAGE='sha256:ab3fe4defd29ba6231229a4d41440ac8bde8218e85870e53876277faa24b35c4'
custody={}
def digest_file(path,compressed=False):
 p=Path(path);assert p.is_file() and not p.is_symlink();h=hashlib.sha256();n=0
 with (gzip.open(p,'rb') if compressed else p.open('rb')) as f:
  while b:=f.read(1<<20):h.update(b);n+=len(b)
 if not compressed:custody[str(p)]={'sha256':h.hexdigest(),'bytes':n}
 return h.hexdigest(),n
def load(p):
 p=Path(p);assert p.is_file() and not p.is_symlink() and p.stat().st_size<1<<20
 digest_file(p);return json.loads(p.read_bytes())
def instant(x):return datetime.datetime.fromisoformat(x.replace('Z','+00:00'))
result=load(ROOT/'RESULT.json');scope=load(ROOT/'SCOPE.json');process=load(ROOT/'PROCESS.json');cleanup=load(ROOT/'CLEANUP.json');receipt=load(E/'receipt.json');plan=load(PLAN)
assert result['exit']==0 and result['stop'] is None and result['cancelled_signal'] is None and result['candidate_approved'] is True and result['errors']==[]
assert result['minimum_free']>=1207959552 and result['peak_owned_bytes']<608<<20 and result['final_observation']['owned']==result['peak_owned_bytes']
assert result['process_cleanup']==cleanup['group'] and cleanup['group']=={'exit':0,'group_absent':True,'pgid':process['pgid'],'pid':process['pid'],'reaped':True}
assert cleanup['errors']==[] and result['container_cleanup']==cleanup['container']
assert scope['command']==process['command'] and process['pid']==process['pgid'] and type(process['pid']) is int
assert (ROOT/'anonymous-docker-config/config.json').read_bytes()==b'{"auths":{}}\n'
owner=scope['owner'];name='bex-w8-common499-cosign-'+owner
def absent(row):
 assert row['argv']==['docker','container','inspect',name] and row['complete'] is True and type(row['exit']) is int and row['exit']==1
 assert row['stdout']=='[]\n' and row['stderr']=='Error response from daemon: No such container: '+name+'\n'
def cleaned(c):
 assert c['absence_proved'] is True and c['errors']==[] and c['owner']==owner and c['name']==name
 absent(c['after'])
 if c['remove'] is not None:
  assert c['remove']=={'argv':['docker','rm','--force',name],'complete':True,'exit':0,'stderr':'','stdout':name+'\n'}
cleaned(cleanup['container'])
assert scope['source_manifest_sha256']==digest_file(SOURCE/'MANIFEST.json')[0]=='bc43ca57b0ffd3e73f9669fb57bfade9f03a88b2b1d7b6d96263880d8e3b6d1b'
for file,row in load(SOURCE/'MANIFEST.json').items():assert digest_file(SOURCE/file)==(row['sha256'],row['bytes'])
assert scope['plan_sha256']==digest_file(PLAN)[0]=='e86f10b0ed4757620db780c266b3e96ecc5ece8f753ddf4b536fca3fd3c5bc81'
for k,d in scope['sizing_inputs'].items():assert digest_file(d['path'])==(d['sha256'],d['bytes'])
assert scope['remaining_floor_bytes']==1207959552 and scope['owned_cap_bytes']==640<<20 and scope['stop_owned_bytes']==608<<20 and scope['layer_workers']==1 and scope['maximum_live_crypto_containers']==1 and scope['crypto_memory_bytes']==512<<20
assert receipt['ref']==result['ref']==scope['candidate_ref']==plan['ref']==REF
for k in ['approved_supplier_candidate','anonymous_manifest_bytes_sha_verified','strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified','strict_actual_Rekor_v2_and_RFC3161_TSA_verified','wrong_source_and_subject_negative_checks_rejected']:assert receipt[k] is True
assert receipt['verification_workflow_sha']==receipt['producer_workflow_sha']==plan['verification_sha']==plan['producer_sha']==SHA
assert receipt['new_manual_layer_reads']==0 and receipt['spdx_additional_verified_root_bindings']==[] and receipt['unaccepted_attachment_results']==[]
for path,expected in receipt['evidence_tool_sha256'].items():assert digest_file(path)[0]==expected
prov=receipt['producer_verification_provenance'];assert prov==load(E/'producer-verification-provenance.json') and prov['mode']=='fresh-build' and prov['generator_sha']==plan['generator_sha']=='f071830616b2f55aa8636bb856454f4a218a9174'
assert prov['common_review_plan_sha256']==digest_file(PLAN)[0]
eq=load(E/'common-source-equivalence.json');assert hashlib.sha256(json.dumps(eq,sort_keys=True).encode()).hexdigest()==prov['common_factory_equivalence_sha256'] and eq['factory_equivalent'] is True
for role in ['producer','verification','generator']:
 assert eq['roles'][role]=={'sha':plan[role+'_sha'],'factory_inputs':plan['factory_inputs']}
 index=load(E/'common-source'/(role+'-source-files.json'));assert index['sha']==plan[role+'_sha']
 for path,desc in index['files'].items():
  assert digest_file(E/'common-source'/role/path)==(desc['sha256'],desc['bytes'])
  if path in plan['factory_inputs']:assert desc==plan['factory_inputs'][path]
  if path in plan['selection_inputs']:assert desc==plan['selection_inputs'][path]
ci=receipt['ci_runtime_scan_signing_evidence'];assert ci==load(E/'ci-runtime-scan-signing-gates.json') and ci['run_id']=='41341287851' and ci['job_id']==124954099391 and ci['job_conclusion']=='success' and ci['primary_build_conclusion']=='success' and ci['recovery_selection_conclusion']=='skipped'
assert set(ci['verified_steps'])=={'expose the fully gated immutable image','scan amd64 candidate','scan arm64 candidate','sign and attest the immutable candidate','verify real native tool transformations without network on both architectures'}
for field in ('runs','jobs'):
 desc=ci['actual_metadata_inputs'][field];assert digest_file(desc['path'])==(desc['sha256'],desc['bytes'])
scope_start=datetime.datetime.fromtimestamp((ROOT/'SCOPE.json').stat().st_mtime,datetime.timezone.utc)
reviewed_at=instant(receipt['reviewed_at'])
assert digest_file(BINARY)==(BINARY_SHA,136437761)
identity='https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main';issuer='https://token.actions.githubusercontent.com'
tufs=set();command_rows=[]
def check_command(record,row,label):
 source=SHA if label!='wrong-source' else '0'*40;digest=REF.split(':')[-1] if label!='wrong-subject' else '0'*64
 inner=['/usr/local/bin/cosign','verify-blob-attestation','--bundle','/bundle.json','--type',row['predicate_type'],'--digest',digest,'--digestAlg','sha256','--certificate-identity',identity,'--certificate-oidc-issuer',issuer,'--certificate-github-workflow-sha',source,'--use-signed-timestamps','--timeout','5m']
 assert len(record['attempts'])==1 and type(record['exit_code']) is int and record['exit_code']==(0 if label=='positive' else 1)
 attempt=record['attempts'][0];tuf=Path(attempt['tuf_cache']);assert tuf.is_relative_to(E/'bundles/cosign-tuf-cache') and tuf.is_dir() and str(tuf) not in tufs;tufs.add(str(tuf))
 outer=['docker','run','--pull=never','--name',name,'--label','bex.w8.owned='+owner,'--read-only','--user','501:20','--cap-drop','ALL','--security-opt','no-new-privileges','--cpus','1','--memory','512m','--pids-limit','64','--network','bridge','--tmpfs','/tmp:rw,noexec,nosuid,size=16m','--mount','type=bind,source='+str(BINARY)+',target=/usr/local/bin/cosign,readonly','--mount','type=bind,source='+row['path']+',target=/bundle.json,readonly','--mount','type=bind,source='+str(tuf)+',target=/tuf','--mount','type=bind,source='+str(ROOT/'anonymous-docker-config')+',target=/docker,readonly','--env','TUF_ROOT=/tuf','--env','DOCKER_CONFIG=/docker','--env','XDG_CACHE_HOME=/tuf/cache','--env','GODEBUG=http2client=0','--entrypoint','/usr/local/bin/cosign',IMAGE,*inner[1:]]
 for rec in (record,attempt):
  assert rec['command']==outer and rec['cosign_argv']==inner and rec['tool_sha256']==BINARY_SHA and rec['container_image_id']==IMAGE and rec['container_cleanup_proved'] is True and rec['bundle_digest']==row['bundle_digest'] and rec['exit_code']==record['exit_code']
 raw=load(attempt['raw_process_receipt']);assert raw['command']==outer and raw['cosign_argv']==inner and raw['complete'] is True and raw['error_class'] is None and raw['process_exit']==record['exit_code'] and raw['owner']==owner and raw['tool_sha256']==BINARY_SHA and raw['container_image_id']==IMAGE
 cleaned(raw['cleanup']);obj=raw['container'];cfg=obj['Config'];hc=obj['HostConfig'];state=obj['State']
 assert obj['Name']=='/'+name and obj['Image']==IMAGE and cfg['Labels']['bex.w8.owned']==owner and cfg['Entrypoint']==['/usr/local/bin/cosign'] and cfg['Cmd']==inner[1:] and obj['Args']==inner[1:] and cfg['User']=='501:20'
 assert hc['Memory']==512<<20 and hc['NanoCpus']==1_000_000_000 and hc['PidsLimit']==64 and hc['ReadonlyRootfs'] is True and hc['CapDrop']==['ALL'] and hc['Privileged'] is False and hc['NetworkMode']=='bridge' and hc['SecurityOpt']==['no-new-privileges']
 assert state['Status']=='exited' and state['Running'] is False and type(state['ExitCode']) is int and state['ExitCode']==record['exit_code'] and state['OOMKilled'] is False and not state['Error']
 binds={m['Destination']:(m['Source'],m['RW']) for m in obj['Mounts'] if m['Type']=='bind'}
 assert binds=={'/usr/local/bin/cosign':(str(BINARY),False),'/bundle.json':(row['path'],False),'/tuf':(str(tuf),True),'/docker':(str(ROOT/'anonymous-docker-config'),False)}
 assert raw['cleanup']['container_id']==obj['Id'] and raw['cleanup']['before']['exit']==0 and json.loads(raw['cleanup']['before']['stdout'])[0]==obj
 err=Path(attempt['stderr']);text=err.read_text();out=err.with_name(err.name.replace('.stderr.log','.stdout.log'));assert out.is_file()
 if label=='positive':assert 'Verified OK' in text
 elif label=='wrong-source':assert 'expected GithubWorkflowSHA' in text and SHA in text
 else:assert 'provided artifact digest does not match any digest in statement' in text
 tuf_rows=[]
 for p in sorted(tuf.rglob('*')):
  if p.is_file():assert not p.is_symlink();h,n=digest_file(p);tuf_rows.append({'path':str(p),'sha256':h,'bytes':n})
 assert tuf_rows
 command_rows.append({'label':label,'bundle_digest':row['bundle_digest'],'exit':record['exit_code'],'outer_command':outer,'inner_command':inner,'raw_process_receipt':attempt['raw_process_receipt'],'process_receipt_sha256':digest_file(attempt['raw_process_receipt'])[0],'container_id':obj['Id'],'started':state['StartedAt'],'finished':state['FinishedAt'],'stderr_sha256':digest_file(err)[0],'stderr_bytes':err.stat().st_size,'stdout_sha256':digest_file(out)[0],'stdout_bytes':out.stat().st_size,'tuf_root':str(tuf),'tuf_files':tuf_rows,'own_cleanup_proved':True})

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
pull=receipt['anonymous_pull'];assert pull['unique_blob_count']==len(expected_blobs)==41 and pull['bytes_hashed_this_finalization']==sum(expected_blobs.values())==1522239193 and pull['bytes_reused_from_prior_verified_full_read']==0
observed={};body_attempts=collections.Counter()
for row in pull['blob_receipts']:
 assert row['schema']=='bex.native-anonymous-blob-pull/v2' and row['registry_repository']=='ghcr.io/bex-co/bex-cnb-run' and row['reused_prior_full_read'] is False
 assert row['digest'] not in observed and expected_blobs[row['digest']]==row['bytes_hashed']
 assert scope_start<=instant(row['anonymous_full_get_at'])<=reviewed_at;observed[row['digest']]=row['bytes_hashed']
 proof=load(E/('ranged-body-'+row['digest'].split(':')[1]+'.json'));assert proof['digest']==row['digest'] and proof['expected_size']==proof['logical_bytes_delivered']==row['bytes_hashed'] and proof['attempts'][-1]['outcome']=='complete'
 body_attempts[len(proof['attempts'])]+=1
assert observed==expected_blobs

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
  counts=collections.Counter(v.get('Severity','UNKNOWN') for resultrow in pred['Results'] for v in resultrow.get('Vulnerabilities',[]));summary=receipt['trivy_summaries_by_architecture'][arch][0];assert sum(counts.values())==summary['vulnerability_count']==6920 and dict(counts)==summary['severity_counts'];findings[arch]={'count':sum(counts.values()),'severity_counts':dict(counts),'all_records_remain_in_full_signed_predicate':True}
 else:assert row['kind']=='image-signature' and arch is None
 positive=load(E/'bundles'/(row['bundle_digest'].split(':')[1]+'-positive.result.json'));check_command(positive,row,'positive')
 payload_checks.append({'kind':row['kind'],'architecture':arch,'bundle_digest':row['bundle_digest'],'bundle_bytes':raw_size,'full_bundle_gzip_sha256':row['full_bundle_gzip_sha256'],'full_predicate_gzip_sha256':row['full_predicate_gzip_sha256'],'canonical_predicate_sha256':hh.hexdigest(),'canonical_predicate_bytes':n,'signed_statement_sha256':row['signed_statement_sha256'],'subject_index':receipt['ref'],'tlog_kind_versions':row['tlog_kind_versions'],'TSA_count':row['tsa_timestamp_count'],'raw_gzip_and_payload_roundtrip_bound':True})
 del pred,st,bundle,vm;gc.collect()
image=next(x for x in receipt['signed_payloads'] if x['kind']=='image-signature')
for label in ['wrong-source','wrong-subject']:check_command(load(E/'bundles'/(image['bundle_digest'].split(':')[1]+'-'+label+'.result.json')),image,label)
assert len(tufs)==7 and len(payload_checks)==5 and set(findings)=={'amd64','arm64'}


policy=load(E/'local-resource-policy.json');assert all(policy[k]==v for k,v in plan['local_resource_policy'].items()) and policy['effective_layer_free_reserve']==1207959552
life=load(ROOT/'linux-cosign-lifecycle.json');assert life['pending'] is False and life['last_process_complete'] is True and life['owner']==owner;cleaned(life['cleanup'])
intervals=sorted((instant(x['started']),instant(x['finished'])) for x in command_rows)
assert all(a<=b for a,b in intervals) and all(intervals[i][1]<=intervals[i+1][0] for i in range(len(intervals)-1))
assert len({x['container_id'] for x in command_rows})==7
summary={'candidate_ref':REF,'receipt_sha256':digest_file(E/'receipt.json')[0],'RESULT_sha256':digest_file(ROOT/'RESULT.json')[0],'SCOPE_sha256':digest_file(ROOT/'SCOPE.json')[0],'source_manifest_sha256':scope['source_manifest_sha256'],'plan_sha256':digest_file(PLAN)[0],'wrapper_result':result,'CI_publication':ci,'producer_verifier_generator':prov,'official_base':receipt['official_base'],'platforms':receipt['platforms'],'anonymous_full_read_receipts':{'unique_count':41,'total_bytes':sum(observed.values()),'reused_bytes':0,'all_digests_sizes_and_timestamps_match_actual_manifests':True,'body_attempt_counts':dict(body_attempts),'manual_decompressed_reads':0,'SPDX_registry_roots_direct':True,'limitation':'Layer bodies were streamed/discarded by original verifier; this audit checks current complete-read receipts and source gates, not another network read or independent layer rehash.'},'signed_payload_checks':payload_checks,'strict_actual_Linux_command_checks':command_rows,'findings_preserved':findings,'serial_crypto_container_intervals_proved':True,'scope':'Current0941 supplier TEST candidate identity and actual signed report/custody evidence only. No production/generated/27-family/hosted/adoption proof. No new external action.'}
(R/'AUDIT.json').write_text(json.dumps(summary,indent=2)+'\n')
(R/'INPUT-CUSTODY.json').write_text(json.dumps(custody,indent=2)+'\n')
print(json.dumps({'PASS':True,'signed_payloads':len(payload_checks),'strict_Linux_commands':len(command_rows),'anonymous_full_reads':len(observed),'full_read_bytes':sum(observed.values()),'retained_findings_per_arch':{k:v['count'] for k,v in findings.items()},'input_custody_files':len(custody),'network_crypto_Docker_repeated':False}))
