from pathlib import Path
from types import SimpleNamespace
import importlib.util,json,tempfile,copy,os,hashlib
s=importlib.util.spec_from_file_location('generic','/tmp/loopx-w8-native-review-v8.py');g=importlib.util.module_from_spec(s);s.loader.exec_module(g)
s=importlib.util.spec_from_file_location('v','/tmp/loopx-w8-verify-native-artifact.py');v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
base=dict(name='node-16-bookworm',sha='4330878c280bc9748444c8e1e50ff7388df1228a',run='41029599983',repo_root='/Users/tianpan/projects/bex7',policy_module='/tmp/loopx-w8-verify-native-artifact.py',output_root='/tmp/native-policy-never-approved',cache_root='/tmp/native-policy-guard-cache',layer_workers=2);rejected=[]
def reject(label,fn):
 try:fn()
 except RuntimeError:rejected.append(label);return
 raise AssertionError(label+' accepted')
for label,changes in [('invalid-source-sha',{'sha':'bad'}),('invalid-run-identity',{'run':'1;bad'}),('invalid-matrix-name',{'name':'../escape'}),('repository-full-payload-output',{'output_root':'/Users/tianpan/projects/bex7/.pm'})]:
 d=dict(base,**changes);reject(label,lambda d=d:g.Review(SimpleNamespace(**d)))
old=os.environ.get('FRESHNESS_DIGESTS');os.environ['FRESHNESS_DIGESTS']='forbidden-guard-input'
try:reject('fixture-digest-injection',lambda:g.Review(SimpleNamespace(**base)))
finally:
 if old is None:os.environ.pop('FRESHNESS_DIGESTS',None)
 else:os.environ['FRESHNESS_DIGESTS']=old
proof=Path('/tmp/loopx-w8-native-modern-review/docker-root-proof');m=json.loads((proof/'amd64-manifest.json').read_text());c=json.loads((proof/'amd64-config.json').read_text());desc=m['layers'][0];diff=c['rootfs']['diff_ids'][0];actual=json.loads((proof/'verified-layers'/(desc['digest'].split(':')[1]+'.json')).read_text())
for label,changes in [('forged-compressed-size',{'compressed_bytes':actual['compressed_bytes']+1}),('foreign-config-diffid',{'uncompressed_digest':'sha256:'+'0'*64}),('unbounded-decompressed-size',{'uncompressed_bytes':(64<<30)+1}),('foreign-registry-cache',{'registry_repository':'ghcr.io/foreign/image'}),('missing-actual-decompression',{'manual_decompressed_layer_hashing_performed':False})]:
 with tempfile.TemporaryDirectory(prefix='native-layer-guard-') as td:
  r=g.Review.__new__(g.Review);r.v=v;r.layer_cache=Path(td);r.install_compressed_receipt=lambda x:(_ for _ in ()).throw(AssertionError('invalid cache was installed'));record=dict(actual,**changes);(r.layer_cache/(desc['digest'].split(':')[1]+'.json')).write_text(json.dumps(record));reject(label,lambda:r.layer(('amd64',0,desc,diff)))
from unittest.mock import patch
from collections import namedtuple
Usage=namedtuple('Usage','total used free')
with patch.object(g.shutil,'disk_usage',return_value=Usage(2<<30,2<<30,0)):
 reject('scratch-empty-fails-closed',lambda:g.require_capacity(Path('/tmp')))
with patch.object(g.shutil,'disk_usage',return_value=Usage(2<<30,1<<30,g.DISK_RESERVE+16)):
 reject('scratch-descriptor-budget-preserves-reserve',lambda:g.require_capacity(Path('/tmp'),17))
 assert g.require_capacity(Path('/tmp'),16)==g.DISK_RESERVE+16
rejected.append('exact-capacity-boundary-accepted')
import io
class Response(io.BytesIO):
 def __init__(self,data,status=200,headers=None):super().__init__(data);self.status=status;self.headers=headers or {}
