"""Copy only the authorized small compatibility evidence; preserve every raw byte."""
import datetime,gzip,hashlib,json,os,pathlib,re,shutil,stat,subprocess,tempfile
R=pathlib.Path('/Users/tianpan/projects/bex');D=pathlib.Path(tempfile.mkdtemp(prefix='bex-m188-current-tree-portable-archive-',dir='/tmp'));H=lambda b:hashlib.sha256(b).hexdigest();sources={};groups={}
def add(src,relative,role):
 p=pathlib.Path(src);assert p.is_file() and not p.is_symlink(),str(p);assert not any(x in p.parts for x in ('.env','__pycache__')) and not p.name.endswith(('.kubeconfig','.pyc'))
 if str(p) in sources:
  sources[str(p)]['roles'].append(role);return
 assert relative not in groups,relative;groups[relative]=str(p);sources[str(p)]={'archive_relative_raw':relative,'roles':[role]}
def directory(src,relative,role):
 p=pathlib.Path(src)
 for f in sorted(p.rglob('*')):
  if f.is_file() and '__pycache__' not in f.parts:add(f,str(pathlib.PurePosixPath(relative)/f.relative_to(p)),role)
directory('/tmp/bex-m188-current-tree-2cc-proposal-v2','proposal-v2','final_approved_proposal_package')
for p in ['/tmp/bex-m188-test-current-tree-2cc-proposal-v2.py','/tmp/bex-m188-build-current-tree-2cc-proposal.py','/tmp/bex-m188-create-independent-2cc-review.py','/tmp/bex-m188-build-current-tree-portable-archive.py']:add(p,'harness/'+pathlib.Path(p).name,'actual_harness_or_evidence_builder')
directory('/tmp/bex-m188-941-compatibility-25rhxfrw','independent-node-source-review','independent_exact941_9tests_source_and_historical_checks')
directory('/tmp/bex-m188-a349-independent-review-w4eh3k1q','independent-rejected-a349-review','rejected_inactive_a349_review')
directory('/tmp/bex-m188-4cfe-independent-review-r1ch760s','independent-approved-4cfe-review','approved_4cfe_review')
directory('/tmp/bex-m188-941-source-review','cnb-local-fetch-tests','independent_exact941_local_9tests')
for p in ['/tmp/bex-m188-941-root-source-closure-audit.json','/tmp/bex-m188-2cc-root-source-closure-audit.json','/tmp/bex-m188-current-tree-profile-root-approval.json','/tmp/bex-m188-current-tree-profile-activation.json','/tmp/bex-m188-current-tree-profile-context.json','/tmp/bex-m188-guard-receipt-context-after-2cc-current-tree-compatibility.json']:add(p,'root-policy/'+pathlib.Path(p).name,'root_audit_approval_activation_or_frozen_context')
for digest in ['2a8363b55421914c9a133594870fc859621ec185cc29a7f733629d4ef69e16c2','4cfe9fb888c581dade2209f3a7cd12e723c91ed80d563c261f52b546d1438865']:add('/tmp/bex-m188-guard-snapshots/'+digest+'.py','frozen-guards/'+digest+'.py','frozen_old_or_new_guard')
for p in ['/tmp/bex-m188-current-tree-2cc-proposal/a349f140441def0e14c43c5423f4aa4d146a95fa1b97d5e93515f175920ca55e.py','/tmp/bex-m188-current-tree-2cc-proposal/guard-final.diff']:add(p,'rejected-never-active-a349/'+pathlib.Path(p).name,'rejected_never_active_missing_explicit_ancestry')
review=json.loads(pathlib.Path('/tmp/bex-m188-current-tree-2cc-proposal-v2/independent-source-closure-review.json').read_bytes());assert len(review['input_manifest'])==37
for x in review['input_manifest']:
 p=R/x['path'];raw=p.read_bytes();assert H(raw)==x['sha256'] and len(raw)==x['bytes'],x['path'];add(p,'current-source/'+x['path'],'exact_current2cc_source_closure_input')
