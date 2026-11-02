"""Tiny invented signed-looking DATA for field-binding controls, never real crypto."""
import base64,copy,gzip,json
import core as c
import readiness as r
import retained_v11_predicates as v11

def encoded(value):return json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
class Facts:
    def __init__(self):
        self.docs={};self.i=0
        roles={'producer':'a'*40,'verifier':'b'*40,'generator':'c'*40,'utility_source':'d'*40,'family_generator':'e'*40}
        layers={arch:(arch+'-CONTROL_ONLY-layer').encode() for arch in ('amd64','arm64')};manifests={};configs={};children=[];platforms={};blobs=[]
        for arch in ('amd64','arm64'):
            config=encoded({'os':'linux','architecture':arch,'rootfs':{'diff_ids':['sha256:'+c.sha(layers[arch])]}});configref=self.put('config-'+arch,config);configs[arch]=configref
            desc={'digest':'sha256:'+c.sha(config),'size':len(config)};layer={'digest':'sha256:'+c.sha(layers[arch]),'size':len(layers[arch])}
            manifest=encoded({'schemaVersion':2,'config':desc,'layers':[layer]});ref=self.put('manifest-'+arch,manifest);manifests[arch]=ref
            children.append({'digest':'sha256:'+c.sha(manifest),'size':len(manifest),'platform':{'os':'linux','architecture':arch}})
            platforms[arch]={'manifest_digest':'sha256:'+c.sha(manifest),'config_digest':desc['digest']}
            for d in (desc,layer):blobs.append({'digest':d['digest'],'bytes_hashed':d['size'],'registry_repository':'ghcr.io/bex-co/native',
                'anonymous_full_get_at':'2026-11-01T00:00:00Z','reused_prior_full_read':False})
        index=encoded({'schemaVersion':2,'manifests':children});common='ghcr.io/bex-co/native@sha256:'+c.sha(index)
        signed=[];results=[];reports={}
        for kind,arch,ptype in [('image-signature',None,'https://sigstore.dev/cosign/sign/v1'),('sbom','amd64','https://spdx.dev/Document'),('sbom','arm64','https://spdx.dev/Document'),('vulnerability','amd64','https://aquasecurity.github.io/trivy/attestation/vuln/v1'),('vulnerability','arm64','https://aquasecurity.github.io/trivy/attestation/vuln/v1')]:
            predicate={'CONTROL_ONLY':kind,'architecture':arch}
            if kind=='sbom':
                predicate={'spdxVersion':'SPDX-2.3','SPDXID':'SPDXRef-DOCUMENT','documentDescribes':['SPDXRef-Root'],
                    'packages':[{'SPDXID':'SPDXRef-Root','name':'CONTROL_ONLY_root','checksums':[{'algorithm':'SHA256','checksumValue':platforms[arch]['config_digest'].split(':')[1]}]}]+[
                        {'SPDXID':'SPDXRef-'+name,'name':name,'versionInfo':'CONTROL_ONLY','externalRefs':[{'referenceLocator':'pkg:deb/debian/'+name+'@1?arch='+arch}]} for name in sorted(v11.REQUIRED|{'libvips42'})]}
            elif kind=='vulnerability':
                config=json.loads(self.docs[configs[arch]['path']]);predicate={'ArtifactType':'container_image','SchemaVersion':2,'ArtifactName':common,
                    'Metadata':{'ImageID':platforms[arch]['config_digest'],'ImageConfig':config},'Results':[{'Class':'os-pkgs','Type':'debian','Vulnerabilities':[]}]}
            raw_pred=encoded(predicate)
            statement=encoded({'subject':[{'digest':{'sha256':common.split('@sha256:')[1]}}],'predicateType':ptype,'predicate':predicate})
            bundle=encoded({'dsseEnvelope':{'payload':base64.b64encode(statement).decode()},'verificationMaterial':{'tlogEntries':[{'kindVersion':{'kind':'hashedrekord','version':'0.0.2'}}],'timestampVerificationData':{'rfc3161Timestamps':[{'CONTROL_ONLY':'NOT_REAL_SIGNATURE'}]}}})
            bd='sha256:'+c.sha(bundle);path='/private/tmp/CONTROL_ONLY_V11/'+bd[7:]+'.bundle.json';bgz=path+'.gz';pgz=path+'.predicate.gz'
            reports[bgz]=self.put('bundle-'+str(len(signed)),gzip.compress(bundle,mtime=0));reports[pgz]=self.put('predicate-'+str(len(signed)),gzip.compress(raw_pred,mtime=0))
            signed.append({'kind':kind,'architecture':arch,'bundle_digest':bd,'bundle_bytes':len(bundle),'path':path,'predicate_type':ptype,
                'signed_statement_sha256':c.sha(statement),'full_bundle_gzip':bgz,'full_predicate_gzip':pgz,
                'decoded_predicate_sorted_json_sha256':c.sha(raw_pred),'decoded_predicate_sorted_json_bytes':len(raw_pred),
                'full_bundle_gzip_sha256':reports[bgz]['sha256'],'full_predicate_gzip_sha256':reports[pgz]['sha256']})
            results.append({'bundle_digest':bd,'exit_code':0,'command':r.native_command(path,ptype,common.split('@')[1],roles['verifier'])})
        negative_source={'exit_code':1,'command':r.native_command(signed[0]['path'],signed[0]['predicate_type'],common.split('@')[1],'0'*40)}
        negative_subject={'exit_code':1,'command':r.native_command(signed[0]['path'],signed[0]['predicate_type'],'sha256:'+'0'*64,roles['verifier'])}
        crypto={'results':results,'accepted_bundles':[s['bundle_digest'] for s in signed],'wrong_source':negative_source,'wrong_subject':negative_subject}
        tools={'/private/tmp/CONTROL_ONLY_V11/driver.py':'f'*64}
        receipt={'ref':common,'approved_supplier_candidate':True,'producer_workflow_sha':roles['producer'],'verification_workflow_sha':roles['verifier'],
            'producer_verification_provenance':{'producer_sha':roles['producer'],'verification_sha':roles['verifier'],'generator_sha':roles['generator']},
            'evidence_tool_sha256':tools,'platforms':platforms,'anonymous_pull':{'unique_blob_count':len(blobs),'blob_receipts':blobs},'signed_payloads':signed}
        baseline=b'package build\nconst root="CONTROL_ONLY_PREPIN"\n';baseline_ref=self.put('baseline',baseline)
        utility_cases=[];utility_rows=[]
        for arch in ('amd64','arm64'):
            want={'id':'utility-'+arch,'architecture':arch,'command':['CONTROL_ONLY','utility',arch],'observations':{'pg_restore':'18.2','pg15_archive':'restored','pg18_archive':'bounded'},'inputs':{}}
            result=self.result(want['command'],want['observations']);utility_cases.append(want)
            utility_rows.append({'id':want['id'],'architecture':arch,'common_ref':common,'source':roles['utility_source'],'result':result,'inputs':{}})
        family_cases=[];family_rows=[]
        for arch in ('amd64','arm64'):
            want={'id':'family-'+arch,'architecture':arch,'command':['CONTROL_ONLY','family',arch],'observations':{'runtime':'node','phase':'app-node','version':'24.14.0'},'inputs':{}}
            family_cases.append(want);family_rows.append({'id':want['id'],'architecture':arch,'common_ref':common,'source':roles['family_generator'],'result':self.result(want['command'],want['observations']),'inputs':{}})
        image='ghcr.io/bex-co/generated-python@sha256:'+'9'*64
        root={'SPDXID':'SPDXRef-Root','name':'CONTROL_ONLY_generated_python','versionInfo':image.split('@')[1]}
        sbom={'spdxVersion':'SPDX-2.3','packages':[root,{'SPDXID':'SPDXRef-brotli','name':'libbrotli1','versionInfo':'1.0.9-2+deb12u1'}],
              'relationships':[{'spdxElementId':'SPDXRef-DOCUMENT','relationshipType':'DESCRIBES','relatedSpdxElement':'SPDXRef-Root'}]}
        spdx=self.put('generated-SPDX',encoded(sbom));command=['CONTROL_ONLY','generated-python-no-signal']
        generated={'result':self.result(command,{'python':'3.14.8','version_signals':[],'base':common,'image':image,'spdx_sha256':spdx['sha256'],'spdx_image':image}),'spdx':spdx}
        self.contract={'common_ref':common,'roles':roles,'qualifier_tools':tools,'qualification_source':{r.COMMON_SLOT:{'bytes':len(baseline),'sha256':c.sha(baseline)}},
            'adoption_critical_paths':[r.COMMON_SLOT],'adoption_critical_scope':[{'directory':'lego/operator/internal/build','suffix':'.go','exclude_suffixes':['_test.go']}],
            'utility_cases':utility_cases,'family_cases':family_cases,'generated_default_python_command':command,
            'fixed_brotli_versions':['1.0.9-2+deb12u1'],'generated_spdx_root':{'image':image,'SPDXID':root['SPDXID'],'package':root}}
        self.entry={'common_ref':common,'roles':roles,'v11_receipt':self.put('V11',encoded(receipt)),'v11_crypto':self.put('crypto',encoded(crypto)),
            'qualification_source':{r.COMMON_SLOT:baseline_ref},'index':self.put('index',index),'platform_manifests':manifests,'platform_configs':configs,
            'retained_reports':reports,'wrong_source_stderr':self.put('wrong-source',b'expected GithubWorkflowSHA '+roles['verifier'].encode()),
            'wrong_subject_stderr':self.put('wrong-subject',b'provided artifact digest does not match any digest in statement'),
            'utilities':self.put('utilities',encoded({'rows':utility_rows})),'families':self.put('families',encoded({'rows':family_rows})),
            'generated_default_python':self.put('generated',encoded(generated))}
    def put(self,name,raw):
        self.i+=1;path='/private/tmp/CONTROL_ONLY_QUALIFICATION/'+str(self.i)+'-'+name+'.json';self.docs[path]=raw
        return {'path':path,'bytes':len(raw),'sha256':c.sha(raw)}
    def doc(self,ref):return json.loads(self.docs[ref['path']])
    def replace(self,ref,value):
        raw=encoded(value);self.docs[ref['path']]=raw;ref.update(bytes=len(raw),sha256=c.sha(raw))
    def result(self,command,observation):return {'exit_code':0,'complete':True,'bound':None,'signal':None,'command':command,'stdout':self.put('stdout',encoded(observation))}
