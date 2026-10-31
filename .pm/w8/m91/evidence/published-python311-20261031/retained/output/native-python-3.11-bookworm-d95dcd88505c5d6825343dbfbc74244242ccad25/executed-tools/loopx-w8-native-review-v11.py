#!/usr/bin/env python3
"""Read-only, native-supplier-only review of actual green jobs and immutable signed artifacts."""
import argparse,base64,concurrent.futures,fcntl,gzip,hashlib,importlib.util,json,os,re,shutil,subprocess,tempfile,threading,time,urllib.error,urllib.request
from pathlib import Path
import http.client
import errno
from contextlib import nullcontext

SIGN='https://sigstore.dev/cosign/sign/v1'
TYPES={SIGN,'https://spdx.dev/Document','https://aquasecurity.github.io/trivy/attestation/vuln/v1'}
MAX_UNCOMPRESSED=64<<30
DISK_RESERVE=512<<20
START_FREE=1<<30
LAYER_FILE_BUDGET=2<<30
LAYER_FREE_RESERVE=2<<30

def require_capacity(path,additional=0,reserve=DISK_RESERVE):
 free=shutil.disk_usage(path).free
 if free<reserve+additional:raise RuntimeError("Scratch disk reserve would be exhausted: free="+str(free)+" required="+str(reserve+additional))
 return free

def load(name,path):
 s=importlib.util.spec_from_file_location(name,path);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

def safe_error(error):return re.sub(r'(https?://[^?\s\"]+)\?[^\s\"]+',r'\1?[redacted]',str(error))[-3000:]

def retry_read(operation):
 for attempt in range(3):
  try:return operation()
  except urllib.error.HTTPError as error:
   if error.code!=429 and error.code<500:raise
   if attempt==2:raise
  except (urllib.error.URLError,TimeoutError,ConnectionError,OSError,http.client.HTTPException):
   if attempt==2:raise
  time.sleep(2*(attempt+1))

def read_open(request,timeout):return retry_read(lambda:urllib.request.urlopen(request,timeout=timeout))

def immutable_download(url,headers,path,digest,size,capacity_path,progress,refresh_headers=None,partial_path=None,reserve=DISK_RESERVE,keep_transport_partial=False,write_lock=None):
 tmp=partial_path or path.with_name(path.name+'.tmp-'+str(os.getpid()));last=time.monotonic()
 if tmp.is_symlink() or path.is_symlink():raise RuntimeError('Immutable blob path is a symlink')
 try:
  for attempt in range(3):
   offset=tmp.stat().st_size if tmp.exists() else 0
   if offset>size:raise RuntimeError('Partial immutable blob exceeds descriptor')
   if offset==size:break
   require_capacity(capacity_path,size-offset,reserve);request_headers=dict(headers)
   if offset:request_headers['Range']='bytes='+str(offset)+'-'
   try:
    with urllib.request.urlopen(urllib.request.Request(url,headers=request_headers),timeout=60) as response:
     if response.status==206:
      match=re.fullmatch(r'bytes (\d+)-(\d+)/(\d+)',response.headers.get('Content-Range',''))
      if not match or int(match[1])!=offset or int(match[3])!=size or not offset<=int(match[2])<size:raise RuntimeError('Immutable blob Content-Range differs from requested offset/descriptor')
     elif response.status==200:
      offset=0;require_capacity(capacity_path,size,reserve)
     else:raise RuntimeError('Unexpected immutable blob HTTP status')
     with tmp.open('ab' if offset else 'wb') as file:
      while True:
       chunk=response.read(1<<20)
       if not chunk:break
       if file.tell()+len(chunk)>size or (response.status==206 and file.tell()+len(chunk)>int(match[2])+1):raise RuntimeError('Bundle exceeded descriptor or declared Content-Range')
       with write_lock or nullcontext():
        require_capacity(capacity_path,len(chunk),reserve);file.write(chunk)
       if time.monotonic()-last>30:progress('immutable-bundle-byte-progress',digest=digest,downloaded_bytes=file.tell(),expected_bytes=size,attempt=attempt+1);last=time.monotonic()
    if tmp.stat().st_size==size:break
    raise urllib.error.URLError('Immutable blob response ended before descriptor size')
   except urllib.error.HTTPError as error:
    if error.code==401 and refresh_headers is not None and attempt<2:headers=refresh_headers(url,headers)
    elif error.code!=429 and error.code<500:raise
    if attempt==2:raise
   except (urllib.error.URLError,TimeoutError,ConnectionError,OSError,http.client.HTTPException):
    if attempt==2:raise
   progress('immutable-bundle-bounded-resume',digest=digest,downloaded_bytes=tmp.stat().st_size if tmp.exists() else 0,attempt=attempt+1);time.sleep(2*(attempt+1))
  h=hashlib.sha256();n=0
  with tmp.open('rb') as file:
   while chunk:=file.read(1<<20):h.update(chunk);n+=len(chunk)
  if n!=size or 'sha256:'+h.hexdigest()!=digest:raise RuntimeError('Complete immutable bundle SHA/size differs')
  tmp.replace(path)
 except BaseException as error:
  transport=(isinstance(error,urllib.error.HTTPError) and (error.code==401 or error.code==429 or error.code>=500)) or (not isinstance(error,urllib.error.HTTPError) and isinstance(error,(urllib.error.URLError,TimeoutError,ConnectionError,http.client.HTTPException))) or (isinstance(error,OSError) and error.errno in (errno.ETIMEDOUT,errno.ECONNRESET,errno.ECONNABORTED,errno.EPIPE,errno.ENETUNREACH,errno.EHOSTUNREACH))
  if not (keep_transport_partial and transport):tmp.unlink(missing_ok=True)
  raise

