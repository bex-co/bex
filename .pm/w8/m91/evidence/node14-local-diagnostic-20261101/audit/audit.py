"""Retained actual Node14 donor diagnostic audit; never reruns a command."""
from pathlib import Path
import hashlib,json,os,re,subprocess,types
R=Path(__file__).parent
RUN=Path('/private/tmp/bex-w8-node14-local-pilot-bc3f5cecb438454c85f0edc2');E=RUN/'evidence';C=E/'generated/native-common-node-3815493229/node-14';Q=C/'qa-output'
P=Path('/private/tmp/bex-w8-node14-local-pilot-v4-yarn-rebind-20261101-zejnlirm');F=Path('/private/tmp/bex-w8-linux-family-yarn-compile-20261101-ivbtmxc6')
FIX=F/'source/lego/operator/internal/build/testdata/native-node-qualification'
REF='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
UPSTREAM='node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e'
DONOR='ghcr.io/bex-co/bex-cnb-run@sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc'
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
rows={d['path']:{k:d[k] for k in ('bytes','sha256')} for d in manifest['files']};assert len(rows)==len(manifest['files'])==200
assert {str(p.relative_to(E)) for p in E.rglob('*') if p.is_file()}==set(rows)|{'MANIFEST.json'}
for name,want in rows.items():assert identity(E/name)==want,name
assert identity(P/'run_pilot.py')['sha256']=='e158acde2c44aac290d09f109ddf4f3bf2ca93dc660161dd326fb3ad7cc72c03'
assert identity(P/'PLAN.json')['sha256']=='d178e61ccbc4f1f9adcccb9026780bed17298250fc37d3b0f5575af296321ac0'
assert identity(P/'pilot.sh')['sha256']=='97274b433581d9b0ce2c94b0ad478c1f3661810061ef8da8de17ff3ef70b1004'
assert identity(F/'SOURCE.json')['sha256']==inputs['source_manifest_sha256']==plan['source_manifest_sha256']=='497e75f16b11a723fae593ee6847ea772da638679d06ad9f677333ab8ad28d0a'
source=load(F/'SOURCE.json');assert source['source']==inputs['generator_source']==plan['source_sha']=='6883953e58e750fb41763d1d5e4bd4103bdb74c5' and len(source['files'])==300
for name,d in source['files'].items():
 assert identity(F/'source'/name)=={k:d[k] for k in ('bytes','sha256')} and ((F/'source'/name).stat().st_mode&0o7777)==d['permissions']
assert identity(plan['test_binary'])=={'bytes':41726878,'sha256':'8e7f06b11fcfe0edda9c8b9618b1872cdddcd62f0f3142953b68f12aac1ae45f'}
with Path(plan['test_binary']).open('rb') as f:b=f.read(64)
assert b[:6]==b'\x7fELF\x02\x01' and int.from_bytes(b[18:20],'little')==183
receipt=load(C/'receipt.json');sc=load(C/'source-custody.json');expected=load(C/'expected-node.json')
assert receipt['case']=='node-14' and receipt['common_base']==sc['common']==REF and receipt['owner']==sc['owner']==plan['owner'] and receipt['image_platform']==sc['platform']=='linux/arm64'
assert receipt['network']=='none' and receipt['runtime_command_executed'] is True and receipt['test_prerequisite_overlay'] is False and receipt['static_artifacts_extracted'] is False
assert sc['node_source']==UPSTREAM and sc['node_runtime_artifact']==DONOR and sc['runtime_artifact_mode']=='published-runtime-donors' and sc['runtime_artifact_map'][UPSTREAM]==DONOR and sc['empty_runtime_artifact_map'] is False and sc['test_only_network_overlay'] is False
assert (C/'Dockerfile').read_bytes()==(C/'Dockerfile.canonical').read_bytes() and identity(C/'Dockerfile')['sha256']==sc['canonical_sha256']==sc['executed_sha256']
assert expected['node']==expected['node_env']=='14.21.3' and expected['node_donor']==UPSTREAM and expected['yarn']=='1.22.22' and expected['pnpm']=='10.34.7'
for name in ('addon.c','check.js','check.py','package.json'):assert identity(FIX/name)['sha256']==sc['fixture_sha256'][name]
assert sha(b'14\n')==sc['fixture_sha256']['.node-version'] and sha((C/'expected-node.json').read_bytes()+b'\n')==sc['fixture_sha256']['expected-node.json']
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
 assert primary['node']['modules']==primary['node']['addon']['module_abi']==83 and primary['node']['addon']['headers']=='14.21.3'
 actual={manager:command(manager+'-version-'+phase)['stdout'].strip() for manager in ('npm','yarn','pnpm')}
 tools=parsed('tools-'+phase);assert len(tools)==1;module.validate_tools(tools[0],actual,expected)
 assert actual==primary['tool_versions']=={'npm':'6.14.18','yarn':'1.22.22','pnpm':'10.34.7'}
 assert tools[0]['npm_package_sha256']==primary['npm_package_sha256']
 linked=command('ldd-'+phase);assert 'not found' not in linked['stdout']+linked['stderr']
 if phase=='build':
  command('compile-addon');assert actual==load(Q/'build-tool-versions.json')
  versions=actual
 for manager in ('npm','yarn','pnpm'):
  positive=load(Q/(manager+'-project/package.json'));assert not positive.get('dependencies') and positive['engines']=={'node':'14.x'}
  positives[manager]=identity(Q/(manager+'-project/package.json'))
  if phase=='build':
   installed=command(manager+'-install');assert '--offline' in installed['command']
   events=module.proofs(installed['stdout'])
   for event in ('preinstall','postinstall'):
    matches=[x for x in events if x['lifecycle']==event];assert len(matches)==1;module.validate_lifecycle(matches[0],expected,manager,event)
   rejected=command(manager+'-wrong-engine',False)
   actual_proof=module.validate_engine_rejection(subprocess.CompletedProcess(rejected['command'],rejected['returncode'],rejected['stdout'],rejected['stderr']),manager,'>=15',actual['npm'],cwd=Path('/opt/render/project/src')/rejected['cwd'],selected_node=expected['node'])
   assert actual_proof==load(Q/(manager+'-engine-proof.json'))
  events=parsed(manager+'-run-'+phase);assert len(events)==1;module.validate_lifecycle(events[0],expected,manager,'probe')
 events=parsed('pnpm-exec-'+phase);assert len(events)==1;module.validate_identity(events[0],expected);module.validate_core(events[0])
 proof_summaries.append({'phase':phase,'Node':primary['node']['node'],'ABI':primary['node']['modules'],'NAPI':primary['node']['napi'],'versions':actual,'Yarn_CLI':tools[0]['yarn_cli'],'pnpm_package':tools[0]['pnpm_package'],'npm_package_sha256':tools[0]['npm_package_sha256'],'identity_addon_core_tools_lifecycle_exec':True})
