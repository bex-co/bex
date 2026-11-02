"""Serial unchanged V2 extraction; adds only an outer8GiB physical stop."""
from pathlib import Path
import hashlib,importlib.util,json,shutil,subprocess,time,sys,os
R=Path(__file__).resolve().parent;TOOL=Path('/private/tmp/bex-w8-utility-zip-extractor-v2-source-20261101-evaszd_j/extract_utility_zip.py')
s=importlib.util.spec_from_file_location('own_reap','/private/tmp/bex-w8-common-predeploy-controller-v3-source-20261102-cu0jri4k/retained-v4.py');v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
assert hashlib.sha256(TOOL.read_bytes()).hexdigest()=='365b18ed6583b5d1c0184d412027abcaf19001402b46d7eb123798f10a5e96a4'
assert len(sys.argv)==2 and sys.argv[1] in ('amd64','arm64')
for arch in (sys.argv[1],):
 row=json.loads((R/arch/'download.json').read_bytes());dest=R/arch/'extracted';assert row['complete'] and not dest.exists();
 with __import__('zipfile').ZipFile(R/arch/'original.zip') as z:
  members=z.infolist();assert 0<len(members)<=10000 and sum(x.file_size for x in members)<=512<<20
  projected=sum(((x.file_size+4095)//4096)*4096+4096 for x in members)
  assert shutil.disk_usage(R).free>(8<<30)+projected+(32<<20)
  (R/arch/'PREEXTRACT.json').write_text(json.dumps({'members':len(members),'declared_bytes':sum(x.file_size for x in members),'conservative_allocated_bytes':projected,'physical_floor':8<<30,'margin':32<<20,'observed_free':shutil.disk_usage(R).free},indent=2)+'\n')
 p=None;stop=None;started=time.monotonic();minimum=shutil.disk_usage(R).free
 argv=['/opt/homebrew/bin/python3.14','-B',str(TOOL),str(R/arch/'original.zip'),str(row['observed_bytes']),row['observed_sha256'],str(dest)]
 try:
  with (R/arch/'extract.stdout').open('wb') as out,(R/arch/'extract.stderr').open('wb') as err:
   p=subprocess.Popen(argv,env={'PATH':'/opt/homebrew/bin:/usr/bin:/bin','PYTHONDONTWRITEBYTECODE':'1'},stdin=subprocess.DEVNULL,stdout=out,stderr=err,start_new_session=True)
   while p.poll() is None:
    minimum=min(minimum,shutil.disk_usage(R).free)
    if minimum<8<<30:stop='physical8GiB'
    if time.monotonic()-started>180:stop='extract180s'
    if sum(x.stat().st_size for x in R.rglob('*') if x.is_file())>1536<<20:stop='own1536MiB'
    if stop:break
    time.sleep(.25)
 finally:
  gone=v.stop_group(p) if p else None;minimum=min(minimum,shutil.disk_usage(R).free)
  if minimum<8<<30:stop=stop or 'physical8GiBfinal'
  record={'argv':argv,'exit':p.returncode if p else None,'stop':stop,'groups_absent':gone,'seconds':round(time.monotonic()-started,3),'minimum_free':minimum,'tool_sha256':hashlib.sha256(TOOL.read_bytes()).hexdigest()};(R/arch/'extract-guard.json').write_text(json.dumps(record,indent=2)+'\n')
 print(json.dumps({'arch':arch,**record}),flush=True)
 if not p or p.returncode or stop or not gone:raise SystemExit(1)
