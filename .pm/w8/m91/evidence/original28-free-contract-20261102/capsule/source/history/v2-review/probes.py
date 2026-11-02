"""Independent V2 synthetic source-binder probes; no actual signatures/live I/O."""
from pathlib import Path
import sys,json,copy,gzip,io,tempfile
from unittest.mock import patch
R=Path(__file__).resolve().parent;sys.path.insert(0,str(R/'source'))
import readiness as r,core as c,proof_binding as p,catalog,evidence
from test_adoption import AdoptionControls
from test_adapter import post,NOW
from qualification_fixtures import Facts
out={'scope':'Synthetic source/metadata controls; signatures mocked only for isolated adoption source binder. No original module/app/command execution.'}
def fixture(before,after,edits,mode='100644',zero=False):
 contract,q,entry,docs,_,_=AdoptionControls().fixture();path=r.COMMON_SLOT
 ref=contract['pin_only_deltas'][0]['before'];docs[ref['path']]=before;ref.update(bytes=len(before),sha256=c.sha(before))
 identity={'bytes':len(before),'sha256':c.sha(before)};q['qualification_source'][path]=identity;q['critical_source'][path]=dict(identity)
 contract['critical_files'][path].update(bytes=len(after),sha256=c.sha(after),git_mode=mode,git_blob='f'*40)
 contract['pin_only_deltas']=[dict(qualification_source=q['qualified_baseline_source'],path=path,before=ref,edits=edits)] if edits is not None else []
 body=b'func choose(workspace string) string { return nativeCommonBase }\n';scope=contract['scope_code'][0]
 docs[scope['source']['path']]=body;scope['source'].update(bytes=len(body),sha256=c.sha(body));scope.update(first_byte=after.index(body),last_byte=after.index(body)+len(body),sha256=c.sha(body))
 def git(action,left,right):
  if action=='ancestor':return True
  if action=='changes':return [] if zero else [{'status':'M','path':path}]
  return (before,'100644','f'*40 if zero else 'e'*40) if left==q['qualified_baseline_source'] else (after,mode,'f'*40)
 return contract,q,entry,docs,git
def call(case):
 contract,q,entry,docs,git=case
 with patch.object(p,'bind_signature',return_value={'CONTROL_ONLY':True}) as sig:
  try:result=r.adoption(contract,entry,q,docs.__getitem__,git)
  except c.Unproved as error:return {'accepted':False,'reason':str(error),'signature_calls':sig.call_count}
  return {'accepted':True,'changed':result['changed_critical_paths'],'signature_calls':sig.call_count}
base=AdoptionControls().fixture();old=base[0]['pin_only_deltas'][0]['edits'][0]['before'];new=base[1]['common_ref']
before=('package build\nvar nativeCommonBase = '+json.dumps(old)+'\nfunc choose(workspace string) string { return nativeCommonBase }\nfunc prepare() string { return "baseline" }\n').encode()
after=before.replace(old.encode(),new.encode()).replace(b'"baseline"',b'"changed"')
case=fixture(before,after,None);out['prior_omitted_runtime_delta']=call(case);assert not out['prior_omitted_runtime_delta']['accepted']
wrong='ghcr.io/bex-co/native@sha256:'+'8'*64
before=before+('const unrelatedApproved = '+json.dumps(new)+'\n').encode();after=before.replace(json.dumps(old).encode(),json.dumps(wrong).encode())
case=fixture(before,after,[{'before':old,'after':wrong}]);out['prior_unqualified_target_with_unrelated_qualified_literal']=call(case);assert not out['prior_unqualified_target_with_unrelated_qualified_literal']['accepted']
# Exact tracked first common slot; no actual repo mutation or Go execution.
actual=(Path('/Users/tianpan/projects/bex7')/r.COMMON_SLOT).read_bytes();assert actual.count(b'var nativeCommonBase string\n')==1
for syntax in ('inferred','typed'):
 after=actual.replace(b'var nativeCommonBase string\n',r.common_initializer(new,syntax))
 contract,q,entry,docs,_,_=AdoptionControls().fixture();path=r.COMMON_SLOT;ref=contract['pin_only_deltas'][0]['before'];docs[ref['path']]=actual;ref.update(bytes=len(actual),sha256=c.sha(actual))
 ident={'bytes':len(actual),'sha256':c.sha(actual)};q['qualification_source'][path]=ident;q['critical_source'][path]=dict(ident)
 contract['critical_files'][path].update(bytes=len(after),sha256=c.sha(after));contract['common_initializer_syntax']=syntax
 contract['pin_only_deltas'][0]['edits']=[{'kind':'native-common-base-initializer','before':'var nativeCommonBase string\n','after':r.common_initializer(new,syntax).decode()}]
 scope=contract['scope_code'][0];docs[scope['source']['path']]=after;scope['source'].update(bytes=len(after),sha256=c.sha(after));scope.update(first_byte=0,last_byte=len(after),sha256=c.sha(after))
 def git(action,left,right):
  if action=='ancestor':return True
  if action=='changes':return [{'status':'M','path':path}]
  return (actual,'100644','e'*40) if left==q['qualified_baseline_source'] else (after,'100644','f'*40)
 out['actual_initial_slot_'+syntax]=call((contract,q,entry,docs,git));assert out['actual_initial_slot_'+syntax]['accepted']
