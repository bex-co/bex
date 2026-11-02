"""Retained qualification and separately reviewed adoption binding. No external I/O.

Root must approve the exact capsule AND contract bytes before actual use. Neither
this source nor synthetic tests can create an actual adoption contract.
"""
import gzip,json,re
import core as c
import proof_binding as p
import retained_v11_predicates as v11
import evidence
import known_go_slot

SHA40=re.compile('[a-f0-9]{40}\\Z')
REF=re.compile(r'ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}\Z')
ARCHES={'amd64','arm64'}
NATIVE_IDENTITY='https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main'

def require_source(value):c.require(isinstance(value,str) and SHA40.fullmatch(value),'source SHA missing')
def read(ref,reader):return p.read_ref(ref,reader)
def document(ref,reader):
    value=json.loads(read(ref,reader));c.require(isinstance(value,dict),'proof JSON object missing');return value

def completed_result(result,reader,command=None):
    c.require(isinstance(result,dict),'completed process receipt missing');c.completed(result)
    c.require(isinstance(result.get('command'),list) and result['command']
              and (command is None or result['command']==command),'qualified exact command differs')
    return read(result.get('stdout'),reader)

def bind_rows(raw,expected,common_ref,reader,role,source):
    c.require(isinstance(expected,list) and expected and isinstance(raw.get('rows'),list),'required utility/family catalog absent')
    required={x['id']:x for x in expected};rows={x['id']:x for x in raw['rows']}
    c.require(len(required)==len(expected) and len(rows)==len(raw['rows']) and set(rows)==set(required),'complete proof catalog differs')
    for key,want in required.items():
        row=rows[key]
        c.require(row.get('common_ref')==common_ref and row.get('source')==source and row.get('architecture')==want['architecture']
                  and row['architecture'] in ARCHES,'proof role/source/common/architecture differs')
        observed=json.loads(completed_result(row.get('result'),reader,want['command']))
        c.require(observed==want['observations'],'actual captured utility/family observations differ:'+key)
        inputs=row.get('inputs',{})
        c.require(set(inputs)==set(want.get('inputs',{})),'proof original input catalog differs')
        for name,identity in want.get('inputs',{}).items():
            observed_input=evidence.verify_full(inputs[name],reader);c.require(observed_input==identity,'original utility/archive input differs')
    return {'role':role,'source':source,'ids':sorted(rows)}

def native_command(bundle_path,predicate,index,source):
    return [p.COSIGN,'verify-blob-attestation','--bundle',bundle_path,'--type',predicate,'--digest',index.split(':')[1],
            '--digestAlg','sha256','--certificate-identity',NATIVE_IDENTITY,'--certificate-oidc-issuer',p.ISSUER,
            '--certificate-github-workflow-sha',source,'--use-signed-timestamps','--timeout','5m']

COMMON_SLOT='lego/operator/internal/build/nativebases.go'

def scope_matches(path,scopes):
    from pathlib import PurePosixPath
    for scope in scopes:
        c.require(isinstance(scope,dict),'reviewed critical scope malformed')
        if set(scope)=={'path'}:
            target=scope['path'];c.require(isinstance(target,str) and target and not PurePosixPath(target).is_absolute() and '..' not in PurePosixPath(target).parts,'critical exact scope invalid')
            if path==target:return True
        else:
            c.require(set(scope)=={'directory','suffix','exclude_suffixes'} and isinstance(scope['directory'],str)
                      and scope['directory'] and not PurePosixPath(scope['directory']).is_absolute()
                      and '..' not in PurePosixPath(scope['directory']).parts and scope['suffix']=='.go'
                      and scope['exclude_suffixes']==['_test.go'],'reviewed runtime critical directory scope invalid')
            if path.startswith(scope['directory'].rstrip('/')+'/') and path.endswith('.go') and not path.endswith('_test.go'):return True
    return False

def common_initializer(ref,syntax):
    c.require(isinstance(ref,str) and REF.fullmatch(ref),'qualified common initializer REF missing')
    c.require(syntax in ('inferred','typed'),'exact reviewed Go initializer syntax missing')
    return ('var nativeCommonBase '+('string ' if syntax=='typed' else '')+'= '+json.dumps(ref)+'\n').encode()


