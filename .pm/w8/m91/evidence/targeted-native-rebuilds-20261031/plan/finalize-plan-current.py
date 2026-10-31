import concurrent.futures,datetime,hashlib,json,pathlib,re,shutil,subprocess
OUT=pathlib.Path('/private/tmp/bex-w8-native-targeted-rebuild-plan-20261031-0928')
ROOT=pathlib.Path('/Users/tianpan/projects/bex7')
TARGETS=['python-3.11-bookworm','python-3.13-bookworm','node-26-bookworm']
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def dump(p,a):p.write_text(json.dumps(a,indent=2,sort_keys=True)+'\n')
def get(item):
 name,path=item;start=now()
 p=subprocess.run(['gh','api','--method','GET',path],env=__import__('os').environ|{'GODEBUG':'http2client=0'},capture_output=True,timeout=80)
 if p.returncode:raise RuntimeError('read-only GET failed '+name+'; exit='+str(p.returncode))
 data=json.loads(p.stdout);(OUT/name).write_bytes(p.stdout)
 return name,data,{'file':name,'read_started_at':start,'read_completed_at':now(),'sha256':hashlib.sha256(p.stdout).hexdigest(),'bytes':len(p.stdout),'method':'GET','endpoint':path}
queries=[('final-main-ref.json','repos/bex-co/bex/git/ref/heads/main'),('final-runs.json','repos/bex-co/bex/actions/workflows/native-base-image.yml/runs?per_page=100'),('final-queued.json','repos/bex-co/bex/actions/workflows/native-base-image.yml/runs?per_page=100&status=queued'),('final-running.json','repos/bex-co/bex/actions/workflows/native-base-image.yml/runs?per_page=100&status=in_progress')]
data={};reads=[]
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
 for name,body,row in pool.map(get,queries):data[name]=body;reads.append(row)
source=json.loads((OUT/'source-input-compatibility-current.json').read_text())
assert data['final-main-ref.json']['object']['sha']==source['current_published_main'],'Main advanced: do not use stale plan'
runs=data['final-runs.json'];assert not runs.get('incomplete_results',False) and runs['total_count']==len(runs['workflow_runs'])
allids={str(x['id']) for x in runs['workflow_runs']};known=json.loads(pathlib.Path('/private/tmp/bex-m188-status-known.json').read_text());assert set(known)<=allids
active=[r for r in runs['workflow_runs'] if r['status']!='completed']
assert {r['id'] for r in active}=={r['id'] for n in ['final-queued.json','final-running.json'] for r in data[n]['workflow_runs']},'Active status scope changed during snapshot'
for n in ['final-queued.json','final-running.json']:assert data[n]['total_count']==len(data[n]['workflow_runs'])
queries=[('final-jobs-'+str(r['id'])+'.json','repos/bex-co/bex/actions/runs/'+str(r['id'])+'/jobs?per_page=100') for r in active]
active_rows=[]
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
 for name,body,row in pool.map(get,queries):data[name]=body;reads.append(row)
for r in active:
 body=data['final-jobs-'+str(r['id'])+'.json'];assert body['total_count']==len(body['jobs'])
 native=[j for j in body['jobs'] if j['name'].startswith('build (')]
 assert len(native)==1 and body['total_count']==2,'Unexpected full/multi-row active matrix: root must review backlog'
 j=native[0];match=re.match(r'build \(([^,]+),',j['name']);assert match
 runtime=match.group(1);assert runtime not in TARGETS,'New duplicate target request exists'
 active_rows.append({'run_id':r['id'],'head_sha':r['head_sha'],'event':r['event'],'run_status':r['status'],'run_conclusion':r['conclusion'],'run_attempt':r['run_attempt'],'job_id':j['id'],'runtime':runtime,'job_name':j['name'],'job_status':j['status'],'job_conclusion':j['conclusion'],'actual_in_progress_steps':[{'name':s['name'],'started_at':s['started_at']} for s in j['steps'] if s['status']=='in_progress'],'all_materialized_jobs_count':body['total_count'],'latent_matrix_assessment':'Exactly matrix + one reviewed targeted runtime job; no active full-matrix run in complete workflow inventory. Existing Go/Elixir/Node18 target ownership is preserved.'})
