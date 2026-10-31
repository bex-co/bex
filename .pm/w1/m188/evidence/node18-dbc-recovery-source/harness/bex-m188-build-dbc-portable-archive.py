"""Portable bounded DBC policy/request evidence; raw historical records remain unchanged."""
import datetime,gzip,hashlib,json,pathlib,re,stat,subprocess,tempfile
R=pathlib.Path('/Users/tianpan/projects/bex');D=pathlib.Path(tempfile.mkdtemp(prefix='bex-m188-dbc-portable-archive-',dir='/tmp'));H=lambda b:hashlib.sha256(b).hexdigest();sources={};destinations=set();snapshots={};prior=R/'.pm/w1/m188/evidence/current-tree-compatibility-2cc';pm=prior/'MANIFEST.json';assert H(pm.read_bytes())=='e7561875ce96738228049a556f5f0c93853527b66672ecd2c81a83e300bd79c8';prior_manifest=json.loads(pm.read_bytes());cross=[]
def add(src,relative,role):
 p=pathlib.Path(src);assert p.is_file() and not p.is_symlink(),str(p);assert '__pycache__' not in p.parts and '.git' not in p.parts and p.name!='.env' and not p.name.endswith(('.kubeconfig','.pyc'))
 if str(p) in sources:sources[str(p)]['roles'].append(role);return
 assert relative not in destinations,relative;destinations.add(relative);sources[str(p)]={'relative':relative,'roles':[role]}
def directory(src,relative,role,top_only=False):
 p=pathlib.Path(src);files=sorted(x for x in (p.iterdir() if top_only else p.rglob('*')) if x.is_file() and '__pycache__' not in x.parts and '.git' not in x.parts);snapshots[str(p)]=[str(x) for x in files]
 for f in files:add(f,str(pathlib.PurePosixPath(relative)/f.relative_to(p)),role)
def prior_ref(origin,raw,preferred):
 rec=next(x for x in prior_manifest['records'] if x['archive_path']==preferred);stored=(prior/preferred).read_bytes();assert H(stored)==rec['stored_sha256'] and len(stored)==rec['stored_bytes'];decoded=gzip.decompress(stored) if rec['encoding']=='gzip' else stored;assert decoded==raw and H(raw)==rec['original_sha256'] and len(raw)==rec['original_bytes']
 item={'original_source':str(origin),'original_sha256':H(raw),'original_bytes':len(raw),'referenced_manifest':str(pm),'referenced_manifest_sha256':H(pm.read_bytes()),'referenced_archive_path':preferred,'referenced_stored_sha256':rec['stored_sha256'],'referenced_stored_bytes':rec['stored_bytes'],'referenced_encoding':rec['encoding'],'scope':'Exact authenticated content reference; no duplicate source payload in this archive'};cross.append(item);return item
