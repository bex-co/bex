from pathlib import Path
import datetime,hashlib,json,os,shutil,signal,subprocess,time
ROOT=Path(__file__).resolve().parent
MIB=1<<20;FLOOR=1<<30;MARGIN=128*MIB;CAP=640*MIB
os.umask(0o077)
def digest(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()
def write(name,value):(ROOT/name).write_text(json.dumps(value,indent=2)+"\n")
def size():
 total=0
 for p in ROOT.rglob("*"):
  try:
   if p.is_file() and not p.is_symlink():total+=p.stat().st_size
  except FileNotFoundError:pass
 return total
def alive(pgid):
 try:os.killpg(pgid,0);return True
 except ProcessLookupError:return False
 except PermissionError:return True
config=json.loads((ROOT/"EXECUTION.json").read_text())
assert config["reviewed_entry"] is True and config["layer_workers"]==1
for p,h in config["pinned_sources"].items():assert digest(p)==h,p
assert "FRESHNESS_DIGESTS" not in os.environ
before={p:digest(p) for p in config["pinned_sources"]}
startfree=shutil.disk_usage(ROOT).free
# The cap is an upper bound, not a reservation. Entry uses measured live-byte
# projection; the same whole-run floor and upper cap remain observed throughout.
assert startfree>=config["projected_required_start_bytes"],(startfree,"insufficient measured actual review headroom")
for name in ("evidence","cache","tmp","anonymous-docker-config"):(ROOT/name).mkdir(mode=0o700)
(ROOT/"anonymous-docker-config/config.json").write_text('{"auths":{}}\n')
args=["python3","-B",config["adapter"],"review","--plan",config["plan"],"--repo-root",config["repo"],"--output-root",str(ROOT/"evidence"),"--cache-root",str(ROOT/"cache"),"--layer-workers","1"]
env=os.environ.copy();env.pop("KUBECONFIG",None);env.update(TMPDIR=str(ROOT/"tmp"),DOCKER_CONFIG=str(ROOT/"anonymous-docker-config"),PYTHONDONTWRITEBYTECODE="1")
write("SCOPE.json",{"at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"argv":args,"entry":config,"start_free":startfree,"remaining_floor":FLOOR+MARGIN,"own_byte_cap":CAP,"stop_own_bytes":CAP-32*MIB,"all_kernel_quota_claims":False,"scope":"One exact owned actual V11 review; no foreign files/caches/resources altered; complete evidence retained"})
t=time.monotonic();reason=None;minimum=startfree;peak=0;p=None;rc=None
cancelled={"signal":None};handlers={}
def cancel(signum,frame):cancelled["signal"]=signum
def observe():
 global minimum,peak
 free=shutil.disk_usage(ROOT).free;owned=size();minimum=min(minimum,free);peak=max(peak,owned)
 return {"free":free,"owned":owned,"global_drop":startfree-free,"seconds":time.monotonic()-t,"log":(ROOT/"review.log").stat().st_size}
def violation(row):
 if cancelled["signal"] is not None:return "cancelled-signal-"+str(cancelled["signal"])
 if row["free"]<FLOOR+MARGIN:return "remaining-free-floor"
 if row["owned"]>CAP-32*MIB:return "owned-evidence-cache-cap"
 if row["global_drop"]>CAP-32*MIB:return "observed-global-free-drop"
 if row["log"]>8*MIB:return "parent-output-cap"
 if row["seconds"]>5400:return "whole-review-deadline"
 return None
try:
 for signum in (signal.SIGTERM,signal.SIGINT):handlers[signum]=signal.getsignal(signum);signal.signal(signum,cancel)
 with (ROOT/"review.log").open("wb") as log:
  try:
   if cancelled["signal"] is not None:raise RuntimeError("cancelled before spawn")
   p=subprocess.Popen(args,cwd=config["repo"],env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
   write("PROCESS.json",{"pid":p.pid,"pgid":p.pid,"argv":args})
   while p.poll() is None:
    row=observe();reason=violation(row)
    if reason:break
    time.sleep(1)
  except BaseException as error:reason=reason or "parent-"+type(error).__name__
  finally:
   if p is not None:
    for sig,grace in ((signal.SIGTERM,4),(signal.SIGKILL,3)):
     if not alive(p.pid):break
     try:os.killpg(p.pid,sig)
     except ProcessLookupError:break
     except PermissionError:pass
     end=time.monotonic()+grace
     while time.monotonic()<end and alive(p.pid):p.poll();time.sleep(.1)
    rc=p.wait(timeout=5)
 after={p:digest(p) for p in config["pinned_sources"]};receipts=list((ROOT/"evidence").glob("native-common-bookworm-*/receipt.json"))
 receipt=json.loads(receipts[0].read_text()) if len(receipts)==1 else {}
 final=observe();reason=reason or violation(final)
 absent=p is None or not alive(p.pid)
 success=rc==0 and reason is None and absent and before==after and receipt.get("approved_supplier_candidate") is True and receipt.get("ref")==config["ref"]
 result={"at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"exit":rc,"stop":reason,"seconds":round(time.monotonic()-t,3),"source_unchanged":before==after,"minimum_free":minimum,"peak_owned_bytes":peak,"final_observation":final,"cancelled_signal":cancelled["signal"],"own_group_absent":absent,"log_sha256":digest(ROOT/"review.log"),"candidate_approved":success,"receipt":str(receipts[0]) if len(receipts)==1 else None,"scope":"Actual complete review only if inherited gates and bounded finish succeed; no production/generated/hosted acceptance"}
 write("RESULT.json",result)
 if cancelled["signal"] is not None:
  success=False;result.update(candidate_approved=False,stop="cancelled-signal-"+str(cancelled["signal"]),cancelled_signal=cancelled["signal"]);write("RESULT.json",result)
 print(json.dumps(result))
finally:
 try:
  if cancelled["signal"] is not None:
   success=False
   if "result" in globals():
    result.update(candidate_approved=False,stop="cancelled-signal-"+str(cancelled["signal"]),cancelled_signal=cancelled["signal"])
    write("RESULT.json",result)
 finally:
  for signum,handler in handlers.items():signal.signal(signum,handler)
raise SystemExit(0 if success and cancelled["signal"] is None else 1)