class Interrupted(Response):
 def __init__(self,data,error=None):super().__init__(data);self.once=False;self.error=error or TimeoutError('simulated body interruption')
 def read(self,size=-1):
  if self.once:raise self.error
  self.once=True;return super().read(size)
with tempfile.TemporaryDirectory(prefix='native-download-disk-guard-') as td:
 r=g.Review.__new__(g.Review);r.v=v;r.out=Path(td);r.bundle_dir=Path(td);r.public=lambda:('https://guard.invalid/v2/native',{});r.digest='sha256:'+'1'*64;r.progress=lambda *a,**kw:None
 payload=b'x'*(3<<20);blob='sha256:'+hashlib.sha256(payload).hexdigest();manifest_raw=b'{}';manifest={'subject':{'digest':r.digest},'layers':[{'digest':blob,'size':len(payload)}]};descriptor={'digest':'sha256:'+'2'*64,'size':len(manifest_raw)}
 keep=r.bundle_dir/'existing-active-input.txt';keep.write_text('keep')
 checks=[0]
 def bounded_capacity(*args):
  checks[0]+=1
  if checks[0]>=4:raise RuntimeError('simulated free space consumed by a concurrent owner')
  return g.START_FREE
 with patch.object(v,'manifest_document',return_value=(manifest,manifest_raw,descriptor['digest'])),patch.object(g.urllib.request,'urlopen',return_value=Response(payload)),patch.object(g,'require_capacity',side_effect=bounded_capacity):
  reject('partial-download-disk-failure-cleans-only-owned-temp',lambda:r.fetch_bundle(descriptor))
 assert keep.read_text()=='keep' and not list(r.bundle_dir.glob('*.tmp-*')) and not list(r.bundle_dir.glob('*.bundle.json'))
attempts=[0]
def temporary_read():
 attempts[0]+=1
 if attempts[0]<3:raise g.urllib.error.URLError('simulated TLS timeout')
 return 'verified transport only'
with patch.object(g.time,'sleep'):
 assert g.retry_read(temporary_read)=='verified transport only' and attempts[0]==3
rejected.append('transient-read-bounded-retry-only')
for name,error in [('HTTP404-keeps-referrer-fallback',g.urllib.error.HTTPError('https://guard.invalid',404,'missing',{},None)),('policy-mismatch-never-retried',RuntimeError('wrong cryptographic subject'))]:
 n=[0]
 def bad_read():n[0]+=1;raise error
 try:
  with patch.object(g.time,'sleep'):g.retry_read(bad_read)
 except type(error):pass
 else:raise AssertionError(name+' swallowed')
 assert n[0]==1
 rejected.append(name)
for mode in ('valid-range-resume','incomplete-http-body-resume','server200-restarts-full-body','wrong-range-offset','wrong-range-total','tampered-prefix','overlong-range-body'):
 with tempfile.TemporaryDirectory(prefix='native-range-guard-') as td:
  path=Path(td)/'bundle.json';payload=b'body-hash-and-size-must-match';digest='sha256:'+hashlib.sha256(payload).hexdigest();calls=[]
  def open_range(request,timeout):
   calls.append(dict(request.header_items()))
   if len(calls)==1:return Interrupted(payload[:3] if mode!='tampered-prefix' else b'bad',g.http.client.IncompleteRead(b'',1) if mode=='incomplete-http-body-resume' else None)
   assert request.get_header('Range')=='bytes=3-'
   if mode=='server200-restarts-full-body':return Response(payload)
   start=4 if mode=='wrong-range-offset' else 3;total=len(payload)+1 if mode=='wrong-range-total' else len(payload);end=len(payload)-2 if mode=='overlong-range-body' else len(payload)-1
   return Response(payload[3:],206,{'Content-Range':f'bytes {start}-{end}/{total}'})
  with patch.object(g.urllib.request,'urlopen',side_effect=open_range),patch.object(g.time,'sleep'):
   operation=lambda:g.immutable_download('https://guard.invalid/blob',{},path,digest,len(payload),Path(td),lambda *a,**kw:None)
   if mode in ('valid-range-resume','incomplete-http-body-resume','server200-restarts-full-body'):
    operation();assert path.read_bytes()==payload and len(calls)==2;rejected.append(mode+'-SHA-verified')
   else:
    reject(mode+'-rejected',operation);assert not path.exists() and not list(Path(td).glob('*.tmp-*'))
