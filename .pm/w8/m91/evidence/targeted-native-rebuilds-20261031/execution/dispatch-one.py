import datetime,hashlib,json,os,pathlib,re,shutil,subprocess,sys,time
OUT=pathlib.Path('/private/tmp/bex-w8-native-targeted-dispatch-20261031-0952')
PLAN=pathlib.Path('/private/tmp/bex-w8-native-targeted-rebuild-plan-20261031-0928')
ALLOWED=['python-3.11-bookworm','python-3.13-bookworm','node-26-bookworm']
HEADERS=['-H','Accept: application/vnd.github+json','-H','X-GitHub-Api-Version: 2022-11-28']
ROOT='repos/bex-co/bex/'
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def dump(p,a):p.write_text(json.dumps(a,sort_keys=True,indent=2)+'\n')
def sha(b):return hashlib.sha256(b).hexdigest()
name=sys.argv[1];assert name in ALLOWED
case=OUT/name;case.mkdir(exist_ok=False)
reads=[]
def get(endpoint,label):
 for attempt in range(1,4):
  started=now()
  try:
   p=subprocess.run(['gh','api','--method','GET',*HEADERS,ROOT+endpoint],env=os.environ|{'GODEBUG':'http2client=0'},capture_output=True,timeout=80)
   (case/(label+f'.get{attempt}.stdout')).write_bytes(p.stdout);(case/(label+f'.get{attempt}.stderr')).write_bytes(p.stderr)
   reads.append({'label':label,'attempt':attempt,'endpoint':endpoint,'method':'GET','started_at':started,'ended_at':now(),'exit_code':p.returncode,'stdout_sha256':sha(p.stdout)})
   dump(case/'read-journal.json',reads)
   if p.returncode==0:return json.loads(p.stdout)
  except subprocess.TimeoutExpired as e:
   reads.append({'label':label,'attempt':attempt,'endpoint':endpoint,'method':'GET','started_at':started,'ended_at':now(),'error':'TimeoutExpired'})
   dump(case/'read-journal.json',reads)
  if attempt<3:time.sleep(attempt*2)
 raise RuntimeError('Read-only metadata GET failed; no new POST may follow')
source=json.loads((PLAN/'source-input-compatibility-current.json').read_text())
expected={p:{k:row[k] for k in ['mode','git_blob_id']} for p,row in source['current_native_inputs'].items()}
def selected(p):return p.startswith('deploy/native-base/') and not p.endswith('.md') or p in ['.github/workflows/native-base-image.yml','.github/native-base-recovery.json','lego/operator/internal/build/native.go','.github/scripts/native-oci-stage.py'] or p.startswith('.github/scripts/test_native_oci_') and p.endswith('.py')
verified={}
def check_source(commit,label):
 assert re.fullmatch('[a-f0-9]{40}',commit)
 if commit in verified:return verified[commit]
 if commit==source['current_published_main']:
  result={'source':commit,'native_closure_equal_fixed2cc':True,'method':'Current fresh main SHA equals immutable full source closure previously independently root-reviewed; every selected Git blob/mode and content SHA are frozen in original plan.','source_proof':str(PLAN/'source-input-compatibility-current.json'),'source_proof_sha256':sha((PLAN/'source-input-compatibility-current.json').read_bytes())}
 else:
  tree=get('git/trees/'+commit+'?recursive=1',label+'-tree');assert not tree['truncated']
  actual={x['path']:{'mode':x['mode'],'git_blob_id':x['sha']} for x in tree['tree'] if x['type']=='blob' and selected(x['path'])}
  assert actual==expected,'Native closure changed; STOP dependent POSTs for review'
  result={'source':commit,'native_closure_equal_fixed2cc':True,'method':'Fresh untruncated immutable Git tree path/mode/blob map equals all30 root-reviewed fixed2cc inputs. Git blobs bind the already reviewed content.','selected_inputs':actual}
 dump(case/(label+'-source-proof.json'),result);verified[commit]=result;return result
assert shutil.disk_usage(OUT).free>=2*(1<<30),'Global reserve insufficient'
main=get('git/ref/heads/main','preflight-main')['object']['sha'];check_source(main,'preflight')
allruns=[];total=None
for page in range(1,11):
 body=get('actions/workflows/native-base-image.yml/runs?per_page=100&page='+str(page),'preflight-runs-page'+str(page))
 if total is None:total=body['total_count']
 assert total==body['total_count'] and not body.get('incomplete_results',False),'Inventory changed/incomplete; stop before POST'
 allruns+=body['workflow_runs']
 if len(allruns)>=total:break
assert len(allruns)==total and len({x['id'] for x in allruns})==total,'Inventory not complete'
known=json.loads(pathlib.Path('/private/tmp/bex-m188-status-known.json').read_text());assert set(known)<={str(r['id']) for r in allruns}
active=[]
for run in allruns:
 if run['status']=='completed':continue
 jobs=get('actions/runs/'+str(run['id'])+'/jobs?per_page=100','preflight-jobs-'+str(run['id']))
 assert jobs['total_count']==len(jobs['jobs'])
 native=[j for j in jobs['jobs'] if j['name'].startswith('build (')]
 assert native and len(native)==1 and jobs['total_count']==2,'Unbound/full active matrix; stop before POST'
 j=native[0];match=re.match(r'build \(([^,]+),',j['name']);assert match
 runtime=match.group(1);assert runtime!=name,'Target already has a nonterminal request; no POST'
 active.append({'run_id':run['id'],'source':run['head_sha'],'run_status':run['status'],'runtime':runtime,'job_id':j['id'],'job_status':j['status'],'conclusion':j['conclusion']})
