#!/usr/bin/env python3
"""First actual native review: strict official bundle verification plus all canonical semantic/blob gates."""
from pathlib import Path
import importlib.util,json,hashlib,base64,gzip,os,datetime
s=importlib.util.spec_from_file_location('v','/tmp/loopx-w8-verify-native-artifact.py');v=importlib.util.module_from_spec(s);s.loader.exec_module(v)
s=importlib.util.spec_from_file_location('f','/Users/tianpan/projects/bex7/scripts/lib/toolchain-freshness.py');f=importlib.util.module_from_spec(s);s.loader.exec_module(f)
root=Path('/tmp/loopx-w8-native-modern-review');direct=root/'direct-bundle-proof';sha='4330878c280bc9748444c8e1e50ff7388df1228a';name='node-14-bullseye';base='node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e';digest='sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc';tag='native-'+name+'-'+sha;ref='ghcr.io/'+v.REPOSITORY+'@'+digest;out=v.REVIEWS/tag;out.mkdir(exist_ok=True)
v.require('FRESHNESS_DIGESTS' not in os.environ,'fixture digest injection forbidden')
def progress(stage,**kw):
 e={'at':v.now(),'stage':stage,'verified':False,**kw};v.dump(root/'direct-finalize-progress.json',e);print(json.dumps(e),flush=True)
progress('fresh-CI-and-immutable-metadata')
ci=v.github_candidate_job(sha,name,'41029596739');v.dump(out/'ci-runtime-scan-signing-gates.json',ci)
url,headers=v.public_registry(f,'ghcr.io',v.REPOSITORY);idx,raw,actual=v.manifest_document(url,headers,tag);v.require(actual==digest,'publicsource tag changedindex');(out/'index.json').write_bytes(raw);children=v.image_platforms(idx)
provenance=v.recovery_provenance(sha,name,base,digest,ci,out);v.dump(out/'producer-verification-provenance.json',provenance)
sr,repo,_=f.parse_upstream('docker.io/library/'+base.split('@')[0]);su,sh=v.public_registry(f,sr,repo);source,sraw,_=v.manifest_document(su,sh,base.split('@')[1]);(out/'official-index.json').write_bytes(sraw);sources=v.image_platforms(source)
platforms={};manifests={};identities={};blobs={}
for arch,child in children.items():
 m,mraw,_=v.manifest_document(url,headers,child);official,oraw,_=v.manifest_document(su,sh,sources[arch]);c,craw=v.config_document(url,headers,m['config']);original,ocraw=v.config_document(su,sh,official['config'])
 v.require(c.get('os')==original.get('os')=='linux' and c.get('architecture')==original.get('architecture')==arch,'actualcandidate/sourceplatformmismatch')
 labels=c['config'].get('Labels',{});v.require(labels.get('org.opencontainers.image.revision')==provenance['producer_sha'] and labels.get('org.opencontainers.image.source')=='https://github.com/bex-co/bex','producerlabels/source mismatch')
 before,after=v.official_environment(original),v.official_environment(c);v.require(all(after.get(k)==val for k,val in before.items()),'officialruntimeENVchanged')
 ancestry=v.official_ancestry(m,c,official,original);manifests[arch]=m;platforms[arch]={'manifest_digest':child,'config_digest':m['config']['digest'],'config':c,'official_base_ancestry':ancestry};identities[child]=identities[m['config']['digest']]=arch
 for suffix,data in [('manifest',mraw),('config',craw),('official-manifest',oraw),('official-config',ocraw)]:(out/(arch+'-'+suffix+'.json')).write_bytes(data)
 for d in [m['config'],*m['layers']]:
  h,size=v.descriptor(d)
  if h in blobs:v.require(blobs[h]['size']==size,'conflicting blob size')
  blobs[h]=d
