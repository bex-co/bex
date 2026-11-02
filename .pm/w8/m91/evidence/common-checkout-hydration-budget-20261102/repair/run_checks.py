"""Bounded existing host controls for the private D3 overlay; no runtime resources."""
from pathlib import Path
import datetime,hashlib,importlib.util,json,os,shutil,signal,stat,subprocess,time
R=Path(__file__).resolve().parent;V=R/'view';T=R/'tmp';REPO=Path('/Users/tianpan/projects/bex7')
FLOOR=(8<<30)+(128<<20);CAP=128<<20;WHOLE=600
P=Path('/opt/homebrew/bin/python3.14')
DEP=Path('/private/tmp/bex-w8-node14-local-pilot-v4-source-20261101-bop3sbht/run_pilot.py')
def sha(p):return hashlib.sha256(Path(p).read_bytes()).hexdigest()
def dump(name,value):(R/name).write_text(json.dumps(value,indent=2)+'\n')
def size(path):
 total=0
 def missing(error):
  if not isinstance(error,FileNotFoundError):raise error
 for base,dirs,files in os.walk(path,onerror=missing,followlinks=False):
  for name in files:
   try:s=(Path(base)/name).lstat()
   except FileNotFoundError:continue
   if stat.S_ISREG(s.st_mode):total+=s.st_size
 return total
assert sha(DEP)=='e158acde2c44aac290d09f109ddf4f3bf2ca93dc660161dd326fb3ad7cc72c03'
spec=importlib.util.spec_from_file_location('reviewed_group_cleanup',DEP);cleanup=importlib.util.module_from_spec(spec);spec.loader.exec_module(cleanup)
expected=json.loads((R/'VIEW-MANIFEST.json').read_text())
def view_matches():
 actual={str(p.relative_to(V)) for p in V.rglob('*') if p.is_file()}
 return actual==set(expected) and all(sha(V/rel)==row['sha256'] and (V/rel).stat().st_size==row['bytes'] and oct((V/rel).stat().st_mode&0o777)==row['mode'] for rel,row in expected.items())
assert view_matches();assert shutil.disk_usage(R).free>=FLOOR+CAP+(32<<20)
T.mkdir(mode=0o700);(R/'home').mkdir(mode=0o700)
env=os.environ.copy()
for key in list(env):
 if key.startswith(('BEX_','GITHUB_','RUNNER_')) or key in ('CI','KUBECONFIG','DOCKER_CONTEXT','DOCKER_HOST','PYTHONPATH','PYTHONOPTIMIZE','WORKFLOWS_DIR','PRODUCTION_SECRETS_FILE'):env.pop(key,None)
