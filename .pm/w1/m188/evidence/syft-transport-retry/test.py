from pathlib import Path
import hashlib,json,os,sys,subprocess,datetime,re
D=Path('/tmp/bex-m188-syft-retry-proposal');sha=lambda b:hashlib.sha256(b).hexdigest();before=(D/'native-base-image.before.yml').read_text();proposed=(D/'native-base-image.proposed.yml').read_text()
def extract(s):
 b=s.split('      - name: sign and attest the immutable candidate\n',1)[1].split('      - name: remove GHCR credentials\n',1)[0].split('        run: |\n',1)[1]
 assert all(x.startswith('          ') or not x for x in b.splitlines());return '\n'.join(x[10:] for x in b.splitlines())+'\n'
raw=extract(proposed);old=extract(before);pattern=r'retry_signing\(\) \{.*?^\}'
assert re.search(pattern,raw,re.S|re.M)[0]==re.search(pattern,old,re.S|re.M)[0]
assert raw.count('GODEBUG=http2client=0')==1 and 'timeout --kill-after=10s 30m syft "registry:$REF" --platform "linux/$arch" -o spdx-json > "$output"' in raw
step=raw.replace('${{ env.NATIVE_IMAGE }}','ghcr.io/bex-co/bex-cnb-run').replace('${{ matrix.name }}','ruby-3.2-bookworm').replace('${{ matrix.base }}','ruby:3.2-bookworm@sha256:'+'d'*64);assert '${{' not in step;(D/'actual-proposed-signing-step.raw.sh').write_text(raw);(D/'rendered-signing-step.sh').write_text(step)
B=D/'stubs';B.mkdir(exist_ok=True)
stub='#!'+sys.executable+'\n'+r'''
import pathlib,os,sys,json,subprocess,hashlib
root=pathlib.Path(os.environ['OWN_SYFT_CASE']);mode=os.environ['OWN_SYFT_MODE'];name=pathlib.Path(sys.argv[0]).name;args=sys.argv[1:];events=root/'events.jsonl';past=[json.loads(x) for x in events.read_text().splitlines()] if events.exists() else [];debug=os.environ.get('GODEBUG');arch=args[args.index('--platform')+1].split('/')[1] if '--platform' in args else None
def log(**more):
 with events.open('a') as f:f.write(json.dumps(dict(command=name,args=args,GODEBUG=debug,**more))+'\n')
if name=='sleep':
 log(report_exists={a:(root/('native-'+a+'.spdx.json')).exists() for a in ('amd64','arm64')});sys.exit(0)
if name=='timeout':
 command=args[2];assert args[0]=='--kill-after=10s' and args[1]==('30m' if command=='syft' else '5m');assert debug==('http2client=0' if command=='syft' else 'gctrace=0');log()
 if mode=='deadline_arm' and command=='syft' and arch=='arm64':print('partial deadline report');sys.exit(124)
 sys.exit(subprocess.run(args[2:]).returncode)
if name=='syft':
 assert args==['registry:'+os.environ['REF'],'--platform','linux/'+arch,'-o','spdx-json'];assert debug=='http2client=0';calls=sum(x['command']=='syft' and x.get('architecture')==arch for x in past);out=root/('native-'+arch+'.spdx.json');log(architecture=arch,entry_report_bytes=out.stat().st_size)
 fail=(mode=='transient_arm' and arch=='arm64' and calls==0) or (mode=='all_amd_fail' and arch=='amd64') or (mode=='all_arm_fail' and arch=='arm64')
 if fail:print('{"partial":');sys.exit(17)
 report={'spdxVersion':'SPDX-2.3','documentNamespace':'controlled-full-report-'+arch,'packages':[{'name':'retained-library','versionInfo':'old-version','externalRefs':[{'referenceType':'purl','referenceLocator':'pkg:generic/retained@old'}]}],'unfilteredExtraField':{'unfixed':True,'allFieldsRetained':[1,2,3]},'image':os.environ['REF'],'platform':'linux/'+arch}
 print(json.dumps(report,indent=2));sys.exit(0)
if name=='docker':log();assert args[:4]==['buildx','imagetools','create','--tag'] and args[-1]==os.environ['REF'];sys.exit(0)
assert name=='cosign' and debug=='gctrace=0';log();key=' '.join(args);count=sum(x['command']=='cosign' and ' '.join(x['args'])==key for x in past)
if args[0]=='signing-config':
 assert args[:3]==['signing-config','create','--with-default-rekor-v2'];assert '--out' in args
 if mode=='config_fail':sys.exit(18)
 pathlib.Path(args[args.index('--out')+1]).write_text('{"trusted-profile":"unchanged-Rekor-v2"}');sys.exit(0)
assert args[0] in ('sign','attest') and '--yes' in args and '--oidc-provider=github-actions' in args and args[-1]==os.environ['REF'] and pathlib.Path(args[args.index('--signing-config')+1]).is_file()
if mode=='sign_fail' and args[0]=='sign':sys.exit(19)
if args[0]=='attest':
 predicate=args[args.index('--predicate')+1];obj=json.loads(pathlib.Path(predicate).read_text())
 if predicate.endswith('.spdx.json'):
  assert args[args.index('--type')+1]=='spdxjson' and obj['unfilteredExtraField']['allFieldsRetained']==[1,2,3] and obj['unfilteredExtraField']['unfixed'] is True
 else:assert args[args.index('--type')+1]=='https://aquasecurity.github.io/trivy/attestation/vuln/v1' and obj=={'allFindings':['unfixed-critical-preserved']}
 if mode=='last_attest_fail' and predicate=='native-arm64.trivy.json':sys.exit(23)
sys.exit(0)
'''
for n in ('timeout','sleep','syft','cosign','docker'):
 p=B/n;p.write_text(stub);p.chmod(0o755)
