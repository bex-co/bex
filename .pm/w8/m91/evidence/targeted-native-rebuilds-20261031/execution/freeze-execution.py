import concurrent.futures,datetime,hashlib,json,os,pathlib,re,shutil,subprocess
OUT=pathlib.Path('/private/tmp/bex-w8-native-targeted-dispatch-20261031-0952')
PLAN=pathlib.Path('/private/tmp/bex-w8-native-targeted-rebuild-plan-20261031-0928')
NAMES=['python-3.11-bookworm','python-3.13-bookworm','node-26-bookworm']
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def sha(raw):return hashlib.sha256(raw).hexdigest()
def dump(p,a):p.write_text(json.dumps(a,indent=2,sort_keys=True)+'\n')
results=[json.loads((OUT/n/'bound-result.json').read_text()) for n in NAMES]
def get(item):
 n,rid,kind=item;case=OUT/n;start=now();endpoint='repos/bex-co/bex/actions/runs/'+str(rid)+('/jobs?per_page=100' if kind=='jobs' else '')
 p=subprocess.run(['gh','api','--method','GET','-H','X-GitHub-Api-Version: 2022-11-28',endpoint],env=os.environ|{'GODEBUG':'http2client=0'},capture_output=True,timeout=80)
 (case/('freeze-'+kind+'.stdout')).write_bytes(p.stdout);(case/('freeze-'+kind+'.stderr')).write_bytes(p.stderr)
 assert p.returncode==0
 return n,kind,json.loads(p.stdout),{'runtime':n,'kind':kind,'started_at':start,'ended_at':now(),'exit':0,'stdout_sha256':sha(p.stdout)}
queries=[(n,r['run_id'],kind) for n,r in zip(NAMES,results) for kind in ['run','jobs']];fresh={};reads=[]
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
 for n,kind,body,row in pool.map(get,queries):fresh[n,kind]=body;reads.append(row)
summary=[]
for n,r in zip(NAMES,results):
 case=OUT/n;req=json.loads((case/'request.json').read_text());assert req=={'ref':'main','inputs':{'runtime':n},'return_run_details':True}
 original=json.loads((PLAN/('dispatch-'+n+'.json')).read_text());assert original=={'ref':'main','inputs':{'runtime':n}}
 status=json.loads((case/'post-result.json').read_text());assert status['process_exit']==0 and status['http_status']==200 and status['no_mutation_retry']
 details=json.loads((case/'returned-run-details.json').read_text());assert details['workflow_run_id']==r['run_id']
 inventory=[]
 for f in case.glob('preflight-runs-page*.get1.stdout'):inventory+=json.loads(f.read_text())['workflow_runs']
 assert r['run_id'] not in {x['id'] for x in inventory}
 run=fresh[n,'run'];jobs=fresh[n,'jobs'];assert jobs['total_count']==len(jobs['jobs'])==2
 assert run['id']==r['run_id'] and run['head_sha']==r['source'] and run['event']=='workflow_dispatch' and run['head_branch']=='main' and run['path']=='.github/workflows/native-base-image.yml'
 native=[j for j in jobs['jobs'] if j['name'].startswith('build (')];assert len(native)==1
 job=native[0];assert job['id']==r['native_job_id'] and job['name'].startswith('build ('+n+', ')
 assert any(j['name']=='matrix' and j['conclusion']=='success' for j in jobs['jobs'])
 proofs=[]
 for f in case.glob('*-source-proof.json'):
  a=json.loads(f.read_text())
  if a['source']==r['source'] and a['native_closure_equal_fixed2cc']:proofs.append({'path':str(f.relative_to(OUT)),'sha256':sha(f.read_bytes())})
 assert proofs
 summary.append({'runtime':n,'run_id':run['id'],'run_url':run['html_url'],'job_id':job['id'],'job_url':job['html_url'],'source':run['head_sha'],'run_status':run['status'],'run_conclusion':run['conclusion'],'job_status':job['status'],'job_conclusion':job['conclusion'],'active_steps':[{'name':s['name'],'started_at':s['started_at']} for s in job['steps'] if s['status']=='in_progress'],'matrix_conclusion':'success','native_job_name':job['name'],'official_base_from_exact_reviewed_source_matrix':r['official_base_from_exact_reviewed_source_matrix'],'base_scope_note':'GitHub job display truncates the digest; the full base is independently derived from the exact reviewed source matrix and bound to the actual singleton runtime, not guessed from truncated text.','source_proofs':proofs,'http_status':200,'POST_process_exit':0,'POST_response_sha256':status['response_sha256'],'returned_identity':'Documented return_run_details:true response; not creation-order guessing','attempt_count':1,'raw_request':str((case/'request.json').relative_to(OUT)),'raw_response':str((case/'post-response.raw').relative_to(OUT))})