r=g.Review.__new__(g.Review);r.v=v;r.base='python:3.8-bookworm@sha256:'+'3'*64;r.f=SimpleNamespace(REGISTRY_API={'docker.io':'registry-1.docker.io'});r.progress=lambda *a,**kw:None
with patch.object(v,'public_registry',return_value=('https://registry-1.docker.io/v2/library/python',{'Authorization':'Bearer anonymous-unit-transport-token'})) as auth:
 refreshed=r.refreshed_headers('https://registry-1.docker.io/v2/library/python/manifests/sha256:'+'4'*64,{'Accept':'keep'})
 assert refreshed['Accept']=='keep' and refreshed['Authorization'].startswith('Bearer ');assert auth.call_count==1
 reject('foreign-public-repository-refresh-refused',lambda:r.refreshed_headers('https://registry-1.docker.io/v2/private/other/blobs/sha256:'+'4'*64,{}))
 reject('nonTLS-refresh-refused',lambda:r.refreshed_headers('http://registry-1.docker.io/v2/library/python/blobs/sha256:'+'4'*64,{}))
 assert auth.call_count==1
for always in (False,True):
 calls=[0];refreshes=[0]
 def raw_read(url,headers,bound,method='GET'):
  calls[0]+=1
  if always or calls[0]==1:raise g.urllib.error.HTTPError(url,401,'expired public token',{},None)
  return b'verified-transport-only',None
 r.raw_small_get=raw_read
 def refresh(url,headers):refreshes[0]+=1;return {}
 with patch.object(r,'refreshed_headers',side_effect=refresh):
  if always:
   try:r.small_get('https://registry-1.docker.io/v2/library/python/manifests/sha256:'+'4'*64,{},32)
   except g.urllib.error.HTTPError as e:assert e.code==401
   else:raise AssertionError('persistent401 accepted')
   rejected.append('persistent-public401-remains-unapproved')
  else:
   assert r.small_get('https://registry-1.docker.io/v2/library/python/manifests/sha256:'+'4'*64,{},32)[0]==b'verified-transport-only';rejected.append('expired-public-token-refresh-bounded')
 assert calls[0]==2 and refreshes[0]==1
for case,label,errors,expected in [('TUF-transport-retry','positive',['error getting trusted root from TUF: TLS handshake timeout',''],2),('source-policy-no-retry','positive',['expected GithubWorkflowSHA mismatch'],1),('subject-policy-no-retry','positive',['provided artifact digest does not match any digest in statement'],1),('negative-control-TUF-no-retry','wrong-source',['error getting trusted root from TUF: TLS handshake timeout'],1)]:
 with tempfile.TemporaryDirectory(prefix='native-cosign-retry-guard-') as td:
  r=g.Review.__new__(g.Review);r.v=v;r.a=SimpleNamespace(sha='4'*40);r.digest='sha256:'+'5'*64;r.bundle_dir=Path(td);r.progress=lambda *a,**kw:None;calls=[]
  def execute(args,**kw):
   calls.append(args);err=errors[min(len(calls)-1,len(errors)-1)];kw['stderr'].write(err.encode());return SimpleNamespace(returncode=1 if err else 0)
  with patch.object(g.subprocess,'run',side_effect=execute),patch.object(g.time,'sleep'):
   result=r.verify_command({'bundle_digest':'sha256:'+'6'*64,'path':str(Path(td)/'not-a-real-attestation'),'predicate_type':g.SIGN},label)
  assert len(calls)==expected and all(command==calls[0] for command in calls)
  assert '--use-signed-timestamps' in calls[0] and '--certificate-github-workflow-sha' in calls[0] and not any(flag.startswith('--ignore') for flag in calls[0])
  assert len(result['attempts'])==expected and len(list(Path(td).glob('*attempt*.result.json')))==expected
  rejected.append(case)