class RangedBlobReader:
 """Deliver contiguous logical bytes; the unchanged canonical consumer owns final SHA/size acceptance."""
 def __init__(self,owner,request,timeout,opener=None):
  self.owner=owner;self.v=owner.v;self.timeout=timeout;self.opener=opener or urllib.request.urlopen;self.response=None;self.offset=0;self.attempts=[];self.last_save=time.monotonic()
  self.base='https://ghcr.io/v2/'+self.v.REPOSITORY;self.digest=request.full_url.rsplit('/',1)[-1]
  self.v.require(request.get_method()=='GET' and request.full_url==self.base+'/blobs/'+self.digest and request.get_header('Range') is None,'Ranged reader refused foreign or pre-ranged request')
  self.v.require(self.digest in owner.blobs,'Ranged reader descriptor is not the selected candidate');digest,self.size=self.v.descriptor(owner.blobs[self.digest]);self.v.require(digest==self.digest,'Ranged reader digest differs')
  self.path=owner.out/('ranged-body-'+digest[7:]+'.json')
 def _capacity(self):require_capacity(self.owner.out,2*self.v.MAX_DOCUMENT,LAYER_FREE_RESERVE)
 def _save(self):
  self.v.dump(self.path,{'digest':self.digest,'expected_size':self.size,'logical_bytes_delivered':self.offset,'candidate_approved':False,'attempts':self.attempts,'scope':'Body GET metadata only; no headers/tokens/URLs or bodybytes retained. Canonical whole-SHA/size gate remains required after every logical byte.'});self.last_save=time.monotonic()
 def _close_response(self):
  if self.response is not None:self.response.close();self.response=None
 def _open(self,reason):
  self._capacity();self.v.require(len(self.attempts)<3,'Ranged body exceeded three GET attempts');self._close_response();url,headers=self.owner.public();self.v.require(url==self.base,'Ranged resume refused a different public scope');headers=dict(headers)
  self.v.require(not any(k.lower()=='range' for k in headers),'Fresh public headers already contain Range');ranged=bool(self.attempts)
  if ranged:headers['Range']='bytes='+str(self.offset)+'-'
  row={'body_get_attempt':len(self.attempts)+1,'requested_offset':self.offset,'range_requested':ranged,'reason':reason,'logical_bytes_delivered':0};self.attempts.append(row)
  try:
   self.response=self.opener(urllib.request.Request(url+'/blobs/'+self.digest,headers=headers),timeout=self.timeout);response=self.response
   self.v.require(urllib.parse.urlsplit(response.geturl()).scheme=='https','Ranged response is not HTTPS');self.v.require(type(response.status) is int,'Ranged HTTP status is not numeric');row['http_status']=response.status
   lengths=response.headers.get_all('Content-Length') if hasattr(response.headers,'get_all') else [response.headers.get('Content-Length')];lengths=lengths or []
   self.v.require(len(lengths)==1 and isinstance(lengths[0],str) and re.fullmatch(r'[0-9]+',lengths[0]) is not None,'Ranged response Content-Length missing or ambiguous');row['content_length']=int(lengths[0])
   self.v.require(row['content_length']==self.size-self.offset,'Ranged response Content-Length differs from remaining descriptor')
   ranges=response.headers.get_all('Content-Range') if hasattr(response.headers,'get_all') else ([response.headers['Content-Range']] if 'Content-Range' in response.headers else []);ranges=ranges or []
   if ranged:
    self.v.require(response.status==206 and len(ranges)==1,'Ranged request requires one exact206 Content-Range; ignoredRange200 is rejected');match=re.fullmatch(r'bytes ([0-9]+)-([0-9]+)/([0-9]+)',ranges[0]);self.v.require(match is not None,'Ranged response Content-Range malformed')
    parsed=[int(x) for x in match.groups()];row['content_range']={'start':parsed[0],'end':parsed[1],'total':parsed[2]};self.v.require(parsed==[self.offset,self.size-1,self.size],'Ranged response offset/end/total differs')
   else:self.v.require(response.status==200 and not ranges,'Initial body GET requires200 without Content-Range')
   row['outcome']='reading';self._save()
  except BaseException as error:
   if isinstance(error,urllib.error.HTTPError):
    if type(error.code) is int:row['http_status']=error.code
    error.close()
   row.update(outcome='open-or-header-error',error_type=type(error).__name__);self._close_response();self._save();raise
 def __enter__(self):self._open('initial');return self
 def read(self,amount=-1):
  self.v.require(type(amount) is int and 0<amount<=1<<20,'Ranged read must use a bounded positive chunk')
  while True:
   self._capacity()
   try:data=self.response.read(amount)
   except (urllib.error.URLError,TimeoutError,ConnectionError,OSError,http.client.HTTPException) as error:
    row=self.attempts[-1];row.update(outcome='read-interruption',error_type=type(error).__name__)
    if isinstance(error,http.client.IncompleteRead) and type(error.partial) is bytes:row['exception_partial_bytes_discarded']=len(error.partial)
    self._close_response();self._save()
    if self.offset>=self.size or len(self.attempts)==3:raise
    self._open('read-interruption');continue
   self.v.require(isinstance(data,bytes),'Ranged body read did not return bytes')
   if data:
    self.offset+=len(data);self.attempts[-1]['logical_bytes_delivered']+=len(data)
    if self.offset>self.size:self.attempts[-1]['outcome']='overlong-body';self._save()
    elif time.monotonic()-self.last_save>=30:self._save()
    return data
   self.attempts[-1]['outcome']='complete' if self.offset==self.size else 'premature-EOF';self._close_response();self._save()
   if self.offset==self.size:return b''
   self.v.require(self.offset<self.size,'Ranged body overlong');self.v.require(len(self.attempts)<3,'Premature EOF exhausted three body GET attempts');self._open('premature-EOF')
 def __exit__(self,error_type,error,traceback):
  self._close_response()
  if error_type is not None and self.attempts:self.attempts[-1].setdefault('consumer_error_type',error_type.__name__)
  self._save()

class Counted:
 def __init__(self,response,bound):self.response=response;self.bound=bound;self.n=0;self.sha=hashlib.sha256()
 def read(self,size=-1):
  data=self.response.read(size);self.n+=len(data)
  if self.n>self.bound:raise RuntimeError('Compressed descriptor exceeded')
  self.sha.update(data);return data

