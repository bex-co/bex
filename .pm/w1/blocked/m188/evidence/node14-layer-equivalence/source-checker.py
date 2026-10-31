#!/usr/bin/env python3
"""Bounded real anonymous compressed+decompressed byte proof of Stereoscope Docker root digests."""
from pathlib import Path
import importlib.util,json,hashlib,gzip,urllib.request,datetime,time,fcntl,concurrent.futures
s=importlib.util.spec_from_file_location('v','/tmp/loopx-w8-verify-native-artifact.py');v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
s=importlib.util.spec_from_file_location('f','/Users/tianpan/projects/bex7/scripts/lib/toolchain-freshness.py');f=importlib.util.module_from_spec(s);s.loader.exec_module(f)
root=Path('/tmp/loopx-w8-native-modern-review/docker-root-proof');root.mkdir(exist_ok=True);cache=root/'verified-layers';cache.mkdir(exist_ok=True);v2=v.BlobReceipts(v.REVIEWS/'.public-blob-receipts-v2')
index_digest='sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc';MAX_UNCOMPRESSED=64<<30

def emit(stage,**kw):print(json.dumps({'at':v.now(),'stage':stage,**kw}),flush=True)
class Counted:
 def __init__(self,response,expected):self.response=response;self.expected=expected;self.n=0;self.sha=hashlib.sha256()
 def read(self,n=-1):
  d=self.response.read(n);self.n+=len(d);v.require(self.n<=self.expected,'compressed descriptor size exceeded');self.sha.update(d);return d

def layer(desc,diff,arch,ordinal):
 digest,size=v.descriptor(desc);path=cache/(digest.split(':')[1]+'.json')
 with (cache/(digest.split(':')[1]+'.lock')).open('a+') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX)
  if path.exists():
   r=json.loads(path.read_text());v.require(r['compressed_digest']==digest and r['compressed_bytes']==size and r['uncompressed_digest']==diff,'cached layer proof mismatch');return r
  url,headers=v.public_registry(f,'ghcr.io',v.REPOSITORY);emit('anonymous-decompressed-layer-read',arch=arch,layer=ordinal,compressed_digest=digest,expected_compressed_bytes=size)
  h=hashlib.sha256();total=0;last=time.monotonic()
  with urllib.request.urlopen(urllib.request.Request(url+'/blobs/'+digest,headers=headers),timeout=60) as response:
   reader=Counted(response,size)
   media=desc['mediaType']
   if media.endswith('+gzip') or media.endswith('.gzip'):body=gzip.GzipFile(fileobj=reader,mode='rb')
   elif media in ('application/vnd.oci.image.layer.v1.tar','application/vnd.docker.image.rootfs.diff.tar'):body=reader
   else:raise RuntimeError('unsupported layer compression for bounded proof: '+media)
   while True:
    data=body.read(1<<20)
    if not data:break
    total+=len(data);v.require(total<=MAX_UNCOMPRESSED,'uncompressed layer exceeds bound');h.update(data)
    if time.monotonic()-last>30:emit('layer-read-progress',arch=arch,layer=ordinal,compressed_bytes=reader.n,uncompressed_bytes=total);last=time.monotonic()
   if body is not reader:body.close()
   v.require(reader.n==size and 'sha256:'+reader.sha.hexdigest()==digest,'compressed layer SHA/size differs from immutable descriptor')
  v.require('sha256:'+h.hexdigest()==diff,'actual decompressed layer SHA differs from verified config diffID')
  r={'schema':'bex.native-compressed-and-diffid-proof/v1','registry_repository':'ghcr.io/'+v.REPOSITORY,'compressed_digest':digest,'compressed_bytes':size,'uncompressed_digest':diff,'uncompressed_bytes':total,'anonymous_full_get_at':v.now(),'manual_decompressed_layer_hashing_performed':True};v.dump(path,r)
  with v2.locked_cache():
   data=v2.read();data[digest]={'schema':v.CACHE_SCHEMA,'registry_repository':'ghcr.io/'+v.REPOSITORY,'digest':digest,'bytes_hashed':size,'anonymous_full_get_at':r['anonymous_full_get_at']};v.dump(v2.data,{'schema':v.CACHE_SCHEMA,'blobs':data})
  emit('layer-read-verified',arch=arch,layer=ordinal,compressed_bytes=size,uncompressed_bytes=total);return r

url,headers=v.public_registry(f,'ghcr.io',v.REPOSITORY);index,raw,digest=v.manifest_document(url,headers,index_digest);(root/'index.json').write_bytes(raw);children=v.image_platforms(index)
def prove(item):
 arch,child=item;url,headers=v.public_registry(f,'ghcr.io',v.REPOSITORY);manifest,mraw,_=v.manifest_document(url,headers,child);config,craw=v.config_document(url,headers,manifest['config']);v.require(config['architecture']==arch and config['os']=='linux','platform differs')
 (root/(arch+'-manifest.json')).write_bytes(mraw);(root/(arch+'-config.json')).write_bytes(craw);diffs=config['rootfs']['diff_ids'];v.require(len(diffs)==len(manifest['layers']),'layer/diffID count mismatch')
 proofs=[layer(d,di,arch,i) for i,(d,di) in enumerate(zip(manifest['layers'],diffs))]
 synthetic={'schemaVersion':2,'mediaType':'application/vnd.docker.distribution.manifest.v2+json','config':{'mediaType':'application/vnd.docker.container.image.v1+json','size':len(craw),'digest':'sha256:'+hashlib.sha256(craw).hexdigest()},'layers':[{'mediaType':'application/vnd.docker.image.rootfs.diff.tar.gzip','size':p['uncompressed_bytes'],'digest':p['uncompressed_digest']} for p in proofs]}
 encoded=json.dumps(synthetic,separators=(',',':'),ensure_ascii=False).encode();synth='sha256:'+hashlib.sha256(encoded).hexdigest();(root/(arch+'-derived-docker-manifest.json')).write_bytes(encoded)
 receipt={'scope':'Actual anonymous full compressed descriptor hashes and manually decompressed diffIDs/sizes; exact Stereoscope0.3.1 Docker archive synthetic manifest algorithm, not guessed alias. Candidate approval still requires completed job and all strict signed reports.','index_digest':index_digest,'architecture':arch,'actual_registry_child_digest':child,'actual_registry_config_digest':manifest['config']['digest'],'derived_docker_manifest_digest':synth,'compressed_bytes_hashed':sum(p['compressed_bytes'] for p in proofs),'decompressed_bytes_hashed':sum(p['uncompressed_bytes'] for p in proofs),'layer_count':len(proofs),'manual_decompressed_layer_hashing_performed':True,'layer_proofs':proofs,'candidate_approved':False}
 if arch=='amd64':receipt['matches_actual_first_signed_spdx_root']=synth=='sha256:99158bec34086e055e5f14a60cdb13f612bea6aa34efd12c9ad778cad2c97afa'
 v.dump(root/(arch+'-receipt.json'),receipt);emit('synthetic-manifest-derived',arch=arch,digest=synth,actual_child=child,actual_config=manifest['config']['digest']);return receipt
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:results=list(pool.map(prove,children.items()))
v.dump(root/'receipt.json',{'index_digest':index_digest,'candidate_approved':False,'architectures':results});emit('both-architecture-decompressed-proof-complete',receipt=str(root/'receipt.json'))
