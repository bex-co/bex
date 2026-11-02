"""Read only retained actual outputs; never rerun an app/native command."""
from pathlib import Path
import ast,base64,hashlib,json,os,re,subprocess,datetime
R=Path(__file__).parent;REPO=Path('/Users/tianpan/projects/bex7');SOURCE='9bc6a5d10b3f479926180698d60a0cfdf238db9b';CURRENT='9039e5037';COMMON='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
BASE=REPO/'.pm/w8/m91/evidence/common-current-e6-20261102/utilities/amd64/artifact/qualification-source.json'
base=json.loads(BASE.read_bytes());custody={};source_rows={}
def sha(b):return hashlib.sha256(b).hexdigest()
def identify(p):
 p=Path(p);assert p.is_file() and not p.is_symlink(),str(p);b=p.read_bytes();d={'bytes':len(b),'sha256':sha(b)};custody[str(p)]=d;return d
def load(p):identify(p);return json.loads(Path(p).read_bytes())
def source(path):
 raw=subprocess.run(['git','show',SOURCE+':'+path],cwd=REPO,capture_output=True,check=True).stdout
 now=subprocess.run(['git','show',CURRENT+':'+path],cwd=REPO,capture_output=True,check=True).stdout
 p=R/'source-fixtures'/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(raw)
 source_rows[path]={'source_sha256':sha(raw),'current9039_sha256':sha(now),'unchanged':raw==now,'bytes':len(raw)}
 return raw