import threading,time
with tempfile.TemporaryDirectory(prefix='native-layer-partial-resume-') as td:
 path=Path(td)/'blob.verified';part=Path(td)/'blob.partial';payload=b'prefix-must-be-full-hash-verified';digest='sha256:'+hashlib.sha256(payload).hexdigest();calls=[0]
 def interrupted(request,timeout):
  calls[0]+=1
  if calls[0]==1:return Interrupted(payload[:3])
  raise TimeoutError('simulated transport remains unavailable')
 with patch.object(g.urllib.request,'urlopen',side_effect=interrupted),patch.object(g.time,'sleep'):
  try:g.immutable_download('https://guard.invalid/blob',{},path,digest,len(payload),Path(td),lambda *a,**kw:None,partial_path=part,reserve=g.LAYER_FREE_RESERVE,keep_transport_partial=True)
  except TimeoutError:pass
  else:raise AssertionError('partial approved')
 assert part.read_bytes()==payload[:3] and not path.exists()
 def resumed(request,timeout):
  assert request.get_header('Range')=='bytes=3-';return Response(payload[3:],206,{'Content-Range':f'bytes3-{len(payload)-1}/{len(payload)}'.replace('bytes3','bytes 3')})
 with patch.object(g.urllib.request,'urlopen',side_effect=resumed):g.immutable_download('https://guard.invalid/blob',{},path,digest,len(payload),Path(td),lambda *a,**kw:None,partial_path=part,reserve=g.LAYER_FREE_RESERVE,keep_transport_partial=True)
 assert path.read_bytes()==payload and not part.exists();rejected.append('stable-layer-partial-resume-full-SHA-before-verified-rename')
for mode in ('disk-enospc','bad-full-SHA'):
 with tempfile.TemporaryDirectory(prefix='native-layer-cleanup-') as td:
  path=Path(td)/'blob.verified';part=Path(td)/'blob.partial';payload=b'body';digest='sha256:'+hashlib.sha256(payload).hexdigest()
  response=OSError(g.errno.ENOSPC,'simulated disk full') if mode=='disk-enospc' else Response(b'baad')
  with patch.object(g.urllib.request,'urlopen',side_effect=response if isinstance(response,Exception) else None,return_value=response),patch.object(g.time,'sleep'):
   try:g.immutable_download('https://guard.invalid/blob',{},path,digest,len(payload),Path(td),lambda *a,**kw:None,partial_path=part,reserve=g.LAYER_FREE_RESERVE,keep_transport_partial=True)
   except (OSError,RuntimeError):pass
   else:raise AssertionError(mode+' accepted')
  assert not path.exists() and not part.exists();rejected.append(mode+'-does-not-retain-unverified-layer')
for mode in ('over-budget','reserve-exhausted','duplicate-prefix','unknown-file'):
 with tempfile.TemporaryDirectory(prefix='native-layer-resource-') as td:
  r=g.Review.__new__(g.Review);r.v=v;r.out=Path(td);digest='sha256:'+'7'*64;r.manifests={'amd64':{'layers':[{'digest':digest,'size':g.LAYER_FILE_BUDGET+1 if mode=='over-budget' else 10}]}}
  directory=r.out/'owned-layer-files';directory.mkdir()
  if mode=='duplicate-prefix':(directory/(digest[7:]+'.partial')).write_bytes(b'x');(directory/(digest[7:]+'.verified')).write_bytes(b'x')
  if mode=='unknown-file':(directory/'foreign.partial').write_bytes(b'x')
  if mode=='reserve-exhausted':
   with patch.object(g.shutil,'disk_usage',return_value=Usage(4<<30,2<<30,g.LAYER_FREE_RESERVE-1)):reject(mode,lambda:r.prepare_layer_files())
  else:reject(mode,lambda:r.prepare_layer_files())
