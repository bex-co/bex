#!/usr/bin/env python3
"""Root-authorized clean Git0b7 validation only; no Go/Docker/network/helper subprocess."""
import hashlib,json,os,signal,stat,subprocess,sys,time,shutil
from pathlib import Path
R=Path(__file__).parent;REPO=Path('/Users/tianpan/projects/bex7');VIEW=R/'validation-view';PLAN=R/'VALIDATION-PLAN.json'
SRC='0b7fa17e5761a3111e3c9933b141f1a832e6eeec';CAP=16<<20;STOP=(8<<30)+(128<<20)
def digest(p):
 d=hashlib.sha256()
 with Path(p).open('rb') as f:
  while b:=f.read(1<<20):d.update(b)
 return {'bytes':Path(p).stat().st_size,'sha256':d.hexdigest()}
def put(p,j):Path(p).write_text(json.dumps(j,indent=2)+'\n')
assert digest(PLAN)['sha256']=='bf18304433c9d37654beac872c9a172a704ce7a4c9299f134bc8cc806035aa75'
plan=json.loads(PLAN.read_text());cp=json.loads((R/'COPY-PLAN.json').read_text());candidate=json.loads((R/'candidate-proposal.json').read_text());c=candidate['candidate']
manifest=json.loads((R/'evidence/utilities/amd64/artifact/qualification-source.json').read_text());assert manifest['source']==SRC and len(manifest['files'])==300
v=os.statvfs(R);initial=v.f_bavail*v.f_frsize;assert initial>=STOP+CAP,'fresh16MiB plus8GiB+128MiB budget not fit'
assert not VIEW.exists();VIEW.mkdir(mode=0o700)
borrowed=set();groups={};start=time.monotonic();peak=0;minimum=initial;failure=None;result={};handlers={};cancel={'signal':None}
def observe():
 global peak,minimum
 seen=set();used=0
 for root in [VIEW]:
  if not root.exists():continue
  for p in [root,*root.rglob('*')]:
   s=p.lstat();key=(s.st_dev,s.st_ino)
   assert not stat.S_ISLNK(s.st_mode),'view symlink'
   if key not in seen and key not in borrowed:used+=s.st_blocks*512
   seen.add(key)
 for n in ['validation-worker.py','validation.sb','validate.stdout','validate.stderr','VALIDATION-WORKER.json']:
  p=R/n
  if p.exists():used+=p.stat().st_blocks*512
 v=os.statvfs(R);free=v.f_bavail*v.f_frsize;minimum=min(minimum,free);peak=max(peak,used)
 if used>=CAP:raise RuntimeError('own16MiB validation allocation stop')
 if free<=STOP:raise RuntimeError('physical8GiB+128MiB stop')
 if cancel['signal'] or time.monotonic()-start>180:raise RuntimeError('cancelled or180s deadline')
 return used

def link(source,relative,expected):
 source=Path(source);target=VIEW/relative
 assert not relative.startswith('/') and '..' not in Path(relative).parts
 assert stat.S_ISREG(source.lstat().st_mode) and digest(source)=={k:expected[k] for k in ('bytes','sha256')}
 if target.exists():assert digest(target)=={k:expected[k] for k in ('bytes','sha256')};return
 target.parent.mkdir(parents=True,exist_ok=True);s=source.stat();borrowed.add((s.st_dev,s.st_ino));os.link(source,target);observe()

def stop(p):
 for sig in [signal.SIGTERM,signal.SIGKILL]:
  try:os.killpg(p.pid,sig)
  except ProcessLookupError:pass
  try:p.wait(timeout=3)
  except subprocess.TimeoutExpired:continue
  try:os.killpg(p.pid,0)
  except ProcessLookupError:groups.pop(p.pid,None);return True
 return False
