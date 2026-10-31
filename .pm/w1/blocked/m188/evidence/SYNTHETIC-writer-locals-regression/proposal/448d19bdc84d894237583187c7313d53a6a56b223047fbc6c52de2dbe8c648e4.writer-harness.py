"""SYNTHETIC writer-only main regression. validate_candidate is mocked; no crypto acceptance."""
import ast,contextlib,datetime,hashlib,importlib.util,io,json,pathlib,re,sys,tempfile
sys.dont_write_bytecode=True
ROOT=pathlib.Path('/Users/tianpan/projects/bex');ACTIVE=pathlib.Path('/tmp/bex-m188-prepare-adoption.py');AGG=pathlib.Path('/tmp/bex-m188-published-acceptance.json');OLD=pathlib.Path('/tmp/bex-m188-guard-snapshots/0bfb0d9e5541614baf87e07065b58c2c5ea849d6c83c152fb2ed698e42c75107.py');NEW=pathlib.Path('/tmp/bex-m188-guard-proposals/exact-writer-locals-fix/3fe1b2bd1a5a44ba7711f33546c1cf1db5539bdb36f08b2e4c8e40ff56e42c58.py');h=lambda p:hashlib.sha256(p.read_bytes()).hexdigest();assert h(ACTIVE)==h(OLD)=='0bfb0d9e5541614baf87e07065b58c2c5ea849d6c83c152fb2ed698e42c75107';assert h(NEW)=='3fe1b2bd1a5a44ba7711f33546c1cf1db5539bdb36f08b2e4c8e40ff56e42c58';assert h(AGG)=='eae0050dbd22e9c5b329ae38f5b1aa19fd868dbec31a123d4fcac606f7f630a1'
activation=json.loads(pathlib.Path('/tmp/bex-m188-unzip-compatibility-activation.json').read_bytes());protected=dict(activation['protected_hashes']);protected[str(OLD)]=h(OLD)
for p,d in protected.items():assert h(pathlib.Path(p))==d
protected_repo={str(ROOT/p):h(ROOT/p) for p in ['lego/operator/internal/build/native.go','lego/operator/internal/build/toolchain-freshness.json','lego/operator/internal/build/nativebases.go','deploy/native-base/matrix.py','scripts/lib/toolchain-freshness.py']}
base=pathlib.Path(tempfile.mkdtemp(prefix='bex-m188-SYNTHETIC-writer-only-',dir='/tmp'));(base/'SYNTHETIC-NOT-AN-ACCEPTANCE.txt').write_text('Writer-layer unit fixture only. validate_candidate is MOCKED. Digests/jobs/sources are synthetic. No signatures or artifact verification. Never adopt these outputs.\n')
def load(name,p):
 s=importlib.util.spec_from_file_location(name,p);g=importlib.util.module_from_spec(s);s.loader.exec_module(g);return g
old,new=load('writer_old_0bfb',OLD),load('writer_proposed_3fe',NEW)
def run_main(g,args):
 saved=sys.argv;out=io.StringIO();err=io.StringIO();sys.argv=[str(g.__file__),*args]
 try:
  with contextlib.redirect_stdout(out),contextlib.redirect_stderr(err):g.main()
 finally:sys.argv=saved
 return {'stdout':out.getvalue(),'stderr':err.getvalue()}
plan_path=base/'actual-input-mappings.json';run_main(old,['--receipt',str(AGG),'--plan',str(plan_path)]);plan=json.loads(plan_path.read_bytes());assert plan['receipt_verified_count']==8 and not plan['complete_receipt_available'];rows=plan['mappings'];assert len(rows)==11
synthetic={'SYNTHETIC_TEST_ONLY':True,'CRYPTO_VALIDATION_MOCKED':True,'source_model':old.SOURCE_MODEL,'state':'complete','remaining':[],'failures':[],'registry_repository':old.REPO,'tag_prefix':'native-','workflow':old.IDENTITY,'required_security_ancestor':old.ANCESTOR,'checked_at':'2026-10-30T00:00:00Z','verified':{}}
for i,row in enumerate(rows,1):
 tag=row['receipt_key'];digest='sha256:'+hashlib.sha256(('SYNTHETIC-WRITER-ONLY:'+tag).encode()).hexdigest();producer=hashlib.sha1(('SYNTHETIC-PRODUCER:'+tag).encode()).hexdigest();verifier=hashlib.sha1(('SYNTHETIC-VERIFIER:'+tag).encode()).hexdigest();synthetic['verified'][tag]={'SYNTHETIC_TEST_ONLY':True,'digest':digest,'job_url':f'https://github.com/bex-co/bex/actions/runs/{900000000000+i}/job/{990000000000+i}','source':producer,'verification_source':verifier,'runtime':row['runtime'],'version':row['version']}
synthetic_path=base/'SYNTHETIC-11-writer-receipt.json';synthetic_path.write_text(json.dumps(synthetic,indent=2)+'\n');calls={'old':[],'new':[]}
def mock_validate(which):
 def validate(c,row,receipt,matrix,native):
  assert receipt['SYNTHETIC_TEST_ONLY'] is True and receipt['CRYPTO_VALIDATION_MOCKED'] is True and c==synthetic['verified'][row['receipt_key']]
  assert matrix.matrix(native)==matrix.matrix((ROOT/old.NATIVE).read_text());calls[which].append(row['receipt_key']);return c['source'],c['verification_source']
 return validate
