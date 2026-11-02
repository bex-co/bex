"""Independent synthetic source/schema probes. No live or original-program I/O."""
import copy,datetime,gzip,hashlib,json,os,stat,sys,tempfile
from pathlib import Path
from unittest.mock import patch
HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE/'source'))
import adapter,catalog,core,readiness,proof_binding
from test_adapter import post
from test_adoption import AdoptionControls
from qualification_fixtures import Facts

BASE=Path('/Users/tianpan/projects/bex7')
OLD=BASE/'.pm/w8/m91/evidence/published-node26-20261031/retained/output/native-node-26-bookworm-e48dd4a66d30343d2b5073ce3c210c7e0dd828cc'
receipt_raw=(OLD/'receipt.json').read_bytes();old=json.loads(receipt_raw)
results={'scope':'Synthetic/frozen-metadata probes only. Original modules/apps/commands/native/crypto not executed; signatures mocked ONLY for isolated adoption source-binding probes.'}
row=next(x for x in old['signed_payloads'] if x['kind']=='sbom' and x['architecture']=='arm64')
with tempfile.TemporaryDirectory(dir=HERE,prefix='proof-size-') as directory:
 target=Path(directory)/'CONTROL_ONLY-small';target.write_bytes(b'x');real_stat=Path.stat
 def sized(path,*args,**kwargs):
  original=real_stat(path,*args,**kwargs)
  if path==target:
   values=list(original);values[6]=row['full_bundle_gzip_bytes'];return os.stat_result(values)
  return original
 with patch.object(Path,'stat',sized),patch.object(Path,'read_bytes',side_effect=AssertionError('NO real report body read')):
  try:adapter.Adapter.proof_reader(object(),target)
  except core.Unproved as error:refused=str(error)
  else:raise AssertionError('expected bound')
 results['runtime_proof_size']={'actual_retained_receipt_sha256':hashlib.sha256(receipt_raw).hexdigest(),
  'actual_bundle_bytes':row['bundle_bytes'],'actual_bundle_gzip_bytes':row['full_bundle_gzip_bytes'],
  'actual_predicate_bytes':row['decoded_predicate_sorted_json_bytes'],'synthetic_stat_sized_to_real_metadata_refusal':refused}
# Compression of synthetic repeated bytes, not a report/fixture program body.
try:core.gunzip_bounded(gzip.compress(b'x'*(16*1024*1024+1),mtime=0),16*1024*1024)
except core.Unproved as error:results['runtime_proof_size']['synthetic_16MiB_plus1_refusal']=str(error)

cat=catalog.Catalog((HERE/'source/original-inputs/prepared-fixtures.json').read_bytes(),(HERE/'source/EMISSION-MAP.json').read_bytes())
secrets=[]
for client in ('bex','render'):
 spec=cat.spec((91,client),datetime.datetime(2026,11,1,2,0,tzinfo=datetime.timezone.utc),set())
 body=post(spec);encoded=(HERE/'source/original-inputs/fixtures/m91/qa402.dump.b64').read_bytes()
 body['secretFiles']=[{'name':'qa402.dump.b64','content':encoded.decode()}]
 try:catalog.validate_post(body,spec)
 except core.Unproved as error:message=str(error)
 else:raise AssertionError('expected existing secret-file blocker')
 secrets.append({'client':client,'encoded_input_bytes':len(encoded),'encoded_input_sha256':core.sha(encoded),'original_has_flag':spec['create_args'].count('--secret-file')==1,
  'actual_schema_fields':['secretFiles','name','content'],'current_refusal':message})
results['m91_authored_QA_secretfile']=secrets

helper=AdoptionControls()
contract,qualified,entry,docs,git,calls=helper.fixture();path=next(iter(contract['critical_files']))
# Keep the actual reviewed all-workspace selector unchanged. Change a different
# function in the same qualified blob, then omit delta coverage entirely.
old_literal=contract['pin_only_deltas'][0]['edits'][0]['before'];new_literal=contract['pin_only_deltas'][0]['edits'][0]['after']
before=docs[contract['pin_only_deltas'][0]['before']['path']]+b'func prepare() string { return "baseline" }\n'
after=before.replace(old_literal.encode(),new_literal.encode()).replace(b'return "baseline"',b'return "changed"')
qualified['qualification_source'][path]={'bytes':len(before),'sha256':core.sha(before)}
contract['critical_files'][path].update(bytes=len(after),sha256=core.sha(after));contract['pin_only_deltas']=[]
selector=b'func choose(workspace string) string { return root }\n';scope=contract['scope_code'][0]
scope.update(first_byte=after.index(selector),last_byte=after.index(selector)+len(selector))
def actual(action,left,right):return True if action=='ancestor' else (after,'100644','f'*40)
with patch.object(proof_binding,'bind_signature',return_value={'CONTROL_ONLY_ALREADY_VALIDATED_SIGNATURE':True}):
 accepted=readiness.adoption(contract,entry,qualified,docs.__getitem__,actual)