main2=get('git/ref/heads/main','immediate-main')['object']['sha'];check_source(main2,'immediate')
original=PLAN/('dispatch-'+name+'.json');payload=json.loads(original.read_text());assert payload=={'ref':'main','inputs':{'runtime':name}}
record=next(x for x in json.loads((PLAN/'FROZEN-MANIFEST.json').read_text())['files'] if x['path']==original.name);assert record['sha256']==sha(original.read_bytes())
payload['return_run_details']=True;dump(case/'request.json',payload)
dump(case/'preflight.json',{'at':now(),'target':name,'source_immediately_before_POST':main2,'official_base':source['target_bases'][name],'total_inventory_runs':total,'known_historical_run_count':len(known),'nonterminal_other_requests':active,'target_duplicate':False,'complete_native_closure_equal_2cc':True,'free_bytes':shutil.disk_usage(OUT).free,'original_payload_sha256':record['sha256'],'request_headers':{'Accept':'application/vnd.github+json','X-GitHub-Api-Version':'2022-11-28'},'only_execution_payload_change':'Top-level return_run_details:true; runtime and ref unchanged.'})
assert not (case/'POST-INTENT.json').exists()
dump(case/'POST-INTENT.json',{'at':now(),'method':'POST','endpoint':ROOT+'actions/workflows/native-base-image.yml/dispatches','request_sha256':sha((case/'request.json').read_bytes()),'rule':'Exactly one subprocess request. No mutation retry on any uncertainty/204. Resolve accepted run through reads.'})
started=now()
try:
 p=subprocess.run(['gh','api','--method','POST',*HEADERS,'--include',ROOT+'actions/workflows/native-base-image.yml/dispatches','--input',str(case/'request.json')],env=os.environ|{'GODEBUG':'http2client=0'},capture_output=True,timeout=90)
 (case/'post-response.raw').write_bytes(p.stdout);(case/'post-stderr.raw').write_bytes(p.stderr)
 raw=p.stdout.replace(b'\r\n',b'\n');parts=raw.split(b'\n\n',1);match=re.match(rb'HTTP/[\d.]+ (\d{3})',parts[0]);status=int(match.group(1)) if match else None
 result={'at':now(),'started_at':started,'process_exit':p.returncode,'http_status':status,'response_sha256':sha(p.stdout),'response_bytes':len(p.stdout),'no_mutation_retry':True}
 dump(case/'post-result.json',result)
 assert p.returncode==0 and status==200 and len(parts)==2,'Uncertain/non-detail POST response; NO RESEND, resolve through reads'
 details=json.loads(parts[1]);rid=details['workflow_run_id'];assert type(rid) is int
 assert details['run_url']=='https://api.github.com/repos/bex-co/bex/actions/runs/'+str(rid) and details['html_url']=='https://github.com/bex-co/bex/actions/runs/'+str(rid)
 dump(case/'returned-run-details.json',details)
 run=get('actions/runs/'+str(rid),'accepted-run');assert run['id']==rid and run['event']=='workflow_dispatch' and run['head_branch']=='main' and run['path']=='.github/workflows/native-base-image.yml'
 check_source(run['head_sha'],'accepted')
 jobs=get('actions/runs/'+str(rid)+'/jobs?per_page=100','accepted-jobs');assert jobs['total_count']==len(jobs['jobs'])
 native=[j for j in jobs['jobs'] if j['name'].startswith('build (')]
 assert len(native)<=1
 if native:assert native[0]['name'].startswith('build ('+name+', ')
 final={'at':now(),'status':'ACCEPTED_HTTP200_WITH_DOCUMENTED_RUN_ID','target':name,'run_id':rid,'html_url':details['html_url'],'source':run['head_sha'],'run_status':run['status'],'run_conclusion':run['conclusion'],'actual_jobs':[{'id':j['id'],'name':j['name'],'status':j['status'],'conclusion':j['conclusion']} for j in jobs['jobs']],'native_job_id':native[0]['id'] if native else None,'native_job_runtime_bound':bool(native),'official_base_from_exact_reviewed_source_matrix':source['target_bases'][name],'scope':'Normal singleton source build requested. No job/candidate/supplier approval inferred.'}
 dump(case/'result.json',final);print(json.dumps(final,indent=2))
except subprocess.TimeoutExpired as e:
 (case/'post-response-timeout.raw').write_bytes(e.stdout or b'');(case/'post-stderr-timeout.raw').write_bytes(e.stderr or b'')
 dump(case/'post-result.json',{'at':now(),'started_at':started,'error':'POST subprocess timeout; acceptance uncertain','no_mutation_retry':True})
 raise