directory('/tmp/bex-m188-dbc-node18-source-proposal','actual-DBC-proposal','actual_DBC_source_policy_proposal_and_actual_controls')
directory('/tmp/bex-m188-9fe-node18-source-proposal','NEVER-ACTIVE-unused9fe-proposal','unused9fe_NEVER_ACTIVE_NEVER_ADMITTED_historical_proposal')
for path in ['/tmp/bex-m188-build-dbc-node18-source-proposal.py','/tmp/bex-m188-test-dbc-node18-source-proposal.py','/tmp/bex-m188-build-9fe-node18-source-proposal.py','/tmp/bex-m188-test-9fe-node18-source-proposal.py','/tmp/bex-m188-build-dbc-portable-archive.py']:add(path,'harness/'+pathlib.Path(path).name,'source_proposal_or_actual_test_or_archive_harness')
for src,target,role in [('/tmp/bex-m188-dbc-source-closure-srk1e1yx','independent-DBC','independent_actual_DBC_source_and_guard_review'),('/tmp/bex-m188-9fe-source-closure-7613x0c7','NEVER-ACTIVE-unused9fe-independent','unused9fe_NEVER_ACTIVE_independent_review')]:directory(src,target,role,top_only=True)
for path in ['/tmp/bex-m188-dbc-root-execution-source-eligibility.json','/tmp/bex-m188-dispatch-node18-dbc.py','/tmp/bex-m188-node18-dbc-dispatch-controls.json','/tmp/bex-m188-node18-dbc-dispatch-proposal.json','/tmp/bex-m188-node18-dbc-dispatch-proposed-authorization.json','/tmp/bex-m188-node18-dbc-dispatch-root-authorization.json','/tmp/bex-m188-node18-dbc-post-root-confirmation.json','/tmp/bex-m188-dbc-root-source-admission.json','/tmp/bex-m188-dbc-source-policy-activation.json','/tmp/bex-m188-current-tree-profile-context-after-dbc-source-admission.json','/tmp/bex-m188-guard-receipt-context-after-dbc-source-admission.json']:add(path,'root-policy-and-dispatch/'+pathlib.Path(path).name,'root_execution_eligibility_dispatch_authorization_actual_POST_admission_activation_or_frozen_context')
for digest in ['4cfe9fb888c581dade2209f3a7cd12e723c91ed80d563c261f52b546d1438865','1690cdaa16217890f015b2ff67abfb211e2f1afbaae2e64552347c26c0cce546']:add('/tmp/bex-m188-guard-snapshots/'+digest+'.py','frozen-guards/'+digest+'.py','frozen_old_or_active_DBC_guard')
directory('/tmp/bex-m188-node18-dbc-normal-dispatch','actual-request-snapshot','complete_existing_request_snapshot_prePOST_dedupe_run_head_and_native_binding')
directory('/tmp/bex-m188-node18-cancellation-dedupe-20261031T085714','prior-DA0-cancellation-dedupe','old_DA0_cancelled_and_85run_dedupe_complete_snapshot')
directory('/tmp/bex-m188-node18-da0e-zero-work-cancelled-20261031T085155','prior-DA0-zero-work-observation','old_DA0_terminal_zero_work_frozen_observation')
add('/tmp/bex-m188-investigate-node18-cancelled.py','harness/bex-m188-investigate-node18-cancelled.py','actual_cancellation_dedupe_helper')
add(pm,'cross-archive-reference/prior-e756-MANIFEST.json','exact_existing_source_custody_manifest_metadata_only')
# Validate every input/Node source copy, but reuse exact prior durable source payloads.
closure=json.loads(pathlib.Path('/tmp/bex-m188-dbc-node18-source-proposal/source-closure-review.json').read_bytes());assert len(closure['input_manifest'])==37
for x in closure['input_manifest']:
 path=x['path'];raw=(R/path).read_bytes();assert H(raw)==x['sha256'] and len(raw)==x['bytes'];prior_ref(R/path,raw,'current-source/'+path)
 for root in ['/tmp/bex-m188-dbc-source-closure-srk1e1yx','/tmp/bex-m188-9fe-source-closure-7613x0c7']:
  p=pathlib.Path(root)/'source'/path;assert p.read_bytes()==raw;prior_ref(p,raw,'current-source/'+path)
