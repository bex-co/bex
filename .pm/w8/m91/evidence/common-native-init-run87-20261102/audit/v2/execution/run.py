from pathlib import Path
import datetime,hashlib,json,os,shutil,signal,subprocess,time
R=Path('/private/tmp/bex-w8-native-init-run87-monitor-20261102-m10__m4u');E=R/'audit-v2-execution'
SOURCES={'audit_native_init_v2.py':'d68e1f3541409aebe62f5eb2b073f2e99c35dd99cc4696757fe752fc69f39fca','test_audit_native_init_v2.py':'7d1d2a22ae2ac9b9c6d3569d665831657b378657ddfea7eeb3143a8911ab3f7a'}
start=time.monotonic();floor=(8<<30)+(32<<20);cancel=[];active=None
rec={'complete':False,'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'PURE_CONTROLS_AND_READONLY_ACTUAL_ARTIFACT_AUDIT_NO_TARGET_OR_CI_ACTION','deadline':60,'log_cap':4<<20,'physical_floor_plus_margin':floor,'minimum_free':shutil.disk_usage(R).free,'processes':[],'errors':[]}
old={s:signal.getsignal(s) for s in (signal.SIGINT,signal.SIGTERM)}
def handler(sig,frame):cancel.append(sig)
def alive(pgid):
 try:os.killpg(pgid,0);return True
 except ProcessLookupError:return False

def reap(p):
 if alive(p.pid):
  os.killpg(p.pid,signal.SIGTERM)
  end=time.monotonic()+2
  while alive(p.pid) and time.monotonic()<end:p.poll();time.sleep(.05)
 if alive(p.pid):os.killpg(p.pid,signal.SIGKILL)
 p.wait(timeout=3)
 return not alive(p.pid)

def observe():
 free=shutil.disk_usage(R).free;rec['minimum_free']=min(rec['minimum_free'],free)
 if free<=floor:raise RuntimeError('physical floor')
 if time.monotonic()-start>=60:raise RuntimeError('whole60s')
 if cancel:raise RuntimeError('cancelled')
 if sum(p.stat().st_size for p in E.iterdir() if p.suffix in ('.stdout','.stderr'))>4<<20:raise RuntimeError('log4MiB')

try:
 for s in old:signal.signal(s,handler)
 for n,want in SOURCES.items():assert hashlib.sha256((R/n).read_bytes()).hexdigest()==want,n
 for name in ('test_audit_native_init_v2.py','audit_native_init_v2.py'):
  observe();row={'script':name,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()};rec['processes'].append(row)
  with (E/(name+'.stdout')).open('xb') as out,(E/(name+'.stderr')).open('xb') as err:
   argv=['/opt/homebrew/bin/python3.14','-B',str(R/name)];row['argv']=argv
   active=subprocess.Popen(argv,cwd=R,stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True);row['pid']=row['pgid']=active.pid
   while active.poll() is None:observe();time.sleep(.1)
   row['exit']=active.wait();row['group_absent']=not alive(active.pid)
   if not row['group_absent']:row['group_absent_after_stop']=reap(active)
   assert row['group_absent'] or row.get('group_absent_after_stop'),'unreaped group'
   active=None
   assert row['exit']==0,name+' failed; stop without rerun'
  observe()
 rec['complete']=True
except BaseException as error:rec['errors'].append(type(error).__name__+':'+str(error))
finally:
 if active is not None:
  try:rec['active_group_absent_after_cleanup']=reap(active)
  except BaseException as error:rec['errors'].append('cleanup:'+type(error).__name__+':'+str(error))
 try:observe()
 except BaseException as error:rec['complete']=False;rec['errors'].append('final:'+type(error).__name__+':'+str(error))
 try:
  for s,previous in old.items():signal.signal(s,previous)
 finally:
  if cancel:rec['complete']=False
  rec.update(cancelled=cancel,seconds=time.monotonic()-start,final_free=shutil.disk_usage(R).free)
  rec['source_posthash']={n:hashlib.sha256((R/n).read_bytes()).hexdigest() for n in SOURCES}
  if rec['source_posthash']!=SOURCES:rec['complete']=False;rec['errors'].append('source changed')
  rec['outputs']={}
  for p in E.iterdir():
   if p.suffix in ('.stdout','.stderr'):
    b=p.read_bytes();rec['outputs'][p.name]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
  (E/'RESULT.json').write_text(json.dumps(rec,indent=2)+'\n')
print(json.dumps(rec));raise SystemExit(0 if rec['complete'] else 1)
