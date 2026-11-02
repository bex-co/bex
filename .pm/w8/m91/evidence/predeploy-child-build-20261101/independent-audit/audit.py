"""Read-only in-place child-build audit and bounded portable proof export."""
from pathlib import Path, PurePosixPath
import ast, datetime, gzip, hashlib, io, json, re, shutil, stat, subprocess, tarfile, zipfile, zlib

OUT=Path(__file__).resolve().parent
R=Path('/private/tmp/bex-w8-common-predeploy-build-20261101-4bff55bed15b1d32')
E=R/'evidence'
S=Path('/private/tmp/bex-w8-common-predeploy-build-source-20261101-wp_wtewn')
X=Path('/private/tmp/bex-w8-common-predeploy-exporters-actual-20261101-d9g07211')
REPO=Path('/Users/tianpan/projects/bex7')
SOURCE='6883953e58e750fb41763d1d5e4bd4103bdb74c5'
COMMON='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
OWNER='bex-pd-7cf7c0d919d6'
CAP=4*1024**2
C=OUT/'capsule'
assert not C.exists()
C.mkdir(mode=0o700)
copied=[]
def sha(b):return hashlib.sha256(b).hexdigest()
def load(p):return json.loads(p.read_bytes())
def identity(p):
    assert p.is_file() and not p.is_symlink()
    h=hashlib.sha256();n=0
    with p.open('rb') as f:
        for b in iter(lambda:f.read(1024**2),b''):h.update(b);n+=len(b)
    return {'bytes':n,'sha256':h.hexdigest(),'mode':format(stat.S_IMODE(p.stat().st_mode),'04o')}
def same(p,w):
    got=identity(p)
    assert all(got[k]==w[k] for k in ('bytes','sha256')),str(p)
    if 'mode' in w:assert got['mode']==w['mode'],str(p)
    return got
def copy(p,name):
    assert not any(k in PurePosixPath(name).parts for k in ('..','.env','config.json'))
    before=identity(p);dest=C/name
    assert before['bytes']<1024**2 and not dest.exists()
    dest.parent.mkdir(parents=True,exist_ok=True)
    shutil.copyfile(p,dest);dest.chmod(int(before['mode'],8))
    assert identity(dest)==before
    copied.append({'path':name,'original_path':str(p),**before})
def dump(p,v):p.write_text(json.dumps(v,indent=2)+'\n')
original_manifest=load(R/'FINAL-MANIFEST.json')
assert original_manifest['controller_result_preserved'] is True
original_names={str(p.relative_to(R)) for p in R.rglob('*') if p.is_file() and p!=R/'FINAL-MANIFEST.json'}
assert original_names==set(original_manifest['files'])
for n,w in original_manifest['files'].items():same(R/n,w)
receipt=load(R/'RECEIPT.json');plan=load(S/'PLAN.json')
assert receipt['state']=='BUILD_STEP_SUCCESS_CLEANUP_RECONCILED'
assert receipt['source_sha']==plan['source_sha']==SOURCE and plan['common_image']==COMMON
for n,w in load(S/'SOURCE-MANIFEST.json').items():same(S/n,w)
source=load(X/'SOURCE.json')
assert source['commit']==SOURCE and len(source['files'])==848
tree={}
raw=subprocess.check_output(['git','ls-tree','-r','-z',SOURCE],cwd=REPO)
for line in raw.split(b'\0'):
    if line:
        data,n=line.split(b'\t',1);mode,kind,oid=data.decode().split()
        tree[n.decode()]={'mode':mode,'kind':kind,'blob':oid}
for n,w in source['files'].items():
    p=X/'source'/n;v=identity(p)
    assert v['bytes']==w['bytes'] and v['sha256']==w['sha256'] and int(v['mode'],8)==w['permissions']
    assert tree[n]=={'mode':w['mode'],'kind':'blob','blob':w['git_blob']}
    assert hashlib.sha1(b'blob '+str(v['bytes']).encode()+b'\0'+p.read_bytes()).hexdigest()==w['git_blob']
current=subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True).strip()
changed=subprocess.check_output(['git','diff','--name-only',SOURCE,current,'--',*source['files']],cwd=REPO,text=True).splitlines()
assert changed==['lego/operator/internal/build/nativequalification_init_test.go']
context=Path(plan['context_manifest']).parent
ctx=load(Path(plan['context_manifest']))
for n,w in ctx.items():same(context/n,w)
assert len([n for n in ctx if n.startswith('context/') and n!='context/Dockerfile'])==45
checks=load(X/'OUTPUT-CHECKS.json')
assert checks['PASS'] is True and len(checks['fixtures42'])==42
for n,w in checks['fixtures42'].items():
    assert (context/'context'/n).read_bytes()==(X/'source'/w['source_path']).read_bytes()