old.validate_candidate=mock_validate('old');new.validate_candidate=mock_validate('new');old_output=base/'SYNTHETIC-old-output-MUST-NOT-EXIST';old_error=None
try:run_main(old,['--receipt',str(synthetic_path),'--plan',str(base/'SYNTHETIC-old-plan.json'),'--prepare-dir',str(old_output)])
except NameError as error:old_error=str(error);assert old_error=="name 'digest' is not defined"
else:raise AssertionError('Old actual main did not reproduce NameError')
assert calls['old']==[rows[0]['receipt_key']] and not old_output.exists()
new_output=base/'SYNTHETIC-new-output-NOT-FOR-ADOPTION';new_run=run_main(new,['--receipt',str(synthetic_path),'--plan',str(base/'SYNTHETIC-new-plan.json'),'--prepare-dir',str(new_output)]);assert calls['new']==[r['receipt_key'] for r in rows]
source=(ROOT/old.NATIVE).read_text();prepared=(new_output/'native.go').read_text();inventory=json.loads((ROOT/old.INVENTORY).read_bytes());generated=json.loads((new_output/'toolchain-freshness.json').read_bytes());assert len(inventory['images'])==44 and len(generated['images'])==55;assert generated['images'][:44]==inventory['images'];assert {k:v for k,v in generated.items() if k!='images'}=={k:v for k,v in inventory.items() if k!='images'};additions=generated['images'][44:];assert len({x['committed'] for x in additions})==11;expected=source;checks=[]
for row,entry in zip(rows,additions):
 c=synthetic['verified'][row['receipt_key']];image=row['inventory_template']['upstream']+'@'+c['digest'];assert entry['id']==row['platform_inventory_id'] and entry['committed']==image and entry['upstream']==row['inventory_template']['upstream'];assert c['job_url'] in entry['source'] and ('producer '+c['source']) in entry['source'] and ('verification '+c['verification_source']) in entry['source'];assert all(other['job_url'] not in entry['source'] for other in synthetic['verified'].values() if other is not c);token='image: "'+row['upstream']+'"';assert expected.count(token)==1;expected=expected.replace(token,'image: "'+image+'", upstream: "'+row['upstream']+'"',1);assert image in prepared;checks.append({'tag':row['receipt_key'],'platform_id':entry['id'],'digest':c['digest'],'job_url':c['job_url'],'matched_own_row':True})
assert prepared==expected
matrix=new.load_module('writer_actual_matrix',ROOT/'deploy/native-base/matrix.py');freshness=new.load_module('writer_actual_freshness',ROOT/'scripts/lib/toolchain-freshness.py');assert matrix.matrix(prepared)==matrix.matrix(source);freshness.parse_inventory(json.dumps(generated))
refusal=base/'REAL-8-incomplete-MUST-NOT-EXIST';refusal_error=None
try:run_main(new,['--receipt',str(AGG),'--plan',str(base/'REAL-8-plan.json'),'--prepare-dir',str(refusal)])
except ValueError as error:refusal_error=str(error);assert refusal_error=='REFUSING adoption copies: receipt is incomplete; all 11 independently verified published candidates are required'
else:raise AssertionError('Real incomplete receipt unexpectedly generated output')
assert not refusal.exists()
for p,d in protected.items():assert h(pathlib.Path(p))==d,p
for p,d in protected_repo.items():assert h(pathlib.Path(p))==d,p
assert h(ACTIVE)==h(OLD) and h(AGG)=='eae0050dbd22e9c5b329ae38f5b1aa19fd868dbec31a123d4fcac606f7f630a1'
report={'schema':'bex.m188.SYNTHETIC-actual-main-writer-regression/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'synthetic_test_only':True,'crypto_validation_tested':False,'genuine_candidate_acceptance_or_adoption':False,'only_mocked_function':'validate_candidate','real_functions_executed':['main plan and writer loop','verify_local_green_proofs','native official matrix parser/equality','freshness.parse_inventory','incomplete11 refusal'],'harness':{'file':__file__,'sha256':h(pathlib.Path(__file__))},'old_guard':{'file':str(OLD),'sha256':h(OLD),'actual_main_result':'NameError','error':old_error,'validator_calls':calls['old'],'output_absent':not old_output.exists()},'proposed_guard':{'file':str(NEW),'sha256':h(NEW),'actual_main_result':'PASS','validator_calls':calls['new']},'synthetic_receipt':{'file':str(synthetic_path),'sha256':h(synthetic_path)},'synthetic_output':{'directory':str(new_output),'native_sha256':h(new_output/'native.go'),'inventory_sha256':h(new_output/'toolchain-freshness.json'),'original44_entries_preserved':True,'all_other_original_inventory_fields_preserved':True,'total_images':55,'all11_digest_and_job_provenance_distinct_and_correct':True,'official_matrix_unchanged':True,'exact_native_row_replacements_only':True,'row_checks':checks},'real8_incomplete_refusal':{'passed':True,'error':refusal_error,'output_absent':not refusal.exists()},'protected_original_files':len(protected),'repository_inputs_unchanged':protected_repo,'active_guard_unchanged':h(ACTIVE),'aggregate_unchanged':h(AGG),'scope':'Clearly synthetic local writer regression only. No signature/runtime/scan verification claim, real aggregate change, active guard change, repository change, source role addition, promotion or actual source adoption.'};out=base/'SYNTHETIC-writer-regression.json';out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'report':str(out),'report_sha256':h(out),'harness_sha256':h(pathlib.Path(__file__)),'old_actual_main':'NameError digest','new_actual_main':'11/55 PASS','real8_refusal':'PASS','protected':len(protected)}))