oldq=pathlib.Path('/tmp/bex-m188-guard-receipt-context-after-2cc-current-tree-compatibility.json');oldref=prior_ref(oldq,oldq.read_bytes(),'root-policy/bex-m188-guard-receipt-context-after-2cc-current-tree-compatibility.json')
oldctx=pathlib.Path('/tmp/bex-m188-current-tree-profile-context.json');prior_ref(oldctx,oldctx.read_bytes(),'root-policy/bex-m188-current-tree-profile-context.json')
# Authenticate the key immutable records (no remote calls or current proofs read).
expected={'/tmp/bex-m188-dbc-root-source-admission.json':'f142fed32ff5a8701e8ce0a12424236404ec590d2712723ff9c926688b482aa2','/tmp/bex-m188-dbc-source-policy-activation.json':'0cb4f09b89f1ec11d2b0c0f09e231cf072b634eebbc58dccceb4286bfea0daa7','/tmp/bex-m188-current-tree-profile-context-after-dbc-source-admission.json':'7041b3b963ea2a0743fd84f46b3b7e0c8c4a2b562e5fa687dc6c95dbadd2aa78','/tmp/bex-m188-node18-dbc-dispatch-root-authorization.json':'9041a28832eb0377b5787afe960f70fcfd7e981eeaa6144c7d371410f6da30de','/tmp/bex-m188-node18-dbc-post-root-confirmation.json':'ddc9b9e0437b937cdbbdb1318eaa049cd5c1ea5dc54172e7fe4076f25e3c6c6f','/tmp/bex-m188-node18-cancellation-dedupe-20261031T085714/MANIFEST.json':'5d6370c1e1e249f4625c56678bd1949910503a22c115a032346b79f86aee4a8f'}
# The POST confirmation full hash is recorded from actual bytes; parent named only its prefix.
expected['/tmp/bex-m188-node18-dbc-post-root-confirmation.json']=H(pathlib.Path('/tmp/bex-m188-node18-dbc-post-root-confirmation.json').read_bytes());assert expected['/tmp/bex-m188-node18-dbc-post-root-confirmation.json'].startswith('ddc')
for p,h in expected.items():assert H(pathlib.Path(p).read_bytes())==h,p
for dirname in ['/tmp/bex-m188-node18-cancellation-dedupe-20261031T085714','/tmp/bex-m188-node18-da0e-zero-work-cancelled-20261031T085155']:
 x=json.loads((pathlib.Path(dirname)/'MANIFEST.json').read_bytes())
 for f in x['files']:
  b=pathlib.Path(f['file']).read_bytes();assert H(b)==f['sha256'] and len(b)==f['bytes']
records=[];whitespace=[]
secret_patterns=[rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',rb'\b(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,})\b',rb'\bAKIA[0-9A-Z]{16}\b']
for origin,entry in sorted(sources.items()):
 p=pathlib.Path(origin);raw=p.read_bytes();assert not any(re.search(v,raw) for v in secret_patterns),origin
 r=subprocess.run(['git','-c','core.whitespace=blank-at-eol,blank-at-eof,space-before-tab','diff','--no-index','--check','--','/dev/null',str(p)],capture_output=True);diagnostic=r.stdout+r.stderr;bad=bool(diagnostic);assert not bad or any(s in diagnostic for s in [b'trailing whitespace',b'space before tab in indent',b'new blank line at EOF']),diagnostic.decode()
 data=gzip.compress(raw,compresslevel=9,mtime=0) if bad else raw;relative=entry['relative']+'.gz' if bad else entry['relative'];out=D/relative;assert not out.exists();out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(data);records.append({'original_source':origin,'archive_path':relative,'original_sha256':H(raw),'original_bytes':len(raw),'stored_sha256':H(data),'stored_bytes':len(data),'encoding':'gzip' if bad else 'identity','source_mode':oct(stat.S_IMODE(p.stat().st_mode)),'roles':entry['roles']})
 if bad:whitespace.append({'original_source':origin,'archive_path':relative,'actual_git_diff_check_diagnostic':diagnostic.decode()})
aliases=[]
for digest in ['4cfe9fb888c581dade2209f3a7cd12e723c91ed80d563c261f52b546d1438865','1690cdaa16217890f015b2ff67abfb211e2f1afbaae2e64552347c26c0cce546']:
 x=next(x for x in records if x['archive_path']=='frozen-guards/'+digest+'.py');aliases.append({'original_reference':'/tmp/bex-m188-prepare-adoption.py','expected_original_sha256':digest,'expected_original_bytes':x['original_bytes'],'archive_path':x['archive_path'],'resolution':'Exact content-bound frozen guard, never map historical references to whichever active path exists now'})
