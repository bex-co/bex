"""Audit retained native-init diagnostics only; no target/tool execution."""
from pathlib import Path
import datetime, hashlib, json, os, re, shutil, stat, struct, subprocess
OUT=Path(__file__).resolve().parent
E=Path('/private/tmp/bex-w8-native-init-local-pilot-e9fbe0e5635547089592094b/evidence')
L=Path('/private/tmp/bex-w8-native-init-yarn-launch-20261101-dl84ogbz')
S=Path('/private/tmp/bex-w8-native-init-yarn-bound-plan-20261101-yy6kanun')
G=E/'generated/native-common-init-4092198803'
REPO=Path('/Users/tianpan/projects/bex7')
C=OUT/'capsule';C.mkdir(mode=0o700)
CAP=4*1024**2
SOURCE='ea9375c1c81e5a58efe354b164bbaf6934a044fa'
COMMON='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
OWNER='bex-nq-e9fbe0e5635547089592094b'
copied=[];omitted=[]
def load(p):return json.loads(p.read_bytes())
def identity(p):
    assert p.is_file() and not p.is_symlink()
    h=hashlib.sha256();n=0
    with p.open('rb') as f:
        for b in iter(lambda:f.read(1<<20),b''):h.update(b);n+=len(b)
    return {'bytes':n,'sha256':h.hexdigest(),'mode':format(stat.S_IMODE(p.stat().st_mode),'04o')}
def same(p,w):
    got=identity(p);assert all(got[k]==w[k] for k in ['bytes','sha256']),str(p)
    return got
def copy(p,name):
    assert not p.is_symlink() and not (set(Path(name).parts)&{'..','.env','config.json'})
    v=identity(p);assert v['bytes']<1024**2
    q=C/name;assert not q.exists();q.parent.mkdir(parents=True,exist_ok=True)
    shutil.copyfile(p,q);q.chmod(int(v['mode'],8));assert identity(q)==v
    copied.append({'path':name,'original_path':str(p),**v})
def dump(p,v):p.write_text(json.dumps(v,indent=2)+'\n')
def elf(p):
    v=identity(p)
    with p.open('rb') as f:
        b=f.read(64);assert b[:7]==b'\x7fELF\x02\x01\x01'
        typ,machine=struct.unpack_from('<HH',b,16);assert typ==2 and machine==183
        phoff=struct.unpack_from('<Q',b,32)[0];ents,num=struct.unpack_from('<HH',b,54)
        assert 0<num<1000 and ents==56
        f.seek(phoff);headers=f.read(ents*num);assert len(headers)==ents*num
        interp=any(struct.unpack_from('<I',headers,i*ents)[0]==3 for i in range(num))
    assert int(v['mode'],8)&0o111 and not interp
    return {**v,'ELF':'64-bit little-endian AArch64 ET_EXEC','PT_INTERP':False}
manifest=load(E/'MANIFEST.json')
assert len(manifest['files'])==188
assert {x['path'] for x in manifest['files']}=={str(p.relative_to(E)) for p in E.rglob('*') if p.is_file() and p!=E/'MANIFEST.json'}
for x in manifest['files']:same(E/x['path'],x)
intent=load(L/'EXECUTION-INTENT.json');plan=intent['plan'];result=load(E/'RESULT.json')
assert result['PASS'] is True and result['failure'] is None and result['cleanup_or_artifact_errors']==[] and result['pending_pgids']==[] and result['cancel_signal'] is None and result['case_pass'] is True
assert result['owner']==plan['owner']==OWNER and plan['source_sha']==SOURCE and plan['common_image']==COMMON and result['elapsed_seconds']<plan['bounds']['whole_seconds']
for n,sha in intent['primary_hashes'].items():assert identity(S/n)['sha256']==sha
for n,w in load(S/'MANIFEST.json').items():same(S/n,w)
source_root=Path(plan['source_root']);compiler=source_root.parent;source=load(Path(plan['source_manifest']))
assert source['source']==SOURCE and len(source['files'])==300
assert identity(Path(plan['source_manifest']))['sha256']==plan['source_manifest_sha256']=='17929596653a4bf08b9f2be4765c3b738241a97c84a2d87a73e7ad81edfa1d1a'
git_tree={}
raw=subprocess.check_output(['git','ls-tree','-r','-z',SOURCE],cwd=REPO)
for row in raw.split(b'\0'):
    if row:
        data,n=row.split(b'\t',1);mode,kind,oid=data.decode().split();git_tree[n.decode()]={'mode':mode,'kind':kind,'blob':oid}