class Review:
 def __init__(self,args):
  self.a=args;self.v=load('native_policy',args.policy_module);self.v.ROOT=Path(args.repo_root).resolve();self.v.require('FRESHNESS_DIGESTS' not in os.environ,'Fixture digest injection forbidden')
  self.raw_small_get=self.v.small_get;self.v.small_get=self.small_get
  self.v.require(re.fullmatch(r'[a-f0-9]{40}',args.sha) and re.fullmatch(r'[a-z0-9.-]+',args.name) and args.run.isdecimal(),'Invalid source/name/run identity')
  self.out=Path(args.output_root).resolve()/('native-'+args.name+'-'+args.sha);self.v.require(self.v.ROOT not in self.out.parents,'Full evidence must stay outside the repository');self.out.mkdir(parents=True,exist_ok=True)
  require_capacity(self.out,START_FREE-DISK_RESERVE);self.tool_hashes={str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in [Path(__file__),Path(args.policy_module)]};executed=self.out/"executed-tools";executed.mkdir(exist_ok=True)
  for p in [Path(__file__),Path(args.policy_module)]:shutil.copyfile(p,executed/p.name)
  self.cache=Path(args.cache_root).resolve();self.cache.mkdir(parents=True,exist_ok=True);self.layer_cache=self.cache/'verified-uncompressed';self.layer_cache.mkdir(exist_ok=True);self.v2=self.v.BlobReceipts(self.cache/'anonymous-compressed-v2',opener=self.open_blob);self.lock=threading.Lock();self.cache_lock=threading.Lock();self.file_lock=threading.Lock();self.platforms={};self.manifests={};self.ids={};self.blobs={};self.crypto=[];self.new_reads=set()
  self.f=load('native_freshness',self.v.ROOT/'scripts/lib/toolchain-freshness.py');self.base=self.v.matrix_base(args.sha,args.name)
  self.v.require(hashlib.sha256(self.v.COSIGN.read_bytes()).hexdigest()==self.v.COSIGN_SHA256,'Verified Cosign binary changed')
 def refreshed_headers(self,url,headers):
  parsed=urllib.parse.urlsplit(url);registries={self.f.REGISTRY_API.get(r,r):r for r in ('ghcr.io','docker.io')};registry=registries.get(parsed.hostname);match=re.fullmatch(r'/v2/(.+)/(?:manifests|blobs|referrers)/[^/]+',parsed.path)
  allowed=self.v.REPOSITORY if registry=='ghcr.io' else 'library/'+self.base.split(':',1)[0]
  self.v.require(parsed.scheme=='https' and registry is not None and match is not None and match[1]==allowed,'Anonymous refresh refused for unreviewed registry scope')
  fresh_url,fresh_headers=self.v.public_registry(self.f,registry,allowed);self.v.require(url.startswith(fresh_url+'/'),'Anonymous refresh returned a different registry scope');self.progress('anonymous-public-token-refresh',registry=registry,repository=allowed)
  return dict(headers,Authorization=fresh_headers['Authorization'])
 def small_get(self,url,headers,bound,method='GET'):
  try:return retry_read(lambda:self.raw_small_get(url,headers,bound,method=method))
  except urllib.error.HTTPError as error:
   if error.code!=401 or '/v2/' not in urllib.parse.urlsplit(url).path:raise
   fresh=self.refreshed_headers(url,headers)
   return retry_read(lambda:self.raw_small_get(url,fresh,bound,method=method))
 def progress(self,stage,**kw):
  with self.lock:
   event={'at':self.v.now(),'stage':stage,'candidate_approved':False,'name':self.a.name,**kw};self.v.dump(self.out/'progress.json',event);print(json.dumps(event),flush=True)
 def public(self):return self.v.public_registry(self.f,'ghcr.io',self.v.REPOSITORY)
 def metadata(self):
  v=self.v;self.progress('actual-CI-and-source-policy');self.ci=v.github_candidate_job(self.a.sha,self.a.name,self.a.run);v.dump(self.out/'ci-runtime-scan-signing-gates.json',self.ci)
  source=self.out/'source';source.mkdir(exist_ok=True);hashes={}
  for path in ['.github/workflows/native-base-image.yml','.github/native-base-recovery.json','lego/operator/internal/build/native.go','deploy/native-base/matrix.py']:
   raw=v.source_bytes(self.a.sha,path,optional=path.endswith('recovery.json'))
   if raw is not None:
    p=source/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(raw);hashes[path]=hashlib.sha256(raw).hexdigest()
  wf=(source/'.github/workflows/native-base-image.yml').read_text();v.require('cosign-release: v3.1.4' in wf and '--with-default-rekor-v2' in wf and not any(x in wf for x in ['<<<<<<<','=======','>>>>>>>']),'Unreviewed modern signing profile')
  v.dump(source/'source-files.json',{'sha':self.a.sha,'files_sha256':hashes})
  self.url,self.headers=self.public();self.tag='native-'+self.a.name+'-'+self.a.sha;index,raw,self.digest=v.manifest_document(self.url,self.headers,self.tag);(self.out/'index.json').write_bytes(raw);self.children=v.image_platforms(index);self.ref='ghcr.io/'+v.REPOSITORY+'@'+self.digest
  self.provenance=v.recovery_provenance(self.a.sha,self.a.name,self.base,self.digest,self.ci,self.out);v.dump(self.out/'producer-verification-provenance.json',self.provenance)
  sr,repo,_=self.f.parse_upstream('docker.io/library/'+self.base.split('@')[0]);su,sh=v.public_registry(self.f,sr,repo);source_index,sraw,_=v.manifest_document(su,sh,self.base.split('@')[1]);(self.out/'official-index.json').write_bytes(sraw);originals=v.image_platforms(source_index)
  for arch,child in self.children.items():
   m,mraw,_=v.manifest_document(self.url,self.headers,child);original,oraw,_=v.manifest_document(su,sh,originals[arch]);c,craw=v.config_document(self.url,self.headers,m['config']);oc,ocraw=v.config_document(su,sh,original['config'])
   v.require(c.get('architecture')==oc.get('architecture')==arch and c.get('os')==oc.get('os')=='linux','Actual platform mismatch');labels=c['config'].get('Labels',{});v.require(labels.get('org.opencontainers.image.revision')==self.provenance['producer_sha'] and labels.get('org.opencontainers.image.source')=='https://github.com/bex-co/bex','Producer label/source mismatch');before,after=v.official_environment(oc),v.official_environment(c);v.require(all(after.get(k)==x for k,x in before.items()),'Official primary ENV changed')
   self.manifests[arch]=m;self.platforms[arch]={'manifest_digest':child,'config_digest':m['config']['digest'],'config':c,'official_base_ancestry':v.official_ancestry(m,c,original,oc)}
   for identity in [child,m['config']['digest']]:v.require(identity not in self.ids,'Ambiguous platform identity');self.ids[identity]=arch
   for suffix,data in [('manifest',mraw),('config',craw),('official-manifest',oraw),('official-config',ocraw)]:(self.out/(arch+'-'+suffix+'.json')).write_bytes(data)
   for d in [m['config'],*m['layers']]:
    h,size=v.descriptor(d)
    if h in self.blobs:v.require(self.blobs[h]['size']==size,'Conflicting blob sizes')
    self.blobs[h]=d
 def fetch_bundle(self,desc):
  v=self.v;url,headers=self.public();m,raw,md=v.manifest_document(url,headers,desc['digest']);v.require(len(raw)==desc['size'] and m.get('subject',{}).get('digest')==self.digest,'Bundle manifest size/subject differs');v.require(len(m.get('layers',[]))==1,'Bundle must contain one payload');(self.bundle_dir/(md.split(':')[1]+'.manifest.json')).write_bytes(raw);layer=m['layers'][0];bd,size=v.descriptor(layer);v.require(size<=v.MAX_ATTESTATIONS,'Bundle read bound exceeded');path=self.bundle_dir/(bd.split(':')[1]+'.bundle.json')
  if path.exists() and path.stat().st_size==size and 'sha256:'+hashlib.sha256(path.read_bytes()).hexdigest()==bd:v.small_get(url+'/blobs/'+bd,headers,0,method='HEAD')
  else:
   immutable_download(url+'/blobs/'+bd,headers,path,bd,size,self.out,self.progress,self.refreshed_headers)
  bundle=json.loads(path.read_text());payload=base64.b64decode(bundle['dsseEnvelope']['payload'],validate=True);st=json.loads(payload);v.require(st.get('predicateType')==desc.get('annotations',{}).get('dev.sigstore.bundle.predicateType') and st['predicateType'] in TYPES,'Predicate annotation/type differs')
  return {'manifest_digest':md,'bundle_digest':bd,'bundle_bytes':size,'path':str(path),'predicate_type':st['predicateType'],'signed_statement_sha256':hashlib.sha256(payload).hexdigest()}
 def fetch_bundles(self):
  self.progress('anonymous-immutable-bundle-download');v=self.v;self.bundle_dir=self.out/'bundles';self.bundle_dir.mkdir(exist_ok=True);url,headers=self.public()
  try:
   raw,_=v.small_get(url+'/referrers/'+self.digest,headers,v.MAX_DOCUMENT);index=json.loads(raw);method='native-referrers';index_sha='sha256:'+hashlib.sha256(raw).hexdigest()
  except urllib.error.HTTPError as error:
   if error.code!=404:raise
   index,raw,index_sha=v.manifest_document(url,headers,self.digest.replace(':','-'));method='sha-verified-OCI-fallback-index'
  (self.bundle_dir/'referrer-index.json').write_bytes(raw);self.discovery={'method':method,'index_sha256':index_sha,'native_API_404':method!='native-referrers'};entries=[d for d in index.get('manifests',[]) if d.get('artifactType')=='application/vnd.dev.sigstore.bundle.v0.3+json' and d.get('annotations',{}).get('dev.sigstore.bundle.predicateType') in TYPES];v.require(0<len(entries)<=64,'Missing or unbounded bundle index');self.discovery['unselected_descriptor_count']=len(index.get('manifests',[]))-len(entries)
  with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:self.bundle_rows=list(pool.map(self.fetch_bundle,entries))
  v.dump(self.bundle_dir/'download-receipt.json',{'discovery':self.discovery,'candidate_approved':False,'bundles':self.bundle_rows})
 def verify_command(self,row,label,source_sha=None,digest=None):
  v=self.v;args=[str(v.COSIGN),'verify-blob-attestation','--bundle',row['path'],'--type',row['predicate_type'],'--digest',(digest or self.digest).split(':')[1],'--digestAlg','sha256','--certificate-identity','https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main','--certificate-oidc-issuer','https://token.actions.githubusercontent.com','--certificate-github-workflow-sha',source_sha or self.a.sha,'--use-signed-timestamps','--timeout','5m']
  base=row['bundle_digest'].split(':')[1]+'-'+label;attempts=[]
  for attempt in range(1,4):
   stem=base+'-attempt'+str(attempt);stdout_path=self.bundle_dir/(stem+'.stdout.log');stderr_path=self.bundle_dir/(stem+'.stderr.log')
   tuf_parent=self.bundle_dir/'cosign-tuf-cache';v.require(not tuf_parent.is_symlink(),'TUF cache parent is a symlink');tuf_parent.mkdir(mode=0o700,exist_ok=True);tuf_cache=Path(tempfile.mkdtemp(prefix=stem+'-',dir=tuf_parent))
   with stdout_path.open('wb') as stdout,stderr_path.open('wb') as stderr:p=subprocess.run(args,env=dict(os.environ,GODEBUG='http2client=0',TUF_ROOT=str(tuf_cache)),stdout=stdout,stderr=stderr,timeout=330)
   result={'bundle_digest':row['bundle_digest'],'exit_code':p.returncode,'command':args,'stderr':str(stderr_path),'tuf_cache':str(tuf_cache)};v.dump(self.bundle_dir/(stem+'.result.json'),result);attempts.append(result)
   text=stderr_path.read_text();transport_only=label=='positive' and 'getting trusted root from TUF' in text and any(message in text for message in ('TLS handshake timeout','i/o timeout','connection reset by peer','context deadline exceeded'))
   if p.returncode==0 or not transport_only or attempt==3:break
   self.progress('strict-Cosign-TUF-transport-retry',bundle_digest=row['bundle_digest'],attempt=attempt);time.sleep(2*attempt)
  result=dict(result,attempts=attempts);v.dump(self.bundle_dir/(base+'.result.json'),result);return result
 def crypto_bundles(self):
  self.progress('strict-keyless-bundle-verification');v=self.v
  with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:results=list(pool.map(lambda r:self.verify_command(r,'positive'),self.bundle_rows))
  self.crypto=results;self.accepted=[r for r,c in zip(self.bundle_rows,results) if c['exit_code']==0];self.rejected=[c for c in results if c['exit_code']!=0];v.require(self.accepted,'No bundle passed the exact source/issuer/subject/tlog/TSA policy');images=[r for r in self.accepted if r['predicate_type']==SIGN];v.require(images,'No actual image signature passed')
  self.negative_source=self.verify_command(images[0],'wrong-source','0'*40);text=Path(self.negative_source['stderr']).read_text();v.require(self.negative_source['exit_code']!=0 and 'expected GithubWorkflowSHA' in text and self.a.sha in text,'Wrong-source negative control did not enforce the actual source claim')
  self.negative_subject=self.verify_command(images[0],'wrong-subject',digest='sha256:'+'0'*64);v.require(self.negative_subject['exit_code']!=0 and 'provided artifact digest does not match any digest in statement' in Path(self.negative_subject['stderr']).read_text(),'Wrong-subject negative control did not enforce the statement subject')
  for row in self.accepted:
   b=json.loads(Path(row['path']).read_text());vm=b['verificationMaterial'];v.require(vm.get('tlogEntries') and all(x.get('kindVersion')=={'kind':'hashedrekord','version':'0.0.2'} for x in vm['tlogEntries']) and vm.get('timestampVerificationData',{}).get('rfc3161Timestamps'),'Actual v2 log/TSA material missing')
  v.dump(self.bundle_dir/'crypto-results.json',{'accepted_bundles':[r['bundle_digest'] for r in self.accepted],'results':results,'wrong_source':self.negative_source,'wrong_subject':self.negative_subject,'candidate_approved':False})
 def signed_statements(self):
  for row in self.accepted:
   b=json.loads(Path(row['path']).read_text());raw=base64.b64decode(b['dsseEnvelope']['payload'],validate=True);self.v.require(hashlib.sha256(raw).hexdigest()==row['signed_statement_sha256'],'Accepted bundle payload changed');yield row,b,json.loads(raw),raw
 def install_compressed_receipt(self,record):
  v=self.v
  with self.cache_lock,self.v2.locked_cache():
   rows=self.v2.read();rows[record['compressed_digest']]={'schema':v.CACHE_SCHEMA,'registry_repository':'ghcr.io/'+v.REPOSITORY,'digest':record['compressed_digest'],'bytes_hashed':record['compressed_bytes'],'anonymous_full_get_at':record['anonymous_full_get_at']};v.dump(self.v2.data,{'schema':v.CACHE_SCHEMA,'blobs':rows})
 def prepare_layer_files(self):
  self.owned_layers=self.out/'owned-layer-files';self.owned_layers.mkdir(exist_ok=True);allowed={digest.split(':')[1]:size for digest,size in (self.v.descriptor(d) for m in self.manifests.values() for d in m['layers'])}
  self.v.require(sum(allowed.values())<=LAYER_FILE_BUDGET,'Selected case exceeds own2GiB layer-file descriptor budget')
  actual=0;seen=set()
  for path in self.owned_layers.iterdir():
   stem=path.name.split('.',1)[0];self.v.require(stem not in seen and path.name==stem+path.suffix,'Duplicate or malformed own layer prefix');seen.add(stem);self.v.require(not path.is_symlink() and path.is_file() and stem in allowed and path.suffix in ('.partial','.verified'),'Unrecognized file in own layer-file area');size=path.stat().st_size;self.v.require(size<=allowed[stem],'Own layer-file prefix exceeds descriptor');actual+=size
  self.v.require(actual<=LAYER_FILE_BUDGET,'Own layer files exceed2GiB budget');require_capacity(self.owned_layers,0,LAYER_FREE_RESERVE)
  self.v.dump(self.out/'layer-file-resource-policy.json',{'maximum_case_bytes':LAYER_FILE_BUDGET,'minimum_remaining_free_bytes':LAYER_FREE_RESERVE,'planned_unique_compressed_bytes':sum(allowed.values()),'existing_actual_bytes':actual,'full_compressed_SHA_size_before_decompression':True,'stable_bounded_partials':True,'atomic_verified_rename':True,'remove_completed_own_files_only_after_proof':True})
 def layer(self,task):
  arch,i,desc,diff=task;v=self.v;digest,size=v.descriptor(desc);path=self.layer_cache/(digest.split(':')[1]+'.json')
  with (self.layer_cache/(digest.split(':')[1]+'.lock')).open('a+') as lock:
   fcntl.flock(lock,fcntl.LOCK_EX)
   if path.exists():
    r=json.loads(path.read_text());v.require(r.get('schema')=='bex.native-compressed-and-diffid-proof/v1' and r.get('registry_repository')=='ghcr.io/'+v.REPOSITORY and r.get('compressed_digest')==digest and r.get('compressed_bytes')==size and r.get('uncompressed_digest')==diff and r.get('manual_decompressed_layer_hashing_performed') is True and type(r.get('uncompressed_bytes')) is int and 0<=r['uncompressed_bytes']<=MAX_UNCOMPRESSED,'Invalid prior decompressed proof');self.install_compressed_receipt(r);return arch,i,r
   self.progress('bounded-own-layer-download',architecture=arch,layer=i,expected_compressed_bytes=size);url,headers=self.public();blob=self.owned_layers/(digest.split(':')[1]+'.verified');partial=blob.with_suffix('.partial')
   if blob.exists():
    h=hashlib.sha256();n=0
    with blob.open('rb') as file:
     while data:=file.read(1<<20):h.update(data);n+=len(data)
    v.require(n==size and 'sha256:'+h.hexdigest()==digest,'Retained verified layer file SHA/size differs')
   else:immutable_download(url+'/blobs/'+digest,headers,blob,digest,size,self.owned_layers,self.progress,self.refreshed_headers,partial,LAYER_FREE_RESERVE,True,self.file_lock)
   v.require(blob.stat().st_size==size,'Verified layer file size changed');self.progress('actual-local-layer-decompression',architecture=arch,layer=i,verified_compressed_bytes=size);h=hashlib.sha256();n=0;last=time.monotonic();media=desc['mediaType']
   if media.endswith('+gzip') or media.endswith('.gzip'):body=gzip.open(blob,'rb')
   elif media in ('application/vnd.oci.image.layer.v1.tar','application/vnd.docker.image.rootfs.diff.tar'):body=blob.open('rb')
   else:raise RuntimeError('Unsupported layer compression for strict proof: '+media)
   with body:
    while data:=body.read(1<<20):
     n+=len(data);v.require(n<=MAX_UNCOMPRESSED,'Decompressed layer exceeds bound');h.update(data)
     if time.monotonic()-last>30:self.progress('local-layer-proof-progress',architecture=arch,layer=i,decompressed_bytes=n);last=time.monotonic()
   v.require('sha256:'+h.hexdigest()==diff,'Actually decompressed diffID differs from verified config');r={'schema':'bex.native-compressed-and-diffid-proof/v1','registry_repository':'ghcr.io/'+v.REPOSITORY,'compressed_digest':digest,'compressed_bytes':size,'uncompressed_digest':diff,'uncompressed_bytes':n,'anonymous_full_get_at':v.now(),'manual_decompressed_layer_hashing_performed':True};v.dump(path,r);self.install_compressed_receipt(r)
   with self.cache_lock:
    cleanup=self.out/'own-layer-file-cleanup.json';records=json.loads(cleanup.read_text()) if cleanup.exists() else {'removed':[],'reclaimed_bytes':0};v.require(blob.stat().st_size==size,'Completed own layer size changed before cleanup');blob.unlink();records['removed'].append({'digest':digest,'bytes':size,'proof_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'removed_at':v.now()});records['reclaimed_bytes']+=size;v.dump(cleanup,records)
   with self.lock:self.new_reads.add(digest)
   return arch,i,r
 def layer_tasks(self,tasks):
  items=[]
  with concurrent.futures.ThreadPoolExecutor(max_workers=self.a.layer_workers) as pool:
   futures={pool.submit(self.layer,task):task for task in tasks}
   for future in concurrent.futures.as_completed(futures):
    try:items.append(future.result())
    except BaseException as error:
     canceled=sum(1 for pending in futures if pending.cancel());arch,i,desc,diff=futures[future];event={'at':self.v.now(),'candidate_approved':False,'architecture':arch,'layer':i,'digest':desc['digest'],'error_type':type(error).__name__,'error':safe_error(error),'canceled_only_unstarted_own_futures':canceled};self.v.dump(self.out/'own-layer-read-failure.json',event);self.progress('own-layer-read-failure',**{k:x for k,x in event.items() if k not in ('at','candidate_approved')});raise
  return items
 def derive_docker_roots_if_needed(self):
  v=self.v;needed=False
  for row,b,st,raw in self.signed_statements():
   if row['predicate_type']==v.SPDX_TYPE:
    try:v.spdx_binding(st['predicate'],self.ids)
    except RuntimeError as error:
     if 'SPDX root names an unrelated image digest' not in str(error):raise
     needed=True
  self.bindings=[]
  if not needed:return
  self.progress('exact-Docker-root-proof-required');proof=self.out/'actual-layer-proof';proof.mkdir(exist_ok=True);(proof/'verified-layers').mkdir(exist_ok=True);tasks=[]
  for arch,m in self.manifests.items():
   diffs=self.platforms[arch]['config']['rootfs']['diff_ids'];v.require(len(diffs)==len(m['layers']),'Layer/diffID cardinality differs');tasks.extend((arch,i,d,diff) for i,(d,diff) in enumerate(zip(m['layers'],diffs)))
  tasks.sort(key=lambda t:t[2]['size'],reverse=True);self.prepare_layer_files();items=self.layer_tasks(tasks)
  bykey={(a,i):r for a,i,r in items};rows=[]
  for arch,m in self.manifests.items():
   layers=[bykey[(arch,i)] for i in range(len(m['layers']))]
   for r in layers:shutil.copyfile(self.layer_cache/(r['compressed_digest'].split(':')[1]+'.json'),proof/'verified-layers'/(r['compressed_digest'].split(':')[1]+'.json'))
   synthetic={'schemaVersion':2,'mediaType':'application/vnd.docker.distribution.manifest.v2+json','config':{'mediaType':'application/vnd.docker.container.image.v1+json','size':m['config']['size'],'digest':self.platforms[arch]['config_digest']},'layers':[{'mediaType':'application/vnd.docker.image.rootfs.diff.tar.gzip','size':r['uncompressed_bytes'],'digest':r['uncompressed_digest']} for r in layers]};raw=json.dumps(synthetic,separators=(',',':'),ensure_ascii=False).encode();alias='sha256:'+hashlib.sha256(raw).hexdigest();(proof/(arch+'-derived-docker-manifest.json')).write_bytes(raw);rows.append({'architecture':arch,'actual_registry_child_digest':self.platforms[arch]['manifest_digest'],'actual_registry_config_digest':self.platforms[arch]['config_digest'],'derived_docker_manifest_digest':alias,'manual_decompressed_layer_hashing_performed':True,'layer_proofs':layers})
  v.dump(proof/'receipt.json',{'index_digest':self.digest,'candidate_approved':False,'architectures':rows});aliases,self.bindings=v.docker_source_root_aliases(proof,self.digest,self.platforms,self.manifests)
  for alias,arch in aliases.items():v.require(alias not in self.ids or self.ids[alias]==arch,'Conflicting independently derived root');self.ids[alias]=arch;self.platforms[arch]['official_base_ancestry']['manual_decompressed_layer_hashing_performed']=True
 def open_blob(self,request,timeout=60):return RangedBlobReader(self,request,timeout)
 def record_blob_failure(self,error):
  state={};trace=error.__traceback__
  while trace is not None:
   frame=trace.tb_frame
   if frame.f_code is self.v.BlobReceipts.pull.__code__ and frame.f_locals.get('self') is self.v2:
    digest=frame.f_locals.get('digest');size=frame.f_locals.get('size');total=frame.f_locals.get('total');hashed=frame.f_locals.get('hashed')
    if isinstance(digest,str) and re.fullmatch(r'sha256:[a-f0-9]{64}',digest):state['digest']=digest
    if type(size) is int and size>=0:state['expected_size']=size
    if type(total) is int and total>=0:state['total']=total
    if type(hashed) is type(hashlib.sha256()):state['observed_sha256']='sha256:'+hashed.hexdigest()
    break
   trace=trace.tb_next
  classification='unclassified';integrity_messages={'blob returned more bytes than its immutable descriptor','anonymous blob bytes differ from descriptor digest/size'}
  if type(error) is RuntimeError and str(error) in integrity_messages and {'digest','expected_size','total','observed_sha256'}<=state.keys():
   if state['total']<state['expected_size']:classification='short-body'
   elif state['total']>state['expected_size']:classification='overlong-body'
   elif state['observed_sha256']!=state['digest']:classification='full-size-sha-mismatch'
  event={'at':self.v.now(),'candidate_approved':False,'error_type':type(error).__name__,'classification':classification,'canonical_pull_state':state,'scope':'Whitelisted canonical pull traceback state only. The hash is the canonical incremental state: an overlong chunk is counted before hashing. No retry, cache receipt or approval is created by this diagnostic.'}
  fd,path=tempfile.mkstemp(prefix='anonymous-blob-failure-',suffix='.json',dir=self.out);os.close(fd);self.v.dump(Path(path),event)
  self.progress('anonymous-blob-read-failure',diagnostic_path=path,classification=classification,error_type=event['error_type'])
 def pull_blobs(self):
  def progress(stage,**fields):
   require_capacity(self.out,2*self.v.MAX_DOCUMENT,LAYER_FREE_RESERVE);self.progress(stage,**fields)
  def pull(desc):
   require_capacity(self.out,2*self.v.MAX_DOCUMENT,LAYER_FREE_RESERVE);url,headers=self.public()
   try:result=self.v2.pull(url,headers,desc,progress)
   except BaseException as error:
    try:self.record_blob_failure(error)
    except Exception as diagnostic_error:print(json.dumps({'stage':'blob-failure-diagnostic-write-failed','candidate_approved':False,'error_type':type(diagnostic_error).__name__}),flush=True)
    raise
   require_capacity(self.out,2*self.v.MAX_DOCUMENT,LAYER_FREE_RESERVE);return result
  with concurrent.futures.ThreadPoolExecutor(max_workers=self.a.layer_workers) as pool:
   futures=[pool.submit(pull,desc) for desc in self.blobs.values()]
   try:completed={future:future.result() for future in concurrent.futures.as_completed(futures)}
   except BaseException:
    for future in futures:future.cancel()
    raise
  return [completed[future] for future in futures]
 def finish(self):
  v=self.v;self.progress('independent-signed-report-bindings');v.require(all(hashlib.sha256(Path(p).read_bytes()).hexdigest()==h for p,h in self.tool_hashes.items()),'Executed review source changed during this review');require_capacity(self.out,256<<20);sboms={};vulns={};signed=[];image_count=0
  for row,b,st,raw in self.signed_statements():
   v.require(any(x.get('digest',{}).get('sha256')==self.digest.split(':')[1] for x in st.get('subject',[])),'Signed subject differs');pt=row['predicate_type'];predicate=st['predicate'];canonical=json.dumps(predicate,sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode()
   if pt==SIGN:kind='image-signature';arch=None;image_count+=1
   elif pt==v.SPDX_TYPE:kind='sbom';arch,summary=v.spdx_binding(predicate,self.ids);sboms.setdefault(arch,[]).append(summary)
   elif pt==v.TRIVY_TYPE:kind='vulnerability';arch,summary=v.trivy_binding(predicate,self.ids,self.platforms,self.ref);vulns.setdefault(arch,[]).append(summary)
   else:raise RuntimeError('Unexpected accepted predicate')
   name=kind+'-'+str(arch)+'-'+row['bundle_digest'].split(':')[1][:12]+'.predicate.json.gz';p=self.out/name
   with gzip.open(p,'wb') as file:file.write(canonical)
   vm=b['verificationMaterial'];record=dict(row,kind=kind,architecture=arch,decoded_predicate_sorted_json_sha256=hashlib.sha256(canonical).hexdigest(),decoded_predicate_sorted_json_bytes=len(canonical),full_predicate_gzip=str(p),full_predicate_gzip_sha256=hashlib.sha256(p.read_bytes()).hexdigest(),tlog_kind_versions=[x['kindVersion'] for x in vm['tlogEntries']],tsa_timestamp_count=len(vm['timestampVerificationData']['rfc3161Timestamps']));signed.append(record)
  v.require(image_count>=1 and set(sboms)==set(vulns)=={'amd64','arm64'},'Both actual signed architecture report pairs are required')
  self.progress('all-reports-bound-current-anonymous-image-blobs');pulls=self.pull_blobs()
  for row in signed:
   p=Path(row['path']);z=p.with_name(p.name+'.gz')
   with p.open('rb') as src,gzip.open(z,'wb') as dst:shutil.copyfileobj(src,dst,1<<20)
   row['full_bundle_gzip']=str(z);row['full_bundle_gzip_sha256']=hashlib.sha256(z.read_bytes()).hexdigest();row['full_bundle_gzip_bytes']=z.stat().st_size
  receipt={'reviewed_at':v.now(),'approved_supplier_candidate':True,'official_base':self.base,'ref':self.ref,'tag':'ghcr.io/'+v.REPOSITORY+':'+self.tag,'verification_workflow_sha':self.a.sha,'producer_workflow_sha':self.provenance['producer_sha'],'producer_verification_provenance':self.provenance,'ci_runtime_scan_signing_evidence':self.ci,'platforms':{a:{k:x for k,x in p.items() if k!='config'} for a,p in self.platforms.items()},'anonymous_manifest_bytes_sha_verified':True,'anonymous_pull':{'all_unique_config_and_layer_bytes_fully_hashed':True,'unique_blob_count':len(pulls),'bytes_hashed_this_finalization':sum(x['bytes_hashed'] for x in pulls if not x['reused_prior_full_read']),'bytes_reused_from_prior_verified_full_read':sum(x['bytes_hashed'] for x in pulls if x['reused_prior_full_read']),'prior_receipts_also_rechecked_by_anonymous_head':True,'blob_receipts':pulls},'new_manual_layer_reads':len(self.new_reads),'spdx_additional_verified_root_bindings':self.bindings,'strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified':True,'strict_actual_Rekor_v2_and_RFC3161_TSA_verified':True,'wrong_source_and_subject_negative_checks_rejected':True,'signed_payloads':signed,'package_summaries_by_architecture':sboms,'trivy_summaries_by_architecture':vulns,'bundle_discovery':self.discovery,'unaccepted_attachment_results':self.rejected,'verification_method':'Immutable manifest/blob SHA and size verification; official Cosign3.1.4 verify-blob-attestation with exact source/issuer/workflow identity/subject/tlog/explicit TSA. No ignore flags. Canonical strict per-architecture image/report binders and actual decompressed-byte proof where necessary.','scope':'Published supplier identity/runtime/report evidence only. All findings retained; no zero-CVE claim or security waiver. Complete catalog adoption, generated exact-image replay and original hosted acceptance remain separate. No production pin/resource changed.','evidence_tool_sha256':self.tool_hashes,'scratch_capacity_policy':{'reserve_bytes':DISK_RESERVE,'start_minimum_free_bytes':START_FREE,'bundle_chunk_size':1<<20,'owned_partial_download_cleanup_on_error':True},'transport_read_policy':{'maximum_attempts':3,'retry_only':'TLS/network/timeout/OSError and HTTP429/5xx; not404,signature,subject,source,hash or report policy errors','TLS_verification':'unchanged enabled','Cosign_TUF_cache':'A unique owned TUF_ROOT per invocation; default production trust/mirror/current refresh and strict verification flags are unchanged.','full_blob_failure_diagnostics':'Whitelisted canonical-pull digest/expected_size/total/observed_sha256 and error class only; no added retries or acceptance.','full_blob_range_resume':{'maximum_body_GET_attempts':3,'resume_only':'PrematureEOF or exception inside response.read; no HTTP/header/hash-policy retries','Range200':'Failclosed','206':'Exact offset,end,total and Content-Length of remaining descriptor','headers':'Fresh same public repository scope for each body GET; no tokens/URLs retained','storage':'Stream/discard, bounded1MiB chunks; unchanged canonical wholeSHA+size and receipt gate'},'full_anonymous_blob_read_policy':{'workers':self.a.layer_workers,'ordered_complete_receipts':True,'minimum_free_reserve':LAYER_FREE_RESERVE,'cancel_only_unstarted_own_futures_after_first_failure':True},'own_layer_file_policy':{'maximum_case_bytes':LAYER_FILE_BUDGET,'minimum_free_reserve':LAYER_FREE_RESERVE,'full_SHA_size_before_decompression':True,'retain_bounded_transport_partials':True,'cancel_only_unstarted_own_futures_after_first_failure':True},'Cosign_positive_transport_retry':'at most3 identical strict commands, only observed TUF-network timeout/reset errors; each raw result/log retained; policy failures never retried or accepted','anonymous_401_refresh':'same exact public registry/repository only; one metadata refresh or at most2 bounded blob refreshes; no user credentials or private scopes','bundle_HTTP_Range_resume':'at most3 end-to-end attempts; exact Content-Range offset/total required, server200 restarts, final complete descriptor SHA/size verified before parsing'}}
  v.dump(self.out/'receipt.json',receipt);v.dump(self.out/'progress.json',{'at':v.now(),'stage':'complete','candidate_approved':True,'ref':self.ref});print(json.dumps({'candidate_approved':True,'name':self.a.name,'ref':self.ref,'receipt':str(self.out/'receipt.json')}),flush=True)
 def run(self):
  self.metadata();self.fetch_bundles();self.crypto_bundles();self.derive_docker_roots_if_needed();self.finish()

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--name',required=True);p.add_argument('--sha',required=True);p.add_argument('--run',required=True);p.add_argument('--repo-root',default='/Users/tianpan/projects/bex7');p.add_argument('--policy-module',default='/tmp/loopx-w8-verify-native-artifact-ranged.py');p.add_argument('--output-root',default='/tmp/loopx-w8-native-supplier-reviews');p.add_argument('--cache-root',default='/tmp/loopx-w8-native-review-cache');p.add_argument('--layer-workers',type=int,default=2);args=p.parse_args();p.error('layer-workers must be1–4') if not 1<=args.layer_workers<=4 else None;review=None
 try:review=Review(args);review.run()
 except Exception as error:
  if review is not None:review.v.dump(review.out/'failed-review.json',{'at':review.v.now(),'candidate_approved':False,'error_type':type(error).__name__,'error':safe_error(error)})
  raise
if __name__=='__main__':main()
