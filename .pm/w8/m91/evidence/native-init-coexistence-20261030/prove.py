#!/usr/bin/env python3
from pathlib import Path
import hashlib,json,os,shlex,subprocess,time
P=Path(__file__).resolve().parent
M=json.loads((P/'run-context.json').read_text());TAG=M['image_tag'];OWNER='native-init-eba08e70'
def save(n,x): (P/n).write_text(json.dumps(x,indent=2,ensure_ascii=False)+'\n')
def run(a,timeout=30,check=True):
 r=subprocess.run(a,capture_output=True,timeout=timeout)
 if check and r.returncode:raise RuntimeError((a[:3],r.returncode,r.stderr.decode(errors='replace')[-3000:]))
 return r
image=json.loads(run(['docker','image','inspect',TAG]).stdout)[0]
assert image['Config']['Entrypoint']==['/native-init']
assert image['Config']['Cmd']==['/bin/bash','-c','/usr/bin/python3 /opt/render/project/src/init-proof.py']
assert image['Architecture']=='arm64';M['owned_images']=[{'tag':TAG,'config_id':image['Id']}];save('run-context.json',M)
save('generated-image-inspect.json',image)
baseenv=dict(x.split('=',1) for x in image['Config']['Env']);expectedpath=baseenv['PATH']
literal='literal $(NOT_EXPANDED) $HOME; "雪"\ntrailing\n'
common=['--env','QA_LITERAL='+literal,'--env','QA_EMPTY=','--env','QA_EXPECT_PATH='+expectedpath]
receipts=[]
def create(name,args,env=None):
 assert run(['docker','inspect',name],check=False).returncode!=0,'no adoption'
 command=['docker','create','--name',name,'--label','bex.qa.owner='+OWNER,'--network','none','--memory','1g','--cpus','2','--pids-limit','256']+common+(env or [])+[image['Id']]+args
 cid=run(command).stdout.decode().strip();M['owned_containers'].append({'id':cid,'name':name});save('run-context.json',M)
 return cid

def inspect(cid):
 x=json.loads(run(['docker','inspect',cid]).stdout)[0]
 assert x['Id']==cid and x['Image']==image['Id'] and x['Config']['Labels']['bex.qa.owner']==OWNER
 assert x['HostConfig']['NetworkMode']=='none' and x['Config']['Entrypoint']==['/native-init']
 return x

def execute(label,args,expected,env=None):
 cid=create('bex-native-init-'+label+'-eba08e70',args,env)
 try:
  r=run(['docker','start','--attach',cid],timeout=120,check=False)
  (P/(label+'.log')).write_bytes(r.stdout+r.stderr)
  x=inspect(cid);save(label+'-container.json',x)
  assert not x['State']['Running'] and not x['State']['OOMKilled'] and x['State']['ExitCode']==expected,(label,x['State'])
  if label=='default':
   assert x['Config']['Cmd']==image['Config']['Cmd'] and x['Path']=='/native-init' and x['Args']==image['Config']['Cmd']
   assert r.returncode==0
   proofs={}
   for line in (r.stdout+r.stderr).decode().splitlines():
    if line.startswith('BEX_INIT_'):
     k,v=line.split(' ',1);proofs[k]=json.loads(v)
   assert proofs['BEX_INIT_PROCESS']['pid1_exe']=='/native-init' and proofs['BEX_INIT_PROCESS']['native_init_sha256']==M['native_init_binary_sha256']
   assert len(proofs['BEX_INIT_UTILITIES']['original_program_cases'])==13 and len(proofs['BEX_INIT_UTILITIES']['extra_behavior_checks'])==10
   assert sum(map(len,proofs['BEX_INIT_HTTP_PROOFS'].values()))==39 and proofs['BEX_INIT_CHILD_FINISHED']['exit_code']==0
   save('default-runtime-checks.json',proofs)
   run(['docker','cp',cid+':/opt/render/project/src/qa-output',str(P/'qa-output')],timeout=60)
  else:
   q=json.loads(r.stdout);assert q['pid1']=='/native-init' and q['PATH']==('' if label=='empty-path' else '/usr/bin') and q['literal']==literal and q['empty']==''
  receipts.append({'case':label,'id':cid,'image_config_id':x['Image'],'default_ENTRYPOINT':True,'default_CMD':label=='default','network':'none','exit_code':expected,'OOMKilled':False,'pass':True})
 finally:
  inspect(cid);run(['docker','rm','-f',cid]);assert run(['docker','inspect',cid],check=False).returncode!=0