for n,w in source['files'].items():
    p=source_root/n;v=same(p,w)
    assert int(v['mode'],8)==w['permissions'] and git_tree[n]=={'mode':w['git_mode'],'kind':'blob','blob':w['git_blob']}
    assert hashlib.sha1(b'blob '+str(v['bytes']).encode()+b'\0'+p.read_bytes()).hexdigest()==w['git_blob']
test_elf=elf(Path(plan['test_binary']));assert test_elf['sha256']==plan['test_binary_sha256']=='f87d0fc88e4a5ee8b2e7abf63fdcc017832d4eceaac84d4098a7e0b7f0e2055d'
sdk_root=Path(plan['linux_go_root']).parent.parent
sdk_manifest=Path(plan['linux_go_custody']['manifest'])
assert identity(sdk_manifest)['sha256']==plan['linux_go_custody']['manifest_sha256']=='75d9eb9155cd3a0940ff983894d061861ca4401988d3a9a52949a312abf1f213'
sdk_receipt=Path(plan['linux_go_custody']['receipt'])
assert identity(sdk_receipt)['sha256']==plan['linux_go_custody']['receipt_sha256']
sdk_go=elf(Path(plan['linux_go_root'])/'bin/go');assert sdk_go['sha256']==plan['linux_go_binary_sha256']=='d487fddb6939f3e78f7293fcbdeae610ba8ceae966d3faa4b2094a5688559230'
before=load(L/'TOOLCHAIN-BEFORE.json');after=load(L/'TOOLCHAIN-AFTER.json')
assert before==after=={'PASS':True,'paths':17372,'manifest_sha256':plan['linux_go_custody']['manifest_sha256']}
commands=load(E/'commands.json');cmd={x['label']:x for x in commands}
assert len(cmd)==len(commands)==51 and all(x['direct_child_reaped'] is True and x['group_absent'] is True and x['bound'] is None for x in commands)
case_at=next(i for i,x in enumerate(commands) if x['label']=='case')
for i,x in enumerate(commands):
    if x['pending_pgids']:assert i<case_at and x['label'].startswith(('builder-observe-','builder-du-','owned-stats-')) and x['pending_pgids']==[cmd['case']['pgid']]
assert cmd['case']['exit_code']==0
runner_args=cmd['runner-create']['argv']
assert runner_args[runner_args.index('--network')+1]=='host'
assert 'type=bind,src='+plan['linux_go_root']+',dst=/usr/local/go,readonly' in runner_args
assert 'type=bind,src='+plan['test_binary']+',dst=/bex-build.test,readonly' in runner_args
env={runner_args[i+1].split('=',1)[0]:runner_args[i+1].split('=',1)[1] for i,v in enumerate(runner_args) if v=='--env'}
assert env['BEX_NATIVE_TEST_INIT_PROFILE']=='1' and env['BEX_BUILDKIT_INTEGRATION']=='1' and env['BEX_NATIVE_TEST_COMMON_IMAGE']==COMMON and env['BEX_NATIVE_TEST_OWNER']==OWNER
assert env['GOROOT']=='/usr/local/go' and env['PATH'].startswith('/usr/local/go/bin:') and env['GOTOOLCHAIN']=='local' and env['GOPROXY']=='off' and env['GOSUMDB']=='off'
assert '-test.run=^TestCommonNativeInitQualification$' in (S/'pilot.sh').read_text()
custody=load(G/'source-custody.json');expected=custody['expected']
assert custody['common']==COMMON and custody['platform']=='linux/arm64' and custody['build_network']=='none'
assert expected['node']=='24.22.1' and expected['arch']=='arm64' and set(expected['commands'])=={'npm','yarn','pnpm','direct'}
staged=elf(G/'init/native-init')
assert {k:staged[k] for k in ('sha256','bytes')}=={k:expected['init'][k] for k in ('sha256','bytes')} and expected['init']['machine']==183 and expected['init']['executable'] is True
assert staged['sha256']=='329a7736b12aef107861f75dd2012633ee4d29134295904b1d4afc0b258b7dd8'
recipe=(G/'Dockerfile.canonical').read_text()
assert recipe.count('\nFROM '+COMMON+'\n')==1 and 'COPY --link --from=bex-init /native-init /\nENTRYPOINT ["/native-init"]' in recipe
assert 'node build.js' in recipe and 'CMD ["/bin/bash","-c","node capture-env.js npm && exec npm start"]' in recipe
image=(G/'image-identity.txt').read_text().splitlines()
assert len(image)==3 and re.fullmatch('sha256:[a-f0-9]{64}',image[0]) and image[1:] == ['linux/arm64','["/native-init"]']
proof_source=source_root/'lego/operator/internal/build/nativequalification_init_test.go'
go_source=proof_source.read_text()
assert 'response.status != 200 || response.body != "held 雪 λ🙂"' in go_source
assert 'before != staged' in go_source and 'nativeInitQualificationValidate(expected, proof, launch, true)' in go_source
def lower(v):
    if isinstance(v,dict):return {k.lower():lower(x) for k,x in v.items()}
    if isinstance(v,list):return [lower(x) for x in v]
    return v
