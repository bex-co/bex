"""Fixed W8/218 Free clone acceptance; no action occurs on import."""
from pathlib import Path
import argparse,fcntl,hashlib,json,os,re,secrets,subprocess,sys,time
import client_primitives as client
from clone_scope import Journal,Relay,view,now

P=Path(__file__).resolve().parent
ROOT=Path('/Users/tianpan/projects/bex7')
REQUIRED_API='9fe393b77814dc5199372419d10dad974e7b27e1'
WORKSPACE='tea-da2isimlm39c739m4ofg'
IMAGE='docker.io/ealen/echo-server:0.9.2'
ROLES=('source','clone-bex','clone-render')
ACTIONS=('auth','source','clone-bex','invalid-bex','cleanup-bex','clone-render','invalid-render','cleanup-render','cleanup-source','reconcile','logout','status')

def sha(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()

def check_ready(root=P):
 # Root supplies this binding only after its actual signed Ready rollout review.
 x=json.loads((Path(root)/'api-ready.json').read_text())
 assert x['requiredAncestor']==REQUIRED_API and x['ready'] is True and x['signedReady'] is True
 assert re.fullmatch('[0-9a-f]{40}',x['source']) and re.fullmatch(r'[^\s]+@sha256:[0-9a-f]{64}',x['image'])
 assert x['readinessProof']['sha256']==sha(x['readinessProof']['path'])
 assert subprocess.run(['git','merge-base','--is-ancestor',REQUIRED_API,x['source']],cwd=ROOT,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0
 return {k:x[k] for k in ('requiredAncestor','source','image','ready','signedReady','readinessProof')}

def check_inputs(root=P):
 expected=json.loads((P/'reuse-manifest.json').read_text())['binarySha256']
 for name,digest in expected.items():assert sha(Path(root)/name)==digest,'public client changed'

def setup_journal(root=P):
 j=Journal(root,WORKSPACE)
 if 'nonce' not in j.data:j.data['nonce']=secrets.token_hex(6);j.save()
 client.configure(root,WORKSPACE,j.data)
 return j

def name(j,role):return 'w8-218-'+j.data['nonce']+'-'+role

def create(j,who,role,backend=None):
 intent=j.start(role,name(j,role),who)
 if role=='source':args=['services','create','--name',intent['name'],'--type','web_service','--runtime','image','--image',IMAGE,'--plan','free','--region','frankfurt','--confirm','-o','json']
 else:args=['services','create','--from',j.source()['id'],'--name',intent['name'],'--confirm','-o','json']
 with Relay(j,'source' if role=='source' else 'clone',intent,backend=backend) as relay:
  result=client.cli(args,who,host=relay.host)
 result.update(at=now(),role=role,acceptanceCommandOmitsRegion=role!='source');j.record(result)
 assert result['exit']==0,'client failed; preserve intent and raw relay receipts'
 resource=j.own(intent['id']);assert resource['state']=='created'
 if role!='source':
  reads=[e for e in relay.events if e['method']=='GET' and e.get('HTTP')==200]
  assert reads and all(e['real_region_readback']=='frankfurt' for e in reads),'real source region differs from accepted public alias'
  assert resource['create_view']['serviceDetails']['region']=='frankfurt'
 return result

def invalid(j,who,backend=None):
 with Relay(j,'invalid',backend=backend) as relay:
  args=['services','create','--from',j.source()['id'],'--name',name(j,'invalid-'+who),'--region','w8-invalid-region','--confirm','-o','json']
  result=client.cli(args,who,host=relay.host)
 result.update(at=now(),criterion='explicit invalid region',forwardedPOSTs=sum(e['method']=='POST' for e in relay.events));j.record(result)
 assert result['exit']!=0 and 'region' in (result['stdout']+result['stderr']).lower() and 'w8-invalid-region' in (result['stdout']+result['stderr']) and result['forwardedPOSTs']==0
 return result

def exact_read(j,resource):
 j.own(resource['id']);r=client.api('GET','/v1/services/'+resource['id']);j.record(r)
 if r['HTTP']=='200':
  v=view(r['response'].encode(),[]);assert v['id']==resource['id'] and v['ownerId']==WORKSPACE and v['name']==resource['name']
 return r

def cleanup(j,role,backend=None):
 resource=next(r for r in j.data['resources'] if r['role']==role and r.get('id'));j.own(resource['id'])
 if resource['state']=='deleted':return {'alreadyVerified404':resource['id']}
 relay=Relay(j,'cleanup',cleanup_id=resource['id'],backend=backend)
 status,_,_=relay.dispatch('DELETE','/v1/services/'+resource['id'],b'',{'Authorization':'Bearer '+client.private_token(),'X-Bex-Workspace':WORKSPACE,'User-Agent':'render-cli/2.29.0'})
 result={'at':now(),'role':role,'cleanupOnly':True,'method':'DELETE','id':resource['id'],'HTTP':status};j.record(result)
 for attempt in range(30):
  observed=exact_read(j,resource)
  if observed['HTTP']=='404':resource.update(state='deleted',absence={'HTTP':404,'at':now(),'id':resource['id']});j.save();return result
  assert observed['HTTP']=='200','cleanup absence unproved; preserve own ID'
  time.sleep(1)
 raise AssertionError('exact owned ID still present; cleanup remains open')

def reconcile(j):
 # Unknown POSTs with no exact 201 identity remain blocked; never list or infer by name.
 for resource in j.data['resources']:
  if not resource['forwarded_once']:resource['state']='not-created';j.save()
  if resource.get('id') and resource['state']!='deleted':exact_read(j,resource)
 return {'unknownIntents':[r['role'] for r in j.data['resources'] if r['state'] not in ('created','deleted','not-created')]}

def main():
 if sys.flags.optimize:raise RuntimeError('assertion guards require ordinary Python without -O')
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('action',choices=ACTIONS);args=parser.parse_args()
 os.umask(0o077);check_inputs()
 with (P/'driver.lock').open('a') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB);j=setup_journal()
  if args.action in ('auth','source','clone-bex','clone-render','invalid-bex','invalid-render'):
   readiness=check_ready();j.data['apiReadyBinding']=readiness;j.save()
   if args.action!='auth':client.private_token()
  if args.action=='auth':
   assert not (P/'credentials/cli.yaml').exists(),'own credential already exists; do not regrant'
   code=subprocess.run([sys.executable,str(P/'auth.py')],cwd=ROOT).returncode;raise SystemExit(code)
  if args.action=='source':out=create(j,'bex','source')
  elif args.action.startswith('clone-'):
   out=create(j,args.action[6:],args.action);readback=exact_read(j,j.own(next(r['id'] for r in j.data['resources'] if r['role']==args.action)));assert readback['HTTP']=='200' and view(readback['response'].encode(),[])['serviceDetails']['region']=='frankfurt'
  elif args.action.startswith('invalid-'):out=invalid(j,args.action[8:])
  elif args.action.startswith('cleanup-'):out=cleanup(j,'source' if args.action=='cleanup-source' else 'clone-'+args.action[8:])
  elif args.action=='reconcile':out=reconcile(j)
  elif args.action=='logout':
   assert all(r['state'] in ('deleted','not-created') for r in j.data['resources']),'finish exact resource cleanup before revoking own auth'
   raise SystemExit(subprocess.run([sys.executable,str(P/'cleanup-auth.py')],cwd=ROOT).returncode)
  else:out={'scope':j.data['scope'],'resources':[{k:r[k] for k in ('role','name','state','id') if k in r} for r in j.data['resources']]}
  print(json.dumps(out,indent=2))

if __name__=='__main__':main()