recipe=(context/'Dockerfile.canonical').read_bytes()
assert recipe==(context/'Dockerfile.executed').read_bytes()==(context/'context/Dockerfile').read_bytes()
assert sha(recipe)==plan['dockerfile_sha256']=='5e171db5e194d52efa00ff495ca29af50d8792a0fdb8e33d13265d47a5ea003a'
assert recipe.count(('FROM '+COMMON+'\n').encode())==1
assert b'/usr/bin/python3 utility-check.py build && /usr/bin/python3 original-cases.py build node' in recipe
for n,w in plan['tools_sha256'].items():assert identity(Path(plan['tools_root'])/n)['sha256']==w
assert identity((Path(plan['host_buildx_plugin_dir'])/'docker-buildx').resolve())['sha256']==plan['host_buildx_plugin_sha256']
commands=load(E/'commands.json');cmd={x['label']:x for x in commands}
assert len(cmd)==len(commands)==379
assert all(x['direct_child_reaped'] is True and x['group_absent'] is True and x['bound'] is None for x in commands)
case_position=next(i for i,x in enumerate(commands) if x['label']=='case')
for i,x in enumerate(commands):
    if x['pending_pgids']:
        assert i<case_position and x['label'].startswith(('builder-observe-','builder-du-')) and x['pending_pgids']==[cmd['case']['pgid']]
result=load(E/'RESULT.json')
assert result['PASS'] is False and result['build_exit0'] is True and result['failure'] is None
assert result['cleanup_errors']==['existing cleanup helper nonzero'] and result['pending_pgids']==[] and result['source_unchanged'] is True and result['handlers_restored'] is True
assert cmd['case']['exit_code']==0 and cmd['case']['argv']==['docker','start','--attach',result['runner_cid']]
runner=load(E/'runner-cleanup-before.stdout')[0]
assert runner['Id']==result['runner_cid'] and runner['Name']=='/'+OWNER+'-runner' and runner['Config']['Labels']['co.bex.native-qualification']==OWNER
assert runner['State']['Running'] is False and runner['State']['ExitCode']==0
builder=load(E/'builder-cleanup-before.stdout')[0];volume=load(E/'volume-cleanup-before.stdout')[0]
assert builder['Name']=='/buildx_buildkit_'+OWNER+'0' and builder['Config']['Image']==plan['buildkit_image']
assert volume['Name']=='buildx_buildkit_'+OWNER+'0_state'
assert [x['Name'] for x in builder['Mounts'] if x['Destination']=='/var/lib/buildkit']==[volume['Name']]
for kind,before in [('runner',runner),('builder',builder)]:
    assert cmd[kind+'-remove']['argv']==['docker','rm','--force',before['Id']] and cmd[kind+'-remove']['exit_code']==0
    label=kind+'-cleanup-after'
    assert cmd[label]['exit_code']==1 and (E/(label+'.stdout')).read_bytes()==b'[]\n'
    assert (E/(label+'.stderr')).read_bytes()==('Error response from daemon: No such container: '+before['Name'][1:]+'\n').encode()
assert cmd['volume-remove']['argv']==['docker','volume','rm',volume['Name']] and cmd['volume-remove']['exit_code']==0
assert cmd['volume-cleanup-after']['exit_code']==1 and (E/'volume-cleanup-after.stdout').read_bytes()==b'[]\n'
assert (E/'volume-cleanup-after.stderr').read_bytes()==('Error response from daemon: get '+volume['Name']+': no such volume\n').encode()
for kind in ['container','image']:
    row=cmd[kind+'-owned-absence']
    assert row['exit_code']==0 and row['argv'][-1]=='label=co.bex.native-qualification='+OWNER
    assert (E/row['stdout']).read_bytes()==b'' and (E/row['stderr']).read_bytes()==b''