oldids={'python-3.11-bookworm':(41129044087,124319471502),'python-3.13-bookworm':(41129116273,124319435370),'node-26-bookworm':(41129008590,124317150933)}
old=[]
for name,(rid,jid) in oldids.items():
 run=next(r for r in runs['workflow_runs'] if r['id']==rid);assert run['status']=='completed' and run['conclusion']=='failure'
 jobs=json.loads((OUT/('jobs-'+str(rid)+'.json')).read_text());j=next(j for j in jobs['jobs'] if j['id']==jid);assert j['status']=='completed' and j['conclusion']=='failure'
 old.append({'runtime':name,'run_id':rid,'job_id':jid,'source':run['head_sha'],'run_status':run['status'],'run_conclusion':run['conclusion'],'job_conclusion':j['conclusion'],'failed_steps':[s['name'] for s in j['steps'] if s['conclusion']=='failure'],'raw_logs':'Previously retained originals only; no log redownload in this plan.'})
free=shutil.disk_usage(OUT).free
plan={'schema':'bex.native-targeted-normal-rebuild-plan/private-v1','at':now(),'status':'PREPARED_ONLY_NOT_DISPATCHED','expected_published_main':source['current_published_main'],'targets':TARGETS,'endpoint_for_parent_review':'POST /repos/bex-co/bex/actions/workflows/native-base-image.yml/dispatches','payloads':[{'runtime':n,'file':'dispatch-'+n+'.json','payload':{'ref':'main','inputs':{'runtime':n}},'official_base':source['target_bases'][n],'mode':'normal source rebuild; no old-index recovery'} for n in TARGETS],'deduplication':{'complete_inventory_run_count':len(runs['workflow_runs']),'known_historical_run_count':len(known),'every_known_run_in_complete_current_inventory':True,'nonterminal_runs':active_rows,'target_nonterminal_duplicates':[],'active_full_matrix_or_implicit_target_backlog_observed':False,'automatic_fix_runs':[{'run_id':x,'status':next(r['status'] for r in runs['workflow_runs'] if r['id']==x),'conclusion':next(r['conclusion'] for r in runs['workflow_runs'] if r['id']==x)} for x in [41182921386,41184126238]]},'original_failed_publications':old,'existing_failure_evidence':['/private/tmp/bex-w8-native-failure-triage-20261031-0805','/private/tmp/bex-w8-native-failures-20261031/python311/job.log','/private/tmp/bex-w8-native-failures-20261031/python313/job.log'],'capacity_and_fit':{'local_free_bytes_at_freeze':free,'plan_only_no_local_build_or_payload_reads':True,'existing_runner_labels':source['factory_shape']['build_runner_labels'],'new_runners_or_resources':False,'each_payload_selects_exactly_one_existing_reviewed_name':True,'per_runtime_concurrency_cancel_in_progress':False,'three_targeted_runs_can_queue_on_existing_runner_capacity':'No throughput or immediate-start promise; matrix max-parallel=2 applies within each run and does not globally cap three singleton runs. Existing production work and parent PG gate remain untouched.','parent_PG_builder_gate':'3GiB floor plus6GiB headroom; no action or cleanup by this agent.'},'required_just_in_time_parent_guards':['Fresh-read main SHA and native input closure before any future dispatch; ref=main resolves at dispatch time. If main advances with unchanged reviewed native closure, bind actual resulting source/job identity; if native inputs change, re-review plan.','Fresh whole-workflow status and jobs inventory, including queued/full matrices and known requests; do not dispatch a target if a peer has already requested it.','After each authorized dispatch, record actual new run ID, exact head SHA, selected singleton runtime and full base. Do not infer run identity from creation order.','Retain both architecture probes, both complete scans, full SPDX/signing/attestation/discovery gates; a green job is not independent supplier approval.'],'not_authorized':['No dispatch, cancellation, rerun, new language/base scope, adoption, production pin, source/catalog edit, Docker, fixtures or cleanup performed.'],'reads':reads}
dump(OUT/'rebuild-plan.json',plan)
print(json.dumps({'at':plan['at'],'source':plan['expected_published_main'],'inventory':len(runs['workflow_runs']),'active':active_rows,'targets':TARGETS,'free_bytes':free},indent=2))