env.update(TMPDIR=str(T),HOME=str(R/'home'),PYTHONDONTWRITEBYTECODE='1',PYTHONPYCACHEPREFIX=str(T/'unused-pyc'),PATH='/Users/tianpan/.nvm/versions/node/v24.16.0/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin')
policy='(version 1) (allow default) (deny network-outbound) (allow network-outbound (remote ip "localhost:*"))'
commands=[('checkout-controls',[str(P),'-B','-m','unittest','discover','-s','.github/scripts','-p','test_native_oci_common_qualification.py','-k','CandidateCheckoutControls'],60),('workflow-guard',[str(P),'-B','-m','unittest','discover','-s','.github/scripts','-p','test_native_oci_common_qualification.py','-k','test_workflow_preserves_complete_amd64_job_before_pilot'],60),('actionlint-configured',['/opt/homebrew/bin/actionlint','-shellcheck=','-pyflakes=','-config-file',str(R/'actionlint.yaml'),'.github/workflows/native-common-image.yml'],60),('actions-guard',['/bin/bash','scripts/github-actions-validate.sh'],60)]
source={str(p.relative_to(R/'source')):sha(p) for p in sorted((R/'source').rglob('*')) if p.is_file()}
before={'foreign_go_work_sum':sha(REPO/'lego/go.work.sum'),'repository_preimages':{rel:sha(REPO/rel) for rel in json.loads((R/'PREIMAGE-PATHS.json').read_text())},'private_source':source,'view_manifest':sha(R/'VIEW-MANIFEST.json')}
dump('CHECK-SOURCE-BEFORE.json',before);dump('SCOPE.json',{'commands':commands,'cwd':str(V),'network':'Sandbox denies non-loopback outbound connections; existing loopback HTTP fixture allowed. No Go/Docker/registry/cluster/integration execution.','sandbox_policy':policy,'physical_floor_plus_margin':FLOOR,'owned_TMP_cap':CAP,'whole_seconds':WHOLE,'source':before,'shared_caches':'none created/cloned/deleted; existing tools read only','resources':'operating disk/deadline bounds, no host capacity reservation'})
start=time.monotonic();minimum=shutil.disk_usage(R).free;peak=0;groups={};handlers={};signals=[];results=[];failure=None
try:
 for sig in (signal.SIGINT,signal.SIGTERM):handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:signals.append(n))
 for label,argv,seconds in commands:
  if signals or groups or not view_matches():raise RuntimeError('cancelled, pending group or changed source')
  if time.monotonic()-start>WHOLE or shutil.disk_usage(R).free<FLOOR or size(T)>CAP:raise RuntimeError('pre-spawn operating guard')
  began=time.monotonic();bound=None;process=None;absent=False
  actual=['/usr/bin/sandbox-exec','-p',policy,*argv]
  with (R/(label+'.stdout')).open('wb') as out,(R/(label+'.stderr')).open('wb') as err:
   try:
    process=subprocess.Popen(actual,cwd=V,env=env,stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True);groups[process.pid]=process
    dump(label+'-PROCESS.json',{'pid':process.pid,'pgid':process.pid,'argv':actual,'cwd':str(V)})
    while process.poll() is None:
     free=shutil.disk_usage(R).free;owned=size(T);minimum=min(minimum,free);peak=max(peak,owned)
     if signals or free<FLOOR or owned>CAP or time.monotonic()-began>seconds or time.monotonic()-start>WHOLE:
      bound={'free':free,'owned_TMP':owned,'seconds':time.monotonic()-began,'signals':signals};break
     time.sleep(.15)
   except BaseException as error:bound={'observer_error':type(error).__name__,'message':str(error)}
   finally:
    if process is not None:
     absent=cleanup.stop_group(process)
     if absent:groups.pop(process.pid,None)
  free=shutil.disk_usage(R).free;owned=size(T);minimum=min(minimum,free);peak=max(peak,owned)
  if signals or free<FLOOR or owned>CAP or time.monotonic()-start>WHOLE:bound=bound or {'late_guard':True,'signals':signals,'free':free,'owned_TMP':owned}
  result={'label':label,'argv':actual,'exit':process.returncode if process else None,'bound':bound,'seconds':time.monotonic()-began,'group_absent':absent,'stdout_sha256':sha(R/(label+'.stdout')),'stderr_sha256':sha(R/(label+'.stderr'))};results.append(result);dump(label+'-RESULT.json',result);print(json.dumps(result),flush=True)
  if process is None or process.returncode or bound or not absent:break
except BaseException as error:failure=type(error).__name__+': '+str(error)
finally:
 for pgid,process in list(groups.items()):
  if cleanup.stop_group(process):groups.pop(pgid,None)
 for sig,handler in handlers.items():signal.signal(sig,handler)
 after={'foreign_go_work_sum':sha(REPO/'lego/go.work.sum'),'repository_preimages':{rel:sha(REPO/rel) for rel in before['repository_preimages']},'private_source':{str(p.relative_to(R/'source')):sha(p) for p in sorted((R/'source').rglob('*')) if p.is_file()},'view_manifest':sha(R/'VIEW-MANIFEST.json')}
 dump('CHECK-SOURCE-AFTER.json',after)
 if not groups:shutil.rmtree(T);shutil.rmtree(R/'home')
 report={'PASS':len(results)==len(commands) and all(r['exit']==0 and r['bound'] is None and r['group_absent'] for r in results) and before==after and view_matches() and not groups and not signals and failure is None,'results':results,'failure':failure,'source_unchanged':before==after,'view_unchanged':view_matches(),'minimum_free_bytes':minimum,'peak_TMP_bytes':peak,'own_TMP_gone':not T.exists(),'pending_pgids':sorted(groups),'signals':signals,'elapsed_seconds':time.monotonic()-start,'actual_runtime_or_adoption_claim':False}
 dump('RESULT.json',report)
print(json.dumps(report),flush=True)
raise SystemExit(0 if report['PASS'] else 1)