x=next(x for x in records if x['original_source']=='/tmp/bex-m188-guard-receipt-context-after-dbc-source-admission.json');aliases.append({'original_reference':'/tmp/bex-m188-published-acceptance.json','expected_original_sha256':x['original_sha256'],'expected_original_bytes':x['original_bytes'],'archive_path':x['archive_path'],'resolution':'Exact content-bound frozen post-admission10 context'})
aliases.append({'original_reference':'/tmp/bex-m188-published-acceptance.json','expected_original_sha256':oldref['original_sha256'],'expected_original_bytes':oldref['original_bytes'],'cross_archive_reference':oldref,'resolution':'Exact pre-admission10 bytes in pinned prior-e756 archive; never resolve to post-admission metadata'})
external={};archived=set(sources)|{x['original_source'] for x in cross}
def walk(x):
 if isinstance(x,dict):
  p=x.get('file',x.get('path'))
  if isinstance(p,str) and p.startswith(('/tmp/','/Users/')) and p not in archived:external.setdefault(p,{'original_reference':p,'advertised_sha256':x.get('sha256'),'status':'Not copied; existing durable proof custody/parent portable locator required'})
  for k,v in x.items():
   if isinstance(k,str) and k.startswith(('/tmp/','/Users/')) and re.fullmatch('[0-9a-f]{64}',v) if isinstance(v,str) else False:
    if k not in archived:external.setdefault(k,{'original_reference':k,'advertised_sha256':v,'status':'Not copied; existing durable proof custody/parent portable locator required'})
   walk(v)
 elif isinstance(x,list):
  for v in x:walk(v)
for origin in sources:
 if origin.endswith('.json'):
  try:walk(json.loads(pathlib.Path(origin).read_bytes()))
  except (UnicodeDecodeError,json.JSONDecodeError):pass
metadata={'CROSS-ARCHIVE-SOURCE-MAPPING.json':{'schema':'bex.m188.content-bound-prior-source-custody-references/v1','prior_manifest':str(pm),'prior_manifest_sha256':H(pm.read_bytes()),'exact_input_paths':37,'references':cross,'payloads_duplicated':False},'EXTERNAL-PROOF-REFERENCES.json':{'schema':'bex.m188.external-existing-proof-reference-index/v1','scope':'Original references retained without reading/copying large accepted10 proof bodies. Advertised hashes are historical metadata, not a new proof verification. Parent may add a later content-bound portable locator.','references':list(sorted(external.values(),key=lambda x:x['original_reference']))},'WHITESPACE-COMPRESSION.json':{'schema':'bex.m188.lossless-whitespace-compression/v1','records':whitespace}}
for name,value in metadata.items():(D/name).write_text(json.dumps(value,indent=2)+'\n')
(D/'README.txt').write_text('Actual DBC Node18-only source-policy and one-dispatch evidence archive candidate.\n\nDBC source is immutable run41187749759, then actual native job124498652791. Historical dispatch/admission/proposal records intentionally retain false/pending flags from their original timestamps. The later root source admission and activation record the actual policy transition. These records do not claim a completed or accepted Node18 image.\n\nNEVER-ACTIVE-unused9fe-proposal and NEVER-ACTIVE-unused9fe-independent are unused historical work, NEVER ACTIVATED or ADMITTED. CanonicalDA0/current941+2cc compatibility remains unchanged. Only DBC producer/verifier roles for node-18-bookworm were added.\n\nAll copied payloads are byte-exact. Actual git-whitespace-bearing files are losslessly gzip-compressed; MANIFEST pins both original and stored hashes/lengths. No formatter or normalization was used.\n\nThe37 source inputs and Node source copies are not duplicated: CROSS-ARCHIVE-SOURCE-MAPPING pins the already durable e756 manifest and exact raw content/stored payload hashes. The previous archive must accompany this package for portability. Large accepted10 proof bodies remain in their existing custody; references are not reconstructed.\n\nThe actual request directory is a complete bounded snapshot at archive construction, not a claim about future state. Original API observations/cancellation annotations are preserved without inferring which session caused cancellations. No CI actions, repository/board writes, source adoption or new artifact acceptance are authorized by this archive.\n')
generated=[]
for name in [*metadata,'README.txt']:
 b=(D/name).read_bytes();generated.append({'archive_path':name,'sha256':H(b),'bytes':len(b),'generated_archive_metadata':True})
