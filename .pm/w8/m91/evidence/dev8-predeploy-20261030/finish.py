#!/usr/bin/env python3
from pathlib import Path
import copy,hashlib,json,subprocess,sys,time,urllib.request
P=Path(__file__).resolve().parent;sys.path.insert(0,str(P));import run as h
h.guard();m=h.M
# Capture observed auth-denied state before its bounded cancellation.
j=json.loads((P/'negative-job.json').read_text());name=j['metadata']['name']
a=json.loads(h.cmd(h.K+['-n','dev-8','get','job',name,'-o','json']));pods=json.loads(h.cmd(h.K+['-n','dev-8','get','pods','-l','job-name='+name,'-o','json']))
assert len(pods['items'])==1
p=pods['items'][0];assert p['metadata']['ownerReferences'][0]['uid']==a['metadata']['uid']
s=p['status']['containerStatuses'][0];assert not s['started'] and s['restartCount']==0 and s['imageID']==''
assert s['state']['waiting']['reason'] in ('ErrImagePull','ImagePullBackOff') and 'no basic auth credentials' in s['state']['waiting']['message']
assert not p['spec'].get('imagePullSecrets') and p['spec']['containers'][0]['imagePullPolicy']=='Always'
h.save('negative-job-current.json',a);h.save('negative-pods-current.json',pods)
h.save('negative-events.json',json.loads(h.cmd(h.K+['-n','dev-8','get','events','--field-selector','involvedObject.uid='+p['metadata']['uid'],'-o','json'])))
h.save('negative-result.json',{'pass':True,'job_uid':a['metadata']['uid'],'pod_uid':p['metadata']['uid'],'same_image_manifest':m['image_manifest'],'previous_success_cached_image':True,'omitted_pull_secret':True,'PullAlways':True,'observed_reason':s['state']['waiting']['reason'],'observed_error':s['state']['waiting']['message'],'container_never_started':True,'restart_count':0,'scope':'bounded authentication-denial observation; canceled by UID, not a terminal Job-failure/deadline proof'})
# Exact expected API defaults; reject any template/security relaxation.
jobdiff=json.loads((P/'job-template-admission-differences.json').read_text())
allowed={'/metadata/labels/batch.kubernetes.io/controller-uid','/metadata/labels/batch.kubernetes.io/job-name','/metadata/labels/controller-uid','/metadata/labels/job-name','/spec/containers/0/terminationMessagePath','/spec/containers/0/terminationMessagePolicy','/spec/dnsPolicy','/spec/schedulerName','/spec/terminationGracePeriodSeconds','/spec/volumes/0/projected/defaultMode'}
assert {x['path'] for x in jobdiff}==allowed and all(x['requested'] is None for x in jobdiff)
poddiff=json.loads((P/'pod-spec-admission-differences.json').read_text());assert {x['path'] for x in poddiff}=={'/nodeName','/preemptionPolicy','/priority','/serviceAccount','/serviceAccountName','/tolerations'} and all(x['requested'] is None for x in poddiff)
# Retain public image config, controller source identities and exact proxy lifetime.
url='http://localhost:32828/v2/native-node24-extended-qa/blobs/'+m['image_config']
with urllib.request.urlopen(url,timeout=10) as r:config=r.read()
assert 'sha256:'+hashlib.sha256(config).hexdigest()==m['image_config'];(P/'source-image-config.json').write_bytes(config)
paths=['lego/operator/internal/controller/app_controller.go','lego/operator/internal/controller/shell_command.go','lego/operator/internal/controller/deployment_projection.go','lego/operator/internal/controller/container_policy.go','lego/operator/internal/predeploy/predeploy.go','lego/operator/internal/execution/security.go','lego/operator/internal/controller/predeploy_pull_secret_test.go']
repo=Path('/Users/tianpan/projects/bex7');h.save('controller-source-identities.json',{x:{'bytes':len((repo/x).read_bytes()),'sha256':hashlib.sha256((repo/x).read_bytes()).hexdigest()} for x in paths})
proxy=json.loads(h.cmd(['docker','inspect',m['proxy_container_id']]))[0]
assert proxy['Id']==m['proxy_container_id'] and proxy['Name']=='/'+m['proxy_name'] and proxy['Config']['Labels']['bex.qa.owner']==h.N
assert proxy['Image']==m['proxy_image_config'] and proxy['HostConfig']['NetworkMode']=='container:'+m['tenant_node_docker_id']
assert all(x['Type']=='volume' for x in proxy.get('Mounts',[])), 'no bind host mounts'
volumes=[x['Name'] for x in proxy.get('Mounts',[]) if x['Type']=='volume']
h.save('proxy-receipt.json',{'container_id':proxy['Id'],'name':proxy['Name'],'image_config':proxy['Image'],'image_manifest':m['proxy_image_manifest'],'network_mode':proxy['HostConfig']['NetworkMode'],'loopback_port':m['proxy_port'],'no_host_bind_mounts':True,'own_anonymous_volumes':volumes,'started_at':proxy['State']['StartedAt'],'registry_manifest_matches_source':True,'synthetic_basic_auth_only':True})
log=subprocess.run(['docker','logs',m['proxy_container_id']],capture_output=True);(P/'proxy.log').write_bytes(log.stdout+log.stderr)
# UID-precondition foreground deletes; never delete namespace, node, shared service,
# existing SA/ConfigMap, or images/caches owned by somebody else.
api={'Job':'/apis/batch/v1/namespaces/dev-8/jobs/','Secret':'/api/v1/namespaces/dev-8/secrets/','NetworkPolicy':'/apis/networking.k8s.io/v1/namespaces/dev-8/networkpolicies/','ConfigMap':'/api/v1/namespaces/dev-8/configmaps/'}
cleanup=[];records=h.ledger()
ordered=[x for x in records if x['kind']=='Job']+[x for x in records if x['kind'] not in ('Job','ConfigMap')]+[x for x in records if x['kind']=='ConfigMap']
for entry in ordered:
 assert entry['uid'] and entry['namespace']=='dev-8'
 opts={'apiVersion':'v1','kind':'DeleteOptions','preconditions':{'uid':entry['uid']},'propagationPolicy':'Foreground'}
 f=P/('delete-'+entry['name']+'.json');h.save(f.name,opts)
 r=subprocess.run(h.K+['delete','--raw',api[entry['kind']]+entry['name'],'-f',str(f)],capture_output=True,timeout=20)
 if r.returncode:
  assert b'NotFound' in r.stderr, r.stderr.decode(errors='replace')
  state='already-absent'
 else:
  response=json.loads(r.stdout);assert response.get('metadata',{}).get('uid',entry['uid'])==entry['uid']
  h.save('delete-response-'+entry['name']+'.json',response);state='UID-precondition-delete-accepted'
 r=subprocess.run(h.K+['-n','dev-8','wait','--for=delete',entry['kind'].lower()+'/'+entry['name'],'--timeout=30s'],capture_output=True,timeout=35)
 assert r.returncode==0,(entry,r.stderr.decode())
 if entry['kind']=='Job':
  remaining=json.loads(h.cmd(h.K+['-n','dev-8','get','pods','-l','job-name='+entry['name'],'-o','json']));assert not remaining['items']
 cleanup.append(dict(entry,cleanup=state,verified_absent_at=h.now().isoformat()))
 h.save('cleanup-progress.json',cleanup)