results['omitted_pin_delta_coverage']={'accepted':bool(accepted),'qualified_and_adopted_runtime_function_differ':True,'reviewed_selector_body_unchanged':True,
 'pin_only_deltas':[],'scope':'Isolated source binder; actual signatures NOT exercised'}

contract,qualified,entry,docs,git,calls=helper.fixture();path=next(iter(contract['critical_files']))
old_literal=contract['pin_only_deltas'][0]['edits'][0]['before'];new_literal=contract['pin_only_deltas'][0]['edits'][0]['after'];unqualified='ghcr.io/bex-co/native@sha256:'+'8'*64
before=docs[contract['pin_only_deltas'][0]['before']['path']]+('const unrelatedApproved = '+json.dumps(new_literal)+'\n').encode()
after=before.replace(json.dumps(old_literal).encode(),json.dumps(unqualified).encode())
before_ref=contract['pin_only_deltas'][0]['before'];docs[before_ref['path']]=before;before_ref.update(bytes=len(before),sha256=core.sha(before))
qualified['qualification_source'][path]={'bytes':len(before),'sha256':core.sha(before)}
contract['pin_only_deltas'][0]['edits'][0]['after']=unqualified
contract['critical_files'][path].update(bytes=len(after),sha256=core.sha(after));selector=b'func choose(workspace string) string { return root }\n'
contract['scope_code'][0].update(first_byte=after.index(selector),last_byte=after.index(selector)+len(selector))
with patch.object(proof_binding,'bind_signature',return_value={'CONTROL_ONLY_ALREADY_VALIDATED_SIGNATURE':True}):
 accepted=readiness.adoption(contract,entry,qualified,docs.__getitem__,lambda action,left,right:True if action=='ancestor' else (after,'100644','f'*40))
results['unqualified_delta_target']={'accepted':bool(accepted),'after_ref_not_in_qualified_refs':unqualified not in qualified['adoptable_refs'],
 'qualified_literal_exists_in_an_unrelated_constant':True,'scope':'Isolated source binder; actual signatures NOT exercised'}

contract,qualified,entry,docs,git,calls=helper.fixture();actual_path='lego/operator/internal/build/nativebases.go';before=(BASE/actual_path).read_bytes()
old_declaration=b'var nativeCommonBase string';new_declaration=('var nativeCommonBase string = '+json.dumps(qualified['common_ref'])).encode();assert before.count(old_declaration)==1
after=before.replace(old_declaration,new_declaration,1)
ref=contract['pin_only_deltas'][0]['before'];docs[ref['path']]=before;ref.update(bytes=len(before),sha256=core.sha(before))
qualified['qualification_source']={actual_path:{'bytes':len(before),'sha256':core.sha(before)}}
contract['critical_files']={actual_path:{'bytes':len(after),'sha256':core.sha(after),'git_mode':'100644','git_blob':'f'*40}}
contract['pin_only_deltas']=[{'qualification_source':qualified['qualified_baseline_source'],'path':actual_path,'before':ref,
 'edits':[{'before':old_declaration.decode(),'after':new_declaration.decode()}]}]
try:readiness.adoption(contract,entry,qualified,docs.__getitem__,lambda action,left,right:True if action=='ancestor' else (after,'100644','f'*40))
except core.Unproved as error:message=str(error)
else:raise AssertionError('expected zero-value grammar blocker')
results['actual_first_common_pin_slot']={'tracked_path':actual_path,'actual_before_declaration':old_declaration.decode(),
 'only_candidate_initializer_added':True,'current_refusal':message,'actual_repo_mutated':False,'signature_called':False}

# Actual retained old V11 receipt cannot be promoted with summary booleans. Stop
# at source-role binding; never read a report body or invoke crypto.
f=Facts();roles=f.contract['roles'];roles.update(producer=old['producer_workflow_sha'],verifier=old['verification_workflow_sha'])
f.entry['roles']=copy.deepcopy(roles);f.entry['common_ref']=old['ref'];f.contract['common_ref']=old['ref']
old_copy=copy.deepcopy(old);old_copy.update(currentQualified=True,allFamiliesPassed=True,adoptionReady=True,allWorkspaces=True)
f.replace(f.entry['v11_receipt'],old_copy)
try:readiness.qualification(f.entry,f.contract,f.docs.__getitem__)
except core.Unproved as error:message=str(error)
else:raise AssertionError('old receipt upgraded')
results['actual_old_V11_booleans_do_not_upgrade']={'retained_receipt_sha256':core.sha(receipt_raw),'refused':message,
 'extra_booleans':['currentQualified','allFamiliesPassed','adoptionReady','allWorkspaces'],'actual_old_receipt_unchanged':True,
 'scope':'Existing old receipt JSON copied into synthetic reader; missing actual current role proof refuses before report/crypto/live steps.'}
(HERE/'PROBE-RESULTS.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps(results,indent=2))
