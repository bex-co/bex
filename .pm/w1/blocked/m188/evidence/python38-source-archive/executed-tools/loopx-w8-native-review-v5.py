#!/usr/bin/env python3
"""Read-only, native-supplier-only review of actual green jobs and immutable signed artifacts."""
import argparse,base64,concurrent.futures,fcntl,gzip,hashlib,importlib.util,json,os,re,shutil,subprocess,tempfile,threading,time,urllib.error,urllib.request
from pathlib import Path
import http.client

SIGN='https://sigstore.dev/cosign/sign/v1'
TYPES={SIGN,'https://spdx.dev/Document','https://aquasecurity.github.io/trivy/attestation/vuln/v1'}
MAX_UNCOMPRESSED=64<<30
DISK_RESERVE=512<<20
START_FREE=1<<30

def require_capacity(path,additional=0):
 free=shutil.disk_usage(path).free
 if free<DISK_RESERVE+additional:raise RuntimeError("Scratch disk reserve would be exhausted: free="+str(free)+" required="+str(DISK_RESERVE+additional))
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

def immutable_download(url,headers,path,digest,size,capacity_path,progress):
 tmp=path.with_name(path.name+'.tmp-'+str(os.getpid()));last=time.monotonic()
 try:
  for attempt in range(3):
   offset=tmp.stat().st_size if tmp.exists() else 0
   if offset>size:raise RuntimeError('Partial immutable blob exceeds descriptor')
   if offset==size:break
   require_capacity(capacity_path,size-offset);request_headers=dict(headers)
   if offset:request_headers['Range']='bytes='+str(offset)+'-'
   try:
    with urllib.request.urlopen(urllib.request.Request(url,headers=request_headers),timeout=60) as response:
     if response.status==206:
      match=re.fullmatch(r'bytes (\d+)-(\d+)/(\d+)',response.headers.get('Content-Range',''))
      if not match or int(match[1])!=offset or int(match[3])!=size or not offset<=int(match[2])<size:raise RuntimeError('Immutable blob Content-Range differs from requested offset/descriptor')
     elif response.status==200:
      offset=0;require_capacity(capacity_path,size)
     else:raise RuntimeError('Unexpected immutable blob HTTP status')
     with tmp.open('ab' if offset else 'wb') as file:
      while True:
       chunk=response.read(1<<20)
       if not chunk:break
       if file.tell()+len(chunk)>size or (response.status==206 and file.tell()+len(chunk)>int(match[2])+1):raise RuntimeError('Bundle exceeded descriptor or declared Content-Range')
       require_capacity(capacity_path,len(chunk));file.write(chunk)
       if time.monotonic()-last>30:progress('immutable-bundle-byte-progress',digest=digest,downloaded_bytes=file.tell(),expected_bytes=size,attempt=attempt+1);last=time.monotonic()
    if tmp.stat().st_size==size:break
    raise urllib.error.URLError('Immutable blob response ended before descriptor size')
   except urllib.error.HTTPError as error:
    if error.code!=429 and error.code<500:raise
    if attempt==2:raise
   except (urllib.error.URLError,TimeoutError,ConnectionError,OSError,http.client.HTTPException):
    if attempt==2:raise
   progress('immutable-bundle-bounded-resume',digest=digest,downloaded_bytes=tmp.stat().st_size if tmp.exists() else 0,attempt=attempt+1);time.sleep(2*(attempt+1))
  h=hashlib.sha256();n=0
  with tmp.open('rb') as file:
   while chunk:=file.read(1<<20):h.update(chunk);n+=len(chunk)
  if n!=size or 'sha256:'+h.hexdigest()!=digest:raise RuntimeError('Complete immutable bundle SHA/size differs')
  tmp.replace(path)
 except BaseException:
  tmp.unlink(missing_ok=True);raise

class Counted:
 def __init__(self,response,bound):self.response=response;self.bound=bound;self.n=0;self.sha=hashlib.sha256()
 def read(self,size=-1):
  data=self.response.read(size);self.n+=len(data)
  if self.n>self.bound:raise RuntimeError('Compressed descriptor exceeded')
  self.sha.update(data);return data

