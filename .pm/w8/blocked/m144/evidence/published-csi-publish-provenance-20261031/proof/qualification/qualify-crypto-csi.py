"""Original CSI Cosign policy, fixed to this producer; private proof writes only."""
from pathlib import Path
import base64,gzip,hashlib,json,os,runpy,subprocess,tempfile,re
P=Path(__file__).resolve().parent
SOURCE='dd64e4ff7888828faa1b547f3a637b76d7e7da91'
IDENTITY='https://github.com/bex-co/bex/.github/workflows/hcloud-csi-image.yml@refs/heads/main'
COSIGN=Path('/private/tmp/bex7-native-acceptance-novb586u/cosign')
assert hashlib.sha256(COSIGN.read_bytes()).hexdigest()=='9f9b8cd557cf2e5b3df3373a00baa005a86b02391e3e06dd0ce37b3a1533d3a4'
state=runpy.run_path(str(P/'registry-review.py'));meta=state['receipt'];reference=meta['candidate_ref'];digest=meta['digest']
config=P/'anonymous-cosign-config-csi';config.mkdir(mode=0o700,exist_ok=False);(config/'config.json').write_text('{"auths":{}}\n')
trust=['--certificate-identity='+IDENTITY,'--certificate-oidc-issuer=https://token.actions.githubusercontent.com','--certificate-github-workflow-sha='+SOURCE,'--insecure-ignore-sct=false','--insecure-ignore-tlog=false']
results=[]
for name,verb,extra in [('signature','verify',[]),('sbom','verify-attestation',['--type','spdxjson'])]:
 cacheParent=P/'cosign-tuf-csi';cacheParent.mkdir(mode=0o700,exist_ok=True);cache=Path(tempfile.mkdtemp(prefix=name+'-',dir=cacheParent))
 argv=[str(COSIGN),verb,*extra,*trust,reference];env=dict(os.environ,DOCKER_CONFIG=str(config),TUF_ROOT=str(cache),GODEBUG='http2client=0')
 for key in ['DOCKER_AUTH_CONFIG','REGISTRY_AUTH_FILE']:env.pop(key,None)
 with (P/(name+'.stdout.jsonl')).open('wb') as out,(P/(name+'.stderr.txt')).open('wb') as err:r=subprocess.run(argv,env=env,stdout=out,stderr=err,timeout=330)
 record={'command':argv,'exit':r.returncode,'TUF_ROOT':str(cache),'stdout':name+'.stdout.jsonl','stderr':name+'.stderr.txt','oneAttemptNoPolicyRetry':True,'timestampPolicy':'OriginalCSI defaults: strict certificate/SCT/transparency and trusted log timestamp validation; no native-only tlogversion/RFC3161 format requirement.'}
 (P/(name+'.verification-result.json')).write_text(json.dumps(record,indent=2)+'\n');results.append(record)
 assert r.returncode==0,'original strict CSI '+name+' verification failed; retain actual failure, no retry/waiver'

def objects(text):
 decoder=json.JSONDecoder();text=text.strip();rows=[]
 while text:
  value,end=decoder.raw_decode(text);rows.extend(value if isinstance(value,list) else [value]);text=text[end:].strip()
 return rows

signatures=objects((P/'signature.stdout.jsonl').read_text());assert signatures
assert all(x.get('critical',{}).get('image',{}).get('docker-manifest-digest')==digest for x in signatures),'verified image subject differs'
assert any(x.get('critical',{}).get('type')=='https://sigstore.dev/cosign/sign/v1' for x in signatures),'missing verified image signature'
envelopes=objects((P/'sbom.stdout.jsonl').read_text());statements=[]
for envelope in envelopes:
 payload=base64.b64decode(envelope['payload'],validate=True);statement=json.loads(payload);assert statement.get('predicateType')=='https://spdx.dev/Document'
 assert any(x.get('digest',{}).get('sha256')==digest[7:] for x in statement.get('subject',[])),'verified SPDX subject differs'
 statements.append(statement);(P/('signed-spdx-'+hashlib.sha256(payload).hexdigest()+'.json.gz')).write_bytes(gzip.compress(payload,mtime=0))
assert statements,'missing verified SPDX'
packages=[p for s in statements for p in s.get('predicate',{}).get('packages',[])];names={p.get('name','') for p in packages}
assert any('cryptsetup' in n for n in names) and any('e2fsprogs' in n for n in names),'original filesystem package requirement not met'
summaries=[]
for statement in statements:
 spdx=statement['predicate'];packages=spdx.get('packages',[]);arches=sorted({a for p in packages for ref in p.get('externalRefs',[]) for a in re.findall(r'(?:[?&])arch=([^&]+)',ref.get('referenceLocator',''))})
 summaries.append({'spdxVersion':spdx.get('spdxVersion'),'subject':statement['subject'],'packageCount':len(packages),'packageArchitectureQualifiers':arches,'filesystemPackages':[{'name':p.get('name'),'version':p.get('versionInfo')} for p in packages if any(n in p.get('name','') for n in ['cryptsetup','e2fsprogs','blkid','device-mapper'])]})
report={'reference':reference,'source':SOURCE,'strictIdentity':IDENTITY,'strictIssuer':'https://token.actions.githubusercontent.com','cosignVersion':'v3.1.4','cosignSha256':hashlib.sha256(COSIGN.read_bytes()).hexdigest(),'signature_verified':True,'spdx_attestation_verified':True,'verifiedSignatureCount':len(signatures),'verificationResults':results,'originalPolicyPreserved':True,'requiredNativeTlogOrTSAFormat':False,'sbomSummaries':summaries,'sbomScope':'Only actual inventory architecture qualifiers above; no inferred arm64 SBOM. Runtime children separately require original metadata/baseclosure and bounded anonymous probes.','qualifiedForPinReview':False,'productionOrPinWrites':False}
(P/'crypto-review.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k] for k in ['reference','source','signature_verified','spdx_attestation_verified','originalPolicyPreserved','qualifiedForPinReview']},indent=2))
