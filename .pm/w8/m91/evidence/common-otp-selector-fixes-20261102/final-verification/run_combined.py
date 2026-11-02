from pathlib import Path
import hashlib,importlib.util,json,os,shutil,signal,subprocess,time
R=Path(__file__).resolve().parent;ROOT=Path('/Users/tianpan/projects/bex7');rows=json.loads((R/'INTENDED-SOURCE.json').read_text());TMP=R/'go-tmp';TMP.mkdir(mode=0o700)
vp=Path('/private/tmp/bex-w8-predeploy-guarded-retry-data-20261102-nq10_wn9/retained-v4.py');assert hashlib.sha256(vp.read_bytes()).hexdigest()=='e158acde2c44aac290d09f109ddf4f3bf2ca93dc660161dd326fb3ad7cc72c03'
s=importlib.util.spec_from_file_location('combined_reap',vp);v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
def hashes():return {r['relative']:hashlib.sha256((ROOT/r['relative']).read_bytes()).hexdigest() for r in rows}
def size():
 total=0
 def walk_error(e):
  if not isinstance(e,FileNotFoundError):raise e
 for base,ds,fs in os.walk(TMP,onerror=walk_error):
  for n in fs:
   try:total+=(Path(base)/n).stat().st_size
   except FileNotFoundError:pass
 return total
before=hashes();assert before=={r['relative']:r['after'] for r in rows};sum_before=hashlib.sha256((ROOT/'lego/go.work.sum').read_bytes()).hexdigest()
env=dict(os.environ)
for k in list(env):
 if k.startswith(('BEX_','GITHUB_','RUNNER_')) or k in ('CI','PYTHONPATH','PYTHONOPTIMIZE','KUBECONFIG','DOCKER_CONTEXT'):env.pop(k,None)
env.update(GOCACHE='/Users/tianpan/Library/Caches/go-build',GOWORK='off',GOFLAGS='-mod=readonly',GOPROXY='off',GOSUMDB='off',GOTOOLCHAIN='local',GOENV='off',GOTELEMETRY='off',GOMAXPROCS='2',GOMEMLIMIT='512MiB',GOTMPDIR=str(TMP),TMPDIR=str(TMP),PYTHONDONTWRITEBYTECODE='1')
argv=['/opt/homebrew/Cellar/go/1.26.4/libexec/bin/go','test','-p','1','-count=1','./internal/build'];proc=None;absent=False;reason=None;minimum=shutil.disk_usage(R).free;peak=0;began=time.monotonic();cancel=[None];handlers={}
try:
 for sig in (signal.SIGTERM,signal.SIGINT):handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:cancel.__setitem__(0,n))
 assert minimum>8<<30
 (R/'GO-COMMAND.json').write_text(json.dumps({'argv':argv,'cwd':str(ROOT/'lego/operator'),'source':before,'shared_cache_scope':'ordinary Go-managed content-addressed reads/writes; never pruned/deleted','private_TMP':str(TMP),'physical_floor_bytes':8<<30,'own_TMP_cap':512<<20,'seconds':600,'expensive_integration_flags_removed':True},indent=2)+'\n')
 with (R/'go.stdout').open('wb') as out,(R/'go.stderr').open('wb') as err:
  proc=subprocess.Popen(argv,cwd=ROOT/'lego/operator',env=env,stdout=out,stderr=err,start_new_session=True)
  while proc.poll() is None:
   free=shutil.disk_usage(R).free;minimum=min(minimum,free);peak=max(peak,size())
   if free<8<<30:reason='physical8GiBfloor';break
   if peak>512<<20:reason='ownTMP512MiBcap';break
   if time.monotonic()-began>600 or cancel[0]:reason='deadline/cancel';break
   time.sleep(.5)
finally:
 if proc is not None:absent=v.stop_group(proc)
 for sig,h in handlers.items():signal.signal(sig,h)
 after=hashes();sum_after=hashlib.sha256((ROOT/'lego/go.work.sum').read_bytes()).hexdigest()
 if absent:shutil.rmtree(TMP)
 result={'PASS':proc is not None and proc.returncode==0 and absent and reason is None and before==after and sum_before==sum_after,'exit':proc.returncode if proc else None,'bound':reason,'group_absent':absent,'elapsed_seconds':time.monotonic()-began,'minimum_free_bytes':minimum,'peak_own_TMP_bytes':peak,'own_TMP_absent':not TMP.exists(),'source_unchanged':before==after,'foreign_sum_unchanged':sum_before==sum_after,'actual_image_CI_or_adoption_claim':False}
 (R/'GO-RESULT.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result),flush=True)
raise SystemExit(0 if result['PASS'] else 1)