def validate(proof,launch,drained):
    p=lower(proof);a=p['identity'];command=expected['commands'][launch]
    assert p['run']==expected['run'] and a['node']==expected['node'] and a['executable']=='/usr/local/bin/node' and a['baseenv']==expected['env'] and a['launch']==launch
    n=a['init'];assert n['sha256']==expected['init']['sha256'] and n['machine']==183 and n['executable'] is True and n['pid1']=='/native-init'
    assert n['argv']==['/native-init','/bin/bash','-c',command]
    for k,value in expected['env'].items():
        if k!='PATH' or launch=='direct':assert a['env'][k]==value
    if drained:assert p['phase']=='runtime' and p['pass'] is True and p['holds']==p['completed']==p['signals']==1 and p['events']==['listening','hold_begin','SIGTERM','hold_end','closed']
# Go struct JSON field names are case-insensitive, but ENV variable names remain exact.
# Lower only struct field keys, preserving the nested ENV maps for source-equivalent identity comparisons.
def lower(v):
    if isinstance(v,dict):return {k.lower():(x if k.lower() in ('env','baseenv') else lower(x)) for k,x in v.items()}
    if isinstance(v,list):return [lower(x) for x in v]
    return v
rows=[]
for launch in ['npm','yarn','pnpm','direct']:
    d=G/launch;receipt=load(d/'receipt.json');runtime=load(d/'qa-output/runtime.json');build=load(d/'qa-output/build.json');captured=load(d/'qa-output/launch.json')
    assert receipt['launch']==launch and receipt['common']==COMMON and receipt['platform']=='linux/arm64' and receipt['owner']==OWNER and receipt['pass'] is True
    validate(runtime,launch,True);validate(receipt['proof'],launch,True);validate(receipt['pre_stop'],launch,False)
    assert lower(receipt['proof'])==lower(runtime)
    assert receipt['pre_stop']['Events']==['listening','hold_begin'] and receipt['pre_stop']['Signals']==receipt['pre_stop']['Holds']==receipt['pre_stop']['Completed']==0
    assert receipt['response']=='held 雪 λ🙂'
    primary={'node':expected['node'],'executable':'/usr/local/bin/node','env':expected['env']}
    assert build=={'phase':'build','primary':primary,'pass':True}
    assert captured=={'run':expected['run'],'launch':launch,'primary':primary}
    running=load(d/'running.log');stopped=load(d/'stopped.log');rid=(d/'create.stdout').read_text().strip()
    assert re.fullmatch('[0-9a-f]{64}',rid) and (d/'create.stderr').read_bytes()==b''
    assert running['ID']==stopped['ID']==receipt['container']['ID']==rid and running['Image']==stopped['Image']==image[0]
    assert running['Running'] is True and stopped['Running'] is False and running['Network']==stopped['Network']=='bridge'
    assert running['Labels']['co.bex.native-qualification']==stopped['Labels']['co.bex.native-qualification']==OWNER
    ports=running['Ports'];assert set(ports)=={'10000/tcp'} and len(ports['10000/tcp'])==1
    port=ports['10000/tcp'][0];assert port['HostIp']=='127.0.0.1' and 1<=int(port['HostPort'])<=65535
    status=stopped['ExitCode']
    assert status==0 or (launch!='direct' and status==143) or (launch=='yarn' and status==1)
    assert status=={'npm':143,'yarn':1,'pnpm':143,'direct':0}[launch]
    assert receipt['container']['ExitCode']==status
    assert receipt['command_override'] is None if launch=='npm' else receipt['command_override']==['/bin/bash','-c',expected['commands'][launch]]
    cleanup=load(d/'cleanup.json');assert cleanup['container_id']==rid and cleanup['owner']==OWNER and cleanup['absence_proved'] is True and cleanup['inspection']=='\nError: No such object: '+rid+'\n'
    assert (d/'runtime.log').read_bytes()==(d/'cleanup-runtime.log').read_bytes() and b'"pass": true' in (d/'runtime.log').read_bytes()
    rows.append({'launch':launch,'id':rid,'status':status,'full_identity_and_drain_invariants_reapplied':True,'events':runtime['events'],'held_body':receipt['response'],'held_HTTP200':'enforced by successful source-bound Go test; raw HTTPstatus/headers not separately retained','cleanup_absence':True,'receipt':identity(d/'receipt.json'),'runtime':identity(d/'qa-output/runtime.json')})
