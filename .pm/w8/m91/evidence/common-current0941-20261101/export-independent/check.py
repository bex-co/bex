"""Read-only current0941 portable export/apply-boundary review; UNIT validators only."""
from pathlib import Path
import ast,base64,copy,gzip,hashlib,importlib.util,json,os,stat
from unittest.mock import patch
R=Path(__file__).parent
A=Path('/private/tmp/bex-w8-common0941-portable-export-20261101-66f0cxux')
STAGE=A/'staging';PREFIX=Path('.pm/w8/m91/evidence/common-current0941-20261101');E=STAGE/PREFIX
REPO=Path('/Users/tianpan/projects/bex7')
RUN=Path('/private/tmp/bex-w8-common499-review-2ff8ba6f1f039942');CASE=RUN/'evidence/native-common-bookworm-1feba939b4697894e800ed0abb4f25a0bf415641'
U=Path('/private/tmp/bex-w8-1feba-utility-intake-20261101-r5updvtw')
REF='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
PRODUCER='1feba939b4697894e800ed0abb4f25a0bf415641';GENERATOR='f071830616b2f55aa8636bb856454f4a218a9174'
def digest(p,unzip=False):
 p=Path(p);assert p.is_file() and not p.is_symlink();h=hashlib.sha256();n=0
 with (gzip.open(p,'rb') if unzip else p.open('rb')) as f:
  while b:=f.read(1<<20):h.update(b);n+=len(b)
 return {'sha256':h.hexdigest(),'bytes':n}
def load(p):return json.loads(Path(p).read_bytes())
def verify_manifest(root,want):
 assert digest(root/'MANIFEST.json')['sha256']==want
 rows=load(root/'MANIFEST.json')
 actual={str(p.relative_to(root)) for p in root.rglob('*') if p.is_file()}
 assert actual==set(rows)|{'MANIFEST.json'}
 for name,row in rows.items():assert digest(root/name)==row,name
 return len(rows)
author_count=verify_manifest(A,'ba2b27c4ab9ce27a3aa756a3fd7159282f9f321dd3088535bd41cb18f536d84c')
assert digest(A/'RECEIPT.json')['sha256']=='e90175002b394bb5b7e4e14eb86b6a8cf7ced76d0aa9968796593aba4c825442'
evidence_count=verify_manifest(E,'fa94f06d4de9377e10bcf1105ac6fdaf409212b1de7b0e3dab8560b0ee5fba5f')
candidate=STAGE/'.github/native-common-test-candidate.json'
assert digest(candidate)=={'bytes':4485,'sha256':'0d7ba9d20a88213aed4c7b58d3afa52f5842199aef8b19d866603b96428e345e'}
record=load(candidate);c=record['candidate'];receipt=load(A/'RECEIPT.json');actual=load(CASE/'receipt.json')
assert c['ref']==actual['ref']==REF and c['producer']==c['verification']==PRODUCER
assert receipt['apply_only']==[str(PREFIX),'.github/native-common-test-candidate.json']
assert not (REPO/PREFIX).exists()
original_candidate=digest(REPO/'.github/native-common-test-candidate.json')
assert load(REPO/'.github/native-common-test-candidate.json')['candidate']['ref']!=REF
custody=load(E/'CUSTODY.json');raw=load(E/'RAW-CUSTODY.json')
for key,desc in custody['files'].items():
 assert set(desc)=={'path','sha256','bytes'} and desc['path'].startswith(str(PREFIX)+'/')
 assert digest(STAGE/desc['path'])=={k:desc[k] for k in ('sha256','bytes')}
by_rep={};raw_bytes=0
for key,row in raw['files'].items():
 rep=row['representation'];by_rep[rep]=by_rep.get(rep,0)+1;retained=STAGE/row['retained']['path']
 assert row['retained']['path'].startswith(str(PREFIX)+'/')
 assert digest(retained)=={k:row['retained'][k] for k in ('sha256','bytes')}
 if rep=='byte-exact-copy':
  original=Path(key);assert original.is_absolute() and not any(part in ('anonymous-docker-config','private') for part in original.relative_to('/private/tmp').parts)
  assert digest(original)==digest(retained)=={'sha256':row['raw_sha256'],'bytes':row['raw_bytes']};raw_bytes+=row['raw_bytes']
 elif rep=='original-existing-gzip-no-recompression':
  assert digest(retained,True)=={'sha256':row['raw_sha256'],'bytes':row['raw_bytes']}
 elif rep=='explicit-JSON-object-selection':
  collection=load(STAGE/row['original_collection']['path'])['artifacts'];selected=[x for x in collection if x['id']==row['selector']['id']]
  assert len(selected)==1 and load(retained)==selected[0] and row['raw_HTTP_body_claim'] is False
 elif rep=='actual-local-Git-blob':
  assert key=='git:'+PRODUCER+':'+row['path'] and row['source']==PRODUCER
  assert digest(retained)=={'sha256':row['raw_sha256'],'bytes':row['raw_bytes']}
  expected_mode=0o755 if row['git_mode']=='100755' else 0o644;assert stat.S_IMODE(retained.stat().st_mode)==expected_mode
 else:raise AssertionError(rep)
