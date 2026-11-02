from pathlib import Path
import datetime,hashlib,importlib.util,json,os,signal,subprocess,time,urllib.parse
R=Path(__file__).resolve().parent
started=time.monotonic();rows=[];tags=[];failure=None;cancel=[];pending=[]
previous={x:signal.getsignal(x) for x in (signal.SIGINT,signal.SIGTERM)}
for x in previous:signal.signal(x,lambda n,f:cancel.append(n))
dep=Path('/private/tmp/bex-w8-node14-local-pilot-v4-source-20261101-bop3sbht/run_pilot.py');spec=importlib.util.spec_from_file_location('reviewed_own_group_cleanup',dep);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
prefixes=['1.18.4-erlang-28.0-debian-','1.18.4-erlang-28.0.'];total=0
try:
 for part,prefix in enumerate(prefixes,1):
  for page in range(1,31):
   if cancel:raise RuntimeError('cancelled')
   if time.monotonic()-started>60:raise RuntimeError('metadata whole60s')
   fs=os.statvfs(R)
   if fs.f_bavail*fs.f_frsize<(1<<30)+(128<<20)+(16<<20):raise RuntimeError('metadata floor')
   url='https://hub.docker.com/v2/repositories/hexpm/elixir/tags?'+urllib.parse.urlencode({'name':prefix,'page_size':'100','page':str(page)})
   body=R/('part'+str(part)+'-page'+str(page)+'.json')
   args=['/usr/bin/curl','--disable','--silent','--show-error','--max-time','10','--max-filesize','4194304','--proto','=https','--output',str(body),'--write-out','%{http_code}',url]
   p=None;out=err=b'';absent=False;begin=time.monotonic();command_error=None
   try:
    p=subprocess.Popen(args,stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
    while True:
     try:out,err=p.communicate(timeout=.25);break
     except subprocess.TimeoutExpired:
      if cancel or time.monotonic()-begin>12 or time.monotonic()-started>60:
       command_error='cancel-or-parent-deadline';break
   finally:
    if p is not None:
     absent=m.stop_group(p)
     if not absent:pending.append(p.pid)
     tailout,tailerr=p.communicate(timeout=2);out,err=tailout,tailerr
   (R/(body.stem+'.status')).write_bytes(out);(R/(body.stem+'.stderr')).write_bytes(err)
   data=body.read_bytes() if body.exists() else b'';total+=len(data)
   row={'prefix':prefix,'page':page,'url':url,'argv':args,'exit':p.returncode if p else None,'HTTP_status':out.decode(errors='replace'),'body_bytes':len(data),'body_sha256':hashlib.sha256(data).hexdigest(),'stderr_sha256':hashlib.sha256(err).hexdigest(),'seconds':time.monotonic()-begin,'process_group_absent':absent,'parent_failure':command_error};rows.append(row)
   (R/'REQUESTS.json').write_text(json.dumps(rows,indent=2)+'\n')
   if pending or command_error or p.returncode!=0 or out!=b'200':raise RuntimeError('actual request refused or incomplete')
   if len(data)>4<<20 or total>16<<20:raise RuntimeError('metadata bytecap')
   doc=json.loads(data);results=doc['results'];nxt=doc.get('next');assert isinstance(results,list) and (nxt is None or isinstance(nxt,str))
   names=[x.get('name','') for x in results];assert all(isinstance(x,str) for x in names)
   tags.extend(x for x in names if x.startswith(prefix))
   if not nxt or not results:break
  else:raise RuntimeError('original30pages exhausted')
except BaseException as e:failure=type(e).__name__+': '+str(e)
finally:
 for x,h in reversed(list(previous.items())):signal.signal(x,h)
 if cancel:failure=failure or 'cancelled'
 result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'complete':failure is None and not pending and not cancel,'failure':failure,'pending_pgids':pending,'cancelled':cancel,'requests':len(rows),'total_metadata_bytes':total,'seconds':time.monotonic()-started,'returned_tags':tags,'prefixes':prefixes,'per_request_network_seconds':10,'whole_metadata_seconds':60,'retry_count':0,'scope':'ActualpublicHubtaglisting via defaultverified/usr/bin/curl only; same name/page/page_size as maintainedresolver. No GoTLS/process identity or finalpin/digest/generator/image/runtimequalification claimed. No redirect/auth/credential/trustfallback.'}
 (R/'RESULT.json').write_text(json.dumps(result,indent=2)+'\n')
 print(json.dumps({k:result[k] for k in ('complete','failure','requests','total_metadata_bytes','seconds','pending_pgids')}))
raise SystemExit(0 if result['complete'] else 1)