# Small independent follow-on: a comment containing the required declaration
# must not stand in for the actual variable when no source changes are declared.
raw=('package build\nvar nativeCommonBase = '+json.dumps(old)+'\n// '+r.common_initializer(new,'inferred').decode()+'func choose(workspace string) string { return nativeCommonBase }\n').encode()
case=fixture(raw,raw,None,zero=True);out['comment_is_not_actual_common_slot']=call(case)
# Mode changes are source changes, never pin-only.
basebefore=('package build\nvar nativeCommonBase = '+json.dumps(old)+'\nfunc choose(workspace string) string { return nativeCommonBase }\n').encode();baseafter=basebefore.replace(old.encode(),new.encode())
out['changed_mode']=call(fixture(basebefore,baseafter,[{'before':old,'after':new}],mode='100755'));assert not out['changed_mode']['accepted']
cat=catalog.Catalog((R/'source/original-inputs/prepared-fixtures.json').read_bytes(),(R/'source/EMISSION-MAP.json').read_bytes());files=[]
for client in ('bex','render'):
 spec=cat.spec((91,client),NOW,set());body=post(spec);assert catalog.validate_post(body,spec)
 good=copy.deepcopy(body);good['secretFiles'][0]['content']=good['secretFiles'][0]['content'].rstrip('\n')
 try:catalog.validate_post(good,spec)
 except c.Unproved:refused=True
 else:refused=False
 assert refused;files.append({'client':client,'exact_file_accepted':True,'removedLF_rejected':True,'source_args_unchanged':spec['create_args']==cat.spec((91,client),NOW,set())['create_args']})
out['m91_exact_public_archive']=files
# Retain metadata cap, but allow legitimate external compressed report shapes.
raw=b'{"CONTROL_ONLY":"'+b'x'*(16*1024*1024+1)+b'"}';compressed=gzip.compress(raw,mtime=0)
with tempfile.TemporaryDirectory(dir=R,prefix='own-probe-') as tmp:
 file=Path(tmp)/'report.gz';file.write_bytes(compressed);ref={'path':str(file),'bytes':len(compressed),'sha256':c.sha(compressed)}
 reader=evidence.Access(lambda path:Path(path).read_bytes())
 assert evidence.gunzip_full(ref,reader)==raw
 for trailer in (b'junk',gzip.compress(b'other',mtime=0)):
  blob=compressed+trailer;bad=dict(ref,bytes=len(blob),sha256=c.sha(blob))
  try:evidence.gunzip_full(bad,lambda path:blob)
  except c.Unproved:continue
  raise AssertionError('trailing/member accepted')
 out['full_external_report']={'decoded_bytes':len(raw),'exact_decoded':True,'junk_and_secondmember_rejected':True,'external_tmp_removed_on_exit':True}
# Null admission stays before proof reader; source PASS has no live authority.
try:r.admit({},None,NOW,lambda path:(_ for _ in ()).throw(AssertionError('reader called')),None)
except c.Unproved as error:out['null_adoption_refuses_before_reader']=str(error)
else:raise AssertionError('null admitted')
# Actual retained old receipt still cannot be promoted with boolean labels.
oldpath=Path('/Users/tianpan/projects/bex7/.pm/w8/m91/evidence/published-node26-20261031/retained/output/native-node-26-bookworm-e48dd4a66d30343d2b5073ce3c210c7e0dd828cc/receipt.json')
oldraw=oldpath.read_bytes();old=json.loads(oldraw);facts=Facts();roles=facts.contract['roles']
roles.update(producer=old['producer_workflow_sha'],verifier=old['verification_workflow_sha']);facts.entry['roles']=copy.deepcopy(roles)
facts.entry['common_ref']=old['ref'];facts.contract['common_ref']=old['ref'];oldcopy=copy.deepcopy(old)
oldcopy.update(currentQualified=True,allFamiliesPassed=True,adoptionReady=True,allWorkspaces=True);facts.replace(facts.entry['v11_receipt'],oldcopy)
try:r.qualification(facts.entry,facts.contract,facts.docs.__getitem__)
except c.Unproved as error:out['actual_old_V11_booleans_do_not_upgrade']={'old_receipt_sha256':c.sha(oldraw),'reason':str(error),'original_unchanged':True}
else:raise AssertionError('old receipt booleans accepted')
(R/'PROBE-RESULTS.json').write_text(json.dumps(out,indent=2)+'\n');print(json.dumps(out,indent=2))