# Explicitly authenticate policy/canonical snapshots before copying.
for p,expected in {'/tmp/bex-m188-current-tree-profile-root-approval.json':'98b53a544d1867d92de07737e5a6070d09a2e364ef907fbbf57402ff1bbeee08','/tmp/bex-m188-current-tree-profile-activation.json':'85683153b7a17b4548226ed64e2950e83c52448850105dba7ac63e305770493a','/tmp/bex-m188-current-tree-profile-context.json':'e0d2f82905f7105ff321d0c05909e63936c7cb5993776100ed362dd6771368d2','/tmp/bex-m188-guard-receipt-context-after-2cc-current-tree-compatibility.json':'8c9bf1fee66deafdea86a360ead9c7038843d7f9e81dfabc25bd055eb677b3c4'}.items():assert H(pathlib.Path(p).read_bytes())==expected,p
records=[];whitespace=[]
for origin,entry in sorted(sources.items()):
 p=pathlib.Path(origin);raw=p.read_bytes();rel=entry['archive_relative_raw'];result=subprocess.run(['git','-c','core.whitespace=blank-at-eol,blank-at-eof,space-before-tab','diff','--no-index','--check','--','/dev/null',str(p)],capture_output=True);errors=result.stdout+result.stderr
 assert result.returncode in (0,1,2,3),result.returncode
 # Only actual whitespace diagnostics select lossless compression; other git errors are not ignored.
 bad=bool(errors);assert not bad or any(phrase in errors for phrase in [b'trailing whitespace',b'space before tab in indent',b'new blank line at EOF']),errors.decode()
 payload=gzip.compress(raw,compresslevel=9,mtime=0) if bad else raw
 stored=rel+'.gz' if bad else rel;out=D/stored;out.parent.mkdir(parents=True,exist_ok=True);assert not out.exists();out.write_bytes(payload)
 records.append({'original_source':origin,'archive_path':stored,'stored_sha256':H(payload),'stored_bytes':len(payload),'original_sha256':H(raw),'original_bytes':len(raw),'encoding':'gzip' if bad else 'identity','source_mode':oct(stat.S_IMODE(p.stat().st_mode)),'roles':entry['roles'],'lossless_compression_reason':'actual_git_diff_check_whitespace_diagnostic' if bad else None})
 if bad:whitespace.append({'original_source':origin,'raw_git_whitespace_diagnostic':errors.decode()})
# Record external references without copying, reading or hashing large proof bodies.
external={};archived=set(sources)
def walk(o):
 if isinstance(o,dict):
  path=o.get('file',o.get('path'))
  if isinstance(path,str) and path.startswith(('/tmp/','/Users/')) and path not in archived:external.setdefault(path,{'original_reference':path,'advertised_sha256':o.get('sha256'),'portable_status':'not_in_this_archive; resolve through existing accepted-evidence custody/portable locator'})
  for k,v in o.items():
   if isinstance(k,str) and k.startswith(('/tmp/','/Users/')) and isinstance(v,str) and re.fullmatch('[0-9a-f]{64}',v) and k not in archived:external.setdefault(k,{'original_reference':k,'advertised_sha256':v,'portable_status':'not_in_this_archive; resolve through existing accepted-evidence custody/portable locator'})
   walk(v)
 elif isinstance(o,list):
  for x in o:walk(x)
for origin in sources:
 if origin.endswith('.json'):
  try:walk(json.loads(pathlib.Path(origin).read_bytes()))
  except (UnicodeDecodeError,json.JSONDecodeError):pass
external_record={'schema':'bex.m188.portable-archive-external-reference-index/v1','scope':'References retained without copying already durable ten-candidate proof bodies; advertised hashes are original metadata, not newly authenticated by this archive. Parent may provide a later portable locator.','references':sorted(external.values(),key=lambda x:x['original_reference'])}
(D/'EXTERNAL-REFERENCES.json').write_text(json.dumps(external_record,indent=2)+'\n')
(D/'README.txt').write_text('Exact current-tree compatibility evidence archive candidate.\n\nCanonical artifact source remains DA0.941 recipe and2cc workflow are checkout-only compatibility anchors; no artifact roles were added. Historical proposal and review files retain their original not-activated wording and timestamps. The root approval and activation files record the later actual transition.\n\na349 under rejected-never-active-a349 was NEVER ACTIVE and was superseded because explicit current-checkout ancestry was missing.4cfe adds exact941->2cc->currentHEAD ancestry and was root-approved/activated.\n\nPayload files preserve exact source bytes. Files with actual git-diff whitespace diagnostics are losslessly gzip-compressed; MANIFEST hashes both stored bytes and decompressed origin bytes. Do not format, normalize or rewrite archived raw records.\n\nThis bounded package excludes existing large ten-candidate cryptographic proof bodies and does not assert portable resolution of their original absolute paths. EXTERNAL-REFERENCES records those references for later parent-owned portable locators. No secrets or production configs are selected. No actual image pin adoption or candidate promotion is authorized by this archive.\n')
(D/'WHITESPACE-COMPRESSION.json').write_text(json.dumps({'schema':'bex.m188.archive-lossless-whitespace-selection/v1','records':whitespace},indent=2)+'\n')
generated=[]
for name in ['EXTERNAL-REFERENCES.json','README.txt','WHITESPACE-COMPRESSION.json']:
 b=(D/name).read_bytes();generated.append({'archive_path':name,'sha256':H(b),'bytes':len(b),'generated_archive_metadata':True})