nodebase='lego/operator/internal/build/testdata/native-node-qualification/'
node_raw={n:source(nodebase+n) for n in ('addon.c','check.js','check.py','package.json')}
elixirbase='lego/operator/internal/build/testdata/native-elixir-qualification/'
elixir_raw={n:source(elixirbase+n) for n in ('mix.exs','check.py','check_test.py','bex_nif.c','rebar.config','lib/bex_mix.ex','src/bex_probe.app.src','src/bex_nif.erl','src/bex_probe.erl')}
assert all(x['unchanged'] for x in source_rows.values())
# Execute only unchanged pure predicates, never fixture imports/main/run/identity.
node_names={'validate_identity','validate_core','validate_tools','validate_lifecycle','proofs','validate_engine_rejection'}
tree=ast.parse(node_raw['check.py']);ns={'hashlib':hashlib,'json':json,'os':os,'Path':Path,'PAYLOAD':'bex Node café λ 雪'.encode(),'ENGINE_SENTINEL':'bex-node-engine-sentinel'}
functions=[x for x in tree.body if isinstance(x,ast.FunctionDef) and x.name in node_names];assert len(functions)==len(node_names)
exec(compile(ast.Module(body=functions,type_ignores=[]),'<unchanged-pure-node-predicates>','exec'),ns)
etree=ast.parse(elixir_raw['check.py']);ef=next(x for x in etree.body if isinstance(x,ast.FunctionDef) and x.name=='check_result');es={'base64':base64,'hashlib':hashlib,'TEXT':'bex café λ 雪'};exec(compile(ast.Module(body=[ef],type_ignores=[]),'<unchanged-pure-elixir-predicate>','exec'),es)
items=load(R/'INPUTS.json')['items'];summaries=[]
for item in items:
 key=item['key'];a=R/key/'extracted/artifact';manifest=load(a/'MANIFEST.json');files={x['path']:{k:x[k] for k in ('bytes','sha256')} for x in manifest['files']};assert len(files)==len(manifest['files'])
 assert {str(p.relative_to(a)) for p in a.rglob('*') if p.is_file()}==set(files)|{'MANIFEST.json'}
 for name,want in files.items():assert identify(a/name)==want,name
 s=load(a/'qualification-source.json');assert s['source']==SOURCE and len(s['files'])==300 and s['files']==base['files']
 for p,row in source_rows.items():assert s['files'][p]['sha256']==row['source_sha256']
 inp=load(a/'inputs.json');arch=key.rsplit('-',1)[1];assert inp['platform']=='linux/'+arch and inp['common_image']==COMMON and inp['generator_source']==SOURCE and inp['producer']==inp['verification']=='1feba939b4697894e800ed0abb4f25a0bf415641'
 assert inp['baseline_status']=='current-complete' and inp['utility_baseline_source']==base['source']=='e6a2386101a78d679f729b7ec0d251373c0aa678' and inp['candidate_receipt_sha256']=='ac17d21d7e5a09c8ea41fece0333b8f613900c7b5f793e8bf2edcdb160e59426'
 jobs=load(R/(key+'.jobs.json'));job=next(x for x in jobs['jobs'] if x['id']==item['job']);assert job['status']=='completed' and job['conclusion']=='success' and job['head_sha']==SOURCE
 for step in job['steps']:
  if step['name'] in ("measure the selected family case without claiming full-family acceptance","remove only this job's labeled resources, exact builder and credentials","hash retained successes and failures") or 'upload-artifact@' in step['name']:assert step['conclusion']=='success'
 log=(R/(key+'.job.log')).read_text();identify(R/(key+'.job.log'));plain=re.sub(r'^\ufeff?\d{4}-\d\d-\d\dT\S+ ', '',log,flags=re.M)
 qual=(a/'qualification.log').read_text();assert qual.strip() in plain and qual.count('--- PASS:')==2 and '--- FAIL:' not in qual and '--- SKIP:' not in qual
 cleanup=load(a/'cleanup.json');assert cleanup['owner']==inp['owner'] and all(x['exit_code']==0 for x in cleanup['operations']);auth=next(x for x in cleanup['operations'] if x['resource']=='docker-auth');assert auth['logout_exit_code']==0 and auth['removed'] is True
 assert any('"owner": "'+inp['owner']+'"' in line for line in log.splitlines())
 generated=next((a/'generated').glob('*/*'));rec=load(generated/'receipt.json');assert rec['owner']==rec['builder']==inp['owner'] and rec['common_base']==COMMON and rec['image_platform']=='linux/'+arch and rec['network']=='none' and rec['runtime_command_executed'] is True and rec['static_artifacts_extracted'] is False and rec['test_prerequisite_overlay'] is False
 canonical=(generated/'Dockerfile.canonical').read_bytes();assert canonical==(generated/'Dockerfile').read_bytes();assert COMMON.encode() in canonical and b'--network=default' not in canonical
 artifact_doc=load(R/('node22-artifacts.json' if key.startswith('node22') else 'elixir117-artifacts.json'));am=next(x for x in artifact_doc['artifacts'] if x['id']==item['artifact']);assert am['size_in_bytes']==item['zip_bytes'] and am['digest']=='sha256:'+item['zip_sha256'] and not am['expired'] and am['workflow_run']['id']==item['run'] and am['workflow_run']['head_sha']==SOURCE
 assert identify(R/key/'original.zip')=={'bytes':item['zip_bytes'],'sha256':item['zip_sha256']}
 q=generated/'qa-output';summary={'key':key,'run':item['run'],'job':item['job'],'artifact':item['artifact'],'arch':arch,'actual_generator_source':SOURCE,'functional300_equal_e6':True,'original_members':len(files)+1,'canonical_equals_executed_sha256':sha(canonical),'image_config_id':rec['image_config_id'],'network':'none','cleanup':'ActualCIhelperACK0/builderremoved/logout0/configremoved pluswholejobSUCCESS; no newdaemoninspection.'}
 if key.startswith('node22'):
  expected=load(generated/'expected-node.json');sc=load(generated/'source-custody.json');assert expected['node']==expected['node_env']=='22.23.4' and expected['yarn']=='1.22.22' and expected['pnpm']=='10.34.7'
  assert sc['canonical_sha256']==sc['executed_sha256']==sha(canonical) and sc['owner']==inp['owner'] and sc['platform']=='linux/arm64' and sc['common']==COMMON and sc['empty_runtime_artifact_map'] is True and sc['test_only_network_overlay'] is False and sc['node_source']==sc['node_runtime_artifact']==expected['node_donor']
  for name,b in node_raw.items():assert sha(b)==sc['fixture_sha256'][name]
  assert sha(b'22\n')==sc['fixture_sha256']['.node-version'] and sha((generated/'expected-node.json').read_bytes()+b'\n')==sc['fixture_sha256']['expected-node.json']
  def command(name,success=True):
   x=load(q/(name+'.json'));assert x['timed_out'] is False and type(x['returncode']) is int and x['returncode']==(0 if success else 1);return x
  def parsed(name):return ns['proofs'](command(name)['stdout'])
  phases=[]
  for phase in ('build','runtime'):
   primary=load(q/('node-'+phase+'.json'));assert primary['pass'] is True and primary['phase']==phase;ns['validate_identity'](primary['node'],expected);ns['validate_core'](primary['node']);assert primary['node']['modules']==primary['node']['addon']['module_abi']==127
   versions={m:command(m+'-version-'+phase)['stdout'].strip() for m in ('npm','yarn','pnpm')};assert versions==primary['tool_versions']=={'npm':'10.9.9','yarn':'1.22.22','pnpm':'10.34.7'}
   tools=parsed('tools-'+phase);assert len(tools)==1;ns['validate_tools'](tools[0],versions,expected);assert tools[0]['npm_package_sha256']==primary['npm_package_sha256']
   linked=command('ldd-'+phase);assert 'not found' not in linked['stdout']+linked['stderr']
   if phase=='build':command('compile-addon');assert versions==load(q/'build-tool-versions.json')
   for manager in ('npm','yarn','pnpm'):
    positive=load(q/(manager+'-project/package.json'));assert not positive.get('dependencies') and positive['engines']=={'node':'22.x'}
    if phase=='build':
     installed=command(manager+'-install');assert '--offline' in installed['command'];events=ns['proofs'](installed['stdout'])
     for event in ('preinstall','postinstall'):
      matches=[x for x in events if x['lifecycle']==event];assert len(matches)==1;ns['validate_lifecycle'](matches[0],expected,manager,event)
     rejected=command(manager+'-wrong-engine',False);ep=ns['validate_engine_rejection'](subprocess.CompletedProcess(rejected['command'],rejected['returncode'],rejected['stdout'],rejected['stderr']),manager,'>=23',versions['npm'],cwd=Path('/opt/render/project/src')/rejected['cwd'],selected_node=expected['node']);assert ep==load(q/(manager+'-engine-proof.json'))
    events=parsed(manager+'-run-'+phase);assert len(events)==1;ns['validate_lifecycle'](events[0],expected,manager,'probe')
   events=parsed('pnpm-exec-'+phase);assert len(events)==1;ns['validate_identity'](events[0],expected);ns['validate_core'](events[0]);phases.append({'phase':phase,'versions':versions,'ABI':127,'identity_addon_crypto_Brotli_lifecycle':True,'Yarn_cli':tools[0]['yarn_cli']})
  addon=(q/'bex_node.node').read_bytes();assert addon[:6]==b'\x7fELF\x02\x01' and int.from_bytes(addon[18:20],'little')==183
  summary.update(Node='22.23.4',phases=phases,addon_ELF_machine=183,all3_engine_negatives=True,selected_upstream=sc['node_source'])
 else:
  expected=load(generated/'expected.json');assert expected=={'elixir':'1.17.3','env':{'ELIXIR_VERSION':'v1.17.3','MIX_ENV':'prod','OTP_VERSION':'27.3.4.19','REBAR3_VERSION':'3.27.0','REBAR_VERSION':'2.6.4'},'node':'24.22.1','otp':'27.3.4.19'}
  assert inp['selection']['profile']=='elixir' and inp['selection']['case']=='elixir-1.17'
  assert b'FROM elixir:1.17@' not in canonical and b'COPY --link --from=elixir:1.17@sha256:0ff6cbfc498e86d21d1f8939672f32acaffe9e5be8924da7942ce69fd34e2c68 /usr/local/ /usr/local/' in canonical
  assert b'check.py build' in canonical and b'check.py runtime' in canonical and b'ENV MIX_ENV=prod' in canonical
  commands=[json.loads(x) for x in (q/'commands.jsonl').read_text().splitlines()];assert len(commands)==19 and all(x['exit']==0 for x in commands)
  compile_row=next(x for x in commands if x['command'][0]=='cc');assert compile_row['command']==['cc','-shared','-fPIC','-Wall','-Wextra','-Werror','-I/usr/local/lib/erlang/usr/include','bex_nif.c','-o','priv/bex_nif.so','-lffi','-lssl','-lcrypto']
  for needed in [['rebar','compile'],['rebar3','as','prod','compile'],['elixir','-pa','_build/prod/lib/bex_probe/ebin','-S','mix','compile','--warnings-as-errors']]:assert len([x for x in commands if x['command']==needed])==1
  build_plain=re.sub(r'^#\d+ [0-9.]+ ?', '',(generated/'build.log').read_text(),flags=re.M)
  for phase in ('build','runtime'):
   proof=load(q/(phase+'.json'));assert proof['pass'] is True and proof['phase']==phase and proof['expected']==expected and proof['otp_root']=='/usr/local/lib/erlang';es['check_result'](proof['erlang_result']);es['check_result'](proof['elixir_result'])
   assert re.search(r'(?<![\d.])2\.6\.4(?![\d.])',proof['rebar_versions']['rebar']) and re.search(r'(?<![\d.])3\.27\.0(?![\d.])',proof['rebar_versions']['rebar3'])
   for key2 in ('erlang_result','elixir_result'):assert len([x for x in commands if x['stdout'].strip()==proof[key2]])>=2
   if phase=='build':assert (q/'build.json').read_text().strip() in build_plain
   else:assert (q/'runtime.json').read_bytes()==(generated/'runtime.log').read_bytes()
  summary.update(Elixir='1.17.3',OTP='27.3.4.19',Node='24.22.1',rebar='2.6.4',rebar3='3.27.0',actual_commands=19,matching_OTP_headers_C_NIF_OpenSSL_FFI_rebar_Mix_Erlang_and_Elixir_results=True,ELF_scope='Runtimeimageplatform independentlyrecorded; NIFcompiled/loadedbybothlanguages in actualsuccess. NIF/OTPbinaryELFbytes not exported, so no standalonebinaryELF claim. NoHTTPserveroracles inthisfixture.')
 summaries.append(summary)
