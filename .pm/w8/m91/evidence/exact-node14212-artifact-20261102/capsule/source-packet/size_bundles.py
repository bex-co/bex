"""Approved metadata-only exact supplier bundle descriptor sizing; no payloads."""
from pathlib import Path
import hashlib,importlib.util,json,time,urllib.error,shutil
HERE=Path(__file__).resolve().parent
REPO=Path('/Users/tianpan/projects/bex7')
spec=importlib.util.spec_from_file_location('fixed_exact',HERE/'exact-node-v11-adapter.py');x=importlib.util.module_from_spec(spec);spec.loader.exec_module(x)
plan=json.loads((HERE/'PLAN-ACTUAL.json').read_bytes());x.validate_plan(plan,REPO,executing=True)
driver=x.import_pinned('loopx-w8-native-review-v11.py');policy=driver.load('native_policy',HERE/'retained/loopx-w8-verify-native-artifact-ranged.py');policy.ROOT=REPO
freshness=driver.load('native_freshness',REPO/'scripts/lib/toolchain-freshness.py')
OUT=HERE/'bundle-sizing';assert not OUT.exists();OUT.mkdir(mode=0o700);began=time.monotonic()
def sha(b):return hashlib.sha256(b).hexdigest()
def save(name,b):
    assert time.monotonic()-began<210 and sum(p.stat().st_size for p in OUT.iterdir() if p.is_file())+len(b)<=32<<20
    assert shutil.disk_usage(OUT).free>(1<<30)+(128<<20)+len(b)
    p=OUT/name
    with p.open('xb') as f:assert f.write(b)==len(b)
def put(name,v):save(name,(json.dumps(v,indent=2)+'\n').encode())
url,headers=policy.public_registry(freshness,'ghcr.io',policy.REPOSITORY);subject=plan['ref'].split('@')[1]
try:
    raw,_=policy.small_get(url+'/referrers/'+subject,headers,policy.MAX_DOCUMENT);index=json.loads(raw);method='native-referrers'
except urllib.error.HTTPError as error:
    if error.code!=404:raise
    index,raw,_=policy.manifest_document(url,headers,subject.replace(':','-'));method='sha-verified-OCI-fallback-index'
save('referrer-index.json',raw)
entries=[r for r in index.get('manifests',[]) if r.get('artifactType')=='application/vnd.dev.sigstore.bundle.v0.3+json' and r.get('annotations',{}).get('dev.sigstore.bundle.predicateType') in driver.TYPES]
assert 0<len(entries)<=64
bundles={};records=[]
for row in entries:
    bundle,braw,bd=policy.manifest_document(url,headers,row['digest'])
    assert len(braw)==row['size'] and bundle.get('subject',{}).get('digest')==subject and len(bundle.get('layers',[]))==1
    save(bd.split(':')[1]+'.manifest.json',braw)
    key,n=policy.descriptor(bundle['layers'][0]);assert n<=policy.MAX_ATTESTATIONS and (key not in bundles or bundles[key]==n)
    bundles[key]=n;records.append({'manifest_digest':bd,'bundle_digest':key,'bytes':n,'predicate_type':row['annotations']['dev.sigstore.bundle.predicateType']})
put('bundles.json',{'ref':plan['ref'],'candidate_approved':False,'referrer_index_sha256':sha(raw),'bundle_descriptors':records,'unique_bundle_payload_bytes':sum(bundles.values()),'largest_bundle_payload_bytes':max(bundles.values()),'method':'Same retained policy SHA/size checked referrer+bundle manifests only; no bundle/report payload read','discovery':method})
print(json.dumps({'metadata_only':True,'bundles':len(records),'bytes':sum(bundles.values()),'largest':max(bundles.values()),'raw_documents':1+len(records),'candidate_approved':False}))

