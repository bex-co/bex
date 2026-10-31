from pathlib import Path
import os,sys,json,re,subprocess,hashlib,datetime,shlex
ROOT=Path('/tmp/bex-m188-trivy-wrapper-merged-review');WF=Path('/Users/tianpan/projects/bex/.github/workflows/native-base-image.yml');text=WF.read_text();sha=lambda b:hashlib.sha256(b).hexdigest()
block=text.split('      - name: prepare bounded full-scan retries\n',1)[1].split('      - name: scan amd64 candidate\n',1)[0].split('        run: |\n',1)[1];assert all(l.startswith('          ') or not l for l in block.splitlines());prepare='\n'.join(l[10:] for l in block.splitlines())+'\n';(ROOT/'actual-workflow-prepare.sh').write_text(prepare)
runner=ROOT/'runner-temp';runner.mkdir(exist_ok=True);p=subprocess.run(['bash',str(ROOT/'actual-workflow-prepare.sh')],env={'PATH':os.environ['PATH'],'RUNNER_TEMP':str(runner)},capture_output=True,text=True);assert p.returncode==0,p.stderr;wrapper=runner/'native-trivy';assert wrapper.is_file() and os.access(wrapper,os.X_OK)
action=(ROOT/'official-action.yaml').read_text();chunk=action.split('        set_env_var_if_provided() {',1)[1].split('    - name: Run Trivy',1)[0];setup='set_env_var_if_provided() {'+chunk;setup='\n'.join(l[8:] if l.startswith('        ') else l for l in setup.splitlines())+'\n';(ROOT/'official-input-environment.sh').write_text(setup)
assert 'TRIVY_CMD="${TRIVY_CMD:-trivy}"' in (ROOT/'official-entrypoint.sh').read_text() and 'cmd=("$TRIVY_CMD" "$scanType" "$scanRef")' in (ROOT/'official-entrypoint.sh').read_text()
B=ROOT/'stubs';B.mkdir(exist_ok=True)
stub='#!'+sys.executable+'\n'+r'''
import os,sys,json,subprocess,pathlib
name=pathlib.Path(sys.argv[0]).name;args=sys.argv[1:];events=pathlib.Path(os.environ['OWN_TASK_EVENTS']);prior=[json.loads(x) for x in events.read_text().splitlines()] if events.exists() else []
def log(x):
 with events.open('a') as f:f.write(json.dumps(dict(command=name,args=args,**x))+'\n')
if name=='sleep': log({});sys.exit(0)
if name=='timeout':
 log({});assert args[:2]==['--kill-after=10s','21m'] and args[2]=='trivy'
 if os.environ.get('OWN_TASK_FORCE_TIMEOUT')=='yes':
  pathlib.Path(os.environ['TRIVY_OUTPUT']).write_text('partial timeout report');sys.exit(124)
 sys.exit(subprocess.run(args[2:]).returncode)
assert name=='trivy'
out=pathlib.Path(os.environ['TRIVY_OUTPUT']);calls=sum(x['command']=='trivy' for x in prior);seq=json.loads(os.environ['OWN_TASK_SCAN_EXITS']);code=seq[min(calls,len(seq)-1)];log({'existing_report_before_scan':out.exists(),'scan_environment':{k:v for k,v in os.environ.items() if k.startswith('TRIVY_') or k=='GODEBUG'},'exit_code':code})
if code:out.write_text('partial failed report '+str(calls))
else:out.write_text(json.dumps({'ArtifactName':args[1],'Metadata':{'ImageConfig':{'architecture':os.environ['TRIVY_PLATFORM'].split('/')[1]}},'Results':[{'Vulnerabilities':[{'VulnerabilityID':'CVE-test-retained','Status':'will_not_fix'}]}]}))
sys.exit(code)
'''
for n in ('trivy','timeout','sleep'):
 q=B/n;q.write_text(stub);q.chmod(0o755)