assert len({r['run_id'] for r in summary})==3
frozen=json.loads((PLAN/'FROZEN-MANIFEST.json').read_text())
for x in frozen['files']:
 raw=(PLAN/x['path']).read_bytes();assert len(raw)==x['bytes'] and sha(raw)==x['sha256']
receipt={'schema':'bex.native-targeted-dispatch-actual/private-v1','frozen_at':now(),'outcome':'Three authorized singleton normal-source dispatches each accepted once with HTTP200 and direct returned run identity; all3 actual native jobs bound after successful matrices.','mutations':{'POST_count':3,'success_count':3,'uncertain_or_204_count':0,'mutation_retries':0,'cancel_count':0,'rerun_count':0,'full_matrix_dispatch_count':0},'actual_runs':summary,'original_plan':str(PLAN),'original_plan_manifest_sha256':sha((PLAN/'FROZEN-MANIFEST.json').read_bytes()),'all71_original_plan_payload_hashes_unchanged':True,'scope':'Fresh source/dedupe guards passed separately before each POST. Every actual source retains all30 fixed2cc native inputs. All original proofs/histories preserved. No repository/catalog/Git/Docker/fixtures/cleanup/adoption changes. Neither accepted CI requests nor future green jobs approve a supplier.','official_response_contract':'https://docs.github.com/en/rest/actions/workflows?apiVersion=2022-11-28#create-a-workflow-dispatch-event','pinned_api_header':'2022-11-28','source_mutability':'Dispatch ref was main; actual d95/e48 source identities were captured and independently checked after acceptance. Both were compatible with fixed2cc.','fresh_final_reads':reads,'global_free_bytes':shutil.disk_usage(OUT).free}
dump(OUT/'actual-execution-receipt.json',receipt)
files=[]
for f in sorted(OUT.rglob('*')):
 if f.is_file() and f.name!='FROZEN-MANIFEST.json':
  b=f.read_bytes();files.append({'path':str(f.relative_to(OUT)),'bytes':len(b),'sha256':sha(b)})
assert sum(x['bytes'] for x in files)<2*(1<<30) and receipt['global_free_bytes']>=2*(1<<30)
manifest={'at':now(),'file_count':len(files),'bytes':sum(x['bytes'] for x in files),'files':files,'scope':'All private execution-root regular files except this manifest. No auth/token/request-secret material or public payload downloads.'}
dump(OUT/'FROZEN-MANIFEST.json',manifest)
for x in files:
 b=(OUT/x['path']).read_bytes();assert len(b)==x['bytes'] and sha(b)==x['sha256']
assert {str(f.relative_to(OUT)) for f in OUT.rglob('*') if f.is_file()}=={x['path'] for x in files}|{'FROZEN-MANIFEST.json'}
print(json.dumps({'runs':summary,'receipt_sha256':sha((OUT/'actual-execution-receipt.json').read_bytes()),'manifest_sha256':sha((OUT/'FROZEN-MANIFEST.json').read_bytes()),'payload_count':len(files),'payload_bytes':manifest['bytes'],'free_bytes':receipt['global_free_bytes']},indent=2))