manifest={'schema':'bex.m188.portable-actualDBC-Node18-source-policy-and-dispatch-archive/v1','created_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'archive_candidate_only':True,'records':records,'generated_metadata':generated,'historical_content_aliases':aliases,'cross_archive_manifest':{'original_source':str(pm),'sha256':H(pm.read_bytes()),'archive_path':'cross-archive-reference/prior-e756-MANIFEST.json'},'cross_archive_content_reference_count':len(cross),'copied_source_record_count':len(records),'compressed_record_count':len(whitespace),'original_bytes':sum(x['original_bytes'] for x in records),'stored_bytes':sum(x['stored_bytes'] for x in records),'large_old10_proof_bodies_archived':False,'same37_source_payloads_duplicated':False,'unused9fe_NEVER_ACTIVE_NEVER_ADMITTED':True}
(D/'MANIFEST.json').write_text(json.dumps(manifest,indent=2)+'\n')
assert len({x['original_source'] for x in records})==len(records) and len({x['archive_path'] for x in records})==len(records)
for x in records:
 b=(D/x['archive_path']).read_bytes();assert H(b)==x['stored_sha256'] and len(b)==x['stored_bytes'];raw=gzip.decompress(b) if x['encoding']=='gzip' else b;assert H(raw)==x['original_sha256'] and len(raw)==x['original_bytes'] and raw==pathlib.Path(x['original_source']).read_bytes()
 if x['encoding']=='identity':
  r=subprocess.run(['git','-c','core.whitespace=blank-at-eol,blank-at-eof,space-before-tab','diff','--no-index','--check','--','/dev/null',str(D/x['archive_path'])],capture_output=True);assert not r.stdout and not r.stderr
for x in cross:
 raw=pathlib.Path(x['original_source']).read_bytes();assert H(raw)==x['original_sha256'] and len(raw)==x['original_bytes'];b=(prior/x['referenced_archive_path']).read_bytes();assert H(b)==x['referenced_stored_sha256'];assert (gzip.decompress(b) if x['referenced_encoding']=='gzip' else b)==raw
for x in generated:
 b=(D/x['archive_path']).read_bytes();assert H(b)==x['sha256'] and len(b)==x['bytes']
assert {str(p.relative_to(D)) for p in D.rglob('*') if p.is_file()}=={x['archive_path'] for x in records}|{x['archive_path'] for x in generated}|{'MANIFEST.json'}
# Verify complete source snapshot selections stayed stable during the copy.
for directory_name,selected in snapshots.items():
 p=pathlib.Path(directory_name);top=directory_name in ['/tmp/bex-m188-dbc-source-closure-srk1e1yx','/tmp/bex-m188-9fe-source-closure-7613x0c7'];actual=sorted(str(x) for x in (p.iterdir() if top else p.rglob('*')) if x.is_file() and '__pycache__' not in x.parts and '.git' not in x.parts);assert actual==selected,directory_name
out={'archive':str(D),'manifest_sha256':H((D/'MANIFEST.json').read_bytes()),'source_records':len(records),'compressed_records':len(whitespace),'generated_metadata_records':len(generated),'total_files':sum(p.is_file() for p in D.rglob('*')),'original_payload_bytes':manifest['original_bytes'],'stored_payload_bytes':manifest['stored_bytes'],'total_bytes':sum(p.stat().st_size for p in D.rglob('*') if p.is_file()),'cross_archive_content_references':len(cross),'external_large_proof_references':len(external),'all_stored_original_source_and_crossarchive_hash_bytes_verified':True,'unique_source_and_destination_paths':True,'complete_selected_snapshots_stable':True,'known_privatekey_and_credential_token_patterns':0,'secrets_or_production_configs_selected':False,'no_repo_guard_Q_or_CI_mutation':True}
v=pathlib.Path(str(D)+'-verification.json');v.write_text(json.dumps(out,indent=2)+'\n');print(json.dumps(out,indent=2));print('VERIFICATION',v,H(v.read_bytes()))