ref='ghcr.io/bex-co/bex-cnb-run@sha256:'+'a'*64
results=[]
def case(name,arch,exits,expected_rc,expected_calls,sleeps,stale=False,missing=False,forced_timeout=False,report_name=None):
 d=ROOT/name;d.mkdir(exist_ok=True);events=d/'events.jsonl';events.unlink(missing_ok=True);output=d/(report_name or ('native-'+arch+'.trivy.json'));output.unlink(missing_ok=True)
 if stale:output.write_text('preexisting stale report')
 env={'PATH':str(B)+os.pathsep+os.environ['PATH'],'LC_ALL':'C','TRIVY_CMD':str(wrapper),'TRIVY_PLATFORM':'linux/'+arch,'GODEBUG':'http2client=0','INPUT_SCAN_TYPE':'image','INPUT_IMAGE_REF':ref,'INPUT_SCAN_REF':'.','INPUT_TRIVYIGNORES':'','INPUT_GITHUB_PAT':'','INPUT_LIMIT_SEVERITIES_FOR_SARIF':'','OWN_TASK_EVENTS':str(events),'OWN_TASK_SCAN_EXITS':json.dumps(exits),'OWN_TASK_FORCE_TIMEOUT':'yes' if forced_timeout else 'no'}
 for var in set(re.findall(r'\$([A-Z_]+)',setup)):env.setdefault(var,'')
 env.update(INPUT_FORMAT='json',INPUT_TIMEOUT='20m',INPUT_EXIT_CODE='0',INPUT_OUTPUT='' if missing else str(output),INPUT_IGNORE_UNFIXED='false',INPUT_VULN_TYPE='os,library',INPUT_SEVERITY='UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL')
 p=subprocess.run(['bash',str(ROOT/'official-input-environment.sh')],cwd=d,env=env,capture_output=True,text=True,timeout=10);assert p.returncode==0,p.stderr
 p=subprocess.run(['bash',str(ROOT/'official-entrypoint.sh')],cwd=d,env=env,capture_output=True,text=True,timeout=10);(d/'stdout.txt').write_text(p.stdout);(d/'stderr.txt').write_text(p.stderr)
 ev=[json.loads(l) for l in events.read_text().splitlines()] if events.exists() else [];trivy=[x for x in ev if x['command']=='trivy'];timeouts=[x for x in ev if x['command']=='timeout'];actual_sleep=[x['args'] for x in ev if x['command']=='sleep'];assert (p.returncode==0)==(expected_rc==0),(name,p.returncode,p.stderr);assert len(trivy)==expected_calls,(name,trivy);assert actual_sleep==[[str(x)] for x in sleeps],(name,actual_sleep)
 for x in trivy:
  assert x['args']==['image',ref] and x['existing_report_before_scan'] is False
  e=x['scan_environment'];assert e['TRIVY_PLATFORM']=='linux/'+arch and e['TRIVY_FORMAT']=='json' and e['TRIVY_TIMEOUT']=='20m' and e['TRIVY_EXIT_CODE']=='0' and e['TRIVY_OUTPUT']==str(output) and e['TRIVY_CMD']==str(wrapper) and e['GODEBUG']=='http2client=0'
  assert not any(k in e for k in ('TRIVY_IGNORE_UNFIXED','TRIVY_SEVERITY','TRIVY_SKIP_DIRS','TRIVY_SKIP_FILES','TRIVY_SCANNERS','TRIVY_IGNOREFILE'))
 if expected_rc==0:
  assert output.is_file();report=json.loads(output.read_text());assert report['ArtifactName']==ref and report['Metadata']['ImageConfig']['architecture']==arch and report['Results'][0]['Vulnerabilities'][0]['Status']=='will_not_fix'
 else:assert not output.exists()
 if missing:assert 'missing scan report path' in p.stderr and not timeouts
 if forced_timeout:assert len(timeouts)==3
 result={'case':name,'architecture':arch,'action_entrypoint_exit':p.returncode,'scanner_calls':len(trivy),'outer_deadline_calls':len(timeouts),'backoff_seconds':[int(x[0]) for x in actual_sleep],'report_exists_after':output.exists(),'all_entry_scan_reports_absent':all(not x['existing_report_before_scan'] for x in trivy),'args_and_full_scan_environment_preserved':True,'result':'PASS','event_log':str(events),'stdout':str(d/'stdout.txt'),'stderr':str(d/'stderr.txt')};results.append(result)
case('amd64-success','amd64',[0],0,1,[])
case('arm64-success-spaced-output','arm64',[0],0,1,[],report_name='native arm64 report.json')
case('partial-failure-then-success','amd64',[17,0],0,2,[5],stale=True)
case('all-three-fail','arm64',[17,23,42],1,3,[5,10],stale=True)
case('missing-report-path','amd64',[0],1,0,[],missing=True)
case('outer-timeout-all-three','amd64',[0],1,0,[5,10],stale=True,forced_timeout=True)
# Workflow integration: same pinned action, both hooks, all existing report options unchanged.
for arch in ('amd64','arm64'):
 scan=text.split('      - name: scan '+arch+' candidate\n',1)[1].split('      - ',1)[0]
 for phrase in ('aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25','TRIVY_PLATFORM: linux/'+arch,'GODEBUG: http2client=0','TRIVY_CMD: ${{ runner.temp }}/native-trivy','cache: false','format: json','output: native-'+arch+'.trivy.json','timeout: "20m"','exit-code: "0"'):assert phrase in scan,phrase
assert WF.read_text()==text,'Workflow changed while merged review was running'
receipt={'schema':'bex.independent-pinned-trivy-wrapper-integration-test/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'result':'PASS all6 cases through actual pinned official environment generator and entrypoint; no workflow defects found in reviewed delta','workflow':str(WF),'workflow_sha256':sha(WF.read_bytes()),'http2_workaround_env_forwarded':'GODEBUG=http2client=0 verified at actual scanner stub for both architectures and every retry','wrapper_sha256':sha(wrapper.read_bytes()),'actual_prepare_step_sha256':sha((ROOT/'actual-workflow-prepare.sh').read_bytes()),'official_action_commit':'ed142fd0673e97e23eac54620cfb913e5ce36c25','official_entrypoint_source':'https://raw.githubusercontent.com/aquasecurity/trivy-action/ed142fd0673e97e23eac54620cfb913e5ce36c25/entrypoint.sh','official_entrypoint_sha256':sha((ROOT/'official-entrypoint.sh').read_bytes()),'official_action_source':'https://raw.githubusercontent.com/aquasecurity/trivy-action/ed142fd0673e97e23eac54620cfb913e5ce36c25/action.yaml','official_action_sha256':sha((ROOT/'official-action.yaml').read_bytes()),'official_hook':'entrypoint TRIVY_CMD defaults to trivy then invokes quoted cmd array; action input environment generator exports exact output/timeout/exit-code before entrypoint; inherited TRIVY_CMD and platform are preserved','tests':results,'quality_findings':[],'limits':'Controlled scanner/timeout/sleep stubs verify integration, cleanup, argv/environment, retry count/backoff and exit propagation. No real scan, image pull, vulnerability engine result, actual21-minute wall clock or remote workflow execution claimed. Full20m scan behavior and unchanged findings are preserved by identical forwarded environment/arguments; only successful scanner exit permits success.','repository_edits':False,'remote_mutations':False};p=ROOT/'receipt.json';p.write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps({'receipt':str(p),'sha256':sha(p.read_bytes()),'official_entrypoint_sha256':receipt['official_entrypoint_sha256'],'wrapper_sha256':receipt['wrapper_sha256'],'cases':results},indent=2))