aliases=[]
for expected in ['2a8363b55421914c9a133594870fc859621ec185cc29a7f733629d4ef69e16c2','4cfe9fb888c581dade2209f3a7cd12e723c91ed80d563c261f52b546d1438865']:
 candidates=[x for x in records if x['original_sha256']==expected and 'frozen-guards/' in x['archive_path']];assert len(candidates)==1;aliases.append({'original_reference':'/tmp/bex-m188-prepare-adoption.py','expected_original_sha256':expected,'expected_original_bytes':candidates[0]['original_bytes'],'archive_path':candidates[0]['archive_path'],'resolution':'content-bound frozen snapshot; original record references are unmodified'})
manifest={'schema':'bex.m188.portable-current-tree-compatibility-archive/v1','created_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'archive_candidate_only':True,'canonical_artifact_source':review['canonical_artifact_source'],'current_recipe_anchor':review['current_recipe_anchor'],'current_workflow_anchor':review['current_workflow_anchor'],'records':records,'generated_metadata':generated,'record_count':len(records),'compressed_record_count':len(whitespace),'original_bytes':sum(x['original_bytes'] for x in records),'stored_bytes':sum(x['stored_bytes'] for x in records),'current_source_closure_inputs':37,'new_artifact_source_roles':[],'large_accepted_proof_bodies_archived':False,'historical_content_aliases':aliases}
(D/'MANIFEST.json').write_text(json.dumps(manifest,indent=2)+'\n')
# Independent pass over the finished payload, source origin, complete file set and collision rules.
assert len({x['original_source'] for x in records})==len(records) and len({x['archive_path'] for x in records})==len(records)
for x in records:
 out=D/x['archive_path'];b=out.read_bytes();assert H(b)==x['stored_sha256'] and len(b)==x['stored_bytes'];raw=gzip.decompress(b) if x['encoding']=='gzip' else b;assert H(raw)==x['original_sha256'] and len(raw)==x['original_bytes'];assert pathlib.Path(x['original_source']).read_bytes()==raw
for x in generated:
 b=(D/x['archive_path']).read_bytes();assert H(b)==x['sha256'] and len(b)==x['bytes']
assert {str(x.relative_to(D)) for x in D.rglob('*') if x.is_file()}=={x['archive_path'] for x in records}|{x['archive_path'] for x in generated}|{'MANIFEST.json'}
# Final no-index checks ensure the selected raw files left as text do not reintroduce whitespace errors.
for x in records:
 if x['encoding']=='identity':
  r=subprocess.run(['git','-c','core.whitespace=blank-at-eol,blank-at-eof,space-before-tab','diff','--no-index','--check','--','/dev/null',str(D/x['archive_path'])],capture_output=True);assert not r.stdout and not r.stderr
summary={'archive':str(D),'manifest_sha256':H((D/'MANIFEST.json').read_bytes()),'manifest_bytes':(D/'MANIFEST.json').stat().st_size,'source_records':len(records),'generated_metadata_records':len(generated),'compressed_records':len(whitespace),'original_bytes':manifest['original_bytes'],'stored_payload_bytes':manifest['stored_bytes'],'total_files':sum(1 for x in D.rglob('*') if x.is_file()),'total_bytes':sum(x.stat().st_size for x in D.rglob('*') if x.is_file()),'external_reference_count':len(external),'verification':'All stored bytes, decompressed origin bytes, original source bytes, generated metadata, unique source/archive paths and exact complete file set verified; all uncompressed raw payloads pass actual git diff --check.'}
summary_path=pathlib.Path(str(D)+'-verification.json');summary_path.write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary,indent=2));print('VERIFICATION',summary_path,H(summary_path.read_bytes()))