aliases,bindings=v.docker_source_root_aliases(root/'docker-root-proof',digest,platforms,manifests)
for alias,arch in aliases.items():
 v.require(alias not in identities or identities[alias]==arch,'ambiguous actual/synthetic identity');identities[alias]=arch;platforms[arch]['official_base_ancestry']['manual_decompressed_layer_hashing_performed']=True
progress('strict-verified-bundles-and-independent-report-bindings')
download=json.loads((direct/'download-receipt.json').read_text());crypto=json.loads((direct/'strict-report-crypto-results.json').read_text());byhash={r['bundle_digest']:r for r in crypto['reports']};positive=json.loads((direct/'image-positive-result.json').read_text());negative=json.loads((direct/'image-wrong-source-negative-result.json').read_text())
v.require(positive['exit_code']==0 and positive['expected_source_sha']==sha and negative['exit_code']!=0,'actualimage cryptoorwrongsourceguardfailed')
negtext=(direct/'image-wrong-source-negative.stderr.log').read_text();v.require('expected GithubWorkflowSHA' in negtext and sha in negtext,'negativeguarddidnotfailforactualsourcepolicy')
v.require(hashlib.sha256(v.COSIGN.read_bytes()).hexdigest()==v.COSIGN_SHA256,'verified toolchanged')
sboms={};vulns={};signed=[];image_count=0
for row in download['bundles']:
 p=Path(row['path']);raw=p.read_bytes();v.require('sha256:'+hashlib.sha256(raw).hexdigest()==row['bundle_digest'] and len(raw)==row['bundle_bytes'],'actualverifiedbundlebyteschanged')
 b=json.loads(raw);env=b['dsseEnvelope'];payload=base64.b64decode(env['payload'],validate=True);statement=json.loads(payload);pt=statement['predicateType']
 v.require(any(x.get('digest',{}).get('sha256')==digest.split(':')[1] for x in statement.get('subject',[])),'signedsubjectdiffers')
 tlogs=b['verificationMaterial'].get('tlogEntries',[]);timestamps=b['verificationMaterial'].get('timestampVerificationData',{}).get('rfc3161Timestamps',[])
 v.require(tlogs and all(x.get('kindVersion')=={'kind':'hashedrekord','version':'0.0.2'} for x in tlogs) and timestamps,'actualv2log/TSA materialmissing')
 if pt=='https://sigstore.dev/cosign/sign/v1':image_count+=1;arch=None;kind='image-signature'
 else:
  cr=byhash[row['bundle_digest']];v.require(cr['exit_code']==0 and cr['expected_source_sha']==sha and cr['explicit_tsa_required'] is True,'strictreportcryptofailed')
  v.require('--certificate-identity' in cr['command'] and '--certificate-oidc-issuer' in cr['command'] and '--certificate-github-workflow-sha' in cr['command'] and '--digest' in cr['command'] and not any('insecure-ignore' in a for a in cr['command']),'strictcrypto flagschanged')
  if pt==v.SPDX_TYPE:arch,summary=v.spdx_binding(statement['predicate'],identities);kind='sbom';sboms.setdefault(arch,[]).append(summary)
  elif pt==v.TRIVY_TYPE:arch,summary=v.trivy_binding(statement['predicate'],identities,platforms,ref);kind='vulnerability';vulns.setdefault(arch,[]).append(summary)
  else:raise RuntimeError('unexpected currentverifiedpredicatetype')
  target=out/(kind+'-'+arch+'-'+row['bundle_digest'].split(':')[1][:12]+'.predicate.json.gz')
  with gzip.open(target,'wt') as fz:json.dump(statement['predicate'],fz)
 signed.append({'kind':kind,'architecture':arch,'manifest_digest':row['manifest_digest'],'bundle_digest':row['bundle_digest'],'bundle_bytes':row['bundle_bytes'],'bundle_file':str(p),'signed_statement_sha256':hashlib.sha256(payload).hexdigest(),'tlog_kind_versions':[x['kindVersion'] for x in tlogs],'tsa_timestamp_count':len(timestamps),'strict_cryptographic_verification':'officialCosign3.1.4 verify-blob-attestation with exactsubjectdigest, issuer, workflowidentity, sourceSHA and explicitRFC3161required; noignoreflags'})
