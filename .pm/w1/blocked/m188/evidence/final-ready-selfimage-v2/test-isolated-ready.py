"""SYNTHETIC kube-read actual-main controls. No kubectl, secrets, source/signature acceptance."""
from pathlib import Path
import contextlib,copy,datetime,hashlib,importlib.util,io,json,subprocess,sys
sys.dont_write_bytecode=True
D=Path('/tmp/bex-m188-ready-v2-proposal');OLD=Path('/tmp/bex-m188-final-operator-ready.py');NEW=D/'7919ea686748433f2cf39d219eff7264feefe81e351bb96e1798a2fb6ae30ffc.py';H=lambda p:hashlib.sha256(Path(p).read_bytes()).hexdigest();assert H(OLD)=='1207980bfec032628e0f4f941da910cf61bcefea76f974486d70edeb69f19b6a';assert H(NEW)=='7919ea686748433f2cf39d219eff7264feefe81e351bb96e1798a2fb6ae30ffc'
protected={p:H(p) for p in ['/tmp/bex-m188-final-operator-ready.py','/tmp/bex-m188-final-t004-plan.json','/tmp/bex-m188-final-t004-plan-census-v2-addendum.json','/tmp/bex-m188-prepare-adoption.py','/tmp/bex-m188-published-acceptance.json']}
def load(name,path):
 s=importlib.util.spec_from_file_location(name,path);g=importlib.util.module_from_spec(s);s.loader.exec_module(g);return g
old,new=load('ready_original120',OLD),load('ready_inactive_v2',NEW)
def encode(x):return (json.dumps(x,separators=(',',':'))+'\n').encode()
def digest(b):return 'sha256:'+hashlib.sha256(b).hexdigest()
child=encode({'schemaVersion':2,'mediaType':'application/vnd.oci.image.manifest.v1+json','config':{'digest':'sha256:'+'b'*64,'size':0,'mediaType':'application/vnd.oci.image.config.v1+json'},'layers':[]});root=encode({'schemaVersion':2,'mediaType':'application/vnd.oci.image.index.v1+json','manifests':[{'mediaType':'application/vnd.oci.image.manifest.v1+json','digest':digest(child),'size':len(child),'platform':{'architecture':'amd64','os':'linux'}}]});image='ghcr.io/bex-co/bex-operator@'+digest(root);env=[{'name':'POD_NAMESPACE','valueFrom':{'fieldRef':{'fieldPath':'metadata.namespace'}}},{'name':'POD_NAME','valueFrom':{'fieldRef':{'apiVersion':'v1','fieldPath':'metadata.name'}}}];container={'name':'manager','image':image,'env':env};base={'deployment':{'metadata':{'name':'bex-controller-manager','namespace':'bex-system','uid':'deployment-uid','generation':2},'spec':{'replicas':1,'selector':{'matchLabels':{'control-plane':'controller-manager'}},'template':{'spec':{'containers':[container]}}},'status':{'observedGeneration':2,'updatedReplicas':1,'readyReplicas':1,'conditions':[{'type':'Available','lastTransitionTime':'2099-01-01T00:00:00Z'}]}},'replicasets':{'items':[{'metadata':{'uid':'rs-uid','ownerReferences':[{'kind':'Deployment','uid':'deployment-uid','controller':True}]}}]},'pods':{'items':[{'metadata':{'name':'manager-current','namespace':'bex-system','uid':'pod-current','ownerReferences':[{'kind':'ReplicaSet','uid':'rs-uid','controller':True}]},'spec':{'containers':[copy.deepcopy(container)]},'status':{'phase':'Running','containerStatuses':[{'name':'manager','image':image,'imageID':'docker-pullable://ghcr.io/bex-co/bex-operator@'+digest(child),'ready':True}],'conditions':[{'type':'Ready','status':'True','lastTransitionTime':'2026-10-31T03:00:00Z'}]}}]},'gitops':{'spec':{'source':{'kustomize':{'images':['controller='+image]}}},'status':{'sync':{'status':'Synced'},'health':{'status':'Healthy'}}}}
SECRET='SYNTHETIC_DO_NOT_DISCLOSE_VALUE';REFNAME='SYNTHETIC_DO_NOT_DISCLOSE_SECRET_NAME';REFKEY='SYNTHETIC_DO_NOT_DISCLOSE_KEY';cases=[]
def add(name,mutate=None,oldready=False,newready=False,since=None,rootbytes=root,childbytes=child):
 x=copy.deepcopy(base)
 if mutate:mutate(x)
 cases.append({'name':name,'data':x,'oldready':oldready,'newready':newready,'since':since,'rootbytes':rootbytes,'childbytes':childbytes})
def ctr(x,where):return x['deployment']['spec']['template']['spec']['containers'][0] if where=='template' else x['pods']['items'][0]['spec']['containers'][0]
def replace_env(x,where,name,value):
 c=ctr(x,where);c['env']=[e for e in c['env'] if e['name']!=name]+[dict(name=name,**value)]