with tempfile.TemporaryDirectory(prefix='native-own-futures-') as td:
 r=g.Review.__new__(g.Review);r.v=v;r.out=Path(td);r.a=SimpleNamespace(layer_workers=1);r.progress=lambda *a,**kw:None;started=[]
 def layer(task):
  started.append(task[1])
  if task[1]==0:raise TimeoutError('own transport failure')
  time.sleep(.15);return task
 r.layer=layer;tasks=[('amd64',i,{'digest':'sha256:'+str(i).zfill(64)},'unused') for i in range(20)]
 try:r.layer_tasks(tasks)
 except TimeoutError:pass
 else:raise AssertionError('own failure hidden')
 event=json.loads((r.out/'own-layer-read-failure.json').read_text());assert event['canceled_only_unstarted_own_futures']>=18 and len(started)<=2 and event['candidate_approved'] is False;rejected.append('as-completed-surfaces-first-error-cancels-only-unstarted-own-work')
for mode in ('valid-full-proof-then-cleanup','bad-compressed-SHA','bad-uncompressed-diffID'):
 with tempfile.TemporaryDirectory(prefix='native-layer-fullproof-') as td:
  r=g.Review.__new__(g.Review);r.v=v;r.out=Path(td);r.layer_cache=r.out/'cache';r.layer_cache.mkdir();r.owned_layers=r.out/'owned';r.owned_layers.mkdir();r.cache_lock=threading.Lock();r.lock=threading.Lock();r.file_lock=threading.Lock();r.new_reads=set();r.progress=lambda *a,**kw:None;r.public=lambda:('https://guard.invalid/v2/image',{});r.install_compressed_receipt=lambda record:None
  raw=b'These are unit transport bytes, not an image or package inventory.'*100;compressed=g.gzip.compress(raw);digest='sha256:'+hashlib.sha256(compressed).hexdigest();diff='sha256:'+hashlib.sha256(raw).hexdigest();blob=r.owned_layers/(digest[7:]+'.verified');blob.write_bytes(compressed if mode!='bad-compressed-SHA' else b'bad!'+compressed[4:]);desc={'digest':digest,'size':len(compressed),'mediaType':'application/vnd.oci.image.layer.v1.tar+gzip'}
  if mode=='bad-uncompressed-diffID':diff='sha256:'+'0'*64
  if mode=='valid-full-proof-then-cleanup':
   a,i,proof=r.layer(('amd64',0,desc,diff));assert proof['manual_decompressed_layer_hashing_performed'] and proof['uncompressed_digest']==diff and not blob.exists();cleanup=json.loads((r.out/'own-layer-file-cleanup.json').read_text());assert cleanup['reclaimed_bytes']==len(compressed) and len(cleanup['removed'])==1;rejected.append(mode)
  elif mode=='bad-compressed-SHA':
   with patch.object(g.gzip,'open',side_effect=AssertionError('decompressed before compressed SHA passed')):reject(mode,lambda:r.layer(('amd64',0,desc,diff)))
   assert blob.exists() and not (r.out/'own-layer-file-cleanup.json').exists()
  else:
   reject(mode,lambda:r.layer(('amd64',0,desc,diff)));assert blob.exists() and not (r.out/'own-layer-file-cleanup.json').exists()
result={'scope':'Argument/cache rejection guards using genuine prior byte receipts; no artifact approval or fabricated package inventory.','rejected_cases':rejected,'generic_driver_sha256':hashlib.sha256(Path('/tmp/loopx-w8-native-review-v8.py').read_bytes()).hexdigest()};Path('/tmp/loopx-w8-python310-peer-controls-fd358/guard-tests-result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
