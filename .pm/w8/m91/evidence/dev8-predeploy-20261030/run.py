#!/usr/bin/env python3
"""One owned dev-8 predeploy proof, with fixed UID/name/resource boundaries."""
import base64,datetime,hashlib,json,os,re,secrets,subprocess,sys,time,urllib.error,urllib.request
from pathlib import Path
P=Path(__file__).resolve().parent
M=json.loads((P/'manifest.json').read_text()); N=M['name']; NS='dev-8'
K=['kubectl','--kubeconfig','/Users/tianpan/projects/bex7/.pm/w8/dev-8/.kubeconfig']
def now():return datetime.datetime.now(datetime.timezone.utc)
def sha(b):return hashlib.sha256(b).hexdigest()
def save(n,x):
 f=P/n;f.write_text(json.dumps(x,indent=2,ensure_ascii=False)+'\n');f.chmod(0o600)
def cmd(a,data=None,timeout=30):
 r=subprocess.run(a,input=data,capture_output=True,timeout=timeout)
 if r.returncode:raise RuntimeError(f'command failed: {a[0]} {a[1:3]}: {r.stderr.decode(errors="replace")[:1000]}')
 return r.stdout

def guard():
 assert now()<datetime.datetime.fromisoformat(M['expires_at'])
 assert N.startswith('qa-'+now().strftime('%Y%m%d')+'-w8-predeploy-')
 assert cmd(K+['get','namespace',NS,'-o','jsonpath={.metadata.uid}']).decode()==M['namespace_uid']
 assert cmd(K+['-n',NS,'get','sa','default','-o','jsonpath={.metadata.uid}']).decode()==M['default_serviceaccount_uid']

def ledger():return json.loads((P/'ledger.json').read_text())
def create(obj):
 guard();kind=obj['kind'];name=obj['metadata']['name'];assert obj['metadata']['namespace']==NS
 assert name.startswith(N) or (kind=='Job' and name.startswith('predeploy-'+N))
 records=ledger();assert not any(x['kind']==kind and x['name']==name for x in records),'no retry/adoption'
 body=json.dumps(obj,separators=(',',':')).encode();entry={'kind':kind,'name':name,'namespace':NS,'created_at':now().isoformat(),'request_sha256':sha(body),'uid':None}
 records.append(entry);save('ledger.json',records)
 r=subprocess.run(K+['create','-f','-','-o','json'],input=body,capture_output=True,timeout=30)
 if r.returncode:
  (P/(kind+'-create-error.txt')).write_bytes(r.stderr)
  raise RuntimeError('create failed; no resource retry: '+kind+' '+name)
 result=json.loads(r.stdout);assert result['metadata']['name']==name and result['metadata']['namespace']==NS
 entry['uid']=result['metadata']['uid'];entry['api_created_at']=result['metadata']['creationTimestamp'];save('ledger.json',records)
 if kind=='Secret':
  save('created-'+name+'.json',{'apiVersion':'v1','kind':'Secret','metadata':result['metadata'],'type':result.get('type'),'data_keys':sorted(result.get('data',{}))})
 else:save('created-'+name+'.json',result)
 return result

def metadata(name,anchor=None):
 m={'name':name,'namespace':NS,'labels':{'bex.co/qa-owner':N}}
 if anchor:m['ownerReferences']=[{'apiVersion':'v1','kind':'ConfigMap','name':N,'uid':anchor,'controller':True}]
 return m

