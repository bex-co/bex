"""Retained actual Node18 donor diagnostic audit; never reruns a command."""
from pathlib import Path
import hashlib,json,os,re,subprocess,types
R=Path(__file__).parent
RUN=Path('/private/tmp/bex-w8-node18-local-pilot-33f4dbbf9ad545fc81992e01');E=RUN/'evidence';C=E/'generated/native-common-node-1783532027/node-18';Q=C/'qa-output'
P=Path('/private/tmp/bex-w8-node18-local-pilot-source-20261101-al0nmy2i');F=Path('/private/tmp/bex-w8-linux-nativeinit-yarn-compile-20261101-8ir29bky')
FIX=F/'source/lego/operator/internal/build/testdata/native-node-qualification'
REF='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
UPSTREAM='node:18-bookworm@sha256:c6ae79e38498325db67193d391e6ec1d224d96c693a8a4d943498556716d3783'
DONOR='ghcr.io/bex-co/bex-cnb-run@sha256:6c40aaf4af55979e9b923e5949704f562d8e334e7370022f0c3584341742ddd1'
custody={}
def sha(b):return hashlib.sha256(b).hexdigest()
def identity(p):
 p=Path(p);assert p.is_file() and not p.is_symlink();h=hashlib.sha256();n=0
 with p.open('rb') as f:
  while b:=f.read(1<<20):h.update(b);n+=len(b)
 d={'bytes':n,'sha256':h.hexdigest()};custody[str(p)]=d;return d
def load(p):identity(p);return json.loads(Path(p).read_bytes())
plan=load(P/'PLAN.json');result=load(E/'RESULT.json');inputs=load(E/'inputs.json');manifest=load(E/'MANIFEST.json')
assert result['PASS'] is True and result['case_pass'] is True and result['failure'] is None and result['cleanup_or_artifact_errors']==[] and result['cancel_signal'] is None and result['pending_pgids']==[]
assert result['owner']==inputs['owner']==plan['owner'] and inputs['common_image']==plan['common_image']==REF and inputs['platform']==plan['platform']=='linux/arm64'
rows={d['path']:{k:d[k] for k in ('bytes','sha256')} for d in manifest['files']};assert len(rows)==len(manifest['files'])==206
assert {str(p.relative_to(E)) for p in E.rglob('*') if p.is_file()}==set(rows)|{'MANIFEST.json'}
for name,want in rows.items():assert identity(E/name)==want,name
assert identity(P/'run_pilot.py')['sha256']=='c97c35c9fce020a5067e4c22824bd2ade42e88390b5d7e28dcd394b7bcd66a3e'
assert identity(P/'PLAN.json')['sha256']=='62ecff88865426f56ce742ed849156c1c6beb3091609587677076599953d210d'
assert identity(P/'pilot.sh')['sha256']=='e3fe9187d2085bfedfd6cb35981ab46a702a1ccbf6d28b04a484a9a5d5bcfbe0'
assert identity(F/'SOURCE.json')['sha256']==inputs['source_manifest_sha256']==plan['source_manifest_sha256']=='17929596653a4bf08b9f2be4765c3b738241a97c84a2d87a73e7ad81edfa1d1a'
source=load(F/'SOURCE.json');assert source['source']==inputs['generator_source']==plan['source_sha']=='ea9375c1c81e5a58efe354b164bbaf6934a044fa' and len(source['files'])==300
for name,d in source['files'].items():
 assert identity(F/'source'/name)=={k:d[k] for k in ('bytes','sha256')} and ((F/'source'/name).stat().st_mode&0o7777)==d['permissions']
assert identity(plan['test_binary'])=={'bytes':41727286,'sha256':'f87d0fc88e4a5ee8b2e7abf63fdcc017832d4eceaac84d4098a7e0b7f0e2055d'}
with Path(plan['test_binary']).open('rb') as f:b=f.read(64)
assert b[:6]==b'\x7fELF\x02\x01' and int.from_bytes(b[18:20],'little')==183
receipt=load(C/'receipt.json');sc=load(C/'source-custody.json');expected=load(C/'expected-node.json')
assert receipt['case']=='node-18' and receipt['common_base']==sc['common']==REF and receipt['owner']==sc['owner']==plan['owner'] and receipt['image_platform']==sc['platform']=='linux/arm64'
assert receipt['network']=='none' and receipt['runtime_command_executed'] is True and receipt['test_prerequisite_overlay'] is False and receipt['static_artifacts_extracted'] is False
assert sc['node_source']==UPSTREAM and sc['node_runtime_artifact']==DONOR and sc['runtime_artifact_mode']=='published-runtime-donors' and sc['runtime_artifact_map'][UPSTREAM]==DONOR and sc['empty_runtime_artifact_map'] is False and sc['test_only_network_overlay'] is False
assert (C/'Dockerfile').read_bytes()==(C/'Dockerfile.canonical').read_bytes() and identity(C/'Dockerfile')['sha256']==sc['canonical_sha256']==sc['executed_sha256']
assert expected['node']==expected['node_env']=='18.20.8' and expected['node_donor']==UPSTREAM and expected['yarn']=='1.22.22' and expected['pnpm']=='10.34.7'
for name in ('addon.c','check.js','check.py','package.json'):assert identity(FIX/name)['sha256']==sc['fixture_sha256'][name]
assert sha(b'18\n')==sc['fixture_sha256']['.node-version'] and sha((C/'expected-node.json').read_bytes()+b'\n')==sc['fixture_sha256']['expected-node.json']
module=types.ModuleType('captured_node_oracle');module.__file__=str(FIX/'check.py');exec(compile((FIX/'check.py').read_bytes(),str(FIX/'check.py'),'exec'),module.__dict__)
assert identity(FIX/'check.py')['sha256']=='c7e16114b0141ad7f98c13d9425d16f2957e21ffe4c47b254bb27f551f5fa748'
versions={};proof_summaries=[];positives={}
def command(name,success=True):
 x=load(Q/(name+'.json'));assert x['timed_out'] is False and type(x['returncode']) is int and x['returncode']==(0 if success else 1)
 return x
