"""SYNTHETIC K8s-read fixtures only; never calls kubectl or reads credentials."""
import contextlib,copy,datetime,hashlib,importlib.util,io,json,pathlib,subprocess,sys
sys.dont_write_bytecode=True
D=pathlib.Path('/tmp/bex-m188-census-v2-proposal');OLD=pathlib.Path('/tmp/bex-m188-final-live-census.py');NEW=D/'02231095776af7c1a38236fe46ab7f37fb82509fd1536d366c82d9e68e484913.py';STRICT=pathlib.Path('/tmp/bex-m188-priority-census-20261031-002113Z-5k8yle4y/pre-adoption-priority-census.raw.json');h=lambda p:hashlib.sha256(p.read_bytes()).hexdigest();assert h(OLD)=='c06ec446e5313729d8177d70d7cbd0627050a139a1af612a4a2571b66356f1ff' and h(NEW)=='02231095776af7c1a38236fe46ab7f37fb82509fd1536d366c82d9e68e484913';assert h(STRICT)=='f185407036de27f362f74302847549664a8738d247f9c41a43e668a621ceae6f';revision=json.loads(STRICT.read_bytes())['source_scope']['source_sha'];protected={str(p):h(p) for p in [OLD,STRICT,pathlib.Path('/tmp/bex-m188-prepare-adoption.py'),pathlib.Path('/tmp/bex-m188-published-acceptance.json')]}
def load(name,p):
 s=importlib.util.spec_from_file_location(name,p);g=importlib.util.module_from_spec(s);s.loader.exec_module(g);return g
old,new=load('census_original_c06',OLD),load('census_inactive_v2',NEW)
app={'apiVersion':'app.bex.co/v1alpha1','kind':'App','metadata':{'namespace':'fixture-team','name':'fixture-app','uid':'current-fixture-app-uid'},'spec':{'runtime':'python','builder':'native','type':'web_service'},'status':{'phase':'Failed'}}
image='registry.invalid/SYNTHETIC-retained:unknown@sha256:'+'5'*64
owner={'apiVersion':'app.bex.co/v1alpha1','kind':'App','name':'fixture-app','uid':'current-fixture-app-uid','controller':True}
work={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'namespace':'fixture-team','name':'fixture-app','generation':3,'ownerReferences':[owner]},'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'app','image':image}]}}}}
cases=[]
def add(name,w,expected,old_expected=None,a=None,pods=None):cases.append({'name':name,'workloads':[] if w is None else [w],'expected_templates':expected,'expected_old_templates':old_expected,'app':copy.deepcopy(a or app),'pods':pods or []})
add('label_less_current_UID_scale_zero',copy.deepcopy(work),1,0)
w=copy.deepcopy(work);w['metadata']['ownerReferences'][0]['uid']='prior-fixture-app-uid';add('wrong_UID_same_App_name',w,0,0)
w=copy.deepcopy(w);w['metadata']['labels']={'app.bex.co/app':'fixture-app'};add('wrong_UID_matching_label_refused',w,0,1)
w=copy.deepcopy(work);w['metadata']['namespace']='foreign-fixture-team';w['metadata']['labels']={'app.bex.co/app':'fixture-app'};add('other_namespace_refused',w,0,0)
w=copy.deepcopy(work);w['metadata'].pop('ownerReferences');w['metadata']['labels']={'app.bex.co/app':'fixture-app'};add('legacy_label_only_match_preserved',w,1,1)
w=copy.deepcopy(work);w['metadata']['labels']={'app.bex.co/app':'fixture-app'};add('label_and_current_owner_preserved',w,1,1)
w=copy.deepcopy(work);w['metadata'].pop('ownerReferences');add('exact_name_without_owner_or_label_refused',w,0,0)
w=copy.deepcopy(work);w['metadata']['labels']={'app.bex.co/app':'fixture-app'};bad=copy.deepcopy(owner);bad['uid']='conflicting-uid';w['metadata']['ownerReferences'].append(bad);add('multiple_App_owners_with_conflict_refused',w,0,1)
w=copy.deepcopy(work);w['metadata']['labels']={'app.bex.co/app':'fixture-app'};w['metadata']['ownerReferences'][0]['apiVersion']='foreign.example/v1';add('foreign_App_API_group_refused',w,0,1)
w=copy.deepcopy(work);w['metadata']['labels']={'app.bex.co/app':'fixture-app'};w['metadata']['ownerReferences'][0]['name']='different-fixture-app';add('same_UID_wrong_owner_name_refused',w,0,1)
w=copy.deepcopy(work);w['kind']='CronJob';w['apiVersion']='batch/v1';w['spec']={'suspend':True,'jobTemplate':{'spec':{'template':{'spec':{'containers':[{'name':'app','image':image}]}}}}};add('label_less_UID_owned_suspended_CronJob_retained',w,1,0)
a=copy.deepcopy(app);a['status']={'phase':'Canceled','artifactImage':image};add('candidate_artifact_is_not_active_release_evidence',None,0,0,a)
pod={'metadata':{'namespace':'fixture-team','name':'fixture-running-pod','labels':{'app.bex.co/app':'fixture-app'}},'spec':{'containers':[{'name':'app','image':image}]},'status':{'phase':'Running','containerStatuses':[{'name':'app','imageID':'unknown','ready':False}],'conditions':[]}};add('running_unknown_runtime_probe_remains_unresolved',copy.deepcopy(work),1,None,pods=[pod])
original_run=subprocess.run;results=[];fixture_dir=D/'SYNTHETIC-fixture-results';fixture_dir.mkdir()
def execute(g,which,case):
 out=fixture_dir/(case['name']+'.'+which+'.json');calls=[]
 def fake_run(argv,*args,**kwargs):
  if argv[0]=='git':
   assert argv[:2]==['git','show'] and len(argv)==3;return original_run(argv,*args,**kwargs)
  assert argv[0]=='kubectl','Unexpected process '+repr(argv)
  if 'get' in argv:
   kind=argv[argv.index('get')+1];mapping={'apps.app.bex.co':[case['app']],'pods':case['pods'],'deployments.apps,cronjobs.batch':case['workloads']};assert kind in mapping;calls.append('FIXTURE-get:'+kind);return subprocess.CompletedProcess(argv,0,json.dumps({'items':mapping[kind]}),'')
  assert 'exec' in argv and case['pods'];calls.append('FIXTURE-exec:empty-public-runtime-result');return subprocess.CompletedProcess(argv,0,'','')
 saved_argv=sys.argv;subprocess.run=fake_run;sys.argv=[str(g.__file__),'--source-sha',revision,'--kubeconfig','/SYNTHETIC/NOT-READ.kubeconfig','--output',str(out)]
 try:
  with contextlib.redirect_stdout(io.StringIO()):g.main()
 finally:sys.argv=saved_argv;subprocess.run=original_run
 data=json.loads(out.read_bytes());assert len(data['rows'])==1 and data['zero_affected_conclusion_allowed'] is False and data['no_apply_performed'] is True;row=data['rows'][0];assert row['unresolved'];return data,row,{'file':str(out),'sha256':h(out),'mocked_kubectl_calls':calls}
