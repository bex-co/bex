"""Record actual public CSI bundle format after strict original verification.
This is metadata custody, not an additional trust verifier or format policy.
"""
from pathlib import Path
from urllib.parse import urlencode
import base64,datetime,hashlib,json,runpy
P=Path(__file__).resolve().parent
crypto=json.loads((P/'crypto-review.json').read_text())
assert crypto['source']=='dd64e4ff7888828faa1b547f3a637b76d7e7da91' and crypto['signature_verified'] and crypto['spdx_attestation_verified'] and crypto['originalPolicyPreserved']
state=runpy.run_path(str(P/'registry-review.py'));registry=state['registry'];meta=state['receipt'];assert crypto['reference']==meta['candidate_ref']
raw=registry.request('https://ghcr.io/v2/bex-co/bex-operator/referrers/'+meta['digest'],state['ACCEPT']);(P/'candidate-referrer-index.json').write_bytes(raw);descriptors=json.loads(raw)['manifests'];assert len(descriptors)<=16,'bounded metadata inventory exceeded'
records=[]
for descriptor in descriptors:
 manifest=registry.get('manifests',descriptor['digest'],'referrer-'+descriptor['digest'][7:]+'.json')
 layers=[layer for layer in manifest.get('layers',[]) if layer.get('mediaType','').startswith('application/vnd.dev.sigstore.bundle.')]
 for layer in layers:
  bundle=registry.get('blobs',layer['digest'],'bundle-'+layer['digest'][7:]+'.json')
  envelope=bundle.get('dsseEnvelope');statement=json.loads(base64.b64decode(envelope['payload'],validate=True)) if envelope else {};vm=bundle.get('verificationMaterial',{})
  records.append({'manifestDigest':descriptor['digest'],'descriptorArtifactType':descriptor.get('artifactType'),'manifestArtifactType':manifest.get('artifactType'),'layerMediaType':layer['mediaType'],'bundleDigest':layer['digest'],'bundleMediaType':bundle.get('mediaType'),'signedPayloadKind':'dsseEnvelope' if envelope else 'messageSignature' if bundle.get('messageSignature') else 'unknown','predicateType':statement.get('predicateType'),'subjects':statement.get('subject'),'tlogKindVersions':[entry.get('kindVersion') for entry in vm.get('tlogEntries',[])],'tlogIntegratedTimes':[entry.get('integratedTime') for entry in vm.get('tlogEntries',[])],'rfc3161TimestampCount':len(vm.get('timestampVerificationData',{}).get('rfc3161Timestamps',[]))})
report={'atUTC':datetime.datetime.now(datetime.timezone.utc).isoformat(),'reference':meta['candidate_ref'],'source':meta['source'],'scope':'Actual immutable public metadata custody after strict original CSI verification; no new required tlog/TSA format or independent trust claim.','rawReferrerIndexSHA256':hashlib.sha256(raw).hexdigest(),'bundleRecords':records,'descriptorCount':len(descriptors),'allRetrievedRawDigestMetadataVerified':True,'approvalAuthority':False}
(P/'actual-bundle-format.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
