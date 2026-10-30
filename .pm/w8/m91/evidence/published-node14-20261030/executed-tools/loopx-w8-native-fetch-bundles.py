from pathlib import Path
import importlib.util,json,hashlib,urllib.request,concurrent.futures,time
s=importlib.util.spec_from_file_location('v','/tmp/loopx-w8-verify-native-artifact.py');v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
s=importlib.util.spec_from_file_location('f','/Users/tianpan/projects/bex7/scripts/lib/toolchain-freshness.py');f=importlib.util.module_from_spec(s);s.loader.exec_module(f)
root=Path('/tmp/loopx-w8-native-modern-review/direct-bundle-proof');root.mkdir(exist_ok=True);index=json.loads(Path('/tmp/loopx-w8-native-modern-review/referrer-diagnosis/fallback-index.json').read_text());subject='sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc'
entries=[d for d in index['manifests'] if d.get('artifactType')=='application/vnd.dev.sigstore.bundle.v0.3+json']
def fetch(desc):
 url,h=v.public_registry(f,'ghcr.io',v.REPOSITORY);m,raw,digest=v.manifest_document(url,h,desc['digest']);v.require(len(raw)==desc['size'],'bundlemanifest size differs');v.require(m.get('subject',{}).get('digest')==subject,'bundlemanifest unrelatedsubject');v.require(len(m['layers'])==1,'bundlemanifest musthaveonepayload')
 (root/(digest.split(':')[1]+'.manifest.json')).write_bytes(raw);layer=m['layers'][0];ld,size=v.descriptor(layer);v.require(size<=v.MAX_ATTESTATIONS,'bundle exceedsbound');path=root/(ld.split(':')[1]+'.bundle.json');sha=hashlib.sha256();n=0
 with urllib.request.urlopen(urllib.request.Request(url+'/blobs/'+ld,headers=h),timeout=60) as response,path.open('wb') as out:
  while True:
   b=response.read(1<<20)
   if not b:break
   n+=len(b);v.require(n<=size,'bundle exceededdescriptor');sha.update(b);out.write(b)
 v.require(n==size and 'sha256:'+sha.hexdigest()==ld,'bundleblobbytesdiffer');bundle=json.loads(path.read_text());envelope=bundle['dsseEnvelope'];statement=json.loads(__import__('base64').b64decode(envelope['payload']));pt=statement['predicateType'];v.require(pt==desc['annotations']['dev.sigstore.bundle.predicateType'],'predicateannotationdiffersfromsignedpayload')
 vm=bundle['verificationMaterial'];tl=vm.get('tlogEntries',[]);ts=vm.get('timestampVerificationData',{}).get('rfc3161Timestamps',[])
 record={'manifest_digest':digest,'bundle_digest':ld,'bundle_bytes':n,'path':str(path),'predicate_type':pt,'subject':statement['subject'],'tlog_kind_versions':[x.get('kindVersion') for x in tl],'tsa_timestamp_count':len(ts),'statement_sha256':hashlib.sha256(__import__('base64').b64decode(envelope['payload'])).hexdigest()};print(json.dumps(record),flush=True);return record
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:records=list(pool.map(fetch,entries))
v.dump(root/'download-receipt.json',{'scope':'Actual immutableSHA/sizeverifiedpublicbundlefiles only; no crypto/source approval claimeduntilofficialstrictverifierpasses','candidate_approved':False,'subject':subject,'bundles':records})
