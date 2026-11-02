"""Own bounded local file-reference stream; signed URL stays stdin/RAM only."""
from pathlib import Path
import hashlib,importlib.util,json,os,shutil,signal,subprocess,sys,termios,time
R=Path(__file__).resolve().parent;arch=sys.argv[1];scope=R/arch;assert arch in ('amd64','arm64') and scope.is_dir()
s=importlib.util.spec_from_file_location('owned_reap','/private/tmp/bex-w8-common-predeploy-controller-v3-source-20261102-cu0jri4k/retained-v4.py');v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
inputs=json.loads((R/'INPUTS.json').read_bytes())['inputs'][arch];size=inputs[1];assert shutil.disk_usage(R).free>(8<<30)+size+(32<<20)
state=termios.tcgetattr(sys.stdin.fileno());quiet=list(state);quiet[3]&=~termios.ECHO;termios.tcsetattr(sys.stdin.fileno(),termios.TCSANOW,quiet)
p=None;out=err=b'';stop=None;gone=None;began=time.monotonic();minimum=shutil.disk_usage(R).free;handlers={};cancel={'signal':None}
try:
 for sig in (signal.SIGINT,signal.SIGTERM):handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:cancel.update(signal=n))
 print('READY',flush=True);request=sys.stdin.readline().encode();assert 0<len(request)<=16384
 env={'PATH':'/opt/homebrew/bin:/usr/bin:/bin','PYTHONDONTWRITEBYTECODE':'1'}
 p=subprocess.Popen(['/opt/homebrew/bin/python3.14','-B',str(R/'download.py'),arch],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
 first=True
 while True:
  try:out,err=p.communicate(input=request if first else None,timeout=.25);break
  except subprocess.TimeoutExpired as e:
   first=False;minimum=min(minimum,shutil.disk_usage(R).free)
   if cancel['signal']:stop='cancelled'
   if minimum<8<<30:stop='physical8GiB'
   if time.monotonic()-began>240:stop='whole240s'
   if len(e.output or b'')+len(e.stderr or b'')>1<<20:stop='output1MiB'
   if sum(x.stat().st_size for x in R.rglob('*') if x.is_file())>1536<<20:stop='own1536MiB'
   if stop:break
except BaseException as e:stop=stop or type(e).__name__
finally:
 if p is not None:
  gone=v.stop_group(p);out,err=p.communicate(timeout=3)
 minimum=min(minimum,shutil.disk_usage(R).free)
 if minimum<8<<30:stop=stop or 'physical8GiBfinal'
 try:
  for sig,h in reversed(list(handlers.items())):signal.signal(sig,h)
 finally:termios.tcsetattr(sys.stdin.fileno(),termios.TCSANOW,state)
 if cancel['signal']:stop=stop or 'cancelled'
 (scope/'download.stdout').write_bytes(out);(scope/'download.stderr').write_bytes(err)
 record={'exit':p.returncode if p else None,'stop':stop,'group_absent':gone,'seconds':round(time.monotonic()-began,3),'minimum_free':minimum,'arch':arch,'artifact_id':inputs[0],'signed_URL_retained':False,'stdout_bytes':len(out),'stderr_bytes':len(err),'stdout_sha256':hashlib.sha256(out).hexdigest(),'stderr_sha256':hashlib.sha256(err).hexdigest()};(scope/'download-guard.json').write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps(record),flush=True);raise SystemExit(0 if p and p.returncode==0 and not stop and gone else 1)