# Exact actual case file coverage excluding only the five original raw bundles.
raw_bundles={Path(x['path']) for x in actual['signed_payloads']}
case_expected={str(p) for p in CASE.rglob('*') if p.is_file()}-{str(p) for p in raw_bundles}
case_observed={k for k,v in raw['files'].items() if v['representation']=='byte-exact-copy' and k.startswith(str(CASE)+'/')}
assert case_expected==case_observed
gzip_checks=[]
for row in actual['signed_payloads']:
 for key in ('full_bundle_gzip','full_predicate_gzip'):
  desc=custody['files'][row[key]];p=STAGE/desc['path'];assert digest(p)['sha256']==row[key+'_sha256'] and digest(p)==digest(row[key])
  gzip_checks.append({'kind':row['kind'],'architecture':row['architecture'],'form':key,'sha256':digest(p)['sha256']})
assert len(gzip_checks)==10
assert load(E/'actual/receipt.json')==actual and load(E/'outer/RESULT.json')==load(RUN/'RESULT.json')
assert actual['anonymous_pull']['unique_blob_count']==41 and actual['anonymous_pull']['bytes_hashed_this_finalization']==1522239193
assert load(E/'EXPORT.json')['trivy_summaries_unfiltered']==actual['trivy_summaries_by_architecture']
assert all(x[0]['vulnerability_count']==6920 for x in actual['trivy_summaries_by_architecture'].values())
utility_checks={};all_members=0
for arch in ('amd64','arm64'):
 old=U/arch/'extracted/artifact';new=E/'utilities'/arch/'artifact';m=load(old/'MANIFEST.json')
 assert len(m['files'])==355 and (new/'MANIFEST.json').read_bytes()==(old/'MANIFEST.json').read_bytes()
 expected={x['path'] for x in m['files']}|{'MANIFEST.json'}
 assert {str(p.relative_to(new)) for p in new.rglob('*') if p.is_file()}==expected
 assert {str(p.relative_to(old)) for p in old.rglob('*') if p.is_file()}==expected
 for name in expected:assert digest(new/name)==digest(old/name)
 extraction=load(U/arch/'extracted/extraction.json');assert extraction['complete'] is True and extraction['completed_members']==356 and extraction['actual_file_bytes']==sum((new/n).stat().st_size for n in expected)
 assert c['utilities'][arch]['manifest']['sha256']==digest(new/'MANIFEST.json')['sha256']
 utility_checks[arch]={'members':len(expected),'bytes':sum((new/n).stat().st_size for n in expected),'original_manifest_sha256':digest(new/'MANIFEST.json')['sha256'],'exact':True};all_members+=len(expected)
assert all_members==712
assert (E/'utilities/amd64/artifact/qualification-source.json').read_bytes()==(E/'utilities/arm64/artifact/qualification-source.json').read_bytes()
baseline=load(E/'utilities/amd64/artifact/qualification-source.json');assert baseline['source']==PRODUCER and len(baseline['files'])==300
# The staged validator and its dependencies remain exact published/current code.
module=STAGE/'.github/scripts/native-common-qualify.py'
assert digest(module)==digest(REPO/'.github/scripts/native-common-qualify.py') and digest(module)['sha256']=='f564dae9b771de166d18b16564165b8140a7bb857769cb94fed390cbdec406e5'
spec=importlib.util.spec_from_file_location('independent_current0941_validator',module);h=importlib.util.module_from_spec(spec);spec.loader.exec_module(h)
assert h.source_inventory(STAGE)==baseline['files'] and len(h.source_inventory(STAGE))==300
image=load(E/'IMAGE-INPUTS.json');assert image['source']==PRODUCER and len(image['files'])==33 and image['files']==h.image_source_inventory(STAGE)
for name,want in baseline['files'].items():assert h.file_identity(REPO/name)==want
real_open=Path.open
def no_write(self,mode='r',*args,**kwargs):
 assert not any(x in mode for x in ('w','a','x','+')),'UNIT workflow/file write attempted'
 return real_open(self,mode,*args,**kwargs)