assert not (R/(OWNER+'-docker-config')).exists()
helper=load(E/'helper-cleanup-command.json')
assert helper['exit_code']==1 and 'refusing cleanup without an exact qualification owner' in helper['stderr']
assert OWNER.startswith('bex-pd-') and not re.fullmatch(r'bex-nq-[a-f0-9]{24}',OWNER)
assert not any('prune' in x['argv'] or x['argv'][1:3]==['context','use'] for x in commands)
assert result['minimum_physical_free_bytes']>=plan['bounds']['remaining_host_floor_bytes']
assert result['peak_combined_observed_bytes']<=plan['bounds']['combined_owned_bytes']
assert result['maximum_oci_observed_bytes']<=plan['bounds']['oci_bytes']
assert result['final_log_bytes']<=plan['bounds']['log_bytes']
assert result['elapsed_seconds']<=plan['bounds']['whole_seconds']
# Full local archive and every compressed blob are freshly hashed, without loading or running.
archive=E/'child.oci'
assert identity(archive)['sha256']==receipt['OCI_tar']['sha256']=='fb39d8c2495f5d7f7d62ef3cbbd69bdb8caf910bd64a0777f270a6a43f5629bd'
assert archive.stat().st_size==941516800
items={};meta={}
with tarfile.open(archive,'r:') as t:
    members=t.getmembers()
    assert len(members)==40
    for m in members:
        assert not PurePosixPath(m.name).is_absolute() and '..' not in PurePosixPath(m.name).parts and '\\' not in m.name
        if m.isdir():continue
        assert m.isfile() and m.name not in items
        f=t.extractfile(m);h=hashlib.sha256();n=0;small=[]
        while True:
            b=f.read(1024**2)
            if not b:break
            h.update(b);n+=len(b)
            if m.size<1024**2:small.append(b)
        assert n==m.size
        items[m.name]={'bytes':n,'sha256':h.hexdigest()}
        if m.name.startswith('blobs/sha256/'):assert m.name.split('/')[-1]==h.hexdigest()
        if small:
            body=b''.join(small)
            try:x=json.loads(body)
            except (ValueError,UnicodeDecodeError):continue
            if isinstance(x,dict):meta[m.name]=(body,x)
    expected=load(E/'OCI-IDENTITY.json')
    assert items==expected['blob_custody']
    def desc(d):
        assert type(d['size']) is int and re.fullmatch('sha256:[0-9a-f]{64}',d['digest'])
        name='blobs/sha256/'+d['digest'][7:]
        assert items[name]=={'bytes':d['size'],'sha256':d['digest'][7:]}
        return name
    index=meta['index.json'][1]
    assert len(index['manifests'])==1 and meta['oci-layout'][1]=={'imageLayoutVersion':'1.0.0'}
    key=desc(index['manifests'][0]);manifest=meta[key][1]
    assert index['manifests'][0]['digest']==receipt['platform_manifest']['digest']
    assert index['manifests'][0]['platform']=={'architecture':'arm64','os':'linux'}
    config=meta[desc(manifest['config'])][1]
    assert manifest['config']['digest']==receipt['config']['digest']
    assert len(manifest['layers'])==len(config['rootfs']['diff_ids'])==35
    assert sum(x['size'] for x in manifest['layers'])==941464782
    for d in manifest['layers']:desc(d)
    assert config['os']=='linux' and config['architecture']=='arm64'
    assert config['config']['Env']==receipt['actual_ENV'] and config['config']['Cmd']==receipt['actual_CMD']
    assert config['config'].get('Entrypoint') is None and config['config']['WorkingDir']=='/opt/render/project/src'
    assert config['config']['Labels']['co.bex.native-qualification']==OWNER
    assert config['config']['Labels']['org.opencontainers.image.revision']=='1feba939b4697894e800ed0abb4f25a0bf415641'
    metadata=load(E/'build-metadata.json')
    assert metadata['containerimage.digest']==receipt['platform_manifest']['digest'] and metadata['containerimage.config.digest']==receipt['config']['digest']
    last=desc(manifest['layers'][-1]);prefix='opt/render/project/src/';selected={};total=0;count=0;seen=set()
    # Bound only the final owned build layer. Select public artifacts/qa-output, never system/auth/cache files.
    with gzip.GzipFile(fileobj=t.extractfile(last)) as z,tarfile.open(fileobj=z,mode='r|') as layer:
        for m in layer:
            count+=1;total+=m.size;assert count<=10000 and total<=64*1024**2
            assert m.name not in seen;seen.add(m.name)
            assert not PurePosixPath(m.name).is_absolute() and '..' not in PurePosixPath(m.name).parts and '\\' not in m.name
            wanted=m.name.startswith(prefix+'qa-output/') or m.name.startswith(prefix+'public/')
            if not wanted or m.isdir():continue
            assert m.isfile() and m.size<=256*1024
            body=layer.extractfile(m).read();assert len(body)==m.size
            name='build-artifacts/'+m.name[len(prefix):]
            dest=C/name;dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(body);dest.chmod(0o600)
            selected[name]={'bytes':len(body),'sha256':sha(body),'original_tar':str(archive),'compressed_layer_sha256':last.rsplit('/',1)[-1],'original_member':m.name,'source_mode':oct(m.mode)}
    assert len(selected)==94 and sum(x['bytes'] for x in selected.values())==318395
