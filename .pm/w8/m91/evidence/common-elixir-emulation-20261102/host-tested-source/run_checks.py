"""Existing bounded shared-cache Go-check pattern, scoped to the two-file overlay."""
from pathlib import Path
import datetime,hashlib,importlib.util,json,os,shutil,signal,stat,subprocess,time
R=Path(__file__).resolve().parent;REPO=Path('/Users/tianpan/projects/bex7');TMP=R/'tmp';CACHE=Path('/Users/tianpan/Library/Caches/go-build')
FLOOR=8<<30;MARGIN=128<<20;TMPCAP=512<<20
GO=Path('/opt/homebrew/Cellar/go/1.26.4/libexec/bin/go');LINT=REPO/'lego/operator/bin/golangci-lint'
TMP.mkdir(mode=0o700)
def digest(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()
def dump(name,value):(R/name).write_text(json.dumps(value,indent=2)+'\n')
def size(path):
 total=0
 def ignore_missing(error):
  if not isinstance(error,FileNotFoundError):raise error
 for base,dirs,files in os.walk(path,onerror=ignore_missing,followlinks=False):
  for name in files:
   try:info=(Path(base)/name).lstat()
   except FileNotFoundError:continue
   if stat.S_ISREG(info.st_mode):total+=info.st_size
 return total
# Definition-only imports; both files are hash-bound and main-guarded.
dep=Path('/private/tmp/bex-w8-node14-local-pilot-v4-source-20261101-bop3sbht/run_pilot.py');assert digest(dep)=='e158acde2c44aac290d09f109ddf4f3bf2ca93dc660161dd326fb3ad7cc72c03'
spec=importlib.util.spec_from_file_location('reviewed_cleanup',dep);cleanup=importlib.util.module_from_spec(spec);spec.loader.exec_module(cleanup)
helper_path=REPO/'.github/scripts/native-common-qualify.py';helper_sha=digest(helper_path);spec=importlib.util.spec_from_file_location('source_identity_only',helper_path);helper=importlib.util.module_from_spec(spec);spec.loader.exec_module(helper)
def inventory():
 physical=helper.source_inventory(REPO)
 return {'physical_functional_inventory':physical,'overlay_sources':{name:digest(name) for name in json.loads((R/'overlay.json').read_bytes())['Replace'].values()},'foreign_go_work_sum':digest(REPO/'lego/go.work.sum'),'operator_mod':digest(REPO/'lego/operator/go.mod'),'operator_sum':digest(REPO/'lego/operator/go.sum'),'types_mod':digest(REPO/'lego/types/go.mod'),'types_sum':digest(REPO/'lego/types/go.sum'),'helper':digest(helper_path),'go':digest(GO),'lint':digest(LINT)}
before=inventory();dump('SOURCE-BEFORE.json',before)
startfree=shutil.disk_usage(R).free;assert startfree>=FLOOR+MARGIN
for path in json.loads((R/'overlay.json').read_bytes())['Replace']:
 original=R/'before'/Path(path).relative_to(REPO);assert digest(original)==digest(path)
env=os.environ.copy()
for key in list(env):
 if key.startswith(('BEX_','GITHUB_','RUNNER_')) or key in ('CI','KUBECONFIG','DOCKER_CONTEXT','PYTHONPATH','PYTHONOPTIMIZE'):env.pop(key,None)
env.update(TMPDIR=str(TMP),GOTMPDIR=str(TMP),GOCACHE=str(CACHE),GOLANGCI_LINT_CACHE=str(TMP/'lint-cache'),GOWORK='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOTELEMETRY='off',GOENV='off',GOMAXPROCS='2',GOMEMLIMIT='512MiB',GOFLAGS='-mod=readonly -overlay='+str(R/'overlay.json'),PYTHONDONTWRITEBYTECODE='1')
commands=[('go-version',[str(GO),'version'],10),('lint-version',[str(LINT),'version'],10),('focused',[str(GO),'test','-p','1','-count=1','./internal/build','-run','^(TestCommonElixir|TestNativeUtilityPrerequisiteRecipe|TestNativeUtilityBuildUses|TestNativeUtilityWheelPreparation)'],300),('full-buildpkg',[str(GO),'test','-p','1','-count=1','./internal/build'],600),('scoped-lint',[str(LINT),'run','--concurrency','2','./internal/build'],300)]
dump('SCOPE.json',{'at':datetime.datetime.now(datetime.UTC).isoformat(),'commands':commands,'cwd':str(REPO/'lego/operator'),'overlay':json.loads((R/'overlay.json').read_bytes()),'helper_sha256':helper_sha,'source_before':before,'start_free':startfree,'physical_floor_bytes':FLOOR,'margin_bytes':MARGIN,'owned_TMP_cap':TMPCAP,'cache':'Ordinary Go-managed shared content-addressed cache reads/writes; never pruned/deleted. Own lint cache and TMP removed after allgroups absence.','CPU_memory':'GOMAXPROCS2/GOMEMLIMIT512MiB, go-p1/lint-concurrency2; Go operating controls, not a hard OS RSS/CPU quota.','physical_source_scope':'Exact current working functional set including retained ignoredpycs; no clean physical300 qualification asserted.','no_Docker_or_network_integration_flags':True})
results=[];minimum=startfree;peak=0;registry={};cancelled=[];handlers={};failure=None
try:
 for sig in (signal.SIGINT,signal.SIGTERM):handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:cancelled.append(n))
 for label,argv,seconds in commands:
  started=time.monotonic();stop=None;process=None;absent=False
  if registry:raise RuntimeError('prior groups unproved')
  assert inventory()==before,'source changed before command'
  with (R/(label+'.stdout')).open('wb') as out,(R/(label+'.stderr')).open('wb') as err:
   try:
    process=subprocess.Popen(argv,cwd=REPO/'lego/operator',env=env,stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True);registry[process.pid]=process
    dump(label+'-PROCESS.json',{'pid':process.pid,'pgid':process.pid,'argv':argv})
    while process.poll() is None:
     free=shutil.disk_usage(R).free;owned=size(TMP);minimum=min(minimum,free);peak=max(peak,owned)
     if cancelled or free<FLOOR+MARGIN or owned>TMPCAP-MARGIN or time.monotonic()-started>seconds:
      stop={'free':free,'owned_TMP':owned,'seconds':time.monotonic()-started,'signals':cancelled};break
     time.sleep(.25)
   except BaseException as error:stop={'observer_or_launch_error':type(error).__name__,'message':str(error)}
   finally:
    if process is not None:
     absent=cleanup.stop_group(process)
     if absent:registry.pop(process.pid,None)
  free=shutil.disk_usage(R).free;owned=size(TMP);minimum=min(minimum,free);peak=max(peak,owned)
  if cancelled or free<FLOOR+MARGIN or owned>TMPCAP-MARGIN:stop=stop or {'late_bound':True,'free':free,'signals':cancelled}
  result={'label':label,'argv':argv,'exit':process.returncode if process is not None else None,'bound':stop,'seconds':time.monotonic()-started,'group_absent':absent,'stdout_sha256':digest(R/(label+'.stdout')),'stderr_sha256':digest(R/(label+'.stderr'))};results.append(result);dump(label+'-RESULT.json',result);print(json.dumps(result),flush=True)
  if process is None or process.returncode or stop or not absent:break
except BaseException as error:failure=type(error).__name__+':'+str(error)
finally:
 for pgid,process in list(registry.items()):
  if cleanup.stop_group(process):registry.pop(pgid,None)
 for sig,handler in handlers.items():signal.signal(sig,handler)
 after=inventory();dump('SOURCE-AFTER.json',after)
 if not registry:shutil.rmtree(TMP)
 report={'PASS':len(results)==len(commands) and all(r['exit']==0 and r['bound'] is None and r['group_absent'] for r in results) and not registry and before==after and not cancelled and failure is None,'results':results,'failure':failure,'source_unchanged':before==after,'physical_source_entries':len(before['physical_functional_inventory']),'ignored_pycs_retained':[p for p in before['physical_functional_inventory'] if p.endswith('.pyc')],'foreign_sum_unchanged':before['foreign_go_work_sum']==after['foreign_go_work_sum'],'minimum_free_bytes':minimum,'peak_owned_TMP_bytes':peak,'shared_cache_retained':CACHE.is_dir(),'own_TMP_removed':not TMP.exists(),'pending_pgids':sorted(registry),'cancelled':cancelled,'actual_image_CI_or_adoption_claim':False}
 dump('RESULT.json',report)
print(json.dumps(report),flush=True)
raise SystemExit(0 if report['PASS'] else 1)