add('valid_both_template_and_current_Pod',oldready=True,newready=True,since='2026-10-31T03:00:00Z')
for name,where,var,value in [('Pod_name_literal_points_to_other_Pod','pod','POD_NAME',{'value':SECRET}),('Pod_namespace_literal','pod','POD_NAMESPACE',{'value':SECRET}),('template_name_literal','template','POD_NAME',{'value':SECRET}),('Pod_namespace_secret','pod','POD_NAMESPACE',{'valueFrom':{'secretKeyRef':{'name':REFNAME,'key':REFKEY}}}),('template_name_ConfigMap','template','POD_NAME',{'valueFrom':{'configMapKeyRef':{'name':REFNAME,'key':REFKEY}}}),('Pod_resource_source','pod','POD_NAME',{'valueFrom':{'resourceFieldRef':{'resource':'limits.cpu'}}}),('Pod_unknown_source','pod','POD_NAME',{'valueFrom':{'UNRECOGNIZED_SECRET_SHAPED_KEY':{'name':REFNAME}}}),('Pod_wrong_fieldPath','pod','POD_NAME',{'valueFrom':{'fieldRef':{'fieldPath':'metadata.namespace'}}}),('Pod_wrong_fieldRef_apiVersion','pod','POD_NAME',{'valueFrom':{'fieldRef':{'fieldPath':'metadata.name','apiVersion':'v2'}}}),('Pod_mixed_literal_and_fieldRef','pod','POD_NAME',{'value':SECRET,'valueFrom':{'fieldRef':{'fieldPath':'metadata.name'}}}),('Pod_mixed_unknown_source','pod','POD_NAME',{'valueFrom':{'fieldRef':{'fieldPath':'metadata.name'},'secretKeyRef':{'name':REFNAME,'key':REFKEY}}})]:
 add(name,lambda x,w=where,n=var,v=value:replace_env(x,w,n,v),True,False)
add('Pod_duplicate_name_sources',lambda x:ctr(x,'pod')['env'].append({'name':'POD_NAME','valueFrom':{'fieldRef':{'fieldPath':'metadata.name'}}}),True,False)
add('template_duplicate_namespace_sources',lambda x:ctr(x,'template')['env'].append({'name':'POD_NAMESPACE','valueFrom':{'fieldRef':{'fieldPath':'metadata.namespace'}}}),True,False)
add('Pod_missing_name_source',lambda x:ctr(x,'pod').update(env=[e for e in ctr(x,'pod')['env'] if e['name']!='POD_NAME']),True,False)
add('Pod_injected_BackupHelper_override',lambda x:ctr(x,'pod')['env'].append({'name':'BEX_BACKUP_HELPER_IMAGE','value':SECRET}),True,False)
add('Pod_injected_envFrom_Secret',lambda x:ctr(x,'pod').update(envFrom=[{'secretRef':{'name':REFNAME}}]),True,False)
add('template_BackupHelper_override_still_refused',lambda x:ctr(x,'template')['env'].append({'name':'BEX_BACKUP_HELPER_IMAGE','value':SECRET}))
add('template_envFrom_still_refused',lambda x:ctr(x,'template').update(envFrom=[{'secretRef':{'name':REFNAME}}]))
add('Pod_not_Ready_still_refused',lambda x:x['pods']['items'][0]['status']['conditions'][0].update(status='False'))
add('wrong_runtime_imageID_still_refused',lambda x:x['pods']['items'][0]['status']['containerStatuses'][0].update(imageID='docker://sha256:'+'f'*64))
add('stale_Deployment_generation_still_refused',lambda x:x['deployment']['status'].update(observedGeneration=1))
add('wrong_Deployment_image_still_refused',lambda x:ctr(x,'template').update(image='ghcr.io/bex-co/bex-operator@sha256:'+'f'*64))
add('GitOps_not_synced_still_refused',lambda x:x['gitops']['status']['sync'].update(status='OutOfSync'))
add('GitOps_wrong_image_still_refused',lambda x:x['gitops']['spec']['source']['kustomize'].update(images=['controller=ghcr.io/bex-co/bex-operator@sha256:'+'f'*64]))
add('zero_replicas_still_refused',lambda x:x['deployment']['spec'].update(replicas=0))
def two_current(x):
 p=copy.deepcopy(x['pods']['items'][0]);p['metadata'].update(name='manager-second',uid='pod-second');p['status']['conditions'][0]['lastTransitionTime']='2026-10-31T03:05:00Z';x['pods']['items'].append(p)
