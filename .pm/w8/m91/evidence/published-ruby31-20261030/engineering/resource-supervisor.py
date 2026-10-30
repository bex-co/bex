import datetime,hashlib,json,os,signal,shutil,subprocess,time
from pathlib import Path
OUT=Path('/tmp/loopx-w8-native-review-ruby31-v11-fd358');CACHE=Path('/tmp/loopx-w8-native-cache-ruby31-v11-fd358');CONTROLS=Path('/tmp/loopx-w8-native-controls-ruby31-v11-fd358')
CASE=OUT/'native-ruby-3.1-bookworm-fd358d2010f8b5e2715250d7148a505ba12c1cb6'
CMD=['python3','/tmp/loopx-w8-native-review-v11.py','--policy-module','/tmp/loopx-w8-verify-native-artifact-ranged.py','--repo-root','/Users/tianpan/projects/bex7','--name','ruby-3.1-bookworm','--sha','fd358d2010f8b5e2715250d7148a505ba12c1cb6','--run','41063733348','--output-root',str(OUT),'--cache-root',str(CACHE),'--layer-workers','2']
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def own_bytes():return sum(p.stat().st_size for root in (OUT,CACHE,CONTROLS) for p in root.rglob('*') if p.is_file() and not p.is_symlink())
assert hashlib.sha256(Path('/tmp/loopx-w8-native-review-v11.py').read_bytes()).hexdigest()=='ea03d2a9c7a21fde50c0d55f6ead84910c2c431f8c7b52c490624fc5ff8bff85'
assert hashlib.sha256(Path('/tmp/loopx-w8-verify-native-artifact-ranged.py').read_bytes()).hexdigest()=='ca4a765d547b92f567f7e08149183a8cb094249d544ae594655c92b93d09ae15'
start=now();log=OUT/'v11-case-attempt4.log';policy={'minimum_free_bytes':2<<30,'maximum_owned_case_bytes':2<<30,'proactive_free_stop_bytes':(2<<30)+(128<<20),'proactive_owned_stop_bytes':(2<<30)-(64<<20),'poll_seconds':2,'maximum_elapsed_seconds':7200,'termination_scope':'Only the new local subprocess session/process group created by this supervisor; no CI or other process'}
source_hashes={p:hashlib.sha256(Path(p).read_bytes()).hexdigest() for p in ['/tmp/loopx-w8-native-review-v11.py','/tmp/loopx-w8-verify-native-artifact-ranged.py']};events=[];reason=None;peak=0;minimum=shutil.disk_usage('/tmp').free;last=0;t0=time.monotonic()
assert minimum>policy['proactive_free_stop_bytes'] and own_bytes()<policy['proactive_owned_stop_bytes']
with log.open('wb') as stream:
 child=subprocess.Popen(CMD,stdout=stream,stderr=subprocess.STDOUT,env=dict(os.environ,GODEBUG='http2client=0'),start_new_session=True)
 while child.poll() is None:
  free=shutil.disk_usage('/tmp').free;owned=own_bytes();peak=max(peak,owned);minimum=min(minimum,free)
  index=CASE/'index.json'
  if index.exists() and hashlib.sha256(index.read_bytes()).hexdigest()!='5f4dc113a6b3e98217f98b98f99824e8245c74b45dff760aaace3983cecd9bfe':reason='exact-assigned-index-mismatch'
  if free<policy['proactive_free_stop_bytes']:reason='proactive-global-free-reserve'
  elif owned>=policy['proactive_owned_stop_bytes']:reason='proactive-own-case-byte-budget'
  elif time.monotonic()-t0>policy['maximum_elapsed_seconds']:reason='bounded-own-review-duration'
  if reason:
   os.killpg(child.pid,signal.SIGTERM)
   try:child.wait(timeout=15)
   except subprocess.TimeoutExpired:os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=10)
   break
  if time.monotonic()-last>=30:
   p=CASE/'progress.json';progress=json.loads(p.read_text()) if p.exists() else None
   event={'at':now(),'free_bytes':free,'owned_bytes':owned,'actual_progress':progress};events.append(event);print(json.dumps(event),flush=True);last=time.monotonic()
  time.sleep(2)
 rc=child.wait()
receipt={'started_at':start,'completed_at':now(),'actual_returncode':rc,'command':CMD,'executed_source_sha256':source_hashes,'log':str(log),'resource_policy':policy,'minimum_observed_free_bytes':minimum,'maximum_observed_owned_bytes':peak,'stopped_reason':reason,'resource_observations':events,'supervisor_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}
(OUT/'process-exit-case-attempt4.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps({'actual_exit':rc,'stopped_reason':reason,'receipt':str(OUT/'process-exit-case-attempt4.json')}),flush=True)
raise SystemExit(rc if rc>=0 else 128-rc)