addon=(Q/'bex_node.node').read_bytes();assert addon[:6]==b'\x7fELF\x02\x01' and int.from_bytes(addon[18:20],'little')==183
commands=load(E/'commands.json');assert len(commands)==63
case_position=next(i for i,x in enumerate(commands) if x['label']=='case');case_pgid=commands[case_position]['pgid']
for i,x in enumerate(commands):
 assert x['bound'] is None and x['group_absent'] is True and x['direct_child_reaped'] is True and type(x['pgid']) is int
 if x['pending_pgids']:assert i<case_position and x['pending_pgids']==[case_pgid] and x['label'].startswith(('builder-observe-','builder-du-','owned-stats-'))
def row(label):
 values=[x for x in commands if x['label']==label];assert len(values)==1;return values[0]
case=row('case');assert case['exit_code']==0 and b'--- PASS: TestCommonNodeGeneratedImages/node-14 ' in (E/'case.stdout').read_bytes()
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
summary={'PASS':True,'scope':'ONE actual ARM64 Node14 published-donor local diagnostic; pending amended complete utilityCI baseline, no fullfamily/adoption/hosted/native-init/K8s claim','owner':plan['owner'],'result':result,'source_commit':source['source'],'source_manifest':identity(F/'SOURCE.json'),'ELF':identity(plan['test_binary']),'controller':identity(P/'run_pilot.py'),'PLAN':identity(P/'PLAN.json'),'common':REF,'official_Node14':UPSTREAM,'reviewed_donor':DONOR,'canonical_executed_sha256':sc['canonical_sha256'],'network_overlay':False,'runtime_network':'none (actual receipt and unchanged source-bound run helper)','phases':proof_summaries,'positive_projects':positives,'all_three_engine_negatives_canonical_exit1':True,'npm_yarn_pnpm_pre_post_probe_and_pnpm_exec':True,'addon_ELF_ARM64':True,'commands':len(commands),'all_recorded_command_groups_reaped_absent':True,'runner_builder_volume_labelled_outputs_absent':True,'own_config_absent':True,'original_evidence_files':len(rows),'fixture_emission_custody':{'copied_source_fixture4':sc['fixture_sha256'],'node_version':'14\n from unchanged commonNodeQualificationFiles, hash matches retained custody','expected_node_source':'captured expected-node.json plus one authored finalLF, hash matches retained source custody; original evidence file stays byteexact withoutLF'},'limits':['No tests, Node/native programs, command/resource/network/Go/Docker execution repeated. Existing pure validator functions re-applied only to retained actual outputs.','Some runtime recipe/command facts bind actual retained receipt to exact source/ELF; no new daemon inspection or full runtime image export.','Compiler/test ELF and source300 bodies remain private; portable capsule contains exact hashes/manifests and selected original fixture/program bodies.']}
(R/'AUDIT.json').write_text(json.dumps(summary,indent=2)+'\n');(R/'INPUT-CUSTODY.json').write_text(json.dumps(custody,indent=2)+'\n')
print(json.dumps({'PASS':True,'phases':2,'managers':3,'engine_negatives':3,'commands':len(commands),'original_files':len(rows),'source300':len(source['files']),'actual_Docker_Go_Node_network_resource_commands':False}))