results=[]
for mode,ok,syft_counts,tag_count,attest_count in [('healthy',True,(1,1),2,4),('transient_arm',True,(1,2),2,4),('all_amd_fail',False,(3,0),0,0),('all_arm_fail',False,(1,3),0,2),('deadline_arm',False,(1,0),0,2),('sign_fail',False,(0,0),0,0),('last_attest_fail',False,(1,1),0,6),('config_fail',False,(0,0),0,0)]:
 root=D/mode;root.mkdir(exist_ok=True);(root/'events.jsonl').unlink(missing_ok=True)
 for a in ('amd64','arm64'):
  output=root/('native-'+a+'.spdx.json');output.unlink(missing_ok=True);sentinel=root/('stale-'+a+'.txt');sentinel.write_text('owned stale report sentinel');output.symlink_to(sentinel)
  (root/('native-'+a+'.trivy.json')).write_text(json.dumps({'allFindings':['unfixed-critical-preserved']}))
 env={'PATH':str(B)+os.pathsep+os.environ['PATH'],'OWN_SYFT_CASE':str(root),'OWN_SYFT_MODE':mode,'GODEBUG':'gctrace=0','REF':'ghcr.io/bex-co/bex-cnb-run@sha256:'+'a'*64,'PRODUCER':'b'*40,'GITHUB_SHA':'c'*40,'GITHUB_STEP_SUMMARY':str(root/'summary.txt'),'RUNNER_TEMP':str(root)}
 p=subprocess.run(['bash','-e',str(D/'rendered-signing-step.sh')],cwd=root,env=env,capture_output=True,text=True,timeout=15);(root/'stdout.txt').write_text(p.stdout);(root/'stderr.txt').write_text(p.stderr);events=[json.loads(x) for x in (root/'events.jsonl').read_text().splitlines()];scans=[x for x in events if x['command']=='syft'];tags=[x for x in events if x['command']=='docker'];cosigns=[x for x in events if x['command']=='cosign'];attests=[x for x in cosigns if x['args'][0]=='attest'];timeouts=[x for x in events if x['command']=='timeout'];sleeps=[x for x in events if x['command']=='sleep'];actual=tuple(sum(x.get('architecture')==a for x in scans) for a in ('amd64','arm64'))
 assert (p.returncode==0)==ok,(mode,p.stderr);assert actual==syft_counts and len(tags)==tag_count and len(attests)==attest_count,(mode,actual,len(tags),len(attests));assert all(x['entry_report_bytes']==0 for x in scans);assert all(x['args'] in (['5'],['10']) for x in sleeps)
 for a in ('amd64','arm64'):assert (root/('stale-'+a+'.txt')).read_text()=='owned stale report sentinel'
 if mode in ('all_amd_fail','all_arm_fail','deadline_arm'):
  failed='amd64' if mode=='all_amd_fail' else 'arm64';assert not (root/('native-'+failed+'.spdx.json')).exists();assert [x['args'] for x in sleeps]==[['5'],['10']];assert all(not x['report_exists'][failed] for x in sleeps);assert sum(x['command']=='timeout' and x['args'][2]=='syft' and 'linux/'+failed in x['args'] for x in events)==3
 if mode=='transient_arm':assert [x['args'] for x in sleeps]==[['5']] and all(not x['report_exists']['arm64'] for x in sleeps)
 if mode in ('all_arm_fail','deadline_arm'):assert all('native-arm64.spdx.json' not in x['args'] and 'native-arm64.trivy.json' not in x['args'] for x in attests)
 if mode in ('sign_fail','config_fail','last_attest_fail'):assert [x['args'] for x in sleeps]==[['5'],['10']]
 for a in ('amd64','arm64'):
  out=root/('native-'+a+'.spdx.json')
  if out.exists() and not out.is_symlink():
   expected={'spdxVersion':'SPDX-2.3','documentNamespace':'controlled-full-report-'+a,'packages':[{'name':'retained-library','versionInfo':'old-version','externalRefs':[{'referenceType':'purl','referenceLocator':'pkg:generic/retained@old'}]}],'unfilteredExtraField':{'unfixed':True,'allFieldsRetained':[1,2,3]},'image':env['REF'],'platform':'linux/'+a};assert out.read_text()==json.dumps(expected,indent=2)+'\n'
  assert json.loads((root/('native-'+a+'.trivy.json')).read_text())=={'allFindings':['unfixed-critical-preserved']}
 results.append({'case':mode,'pass':True,'shell_exit':p.returncode,'syft_calls_by_architecture':dict(zip(('amd64','arm64'),actual)),'syft_outer_deadline_attempts':sum(x['args'][2]=='syft' for x in timeouts),'cosign_5m_attempts':len(cosigns),'attestation_attempts':len(attests),'discovery_tags':len(tags),'backoff_seconds':[int(x['args'][0]) for x in sleeps],'localized_Syft_GODEBUG_and_unchanged_cosign_environment':True,'owned_stale_sentinel_bytes_unchanged':True,'removed_stale_symlink_architectures':sorted({x['args'][x['args'].index('--platform')+1].split('/')[1] for x in timeouts if x['args'][2]=='syft'}),'full_successful_reports_and_all_Trivy_findings_unchanged':True,'event_log':str(root/'events.jsonl')})