for case in cases:
 before,brow,bproof=execute(old,'old',case);after,arow,aproof=execute(new,'v2',case);assert len(arow['workload_templates'])==case['expected_templates'],case['name']
 if case['expected_old_templates'] is not None:assert len(brow['workload_templates'])==case['expected_old_templates'],case['name']
 if case['name']=='label_less_current_UID_scale_zero':
  assert arow['workload_templates'][0]['replicas']==0 and arow['fallback_active_image_candidates']==[image] and not arow['pods'] and not arow['affected_signals'];assert any('No-pod is not safe' in x for x in arow['unresolved'])
 if case['name']=='label_less_UID_owned_suspended_CronJob_retained':assert arow['workload_templates'][0]['suspended'] is True and image in arow['fallback_active_image_candidates']
 if case['name']=='candidate_artifact_is_not_active_release_evidence':assert not arow['fallback_active_image_candidates']
 if case['name']=='running_unknown_runtime_probe_remains_unresolved':assert any('version probe incomplete' in x for x in arow['unresolved'])
 results.append({'case':case['name'],'passed':True,'old_templates':len(brow['workload_templates']),'new_templates':len(arow['workload_templates']),'zero_conclusion_remains_false':True,'old':bproof,'proposed':aproof})
for p,d in protected.items():assert h(pathlib.Path(p))==d,p
report={'schema':'bex.m188.INACTIVE-census-owner-UID-isolated-controls/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'synthetic_fixture_only':True,'actual_production_kubectl_calls':0,'actual_main_and_source_scope_executed':True,'exact_deployed_source_for_scope':revision,'old_helper':{'file':str(OLD),'sha256':h(OLD)},'inactive_proposal':{'file':str(NEW),'sha256':h(NEW)},'harness':{'file':__file__,'sha256':h(pathlib.Path(__file__))},'controls':results,'all13_controls_passed':True,'predicate_audit':['Same namespace precedes ownership and label checks.','Current App owner UID/name/API group required if any App owner exists; any conflict rejects even matching labels.','No exact-workload-name fallback added; existing owner-less app label match retained.','No-running-pod branch always adds unresolved regardless replicas0/suspended/Failed/Canceled/deleting; active template/status image only becomes a pointer to investigate.','Retained image/no-pod does not imply served, healthy, patched, safe, or excluded. No artifact metadata resolution is performed.','Candidate-only artifactImage is not promoted to active status image evidence.','Running pod with unknown/incomplete required runtime probe remains unresolved.','Global zero predicate remains not affected AND not unresolved, with existing explicit customer/static exclusions and not-a-security-acceptance limits unchanged.','Pod label matching, runtime scope/probes and final zero predicate are unchanged by this narrowly scoped workload ownership fix.'], 'source_ownership_pointers':['lego/operator/internal/controller/release_config_snapshot.go:732 SetControllerReference(app,dep)','lego/operator/internal/controller/app_controller.go:4718 SetControllerReference(app,cj)'],'preserved_hashes':protected,'old_strict_report_unresolved_count':5,'proposal_not_activated':True,'scope':'Only TEMP inactive proposal and synthetic local read fixtures. No production census, credential/secret/Appenv access, old helper/report/activeguard/aggregate/source/board/remote mutation.'};out=D/'isolated-controls.json';out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'report':str(out),'sha256':h(out),'harness_sha256':h(pathlib.Path(__file__)),'controls':len(results),'all_passed':True,'old_helper_unchanged':h(OLD),'initial_strict_report_unchanged':h(STRICT)}))
