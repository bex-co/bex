from pathlib import Path
from urllib.parse import urlencode,urlparse
import base64,http.client,json,os,secrets,shutil,socket,subprocess,time
OUT=Path(__file__).resolve().parent
state={'docker_endpoint':'unix:///Users/tianpan/.orbstack/run/docker.sock'};meta=json.loads((OUT/'raw-metadata-review.json').read_text());crypto=json.loads((OUT/'crypto-review.json').read_text())
assert crypto['signature_verified'] and crypto['spdx_attestation_verified'] and crypto['source']==meta['source']=='dd64e4ff7888828faa1b547f3a637b76d7e7da91' and crypto['reference']==meta['candidate_ref']
MIN_FREE=2*1024**3
layers={}
for arch in ('amd64','arm64'):
 manifest=json.loads((OUT/('candidate-'+arch+'-manifest.json')).read_text())
 for layer in manifest['layers']:layers[layer['digest']]=layer['size']
compressed=sum(layers.values());assert compressed<512*1024**2,'unexpectedly large CSI runtime payload'
required=MIN_FREE+max(512*1024**2,compressed*4)
assert shutil.disk_usage(OUT).free>=required,'insufficient free space for bounded CSI probes'
endpoint=urlparse(state['docker_endpoint']);assert endpoint.scheme=='unix'
class UnixHTTP(http.client.HTTPConnection):
 def __init__(self):super().__init__('localhost',timeout=180)
 def connect(self):
  self.sock=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM);self.sock.settimeout(self.timeout);self.sock.connect(endpoint.path)
def request(method,path,headers=None):
 conn=UnixHTTP();conn.request(method,path,headers=headers or {});return conn,conn.getresponse()
