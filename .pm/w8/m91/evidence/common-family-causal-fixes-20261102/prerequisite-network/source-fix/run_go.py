from pathlib import Path
import hashlib,importlib.util,json,os,shutil,signal,subprocess,sys,time
R=Path(__file__).resolve().parent;os.umask(0o077)
s=importlib.util.spec_from_file_location('owned_reap','/private/tmp/bex-w8-common-predeploy-controller-v3-source-20261102-cu0jri4k/retained-v4.py');v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
label,overlay,pattern=sys.argv[1:];tmp=R/(label+'-tmp');tmp.mkdir();os.chown(tmp,-1,os.getgid());os.chmod(tmp,0o700)
E={k:os.environ[k] for k in ('HOME','USER','LOGNAME') if k in os.environ};E.update(PATH='/opt/homebrew/bin:/usr/bin:/bin',TMPDIR=str(tmp),GOTMPDIR=str(tmp),GOWORK='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOENV='off',GOMAXPROCS='2',GOMEMLIMIT='512MiB',PYTHONDONTWRITEBYTECODE='1')
argv=['/opt/homebrew/bin/go','test','-p=1','-mod=readonly','-count=1','-timeout=180s','-overlay='+str(R/overlay),'-run='+pattern,'-v','./internal/build'];began=time.monotonic();before=shutil.disk_usage(R).free;assert before>=8<<30
p=None;stop=None;mins=before;handlers={};cancel={'signal':None};gone=None
try:
 for sig in (signal.SIGINT,signal.SIGTERM):handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:cancel.update(signal=n))
 with (R/(label+'.stdout')).open('wb') as out,(R/(label+'.stderr')).open('wb') as err:
  p=subprocess.Popen(argv,cwd='/Users/tianpan/projects/bex7/lego/operator',env=E,stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True)
  while p.poll() is None:
   mins=min(mins,shutil.disk_usage(R).free)
   if mins<8<<30:stop='physical-floor'
   if cancel['signal']:stop='cancelled'
   if time.monotonic()-began>240:stop='whole-timeout'
   if out.tell()+err.tell()>16<<20:stop='output-bound'
   if stop:break
   time.sleep(.25)
finally:
 if p is not None:gone=v.stop_group(p)
 for sig,h in handlers.items():signal.signal(sig,h)
 if cancel['signal']:stop=stop or 'cancelled'
 record={'argv':argv,'exit':None if p is None else p.returncode,'stop':stop,'groups_absent':gone,'seconds':round(time.monotonic()-began,3),'free_before':before,'minimum_free':mins,'free_after':shutil.disk_usage(R).free,'normal_shared_cache':True,'network_compile_env':'GOPROXY=off GOSUMDB=off GOWORK=off GOTOOLCHAIN=local; no integration flags','result_is_source_test_only':True,'output':{}}
 for suffix in ('stdout','stderr'):
  f=R/(label+'.'+suffix);b=f.read_bytes();record['output'][suffix]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
 if gone:shutil.rmtree(tmp)
 record['owned_compile_tmp_absent']=not tmp.exists();(R/(label+'.process.json')).write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps(record));raise SystemExit(0 if not stop and gone else 2)
