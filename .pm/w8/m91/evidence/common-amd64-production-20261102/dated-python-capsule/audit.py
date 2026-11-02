"""Read-only five dated actual Python outputs; architecture/profile input adaptation only."""
from pathlib import Path
import ast,base64,datetime,hashlib,json,re,subprocess
import sys
_SUPPORT = Path(__file__).resolve().parent/'support'
for _row in json.loads((_SUPPORT/'SOURCE.json').read_bytes())['files']:
 _body=(_SUPPORT/_row['path']).read_bytes();assert len(_body)==_row['bytes'] and hashlib.sha256(_body).hexdigest()==_row['sha256']
sys.path.insert(0,str(_SUPPORT))
from packaging.version import Version
R=Path(__file__).resolve().parent;REPO=Path('/Users/tianpan/projects/bex7');SOURCE='9bc6a5d10b3f479926180698d60a0cfdf238db9b'
COMMON='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0';PRODUCER='1feba939b4697894e800ed0abb4f25a0bf415641'
def need(ok,why):
 if not ok:raise AssertionError(why)
def sha(body):return hashlib.sha256(body).hexdigest()
def load(p):return json.loads(Path(p).read_bytes())
def identity(p):b=Path(p).read_bytes();return {'bytes':len(b),'sha256':sha(b)}
def git(path):return subprocess.check_output(['git','show',SOURCE+':'+path],cwd=REPO)
def pure(path,name):
 body=git(path);tree=ast.parse(body);node=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name==name);space={'Version':Version};exec(compile(ast.Module(body=[node],type_ignores=[]),path,'exec'),space);return space[name],{'path':path,'source_sha256':sha(body),'function':name,'function_AST_sha256':sha(ast.dump(node,include_attributes=False).encode())}
