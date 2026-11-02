"""Exact three-step read-only audit invocation; no target runtime or network."""
from pathlib import Path
import hashlib, importlib.util, json, os, shutil, signal, subprocess, sys, time

R = Path("/private/tmp/bex-w8-run91-terminal-intake-bound-20261102-bymjzmq5")
S = Path("/private/tmp/bex-w8-run91-bootstrap-accounting-source-20261102-xhm6a_ja")
HERE = Path(__file__).resolve().parent
REAP = Path("/private/tmp/bex-w8-common-predeploy-controller-v3-source-20261102-cu0jri4k/retained-v4.py")
def sha(p): return hashlib.sha256(Path(p).read_bytes()).hexdigest()
assert sha(REAP) == "e158acde2c44aac290d09f109ddf4f3bf2ca93dc660161dd326fb3ad7cc72c03"
assert sha(S/"audit-bootstrap-accounting-v2.py") == "c7e70707760ccd39bc4121436194ddc61fca28f8de9895f6b486b8ecf23a51e0"
assert sha(R/"bind_audit.py") == "794ca9d431227472df6fb746b3840b78cfce91a9c01919515da17085c2fb65ab"
assert shutil.disk_usage(R).free > (8<<30)+(32<<20)
for n in ("ACTUAL-EXTRACTION-BOUND.json", "REVIEW.json", "SOURCE-CUSTODY.json"):
    assert not (R/n).exists() and not (R/n).is_symlink()
m = json.loads((S/"MANIFEST.json").read_bytes())
for row in m["files"]:
    p=S/row["path"]
    assert p.is_file() and not p.is_symlink() and p.stat().st_size==row["bytes"] and sha(p)==row["sha256"]
spec=importlib.util.spec_from_file_location("exact_reviewed_reaper",REAP)
v=importlib.util.module_from_spec(spec);spec.loader.exec_module(v)
# The first wrapper left only this exact owned source copy; never overwrite it.
target=R/"audit-bootstrap-accounting-v2.py"
assert target.is_file() and not target.is_symlink() and sha(target)==sha(S/target.name)
protected=[S/x["path"] for x in m["files"]]+[REAP,R/"audit.py",R/"bind_audit.py",target,
 R/"BINDING.json",R/"jobs-final.json",R/"artifacts-final.json",R/"job-amd64.log"]
before={str(p):sha(p) for p in protected}
# Python's audit hook permits only the auditor's four exact read-only Git shapes.
BOOT = r"""
import os,re,runpy,sys
REPO="/Users/tianpan/projects/bex7"
SOURCE="7ad69bc18ba338ef27733eed0a77adb27477cc5a"
def guard(event,args):
    if event.startswith("socket."):
        raise RuntimeError("offline audit: socket denied")
    if event=="subprocess.Popen":
        executable,argv,cwd,env=args
        ok=(isinstance(argv,(list,tuple)) and executable=="git" and argv[0]=="git"
            and cwd is not None and os.fspath(cwd)==REPO)
        tail=list(argv[1:]) if ok else []
        ok=ok and (tail==["rev-parse","HEAD"] or tail==["cat-file","--batch"]
            or (len(tail)==4 and tail[:3]==["ls-tree","-r","-z"] and re.fullmatch("[a-f0-9]{40}",tail[3]))
            or (len(tail)==2 and tail[0]=="show" and tail[1].startswith(SOURCE+":")))
        if not ok: raise RuntimeError("offline audit: non-read-only-Git child denied")
sys.addaudithook(guard)
script=sys.argv[1]
sys.argv=[script]
runpy.run_path(script,run_name="__main__")
"""
commands=[("controls",S/"test_accounting.py"),("bind",R/"bind_audit.py"),("audit",target)]
record={"complete":False,"steps":[],"stop":None,"groups_absent":True,"scope":"DATED7ad utility content only; no current baseline/adoption authority"}
began=time.monotonic();minimum=shutil.disk_usage(R).free;previous={};cancel={"signal":None}
def flag(n,frame): cancel["signal"]=n
try:
    for sig in (signal.SIGINT,signal.SIGTERM):
        previous[sig]=signal.getsignal(sig);signal.signal(sig,flag)
    for label,path in commands:
        if cancel["signal"]: raise RuntimeError("cancelled")
        p=None;out=err=b"";stop=None;gone=False;started=time.monotonic()
        argv=["/opt/homebrew/bin/python3.14","-B","-c",BOOT,str(path)]
        try:
            p=subprocess.Popen(argv,env={"PATH":"/opt/homebrew/bin:/usr/bin:/bin","PYTHONDONTWRITEBYTECODE":"1"},
                 stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
            while True:
                try:
                    out,err=p.communicate(timeout=.25);break
                except subprocess.TimeoutExpired as e:
                    minimum=min(minimum,shutil.disk_usage(R).free)
                    if cancel["signal"]: stop="cancelled"
                    if time.monotonic()-began>60: stop="whole60s"
                    if minimum<(8<<30)+(32<<20): stop="physical8GiB32MiB"
                    if len(e.output or b"")+len(e.stderr or b"")>4<<20: stop="output4MiB"
                    if stop: break
        finally:
            if p is not None:
                try:
                    gone=v.stop_group(p);out,err=p.communicate(timeout=3)
                except BaseException as e:
                    stop=stop or ("reap-"+type(e).__name__);gone=False
            minimum=min(minimum,shutil.disk_usage(R).free)
            if cancel["signal"]: stop=stop or "cancelled"
            if time.monotonic()-began>60: stop=stop or "whole60s"
            if minimum<(8<<30)+(32<<20): stop=stop or "physical8GiB32MiB"
            if len(out)+len(err)>4<<20: stop=stop or "output4MiB"
            (HERE/(label+".stdout")).write_bytes(out);(HERE/(label+".stderr")).write_bytes(err)
            step={"name":label,"argv_script":str(path),"exit":p.returncode if p else None,"stop":stop,
                  "group_absent":gone,"seconds":round(time.monotonic()-started,3),
                  "stdout_bytes":len(out),"stderr_bytes":len(err),
                  "stdout_sha256":hashlib.sha256(out).hexdigest(),"stderr_sha256":hashlib.sha256(err).hexdigest()}
            record["steps"].append(step);record["groups_absent"] &= gone
        if not p or p.returncode!=0 or stop or not gone:
            raise RuntimeError(label+" failed")
    record["complete"]=True
except BaseException as error:
    record["stop"]=type(error).__name__
finally:
    try:
        for sig,old in reversed(list(previous.items())): signal.signal(sig,old)
    finally:
        record.update(seconds=round(time.monotonic()-began,3),minimum_free=minimum,
                      source_posthashes={str(p):sha(p) for p in protected},
                      sources_unchanged=all(sha(p)==digest for p,digest in before.items()),
                      cancelled=cancel["signal"] is not None)
        if cancel["signal"] or not record["sources_unchanged"]: record["complete"]=False
        (HERE/"RESULT.json").write_text(json.dumps(record,indent=2)+"\n")
print(json.dumps(record))
raise SystemExit(0 if record["complete"] else 1)