v.require(image_count==1 and set(sboms)==set(vulns)=={'amd64','arm64'},'independentactualsignedreportcoverageincomplete')
progress('all-signed-reports-bound-checking-current-anonymous-blobs',report_count=len(signed))
url,headers=v.public_registry(f,'ghcr.io',v.REPOSITORY);cache=v.BlobReceipts(v.REVIEWS/'.public-blob-receipts-v2');pulls=[cache.pull(url,headers,d,lambda stage,**kw:progress(stage,**kw)) for d in blobs.values()]
receipt={'reviewed_at':v.now(),'approved_supplier_candidate':True,'official_base':base,'tag':'ghcr.io/'+v.REPOSITORY+':'+tag,'ref':ref,'workflow_sha':sha,'verification_workflow_sha':sha,'producer_workflow_sha':provenance['producer_sha'],'producer_verification_provenance':provenance,'ci_runtime_scan_signing_evidence':ci,'platforms':{a:{k:x for k,x in p.items() if k!='config'} for a,p in platforms.items()},'anonymous_manifest_bytes_sha_verified':True,'anonymous_pull':{'all_unique_config_and_layer_bytes_fully_hashed':True,'byte_hash_scope':'Allcompresseddescriptors plus separatelyactuallydecompressedDiffIDs/sizes; exactDocker-source-rootprooflinked','unique_blob_count':len(pulls),'bytes_hashed_this_finalization':sum(p['bytes_hashed'] for p in pulls if not p['reused_prior_full_read']),'bytes_reused_from_prior_verified_full_read':sum(p['bytes_hashed'] for p in pulls if p['reused_prior_full_read']),'prior_receipts_also_rechecked_by_anonymous_head':True,'blob_receipts':pulls},'spdx_additional_verified_root_bindings':bindings,'strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified':True,'strict_actual_Rekor_v2_and_RFC3161_TSA_verified':True,'actual_wrong_workflow_source_sha_negative_check_rejected':True,'verification_method':'EveryimmutablebundledownloadedfromSHA-verifiedOCI1.1fallbackindexandverifiedwithofficialCosignverify-blob-attestation; exactimageindexdigestclaims+issuer+workflowidentity+currentSHA+tlog+explicitTSAchecked. No registrydiscoveryassumption orignoredsecuritycheck.','registry_discovery_limitation':'GHCRnativeReferrersAPI404; genericcosignverify-attestationtimedout/reportednovalidbundles; direct5immutablebundleverificationsallpassed. Originalfailedattemptsretained.','signed_payloads':signed,'package_summaries_by_architecture':sboms,'trivy_summaries_by_architecture':vulns,'raw_bundle_evidence_root':str(direct),'scope':'PublishedNode14suppliercandidateONLY. All27catalogadoption, generatedexactimages/Python3.14.8SBOM, hostedrolloutandserviceacceptanceremainseparate. No productionpin/resourcechanged.','evidence_tool_sha256':{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in [Path('/tmp/loopx-w8-verify-native-artifact.py'),Path('/tmp/loopx-w8-native-prove-docker-sbom-roots.py'),Path('/tmp/loopx-w8-native-verify-report-bundles.py'),Path('/tmp/loopx-w8-native-verify-one-bundle.py'),Path(__file__)]}}
v.dump(out/'receipt.json',receipt);v.dump(root/'direct-finalize-result.json',{'at':v.now(),'candidate_approved':True,'ref':ref,'receipt':str(out/'receipt.json')});print(json.dumps({'candidate_approved':True,'ref':ref,'architectures':sorted(sboms),'receipt':str(out/'receipt.json')}),flush=True)
