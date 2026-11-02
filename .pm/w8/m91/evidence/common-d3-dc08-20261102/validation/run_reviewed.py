from pathlib import Path
import datetime, hashlib, json, os, shutil, signal, subprocess, time
R=Path('/private/tmp/bex-w8-d3-dc08-publication-plan-20261102-mn5rlwhv')
E=R/'execution-20261102-actual'
VIEW=R/'clean-git301-view'
FLOOR=(8<<30)+(32<<20)
started=time.monotonic(); cancelled=[]; processes=[]; current=None
rec={'complete':False,'scope':'LOCAL_READONLY_HELPER_VALIDATION_NOT_CI_OR_ADOPTION','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'deadline_seconds':300,'physical_floor_plus_margin':FLOOR,'minimum_free':shutil.disk_usage(R).free,'peak_new_view_allocated_bytes':0,'peak_view_apparent_bytes':0,'processes':processes,'errors':[]}
def cancel(sig,frame):cancelled.append(sig)
previous={s:signal.getsignal(s) for s in (signal.SIGINT,signal.SIGTERM)}
def group_alive(pgid):
    try:os.killpg(pgid,0);return True
    except ProcessLookupError:return False

def stop(p):
    if group_alive(p.pid):
        os.killpg(p.pid,signal.SIGTERM)
        until=time.monotonic()+3
        while group_alive(p.pid) and time.monotonic()<until:
            p.poll();time.sleep(.05)
    if group_alive(p.pid):os.killpg(p.pid,signal.SIGKILL)
    p.wait(timeout=5)
    until=time.monotonic()+2
    while group_alive(p.pid) and time.monotonic()<until:time.sleep(.05)
    return not group_alive(p.pid)

def observe():
    free=shutil.disk_usage(R).free;rec['minimum_free']=min(rec['minimum_free'],free)
    apparent=allocated=0
    if VIEW.exists():
        for root,dirs,files in os.walk(VIEW,followlinks=False):
            allocated+=os.lstat(root).st_blocks*512
            for name in files:
                q=Path(root)/name;st=q.lstat();apparent+=st.st_size
                if st.st_nlink==1:allocated+=st.st_blocks*512
    rec['peak_new_view_allocated_bytes']=max(rec['peak_new_view_allocated_bytes'],allocated)
    rec['peak_view_apparent_bytes']=max(rec['peak_view_apparent_bytes'],apparent)
    if free<=FLOOR:raise RuntimeError('physical floor')
    if time.monotonic()-started>=300:raise RuntimeError('outer300s')
    if cancelled:raise RuntimeError('cancelled')

try:
    for sig in previous:signal.signal(sig,cancel)
    for name,want in [('stage_view.py','7a57bfe1f01b85d520bba7a0ef49506719569b3f21ae344ed5b90748bd067f5b'),('validate_view.py','6f8b674e7e0ee200bbf722897afec3bb52939eb1c091f5ec43d6cafacf2787ff')]:
        assert hashlib.sha256((R/name).read_bytes()).hexdigest()==want,name
        observe();entry={'script':name,'sha256':want,'argv':['/opt/homebrew/bin/python3.14','-B',str(R/name)],'at':datetime.datetime.now(datetime.timezone.utc).isoformat()};processes.append(entry)
        with (E/(name+'.stdout')).open('xb') as out,(E/(name+'.stderr')).open('xb') as err:
            current=subprocess.Popen(entry['argv'],cwd=R,stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True)
            entry['pid']=current.pid;entry['pgid']=current.pid
            while current.poll() is None:
                observe()
                if out.tell()+err.tell()>16<<20:raise RuntimeError('full log exceeds16MiB')
                time.sleep(.2)
            entry['exit_code']=current.wait();entry['group_absent']=not group_alive(current.pid)
            if not entry['group_absent']:entry['group_absent_after_cleanup']=stop(current)
            assert entry['group_absent'] or entry.get('group_absent_after_cleanup'),'unreaped process group'
            entry['finished']=datetime.datetime.now(datetime.timezone.utc).isoformat();current=None
            assert entry['exit_code']==0,name+' nonzero'
        observe()
    rec['complete']=True
except BaseException as ex:
    rec['errors'].append(type(ex).__name__+':'+str(ex))
finally:
    if current is not None:
        try:rec['active_group_absent_after_cleanup']=stop(current)
        except BaseException as ex:rec['errors'].append('cleanup:'+type(ex).__name__+':'+str(ex))
    try:
        observe()
    except BaseException as ex:
        rec['complete']=False;rec['errors'].append('final_observation:'+type(ex).__name__+':'+str(ex))
    try:
        for sig,old in previous.items():signal.signal(sig,old)
    finally:
        if cancelled:rec['complete']=False
        rec.update(cancelled=cancelled,seconds=time.monotonic()-started,final_free=shutil.disk_usage(R).free)
        rec['outputs']={}
        for q in E.iterdir():
            if q.is_file() and q.suffix in ('.stdout','.stderr'):
                b=q.read_bytes();rec['outputs'][q.name]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
        (E/'RESULT.json').write_text(json.dumps(rec,indent=2)+'\n')
print(json.dumps(rec))
raise SystemExit(0 if rec['complete'] else 1)