amd_root=Path('/private/tmp/bex-w8-node22-26-amd-independent-20261102-vt29xqvb');amd=load(amd_root/'REVIEW.json');amd_case=next(x for x in amd['cases'] if x['case']=='node-22');assert amd_case['assessment']=='CLEAN' and amd_case['Node']=='22.23.4' and amd_case['source_header']==SOURCE and amd_case['canonical_equals_executed']==summaries[0]['canonical_equals_executed_sha256']
report={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'assessment':'CLEAN_ACTUAL_OLD_SOURCE_NODE22_ARM_AND_ELIXIR117_BOTH_ARCH','common':COMMON,'source':SOURCE,'current_source9039_accepted':False,'items':summaries,'Node22_paired_AMD_independent':{'path':str(amd_root),'review':identify(amd_root/'REVIEW.json'),'job':amd_case['job_id'],'artifact':amd_case['artifact_id'],'scope':'ExistingindependentAMDactualaudit reused, notrefetched/reexecuted; pair oldsource9bc only.'},'case_fixture_sources_9039_unchanged':source_rows,'limits':['Oldsource9bc only; all300capturedfiles equal historicale6baseline; revised9039fullbaseline currentlypending separateCI. No newsourceauthority/adoption/hosted/fullfamily conclusion.','Retainedactualmetadata/proof/CRC/SHA/ELF audit only; noGo/native/app/Node/Elixir/image/Docker/networkruntimecommand reexecuted.','ActualsourceretainedENV/PATH/version/cwd assertions bind emittedproofsuccess; not an independentfreshenvdump.','CIownerbuilderremove/configlogout+removedACKs and wholejobsuccess retained; no newdaemoninspection or inferredlocalprocessgroupreceipt.']}
(R/'AUDIT.json').write_text(json.dumps(report,indent=2)+'\n');(R/'INPUT-CUSTODY.json').write_text(json.dumps(custody,indent=2)+'\n');print(json.dumps({'assessment':report['assessment'],'items':[x['key'] for x in summaries],'report_sha256':identify(R/'AUDIT.json')['sha256']}))