# Remove only the reviewed owned Docker ID and its own anonymous volumes.
h.cmd(['docker','rm','-f','-v',m['proxy_container_id']])
assert subprocess.run(['docker','inspect',m['proxy_container_id']],capture_output=True).returncode!=0
for volume in volumes:assert subprocess.run(['docker','volume','inspect',volume],capture_output=True).returncode!=0
# Synthetic transient auth material is deliberately excluded from evidence.
for f in (P/'private').iterdir():f.unlink()
(P/'private').rmdir()
final=json.loads(h.cmd(h.K+['-n','dev-8','get','apps.app.bex.co,jobs,pods,secrets,configmaps,networkpolicies,resourcequotas,limitranges','-o','json']))
remaining=[{'kind':x['kind'],'name':x['metadata']['name'],'uid':x['metadata']['uid']} for x in final['items']]
assert remaining==m['baseline'];h.guard()
h.save('cleanup-result.json',{'pass':True,'owned_kubernetes_resources':cleanup,'proxy_exact_id_removed':m['proxy_container_id'],'own_proxy_volumes_removed':volumes,'synthetic_credentials_removed':True,'baseline_remaining':remaining,'namespace_uid_unchanged':m['namespace_uid'],'default_sa_uid_unchanged':m['default_serviceaccount_uid'],'shared_zot_containerd_operator_monitoring_not_modified':True,'runtime_image_cache_note':'Kubelet-pulled image may remain in shared local containerd content cache; no shared cache/prune mutation performed','completed_at':h.now().isoformat()})
print('negative authenticated-pull boundary proved; all owned API resources/proxy/auth files cleaned; baseline exact UIDs preserved')