conn,response=request('GET','/version');assert response.status==200;version=json.loads(response.read())['ApiVersion'];conn.close()
config=OUT/'anonymous-docker-config';config.mkdir(mode=0o700,exist_ok=False);(config/'config.json').write_text('{"auths":{}}\n')
env=dict(os.environ,DOCKER_CONFIG=str(config),DOCKER_HOST=state['docker_endpoint']);env.pop('DOCKER_AUTH_CONFIG',None);env.pop('REGISTRY_AUTH_FILE',None);env.pop('DOCKER_CONTEXT',None)
docker=shutil.which('docker');assert docker
checks={};minimum=shutil.disk_usage(OUT).free;nonce=secrets.token_hex(8)
for arch in ('amd64','arm64'):
 assert shutil.disk_usage(OUT).free>=required,'free space dropped before anonymous pull'
 ref='ghcr.io/bex-co/bex-operator@'+meta['candidate']['platforms'][arch]['manifest']
 inspect=subprocess.run([docker,'image','inspect',ref],env=env,capture_output=True,text=True);preexisting=inspect.returncode==0
 statuses=[];started=time.monotonic()
 # The Engine API receives an explicit empty AuthConfig. No host credential
 # helper, registry login or Docker client keychain participates in this pull.
 conn,response=request('POST','/v'+version+'/images/create?'+urlencode({'fromImage':ref,'platform':'linux/'+arch}),{'X-Registry-Auth':base64.b64encode(b'{}').decode()})
 try:
  assert response.status==200,'anonymous Docker pull HTTP '+str(response.status)
  while True:
   line=response.readline()
   if not line:break
   row=json.loads(line);statuses.append(row)
   assert not row.get('error') and not row.get('errorDetail'),row.get('error') or row.get('errorDetail')
   minimum=min(minimum,shutil.disk_usage(OUT).free);assert minimum>=MIN_FREE,'global free-space reserve reached'
 finally:conn.close()
 (OUT/('anonymous-pull-'+arch+'.jsonl')).write_text('\n'.join(json.dumps(r) for r in statuses)+'\n')
 results=[]
 try:
  actual=json.loads(subprocess.run([docker,'image','inspect',ref],env=env,capture_output=True,text=True,check=True).stdout)[0]
  expected=meta['candidate']['platforms'][arch]
  assert actual['Id']==expected['config'] and actual['Architecture']==arch and ref in actual['RepoDigests'] and actual['RootFS']['Layers']==expected['diff_ids'],'actual pulled image binding differs'
  (OUT/('actual-image-'+arch+'.json')).write_text(json.dumps(actual,indent=2)+'\n')
  for name,argv in [('driver-help',['/bin/hcloud-csi-driver','--help']),('cryptsetup-version',['cryptsetup','--version']),('blkid-help',['blkid','--help']),('dumpe2fs-version',['dumpe2fs','-V'])]:
   label='bex-m144-dd64-'+nonce;owner=label+'-'+arch+'-'+name
   args=[docker,'create','--name',owner,'--label','bex.qa.owner='+label,'--pull=never','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--pids-limit','32','--memory','64m','--platform','linux/'+arch,'--entrypoint',argv[0],expected['config'],*argv[1:]]
   previous=subprocess.run([docker,'inspect',owner],env=env,capture_output=True,text=True,timeout=30);assert previous.returncode!=0 and ('no such object' in previous.stderr.lower() or 'no such container' in previous.stderr.lower()),'unique probe name already exists or identity unknown'
   (OUT/('owned-container-intent-'+arch+'-'+name+'.json')).write_text(json.dumps({'name':owner,'label':label,'image':expected['config'],'command':args,'preexisting':False},indent=2)+'\n')
   cid=None
   try:
    created=subprocess.run(args,text=True,capture_output=True,env=env,timeout=30);assert created.returncode==0,created.stderr[-600:];cid=created.stdout.strip();assert len(cid)==64 and all(c in '0123456789abcdef' for c in cid)
    (OUT/('owned-container-'+arch+'-'+name+'.json')).write_text(json.dumps({'id':cid,'name':owner,'label':label,'image':expected['config'],'command':args},indent=2)+'\n')
    result=subprocess.run([docker,'start','--attach',cid],text=True,capture_output=True,env=env,timeout=90);text=result.stdout+result.stderr
    (OUT/('anonymous-'+arch+'-'+name+'.txt')).write_text(text)
    final=json.loads(subprocess.run([docker,'inspect',cid],text=True,capture_output=True,env=env,check=True).stdout)[0]
    assert final['Config']['Labels']['bex.qa.owner']==label and final['Image']==expected['config']
    results.append({'name':name,'argv':argv,'exit_code':result.returncode,'container_exit_code':final['State']['ExitCode'],'output_bytes':len(text.encode()),'owned_container_id':cid})
    assert result.returncode==0 and final['State']['ExitCode']==0,name+' failed on '+arch+': '+text[:600]
    if name=='blkid-help':assert '--probe' in text and '--output' in text and '--match-tag' in text,'blkid lacks fixed inspection flags'
   finally:
    current=subprocess.run([docker,'inspect',cid or owner],env=env,capture_output=True,text=True,timeout=30)
    if current.returncode==0:
     info=json.loads(current.stdout)[0];assert (cid is None or info['Id']==cid) and info['Name']=='/'+owner and info['Config']['Labels']['bex.qa.owner']==label and info['Image']==expected['config'],'refuse foreign cleanup'
     cid=info['Id'];removed=subprocess.run([docker,'rm','-f',cid],text=True,capture_output=True,env=env,timeout=30)
     (OUT/('owned-container-cleanup-'+arch+'-'+name+'.txt')).write_text(removed.stdout+removed.stderr);assert removed.returncode==0,'exact own container cleanup failed'
     absent=subprocess.run([docker,'inspect',cid],env=env,capture_output=True,text=True);assert absent.returncode!=0 and ('no such object' in absent.stderr.lower() or 'no such container' in absent.stderr.lower()),'exact own container absence unproved'
    else:assert 'no such object' in current.stderr.lower() or 'no such container' in current.stderr.lower(),'probe cleanup identity unknown'
   minimum=min(minimum,shutil.disk_usage(OUT).free);assert minimum>=MIN_FREE,'global free-space reserve reached'

 finally:
  cleanup=None
  if not preexisting:
   cleanup=subprocess.run([docker,'image','rm',ref],text=True,capture_output=True,env=env,timeout=30)
   (OUT/('owned-image-cleanup-'+arch+'.txt')).write_text(cleanup.stdout+cleanup.stderr);assert cleanup.returncode==0,'exact newly pulled image cleanup failed'
   absent=subprocess.run([docker,'image','inspect',ref],text=True,capture_output=True,env=env,timeout=30);assert absent.returncode!=0 and 'no such image' in absent.stderr.lower(),'new own image-reference absence unproved'
 checks[arch]={'actual_config_id':expected['config'],'actual_root_diff_ids_match':True,'anonymous_engine_auth_config':{},'reference':ref,'preexisting_local_image':preexisting,'pull_elapsed_seconds':round(time.monotonic()-started,3),'execution_results':results,'owned_image_cleanup_exit':cleanup.returncode if cleanup else None,'free_bytes_after':shutil.disk_usage(OUT).free}
 print(json.dumps({'architecture':arch,'anonymous_pull_and_execution':'PASS','reference':ref,'free_bytes':checks[arch]['free_bytes_after']}),flush=True)
r={'reference':meta['candidate_ref'],'platforms':checks,'compressed_unique_layer_bytes':compressed,'global_free_reserve_bytes':MIN_FREE,'minimum_observed_free_bytes':minimum,'docker_endpoint_kind':'existing local Unix socket','no_provider_or_cluster_access':True,'no_host_mounts_or_network_in_probe_containers':True}
(OUT/'anonymous-probes.json').write_text(json.dumps(r,indent=2)+'\n')
print(json.dumps({'both_architectures_pass':True,'minimum_observed_free_bytes':minimum,'compressed_unique_layer_bytes':compressed},indent=2))