def qualification(entry,contract,reader):
    c.require(isinstance(entry,dict) and isinstance(contract,dict),'qualification contract missing')
    common=entry.get('common_ref');c.require(isinstance(common,str) and REF.fullmatch(common) and common==contract.get('common_ref'),'same common immutable index missing')
    roles=entry.get('roles',{})
    for name in ('producer','verifier','generator','utility_source','family_generator'):require_source(roles.get(name))
    c.require(roles==contract.get('roles'),'retained qualification role sources differ')
    receipt=document(entry.get('v11_receipt'),reader);crypto=document(entry.get('v11_crypto'),reader)
    c.require(receipt.get('ref')==common and receipt.get('approved_supplier_candidate') is True
              and receipt.get('producer_workflow_sha')==roles['producer']
              and receipt.get('verification_workflow_sha')==roles['verifier'],'V11 actual producer/verifier/common differs')
    provenance=receipt.get('producer_verification_provenance',{})
    c.require(provenance.get('producer_sha')==roles['producer'] and provenance.get('verification_sha')==roles['verifier']
              and provenance.get('generator_sha')==roles['generator'],'separate retained generator provenance differs')
    c.require(receipt.get('evidence_tool_sha256')==contract.get('qualifier_tools') and contract.get('qualifier_tools'),
              'reviewed executed V11 tool source baseline differs')
    # Actual retained baseline source bytes, not future pinned working tree.
    baseline=entry.get('qualification_source',{})
    c.require(set(baseline)==set(contract.get('qualification_source',{})) and baseline,'qualification baseline catalog incomplete')
    critical_paths=contract.get('adoption_critical_paths');scopes=contract.get('adoption_critical_scope')
    c.require(isinstance(critical_paths,list) and critical_paths and len(set(critical_paths))==len(critical_paths)
              and isinstance(scopes,list) and scopes and COMMON_SLOT in critical_paths,
              'closed qualified adoption-critical snapshot/scope missing')
    c.require(set(critical_paths)=={path for path in baseline if scope_matches(path,scopes)},
              'qualified critical source set differs from reviewed closed scope')
    for path,expected in contract['qualification_source'].items():
        body=read(baseline[path],reader);c.require({'bytes':len(body),'sha256':c.sha(body)}==expected,'retained qualification factory input differs')
    index_raw=read(entry.get('index'),reader);index=json.loads(index_raw)
    c.require('sha256:'+c.sha(index_raw)==common.split('@')[1],'actual common index bytes differ')
    platforms=receipt.get('platforms',{});c.require(set(platforms)==ARCHES,'both actual common platforms missing')
    children={d['platform']['architecture']:d for d in index.get('manifests',[]) if d.get('platform',{}).get('os')=='linux' and d.get('platform',{}).get('architecture') in ARCHES}
    c.require(set(children)==ARCHES and len([d for d in index['manifests'] if d.get('platform',{}).get('architecture') in ARCHES])==2,'unique actual platform pair missing')
    descriptors={};actual_platforms={};actual_manifests={};identities={}
    for arch in sorted(ARCHES):
        raw=read(entry['platform_manifests'][arch],reader);manifest=json.loads(raw)
        c.require(children[arch]['digest']=='sha256:'+c.sha(raw)==platforms[arch]['manifest_digest']
                  and children[arch]['size']==len(raw),'actual common child identity differs')
        config_raw=read(entry['platform_configs'][arch],reader);config=json.loads(config_raw)
        c.require(manifest['config']['digest']=='sha256:'+c.sha(config_raw)==platforms[arch]['config_digest']
                  and manifest['config']['size']==len(config_raw) and config['os']=='linux' and config['architecture']==arch,'common actual config differs')
        actual_platforms[arch]=dict(platforms[arch],config=config);actual_manifests[arch]=manifest
        identities[platforms[arch]['manifest_digest']]=arch;identities[platforms[arch]['config_digest']]=arch
        for descriptor in [manifest['config'],*manifest['layers']]:
            c.require(type(descriptor.get('size')) is int and descriptor['size']>0,'actual blob descriptor size missing')
            digest=descriptor['digest'];c.require(digest not in descriptors or descriptors[digest]==descriptor['size'],'conflicting blob descriptor')
            descriptors[digest]=descriptor['size']
    pull=receipt.get('anonymous_pull',{});blobs=pull.get('blob_receipts',[])
    c.require(type(pull.get('unique_blob_count')) is int and pull['unique_blob_count']==len(descriptors)==len(blobs)
              and {x.get('digest') for x in blobs}==set(descriptors),'complete full-byte blob receipts missing')
    for row in blobs:
        c.require(type(row.get('bytes_hashed')) is int and row['bytes_hashed']==descriptors[row['digest']]
                  and row.get('registry_repository')=='ghcr.io/'+common.split('@')[0].removeprefix('ghcr.io/')
                  and isinstance(row.get('anonymous_full_get_at'),str)
                  and type(row.get('reused_prior_full_read')) is bool
                  and (not row['reused_prior_full_read'] or isinstance(row.get('current_anonymous_head_at'),str)),
                  'full actual blob size/hash/head custody incomplete')
    # Exact V11 synthetic Docker-export roots, if actually needed, remain
    # linked to the retained per-layer decompression receipts; no new layers.
    for binding in receipt.get('spdx_additional_verified_root_bindings',[]):
        directory=binding['proof_directory'];refs=entry.get('docker_root_proofs',{})
        class BoundPath:
            def __init__(self,path):self.path=str(path)
            def __truediv__(self,name):return BoundPath(self.path.rstrip('/')+'/'+str(name))
            def read_bytes(self):return read(refs[self.path],reader)
            def read_text(self):return self.read_bytes().decode()
            def __str__(self):return self.path
        original_path=v11.Path;v11.Path=BoundPath
        try:aliases,bindings=v11.docker_source_root_aliases(directory,common.split('@')[1],actual_platforms,actual_manifests)
        finally:v11.Path=original_path
        c.require(binding in bindings,'retained V11 decompressed-root aggregate differs')
        for digest,arch in aliases.items():
            c.require(digest not in identities or identities[digest]==arch,'ambiguous actual root identity');identities[digest]=arch
    signed=receipt.get('signed_payloads',[]);accepted={x['bundle_digest']:x for x in crypto.get('results',[]) if type(x.get('exit_code')) is int and x['exit_code']==0}
    c.require(signed and len(accepted)==len(crypto.get('accepted_bundles',[])) and set(accepted)==set(crypto['accepted_bundles']), 'actual strict bundle process results incomplete')
    seen=set();kinds=set()
    for row in signed:
        bd=row.get('bundle_digest');c.require(bd in accepted and bd not in seen,'strict signed bundle missing/duplicate');seen.add(bd)
        expected_type={'image-signature':'https://sigstore.dev/cosign/sign/v1','sbom':'https://spdx.dev/Document','vulnerability':'https://aquasecurity.github.io/trivy/attestation/vuln/v1'}
        c.require(row.get('predicate_type')==expected_type.get(row.get('kind')),'canonical V11 predicate kind differs')
        c.require(entry['retained_reports'][row['full_bundle_gzip']]['sha256']==row.get('full_bundle_gzip_sha256')
                  and entry['retained_reports'][row['full_predicate_gzip']]['sha256']==row.get('full_predicate_gzip_sha256'),
                  'actual retained full report gzip receipt differs')
        positive=accepted[bd];c.require(positive.get('command')==native_command(row['path'],row['predicate_type'],common.split('@')[1],roles['verifier'])
                                     and positive.get('bound') is None and positive.get('signal') is None,'strict V11 command differs')
        bundle_raw=evidence.gunzip_full(entry['retained_reports'][row['full_bundle_gzip']],reader)
        c.require('sha256:'+c.sha(bundle_raw)==bd and len(bundle_raw)==row['bundle_bytes'],'full retained signed bundle differs')
        bundle=json.loads(bundle_raw);material=bundle.get('verificationMaterial',{})
        c.require(material.get('tlogEntries') and all(x.get('kindVersion')=={'kind':'hashedrekord','version':'0.0.2'} for x in material['tlogEntries'])
                  and material.get('timestampVerificationData',{}).get('rfc3161Timestamps'),'actual v2/TSA material missing')
        import base64
        statement_raw=base64.b64decode(bundle['dsseEnvelope']['payload'],validate=True);statement=json.loads(statement_raw)
        c.require(c.sha(statement_raw)==row['signed_statement_sha256'] and statement['predicateType']==row['predicate_type']
                  and any(x.get('digest',{}).get('sha256')==common.split('@sha256:')[1] for x in statement.get('subject',[])),
                  'actual signed source/report subject differs')
        predicate=evidence.gunzip_full(entry['retained_reports'][row['full_predicate_gzip']],reader)
        canonical=json.dumps(statement['predicate'],sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode()
        c.require(predicate==canonical and c.sha(predicate)==row['decoded_predicate_sorted_json_sha256']
                  and len(predicate)==row['decoded_predicate_sorted_json_bytes'],'retained full predicate binding differs')
        if row['kind']=='sbom':
            actual_arch,_=v11.spdx_binding(statement['predicate'],identities)
            c.require(actual_arch==row.get('architecture'),'signed full SPDX architecture differs')
        elif row['kind']=='vulnerability':
            actual_arch,_=v11.trivy_binding(statement['predicate'],identities,actual_platforms,common)
            c.require(actual_arch==row.get('architecture'),'signed full Trivy architecture differs')
        else:c.require(row['kind']=='image-signature' and row.get('architecture') is None,'unexpected signed report kind')
        kinds.add((row['kind'],row.get('architecture')))
    c.require(seen==set(accepted) and {('image-signature',None),('sbom','amd64'),('sbom','arm64'),('vulnerability','amd64'),('vulnerability','arm64')}<=kinds,
              'actual image and both architecture signed report pairs missing')
    for name,diagnostic in [('wrong_source','expected GithubWorkflowSHA'),('wrong_subject','provided artifact digest does not match any digest in statement')]:
        result=crypto.get(name,{})
        c.require(type(result.get('exit_code')) is int and result['exit_code']==1 and result.get('bound') is None and result.get('signal') is None,
                  'canonical strict negative refusal missing')
        expected=native_command(result['command'][3],result['command'][5],('sha256:'+'0'*64) if name=='wrong_subject' else common.split('@')[1],('0'*40) if name=='wrong_source' else roles['verifier'])
        c.require(result['command']==expected and diagnostic.encode() in read(entry[name+'_stderr'],reader),'strict negative command/diagnostic differs')
    utilities=bind_rows(document(entry.get('utilities'),reader),contract.get('utility_cases'),common,reader,'utilities',roles['utility_source'])
    c.require({x['architecture'] for x in contract['utility_cases']}==ARCHES,'original both utility arches missing')
    families=bind_rows(document(entry.get('families'),reader),contract.get('family_cases'),common,reader,'families',roles['family_generator'])
    generated=document(entry.get('generated_default_python'),reader)
    output=json.loads(completed_result(generated.get('result'),reader,contract['generated_default_python_command']))
    c.require(output.get('python')=='3.14.8' and output.get('version_signals')==[] and output.get('base')==common
              and isinstance(output.get('image'),str) and REF.fullmatch(output['image']), 'actual generated no-signal Python3.14.8 identity missing')
    spdx=evidence.full_document(generated.get('spdx'),reader)
    c.require(spdx.get('spdxVersion')=='SPDX-2.3' and output['spdx_sha256']==generated['spdx']['sha256']
              and output['spdx_image']==output['image'],'actual generated exact-image full SPDX missing')
    root=contract.get('generated_spdx_root',{})
    c.require(root.get('image')==output['image'] and isinstance(root.get('SPDXID'),str)
              and isinstance(root.get('package'),dict) and root['package'].get('SPDXID')==root['SPDXID']
              and root['package'] in spdx.get('packages',[])
              and any(x.get('relationshipType')=='DESCRIBES' and x.get('relatedSpdxElement')==root['SPDXID']
                      for x in spdx.get('relationships',[])), 'reviewed generated exact-image SPDX root binding missing')
    packages=[x for x in spdx.get('packages',[]) if x.get('name')=='libbrotli1']
    c.require(packages and all(x.get('versionInfo') in contract.get('fixed_brotli_versions',[]) for x in packages), 'actual generated SPDX Brotli fixed package missing')
    return {'common_ref':common,'roles':roles,'utilities':utilities,'families':families,'generated_default_python':output['image'],
            'qualified_baseline_source':roles['generator'],'qualification_source':contract['qualification_source'],
            'critical_source':{path:contract['qualification_source'][path] for path in critical_paths},
            'critical_scope':scopes,'adoptable_refs':sorted({common,output['image']}|{x['observations']['image'] for x in contract['family_cases']
                if isinstance(x.get('observations',{}).get('image'),str) and REF.fullmatch(x['observations']['image'])})}

def adoption(contract,entry,qualified,reader,git):
    c.require(isinstance(contract,dict) and contract.get('schema')=='bex.original28.adoption-contract/v1','actual reviewed adoption contract absent')
    source=contract.get('source');require_source(source);require_source(contract.get('required_ancestor'))
    c.require(entry.get('source')==source and entry.get('image')==contract.get('operator_image')
              and git('ancestor',contract['required_ancestor'],source) is True,'actual adopted signed source differs')
    files=contract.get('critical_files',{});baseline_contract=qualified.get('critical_source',{});scopes=qualified.get('critical_scope')
    c.require(isinstance(files,dict) and files and contract.get('scope_code') and contract.get('pins')
              and isinstance(baseline_contract,dict) and baseline_contract and isinstance(scopes,list) and scopes
              and COMMON_SLOT in baseline_contract and set(files)==set(baseline_contract),
              'closed critical source set missing or includes additions/deletions')
    actual_files={};before_files={};changed=set()
    for path,expected in files.items():
        baseline,base_mode,base_blob=git('blob',qualified['qualified_baseline_source'],path)
        c.require({'bytes':len(baseline),'sha256':c.sha(baseline)}==baseline_contract[path]
                  and baseline_contract[path]==qualified['qualification_source'].get(path),
                  'actual qualified critical baseline differs from retained identity:'+path)
        raw,mode,blob=git('blob',source,path)
        c.require({'bytes':len(raw),'sha256':c.sha(raw),'git_mode':mode,'git_blob':blob}==expected,'adoption critical whole blob differs:'+path)
        c.require(base_mode==mode,'unreviewed critical file mode change')
        actual_files[path]=raw;before_files[path]=baseline
        if baseline!=raw or base_blob!=blob:changed.add(path)
    changes=git('changes',qualified['qualified_baseline_source'],source)
    c.require(isinstance(changes,list) and all(isinstance(row,dict) and set(row)=={'status','path'}
              and row['status'] in ('A','D','M','T') and isinstance(row['path'],str) for row in changes),
              'actual source change-set observation missing')
    scoped=[row for row in changes if scope_matches(row['path'],scopes)]
    c.require(len({row['path'] for row in scoped})==len(scoped)
              and all(row['status']=='M' and row['path'] in baseline_contract for row in scoped)
              and {row['path'] for row in scoped}==changed,
              'actual critical additions/deletions/unlisted changes differ from closed snapshot')
    deltas=contract.get('pin_only_deltas')
    c.require(isinstance(deltas,list) and all(isinstance(row,dict) and isinstance(row.get('path'),str) for row in deltas)
              and len({row['path'] for row in deltas})==len(deltas)
              and {row['path'] for row in deltas}==changed,
              'every changed critical blob requires one exact reviewed pin delta')
    syntax=contract.get('common_initializer_syntax','inferred')
    for delta in deltas:
        path=delta['path']
        c.require(delta.get('qualification_source')==qualified['qualified_baseline_source']
                  and path in changed and read(delta['before'],reader)==before_files[path],
                  'pin delta does not bind complete retained qualified baseline')
        out=before_files[path];edits=delta.get('edits')
        c.require(isinstance(edits,list) and edits,'changed critical blob has no strict pin transformation')
        for edit in edits:
            c.require(isinstance(edit,dict),'pin edit malformed')
            kind=edit.get('kind','immutable-ref')
            if kind=='native-common-base-initializer':
                c.require(path==COMMON_SLOT and set(edit)=={'kind','before','after'},'initial common pin outside exact known declaration')
                before=b'var nativeCommonBase string\n';after=common_initializer(qualified['common_ref'],syntax)
                actual_before=known_go_slot.declaration(out)
                c.require(actual_before['form']=='zero' and out[actual_before['start']:actual_before['end']]==before,
                          'initial common pin does not address the actual top-level zero-value slot')
                c.require(edit['before']==before.decode() and edit['after']==after.decode(),
                          'initial common pin is not the exact reviewed declaration mapping')
            else:
                c.require(kind=='immutable-ref' and set(edit)<={'kind','before','after'},'unreviewed pin edit grammar')
                old,new=edit.get('before'),edit.get('after')
                c.require(isinstance(old,str) and isinstance(new,str) and REF.fullmatch(old) and REF.fullmatch(new)
                          and new in qualified['adoptable_refs'],'unreviewed runtime edit is not qualified immutable pin-only')
                before=json.dumps(old).encode();after=json.dumps(new).encode()
            if kind=='native-common-base-initializer':
                # Replace only the recognized actual declaration span. Identical
                # text inside comments/literals is neither a slot nor an edit.
                out=out[:actual_before['start']]+after+out[actual_before['end']:]
            else:
                c.require(out.count(before)==1,'reviewed pin span is not unique')
                out=out.replace(before,after,1)
        c.require(out==actual_files[path],'unreviewed function/scope/comment/source change beyond exact pin spans')
    fixed=common_initializer(qualified['common_ref'],syntax)
    slot=actual_files[COMMON_SLOT]
    actual_slot=known_go_slot.declaration(slot)
    c.require(actual_slot['value']==qualified['common_ref'] and actual_slot['form']==syntax
              and slot[actual_slot['start']:actual_slot['end']]==fixed,
              'actual known common slot is empty or differs from qualified REF')
    for pin in contract['pins']:
        c.require(pin['qualified_ref'] in qualified['adoptable_refs'] and pin['path'] in actual_files
                  and pin['literal'].encode() in actual_files[pin['path']] and pin['literal']==json.dumps(pin['qualified_ref']),
                  'actual every-workspace adopted immutable common/runtime pin differs')
    for scope in contract['scope_code']:
        c.require(scope.get('scope')=='all-workspaces' and scope['path'] in actual_files
                  and type(scope.get('first_byte')) is int and type(scope.get('last_byte')) is int
                  and 0<=scope['first_byte']<scope['last_byte']<=len(actual_files[scope['path']]),'reviewed all-workspace code binding absent')
        body=actual_files[scope['path']][scope['first_byte']:scope['last_byte']]
        c.require(c.sha(body)==scope['sha256'] and body==read(scope['source'],reader), 'actual all-workspace source body differs; bool is not proof')
    p.bind_signature(entry,reader)
    c.require(entry.get('APIApproval') is not None,'separate actual API approval required')
    p.bind_signature(entry['APIApproval'],reader)
    return {'source':source,'image':entry['image'],'critical_files':files,'common_ref':qualified['common_ref'],'changed_critical_paths':sorted(changed)}

def admit(capsule,contract,now,reader,git):
    # Deliberately first: null/missing adopted authority refuses before auth and
    # before any large qualification proof consumption. Source prep leaves null.
    c.require(isinstance(contract,dict) and isinstance(contract.get('adoption'),dict),'actual reviewed adoption contract absent before auth')
    c.require(contract.get('schema')=='bex.original28.admission-contract/v1' and isinstance(capsule,dict),'admission contract/capsule schema missing')
    qualified=qualification(capsule.get('qualification'),contract.get('qualification'),reader)
    adopted=adoption(contract['adoption'],capsule.get('ready',{}),qualified,reader,git)
    observed=p.bind_inventory(capsule['ready'],now,reader)
    # Binding does not replace health: all observed generations/counts/spec and
    # running Ready runtime refs must independently satisfy the approved carriers.
    for field,pods,image in [('Operator','ReadyPods',adopted['image']),('API','APIReadyPods',capsule['ready']['APIApproval']['image'])]:
        dep=capsule['ready'][field];count=dep['replicas']
        c.require(type(dep['generation']) is int and dep['generation']>0 and type(dep['observedGeneration']) is int
                  and dep['generation']==dep['observedGeneration'] and type(count) is int and count>0
                  and all(type(dep[k]) is int and dep[k]==count for k in ('updatedReplicas','readyReplicas','availableReplicas'))
                  and dep['image']==image,'actual adopted rollout unhealthy')
        rows=capsule['ready'][pods]
        c.require(len(rows)==count==len({r['uid'] for r in rows}) and all(r['uid'] and r['phase']=='Running' and r['ready'] is True
                  and r['image']==r['imageID']==image and r['deletingAt'] in ('',None) for r in rows),'actual adopted runtime pod proof unhealthy')
    return {'state':'PASS','bound_evidence':{'qualification':qualified,'adoption':adopted,'observation':observed}}