def prepare():
 guard();assert ledger()==[]
 baseline=json.loads(cmd(K+['-n',NS,'get','apps.app.bex.co,jobs,pods,secrets,configmaps,networkpolicies,resourcequotas,limitranges','-o','json']))
 got=[{'kind':x['kind'],'name':x['metadata']['name'],'uid':x['metadata']['uid']} for x in baseline['items']]
 assert got==M['baseline']
 a=create({'apiVersion':'v1','kind':'ConfigMap','metadata':metadata(N),'data':{'purpose':'scratch canonical dev-8 predeploy Job; not an App CR','expires_at':M['expires_at']}})
 uid=a['metadata']['uid'];M['anchor_uid']=uid
 # Registry proxy uses only this owned process/rootfs and one free node-loopback port.
 assert cmd(['docker','inspect',M['tenant_node'],'--format','{{.Id}}']).decode().strip()==M['tenant_node_docker_id']
 ports=cmd(['docker','exec',M['tenant_node'],'ss','-H','-ltn']).decode()
 port=next(x for x in range(43829,43929) if not re.search(r':'+str(x)+r'\s',ports))
 password=secrets.token_urlsafe(32);user='dev8-qa'
 private=P/'private';private.mkdir(mode=0o700)
 ht=cmd(['/usr/sbin/htpasswd','-niB',user],(password+'\n').encode())
 (private/'htpasswd').write_bytes(ht);(private/'htpasswd').chmod(0o600)
 # Avoid credentials in argv, registry env, manifests, logs or returned tool text.
 auth=base64.b64encode((user+':'+password).encode()).decode()
 config={'auths':{'localhost:'+str(port):{'auth':auth}}}
 save('private/dockerconfig.json',config)
 proxyname=N+'-registry';M.update(proxy_name=proxyname,proxy_port=port)
 save('manifest.json',M)
 cid=cmd(['docker','create','--name',proxyname,'--label','bex.qa.owner='+N,'--network','container:'+M['tenant_node_docker_id'],'--memory','512m','--cpus','1','--cap-drop','ALL','--security-opt','no-new-privileges','-e','REGISTRY_HTTP_ADDR=127.0.0.1:'+str(port),'-e','REGISTRY_PROXY_REMOTEURL=http://host.docker.internal:32828','-e','REGISTRY_AUTH=htpasswd','-e','REGISTRY_AUTH_HTPASSWD_REALM='+N,'-e','REGISTRY_AUTH_HTPASSWD_PATH=/tmp/qa-htpasswd','-e','REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=/tmp/qa-registry-cache',M['proxy_image_config']]).decode().strip()
 M['proxy_container_id']=cid;save('manifest.json',M)
 cmd(['docker','cp',str(private/'htpasswd'),cid+':/tmp/qa-htpasswd'])
 cmd(['docker','start',cid]);M['proxy_started_at']=now().isoformat();save('manifest.json',M)
 # Node curl's stdin config carries synthetic credentials without argv/log exposure.
 url='http://localhost:'+str(port)+'/v2/native-node24-extended-qa/manifests/'+M['image_manifest']
 curl='url = "'+url+'"\nuser = "'+user+':'+password+'"\nheader = "Accept: application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json"\n'
 deadline=time.monotonic()+20
 while True:
  try:
   raw=cmd(['docker','exec','-i',M['tenant_node'],'curl','--fail','--silent','--show-error','--max-time','5','--config','-'],curl.encode())
   break
  except RuntimeError:
   if time.monotonic()>deadline:raise
   time.sleep(.5)
 assert 'sha256:'+sha(raw)==M['image_manifest'];manifest=json.loads(raw);assert manifest['config']['digest']==M['image_config']
 save('transport-manifest.json',manifest)
 unauth=cmd(['docker','exec',M['tenant_node'],'curl','--silent','--output','/dev/null','--write-out','%{http_code}','--max-time','5',url]).decode()
 assert unauth=='401';M['unauthenticated_registry_http']=401;M['transport_image']='localhost:'+str(port)+'/native-node24-extended-qa@'+M['image_manifest'];save('manifest.json',M)
 encoded={k:base64.b64encode(v).decode() for k,v in {'QA_SECRET_MARKER':b'own-synthetic-only','QA_OVERRIDE':b'secret-must-lose','PORT':b'9','proof.txt':b'own-dev8-file\x00\xff\n'}.items()}
 create({'apiVersion':'v1','kind':'Secret','metadata':metadata(N+'-env-files',uid),'type':'Opaque','data':encoded})
 create({'apiVersion':'v1','kind':'Secret','metadata':metadata(N+'-pull',uid),'type':'kubernetes.io/dockerconfigjson','data':{'.dockerconfigjson':base64.b64encode(json.dumps(config).encode()).decode()}})
 create({'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':metadata(N+'-deny-egress',uid),'spec':{'podSelector':{'matchLabels':{'app.bex.co/app-uid':uid}},'policyTypes':['Egress'],'egress':[]}})
 save('generator-input.json',{'Name':N,'Namespace':NS,'AnchorUID':uid,'Image':M['transport_image'],'PullSecret':N+'-pull','EnvSecret':N+'-env-files','FileSecret':N+'-env-files'})
 print('prepared owned anchor, synthetic secrets, exact-selector policy and proxy; digest identical')

def generate():
 guard();env=dict(os.environ,GOWORK='off',BEX_DEV8_OUTPUT=str(P))
 with (P/'generator.log').open('wb') as log:
  r=subprocess.run(['go','test','-overlay',str(P/'overlay.json'),'./internal/controller','-run','^TestScratchDev8PredeployTemplate$','-count=1','-v'],cwd='/Users/tianpan/projects/bex7/lego/operator',env=env,stdout=log,stderr=subprocess.STDOUT,timeout=300)
 assert r.returncode==0,'canonical export failed; see generator.log'
 j=json.loads((P/'canonical-job.json').read_text());s=j['spec']['template']['spec'];c=s['containers'][0]
 assert c['image']==M['transport_image'] and c['imagePullPolicy']=='Always' and c['command'][:2]==['/bin/sh','-c']
 assert s['hostUsers'] is False and s['automountServiceAccountToken'] is False and s['nodeSelector']=={'bex.co/pool':'tenant'}
 assert j['spec']['backoffLimit']==0 and j['spec']['activeDeadlineSeconds']==600
 assert c['resources']['requests']==c['resources']['limits'] and c['resources']['limits']['memory']=='512Mi'
 j['metadata'].pop('resourceVersion',None);j['metadata'].pop('creationTimestamp',None)
 j['metadata']['ownerReferences']=metadata(N,M['anchor_uid'])['ownerReferences']
 assert j['spec']['template']==json.loads((P/'canonical-job.json').read_text())['spec']['template']
 save('success-job.json',j);M['canonical_template_sha256']=sha(json.dumps(j['spec']['template'],sort_keys=True,separators=(',',':')).encode());save('manifest.json',M)
 print('canonical controller template exported; metadata-only owner adjustment; starter resources preserved')

