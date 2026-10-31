"""Owned local Go gate runner: conservative physical-delta budget, no cleanup."""
from pathlib import Path
import datetime, hashlib, json, os, shutil, signal, subprocess, time, tempfile
ROOT=Path('/Users/tianpan/projects/bex7')
HERE=Path(__file__).resolve().parent
FLOOR=3*1024**3
BUDGET=1073741824
MARGIN=64*1024**2
MINIMUM_START=4294967296

def at():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def snapshot():
    prior=json.loads((HERE/'preflight.json').read_text())
    rows=[]
    for row in prior['inputs']:
        p=ROOT/row['path'];data=p.read_bytes()
        rows.append(dict(path=row['path'],bytes=len(data),sha256=hashlib.sha256(data).hexdigest(),mode=oct(p.stat().st_mode&0o777)))
    return rows

def main():
    start_free=shutil.disk_usage(ROOT).free
    record=dict(at=at(),start_free_bytes=start_free,floor_bytes=FLOOR,additional_budget_bytes=BUDGET,budget_metric='Conservative decrease in global physical free bytes, including any concurrent allocations; not attributed solely to this child.',before=snapshot(),steps=[])
    path=HERE/'actual-gates.json'
    def save():
        temporary=path.with_suffix('.tmp');temporary.write_text(json.dumps(record,indent=2)+'\n');temporary.replace(path)
    if start_free<MINIMUM_START:
        record.update(status='capacity-preflight-stop',after=snapshot(),finished_at=at());save();return 75
    if record['before']!=json.loads((HERE/'preflight.json').read_text())['inputs']:
        record.update(status='input-changed-before-start',finished_at=at());save();return 76
    temporary=Path(tempfile.mkdtemp(prefix='bex-w8-common-gates-a2-'))
    record.update(owned_tmpdir=str(temporary),tmp_uid=temporary.stat().st_uid,tmp_gid=temporary.stat().st_gid,proactive_margin_bytes=MARGIN)
    if temporary.stat().st_gid not in os.getgroups():
        record.update(status='tmp-group-preflight-stop',finished_at=at());save();return 76
    minimum=start_free
    for name in ('test','lint'):
        step=dict(command=['make',name],started_at=at(),log=str(HERE/('make-'+name+'.log')))
        record['steps'].append(step);save()
        env=dict(os.environ,GOMAXPROCS='2',TMPDIR=str(temporary))
        with open(step['log'],'wb') as log:
            child=subprocess.Popen(step['command'],cwd=ROOT/'lego/operator',env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
            step['owned_child_pid']=child.pid;save()
            while child.poll() is None:
                free=shutil.disk_usage(ROOT).free;minimum=min(minimum,free)
                record.update(min_free_bytes=minimum,physical_delta_bytes=max(0,start_free-free),last_sample_at=at())
                if free<FLOOR+MARGIN or start_free-free>BUDGET:
                    step['capacity_stop_at']=at();step['free_at_stop']=free
                    os.killpg(child.pid,signal.SIGTERM)
                    try:child.wait(timeout=5)
                    except subprocess.TimeoutExpired:os.killpg(child.pid,signal.SIGKILL);child.wait()
                    break
                save();time.sleep(.5)
        step.update(actual_exit=child.returncode,finished_at=at());save()
        if 'capacity_stop_at' in step:
            record['status']='capacity-stop';break
        if child.returncode:
            record['status']='gate-failed';break
    else:record['status']='passed'
    record.update(after=snapshot(),finished_at=at(),end_free_bytes=shutil.disk_usage(ROOT).free)
    record['inputs_unchanged']=record['before']==record['after']
    save()
    return 0 if record['status']=='passed' and record['inputs_unchanged'] else 75 if record['status']=='capacity-stop' else 1

if __name__=='__main__':raise SystemExit(main())