def refuse(*args,**kwargs):raise AssertionError('UNIT external process/workflow action attempted')
with patch.dict(os.environ,{'GITHUB_SHA':GENERATOR,'GITHUB_OUTPUT':str(R/'FORBIDDEN-GITHUB-OUTPUT'),'GITHUB_ENV':str(R/'FORBIDDEN-GITHUB-ENV')}),patch.object(h.subprocess,'run',side_effect=refuse),patch.object(h.subprocess,'Popen',side_effect=refuse),patch.object(h,'dump',side_effect=refuse),patch.object(Path,'open',no_write):
 image_result=h.validate_image_candidate(root=STAGE);baseline_result=h.validate_test_candidate(root=STAGE)
 null=copy.deepcopy(record);null['candidate']['source']=None;null['candidate']['utilities']=None
 real_json=h.json_file
 def null_json(p):return copy.deepcopy(null) if p==candidate else real_json(p)
 with patch.object(h,'json_file',side_effect=null_json):
  h.validate_image_candidate(root=STAGE)
  try:h.validate_test_candidate(root=STAGE)
  except ValueError as error:negative=str(error)
  else:raise AssertionError('null baseline accepted')
assert negative=='family reuse requires a complete utility baseline source'
assert digest(candidate)=={'bytes':4485,'sha256':'0d7ba9d20a88213aed4c7b58d3afa52f5842199aef8b19d866603b96428e345e'} and digest(REPO/'.github/native-common-test-candidate.json')==original_candidate
assert not (R/'FORBIDDEN-GITHUB-OUTPUT').exists() and not (R/'FORBIDDEN-GITHUB-ENV').exists()
# Bind minimal reusable assembler changes against its exact recorded old source.
base=REPO/'.pm/w8/m91/evidence/common-artifact-review-20261101/export-review/author/assemble.py.gz'
old=gzip.decompress(base.read_bytes());assert hashlib.sha256(old).hexdigest()=='59e8931d46e8a1c7851764a6a0c62f92025d611d7c8aa8cd2c7818a7510c8905'
new=(A/'assemble.py').read_bytes();assert hashlib.sha256(new).hexdigest()=='1c70c873d9c6f821bd008322e7d7fbab39f3013f54259538a1e81bf225359146'
def fns(body):return {n.name:ast.dump(n,include_attributes=False) for n in ast.parse(body).body if isinstance(n,(ast.FunctionDef,ast.ClassDef))}
oldfn,newfn=fns(old),fns(new);changed=[x for x in oldfn if oldfn[x]!=newfn[x]];assert changed==['main']
assert set(newfn)-set(oldfn)=={'metadata','outer_complete','utility_baseline'}
executed=gzip.decompress((A/'history/assemble-executed.py.gz').read_bytes());assert hashlib.sha256(executed).hexdigest()=='04eb559b6fc1ccc170361532992dcc5711192edd0b7dddb0226c530f5dbdcdff'
def replace_strings(body):
 tree=ast.parse(body)
 class Strings(ast.NodeTransformer):
  def visit_Constant(self,n):return ast.copy_location(ast.Constant(value='<string>'),n) if isinstance(n.value,str) else n
 return ast.dump(Strings().visit(tree),include_attributes=False)
assert replace_strings(new)==replace_strings(executed)
summary={'PASS':True,'author_manifest_sha256':digest(A/'MANIFEST.json')['sha256'],'author_payloads':author_count,'evidence_manifest_sha256':digest(E/'MANIFEST.json')['sha256'],'evidence_payloads':evidence_count,'evidence_bytes':sum(p.stat().st_size for p in E.rglob('*') if p.is_file()),'apply_only':[str(PREFIX),'.github/native-common-test-candidate.json'],'candidate':digest(candidate),'RAW_custody_rows':len(raw['files']),'CUSTODY_rows':len(custody['files']),'representations':by_rep,'all_original_case_files':len(case_expected),'original_signed_gzip_forms':gzip_checks,'utilities':utility_checks,'all_original_members':all_members,'source300_exact':True,'image33_bytes_modes_exact':True,'validator_sha256':digest(module)['sha256'],'UNIT_validators':{'image':'PASS','both_original_utility_baseline':'PASS','nullbaseline':negative,'controlled_GITHUB_SHA':GENERATOR,'subprocess_and_workflow_file_writes_trapped':True,'frozen_stage_candidate_unchanged':True},'assembler_AST':{'old_existing_functions_unchanged_except':changed,'new_functions':sorted(set(newfn)-set(oldfn)),'executed_to_final_change':'string constants only; author metadata calibration retained'},'all6920_findings_per_architecture_retained':True,'shared_candidate_unchanged':True,'physical_worktree_qualification_claim':False,'actual_network_crypto_Docker_Go_CI_auth_repo_writes':False}
(R/'CHECKS.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps({'PASS':True,'author_payloads':author_count,'evidence_payloads':evidence_count,'original_case_files':len(case_expected),'utility_members':all_members,'validators':'image+baselinePASS/nullbaselineREFUSED','candidate_unchanged':True}))