add('SINCE_latest_actual_current_Pod_Ready_not_Available',two_current,True,True,'2026-10-31T03:05:00Z')
def deleting_old(x):
 p=copy.deepcopy(x['pods']['items'][0]);p['metadata'].update(name='manager-deleting-old',uid='pod-old',deletionTimestamp='2026-10-31T03:10:00Z');p['spec']['containers'][0].update(env=[]);p['status']['conditions'][0].update(status='False');x['pods']['items'].append(p)
add('deleting_old_Pod_not_current_binding_gate',deleting_old,True,True,'2026-10-31T03:00:00Z')
add('authenticated_root_hash_mismatch_rejected',rootbytes=root+b' ')
add('authenticated_child_hash_size_mismatch_rejected',childbytes=child+b' ')
original_run=subprocess.run;results=[];O=D/'SYNTHETIC-control-results';O.mkdir()
def execute(g,version,case):
 directory=O/(case['name']+'-'+version);directory.mkdir();(directory/'root.raw.json').write_bytes(case['rootbytes']);(directory/'child.raw.json').write_bytes(case['childbytes']);out=directory/'ready.json';calls=[]
 def fake_run(argv,*args,**kwargs):
  assert argv[0]=='kubectl' and 'get' in argv and all(t not in argv for t in ['exec','apply','patch','delete','create']);kind=argv[argv.index('get')+1];data={'deployment':case['data']['deployment'],'replicasets':case['data']['replicasets'],'pods':case['data']['pods'],'applications.argoproj.io':case['data']['gitops']}[kind];calls.append('FIXTURE-get:'+kind);return subprocess.CompletedProcess(argv,0,json.dumps(data),'')
 argv=sys.argv;subprocess.run=fake_run;sys.argv=[str(g.__file__),'--kubeconfig','/SYNTHETIC/NEVER-READ.kubeconfig','--expected-image',image,'--root-manifest',str(directory/'root.raw.json'),'--amd64-manifest',str(directory/'child.raw.json'),'--output',str(out)];stdout=io.StringIO();error=None
 try:
  with contextlib.redirect_stdout(stdout):g.main()
 except AssertionError as exc:error=type(exc).__name__
 finally:sys.argv=argv;subprocess.run=original_run
 text=stdout.getvalue()+(out.read_text() if out.exists() else '');assert all(s not in text for s in [SECRET,REFNAME,REFKEY,'UNRECOGNIZED_SECRET_SHAPED_KEY']),case['name'];d=json.loads(out.read_bytes()) if out.exists() else None
 if d and not d['rollout_ready']:assert d['rollout_since_latest_expected_image_Pod_Ready'] is None
 return d,{'file':str(out) if out.exists() else None,'sha256':H(out) if out.exists() else None,'exception':error,'mocked_calls':calls,'no_env_or_Secret_value_disclosure':True}
for c in cases:
 a,ap=execute(old,'old',c);b,bp=execute(new,'v2',c)
 assert bool(a and a['rollout_ready'])==c['oldready'],c['name'];assert bool(b and b['rollout_ready'])==c['newready'],c['name']
 if c['newready']:assert b['rollout_since_latest_expected_image_Pod_Ready']==c['since']
 results.append({'case':c['name'],'passed':True,'old_ready':bool(a and a['rollout_ready']),'new_ready':bool(b and b['rollout_ready']),'new_since':b['rollout_since_latest_expected_image_Pod_Ready'] if b else None,'old':ap,'new':bp})
for p,v in protected.items():assert H(p)==v,p
report={'schema':'bex.m188.INACTIVE-ready-selfimage-v2-controls/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'synthetic_kubectl_fixtures_only':True,'actual_production_calls':0,'actual_old_and_new_main_executed':True,'signature_or_deployed_source_acceptance_not_tested':True,'old_helper':{'file':str(OLD),'sha256':H(OLD)},'inactive_proposal':{'file':str(NEW),'sha256':H(NEW)},'harness':{'file':__file__,'sha256':H(__file__)},'all_controls_passed':True,'controls_count':len(results),'old_false_positive_new_rejections':sum(r['old_ready'] and not r['new_ready'] for r in results),'results':results,'all_failures_have_no_accepted_since':True,'source_kind_count_allowlisted_fieldPath_only_no_values_or_secret_names':True,'all_old_manifest_owner_image_ready_gitops_since_gates_preserved':True,'deleting_old_Pods_remain_excluded_from_currentReady_gates':True,'preserved_hashes':protected,'not_activated':True,'limits':'Tiny unsignedsyntheticrawmanifest chain exercises hash/size/Readytool behavior only. Fulloperator signature/source/adoption closure remains separate mandatoryplanprerequisite and actualdeployedsource contract must beconfirmed. No realkubeconfig/Secret/Appenv reads or production calls.'};p=D/'isolated-controls.json';p.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'controls':len(results),'old_false_positives_new_rejected':report['old_false_positive_new_rejections'],'report':str(p),'sha256':H(p),'harness_sha256':H(__file__),'all_passed':True}))