validate_app,app_source=pure('lego/operator/internal/build/testdata/native-legacy-python/check.py','validate_identity')
validate_modern,modern_tool_source=pure('lego/operator/internal/build/testdata/native-modern-python/check-tool.py','validate_tool')
validate_legacy,legacy_tool_source=pure('lego/operator/internal/build/testdata/native-legacy-python/check-tool.py','validate_tool')
baseline=load(REPO/'.pm/w8/m91/evidence/common-current-e6-20261102/utilities/amd64/artifact/qualification-source.json')
expected_source=load(R/'EXPECTED-SOURCE.json')
need(baseline['files']==expected_source['files'] and expected_source['source']==SOURCE,'anchored dated source closure')
results=[]
for item in load(R/'INPUTS.json'):
 line=item['case'];version=item['version'];arch=item['architecture'];legacy=item['profile']=='legacy-python';platform='linux/'+arch;case=R/'outputs'/item['key'];A=case/'extracted/artifact';extraction=load(case/'extracted/extraction.json');entries=load(A/'MANIFEST.json')['files'];rows={x['path']:x for x in entries}
 validate_tool=(lambda data,_version:validate_legacy(data)) if legacy else validate_modern
 solve="poetry==2.1.3 'virtualenv<20.33.0'" if legacy else 'poetry==2.1.3'
 actual={str(p.relative_to(A)) for p in A.rglob('*') if p.is_file()};need(actual==set(rows)|{'MANIFEST.json'} and len(rows)==len(entries),'manifest bijection')
 need(extraction['complete'] is True and extraction['completed_members']==len(actual),'actual extraction incomplete')
 for n,row in rows.items():need(identity(A/n)=={k:row[k] for k in ('bytes','sha256')},'retained member changed:'+n)
 inp=load(A/'inputs.json');captured=load(A/'qualification-source.json');need(captured['source']==SOURCE and captured['files']==baseline['files'] and len(captured['files'])==300,'actual source header/files differ')
 need(inp['common_image']==COMMON and inp['producer']==inp['verification']==PRODUCER and inp['generator_source']==SOURCE and inp['platform']==platform,'image/generator role differs')
 need(inp['selection']['profile']==item['profile'] and inp['selection']['case']==line and inp['selection']['selector']=='^'+('TestNativeLegacyPythonGeneratedImages' if legacy else 'TestNativeModernPythonGeneratedImages')+'$/^'+re.escape(line)+'$' and inp['selection']['flag']==('BEX_NATIVE_LEGACY_PYTHON_INTEGRATION' if legacy else 'BEX_NATIVE_TEST_MODERN_PYTHON_PROFILE'),'selected case differs')
 need(inp['baseline_status']=='current-complete' and inp['utility_baseline_source']==baseline['source'] and inp['candidate_receipt_sha256']=='ac17d21d7e5a09c8ea41fece0333b8f613900c7b5f793e8bf2edcdb160e59426','baseline receipt differs')
 job=next(j for j in load(R/'metadata'/('jobs-'+str(item['run_id'])+'.json'))['jobs'] if j['id']==item['job_id']);need(job['head_sha']==SOURCE and job['run_id']==item['run_id'] and job['run_attempt']==1 and job['status']=='completed' and job['conclusion']=='success','actual job identity/status')
 steps={s['name']:s for s in job['steps']};gate='measure the selected family case without claiming full-family acceptance';need(steps[gate]['conclusion']=='success' and steps['remove only this job\'s labeled resources, exact builder and credentials']['conclusion']=='success','actual measure/cleanup gates')
 archive=next(x for x in load(R/'metadata'/('artifacts-'+str(item['run_id'])+'.json'))['artifacts'] if x['id']==item['artifact_id'])
 need(archive['workflow_run']['head_sha']==SOURCE and archive['workflow_run']['id']==item['run_id'] and archive['size_in_bytes']==extraction['zip_bytes'] and archive['digest']=='sha256:'+extraction['zip_sha256'],'actual artifact ZIP provenance')
 ci=(case/'job.log').read_bytes();raw=b''.join(re.sub(rb'^\xef\xbb\xbf',b'',re.sub(rb'^\d{4}-\d\d-\d\dT\S+Z ',b'',line)) for line in ci.splitlines(keepends=True));ql=(A/'qualification.log').read_bytes();need(ql in raw,'artifact Go log detached from actual CI')
 passed=re.findall(rb'--- (PASS|FAIL|SKIP): (\S+) \(',ql);need(len(passed)==2 and all(p[0]==b'PASS' for p in passed) and ('TestNativeLegacyPythonGeneratedImages/' if legacy else 'TestNativeModernPythonGeneratedImages/').encode()+line.encode() in ql,'selected Go case did not pass')
 cleanup=load(A/'cleanup.json');need(cleanup['owner']==inp['owner'] and all(x['exit_code']==0 for x in cleanup['operations']),'cleanup receipt')
 auth=next(x for x in cleanup['operations'] if x['resource']=='docker-auth');need(auth['removed'] is True and auth['logout_exit_code']==0,'own auth cleanup')
 expected_cleanup={'owner':cleanup['owner'],'operations':[{k:v for k,v in x.items() if x['resource']!='docker-auth' or k in ('resource','exit_code','logout_exit_code','removed','removal_error')} for x in cleanup['operations']]}
 packets=[json.loads(s.split('BEX_NATIVE_CLEANUP_RESULT ',1)[1]) for s in ci.decode().splitlines() if 'BEX_NATIVE_CLEANUP_RESULT ' in s];need(packets==[expected_cleanup],'cleanup/log binding')
 roots=list((A/'generated').iterdir());need(len(roots)==1,'one profile root');G=roots[0]/version;need(G.is_dir(),'exact patch evidence absent')
 sc=load(G/'source-custody.json');receipt=load(G/'receipt.json');canonical=(G/'Dockerfile.canonical').read_bytes();executed=(G/'Dockerfile').read_bytes();overlay=b'COPY .bex-test-wheels/ /opt/bex-test-wheels/\nENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n';marker=('\nFROM '+COMMON+'\n').encode()
 need(canonical.count(marker)==1 and executed==canonical.replace(marker,marker+overlay,1) and ("--no-input "+solve+" && ") in canonical.decode() and (legacy or 'virtualenv<' not in canonical.decode()),'canonical/wheel-only overlay or legacycap changed')
 need(sc['canonical_sha256']==sha(canonical) and sc['executed_sha256']==sha(executed) and sc['common']==COMMON and sc['platform']==platform and sc['owner']==sc['builder']==inp['owner'] and sc['wheel_solve']==solve,'source custody')
 fixture={n:git('lego/operator/internal/build/testdata/native-legacy-python/'+n).decode().replace('@APP_VERSION@',version).encode() for n in ('pyproject.toml','setup.py','legacy_probe.c','check.py')};fixture['check-tool.py']=git('lego/operator/internal/build/testdata/'+('native-legacy-python' if legacy else 'native-modern-python')+'/check-tool.py').decode().replace('@APP_VERSION@',version).encode();
 if not legacy:fixture['.node-version']=b'24\n'
 need({n:sha(b) for n,b in fixture.items()}==sc['fixture_sha256'],'unchanged fixture emission')
 need(receipt['common_base']==COMMON and receipt['image_platform']==platform and receipt['network']=='none' and receipt['runtime_command_executed'] is True and receipt['test_prerequisite_overlay'] is True and receipt['owner']==inp['owner'] and re.fullmatch('sha256:[a-f0-9]{64}',receipt['image_config_id']),'actual runtime config/platform')
 wm=load(G/'test-wheelhouse.json')['files'];wheels=G/'prepared-wheels/wheels';need({p.name for p in wheels.iterdir()}=={x['filename'] for x in wm},'complete wheel closure')
 for row in wm:need(identity(wheels/row['filename'])=={k:row[k] for k in ('bytes','sha256')},'wheel identity')
 need(any(x['filename'].startswith('poetry-2.1.3-') for x in wm) and not any(x['filename'].lower().startswith(('django-','pyvips-')) for x in wm),'Poetry-only closure')
 meta=(G/'tool-python.json').read_bytes();tool=json.loads(meta);validate_tool(tool,version);need(tool['machine']=={'amd64':'x86_64','arm64':'aarch64'}[arch] and tool['os']=='Linux' and meta==(G/'prepared-wheels/tool-python.json').read_bytes() and sc['tool_metadata_sha256']==sha(meta) and sc['wheel_manifest_sha256']==sha((G/'test-wheelhouse.json').read_bytes()),'selected target tool metadata')
 if legacy:
  boot=[x for x in canonical.decode().splitlines() if x.startswith('RUN /usr/bin/python3 -m venv /opt/bex/poetry && ')]
  need(len(boot)==1 and '--no-input '+solve+' && ' in boot[0],'canonical legacy system-Python default bootstrap')
  prereq='# syntax=docker/dockerfile:1.7\nFROM '+COMMON+' AS bex-test-prerequisites\nRUN --network=default /usr/bin/python3 -m venv /opt/bex/test-wheel-builder && /opt/bex/test-wheel-builder/bin/pip wheel --disable-pip-version-check --only-binary=:all: --wheel-dir /opt/bex-test-wheels '+solve+'\nENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n'+boot[0].replace('RUN ','RUN --network=none ',1)+'\nCOPY check-tool.py /opt/bex-test-check-tool.py\nRUN --network=none /opt/bex/poetry/bin/python -I /opt/bex-test-check-tool.py > /opt/bex-test-tool-python.json\nFROM scratch\nCOPY --from=bex-test-prerequisites /opt/bex-test-wheels/ /wheels/\nCOPY --from=bex-test-prerequisites /opt/bex-test-tool-python.json /tool-python.json\n'
  need((G/'Dockerfile.prerequisites').read_text()==prereq and sha(prereq.encode())==sc['prerequisite_recipe_sha256'] and tool['virtualenv_cap_satisfied'] is True,'legacy same system-Python solve/platform/cap closure')
 else:
  prefix=canonical.decode().split('\nCOPY . .\n',1)[0].replace(marker.decode(), '\nFROM '+COMMON+' AS bex-test-prerequisites\n',1);bootstrap='\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && ';acquire='\nRUN --network=default /usr/local/bin/python -m pip wheel --disable-pip-version-check --wheel-dir /opt/bex-test-wheels poetry==2.1.3\nENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n';need(prefix.count(bootstrap)==1,'one default appPoetry bootstrap');prereq=prefix.replace(bootstrap,acquire+bootstrap.replace('RUN ','RUN --network=none ',1),1)+'\nCOPY check-tool.py /opt/bex-test-check-tool.py\nRUN --network=none /opt/bex/poetry/bin/python -I /opt/bex-test-check-tool.py > /opt/bex-test-tool-python.json\nFROM scratch\nCOPY --from=bex-test-prerequisites /opt/bex-test-wheels/ /wheels/\nCOPY --from=bex-test-prerequisites /opt/bex-test-tool-python.json /tool-python.json\n';need((G/'Dockerfile.prerequisites').read_text()==prereq and sha(prereq.encode())==sc['prerequisite_recipe_sha256'],'prerequisite recipe beyond fixed wheel acquisition')
 for phase in ('build','runtime'):
  proof=load(G/'qa-output'/('legacy-'+phase+'.json'));need(proof['phase']==phase and proof['automatic_poetry_run_executed'] is True and proof['pass'] is True,'complete phase proof');validate_app(proof['app'],version,False);validate_app(proof['project'],version,True);validate_tool(proof['private_tool'],version)
  for kind in ('app','project'):
   data=proof[kind];need(data['openssl'].startswith('OpenSSL ') and data['libpython_soname'].startswith('libpython'+line+'.so') and len(data['linked'])==3 and all('not found' not in text for text in data['linked'].values()),'actual SSL/FFI/SONAME/link closure')
 results.append({'profile':item['profile'],'arch':arch,'line':line,'version':version,'run_id':item['run_id'],'job_id':item['job_id'],'artifact_id':archive['id'],'files':len(actual),'manifest':identity(A/'MANIFEST.json'),'source_manifest':identity(A/'qualification-source.json'),'image_config_id':receipt['image_config_id'],'wheels':len(wm),'private_Poetry_python':tool['python'],'virtualenv':tool['virtualenv'],'build_runtime_auto_project_and_C_SSL_FFI_loader_proofs':'PASS','cleanup_actual_CI_log_bound':True})
report={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'outcome':'FIVE_DATED_9BC_PYTHON_CONTENT_AUDIT_PASS_ONLY','actual_generator_source':SOURCE,'baseline_source':baseline['source'],'actual_header_files_300_equal_baseline':True,'common':COMMON,'pure_original_validator_sources':[app_source,modern_tool_source,legacy_tool_source],'cases':results,'scope':'Only these five retained actual9bc outputs independentlyread: modern3.9/3.10/3.11 ARM and legacy3.7 AMD/ARM. Pure original validate_identity/validate_tool definitions extractedunchangedfromactualGit; no originalapp/main/observe/native/Go/Docker/network execution. Dated content audit only; no current-source baseline/family/production/hosted admission.'}
(R/'REVIEW.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'outcome':report['outcome'],'versions':[x['version'] for x in results],'report':identity(R/'REVIEW.json')},indent=2))
