from pathlib import Path
import datetime,errno,hashlib,importlib.util,json,os,shutil,signal,stat,subprocess,time
R=Path(__file__).resolve().parent;REPO=Path('/Users/tianpan/projects/bex7');TMP=R/'tmp';CACHE=Path('/Users/tianpan/Library/Caches/go-build')
FLOOR=8<<30;MARGIN=128<<20;TMPCAP=512<<20
TMP.mkdir(mode=0o700);os.chown(TMP,-1,os.getgid())
source=Path('/private/tmp/bex-w8-go-rust-metadata-source-20261102-ubj864aa/source')
paths=[source/"lego/operator/internal/build/dockerhub.go",source/"lego/operator/internal/build/dockerhub_test.go",REPO/"lego/operator/internal/build/dockerhub.go",REPO/"lego/operator/internal/build/dockerhub_test.go",REPO/"lego/go.work.sum"]
def hashes():return {str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}
def dump(n,d):(R/n).write_text(json.dumps(d,indent=2)+'\n')
def size(p):
 def onerror(error):
  if error.errno != errno.ENOENT:raise error
 total=0
 for base,dirs,files in os.walk(p,onerror=onerror,followlinks=False):
  for name in files:
   try:info=(Path(base)/name).lstat()
   except FileNotFoundError:continue
   if stat.S_ISREG(info.st_mode):total+=info.st_size
 return total
def group_alive(pid):
 try:os.killpg(pid,0);return True
 except ProcessLookupError:return False
# Exact reviewed TERM/KILL/reap/group-absence helper, no Docker code invoked.
dep=Path('/private/tmp/bex-w8-node14-local-pilot-v4-source-20261101-bop3sbht/run_pilot.py');spec=importlib.util.spec_from_file_location('reviewed_group_cleanup',dep);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
before=hashes();startfree=shutil.disk_usage(R).free
assert startfree>=FLOOR+MARGIN,(startfree,'below physical floor plus margin')
env=os.environ.copy();env.update(TMPDIR=str(TMP),GOCACHE=str(CACHE),GOWORK='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOMAXPROCS='2',GOMEMLIMIT='512MiB',GOFLAGS='-mod=readonly')
for key in list(env):
 if key=='KUBECONFIG' or key=='BEX_BUILDKIT_INTEGRATION' or key.startswith('BEX_NATIVE_'):env.pop(key,None)
version=subprocess.check_output(['go','version'],env=env,text=True).strip();assert version.startswith('go version go1.26.')
assert CACHE.is_dir()
commands=[('old-red',['go','test','-overlay',str(R/'old-red-overlay.json'),'-p','1','-count=1','./internal/build','-run','^TestImageMetadataOfficialGoAndRust$'],300),('new-focused',['go','test','-overlay',str(R/'new-overlay.json'),'-p','1','-count=1','./internal/build','-run','^(TestImageMetadata.*|TestDockerHubResolver)$'],300)]
dump('SCOPE.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'version':version,'source_before':before,'commands':commands,'start_free':startfree,'physical_free_floor':FLOOR,'margin':MARGIN,'owned_tmp_cap':TMPCAP,'cache':'Root-authorized standard shared content-addressed Go build cache; ordinary Go-managed reads/writes only. No direct edit/delete/prune/cachecleanup. Module/source reads unchanged; offline/modreadonly.','scope':'Focused localhost HTTP unit controls with old-code regression replay; no external network, Docker, runtimeimage or full-buildpackage claim.'})
results=[];minimum=startfree;peak=0;pending=[];registry={};cancelled=[]
previous={sig:signal.getsignal(sig) for sig in (signal.SIGINT,signal.SIGTERM)}
for sig in previous:signal.signal(sig,lambda signum,frame:cancelled.append(signum))
try:
 for label,args,seconds in commands:
  started=time.monotonic();stop=None
  p=None;absent=False
  with (R/(label+'.log')).open('wb') as out:
   try:
    p=subprocess.Popen(args,cwd=REPO/'lego/operator',env=env,stdin=subprocess.DEVNULL,stdout=out,stderr=subprocess.STDOUT,start_new_session=True)
    registry[p.pid]=p
    dump(label+'-PROCESS.json',{'pid':p.pid,'pgid':p.pid,'args':args})
    while p.poll() is None:
     free=shutil.disk_usage(R).free;owned=size(TMP);minimum=min(minimum,free);peak=max(peak,owned)
     if cancelled or free<FLOOR+MARGIN or owned>TMPCAP-MARGIN or time.monotonic()-started>seconds:
      stop={'free':free,'owned_tmp':owned,'seconds':time.monotonic()-started,'signals':cancelled};break
     time.sleep(.25)
   except BaseException as error:stop={'observer_or_launch_error':type(error).__name__,'message':str(error)}
   finally:
    if p is not None:
     absent=m.stop_group(p)
     if absent:registry.pop(p.pid,None)
     else:pending.append(p.pid)
  free=shutil.disk_usage(R).free;minimum=min(minimum,free)
  if free<FLOOR+MARGIN or size(TMP)>TMPCAP-MARGIN:stop=stop or {'late_resource_bound':True,'free':free}
  if cancelled:stop=stop or {'cancelled_signals':cancelled}
  result={'expected_exit':1 if label=='old-red' else 0,'label':label,'command':args,'exit':p.returncode if p is not None else None,'bound':stop,'seconds':time.monotonic()-started,'group_absent':absent,'log_sha256':hashlib.sha256((R/(label+'.log')).read_bytes()).hexdigest()};results.append(result);dump(label+'-RESULT.json',result)
  if p is None or p.returncode!=result['expected_exit'] or stop or not absent:break
finally:
 for pid,process in list(registry.items()):
  if m.stop_group(process):registry.pop(pid,None)
 pending=sorted(registry)
 for signum,handler in previous.items():signal.signal(signum,handler)
 if not pending:
  shutil.rmtree(TMP)
 after=hashes();report={'results':results,'source_unchanged':before==after,'source_after':after,'minimum_free':minimum,'peak_owned_tmp':peak,'shared_build_cache_retained':CACHE.is_dir(),'tmp_removed':not TMP.exists(),'pending_pgids':pending,'cancelled_signals':cancelled,'scope':'No networkdownloads/modulewrites/Docker/DB/source or production changes. Root-authorized Go-managed shared build-cache writes only; focused HTTP unit + expected-oldRED via exact private two-file Gooverlay. No sharedcache deletion.'};dump('RESULT.json',report)
print(json.dumps(report,indent=2))
raise SystemExit(0 if len(results)==2 and all(x['exit']==x['expected_exit'] and x['bound'] is None and x['group_absent'] for x in results) and before==after and not pending and not cancelled else 1)