def parsed(name):return module.proofs(command(name)['stdout'])
for phase in ('build','runtime'):
 primary=load(Q/('node-'+phase+'.json'));assert primary['pass'] is True and primary['phase']==phase
 module.validate_identity(primary['node'],expected);module.validate_core(primary['node'])
 assert primary['node']['modules']==primary['node']['addon']['module_abi']==108 and primary['node']['addon']['headers']=='18.20.8'
 actual={manager:command(manager+'-version-'+phase)['stdout'].strip() for manager in ('npm','yarn','pnpm')}
 tools=parsed('tools-'+phase);assert len(tools)==1;module.validate_tools(tools[0],actual,expected)
 assert actual==primary['tool_versions']=={'npm':'10.8.2','yarn':'1.22.22','pnpm':'10.34.7'}
 assert tools[0]['npm_package_sha256']==primary['npm_package_sha256']
 linked=command('ldd-'+phase);assert 'not found' not in linked['stdout']+linked['stderr']
 if phase=='build':
  command('compile-addon');assert actual==load(Q/'build-tool-versions.json')
  versions=actual
 for manager in ('npm','yarn','pnpm'):
  positive=load(Q/(manager+'-project/package.json'));assert not positive.get('dependencies') and positive['engines']=={'node':'18.x'}
  positives[manager]=identity(Q/(manager+'-project/package.json'))
  if phase=='build':
   installed=command(manager+'-install');assert '--offline' in installed['command']
   events=module.proofs(installed['stdout'])
   for event in ('preinstall','postinstall'):
    matches=[x for x in events if x['lifecycle']==event];assert len(matches)==1;module.validate_lifecycle(matches[0],expected,manager,event)
   rejected=command(manager+'-wrong-engine',False)
   actual_proof=module.validate_engine_rejection(subprocess.CompletedProcess(rejected['command'],rejected['returncode'],rejected['stdout'],rejected['stderr']),manager,'>=19',actual['npm'],cwd=Path('/opt/render/project/src')/rejected['cwd'],selected_node=expected['node'])
   assert actual_proof==load(Q/(manager+'-engine-proof.json'))
  events=parsed(manager+'-run-'+phase);assert len(events)==1;module.validate_lifecycle(events[0],expected,manager,'probe')
 events=parsed('pnpm-exec-'+phase);assert len(events)==1;module.validate_identity(events[0],expected);module.validate_core(events[0])
 proof_summaries.append({'phase':phase,'Node':primary['node']['node'],'ABI':primary['node']['modules'],'NAPI':primary['node']['napi'],'versions':actual,'Yarn_CLI':tools[0]['yarn_cli'],'pnpm_package':tools[0]['pnpm_package'],'npm_package_sha256':tools[0]['npm_package_sha256'],'identity_addon_core_tools_lifecycle_exec':True})
addon=(Q/'bex_node.node').read_bytes();assert addon[:6]==b'\x7fELF\x02\x01' and int.from_bytes(addon[18:20],'little')==183
commands=load(E/'commands.json');assert len(commands)==66
case_position=next(i for i,x in enumerate(commands) if x['label']=='case');case_pgid=commands[case_position]['pgid']
for i,x in enumerate(commands):
 assert x['bound'] is None and x['group_absent'] is True and x['direct_child_reaped'] is True and type(x['pgid']) is int
 if x['pending_pgids']:assert i<case_position and x['pending_pgids']==[case_pgid] and x['label'].startswith(('builder-observe-','builder-du-','owned-stats-'))
def row(label):
 values=[x for x in commands if x['label']==label];assert len(values)==1;return values[0]