try:
 for sig in [signal.SIGINT,signal.SIGTERM]:handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:cancel.update(signal=n))
 paths=sorted(manifest['files']);p=subprocess.Popen(['git','cat-file','--batch'],cwd=REPO,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True);groups[p.pid]=p
 try:raw,err=p.communicate(''.join(SRC+':'+x+'\n' for x in paths).encode(),timeout=20);assert p.returncode==0 and not err
 finally:assert stop(p),'Git reader group remains'
 off=0
 for rel in paths:
  end=raw.index(b'\n',off);blob,kind,size=raw[off:end].decode().split();size=int(size);beg=end+1;b=raw[beg:beg+size];assert raw[beg+size:beg+size+1]==b'\n';off=beg+size+1;d=manifest['files'][rel]
  assert kind=='blob' and len(b)==d['bytes'] and hashlib.sha256(b).hexdigest()==d['sha256']
  target=VIEW/rel;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b);target.chmod(d['permissions']);observe()
 assert off==len(raw)
 descriptors=[c['publication']['metadata']]+list(c['approval'].values());custody=json.loads((REPO/c['approval']['custody']['path']).read_text());descriptors+=list(custody['files'].values())
 for d in descriptors:link(REPO/d['path'],d['path'],d)
 for d in cp['files']:link(d['source'],cp['evidence_destination']+'/'+d['path'],d)
 target=VIEW/'.github/native-common-test-candidate.json';target.write_bytes((R/'candidate-proposal.json').read_bytes());target.chmod(0o600)
 worker='''import hashlib,json,os,sys,types,socket\nfrom pathlib import Path\nfrom unittest.mock import patch\nsys.dont_write_bytecode=True\nview=Path(sys.argv[1]);result=Path(sys.argv[2]);helperpath=view/'.github/scripts/native-common-qualify.py'\nassert hashlib.sha256(helperpath.read_bytes()).hexdigest()=='f564dae9b771de166d18b16564165b8140a7bb857769cb94fed390cbdec406e5'\nmodule=types.ModuleType('isolated_current0b7_validation');module.__file__=str(helperpath)\nexec(compile(helperpath.read_bytes(),str(helperpath),'exec'),module.__dict__)\nexpected=json.loads((view/'.pm/w8/m91/evidence/common-current-0b7-20261102/utilities/amd64/artifact/qualification-source.json').read_text())\nwith patch.object(module.subprocess,'run',side_effect=AssertionError('helper subprocess forbidden')),patch.object(module.subprocess,'Popen',side_effect=AssertionError('helper Popen forbidden')),patch.object(socket,'socket',side_effect=AssertionError('network forbidden')):\n image=module.validate_image_candidate(view)\n full=module.validate_test_candidate(view)\n assert module.source_inventory(view)==expected['files']\n out={'PASS':True,'image_baseline_status':image['baseline_status'],'test_baseline_status':full['baseline_status'],'source':full['source']['sha'],'ref':full['ref'],'producer':full['producer'],'verification':full['verification'],'generator_source':full['generator_source'],'source_files':len(expected['files']),'image_input_count':len(module.image_source_inventory(view)),'architectures':list(full['utilities']),'unchanged_helper':True,'helper_subprocess_forbidden':True,'network_forbidden':True}\n result.write_text(json.dumps(out,indent=2)+'\\n');print(json.dumps(out))\n'''
 (R/'validation-worker.py').write_text(worker)
 profile='(version 1)\n(allow default)\n(deny network*)\n(deny file-write*)\n(allow file-write* (subpath '+json.dumps(str(R))+'))\n(deny file-write* (subpath '+json.dumps(str(VIEW))+'))\n'
 (R/'validation.sb').write_text(profile);observe();env={'PATH':'/opt/homebrew/bin:/usr/bin:/bin','PYTHONDONTWRITEBYTECODE':'1','GITHUB_SHA':SRC,'TMPDIR':str(R)}
 argv=['/usr/bin/sandbox-exec','-f',str(R/'validation.sb'),'/opt/homebrew/bin/python3.14','-B',str(R/'validation-worker.py'),str(VIEW),str(R/'VALIDATION-WORKER.json')]
 with (R/'validate.stdout').open('xb') as out,(R/'validate.stderr').open('xb') as err:
  p=subprocess.Popen(argv,cwd=VIEW,env=env,stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True);groups[p.pid]=p
  try:
   while p.poll() is None:
    observe()
    if max((R/'validate.stdout').stat().st_size,(R/'validate.stderr').stat().st_size)>1<<20:raise RuntimeError('log1MiB bound')
    time.sleep(.1)
  finally:gone=stop(p)
 assert gone and p.returncode==0,'validator child failure'
 result=json.loads((R/'VALIDATION-WORKER.json').read_text());assert result['PASS'];observe()
 # Source modes/bytes and all borrowed original evidence remain unchanged.
 for rel,d in manifest['files'].items():
  q=VIEW/rel;assert digest(q)=={k:d[k] for k in ('bytes','sha256')} and stat.S_IMODE(q.stat().st_mode)==d['permissions']
 for d in cp['files']:assert digest(d['source'])=={k:d[k] for k in ('bytes','sha256')}
except BaseException as error:failure=type(error).__name__+': '+str(error)
finally:
 for p in list(groups.values()):stop(p)
 for sig,h in handlers.items():signal.signal(sig,h)
 # Only this unique validation view; no other path removal.
 assert VIEW.parent==R and VIEW.name=='validation-view' and not VIEW.is_symlink()
 shutil.rmtree(VIEW)
 receipt={'PASS':failure is None and result.get('PASS') is True,'failure':failure,'source':SRC,'GITHUB_SHA':SRC,'validation':result,'seconds':round(time.monotonic()-start,3),'peak_new_allocated_bytes':peak,'initial_free_bytes':initial,'minimum_free_bytes':minimum,'owned_view_removed':not VIEW.exists(),'pending_groups':list(groups),'handlers_restored':all(signal.getsignal(s)==h for s,h in handlers.items()),'candidate':digest(R/'candidate-proposal.json'),'helper':plan['helper'],'source300_modes_unchanged':failure is None,'ignored_workingtree_pycs':'not read/copied/modified','repo_mutation':False,'Go_Docker_network_or_helper_subprocess':False,'stdout':digest(R/'validate.stdout') if (R/'validate.stdout').exists() else None,'stderr':digest(R/'validate.stderr') if (R/'validate.stderr').exists() else None}
 put(R/'VALIDATION.json',receipt);print(json.dumps(receipt),flush=True)
raise SystemExit(0 if receipt['PASS'] else 1)