class Review:
 def __init__(self,args):
  self.a=args;self.v=load('native_policy',args.policy_module);self.v.ROOT=Path(args.repo_root).resolve();self.v.require('FRESHNESS_DIGESTS' not in os.environ,'Fixture digest injection forbidden')
  raw_small_get=self.v.small_get;self.v.small_get=lambda *a,**kw:retry_read(lambda:raw_small_get(*a,**kw))
  self.v.require(re.fullmatch(r'[a-f0-9]{40}',args.sha) and re.fullmatch(r'[a-z0-9.-]+',args.name) and args.run.isdecimal(),'Invalid source/name/run identity')
  self.out=Path(args.output_root).resolve()/('native-'+args.name+'-'+args.sha);self.v.require(self.v.ROOT not in self.out.parents,'Full evidence must stay outside the repository');self.out.mkdir(parents=True,exist_ok=True)
  require_capacity(self.out,START_FREE-DISK_RESERVE);self.tool_hashes={str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in [Path(__file__),Path(args.policy_module)]};executed=self.out/"executed-tools";executed.mkdir(exist_ok=True)
  for p in [Path(__file__),Path(args.policy_module)]:shutil.copyfile(p,executed/p.name)
  self.cache=Path(args.cache_root).resolve();self.cache.mkdir(parents=True,exist_ok=True);self.layer_cache=self.cache/'verified-uncompressed';self.layer_cache.mkdir(exist_ok=True);self.v2=self.v.BlobReceipts(self.cache/'anonymous-compressed-v2');self.lock=threading.Lock();self.cache_lock=threading.Lock();self.platforms={};self.manifests={};self.ids={};self.blobs={};self.crypto=[];self.new_reads=set()
  self.f=load('native_freshness',self.v.ROOT/'scripts/lib/toolchain-freshness.py');self.base=self.v.matrix_base(args.sha,args.name)
  self.v.require(hashlib.sha256(self.v.COSIGN.read_bytes()).hexdigest()==self.v.COSIGN_SHA256,'Verified Cosign binary changed')
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
   immutable_download(url+'/blobs/'+bd,headers,path,bd,size,self.out,self.progress)
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
  v=self.v;stem=row['bundle_digest'].split(':')[1]+'-'+label;args=[str(v.COSIGN),'verify-blob-attestation','--bundle',row['path'],'--type',row['predicate_type'],'--digest',(digest or self.digest).split(':')[1],'--digestAlg','sha256','--certificate-identity','https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main','--certificate-oidc-issuer','https://token.actions.githubusercontent.com','--certificate-github-workflow-sha',source_sha or self.a.sha,'--use-signed-timestamps','--timeout','5m']
  with (self.bundle_dir/(stem+'.stdout.log')).open('wb') as stdout,(self.bundle_dir/(stem+'.stderr.log')).open('wb') as stderr:p=subprocess.run(args,env=dict(os.environ,GODEBUG='http2client=0'),stdout=stdout,stderr=stderr,timeout=330)
  result={'bundle_digest':row['bundle_digest'],'exit_code':p.returncode,'command':args,'stderr':str(self.bundle_dir/(stem+'.stderr.log'))};v.dump(self.bundle_dir/(stem+'.result.json'),result);return result
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
 def layer(self,task):
  arch,i,desc,diff=task;v=self.v;digest,size=v.descriptor(desc);path=self.layer_cache/(digest.split(':')[1]+'.json')
  with (self.layer_cache/(digest.split(':')[1]+'.lock')).open('a+') as lock:
   fcntl.flock(lock,fcntl.LOCK_EX)
   if path.exists():
    r=json.loads(path.read_text());v.require(r.get('schema')=='bex.native-compressed-and-diffid-proof/v1' and r.get('registry_repository')=='ghcr.io/'+v.REPOSITORY and r.get('compressed_digest')==digest and r.get('compressed_bytes')==size and r.get('uncompressed_digest')==diff and r.get('manual_decompressed_layer_hashing_performed') is True and type(r.get('uncompressed_bytes')) is int and 0<=r['uncompressed_bytes']<=MAX_UNCOMPRESSED,'Invalid prior decompressed proof');self.install_compressed_receipt(r);return arch,i,r
   self.progress('anonymous-compressed-and-decompressed-read',architecture=arch,layer=i,expected_compressed_bytes=size);url,headers=self.public();h=hashlib.sha256();n=0;last=time.monotonic()
   with read_open(urllib.request.Request(url+'/blobs/'+digest,headers=headers),timeout=60) as response:
    reader=Counted(response,size);media=desc['mediaType']
    if media.endswith('+gzip') or media.endswith('.gzip'):body=gzip.GzipFile(fileobj=reader,mode='rb')
    elif media in ('application/vnd.oci.image.layer.v1.tar','application/vnd.docker.image.rootfs.diff.tar'):body=reader
    else:raise RuntimeError('Unsupported layer compression for strict proof: '+media)
    while True:
     data=body.read(1<<20)
     if not data:break
     n+=len(data);v.require(n<=MAX_UNCOMPRESSED,'Decompressed layer exceeds bound');h.update(data)
     if time.monotonic()-last>30:self.progress('layer-read-progress',architecture=arch,layer=i,compressed_bytes=reader.n,decompressed_bytes=n);last=time.monotonic()
    if body is not reader:body.close()
    v.require(reader.n==size and 'sha256:'+reader.sha.hexdigest()==digest,'Compressed descriptor SHA/size differs')
   v.require('sha256:'+h.hexdigest()==diff,'Actually decompressed diffID differs from verified config');r={'schema':'bex.native-compressed-and-diffid-proof/v1','registry_repository':'ghcr.io/'+v.REPOSITORY,'compressed_digest':digest,'compressed_bytes':size,'uncompressed_digest':diff,'uncompressed_bytes':n,'anonymous_full_get_at':v.now(),'manual_decompressed_layer_hashing_performed':True};v.dump(path,r);self.install_compressed_receipt(r)
   with self.lock:self.new_reads.add(digest)
   return arch,i,r
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
  tasks.sort(key=lambda t:t[2]['size'],reverse=True)
  with concurrent.futures.ThreadPoolExecutor(max_workers=self.a.layer_workers) as pool:items=list(pool.map(self.layer,tasks))
  bykey={(a,i):r for a,i,r in items};rows=[]
  for arch,m in self.manifests.items():
   layers=[bykey[(arch,i)] for i in range(len(m['layers']))]
   for r in layers:shutil.copyfile(self.layer_cache/(r['compressed_digest'].split(':')[1]+'.json'),proof/'verified-layers'/(r['compressed_digest'].split(':')[1]+'.json'))
   synthetic={'schemaVersion':2,'mediaType':'application/vnd.docker.distribution.manifest.v2+json','config':{'mediaType':'application/vnd.docker.container.image.v1+json','size':m['config']['size'],'digest':self.platforms[arch]['config_digest']},'layers':[{'mediaType':'application/vnd.docker.image.rootfs.diff.tar.gzip','size':r['uncompressed_bytes'],'digest':r['uncompressed_digest']} for r in layers]};raw=json.dumps(synthetic,separators=(',',':'),ensure_ascii=False).encode();alias='sha256:'+hashlib.sha256(raw).hexdigest();(proof/(arch+'-derived-docker-manifest.json')).write_bytes(raw);rows.append({'architecture':arch,'actual_registry_child_digest':self.platforms[arch]['manifest_digest'],'actual_registry_config_digest':self.platforms[arch]['config_digest'],'derived_docker_manifest_digest':alias,'manual_decompressed_layer_hashing_performed':True,'layer_proofs':layers})
  v.dump(proof/'receipt.json',{'index_digest':self.digest,'candidate_approved':False,'architectures':rows});aliases,self.bindings=v.docker_source_root_aliases(proof,self.digest,self.platforms,self.manifests)
  for alias,arch in aliases.items():v.require(alias not in self.ids or self.ids[alias]==arch,'Conflicting independently derived root');self.ids[alias]=arch;self.platforms[arch]['official_base_ancestry']['manual_decompressed_layer_hashing_performed']=True
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
  self.progress('all-reports-bound-current-anonymous-image-blobs');url,headers=self.public();pulls=[self.v2.pull(url,headers,d,lambda stage,**kw:self.progress(stage,**kw)) for d in self.blobs.values()]
  for row in signed:
   p=Path(row['path']);z=p.with_name(p.name+'.gz')
   with p.open('rb') as src,gzip.open(z,'wb') as dst:shutil.copyfileobj(src,dst,1<<20)
   row['full_bundle_gzip']=str(z);row['full_bundle_gzip_sha256']=hashlib.sha256(z.read_bytes()).hexdigest();row['full_bundle_gzip_bytes']=z.stat().st_size
  receipt={'reviewed_at':v.now(),'approved_supplier_candidate':True,'official_base':self.base,'ref':self.ref,'tag':'ghcr.io/'+v.REPOSITORY+':'+self.tag,'verification_workflow_sha':self.a.sha,'producer_workflow_sha':self.provenance['producer_sha'],'producer_verification_provenance':self.provenance,'ci_runtime_scan_signing_evidence':self.ci,'platforms':{a:{k:x for k,x in p.items() if k!='config'} for a,p in self.platforms.items()},'anonymous_manifest_bytes_sha_verified':True,'anonymous_pull':{'all_unique_config_and_layer_bytes_fully_hashed':True,'unique_blob_count':len(pulls),'bytes_hashed_this_finalization':sum(x['bytes_hashed'] for x in pulls if not x['reused_prior_full_read']),'bytes_reused_from_prior_verified_full_read':sum(x['bytes_hashed'] for x in pulls if x['reused_prior_full_read']),'prior_receipts_also_rechecked_by_anonymous_head':True,'blob_receipts':pulls},'new_manual_layer_reads':len(self.new_reads),'spdx_additional_verified_root_bindings':self.bindings,'strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified':True,'strict_actual_Rekor_v2_and_RFC3161_TSA_verified':True,'wrong_source_and_subject_negative_checks_rejected':True,'signed_payloads':signed,'package_summaries_by_architecture':sboms,'trivy_summaries_by_architecture':vulns,'bundle_discovery':self.discovery,'unaccepted_attachment_results':self.rejected,'verification_method':'Immutable manifest/blob SHA and size verification; official Cosign3.1.4 verify-blob-attestation with exact source/issuer/workflow identity/subject/tlog/explicit TSA. No ignore flags. Canonical strict per-architecture image/report binders and actual decompressed-byte proof where necessary.','scope':'Published supplier identity/runtime/report evidence only. All findings retained; no zero-CVE claim or security waiver. Complete catalog adoption, generated exact-image replay and original hosted acceptance remain separate. No production pin/resource changed.','evidence_tool_sha256':self.tool_hashes,'scratch_capacity_policy':{'reserve_bytes':DISK_RESERVE,'start_minimum_free_bytes':START_FREE,'bundle_chunk_size':1<<20,'owned_partial_download_cleanup_on_error':True},'transport_read_policy':{'maximum_attempts':3,'retry_only':'TLS/network/timeout/OSError and HTTP429/5xx; not404,signature,subject,source,hash or report policy errors','TLS_verification':'unchanged enabled','bundle_HTTP_Range_resume':'at most3 end-to-end attempts; exact Content-Range offset/total required, server200 restarts, final complete descriptor SHA/size verified before parsing'}}
  v.dump(self.out/'receipt.json',receipt);v.dump(self.out/'progress.json',{'at':v.now(),'stage':'complete','candidate_approved':True,'ref':self.ref});print(json.dumps({'candidate_approved':True,'name':self.a.name,'ref':self.ref,'receipt':str(self.out/'receipt.json')}),flush=True)
 def run(self):
  self.metadata();self.fetch_bundles();self.crypto_bundles();self.derive_docker_roots_if_needed();self.finish()

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--name',required=True);p.add_argument('--sha',required=True);p.add_argument('--run',required=True);p.add_argument('--repo-root',default='/Users/tianpan/projects/bex7');p.add_argument('--policy-module',default='/tmp/loopx-w8-verify-native-artifact.py');p.add_argument('--output-root',default='/tmp/loopx-w8-native-supplier-reviews');p.add_argument('--cache-root',default='/tmp/loopx-w8-native-review-cache');p.add_argument('--layer-workers',type=int,default=2);args=p.parse_args();p.error('layer-workers must be1–4') if not 1<=args.layer_workers<=4 else None;review=None
 try:review=Review(args);review.run()
 except Exception as error:
  if review is not None:review.v.dump(review.out/'failed-review.json',{'at':review.v.now(),'candidate_approved':False,'error_type':type(error).__name__,'error':safe_error(error)})
  raise
if __name__=='__main__':main()