# Independently reapply the exact pure maintained validator functions to actual original build stdout.
context_root=context/'context';specs=load(context_root/'original-cases.json');node=[x for x in specs if x['runtime']=='node']
assert {x['mid'] for x in node}=={91,93,98,99,101,103,107}
oracle_rows=[]
for x in node:
    mid=x['mid'];work=C/'build-artifacts/qa-output'/('m'+str(mid))
    for name,w in x['primary_reference']['files'].items():same(work/name,w)
    for name,w in x['source_file_identities'].items():
        if name.endswith('-marker.json') or (mid==103 and name=='qm.json'):assert load(work/name)==x['marker']
        else:same(work/name,w)
    validator=context_root/'cases'/('m'+str(mid))/'validators.py';tree=ast.parse(validator.read_bytes())
    assert all(isinstance(n,ast.FunctionDef) for n in tree.body)
    scope={'reference':x['validation_reference'],'hashlib':hashlib,'json':json,'expected_module_sha':x.get('expected_module_sha')}
    exec(compile(tree,str(validator),'exec'),scope)
    stdout=(C/'build-artifacts/qa-output'/('build-'+str(mid)+'.log')).read_bytes()
    proofs=[]
    for match in re.finditer(r'QA[A-Za-z0-9_]* (\{[^\n]+\})',stdout.decode()):
        try:proofs.append(json.loads(match[1]))
        except ValueError:pass
    assert any(scope['valid_after'](v,x) for v in proofs)
    oracle_rows.append({'mid':mid,'pure_original_valid_after':True,'authored_source_and_data':True,'validator_sha256':identity(validator)['sha256'],'actual_stdout':identity(C/'build-artifacts/qa-output'/('build-'+str(mid)+'.log'))})
maintained=ast.parse((context_root/'utility-check.py').read_bytes())
checks_expected=[n.args[0].value for n in ast.walk(maintained) if isinstance(n,ast.Call) and isinstance(n.func,ast.Attribute) and isinstance(n.func.value,ast.Name) and n.func.value.id=='results' and n.func.attr=='append' and len(n.args)==1 and isinstance(n.args[0],ast.Constant)]
raw=(E/'case.stderr').read_bytes();summaries=[]
for line in raw.splitlines():
    if re.match(rb'#[0-9]+ [0-9.]+ \{"phase": "build"',line):
        summaries.append(json.loads(line[line.index(b'{'):]))
assert len(summaries)==2 and summaries[0]=={'phase':'build','checks':checks_expected,'pass':True}
assert len(checks_expected)==11 and summaries[1]['pass'] is True and summaries[1]['mode']=='node' and summaries[1]['python']=='3.11.2'
public=C/'build-artifacts/public'
payloads={'empty':b'','binary':bytes(range(256))*64,'unicode':('Crème 雪\n'*100).encode()}
for name,body in payloads.items():
    b=(public/(name+'.gz')).read_bytes()
    assert b[:8]==b'\x1f\x8b\x08\x00\x00\x00\x00\x00' and gzip.decompress(b)==body
    assert int.from_bytes(b[-8:-4],'little')==zlib.crc32(body) and int.from_bytes(b[-4:],'little')==len(body)
zips={'empty.txt':b'','greeting.txt':('Crème brûlée & 李\n'*100).encode(),'two words.bin':bytes(range(256))*64}
for name,method in [('stored',0),('deflated',8)]:
    with zipfile.ZipFile(public/(name+'.zip')) as z:
        assert set(z.namelist())==set(zips) and z.testzip() is None
        for k,b in zips.items():assert z.read(k)==b and z.getinfo(k).CRC==zlib.crc32(b)
        assert all(v.compress_type==0 for v in z.infolist()) if method==0 else any(v.compress_type==8 for v in z.infolist())
# Copy complete small actual evidence and public input context. Huge OCI and tools remain hash-bound external originals.
for p in sorted(E.rglob('*')):
    if p.is_file() and p.name!='child.oci':copy(p,'actual/evidence/'+str(p.relative_to(E)))
for n in ['RECEIPT.json','ACTUAL-BUILD-PROOFS.json','CLEANUP-RECONCILIATION.json','FINAL-MANIFEST.json','inspect_oci.py','seal.py','oci-inspect.stdout','oci-inspect.stderr','seal.stdout','seal.stderr']:copy(R/n,'actual/'+n)
for n in ['SOURCE-MANIFEST.json','PLAN.json','retained-v4.py','run_build.py','build.sh']:copy(S/n,'controller/'+n)
for p in sorted(context.rglob('*')):
    if p.is_file():copy(p,'exported-context/'+str(p.relative_to(context)))