stdout=(E/'case.stdout').read_text();assert '--- PASS: TestCommonNativeInitQualification (165.49s)' in stdout and stdout.endswith('PASS\n')
assert all('--- PASS: TestCommonNativeInitQualification/'+x+' ' in stdout for x in ['npm','yarn','pnpm','direct'])
cleanup=load(E/'cleanup.json');helper=load(E/'helper-cleanup-command.json')
assert cleanup['owner']==OWNER and helper['exit_code']==0 and helper['stderr']==''
packet=json.loads(helper['stdout'].strip().split('BEX_NATIVE_CLEANUP_RESULT ',1)[1])
assert packet['owner']==OWNER and all(x['exit_code']==0 for x in cleanup['operations'])
auth=next(x for x in cleanup['operations'] if x['resource']=='docker-auth')
assert auth['removed'] is True and auth['logout_exit_code']==0
assert not Path(auth['identity']).exists()
for kind,name in [('runner',OWNER+'-runner'),('builder','buildx_buildkit_'+OWNER+'0')]:
    row=cmd[kind+'-cleanup-after']
    assert row['exit_code']==1 and (E/row['stdout']).read_bytes()==b'[]\n'
    assert (E/row['stderr']).read_bytes()==('Error response from daemon: No such container: '+name+'\n').encode()
volume='buildx_buildkit_'+OWNER+'0_state';row=cmd['volume-cleanup-after']
assert row['exit_code']==1 and (E/row['stdout']).read_bytes()==b'[]\n' and (E/row['stderr']).read_bytes()==('Error response from daemon: get '+volume+': no such volume\n').encode()
for kind in ['image','container']:
    row=cmd[kind+'-owned-absence'];assert row['exit_code']==0 and (E/row['stdout']).read_bytes()==b'' and (E/row['stderr']).read_bytes()==b''
assert not any('prune' in x['argv'] for x in commands)
for p in sorted(E.rglob('*')):
    if p.is_file():
        if p==G/'init/native-init':omitted.append({'original_path':str(p),**identity(p),'reason':'native-init binary body omitted'})
        else:copy(p,'actual-evidence/'+str(p.relative_to(E)))
for p in sorted(S.iterdir()):
    if p.is_file():copy(p,'controller/'+p.name)
for p in sorted(L.iterdir()):
    if p.is_file():copy(p,'outer-launch/'+p.name)
