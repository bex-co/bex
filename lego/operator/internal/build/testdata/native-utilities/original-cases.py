"""Unchanged original program/data/validator replay, scoped by actual primary."""
from pathlib import Path
import hashlib,http.client,json,os,re,signal,socket,subprocess,sys,time
ROOT=Path(__file__).resolve().parent
PHASE=sys.argv[1]
MODE=sys.argv[2]
if MODE=='python':
 assert sys.executable.startswith('/usr/local/') and sys.version.startswith('3.13.'),(sys.executable,sys.version)
else:
 assert MODE=='node' and sys.executable.startswith('/usr/bin/'),(MODE,sys.executable)
SPECS=[x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['runtime']==('python' if MODE=='python' else 'node')]
assert len(SPECS)==(6 if MODE=='python' else 1)
OUTPUT=ROOT/'qa-output';OUTPUT.mkdir(exist_ok=True)
ENV=os.environ.copy();ENV['PIP_NO_INDEX']='1';ENV['PIP_DISABLE_PIP_VERSION_CHECK']='1'
def sha(b):return hashlib.sha256(b).hexdigest()

def validation(x):
 source=(ROOT/'cases'/('m'+str(x['mid']))/'validators.py').read_text()
 scope={'reference':x['validation_reference'],'hashlib':hashlib,'json':json,'expected_module_sha':x.get('expected_module_sha')}
 exec(source,scope);return scope

def original_cases():
 results=[]
 for x in SPECS:
  mid=x['mid'];work=OUTPUT/('m'+str(mid));env=ENV.copy()
  if PHASE=='build':
   work.mkdir(exist_ok=True)
   for name,identity in x['primary_reference']['files'].items():
    b=(ROOT/'source-carrier'/name).read_bytes();assert {'bytes':len(b),'sha256':sha(b)}==identity;(work/name).write_bytes(b)
   command=subprocess.run(['/bin/bash','-c',x['commands']['initial']['build']],cwd=work,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=90)
   if command.returncode:raise RuntimeError((command.args,command.returncode,command.stdout.decode(errors='replace')[-1500:],command.stderr.decode(errors='replace')[-1500:]))
   stdout=command.stdout
   (OUTPUT/('build-'+str(mid)+'.log')).write_bytes(stdout)
   v=validation(x);proofs=[]
   for match in re.finditer(r'QA[A-Za-z0-9_]* (\{[^\n]+\})',stdout.decode()):
    try:proofs.append(json.loads(match[1]))
    except ValueError:pass
   assert any(v['valid_after'](q,x) for q in proofs),(mid,'authored build/source/data proof missing')
   results.append({'mid':mid,'original_authored_build':True,'source_and_data_proof':True})
  else:
   assert work.is_dir();vs=validation(x)
   with socket.socket() as port_socket:port_socket.bind(('127.0.0.1',0));port=port_socket.getsockname()[1]
   env['PORT']=str(port);log=(OUTPUT/('server-'+str(mid)+'-'+PHASE+'.log')).open('wb')
   process=subprocess.Popen(['/bin/bash','-c',x['commands']['initial']['start']],cwd=work,env=env,stdout=log,stderr=log,start_new_session=True)
   try:
    for _ in range(150):
     assert process.poll() is None,(mid,'authored server exited')
     try:
      with socket.create_connection(('127.0.0.1',port),timeout=.1):break
     except OSError:time.sleep(.03)
    else:raise RuntimeError((mid,'authored server did not listen'))
    current=dict(x,create_view={'serviceDetails':{'url':'http://127.0.0.1:'+str(port)}});fixture=ROOT/'cases'/('m'+str(mid))
    def record(i,v):pass
    if mid==94:
     responses=[]
     for i,path in enumerate(['/rows','/','/rows']):
      path+='?qa409='+x['nonce']+'-'+str(i+1);c=http.client.HTTPConnection('127.0.0.1',port,timeout=5);c.request('GET',path);r=c.getresponse();b=r.read();q=json.loads(b);assert r.status==200 and vs['valid_after'](q['build'],x) and vs['valid_after'](q['runtime'],x) and q['build']['database']==q['runtime']['database'];responses.append({'path':path,'pass':True});c.close()
    else:
     scope={};exec((fixture/'http.py').read_text(),scope);fn=scope['exercise'];valid=vs['valid_after'];ref=x['validation_reference']
     if mid in (95,96):responses=fn(current,[],fixture,valid,ref)
     elif mid==103:responses=fn(current,fixture,ref,valid,record)
     else:responses=fn(current,[],fixture,valid,ref,record)
    assert len(responses)==3 and all(q['pass'] for q in responses),(mid,'independent HTTP proof failed')
    (OUTPUT/('runtime-'+str(mid)+'-'+PHASE+'.json')).write_text(json.dumps(responses,ensure_ascii=False))
    results.append({'mid':mid,'original_authored_runtime_HTTP_proofs':len(responses),'independent_source_and_data_proofs':True})
   finally:
    if process.poll() is None:os.killpg(process.pid,signal.SIGTERM)
    process.wait(timeout=5);log.close()
 return results
print(json.dumps({'phase':PHASE,'mode':MODE,'python':sys.version.split()[0],'original_cases':original_cases(),'pass':True},ensure_ascii=False))