repo=Path('.github/workflows/native-base-image.yml');assert sha(repo.read_bytes())=='9b56bb240a8838947bb71abc046244b6e4e8d4c49c882c632fa425d07111db91'
receipt={'schema':'bex.proposed-syft-bounded-retry-controls/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'method':'Actual proposed complete signing shell extracted, GitHub expression values rendered, executed underbash-e with recording Syft/Cosign/timeout/sleep/docker stubs; actual filesystem cleanup and redirected report bytes','baseline_repository_workflow_sha256':sha(repo.read_bytes()),'proposed_workflow_sha256':sha((D/'native-base-image.proposed.yml').read_bytes()),'patch_sha256':sha((D/'native-base-image.patch').read_bytes()),'raw_signing_shell_sha256':sha(raw.encode()),'rendered_signing_shell_sha256':sha(step.encode()),'test_source_sha256':sha(Path(__file__).read_bytes()),'retry_signing_body_identical_to_baseline':True,'cases':results,'limits':'Controlled failures/deadline exit124 verify exact forwarded30m/10sargs, counts/backoff, cleanup and failclosed ordering; no actual30minute wait/realSyft/Cosign/image/registry run claimed. Successful AMD attestations precede ARM failure; no corresponding ARM attestation or discovery occurs. Config/trust/5m signing behavior unchanged.','repository_workflow_unchanged':True,'repo_board_remote_mutations':False};p=D/'behavior-receipt.json';p.write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps({'receipt':str(p),'sha256':sha(p.read_bytes()),'proposed_sha256':receipt['proposed_workflow_sha256'],'patch_sha256':receipt['patch_sha256'],'tests_sha256':receipt['test_source_sha256'],'cases':results},indent=2))