for n in ['SOURCE.json','SOURCE-DIFF.json','ELF.json','HOST-COMPILER.json','RECEIPT.json','RESULT.json','COMMAND.json','CACHE-CLEANUP.json','MANIFEST.json','compile.stdout','compile.stderr']:copy(compiler/n,'compile/'+n)
for n in ['.node-version','package.json','identity.js','build.js','capture-env.js','server.js']:copy(source_root/'lego/operator/internal/build/testdata/native-common-init'/n,'fixture/'+n)
for n in ['nativequalification_init_test.go','nativeinit_test.go']:copy(source_root/'lego/operator/internal/build'/n,'source/'+n)
for n in ['main.go','subreaper_linux.go','subreaper_other.go']:copy(source_root/'lego/operator/cmd/native-init'/n,'source/native-init/'+n)
for n in ['RECEIPT.json','evidence/ELF.json','evidence/GOROOT-ROOT.json','evidence/go-version.stdout','evidence/go-version.stderr','evidence/PLATFORM-BINDING.json']:copy(sdk_root/n,'sdk/'+n)
omitted.extend([{'original_path':str(Path(plan['test_binary'])),**test_elf,'reason':'test ELF body omitted'},{'original_path':str(Path(plan['linux_go_root'])/'bin/go'),**sdk_go,'reason':'SDK/tool body omitted'},{'original_path':str(sdk_manifest),**identity(sdk_manifest),'reason':'full17372-path SDK manifest body omitted; Root BEFORE/AFTER exactfullSDK guard receipts included'}])
report={'schema':'bex.actual-native-init-four-launch-independent-review/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'outcome':'CLEAN_LOCAL_ARM64_NATIVE_INIT_FOUR_LAUNCH_DIAGNOSTIC','actual_result':identity(E/'RESULT.json'),'source':SOURCE,'source_paths':300,'source_manifest':identity(Path(plan['source_manifest'])),'all_snapshot_bytes_modes_Git_blob_IDs_verified':True,'common':COMMON,'common_producer_verifier':'1feba939b4697894e800ed0abb4f25a0bf415641','test_ELF':test_elf,'staged_native_init':staged,'compiled_vs_staged':'successful canonical Go test requires exact compiled/staged identity equality; staged body independently hashed/ELF-checked. Original temporary compiled body not separately retained.','native_init_oracle_source':identity(proof_source),'oracle_reapplication':'Read-only source-equivalent identity/address/drain/status checks applied to original retained JSON/logs; no Go or original program execution.','cases':rows,'all_51_direct_children_and_groups_recorded_reaped_absent':True,'temporary_active_case_registry':'34 observer commands retained the running case PGID; all own groups and final registry are absent/reaped.','cleanup':{'four_workload_exact_ID_absences':True,'runner_builder_volume_exact_absences':True,'owner_images_and_containers_absent':True,'config_absent':True},'SDK':{'go_binary':sdk_go,'manifest':identity(sdk_manifest),'Root_before_after':before,'scope':'Reused Root actual17372-path fullSDK BEFORE/AFTER guards; bin/go and manifestfile hashes independently checked, no new compiler execution.'},'original_failed_first_run':'Untouched, not relabeled; outside this capsule.','pending_current_CI':{'run_id':intent['actual_current_CI'],'source':SOURCE,'status_at_launch':'pending; no new CI acceptance inferred by this diagnostic'},'limits':['ONE local ARM64 four-launch native-init diagnostic; no botharch/fullfamily/hosted/Kubernetes/currentCI/baseline/adoption claim.','No Docker/Go/network/cluster/resource/repository actions were performed by this audit.','Held HTTP200 is enforced by the successful exact Go test; original body is retained, raw HTTPheaders/status not separately retained.','All included actual bytes preserved; native-init/testELF/SDK/tool bodies omitted with exact identities. Root fullSDK guard receipts are retained without claiming a repeated17372-body independent audit.']}
dump(OUT/'REVIEW.json',report);copy(OUT/'REVIEW.json','REVIEW.json')
dump(C/'CUSTODY.json',{'files':copied,'omitted':omitted,'scope':'Byte-exact included original regular files; no Docker credential config/auth/cache or executable binary bodies copied. Full original evidence MANIFEST retained, including omitted binary identity.'})
assert sum(p.stat().st_size for p in C.rglob('*') if p.is_file())<CAP
for x in manifest['files']:same(E/x['path'],x)
print(json.dumps({'outcome':report['outcome'],'root':str(OUT),'review':identity(OUT/'REVIEW.json'),'capsule_files':sum(p.is_file() for p in C.rglob('*')),'capsule_bytes':sum(p.stat().st_size for p in C.rglob('*') if p.is_file())},indent=2))