try:
 execute('default',[],0)
 for name,path in [('empty-path',''),('narrow-path','/usr/bin')]:
  code='import json,os; assert os.readlink("/proc/1/exe")=="/native-init"; print(json.dumps({"pid1":os.readlink("/proc/1/exe"),"pid":os.getpid(),"ppid":os.getppid(),"PATH":os.environ["PATH"],"literal":os.environ["QA_LITERAL"],"empty":os.environ["QA_EMPTY"]})); raise SystemExit(37)'
  # Absolute shell matches the canonical command launch, retaining PATH exactly.
  execute(name,['/bin/bash','-c','exec /usr/bin/python3 -c '+shlex.quote(code)],37,['--env','PATH='+path])
 # A short same-image direct child proves signal forwarding to a real handler.
 code='import os,signal,sys,time; assert os.readlink("/proc/1/exe")=="/native-init"; signal.signal(signal.SIGTERM,lambda s,f:(print("BEX_TERM_RECEIVED",flush=True),sys.exit(23))); print("BEX_TERM_READY",flush=True); time.sleep(30)'
 cid=create('bex-native-init-term-eba08e70',['/usr/bin/python3','-c',code])
 try:
  run(['docker','start',cid]);deadline=time.monotonic()+10
  while True:
   logs=run(['docker','logs',cid]).stdout
   if b'BEX_TERM_READY' in logs:break
   assert time.monotonic()<deadline;time.sleep(.1)
  before=inspect(cid);assert before['State']['Running'];save('term-before-container.json',before)
  run(['docker','kill','--signal','TERM',cid]);run(['docker','wait',cid],timeout=10)
  x=inspect(cid);logs=run(['docker','logs',cid]).stdout;(P/'term.log').write_bytes(logs);save('term-container.json',x)
  assert logs.count(b'BEX_TERM_RECEIVED')==1 and x['State']['ExitCode']==23 and not x['State']['OOMKilled']
  receipts.append({'case':'TERM forwarding','id':cid,'image_config_id':x['Image'],'default_ENTRYPOINT':True,'default_CMD':False,'network':'none','one_real_child_signal_handler':True,'exit_code':23,'pass':True})
 finally:
  inspect(cid);run(['docker','rm','-f',cid]);assert run(['docker','inspect',cid],check=False).returncode!=0
 save('process-results.json',receipts)
finally:
 for c in M['owned_containers']:
  if run(['docker','inspect',c['id']],check=False).returncode==0:
   inspect(c['id']);run(['docker','rm','-f',c['id']])
 # Remove only our new tag/config. Existing immutable supplier holders/registry stay.
 current=json.loads(run(['docker','image','inspect',TAG]).stdout)[0];assert current['Id']==image['Id']
 run(['docker','image','rm',TAG])
 save('cleanup-result.json',{'all_owned_containers_absent':all(run(['docker','inspect',c['id']],check=False).returncode!=0 for c in M['owned_containers']),'own_generated_image_tag_absent':run(['docker','image','inspect',TAG],check=False).returncode!=0,'removed_generated_config_id':image['Id'],'no_new_builder_container_or_builder_instance':True,'existing_supplier_registry_proofholders_preserved':True,'shared_build_cache_not_pruned':True})
print('actual native-init PID1/default CMD utility coexistence, empty/narrow PATH, exit37 and TERM->exit23 passed; owned containers/image tag cleaned')