def start():
 guard();j=json.loads((P/'success-job.json').read_text());create(j);print('created success Job')

def capture():
 guard();j=json.loads((P/'success-job.json').read_text());name=j['metadata']['name']
 actual=json.loads(cmd(K+['-n',NS,'get','job',name,'-o','json']));save('success-job-current.json',actual)
 pods=json.loads(cmd(K+['-n',NS,'get','pods','-l','job-name='+name,'-o','json']))
 save('success-pods-current.json',pods)
 if len(pods['items'])==1:
  pod=pods['items'][0];assert pod['metadata']['ownerReferences'][0]['uid']==actual['metadata']['uid']
  events=json.loads(cmd(K+['-n',NS,'get','events','--field-selector','involvedObject.uid='+pod['metadata']['uid'],'-o','json']));save('success-events.json',events)
  if pod.get('status',{}).get('containerStatuses'):
   log=subprocess.run(K+['-n',NS,'logs',pod['metadata']['name']],capture_output=True,timeout=20)
   if log.returncode==0:(P/'success-pod.log').write_bytes(log.stdout)
 print(json.dumps({'job':name,'status':actual.get('status',{}),'pods':[{'name':p['metadata']['name'],'phase':p.get('status',{}).get('phase'),'waiting':[s.get('state') for s in p.get('status',{}).get('containerStatuses',[])]} for p in pods['items']]}))


def finish_prepare():
 guard()
 uid=M['anchor_uid'];port=M['proxy_port']
 config=json.loads((P/'private/dockerconfig.json').read_text())
 user,password=base64.b64decode(config['auths']['localhost:'+str(port)]['auth']).decode().split(':',1)
 assert json.loads(cmd(K+['-n',NS,'get','configmap',N,'-o','json']))['metadata']['uid']==uid
 assert cmd(['docker','inspect',M['proxy_container_id'],'--format','{{.Name}}']).decode().strip()=='/'+M['proxy_name']
 # Node curl's stdin config carries synthetic credentials without argv/log exposure.
 url='http://localhost:'+str(port)+'/v2/native-node24-extended-qa/manifests/'+M['image_manifest']
 curl='url = "'+url+'"\nuser = "'+user+':'+password+'"\nheader = "Accept: application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json"\n'
 deadline=time.monotonic()+20
 while True:
  try:
   raw=cmd(['docker','exec','-i',M['tenant_node'],'curl','--fail','--silent','--show-error','--max-time','5','--config','-'],curl.encode())
   break
  except RuntimeError:
   if time.monotonic()>deadline:raise
   time.sleep(.5)
 assert 'sha256:'+sha(raw)==M['image_manifest'];manifest=json.loads(raw);assert manifest['config']['digest']==M['image_config']
 save('transport-manifest.json',manifest)
 unauth=cmd(['docker','exec',M['tenant_node'],'curl','--silent','--output','/dev/null','--write-out','%{http_code}','--max-time','5',url]).decode()
 assert unauth=='401';M['unauthenticated_registry_http']=401;M['transport_image']='localhost:'+str(port)+'/native-node24-extended-qa@'+M['image_manifest'];save('manifest.json',M)
 encoded={k:base64.b64encode(v).decode() for k,v in {'QA_SECRET_MARKER':b'own-synthetic-only','QA_OVERRIDE':b'secret-must-lose','PORT':b'9','proof.txt':b'own-dev8-file\x00\xff\n'}.items()}
 create({'apiVersion':'v1','kind':'Secret','metadata':metadata(N+'-env-files',uid),'type':'Opaque','data':encoded})
 create({'apiVersion':'v1','kind':'Secret','metadata':metadata(N+'-pull',uid),'type':'kubernetes.io/dockerconfigjson','data':{'.dockerconfigjson':base64.b64encode(json.dumps(config).encode()).decode()}})
 create({'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':metadata(N+'-deny-egress',uid),'spec':{'podSelector':{'matchLabels':{'app.bex.co/app-uid':uid}},'policyTypes':['Egress'],'egress':[]}})
 save('generator-input.json',{'Name':N,'Namespace':NS,'AnchorUID':uid,'Image':M['transport_image'],'PullSecret':N+'-pull','EnvSecret':N+'-env-files','FileSecret':N+'-env-files'})
 print('prepared owned anchor, synthetic secrets, exact-selector policy and proxy; digest identical')


if __name__=='__main__':globals()[sys.argv[1]]()