for n in ['SOURCE.json','OUTPUT-CHECKS.json','RUNTIME-RESULT.json','RUNTIME-COMMANDS.json','COMPILE-COMMANDS.json','COMPILE-RESULT.json','overlay-B.json','MANIFEST.json','RECEIPT.json','NEXT-BUILD.json']:copy(X/n,'exporter/'+n)
report={'schema':'bex.predeploy-child-build-independent-review/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'outcome':'CLEAN_BUILD_ONLY_WITH_ORIGINAL_WRAPPER_FAILURE_AND_SEPARATE_CLEANUP_RECONCILIATION','original_result':identity(E/'RESULT.json'),'original_controller_PASS':False,'original_controller_exit':1,'build_step_exit':0,'original_cleanup_error':'strict CI helper refuses local bex-pd owner; actual fallback cleanup is separate and retained','cleanup_reconciled':True,'all_379_command_groups_recorded_reaped_absent':True,'source_snapshot':{'commit':SOURCE,'paths':848,'manifest':identity(X/'SOURCE.json'),'all_snapshot_bytes_modes_Git_blob_IDs_verified':True,'comparison_current_HEAD':current,'changed_paths':changed,'B_inputs_unchanged':True},'canonical_recipe':identity(context/'Dockerfile.canonical'),'fixtures42_unchanged':True,'actual_build_checks':11,'seven_original_pure_oracle_reapplications':oracle_rows,'static_gzip_ZIP_empty_binary_Unicode_CRC_checks':True,'OCI':{'tar':identity(archive),'index':identity(E/'oci-metadata/index.json'),'platform_manifest':receipt['platform_manifest'],'config':receipt['config'],'layers':35,'compressed_layer_bytes':941464782,'all_compressed_blobs_freshly_SHA256_checked':True,'labels':receipt['actual_labels'],'ENV':receipt['actual_ENV'],'CMD':receipt['actual_CMD'],'revision_is_inherited_common_producer':'1feba939b4697894e800ed0abb4f25a0bf415641'},'selected_owned_final_layer_artifacts':{'regular_files':94,'bytes':318395,'compressed_layer_sha256':last.rsplit('/',1)[-1],'declared_full_layer_uncompressed_bytes':total,'full_layer_members':count},'bounds':plan['bounds'],'actual_observed_resources':receipt['observed_resources'],'limitations':['ONE local ARM64 canonical child build; no runtime CMD, HTTP, predeploy phase, Kubernetes Job, publication, adoption, botharch or wholefamily proof.','Actual generation source688 differs from the inherited1feba OCI revision; source/context/command custody binds688 independently.','Original wrapper FAILED remains immutable; reconciled cleanup does not turn the wrapper result into PASS.','Raw OCI/toolbinaries/fullsource/cache excluded from capsule with exact identities retained. Local OCI compressed bytes and selected original owned final-layer build artifacts were read only; no load/import/execution/network.']}
dump(OUT/'REVIEW.json',report)
copy(OUT/'REVIEW.json','REVIEW.json')
dump(C/'BUILD-ARTIFACT-CUSTODY.json',selected)
dump(C/'CUSTODY.json',{'files':copied,'selected_layer_artifacts':'BUILD-ARTIFACT-CUSTODY.json','excluded':{'huge_OCI':{'original_path':str(archive),**identity(archive)},'tool_binaries':plan['tools_sha256'],'full_source_snapshot':'exporter/SOURCE.json'},'scope':'Byte-exact included regular files. Paths/modes/hashes preserved; raw layer-derived owned public build artifacts copied byte-exact with archive/layer/member binding. No auth/config/cache bodies or tool executables copied.'})
assert sum(p.stat().st_size for p in C.rglob('*') if p.is_file())<CAP
for n,w in original_manifest['files'].items():
    if n!='evidence/child.oci':same(R/n,w)
assert identity(R/'FINAL-MANIFEST.json')['sha256']==sha(json.dumps(original_manifest,indent=2).encode()+b'\n') or load(R/'FINAL-MANIFEST.json')==original_manifest
print(json.dumps({'outcome':report['outcome'],'root':str(OUT),'capsule_files':sum(p.is_file() for p in C.rglob('*')),'capsule_bytes':sum(p.stat().st_size for p in C.rglob('*') if p.is_file()),'review':identity(OUT/'REVIEW.json')},indent=2))