case=row('case');assert case['exit_code']==0 and b'--- PASS: TestCommonNodeGeneratedImages/node-18 ' in (E/'case.stdout').read_bytes()
assert (E/'socket-alias-preflight.txt').read_text().strip()=='linux/arm64' and (E/'buildkit-pull.exit').read_text().strip()=='0'
created=row('runner-create');runner=load(E/'runner-cid.json');assert created['exit_code']==0 and (E/created['stdout']).read_text().strip()==runner['id']
argv=created['argv'];assert argv[argv.index('--memory')+1]=='512m' and argv[argv.index('--cpus')+1]=='2' and argv[argv.index('--platform')+1]=='linux/arm64'
env={argv[i+1].split('=',1)[0]:argv[i+1].split('=',1)[1] for i,x in enumerate(argv) if x=='--env'}
assert env['BEX_NATIVE_TEST_NODE_DONORS']=='1' and env['BEX_NATIVE_TEST_INIT_PROFILE']=='0' and env['BEX_NATIVE_TEST_COMMON_IMAGE']==REF
assert env['DOCKER_CONFIG']==inputs['docker_config'] and env['DOCKER_HOST']==plan['endpoint']
for label in ('before-runner','before-builder','runner-cleanup-after','builder-cleanup-after'):
 x=row(label);assert x['exit_code']==1 and (E/x['stdout']).read_bytes().strip() in (b'',b'[]')
 assert ('No such container: '+x['argv'][-1]) in (E/x['stderr']).read_text() or ('No such object: '+x['argv'][-1]) in (E/x['stderr']).read_text()
for label in ('before-volume','volume-cleanup-after'):
 x=row(label);assert x['exit_code']==1 and plan['owner'] in (E/x['stderr']).read_text() and 'no such volume' in (E/x['stderr']).read_text().lower()
for label in ('container-owned-absence','image-owned-absence'):
 x=row(label);assert x['exit_code']==0 and (E/x['stdout']).read_bytes().strip()==b''
 assert x['argv'][-1]=='label=co.bex.native-qualification='+plan['owner']
cleanup=load(E/'cleanup.json');assert cleanup['owner']==plan['owner']
for op in cleanup['operations']:assert op['exit_code']==0
auth=[x for x in cleanup['operations'] if x['resource']=='docker-auth'];assert len(auth)==1 and auth[0]['logout_exit_code']==0 and auth[0]['removed'] is True and auth[0]['identity']==inputs['docker_config']
assert not Path(inputs['docker_config']).exists()
helper=load(E/'helper-cleanup-command.json');assert helper['exit_code']==0 and helper['argv'][1]==str(F/'source/.github/scripts/native-common-qualify.py') and helper['argv'][-2:]==['cleanup',str(E)]
for p in E.glob('builder-du-*.stdout'):assert int(p.read_bytes().split()[0])*1024<=plan['bounds']['builder_state_polled_bytes']
summary={'PASS':True,'scope':'ONE actual ARM64 Node18 published-donor local diagnostic; local diagnostic source snapshot only, no fullfamily/adoption/hosted/native-init/K8s claim','owner':plan['owner'],'result':result,'source_commit':source['source'],'source_manifest':identity(F/'SOURCE.json'),'ELF':identity(plan['test_binary']),'controller':identity(P/'run_pilot.py'),'PLAN':identity(P/'PLAN.json'),'common':REF,'official_Node18':UPSTREAM,'reviewed_donor':DONOR,'canonical_executed_sha256':sc['canonical_sha256'],'network_overlay':False,'runtime_network':'none (actual receipt and unchanged source-bound run helper)','phases':proof_summaries,'positive_projects':positives,'all_three_engine_negatives_canonical_exit1':True,'npm_yarn_pnpm_pre_post_probe_and_pnpm_exec':True,'addon_ELF_ARM64':True,'commands':len(commands),'all_recorded_command_groups_reaped_absent':True,'runner_builder_volume_labelled_outputs_absent':True,'own_config_absent':True,'original_evidence_files':len(rows),'fixture_emission_custody':{'copied_source_fixture4':sc['fixture_sha256'],'node_version':'18\n from unchanged commonNodeQualificationFiles, hash matches retained custody','expected_node_source':'captured expected-node.json plus one authored finalLF, hash matches retained source custody; original evidence file stays byteexact withoutLF'},'limits':['No tests, Node/native programs, command/resource/network/Go/Docker execution repeated. Existing pure validator functions re-applied only to retained actual outputs.','Some runtime recipe/command facts bind actual retained receipt to exact source/ELF; no new daemon inspection or full runtime image export.','Compiler/test ELF and source300 bodies remain private; portable capsule contains exact hashes/manifests and selected original fixture/program bodies.']}
(R/'AUDIT.json').write_text(json.dumps(summary,indent=2)+'\n');(R/'INPUT-CUSTODY.json').write_text(json.dumps(custody,indent=2)+'\n')
print(json.dumps({'PASS':True,'phases':2,'managers':3,'engine_negatives':3,'commands':len(commands),'original_files':len(rows),'source300':len(source['files']),'actual_Docker_Go_Node_network_resource_commands':False}))
